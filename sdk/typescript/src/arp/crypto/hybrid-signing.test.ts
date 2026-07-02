/**
 * Round-trip tests for hybrid Ed25519 + ML-DSA signing.
 *
 * Coverage:
 * - Key generation for all three ML-DSA variants produces FIPS 204 sizes
 * - Sign/verify round trip succeeds
 * - Tampered payload, ed25519 half, and ml-dsa half all reject
 * - Non-short-circuit: a broken ed25519 half does not skip ml-dsa verification
 *   (the `ed25519Valid`/`mldsaValid` breakdown reflects both halves being
 *   evaluated on every call, which matters for side-channel resistance)
 * - Encode/decode round trip for transport shapes
 * - Algorithm mismatch between signature and public key is rejected
 */

import { describe, it, expect } from 'vitest';
import {
  HybridCryptoError,
  KEY_SIZES,
  decodeHybridPublicKey,
  decodeHybridSignature,
  encodeHybridPublicKey,
  encodeHybridSignature,
  generateHybridKeyPair,
  getHybridPublicKey,
  hybridAlgorithmFor,
  hybridSign,
  hybridVerify,
  validateKeySize,
} from './index';
import type { HybridPublicKey, HybridSignature, MLDsaVariant } from './index';

const VARIANTS: MLDsaVariant[] = ['ML-DSA-44', 'ML-DSA-65', 'ML-DSA-87'];

describe('arp/crypto hybrid-signing', () => {
  describe('hybridAlgorithmFor', () => {
    it('maps every ML-DSA variant to its canonical algorithm string', () => {
      expect(hybridAlgorithmFor('ML-DSA-44')).toBe('Ed25519+ML-DSA-44');
      expect(hybridAlgorithmFor('ML-DSA-65')).toBe('Ed25519+ML-DSA-65');
      expect(hybridAlgorithmFor('ML-DSA-87')).toBe('Ed25519+ML-DSA-87');
    });
  });

  describe('generateHybridKeyPair', () => {
    for (const variant of VARIANTS) {
      it(`produces FIPS 204 / Ed25519 key sizes for ${variant}`, async () => {
        const keys = await generateHybridKeyPair(variant);

        expect(keys.algorithm).toBe(hybridAlgorithmFor(variant));
        expect(keys.mldsa.variant).toBe(variant);
        expect(keys.createdAt).toBeInstanceOf(Date);

        expect(keys.ed25519.publicKey.length).toBe(KEY_SIZES.Ed25519.publicKey);
        expect(keys.ed25519.privateKey.length).toBe(KEY_SIZES.Ed25519.privateKey);
        expect(keys.mldsa.publicKey.length).toBe(KEY_SIZES[variant].publicKey);
        expect(keys.mldsa.privateKey.length).toBe(KEY_SIZES[variant].privateKey);
      });
    }
  });

  describe('hybridSign and hybridVerify round trip', () => {
    for (const variant of VARIANTS) {
      it(`signs and verifies a payload with ${variant}`, async () => {
        const keys = await generateHybridKeyPair(variant);
        const pub = getHybridPublicKey(keys);
        const payload = new TextEncoder().encode(
          `round-trip test payload for ${variant}`,
        );

        const sig = await hybridSign(payload, keys);

        expect(sig.algorithm).toBe(hybridAlgorithmFor(variant));
        expect(sig.ed25519Sig.length).toBe(KEY_SIZES.Ed25519.signature);
        expect(sig.mldsaSig.length).toBe(KEY_SIZES[variant].signature);

        const result = await hybridVerify(payload, sig, pub);
        expect(result.valid).toBe(true);
        expect(result.ed25519Valid).toBe(true);
        expect(result.mldsaValid).toBe(true);
        expect(result.reason).toBeUndefined();
      });
    }

    it('rejects a tampered payload', async () => {
      const keys = await generateHybridKeyPair('ML-DSA-65');
      const pub = getHybridPublicKey(keys);
      const payload = new TextEncoder().encode('original payload');
      const sig = await hybridSign(payload, keys);

      const tampered = new TextEncoder().encode('tampered payload');
      const result = await hybridVerify(tampered, sig, pub);

      expect(result.valid).toBe(false);
      expect(result.ed25519Valid).toBe(false);
      expect(result.mldsaValid).toBe(false);
      expect(result.reason).toBeDefined();
    });

    it('rejects a tampered ed25519 half while ml-dsa half still signed the original', async () => {
      const keys = await generateHybridKeyPair('ML-DSA-65');
      const pub = getHybridPublicKey(keys);
      const payload = new TextEncoder().encode('payload');
      const sig = await hybridSign(payload, keys);

      // Flip one byte of the ed25519 signature.
      const broken: HybridSignature = {
        ...sig,
        ed25519Sig: new Uint8Array(sig.ed25519Sig),
      };
      broken.ed25519Sig[0] ^= 0xff;

      const result = await hybridVerify(payload, broken, pub);
      expect(result.valid).toBe(false);
      expect(result.ed25519Valid).toBe(false);
      // Non-short-circuit property: ml-dsa verification still ran and saw a
      // valid signature for the original payload. This is the visible proof
      // that hybridVerify evaluates both halves on every call.
      expect(result.mldsaValid).toBe(true);
    });

    it('rejects a tampered ml-dsa half while ed25519 half still signed the original', async () => {
      const keys = await generateHybridKeyPair('ML-DSA-65');
      const pub = getHybridPublicKey(keys);
      const payload = new TextEncoder().encode('payload');
      const sig = await hybridSign(payload, keys);

      const broken: HybridSignature = {
        ...sig,
        mldsaSig: new Uint8Array(sig.mldsaSig),
      };
      broken.mldsaSig[0] ^= 0xff;

      const result = await hybridVerify(payload, broken, pub);
      expect(result.valid).toBe(false);
      expect(result.ed25519Valid).toBe(true);
      expect(result.mldsaValid).toBe(false);
    });

    it('rejects an algorithm mismatch between signature and public key', async () => {
      const a = await generateHybridKeyPair('ML-DSA-44');
      const b = await generateHybridKeyPair('ML-DSA-65');
      const payload = new TextEncoder().encode('x');
      const sigA = await hybridSign(payload, a);
      const pubB = getHybridPublicKey(b);

      const result = await hybridVerify(payload, sigA, pubB);
      expect(result.valid).toBe(false);
      expect(result.ed25519Valid).toBe(false);
      expect(result.mldsaValid).toBe(false);
      expect(result.reason).toMatch(/algorithm mismatch/);
    });

    it('rejects a signature verified against a different key', async () => {
      const signer = await generateHybridKeyPair('ML-DSA-65');
      const attacker = await generateHybridKeyPair('ML-DSA-65');
      const payload = new TextEncoder().encode('payload');

      const sig = await hybridSign(payload, signer);
      const result = await hybridVerify(payload, sig, getHybridPublicKey(attacker));

      expect(result.valid).toBe(false);
      expect(result.ed25519Valid).toBe(false);
      expect(result.mldsaValid).toBe(false);
    });
  });

  describe('non-short-circuit hybridVerify', () => {
    it('evaluates the ml-dsa half even when the ed25519 half is rejected', async () => {
      // The tampered-ed25519 test above already demonstrates this, but repeat
      // the assertion here with explicit framing so grep lands on this file
      // if the non-short-circuit contract is ever questioned.
      const keys = await generateHybridKeyPair('ML-DSA-44');
      const pub = getHybridPublicKey(keys);
      const payload = new Uint8Array([1, 2, 3, 4, 5]);
      const sig = await hybridSign(payload, keys);

      const broken: HybridSignature = {
        ...sig,
        ed25519Sig: new Uint8Array(sig.ed25519Sig),
      };
      broken.ed25519Sig[10] ^= 0xaa;

      const result = await hybridVerify(payload, broken, pub);
      expect(result.ed25519Valid).toBe(false);
      // If hybridVerify short-circuited, mldsaValid would be false (its default).
      // Asserting true is the proof the ml-dsa half was still evaluated.
      expect(result.mldsaValid).toBe(true);
      expect(result.valid).toBe(false);
    });
  });

  describe('transport encoding', () => {
    it('encodes and decodes a hybrid signature without loss', async () => {
      const keys = await generateHybridKeyPair('ML-DSA-65');
      const payload = new TextEncoder().encode('encode test');
      const sig = await hybridSign(payload, keys);

      const encoded = encodeHybridSignature(sig);
      expect(typeof encoded.ed25519Sig).toBe('string');
      expect(typeof encoded.mldsaSig).toBe('string');
      expect(encoded.alg).toBe(sig.algorithm);
      expect(encoded.ts).toBe(sig.timestamp);

      const decoded = decodeHybridSignature(encoded);
      expect(decoded.algorithm).toBe(sig.algorithm);
      expect(decoded.timestamp).toBe(sig.timestamp);
      expect(Array.from(decoded.ed25519Sig)).toEqual(Array.from(sig.ed25519Sig));
      expect(Array.from(decoded.mldsaSig)).toEqual(Array.from(sig.mldsaSig));

      // The decoded signature must still verify.
      const result = await hybridVerify(payload, decoded, getHybridPublicKey(keys));
      expect(result.valid).toBe(true);
    });

    it('encodes and decodes a hybrid public key without loss', async () => {
      const keys = await generateHybridKeyPair('ML-DSA-44');
      const pub = getHybridPublicKey(keys);

      const encoded = encodeHybridPublicKey(pub);
      expect(encoded.algorithm).toBe(pub.algorithm);
      expect(encoded.mldsaVariant).toBe('ML-DSA-44');
      expect(typeof encoded.ed25519PublicKey).toBe('string');
      expect(typeof encoded.mldsaPublicKey).toBe('string');

      const decoded: HybridPublicKey = decodeHybridPublicKey(encoded);
      expect(Array.from(decoded.ed25519PublicKey)).toEqual(
        Array.from(pub.ed25519PublicKey),
      );
      expect(Array.from(decoded.mldsaPublicKey)).toEqual(
        Array.from(pub.mldsaPublicKey),
      );

      // Round-trip a real signature through the decoded public key.
      const payload = new TextEncoder().encode('transport');
      const sig = await hybridSign(payload, keys);
      const result = await hybridVerify(payload, sig, decoded);
      expect(result.valid).toBe(true);
    });
  });

  describe('validateKeySize', () => {
    it('accepts correctly sized keys and signatures', () => {
      expect(
        validateKeySize('Ed25519', 'publicKey', new Uint8Array(32)),
      ).toBe(true);
      expect(
        validateKeySize('ML-DSA-44', 'signature', new Uint8Array(2420)),
      ).toBe(true);
      expect(
        validateKeySize('Ed25519+ML-DSA-87', 'publicKey', new Uint8Array(2592)),
      ).toBe(true);
    });

    it('rejects wrong sizes', () => {
      expect(
        validateKeySize('Ed25519', 'publicKey', new Uint8Array(31)),
      ).toBe(false);
      expect(
        validateKeySize('ML-DSA-65', 'signature', new Uint8Array(3000)),
      ).toBe(false);
    });
  });

  describe('HybridCryptoError', () => {
    it('is raised with a namespaced message', () => {
      const err = new HybridCryptoError('bad thing happened');
      expect(err.name).toBe('HybridCryptoError');
      expect(err.message).toContain('[arp/crypto]');
      expect(err.message).toContain('bad thing happened');
    });
  });
});
