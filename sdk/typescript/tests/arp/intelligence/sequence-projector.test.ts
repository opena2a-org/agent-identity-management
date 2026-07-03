import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import * as os from 'os';
import * as path from 'path';
import * as fs from 'fs';
import {
  projectSequences,
  projectSequencesFromFile,
  SequenceProjector,
  type LoggedActionEvent,
} from '../../../src/arp/intelligence/sequence-projector';

/**
 * Unit coverage for the offline SequenceProjector.
 *
 * The projector assembles per-(agentId, sessionId) ordered in-scope action
 * sequences from the append-only event log, projecting each event to the
 * consistency-model tuple. It is pure and offline — these tests pin grouping,
 * ordering, the in-scope filter, field projection, and the provenance-by-
 * ordering taint channel.
 */

function ev(overrides: Partial<LoggedActionEvent>): LoggedActionEvent {
  return {
    agentId: 'agent-1',
    sessionId: 'sess-1',
    sequenceNum: 1,
    eventType: 'TOOL_CALL',
    capability: 'data:read',
    toolName: 'http_get',
    argHash: 'abc123',
    timestampDelta: 100,
    responseSize: 0,
    responseCode: 0,
    l0Decision: 'allow',
    ...overrides,
  };
}

describe('SequenceProjector — grouping and ordering', () => {
  it('groups by (agentId, sessionId) and sorts by sequenceNum', () => {
    const records: LoggedActionEvent[] = [
      ev({ agentId: 'a', sessionId: 's1', sequenceNum: 3, capability: 'c' }),
      ev({ agentId: 'a', sessionId: 's1', sequenceNum: 1, capability: 'c' }),
      ev({ agentId: 'a', sessionId: 's2', sequenceNum: 1, capability: 'c' }),
      ev({ agentId: 'a', sessionId: 's1', sequenceNum: 2, capability: 'c' }),
      ev({ agentId: 'b', sessionId: 's1', sequenceNum: 1, capability: 'c' }),
    ];

    const seqs = projectSequences(records, { grantedScope: new Set(['c']) });

    expect(seqs).toHaveLength(3);
    const s1 = seqs.find((s) => s.agentId === 'a' && s.sessionId === 's1')!;
    expect(s1.actions.map((a) => a.seq)).toEqual([1, 2, 3]);
  });

  it('keeps duplicate sequence numbers in arrival order (stable tiebreak)', () => {
    const records: LoggedActionEvent[] = [
      ev({ sequenceNum: 1, toolName: 'first', capability: 'c' }),
      ev({ sequenceNum: 1, toolName: 'second', capability: 'c' }),
    ];
    const seqs = projectSequences(records, { grantedScope: new Set(['c']) });
    expect(seqs[0].actions.map((a) => a.tool)).toEqual(['first', 'second']);
  });
});

describe('SequenceProjector — in-scope filter', () => {
  it('excludes non-allow events (block/alert are a different signal)', () => {
    const records: LoggedActionEvent[] = [
      ev({ sequenceNum: 1, capability: 'c', l0Decision: 'allow' }),
      ev({ sequenceNum: 2, capability: 'c', l0Decision: 'block' }),
      ev({ sequenceNum: 3, capability: 'c', l0Decision: 'alert' }),
    ];
    const seqs = projectSequences(records, { grantedScope: new Set(['c']) });
    expect(seqs[0].actions.map((a) => a.seq)).toEqual([1]);
  });

  it('excludes allowed events whose capability is outside the granted scope', () => {
    const records: LoggedActionEvent[] = [
      ev({ sequenceNum: 1, capability: 'granted' }),
      ev({ sequenceNum: 2, capability: 'not-granted' }),
    ];
    const seqs = projectSequences(records, { grantedScope: new Set(['granted']) });
    expect(seqs[0].actions.map((a) => a.capability)).toEqual(['granted']);
  });

  it("'*' in the granted scope matches any capability", () => {
    const records: LoggedActionEvent[] = [
      ev({ sequenceNum: 1, capability: 'anything' }),
      ev({ sequenceNum: 2, capability: 'else' }),
    ];
    const seqs = projectSequences(records, { grantedScope: new Set(['*']) });
    expect(seqs[0].actions).toHaveLength(2);
  });

  it('with no granted scope, in-scope degrades to allow-only', () => {
    const records: LoggedActionEvent[] = [
      ev({ sequenceNum: 1, capability: 'x', l0Decision: 'allow' }),
      ev({ sequenceNum: 2, capability: 'y', l0Decision: 'block' }),
    ];
    const seqs = projectSequences(records);
    expect(seqs[0].actions.map((a) => a.seq)).toEqual([1]);
  });
});

describe('SequenceProjector — field projection', () => {
  it('projects enrichment fields and defaults absent ones to null', () => {
    const records: LoggedActionEvent[] = [
      ev({
        sequenceNum: 7,
        capability: 'data:export',
        toolName: 'http_post',
        argHash: 'deadbeef',
        classification: 'network-egress',
        objective: 'dataops:export',
        dataScopeTouched: 'customer.billing',
        egressTargetHash: 'sha256host',
        volumeBytes: 81920,
        timestampDelta: 1180,
      }),
    ];
    const [seq] = projectSequences(records, { grantedScope: new Set(['data:export']) });
    expect(seq.actions[0]).toMatchObject({
      seq: 7,
      capability: 'data:export',
      tool: 'http_post',
      argHash: 'deadbeef',
      objective: 'dataops:export',
      classification: 'network-egress',
      dataScopeTouched: 'customer.billing',
      egressTargetHash: 'sha256host',
      volumeBytes: 81920,
      dtMs: 1180,
      inScope: true,
    });
  });

  it('defaults classification / egress / objective to null on a bare twin record', () => {
    const [seq] = projectSequences([ev({ capability: 'c', responseSize: 42 })], {
      grantedScope: new Set(['c']),
    });
    const a = seq.actions[0];
    expect(a.classification).toBeNull();
    expect(a.egressTargetHash).toBeNull();
    expect(a.objective).toBeNull();
    expect(a.dataScopeTouched).toBeNull();
    // volumeBytes falls back to responseSize when no explicit volume is logged.
    expect(a.volumeBytes).toBe(42);
  });

  it('capabilityToObjective overrides the logged objective', () => {
    const [seq] = projectSequences(
      [ev({ capability: 'data:read', objective: 'stale' })],
      {
        grantedScope: new Set(['data:read']),
        capabilityToObjective: (c) => (c === 'data:read' ? 'dataops:read' : null),
      },
    );
    expect(seq.actions[0].objective).toBe('dataops:read');
  });

  it('stamps vocabVersion and purposeRef on every sequence', () => {
    const [seq] = projectSequences([ev({ capability: 'c' })], {
      grantedScope: new Set(['c']),
      vocabVersion: 'v2',
      purposeRef: 'did:opena2a:purpose:42',
    });
    expect(seq.vocabVersion).toBe('v2');
    expect(seq.purposeRef).toBe('did:opena2a:purpose:42');
  });
});

describe('SequenceProjector — provenance-by-ordering taint', () => {
  it('a sink action records the seqs of prior source actions in the session', () => {
    const records: LoggedActionEvent[] = [
      // seq 1: a read that returns data → source
      ev({ sequenceNum: 1, eventType: 'MEMORY_READ', capability: 'c', responseSize: 200 }),
      // seq 2: an external call that returns data → also a source
      ev({ sequenceNum: 2, eventType: 'EXTERNAL_CALL', capability: 'c', responseSize: 50 }),
      // seq 3: a write → sink; its taint candidates are seqs 1 and 2
      ev({ sequenceNum: 3, eventType: 'MEMORY_WRITE', capability: 'c', responseSize: 0 }),
    ];
    const [seq] = projectSequences(records, { grantedScope: new Set(['c']) });
    const sink = seq.actions.find((a) => a.seq === 3)!;
    expect(sink.taintSourceSeqs).toEqual([1, 2]);
  });

  it('a source action is never its own taint source', () => {
    const records: LoggedActionEvent[] = [
      ev({ sequenceNum: 1, eventType: 'EXTERNAL_CALL', capability: 'c', responseSize: 10 }),
    ];
    const [seq] = projectSequences(records, { grantedScope: new Set(['c']) });
    // EXTERNAL_CALL is a sink type, but there is no PRIOR source, so empty.
    expect(seq.actions[0].taintSourceSeqs).toEqual([]);
  });

  it('non-sink actions carry an empty taint set', () => {
    const records: LoggedActionEvent[] = [
      ev({ sequenceNum: 1, eventType: 'MEMORY_READ', capability: 'c', responseSize: 100 }),
      ev({ sequenceNum: 2, eventType: 'TOOL_CALL', capability: 'c', responseSize: 0 }),
    ];
    const [seq] = projectSequences(records, { grantedScope: new Set(['c']) });
    expect(seq.actions.find((a) => a.seq === 2)!.taintSourceSeqs).toEqual([]);
  });
});

describe('SequenceProjector — robustness', () => {
  it('skips malformed records without aborting the projection', () => {
    const records = [
      ev({ sequenceNum: 1, capability: 'c' }),
      { agentId: '', sessionId: 's', sequenceNum: 2 } as unknown as LoggedActionEvent,
      { foo: 'bar' } as unknown as LoggedActionEvent,
      ev({ sequenceNum: 3, capability: 'c' }),
    ];
    const [seq] = projectSequences(records, { grantedScope: new Set(['c']) });
    expect(seq.actions.map((a) => a.seq)).toEqual([1, 3]);
  });
});

describe('projectSequencesFromFile', () => {
  let tmpDir: string;
  beforeEach(() => {
    tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'seq-proj-'));
  });
  afterEach(() => {
    fs.rmSync(tmpDir, { recursive: true, force: true });
  });

  it('reads a JSONL log, skipping blank and malformed lines', async () => {
    const logPath = path.join(tmpDir, 'events.jsonl');
    const lines = [
      JSON.stringify(ev({ sequenceNum: 1, capability: 'c' })),
      '',
      '{ not valid json',
      JSON.stringify(ev({ sequenceNum: 2, capability: 'c' })),
    ];
    fs.writeFileSync(logPath, lines.join('\n') + '\n');

    const seqs = await projectSequencesFromFile(logPath, {
      grantedScope: new Set(['c']),
    });
    expect(seqs).toHaveLength(1);
    expect(seqs[0].actions.map((a) => a.seq)).toEqual([1, 2]);
  });

  it('returns an empty array for a missing log file', async () => {
    const seqs = await projectSequencesFromFile(path.join(tmpDir, 'absent.jsonl'));
    expect(seqs).toEqual([]);
  });

  it('SequenceProjector class binds options', async () => {
    const logPath = path.join(tmpDir, 'events.jsonl');
    fs.writeFileSync(logPath, JSON.stringify(ev({ sequenceNum: 1, capability: 'c' })) + '\n');
    const projector = new SequenceProjector({ grantedScope: new Set(['c']), vocabVersion: 'v1' });
    const seqs = await projector.projectFromFile(logPath);
    expect(seqs[0].vocabVersion).toBe('v1');
  });
});
