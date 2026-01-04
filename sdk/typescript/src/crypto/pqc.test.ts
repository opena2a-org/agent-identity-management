/**
 * Tests for Post-Quantum Cryptography module
 */

import { describe, it, expect, beforeAll, beforeEach } from 'vitest';
import {
  isPQCAvailable,
  generateMLDSAKeyPair,
  signMLDSA,
  verifyMLDSA,
  generateHybridKeyPair,
  hybridSign,
  hybridVerify,
  getHybridPublicKey,
  encodeHybridSignature,
  decodeHybridSignature,
  encodeHybridPublicKey,
  decodeHybridPublicKey,
  getMLDSAVariant,
  isHybridAlgorithm,
  isPQCAlgorithm,
  validateKeySize,
  KEY_SIZES,
  type Algorithm,
} from './pqc';

describe('PQC Module', () => {
  let pqcAvailable: boolean;

  beforeAll(async () => {
    pqcAvailable = await isPQCAvailable();
    if (!pqcAvailable) {
      console.log('Note: @noble/post-quantum not installed, some tests will be skipped');
    }
  });

  describe('Algorithm Helpers (no PQC dependency)', () => {
    it('should get ML-DSA variant from algorithm', () => {
      expect(getMLDSAVariant('ML-DSA-44')).toBe('ML-DSA-44');
      expect(getMLDSAVariant('ML-DSA-65')).toBe('ML-DSA-65');
      expect(getMLDSAVariant('ML-DSA-87')).toBe('ML-DSA-87');
      expect(getMLDSAVariant('Ed25519+ML-DSA-44')).toBe('ML-DSA-44');
      expect(getMLDSAVariant('Ed25519+ML-DSA-65')).toBe('ML-DSA-65');
      expect(getMLDSAVariant('Ed25519+ML-DSA-87')).toBe('ML-DSA-87');
      expect(getMLDSAVariant('Ed25519' as Algorithm)).toBe('ML-DSA-65'); // Default
    });

    it('should identify hybrid algorithms', () => {
      expect(isHybridAlgorithm('Ed25519+ML-DSA-44')).toBe(true);
      expect(isHybridAlgorithm('Ed25519+ML-DSA-65')).toBe(true);
      expect(isHybridAlgorithm('Ed25519+ML-DSA-87')).toBe(true);
      expect(isHybridAlgorithm('Ed25519')).toBe(false);
      expect(isHybridAlgorithm('ML-DSA-65')).toBe(false);
    });

    it('should identify PQC algorithms', () => {
      expect(isPQCAlgorithm('ML-DSA-44')).toBe(true);
      expect(isPQCAlgorithm('ML-DSA-65')).toBe(true);
      expect(isPQCAlgorithm('ML-DSA-87')).toBe(true);
      expect(isPQCAlgorithm('Ed25519+ML-DSA-44')).toBe(true);
      expect(isPQCAlgorithm('Ed25519+ML-DSA-65')).toBe(true);
      expect(isPQCAlgorithm('Ed25519+ML-DSA-87')).toBe(true);
      expect(isPQCAlgorithm('Ed25519')).toBe(false);
    });

    it('should have correct key sizes defined', () => {
      expect(KEY_SIZES['ML-DSA-44'].publicKey).toBe(1312);
      expect(KEY_SIZES['ML-DSA-44'].privateKey).toBe(2560);
      expect(KEY_SIZES['ML-DSA-44'].signature).toBe(2420);

      expect(KEY_SIZES['ML-DSA-65'].publicKey).toBe(1952);
      expect(KEY_SIZES['ML-DSA-65'].privateKey).toBe(4032);
      expect(KEY_SIZES['ML-DSA-65'].signature).toBe(3309);

      expect(KEY_SIZES['ML-DSA-87'].publicKey).toBe(2592);
      expect(KEY_SIZES['ML-DSA-87'].privateKey).toBe(4896);
      expect(KEY_SIZES['ML-DSA-87'].signature).toBe(4627);

      expect(KEY_SIZES.Ed25519.publicKey).toBe(32);
      expect(KEY_SIZES.Ed25519.privateKey).toBe(32); // Seed size in @noble/ed25519
      expect(KEY_SIZES.Ed25519.signature).toBe(64);
    });
  });

  describe('ML-DSA Key Generation', () => {
    it('should generate ML-DSA-44 key pair', async () => {
      if (!pqcAvailable) return;

      const keyPair = await generateMLDSAKeyPair('ML-DSA-44');

      expect(keyPair.algorithm).toBe('ML-DSA-44');
      expect(keyPair.publicKey.length).toBe(KEY_SIZES['ML-DSA-44'].publicKey);
      expect(keyPair.privateKey.length).toBe(KEY_SIZES['ML-DSA-44'].privateKey);
      expect(keyPair.createdAt).toBeInstanceOf(Date);
    });

    it('should generate ML-DSA-65 key pair (default)', async () => {
      if (!pqcAvailable) return;

      const keyPair = await generateMLDSAKeyPair();

      expect(keyPair.algorithm).toBe('ML-DSA-65');
      expect(keyPair.publicKey.length).toBe(KEY_SIZES['ML-DSA-65'].publicKey);
      expect(keyPair.privateKey.length).toBe(KEY_SIZES['ML-DSA-65'].privateKey);
    });

    it('should generate ML-DSA-87 key pair', async () => {
      if (!pqcAvailable) return;

      const keyPair = await generateMLDSAKeyPair('ML-DSA-87');

      expect(keyPair.algorithm).toBe('ML-DSA-87');
      expect(keyPair.publicKey.length).toBe(KEY_SIZES['ML-DSA-87'].publicKey);
      expect(keyPair.privateKey.length).toBe(KEY_SIZES['ML-DSA-87'].privateKey);
    });

    it('should generate unique key pairs', async () => {
      if (!pqcAvailable) return;

      const keyPair1 = await generateMLDSAKeyPair('ML-DSA-65');
      const keyPair2 = await generateMLDSAKeyPair('ML-DSA-65');

      expect(keyPair1.publicKey).not.toEqual(keyPair2.publicKey);
      expect(keyPair1.privateKey).not.toEqual(keyPair2.privateKey);
    });
  });

  describe('ML-DSA Sign/Verify', () => {
    it('should sign and verify a message with ML-DSA-65', async () => {
      if (!pqcAvailable) return;

      const keyPair = await generateMLDSAKeyPair('ML-DSA-65');
      const message = new TextEncoder().encode('Test message for ML-DSA signature');

      const signature = await signMLDSA('ML-DSA-65', keyPair.privateKey, message);
      expect(signature.length).toBe(KEY_SIZES['ML-DSA-65'].signature);

      const isValid = await verifyMLDSA('ML-DSA-65', keyPair.publicKey, message, signature);
      expect(isValid).toBe(true);
    });

    it('should sign and verify with all ML-DSA variants', async () => {
      if (!pqcAvailable) return;

      const variants: Array<'ML-DSA-44' | 'ML-DSA-65' | 'ML-DSA-87'> = [
        'ML-DSA-44',
        'ML-DSA-65',
        'ML-DSA-87',
      ];

      for (const variant of variants) {
        const keyPair = await generateMLDSAKeyPair(variant);
        const message = new TextEncoder().encode(`Test message for ${variant}`);

        const signature = await signMLDSA(variant, keyPair.privateKey, message);
        expect(signature.length).toBe(KEY_SIZES[variant].signature);

        const isValid = await verifyMLDSA(variant, keyPair.publicKey, message, signature);
        expect(isValid).toBe(true);
      }
    });

    it('should fail verification for tampered message', async () => {
      if (!pqcAvailable) return;

      const keyPair = await generateMLDSAKeyPair('ML-DSA-65');
      const message = new TextEncoder().encode('Original message');
      const tamperedMessage = new TextEncoder().encode('Tampered message');

      const signature = await signMLDSA('ML-DSA-65', keyPair.privateKey, message);

      const isValid = await verifyMLDSA('ML-DSA-65', keyPair.publicKey, tamperedMessage, signature);
      expect(isValid).toBe(false);
    });

    it('should fail verification for tampered signature', async () => {
      if (!pqcAvailable) return;

      const keyPair = await generateMLDSAKeyPair('ML-DSA-65');
      const message = new TextEncoder().encode('Test message');

      const signature = await signMLDSA('ML-DSA-65', keyPair.privateKey, message);
      const tamperedSig = new Uint8Array(signature);
      tamperedSig[0] ^= 0x01;

      const isValid = await verifyMLDSA('ML-DSA-65', keyPair.publicKey, message, tamperedSig);
      expect(isValid).toBe(false);
    });

    it('should fail verification with wrong key', async () => {
      if (!pqcAvailable) return;

      const keyPair1 = await generateMLDSAKeyPair('ML-DSA-65');
      const keyPair2 = await generateMLDSAKeyPair('ML-DSA-65');
      const message = new TextEncoder().encode('Test message');

      const signature = await signMLDSA('ML-DSA-65', keyPair1.privateKey, message);

      const isValid = await verifyMLDSA('ML-DSA-65', keyPair2.publicKey, message, signature);
      expect(isValid).toBe(false);
    });
  });

  describe('Hybrid Key Generation', () => {
    it('should generate hybrid Ed25519+ML-DSA-65 key pair', async () => {
      if (!pqcAvailable) return;

      const keyPair = await generateHybridKeyPair('ML-DSA-65');

      expect(keyPair.algorithm).toBe('Ed25519+ML-DSA-65');
      expect(keyPair.ed25519.publicKey.length).toBe(KEY_SIZES.Ed25519.publicKey);
      expect(keyPair.ed25519.privateKey.length).toBe(KEY_SIZES.Ed25519.privateKey);
      expect(keyPair.mldsa.publicKey.length).toBe(KEY_SIZES['ML-DSA-65'].publicKey);
      expect(keyPair.mldsa.privateKey.length).toBe(KEY_SIZES['ML-DSA-65'].privateKey);
      expect(keyPair.createdAt).toBeInstanceOf(Date);
    });

    it('should generate hybrid keys for all variants', async () => {
      if (!pqcAvailable) return;

      const variants: Array<'ML-DSA-44' | 'ML-DSA-65' | 'ML-DSA-87'> = [
        'ML-DSA-44',
        'ML-DSA-65',
        'ML-DSA-87',
      ];

      for (const variant of variants) {
        const keyPair = await generateHybridKeyPair(variant);
        expect(keyPair.algorithm).toBe(`Ed25519+${variant}`);
        expect(keyPair.mldsa.algorithm).toBe(variant);
      }
    });
  });

  describe('Hybrid Sign/Verify', () => {
    it('should sign and verify with hybrid keys', async () => {
      if (!pqcAvailable) return;

      const keyPair = await generateHybridKeyPair('ML-DSA-65');
      const message = new TextEncoder().encode('Test message for hybrid signature');

      const signature = await hybridSign(keyPair, message);
      expect(signature.algorithm).toBe('Ed25519+ML-DSA-65');
      expect(signature.ed25519Sig.length).toBe(KEY_SIZES.Ed25519.signature);
      expect(signature.mldsaSig.length).toBe(KEY_SIZES['ML-DSA-65'].signature);
      expect(signature.timestamp).toBeGreaterThan(0);

      const publicKey = getHybridPublicKey(keyPair);
      const isValid = await hybridVerify(publicKey, message, signature);
      expect(isValid).toBe(true);
    });

    it('should fail if Ed25519 signature is invalid', async () => {
      if (!pqcAvailable) return;

      const keyPair = await generateHybridKeyPair('ML-DSA-65');
      const message = new TextEncoder().encode('Test message');

      const signature = await hybridSign(keyPair, message);

      // Tamper with Ed25519 signature
      const tamperedSig = { ...signature };
      tamperedSig.ed25519Sig = new Uint8Array(signature.ed25519Sig);
      tamperedSig.ed25519Sig[0] ^= 0x01;

      const publicKey = getHybridPublicKey(keyPair);
      const isValid = await hybridVerify(publicKey, message, tamperedSig);
      expect(isValid).toBe(false);
    });

    it('should fail if ML-DSA signature is invalid', async () => {
      if (!pqcAvailable) return;

      const keyPair = await generateHybridKeyPair('ML-DSA-65');
      const message = new TextEncoder().encode('Test message');

      const signature = await hybridSign(keyPair, message);

      // Tamper with ML-DSA signature
      const tamperedSig = { ...signature };
      tamperedSig.mldsaSig = new Uint8Array(signature.mldsaSig);
      tamperedSig.mldsaSig[0] ^= 0x01;

      const publicKey = getHybridPublicKey(keyPair);
      const isValid = await hybridVerify(publicKey, message, tamperedSig);
      expect(isValid).toBe(false);
    });

    it('should require BOTH signatures to be valid', async () => {
      if (!pqcAvailable) return;

      const keyPair = await generateHybridKeyPair('ML-DSA-65');
      const message = new TextEncoder().encode('Test message');

      const signature = await hybridSign(keyPair, message);

      // Tamper with both signatures
      const tamperedSig = { ...signature };
      tamperedSig.ed25519Sig = new Uint8Array(signature.ed25519Sig);
      tamperedSig.ed25519Sig[0] ^= 0x01;
      tamperedSig.mldsaSig = new Uint8Array(signature.mldsaSig);
      tamperedSig.mldsaSig[0] ^= 0x01;

      const publicKey = getHybridPublicKey(keyPair);
      const isValid = await hybridVerify(publicKey, message, tamperedSig);
      expect(isValid).toBe(false);
    });

    it('should fail with wrong public keys', async () => {
      if (!pqcAvailable) return;

      const keyPair1 = await generateHybridKeyPair('ML-DSA-65');
      const keyPair2 = await generateHybridKeyPair('ML-DSA-65');
      const message = new TextEncoder().encode('Test message');

      const signature = await hybridSign(keyPair1, message);
      const publicKey = getHybridPublicKey(keyPair2);

      const isValid = await hybridVerify(publicKey, message, signature);
      expect(isValid).toBe(false);
    });
  });

  describe('Encoding/Decoding', () => {
    it('should encode and decode hybrid signature', async () => {
      if (!pqcAvailable) return;

      const keyPair = await generateHybridKeyPair('ML-DSA-65');
      const message = new TextEncoder().encode('Test message');

      const signature = await hybridSign(keyPair, message);
      const encoded = encodeHybridSignature(signature);

      expect(encoded.alg).toBe(signature.algorithm);
      expect(typeof encoded.ed25519Sig).toBe('string');
      expect(typeof encoded.mldsaSig).toBe('string');
      expect(encoded.ts).toBe(signature.timestamp);

      const decoded = decodeHybridSignature(encoded);
      expect(decoded.algorithm).toBe(signature.algorithm);
      expect(decoded.ed25519Sig).toEqual(signature.ed25519Sig);
      expect(decoded.mldsaSig).toEqual(signature.mldsaSig);
      expect(decoded.timestamp).toBe(signature.timestamp);

      // Verify decoded signature works
      const publicKey = getHybridPublicKey(keyPair);
      const isValid = await hybridVerify(publicKey, message, decoded);
      expect(isValid).toBe(true);
    });

    it('should encode and decode hybrid public key', async () => {
      if (!pqcAvailable) return;

      const keyPair = await generateHybridKeyPair('ML-DSA-65');
      const publicKey = getHybridPublicKey(keyPair);

      const encoded = encodeHybridPublicKey(publicKey);
      expect(encoded.algorithm).toBe('Ed25519+ML-DSA-65');
      expect(typeof encoded.ed25519PublicKey).toBe('string');
      expect(typeof encoded.mldsaPublicKey).toBe('string');
      expect(encoded.mldsaVariant).toBe('ML-DSA-65');

      const decoded = decodeHybridPublicKey(encoded);
      expect(decoded.ed25519Pub).toEqual(publicKey.ed25519Pub);
      expect(decoded.mldsaPub).toEqual(publicKey.mldsaPub);
      expect(decoded.mldsaVariant).toBe(publicKey.mldsaVariant);
    });
  });

  describe('Key Size Validation', () => {
    it('should validate key sizes', async () => {
      if (!pqcAvailable) return;

      const keyPair = await generateMLDSAKeyPair('ML-DSA-65');

      expect(validateKeySize('ML-DSA-65', 'publicKey', keyPair.publicKey)).toBe(true);
      expect(validateKeySize('ML-DSA-65', 'privateKey', keyPair.privateKey)).toBe(true);

      // Wrong size should fail
      expect(validateKeySize('ML-DSA-65', 'publicKey', new Uint8Array(10))).toBe(false);
      expect(validateKeySize('ML-DSA-65', 'privateKey', new Uint8Array(10))).toBe(false);
    });
  });

  describe('Large Message Handling', () => {
    it('should sign and verify large messages', async () => {
      if (!pqcAvailable) return;

      const keyPair = await generateHybridKeyPair('ML-DSA-65');
      const largeMessage = new Uint8Array(64 * 1024); // 64KB (under crypto limit)
      crypto.getRandomValues(largeMessage);

      const signature = await hybridSign(keyPair, largeMessage);
      const publicKey = getHybridPublicKey(keyPair);
      const isValid = await hybridVerify(publicKey, largeMessage, signature);

      expect(isValid).toBe(true);
    });

    it('should sign and verify empty message', async () => {
      if (!pqcAvailable) return;

      const keyPair = await generateHybridKeyPair('ML-DSA-65');
      const emptyMessage = new Uint8Array(0);

      const signature = await hybridSign(keyPair, emptyMessage);
      const publicKey = getHybridPublicKey(keyPair);
      const isValid = await hybridVerify(publicKey, emptyMessage, signature);

      expect(isValid).toBe(true);
    });
  });

  describe('Interoperability', () => {
    it('should verify signatures correctly', async () => {
      if (!pqcAvailable) return;

      const keyPair = await generateHybridKeyPair('ML-DSA-65');
      const message = new TextEncoder().encode('Same message');

      const sig1 = await hybridSign(keyPair, message);
      const sig2 = await hybridSign(keyPair, message);

      // Both should verify correctly
      const publicKey = getHybridPublicKey(keyPair);
      expect(await hybridVerify(publicKey, message, sig1)).toBe(true);
      expect(await hybridVerify(publicKey, message, sig2)).toBe(true);
    });
  });
});
