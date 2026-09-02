/**
 * AIM-06 — one enforcement deadline over the whole verification call (TypeScript).
 *
 * AC3: `enforcementTimeout` (milliseconds) in AIMClientConfig, resolved
 * constructor argument → AIM_ENFORCEMENT_TIMEOUT_MS → DEFAULT_ENFORCEMENT_TIMEOUT
 * (5000). ONE deadline, started on entry to verifyAction, bounds the OAuth
 * token exchange AND the verify POST: each fetch is aborted at the time left
 * on the shared deadline, never given a fresh budget. Expiry surfaces as the
 * existing NetworkError('Request timed out').
 *
 * AC4: DEFAULT_TIMEOUT stays 30000, and no seconds-spelled environment
 * variable exists.
 *
 * Fixture: a local TCP listener (net.createServer on 127.0.0.1:0) that accepts,
 * records the request line, counts connections, and answers only what its
 * handler scripts — everything else is held open and never answered. Wall time
 * is measured by performance.now() around the single call.
 */

import net from 'node:net';
import fs from 'node:fs';
import path from 'node:path';
import { describe, it, expect, beforeEach, afterEach, afterAll } from 'vitest';

import { AIMClient } from '../src/client/AIMClient';
import type { AIMClientConfig } from '../src/types';
import { NetworkError } from '../src/exceptions';
import { generateKeyPair, toBase64 } from '../src/crypto/ed25519';

type Reply = { status: number; body: string; delayMs?: number } | 'stall';
type Handler = (requestLine: string) => Reply;

class StubServer {
  url = '';
  connections = 0;
  requestLines: string[] = [];
  private server: net.Server;
  private sockets = new Set<net.Socket>();

  private constructor(handler: Handler) {
    this.server = net.createServer((socket) => {
      this.connections += 1;
      this.sockets.add(socket);
      let data = Buffer.alloc(0);
      let answered = false;
      socket.on('data', (chunk) => {
        if (answered) return;
        data = Buffer.concat([data, chunk]);
        const headerEnd = data.indexOf('\r\n\r\n');
        if (headerEnd === -1) return;
        answered = true;
        const requestLine = data.subarray(0, data.indexOf('\r\n')).toString('latin1');
        this.requestLines.push(requestLine);
        const reply = handler(requestLine);
        if (reply === 'stall') {
          return; // accepted, recorded, never answered
        }
        const send = () => {
          if (socket.destroyed) return;
          const payload = Buffer.from(reply.body);
          socket.write(
            `HTTP/1.1 ${reply.status} X\r\n` +
              `Content-Type: application/json\r\n` +
              `Content-Length: ${payload.length}\r\n` +
              `Connection: close\r\n\r\n`
          );
          socket.write(payload);
          socket.end();
        };
        if (reply.delayMs) {
          setTimeout(send, reply.delayMs);
        } else {
          send();
        }
      });
      socket.on('error', () => undefined);
    });
  }

  static start(handler: Handler): Promise<StubServer> {
    const stub = new StubServer(handler);
    return new Promise((resolve) => {
      stub.server.listen(0, '127.0.0.1', () => {
        const address = stub.server.address() as net.AddressInfo;
        stub.url = `http://127.0.0.1:${address.port}`;
        resolve(stub);
      });
    });
  }

  close(): Promise<void> {
    for (const socket of this.sockets) socket.destroy();
    return new Promise((resolve) => this.server.close(() => resolve()));
  }
}

const stallEverything: Handler = () => 'stall';

/** Answer POST /oauth/token (after delayMs) with the TokenResponse shape from
 * the contract; hold every other request (the verify POST) open forever. */
function tokenThenStall(delayMs: number): Handler {
  return (requestLine) =>
    requestLine.startsWith('POST /oauth/token')
      ? {
          status: 200,
          delayMs,
          body: '{"access_token":"t","token_type":"Bearer","expires_in":300}',
        }
      : 'stall';
}

async function credentialedClient(
  baseUrl: string,
  config: AIMClientConfig = {}
): Promise<AIMClient> {
  const client = new AIMClient({ baseUrl, apiKey: 'test-api-key', ...config });
  const keyPair = await generateKeyPair();
  client.setCredentials({
    agentId: '550e8400-e29b-41d4-a716-446655440000',
    publicKey: toBase64(keyPair.publicKey),
    privateKey: toBase64(keyPair.privateKey),
    organizationId: 'org-1',
    createdAt: new Date().toISOString(),
  });
  return client;
}

const MANAGED_ENV = [
  'AIM_ENFORCEMENT_TIMEOUT_MS',
  'AIM_BASE_URL',
  'AIM_API_KEY',
  'AIM_ORGANIZATION_ID',
  'AIM_AGENT_ID',
  'AIM_PRIVATE_KEY',
  'AIM_PUBLIC_KEY',
] as const;
const savedEnv: Record<string, string | undefined> = {};
for (const name of MANAGED_ENV) savedEnv[name] = process.env[name];

const servers: StubServer[] = [];
async function startServer(handler: Handler): Promise<StubServer> {
  const server = await StubServer.start(handler);
  servers.push(server);
  return server;
}

beforeEach(() => {
  for (const name of MANAGED_ENV) delete process.env[name];
});

afterEach(async () => {
  while (servers.length > 0) await servers.pop()!.close();
});

afterAll(() => {
  for (const name of MANAGED_ENV) {
    if (savedEnv[name] === undefined) delete process.env[name];
    else process.env[name] = savedEnv[name];
  }
});

describe('the enforcement deadline (AIM-06)', () => {
  it(
    'AIM-06.AC3 (a) a stalled token exchange is ended by enforcementTimeout: 500 on the credentialed path',
    async () => {
      // Base (origin/main 5405500a, built dist, this fixture): STILL HANGING at
      // a 20 000 ms watchdog, 1 connection, request line POST /oauth/token —
      // the token fetch carried no signal, AbortController or timeout.
      const server = await startServer(stallEverything);
      const client = await credentialedClient(server.url, { enforcementTimeout: 500 });

      const start = performance.now();
      const error: unknown = await client.verifyAction({ action: 'db:read' }).then(
        () => null,
        (rejection: unknown) => rejection
      );
      const elapsed = performance.now() - start;

      expect(error).toBeInstanceOf(NetworkError);
      expect((error as Error).message).toMatch(/Request timed out/);
      expect(elapsed).toBeLessThan(1500);
      expect(server.connections).toBe(1);
      expect(server.requestLines[0]).toMatch(/^POST \/oauth\/token HTTP/);
    },
    10000
  );

  it(
    'AIM-06.AC3 (b) AIM_ENFORCEMENT_TIMEOUT_MS=2000 bounds the credentialed path with no constructor option',
    async () => {
      // Base (origin/main 5405500a): still hanging at the 20 000 ms watchdog.
      // A value of 2000 is a 2 s deadline — were it misread as seconds the
      // call would still be pending at the 3 000 ms bound (AC4).
      process.env.AIM_ENFORCEMENT_TIMEOUT_MS = '2000';
      const server = await startServer(stallEverything);
      const client = await credentialedClient(server.url);

      const start = performance.now();
      await expect(client.verifyAction({ action: 'db:read' })).rejects.toThrowError(
        /Request timed out/
      );
      const elapsed = performance.now() - start;

      expect(elapsed).toBeLessThan(3000);
      expect(server.requestLines[0]).toMatch(/^POST \/oauth\/token HTTP/);
    },
    10000
  );

  it(
    'AIM-06.AC3 (c) the token exchange and the verify POST spend ONE deadline, not two budgets',
    async () => {
      // Token answered after 1500 ms, verify POST never answered, deadline
      // 2000 ms: one shared deadline ends the call at ~2000 ms; two separate
      // 2000 ms budgets would take 3500 ms or more. Base reading
      // (origin/main 5405500a, host run): 31 531 ms — the verify POST hung
      // for DEFAULT_TIMEOUT. Only the 3000 ms bound is asserted.
      const server = await startServer(tokenThenStall(1500));
      const client = await credentialedClient(server.url, { enforcementTimeout: 2000 });

      const start = performance.now();
      await expect(client.verifyAction({ action: 'db:read' })).rejects.toThrowError(
        /Request timed out/
      );
      const elapsed = performance.now() - start;

      expect(elapsed).toBeLessThan(3000);
      // The second phase really started: the verify POST went out after the
      // token answer and was the fetch in flight when the deadline expired.
      expect(server.requestLines.some((line) => line.startsWith('POST /api/v1/verify'))).toBe(
        true
      );
    },
    10000
  );

  it(
    'AIM-06.AC3 (d) with no enforcement option anywhere the 5000 ms DEFAULT_ENFORCEMENT_TIMEOUT applies',
    async () => {
      // Token answered at once, verify POST never answered. Base
      // (origin/main 5405500a, token manager nulled): 30 008 ms, 1 connection.
      const server = await startServer(tokenThenStall(0));
      const client = await credentialedClient(server.url);

      const start = performance.now();
      await expect(client.verifyAction({ action: 'db:read' })).rejects.toThrowError(
        /Request timed out/
      );
      const elapsed = performance.now() - start;

      expect(elapsed).toBeGreaterThanOrEqual(4900);
      expect(elapsed).toBeLessThan(6000);
    },
    15000
  );

  it(
    'AIM-06.AC3 (e) the general timeout still bounds non-enforcement requests; verifyAction is not bound by it',
    async () => {
      // Base (origin/main 5405500a): with the token manager nulled the verify
      // POST rejected NetworkError at 509 ms under timeout: 500 — the :221
      // abort already bounded a single request; what changes is that
      // verifyAction now follows the enforcement deadline instead. The
      // non-enforcement reading below is measured by this test (base reading
      // on a non-enforcement method NOT VERIFIED at intake).
      const server = await startServer(stallEverything);
      const client = await credentialedClient(server.url, { timeout: 500 });
      // As at intake: drop the token manager so request() takes the X-API-Key
      // branch and each request goes straight to its endpoint.
      (client as unknown as { tokenManager: null }).tokenManager = null;

      const startPlain = performance.now();
      await expect(
        (
          client as unknown as {
            request: (method: string, path: string) => Promise<unknown>;
          }
        ).request('GET', '/api/v1/agents/x')
      ).rejects.toThrowError(/Request timed out/);
      const plainElapsed = performance.now() - startPlain;
      expect(plainElapsed).toBeLessThan(1500);

      const startVerify = performance.now();
      await expect(client.verifyAction({ action: 'db:read' })).rejects.toThrowError(
        /Request timed out/
      );
      const verifyElapsed = performance.now() - startVerify;
      expect(verifyElapsed).toBeGreaterThanOrEqual(4900);
      expect(verifyElapsed).toBeLessThan(6000);
    },
    15000
  );

  it('AIM-06.AC4 DEFAULT_TIMEOUT is still 30000 beside DEFAULT_ENFORCEMENT_TIMEOUT = 5000', () => {
    const source = fs.readFileSync(
      path.join(__dirname, '..', 'src', 'client', 'AIMClient.ts'),
      'utf8'
    );
    expect(source).toMatch(/const DEFAULT_TIMEOUT = 30000;/);
    expect(source).toMatch(/const DEFAULT_ENFORCEMENT_TIMEOUT = 5000;/);
  });
});
