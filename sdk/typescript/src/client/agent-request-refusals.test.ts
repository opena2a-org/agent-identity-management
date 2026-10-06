/**
 * The Fix table for a 401 answered to a request signed with agent-request-v1.
 */

import { createHash } from 'node:crypto';
import { existsSync, readdirSync, readFileSync, statSync } from 'node:fs';
import { join } from 'node:path';
import { describe, expect, it } from 'vitest';
import {
  AGENT_REQUEST_FIX_ROWS,
  agentRequestAuthenticationError,
  agentRequestFixRow,
} from './agent-request-refusals';
import { AuthenticationError } from '../exceptions';
import * as sdk from '../index';

const SDK_ROOT = join(__dirname, '..', '..');
const BACKEND = join(SDK_ROOT, '..', '..', 'apps', 'backend');

const fixOf = (id: string) => {
  const row = AGENT_REQUEST_FIX_ROWS.find((r) => r.id === id);
  if (!row) throw new Error(`no row ${id}`);
  return row.fix;
};

const REVOKED = 'Agent is not permitted to authenticate (status: revoked)';
const REVOKED_FIX =
  'This agent is revoked: ask an AIM admin or manager to register a new agent to replace it.';

function filesUnder(dir: string, keep: (path: string) => boolean): string[] {
  const out: string[] = [];
  for (const name of readdirSync(dir)) {
    const path = join(dir, name);
    if (statSync(path).isDirectory()) {
      if (name !== 'node_modules') out.push(...filesUnder(path, keep));
    } else if (keep(path)) {
      out.push(path);
    }
  }
  return out;
}

describe('agent-request 401 refusals: message', () => {
  it('a revoked agent gets the server error, then the revoked Fix, byte for byte', () => {
    const error = agentRequestAuthenticationError({ error: REVOKED });
    expect(error).toBeInstanceOf(AuthenticationError);
    expect(error.message).toBe(`${REVOKED}\nFix: ${REVOKED_FIX}`);
    expect(createHash('sha256').update(`${fixOf('agentRevoked')}\n`).digest('hex')).toBe(
      '2edd7820c21e40d9d497803f25790449e16911616881d808f1734e492d3a049a'
    );
  });

  it('a suspended agent still gets the reactivate Fix', () => {
    const suspended = 'Agent is not permitted to authenticate (status: suspended)';
    expect(agentRequestAuthenticationError({ error: suspended }).message).toBe(
      `${suspended}\nFix: Ask an AIM admin or manager to reactivate this agent.`
    );
    const pending = 'Agent is not permitted to authenticate (status: pending)';
    expect(agentRequestFixRow(pending).id).toBe('agentStatus');
  });

  it('the revoked row is matched only on the exact string', () => {
    expect(agentRequestFixRow(`${REVOKED} `).id).toBe('agentStatus');
    expect(agentRequestFixRow(REVOKED.toLowerCase()).id).toBe('other');
  });

  it('every listed string selects its row, and an unlisted string gets the catch-all row', () => {
    for (const row of AGENT_REQUEST_FIX_ROWS) {
      for (const exact of row.exact) {
        expect(agentRequestFixRow(exact).id).toBe(row.id);
        expect(agentRequestAuthenticationError({ error: exact }).message).toBe(`${exact}\nFix: ${row.fix}`);
      }
      for (const prefix of row.prefix) {
        expect(agentRequestFixRow(`${prefix}x`).id).toBe(row.id);
      }
    }
    expect(agentRequestFixRow('Something else entirely').id).toBe('other');
    expect(agentRequestFixRow(undefined).id).toBe('other');
    expect(agentRequestAuthenticationError({}).message).toBe(`Unknown error\nFix: ${fixOf('other')}`);
    expect(agentRequestAuthenticationError(null).message).toBe(`Unknown error\nFix: ${fixOf('other')}`);
  });

  it('matching is case-sensitive', () => {
    expect(agentRequestFixRow('request timestamp expired or invalid').id).toBe('other');
    expect(agentRequestFixRow('Invalid signature').id).toBe('requestChanged');
    expect(agentRequestFixRow('invalid signature').id).toBe('other');
  });

  it('a user-session refusal gets userSession, and agentRecord when the call was updateAgent', () => {
    expect(agentRequestFixRow('Authentication required').id).toBe('userSession');
    expect(agentRequestFixRow('Authentication required', undefined, 'updateAgent').id).toBe('agentRecord');
    expect(agentRequestFixRow('Authentication required', undefined, 'createNamespace').id).toBe('userSession');
    expect(agentRequestFixRow('a user session is required', 'userSessionRequired').id).toBe('userSession');
    expect(agentRequestFixRow('a user session is required', 'userSessionRequired', 'updateAgent').id).toBe('agentRecord');
  });

  it('a server error is capped as parseAPIError caps it, and the Fix follows', () => {
    const long = 'x'.repeat(600);
    const message = agentRequestAuthenticationError({ error: long }).message;
    expect(message).toBe(`${'x'.repeat(512)}… [truncated]\nFix: ${fixOf('other')}`);
  });

  it('keeps the server body as details', () => {
    const body = { error: REVOKED, reasonCode: 'agentStatusDenied' };
    expect(agentRequestAuthenticationError(body).details).toEqual(body);
    expect(agentRequestAuthenticationError(body).statusCode).toBe(401);
  });
});

describe('agent-request 401 refusals: line rules', () => {
  const NEW_IDENTITY = [/new agent/gi, /another agent/gi, /different agent/gi, /register a new/gi];
  const newIdentityHits = (text: string) =>
    NEW_IDENTITY.reduce((n, pattern) => n + (text.match(pattern)?.length ?? 0), 0);

  it('no Fix line except the revoked row offers a new identity; the withdrawn status text reads 2', () => {
    for (const row of AGENT_REQUEST_FIX_ROWS) {
      if (row.id === 'agentRevoked') continue;
      expect([row.id, newIdentityHits(row.fix)]).toEqual([row.id, 0]);
    }
    const withdrawn =
      'Ask an AIM admin or manager to reactivate this agent, or register a new agent with `registerAgent` and load its credentials.';
    expect(newIdentityHits(withdrawn)).toBe(2);
    // The revoked row names the admin or manager as the actor and no SDK call.
    expect(fixOf('agentRevoked')).not.toContain('`');
  });

  it('each Fix line is ASCII, at most 240 characters, static, and names no credential header or key option', () => {
    for (const row of AGENT_REQUEST_FIX_ROWS) {
      const line = `Fix: ${row.fix}`;
      expect(line.length).toBeLessThanOrEqual(240);
      expect(line).toMatch(/^[\x20-\x7e]+$/);
      expect(line).not.toContain('${');
      for (const banned of ['Authorization', 'X-API-Key', 'apiKey', 'AIM_API_KEY', 'Bearer']) {
        expect(line).not.toContain(banned);
      }
    }
  });

  it('each backticked reference resolves to an export, an AIMClient option or an environment variable the SDK reads', () => {
    const oauth = readFileSync(join(SDK_ROOT, 'src', 'auth', 'oauth.ts'), 'utf8');
    const envVars = new Set([...oauth.matchAll(/process\.env\.([A-Z_]+)/g)].map((m) => m[1]));
    const types = readFileSync(join(SDK_ROOT, 'src', 'types', 'index.ts'), 'utf8');
    const configBlock = types.slice(types.indexOf('export interface AIMClientConfig'));
    const options = new Set(
      [...configBlock.slice(0, configBlock.indexOf('\n}')).matchAll(/^\s+([a-zA-Z]+)\??:/gm)].map((m) => m[1])
    );
    const refs = AGENT_REQUEST_FIX_ROWS.flatMap((r) => [...r.fix.matchAll(/`([^`]+)`/g)].map((m) => m[1]));
    expect(refs.length).toBeGreaterThan(0);
    for (const ref of refs) {
      const resolves = ref in sdk || options.has(ref) || envVars.has(ref);
      expect([ref, resolves]).toEqual([ref, true]);
    }
  });

  it('the struck finality clause appears nowhere in the SDK source or a built package', () => {
    const needle = ['will', 'not', 'work', 'again'].join(' ');
    const roots = [join(SDK_ROOT, 'src'), join(SDK_ROOT, 'dist')].filter((d) => existsSync(d));
    const files = roots.flatMap((d) => filesUnder(d, (p) => /\.(ts|mts|cts|js|mjs|cjs)$/.test(p)));
    expect(files.length).toBeGreaterThan(0);
    expect(files.filter((f) => readFileSync(f, 'utf8').includes(needle))).toEqual([]);
    // Control: the detector reads a line that carries the phrase.
    expect(`its keys ${needle}`.includes(needle)).toBe(true);
  });
});

describe('agent-request 401 refusals: server strings', () => {
  // Listed for the hosted deployment's server; this repository's server does not send it.
  const NOT_IN_THIS_SERVER = new Set(['Agent is in hybrid mode and the request is not hybrid-signed']);

  it('each self-hosted key string appears verbatim in the server source', () => {
    const goSources = filesUnder(BACKEND, (p) => p.endsWith('.go') && !p.endsWith('_test.go'))
      .map((p) => readFileSync(p, 'utf8'))
      .join('\n');
    // The revoked string is composed from the status prefix and the status
    // name, so it is checked as that composition below.
    const keys = AGENT_REQUEST_FIX_ROWS.flatMap((r) => [...r.exact, ...r.prefix]).filter(
      (k) => !NOT_IN_THIS_SERVER.has(k) && k !== REVOKED
    );
    expect(keys.length).toBe(20);
    for (const key of keys) {
      expect([key, goSources.includes(key)]).toEqual([key, true]);
    }
    // The status refusal is built from this prefix and the status name.
    expect(goSources).toContain('"Agent is not permitted to authenticate (status: " + string(status) + ")"');
    expect(goSources).toMatch(/AgentStatusRevoked\s+AgentStatus = "revoked"/);
  });
});
