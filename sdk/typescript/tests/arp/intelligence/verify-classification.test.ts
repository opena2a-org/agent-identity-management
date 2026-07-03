/**
 * Unit coverage for the NanoMind-Guard classification verifier.
 *
 * Coverage:
 *   1. Happy path -- a signed result round-trips through verifyClassification
 *      and the cleared classification can be written into event.data.
 *   2. Schema errors -- missing or wrong-typed fields reject with SCHEMA_ERROR
 *      before any crypto runs.
 *   3. Algorithm lane -- ML-DSA-65 or a Guard key carrying the wrong variant
 *      is refused with ALGORITHM_UNSUPPORTED (no downgrade, no cross-lane key
 *      replay between manifest signing and classification signing).
 *   4. Key format errors -- malformed base64 or wrong-length keys/signatures
 *      reject with KEY_FORMAT_ERROR.
 *   5. Signature invalid -- tampering with the payload, the ed25519 half, or
 *      the ml-dsa half each reject with SIGNATURE_INVALID. The non-short-
 *      circuit verify property is exercised by tampering each half
 *      independently.
 *   6. Freshness -- a valid signature over a stale timestamp rejects with
 *      STALE. A signature from slightly-in-the-future is tolerated under
 *      the default skew window but rejected once past futureSkewMs.
 *   7. Tier rejection matrix -- absolute deny classes reject at every tier,
 *      unknown classes reject as parse-to-deny, and a class whose minimum
 *      tier exceeds the manifest tier rejects with TIER_REJECTED.
 *   8. applyVerifiedClassification helper -- writes event.data.classification
 *      on success, leaves it untouched on failure.
 *   9. Producer -> coordinator loop -- a verified classification is handed
 *      to an IntelligenceCoordinator with the matching manifest, and the
 *      coordinator's L0-comply gate enforces the per-manifest envelope on
 *      top of the tier ceiling.
 *
 * Deterministic key material: Ed25519 private key is 32 bytes of 0x03 and
 * the ML-DSA-44 seed is 32 bytes of 0x04. These are intentionally disjoint
 * from the manifest-loader fixtures (0x01 / 0x02) so a test swap between
 * the two lanes cannot accidentally produce a round-trip.
 */

import * as os from 'os';
import * as path from 'path';
import * as fs from 'fs';

import * as ed25519 from '@noble/ed25519';
import { ml_dsa44 } from '@noble/post-quantum/ml-dsa';
import { describe, it, expect, beforeAll, beforeEach, afterEach } from 'vitest';

import {
  ABSOLUTE_DENY_CLASSES,
  CLASSIFICATION_MIN_TIER,
  DEFAULT_FUTURE_SKEW_MS,
  DEFAULT_MAX_AGE_MS,
  GUARD_SIGNATURE_ALGORITHM,
  applyVerifiedClassification,
  canonicalizeGuardResultPayload,
  verifyClassification,
} from '../../../src/arp/intelligence/verify-classification';
import { IntelligenceCoordinator } from '../../../src/arp/intelligence/coordinator';
import type {
  ARPConfig,
  ARPEvent,
  CapabilityManifest,
  CapabilityTier,
  NanoMindGuardResult,
  NanoMindGuardVerifyOptions,
} from '../../../src/arp/types';
import type {
  EncodedHybridPublicKey,
  EncodedHybridSignature,
} from '../../../src/arp/crypto/types';

// --- Deterministic key material and signing helpers ------------------------

const ED_PRIV = new Uint8Array(32).fill(0x03);
const MLDSA_SEED = new Uint8Array(32).fill(0x04);

function toBase64(bytes: Uint8Array): string {
  return Buffer.from(bytes).toString('base64');
}

function fromBase64(encoded: string): Uint8Array {
  return new Uint8Array(Buffer.from(encoded, 'base64'));
}

interface GuardKeys {
  edPub: Uint8Array;
  mldsaPub: Uint8Array;
  mldsaSecret: Uint8Array;
  publicKey: EncodedHybridPublicKey;
}

let guardKeys: GuardKeys;

async function makeGuardKeys(): Promise<GuardKeys> {
  const edPub = await ed25519.getPublicKeyAsync(ED_PRIV);
  const mldsaKeys = ml_dsa44.keygen(MLDSA_SEED);
  const publicKey: EncodedHybridPublicKey = {
    algorithm: GUARD_SIGNATURE_ALGORITHM,
    ed25519PublicKey: toBase64(edPub),
    mldsaPublicKey: toBase64(mldsaKeys.publicKey),
    mldsaVariant: 'ML-DSA-44',
  };
  return {
    edPub,
    mldsaPub: mldsaKeys.publicKey,
    mldsaSecret: mldsaKeys.secretKey,
    publicKey,
  };
}

async function signResult(
  payload: Omit<NanoMindGuardResult, 'signature'>,
  keys: GuardKeys = guardKeys,
  algOverride?: string,
): Promise<NanoMindGuardResult> {
  const canonical = canonicalizeGuardResultPayload(
    payload as unknown as Record<string, unknown>,
  );
  const edSig = await ed25519.signAsync(canonical, ED_PRIV);
  const mldsaSig = ml_dsa44.sign(keys.mldsaSecret, canonical);
  const signature: EncodedHybridSignature = {
    alg: (algOverride ?? GUARD_SIGNATURE_ALGORITHM) as EncodedHybridSignature['alg'],
    ed25519Sig: toBase64(edSig),
    mldsaSig: toBase64(mldsaSig),
    ts: payload.timestamp,
  };
  return { ...payload, signature };
}

// --- Fixture factories ------------------------------------------------------

const FIXED_NOW = 1_745_000_000_000; // 2025-04-18T... stable clock for tests

function basePayload(
  overrides: Partial<Omit<NanoMindGuardResult, 'signature'>> = {},
): Omit<NanoMindGuardResult, 'signature'> {
  return {
    classification: 'code-generation',
    confidence: 0.87,
    modelVersion: 'nanomind-guard-0.1.0',
    contentHash: 'a'.repeat(64),
    timestamp: FIXED_NOW,
    ...overrides,
  };
}

function makeManifest(
  tier: CapabilityTier = 'execute',
  overrides: Partial<CapabilityManifest['comply']> = {},
): CapabilityManifest {
  return {
    version: '1.0.0',
    agentId: 'verify-classification-test-agent',
    tier,
    comply: {
      permitted_classes: ['code-generation', 'documentation'],
      prohibited_classes: ['credential-access'],
      on_violation: 'deny',
      ...overrides,
    },
    issuedAt: '2026-04-14T00:00:00.000Z',
    ed25519PublicKey: 'unused-in-unit-test',
    mldsa65PublicKey: 'unused-in-unit-test',
  };
}

function makeOptions(
  overrides: Partial<NanoMindGuardVerifyOptions> = {},
): NanoMindGuardVerifyOptions {
  return {
    guardPublicKey: guardKeys.publicKey,
    manifest: makeManifest(),
    now: () => FIXED_NOW,
    ...overrides,
  };
}

beforeAll(async () => {
  guardKeys = await makeGuardKeys();
});

// --- Tests ------------------------------------------------------------------

describe('arp/intelligence/verify-classification', () => {
  describe('happy path', () => {
    it('accepts a valid signed result and returns the cleared classification', async () => {
      const result = await signResult(basePayload());
      const verify = await verifyClassification(result, makeOptions());

      expect(verify.valid).toBe(true);
      if (verify.valid) {
        expect(verify.classification).toBe('code-generation');
        expect(verify.tier).toBe('execute');
        expect(verify.confidence).toBe(0.87);
      }
    });

    it('documentation is permitted at every tier down to minimal', async () => {
      const result = await signResult(
        basePayload({ classification: 'documentation' }),
      );
      const verify = await verifyClassification(
        result,
        makeOptions({ manifest: makeManifest('minimal') }),
      );
      expect(verify.valid).toBe(true);
    });

    it('privileged tier permits system-mutation (and everything beneath it)', async () => {
      const result = await signResult(
        basePayload({ classification: 'system-mutation' }),
      );
      const verify = await verifyClassification(
        result,
        makeOptions({ manifest: makeManifest('privileged') }),
      );
      expect(verify.valid).toBe(true);
    });
  });

  describe('schema errors', () => {
    it('rejects non-object result', async () => {
      const verify = await verifyClassification(
        null as unknown as NanoMindGuardResult,
        makeOptions(),
      );
      expect(verify.valid).toBe(false);
      if (!verify.valid) expect(verify.code).toBe('SCHEMA_ERROR');
    });

    it('rejects empty classification', async () => {
      const result = await signResult(basePayload({ classification: '' }));
      const verify = await verifyClassification(result, makeOptions());
      expect(verify.valid).toBe(false);
      if (!verify.valid) expect(verify.code).toBe('SCHEMA_ERROR');
    });

    it('rejects confidence out of [0, 1]', async () => {
      const result = await signResult(basePayload({ confidence: 1.5 }));
      const verify = await verifyClassification(result, makeOptions());
      expect(verify.valid).toBe(false);
      if (!verify.valid) expect(verify.code).toBe('SCHEMA_ERROR');
    });

    it('rejects non-finite confidence', async () => {
      const result = await signResult(basePayload({ confidence: NaN }));
      const verify = await verifyClassification(result, makeOptions());
      expect(verify.valid).toBe(false);
      if (!verify.valid) expect(verify.code).toBe('SCHEMA_ERROR');
    });

    it('rejects non-hex contentHash', async () => {
      const result = await signResult(
        basePayload({ contentHash: 'not-a-hash' }),
      );
      const verify = await verifyClassification(result, makeOptions());
      expect(verify.valid).toBe(false);
      if (!verify.valid) expect(verify.code).toBe('SCHEMA_ERROR');
    });

    it('rejects contentHash with wrong length', async () => {
      const result = await signResult(basePayload({ contentHash: 'ab'.repeat(16) }));
      const verify = await verifyClassification(result, makeOptions());
      expect(verify.valid).toBe(false);
      if (!verify.valid) expect(verify.code).toBe('SCHEMA_ERROR');
    });

    it('rejects signature block that is not a mapping', async () => {
      const result = await signResult(basePayload());
      (result as unknown as { signature: unknown }).signature = 'not-an-object';
      const verify = await verifyClassification(result, makeOptions());
      expect(verify.valid).toBe(false);
      if (!verify.valid) expect(verify.code).toBe('SCHEMA_ERROR');
    });

    it('rejects signature with missing fields', async () => {
      const result = await signResult(basePayload());
      (result.signature as unknown as Record<string, unknown>).ed25519Sig = '';
      const verify = await verifyClassification(result, makeOptions());
      expect(verify.valid).toBe(false);
      if (!verify.valid) expect(verify.code).toBe('SCHEMA_ERROR');
    });
  });

  describe('algorithm lane', () => {
    it('rejects signature advertising Ed25519+ML-DSA-65 (manifest lane)', async () => {
      const payload = basePayload();
      const result = await signResult(payload, guardKeys, 'Ed25519+ML-DSA-65');
      const verify = await verifyClassification(result, makeOptions());
      expect(verify.valid).toBe(false);
      if (!verify.valid) expect(verify.code).toBe('ALGORITHM_UNSUPPORTED');
    });

    it('rejects guard public key that carries the wrong algorithm', async () => {
      const result = await signResult(basePayload());
      const badKey: EncodedHybridPublicKey = {
        ...guardKeys.publicKey,
        algorithm: 'Ed25519+ML-DSA-65',
        mldsaVariant: 'ML-DSA-65',
      };
      const verify = await verifyClassification(
        result,
        makeOptions({ guardPublicKey: badKey }),
      );
      expect(verify.valid).toBe(false);
      if (!verify.valid) expect(verify.code).toBe('ALGORITHM_UNSUPPORTED');
    });

    it('rejects guard public key with correct algorithm but wrong mldsaVariant string', async () => {
      const result = await signResult(basePayload());
      const badKey: EncodedHybridPublicKey = {
        ...guardKeys.publicKey,
        mldsaVariant: 'ML-DSA-87',
      };
      const verify = await verifyClassification(
        result,
        makeOptions({ guardPublicKey: badKey }),
      );
      expect(verify.valid).toBe(false);
      if (!verify.valid) expect(verify.code).toBe('ALGORITHM_UNSUPPORTED');
    });
  });

  describe('key format errors', () => {
    it('rejects ed25519 public key that is wrong length after base64 decode', async () => {
      const result = await signResult(basePayload());
      const badKey: EncodedHybridPublicKey = {
        ...guardKeys.publicKey,
        ed25519PublicKey: toBase64(new Uint8Array(16)), // 16 bytes, not 32
      };
      const verify = await verifyClassification(
        result,
        makeOptions({ guardPublicKey: badKey }),
      );
      expect(verify.valid).toBe(false);
      if (!verify.valid) expect(verify.code).toBe('KEY_FORMAT_ERROR');
    });

    it('rejects ml-dsa-44 public key that is wrong length', async () => {
      const result = await signResult(basePayload());
      const badKey: EncodedHybridPublicKey = {
        ...guardKeys.publicKey,
        mldsaPublicKey: toBase64(new Uint8Array(100)),
      };
      const verify = await verifyClassification(
        result,
        makeOptions({ guardPublicKey: badKey }),
      );
      expect(verify.valid).toBe(false);
      if (!verify.valid) expect(verify.code).toBe('KEY_FORMAT_ERROR');
    });

    it('rejects ed25519 signature that is wrong length', async () => {
      const result = await signResult(basePayload());
      result.signature.ed25519Sig = toBase64(new Uint8Array(32));
      const verify = await verifyClassification(result, makeOptions());
      expect(verify.valid).toBe(false);
      if (!verify.valid) expect(verify.code).toBe('KEY_FORMAT_ERROR');
    });

    it('rejects ml-dsa-44 signature that is wrong length', async () => {
      const result = await signResult(basePayload());
      result.signature.mldsaSig = toBase64(new Uint8Array(200));
      const verify = await verifyClassification(result, makeOptions());
      expect(verify.valid).toBe(false);
      if (!verify.valid) expect(verify.code).toBe('KEY_FORMAT_ERROR');
    });
  });

  describe('signature invalid (parse-to-deny)', () => {
    function flipFirstByte(b64: string): string {
      const bytes = fromBase64(b64);
      bytes[0] = bytes[0] ^ 0x01;
      return toBase64(bytes);
    }

    it('rejects a payload whose classification was mutated post-signing', async () => {
      const result = await signResult(basePayload());
      result.classification = 'documentation';
      const verify = await verifyClassification(result, makeOptions());
      expect(verify.valid).toBe(false);
      if (!verify.valid) expect(verify.code).toBe('SIGNATURE_INVALID');
    });

    it('rejects a payload whose contentHash was mutated post-signing', async () => {
      const result = await signResult(basePayload());
      result.contentHash = 'b'.repeat(64);
      const verify = await verifyClassification(result, makeOptions());
      expect(verify.valid).toBe(false);
      if (!verify.valid) expect(verify.code).toBe('SIGNATURE_INVALID');
    });

    it('rejects a flipped ed25519 half and reports the ed25519 rejection', async () => {
      const result = await signResult(basePayload());
      result.signature.ed25519Sig = flipFirstByte(result.signature.ed25519Sig);
      const verify = await verifyClassification(result, makeOptions());
      expect(verify.valid).toBe(false);
      if (!verify.valid) {
        expect(verify.code).toBe('SIGNATURE_INVALID');
        expect(verify.reason.toLowerCase()).toMatch(/ed25519/);
      }
    });

    it('rejects a flipped ml-dsa half and reports the ml-dsa rejection', async () => {
      const result = await signResult(basePayload());
      result.signature.mldsaSig = flipFirstByte(result.signature.mldsaSig);
      const verify = await verifyClassification(result, makeOptions());
      expect(verify.valid).toBe(false);
      if (!verify.valid) {
        expect(verify.code).toBe('SIGNATURE_INVALID');
        expect(verify.reason).toMatch(/ML-DSA-44|mldsa/i);
      }
    });

    it('rejects a signature produced with a different guard key', async () => {
      const otherEdPriv = new Uint8Array(32).fill(0x05);
      const otherMldsa = ml_dsa44.keygen(new Uint8Array(32).fill(0x06));
      const payload = basePayload();
      const canonical = canonicalizeGuardResultPayload(
        payload as unknown as Record<string, unknown>,
      );
      const edSig = await ed25519.signAsync(canonical, otherEdPriv);
      const mldsaSig = ml_dsa44.sign(otherMldsa.secretKey, canonical);
      const result: NanoMindGuardResult = {
        ...payload,
        signature: {
          alg: GUARD_SIGNATURE_ALGORITHM,
          ed25519Sig: toBase64(edSig),
          mldsaSig: toBase64(mldsaSig),
          ts: payload.timestamp,
        },
      };
      const verify = await verifyClassification(result, makeOptions());
      expect(verify.valid).toBe(false);
      if (!verify.valid) expect(verify.code).toBe('SIGNATURE_INVALID');
    });
  });

  describe('freshness', () => {
    it('accepts a signature at exactly the default max age', async () => {
      const result = await signResult(basePayload());
      const verify = await verifyClassification(
        result,
        makeOptions({ now: () => FIXED_NOW + DEFAULT_MAX_AGE_MS }),
      );
      expect(verify.valid).toBe(true);
    });

    it('rejects a signature one millisecond past the default max age', async () => {
      const result = await signResult(basePayload());
      const verify = await verifyClassification(
        result,
        makeOptions({ now: () => FIXED_NOW + DEFAULT_MAX_AGE_MS + 1 }),
      );
      expect(verify.valid).toBe(false);
      if (!verify.valid) expect(verify.code).toBe('STALE');
    });

    it('rejects a signature more than futureSkewMs in the future', async () => {
      const result = await signResult(basePayload());
      const verify = await verifyClassification(
        result,
        makeOptions({ now: () => FIXED_NOW - DEFAULT_FUTURE_SKEW_MS - 1 }),
      );
      expect(verify.valid).toBe(false);
      if (!verify.valid) expect(verify.code).toBe('FUTURE_DATED');
    });

    it('tolerates a signature within the default future skew window', async () => {
      const result = await signResult(basePayload());
      const verify = await verifyClassification(
        result,
        makeOptions({ now: () => FIXED_NOW - DEFAULT_FUTURE_SKEW_MS + 1 }),
      );
      expect(verify.valid).toBe(true);
    });

    it('freshness is evaluated AFTER signature verification', async () => {
      // Tampered signature on a stale timestamp should still fail as
      // SIGNATURE_INVALID, not STALE. This is the ordering invariant: an
      // attacker who fiddles with the timestamp cannot distinguish
      // "signature broken" from "timestamp too old".
      const result = await signResult(basePayload());
      result.signature.ed25519Sig = '0'.repeat(
        result.signature.ed25519Sig.length,
      );
      const verify = await verifyClassification(
        result,
        makeOptions({ now: () => FIXED_NOW + DEFAULT_MAX_AGE_MS + 1_000_000 }),
      );
      expect(verify.valid).toBe(false);
      if (!verify.valid) {
        // The key-format check catches the zeroed signature bytes as
        // KEY_FORMAT_ERROR (length OK, but the padding is wrong). Either
        // KEY_FORMAT_ERROR or SIGNATURE_INVALID is acceptable here; STALE
        // is not, because freshness must never run before the signature
        // path has rejected the payload.
        expect(verify.code).not.toBe('STALE');
        expect(['KEY_FORMAT_ERROR', 'SIGNATURE_INVALID']).toContain(verify.code);
      }
    });
  });

  describe('tier rejection matrix', () => {
    it('absolute deny: credential-access rejects at privileged tier', async () => {
      const result = await signResult(
        basePayload({ classification: 'credential-access' }),
      );
      const verify = await verifyClassification(
        result,
        makeOptions({ manifest: makeManifest('privileged') }),
      );
      expect(verify.valid).toBe(false);
      if (!verify.valid) expect(verify.code).toBe('ABSOLUTE_DENY');
    });

    it('absolute deny: credential-exfiltration rejects at privileged tier', async () => {
      const result = await signResult(
        basePayload({ classification: 'credential-exfiltration' }),
      );
      const verify = await verifyClassification(
        result,
        makeOptions({ manifest: makeManifest('privileged') }),
      );
      expect(verify.valid).toBe(false);
      if (!verify.valid) expect(verify.code).toBe('ABSOLUTE_DENY');
    });

    it('unknown classification rejects as UNKNOWN_CLASSIFICATION (parse-to-deny)', async () => {
      const result = await signResult(
        basePayload({ classification: 'hypothetical-new-class' }),
      );
      const verify = await verifyClassification(
        result,
        makeOptions({ manifest: makeManifest('privileged') }),
      );
      expect(verify.valid).toBe(false);
      if (!verify.valid) expect(verify.code).toBe('UNKNOWN_CLASSIFICATION');
    });

    it('minimal tier rejects code-generation with TIER_REJECTED', async () => {
      const result = await signResult(basePayload()); // code-generation
      const verify = await verifyClassification(
        result,
        makeOptions({ manifest: makeManifest('minimal') }),
      );
      expect(verify.valid).toBe(false);
      if (!verify.valid) expect(verify.code).toBe('TIER_REJECTED');
    });

    it('read tier rejects data-write', async () => {
      const result = await signResult(
        basePayload({ classification: 'data-write' }),
      );
      const verify = await verifyClassification(
        result,
        makeOptions({ manifest: makeManifest('read') }),
      );
      expect(verify.valid).toBe(false);
      if (!verify.valid) expect(verify.code).toBe('TIER_REJECTED');
    });

    it('execute tier rejects system-mutation', async () => {
      const result = await signResult(
        basePayload({ classification: 'system-mutation' }),
      );
      const verify = await verifyClassification(
        result,
        makeOptions({ manifest: makeManifest('execute') }),
      );
      expect(verify.valid).toBe(false);
      if (!verify.valid) expect(verify.code).toBe('TIER_REJECTED');
    });

    it('mutate tier rejects privileged-operation', async () => {
      const result = await signResult(
        basePayload({ classification: 'privileged-operation' }),
      );
      const verify = await verifyClassification(
        result,
        makeOptions({ manifest: makeManifest('mutate') }),
      );
      expect(verify.valid).toBe(false);
      if (!verify.valid) expect(verify.code).toBe('TIER_REJECTED');
    });

    it('absolute deny set contains the expected classes', () => {
      expect(ABSOLUTE_DENY_CLASSES.has('credential-access')).toBe(true);
      expect(ABSOLUTE_DENY_CLASSES.has('credential-exfiltration')).toBe(true);
      expect(ABSOLUTE_DENY_CLASSES.has('documentation')).toBe(false);
    });

    it('tier registry contains all expected tier floors', () => {
      expect(CLASSIFICATION_MIN_TIER['documentation']).toBe('minimal');
      expect(CLASSIFICATION_MIN_TIER['code-generation']).toBe('execute');
      expect(CLASSIFICATION_MIN_TIER['system-mutation']).toBe('privileged');
    });
  });

  describe('applyVerifiedClassification', () => {
    it('writes event.data.classification on success', async () => {
      const result = await signResult(basePayload());
      const data: Record<string, unknown> = {};
      const verify = await applyVerifiedClassification(
        data,
        result,
        makeOptions(),
      );
      expect(verify.valid).toBe(true);
      expect(data.classification).toBe('code-generation');
    });

    it('does NOT write event.data.classification on failure', async () => {
      const result = await signResult(basePayload());
      result.classification = 'documentation'; // tamper
      const data: Record<string, unknown> = { previous: 'x' };
      const verify = await applyVerifiedClassification(
        data,
        result,
        makeOptions(),
      );
      expect(verify.valid).toBe(false);
      expect(data.classification).toBeUndefined();
      // Other fields are untouched.
      expect(data.previous).toBe('x');
    });

    it('does NOT overwrite a pre-existing classification on failure', async () => {
      const result = await signResult(basePayload({ classification: 'credential-access' }));
      const data: Record<string, unknown> = { classification: 'previously-set' };
      const verify = await applyVerifiedClassification(
        data,
        result,
        makeOptions({ manifest: makeManifest('privileged') }),
      );
      expect(verify.valid).toBe(false);
      expect(data.classification).toBe('previously-set');
    });
  });

  describe('producer -> coordinator end-to-end loop', () => {
    let tmpDir: string;

    beforeEach(() => {
      tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'verify-class-loop-'));
    });

    afterEach(() => {
      fs.rmSync(tmpDir, { recursive: true, force: true });
    });

    function arpConfig(): ARPConfig {
      return {
        agentName: 'verify-classification-loop',
        intelligence: { enabled: false },
      };
    }

    function freshEvent(): ARPEvent {
      return {
        id: `evt-${Math.random().toString(36).slice(2)}`,
        timestamp: new Date().toISOString(),
        source: 'prompt',
        category: 'normal',
        severity: 'info',
        description: 'end-to-end loop test event',
        data: {},
        classifiedBy: 'L0-rules',
      };
    }

    it('verified + manifest-permitted classification passes the coordinator gate', async () => {
      // Manifest at 'execute' tier permits 'code-generation' in both the
      // rejection matrix and the per-manifest envelope. enforce=true so the
      // active comply gate is exercised (the default is detection-only).
      const manifest = makeManifest('execute', { enforce: true });
      const coord = new IntelligenceCoordinator(arpConfig(), tmpDir, manifest);

      const result = await signResult(basePayload());
      const event = freshEvent();
      const verify = await applyVerifiedClassification(
        event.data,
        result,
        makeOptions({ manifest }),
      );
      expect(verify.valid).toBe(true);

      // The coordinator should now pass the event through unchanged.
      const assessment = await coord.analyze(event);
      expect(assessment).toBeNull();
      // Comply gate did not fire (event passed the envelope).
      expect(event.classifiedBy).not.toBe('L0-comply');
      expect(event.data.comply).toBeUndefined();
      expect(event.data.classification).toBe('code-generation');
    });

    it('verifier passes but coordinator comply gate denies an unknown-to-manifest class', async () => {
      // The tier ceiling permits 'network-egress' at execute tier, but the
      // per-manifest envelope only lists code-generation and documentation.
      // The second layer (coordinator) is where the envelope is enforced.
      // permitted: code-gen, docs; enforce=true to exercise the active gate.
      const manifest = makeManifest('execute', { enforce: true });
      const coord = new IntelligenceCoordinator(arpConfig(), tmpDir, manifest);

      const result = await signResult(
        basePayload({ classification: 'network-egress' }),
      );
      const event = freshEvent();
      const verify = await applyVerifiedClassification(
        event.data,
        result,
        makeOptions({ manifest }),
      );
      expect(verify.valid).toBe(true);
      expect(event.data.classification).toBe('network-egress');

      const assessment = await coord.analyze(event);
      expect(assessment).toBeNull();
      // L0-comply fired with reason=unknown (parse-to-deny on the
      // per-manifest envelope), even though the class passed the tier
      // ceiling.
      expect(event.classifiedBy).toBe('L0-comply');
      const decision = event.data.comply as {
        reason: string;
        classification: string;
      };
      expect(decision.reason).toBe('unknown');
      expect(decision.classification).toBe('network-egress');
    });

    it('verifier rejection means coordinator never sees a classification', async () => {
      const manifest = makeManifest('minimal');
      const coord = new IntelligenceCoordinator(arpConfig(), tmpDir, manifest);

      // Minimal tier rejects code-generation at the verifier stage.
      const result = await signResult(basePayload());
      const event = freshEvent();
      const verify = await applyVerifiedClassification(
        event.data,
        result,
        makeOptions({ manifest }),
      );
      expect(verify.valid).toBe(false);
      // event.data.classification was never written, so the coordinator
      // sees an unclassified event and its comply gate is inert.
      expect(event.data.classification).toBeUndefined();

      await coord.analyze(event);
      expect(event.classifiedBy).not.toBe('L0-comply');
      expect(event.data.comply).toBeUndefined();
    });
  });

  describe('canonicalization', () => {
    it('is stable under key reordering', () => {
      const a = canonicalizeGuardResultPayload({
        b: 2,
        a: 1,
        nested: { y: 'z', x: 'w' },
      });
      const b = canonicalizeGuardResultPayload({
        nested: { x: 'w', y: 'z' },
        a: 1,
        b: 2,
      });
      expect(Buffer.from(a).toString('utf8')).toBe(
        Buffer.from(b).toString('utf8'),
      );
    });

    it('strips the signature field before serializing', () => {
      const withSig = canonicalizeGuardResultPayload({
        a: 1,
        signature: { alg: 'anything' },
      });
      const withoutSig = canonicalizeGuardResultPayload({ a: 1 });
      expect(Buffer.from(withSig).toString('utf8')).toBe(
        Buffer.from(withoutSig).toString('utf8'),
      );
    });
  });
});
