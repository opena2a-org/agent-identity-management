/**
 * Tests for submitGTINEvent destination resolution (AIM-11 item 8): the GTIN
 * channel must resolve its registry URL through the same chain as every other
 * signature-telemetry transport — explicit argument, else OPENA2A_REGISTRY_URL,
 * else the default — instead of hardcoding the production registry.
 *
 * HARD RULE (AIM-11.AC9): these tests must never reach the production
 * registry. Every request funnels through a guarding fetch stub that only
 * forwards to 127.0.0.1; anything else is answered locally and recorded.
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { createServer, type Server } from 'http';
import { submitGTINEvent, type GTINPayload } from './gtin';

const PAYLOAD: GTINPayload = {
  sensorToken: 'sensor-test-token',
  eventType: 'unexpected_dns',
  packageName: 'test-package',
  packageVersion: '1.0.0',
  daySinceInstall: 1,
  runtimeEnv: 'node',
  triggeredAt: '2026-09-09T00:00:00Z',
};

let server: Server;
let loopbackBase: string;
let received: Array<{ method: string | undefined; url: string | undefined; body: string }>;
let attempted: string[];
const realFetch = globalThis.fetch;
const savedRegistryEnv = process.env.OPENA2A_REGISTRY_URL;

beforeEach(async () => {
  received = [];
  attempted = [];

  server = createServer((req, res) => {
    let body = '';
    req.on('data', (chunk) => (body += chunk));
    req.on('end', () => {
      received.push({ method: req.method, url: req.url, body });
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ eventId: 'evt-loopback-1' }));
    });
  });
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  const address = server.address();
  const port = typeof address === 'object' && address ? address.port : 0;
  loopbackBase = `http://127.0.0.1:${port}`;

  // Guard: forward loopback requests to the real server; swallow anything else
  // so a regression can NEVER produce non-loopback egress from this test.
  vi.stubGlobal('fetch', ((input: RequestInfo | URL, init?: RequestInit) => {
    const url = String(input);
    attempted.push(url);
    if (url.startsWith('http://127.0.0.1:')) return realFetch(input, init);
    return Promise.resolve(new Response('{}', { status: 599 }));
  }) as typeof fetch);
});

afterEach(async () => {
  vi.unstubAllGlobals();
  if (savedRegistryEnv === undefined) delete process.env.OPENA2A_REGISTRY_URL;
  else process.env.OPENA2A_REGISTRY_URL = savedRegistryEnv;
  await new Promise<void>((resolve) => server.close(() => resolve()));
});

describe('submitGTINEvent registry URL resolution (item 8)', () => {
  it('AIM-11.AC8 honours OPENA2A_REGISTRY_URL: the POST arrives at the loopback server', async () => {
    process.env.OPENA2A_REGISTRY_URL = loopbackBase;

    const result = await submitGTINEvent(PAYLOAD);

    expect(received).toHaveLength(1);
    expect(received[0].method).toBe('POST');
    expect(received[0].url).toBe('/api/v1/telemetry/runtime');
    expect(JSON.parse(received[0].body).sensorToken).toBe('sensor-test-token');
    expect(result.success).toBe(true);
    expect(result.eventId).toBe('evt-loopback-1');
  });

  it('AIM-11.AC8 an explicit registryUrl argument wins over the environment', async () => {
    process.env.OPENA2A_REGISTRY_URL = 'http://127.0.0.1:1'; // would be refused
    const result = await submitGTINEvent(PAYLOAD, loopbackBase);

    expect(received).toHaveLength(1);
    expect(result.success).toBe(true);
  });

  it('AIM-11.AC9 no request is addressed to api.oa2a.org or any non-loopback host', async () => {
    process.env.OPENA2A_REGISTRY_URL = loopbackBase;
    await submitGTINEvent(PAYLOAD);
    await submitGTINEvent(PAYLOAD, loopbackBase);

    for (const url of attempted) {
      expect(url.startsWith('http://127.0.0.1:')).toBe(true);
      expect(url).not.toContain('api.oa2a.org');
    }
  });
});
