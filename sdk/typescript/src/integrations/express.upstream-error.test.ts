/**
 * An upstream 5xx through the README's Express example (#450).
 *
 * Before the fix, `aimErrorHandler` answered only denials and authentication
 * failures and passed every other SDK error to `next(error)`. An AIM server
 * answering 500 therefore reached Express's default handler, which outside
 * `NODE_ENV=production` rendered an HTML page carrying the stack trace and
 * local file paths, and printed the stack to stderr.
 *
 * The README's middleware stack runs here against a fake AIM server on
 * loopback, with the real SDK client making real signed requests. `express`
 * is only a peer dependency of this package, so the app is served by
 * `expressLike` below: node:http plus the parts of Express's contract these
 * middlewares use (`req.path`, `req.query`, `req.ip`, `res.status().json()`,
 * `res.headersSent`), Express's error dispatch (an error skips to the next
 * four-argument handler), and its development-mode final handler, which
 * prints the stack and renders it as HTML. That final handler is what the fix
 * must keep an SDK error away from.
 */

import { createServer, type IncomingMessage, type Server, type ServerResponse } from 'node:http';
import type { AddressInfo } from 'node:net';
import type { NextFunction, Request, Response } from 'express';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { generateKeyPair, toBase64 } from '../crypto/ed25519';
import { aimErrorHandler, createAIMMiddleware, verifyAction } from './express';

const AGENT_ID = '5f0c8a52-3b1e-4c1f-9a7d-2d6f3e1b9c40';

type Handler = (...args: never[]) => unknown;

/** node:http serving a middleware list with Express's dispatch and final handler. */
function expressLike(stack: Array<{ method?: string; path?: string; fn: Handler }>): Server {
  return createServer((rawReq: IncomingMessage, rawRes: ServerResponse) => {
    const url = new URL(rawReq.url ?? '/', 'http://localhost');
    const req = Object.assign(rawReq, {
      path: url.pathname,
      query: Object.fromEntries(url.searchParams),
      ip: rawReq.socket.remoteAddress,
    }) as unknown as Request;
    const res = Object.assign(rawRes, {
      status(code: number) {
        rawRes.statusCode = code;
        return res;
      },
      json(body: unknown) {
        rawRes.setHeader('Content-Type', 'application/json; charset=utf-8');
        rawRes.end(JSON.stringify(body));
        return res;
      },
    }) as unknown as Response;

    let i = 0;
    const next: NextFunction = (err?: unknown) => {
      while (i < stack.length) {
        const layer = stack[i++];
        if (layer.method && layer.method !== rawReq.method) continue;
        if (layer.path && layer.path !== url.pathname) continue;
        const isErrorHandler = layer.fn.length === 4;
        if (err !== undefined && !isErrorHandler) continue;
        if (err === undefined && isErrorHandler) continue;
        const run = layer.fn as (...args: unknown[]) => unknown;
        void (err !== undefined ? run(err, req, res, next) : run(req, res, next));
        return;
      }
      // Express's final handler outside NODE_ENV=production.
      if (err !== undefined) {
        const stackText = (err as Error).stack ?? String(err);
        console.error(stackText);
        rawRes.statusCode = 500;
        rawRes.setHeader('Content-Type', 'text/html; charset=utf-8');
        rawRes.end(`<!DOCTYPE html><html><body><pre>${stackText}</pre></body></html>`);
      } else {
        rawRes.statusCode = 404;
        rawRes.end();
      }
    };
    next();
  });
}

let aimServer: Server;
let appServer: Server;
let aimStatus: number;
let aimBody: unknown;

function listen(server: Server): Promise<string> {
  return new Promise((resolve) => {
    server.listen(0, '127.0.0.1', () => {
      resolve(`http://127.0.0.1:${(server.address() as AddressInfo).port}`);
    });
  });
}

function close(server: Server | undefined): Promise<void> {
  return new Promise((resolve) => (server ? server.close(() => resolve()) : resolve()));
}

/** The README example, pointed at the fake AIM server. */
function readmeApp(baseUrl: string): Server {
  return expressLike([
    { fn: createAIMMiddleware({ baseUrl, apiKey: 'test-api-key' }) },
    { method: 'POST', path: '/api/data', fn: verifyAction('data:write') },
    {
      method: 'POST',
      path: '/api/data',
      fn: (_req: Request, res: Response) => res.json({ success: true }),
    },
    { fn: aimErrorHandler },
  ]);
}

beforeEach(async () => {
  const { privateKey, publicKey } = await generateKeyPair();
  vi.stubEnv('AIM_AGENT_ID', AGENT_ID);
  vi.stubEnv('AIM_PRIVATE_KEY', toBase64(privateKey));
  vi.stubEnv('AIM_PUBLIC_KEY', toBase64(publicKey));
  vi.stubEnv('AIM_ORGANIZATION_ID', 'b2a7c1d4-0e9f-4a3b-8c6d-5e4f3a2b1c0d');

  aimStatus = 500;
  aimBody = { error: 'boom' };
  aimServer = createServer((req, res) => {
    res.setHeader('Content-Type', 'application/json');
    if (req.url === '/oauth/token') {
      res.end(JSON.stringify({ access_token: 'token', expires_in: 300 }));
    } else if (req.url === `/api/v1/agents/${AGENT_ID}`) {
      res.end(JSON.stringify({ id: AGENT_ID, trustScore: 0.9 }));
    } else if (req.url === '/api/v1/verify') {
      res.statusCode = aimStatus;
      res.end(JSON.stringify(aimBody));
    } else {
      res.statusCode = 404;
      res.end('{}');
    }
  });
  const aimUrl = await listen(aimServer);
  appServer = readmeApp(aimUrl);
});

afterEach(async () => {
  vi.unstubAllEnvs();
  vi.restoreAllMocks();
  await close(appServer);
  await close(aimServer);
});

async function postData(): Promise<{ status: number; contentType: string; text: string }> {
  const appUrl = await listen(appServer);
  const response = await fetch(`${appUrl}/api/data`, { method: 'POST' });
  return {
    status: response.status,
    contentType: response.headers.get('content-type') ?? '',
    text: await response.text(),
  };
}

describe('aimErrorHandler with an upstream error, through the README example', () => {
  it('answers an upstream 500 as JSON in the Fastify plugin shape', async () => {
    const stderr = vi.spyOn(console, 'error').mockImplementation(() => {});

    const { status, contentType, text } = await postData();

    expect(status).toBe(500);
    expect(contentType).toMatch(/application\/json/);
    expect(JSON.parse(text)).toEqual({
      statusCode: 500,
      code: 'API_ERROR',
      error: 'Internal Server Error',
      message: 'boom',
    });
    // No stack, no file path, no HTML: the body is exactly the four fields.
    expect(text).not.toMatch(/<!DOCTYPE|<pre>|\bat \S+ \(|node_modules|\.[jt]s:\d+/);
    // The final handler prints the stack with console.error.
    expect(stderr).not.toHaveBeenCalled();
  });

  it('keeps an upstream 503 status rather than flattening it to 500', async () => {
    aimStatus = 503;
    aimBody = { message: 'maintenance' };

    const { status, text } = await postData();

    expect(status).toBe(503);
    expect(JSON.parse(text)).toEqual({
      statusCode: 503,
      code: 'API_ERROR',
      error: 'Service Unavailable',
      message: 'maintenance',
    });
  });

  it('still answers a server-side denial with the verification body', async () => {
    aimStatus = 403;
    aimBody = { message: 'policy refused' };

    const { status, text } = await postData();

    expect(status).toBe(403);
    expect(JSON.parse(text)).toEqual({ error: 'Action denied', reason: 'policy refused' });
  });

  it('lets a success reach the route handler', async () => {
    aimStatus = 200;
    aimBody = { actionAllowed: true, verified: true, trustScore: 0.9, agentId: AGENT_ID };

    const { status, text } = await postData();

    expect(status).toBe(200);
    expect(JSON.parse(text)).toEqual({ success: true });
  });

  it('answers an unreachable AIM on a cold start as a JSON network error', async () => {
    // A fresh app has no cached token, so the first thing the SDK does is the
    // token exchange, and that fetch is the one that rejects.
    const port = await new Promise<number>((resolve) => {
      const probe = createServer();
      probe.listen(0, '127.0.0.1', () => {
        const { port: free } = probe.address() as AddressInfo;
        probe.close(() => resolve(free));
      });
    });
    appServer = readmeApp(`http://127.0.0.1:${port}`);
    const stderr = vi.spyOn(console, 'error').mockImplementation(() => {});

    const { status, contentType, text } = await postData();

    expect(status).toBe(500);
    expect(contentType).toMatch(/application\/json/);
    const body = JSON.parse(text) as Record<string, unknown>;
    expect(Object.keys(body).sort()).toEqual(['code', 'error', 'message', 'statusCode']);
    expect(body).toMatchObject({
      statusCode: 500,
      code: 'NETWORK_ERROR',
      error: 'Internal Server Error',
    });
    expect(body.message).toContain('/oauth/token');
    expect(body.message).toContain('ECONNREFUSED');
    expect(text).not.toMatch(/<!DOCTYPE|<pre>|\bat \S+ \(|node_modules|\.[jt]s:\d+/);
    expect(stderr).not.toHaveBeenCalled();
  });
});
