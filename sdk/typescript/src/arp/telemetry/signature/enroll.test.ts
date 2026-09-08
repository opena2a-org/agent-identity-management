import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { mkdtempSync, rmSync, readFileSync, existsSync } from 'fs';
import { tmpdir } from 'os';
import { join } from 'path';
import * as ed25519 from '@noble/ed25519';
import {
  ENROLL_SCHEMA_VERSION,
  SENSOR_ENROLL_PATH,
  buildEnrollCanonical,
  buildEnrollProof,
  enrollSensor,
  manualEnrollCurl,
  readEnrollmentRecord,
} from './enroll';
import { loadSensorId } from './sensor-identity';

const RE_NONCE = /^[A-Za-z0-9_-]{8,128}$/;
const SENSOR_ID_FILE = 'telemetry-sensor-id';

let home: string;
beforeEach(() => {
  home = mkdtempSync(join(tmpdir(), 'arp-enroll-'));
  process.env.OPENA2A_HOME = home;
  delete process.env.OPENA2A_SENSOR_ID;
});
afterEach(() => {
  delete process.env.OPENA2A_HOME;
  delete process.env.OPENA2A_SENSOR_ID;
  rmSync(home, { recursive: true, force: true });
});

describe('buildEnrollCanonical — byte-parity with the Go server EnrollCanonical()', () => {
  it('joins fields in the exact registry order with the enroll schema prefix', () => {
    const c = buildEnrollCanonical({ publicKey: 'pk', nonce: 'n12345678', signedAt: 1234567890 });
    expect(c).toBe('telemetry-enroll-v1|pk|n12345678|1234567890');
  });

  it('uses a schema prefix distinct from the ingest and purge schemas (no cross-replay)', () => {
    const c = buildEnrollCanonical({ publicKey: 'pk', nonce: 'n12345678', signedAt: 1 });
    expect(c.startsWith(ENROLL_SCHEMA_VERSION + '|')).toBe(true);
    expect(c.startsWith('telemetry-sig-v1|')).toBe(false);
    expect(c.startsWith('telemetry-purge-v1|')).toBe(false);
  });
});

describe('buildEnrollProof', () => {
  it('binds the public key verbatim and produces a verifying signature', async () => {
    const now = new Date('2026-06-26T12:00:00Z');
    const { url, body } = await buildEnrollProof({ now });

    expect(url).toBe(SENSOR_ENROLL_PATH);
    expect(body.publicKey).toMatch(/^[0-9a-f]{64}$/); // lowercase hex
    expect(body.nonce).toMatch(RE_NONCE);
    expect(body.signedAt).toBe(Math.floor(now.getTime() / 1000));

    const canonical = buildEnrollCanonical({
      publicKey: body.publicKey,
      nonce: body.nonce,
      signedAt: body.signedAt,
    });
    const ok = await ed25519.verifyAsync(
      Buffer.from(body.signature, 'hex'),
      new TextEncoder().encode(canonical),
      Buffer.from(body.publicKey, 'hex'),
    );
    expect(ok).toBe(true);
  });

  it('uses a fresh nonce on each call', async () => {
    const a = await buildEnrollProof();
    const b = await buildEnrollProof();
    expect(a.body.nonce).not.toBe(b.body.nonce);
  });
});

describe('enrollSensor', () => {
  it('POSTs the signed body and returns pending state on 201', async () => {
    let captured: { url: string; init: RequestInit } | undefined;
    const fakeFetch = (async (url: string, init: RequestInit) => {
      captured = { url, init };
      return {
        ok: true,
        status: 201,
        json: async () => ({ sensorId: '11111111-1111-1111-1111-111111111111', state: 'pending', message: 'pending admin approval' }),
      } as unknown as Response;
    }) as unknown as typeof fetch;

    const res = await enrollSensor({ registryUrl: 'https://r.test' }, { fetchImpl: fakeFetch });
    expect(res.ok).toBe(true);
    expect(res.state).toBe('pending');
    expect(res.sensorId).toBe('11111111-1111-1111-1111-111111111111');
    expect(captured?.init.method).toBe('POST');
    expect(captured?.url).toBe('https://r.test/api/v1/telemetry/sensors/enroll');
    const sent = JSON.parse((captured?.init.body as string) ?? '{}');
    expect(sent.signature).toBeTruthy();
    expect(sent.publicKey).toMatch(/^[0-9a-f]{64}$/);
    expect(sent.sensorId).toBeUndefined(); // enroll does not send a sensorId
  });

  it('adopts the registry-assigned sensorId locally and records enrollment state', async () => {
    const assigned = '22222222-2222-2222-2222-222222222222';
    const fakeFetch = (async () =>
      ({ ok: true, status: 201, json: async () => ({ sensorId: assigned, state: 'pending' }) }) as unknown as Response) as unknown as typeof fetch;

    await enrollSensor({ registryUrl: 'https://r.test' }, { fetchImpl: fakeFetch });

    // The local sensor id now equals the registry-assigned id (so future reports verify).
    expect(loadSensorId()).toBe(assigned);
    expect(readFileSync(join(home, SENSOR_ID_FILE), 'utf8').trim()).toBe(assigned);

    const rec = readEnrollmentRecord();
    expect(rec?.sensorId).toBe(assigned);
    expect(rec?.state).toBe('pending');
  });

  it('reports verified state on idempotent re-enroll of an approved key', async () => {
    const assigned = '33333333-3333-3333-3333-333333333333';
    const fakeFetch = (async () =>
      ({ ok: true, status: 201, json: async () => ({ sensorId: assigned, state: 'verified' }) }) as unknown as Response) as unknown as typeof fetch;
    const res = await enrollSensor({ registryUrl: 'https://r.test' }, { fetchImpl: fakeFetch });
    expect(res.state).toBe('verified');
    expect(readEnrollmentRecord()?.state).toBe('verified');
  });

  it('returns ok=false with the status + server error on a non-2xx (does not throw)', async () => {
    const fakeFetch = (async () =>
      ({ ok: false, status: 401, json: async () => ({ error: 'unauthorized' }) }) as unknown as Response) as unknown as typeof fetch;
    const res = await enrollSensor({ registryUrl: 'https://r.test' }, { fetchImpl: fakeFetch });
    expect(res.ok).toBe(false);
    expect(res.status).toBe(401);
    expect(res.error).toContain('unauthorized');
    // A failed enroll must NOT write a local enrollment record or adopt an id.
    expect(readEnrollmentRecord()).toBeNull();
    expect(existsSync(join(home, SENSOR_ID_FILE))).toBe(false);
  });

  it('fails OPEN on a network error — resolves with ok=false, never throws', async () => {
    const fakeFetch = (async () => {
      throw new Error('ECONNREFUSED');
    }) as unknown as typeof fetch;
    const res = await enrollSensor({ registryUrl: 'https://r.test' }, { fetchImpl: fakeFetch });
    expect(res.ok).toBe(false);
    expect(res.error).toContain('ECONNREFUSED');
    expect(res.url).toBe('https://r.test/api/v1/telemetry/sensors/enroll');
  });

  it('strips a trailing slash from the registry base URL', async () => {
    let capturedUrl = '';
    const fakeFetch = (async (url: string) => {
      capturedUrl = url;
      return { ok: true, status: 201, json: async () => ({ sensorId: 'x', state: 'pending' }) } as unknown as Response;
    }) as unknown as typeof fetch;
    await enrollSensor({ registryUrl: 'https://r.test/' }, { fetchImpl: fakeFetch });
    expect(capturedUrl).toBe('https://r.test/api/v1/telemetry/sensors/enroll');
    expect(capturedUrl).not.toContain('//api');
  });
});

describe('manualEnrollCurl', () => {
  it('renders a runnable POST curl with the signed body', async () => {
    const res = await enrollSensor({ registryUrl: 'https://r.test' }, {
      fetchImpl: (async () => {
        throw new Error('offline');
      }) as unknown as typeof fetch,
    });
    const curl = manualEnrollCurl(res);
    expect(curl).toContain("curl -X POST 'https://r.test/api/v1/telemetry/sensors/enroll'");
    expect(curl).toContain("-H 'Content-Type: application/json'");
    expect(curl).toContain(res.body.signature);
  });
});

describe('readEnrollmentRecord', () => {
  it('returns null when no enrollment has happened', () => {
    expect(readEnrollmentRecord()).toBeNull();
  });
});

describe('a hostile registry cannot place terminal-driving bytes in the persisted id', () => {
  // CISO pre-tag blocker B1 (2026-08-22): the assigned sensorId is persisted
  // and later printed by `telemetry status`, so it is shape-validated where it
  // enters — rejected, not repaired.
  const HOSTILE_IDS = [
    '\x1b[2K\x1b[1Aowned',
    'abc\x9b31mdef',
    'id\rwith\roverwrites',
    'x'.repeat(129), // over-long
    'has spaces and; semicolons',
  ];

  for (const hostile of HOSTILE_IDS) {
    it(`rejects ${JSON.stringify(hostile.slice(0, 20))}...`, async () => {
      const fakeFetch = (async () =>
        ({ ok: true, status: 201, json: async () => ({ sensorId: hostile, state: 'pending' }) }) as unknown as Response) as unknown as typeof fetch;

      const res = await enrollSensor({ registryUrl: 'https://r.test' }, { fetchImpl: fakeFetch });
      expect(res.ok).toBe(true);
      expect(res.sensorId).toBeUndefined();
      // Nothing hostile was persisted.
      expect(existsSync(join(home, SENSOR_ID_FILE))).toBe(false);
    });
  }

  it('still accepts a normal service-account id', async () => {
    const fakeFetch = (async () =>
      ({ ok: true, status: 201, json: async () => ({ sensorId: 'svc-acct.0042', state: 'verified' }) }) as unknown as Response) as unknown as typeof fetch;
    const res = await enrollSensor({ registryUrl: 'https://r.test' }, { fetchImpl: fakeFetch });
    expect(res.sensorId).toBe('svc-acct.0042');
  });
});
