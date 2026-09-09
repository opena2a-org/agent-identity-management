/**
 * AIM-12 item 14: the LocalVerifier constructor stored whatever it was handed.
 * From JavaScript (or a mis-cast config object) `trustedIssuers: 'did:one'` or
 * `publicKeys: {...}` was accepted silently and every later verification
 * failed closed with a misleading reason — or, worse, an anchors object of the
 * wrong shape behaved differently from what the operator believed they pinned.
 * Trust anchors are exactly the input that must fail loudly at construction.
 */
import { describe, it, expect } from 'vitest';
import { LocalVerifier } from './LocalVerifier';
import { ConfigurationError } from '../exceptions';

const KEY = {
  algorithm: 'Ed25519',
  publicKeyHex: 'a'.repeat(64),
};

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
