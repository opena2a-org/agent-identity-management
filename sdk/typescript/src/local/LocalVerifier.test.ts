import { describe, it, expect } from 'vitest';
import * as crypto from 'node:crypto';
import { canonicalPayload, canonicalPayloadV11 } from '@opena2a/atx-verify';
import type { Atx } from '@opena2a/atx-verify';
import { LocalVerifier, CrlCache, type LocalVerificationConfig } from './LocalVerifier';

// --- test helpers: mint real Ed25519 keys and sign real ATX credentials so the
// tests exercise the actual shared verifier, not a mock. ---

const ISSUER_DID = 'did:opena2a:issuer-1';
const FIXED_NOW = () => new Date('2026-06-15T00:00:00Z');

function genKey(): { privateKey: crypto.KeyObject; rawHex: string } {
  const { publicKey, privateKey } = crypto.generateKeyPairSync('ed25519');
  const spki = publicKey.export({ type: 'spki', format: 'der' }) as Buffer;
  // raw Ed25519 public key = last 32 bytes of the SPKI DER.
  const rawHex = spki.subarray(spki.length - 32).toString('hex');
  return { privateKey, rawHex };
}

function baseAtx(overrides: Partial<Atx> = {}): Atx {
  return {
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
    capabilities: ['file:read', 'net:fetch'],
    signatures: [],
    ...overrides,
  };
}

function sign(atx: Atx, privateKey: crypto.KeyObject): Atx {
  const payload = atx.atcVersion === '1.1' ? canonicalPayloadV11(atx) : canonicalPayload(atx);
  const value = crypto.sign(null, payload, privateKey).toString('base64');
  return { ...atx, signatures: [{ algorithm: 'Ed25519', keyId: 'k1', value }] };
}

function anchors(rawHex: string, extra: Partial<LocalVerificationConfig> = {}): LocalVerificationConfig {
  return {
    trustedIssuers: [ISSUER_DID],
    publicKeys: [{ algorithm: 'Ed25519', publicKeyHex: rawHex }],
    now: FIXED_NOW,
    ...extra,
  };
}

describe('LocalVerifier.verifyCredential', () => {
  it('verifies a valid v1.1 credential offline', async () => {
    const { privateKey, rawHex } = genKey();
    const atx = sign(baseAtx(), privateKey);
    const verifier = new LocalVerifier(anchors(rawHex));

    const result = await verifier.verifyCredential(atx);

    expect(result.valid).toBe(true);
    expect(result.context?.agentId).toBe('agent-123');
    expect(result.context?.signedCapabilities).toBe(true);
    expect(result.context?.trustScore).toBeCloseTo(87.5);
  });

  it('ready() pre-loads so a subsequent verify still succeeds', async () => {
    const { privateKey, rawHex } = genKey();
    const atx = sign(baseAtx(), privateKey);
    const verifier = new LocalVerifier(anchors(rawHex));
    await verifier.ready();
    const result = await verifier.verifyCredential(atx);
    expect(result.valid).toBe(true);
  });

  it('rejects a tampered signature (fail closed)', async () => {
    const { privateKey, rawHex } = genKey();
    const signed = sign(baseAtx(), privateKey);
    // flip the last base64 char of the signature
    const v = signed.signatures[0].value;
    const tampered: Atx = {
      ...signed,
      signatures: [{ ...signed.signatures[0], value: v.slice(0, -2) + (v.endsWith('A') ? 'B' : 'A') + '=' }],
    };
    const verifier = new LocalVerifier(anchors(rawHex));

    const result = await verifier.verifyCredential(tampered);

    expect(result.valid).toBe(false);
    expect(result.rejectCategory).toBe('SIGNATURE_INVALID');
  });

  it('rejects an expired credential', async () => {
    const { privateKey, rawHex } = genKey();
    const atx = sign(baseAtx({ expiresAt: '2026-06-10T00:00:00Z' }), privateKey);
    const verifier = new LocalVerifier(anchors(rawHex));

    const result = await verifier.verifyCredential(atx);

    expect(result.valid).toBe(false);
    expect(result.rejectCategory).toBe('EXPIRED');
  });

  it('rejects a credential on the CRL', async () => {
    const { privateKey, rawHex } = genKey();
    const atx = sign(baseAtx(), privateKey);
    const verifier = new LocalVerifier(
      anchors(rawHex, { crl: { entries: [{ agentId: 'agent-123', reason: 'key compromise' }] } }),
    );

    const result = await verifier.verifyCredential(atx);

    expect(result.valid).toBe(false);
    expect(result.rejectCategory).toBe('REVOKED');
  });

  it('rejects an untrusted issuer', async () => {
    const { privateKey, rawHex } = genKey();
    const atx = sign(baseAtx(), privateKey);
    const verifier = new LocalVerifier(anchors(rawHex, { trustedIssuers: [] }));

    const result = await verifier.verifyCredential(atx);

    expect(result.valid).toBe(false);
    expect(result.rejectCategory).toBe('UNTRUSTED_ISSUER');
  });

  it('rejects when the signing key is not among the configured anchors', async () => {
    // Trusted issuer, valid signature — but signed by a key the verifier does not
    // hold. Proves the signature is actually checked against the configured keys,
    // not blindly accepted because the issuer DID is trusted.
    const signer = genKey();
    const other = genKey();
    const atx = sign(baseAtx(), signer.privateKey);
    const verifier = new LocalVerifier(anchors(other.rawHex)); // only the wrong key

    const result = await verifier.verifyCredential(atx);

    expect(result.valid).toBe(false);
    expect(result.rejectCategory).toBe('SIGNATURE_INVALID');
  });

  // Key-to-issuer binding (@opena2a/atx-verify >= 0.2.0). With a multi-issuer
  // anchor set, a credential claiming issuer A but signed by trusted issuer B's
  // key is REJECTED, because B's key (keyId controller = issuer-B) is not
  // controlled by the credential's issuer (A). One trusted issuer cannot
  // impersonate another. (Engaging binding requires the key to carry a DID-URL
  // keyId; a key with no keyId stays unbound for back-compat.)
  it('rejects a cross-issuer signature in a multi-issuer anchor set', async () => {
    const ISSUER_B = 'did:opena2a:issuer-B';
    const issuerB = genKey();
    const atxClaimingA = sign(baseAtx({ issuerDid: ISSUER_DID }), issuerB.privateKey);
    const verifier = new LocalVerifier(
      anchors(issuerB.rawHex, {
        trustedIssuers: [ISSUER_DID, ISSUER_B],
        publicKeys: [
          { algorithm: 'Ed25519', publicKeyHex: issuerB.rawHex, keyId: `${ISSUER_B}#key-1` },
        ],
      }),
    );

    const result = await verifier.verifyCredential(atxClaimingA);

    expect(result.valid).toBe(false);
    expect(result.rejectCategory).toBe('SIGNATURE_INVALID');
  });
});

describe('LocalVerifier.authorize (local broker policy)', () => {
  it('allows an action covered by a signed capability', async () => {
    const { privateKey, rawHex } = genKey();
    const atx = sign(baseAtx(), privateKey);
    const verifier = new LocalVerifier(anchors(rawHex));

    const res = await verifier.authorize(atx, { action: 'file:read' });

    expect(res.verified).toBe(true);
    expect(res.actionAllowed).toBe(true);
    expect(res.source).toBe('local');
    expect(res.denialReason).toBeUndefined();
  });

  it('denies an action not in the granted capabilities', async () => {
    const { privateKey, rawHex } = genKey();
    const atx = sign(baseAtx(), privateKey);
    const verifier = new LocalVerifier(anchors(rawHex));

    const res = await verifier.authorize(atx, { action: 'file:delete' });

    expect(res.verified).toBe(true);
    expect(res.actionAllowed).toBe(false);
    expect(res.denialReason).toContain('file:delete');
  });

  it('allows a wildcard capability', async () => {
    const { privateKey, rawHex } = genKey();
    const atx = sign(baseAtx({ capabilities: ['*'] }), privateKey);
    const verifier = new LocalVerifier(anchors(rawHex));

    const res = await verifier.authorize(atx, { action: 'anything:goes' });

    expect(res.actionAllowed).toBe(true);
  });

  it('refuses to authorize on forgeable v1.0 capabilities by default', async () => {
    const { privateKey, rawHex } = genKey();
    const atx = sign(baseAtx({ atcVersion: '1.0' }), privateKey);
    const verifier = new LocalVerifier(anchors(rawHex));

    const res = await verifier.authorize(atx, { action: 'file:read' });

    // credential is cryptographically valid, but its capabilities are not
    // signature-covered (v1.0), so capability-based authorization is refused.
    expect(res.verified).toBe(true);
    expect(res.signedCapabilities).toBe(false);
    expect(res.actionAllowed).toBe(false);
    expect(res.denialReason).toContain('v1.0');
  });

  it('allows v1.0 capability authorization only when explicitly opted in', async () => {
    const { privateKey, rawHex } = genKey();
    const atx = sign(baseAtx({ atcVersion: '1.0' }), privateKey);
    const verifier = new LocalVerifier(anchors(rawHex));

    const res = await verifier.authorize(atx, {
      action: 'file:read',
      requireSignedCapabilities: false,
    });

    expect(res.actionAllowed).toBe(true);
  });

  it('denies (fail closed) when the credential does not verify', async () => {
    const { privateKey, rawHex } = genKey();
    const atx = sign(baseAtx({ expiresAt: '2026-06-10T00:00:00Z' }), privateKey);
    const verifier = new LocalVerifier(anchors(rawHex));

    const res = await verifier.authorize(atx, { action: 'file:read' });

    expect(res.verified).toBe(false);
    expect(res.actionAllowed).toBe(false);
    expect(res.rejectCategory).toBe('EXPIRED');
  });
});

describe('LocalVerifier with a CrlCache (async-refreshed revocation)', () => {
  // Cache clock is independent of the ATX verification clock (FIXED_NOW).
  function cacheClock(start = 0) {
    let t = start;
    return { now: () => t, advance: (ms: number) => (t += ms) };
  }

  it('rejects a credential listed in the fresh cached CRL', async () => {
    const { privateKey, rawHex } = genKey();
    const atx = sign(baseAtx(), privateKey);
    const cache = new CrlCache({
      fetch: async () => ({ entries: [{ agentId: 'agent-123', reason: 'key compromise' }] }),
      ttlMs: 1000,
      now: cacheClock().now,
    });
    await cache.refreshNow();

    const verifier = new LocalVerifier(anchors(rawHex, { crlCache: cache }));
    const result = await verifier.verifyCredential(atx);

    expect(result.valid).toBe(false);
    expect(result.rejectCategory).toBe('REVOKED');
  });

  it('verifies when the agent is absent from the fresh cached CRL', async () => {
    const { privateKey, rawHex } = genKey();
    const atx = sign(baseAtx(), privateKey);
    const cache = new CrlCache({
      fetch: async () => ({ entries: [{ agentId: 'someone-else' }] }),
      ttlMs: 1000,
      now: cacheClock().now,
    });
    await cache.refreshNow();

    const verifier = new LocalVerifier(anchors(rawHex, { crlCache: cache }));
    const result = await verifier.verifyCredential(atx);

    expect(result.valid).toBe(true);
  });

  it('soft-opens on revocation when the cache is stale (default policy)', async () => {
    const { privateKey, rawHex } = genKey();
    const atx = sign(baseAtx(), privateKey);
    const ck = cacheClock();
    const cache = new CrlCache({
      // The list WOULD revoke this agent, but it goes stale before verification.
      fetch: async () => ({ entries: [{ agentId: 'agent-123' }] }),
      ttlMs: 1000,
      now: ck.now,
    });
    await cache.refreshNow();
    ck.advance(1001); // stale: beyond TTL

    const verifier = new LocalVerifier(anchors(rawHex, { crlCache: cache }));
    const result = await verifier.verifyCredential(atx);

    // Revocation is not enforced on a stale list (signature/expiry/issuer stay hard).
    expect(result.valid).toBe(true);
  });

  it('fails closed on a stale cache when onStale is "reject"', async () => {
    const { privateKey, rawHex } = genKey();
    const atx = sign(baseAtx(), privateKey);
    const ck = cacheClock();
    const cache = new CrlCache({
      fetch: async () => ({ entries: [] }), // even an empty (no-revocations) list
      ttlMs: 1000,
      onStale: 'reject',
      now: ck.now,
    });
    await cache.refreshNow();
    ck.advance(1001); // stale

    const verifier = new LocalVerifier(anchors(rawHex, { crlCache: cache }));
    const result = await verifier.verifyCredential(atx);

    expect(result.valid).toBe(false);
    expect(result.rejectCategory).toBe('REVOKED');
    expect(result.reason).toMatch(/stale/i);
  });

  it('authorize() denies a revoked-by-cache credential', async () => {
    const { privateKey, rawHex } = genKey();
    const atx = sign(baseAtx(), privateKey);
    const cache = new CrlCache({
      fetch: async () => ({ entries: [{ agentId: 'agent-123' }] }),
      ttlMs: 1000,
      now: cacheClock().now,
    });
    await cache.refreshNow();

    const verifier = new LocalVerifier(anchors(rawHex, { crlCache: cache }));
    const res = await verifier.authorize(atx, { action: 'file:read' });

    expect(res.verified).toBe(false);
    expect(res.actionAllowed).toBe(false);
    expect(res.rejectCategory).toBe('REVOKED');
  });
});

describe('LocalVerifier raw-text entry (strict parse)', () => {
  // A decoy member placed before the signed one. JSON.parse keeps the last
  // member, so the parsed object carries only the signed capabilities; a
  // first-wins parser elsewhere would read the decoy. The raw entry refuses both.
  function withDecoyCapabilities(atx: Atx): string {
    return '{"capabilities":["*"],' + JSON.stringify(atx).slice(1);
  }

  it('verifies a credential from its JSON text and from its bytes as it does the object', async () => {
    const { privateKey, rawHex } = genKey();
    const atx = sign(baseAtx(), privateKey);
    const verifier = new LocalVerifier(anchors(rawHex));

    const fromObject = await verifier.verifyCredential(atx);
    const fromText = await verifier.verifyCredential(JSON.stringify(atx));
    const fromBytes = await verifier.verifyCredential(Buffer.from(JSON.stringify(atx), 'utf8'));

    expect(fromObject.valid).toBe(true);
    expect(fromText).toEqual(fromObject);
    expect(fromBytes).toEqual(fromObject);
  });

  it('rejects a duplicate member as MALFORMED and names it', async () => {
    const { privateKey, rawHex } = genKey();
    const atx = sign(baseAtx(), privateKey);
    const verifier = new LocalVerifier(anchors(rawHex));

    const result = await verifier.verifyCredential(withDecoyCapabilities(atx));

    expect(result.valid).toBe(false);
    expect(result.rejectCategory).toBe('MALFORMED');
    expect(result.reason).toContain('duplicate member "capabilities"');
  });

  it('rejects text that is not a JSON object as MALFORMED without throwing', async () => {
    const { rawHex } = genKey();
    const verifier = new LocalVerifier(anchors(rawHex));

    for (const bad of ['', 'null', '[]', '{"agentId":', '{} trailing']) {
      const result = await verifier.verifyCredential(bad);
      expect(result.valid, `accepted: ${JSON.stringify(bad)}`).toBe(false);
      expect(result.rejectCategory).toBe('MALFORMED');
    }
  });

  it('authorize() allows a signed capability from the raw text', async () => {
    const { privateKey, rawHex } = genKey();
    const atx = sign(baseAtx(), privateKey);
    const verifier = new LocalVerifier(anchors(rawHex));

    const res = await verifier.authorize(JSON.stringify(atx), { action: 'file:read' });

    expect(res.verified).toBe(true);
    expect(res.actionAllowed).toBe(true);
    expect(res.agentId).toBe('agent-123');
  });

  it('authorize() denies a duplicate-member credential and leaves identity fields empty', async () => {
    const { privateKey, rawHex } = genKey();
    const atx = sign(baseAtx(), privateKey);
    const verifier = new LocalVerifier(anchors(rawHex));

    const res = await verifier.authorize(withDecoyCapabilities(atx), { action: 'file:read' });

    expect(res.verified).toBe(false);
    expect(res.actionAllowed).toBe(false);
    expect(res.rejectCategory).toBe('MALFORMED');
    expect(res.denialReason).toContain('duplicate member "capabilities"');
    expect(res.agentId).toBe('');
    expect(res.agentDid).toBe('');
    expect(res.issuerDid).toBe('');
  });

  it('applies the "reject" stale policy to raw input too', async () => {
    const { privateKey, rawHex } = genKey();
    const atx = sign(baseAtx(), privateKey);
    let t = 0;
    const cache = new CrlCache({
      fetch: async () => ({ entries: [] }),
      ttlMs: 1000,
      onStale: 'reject',
      now: () => t,
    });
    await cache.refreshNow();
    t += 1001; // stale

    const verifier = new LocalVerifier(anchors(rawHex, { crlCache: cache }));
    const result = await verifier.verifyCredential(JSON.stringify(atx));

    expect(result.valid).toBe(false);
    expect(result.rejectCategory).toBe('REVOKED');
    expect(result.reason).toMatch(/stale/i);
  });
});
