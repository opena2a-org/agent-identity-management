/**
 * A2AClient.registerSkill sends each example as a string (#652).
 *
 * POST /api/v1/a2a/skills binds `examples` as []string. The client sent the
 * caller's skill as it was, examples as objects ({ name, input, output }),
 * which the route cannot bind: it answered 400. The request is pinned in
 * tests/fixtures/sdk_requests/a2a_register_skill.json, which the backend's
 * handler test POSTs to that route.
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { readFileSync } from 'fs';
import { join } from 'path';
import { A2AClient } from './A2AClient';
import type { AIMClient } from '../client/AIMClient';
import type { A2ASkillRegistration } from './types';

const HANDLER = join(
  __dirname, '..', '..', '..', '..', 'apps', 'backend', 'internal', 'interfaces', 'http', 'handlers', 'a2a_handler.go'
);
const FIXTURE = join(__dirname, '..', '..', 'tests', 'fixtures', 'sdk_requests', 'a2a_register_skill.json');
const AGENT_ID = '550e8400-e29b-41d4-a716-446655440000';

interface RequestFixture {
  method: string;
  path: string;
  body: A2ASkillRegistration & { examples: string[] };
}

const readFixture = (): RequestFixture => JSON.parse(readFileSync(FIXTURE, 'utf8')) as RequestFixture;

/** The stored skill the server answers for a registration. */
const stored = (skill: A2ASkillRegistration) => ({
  id: '6ba7b810-9dad-11d1-80b4-00c04fd430c8',
  agentId: AGENT_ID,
  skillId: skill.id ?? '',
  name: skill.name,
  description: skill.description,
  tags: skill.tags,
  inputModes: skill.inputModes,
  outputModes: skill.outputModes,
  examples: skill.examples,
  invocationCount: 0,
  successCount: 0,
  failureCount: 0,
  isVerified: false,
  attestationCount: 0,
  createdAt: '2026-11-24T12:00:00Z',
  updatedAt: '2026-11-24T12:00:00Z',
});

describe('registerSkill sends examples as strings', () => {
  const mockFetch = vi.fn();
  let client: A2AClient;

  beforeEach(() => {
    mockFetch.mockReset();
    vi.stubGlobal('fetch', mockFetch);
    client = new A2AClient({
      config: { baseUrl: 'http://test.example.com' },
      getCredentials: vi.fn().mockReturnValue({ agentId: AGENT_ID }),
      getAccessToken: vi.fn().mockResolvedValue('test-access-token'),
    } as unknown as AIMClient);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  const respond = (body: unknown) =>
    mockFetch.mockResolvedValueOnce({ ok: true, json: () => Promise.resolve(body) } as Response);

  const sent = () => {
    expect(mockFetch).toHaveBeenCalledTimes(1);
    const [url, init] = mockFetch.mock.calls[0];
    return {
      method: init?.method as string,
      path: new URL(String(url)).pathname,
      body: JSON.parse(init?.body as string) as Record<string, unknown>,
    };
  };

  it('the route binds examples as strings', () => {
    const source = readFileSync(HANDLER, 'utf8');
    const handler = /func \(h \*A2AHandler\) RegisterSkill\(c fiber\.Ctx\) error \{([\s\S]*?)\n\}\n/.exec(source);
    expect(handler, 'RegisterSkill is declared in a2a_handler.go').not.toBeNull();
    expect(handler?.[1]).toMatch(/Examples\s+\[\]string\s+`json:"examples"`/);
  });

  it('a skill with one example is sent as the pinned request, and read back with its example', async () => {
    const fixture = readFixture();
    respond(stored(fixture.body));

    const skill = await client.registerSkill(fixture.body);

    const request = sent();
    expect(request.method).toBe(fixture.method);
    expect(request.path).toBe(fixture.path);
    expect(request.body).toEqual(fixture.body);
    expect(skill.skillId).toBe('summarize');
    expect(skill.examples).toEqual(['Summarize the nightly report']);
  });

  it('an example passed as an object is sent as its JSON text', async () => {
    respond(stored({ name: 'Add' }));
    const example = { name: 'add', input: { a: 2, b: 3 }, output: { result: 5 } };

    await client.registerSkill({ name: 'Add', examples: [example] } as unknown as A2ASkillRegistration);

    expect(sent().body.examples).toEqual([JSON.stringify(example)]);
  });

  it('sends only the members the route reads', async () => {
    respond(stored({ name: 'Add' }));

    await client.registerSkill({
      agentId: AGENT_ID,
      name: 'Add',
      inputSchema: { type: 'object' },
      isVerified: true,
      attestationCount: 3,
    } as unknown as A2ASkillRegistration);

    expect(sent().body).toEqual({ agentId: AGENT_ID, name: 'Add' });
  });
});
