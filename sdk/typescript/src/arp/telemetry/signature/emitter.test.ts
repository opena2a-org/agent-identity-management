import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { mkdtempSync, rmSync, writeFileSync } from 'fs';
import { tmpdir } from 'os';
import { join } from 'path';
import { SignatureEmitter, deriveOutcome } from './emitter';
import { readAuditRecords } from './audit-log';
import type { ARPEvent } from '../../types';

function event(partial: Partial<ARPEvent>): ARPEvent {
  return {
    id: 'e1', timestamp: '2026-06-25T00:00:00.000Z', source: 'network',
    category: 'threat', severity: 'high', description: 'x', data: {},
    classifiedBy: 'L0-rules', ...partial,
  };
}

let home: string;
let fetchMock: ReturnType<typeof vi.fn>;

beforeEach(() => {
  home = mkdtempSync(join(tmpdir(), 'arp-emit-'));
  process.env.OPENA2A_HOME = home;
  fetchMock = vi.fn();
  vi.stubGlobal('fetch', fetchMock);
});
afterEach(() => {
  vi.unstubAllGlobals();
  delete process.env.OPENA2A_HOME;
  rmSync(home, { recursive: true, force: true });
});

describe('SignatureEmitter — fail closed on redaction', () => {
  it('does not queue non-anomalous events', () => {
    const e = new SignatureEmitter({ registryUrl: 'https://r.test' });
    e.onEvent(event({ category: 'normal' }));
    e.onEvent(event({ source: 'heartbeat' }));
    expect(e.getQueueLength()).toBe(0);
  });

  it('queues an anomalous, reducible event', () => {
    const e = new SignatureEmitter({ registryUrl: 'https://r.test' });
    e.onEvent(event({ category: 'threat', source: 'network' }));
    expect(e.getQueueLength()).toBe(1);
  });
});

describe('SignatureEmitter — audit BEFORE transmit', () => {
  it('writes the exact bytes locally before POSTing, and POSTs those same bytes', async () => {
    fetchMock.mockResolvedValue({ ok: true, status: 202 } as Response);
    const e = new SignatureEmitter({ registryUrl: 'https://r.test' });
    e.onEvent(event({ category: 'threat', source: 'network' }));
    await e.flush();

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const sentBody = fetchMock.mock.calls[0][1].body as string;

    const records = await readAuditRecords(100);
    const queued = records.find((r) => r.phase === 'queued');
    const sent = records.find((r) => r.phase === 'sent');
    expect(queued).toBeDefined();
    expect(sent).toBeDefined();
    // the audited bytes ARE the transmitted bytes
    expect(queued!.body).toBe(sentBody);
    // queued was written before sent (file append order)
    expect(records.indexOf(queued!)).toBeLessThan(records.indexOf(sent!));
    // and the bytes carry no free text / identity beyond the allowlisted schema
    const parsed = JSON.parse(sentBody);
    expect(Object.keys(parsed).sort()).toEqual(
      ['behavioralHash', 'count', 'nonce', 'orgPseudonym', 'outcome', 'publicKey', 'schemaVersion', 'sensorId', 'severity', 'signature', 'signedAt', 'techniqueId'].sort(),
    );
  });
});

describe('SignatureEmitter — fails OPEN on transport error', () => {
  it('re-buffers on a network error and never throws', async () => {
    fetchMock.mockRejectedValue(new Error('ECONNREFUSED'));
    const e = new SignatureEmitter({ registryUrl: 'https://r.test' });
    e.onEvent(event({ category: 'threat', source: 'network' }));
    await expect(e.flush()).resolves.toBeUndefined();
    // signal re-buffered for the next flush
    expect(e.getQueueLength()).toBe(1);
    const records = await readAuditRecords(100);
    expect(records.some((r) => r.phase === 'buffered')).toBe(true);
  });

  it('does NOT re-buffer on a 4xx (non-retryable schema/disabled rejection)', async () => {
    fetchMock.mockResolvedValue({ ok: false, status: 400 } as Response);
    const e = new SignatureEmitter({ registryUrl: 'https://r.test' });
    e.onEvent(event({ category: 'threat', source: 'network' }));
    await e.flush();
    expect(e.getQueueLength()).toBe(0);
    const records = await readAuditRecords(100);
    expect(records.some((r) => r.phase === 'failed')).toBe(true);
  });

  it('re-buffers on a 5xx (retryable)', async () => {
    fetchMock.mockResolvedValue({ ok: false, status: 503 } as Response);
    const e = new SignatureEmitter({ registryUrl: 'https://r.test' });
    e.onEvent(event({ category: 'threat', source: 'network' }));
    await e.flush();
    expect(e.getQueueLength()).toBe(1);
  });
});

describe('SignatureEmitter — aggregation', () => {
  it('aggregates identical shapes into one submission with a count', async () => {
    fetchMock.mockResolvedValue({ ok: true, status: 202 } as Response);
    const e = new SignatureEmitter({ registryUrl: 'https://r.test' });
    for (let i = 0; i < 4; i++) e.onEvent(event({ category: 'threat', source: 'network' }));
    await e.flush();
    expect(fetchMock).toHaveBeenCalledTimes(1);
    const body = JSON.parse(fetchMock.mock.calls[0][1].body as string);
    expect(body.count).toBe(4);
  });
});

describe('SignatureEmitter — audit failure blocks transmit (P1 fix)', () => {
  it('does NOT POST if the queued audit write fails; re-buffers instead', async () => {
    fetchMock.mockResolvedValue({ ok: true, status: 202 } as Response);
    // Make ~/.opena2a unwritable by pointing OPENA2A_HOME at a path under a file.
    const blocker = join(home, 'not-a-dir');
    writeFileSync(blocker, 'x');
    process.env.OPENA2A_HOME = join(blocker, 'sub'); // mkdir/append will fail

    const e = new SignatureEmitter({ registryUrl: 'https://r.test' });
    e.onEvent(event({ category: 'threat', source: 'network' }));
    await e.flush();

    expect(fetchMock).not.toHaveBeenCalled(); // never transmitted unaudited bytes
    expect(e.getQueueLength()).toBe(1); // re-buffered for retry
  });
});

describe('SignatureEmitter — count preserved across retry (P2 fix)', () => {
  it('keeps the aggregated occurrence count when a send fails and retries', async () => {
    fetchMock.mockRejectedValueOnce(new Error('down')); // first flush fails
    const e = new SignatureEmitter({ registryUrl: 'https://r.test' });
    for (let i = 0; i < 4; i++) e.onEvent(event({ category: 'threat', source: 'network' }));
    await e.flush(); // aggregates to count=4, fails, re-buffers WITH count

    fetchMock.mockResolvedValue({ ok: true, status: 202 } as Response);
    await e.flush(); // retry succeeds
    const body = JSON.parse(fetchMock.mock.calls.at(-1)![1].body as string);
    expect(body.count).toBe(4); // not reset to 1
  });
});

describe('SignatureEmitter — runtime opt-out stops flush (P2 fix)', () => {
  it('discards the queue and sends nothing once opted out at runtime', async () => {
    fetchMock.mockResolvedValue({ ok: true, status: 202 } as Response);
    const e = new SignatureEmitter({ registryUrl: 'https://r.test' });
    e.onEvent(event({ category: 'threat', source: 'network' }));
    process.env.OPENA2A_TELEMETRY_OPTOUT = '1'; // opt out after queueing
    await e.flush();
    expect(fetchMock).not.toHaveBeenCalled();
    expect(e.getQueueLength()).toBe(0);
    delete process.env.OPENA2A_TELEMETRY_OPTOUT;
  });
});

describe('deriveOutcome', () => {
  it('maps explicit enforcement flags to blocked', () => {
    expect(deriveOutcome(event({ data: { blocked: true } }))).toBe('blocked');
    expect(deriveOutcome(event({ data: { enforcementAction: 'kill' } }))).toBe('blocked');
  });
  it('defaults to detected', () => {
    expect(deriveOutcome(event({ data: {} }))).toBe('detected');
  });
});
