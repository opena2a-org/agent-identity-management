/**
 * AIM-12 item 14: the LocalVerifier constructor stored whatever it was handed.
 * From JavaScript (or a mis-cast config object) `trustedIssuers: 'did:one'` or
 * `publicKeys: {...}` was accepted silently and every later verification
 * failed closed with a misleading reason — or, worse, an anchors object of the
 * wrong shape behaved differently from what the operator believed they pinned.
 * Trust anchors are exactly the input that must fail loudly at construction.
 */
import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { LocalVerifier } from './LocalVerifier';
import { ConfigurationError } from '../exceptions';

const KEY = {
  algorithm: 'Ed25519',
  publicKeyHex: 'a'.repeat(64),
};

interface FixtureKey {
  algorithm: string;
  publicKeyHex: string;
  keyId?: string;
}

const HYBRID = JSON.parse(
  readFileSync(
    join(__dirname, '..', '..', '..', 'java', 'src', 'test', 'resources', 'atx-fixtures', 'v1_1-baseline-valid-hybrid.json'),
    'utf8',
  ),
) as { atx: never; verifierState: { clockRfc3339: string; trustedIssuers: string[]; publicKeys: FixtureKey[] } };

/** The hybrid fixture's anchors, with its ML-DSA-65 key's hex replaced. */
function hybridAnchorsWithMldsaHex(publicKeyHex: unknown) {
  const vs = HYBRID.verifierState;
  return {
    trustedIssuers: vs.trustedIssuers,
    publicKeys: vs.publicKeys.map((k) =>
      k.algorithm === 'ML-DSA-65' ? { ...k, publicKeyHex: publicKeyHex as string } : { ...k },
    ),
    now: () => new Date(vs.clockRfc3339),
  };
}

function mldsaHex(): string {
  const key = HYBRID.verifierState.publicKeys.find((k) => k.algorithm === 'ML-DSA-65');
  if (!key) {
    throw new Error('hybrid fixture has no ML-DSA-65 key');
  }
  return key.publicKeyHex;
}

describe('a malformed ML-DSA-65 trust anchor fails at construction', () => {
  // A mistyped key used to be dropped at verification, so every hybrid
  // credential rejected with "no eligible ML-DSA-65 trust anchor".
  it('a 3903-hex-character ML-DSA-65 key throws ConfigurationError naming the key', () => {
    const construct = () => new LocalVerifier(hybridAnchorsWithMldsaHex(mldsaHex().slice(0, 3903)));
    expect(construct).toThrow(ConfigurationError);
    expect(construct).toThrow(/did:opena2a:authority:opena2a\.org#key-1-pqc/);
    expect(construct).toThrow(/3904 hex characters/);
  });

  it('an ML-DSA-65 key one hex character too long throws ConfigurationError', () => {
    expect(() => new LocalVerifier(hybridAnchorsWithMldsaHex(`${mldsaHex()}0`))).toThrow(ConfigurationError);
  });

  it('an ML-DSA-65 key of the right length with a non-hex character throws ConfigurationError', () => {
    expect(() => new LocalVerifier(hybridAnchorsWithMldsaHex(`${mldsaHex().slice(0, 3903)}g`))).toThrow(
      ConfigurationError,
    );
  });

  it('an ML-DSA-65 key whose publicKeyHex is not a string throws ConfigurationError', () => {
    expect(() => new LocalVerifier(hybridAnchorsWithMldsaHex(undefined))).toThrow(ConfigurationError);
  });

  it('the well-formed ML-DSA-65 key constructs and the hybrid credential verifies', async () => {
    const verifier = new LocalVerifier(hybridAnchorsWithMldsaHex(mldsaHex()));
    const result = await verifier.verifyCredential(HYBRID.atx);
    expect(result.valid).toBe(true);
    expect(result.mldsaPresent).toBe(true);
  });
});

describe('trust anchors are validated at construction', () => {
  it('AIM-12.AC3 item 14: a non-array trustedIssuers throws ConfigurationError', () => {
    expect(
      () =>
        new LocalVerifier({
          trustedIssuers: 'did:aim:issuer' as unknown as string[],
          publicKeys: [KEY],
        }),
    ).toThrow(ConfigurationError);
  });

  it('AIM-12.AC3 item 14: a non-string entry in trustedIssuers throws ConfigurationError', () => {
    expect(
      () =>
        new LocalVerifier({
          trustedIssuers: [42] as unknown as string[],
          publicKeys: [KEY],
        }),
    ).toThrow(ConfigurationError);
  });

  it('AIM-12.AC3 item 14: a non-array publicKeys throws ConfigurationError', () => {
    expect(
      () =>
        new LocalVerifier({
          trustedIssuers: ['did:aim:issuer'],
          publicKeys: { ed25519: 'aa' } as never,
        }),
    ).toThrow(ConfigurationError);
  });

  it('AIM-12.AC3 item 14: a non-object publicKeys entry throws ConfigurationError', () => {
    expect(
      () =>
        new LocalVerifier({
          trustedIssuers: ['did:aim:issuer'],
          publicKeys: ['aa'] as never,
        }),
    ).toThrow(ConfigurationError);
  });

  it('valid anchors still construct — including the deliberate empty-trustedIssuers case the suite pins', () => {
    expect(
      () => new LocalVerifier({ trustedIssuers: [], publicKeys: [KEY] }),
    ).not.toThrow();
    expect(
      () => new LocalVerifier({ trustedIssuers: ['did:aim:issuer'], publicKeys: [KEY] }),
    ).not.toThrow();
  });
});
