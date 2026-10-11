/**
 * AIMClient sends each request to a route AIM serves, in the shape it reads (#662).
 *
 * verifyAction posted to /api/v1/verify, which AIM does not serve. It posts to
 * /api/v1/sdk-api/verifications, signed in the body, and makes no token
 * exchange. reportCapabilities registers each capability on
 * /api/v1/sdk-api/agents/:id/capabilities/register; it and attestIsolation sign
 * as the agent (agent-request-v1), because the SDK-API routes refuse the OAuth
 * service token. updateAgent sends only the fields the server applies.
 *
 * The requests are pinned in tests/fixtures/sdk_requests. The backend's
 * sdk_typescript_request_contract_test.go posts the same bodies to the handlers
 * of those routes, and sdk_typescript_routes_contract_test.go checks that the
 * server mounts the paths.
 */

import fs from 'node:fs';
import path from 'node:path';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { AIMClient } from './AIMClient';
import { verificationSigningMessage } from './sdk-verification';
import {
  ActionDeniedError,
  AuthorizationError,
  ConfigurationError,
  NetworkError,
  VerificationError,
} from '../exceptions';
import { fromBase64, generateKeyPair, toBase64, verify } from '../crypto/ed25519';
import { NetworkIsolation, SandboxType } from '../isolation/index';
import type { AgentCredentials, RiskLevel } from '../types';

const BASE_URL = 'https://aim.example.com';
// RFC 8032 test 1: the key the verification fixtures are signed with.
const FIXTURE_SEED = 'nWGxne/9WmC6hEr0kuwsxERJxWl7MmkZcDusAxyuf2A=';
const FIXTURE_PUBLIC_KEY = '11qYAYKxCrfVS/7TyWQHOg7hcvPapiMlrwIaaPcHURo=';
const AGENT_ID = 'b3c1d2e4-5f60-4a7b-8c9d-0e1f2a3b4c5d';
const AGENT_ENV = ['AIM_AGENT_ID', 'AIM_PRIVATE_KEY', 'AIM_PUBLIC_KEY', 'AIM_ORGANIZATION_ID', 'AIM_BASE_URL', 'AIM_API_KEY'];

interface RequestFixture {
  method: string;
  path: string;
  body: Record<string, unknown>;
}

function readFixture(name: string): RequestFixture {
  const file = path.join(__dirname, '..', '..', 'tests', 'fixtures', 'sdk_requests', `${name}.json`);
  return JSON.parse(fs.readFileSync(file, 'utf8')) as RequestFixture;
}

const mockFetch = vi.fn();

function answer(status: number, body: unknown) {
  return {
    ok: status >= 200 && status < 300,
    status,
    headers: new Headers(),
    json: async () => body,
    text: async () => JSON.stringify(body),
  };
}

function fixtureClient(agentId: string): AIMClient {
  const client = new AIMClient({ baseUrl: BASE_URL, apiKey: 'dashboard-key' });
  client.setCredentials({
    agentId,
    privateKey: FIXTURE_SEED,
    publicKey: FIXTURE_PUBLIC_KEY,
    organizationId: 'org-1',
    createdAt: '2026-10-11T00:00:00.000Z',
  });
  return client;
}

async function agentClient(): Promise<{ client: AIMClient; credentials: AgentCredentials; publicKey: Uint8Array }> {
  const keyPair = await generateKeyPair();
  const credentials: AgentCredentials = {
    agentId: AGENT_ID,
    privateKey: toBase64(keyPair.privateKey),
    publicKey: toBase64(keyPair.publicKey),
    organizationId: 'org-1',
    createdAt: '2026-10-11T00:00:00.000Z',
  };
  const client = new AIMClient({ baseUrl: BASE_URL, apiKey: 'dashboard-key' });
  client.setCredentials(credentials);
  return { client, credentials, publicKey: keyPair.publicKey };
}

function sent(i: number): { url: string; method: string; headers: Record<string, string>; body: string } {
  const [url, init] = mockFetch.mock.calls[i];
  return { url: String(url), method: init.method, headers: init.headers, body: init.body };
}

/** Checks the agent-request-v1 headers the SDK-API group's middleware verifies. */
async function expectSignedAsAgent(
  request: ReturnType<typeof sent>,
  publicKey: Uint8Array
): Promise<void> {
  expect(request.headers['Authorization']).toBeUndefined();
  expect(request.headers['X-API-Key']).toBeUndefined();
  expect(request.headers['X-Agent-ID']).toBe(AGENT_ID);
  expect(request.headers['X-Public-Key']).toBe(toBase64(publicKey));
  const target = request.url.slice(BASE_URL.length);
  const message = new TextEncoder().encode(
    `${request.method}\n${target}\n${request.headers['X-Timestamp']}\n${request.body}`
  );
  expect(await verify(fromBase64(request.headers['X-Signature']), message, publicKey)).toBe(true);
}

let savedEnv: Record<string, string | undefined>;

beforeEach(() => {
  savedEnv = Object.fromEntries(AGENT_ENV.map((name) => [name, process.env[name]]));
  for (const name of AGENT_ENV) delete process.env[name];
  mockFetch.mockReset();
  vi.stubGlobal('fetch', mockFetch);
});

afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
  for (const name of AGENT_ENV) {
    if (savedEnv[name] === undefined) delete process.env[name];
    else process.env[name] = savedEnv[name];
  }
});

describe('verifyAction', () => {
  it.each(['verify_action', 'verify_action_minimal'])(
    'sends the %s fixture: its method, path and body, with no token exchange and no credential header',
    async (name) => {
      const fx = readFixture(name);
      const body = fx.body;
      vi.useFakeTimers({ toFake: ['Date'] });
      vi.setSystemTime(new Date(body.timestamp as string));
      mockFetch.mockResolvedValueOnce(
        answer(201, { id: 'evt-1', status: 'approved', trustScore: 0.8, enforcementMode: 'strict', riskLevel: 'low' })
      );

      const result = await fixtureClient(body.agentId as string).verifyAction({
        action: body.capability as string,
        resource: body.resource as string | undefined,
        // The minimal request is made with no context at all.
        context: name === 'verify_action' ? (body.context as Record<string, unknown>) : undefined,
        riskLevel: body.riskLevel as RiskLevel | undefined,
      });

      expect(result.actionAllowed).toBe(true);
      expect(mockFetch).toHaveBeenCalledTimes(1);
      const request = sent(0);
      expect(request.url).toBe(`${BASE_URL}${fx.path}`);
      expect(request.method).toBe(fx.method);
      expect(JSON.parse(request.body)).toEqual(body);
      for (const header of ['Authorization', 'X-API-Key', 'X-AIM-Signature', 'X-Agent-ID']) {
        expect(request.headers[header]).toBeUndefined();
      }
    }
  );

  it('signs the message the server rebuilds from the body', async () => {
    const body = readFixture('verify_action').body;
    const message = verificationSigningMessage({
      agentId: body.agentId as string,
      capability: body.capability as string,
      resource: body.resource as string,
      context: body.context as Record<string, unknown>,
      timestamp: body.timestamp as string,
    });

    expect(
      await verify(
        fromBase64(body.signature as string),
        new TextEncoder().encode(message),
        fromBase64(FIXTURE_PUBLIC_KEY)
      )
    ).toBe(true);
    expect(message.startsWith('{"action_type": "file:write", "agent_id": ')).toBe(true);
    // Names in UTF-8 byte order: U+E000 before U+1F600, the reverse of UTF-16 order.
    expect(message.indexOf(`"${String.fromCodePoint(0xe000)}"`)).toBeLessThan(
      message.indexOf(`"${String.fromCodePoint(0x1f600)}"`)
    );
    expect(message.indexOf('"Zeta"')).toBeLessThan(message.indexOf('"zeta"'));
    // U+2028 is escaped as the server's encoder escapes it; HTML characters are not.
    expect(message).toContain('line\\u2028paragraph\\u2029end');
    expect(message).not.toContain(String.fromCodePoint(0x2028));
    expect(message).toContain('<a href=\\"x\\">&amp;</a>');
    expect(message).toContain('[3, 1.5, -2, 1e+21, 1e-7, 0.000001, 0.1, 12345678901234567000, true, false, null]');
  });

  it('signs an absent resource as null and a lone surrogate as U+FFFD, as the server reads them', () => {
    const message = verificationSigningMessage({
      agentId: AGENT_ID,
      capability: 'db:read',
      context: { note: `a${String.fromCharCode(0xd800)}b` },
      timestamp: '2026-10-11T12:00:00.000Z',
    });
    expect(message).toBe(
      `{"action_type": "db:read", "agent_id": "${AGENT_ID}", "context": {"note": "a${String.fromCodePoint(0xfffd)}b"}, ` +
        '"resource": null, "timestamp": "2026-10-11T12:00:00.000Z"}'
    );
  });

  it('returns the decision for an action allowed in monitoring mode', async () => {
    const { client } = await agentClient();
    mockFetch.mockResolvedValueOnce(
      answer(201, {
        id: 'evt-2',
        status: 'auto-approved',
        approvedBy: 'monitoring-mode',
        trustScore: 0.61,
        enforcementMode: 'monitoring',
        riskLevel: 'medium',
        riskAutoDetected: true,
      })
    );

    const result = await client.verifyAction({ action: 'api:call' });

    expect(result).toMatchObject({
      verified: true,
      actionAllowed: true,
      agentId: AGENT_ID,
      trustScore: 0.61,
      riskLevel: 'medium',
      eventId: 'evt-2',
      status: 'auto-approved',
      enforcementMode: 'monitoring',
    });
  });

  it('throws ActionDeniedError with the server reason and trust score for a 403 denial', async () => {
    const { client } = await agentClient();
    mockFetch.mockResolvedValueOnce(
      answer(403, {
        id: 'evt-3',
        status: 'denied',
        denialReason: 'Capability file:write is not granted',
        trustScore: 0.42,
        enforcementMode: 'strict',
        riskLevel: 'high',
      })
    );

    const error = await client.verifyAction({ action: 'file:write' }).catch((e: unknown) => e);

    expect(error).toBeInstanceOf(ActionDeniedError);
    expect((error as ActionDeniedError).reason).toBe('Capability file:write is not granted');
    expect((error as ActionDeniedError).trustScore).toBe(0.42);
    expect((error as ActionDeniedError).details).toEqual({ status: 'denied', eventId: 'evt-3' });
  });

  it('throws ActionDeniedError while the action awaits an administrator', async () => {
    const { client } = await agentClient();
    mockFetch.mockResolvedValueOnce(
      answer(201, { id: 'evt-4', status: 'pending', trustScore: 0.5, enforcementMode: 'strict', riskLevel: 'high' })
    );

    await expect(
      client.verifyAction({ action: 'db:drop', context: { jit_access: true } })
    ).rejects.toThrow(/awaiting an administrator's approval/);
  });

  it('keeps a 403 with no decision an AuthorizationError', async () => {
    const { client } = await agentClient();
    mockFetch.mockResolvedValueOnce(answer(403, { error: 'Agent status is suspended, cannot perform actions' }));

    const error = await client.verifyAction({ action: 'db:read' }).catch((e: unknown) => e);

    expect(error).toBeInstanceOf(AuthorizationError);
    expect(error).not.toBeInstanceOf(ActionDeniedError);
    expect((error as Error).message).toBe('Agent status is suspended, cannot perform actions');
  });

  it('never reads an answer with no known decision as an allow', async () => {
    const { client } = await agentClient();
    mockFetch.mockResolvedValueOnce(answer(200, { verified: true, actionAllowed: true }));

    await expect(client.verifyAction({ action: 'db:read' })).rejects.toThrow(VerificationError);
  });
});

describe('reportCapabilities', () => {
  it('registers each distinct capability once, signed as the agent, and reports each outcome', async () => {
    const fx = readFixture('register_capability');
    const { client, publicKey } = await agentClient();
    mockFetch
      .mockResolvedValueOnce(answer(201, { success: true, capabilityType: 'file:read', status: 'granted', message: 'Capability auto-granted (monitoring mode)' }))
      .mockResolvedValueOnce(answer(409, { success: true, capabilityType: 'db:write', status: 'already_exists', message: 'Capability already granted to this agent' }))
      .mockResolvedValueOnce(answer(202, { success: true, capabilityType: 'net:connect', status: 'pending', message: 'awaiting admin approval', requestId: 'req-1' }));

    const report = await client.reportCapabilities(['file:read', 'db:write', 'file:read', 'net:connect']);

    expect(mockFetch).toHaveBeenCalledTimes(3);
    const first = sent(0);
    expect(first.method).toBe(fx.method);
    expect(first.url).toBe(`${BASE_URL}${fx.path.replace('{id}', AGENT_ID)}`);
    expect(JSON.parse(first.body)).toEqual(fx.body);
    for (let i = 0; i < 3; i++) {
      await expectSignedAsAgent(sent(i), publicKey);
    }
    expect(JSON.parse(sent(2).body)).toEqual({ capabilityType: 'net:connect' });
    expect(report).toEqual({
      granted: 2,
      pending: 1,
      total: 3,
      results: [
        { capabilityType: 'file:read', status: 'granted', message: 'Capability auto-granted (monitoring mode)' },
        { capabilityType: 'db:write', status: 'already_exists', message: 'Capability already granted to this agent' },
        { capabilityType: 'net:connect', status: 'pending', message: 'awaiting admin approval', requestId: 'req-1' },
      ],
    });
  });

  it('refuses an entry that is not a non-empty string before sending anything', async () => {
    const { client } = await agentClient();

    await expect(client.reportCapabilities(['file:read', ''])).rejects.toThrow(ConfigurationError);
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it('stops at an answer with no known outcome, and sends nothing after it', async () => {
    const { client } = await agentClient();
    mockFetch.mockResolvedValueOnce(answer(201, { success: true }));

    await expect(client.reportCapabilities(['file:read', 'db:write'])).rejects.toThrow(VerificationError);
    expect(mockFetch).toHaveBeenCalledTimes(1);
  });

  it('stops at a refusal, and sends nothing after it', async () => {
    const { client } = await agentClient();
    mockFetch.mockResolvedValueOnce(answer(400, { error: 'invalid capability type' }));

    await expect(client.reportCapabilities(['file:read', 'db:write'])).rejects.toThrow('invalid capability type');
    expect(mockFetch).toHaveBeenCalledTimes(1);
  });
});

describe('attestIsolation', () => {
  it('signs the attestation as the agent, with no token exchange', async () => {
    const { client, publicKey } = await agentClient();
    mockFetch.mockResolvedValueOnce(
      answer(201, { id: 'att-1', agentId: AGENT_ID, score: 0.6, sandbox: 'gvisor', network: 'firewall', filesystem: 'none', process: 'none', reportedAt: '2026-10-11T12:00:00Z' })
    );

    const result = await client.attestIsolation({ sandbox: SandboxType.GVisor, network: NetworkIsolation.Firewall });

    expect(result.score).toBe(0.6);
    expect(mockFetch).toHaveBeenCalledTimes(1);
    const request = sent(0);
    expect(request.url).toBe(`${BASE_URL}/api/v1/sdk-api/agents/${AGENT_ID}/isolation-attestation`);
    expect(JSON.parse(request.body)).toEqual({ sandbox: 'gvisor', network: 'firewall', filesystem: 'none', process: 'none' });
    await expectSignedAsAgent(request, publicKey);
  });
});

describe('updateAgent', () => {
  it('sends the fixture body: the fields the server applies, never name', async () => {
    const fx = readFixture('update_agent');
    const { client } = await agentClient();
    mockFetch.mockImplementation(async (url: string) =>
      String(url) === `${BASE_URL}/api/v1/oauth/token`
        ? answer(200, { access_token: 'token', token_type: 'Bearer', expires_in: 7200 })
        : answer(200, { id: AGENT_ID, name: 'report-writer', displayName: 'Report Writer' })
    );

    await client.updateAgent({
      name: 'renamed',
      communityIntelligenceOptIn: true,
      ...(fx.body as Record<string, never>),
    });

    const put = mockFetch.mock.calls.findIndex(([, init]) => init?.method === 'PUT');
    expect(put).toBeGreaterThanOrEqual(0);
    const request = sent(put);
    expect(request.url).toBe(`${BASE_URL}${fx.path.replace('{id}', AGENT_ID)}`);
    expect(JSON.parse(request.body)).toEqual(fx.body);
  });
});

describe('the token exchange', () => {
  it('names the token endpoint the server serves when AIM cannot be reached', async () => {
    const { client } = await agentClient();
    mockFetch.mockRejectedValue(new TypeError('fetch failed'));

    const error = await client.updateAgent({ displayName: 'x' }).catch((e: unknown) => e);

    expect(error).toBeInstanceOf(NetworkError);
    expect((error as Error).message).toContain(`(POST ${BASE_URL}/api/v1/oauth/token)`);
  });
});
