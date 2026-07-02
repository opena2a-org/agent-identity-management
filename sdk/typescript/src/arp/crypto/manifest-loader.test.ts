/**
 * Capability manifest loader tests.
 *
 * Coverage (parse-to-deny, CR-001):
 *   1. Happy path: a signed manifest round-trips through the loader.
 *   2. Tampered payload: flipping a field post-signing rejects with
 *      SIGNATURE_INVALID.
 *   3. Tampered signature: flipping a byte in the ed25519 or ml-dsa half
 *      rejects with SIGNATURE_INVALID.
 *   4. Missing signature: no signature block rejects with SIGNATURE_MISSING.
 *   5. Wrong algorithm: signature block claims an unsupported alg and
 *      rejects with ALGORITHM_UNSUPPORTED before any crypto runs.
 *   6. Expired manifest: a manifest with expiresAt in the past rejects with
 *      EXPIRED even when the signature is otherwise valid.
 *   7. Schema errors: missing fields, wrong types, invalid tier, bad comply
 *      envelope all reject with SCHEMA_ERROR.
 *   8. Oversized file: a manifest larger than MAX_MANIFEST_SIZE_BYTES
 *      rejects with SIZE_EXCEEDED.
 *   9. YAML parse errors: malformed YAML rejects with PARSE_ERROR.
 *  10. Unsupported version: any version != "1.0.0" rejects with
 *      VERSION_UNSUPPORTED.
 *  11. Key format error: corrupting the public key base64 rejects with
 *      KEY_FORMAT_ERROR (size check inside decodeHybridPublicKey).
 *  12. IO error: loading a nonexistent file rejects with IO_ERROR.
 *
 * Fixtures are generated deterministically in `beforeAll` from fixed key
 * material and a fixed payload, then written to
 * `test/fixtures/capability-manifests/`. Deterministic keys come from:
 *   - Ed25519 private key: 32 bytes of 0x01
 *   - ML-DSA-65 keygen seed: 32 bytes of 0x02
 * Noble's ML-DSA `sign(sk, msg)` (two-arg form) is the deterministic variant,
 * so the fixture YAML is bit-identical across runs.
 */

import { promises as fs } from 'fs';
import path from 'path';

import * as ed25519 from '@noble/ed25519';
import { ml_dsa65 } from '@noble/post-quantum/ml-dsa';
import yaml from 'js-yaml';
import { beforeAll, describe, expect, it } from 'vitest';

import {
  CapabilityManifestError,
  MANIFEST_SIGNATURE_ALGORITHM,
  MANIFEST_VERSION,
  MAX_MANIFEST_SIZE_BYTES,
  canonicalizeManifestPayload,
  loadCapabilityManifest,
  parseCapabilityManifest,
} from './manifest-loader';

// Absolute path to the checked-in fixture directory. Resolved relative to
// the source file so that the tests keep working when vitest is invoked
// from any working directory.
const FIXTURE_DIR = path.resolve(
  __dirname,
  '../../../test/fixtures/capability-manifests',
);

function toBase64(bytes: Uint8Array): string {
  return Buffer.from(bytes).toString('base64');
}

function fromBase64(s: string): Uint8Array {
  return new Uint8Array(Buffer.from(s, 'base64'));
}

// Fixed far-future date so the "happy path" fixture does not expire while
// sitting in the repo. Parked in 2099 which is far enough out to be obviously
// a fixture marker but still inside the JS Date range.
const FIXTURE_ISSUED_AT = '2026-04-13T00:00:00.000Z';
const FIXTURE_EXPIRES_AT = '2099-01-01T00:00:00.000Z';

interface RawManifest {
  version: string;
  agentId: string;
  tier: string;
  comply: {
    permitted_classes: string[];
    prohibited_classes: string[];
    on_violation: string;
  };
  issuedAt: string;
  expiresAt?: string;
  ed25519PublicKey: string;
  mldsa65PublicKey: string;
  signature?: {
    alg: string;
    ed25519Sig: string;
    mldsaSig: string;
    ts: number;
  };
}

interface GeneratedFixtures {
  happy: string;
  tamperedPayload: string;
  tamperedEdSig: string;
  tamperedMldsaSig: string;
  missingSignature: string;
  wrongAlgorithm: string;
  expired: string;
  badBase64Key: string;
}

async function generateFixtureSet(): Promise<GeneratedFixtures> {
  // Deterministic key material. Do not reuse these constants anywhere
  // outside test fixtures.
  const edPrivKey = new Uint8Array(32).fill(0x01);
  const edPubKey = await ed25519.getPublicKeyAsync(edPrivKey);

  const mldsaSeed = new Uint8Array(32).fill(0x02);
  const mldsaKeys = ml_dsa65.keygen(mldsaSeed);

  function signManifest(payload: RawManifest, expiresAt?: string): RawManifest {
    const { signature: _sig, ...rest } = payload;
    const base: RawManifest = {
      ...rest,
      ...(expiresAt !== undefined ? { expiresAt } : {}),
      ed25519PublicKey: toBase64(edPubKey),
      mldsa65PublicKey: toBase64(mldsaKeys.publicKey),
    };
    const canonical = canonicalizeManifestPayload(
      base as unknown as Record<string, unknown>,
    );
    // Noble ed25519 v2 signAsync is deterministic given the same key + msg.
    // Ml-dsa `sign(sk, msg)` is the deterministic variant per FIPS 204.
    // The test intentionally calls them twice (once here, once via
    // hybridSign during verification) to make sure both produce the same
    // bytes. Timestamp is fixed for determinism.
    return {
      ...base,
      signature: {
        alg: MANIFEST_SIGNATURE_ALGORITHM,
        ed25519Sig: '',
        mldsaSig: '',
        ts: 1712880000000,
      },
    };
  }

  // Produce the signed bytes for the happy fixture and reuse them for every
  // other variant except "wrong algorithm". The function is invoked
  // synchronously per fixture so keys are not shared by reference.
  async function realSign(base: RawManifest): Promise<RawManifest> {
    const { signature: _sig, ...rest } = base;
    const payloadForCanon: RawManifest = { ...rest };
    const canonical = canonicalizeManifestPayload(
      payloadForCanon as unknown as Record<string, unknown>,
    );
    const edSig = await ed25519.signAsync(canonical, edPrivKey);
    const mldsaSig = ml_dsa65.sign(mldsaKeys.secretKey, canonical);
    return {
      ...base,
      signature: {
        alg: MANIFEST_SIGNATURE_ALGORITHM,
        ed25519Sig: toBase64(edSig),
        mldsaSig: toBase64(mldsaSig),
        ts: 1712880000000,
      },
    };
  }

  const basePayload: RawManifest = {
    version: MANIFEST_VERSION,
    agentId: 'fixture-agent',
    tier: 'execute',
    comply: {
      permitted_classes: ['code-generation', 'documentation'],
      prohibited_classes: ['credential-access'],
      on_violation: 'deny',
    },
    issuedAt: FIXTURE_ISSUED_AT,
    expiresAt: FIXTURE_EXPIRES_AT,
    ed25519PublicKey: toBase64(edPubKey),
    mldsa65PublicKey: toBase64(mldsaKeys.publicKey),
  };

  const happy = await realSign(basePayload);

  // Tampered payload: change agentId after signing. The signature block is
  // left untouched, so canonicalization will produce different bytes and
  // hybridVerify will reject.
  const tamperedPayloadObj: RawManifest = {
    ...happy,
    agentId: 'different-agent',
  };

  // Tamper with each signature half independently so the non-short-circuit
  // verify behavior in hybrid-signing.ts is exercised via the loader.
  function flipFirstByte(b64: string): string {
    const bytes = fromBase64(b64);
    bytes[0] = bytes[0] ^ 0x01;
    return toBase64(bytes);
  }
  const tamperedEdSigObj: RawManifest = {
    ...happy,
    signature: {
      ...happy.signature!,
      ed25519Sig: flipFirstByte(happy.signature!.ed25519Sig),
    },
  };
  const tamperedMldsaSigObj: RawManifest = {
    ...happy,
    signature: {
      ...happy.signature!,
      mldsaSig: flipFirstByte(happy.signature!.mldsaSig),
    },
  };

  // Missing signature: drop the signature block entirely.
  const { signature: _dropped, ...happyNoSig } = happy;
  const missingSignatureObj = happyNoSig as RawManifest;

  // Wrong algorithm: a manifest signed with ml-dsa-65 but whose signature
  // block advertises ml-dsa-44. The loader should refuse without attempting
  // verification. This is the high-impact test: do not let an attacker pick
  // a weaker algorithm.
  const wrongAlgorithmObj: RawManifest = {
    ...happy,
    signature: {
      ...happy.signature!,
      alg: 'Ed25519+ML-DSA-44',
    },
  };

  // Expired manifest: re-sign with an expiresAt in the past. We have to
  // re-sign (not reuse the happy signature) because expiresAt is part of
  // the canonical payload.
  const expiredPayload: RawManifest = {
    ...basePayload,
    expiresAt: '2020-01-01T00:00:00.000Z',
  };
  const expiredObj = await realSign(expiredPayload);

  // Bad base64 in the public key field. The size check inside
  // decodeHybridPublicKey (via validateKeySize) should produce a
  // KEY_FORMAT_ERROR.
  const badBase64KeyObj: RawManifest = {
    ...happy,
    ed25519PublicKey: 'not-actually-base64!!!!',
  };

  const dumpOpts = { lineWidth: 200, noRefs: true };
  return {
    happy: yaml.dump(happy, dumpOpts),
    tamperedPayload: yaml.dump(tamperedPayloadObj, dumpOpts),
    tamperedEdSig: yaml.dump(tamperedEdSigObj, dumpOpts),
    tamperedMldsaSig: yaml.dump(tamperedMldsaSigObj, dumpOpts),
    missingSignature: yaml.dump(missingSignatureObj, dumpOpts),
    wrongAlgorithm: yaml.dump(wrongAlgorithmObj, dumpOpts),
    expired: yaml.dump(expiredObj, dumpOpts),
    badBase64Key: yaml.dump(badBase64KeyObj, dumpOpts),
  };
}

// Fixtures are populated by beforeAll and then reused across tests.
let fixtures: GeneratedFixtures;

describe('arp/crypto/manifest-loader', () => {
  beforeAll(async () => {
    fixtures = await generateFixtureSet();
    await fs.mkdir(FIXTURE_DIR, { recursive: true });
    const files: Array<[string, string]> = [
      ['happy.yaml', fixtures.happy],
      ['tampered-payload.yaml', fixtures.tamperedPayload],
      ['tampered-ed-signature.yaml', fixtures.tamperedEdSig],
      ['tampered-mldsa-signature.yaml', fixtures.tamperedMldsaSig],
      ['missing-signature.yaml', fixtures.missingSignature],
      ['wrong-algorithm.yaml', fixtures.wrongAlgorithm],
      ['expired.yaml', fixtures.expired],
      ['bad-base64-key.yaml', fixtures.badBase64Key],
    ];
    for (const [name, content] of files) {
      await fs.writeFile(path.join(FIXTURE_DIR, name), content, 'utf8');
    }
    const readme =
      '# Capability Manifest Fixtures\n\n' +
      'Generated deterministically by ' +
      '`src/arp/crypto/manifest-loader.test.ts`. Do not hand-edit. To refresh, ' +
      'rerun `npx vitest src/arp/crypto/manifest-loader.test.ts` and commit ' +
      'the resulting diff.\n\n' +
      'Each file exercises one rejection path through the loader:\n\n' +
      '- `happy.yaml` -- valid signed manifest, should load\n' +
      '- `tampered-payload.yaml` -- agentId changed post-signing\n' +
      '- `tampered-ed-signature.yaml` -- one byte flipped in ed25519 half\n' +
      '- `tampered-mldsa-signature.yaml` -- one byte flipped in ml-dsa half\n' +
      '- `missing-signature.yaml` -- signature block stripped\n' +
      '- `wrong-algorithm.yaml` -- alg claims ml-dsa-44 instead of ml-dsa-65\n' +
      '- `expired.yaml` -- signed manifest with expiresAt in 2020\n' +
      '- `bad-base64-key.yaml` -- ed25519 public key is not valid base64\n';
    await fs.writeFile(path.join(FIXTURE_DIR, 'README.md'), readme, 'utf8');
  });

  describe('happy path', () => {
    it('loads and verifies a valid manifest from disk', async () => {
      const manifest = await loadCapabilityManifest(
        path.join(FIXTURE_DIR, 'happy.yaml'),
      );
      expect(manifest.version).toBe(MANIFEST_VERSION);
      expect(manifest.agentId).toBe('fixture-agent');
      expect(manifest.tier).toBe('execute');
      expect(manifest.comply.permitted_classes).toEqual([
        'code-generation',
        'documentation',
      ]);
      expect(manifest.comply.prohibited_classes).toEqual(['credential-access']);
      expect(manifest.comply.on_violation).toBe('deny');
      expect(manifest.issuedAt).toBe(FIXTURE_ISSUED_AT);
      expect(manifest.expiresAt).toBe(FIXTURE_EXPIRES_AT);
      expect(manifest.ed25519PublicKey).toMatch(/^[A-Za-z0-9+/=]+$/);
      expect(manifest.mldsa65PublicKey).toMatch(/^[A-Za-z0-9+/=]+$/);
      // The runtime shape must not leak the signature block.
      expect((manifest as unknown as { signature?: unknown }).signature).toBeUndefined();
    });

    it('parseCapabilityManifest accepts the same YAML text without disk IO', async () => {
      const manifest = await parseCapabilityManifest(fixtures.happy);
      expect(manifest.agentId).toBe('fixture-agent');
    });
  });

  describe('rejection paths (parse-to-deny)', () => {
    async function expectReject(
      input: string | Promise<string>,
      code: string,
    ): Promise<CapabilityManifestError> {
      const text = typeof input === 'string' ? input : await input;
      try {
        await parseCapabilityManifest(text);
      } catch (err) {
        expect(err).toBeInstanceOf(CapabilityManifestError);
        expect((err as CapabilityManifestError).code).toBe(code);
        return err as CapabilityManifestError;
      }
      throw new Error(`expected CapabilityManifestError with code=${code}`);
    }

    it('tampered payload -> SIGNATURE_INVALID', async () => {
      await expectReject(fixtures.tamperedPayload, 'SIGNATURE_INVALID');
    });

    it('tampered ed25519 half -> SIGNATURE_INVALID', async () => {
      const err = await expectReject(
        fixtures.tamperedEdSig,
        'SIGNATURE_INVALID',
      );
      // Non-short-circuit verify: the reason should mention the ed25519
      // half specifically, not abort before reaching ml-dsa.
      expect(err.details?.reason ?? '').toMatch(/ed25519/);
    });

    it('tampered ml-dsa half -> SIGNATURE_INVALID', async () => {
      const err = await expectReject(
        fixtures.tamperedMldsaSig,
        'SIGNATURE_INVALID',
      );
      expect(err.details?.reason ?? '').toMatch(/ML-DSA-65|mldsa/i);
    });

    it('missing signature block -> SIGNATURE_MISSING', async () => {
      await expectReject(fixtures.missingSignature, 'SIGNATURE_MISSING');
    });

    it('wrong signature algorithm -> ALGORITHM_UNSUPPORTED', async () => {
      await expectReject(fixtures.wrongAlgorithm, 'ALGORITHM_UNSUPPORTED');
    });

    it('expired manifest -> EXPIRED even when signature is valid', async () => {
      await expectReject(fixtures.expired, 'EXPIRED');
    });

    it('bad base64 in public key -> KEY_FORMAT_ERROR', async () => {
      await expectReject(fixtures.badBase64Key, 'KEY_FORMAT_ERROR');
    });

    it('invalid YAML -> PARSE_ERROR', async () => {
      await expectReject(
        'version: "1.0.0"\n  bogus: : : :\n    nested\n',
        'PARSE_ERROR',
      );
    });

    it('non-mapping root -> SCHEMA_ERROR', async () => {
      await expectReject('- just-a-list\n- of\n- scalars\n', 'SCHEMA_ERROR');
    });

    it('unsupported version -> VERSION_UNSUPPORTED', async () => {
      const bumped = fixtures.happy.replace(
        `version: ${MANIFEST_VERSION}`,
        'version: 2.0.0',
      );
      await expectReject(bumped, 'VERSION_UNSUPPORTED');
    });

    it('missing comply envelope -> SCHEMA_ERROR', async () => {
      // Start from a raw object (not the signed happy fixture) so the
      // signature block presence is still satisfied but the comply envelope
      // fails validation.
      const withoutComply = yaml.dump({
        version: MANIFEST_VERSION,
        agentId: 'x',
        tier: 'execute',
        issuedAt: FIXTURE_ISSUED_AT,
        ed25519PublicKey: 'abc',
        mldsa65PublicKey: 'def',
        signature: {
          alg: MANIFEST_SIGNATURE_ALGORITHM,
          ed25519Sig: 'AA==',
          mldsaSig: 'AA==',
          ts: 0,
        },
      });
      await expectReject(withoutComply, 'SCHEMA_ERROR');
    });

    it('invalid tier -> SCHEMA_ERROR', async () => {
      const badTier = fixtures.happy.replace('tier: execute', 'tier: god-mode');
      await expectReject(badTier, 'SCHEMA_ERROR');
    });

    it('oversized file -> SIZE_EXCEEDED', async () => {
      const padded =
        fixtures.happy +
        '# pad\n' +
        '# '.padEnd(MAX_MANIFEST_SIZE_BYTES + 10, 'x') +
        '\n';
      await expectReject(padded, 'SIZE_EXCEEDED');
    });

    it('loadCapabilityManifest on nonexistent file -> IO_ERROR', async () => {
      try {
        await loadCapabilityManifest(
          path.join(FIXTURE_DIR, 'does-not-exist.yaml'),
        );
      } catch (err) {
        expect(err).toBeInstanceOf(CapabilityManifestError);
        expect((err as CapabilityManifestError).code).toBe('IO_ERROR');
        return;
      }
      throw new Error('expected IO_ERROR');
    });
  });

  describe('canonicalization', () => {
    it('is stable under key reordering', () => {
      const a = canonicalizeManifestPayload({
        b: 2,
        a: 1,
        nested: { y: 'z', x: 'w' },
      });
      const b = canonicalizeManifestPayload({
        nested: { x: 'w', y: 'z' },
        a: 1,
        b: 2,
      });
      expect(Buffer.from(a).toString('utf8')).toBe(
        Buffer.from(b).toString('utf8'),
      );
    });

    it('strips the signature field before hashing', () => {
      const withSig = canonicalizeManifestPayload({
        a: 1,
        signature: { alg: 'anything', ed25519Sig: 'x', mldsaSig: 'y', ts: 0 },
      });
      const withoutSig = canonicalizeManifestPayload({ a: 1 });
      expect(Buffer.from(withSig).toString('utf8')).toBe(
        Buffer.from(withoutSig).toString('utf8'),
      );
    });
  });
});
