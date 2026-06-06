/**
 * Tests for the SDK -> Registry relay. Key guarantees: inert when disabled;
 * uploads only denied_injection_attempt indicators; ships NO identifiers; the
 * cursor advances so records are not re-sent; failures block the cursor (retry)
 * while 4xx rejections advance it; nothing here ever throws.
 */
import { describe, it, expect, afterEach, vi } from 'vitest';
import * as fs from 'fs';
import * as os from 'os';
import * as path from 'path';
import { CorrelatedRelay, SENTINEL_PACKAGE_NAME, RELAY_EVENT_TYPE } from './relay';
import { writeCorrelatedRecord } from './local-writer';
import { buildCorrelatedRecord, type CorrelatedRecord } from './correlated-record';

const tmpDirs: string[] = [];

function freshDir(): string {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'cde-relay-'));
  tmpDirs.push(dir);
  return dir;
}

afterEach(() => {
  for (const dir of tmpDirs.splice(0)) {
    try {
      fs.rmSync(dir, { recursive: true, force: true });
    } catch {
      // best-effort
    }
  }
});

/** Build a record, then pin createdAt for deterministic cursor tests. */
function makeRecord(opts: {
  createdAt: string;
  injection: boolean;
  decision?: 'allow' | 'deny';
}): CorrelatedRecord {
  const decision = opts.decision ?? (opts.injection ? 'deny' : 'allow');
  const rec = buildCorrelatedRecord({
    correlationId: `cde_000000000_${Math.random().toString(16).slice(2, 14)}`,
    agentId: 'agent-secret-id',
    enforcement: {
      decision,
      outcome: decision === 'deny' ? 'DENY_INTENT' : 'ALLOW',
      capability: 'net:connect',
      resource: 'https://evil.example/secret-path',
      deniedReason: 'blocked because injected exfil instruction',
      credentialRef: 'ref:db-prod',
      occurredAt: opts.createdAt,
      source: 'aim-pdp',
    },
    detection: opts.injection
      ? {
          injectionDetected: true,
          attackClass: 'injection',
          techniqueId: 'T-2002',
          techniqueSource: 'interim-mapping',
          confidence: 0.84,
          detector: 'nanomind-guard',
          inputRef: 'sha256:abcd',
          detectedAt: opts.createdAt,
        }
      : undefined,
    intent: opts.injection
      ? { intentClass: 'exfiltration', confidence: 0.7, blocked: true, source: 'nanomind-intent' }
      : undefined,
  });
  return { ...rec, createdAt: opts.createdAt };
}

function okResponse(): Response {
  return { ok: true, status: 201, json: async () => ({ eventId: 'e1' }), text: async () => '' } as unknown as Response;
}
function statusResponse(status: number): Response {
  return { ok: false, status, json: async () => ({}), text: async () => 'err' } as unknown as Response;
}

describe('CorrelatedRelay', () => {
  it('is fully inert when disabled', async () => {
    const dir = freshDir();
    writeCorrelatedRecord(makeRecord({ createdAt: '2026-06-06T00:00:00.000Z', injection: true }), dir);
    const fetchImpl = vi.fn();
    const relay = new CorrelatedRelay({ enabled: false, dataDir: dir, fetchImpl: fetchImpl as unknown as typeof fetch });
    const res = await relay.flushOnce();
    expect(res).toEqual({ considered: 0, uploaded: 0, stoppedOnError: false });
    expect(fetchImpl).not.toHaveBeenCalled();
  });

  it('uploads only denied_injection_attempt indicators and skips the rest', async () => {
    const dir = freshDir();
    writeCorrelatedRecord(makeRecord({ createdAt: '2026-06-06T00:00:01.000Z', injection: true }), dir);
    writeCorrelatedRecord(makeRecord({ createdAt: '2026-06-06T00:00:02.000Z', injection: false }), dir); // allow
    writeCorrelatedRecord(
      makeRecord({ createdAt: '2026-06-06T00:00:03.000Z', injection: false, decision: 'deny' }), // deny w/o injection
      dir
    );

    const fetchImpl = vi.fn(async () => okResponse());
    const relay = new CorrelatedRelay({
      enabled: true,
      dataDir: dir,
      fetchImpl: fetchImpl as unknown as typeof fetch,
    });
    const res = await relay.flushOnce();
    expect(res.considered).toBe(3);
    expect(res.uploaded).toBe(1);
    expect(fetchImpl).toHaveBeenCalledTimes(1);
  });

  it('ships no identifiers — only the anonymized indicator shape', async () => {
    const dir = freshDir();
    writeCorrelatedRecord(makeRecord({ createdAt: '2026-06-06T00:00:01.000Z', injection: true }), dir);
    let sentBody: Record<string, unknown> = {};
    const fetchImpl = vi.fn(async (_url: string, init: RequestInit) => {
      sentBody = JSON.parse(init.body as string);
      return okResponse();
    });
    const relay = new CorrelatedRelay({
      enabled: true,
      dataDir: dir,
      packageName: 'acme/support-agent',
      packageVersion: '2.1.0',
      sensorToken: 'sensor-abc',
      fetchImpl: fetchImpl as unknown as typeof fetch,
    });
    await relay.flushOnce();

    expect(sentBody.eventType).toBe(RELAY_EVENT_TYPE);
    expect(sentBody.packageName).toBe('acme/support-agent');
    expect(sentBody.packageVersion).toBe('2.1.0');
    expect(sentBody.sensorToken).toBe('sensor-abc');
    expect(sentBody.techniqueId).toBe('T-2002');
    expect(sentBody.enforcementOutcome).toBe('deny');
    expect(sentBody.detectionConfidence).toBe(0.84);

    // Identifiers must NEVER appear in the wire payload.
    const wire = JSON.stringify(sentBody);
    expect(wire).not.toContain('agent-secret-id');
    expect(wire).not.toContain('evil.example');
    expect(wire).not.toContain('secret-path');
    expect(wire).not.toContain('db-prod');
    expect(wire).not.toContain('sha256:abcd');
    expect(wire).not.toContain('correlationId');
    expect(sentBody.deniedReason).toBeUndefined();
    expect(sentBody.resource).toBeUndefined();
    expect(sentBody.capability).toBeUndefined();
  });

  it('omits a malformed/tampered techniqueId rather than transmitting it', async () => {
    const dir = freshDir();
    // A locally-tampered record whose techniqueId smuggles free text.
    const rec = makeRecord({ createdAt: '2026-06-06T00:00:01.000Z', injection: true });
    const tampered: CorrelatedRecord = {
      ...rec,
      detection: {
        ...rec.detection!,
        techniqueId: 'T-2002; /Users/v/secret.key; AKIAEXFIL',
        techniqueSource: 'totally-made-up' as never,
      },
    };
    writeCorrelatedRecord(tampered, dir);

    let sentBody: Record<string, unknown> = {};
    const fetchImpl = vi.fn(async (_url: string, init: RequestInit) => {
      sentBody = JSON.parse(init.body as string);
      return okResponse();
    });
    const relay = new CorrelatedRelay({ enabled: true, dataDir: dir, fetchImpl: fetchImpl as unknown as typeof fetch });
    await relay.flushOnce();

    // The indicator is still sent (it is a valid denial), but the malformed
    // record-derived fields are dropped — no free text leaves the machine.
    expect(sentBody.eventType).toBe(RELAY_EVENT_TYPE);
    expect(sentBody.techniqueId).toBeUndefined();
    expect(sentBody.techniqueSource).toBeUndefined();
    expect(JSON.stringify(sentBody)).not.toContain('AKIAEXFIL');
    expect(JSON.stringify(sentBody)).not.toContain('secret.key');
  });

  it('defaults packageName to the sentinel when none is configured', async () => {
    const dir = freshDir();
    writeCorrelatedRecord(makeRecord({ createdAt: '2026-06-06T00:00:01.000Z', injection: true }), dir);
    let sentBody: Record<string, unknown> = {};
    const fetchImpl = vi.fn(async (_url: string, init: RequestInit) => {
      sentBody = JSON.parse(init.body as string);
      return okResponse();
    });
    const relay = new CorrelatedRelay({ enabled: true, dataDir: dir, fetchImpl: fetchImpl as unknown as typeof fetch });
    await relay.flushOnce();
    expect(sentBody.packageName).toBe(SENTINEL_PACKAGE_NAME);
  });

  it('advances the cursor so a second flush re-sends nothing', async () => {
    const dir = freshDir();
    writeCorrelatedRecord(makeRecord({ createdAt: '2026-06-06T00:00:01.000Z', injection: true }), dir);
    const fetchImpl = vi.fn(async () => okResponse());
    const relay = new CorrelatedRelay({ enabled: true, dataDir: dir, fetchImpl: fetchImpl as unknown as typeof fetch });

    const first = await relay.flushOnce();
    expect(first.uploaded).toBe(1);
    const second = await relay.flushOnce();
    expect(second.considered).toBe(0);
    expect(second.uploaded).toBe(0);
    expect(fetchImpl).toHaveBeenCalledTimes(1);
  });

  it('picks up only records appended after the cursor', async () => {
    const dir = freshDir();
    writeCorrelatedRecord(makeRecord({ createdAt: '2026-06-06T00:00:01.000Z', injection: true }), dir);
    const fetchImpl = vi.fn(async () => okResponse());
    const relay = new CorrelatedRelay({ enabled: true, dataDir: dir, fetchImpl: fetchImpl as unknown as typeof fetch });
    await relay.flushOnce();

    writeCorrelatedRecord(makeRecord({ createdAt: '2026-06-06T00:00:05.000Z', injection: true }), dir);
    const res = await relay.flushOnce();
    expect(res.considered).toBe(1);
    expect(res.uploaded).toBe(1);
    expect(fetchImpl).toHaveBeenCalledTimes(2);
  });

  it('blocks the cursor on a 5xx so the record is retried next flush', async () => {
    const dir = freshDir();
    writeCorrelatedRecord(makeRecord({ createdAt: '2026-06-06T00:00:01.000Z', injection: true }), dir);
    const fetchImpl = vi
      .fn()
      .mockResolvedValueOnce(statusResponse(503))
      .mockResolvedValueOnce(okResponse());
    const relay = new CorrelatedRelay({ enabled: true, dataDir: dir, fetchImpl: fetchImpl as unknown as typeof fetch });

    const first = await relay.flushOnce();
    expect(first.uploaded).toBe(0);
    expect(first.stoppedOnError).toBe(true);

    const second = await relay.flushOnce();
    expect(second.considered).toBe(1); // same record, retried
    expect(second.uploaded).toBe(1);
  });

  it('advances the cursor on a 4xx (non-retryable rejection)', async () => {
    const dir = freshDir();
    writeCorrelatedRecord(makeRecord({ createdAt: '2026-06-06T00:00:01.000Z', injection: true }), dir);
    const errors: unknown[] = [];
    const fetchImpl = vi.fn(async () => statusResponse(400));
    const relay = new CorrelatedRelay({
      enabled: true,
      dataDir: dir,
      fetchImpl: fetchImpl as unknown as typeof fetch,
      onError: (e) => errors.push(e),
    });
    const first = await relay.flushOnce();
    expect(first.uploaded).toBe(0);
    expect(first.stoppedOnError).toBe(false); // 4xx treated as handled
    expect(errors.length).toBe(1);

    const second = await relay.flushOnce();
    expect(second.considered).toBe(0); // cursor advanced past the rejected record
  });

  it('caps uploads to batchSize per flush', async () => {
    const dir = freshDir();
    for (let i = 0; i < 5; i++) {
      writeCorrelatedRecord(
        makeRecord({ createdAt: `2026-06-06T00:00:0${i}.000Z`, injection: true }),
        dir
      );
    }
    const fetchImpl = vi.fn(async () => okResponse());
    const relay = new CorrelatedRelay({
      enabled: true,
      dataDir: dir,
      batchSize: 2,
      fetchImpl: fetchImpl as unknown as typeof fetch,
    });
    const first = await relay.flushOnce();
    expect(first.uploaded).toBe(2);
    const second = await relay.flushOnce();
    expect(second.uploaded).toBe(2);
    const third = await relay.flushOnce();
    expect(third.uploaded).toBe(1);
  });

  it('never throws when fetch rejects, and blocks the cursor', async () => {
    const dir = freshDir();
    writeCorrelatedRecord(makeRecord({ createdAt: '2026-06-06T00:00:01.000Z', injection: true }), dir);
    const fetchImpl = vi.fn(async () => {
      throw new Error('network down');
    });
    const relay = new CorrelatedRelay({ enabled: true, dataDir: dir, fetchImpl: fetchImpl as unknown as typeof fetch });
    // flushOnce swallows all errors internally; it resolves, never rejects.
    const res = await relay.flushOnce();
    expect(res).toEqual({ considered: 1, uploaded: 0, stoppedOnError: true });
  });

  it('start() is a no-op when disabled and does not keep the event loop alive', () => {
    const relay = new CorrelatedRelay({ enabled: false });
    expect(() => {
      relay.start();
      relay.stop();
    }).not.toThrow();
  });
});
