/**
 * A request made with agent credentials is signed with the key's 32-byte seed.
 *
 * AIM issues the private key as 64 bytes, the seed then its public key, and
 * `aim-sdk init` writes that form. The client decoded the configured key and
 * passed all 64 bytes to the Ed25519 signer, which takes the seed alone, so
 * `verifyAction` obtained a token and then threw `Uint8Array expected` before
 * the verify request was sent. These tests send a request with each key form
 * and check its signature against the agent's public key: the signature in the
 * body of the verification POST, and the `X-AIM-Signature` header of a request
 * made with the OAuth token.
 */

import { createHash } from 'node:crypto';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { AIMClient } from './AIMClient';
import { fromBase64, generateKeyPair, toBase64, verify } from '../crypto/ed25519';

const BASE_URL = 'https://aim.example.com';
const AGENT_ID = '550e8400-e29b-41d4-a716-446655440000';

const mockFetch = vi.fn();

/** The 64-byte form AIM issues and `aim-sdk init` writes: the seed, then its public key. */
function issuedForm(keyPair: { privateKey: Uint8Array; publicKey: Uint8Array }): Uint8Array {
  const bytes = new Uint8Array(64);
  bytes.set(keyPair.privateKey, 0);
  bytes.set(keyPair.publicKey, 32);
  return bytes;
}

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

async function credentialedClient(privateKeyForm: 'seed' | 'issued') {
  const keyPair = await generateKeyPair();
  const client = new AIMClient({ baseUrl: BASE_URL });
  client.setCredentials({
    agentId: AGENT_ID,
    privateKey: toBase64(privateKeyForm === 'issued' ? issuedForm(keyPair) : keyPair.privateKey),
    publicKey: toBase64(keyPair.publicKey),
    organizationId: 'org-1',
    createdAt: new Date().toISOString(),
  });
  return { client, publicKey: keyPair.publicKey };
}

/** Sends `verifyAction` with the given key form and returns the verification request it sent. */
async function sendVerify(privateKeyForm: 'seed' | 'issued'): Promise<{
  headers: Record<string, string>;
  body: Record<string, unknown>;
  publicKey: Uint8Array;
}> {
  mockFetch.mockImplementation(async () =>
    jsonResponse(
      { id: 'evt-1', status: 'approved', trustScore: 0.8, enforcementMode: 'strict', riskLevel: 'low' },
      201
    )
  );
  const { client, publicKey } = await credentialedClient(privateKeyForm);

  const result = await client.verifyAction({ action: 'db:read' });
  expect(result.actionAllowed).toBe(true);

  // One request, to the verification route: no token exchange precedes it.
  expect(mockFetch).toHaveBeenCalledTimes(1);
  const [url, init] = mockFetch.mock.calls[0];
  expect(String(url)).toBe(`${BASE_URL}/api/v1/sdk-api/verifications`);
  return {
    headers: init.headers as Record<string, string>,
    body: JSON.parse(init.body as string) as Record<string, unknown>,
    publicKey,
  };
}

/**
 * The message the server verifies a verification signature over, written out
 * for a request with no resource and an empty context.
 */
function verificationMessage(body: Record<string, unknown>): Uint8Array {
  return new TextEncoder().encode(
    `{"action_type": "${body.capability}", "agent_id": "${body.agentId}", "context": {}, ` +
      `"resource": null, "timestamp": "${body.timestamp}"}`
  );
}

/** The bytes `createRequestSignature` signs: timestamp:METHOD:path:hex(sha256(body)). */
function signedMessage(timestamp: string, method: string, path: string, body: string): Uint8Array {
  const bodyHash = createHash('sha256').update(body).digest('hex');
  return new TextEncoder().encode(`${timestamp}:${method}:${path}:${bodyHash}`);
}

describe('the request signature AIMClient sends with agent credentials', () => {
  beforeEach(() => {
    mockFetch.mockReset();
    vi.stubGlobal('fetch', mockFetch);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('signs the verification with a 64-byte issued key, using its seed', async () => {
    const { headers, body, publicKey } = await sendVerify('issued');

    expect(headers['Authorization']).toBeUndefined();
    expect(body.agentId).toBe(AGENT_ID);
    expect(body.publicKey).toBe(toBase64(publicKey));
    const signature = fromBase64(body.signature as string);
    expect(await verify(signature, verificationMessage(body), publicKey)).toBe(true);

    const otherKey = await generateKeyPair();
    expect(await verify(signature, verificationMessage(body), otherKey.publicKey)).toBe(false);
  });

  it('signs the verification the same way with a 32-byte seed', async () => {
    const { body, publicKey } = await sendVerify('seed');

    expect(body.publicKey).toBe(toBase64(publicKey));
    const signature = fromBase64(body.signature as string);
    expect(await verify(signature, verificationMessage(body), publicKey)).toBe(true);
  });

  it('signs a request made with the OAuth token with the seed of a 64-byte issued key', async () => {
    mockFetch.mockImplementation(async (url: string) => {
      if (String(url) === `${BASE_URL}/api/v1/oauth/token`) {
        return jsonResponse({ access_token: 'token', token_type: 'Bearer', expires_in: 7200 });
      }
      return jsonResponse({ id: AGENT_ID, name: 'agent', displayName: 'Agent' });
    });
    const { client, publicKey } = await credentialedClient('issued');

    await client.updateAgent({ displayName: 'Agent' });

    const call = mockFetch.mock.calls.find(([url]) => String(url) === `${BASE_URL}/api/v1/agents/${AGENT_ID}`);
    expect(call).toBeDefined();
    const [, init] = call!;
    const headers = init.headers as Record<string, string>;
    expect(headers['Authorization']).toBe('Bearer token');
    expect(headers['X-AIM-Agent-ID']).toBe(AGENT_ID);
    const message = signedMessage(headers['X-AIM-Timestamp'], 'PUT', `/api/v1/agents/${AGENT_ID}`, init.body as string);
    expect(await verify(fromBase64(headers['X-AIM-Signature']), message, publicKey)).toBe(true);
  });
});
