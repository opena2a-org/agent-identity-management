/**
 * A request made with agent credentials is signed with the key's 32-byte seed.
 *
 * AIM issues the private key as 64 bytes, the seed then its public key, and
 * `aim-sdk init` writes that form. The client decoded the configured key and
 * passed all 64 bytes to the Ed25519 signer, which takes the seed alone, so
 * `verifyAction` obtained a token and then threw `Uint8Array expected` before
 * the verify request was sent. These tests send a request with each key form
 * and check the `X-AIM-Signature` header against the agent's public key.
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

function jsonResponse(body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });
}

/** Sends `verifyAction` with the given key form and returns the verify request it sent. */
async function sendVerify(privateKeyForm: 'seed' | 'issued'): Promise<{
  headers: Record<string, string>;
  body: string;
  publicKey: Uint8Array;
}> {
  const keyPair = await generateKeyPair();
  mockFetch.mockImplementation(async (url: string) => {
    if (String(url) === `${BASE_URL}/api/v1/oauth/token`) {
      return jsonResponse({ access_token: 'token', token_type: 'Bearer', expires_in: 7200 });
    }
    return jsonResponse({
      verified: true,
      agentId: AGENT_ID,
      agentName: 'agent',
      trustScore: 80,
      riskLevel: 'low',
      actionAllowed: true,
      timestamp: new Date().toISOString(),
    });
  });

  const client = new AIMClient({ baseUrl: BASE_URL });
  client.setCredentials({
    agentId: AGENT_ID,
    privateKey: toBase64(privateKeyForm === 'issued' ? issuedForm(keyPair) : keyPair.privateKey),
    publicKey: toBase64(keyPair.publicKey),
    organizationId: 'org-1',
    createdAt: new Date().toISOString(),
  });

  const result = await client.verifyAction({ action: 'db:read' });
  expect(result.actionAllowed).toBe(true);

  const verifyCall = mockFetch.mock.calls.find(
    ([url]) => String(url) !== `${BASE_URL}/api/v1/oauth/token`
  );
  expect(verifyCall).toBeDefined();
  const [, init] = verifyCall!;
  return {
    headers: init.headers as Record<string, string>,
    body: init.body as string,
    publicKey: keyPair.publicKey,
  };
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

  it('sends the verify request with a 64-byte issued key, signed with its seed', async () => {
    const { headers, body, publicKey } = await sendVerify('issued');

    expect(headers['Authorization']).toBe('Bearer token');
    expect(headers['X-AIM-Agent-ID']).toBe(AGENT_ID);
    const message = signedMessage(headers['X-AIM-Timestamp'], 'POST', '/api/v1/verify', body);
    const signature = fromBase64(headers['X-AIM-Signature']);
    expect(await verify(signature, message, publicKey)).toBe(true);

    const otherKey = await generateKeyPair();
    expect(await verify(signature, message, otherKey.publicKey)).toBe(false);
  });

  it('signs the verify request the same way with a 32-byte seed', async () => {
    const { headers, body, publicKey } = await sendVerify('seed');

    const message = signedMessage(headers['X-AIM-Timestamp'], 'POST', '/api/v1/verify', body);
    expect(await verify(fromBase64(headers['X-AIM-Signature']), message, publicKey)).toBe(true);
  });
});
