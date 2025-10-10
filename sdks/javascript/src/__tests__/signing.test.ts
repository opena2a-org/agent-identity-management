import {
  generateEd25519Keypair,
  signRequest,
  verifySignature,
  encodePublicKey,
  decodePublicKey,
  encodePrivateKey,
  decodePrivateKey,
  KeyPair,
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

  describe('KeyPair class (OOP approach)', () => {
    describe('KeyPair.generate()', () => {
      it('should generate a valid keypair', () => {
        const keyPair = KeyPair.generate();

        expect(keyPair).toBeDefined();
        expect(keyPair.publicKey).toHaveLength(32);
        expect(keyPair.privateKey).toHaveLength(64);
      });

      it('should generate different keypairs each time', () => {
        const keyPair1 = KeyPair.generate();
        const keyPair2 = KeyPair.generate();

        expect(keyPair1.publicKey).not.toEqual(keyPair2.publicKey);
        expect(keyPair1.privateKey).not.toEqual(keyPair2.privateKey);
      });
    });

    describe('KeyPair.sign() and KeyPair.verify()', () => {
      it('should sign and verify messages correctly', () => {
        const keyPair = KeyPair.generate();
        const message = 'test message for signing';

        const signature = keyPair.sign(message);
        expect(signature).toBeDefined();
        expect(signature.length).toBeGreaterThan(0);

        const valid = keyPair.verify(message, signature);
        expect(valid).toBe(true);
      });

      it('should produce deterministic signatures', () => {
        const keyPair = KeyPair.generate();
        const message = 'test message';

        const sig1 = keyPair.sign(message);
        const sig2 = keyPair.sign(message);

        expect(sig1).toBe(sig2);
      });

      it('should reject invalid signatures', () => {
        const keyPair = KeyPair.generate();
        const message = 'test message';
        const invalidSignature = 'invalid_signature_base64';

        const valid = keyPair.verify(message, invalidSignature);
        expect(valid).toBe(false);
      });

      it('should reject signatures for different messages', () => {
        const keyPair = KeyPair.generate();
        const signature = keyPair.sign('original message');

        const valid = keyPair.verify('different message', signature);
        expect(valid).toBe(false);
      });
    });

    describe('KeyPair.fromBase64()', () => {
      it('should import keypair from base64 private key', () => {
        const original = KeyPair.generate();
        const privateKeyB64 = original.privateKeyBase64();

        const imported = KeyPair.fromBase64(privateKeyB64);

        expect(imported.publicKeyBase64()).toBe(original.publicKeyBase64());
        expect(imported.privateKeyBase64()).toBe(original.privateKeyBase64());
      });

      it('should produce identical signatures after import', () => {
        const original = KeyPair.generate();
        const imported = KeyPair.fromBase64(original.privateKeyBase64());

        const message = 'test message';
        const sig1 = original.sign(message);
        const sig2 = imported.sign(message);

        expect(sig1).toBe(sig2);
      });
    });

    describe('KeyPair.fromPrivateKey()', () => {
      it('should create keypair from 32-byte seed', () => {
        const original = KeyPair.generate();
        const seed = original.privateKey.slice(0, 32);

        const keyPair = KeyPair.fromPrivateKey(seed);

        expect(keyPair.publicKeyBase64()).toBe(original.publicKeyBase64());
      });

      it('should create keypair from 64-byte private key', () => {
        const original = KeyPair.generate();

        const keyPair = KeyPair.fromPrivateKey(original.privateKey);

        expect(keyPair.publicKeyBase64()).toBe(original.publicKeyBase64());
        expect(keyPair.privateKeyBase64()).toBe(original.privateKeyBase64());
      });

      it('should throw error for invalid key length', () => {
        const invalidKey = new Uint8Array(30);

        expect(() => {
          KeyPair.fromPrivateKey(invalidKey);
        }).toThrow('Invalid private key length');
      });
    });

    describe('KeyPair.signPayload()', () => {
      it('should sign JSON payloads', () => {
        const keyPair = KeyPair.generate();
        const payload = {
          name: 'test-agent',
          type: 'ai_agent',
          public_key: keyPair.publicKeyBase64(),
        };

        const signature = keyPair.signPayload(payload);

        expect(signature).toBeDefined();
        expect(signature.length).toBeGreaterThan(0);
      });

      it('should produce deterministic signatures for same payload', () => {
        const keyPair = KeyPair.generate();
        const payload = { name: 'test', value: 123 };

        const sig1 = keyPair.signPayload(payload);
        const sig2 = keyPair.signPayload(payload);

        expect(sig1).toBe(sig2);
      });

      it('should produce same signature as signRequest()', () => {
        const keyPair = KeyPair.generate();
        const payload = { test: 'data' };

        const classSig = keyPair.signPayload(payload);
        const funcSig = signRequest(keyPair.privateKey, payload);

        expect(classSig).toBe(funcSig);
      });
    });

    describe('Key encoding methods', () => {
      it('should encode public key to base64', () => {
        const keyPair = KeyPair.generate();
        const publicKeyB64 = keyPair.publicKeyBase64();

        expect(publicKeyB64).toBeDefined();
        expect(typeof publicKeyB64).toBe('string');

        const decoded = decodePublicKey(publicKeyB64);
        expect(decoded).toEqual(keyPair.publicKey);
      });

      it('should encode private key to base64', () => {
        const keyPair = KeyPair.generate();
        const privateKeyB64 = keyPair.privateKeyBase64();

        expect(privateKeyB64).toBeDefined();
        expect(typeof privateKeyB64).toBe('string');

        const decoded = decodePrivateKey(privateKeyB64);
        expect(decoded).toEqual(keyPair.privateKey);
      });

      it('should encode 32-byte seed to base64', () => {
        const keyPair = KeyPair.generate();
        const seedB64 = keyPair.seedBase64();

        expect(seedB64).toBeDefined();

        const seed = Buffer.from(seedB64, 'base64');
        expect(seed.length).toBe(32);
      });
    });

    describe('Client integration workflow', () => {
      it('should complete full client signing workflow', () => {
        // Generate keypair
        const keyPair = KeyPair.generate();

        // Sign a message
        const message = 'important client message';
        const signature = keyPair.sign(message);

        // Verify signature
        const valid = keyPair.verify(message, signature);
        expect(valid).toBe(true);

        // Export for storage
        const privateKeyB64 = keyPair.privateKeyBase64();
        const publicKeyB64 = keyPair.publicKeyBase64();

        // Reload from storage
        const reloaded = KeyPair.fromBase64(privateKeyB64);

        // Verify keys match
        expect(reloaded.publicKeyBase64()).toBe(publicKeyB64);

        // Verify signatures match
        const signature2 = reloaded.sign(message);
        expect(signature2).toBe(signature);
      });
    });
  });
});
