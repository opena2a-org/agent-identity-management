/**
 * The agent-request-v2 signer against the conformance vectors and against the
 * server's rule on the wire.
 *
 * The vector cells sign the requests of docs/specs/agent-request-v2-vectors.json
 * that AIM's verifier accepts, with the keys derived from the file's labels and
 * each vector's time and nonce, and compare the two headers with the vector's
 * statement and signature lines byte for byte. The wire cells send signed
 * requests with fetch to a loopback server, check that the statement it
 * received names that request the way AIM's verifier compares it, and verify
 * the Ed25519 signature with node:crypto, an implementation independent of the
 * SDK's.
 */

import {
  createHash,
  createPrivateKey,
  createPublicKey,
  generateKeyPairSync,
  randomBytes,
  verify as nodeVerify,
} from 'node:crypto';
import { readdirSync, readFileSync } from 'node:fs';
import { createServer, type IncomingMessage, type Server } from 'node:http';
import type { AddressInfo } from 'node:net';
import { join } from 'node:path';
import { ml_dsa65 } from '@noble/post-quantum/ml-dsa';
import { afterAll, afterEach, beforeAll, describe, expect, it, vi } from 'vitest';
import { createAgentRequestHeaders, type AgentRequestMethod } from './agent-request';
import {
  createAgentRequestV2Headers,
  signAgentRequestV2,
  type AgentRequestV2Options,
} from './agent-request-v2';
import { ConfigurationError } from '../exceptions';
import * as sdk from '../index';

const PAYLOAD_TYPE = 'application/vnd.opena2a.agent-request.v2+json';
const MEMBERS = ['method', 'target', 'audience', 'body_sha256', 'agent_id', 'timestamp', 'nonce'];

/** The DSSE v1 pre-authentication encoding, written from the format's text. */
function pae(payload: Buffer): Buffer {
  const head = `DSSEv1 ${Buffer.byteLength(PAYLOAD_TYPE)} ${PAYLOAD_TYPE} ${payload.length} `;
  return Buffer.concat([Buffer.from(head, 'utf8'), payload]);
}

const sha256 = (data: string | Uint8Array): Buffer => createHash('sha256').update(data).digest();

/** An Ed25519 public key object from its raw 32 bytes in standard base64. */
function ed25519PublicKey(rawB64: string) {
  return createPublicKey({
    key: { kty: 'OKP', crv: 'Ed25519', x: Buffer.from(rawB64, 'base64').toString('base64url') },
    format: 'jwk',
  });
}

/** The raw Ed25519 public key of a 32-byte seed, in standard base64, by node:crypto. */
function ed25519PublicKeyOfSeed(seed: Uint8Array): string {
  // The DER header of an Ed25519 PKCS #8 private key (RFC 8410), then the seed.
  const pkcs8 = Buffer.concat([Buffer.from('302e020100300506032b657004220420', 'hex'), seed]);
  const privateKey = createPrivateKey({ key: pkcs8, format: 'der', type: 'pkcs8' });
  const jwk = createPublicKey(privateKey).export({ format: 'jwk' }) as { x: string };
  return Buffer.from(jwk.x, 'base64url').toString('base64');
}

/** The statement a request carries: strict unpadded base64url of one JSON object. */
function statementOf(header: unknown): { payload: Buffer; statement: Record<string, string> } {
  expect(typeof header).toBe('string');
  const text = header as string;
  expect(text).toMatch(/^[A-Za-z0-9_-]+$/);
  const payload = Buffer.from(text, 'base64url');
  expect(payload.toString('base64url')).toBe(text);
  return { payload, statement: JSON.parse(payload.toString('utf8')) as Record<string, string> };
}

// The conformance vectors.

interface VectorAgent {
  name: string;
  agentId: string;
  hybridMode: boolean;
  ed25519Label: string;
  ed25519: string;
  mlDsa65Label?: string;
  mlDsa65?: string;
}

interface Vector {
  name: string;
  signer: string;
  signedText: string;
  request: {
    method: string;
    target: string;
    body: string;
    statement: string;
    signatures: string[];
  };
  want: { status: number };
}

interface VectorFile {
  scheme: string;
  payloadType: string;
  audience: string;
  agents: VectorAgent[];
  vectors: Vector[];
}

const VECTORS = join(__dirname, '..', '..', '..', '..', 'docs', 'specs', 'agent-request-v2-vectors.json');
const file = JSON.parse(readFileSync(VECTORS, 'utf8')) as VectorFile;

/** The accepted vectors whose form the signer does not write, and why. */
const NOT_WRITTEN: Record<string, string> = {
  'ML-DSA-65 alone, from an agent not in hybrid mode':
    'the Ed25519 key of the credentials signs every statement',
  'statement with line breaks, indentation and its members in another order':
    'the signer writes compact JSON with the members in the order of the format',
};

const accepted = file.vectors.filter((v) => v.want.status === 200);
const written = accepted.filter((v) => !(v.name in NOT_WRITTEN));

function vectorAgent(name: string): VectorAgent {
  const agent = file.agents.find((a) => a.name === name);
  expect(agent, `agent ${name}`).toBeDefined();
  return agent as VectorAgent;
}

/**
 * Signs vector v's request with its signer's keys, at the vector's time and
 * with its nonce unless others are given, and with the signer's ML-DSA-65 key
 * as well when the vector carries an ML-DSA-65 signature.
 */
function signVector(v: Vector, given: { signedAt?: number; nonce?: Uint8Array } = {}) {
  const agent = vectorAgent(v.signer);
  const vectorStatement = JSON.parse(v.request.statement) as Record<string, string>;
  const options: AgentRequestV2Options = {};
  if (v.request.signatures.join(', ').includes('ML-DSA-65:')) {
    options.mlDsa = {
      algorithm: 'ML-DSA-65',
      privateKey: ml_dsa65.keygen(sha256(agent.mlDsa65Label as string)).secretKey,
    };
  }
  return signAgentRequestV2(
    {
      method: v.request.method as AgentRequestMethod,
      url: `${file.audience}${v.request.target}`,
      body: v.request.body === '' ? undefined : v.request.body,
    },
    {
      agentId: agent.agentId,
      privateKey: sha256(agent.ed25519Label).toString('base64'),
      publicKey: agent.ed25519,
    },
    options,
    given.signedAt ?? Date.parse(vectorStatement.timestamp) / 1000,
    given.nonce ?? Buffer.from(vectorStatement.nonce, 'base64url')
  );
}

describe('signAgentRequestV2: the accepted conformance vectors', () => {
  it('the file holds agent-request-v2 vectors, and each accepted vector is one the signer writes or a form it does not write', () => {
    expect(file.scheme).toBe('agent-request-v2');
    expect(file.payloadType).toBe(PAYLOAD_TYPE);
    expect(accepted.filter((v) => v.name in NOT_WRITTEN).map((v) => v.name).sort()).toEqual(
      Object.keys(NOT_WRITTEN).sort()
    );
    expect(written.map((v) => v.name)).toEqual([
      'Ed25519, a GET with a query',
      'Ed25519, a POST with a body',
      'hybrid mode, Ed25519 and ML-DSA-65 on two header lines',
      'hybrid mode, Ed25519 and ML-DSA-65 on one comma-separated line',
      'timestamp 30 seconds behind the server clock',
      'timestamp 30 seconds ahead of the server clock',
    ]);
  });

  it("the keys derived from the labels are the agents' registered keys", () => {
    for (const agent of file.agents) {
      expect(ed25519PublicKeyOfSeed(sha256(agent.ed25519Label))).toBe(agent.ed25519);
      if (agent.mlDsa65Label !== undefined) {
        const { publicKey } = ml_dsa65.keygen(sha256(agent.mlDsa65Label));
        expect(Buffer.from(publicKey).toString('base64')).toBe(agent.mlDsa65);
      }
    }
  });

  for (const v of written) {
    it(v.name, async () => {
      const agent = vectorAgent(v.signer);
      const headers = await signVector(v);

      const { payload } = statementOf(headers['X-Agent-Request']);
      expect(payload.toString('utf8')).toBe(v.request.statement);
      expect(pae(payload).toString('utf8')).toBe(v.signedText);
      expect(headers['X-Agent-Request-Signature']).toBe(v.request.signatures.join(', '));

      const ed25519 = headers['X-Agent-Request-Signature'].split(', ')[0];
      expect(ed25519.startsWith('Ed25519:')).toBe(true);
      const signature = Buffer.from(ed25519.slice('Ed25519:'.length), 'base64url');
      expect(nodeVerify(null, pae(payload), ed25519PublicKey(agent.ed25519), signature)).toBe(true);
    });
  }

  it('control: each written vector signed one second later, or with one nonce bit flipped, is not that vector', async () => {
    for (const v of written) {
      const vectorStatement = JSON.parse(v.request.statement) as Record<string, string>;
      const nonce = Buffer.from(vectorStatement.nonce, 'base64url');
      nonce[0] ^= 0x01;
      const changed = [
        await signVector(v, { signedAt: Date.parse(vectorStatement.timestamp) / 1000 + 1 }),
        await signVector(v, { nonce }),
      ];
      for (const headers of changed) {
        expect(statementOf(headers['X-Agent-Request']).payload.toString('utf8')).not.toBe(v.request.statement);
        expect(headers['X-Agent-Request-Signature']).not.toBe(v.request.signatures.join(', '));
      }
    }
  });
});

// The wire.

const AGENT_ID = '3f8e2b6a-1c4d-4e7f-9a0b-5d6c7e8f9a01';

function makeKey() {
  const { privateKey } = generateKeyPairSync('ed25519');
  const jwk = privateKey.export({ format: 'jwk' }) as { d: string; x: string };
  const seed = Buffer.from(jwk.d, 'base64url');
  const pub = Buffer.from(jwk.x, 'base64url');
  return {
    seed,
    issuedB64: Buffer.concat([seed, pub]).toString('base64'),
    pubB64: pub.toString('base64'),
  };
}

const key = makeKey();
const creds = { agentId: AGENT_ID, privateKey: key.issuedB64, publicKey: key.pubB64 };
const mlDsaKeys = ml_dsa65.keygen(randomBytes(32));

const INJECTED_MS = 1_700_000_123_456;

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
      // take its time from a response.
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

/**
 * What AIM's verifier compares, applied to what the loopback server received:
 * the members and signatures that do not hold, none when it would accept the
 * statement. The members must be in the format's order, and the time in whole
 * seconds, which is what the signer writes.
 */
function serverMismatches(r: Received, withMLDSA: boolean): string[] {
  const { payload, statement } = statementOf(r.headers['x-agent-request']);
  const mismatches: string[] = [];
  const members = MEMBERS.filter((m) => m !== 'body_sha256' || r.body.length > 0);
  if (Object.keys(statement).join(',') !== members.join(',')) mismatches.push('members');
  if (statement.method !== r.method) mismatches.push('method');
  if (statement.target !== r.target) mismatches.push('target');
  if (statement.audience !== base) mismatches.push('audience');
  if (r.body.length > 0 && statement.body_sha256 !== sha256(r.body).toString('base64url')) {
    mismatches.push('body_sha256');
  }
  if (statement.agent_id !== AGENT_ID) mismatches.push('agent_id');
  if (!/^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$/.test(statement.timestamp)) {
    mismatches.push('timestamp');
  }
  if (Buffer.from(statement.nonce, 'base64url').length !== 16) mismatches.push('nonce');

  const message = pae(payload);
  const values = String(r.headers['x-agent-request-signature'])
    .split(',')
    .map((v) => v.trim());
  if (values.length !== (withMLDSA ? 2 : 1)) mismatches.push('signature count');
  const ed25519 = values.find((v) => v.startsWith('Ed25519:'));
  if (
    ed25519 === undefined ||
    !nodeVerify(
      null,
      message,
      ed25519PublicKey(key.pubB64),
      Buffer.from(ed25519.slice('Ed25519:'.length), 'base64url')
    )
  ) {
    mismatches.push('Ed25519 signature');
  }
  if (withMLDSA) {
    const mlDsa = values.find((v) => v.startsWith('ML-DSA-65:'));
    const signature = Buffer.from((mlDsa ?? '').slice('ML-DSA-65:'.length), 'base64url');
    if (mlDsa === undefined || !ml_dsa65.verify(mlDsaKeys.publicKey, message, signature)) {
      mismatches.push('ML-DSA-65 signature');
    }
  }
  return mismatches;
}

async function signAndSend(
  method: AgentRequestMethod,
  pathAndQuery: string,
  body?: string | Uint8Array,
  options?: AgentRequestV2Options
): Promise<Received> {
  const url = `${base}${pathAndQuery}`;
  const headers = await createAgentRequestV2Headers({ method, url, body }, creds, options);
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

describe('createAgentRequestV2Headers: the server rule accepts what fetch sent', () => {
  it('a GET with a query, which carries no body_sha256', async () => {
    const r = await signAndSend('GET', '/api/v1/agents?limit=10&offset=0');
    expect(r.target).toBe('/api/v1/agents?limit=10&offset=0');
    expect(statementOf(r.headers['x-agent-request']).statement).not.toHaveProperty('body_sha256');
    expect(serverMismatches(r, false)).toEqual([]);
  });

  it('a POST whose body is a string, and one whose body is bytes', async () => {
    const text = JSON.stringify({ zeta: 1, alpha: { y: 2, b: 3 } });
    const r = await signAndSend('POST', '/api/v1/x', text);
    expect(r.body.toString('utf8')).toBe(text);
    expect(serverMismatches(r, false)).toEqual([]);

    const bytes = new Uint8Array([0x7b, 0x22, 0x61, 0x22, 0x3a, 0xc3, 0xa9, 0x7d]);
    const s = await signAndSend('POST', '/api/v1/x', bytes);
    expect(Buffer.compare(s.body, Buffer.from(bytes))).toBe(0);
    expect(serverMismatches(s, false)).toEqual([]);
  });

  it('a POST with an empty body, which carries no body_sha256', async () => {
    const r = await signAndSend('POST', '/api/v1/x', '');
    expect(r.body.length).toBe(0);
    expect(statementOf(r.headers['x-agent-request']).statement).not.toHaveProperty('body_sha256');
    expect(serverMismatches(r, false)).toEqual([]);
  });

  it('PUT, PATCH and DELETE', async () => {
    for (const method of ['PUT', 'PATCH', 'DELETE'] as const) {
      const r = await signAndSend(method, '/api/v1/x?y=1', method === 'DELETE' ? undefined : '{"a":1}');
      expect(r.method).toBe(method);
      expect(serverMismatches(r, false)).toEqual([]);
    }
  });

  it('a percent-encoded query is signed as sent, and a fragment is neither sent nor signed', async () => {
    const r = await signAndSend('GET', '/api/v1/x?b=a%20b&c=%2F#frag');
    expect(r.target).toBe('/api/v1/x?b=a%20b&c=%2F');
    expect(serverMismatches(r, false)).toEqual([]);
  });

  it('with an ML-DSA-65 key: one line with an Ed25519 and an ML-DSA-65 signature, both over the statement', async () => {
    const options: AgentRequestV2Options = {
      mlDsa: { algorithm: 'ML-DSA-65', privateKey: mlDsaKeys.secretKey },
    };
    const r = await signAndSend('POST', '/api/v1/x', '{"a":1}', options);
    expect(String(r.headers['x-agent-request-signature'])).toMatch(
      /^Ed25519:[A-Za-z0-9_-]{86}, ML-DSA-65:[A-Za-z0-9_-]+$/
    );
    expect(serverMismatches(r, true)).toEqual([]);
  });

  it('the server rule refuses the statement once the body, target or method differs from what was signed', async () => {
    const r = await signAndSend('POST', '/api/v1/x?a=1', '{"a":1}');
    expect(serverMismatches(r, false)).toEqual([]);
    expect(serverMismatches({ ...r, body: Buffer.from('{"a":2}') }, false)).toEqual(['body_sha256']);
    expect(serverMismatches({ ...r, body: Buffer.alloc(0) }, false)).toEqual(['members']);
    expect(serverMismatches({ ...r, target: '/api/v1/x?a=2' }, false)).toEqual(['target']);
    expect(serverMismatches({ ...r, method: 'PUT' }, false)).toEqual(['method']);
  });
});

describe('createAgentRequestV2Headers: time, nonce, audience and result', () => {
  it('the time is the host clock in whole seconds, never a response Date', async () => {
    vi.useFakeTimers({ toFake: ['Date'] });
    vi.setSystemTime(INJECTED_MS);
    // The first response carries a Date 600 s ahead; the next statement still
    // takes the host clock.
    await signAndSend('GET', '/first');
    const r = await signAndSend('GET', '/second');
    expect(statementOf(r.headers['x-agent-request']).statement.timestamp).toBe('2023-11-14T22:15:23Z');
  });

  it('each statement carries a new 16-byte nonce', async () => {
    const nonces = new Set<string>();
    for (let i = 0; i < 32; i++) {
      const headers = await createAgentRequestV2Headers({ method: 'GET', url: `${base}/p` }, creds);
      const { nonce } = statementOf(headers['X-Agent-Request']).statement;
      expect(Buffer.from(nonce, 'base64url')).toHaveLength(16);
      nonces.add(nonce);
    }
    expect(nonces.size).toBe(32);
  });

  it('the audience is the origin of the url as the URL standard writes it', async () => {
    const cases: Array<[string, string]> = [
      ['https://AIM.Example.TEST/p', 'https://aim.example.test'],
      ['https://aim.example.test:443/p', 'https://aim.example.test'],
      ['http://aim.example.test:80/p', 'http://aim.example.test'],
      ['https://aim.example.test:8443/base/p?q=1', 'https://aim.example.test:8443'],
    ];
    for (const [url, audience] of cases) {
      const headers = await createAgentRequestV2Headers({ method: 'GET', url }, creds);
      expect(statementOf(headers['X-Agent-Request']).statement.audience).toBe(audience);
    }
  });

  it('returns exactly the two headers, with one Ed25519 signature when no ML-DSA key is given', async () => {
    const headers = await createAgentRequestV2Headers({ method: 'GET', url: `${base}/p` }, creds);
    expect(Object.keys(headers).sort()).toEqual(['X-Agent-Request', 'X-Agent-Request-Signature']);
    expect(headers['X-Agent-Request-Signature']).toMatch(/^Ed25519:[A-Za-z0-9_-]{86}$/);
  });
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

describe('createAgentRequestV2Headers: refusals before signing', () => {
  type Request = Parameters<typeof createAgentRequestV2Headers>[0];
  type Credentials = Parameters<typeof createAgentRequestV2Headers>[1];

  it('refuses what createAgentRequestHeaders refuses, with the same message', async () => {
    const cases: Array<[Record<string, unknown>, Record<string, unknown>]> = [
      [{ method: 'get', url: `${base}/p` }, creds],
      [{ method: 'HEAD', url: `${base}/p` }, creds],
      [{ method: 'GET', url: 'http://user@127.0.0.1/p' }, creds],
      [{ method: 'GET', url: `${base}/p?` }, creds],
      [{ method: 'GET', url: '/api/v1/x' }, creds],
      [{ method: 'GET', url: 'ftp://127.0.0.1/p' }, creds],
      [{ method: 'POST', url: `${base}/p`, body: { a: 1 } }, creds],
      [{ method: 'GET', url: `${base}/p` }, { ...creds, agentId: '' }],
      [{ method: 'GET', url: `${base}/p` }, { ...creds, privateKey: key.seed.subarray(0, 31).toString('base64') }],
      [{ method: 'GET', url: `${base}/p` }, { ...creds, publicKey: makeKey().pubB64 }],
    ];
    for (const [request, credentials] of cases) {
      const v1 = await refusalOf(
        createAgentRequestHeaders(request as unknown as Request, credentials as unknown as Credentials)
      );
      const v2 = await refusalOf(
        createAgentRequestV2Headers(request as unknown as Request, credentials as unknown as Credentials)
      );
      expect(v2.message).toMatch(/; nothing was signed\nFix: /);
      expect(v2.message).toBe(v1.message);
    }
  });

  it('refuses an ML-DSA key whose algorithm is not ML-DSA-44, ML-DSA-65 or ML-DSA-87, that is not bytes, or whose length is not its algorithm\'s', async () => {
    const algorithm = /^the ML-DSA algorithm is not ML-DSA-44, ML-DSA-65 or ML-DSA-87; nothing was signed\nFix: /;
    const cases: Array<[unknown, RegExp]> = [
      [{ algorithm: 'ML-DSA-66', privateKey: new Uint8Array(4032) }, algorithm],
      [{ algorithm: 'Ed25519', privateKey: new Uint8Array(4032) }, algorithm],
      [{ privateKey: new Uint8Array(4032) }, algorithm],
      [null, algorithm],
      [{ algorithm: 'ML-DSA-65', privateKey: 'not-bytes' }, /^the ML-DSA private key is not bytes; nothing was signed\nFix: /],
      [
        { algorithm: 'ML-DSA-65', privateKey: new Uint8Array(4031) },
        /^the ML-DSA private key is 4031 bytes long, not 4032 as ML-DSA-65 requires; nothing was signed\nFix: /,
      ],
      [
        { algorithm: 'ML-DSA-44', privateKey: new Uint8Array(4032) },
        /^the ML-DSA private key is 4032 bytes long, not 2560 as ML-DSA-44 requires; nothing was signed\nFix: /,
      ],
    ];
    for (const [mlDsa, expected] of cases) {
      const e = await refusalOf(
        createAgentRequestV2Headers({ method: 'GET', url: `${base}/p` }, creds, {
          mlDsa,
        } as unknown as AgentRequestV2Options)
      );
      expect(e.message).toMatch(expected);
    }
    expect(mlDsaKeys.secretKey).toHaveLength(4032);
  });
});

/** The TypeScript files under dir, tests left out. */
function sourceFiles(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    const path = join(dir, entry.name);
    if (entry.isDirectory()) return sourceFiles(path);
    return entry.name.endsWith('.ts') && !entry.name.endsWith('.test.ts') ? [path] : [];
  });
}

/** A quoted relative module specifier that names the agent-request-v2 module. */
const IMPORTS_V2 = /['"](?:\.{1,2}\/)+(?:[\w.-]+\/)*agent-request-v2(?:\.js|\.ts)?['"]/;

describe('agent-request-v2 in the package', () => {
  it('the package entry point does not export it, and no SDK source outside its module imports it', () => {
    expect(Object.keys(sdk).filter((name) => /V2Headers|AgentRequestV2/.test(name))).toEqual([]);
    const importers = sourceFiles(join(__dirname, '..')).filter(
      (path) => !path.endsWith('agent-request-v2.ts') && IMPORTS_V2.test(readFileSync(path, 'utf8'))
    );
    expect(importers).toEqual([]);
    // Control: the same reading finds this test file's import.
    expect(IMPORTS_V2.test(readFileSync(join(__dirname, 'agent-request-v2.test.ts'), 'utf8'))).toBe(true);
  });

  it('writes nothing to the console when it signs or refuses', async () => {
    const spies = (['log', 'info', 'warn', 'error', 'debug'] as const).map((m) =>
      vi.spyOn(console, m).mockImplementation(() => undefined)
    );
    await createAgentRequestV2Headers({ method: 'GET', url: `${base}/p` }, creds, {
      mlDsa: { algorithm: 'ML-DSA-65', privateKey: mlDsaKeys.secretKey },
    });
    await refusalOf(createAgentRequestV2Headers({ method: 'get' as AgentRequestMethod, url: `${base}/p` }, creds));
    for (const spy of spies) expect(spy).not.toHaveBeenCalled();
  });
});
