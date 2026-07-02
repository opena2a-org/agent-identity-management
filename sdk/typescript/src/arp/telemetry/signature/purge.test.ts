import { describe, it, expect, beforeAll, afterAll } from 'vitest';
import { mkdtempSync, rmSync } from 'fs';
import { tmpdir } from 'os';
import { join } from 'path';
import * as ed25519 from '@noble/ed25519';
import {
  PURGE_SCHEMA_VERSION,
  SIGNATURE_PURGE_PATH,
  buildPurgeCanonical,
  buildPurgeProof,
  purgeRemoteSignatures,
  manualPurgeCurl,
} from './purge';

const RE_NONCE = /^[A-Za-z0-9_-]{8,128}$/;
const RE_UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;

let home: string;
beforeAll(() => {
  home = mkdtempSync(join(tmpdir(), 'arp-purge-'));
  process.env.OPENA2A_HOME = home;
});
afterAll(() => {
  delete process.env.OPENA2A_HOME;
  rmSync(home, { recursive: true, force: true });
});

describe('buildPurgeCanonical — byte-parity with the Go server PurgeCanonical()', () => {
  it('joins fields in the exact registry order with the purge schema prefix', () => {
    const c = buildPurgeCanonical({ sensorId: 'sid', nonce: 'n12345678', signedAt: 1234567890 });
    expect(c).toBe('telemetry-purge-v1|sid|n12345678|1234567890');
  });

  it('uses a schema prefix distinct from the ingest schema (no cross-replay)', () => {
    const c = buildPurgeCanonical({ sensorId: 'sid', nonce: 'n12345678', signedAt: 1 });
    expect(c.startsWith(PURGE_SCHEMA_VERSION + '|')).toBe(true);
    expect(c.startsWith('telemetry-sig-v1|')).toBe(false);
  });
});

describe('buildPurgeProof', () => {
  it('produces a canonical-UUID sensorId, a url-safe nonce, and a verifying signature', async () => {
    const now = new Date('2026-06-25T12:00:00Z');
    const { url, body } = await buildPurgeProof({ now });

    expect(body.sensorId).toMatch(RE_UUID); // canonical lowercase-hyphenated
    expect(body.nonce).toMatch(RE_NONCE);
    expect(body.signedAt).toBe(Math.floor(now.getTime() / 1000));
    expect(url).toBe(`${SIGNATURE_PURGE_PATH}/${body.sensorId}`);

    // The signature must verify against the supplied public key over the canonical.
    const canonical = buildPurgeCanonical({
      sensorId: body.sensorId,
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

  it('uses a fresh nonce on each call (anti-replay)', async () => {
    const a = await buildPurgeProof();
    const b = await buildPurgeProof();
    expect(a.body.nonce).not.toBe(b.body.nonce);
  });
});

describe('purgeRemoteSignatures', () => {
  it('returns ok + deleted count and issues a DELETE with the signed body on 200', async () => {
    let captured: { url: string; init: RequestInit } | undefined;
    const fakeFetch = (async (url: string, init: RequestInit) => {
      captured = { url, init };
      return {
        ok: true,
        status: 200,
        json: async () => ({ deleted: 3 }),
      } as unknown as Response;
    }) as unknown as typeof fetch;

    const res = await purgeRemoteSignatures({ registryUrl: 'https://r.test' }, { fetchImpl: fakeFetch });
    expect(res.ok).toBe(true);
    expect(res.deleted).toBe(3);
    expect(res.status).toBe(200);
    expect(captured?.init.method).toBe('DELETE');
    expect(captured?.url).toBe(`https://r.test${res.url.replace('https://r.test', '')}`);
    expect(captured?.url.startsWith('https://r.test/api/v1/telemetry/signature/sensor/')).toBe(true);
    // The body is the signed proof.
    const sent = JSON.parse((captured?.init.body as string) ?? '{}');
    expect(sent.signature).toBeTruthy();
    expect(sent.publicKey).toBeTruthy();
    expect(sent.sensorId).toMatch(RE_UUID);
  });

  it('returns ok=false with the status on a non-200 (does not throw)', async () => {
    const fakeFetch = (async () =>
      ({ ok: false, status: 401, json: async () => ({}) }) as unknown as Response) as unknown as typeof fetch;
    const res = await purgeRemoteSignatures({ registryUrl: 'https://r.test' }, { fetchImpl: fakeFetch });
    expect(res.ok).toBe(false);
    expect(res.status).toBe(401);
  });

  it('fails OPEN on a network error — resolves with ok=false, never throws', async () => {
    const fakeFetch = (async () => {
      throw new Error('ECONNREFUSED');
    }) as unknown as typeof fetch;
    const res = await purgeRemoteSignatures({ registryUrl: 'https://r.test' }, { fetchImpl: fakeFetch });
    expect(res.ok).toBe(false);
    expect(res.error).toContain('ECONNREFUSED');
    expect(res.url.startsWith('https://r.test/')).toBe(true);
  });

  it('strips a trailing slash from the registry base URL', async () => {
    let capturedUrl = '';
    const fakeFetch = (async (url: string) => {
      capturedUrl = url;
      return { ok: true, status: 200, json: async () => ({ deleted: 0 }) } as unknown as Response;
    }) as unknown as typeof fetch;
    await purgeRemoteSignatures({ registryUrl: 'https://r.test/' }, { fetchImpl: fakeFetch });
    expect(capturedUrl.startsWith('https://r.test/api/v1/')).toBe(true);
    expect(capturedUrl).not.toContain('//api'); // no double slash from the trailing one
  });
});

describe('manualPurgeCurl', () => {
  it('renders a runnable DELETE curl with the signed body', async () => {
    const res = await purgeRemoteSignatures({ registryUrl: 'https://r.test' }, {
      fetchImpl: (async () => {
        throw new Error('offline');
      }) as unknown as typeof fetch,
    });
    const curl = manualPurgeCurl(res);
    expect(curl).toContain("curl -X DELETE 'https://r.test/api/v1/telemetry/signature/sensor/");
    expect(curl).toContain("-H 'Content-Type: application/json'");
    expect(curl).toContain(res.body.signature);
  });
});
