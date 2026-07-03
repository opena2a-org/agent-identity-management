import { describe, it, expect, afterEach } from 'vitest';
import * as os from 'os';
import * as path from 'path';
import * as fs from 'fs';
import * as net from 'net';
import {
  BEHAVIORAL_RISK_WIRE_VERSION,
  CIRCUIT_BREAKER_COOLDOWN_MS,
  CIRCUIT_BREAKER_THRESHOLD,
  InProcessBehavioralRiskSource,
  UnixSocketBehavioralRiskSource,
  defaultBehavioralRiskSocketPath,
  type BehavioralRiskScore,
  type BehavioralRiskScoreable,
} from '../../../src/arp/intelligence/behavioral-risk';
import { startBehavioralRiskServer } from '../../../src/arp/intelligence/behavioral-risk-server';
import type { ARPEvent } from '../../../src/arp/types';
import { RuntimeTwin } from '../../../src/arp/intelligence/runtime-twin';

/**
 * Coverage for the behavioral risk IPC surface.
 *
 * The tests exercise both the in-process source and the unix socket
 * source plus the server that answers its requests. Error paths are
 * asserted by code, not by string matching, so a change to a reason
 * string cannot silently flip a test from passing to "unavailable
 * because of a different reason".
 */

function makeEvent(overrides: Partial<ARPEvent> = {}): ARPEvent {
  return {
    id: overrides.id ?? `evt-${Math.random().toString(36).slice(2)}`,
    timestamp: new Date().toISOString(),
    source: 'prompt',
    category: 'normal',
    severity: 'info',
    description: 'test event',
    data: overrides.data ?? { capability: 'test-capability' },
    classifiedBy: 'L0-rules',
    ...overrides,
  };
}

class StubTwin implements BehavioralRiskScoreable {
  constructor(
    private readonly behavior:
      | { kind: 'score'; score: BehavioralRiskScore }
      | { kind: 'null' }
      | { kind: 'throw'; message: string },
  ) {}

  scoreARPEvent(_event: ARPEvent): BehavioralRiskScore | null {
    if (this.behavior.kind === 'score') return this.behavior.score;
    if (this.behavior.kind === 'null') return null;
    throw new Error(this.behavior.message);
  }
}

function uniqueSocketPath(name: string): string {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'arp-risk-ipc-'));
  return path.join(dir, `${name}.sock`);
}

describe('defaultBehavioralRiskSocketPath', () => {
  it('sanitizes agentId and places the socket under ~/.opena2a/arp on posix', () => {
    if (process.platform === 'win32') return;
    const p = defaultBehavioralRiskSocketPath('weird/agent name');
    expect(p).toContain('behavioral-risk-weird_agent_name.sock');
    expect(p).toContain(path.join(os.homedir(), '.opena2a', 'arp'));
  });
});

describe('InProcessBehavioralRiskSource', () => {
  it('returns ok with the score the twin produced', async () => {
    const twin = new StubTwin({
      kind: 'score',
      score: { score: 0.42, action: 'alert', reason: 'unusual' },
    });
    const src = new InProcessBehavioralRiskSource(twin, 'test-inproc');
    const r = await src.getBehavioralRiskSignal(makeEvent(), 25);
    expect(r.status).toBe('ok');
    if (r.status === 'ok') {
      expect(r.signal.score).toBe(0.42);
      expect(r.signal.action).toBe('alert');
      expect(r.signal.source).toBe('test-inproc');
      expect(r.signal.computedAtMs).toBeGreaterThan(0);
    }
  });

  it('maps null from the twin to NOT_READY', async () => {
    const src = new InProcessBehavioralRiskSource(new StubTwin({ kind: 'null' }));
    const r = await src.getBehavioralRiskSignal(makeEvent(), 25);
    expect(r.status).toBe('unavailable');
    if (r.status === 'unavailable') expect(r.code).toBe('NOT_READY');
  });

  it('wraps a thrown twin exception as INTERNAL_ERROR and never throws', async () => {
    const src = new InProcessBehavioralRiskSource(
      new StubTwin({ kind: 'throw', message: 'baseline file corrupt' }),
    );
    const r = await src.getBehavioralRiskSignal(makeEvent(), 25);
    expect(r.status).toBe('unavailable');
    if (r.status === 'unavailable') {
      expect(r.code).toBe('INTERNAL_ERROR');
      expect(r.reason).toContain('baseline file corrupt');
    }
  });
});

describe('UnixSocketBehavioralRiskSource round-trip via real server', () => {
  let handle: { close: () => Promise<void>; socketPath: string } | null = null;

  afterEach(async () => {
    if (handle) {
      await handle.close();
      handle = null;
    }
  });

  it('delivers the server score on the happy path', async () => {
    const socketPath = uniqueSocketPath('ok');
    handle = await startBehavioralRiskServer({
      twin: new StubTwin({
        kind: 'score',
        score: { score: 0.72, action: 'throttle', reason: 'suspicious burst' },
      }),
      socketPath,
      sourceName: 'test-server',
    });
    const src = new UnixSocketBehavioralRiskSource(socketPath);
    const r = await src.getBehavioralRiskSignal(makeEvent(), 500);
    expect(r.status).toBe('ok');
    if (r.status === 'ok') {
      expect(r.signal.score).toBeCloseTo(0.72);
      expect(r.signal.action).toBe('throttle');
      expect(r.signal.source).toBe('test-server');
    }
  });

  it('surfaces NOT_READY when the server twin returns null', async () => {
    const socketPath = uniqueSocketPath('notready');
    handle = await startBehavioralRiskServer({
      twin: new StubTwin({ kind: 'null' }),
      socketPath,
    });
    const src = new UnixSocketBehavioralRiskSource(socketPath);
    const r = await src.getBehavioralRiskSignal(makeEvent(), 500);
    expect(r.status).toBe('unavailable');
    if (r.status === 'unavailable') expect(r.code).toBe('NOT_READY');
  });

  it('surfaces INTERNAL_ERROR when the server twin throws', async () => {
    const socketPath = uniqueSocketPath('throw');
    handle = await startBehavioralRiskServer({
      twin: new StubTwin({ kind: 'throw', message: 'oops' }),
      socketPath,
    });
    const src = new UnixSocketBehavioralRiskSource(socketPath);
    const r = await src.getBehavioralRiskSignal(makeEvent(), 500);
    expect(r.status).toBe('unavailable');
    if (r.status === 'unavailable') expect(r.code).toBe('INTERNAL_ERROR');
  });

  it('rejects a mismatched wire version as PARSE_ERROR', async () => {
    // Hand-roll a server that speaks version 99 to exercise the client
    // parser guard. The real server only emits version 1.
    const socketPath = uniqueSocketPath('badversion');
    const srv = net.createServer((socket) => {
      socket.once('data', () => {
        socket.write(
          JSON.stringify({
            kind: 'risk_signal_response',
            version: 99,
            score: 0.1,
            action: 'allow',
          }) + '\n',
        );
        socket.end();
      });
    });
    await new Promise<void>((res) => srv.listen(socketPath, () => res()));
    handle = {
      socketPath,
      close: async () => {
        await new Promise<void>((r) => srv.close(() => r()));
      },
    };
    const src = new UnixSocketBehavioralRiskSource(socketPath);
    const r = await src.getBehavioralRiskSignal(makeEvent(), 500);
    expect(r.status).toBe('unavailable');
    if (r.status === 'unavailable') expect(r.code).toBe('PARSE_ERROR');
  });

  it('rejects a non-finite score as PARSE_ERROR even on a correct kind', async () => {
    const socketPath = uniqueSocketPath('badscore');
    const srv = net.createServer((socket) => {
      socket.once('data', () => {
        socket.write(
          JSON.stringify({
            kind: 'risk_signal_response',
            version: BEHAVIORAL_RISK_WIRE_VERSION,
            score: Number.NaN,
            action: 'allow',
          }) + '\n',
        );
        socket.end();
      });
    });
    await new Promise<void>((res) => srv.listen(socketPath, () => res()));
    handle = {
      socketPath,
      close: async () => {
        await new Promise<void>((r) => srv.close(() => r()));
      },
    };
    const src = new UnixSocketBehavioralRiskSource(socketPath);
    const r = await src.getBehavioralRiskSignal(makeEvent(), 500);
    expect(r.status).toBe('unavailable');
    if (r.status === 'unavailable') expect(r.code).toBe('PARSE_ERROR');
  });

  it('times out and fails fast when the server never responds', async () => {
    const socketPath = uniqueSocketPath('hang');
    const openConnections = new Set<net.Socket>();
    const srv = net.createServer((socket) => {
      openConnections.add(socket);
      socket.on('close', () => openConnections.delete(socket));
      // Accept the connection but never write anything. The client must
      // hit the deadline and tear down the socket on its own.
    });
    await new Promise<void>((res) => srv.listen(socketPath, () => res()));
    handle = {
      socketPath,
      close: async () => {
        // Force-destroy any connection the client left behind so
        // server.close() does not block waiting on a half-open socket.
        for (const s of openConnections) s.destroy();
        openConnections.clear();
        await new Promise<void>((r) => srv.close(() => r()));
      },
    };
    const src = new UnixSocketBehavioralRiskSource(socketPath);
    const started = Date.now();
    const r = await src.getBehavioralRiskSignal(makeEvent(), 40);
    const elapsed = Date.now() - started;
    expect(r.status).toBe('unavailable');
    if (r.status === 'unavailable') expect(r.code).toBe('TIMEOUT');
    // The timeout has to actually bound the hot path. Allow some slack for
    // CI scheduler noise but not enough to let an unbounded wait slip
    // through (the server never responds, so without the timeout this
    // would hang forever).
    expect(elapsed).toBeLessThan(500);
  });

  it('returns TRANSPORT_ERROR when the socket path does not exist', async () => {
    const socketPath = uniqueSocketPath('missing');
    // Server is never started: this socket path is unbound.
    const src = new UnixSocketBehavioralRiskSource(socketPath);
    const r = await src.getBehavioralRiskSignal(makeEvent(), 200);
    expect(r.status).toBe('unavailable');
    if (r.status === 'unavailable') expect(r.code).toBe('TRANSPORT_ERROR');
  });
});

describe('UnixSocketBehavioralRiskSource circuit breaker', () => {
  it('opens after the threshold is reached and fast-fails with CIRCUIT_OPEN', async () => {
    const socketPath = uniqueSocketPath('cb-open');
    // No server at this path; every call fails with TRANSPORT_ERROR.
    const src = new UnixSocketBehavioralRiskSource(socketPath);
    for (let i = 0; i < CIRCUIT_BREAKER_THRESHOLD; i++) {
      const r = await src.getBehavioralRiskSignal(makeEvent(), 100);
      expect(r.status).toBe('unavailable');
      if (r.status === 'unavailable') expect(r.code).toBe('TRANSPORT_ERROR');
    }
    // Next call must be a fast-fail CIRCUIT_OPEN without touching the
    // network: we assert that by requiring the result to come back inside
    // a timeout that would be absurdly short for a real connect attempt.
    const started = Date.now();
    const r = await src.getBehavioralRiskSignal(makeEvent(), 100);
    const elapsed = Date.now() - started;
    expect(r.status).toBe('unavailable');
    if (r.status === 'unavailable') expect(r.code).toBe('CIRCUIT_OPEN');
    expect(elapsed).toBeLessThan(20);
  });

  it('cooldown elapses and a successful probe resets the breaker', async () => {
    const socketPath = uniqueSocketPath('cb-cool');
    // Start with a virtual clock so we can advance past the cooldown
    // without waiting 30 seconds in real time.
    let nowMs = 1_000_000;
    const src = new UnixSocketBehavioralRiskSource(socketPath, { now: () => nowMs });

    for (let i = 0; i < CIRCUIT_BREAKER_THRESHOLD; i++) {
      const r = await src.getBehavioralRiskSignal(makeEvent(), 100);
      expect(r.status).toBe('unavailable');
    }
    expect(src._getBreakerStateForTest().open).toBe(true);

    // Still within cooldown: fast-fail.
    const stillOpen = await src.getBehavioralRiskSignal(makeEvent(), 100);
    expect(stillOpen.status).toBe('unavailable');
    if (stillOpen.status === 'unavailable') expect(stillOpen.code).toBe('CIRCUIT_OPEN');

    // Advance past cooldown. The next call must attempt the transport
    // again. We start a real server so the probe succeeds and closes the
    // breaker.
    nowMs += CIRCUIT_BREAKER_COOLDOWN_MS + 1;
    const handle = await startBehavioralRiskServer({
      twin: new StubTwin({
        kind: 'score',
        score: { score: 0.1, action: 'allow', reason: 'ok' },
      }),
      socketPath,
    });
    try {
      const healed = await src.getBehavioralRiskSignal(makeEvent(), 500);
      expect(healed.status).toBe('ok');
      expect(src._getBreakerStateForTest().open).toBe(false);
      expect(src._getBreakerStateForTest().failures).toBe(0);
    } finally {
      await handle.close();
    }
  });
});

describe('RuntimeTwin.scoreARPEvent readonly seam', () => {
  it('returns null when the baseline is not yet trained', () => {
    const twin = new RuntimeTwin('test-agent-seam-1');
    const result = twin.scoreARPEvent(makeEvent());
    expect(result).toBeNull();
  });

  it('returns a score when a baseline is installed via the test seam', () => {
    const twin = new RuntimeTwin('test-agent-seam-2');
    twin._setBaselineForTest({
      eventTypeCounts: { TOOL_CALL: 200 },
      avgTimingDelta: 100,
      stdTimingDelta: 10,
      capabilitySet: new Set(['known']),
      avgResponseSize: 0,
      totalEvents: 200,
      errorRate: 0,
    });
    const result = twin.scoreARPEvent(
      makeEvent({ data: { capability: 'unknown-capability' } }),
    );
    expect(result).not.toBeNull();
    if (result) {
      // Unknown capability alone contributes 0.3 per computeAnomalyScore.
      expect(result.score).toBeGreaterThanOrEqual(0.3);
      expect(result.action).toBeDefined();
    }
  });

  it('does not advance sequenceNum or lastEventTime when scoring', () => {
    const twin = new RuntimeTwin('test-agent-seam-3');
    twin._setBaselineForTest({
      eventTypeCounts: { TOOL_CALL: 200 },
      avgTimingDelta: 0,
      stdTimingDelta: 0,
      capabilitySet: new Set(['known']),
      avgResponseSize: 0,
      totalEvents: 200,
      errorRate: 0,
    });
    // Call scoreARPEvent twice. If convertEvent (the mutating variant)
    // were called instead of convertEventReadonly, the sequence number
    // inside the twin would advance; we cannot read it directly without
    // reflection, but we can assert idempotency: two calls with the same
    // event produce the same score band.
    const event = makeEvent({ data: { capability: 'known' } });
    const a = twin.scoreARPEvent(event);
    const b = twin.scoreARPEvent(event);
    expect(a).not.toBeNull();
    expect(b).not.toBeNull();
    if (a && b) {
      expect(a.action).toBe(b.action);
    }
  });
});

