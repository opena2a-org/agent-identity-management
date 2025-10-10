import {
  generateEd25519Keypair,
  signRequest,
  verifySignature,
  encodePublicKey,
  decodePublicKey,
  encodePrivateKey,
  decodePrivateKey,
} from '../signing';

describe('Ed25519 Signing', () => {
  describe('generateEd25519Keypair', () => {
    it('should generate a valid keypair', () => {
      const { privateKey, publicKey } = generateEd25519Keypair();

      expect(privateKey).toBeDefined();
      expect(publicKey).toBeDefined();
      expect(privateKey.length).toBe(64); // Ed25519 private key size
      expect(publicKey.length).toBe(32); // Ed25519 public key size
    });

    it('should generate different keypairs each time', () => {
      const keypair1 = generateEd25519Keypair();
      const keypair2 = generateEd25519Keypair();

      expect(keypair1.privateKey).not.toEqual(keypair2.privateKey);
      expect(keypair1.publicKey).not.toEqual(keypair2.publicKey);
    });
  });

  describe('signRequest', () => {
    it('should sign data correctly', () => {
      const { privateKey, publicKey } = generateEd25519Keypair();
      const data = {
        agent_id: 'test-agent-123',
        timestamp: '2025-10-09T12:00:00Z',
        type: 'ai_agent',
      };

      const signature = signRequest(privateKey, data);

      expect(signature).toBeDefined();
      expect(typeof signature).toBe('string');
      expect(signature.length).toBeGreaterThan(0);

      // Verify the signature
      const valid = verifySignature(publicKey, data, signature);
      expect(valid).toBe(true);
    });

    it('should produce consistent signatures for sorted keys', () => {
      const { privateKey } = generateEd25519Keypair();

      const data1 = {
        z_field: 'last',
        a_field: 'first',
        m_field: 'middle',
      };

      const data2 = {
        a_field: 'first',
        m_field: 'middle',
        z_field: 'last',
      };

      const sig1 = signRequest(privateKey, data1);
      const sig2 = signRequest(privateKey, data2);

      // Signatures should be identical because keys are sorted
      expect(sig1).toBe(sig2);
    });
  });

  describe('verifySignature', () => {
    it('should verify valid signatures', () => {
      const { privateKey, publicKey } = generateEd25519Keypair();
      const data = { test: 'data', value: 123 };

      const signature = signRequest(privateKey, data);
      const valid = verifySignature(publicKey, data, signature);

      expect(valid).toBe(true);
    });

    it('should reject invalid signatures', () => {
      const { publicKey } = generateEd25519Keypair();
      const data = { test: 'data' };
      const invalidSignature = 'invalid-signature-base64';

      const valid = verifySignature(publicKey, data, invalidSignature);

      expect(valid).toBe(false);
    });

    it('should reject tampered data', () => {
      const { privateKey, publicKey } = generateEd25519Keypair();
      const originalData = { value: 'original' };
      const signature = signRequest(privateKey, originalData);

      const tamperedData = { value: 'tampered' };
      const valid = verifySignature(publicKey, tamperedData, signature);

      expect(valid).toBe(false);
    });

    it('should reject signature from different keypair', () => {
      const keypair1 = generateEd25519Keypair();
      const keypair2 = generateEd25519Keypair();

      const data = { test: 'data' };
      const signature = signRequest(keypair1.privateKey, data);

      // Try to verify with different public key
      const valid = verifySignature(keypair2.publicKey, data, signature);

      expect(valid).toBe(false);
    });
  });

  describe('encodePublicKey / decodePublicKey', () => {
    it('should encode and decode public key correctly', () => {
      const { publicKey } = generateEd25519Keypair();

      const encoded = encodePublicKey(publicKey);
      expect(encoded).toBeDefined();
      expect(typeof encoded).toBe('string');

      const decoded = decodePublicKey(encoded);
      expect(decoded).toEqual(publicKey);
    });

    it('should throw error for invalid public key size', () => {
      const shortKey = 'aGVsbG8='; // "hello" in base64 - too short

      expect(() => {
        decodePublicKey(shortKey);
      }).toThrow('Invalid public key size');
    });
  });

  describe('encodePrivateKey / decodePrivateKey', () => {
    it('should encode and decode private key correctly', () => {
      const { privateKey } = generateEd25519Keypair();

      const encoded = encodePrivateKey(privateKey);
      expect(encoded).toBeDefined();
      expect(typeof encoded).toBe('string');

      const decoded = decodePrivateKey(encoded);
      expect(decoded).toEqual(privateKey);
    });

    it('should throw error for invalid private key size', () => {
      const shortKey = 'aGVsbG8='; // "hello" in base64 - too short

      expect(() => {
        decodePrivateKey(shortKey);
      }).toThrow('Invalid private key size');
    });
  });

  describe('End-to-end signing workflow', () => {
    it('should complete full signing workflow', () => {
      // 1. Generate keypair
      const { privateKey, publicKey } = generateEd25519Keypair();

      // 2. Encode keys for storage
      const privateKeyB64 = encodePrivateKey(privateKey);
      const publicKeyB64 = encodePublicKey(publicKey);

      // 3. Sign some data
      const data = {
        agent_id: 'test-123',
        name: 'test-agent',
        timestamp: new Date().toISOString(),
      };
      const signature = signRequest(privateKey, data);

      // 4. Decode keys from storage
      const decodedPrivateKey = decodePrivateKey(privateKeyB64);
      const decodedPublicKey = decodePublicKey(publicKeyB64);

      // 5. Verify signature with decoded public key
      const valid = verifySignature(decodedPublicKey, data, signature);

      expect(valid).toBe(true);
      expect(decodedPrivateKey).toEqual(privateKey);
      expect(decodedPublicKey).toEqual(publicKey);
    });
  });
});
