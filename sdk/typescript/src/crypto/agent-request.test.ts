/**
 * createAgentRequestHeaders against the server's rule for agent-request-v1.
 *
 * The wire cells send the request with fetch to a loopback server, rebuild the
 * signed bytes from what that server received, the way AIM's agent middleware
 * does (UPPER(method), the raw request target, X-Timestamp as sent, then the
 * raw body when it is not empty, joined by "\n"), and verify X-Signature with
 * node:crypto, an Ed25519 implementation independent of the SDK's.
 */

import { createServer, type IncomingMessage, type Server } from 'node:http';
import type { AddressInfo } from 'node:net';
import { createPublicKey, generateKeyPairSync, verify as nodeVerify } from 'node:crypto';
import { afterAll, afterEach, beforeAll, describe, expect, it, vi } from 'vitest';
import { createAgentRequestHeaders, type AgentRequestMethod } from './agent-request';
import { ConfigurationError } from '../exceptions';
import * as sdk from '../index';

const INJECTED_MS = 1_700_000_123_456;
const INJECTED_SECONDS = '1700000123';

function makeKey() {
  const { privateKey } = generateKeyPairSync('ed25519');
  const jwk = privateKey.export({ format: 'jwk' }) as { d: string; x: string };
  const seed = Buffer.from(jwk.d, 'base64url');
  const pub = Buffer.from(jwk.x, 'base64url');
  return {
    seed,
    pub,
    seedB64: seed.toString('base64'),
    issuedB64: Buffer.concat([seed, pub]).toString('base64'),
    pubB64: pub.toString('base64'),
  };
}

const key = makeKey();
const AGENT_ID = '7d0c1a52-4f7e-4d3b-9a51-0d6c2e6f1b11';
const creds = { agentId: AGENT_ID, privateKey: key.issuedB64, publicKey: key.pubB64 };

interface Received {
  method: string;
  target: string;
  headers: IncomingMessage['headers'];
  body: Buffer;
}

let server: Server;
let base: string;
const received: Received[] = [];

beforeAll(async () => {
  server = createServer((req, res) => {
    const chunks: Buffer[] = [];
    req.on('data', (c: Buffer) => chunks.push(c));
    req.on('end', () => {
      received.push({
        method: req.method ?? '',
        target: req.url ?? '',
        headers: req.headers,
        body: Buffer.concat(chunks),
      });
      // A server clock 600 s away from the injected one: the signer must never
      // take its timestamp from a response.
      res.setHeader('Date', new Date(INJECTED_MS + 600_000).toUTCString());
      res.end('{}');
    });
  });
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  base = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
});

afterAll(async () => {
  await new Promise<void>((resolve) => server.close(() => resolve()));
});

afterEach(() => {
  vi.useRealTimers();
  vi.restoreAllMocks();
  received.length = 0;
});

/** The server's rule, applied to what the loopback server received. */
function serverSignedBytes(r: Received): Buffer {
  const head = `${r.method.toUpperCase()}\n${r.target}\n${String(r.headers['x-timestamp'])}`;
  const parts = [Buffer.from(head, 'utf8')];
  if (r.body.length > 0) parts.push(Buffer.from('\n'), r.body);
  return Buffer.concat(parts);
}

function serverVerifies(r: Received): boolean {
  const publicKey = createPublicKey({
    key: {
      kty: 'OKP',
      crv: 'Ed25519',
      x: Buffer.from(String(r.headers['x-public-key']), 'base64').toString('base64url'),
    },
    format: 'jwk',
  });
  const signature = Buffer.from(String(r.headers['x-signature']), 'base64');
  return nodeVerify(null, serverSignedBytes(r), publicKey, signature);
}

async function signAndSend(
  method: AgentRequestMethod,
  pathAndQuery: string,
  body?: string,
  credentials = creds
): Promise<Received> {
  const url = `${base}${pathAndQuery}`;
  const headers = await createAgentRequestHeaders({ method, url, body }, credentials);
  const res = await fetch(url, {
    method,
    headers: { 'Content-Type': 'application/json', ...headers },
    body,
  });
  await res.text();
  const r = received[received.length - 1];
  expect(r).toBeDefined();
  return r;
}

describe('createAgentRequestHeaders: the server rule verifies what fetch sent', () => {
  it('a GET', async () => {
    vi.useFakeTimers({ toFake: ['Date'] });
    vi.setSystemTime(INJECTED_MS);
    const r = await signAndSend('GET', '/api/v1/sdk-api/agents/' + AGENT_ID);
    expect(serverVerifies(r)).toBe(true);
    expect(r.headers['x-timestamp']).toBe(INJECTED_SECONDS);
  });

  it('a POST whose body object has keys out of sorted order, signed as sent', async () => {
    vi.useFakeTimers({ toFake: ['Date'] });
    vi.setSystemTime(INJECTED_MS);
    const body = JSON.stringify({ zeta: 1, alpha: { y: 2, b: 3 } });
    const r = await signAndSend('POST', `/api/v1/sdk-api/agents/${AGENT_ID}/heartbeat`, body);
    expect(r.body.toString('utf8')).toBe(body);
    expect(serverVerifies(r)).toBe(true);
  });

  it('a POST with an empty body', async () => {
    vi.useFakeTimers({ toFake: ['Date'] });
    vi.setSystemTime(INJECTED_MS);
    const r = await signAndSend('POST', `/api/v1/sdk-api/agents/${AGENT_ID}/heartbeat`, '');
    expect(r.body.length).toBe(0);
    expect(serverVerifies(r)).toBe(true);
  });

  it('a URL with a query string', async () => {
    vi.useFakeTimers({ toFake: ['Date'] });
    vi.setSystemTime(INJECTED_MS);
    const r = await signAndSend('POST', `/api/v1/sdk-api/agents/${AGENT_ID}/heartbeat?probe=1&b=a%20b`, '{}');
    expect(r.target).toBe(`/api/v1/sdk-api/agents/${AGENT_ID}/heartbeat?probe=1&b=a%20b`);
    expect(serverVerifies(r)).toBe(true);
  });

  it('PUT, PATCH and DELETE', async () => {
    for (const method of ['PUT', 'PATCH', 'DELETE'] as const) {
      const r = await signAndSend(method, '/api/v1/x?y=1', method === 'DELETE' ? undefined : '{"a":1}');
      expect(r.method).toBe(method);
      expect(serverVerifies(r)).toBe(true);
    }
  });

  it('a 32-byte seed signs, and the server verifies under the derived public key', async () => {
    const r = await signAndSend('GET', '/p', undefined, { ...creds, privateKey: key.seedB64 });
    expect(r.headers['x-public-key']).toBe(key.pubB64);
    expect(serverVerifies(r)).toBe(true);
  });

  it('the signature fails once the body, query or timestamp differs from what was signed', async () => {
    const r = await signAndSend('POST', '/p?a=1', '{"a":1}');
    expect(serverVerifies(r)).toBe(true);
    expect(serverVerifies({ ...r, body: Buffer.from('{"a":2}') })).toBe(false);
    expect(serverVerifies({ ...r, target: '/p?a=2' })).toBe(false);
    const ts = Number(r.headers['x-timestamp']) + 1;
    expect(serverVerifies({ ...r, headers: { ...r.headers, 'x-timestamp': String(ts) } })).toBe(false);
  });
});

describe('createAgentRequestHeaders: timestamp and result shape', () => {
  it('X-Timestamp is the host clock in decimal seconds, never a response Date', async () => {
    vi.useFakeTimers({ toFake: ['Date'] });
    vi.setSystemTime(INJECTED_MS);
    // The first response carries a Date 600 s ahead; the next signature still
    // uses the host clock.
    await signAndSend('GET', '/first');
    const r = await signAndSend('GET', '/second');
    expect(r.headers['x-timestamp']).toMatch(/^[0-9]+$/);
    expect(Math.abs(Number(r.headers['x-timestamp']) - INJECTED_MS / 1000)).toBeLessThanOrEqual(1);
    expect(r.headers['x-timestamp']).toBe(INJECTED_SECONDS);
  });

  it('returns exactly the four headers, with standard padded base64 values', async () => {
    const headers = await createAgentRequestHeaders({ method: 'GET', url: `${base}/p` }, creds);
    expect(Object.keys(headers).sort()).toEqual(['X-Agent-ID', 'X-Public-Key', 'X-Signature', 'X-Timestamp']);
    expect(headers['X-Agent-ID']).toBe(AGENT_ID);
    expect(Buffer.from(headers['X-Signature'], 'base64').length).toBe(64);
    expect(Buffer.from(headers['X-Signature'], 'base64').toString('base64')).toBe(headers['X-Signature']);
    expect(headers['X-Public-Key']).toBe(key.pubB64);
  });

  it('takes exactly two parameters', () => {
    expect(createAgentRequestHeaders.length).toBe(2);
  });

  it('is exported from the package entry point', () => {
    expect(sdk.createAgentRequestHeaders).toBe(createAgentRequestHeaders);
  });
});

describe('createAgentRequestHeaders: the signed target is the request target fetch sends', () => {
  const cases: Array<[string, string]> = [
    ['/p?', '/p?'],
    ['/p?#frag', '/p?'],
    ['/p#frag', '/p'],
  ];
  for (const [suffix, target] of cases) {
    it(`${suffix} signs ${target}`, async () => {
      const r = await signAndSend('GET', suffix);
      expect(r.target).toBe(target);
      expect(serverVerifies(r)).toBe(true);
    });
  }
});

async function refusalOf(promise: Promise<unknown>): Promise<ConfigurationError> {
  const error = await promise.then(
    () => {
      throw new Error('expected a refusal');
    },
    (e: unknown) => e
  );
  expect(error).toBeInstanceOf(ConfigurationError);
  return error as ConfigurationError;
}

describe('createAgentRequestHeaders: refusals before signing', () => {
  const sign = (request: Record<string, unknown>, credentials: Record<string, unknown> = creds) =>
    createAgentRequestHeaders(
      request as unknown as Parameters<typeof createAgentRequestHeaders>[0],
      credentials as unknown as Parameters<typeof createAgentRequestHeaders>[1]
    );

  it('refuses a lowercase method and never normalizes it', async () => {
    const e = await refusalOf(sign({ method: 'get', url: `${base}/p` }));
    expect(e.message).toMatch(/^the method is not one of GET, POST, PUT, PATCH or DELETE in upper case; nothing was signed\nFix: /);
  });

  it('refuses any other method', async () => {
    for (const method of ['HEAD', 'OPTIONS', 'CONNECT', 'TRACE', 'Post', '', undefined]) {
      await refusalOf(sign({ method, url: `${base}/p` }));
    }
  });

  it('refuses a URL with a username or a password', async () => {
    for (const url of ['http://user@127.0.0.1/p', 'http://:pw@127.0.0.1/p', 'https://u:p@aim.example/p']) {
      const e = await refusalOf(sign({ method: 'GET', url }));
      expect(e.message).toMatch(/^the url carries a username or a password; nothing was signed\nFix: /);
      expect(e.message).not.toContain('pw');
    }
  });

  it('refuses a relative URL and a scheme other than http or https', async () => {
    for (const url of ['/api/v1/x', 'api/v1/x', 'ftp://127.0.0.1/p', 'file:///etc/hosts', '', 42]) {
      const e = await refusalOf(sign({ method: 'GET', url }));
      expect(e.message).toMatch(/^the url is not an absolute http or https URL; nothing was signed\nFix: /);
    }
  });

  it('refuses a body that is neither a string nor bytes', async () => {
    const e = await refusalOf(sign({ method: 'POST', url: `${base}/p`, body: { a: 1 } }));
    expect(e.message).toMatch(/^the body is not a string or bytes; nothing was signed\nFix: /);
  });

  it('refuses a missing agent ID', async () => {
    const e = await refusalOf(sign({ method: 'GET', url: `${base}/p` }, { ...creds, agentId: '' }));
    expect(e.message).toMatch(/^no agent ID is configured; nothing was signed\nFix: /);
  });

  it('sends nothing: the loopback server receives no request from a refusal', async () => {
    await refusalOf(sign({ method: 'get', url: `${base}/p` }));
    expect(received).toHaveLength(0);
  });
});

describe('createAgentRequestHeaders: key form', () => {
  const sign = (privateKey: unknown, publicKey: string = key.pubB64) =>
    createAgentRequestHeaders(
      { method: 'GET', url: `${base}/p` },
      { agentId: AGENT_ID, privateKey: privateKey as string, publicKey }
    );

  const refusals: Array<[string, () => unknown, RegExp]> = [
    ['a missing key', () => '', /^no agent private key is configured; nothing was signed\nFix: /],
    [
      'a 31-byte key',
      () => key.seed.subarray(0, 31).toString('base64'),
      /^the agent private key is 31 bytes long, not 32 or 64; nothing was signed\nFix: /,
    ],
    [
      'a 48-byte key',
      () => Buffer.concat([key.seed, key.pub.subarray(0, 16)]).toString('base64'),
      /^the agent private key is 48 bytes long, not 32 or 64; nothing was signed\nFix: /,
    ],
    [
      'a 64-byte key with byte 63 flipped',
      () => {
        const b = Buffer.concat([key.seed, key.pub]);
        b[63] ^= 0x01;
        return b.toString('base64');
      },
      /^the agent private key is 64 bytes long, and its last 32 bytes are not its public key; nothing was signed\nFix: /,
    ],
    [
      'a 64-byte key whose second half is another key',
      () => Buffer.concat([key.seed, makeKey().pub]).toString('base64'),
      /^the agent private key is 64 bytes long, and its last 32 bytes are not its public key; nothing was signed\nFix: /,
    ],
    [
      'a 44-character string with three non-alphabet characters',
      () => '!' + key.seedB64.slice(1, 20) + '*' + key.seedB64.slice(21, 30) + '~' + key.seedB64.slice(31),
      /^the agent private key is not standard base64; nothing was signed\nFix: /,
    ],
    [
      'base64url instead of standard base64',
      () => Buffer.from(Array.from({ length: 32 }, () => 0xfb)).toString('base64url') + '=',
      /^the agent private key is not standard base64; nothing was signed\nFix: /,
    ],
    [
      'unpadded base64',
      () => key.seedB64.replace(/=+$/, ''),
      /^the agent private key is not standard base64; nothing was signed\nFix: /,
    ],
    [
      'a non-canonical encoding (non-zero padding bits)',
      () => {
        const s = key.seedB64; // 44 characters ending in one '='
        const last = s[42];
        const alphabet = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/';
        const bumped = alphabet[(alphabet.indexOf(last) & ~0x03) | ((alphabet.indexOf(last) + 1) & 0x03)];
        return s.slice(0, 42) + bumped + '=';
      },
      /^the agent private key is not standard base64; nothing was signed\nFix: /,
    ],
    [
      'a hex key',
      () => key.seed.toString('hex'),
      /^the agent private key is 48 bytes long, not 32 or 64; nothing was signed\nFix: /,
    ],
  ];

  for (const [name, input, expected] of refusals) {
    it(`refuses ${name}`, async () => {
      const e = await refusalOf(sign(input()));
      expect(e.message).toMatch(expected);
    });
  }

  it('refuses a key whose public key differs from the configured public key', async () => {
    const e = await refusalOf(sign(key.issuedB64, makeKey().pubB64));
    expect(e.message).toMatch(
      /^the configured agent public key is not the public key of the agent private key; nothing was signed\nFix: /
    );
  });

  it('the seed and the AIM-issued 64-byte form produce the same public key and a verifying signature', async () => {
    const a = await sign(key.seedB64);
    const b = await sign(key.issuedB64);
    expect(a['X-Public-Key']).toBe(key.pubB64);
    expect(b['X-Public-Key']).toBe(key.pubB64);
  });

  it('no refusal message carries an 8-character piece of the key, and the detector flags a control line', async () => {
    const planted = makeKey();
    const forms = [planted.issuedB64, planted.seedB64, planted.seed.toString('hex'), Buffer.concat([planted.seed, planted.pub]).toString('hex')];
    const pieces = new Set<string>();
    for (const form of forms) {
      for (let i = 0; i + 8 <= form.length; i++) pieces.add(form.slice(i, i + 8));
    }
    const leaks = (text: string) => [...pieces].some((p) => text.includes(p));

    const flipped = Buffer.concat([planted.seed, planted.pub]);
    flipped[63] ^= 0x01;
    const inputs = [
      planted.seed.subarray(0, 31).toString('base64'),
      Buffer.concat([planted.seed, planted.pub.subarray(0, 16)]).toString('base64'),
      flipped.toString('base64'),
      '!' + planted.seedB64.slice(1),
      planted.seed.toString('hex'),
    ];
    const captured: string[] = [];
    for (const input of inputs) {
      const e = await refusalOf(sign(input, planted.pubB64));
      captured.push(e.message, String(e.stack));
    }
    const mismatch = await refusalOf(sign(planted.issuedB64, key.pubB64));
    captured.push(mismatch.message, String(mismatch.stack));

    expect(captured.filter(leaks)).toEqual([]);
    expect(leaks(`control: ${planted.issuedB64}`)).toBe(true);
  });

  it('writes nothing to the console when it signs or refuses', async () => {
    const spies = (['log', 'info', 'warn', 'error', 'debug'] as const).map((m) =>
      vi.spyOn(console, m).mockImplementation(() => undefined)
    );
    await sign(key.issuedB64);
    await refusalOf(sign(key.seed.subarray(0, 31).toString('base64')));
    for (const spy of spies) expect(spy).not.toHaveBeenCalled();
  });
});
