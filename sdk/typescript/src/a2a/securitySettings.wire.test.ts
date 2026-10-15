/**
 * A2AClient.updateSecuritySettings sends the members the server reads (#652).
 *
 * PUT /api/v1/a2a/security/settings binds the backend's A2ASecuritySettings. The
 * settings type named request signing `requireSignedRequests` and the nonce window
 * `maxTimestampAge`, which the server does not read. The request is pinned in
 * tests/fixtures/sdk_requests/a2a_update_security_settings.json, which the backend's
 * handler tests post to that route and the Java SDK's tests hold its client to. Here
 * the A2ASecuritySettings interface is read from types.ts and held to the members the
 * backend's A2ASecuritySettings declares, read from the backend source.
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { readFileSync } from 'fs';
import { join } from 'path';
import { A2AClient } from './A2AClient';
import type { AIMClient } from '../client/AIMClient';
import type { A2ASecuritySettings } from './types';

const SERVER_TYPE = join(__dirname, '..', '..', '..', '..', 'apps', 'backend', 'internal', 'domain', 'a2a.go');
const SDK_TYPE = join(__dirname, 'types.ts');
const FIXTURE = join(__dirname, '..', '..', 'tests', 'fixtures', 'sdk_requests', 'a2a_update_security_settings.json');

/** The members that describe the stored record. */
const RECORD_MEMBERS = ['id', 'organizationId', 'createdBy', 'updatedBy', 'createdAt', 'updatedAt'];

interface RequestFixture {
  method: string;
  path: string;
  body: Record<string, unknown>;
}

const readFixture = (): RequestFixture => JSON.parse(readFileSync(FIXTURE, 'utf8')) as RequestFixture;

const declaredBlock = (file: string, pattern: RegExp, what: string): string => {
  const match = pattern.exec(readFileSync(file, 'utf8'));
  if (!match) {
    throw new Error(`${what} is not declared in ${file}`);
  }
  return match[1];
};

/** The JSON members of the backend's A2ASecuritySettings. */
const serverMembers = (): string[] => {
  const block = declaredBlock(SERVER_TYPE, /type A2ASecuritySettings struct \{([\s\S]*?)\n\}/, 'A2ASecuritySettings');
  return [...block.matchAll(/`json:"([^",`]+)/g)].map((m) => m[1]).sort();
};

/** The members of the SDK's A2ASecuritySettings interface. */
const sdkMembers = (): string[] => {
  const block = declaredBlock(SDK_TYPE, /export interface A2ASecuritySettings \{([\s\S]*?)\n\}/, 'A2ASecuritySettings');
  return [...block.matchAll(/^\s*(\w+)\??:/gm)].map((m) => m[1]).sort();
};

describe('A2A security settings are sent with the members the server reads', () => {
  const mockFetch = vi.fn();

  beforeEach(() => {
    mockFetch.mockReset();
    mockFetch.mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({}),
    } as Response);
    vi.stubGlobal('fetch', mockFetch);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('A2ASecuritySettings declares the members the backend declares', () => {
    const server = serverMembers();
    expect(server).toEqual(expect.arrayContaining(RECORD_MEMBERS));
    expect(sdkMembers()).toEqual(server);
  });

  it('the pinned body carries every settings member and no record member', () => {
    const settingsMembers = serverMembers().filter((m) => !RECORD_MEMBERS.includes(m));
    expect(Object.keys(readFixture().body).sort()).toEqual(settingsMembers);
  });

  it('updateSecuritySettings sends the settings as the pinned request', async () => {
    const fixture = readFixture();
    const aimClient = {
      config: { baseUrl: 'http://test.example.com' },
      getCredentials: vi.fn().mockReturnValue({ agentId: 'test-agent-id' }),
    } as unknown as AIMClient;

    await new A2AClient(aimClient).updateSecuritySettings(fixture.body as Partial<A2ASecuritySettings>);

    expect(mockFetch).toHaveBeenCalledTimes(1);
    const [url, init] = mockFetch.mock.calls[0] as [string, RequestInit];
    expect(init.method).toBe(fixture.method);
    expect(new URL(url).pathname).toBe(fixture.path);
    expect(JSON.parse(String(init.body))).toEqual(fixture.body);
  });
});
