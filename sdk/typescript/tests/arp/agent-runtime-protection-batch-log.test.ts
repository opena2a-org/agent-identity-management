import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import * as fs from 'fs';
import * as os from 'os';
import * as path from 'path';
import type { ARPConfig, ARPEvent, IntelligenceConfig } from '../../src/arp/types';

/**
 * With batching on, `analyze()` returns null for an event it queues for an L2 batch, and the
 * ARP logs the event then, before the batch is sent. Once the batch's answer is applied to the
 * event, the ARP logs the event again, and `getEvents()` returns that later record in place of
 * the first, so what the event log shows carries the batch's verdict.
 *
 * Measured with a stub adapter before this change: an event emitted with batching on and
 * flushed by stop() was returned by emit() with `classifiedBy: 'L2-llm'`, severity `critical`
 * and a `kill` recommendation, while `getEvents()` returned it with `classifiedBy: 'L0-rules'`,
 * severity `high` and no `llmAssessment`, and events.jsonl held one line for it.
 */

const stub = vi.hoisted(() => {
  const state = {
    calls: 0,
    fail: false,
    /** The answer to a batch call. */
    batchAnswer: '',
    /** The answer to a single-event call. */
    singleAnswer: '',
  };
  const adapter = {
    name: 'stub',
    async assess(prompt: string): Promise<{ content: string; inputTokens: number; outputTokens: number; model: string }> {
      state.calls += 1;
      await new Promise<void>((resolve) => setImmediate(resolve));
      if (state.fail) throw new Error('stub adapter: the call failed');
      const content = prompt.startsWith('BATCH SECURITY ASSESSMENT') ? state.batchAnswer : state.singleAnswer;
      return { content, inputTokens: 150, outputTokens: 50, model: 'stub' };
    },
    estimateCost(inputTokens: number, outputTokens: number): number {
      return (inputTokens + outputTokens) / 100000;
    },
    async healthCheck(): Promise<boolean> {
      return true;
    },
  };
  return { state, adapter };
});

vi.mock('../../src/arp/intelligence/adapters', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../src/arp/intelligence/adapters')>()),
  createAdapter: () => stub.adapter,
}));

import { AgentRuntimeProtection } from '../../src/arp';
import { LocalLogger } from '../../src/arp/reporting/local-log';

const scratchDirs: string[] = [];

/** A scratch directory named after this test, removed after each test. */
function scratchDir(): string {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'arp-batch-log-'));
  scratchDirs.push(dir);
  return dir;
}

function makeArp(intelligence: IntelligenceConfig = {}): { arp: AgentRuntimeProtection; dataDir: string } {
  const dataDir = scratchDir();
  const config: ARPConfig = {
    agentName: 'batch-log-test',
    dataDir,
    monitors: {
      process: { enabled: false },
      network: { enabled: false },
      filesystem: { enabled: false },
    },
    intelligence: {
      enabled: true,
      adapter: 'anthropic',
      budgetUsd: 100,
      enableBatching: true,
      batchWindowMs: 3600000,
      // The runtime twin writes its own log under the home directory; these tests read dataDir.
      runtimeTwin: { enabled: false },
      ...intelligence,
    },
  };
  return { arp: new AgentRuntimeProtection(config), dataDir };
}

function flagged(n: number, severity: ARPEvent['severity'] = 'high'): Omit<ARPEvent, 'id' | 'timestamp' | 'classifiedBy'> {
  return {
    source: 'network',
    category: 'anomaly',
    severity,
    description: `Outbound connection ${n} to an unexpected host`,
    data: {},
  };
}

/** Every line of events.jsonl, in the order it was written. */
function logLines(dataDir: string): ARPEvent[] {
  const file = path.join(dataDir, 'events.jsonl');
  if (!fs.existsSync(file)) return [];
  return fs
    .readFileSync(file, 'utf-8')
    .split('\n')
    .filter((l) => l.trim())
    .map((l) => JSON.parse(l) as ARPEvent);
}

/** The part of an event the batch's answer changes. */
function verdict(e: ARPEvent): { classifiedBy: string; severity: string; category: string; recommendation?: string } {
  return {
    classifiedBy: e.classifiedBy,
    severity: e.severity,
    category: e.category,
    recommendation: e.llmAssessment?.recommendation,
  };
}

const KILLED = { classifiedBy: 'L2-llm', severity: 'critical', category: 'threat', recommendation: 'kill' };
const UNASSESSED = { classifiedBy: 'L0-rules', severity: 'high', category: 'anomaly', recommendation: undefined };

beforeEach(() => {
  stub.state.calls = 0;
  stub.state.fail = false;
  stub.state.batchAnswer = '';
  stub.state.singleAnswer = '';
});

afterEach(() => {
  for (const dir of scratchDirs.splice(0)) {
    fs.rmSync(dir, { recursive: true, force: true });
  }
});

describe('AgentRuntimeProtection logs a batched event with its L2 verdict', () => {
  it('logs the event again once its batch is answered, and getEvents() returns it with the verdict', async () => {
    stub.state.batchAnswer = '1: KILL - exfiltration';
    const { arp, dataDir } = makeArp();
    await arp.start();
    const event = await arp.getEngine().emit(flagged(1));
    // Queued, not yet sent: the log holds the event as analyze() left it.
    expect(stub.state.calls).toBe(0);
    expect(arp.getEvents().map(verdict)).toEqual([UNASSESSED]);

    await arp.stop();

    expect(stub.state.calls).toBe(1);
    expect(verdict(event)).toEqual(KILLED);
    const events = arp.getEvents();
    expect(events.map((e) => e.id)).toEqual([event.id]);
    expect(verdict(events[0])).toEqual(KILLED);
    expect(events[0].llmAssessment).toEqual(event.llmAssessment);

    // events.jsonl is appended to: the first line stays, and the later one carries the verdict.
    const lines = logLines(dataDir);
    expect(lines.map((e) => e.id)).toEqual([event.id, event.id]);
    expect(lines.map(verdict)).toEqual([UNASSESSED, KILLED]);
  });

  it('logs an event assessed by its own call once, with batching off', async () => {
    stub.state.singleAnswer = 'CONSISTENT: NO\nCONFIDENCE: 0.9\nREASONING: exfiltration\nACTION: KILL';
    const { arp, dataDir } = makeArp({ enableBatching: false });
    await arp.start();
    const event = await arp.getEngine().emit(flagged(1));
    await arp.stop();

    expect(verdict(event)).toEqual(KILLED);
    expect(arp.getEvents().map(verdict)).toEqual([KILLED]);
    expect(logLines(dataDir).map((e) => e.id)).toEqual([event.id]);
  });

  it('returns the events of one batch once each, in the order they were emitted, each with its verdict', async () => {
    stub.state.batchAnswer = ['1: KILL - exfiltration', '2: ALLOW - a host the agent uses', '3: PAUSE - unexpected'].join('\n');
    const { arp, dataDir } = makeArp();
    await arp.start();
    const emitted = [
      await arp.getEngine().emit(flagged(1)),
      await arp.getEngine().emit(flagged(2)),
      await arp.getEngine().emit(flagged(3)),
    ];
    await arp.stop();

    expect(stub.state.calls).toBe(1);
    const events = arp.getEvents();
    expect(events.map((e) => e.id)).toEqual(emitted.map((e) => e.id));
    expect(events.map(verdict)).toEqual([
      KILLED,
      { classifiedBy: 'L2-llm', severity: 'high', category: 'anomaly', recommendation: 'allow' },
      { classifiedBy: 'L2-llm', severity: 'high', category: 'violation', recommendation: 'pause' },
    ]);
    expect(logLines(dataDir)).toHaveLength(6);
  });

  it('counts events, not log lines, in getEvents(limit)', async () => {
    stub.state.batchAnswer = '1: KILL - exfiltration';
    stub.state.singleAnswer = 'CONSISTENT: YES\nCONFIDENCE: 0.8\nREASONING: expected\nACTION: ALLOW';
    const { arp, dataDir } = makeArp();
    await arp.start();
    // A high event is queued for the batch; a critical one is assessed by its own call at once.
    const batched = await arp.getEngine().emit(flagged(1));
    const single = await arp.getEngine().emit(flagged(2, 'critical'));
    await arp.stop();

    // Written: the batched event, the critical event, then the batched event with its verdict.
    expect(logLines(dataDir).map((e) => e.id)).toEqual([batched.id, single.id, batched.id]);
    expect(arp.getEvents().map((e) => e.id)).toEqual([batched.id, single.id]);
    expect(arp.getEvents(1).map((e) => e.id)).toEqual([single.id]);
    expect(verdict(arp.getEvents(2)[0])).toEqual(KILLED);
  });

  it('logs a batched event once when its batch call fails', async () => {
    stub.state.fail = true;
    const { arp, dataDir } = makeArp();
    await arp.start();
    const event = await arp.getEngine().emit(flagged(1));
    await arp.stop();

    expect(stub.state.calls).toBe(1);
    expect(verdict(event)).toEqual(UNASSESSED);
    expect(arp.getEvents().map(verdict)).toEqual([UNASSESSED]);
    expect(logLines(dataDir).map((e) => e.id)).toEqual([event.id]);
  });

  it('logs a batched event once when the budget refuses its batch call', async () => {
    stub.state.batchAnswer = '1: KILL - exfiltration';
    // Room for the batch's estimate when the event is queued; none left when the batch is sent.
    const { arp, dataDir } = makeArp({ budgetUsd: 100, maxCallsPerHour: 1 });
    await arp.start();
    const event = await arp.getEngine().emit(flagged(1));
    // A critical event takes the hour's one call before the batch is sent.
    await arp.getEngine().emit(flagged(2, 'critical'));
    await arp.stop();

    expect(stub.state.calls).toBe(1);
    expect(verdict(event)).toEqual(UNASSESSED);
    expect(logLines(dataDir).filter((e) => e.id === event.id)).toHaveLength(1);
  });
});

describe('LocalLogger reads one record per event', () => {
  function event(id: string, severity: ARPEvent['severity']): ARPEvent {
    return {
      id,
      timestamp: new Date().toISOString(),
      source: 'network',
      category: 'anomaly',
      severity,
      description: `event ${id}`,
      data: {},
      classifiedBy: 'L0-rules',
    };
  }

  it('returns an event logged twice as its later record, where its first record was', () => {
    const logger = new LocalLogger(scratchDir());
    logger.logEvent(event('a', 'high'));
    logger.logEvent(event('b', 'low'));
    logger.logEvent({ ...event('a', 'critical'), classifiedBy: 'L2-llm' });

    expect(logger.readEvents().map((e) => [e.id, e.severity, e.classifiedBy])).toEqual([
      ['a', 'critical', 'L2-llm'],
      ['b', 'low', 'L0-rules'],
    ]);
    expect(logger.tail().map((e) => e.id)).toEqual(['a', 'b']);
  });

  it('applies a limit to events, not to lines', () => {
    const logger = new LocalLogger(scratchDir());
    logger.logEvent(event('a', 'high'));
    logger.logEvent(event('b', 'low'));
    logger.logEvent(event('a', 'critical'));

    expect(logger.readEvents(1).map((e) => e.id)).toEqual(['b']);
    expect(logger.tail(1).map((e) => e.id)).toEqual(['b']);
    expect(logger.readEvents(2).map((e) => [e.id, e.severity])).toEqual([
      ['a', 'critical'],
      ['b', 'low'],
    ]);
  });

  it('keeps every record that has no id', () => {
    const logger = new LocalLogger(scratchDir());
    const withoutId: Partial<ARPEvent> = event('x', 'low');
    delete withoutId.id;
    logger.logEvent(withoutId as ARPEvent);
    logger.logEvent(withoutId as ARPEvent);

    expect(logger.readEvents()).toHaveLength(2);
    expect(logger.tail()).toHaveLength(2);
  });
});
