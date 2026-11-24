/**
 * A2AClient.updateAgentCard sends the card document the server reads (#652).
 *
 * PUT /api/v1/a2a/cards/:id reads cardUrl or cardData and replaces the agent's
 * card with that document; it answers the identifiers of the card's skills as
 * strings (RegisterAgentCardResponse.Skills []string). The client sent the card
 * fields themselves, skills as objects, which the route does not read: it
 * answered 400. The request is pinned in
 * tests/fixtures/sdk_requests/a2a_update_agent_card.json, which the backend's
 * handler test PUTs to that route and the Java SDK's tests hold its client to.
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { readFileSync } from 'fs';
import { join } from 'path';
import { A2AClient } from './A2AClient';
import type { AIMClient } from '../client/AIMClient';
import type { A2AAgentCardUpdate, A2ACardDocument } from './types';

const BACKEND = join(__dirname, '..', '..', '..', '..', 'apps', 'backend');
const HANDLER = join(BACKEND, 'internal', 'interfaces', 'http', 'handlers', 'a2a_handler.go');
const SERVICE = join(BACKEND, 'internal', 'application', 'a2a_service.go');
const FIXTURE = join(__dirname, '..', '..', 'tests', 'fixtures', 'sdk_requests', 'a2a_update_agent_card.json');
const AGENT_ID = '550e8400-e29b-41d4-a716-446655440000';

interface RequestFixture {
  method: string;
  path: string;
  body: { cardData: A2ACardDocument };
}

const readFixture = (): RequestFixture => JSON.parse(readFileSync(FIXTURE, 'utf8')) as RequestFixture;

const declared = (file: string, pattern: RegExp, what: string): string => {
  const match = pattern.exec(readFileSync(file, 'utf8'));
  if (!match) {
    throw new Error(`${what} is not declared in ${file}`);
  }
  return match[1];
};

/** The answer the server gives, its skills the identifiers of the document's skills. */
const registration = (document: A2ACardDocument) => ({
  cardId: '6ba7b810-9dad-11d1-80b4-00c04fd430c8',
  agentId: AGENT_ID,
  cardUrl: `inline://${AGENT_ID}`,
  attestationExpires: '2026-11-25T12:00:00Z',
  skills: (document.skills ?? []).map((skill) => skill.id),
});

describe('updateAgentCard sends the card document the server reads', () => {
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

  it('the route binds cardUrl and cardData and answers skills as strings', () => {
    const handler = declared(HANDLER, /func \(h \*A2AHandler\) UpdateAgentCard\(c fiber\.Ctx\) error \{([\s\S]*?)\n\}\n/, 'UpdateAgentCard');
    expect(handler).toMatch(/CardURL\s+string\s+`json:"cardUrl"`/);
    expect(handler).toMatch(/CardData\s+json\.RawMessage\s+`json:"cardData"`/);
    const response = declared(SERVICE, /type RegisterAgentCardResponse struct \{([\s\S]*?)\n\}/, 'RegisterAgentCardResponse');
    expect(response).toMatch(/Skills\s+\[\]string\s+`json:"skills"`/);
  });

  it('a card document with a skill is sent as the pinned request, and its skills are read as strings', async () => {
    const fixture = readFixture();
    respond(registration(fixture.body.cardData));

    const result = await client.updateAgentCard({ cardData: fixture.body.cardData });

    const request = sent();
    expect(request.method).toBe(fixture.method);
    expect(request.path).toBe(fixture.path.replace('{agentId}', AGENT_ID));
    expect(request.body).toEqual(fixture.body);
    expect(result.skills).toEqual(['summarize']);
    expect(result.cardId).toBe('6ba7b810-9dad-11d1-80b4-00c04fd430c8');
  });

  it('a card URL is sent as cardUrl, to the named agent', async () => {
    respond(registration({ name: 'x', url: 'https://agent.example.com', version: '1' }));

    await client.updateAgentCard({
      agentId: 'other-agent',
      cardUrl: 'https://agent.example.com/.well-known/agent.json',
    });

    const request = sent();
    expect(request.path).toBe('/api/v1/a2a/cards/other-agent');
    expect(request.body).toEqual({ cardUrl: 'https://agent.example.com/.well-known/agent.json' });
  });

  it('card fields with no document are refused with the server answer, naming them, and nothing is sent', async () => {
    const refusal = declared(HANDLER, /const cardDocumentRequiredMessage = "([^"]+)"/, 'cardDocumentRequiredMessage');
    expect(A2AClient.CARD_DOCUMENT_REQUIRED).toBe(refusal);

    const fields = { skills: [{ id: 'summarize', name: 'Summarize' }], version: '2' } as unknown as A2AAgentCardUpdate;
    await expect(client.updateAgentCard(fields)).rejects.toThrow(`${refusal}; it does not read version, skills`);
    await expect(client.updateAgentCard({})).rejects.toThrow(refusal);
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it('a refusal by the server rejects the call', async () => {
    mockFetch.mockResolvedValueOnce({ ok: false, status: 400, statusText: 'Bad Request' } as Response);

    await expect(client.updateAgentCard({ cardData: readFixture().body.cardData })).rejects.toThrow('400');
  });
});
