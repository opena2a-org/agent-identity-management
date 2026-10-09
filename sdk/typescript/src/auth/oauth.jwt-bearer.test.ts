/**
 * The token request must be one the AIM token endpoint accepts.
 *
 * The server (apps/backend/.../oauth_token_handler.go) is mounted at
 * POST /api/v1/oauth/token, accepts only the RFC 7523 JWT-bearer grant, decodes
 * each segment of the client assertion as unpadded base64url, and verifies an
 * Ed25519 signature over the bytes of `header.payload` against the agent's
 * registered public key. The SDK posted to /oauth/token (404), sent
 * `client_credentials` (400 unsupported_grant_type), encoded the segments as
 * padded standard base64, and signed a request-signature string instead of
 * `header.payload`. These tests apply the server's own checks to the request
 * the SDK sends.
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import type { AgentCredentials } from '../types';
import { generateKeyPair, toBase64, verify } from '../crypto/ed25519';
import { OAuthTokenManager } from './oauth';

const BASE_URL = 'https://aim.example.com';
const AGENT_ID = '550e8400-e29b-41d4-a716-446655440000';

const mockFetch = vi.fn();

function fromBase64Url(segment: string): Uint8Array {
  return new Uint8Array(Buffer.from(segment, 'base64url'));
}

/** The 64-byte form AIM issues and `aim-sdk init` writes: the seed, then its public key. */
function issuedForm(keyPair: { privateKey: Uint8Array; publicKey: Uint8Array }): Uint8Array {
  const bytes = new Uint8Array(64);
  bytes.set(keyPair.privateKey, 0);
  bytes.set(keyPair.publicKey, 32);
  return bytes;
}

async function captureTokenRequest(
  privateKeyForm: 'seed' | 'issued' = 'seed'
): Promise<{
  url: string;
  body: URLSearchParams;
  publicKey: Uint8Array;
}> {
  const keyPair = await generateKeyPair();
  const credentials: AgentCredentials = {
    agentId: AGENT_ID,
    privateKey: toBase64(privateKeyForm === 'issued' ? issuedForm(keyPair) : keyPair.privateKey),
    publicKey: toBase64(keyPair.publicKey),
    organizationId: 'org-1',
    createdAt: new Date().toISOString(),
  };
  mockFetch.mockResolvedValueOnce({
    ok: true,
    json: async () => ({ access_token: 'token', token_type: 'Bearer', expires_in: 7200 }),
  });

  await new OAuthTokenManager(BASE_URL, credentials).getAccessToken();

  const [url, init] = mockFetch.mock.calls[0];
  return { url: String(url), body: init.body as URLSearchParams, publicKey: keyPair.publicKey };
}

describe('the token request the server accepts', () => {
  beforeEach(() => {
    mockFetch.mockReset();
    globalThis.fetch = mockFetch;
  });

  it('posts to the mounted token endpoint, /api/v1/oauth/token', async () => {
    const { url } = await captureTokenRequest();
    expect(url).toBe(`${BASE_URL}/api/v1/oauth/token`);
  });

  it('requests the JWT-bearer grant with the agent as client_id', async () => {
    const { body } = await captureTokenRequest();
    expect(body.get('grant_type')).toBe('urn:ietf:params:oauth:grant-type:jwt-bearer');
    expect(body.get('client_id')).toBe(AGENT_ID);
    expect(body.get('client_assertion_type')).toBe(
      'urn:ietf:params:oauth:client-assertion-type:jwt-bearer'
    );
  });

  it('encodes the assertion as three unpadded base64url segments', async () => {
    const { body } = await captureTokenRequest();
    const segments = (body.get('client_assertion') ?? '').split('.');
    expect(segments).toHaveLength(3);
    for (const segment of segments) {
      expect(segment).toMatch(/^[A-Za-z0-9_-]+$/);
    }
    expect(JSON.parse(Buffer.from(fromBase64Url(segments[0])).toString('utf8'))).toEqual({
      alg: 'EdDSA',
      typ: 'JWT',
    });
  });

  it('signs header.payload with the agent key, so the registered public key verifies it', async () => {
    const { body, publicKey } = await captureTokenRequest();
    const [header, payload, signature] = (body.get('client_assertion') ?? '').split('.');

    const signingInput = new TextEncoder().encode(`${header}.${payload}`);
    expect(await verify(fromBase64Url(signature), signingInput, publicKey)).toBe(true);

    const otherKey = await generateKeyPair();
    expect(await verify(fromBase64Url(signature), signingInput, otherKey.publicKey)).toBe(false);
  });

  it('signs with the seed of a 64-byte issued credential, the form the server and aim-sdk init write', async () => {
    const { url, body, publicKey } = await captureTokenRequest('issued');
    expect(url).toBe(`${BASE_URL}/api/v1/oauth/token`);
    const [header, payload, signature] = (body.get('client_assertion') ?? '').split('.');

    const signingInput = new TextEncoder().encode(`${header}.${payload}`);
    expect(await verify(fromBase64Url(signature), signingInput, publicKey)).toBe(true);
  });

  it('carries the claims the server enforces: sub, aud and a short exp', async () => {
    const before = Math.floor(Date.now() / 1000);
    const { body } = await captureTokenRequest();
    const payload = (body.get('client_assertion') ?? '').split('.')[1];
    const claims = JSON.parse(Buffer.from(fromBase64Url(payload)).toString('utf8'));

    expect(claims.sub).toBe(AGENT_ID);
    expect(claims.iss).toBe(AGENT_ID);
    expect(claims.aud).toBe(BASE_URL);
    expect(typeof claims.exp).toBe('number');
    expect(claims.exp).toBeGreaterThan(before);
    // The server refuses an exp more than five minutes plus 60 seconds of skew ahead.
    expect(claims.exp - before).toBeLessThanOrEqual(5 * 60 + 60);
  });
});
