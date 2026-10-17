import { describe, it, expect, afterEach } from 'vitest';
import { spawn } from 'child_process';
import { readFileSync } from 'fs';
import { createServer, IncomingHttpHeaders, IncomingMessage, Server, ServerResponse } from 'http';
import { AddressInfo } from 'net';
import { join } from 'path';
import { pathToFileURL } from 'url';
import * as ts from 'typescript';
import { EXCHANGE_PATH } from '../src/cli/init';
import { SDK_VERIFICATIONS_PATH } from '../src/client/sdk-verification';

/**
 * scripts/hosted-route-preflight.mjs asks a hosted AIM service, before a
 * release, whether it serves the routes the SDK's first-run flows call.
 *
 * `init` exchanged its bootstrap token at a route the hosted service did not
 * serve: POST /api/v1/onboarding/bootstrap-tokens/exchange got the same 404
 * answer as /api/v1/zz-no-such-route, while /api/v1/agents answered 401. The
 * script sends each route, and paths that cannot exist, one POST with the
 * body `{}` and no credential, and counts a route as served when its answer
 * differs from the unknown paths' answer.
 *
 * These cases run it against a loopback server and hold its route table equal
 * to the paths the SDK's sources call.
 */

const SDK_ROOT = join(__dirname, '..');
const REPO_ROOT = join(SDK_ROOT, '..', '..');
const SCRIPT = join(SDK_ROOT, 'scripts', 'hosted-route-preflight.mjs');
const DOC_PATH = join(REPO_ROOT, 'docs', 'testing', 'sdk-publish-gate.md');
const AGENTS_PATH = '/api/v1/agents';
const HOSTED_API = 'https://api.aim.opena2a.org';

interface LoggedRequest {
  method: string;
  path: string;
  headers: IncomingHttpHeaders;
  body: string;
}

interface Answer {
  status: number;
  body: string;
}

type Responder = (path: string) => Answer;

interface Stub {
  baseUrl: string;
  log: LoggedRequest[];
}

const servers: Server[] = [];

afterEach(async () => {
  await Promise.all(servers.splice(0).map((s) => new Promise<void>((done) => s.close(() => done()))));
});

/** A loopback server that answers each request with `respond(path)` and logs it. */
async function startStub(respond: Responder): Promise<Stub> {
  const log: LoggedRequest[] = [];
  const server = createServer((req: IncomingMessage, res: ServerResponse) => {
    const chunks: Buffer[] = [];
    req.on('data', (c: Buffer) => chunks.push(c));
    req.on('end', () => {
      const path = (req.url ?? '').split('?')[0];
      log.push({ method: req.method ?? '', path, headers: req.headers, body: Buffer.concat(chunks).toString('utf-8') });
      const answer = respond(path);
      res.writeHead(answer.status, { 'Content-Type': 'application/json' });
      res.end(answer.body);
    });
  });
  servers.push(server);
  await new Promise<void>((done) => server.listen(0, '127.0.0.1', () => done()));
  const { port } = server.address() as AddressInfo;
  return { baseUrl: `http://127.0.0.1:${port}`, log };
}

/** The not-found answer the AIM backend's error handler gives: it carries the time of the request. */
function notFound(): Answer {
  return {
    status: 404,
    body: JSON.stringify({ error: true, message: 'Not Found', timestamp: new Date().toISOString() }),
  };
}

function unauthorized(): Answer {
  return { status: 401, body: JSON.stringify({ error: 'No authentication token provided' }) };
}

function badRequest(): Answer {
  return { status: 400, body: JSON.stringify({ error: 'invalid request body' }) };
}

interface Run {
  code: number | null;
  stdout: string;
  stderr: string;
}

/**
 * Run the script without blocking this process's event loop: the loopback
 * server answers from the same loop, so a synchronous spawn would never get
 * an answer.
 */
function runPreflight(args: string[]): Promise<Run> {
  return new Promise((resolve, reject) => {
    const child = spawn(process.execPath, [SCRIPT, ...args], {
      cwd: SDK_ROOT,
      env: { PATH: process.env.PATH ?? '', NO_COLOR: '1' },
    });
    let stdout = '';
    let stderr = '';
    child.stdout.on('data', (c: Buffer) => (stdout += c.toString('utf-8')));
    child.stderr.on('data', (c: Buffer) => (stderr += c.toString('utf-8')));
    child.on('error', reject);
    child.on('close', (code) => resolve({ code, stdout, stderr }));
  });
}

/** The route the stub was asked for that is none of the three first-run routes. */
function isFirstRunRoute(path: string): boolean {
  return path === EXCHANGE_PATH || path === AGENTS_PATH || path === SDK_VERIFICATIONS_PATH;
}

describe('hosted-route-preflight against a loopback server', () => {
  it('exits 1 and names the exchange route when it gets the same answer as an unknown path', async () => {
    const stub = await startStub((path) =>
      path === AGENTS_PATH || path === SDK_VERIFICATIONS_PATH ? unauthorized() : notFound(),
    );

    const run = await runPreflight([stub.baseUrl]);

    expect(run.code, run.stdout + run.stderr).toBe(1);
    expect(run.stdout).toContain(
      `not served POST ${EXCHANGE_PATH} (HTTP 404, same answer as an unknown path)`,
    );
    expect(run.stdout).toContain(`served POST ${AGENTS_PATH} (HTTP 401)`);
    expect(run.stdout).toContain(`served POST ${SDK_VERIFICATIONS_PATH} (HTTP 401)`);
    expect(run.stdout).not.toContain(`not served POST ${AGENTS_PATH}`);
    expect(run.stdout).not.toContain(`not served POST ${SDK_VERIFICATIONS_PATH}`);
    // The finding carries a command that shows the route's answer and the re-run that clears it.
    expect(run.stdout).toContain(
      `verify: curl -sS -X POST -H 'Content-Type: application/json' --data '{}' -w ' HTTP %{http_code}\\n' '${stub.baseUrl}${EXCHANGE_PATH}'`,
    );
    expect(run.stdout).toContain(`node scripts/hosted-route-preflight.mjs ${stub.baseUrl}`);
  });

  it('exits 0 when every first-run route answers 400 or 401', async () => {
    const stub = await startStub((path) => {
      if (path === EXCHANGE_PATH) return unauthorized();
      if (path === AGENTS_PATH) return unauthorized();
      if (path === SDK_VERIFICATIONS_PATH) return badRequest();
      return notFound();
    });

    const run = await runPreflight([stub.baseUrl]);

    expect(run.code, run.stdout + run.stderr).toBe(0);
    expect(run.stdout).toContain(`served POST ${EXCHANGE_PATH} (HTTP 401)`);
    expect(run.stdout).toContain(`served POST ${AGENTS_PATH} (HTTP 401)`);
    expect(run.stdout).toContain(`served POST ${SDK_VERIFICATIONS_PATH} (HTTP 400)`);
    expect(run.stdout).not.toContain('not served');
  });

  it('sends every request as a POST of {} with no Authorization, bootstrap-token or cookie header', async () => {
    const stub = await startStub((path) => (path === AGENTS_PATH ? unauthorized() : notFound()));

    await runPreflight([stub.baseUrl]);

    // The unknown paths and the three routes were all asked.
    expect(stub.log.filter((r) => isFirstRunRoute(r.path)).map((r) => r.path).sort()).toEqual(
      [AGENTS_PATH, EXCHANGE_PATH, SDK_VERIFICATIONS_PATH].sort(),
    );
    expect(stub.log.filter((r) => !isFirstRunRoute(r.path)).length).toBeGreaterThanOrEqual(1);
    for (const request of stub.log) {
      expect(request.method, request.path).toBe('POST');
      expect(request.body, request.path).toBe('{}');
      expect(request.headers['content-type'], request.path).toBe('application/json');
      expect(request.headers.authorization, request.path).toBeUndefined();
      expect(request.headers['x-aim-bootstrap-token'], request.path).toBeUndefined();
      expect(request.headers.cookie, request.path).toBeUndefined();
      expect(request.headers['proxy-authorization'], request.path).toBeUndefined();
    }
  });

  it('reads a not-found answer that repeats the request path as the same answer for every path', async () => {
    // Fiber's default not-found text names the path it was asked for.
    const stub = await startStub((path) =>
      path === AGENTS_PATH || path === SDK_VERIFICATIONS_PATH
        ? unauthorized()
        : { status: 404, body: `Cannot POST ${path}` },
    );

    const run = await runPreflight([stub.baseUrl]);

    expect(run.code, run.stdout + run.stderr).toBe(1);
    expect(run.stdout).toContain(`not served POST ${EXCHANGE_PATH} (HTTP 404, same answer as an unknown path)`);
  });

  it('exits 2 without judging any route when two unknown paths get different answers', async () => {
    let n = 0;
    const stub = await startStub((path) => {
      if (isFirstRunRoute(path)) return notFound();
      n += 1;
      return { status: 404, body: JSON.stringify({ error: 'Not Found', requestId: `req-${n}` }) };
    });

    const run = await runPreflight([stub.baseUrl]);

    expect(run.code, run.stdout + run.stderr).toBe(2);
    expect(run.stderr).toContain('different answers');
    expect(run.stdout).not.toMatch(/served POST/);
  });

  it('exits 2 and names the host when the server cannot be reached', async () => {
    const stub = await startStub(() => notFound());
    const closed = servers.pop() as Server;
    await new Promise<void>((done) => closed.close(() => done()));
    const host = new URL(stub.baseUrl).host;

    const run = await runPreflight([stub.baseUrl]);

    expect(run.code, run.stdout + run.stderr).toBe(2);
    expect(run.stderr).toContain(`could not reach ${host}`);
    expect(run.stdout).not.toMatch(/served POST/);
  });

  it('exits 2 with its usage when no base URL is given, and takes none from a default', async () => {
    const run = await runPreflight([]);

    expect(run.code).toBe(2);
    expect(run.stderr).toContain('usage: node scripts/hosted-route-preflight.mjs <base-url>');
  });

  it('refuses a base URL that carries a user name or password', async () => {
    const run = await runPreflight(['https://FAKEUSER:FAKEPASS@127.0.0.1:9']);

    expect(run.code).toBe(2);
    expect(run.stderr).toContain('user name or password');
    expect(run.stderr).not.toContain('FAKEPASS');
  });
});

/**
 * The path `registerAgent` posts to, read from AIMClient.ts: the second
 * argument of the one `this.request(...)` call in the method's body.
 */
function registerAgentRoute(): { method: string; path: string } {
  const file = join(SDK_ROOT, 'src', 'client', 'AIMClient.ts');
  const source = ts.createSourceFile(file, readFileSync(file, 'utf-8'), ts.ScriptTarget.Latest, true);
  const calls: ts.CallExpression[] = [];

  const visitMethodBody = (node: ts.Node): void => {
    if (
      ts.isCallExpression(node) &&
      ts.isPropertyAccessExpression(node.expression) &&
      node.expression.expression.kind === ts.SyntaxKind.ThisKeyword &&
      node.expression.name.text === 'request'
    ) {
      calls.push(node);
    }
    ts.forEachChild(node, visitMethodBody);
  };
  const findMethod = (node: ts.Node): void => {
    if (
      ts.isClassDeclaration(node) &&
      node.name?.text === 'AIMClient'
    ) {
      for (const member of node.members) {
        if (ts.isMethodDeclaration(member) && ts.isIdentifier(member.name) && member.name.text === 'registerAgent' && member.body) {
          visitMethodBody(member.body);
        }
      }
    }
    ts.forEachChild(node, findMethod);
  };
  findMethod(source);

  expect(calls, 'AIMClient.registerAgent makes exactly one this.request(...) call').toHaveLength(1);
  const [method, path] = calls[0].arguments;
  expect(method && ts.isStringLiteral(method), 'its first argument is a string literal').toBe(true);
  expect(path && ts.isStringLiteral(path), 'its second argument is a string literal').toBe(true);
  return { method: (method as ts.StringLiteral).text, path: (path as ts.StringLiteral).text };
}

interface PreflightRoute {
  method: string;
  path: string;
}

async function preflightRoutes(): Promise<PreflightRoute[]> {
  // A computed specifier: the script is plain JavaScript with no declaration file.
  const mod = await import(pathToFileURL(SCRIPT).href);
  return mod.FIRST_RUN_ROUTES as PreflightRoute[];
}

describe('hosted-route-preflight route table', () => {
  it('equals the paths init, registerAgent and verifyAction call', async () => {
    const registerAgent = registerAgentRoute();
    expect(registerAgent.method).toBe('POST');

    const routes = await preflightRoutes();

    expect(routes.map((r) => ({ method: r.method, path: r.path }))).toEqual([
      { method: 'POST', path: EXCHANGE_PATH },
      { method: 'POST', path: registerAgent.path },
      { method: 'POST', path: SDK_VERIFICATIONS_PATH },
    ]);
  });

  it('is named in the publish gate document with the hosted API it runs against', async () => {
    const doc = readFileSync(DOC_PATH, 'utf-8');
    expect(doc).toContain(`node scripts/hosted-route-preflight.mjs ${HOSTED_API}`);
    for (const route of await preflightRoutes()) {
      expect(doc, route.path).toContain(route.path);
    }
  });
});
