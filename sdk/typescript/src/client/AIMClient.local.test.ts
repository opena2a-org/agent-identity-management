/**
 * AIMClient local-verification path: offline ATX verification + local broker
 * policy, with the remote POST retained as fallback.
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import * as crypto from 'node:crypto';
import { canonicalPayloadV11 } from '@opena2a/atx-verify';
import type { Atx } from '@opena2a/atx-verify';
import { AIMClient } from './AIMClient';
import { ActionDeniedError, AuthenticationError, ConfigurationError } from '../exceptions';
import type { LocalVerificationConfig } from '../local';

const ISSUER_DID = 'did:opena2a:issuer-1';
const FIXED_NOW = () => new Date('2026-06-15T00:00:00Z');

function genKey(): { privateKey: crypto.KeyObject; rawHex: string } {
  const { publicKey, privateKey } = crypto.generateKeyPairSync('ed25519');
  const spki = publicKey.export({ type: 'spki', format: 'der' }) as Buffer;
  return { privateKey, rawHex: spki.subarray(spki.length - 32).toString('hex') };
}

function signedAtx(privateKey: crypto.KeyObject, capabilities: string[]): Atx {
  const atx: Atx = {
    atcVersion: '1.1',
    agentId: 'agent-123',
    agentDid: 'did:opena2a:agent-123',
    version: '1.0.0',
    contentHash: 'sha256:abc123',
    issuerDid: ISSUER_DID,
    trustLevel: 3,
    trustScore: 87.5,
    issuedAt: '2026-06-01T00:00:00Z',
    expiresAt: '2026-06-30T00:00:00Z',
    capabilities,
    signatures: [],
  };
  const value = crypto.sign(null, canonicalPayloadV11(atx), privateKey).toString('base64');
  return { ...atx, signatures: [{ algorithm: 'Ed25519', keyId: 'k1', value }] };
}

function localConfig(rawHex: string): LocalVerificationConfig {
  return {
    trustedIssuers: [ISSUER_DID],
    publicKeys: [{ algorithm: 'Ed25519', publicKeyHex: rawHex }],
    now: FIXED_NOW,
  };
}

const mockFetch = vi.fn();

describe('AIMClient local verification', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.stubGlobal('fetch', mockFetch);
    delete process.env.AIM_BASE_URL;
    delete process.env.AIM_API_KEY;
  });
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('verifyActionLocally allows a signed capability without any network call', async () => {
    const { privateKey, rawHex } = genKey();
    const client = new AIMClient({ localVerification: localConfig(rawHex) });
    client.setLocalCredential(signedAtx(privateKey, ['file:read']));

    const result = await client.verifyActionLocally({ action: 'file:read' });

    expect(result.actionAllowed).toBe(true);
    expect(result.verified).toBe(true);
    expect(result.agentId).toBe('agent-123');
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it('verifyAction takes the local path when a credential is cached (no network)', async () => {
    const { privateKey, rawHex } = genKey();
    const client = new AIMClient({ localVerification: localConfig(rawHex) });
    client.setLocalCredential(signedAtx(privateKey, ['file:read']));

    const result = await client.verifyAction({ action: 'file:read' });

    expect(result.actionAllowed).toBe(true);
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it('verifyActionLocally throws ActionDeniedError on a denied action', async () => {
    const { privateKey, rawHex } = genKey();
    const client = new AIMClient({ localVerification: localConfig(rawHex) });
    client.setLocalCredential(signedAtx(privateKey, ['file:read']));

    await expect(client.verifyActionLocally({ action: 'file:delete' })).rejects.toBeInstanceOf(
      ActionDeniedError,
    );
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it('verifyActionLocally throws ConfigurationError when local verification is not configured', async () => {
    const { privateKey } = genKey();
    const client = new AIMClient();
    await expect(
      client.verifyActionLocally({ action: 'file:read' }, signedAtx(privateKey, ['file:read'])),
    ).rejects.toBeInstanceOf(ConfigurationError);
  });

  it('verifyActionLocally requires a credential', async () => {
    const { rawHex } = genKey();
    const client = new AIMClient({ localVerification: localConfig(rawHex) });
    await expect(client.verifyActionLocally({ action: 'file:read' })).rejects.toThrow(
      /no local atx credential/i,
    );
  });

  it('accepts a per-call ATX argument that overrides the cached credential', async () => {
    const { privateKey, rawHex } = genKey();
    const client = new AIMClient({ localVerification: localConfig(rawHex) });
    // no cached credential set; pass per-call
    const result = await client.verifyAction(
      { action: 'net:fetch' },
      signedAtx(privateKey, ['net:fetch']),
    );
    expect(result.actionAllowed).toBe(true);
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it('falls back to the remote path when no local credential is available', async () => {
    const { rawHex } = genKey();
    // local verification configured, but no cached credential and no agent
    // credentials -> verifyAction must fall through to the remote path, which
    // requires registered credentials (proving it did NOT take the local path).
    const client = new AIMClient({ localVerification: localConfig(rawHex) });
    await expect(client.verifyAction({ action: 'file:read' })).rejects.toBeInstanceOf(
      AuthenticationError,
    );
  });

  it('clearing the cached credential reverts verifyAction to the remote path', async () => {
    const { privateKey, rawHex } = genKey();
    const client = new AIMClient({ localVerification: localConfig(rawHex) });
    client.setLocalCredential(signedAtx(privateKey, ['file:read']));
    client.setLocalCredential(null);
    await expect(client.verifyAction({ action: 'file:read' })).rejects.toBeInstanceOf(
      AuthenticationError,
    );
  });

  it('getLocalVerifier exposes the verifier when configured, null otherwise', () => {
    const { rawHex } = genKey();
    expect(new AIMClient({ localVerification: localConfig(rawHex) }).getLocalVerifier()).not.toBeNull();
    expect(new AIMClient().getLocalVerifier()).toBeNull();
  });
});
