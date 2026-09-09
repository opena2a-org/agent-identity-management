/**
 * Tests for OAuth token-response parsing, typed /oauth/token errors, and
 * env-credential diagnostics (AIM-11 items 1, 2 and 9).
 *
 * - The token endpoint may answer in RFC 6749 snake_case ({"access_token", ...})
 *   or the SDK's historical camelCase; both must be read, and a missing expiry
 *   must not poison the token cache into NaN.
 * - A non-2xx from /oauth/token must surface as the SDK's typed error family
 *   with the body excerpt capped, not as a bare Error carrying the raw body.
 * - Partial AIM_* credential env must name the missing variable(s) instead of
 *   the same silent null as a clean environment.
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { OAuthTokenManager, loadCredentialsFromEnv } from './oauth';
import { AIMClient } from '../client/AIMClient';
import { AIMError, AuthenticationError, RateLimitError } from '../exceptions';
import { generateKeyPair, toBase64 } from '../crypto/ed25519';
import type { AgentCredentials } from '../types';

const mockFetch = vi.fn();
vi.stubGlobal('fetch', mockFetch);

const BASE_URL = 'http://127.0.0.1:8080';
const CRED_VARS = ['AIM_AGENT_ID', 'AIM_PRIVATE_KEY', 'AIM_PUBLIC_KEY', 'AIM_ORGANIZATION_ID'];

async function realCredentials(agentId = 'agent-token-shape'): Promise<AgentCredentials> {
  const { privateKey, publicKey } = await generateKeyPair();
  return {
    agentId,
    privateKey: toBase64(privateKey),
    publicKey: toBase64(publicKey),
    organizationId: 'org-token-shape',
    createdAt: new Date().toISOString(),
  };
}

function mockTokenResponse(body: Record<string, unknown>): void {
  mockFetch.mockResolvedValueOnce({
    ok: true,
    json: async () => body,
  });
}

function mockApiOk(body: Record<string, unknown> = { id: 'agent-token-shape' }): void {
  mockFetch.mockResolvedValueOnce({
    ok: true,
    text: async () => JSON.stringify(body),
  });
}

/** Headers of the most recent fetch call. */
function lastRequestHeaders(): Record<string, string> {
  const call = mockFetch.mock.calls.at(-1);
  return (call?.[1]?.headers ?? {}) as Record<string, string>;
}

const savedEnv: Record<string, string | undefined> = {};

beforeEach(() => {
  vi.clearAllMocks();
  for (const name of ['AIM_BASE_URL', 'AIM_API_KEY', ...CRED_VARS]) {
    savedEnv[name] = process.env[name];
    delete process.env[name];
  }
});

afterEach(() => {
  for (const [name, value] of Object.entries(savedEnv)) {
    if (value === undefined) delete process.env[name];
    else process.env[name] = value;
  }
  vi.restoreAllMocks();
});

describe('token response shapes (item 1)', () => {
  it('AIM-11.AC1 RFC 6749 snake_case token response yields "Bearer <token>", never "Bearer undefined"', async () => {
    const client = new AIMClient({ baseUrl: BASE_URL });
    client.setCredentials(await realCredentials());

    mockTokenResponse({ access_token: 'rfc-shaped-token', token_type: 'Bearer', expires_in: 3600 });
    mockApiOk();

    const agent = await client.getAgent();
    expect(agent).not.toBeNull();

    const headers = lastRequestHeaders();
    expect(headers['Authorization']).toBe('Bearer rfc-shaped-token');
    expect(headers['Authorization']).not.toContain('undefined');
  });

  it('AIM-11.AC1 camelCase token response keeps working', async () => {
    const client = new AIMClient({ baseUrl: BASE_URL });
    client.setCredentials(await realCredentials());

    mockTokenResponse({ accessToken: 'camel-shaped-token', tokenType: 'Bearer', expiresIn: 3600 });
    mockApiOk();

    await client.getAgent();
    expect(lastRequestHeaders()['Authorization']).toBe('Bearer camel-shaped-token');
  });

  it('AIM-11.AC1 expires_in drives the cache expiry for the RFC shape', async () => {
    const manager = new OAuthTokenManager(BASE_URL, await realCredentials());

    mockTokenResponse({ access_token: 'rfc-cached', token_type: 'Bearer', expires_in: 3600 });
    await manager.getAccessToken();

    expect(manager.hasValidToken()).toBe(true);
    await manager.getAccessToken();
    expect(mockFetch).toHaveBeenCalledTimes(1);
  });

  it('AIM-11.AC1 a missing expiry must not poison the cache into NaN', async () => {
    const manager = new OAuthTokenManager(BASE_URL, await realCredentials());

    // No expiresIn/expires_in at all: the cache must still hold a real number,
    // not NaN (which makes every validity check false and forces a refetch on
    // every single request).
    mockTokenResponse({ accessToken: 'no-expiry-token', tokenType: 'Bearer' });
    const first = await manager.getAccessToken();
    expect(first).toBe('no-expiry-token');

    expect(manager.hasValidToken()).toBe(true);
    await manager.getAccessToken();
    expect(mockFetch).toHaveBeenCalledTimes(1);
  });

  it('AIM-11.AC1 an unparseable expiry must not poison the cache into NaN', async () => {
    const manager = new OAuthTokenManager(BASE_URL, await realCredentials());

    mockTokenResponse({ access_token: 'weird-expiry-token', token_type: 'Bearer', expires_in: 'soon' });
    await manager.getAccessToken();

    expect(manager.hasValidToken()).toBe(true);
    await manager.getAccessToken();
    expect(mockFetch).toHaveBeenCalledTimes(1);
  });
});

describe('typed /oauth/token errors (item 2)', () => {
  function mockTokenFailure(status: number, body: string, headers?: Record<string, string>): void {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status,
      headers: new Headers(headers ?? {}),
      text: async () => body,
    });
  }

  it('AIM-11.AC2 a 429 from /oauth/token surfaces as RateLimitError', async () => {
    const manager = new OAuthTokenManager(BASE_URL, await realCredentials());
    mockTokenFailure(429, JSON.stringify({ message: 'slow down' }), { 'Retry-After': '7' });

    let caught: unknown;
    try {
      await manager.getAccessToken();
    } catch (err) {
      caught = err;
    }
    expect(caught).toBeInstanceOf(RateLimitError);
    expect((caught as RateLimitError).retryAfter).toBe(7);
  });

  it('AIM-11.AC2 a 401 from /oauth/token surfaces as AuthenticationError', async () => {
    const manager = new OAuthTokenManager(BASE_URL, await realCredentials());
    mockTokenFailure(401, JSON.stringify({ message: 'bad assertion' }));

    let caught: unknown;
    try {
      await manager.getAccessToken();
    } catch (err) {
      caught = err;
    }
    expect(caught).toBeInstanceOf(AuthenticationError);
    expect((caught as AuthenticationError).statusCode).toBe(401);
  });

  it('AIM-11.AC2 other statuses surface as an AIMError subclass with code and statusCode', async () => {
    const manager = new OAuthTokenManager(BASE_URL, await realCredentials());
    mockTokenFailure(503, 'upstream unavailable');

    let caught: unknown;
    try {
      await manager.getAccessToken();
    } catch (err) {
      caught = err;
    }
    expect(caught).toBeInstanceOf(AIMError);
    expect((caught as AIMError).statusCode).toBe(503);
    expect((caught as AIMError).code).toBeTruthy();
  });

  it('AIM-11.AC2 the embedded response-body excerpt is capped at 500 characters', async () => {
    const manager = new OAuthTokenManager(BASE_URL, await realCredentials());
    // The release test observed a 2095-character HTML body embedded verbatim.
    mockTokenFailure(502, 'x'.repeat(2095));

    let caught: unknown;
    try {
      await manager.getAccessToken();
    } catch (err) {
      caught = err;
    }
    expect(caught).toBeInstanceOf(AIMError);
    const message = (caught as AIMError).message;
    expect(message).toContain('Failed to acquire token');
    expect((message.match(/x/g) ?? []).length).toBeLessThanOrEqual(500);
  });
});

describe('env-credential diagnostics (item 9, loader half)', () => {
  it('AIM-11.AC10 a partial credential environment names the missing variables', async () => {
    process.env.AIM_AGENT_ID = 'agent-partial';
    process.env.AIM_PUBLIC_KEY = 'pk-partial';
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {});

    const creds = loadCredentialsFromEnv();

    expect(creds).toBeNull();
    expect(warn).toHaveBeenCalledTimes(1);
    const message = String(warn.mock.calls[0]?.[0] ?? '');
    expect(message).toContain('AIM_PRIVATE_KEY');
    expect(message).toContain('AIM_ORGANIZATION_ID');
  });

  it('AIM-11.AC10 a clean environment stays silent and returns null', () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {});

    expect(loadCredentialsFromEnv()).toBeNull();
    expect(warn).not.toHaveBeenCalled();
  });

  it('AIM-11.AC10 a complete environment still loads without warning', () => {
    process.env.AIM_AGENT_ID = 'agent-full';
    process.env.AIM_PRIVATE_KEY = 'sk-full';
    process.env.AIM_PUBLIC_KEY = 'pk-full';
    process.env.AIM_ORGANIZATION_ID = 'org-full';
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {});

    const creds = loadCredentialsFromEnv();

    expect(creds).not.toBeNull();
    expect(creds!.agentId).toBe('agent-full');
    expect(warn).not.toHaveBeenCalled();
  });
});
