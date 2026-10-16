/**
 * `init` sends the bootstrap token over `http:` only to this machine.
 *
 * An `http:` server address is accepted when its host is `localhost`,
 * `127.0.0.1` or `[::1]`, decided from the URL text and never by resolving it.
 * Any other `http:` address ends the command with exit 1 before the exchange
 * request: nothing is sent, nothing is saved and the token is not printed.
 * `https:` addresses are unchanged.
 *
 * `runInit` is driven with a recording `fetch`, so no request reaches the
 * network and the recorder holds every request the command made.
 */

import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { existsSync, mkdtempSync, readdirSync, rmSync } from 'fs';
import { tmpdir } from 'os';
import { join } from 'path';
import {
  BOOTSTRAP_TOKEN_ENV,
  BOOTSTRAP_TOKEN_HEADER,
  BOOTSTRAP_TOKEN_PREFIX,
  EXCHANGE_PATH,
  invalidServerUrlReason,
  runInit,
} from './init';

const TOKEN = [BOOTSTRAP_TOKEN_PREFIX, 'Pl41nUrl', 'q'.repeat(30), '_-z'].join('');

interface Recorded {
  url: string;
  tokenHeader: string | null;
}

let calls: Recorded[];
let home: string;
let outLines: string[];
let errLines: string[];

function recordingFetch(): typeof fetch {
  return (async (input: string | URL | Request, init?: RequestInit) => {
    const headers = new Headers(init?.headers);
    calls.push({ url: String(input), tokenHeader: headers.get(BOOTSTRAP_TOKEN_HEADER) });
    const sent = JSON.parse(String(init?.body ?? '{}')) as { publicKey?: string };
    return new Response(
      JSON.stringify({
        agentId: '550e8400-e29b-41d4-a716-446655440000',
        organizationId: '660e8400-e29b-41d4-a716-446655440000',
        status: 'pending',
        publicKey: sent.publicKey,
      }),
      { status: 201, headers: { 'Content-Type': 'application/json' } },
    );
  }) as typeof fetch;
}

function deps(env: NodeJS.ProcessEnv = {}) {
  return {
    env: { [BOOTSTRAP_TOKEN_ENV]: TOKEN, ...env },
    homeDir: home,
    fetch: recordingFetch(),
    out: (l: string) => outLines.push(l),
    err: (l: string) => errLines.push(l),
  };
}

function allOutput(): string {
  return [...outLines, ...errLines].join('\n');
}

function savedFiles(): string[] {
  const dir = join(home, '.aim', 'agents');
  return existsSync(dir) ? readdirSync(dir) : [];
}

/** The recorded requests, with whether each carried the token instead of its value. */
function requestsMade(): { url: string; carriesToken: boolean }[] {
  return calls.map((c) => ({ url: c.url, carriesToken: c.tokenHeader === TOKEN }));
}

function expectNothingSentOrSaved(): void {
  expect(requestsMade()).toEqual([]);
  expect(savedFiles()).toEqual([]);
  const output = allOutput();
  expect(output).not.toContain(TOKEN);
  expect(output).not.toContain(TOKEN.slice(BOOTSTRAP_TOKEN_PREFIX.length));
}

beforeEach(() => {
  calls = [];
  outLines = [];
  errLines = [];
  home = mkdtempSync(join(tmpdir(), 'aim-init-cleartext-url-'));
});

afterEach(() => {
  rmSync(home, { recursive: true, force: true });
});

describe('an http: address whose host is not this machine is refused before the exchange', () => {
  it('--url http://aim.example.test exits 1 with zero requests, nothing saved and the token not printed', async () => {
    const code = await runInit(['--url', 'http://aim.example.test'], deps());

    expectNothingSentOrSaved();
    expect(code, allOutput()).toBe(1);
    expect(outLines).toEqual([]);
  });

  it('names the host, says nothing was sent or saved, and prints the https: form of --url', async () => {
    expect(await runInit(['--url', 'http://aim.example.test:8080/api'], deps())).toBe(1);

    expectNothingSentOrSaved();
    const output = errLines.join('\n');
    expect(output).toContain('aim.example.test');
    expect(output).toContain('http:');
    expect(output).toMatch(/nothing was sent or saved/i);
    expect(output).toContain('--url https://aim.example.test');
  });

  it('applies to an address from AIM_URL as well as --url', async () => {
    expect(await runInit([], deps({ AIM_URL: 'http://aim.example.test' }))).toBe(1);

    expectNothingSentOrSaved();
    expect(errLines.join('\n')).toContain('aim.example.test');
  });

  it.each([
    'http://localhost.example.test',
    'http://127.0.0.1.example.test',
    'http://10.0.0.5:8080',
    'http://[2001:db8::1]:8080',
  ])('refuses %s, whose host only resembles or is not a loopback host', async (url) => {
    expect(await runInit(['--url', url], deps())).toBe(1);

    expectNothingSentOrSaved();
  });

  it('shows a host that holds a token-shaped string redacted', async () => {
    const tokenShapedHost = [BOOTSTRAP_TOKEN_PREFIX, 'h0stlabel'].join('');
    expect(await runInit(['--url', `http://${tokenShapedHost}.example.test`], deps())).toBe(1);

    expectNothingSentOrSaved();
    const output = allOutput();
    expect(output).not.toContain(tokenShapedHost);
    expect(output).toContain(`${BOOTSTRAP_TOKEN_PREFIX}[REDACTED]`);
  });

  it('leaves the shared address check unchanged: it still accepts an http: address on any host', () => {
    expect(invalidServerUrlReason('http://aim.example.test')).toBeNull();
  });
});

describe('http: on a loopback host and https: on any host reach the exchange', () => {
  it.each([
    'http://127.0.0.1:8080',
    'http://localhost:8080',
    'http://[::1]:8080',
    'http://LOCALHOST:8080',
    'https://aim.example.test',
  ])('%s sends exactly one exchange request carrying the token header', async (url) => {
    const code = await runInit(['--url', url], deps());

    expect(code, allOutput()).toBe(0);
    expect(calls).toHaveLength(1);
    expect(calls[0].url.endsWith(EXCHANGE_PATH)).toBe(true);
    expect(calls[0].tokenHeader).toBe(TOKEN);
    expect(savedFiles()).toEqual(['my-first-agent.json']);
    expect(allOutput()).not.toContain(TOKEN);
  });

  it('accepts a loopback http: address from AIM_URL', async () => {
    expect(await runInit([], deps({ AIM_URL: 'http://localhost:8080' }))).toBe(0);

    expect(calls).toHaveLength(1);
    expect(calls[0].url).toBe(`http://localhost:8080${EXCHANGE_PATH}`);
    expect(calls[0].tokenHeader).toBe(TOKEN);
  });
});
