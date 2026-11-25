/**
 * A2AClient.evaluatePolicy sends the request the server reads (#652).
 *
 * POST /api/v1/a2a/policies/evaluate binds EvaluateA2APolicyRequest: event,
 * clientAgentId, remoteAgentId and skillId (and an organizationId that may only
 * name the caller's own organization). The client sent targetAgentId, action,
 * resource and context, none of which the route reads, and read an `allowed`
 * member the route does not answer. The request is pinned in
 * tests/fixtures/sdk_requests/a2a_evaluate_policy.json, which the backend's
 * handler test POSTs to that route and the Java SDK's tests hold its client to.
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { readFileSync } from 'fs';
import { join } from 'path';
import { A2AClient } from './A2AClient';
import type { AIMClient } from '../client/AIMClient';

const SERVICE = join(
  __dirname, '..', '..', '..', '..', 'apps', 'backend', 'internal', 'application', 'a2a_service.go'
);
const FIXTURE = join(__dirname, '..', '..', 'tests', 'fixtures', 'sdk_requests', 'a2a_evaluate_policy.json');

interface RequestFixture {
  method: string;
  path: string;
  body: { event: string; clientAgentId: string; remoteAgentId: string; skillId: string };
}

const readFixture = (): RequestFixture => JSON.parse(readFileSync(FIXTURE, 'utf8')) as RequestFixture;

const DECISION = {
  decision: 'DENY',
  policiesEvaluated: ['skill-policy'],
  reason: 'Trust score below threshold (client: 0.00, remote: 0.00, required: 0.30)',
  evaluatedAt: '2026-11-24T12:00:00Z',
};

describe('evaluatePolicy sends the request the server reads', () => {
  const mockFetch = vi.fn();

  const clientFor = (agentId: string, override?: string) =>
    new A2AClient(
      {
        config: { baseUrl: 'http://test.example.com' },
        getCredentials: vi.fn().mockReturnValue({ agentId }),
        getAccessToken: vi.fn().mockResolvedValue('test-access-token'),
      } as unknown as AIMClient,
      override
    );

  beforeEach(() => {
    mockFetch.mockReset();
    vi.stubGlobal('fetch', mockFetch);
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

  it('the server binds event, clientAgentId, remoteAgentId and skillId', () => {
    const declared = /type EvaluateA2APolicyRequest struct \{([\s\S]*?)\n\}/.exec(readFileSync(SERVICE, 'utf8'));
    expect(declared, 'EvaluateA2APolicyRequest is declared in a2a_service.go').not.toBeNull();
    const members = [...(declared?.[1] ?? '').matchAll(/`json:"(\w+)[,"]/g)].map((m) => m[1]);
    expect(members.sort()).toEqual(['clientAgentId', 'event', 'organizationId', 'remoteAgentId', 'skillId']);
    expect(Object.keys(readFixture().body).every((member) => members.includes(member))).toBe(true);
  });

  it('an event that invokes a skill is sent as the pinned request, and the decision is read', async () => {
    const fixture = readFixture();
    respond(DECISION);

    const decision = await clientFor(fixture.body.clientAgentId).evaluatePolicy(
      fixture.body.event,
      fixture.body.remoteAgentId,
      fixture.body.skillId
    );

    const request = sent();
    expect(request.method).toBe(fixture.method);
    expect(request.path).toBe(fixture.path);
    expect(request.body).toEqual(fixture.body);
    expect(decision.decision).toBe('DENY');
    expect(decision.policiesEvaluated).toEqual(['skill-policy']);
  });

  it('an event that invokes no skill sends no skillId', async () => {
    const fixture = readFixture();
    respond({ decision: 'ALLOW', policiesEvaluated: [], evaluatedAt: DECISION.evaluatedAt });

    await clientFor(fixture.body.clientAgentId).evaluatePolicy('task.received', fixture.body.remoteAgentId);

    expect(sent().body).toEqual({
      event: 'task.received',
      clientAgentId: fixture.body.clientAgentId,
      remoteAgentId: fixture.body.remoteAgentId,
    });
  });

  it('the client agent is the one the client was made for', async () => {
    const fixture = readFixture();
    const other = '3f2504e0-4f89-41d3-9a0c-0305e82c3301';
    respond(DECISION);

    await clientFor(fixture.body.clientAgentId, other).evaluatePolicy('task.create', fixture.body.remoteAgentId);

    expect(sent().body.clientAgentId).toBe(other);
  });
});
