/**
 * Integration tests for JavaScript SDK
 * These tests verify the complete workflow of the SDK
 *
 * NOTE: These tests require:
 * - System keyring access (may prompt for password on first run)
 * - Running AIM backend at http://localhost:8080 (for full integration tests)
 */

import {
  generateEd25519Keypair,
  signRequest,
  verifySignature,
  encodePrivateKey,
  encodePublicKey,
} from '../signing';
import { autoDetectMCPs } from '../detection/capability-detection';
import {
  storeCredentials,
  loadCredentials,
  clearCredentials,
  hasCredentials,
} from '../credentials';

describe('SDK Integration Tests', () => {
  // Clean up credentials before and after tests
  beforeEach(async () => {
    await clearCredentials();
  });

  afterEach(async () => {
    await clearCredentials();
  });

  describe('Credential Management Workflow', () => {
    it('should store and load credentials', async () => {
      const { privateKey } = generateEd25519Keypair();

      const testCreds = {
        agentId: 'test-agent-123',
        apiKey: 'test-api-key-456',
        privateKey,
      };

      // Store credentials
      await storeCredentials(testCreds);

      // Check if credentials exist
      const exists = await hasCredentials();
      expect(exists).toBe(true);

      // Load credentials
      const loaded = await loadCredentials();
      expect(loaded).not.toBeNull();
      expect(loaded?.agentId).toBe(testCreds.agentId);
      expect(loaded?.apiKey).toBe(testCreds.apiKey);
      expect(loaded?.privateKey).toEqual(testCreds.privateKey);
    });

    it('should return null when no credentials stored', async () => {
      const loaded = await loadCredentials();
      expect(loaded).toBeNull();

      const exists = await hasCredentials();
      expect(exists).toBe(false);
    });

    it('should clear all credentials', async () => {
      const { privateKey } = generateEd25519Keypair();

      await storeCredentials({
        agentId: 'test-123',
        apiKey: 'key-456',
        privateKey,
      });

      // Verify stored
      let exists = await hasCredentials();
      expect(exists).toBe(true);

      // Clear
      await clearCredentials();

      // Verify cleared
      exists = await hasCredentials();
      expect(exists).toBe(false);

      const loaded = await loadCredentials();
      expect(loaded).toBeNull();
    });
  });

  describe('Complete Agent Registration Flow (Mock)', () => {
    it('should complete registration workflow', async () => {
      // 1. Generate Ed25519 keypair
      const { privateKey, publicKey } = generateEd25519Keypair();
      expect(privateKey.length).toBe(64);
      expect(publicKey.length).toBe(32);

      // 2. Prepare registration payload
      const payload = {
        name: 'test-agent',
        type: 'ai_agent',
        public_key: encodePublicKey(publicKey),
      };

      // 3. Sign the payload
      const signature = signRequest(privateKey, payload);
      expect(signature).toBeDefined();

      // 4. Verify signature (backend would do this)
      const valid = verifySignature(publicKey, payload, signature);
      expect(valid).toBe(true);

      // 5. Store credentials (after successful registration)
      const mockRegistration = {
        id: 'agent-123',
        apiKey: 'key-456',
      };

      await storeCredentials({
        agentId: mockRegistration.id,
        apiKey: mockRegistration.apiKey,
        privateKey,
      });

      // 6. Verify credentials stored
      const loaded = await loadCredentials();
      expect(loaded?.agentId).toBe(mockRegistration.id);
      expect(loaded?.apiKey).toBe(mockRegistration.apiKey);
      expect(loaded?.privateKey).toEqual(privateKey);
    });
  });

  describe('MCP Detection Workflow', () => {
    it('should auto-detect MCPs and collect runtime info', async () => {
      const detection = await autoDetectMCPs();

      // Verify structure
      expect(detection).toBeDefined();
      expect(detection.mcps).toBeDefined();
      expect(Array.isArray(detection.mcps)).toBe(true);
      expect(detection.detectedAt).toBeDefined();
      expect(detection.runtime).toBeDefined();

      // Verify runtime info
      expect(detection.runtime.runtime).toBe('node');
      expect(detection.runtime.node_version).toBeDefined();
      expect(detection.runtime.platform).toBeDefined();
      expect(detection.runtime.arch).toBeDefined();
      expect(detection.runtime.num_cpus).toBeDefined();

      // Verify detectedAt is valid ISO timestamp
      expect(() => new Date(detection.detectedAt)).not.toThrow();
    });
  });

  describe('Signing with Stored Credentials', () => {
    it('should sign requests using stored private key', async () => {
      // Generate and store credentials
      const { privateKey, publicKey } = generateEd25519Keypair();
      await storeCredentials({
        agentId: 'test-123',
        apiKey: 'key-456',
        privateKey,
      });

      // Load credentials
      const loaded = await loadCredentials();
      expect(loaded).not.toBeNull();

      // Sign a request using loaded private key
      const data = {
        mcp_server: 'filesystem',
        timestamp: new Date().toISOString(),
      };

      const signature = signRequest(loaded!.privateKey!, data);
      expect(signature).toBeDefined();

      // Verify signature
      const valid = verifySignature(publicKey, data, signature);
      expect(valid).toBe(true);
    });
  });

  describe('Error Handling', () => {
    it('should handle missing credentials gracefully', async () => {
      const loaded = await loadCredentials();
      expect(loaded).toBeNull();
    });

    it('should handle corrupted private key gracefully', async () => {
      // Store credentials with invalid private key format
      // This tests the error handling in loadCredentials
      await storeCredentials({
        agentId: 'test-123',
        apiKey: 'key-456',
        // privateKey intentionally omitted
      });

      const loaded = await loadCredentials();
      expect(loaded).not.toBeNull();
      expect(loaded?.privateKey).toBeUndefined();
    });
  });
});

describe('End-to-End Workflow (Mock)', () => {
  beforeEach(async () => {
    await clearCredentials();
  });

  afterEach(async () => {
    await clearCredentials();
  });

  it('should complete full agent lifecycle', async () => {
    // Step 1: Register agent (mock)
    const { privateKey, publicKey } = generateEd25519Keypair();

    const registrationPayload = {
      name: 'e2e-test-agent',
      type: 'ai_agent',
      public_key: encodePublicKey(publicKey),
    };

    const signature = signRequest(privateKey, registrationPayload);
    expect(verifySignature(publicKey, registrationPayload, signature)).toBe(true);

    // Mock backend response
    const mockResponse = {
      id: 'agent-e2e-123',
      name: 'e2e-test-agent',
      api_key: 'key-e2e-456',
      public_key: encodePublicKey(publicKey),
    };

    // Step 2: Store credentials
    await storeCredentials({
      agentId: mockResponse.id,
      apiKey: mockResponse.api_key,
      privateKey,
    });

    // Step 3: Verify stored
    const exists = await hasCredentials();
    expect(exists).toBe(true);

    // Step 4: Load credentials for subsequent operations
    const loaded = await loadCredentials();
    expect(loaded?.agentId).toBe(mockResponse.id);

    // Step 5: Auto-detect MCPs
    const detection = await autoDetectMCPs();
    expect(detection.mcps).toBeDefined();

    // Step 6: Sign MCP report request
    const reportPayload = {
      agent_id: loaded!.agentId,
      mcps: detection.mcps.map((m) => m.name),
      timestamp: new Date().toISOString(),
    };

    const reportSignature = signRequest(loaded!.privateKey!, reportPayload);
    expect(reportSignature).toBeDefined();

    // Step 7: Verify signature
    const validReport = verifySignature(publicKey, reportPayload, reportSignature);
    expect(validReport).toBe(true);

    // Step 8: Cleanup
    await clearCredentials();
    const clearedExists = await hasCredentials();
    expect(clearedExists).toBe(false);
  });
});
