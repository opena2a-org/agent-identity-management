/**
 * #449: the README's Express and Fastify examples, driven with a real
 * AIMClient against a fake AIM server on loopback.
 *
 * `createAIMMiddleware({ baseUrl, apiKey })` gave the middleware no agent
 * identity and had no option that could, so every verified route answered 401
 * and AIM was never contacted. The integrations now take `credentials` (or a
 * ready `client`), and a partially set environment names the missing
 * variable instead of "No credentials available".
 *
 * Express and Fastify are optional peers and are not installed for the test
 * run, so the middleware functions are invoked directly with request/response
 * doubles; everything past them (client, token exchange, signed verify POST)
 * is the real code talking HTTP to the fake server.
 */

import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { createServer, type IncomingMessage, type Server } from 'http';
import type { AddressInfo } from 'net';
import { createAIMMiddleware, verifyAction } from './express';
import { aimPlugin, verifyAction as fastifyVerifyAction } from './fastify';
import { AIMClient } from '../client/AIMClient';
import { generateKeyPair, toBase64 } from '../crypto/ed25519';
import type { AgentCredentials } from '../types';

const AGENT_ID = '550e8400-e29b-41d4-a716-446655440000';

type Seen = { method: string; url: string; body: string };

function readBody(req: IncomingMessage): Promise<string> {
  return new Promise((resolve) => {
    let data = '';
    req.on('data', (c) => (data += c));
    req.on('end', () => resolve(data));
  });
}

async function startFakeAim(): Promise<{ server: Server; baseUrl: string; seen: Seen[] }> {
  const seen: Seen[] = [];
  const server = createServer(async (req, res) => {
    const body = await readBody(req);
    seen.push({ method: req.method ?? '', url: req.url ?? '', body });
    res.setHeader('content-type', 'application/json');
    if (req.method === 'POST' && req.url?.startsWith('/api/v1/oauth/token')) {
      res.end('{"access_token":"t","token_type":"Bearer","expires_in":300}');
    } else if (req.method === 'GET' && req.url === `/api/v1/agents/${AGENT_ID}`) {
      res.end(JSON.stringify({ id: AGENT_ID, name: 'readme-agent', trustScore: 0.9 }));
    } else if (req.method === 'POST' && req.url === '/api/v1/verify') {
      res.end(
        JSON.stringify({
          verified: true,
          actionAllowed: true,
          agentId: AGENT_ID,
          agentName: 'readme-agent',
          trustScore: 0.9,
          riskLevel: 'low',
          timestamp: new Date().toISOString(),
        }),
      );
    } else {
      res.statusCode = 404;
      res.end('{"error":"not found"}');
    }
  });
  await new Promise<void>((r) => server.listen(0, '127.0.0.1', r));
  const { port } = server.address() as AddressInfo;
  return { server, baseUrl: `http://127.0.0.1:${port}`, seen };
}

async function agentCredentials(): Promise<AgentCredentials> {
  const kp = await generateKeyPair();
  return {
    agentId: AGENT_ID,
    publicKey: toBase64(kp.publicKey),
    privateKey: toBase64(kp.privateKey),
    organizationId: 'org-1',
    createdAt: new Date().toISOString(),
  };
}

/** Minimal Express request/response doubles. */
function expressDoubles() {
  const req: any = { path: '/api/data', method: 'POST', query: {}, ip: '127.0.0.1' };
  const res: any = {
    statusCode: 200,
    payload: undefined as unknown,
    status(code: number) {
      this.statusCode = code;
      return this;
    },
    json(p: unknown) {
      this.payload = p;
      return this;
    },
  };
  return { req, res };
}

function run(mw: any, req: any, res: any): Promise<unknown> {
  return new Promise((resolve) => {
    Promise.resolve(mw(req, res, (err?: unknown) => resolve(err ?? 'next'))).then(() => resolve('done'));
  });
}

const ENV = ['AIM_AGENT_ID', 'AIM_PRIVATE_KEY', 'AIM_PUBLIC_KEY', 'AIM_ORGANIZATION_ID', 'AIM_BASE_URL'];
let savedEnv: Record<string, string | undefined>;
let fake: Awaited<ReturnType<typeof startFakeAim>>;

beforeEach(async () => {
  savedEnv = Object.fromEntries(ENV.map((k) => [k, process.env[k]]));
  for (const k of ENV) delete process.env[k];
  fake = await startFakeAim();
});

afterEach(async () => {
  for (const k of ENV) {
    if (savedEnv[k] === undefined) delete process.env[k];
    else process.env[k] = savedEnv[k];
  }
  await new Promise<void>((r) => fake.server.close(() => r()));
});

describe('README Express example against a fake AIM server (#449)', () => {
  it('verifies the action at AIM when the middleware is given credentials', async () => {
    const aim = createAIMMiddleware({ baseUrl: fake.baseUrl, credentials: await agentCredentials() });
    const guard = verifyAction('data:write');
    const { req, res } = expressDoubles();

    expect(await run(aim, req, res)).toBe('next');
    expect(await run(guard, req, res)).toBe('next');

    expect(res.statusCode).toBe(200);
    expect(req.aim.verified).toBe(true);
    const verify = fake.seen.find((s) => s.method === 'POST' && s.url === '/api/v1/verify');
    expect(verify, `AIM saw: ${fake.seen.map((s) => `${s.method} ${s.url}`).join(', ')}`).toBeDefined();
    expect(JSON.parse(verify!.body).action_type ?? JSON.parse(verify!.body).action).toBe('data:write');
  });

  it('accepts a ready client', async () => {
    const client = new AIMClient({ baseUrl: fake.baseUrl });
    client.setCredentials(await agentCredentials());
    const { req, res } = expressDoubles();
    await run(createAIMMiddleware({ client }), req, res);
    expect(await run(verifyAction('data:write'), req, res)).toBe('next');
    expect(req.aim.client).toBe(client);
    expect(fake.seen.some((s) => s.url === '/api/v1/verify')).toBe(true);
  });

  it('refuses client and credentials together', async () => {
    const client = new AIMClient({ baseUrl: fake.baseUrl });
    expect(() => createAIMMiddleware({ client, credentials: {} as AgentCredentials })).toThrow(
      /either `client` or `credentials`/,
    );
  });

  it('names a missing AIM_ORGANIZATION_ID instead of "No credentials available"', async () => {
    const creds = await agentCredentials();
    process.env.AIM_AGENT_ID = creds.agentId;
    process.env.AIM_PRIVATE_KEY = creds.privateKey;
    process.env.AIM_PUBLIC_KEY = creds.publicKey;
    const warn = console.warn;
    console.warn = () => {};
    try {
      const { req, res } = expressDoubles();
      await run(createAIMMiddleware({ baseUrl: fake.baseUrl }), req, res);
      await run(verifyAction('data:write'), req, res);
      expect(res.statusCode).toBe(401);
      expect(JSON.stringify(res.payload)).toContain('AIM_ORGANIZATION_ID');
      expect(fake.seen.some((s) => s.url === '/api/v1/verify')).toBe(false);
    } finally {
      console.warn = warn;
    }
  });
});

describe('README Fastify example against a fake AIM server (#449)', () => {
  it('verifies the action at AIM when the plugin is given credentials', async () => {
    const hooks: Record<string, (req: any, reply: any) => Promise<void>> = {};
    const fastify: any = {
      decorateRequest: () => {},
      setErrorHandler: () => {},
      addHook: (name: string, fn: (req: any, reply: any) => Promise<void>) => {
        hooks[name] = fn;
      },
    };
    const credentials = await agentCredentials();
    await new Promise<void>((resolve, reject) =>
      (aimPlugin as any)(fastify, { baseUrl: fake.baseUrl, credentials }, (err?: Error) =>
        err ? reject(err) : resolve(),
      ),
    );

    const req: any = { url: '/api/data', routeOptions: { url: '/api/data' }, method: 'POST', query: {}, ip: '127.0.0.1' };
    const reply: any = {
      statusCode: 200,
      status(c: number) {
        this.statusCode = c;
        return this;
      },
      send(p: unknown) {
        this.payload = p;
        return this;
      },
    };
    for (const hook of Object.values(hooks)) await hook(req, reply);
    await fastifyVerifyAction('data:write')(req, reply);

    expect(reply.statusCode).toBe(200);
    expect(req.aim.verified).toBe(true);
    expect(fake.seen.some((s) => s.method === 'POST' && s.url === '/api/v1/verify')).toBe(true);
  });
});
