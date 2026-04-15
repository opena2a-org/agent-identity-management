/**
 * Tests for SecretsClient - Identity-native secrets management
 *
 * Coverage:
 * - Ed25519 -> X25519 key conversion
 * - Round-trip ECDH + ChaCha20-Poly1305 encryption/decryption
 * - Tampered ciphertext detection
 * - Wrong key rejection
 * - Namespace CRUD delegation
 * - Credential store/rotate delegation
 * - Audit log delegation
 * - Error handling (missing keys, short blobs, incomplete responses)
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import * as nodeCrypto from 'crypto';
import { SecretsClient, edwardsToMontgomeryPrivate } from './SecretsClient';
import type { SecretsClientHost } from './SecretsClient';
import { SecretsError } from '../exceptions';

// ---------------------------------------------------------------------------
// Test helpers: simulate server-side encryption
// ---------------------------------------------------------------------------

/**
 * Simulate what the Go server does when resolving a credential:
 * 1. Generate ephemeral X25519 key pair
 * 2. Convert agent's Ed25519 pub -> X25519 pub
 * 3. ECDH(ephemeral_priv, agent_x25519_pub) -> shared secret
 * 4. SHA-256(shared_secret) -> 256-bit key
 * 5. ChaCha20-Poly1305 encrypt: nonce || ciphertext || tag
 */
function serverEncrypt(
  plaintext: Uint8Array,
  agentEd25519PubKey: Uint8Array,
): { encryptedBlob: Buffer; ephemeralPubKey: Buffer } {
  // Convert agent Ed25519 public key -> X25519 public key
  // Use the birational map: u = (1 + y) / (1 - y) mod p
  const agentX25519Pub = edwardsToMontgomeryPublic(agentEd25519PubKey);

  // Generate ephemeral X25519 key pair
  const ephemeralKeyPair = nodeCrypto.generateKeyPairSync('x25519');

  // Get raw ephemeral public key bytes
  const ephemeralPubDer = ephemeralKeyPair.publicKey.export({ format: 'der', type: 'spki' });
  const ephemeralPubBytes = ephemeralPubDer.slice(-32); // Last 32 bytes of SPKI

  // Build agent X25519 public key object
  const agentX25519PubObj = nodeCrypto.createPublicKey({
    key: Buffer.concat([
      Buffer.from('302a300506032b656e032100', 'hex'),
      Buffer.from(agentX25519Pub),
    ]),
    format: 'der',
    type: 'spki',
  });

  // ECDH
  const sharedSecret = nodeCrypto.diffieHellman({
    privateKey: ephemeralKeyPair.privateKey,
    publicKey: agentX25519PubObj,
  });

  // Derive key
  const encKey = nodeCrypto.createHash('sha256').update(sharedSecret).digest();

  // ChaCha20-Poly1305 encrypt
  const nonce = nodeCrypto.randomBytes(12);
  const cipher = nodeCrypto.createCipheriv('chacha20-poly1305' as string, encKey, nonce, {
    authTagLength: 16,
  });
  const encrypted = Buffer.concat([cipher.update(plaintext), cipher.final()]);
  const tag = cipher.getAuthTag();

  // Format: nonce || ciphertext || tag
  const encryptedBlob = Buffer.concat([nonce, encrypted, tag]);

  return { encryptedBlob, ephemeralPubKey: ephemeralPubBytes };
}

/**
 * Convert Ed25519 public key (compressed Edwards y-coordinate) to X25519 public key
 * (Montgomery u-coordinate). Uses the birational map: u = (1 + y) / (1 - y) mod p.
 *
 * This is a simplified version for testing. The server uses Go's crypto/ecdh which
 * does this conversion internally.
 */
function edwardsToMontgomeryPublic(ed25519Pub: Uint8Array): Uint8Array {
  // Use Node.js crypto to do the conversion via key import/export
  // Import as Ed25519 public key, the conversion happens via the math
  // Actually, Node doesn't have a direct conversion API. We'll use the
  // mathematical conversion ourselves.

  // Ed25519 public key is 32 bytes: the y-coordinate with sign bit in high bit of last byte
  // p = 2^255 - 19
  const p = 2n ** 255n - 19n;

  // Read y coordinate (little-endian, clear top bit)
  let y = 0n;
  for (let i = 0; i < 32; i++) {
    y |= BigInt(ed25519Pub[i]) << BigInt(8 * i);
  }
  y &= (1n << 255n) - 1n; // Clear sign bit

  // u = (1 + y) / (1 - y) mod p
  const one = 1n;
  const numerator = modP(one + y, p);
  const denominator = modP(one - y, p);
  const denominatorInv = modPow(denominator, p - 2n, p); // Fermat's little theorem
  const u = modP(numerator * denominatorInv, p);

  // Write u as 32-byte little-endian
  const result = new Uint8Array(32);
  let val = u;
  for (let i = 0; i < 32; i++) {
    result[i] = Number(val & 0xffn);
    val >>= 8n;
  }

  return result;
}

function modP(a: bigint, p: bigint): bigint {
  return ((a % p) + p) % p;
}

function modPow(base: bigint, exp: bigint, mod: bigint): bigint {
  let result = 1n;
  base = modP(base, mod);
  while (exp > 0n) {
    if (exp & 1n) {
      result = modP(result * base, mod);
    }
    exp >>= 1n;
    base = modP(base * base, mod);
  }
  return result;
}

// ---------------------------------------------------------------------------
// Generate a test Ed25519 key pair using @noble/ed25519
// ---------------------------------------------------------------------------

async function generateTestKeyPair(): Promise<{ privateKey: Uint8Array; publicKey: Uint8Array }> {
  const ed = await import('@noble/ed25519');
  const privateKey = ed.utils.randomPrivateKey();
  const publicKey = await ed.getPublicKeyAsync(privateKey);
  return { privateKey, publicKey };
}

// ---------------------------------------------------------------------------
// Mock host factory
// ---------------------------------------------------------------------------

function createMockHost(overrides: Partial<SecretsClientHost> = {}): SecretsClientHost & {
  request: ReturnType<typeof vi.fn>;
} {
  const request = vi.fn();
  return {
    request,
    getPrivateKey: () => null,
    getPublicKey: () => null,
    ...overrides,
  };
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('SecretsClient', () => {
  describe('edwardsToMontgomeryPrivate', () => {
    it('should produce a 32-byte clamped scalar from a 32-byte seed', () => {
      const seed = nodeCrypto.randomBytes(32);
      const x25519Key = edwardsToMontgomeryPrivate(new Uint8Array(seed));

      expect(x25519Key).toHaveLength(32);
      // Check clamping: low 3 bits of first byte must be 0
      expect(x25519Key[0] & 7).toBe(0);
      // High bit of last byte must be 0, second-highest must be 1
      expect(x25519Key[31] & 128).toBe(0);
      expect(x25519Key[31] & 64).toBe(64);
    });

    it('should extract seed from 64-byte key and produce same result', () => {
      const seed = nodeCrypto.randomBytes(32);
      const extended = new Uint8Array(64);
      extended.set(seed, 0);
      extended.set(nodeCrypto.randomBytes(32), 32); // pub key part

      const from32 = edwardsToMontgomeryPrivate(new Uint8Array(seed));
      const from64 = edwardsToMontgomeryPrivate(extended);

      expect(from32).toEqual(from64);
    });

    it('should be deterministic', () => {
      const seed = nodeCrypto.randomBytes(32);
      const a = edwardsToMontgomeryPrivate(new Uint8Array(seed));
      const b = edwardsToMontgomeryPrivate(new Uint8Array(seed));
      expect(a).toEqual(b);
    });
  });

  describe('resolve — round-trip decrypt', () => {
    it('should decrypt a server-encrypted credential', async () => {
      const { privateKey, publicKey } = await generateTestKeyPair();
      const plaintext = new TextEncoder().encode('ghp_secrettoken123');

      // Simulate server encryption
      const { encryptedBlob, ephemeralPubKey } = serverEncrypt(plaintext, publicKey);

      const host = createMockHost({
        getPrivateKey: () => Buffer.from(privateKey).toString('base64'),
        getPublicKey: () => Buffer.from(publicKey).toString('base64'),
      });

      host.request.mockResolvedValue({
        encryptedBlob: encryptedBlob.toString('base64'),
        ephemeralPubKey: ephemeralPubKey.toString('base64'),
        encryptionAlg: 'X25519-ChaCha20-Poly1305',
      });

      const client = new SecretsClient(host);
      const result = await client.resolve('github-creds', 'read');

      expect(result.credential).toEqual(plaintext);
      expect(result.encryptionAlg).toBe('X25519-ChaCha20-Poly1305');
      expect(result.namespace).toBe('github-creds');

      // Verify the request was signed correctly
      expect(host.request).toHaveBeenCalledWith(
        'POST',
        '/api/v1/secrets/resolve',
        expect.objectContaining({
          namespace: 'github-creds',
          operation: 'read',
          agentPublicKey: Buffer.from(publicKey).toString('base64'),
        }),
      );
    });

    it('should decrypt empty credential', async () => {
      const { privateKey, publicKey } = await generateTestKeyPair();
      const plaintext = new Uint8Array(0);
      const { encryptedBlob, ephemeralPubKey } = serverEncrypt(plaintext, publicKey);

      const host = createMockHost({
        getPrivateKey: () => Buffer.from(privateKey).toString('base64'),
        getPublicKey: () => Buffer.from(publicKey).toString('base64'),
      });
      host.request.mockResolvedValue({
        encryptedBlob: encryptedBlob.toString('base64'),
        ephemeralPubKey: ephemeralPubKey.toString('base64'),
        encryptionAlg: 'X25519-ChaCha20-Poly1305',
      });

      const client = new SecretsClient(host);
      const result = await client.resolve('ns', 'read');
      expect(result.credential).toEqual(plaintext);
    });

    it('should decrypt large credential (4KB)', async () => {
      const { privateKey, publicKey } = await generateTestKeyPair();
      const plaintext = nodeCrypto.randomBytes(4096);
      const { encryptedBlob, ephemeralPubKey } = serverEncrypt(new Uint8Array(plaintext), publicKey);

      const host = createMockHost({
        getPrivateKey: () => Buffer.from(privateKey).toString('base64'),
        getPublicKey: () => Buffer.from(publicKey).toString('base64'),
      });
      host.request.mockResolvedValue({
        encryptedBlob: encryptedBlob.toString('base64'),
        ephemeralPubKey: ephemeralPubKey.toString('base64'),
        encryptionAlg: 'X25519-ChaCha20-Poly1305',
      });

      const client = new SecretsClient(host);
      const result = await client.resolve('ns', 'read');
      expect(result.credential).toEqual(new Uint8Array(plaintext));
    });
  });

  describe('resolve — tamper detection', () => {
    it('should reject tampered ciphertext', async () => {
      const { privateKey, publicKey } = await generateTestKeyPair();
      const plaintext = new TextEncoder().encode('secret');
      const { encryptedBlob, ephemeralPubKey } = serverEncrypt(plaintext, publicKey);

      // Tamper with a byte in the middle of the ciphertext
      encryptedBlob[15] ^= 0xff;

      const host = createMockHost({
        getPrivateKey: () => Buffer.from(privateKey).toString('base64'),
        getPublicKey: () => Buffer.from(publicKey).toString('base64'),
      });
      host.request.mockResolvedValue({
        encryptedBlob: encryptedBlob.toString('base64'),
        ephemeralPubKey: ephemeralPubKey.toString('base64'),
        encryptionAlg: 'X25519-ChaCha20-Poly1305',
      });

      const client = new SecretsClient(host);
      await expect(client.resolve('ns', 'read')).rejects.toThrow(SecretsError);
      await expect(client.resolve('ns', 'read')).rejects.toThrow('Failed to decrypt');
    });

    it('should reject tampered auth tag', async () => {
      const { privateKey, publicKey } = await generateTestKeyPair();
      const plaintext = new TextEncoder().encode('secret');
      const { encryptedBlob, ephemeralPubKey } = serverEncrypt(plaintext, publicKey);

      // Tamper with the last byte (auth tag)
      encryptedBlob[encryptedBlob.length - 1] ^= 0xff;

      const host = createMockHost({
        getPrivateKey: () => Buffer.from(privateKey).toString('base64'),
        getPublicKey: () => Buffer.from(publicKey).toString('base64'),
      });
      host.request.mockResolvedValue({
        encryptedBlob: encryptedBlob.toString('base64'),
        ephemeralPubKey: ephemeralPubKey.toString('base64'),
        encryptionAlg: 'X25519-ChaCha20-Poly1305',
      });

      const client = new SecretsClient(host);
      await expect(client.resolve('ns', 'read')).rejects.toThrow(SecretsError);
    });
  });

  describe('resolve — wrong key rejection', () => {
    it('should fail to decrypt with a different agent key', async () => {
      const agentA = await generateTestKeyPair();
      const agentB = await generateTestKeyPair();

      const plaintext = new TextEncoder().encode('secret-for-A');
      // Encrypted for agent A's public key
      const { encryptedBlob, ephemeralPubKey } = serverEncrypt(plaintext, agentA.publicKey);

      // Try to decrypt with agent B's private key
      const host = createMockHost({
        getPrivateKey: () => Buffer.from(agentB.privateKey).toString('base64'),
        getPublicKey: () => Buffer.from(agentB.publicKey).toString('base64'),
      });
      host.request.mockResolvedValue({
        encryptedBlob: encryptedBlob.toString('base64'),
        ephemeralPubKey: ephemeralPubKey.toString('base64'),
        encryptionAlg: 'X25519-ChaCha20-Poly1305',
      });

      const client = new SecretsClient(host);
      await expect(client.resolve('ns', 'read')).rejects.toThrow(SecretsError);
    });
  });

  describe('resolve — error handling', () => {
    it('should throw if no private key', async () => {
      const host = createMockHost({
        getPrivateKey: () => null,
        getPublicKey: () => 'some-key',
      });

      const client = new SecretsClient(host);
      await expect(client.resolve('ns')).rejects.toThrow('Ed25519 private key required');
    });

    it('should throw if no public key', async () => {
      const host = createMockHost({
        getPrivateKey: () => 'some-key',
        getPublicKey: () => null,
      });

      const client = new SecretsClient(host);
      await expect(client.resolve('ns')).rejects.toThrow('Public key required');
    });

    it('should throw on incomplete server response (no blob)', async () => {
      const { privateKey, publicKey } = await generateTestKeyPair();
      const host = createMockHost({
        getPrivateKey: () => Buffer.from(privateKey).toString('base64'),
        getPublicKey: () => Buffer.from(publicKey).toString('base64'),
      });
      host.request.mockResolvedValue({
        ephemeralPubKey: 'abc',
        // Missing encryptedBlob
      });

      const client = new SecretsClient(host);
      await expect(client.resolve('ns')).rejects.toThrow('incomplete resolution response');
    });

    it('should throw on blob too short', async () => {
      const { privateKey, publicKey } = await generateTestKeyPair();
      const host = createMockHost({
        getPrivateKey: () => Buffer.from(privateKey).toString('base64'),
        getPublicKey: () => Buffer.from(publicKey).toString('base64'),
      });
      // Use a real ephemeral key so ECDH succeeds, but blob is too short
      const nodeCryptoLocal = require('crypto');
      const ephKp = nodeCryptoLocal.generateKeyPairSync('x25519');
      const ephPubDer = ephKp.publicKey.export({ format: 'der', type: 'spki' });
      const ephPubBytes = ephPubDer.slice(-32);

      // 20 bytes < 12 (nonce) + 16 (tag) = 28 minimum
      const shortBlob = Buffer.alloc(20).toString('base64');
      host.request.mockResolvedValue({
        encryptedBlob: shortBlob,
        ephemeralPubKey: ephPubBytes.toString('base64'),
        encryptionAlg: 'X25519-ChaCha20-Poly1305',
      });

      const client = new SecretsClient(host);
      await expect(client.resolve('ns')).rejects.toThrow('too short');
    });
  });

  describe('namespace CRUD', () => {
    let host: ReturnType<typeof createMockHost>;
    let client: SecretsClient;

    beforeEach(() => {
      host = createMockHost();
      client = new SecretsClient(host);
    });

    it('should create a namespace', async () => {
      host.request.mockResolvedValue({ id: 'ns-123', status: 'active' });

      const result = await client.createNamespace('agent-1', 'github-creds', ['read'], [
        'https://api.github.com/*',
      ]);

      expect(host.request).toHaveBeenCalledWith('POST', '/api/v1/secrets/namespaces', {
        agentId: 'agent-1',
        namespace: 'github-creds',
        operations: ['read'],
        urlPatterns: ['https://api.github.com/*'],
      });
      expect(result).toEqual({ id: 'ns-123', status: 'active' });
    });

    it('should list namespaces', async () => {
      host.request.mockResolvedValue({
        namespaces: [{ id: 'ns-1' }, { id: 'ns-2' }],
      });

      const result = await client.listNamespaces('agent-1');
      expect(host.request).toHaveBeenCalledWith(
        'GET',
        '/api/v1/secrets/namespaces?agentId=agent-1',
      );
      expect(result).toHaveLength(2);
    });

    it('should return empty array when no namespaces', async () => {
      host.request.mockResolvedValue({});
      const result = await client.listNamespaces('agent-1');
      expect(result).toEqual([]);
    });

    it('should get a namespace', async () => {
      host.request.mockResolvedValue({ id: 'ns-123', namespace: 'github-creds' });
      const result = await client.getNamespace('ns-123');
      expect(host.request).toHaveBeenCalledWith('GET', '/api/v1/secrets/namespaces/ns-123');
      expect(result).toEqual({ id: 'ns-123', namespace: 'github-creds' });
    });

    it('should delete a namespace', async () => {
      host.request.mockResolvedValue({ deleted: true });
      const result = await client.deleteNamespace('ns-123');
      expect(host.request).toHaveBeenCalledWith('DELETE', '/api/v1/secrets/namespaces/ns-123');
      expect(result).toEqual({ deleted: true });
    });
  });

  describe('credential storage', () => {
    let host: ReturnType<typeof createMockHost>;
    let client: SecretsClient;

    beforeEach(() => {
      host = createMockHost();
      client = new SecretsClient(host);
    });

    it('should store a credential', async () => {
      host.request.mockResolvedValue({ version: 1 });
      const blob = new Uint8Array([1, 2, 3, 4]);

      await client.storeCredential({ namespaceId: 'ns-1', encryptedBlob: blob });

      expect(host.request).toHaveBeenCalledWith(
        'POST',
        '/api/v1/secrets/namespaces/ns-1/credentials',
        expect.objectContaining({
          encryptionAlgorithm: 'X25519-ChaCha20-Poly1305',
        }),
      );
    });

    it('should rotate a credential', async () => {
      host.request.mockResolvedValue({ version: 2 });
      const blob = new Uint8Array([5, 6, 7, 8]);

      await client.rotateCredential({ namespaceId: 'ns-1', encryptedBlob: blob });

      expect(host.request).toHaveBeenCalledWith(
        'POST',
        '/api/v1/secrets/namespaces/ns-1/rotate',
        expect.objectContaining({
          encryptionAlgorithm: 'X25519-ChaCha20-Poly1305',
        }),
      );
    });

    it('should include ephemeral public key when provided', async () => {
      host.request.mockResolvedValue({ version: 1 });
      const blob = new Uint8Array([1, 2, 3]);
      const epk = new Uint8Array([10, 20, 30]);

      await client.storeCredential({
        namespaceId: 'ns-1',
        encryptedBlob: blob,
        ephemeralPublicKey: epk,
      });

      expect(host.request).toHaveBeenCalledWith(
        'POST',
        '/api/v1/secrets/namespaces/ns-1/credentials',
        expect.objectContaining({
          ephemeralPublicKey: expect.any(String),
        }),
      );
    });
  });

  describe('audit log', () => {
    it('should query audit log with defaults', async () => {
      const host = createMockHost();
      host.request.mockResolvedValue({
        entries: [{ operation: 'resolve', result: 'granted' }],
      });

      const client = new SecretsClient(host);
      const result = await client.getAuditLog({ agentId: 'agent-1' });

      expect(host.request).toHaveBeenCalledWith(
        'GET',
        '/api/v1/secrets/audit?agentId=agent-1&limit=50&offset=0',
      );
      expect(result).toHaveLength(1);
    });

    it('should pass since/limit/offset parameters', async () => {
      const host = createMockHost();
      host.request.mockResolvedValue({ entries: [] });

      const client = new SecretsClient(host);
      await client.getAuditLog({
        agentId: 'a1',
        since: '2026-01-01T00:00:00Z',
        limit: 10,
        offset: 5,
      });

      expect(host.request).toHaveBeenCalledWith(
        'GET',
        '/api/v1/secrets/audit?agentId=a1&limit=10&offset=5&since=2026-01-01T00:00:00Z',
      );
    });

    it('should return empty array when no entries', async () => {
      const host = createMockHost();
      host.request.mockResolvedValue({});
      const client = new SecretsClient(host);
      const result = await client.getAuditLog({ agentId: 'a1' });
      expect(result).toEqual([]);
    });
  });
});
