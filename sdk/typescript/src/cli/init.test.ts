/**
 * `npx @opena2a/aim-sdk init` against a local stand-in for the AIM exchange
 * endpoint (a real HTTP server on 127.0.0.1, so the request is the one fetch
 * actually sends: headers, body, URL, redirect handling).
 *
 * What is pinned: the token travels only in the X-AIM-Bootstrap-Token header,
 * never in the URL, the body, the output or the credentials file; the key pair
 * is generated locally and only its public half is sent; the credentials land
 * in ~/.aim/agents/<name>.json with 0600 (directory 0700) in the layout the
 * Python SDK reads; refusals leave nothing on disk; a redirect is not followed.
 */

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createServer, IncomingMessage, Server, ServerResponse } from 'http';
import { AddressInfo } from 'net';
import { existsSync, mkdirSync, mkdtempSync, readFileSync, readdirSync, rmSync, statSync, writeFileSync } from 'fs';
import { tmpdir } from 'os';
import { join } from 'path';
import * as ed from '@noble/ed25519';
import {
  agentCredentialsFileName,
  dashboardUrlFor,
  EXCHANGE_PATH,
  initHelpText,
  invalidServerUrlReason,
  reportedDashboardUrl,
  runInit,
  scrubToken,
} from './init';
import { runAimArp } from '../arp/cli/aim-arp-main';

const TOKEN = 'aim_ob_Xk3q9TfAbCdEfGhIjKlMnOpQrStUvWxYz0123456789_-a';

interface Seen {
  method: string;
  url: string;
  headers: IncomingMessage['headers'];
  body: string;
}

type Handler = (req: Seen, res: ServerResponse) => void;

let server: Server;
let baseUrl: string;
let seen: Seen[];
let handler: Handler;
let home: string;
let outLines: string[];
let errLines: string[];

function created(res: ServerResponse, body: Record<string, unknown>) {
  res.writeHead(201, { 'Content-Type': 'application/json', 'Cache-Control': 'no-store' });
  res.end(JSON.stringify(body));
}

function exchangeOk(req: Seen, res: ServerResponse) {
  const body = JSON.parse(req.body);
  created(res, {
    agentId: '550e8400-e29b-41d4-a716-446655440000',
    organizationId: '660e8400-e29b-41d4-a716-446655440000',
    name: body.name,
    displayName: body.name,
    status: 'pending',
    publicKey: body.publicKey,
    aimUrl: baseUrl,
  });
}

function refuse(status: number, body: Record<string, unknown>): Handler {
  return (_req, res) => {
    res.writeHead(status, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify(body));
  };
}

async function startServer(): Promise<{ server: Server; url: string }> {
  const s = createServer((req, res) => {
    const chunks: Buffer[] = [];
    req.on('data', (c) => chunks.push(c));
    req.on('end', () => {
      const entry = {
        method: req.method ?? '',
        url: req.url ?? '',
        headers: req.headers,
        body: Buffer.concat(chunks).toString('utf8'),
      };
      seen.push(entry);
      handler(entry, res);
    });
  });
  await new Promise<void>((resolve) => s.listen(0, '127.0.0.1', resolve));
  return { server: s, url: `http://127.0.0.1:${(s.address() as AddressInfo).port}` };
}

function deps(env: NodeJS.ProcessEnv = { AIM_BOOTSTRAP_TOKEN: TOKEN }) {
  return {
    env,
    homeDir: home,
    out: (l: string) => outLines.push(l),
    err: (l: string) => errLines.push(l),
  };
}

function allOutput(): string {
  return [...outLines, ...errLines].join('\n');
}

function credentialsPath(name = 'my-first-agent'): string {
  return join(home, '.aim', 'agents', agentCredentialsFileName(name));
}

beforeEach(async () => {
  seen = [];
  handler = exchangeOk;
  outLines = [];
  errLines = [];
  home = mkdtempSync(join(tmpdir(), 'aim-init-'));
  ({ server, url: baseUrl } = await startServer());
});

afterEach(async () => {
  vi.restoreAllMocks();
  await new Promise<void>((resolve) => server.close(() => resolve()));
  rmSync(home, { recursive: true, force: true });
});

describe('a token from AIM_BOOTSTRAP_TOKEN registers my-first-agent', () => {
  it('sends one exchange request with the token only in the header and a locally generated public key', async () => {
    const code = await runInit(['--url', baseUrl], deps());
    expect(code, allOutput()).toBe(0);

    expect(seen).toHaveLength(1);
    const req = seen[0];
    expect(req.method).toBe('POST');
    expect(req.url).toBe(EXCHANGE_PATH);
    expect(req.headers['x-aim-bootstrap-token']).toBe(TOKEN);
    expect(req.headers['content-type']).toContain('application/json');

    const body = JSON.parse(req.body);
    expect(body).toEqual({ name: 'my-first-agent', agentType: 'custom', publicKey: expect.any(String) });
    expect(Buffer.from(body.publicKey, 'base64')).toHaveLength(32);
    expect(req.body).not.toContain(TOKEN);
    expect(req.body).not.toMatch(/private/i);
  });

  it('saves Python-compatible credentials 0600 in a 0700 directory, with a key pair that signs and verifies', async () => {
    expect(await runInit(['--url', baseUrl], deps())).toBe(0);

    const file = credentialsPath();
    expect(statSync(file).mode & 0o777).toBe(0o600);
    expect(statSync(join(home, '.aim', 'agents')).mode & 0o777).toBe(0o700);
    // The temporary file used for the atomic write is gone.
    expect(readdirSync(join(home, '.aim', 'agents'))).toEqual(['my-first-agent.json']);

    const raw = readFileSync(file, 'utf8');
    expect(raw).not.toContain(TOKEN);
    const creds = JSON.parse(raw);
    expect(creds).toMatchObject({
      agent_id: '550e8400-e29b-41d4-a716-446655440000',
      organization_id: '660e8400-e29b-41d4-a716-446655440000',
      aim_url: baseUrl,
      status: 'pending',
      schemaVersion: '1.0',
      type: 'agent',
      name: 'my-first-agent',
    });
    expect(Number.isNaN(Date.parse(creds.registered_at))).toBe(false);

    // The public key saved is the one the server was sent.
    expect(creds.public_key).toBe(JSON.parse(seen[0].body).publicKey);

    // 64-byte seed+public private key, as the Python SDK reads it.
    const priv = new Uint8Array(Buffer.from(creds.private_key, 'base64'));
    expect(priv).toHaveLength(64);
    const pub = new Uint8Array(Buffer.from(creds.public_key, 'base64'));
    expect(Buffer.from(priv.slice(32)).equals(Buffer.from(pub))).toBe(true);
    expect(Buffer.from(await ed.getPublicKeyAsync(priv.slice(0, 32))).equals(Buffer.from(pub))).toBe(true);
    const msg = new TextEncoder().encode('first agent');
    const sig = await ed.signAsync(msg, priv.slice(0, 32));
    expect(await ed.verifyAsync(sig, msg, pub)).toBe(true);
  });

  it('prints the dashboard URL for the agent and never the token', async () => {
    expect(await runInit(['--url', `${baseUrl}/`], deps())).toBe(0);
    const text = allOutput();
    expect(text).toContain(`${baseUrl}/dashboard/agents/550e8400-e29b-41d4-a716-446655440000`);
    expect(text).toContain(credentialsPath());
    expect(text).not.toContain(TOKEN);
    expect(text).not.toContain(TOKEN.slice(7, 15));
    expect(text).not.toContain(JSON.parse(readFileSync(credentialsPath(), 'utf8')).private_key);
  });

  it('links to the dashboard address the server reports, not one derived from the API address', async () => {
    handler = (req, res) => {
      const body = JSON.parse(req.body);
      created(res, {
        agentId: '550e8400-e29b-41d4-a716-446655440000',
        name: body.name,
        status: 'pending',
        publicKey: body.publicKey,
        aimUrl: baseUrl,
        dashboardUrl: 'https://console.example.com/',
      });
    };
    expect(await runInit(['--url', baseUrl], deps())).toBe(0);
    expect(outLines).toContain('  Dashboard:    https://console.example.com/dashboard/agents/550e8400-e29b-41d4-a716-446655440000');
  });

  it('uses AIM_URL when --url is absent', async () => {
    const code = await runInit([], deps({ AIM_BOOTSTRAP_TOKEN: TOKEN, AIM_URL: baseUrl }));
    expect(code, allOutput()).toBe(0);
    expect(seen).toHaveLength(1);
  });
});

describe('flags', () => {
  it('--token and --name register under the given name', async () => {
    const code = await runInit(['--token', TOKEN, '--url', baseUrl, '--name', 'billing-bot'], deps({}));
    expect(code, allOutput()).toBe(0);
    expect(seen[0].headers['x-aim-bootstrap-token']).toBe(TOKEN);
    expect(JSON.parse(seen[0].body).name).toBe('billing-bot');
    expect(existsSync(credentialsPath('billing-bot'))).toBe(true);
    // The flag form is visible in the process list; the user is told so.
    expect(errLines.join('\n')).toContain('AIM_BOOTSTRAP_TOKEN');
    expect(allOutput()).not.toContain(TOKEN);
  });

  it('accepts --flag=value forms, and --token wins over the environment', async () => {
    const other = 'aim_ob_fromEnvironmentfromEnvironmentfromEnvironm';
    const code = await runInit(
      [`--token=${TOKEN}`, `--url=${baseUrl}`, '--name=ops agent'],
      deps({ AIM_BOOTSTRAP_TOKEN: other }),
    );
    expect(code, allOutput()).toBe(0);
    expect(seen[0].headers['x-aim-bootstrap-token']).toBe(TOKEN);
    expect(JSON.parse(seen[0].body).name).toBe('ops agent');
    // The file name is sanitized exactly as the Python SDK does it.
    expect(existsSync(join(home, '.aim', 'agents', 'ops_agent.json'))).toBe(true);
  });

  it('--help prints usage without contacting a server', async () => {
    expect(await runInit(['--help'], deps())).toBe(0);
    expect(outLines.join('\n')).toBe(initHelpText());
    expect(seen).toHaveLength(0);
  });

  it('an unknown option or a bare positional is refused without echoing a pasted token', async () => {
    expect(await runInit([TOKEN], deps({}))).toBe(1);
    expect(await runInit([`--tokn=${TOKEN}`], deps({}))).toBe(1);
    expect(seen).toHaveLength(0);
    expect(allOutput()).not.toContain(TOKEN);
    expect(errLines.join('\n')).toContain('--tokn');
  });

  it('a flag without its value is refused', async () => {
    expect(await runInit(['--url'], deps())).toBe(1);
    expect(errLines.join('\n')).toContain('--url needs a value');
    expect(seen).toHaveLength(0);
  });
});

describe('refusals before any request', () => {
  it('no token: says where to get one and sends nothing', async () => {
    expect(await runInit(['--url', baseUrl], deps({}))).toBe(1);
    expect(errLines.join('\n')).toContain('AIM_BOOTSTRAP_TOKEN=<token> npx @opena2a/aim-sdk init');
    expect(seen).toHaveLength(0);
  });

  it('a value that is not a bootstrap token is refused and not printed', async () => {
    const notAToken = 'sk-FAKE-not-a-bootstrap-token';
    expect(await runInit(['--url', baseUrl], deps({ AIM_BOOTSTRAP_TOKEN: notAToken }))).toBe(1);
    expect(seen).toHaveLength(0);
    expect(allOutput()).not.toContain(notAToken);
    expect(allOutput()).not.toContain('shell history');
  });

  it('a --token value that is not a bootstrap token is refused with the shell history note', async () => {
    const notAToken = 'not-a-token';
    expect(await runInit(['--token', notAToken, '--url', baseUrl], deps({}))).toBe(1);
    expect(seen).toHaveLength(0);
    expect(errLines[0]).toBe('Note: --token is visible in the process list and shell history; AIM_BOOTSTRAP_TOKEN is not.');
    expect(errLines[1]).toContain('that is not a bootstrap token');
    expect(allOutput()).not.toContain(notAToken);
  });

  it('an invalid --url is refused', async () => {
    for (const bad of ['', 'ftp://aim.example.com', 'not a url', `${baseUrl}/?token=${TOKEN}`]) {
      errLines = [];
      expect(await runInit([`--url=${bad}`], deps())).toBe(1);
      expect(errLines.join('\n')).not.toContain(TOKEN);
    }
    expect(seen).toHaveLength(0);
  });

  it('existing credentials for the name are kept and the token is not spent', async () => {
    mkdirSync(join(home, '.aim', 'agents'), { recursive: true });
    writeFileSync(credentialsPath(), '{"agent_id":"existing"}');
    expect(await runInit(['--url', baseUrl], deps())).toBe(1);
    expect(seen).toHaveLength(0);
    expect(readFileSync(credentialsPath(), 'utf8')).toBe('{"agent_id":"existing"}');
    expect(errLines.join('\n')).toContain('--name');
  });
});

describe('server refusals leave nothing on disk and never print the token', () => {
  const cases: Array<[string, number, Record<string, unknown>, string]> = [
    ['expired', 401, { error: 'expired', code: 'bootstrap_token_expired' }, 'expired'],
    ['used', 401, { error: 'used', code: 'bootstrap_token_used' }, 'already registered an agent'],
    ['revoked', 401, { error: 'revoked', code: 'bootstrap_token_revoked' }, 'revoked'],
    ['invalid', 401, { error: 'invalid', code: 'bootstrap_token_invalid' }, 'not recognised'],
    ['duplicate name', 409, { error: 'agent name already exists' }, '--name <another-name>'],
    ['rate limited', 429, { error: 'Too many requests' }, 'Wait a minute'],
    ['no endpoint', 404, { error: 'Cannot POST' }, 'no onboarding endpoint'],
    ['server error echoing the token', 500, { error: `Failed for ${TOKEN}` }, 'aim_ob_[REDACTED]'],
  ];

  for (const [label, status, body, expected] of cases) {
    it(`${label} (HTTP ${status})`, async () => {
      handler = refuse(status, body);
      expect(await runInit(['--url', baseUrl], deps())).toBe(1);
      expect(errLines.join('\n')).toContain(expected);
      expect(allOutput()).not.toContain(TOKEN);
      expect(existsSync(join(home, '.aim', 'agents', 'my-first-agent.json'))).toBe(false);
    });
  }

  it('a 201 without an agent id saves nothing', async () => {
    handler = (_req, res) => created(res, { status: 'pending' });
    expect(await runInit(['--url', baseUrl], deps())).toBe(1);
    expect(existsSync(credentialsPath())).toBe(false);
  });

  it('a server that registers a different public key is refused and nothing is saved', async () => {
    handler = (req, res) => {
      exchangeOk({ ...req, body: JSON.stringify({ ...JSON.parse(req.body), publicKey: 'AAAA' }) }, res);
    };
    expect(await runInit(['--url', baseUrl], deps())).toBe(1);
    expect(errLines.join('\n')).toContain('different public key');
    expect(existsSync(credentialsPath())).toBe(false);
  });
});

describe('transport', () => {
  it('a redirect is not followed, so the token header never reaches the redirect target', async () => {
    const target = await startServer();
    try {
      handler = (req, res) => {
        if (req.url === EXCHANGE_PATH && seen.length === 1) {
          res.writeHead(307, { Location: `${target.url}${EXCHANGE_PATH}` });
          res.end();
        } else {
          exchangeOk(req, res);
        }
      };
      expect(await runInit(['--url', baseUrl], deps())).toBe(1);
      // Both servers share the `seen` log: only the first request happened.
      expect(seen).toHaveLength(1);
      expect(errLines.join('\n')).toContain('redirect');
      expect(existsSync(credentialsPath())).toBe(false);
    } finally {
      await new Promise<void>((resolve) => target.server.close(() => resolve()));
    }
  });

  it('an unreachable server is reported with the address and the token is not printed', async () => {
    const dead = await startServer();
    await new Promise<void>((resolve) => dead.server.close(() => resolve()));
    expect(await runInit(['--url', dead.url], deps())).toBe(1);
    expect(errLines.join('\n')).toContain(`could not reach the AIM server at ${dead.url}`);
    expect(allOutput()).not.toContain(TOKEN);
  });

  it('a server that never answers times out', async () => {
    handler = () => {
      // never respond
    };
    expect(await runInit(['--url', baseUrl], { ...deps(), timeoutMs: 200 })).toBe(1);
    expect(errLines.join('\n')).toContain('no answer within');
    server.closeAllConnections();
  });
});

describe('helpers', () => {
  it('dashboardUrlFor maps API addresses to dashboard addresses', () => {
    expect(dashboardUrlFor('https://api.aim.opena2a.org')).toBe('https://aim.opena2a.org');
    expect(dashboardUrlFor('https://api.community.opena2a.org/')).toBe('https://community.opena2a.org');
    expect(dashboardUrlFor('http://localhost:8080')).toBe('http://localhost:3000');
    expect(dashboardUrlFor('http://127.0.0.1:8080/')).toBe('http://127.0.0.1:3000');
    expect(dashboardUrlFor('http://[::1]:8080')).toBe('http://[::1]:3000');
    expect(dashboardUrlFor('https://aim.example.com')).toBe('https://aim.example.com');
    expect(dashboardUrlFor('https://example.com/aim/api/')).toBe('https://example.com/aim');
  });

  it('dashboardUrlFor keeps port 8080 off loopback and never cuts "/api" out of a host name', () => {
    expect(dashboardUrlFor('https://aim.example.com:8080')).toBe('https://aim.example.com:8080');
    expect(dashboardUrlFor('https://api.example.com')).toBe('https://api.example.com');
    expect(dashboardUrlFor('https://aim-api.example.com')).toBe('https://aim-api.example.com');
    expect(dashboardUrlFor('https://example.com:18080/apis')).toBe('https://example.com:18080/apis');
    // A look-alike of a hosted API host is not the hosted dashboard.
    expect(dashboardUrlFor('https://api.aim.opena2a.org.example.com')).toBe('https://api.aim.opena2a.org.example.com');
  });

  it('reportedDashboardUrl accepts an http(s) dashboard address and refuses anything else', () => {
    expect(reportedDashboardUrl('https://aim.example.com/', 'https://aim-api.example.com')).toBe('https://aim.example.com');
    expect(reportedDashboardUrl('http://localhost:3000', 'http://localhost:8080')).toBe('http://localhost:3000');
    // The unset FRONTEND_URL default, reported by a server reached elsewhere.
    expect(reportedDashboardUrl('http://localhost:3000', 'https://aim-api.example.com')).toBeNull();
    expect(reportedDashboardUrl(undefined, 'https://aim-api.example.com')).toBeNull();
    expect(reportedDashboardUrl('', 'https://aim-api.example.com')).toBeNull();
    expect(reportedDashboardUrl('javascript:alert(1)', 'https://aim-api.example.com')).toBeNull();
    expect(reportedDashboardUrl('https://aim.example.com/?next=x', 'https://aim-api.example.com')).toBeNull();
  });

  it('agentCredentialsFileName keeps letters and digits of any script, as the Python SDK does', () => {
    // Python: "".join(c if c.isalnum() or c in "-_" else "_" for c in name)
    expect(agentCredentialsFileName('café')).toBe('café.json');
    expect(agentCredentialsFileName('エージェント 1')).toBe('エージェント_1.json');
    expect(agentCredentialsFileName('ops agent/../x')).toBe('ops_agent____x.json');
    // One character outside the BMP is one "_", not one per UTF-16 unit.
    expect(agentCredentialsFileName('\u{1F916}bot')).toBe('_bot.json');
    expect(agentCredentialsFileName('my-first_agent')).toBe('my-first_agent.json');
  });

  it('scrubToken redacts every token-shaped string', () => {
    expect(scrubToken(`a ${TOKEN} b ${TOKEN}`)).toBe('a aim_ob_[REDACTED] b aim_ob_[REDACTED]');
  });

  it('invalidServerUrlReason accepts http(s) server addresses only', () => {
    expect(invalidServerUrlReason('https://aim.example.com')).toBeNull();
    expect(invalidServerUrlReason('http://localhost:8080')).toBeNull();
    expect(invalidServerUrlReason('https://FAKEUSER:FAKEPASS@aim.example.com')).not.toBeNull();
  });
});

describe('the shipped bin dispatches init', () => {
  it('`aim-arp init --help` reaches init, and the bin help lists it', async () => {
    const log: string[] = [];
    vi.spyOn(console, 'log').mockImplementation((...a: unknown[]) => {
      log.push(a.join(' '));
    });
    expect(await runAimArp(['init', '--help'])).toBe(0);
    expect(log.join('\n')).toContain('AIM_BOOTSTRAP_TOKEN=<token> npx @opena2a/aim-sdk init');
    log.length = 0;
    expect(await runAimArp(['help'])).toBe(0);
    expect(log.join('\n')).toContain('npx @opena2a/aim-sdk init');
  });
});
