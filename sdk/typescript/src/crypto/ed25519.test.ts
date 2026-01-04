/**
 * Tests for Ed25519 cryptographic operations
 *
 * Tests cover:
 * - Key generation
 * - Signing and verification
 * - Base64 encoding/decoding
 * - Hex encoding/decoding
 * - Request signature creation
 */

import { describe, it, expect } from 'vitest';
import {
  generateKeyPair,
  sign,
  verify,
  toBase64,
  fromBase64,
  toHex,
  fromHex,
  createRequestSignature,
} from './ed25519';

describe('Ed25519 Cryptographic Operations', () => {
  describe('generateKeyPair', () => {
    it('should generate a valid key pair', async () => {
      const keyPair = await generateKeyPair();

      expect(keyPair).toBeDefined();
      expect(keyPair.privateKey).toBeInstanceOf(Uint8Array);
      expect(keyPair.publicKey).toBeInstanceOf(Uint8Array);
    });

    it('should generate 32-byte private key', async () => {
      const keyPair = await generateKeyPair();
      expect(keyPair.privateKey.length).toBe(32);
    });

    it('should generate 32-byte public key', async () => {
      const keyPair = await generateKeyPair();
      expect(keyPair.publicKey.length).toBe(32);
    });

    it('should generate unique key pairs', async () => {
      const keyPair1 = await generateKeyPair();
      const keyPair2 = await generateKeyPair();

      expect(toHex(keyPair1.privateKey)).not.toBe(toHex(keyPair2.privateKey));
      expect(toHex(keyPair1.publicKey)).not.toBe(toHex(keyPair2.publicKey));
    });
  });

  describe('sign and verify', () => {
    it('should sign and verify a message', async () => {
      const keyPair = await generateKeyPair();
      const message = new TextEncoder().encode('Hello, World!');

      const signature = await sign(message, keyPair.privateKey);
      const isValid = await verify(signature, message, keyPair.publicKey);

      expect(isValid).toBe(true);
    });

    it('should produce 64-byte signature', async () => {
      const keyPair = await generateKeyPair();
      const message = new TextEncoder().encode('Test message');

      const signature = await sign(message, keyPair.privateKey);
      expect(signature.length).toBe(64);
    });

    it('should fail verification with wrong public key', async () => {
      const keyPair1 = await generateKeyPair();
      const keyPair2 = await generateKeyPair();
      const message = new TextEncoder().encode('Secret message');

      const signature = await sign(message, keyPair1.privateKey);
      const isValid = await verify(signature, message, keyPair2.publicKey);

      expect(isValid).toBe(false);
    });

    it('should fail verification with tampered message', async () => {
      const keyPair = await generateKeyPair();
      const originalMessage = new TextEncoder().encode('Original');
      const tamperedMessage = new TextEncoder().encode('Tampered');

      const signature = await sign(originalMessage, keyPair.privateKey);
      const isValid = await verify(signature, tamperedMessage, keyPair.publicKey);

      expect(isValid).toBe(false);
    });

    it('should fail verification with corrupted signature', async () => {
      const keyPair = await generateKeyPair();
      const message = new TextEncoder().encode('Test');

      const signature = await sign(message, keyPair.privateKey);
      // Corrupt the signature
      signature[0] = (signature[0] + 1) % 256;

      const isValid = await verify(signature, message, keyPair.publicKey);
      expect(isValid).toBe(false);
    });

    it('should handle empty message', async () => {
      const keyPair = await generateKeyPair();
      const message = new Uint8Array(0);

      const signature = await sign(message, keyPair.privateKey);
      const isValid = await verify(signature, message, keyPair.publicKey);

      expect(isValid).toBe(true);
    });
  });

  describe('toBase64 and fromBase64', () => {
    it('should encode and decode correctly', () => {
      const original = new Uint8Array([1, 2, 3, 4, 5, 255, 0, 128]);

      const encoded = toBase64(original);
      const decoded = fromBase64(encoded);

      expect(decoded).toEqual(original);
    });

    it('should handle empty array', () => {
      const original = new Uint8Array(0);

      const encoded = toBase64(original);
      const decoded = fromBase64(encoded);

      expect(decoded.length).toBe(0);
    });

    it('should produce valid base64 string', () => {
      const data = new Uint8Array([72, 101, 108, 108, 111]); // "Hello"
      const encoded = toBase64(data);

      // Base64 should only contain valid characters
      expect(encoded).toMatch(/^[A-Za-z0-9+/=]*$/);
    });

    it('should encode key pair correctly', async () => {
      const keyPair = await generateKeyPair();

      const encodedPrivate = toBase64(keyPair.privateKey);
      const encodedPublic = toBase64(keyPair.publicKey);

      expect(typeof encodedPrivate).toBe('string');
      expect(typeof encodedPublic).toBe('string');
      expect(encodedPrivate.length).toBeGreaterThan(0);
      expect(encodedPublic.length).toBeGreaterThan(0);
    });

    it('should decode back to original key', async () => {
      const keyPair = await generateKeyPair();

      const encoded = toBase64(keyPair.publicKey);
      const decoded = fromBase64(encoded);

      expect(decoded).toEqual(keyPair.publicKey);
    });
  });

  describe('toHex and fromHex', () => {
    it('should encode and decode correctly', () => {
      const original = new Uint8Array([0, 15, 255, 128, 64]);

      const encoded = toHex(original);
      const decoded = fromHex(encoded);

      expect(decoded).toEqual(original);
    });

    it('should produce lowercase hex string', () => {
      const data = new Uint8Array([171, 205, 239]); // 0xAB, 0xCD, 0xEF
      const encoded = toHex(data);

      expect(encoded).toBe('abcdef');
    });

    it('should handle empty array', () => {
      const original = new Uint8Array(0);

      const encoded = toHex(original);
      expect(encoded).toBe('');

      const decoded = fromHex('');
      expect(decoded.length).toBe(0);
    });

    it('should pad single-digit hex values', () => {
      const data = new Uint8Array([1, 2, 3]);
      const encoded = toHex(data);

      expect(encoded).toBe('010203');
    });

    it('should encode key correctly', async () => {
      const keyPair = await generateKeyPair();
      const hex = toHex(keyPair.publicKey);

      // 32 bytes = 64 hex characters
      expect(hex.length).toBe(64);
      expect(hex).toMatch(/^[0-9a-f]+$/);
    });
  });

  describe('createRequestSignature', () => {
    it('should create signature with timestamp', async () => {
      const keyPair = await generateKeyPair();

      const result = await createRequestSignature(
        keyPair.privateKey,
        'GET',
        '/api/v1/agents'
      );

      expect(result.signature).toBeDefined();
      expect(result.timestamp).toBeDefined();
      expect(typeof result.signature).toBe('string');
      expect(typeof result.timestamp).toBe('number');
    });

    it('should use provided timestamp', async () => {
      const keyPair = await generateKeyPair();
      const fixedTimestamp = 1704067200000; // 2024-01-01 00:00:00

      const result = await createRequestSignature(
        keyPair.privateKey,
        'POST',
        '/api/v1/verify',
        undefined,
        fixedTimestamp
      );

      expect(result.timestamp).toBe(fixedTimestamp);
    });

    it('should include body in signature', async () => {
      const keyPair = await generateKeyPair();
      const body = { action: 'db:read', resource: 'users' };

      const result1 = await createRequestSignature(
        keyPair.privateKey,
        'POST',
        '/api/v1/verify',
        body,
        1000
      );

      const result2 = await createRequestSignature(
        keyPair.privateKey,
        'POST',
        '/api/v1/verify',
        { action: 'db:write', resource: 'users' },
        1000
      );

      // Different bodies should produce different signatures
      expect(result1.signature).not.toBe(result2.signature);
    });

    it('should produce different signatures for different methods', async () => {
      const keyPair = await generateKeyPair();
      const timestamp = Date.now();

      const getResult = await createRequestSignature(
        keyPair.privateKey,
        'GET',
        '/api/v1/agents',
        undefined,
        timestamp
      );

      const postResult = await createRequestSignature(
        keyPair.privateKey,
        'POST',
        '/api/v1/agents',
        undefined,
        timestamp
      );

      expect(getResult.signature).not.toBe(postResult.signature);
    });

    it('should produce different signatures for different paths', async () => {
      const keyPair = await generateKeyPair();
      const timestamp = Date.now();

      const result1 = await createRequestSignature(
        keyPair.privateKey,
        'GET',
        '/api/v1/agents',
        undefined,
        timestamp
      );

      const result2 = await createRequestSignature(
        keyPair.privateKey,
        'GET',
        '/api/v1/verify',
        undefined,
        timestamp
      );

      expect(result1.signature).not.toBe(result2.signature);
    });

    it('should handle string body', async () => {
      const keyPair = await generateKeyPair();

      const result = await createRequestSignature(
        keyPair.privateKey,
        'POST',
        '/api/v1/test',
        'raw string body'
      );

      expect(result.signature).toBeDefined();
    });

    it('should produce base64 encoded signature', async () => {
      const keyPair = await generateKeyPair();

      const result = await createRequestSignature(
        keyPair.privateKey,
        'GET',
        '/api/v1/test'
      );

      // Base64 should only contain valid characters
      expect(result.signature).toMatch(/^[A-Za-z0-9+/=]+$/);
    });

    it('should normalize method to uppercase', async () => {
      const keyPair = await generateKeyPair();
      const timestamp = Date.now();

      const lowerResult = await createRequestSignature(
        keyPair.privateKey,
        'get',
        '/api/v1/test',
        undefined,
        timestamp
      );

      const upperResult = await createRequestSignature(
        keyPair.privateKey,
        'GET',
        '/api/v1/test',
        undefined,
        timestamp
      );

      // Should be the same since method is normalized
      expect(lowerResult.signature).toBe(upperResult.signature);
    });
  });
});
