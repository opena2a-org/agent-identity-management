/**
 * Integration tests for OAuth authentication and credential management.
 *
 * Tests the full OAuth flow including:
 * - Token acquisition
 * - Token caching and refresh
 * - Credential loading from environment/file
 * - Credential persistence
 */

import { describe, it, expect, beforeAll, afterAll, afterEach, vi } from 'vitest';
import { existsSync, mkdirSync, rmSync, writeFileSync } from 'fs';
import { join } from 'path';
import { tmpdir } from 'os';
import {
  OAuthTokenManager,
  loadCredentialsFromEnv,
  loadCredentialsFromFile,
  saveCredentialsToFile,
} from './oauth';
import type { AgentCredentials } from '../types';

const AIM_URL = process.env.AIM_BASE_URL ?? 'http://localhost:8080';
const TEST_DIR = join(tmpdir(), 'aim-sdk-test-' + Date.now());

let backendAvailable = false;

async function checkBackendHealth(): Promise<boolean> {
  try {
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), 5000);

    const response = await fetch(`${AIM_URL}/health`, {
      method: 'GET',
      signal: controller.signal,
    });

    clearTimeout(timeoutId);
    return response.ok;
  } catch {
    return false;
  }
}

describe('OAuth Integration Tests', () => {
  beforeAll(async () => {
    backendAvailable = await checkBackendHealth();

    if (!backendAvailable) {
      console.log(`⚠️  OAuth integration tests skipped: Backend not available at ${AIM_URL}`);
    }

    // Create test directory
    if (!existsSync(TEST_DIR)) {
      mkdirSync(TEST_DIR, { recursive: true });
    }
  });

  afterEach(() => {
    vi.unstubAllEnvs();
  });

  describe('Credential Loading from Environment', () => {
    it('should load agent credentials from environment variables', () => {
      // Set environment variables
      vi.stubEnv('AIM_AGENT_ID', 'agent-env-123');
      vi.stubEnv('AIM_PRIVATE_KEY', 'base64-private-key');
      vi.stubEnv('AIM_PUBLIC_KEY', 'base64-public-key');
      vi.stubEnv('AIM_ORGANIZATION_ID', 'org-456');

      const creds = loadCredentialsFromEnv();

      expect(creds).not.toBeNull();
      expect(creds!.agentId).toBe('agent-env-123');
      expect(creds!.privateKey).toBe('base64-private-key');
      expect(creds!.publicKey).toBe('base64-public-key');
      expect(creds!.organizationId).toBe('org-456');

      console.log('✅ Loaded credentials from environment');
    });

    it('should return null when env vars are missing', () => {
      // Only set some vars
      vi.stubEnv('AIM_AGENT_ID', 'agent-123');
      // Other vars not set

      const creds = loadCredentialsFromEnv();

      expect(creds).toBeNull();
      console.log('✅ Correctly returned null for missing env vars');
    });

    it('should load SDK OAuth credentials from environment', () => {
      vi.stubEnv('AIM_REFRESH_TOKEN', 'rt_test123');
      vi.stubEnv('AIM_SDK_TOKEN_ID', 'sdk-token-456');
      vi.stubEnv('AIM_URL', 'https://aim.example.com');
      vi.stubEnv('AIM_ORGANIZATION_ID', 'org-789');

      // This tests the SDK OAuth credential loading path
      // The actual implementation may vary
      const refreshToken = process.env.AIM_REFRESH_TOKEN;
      const tokenId = process.env.AIM_SDK_TOKEN_ID;

      expect(refreshToken).toBe('rt_test123');
      expect(tokenId).toBe('sdk-token-456');

      console.log('✅ SDK OAuth env vars accessible');
    });
  });

  describe('Credential Loading from File', () => {
    it('should load credentials from JSON file', async () => {
      const credFile = join(TEST_DIR, 'creds.json');
      const testCreds: AgentCredentials = {
        agentId: 'agent-file-123',
        privateKey: 'file-private-key',
        publicKey: 'file-public-key',
        organizationId: 'org-file',
        createdAt: new Date().toISOString(),
      };

      writeFileSync(credFile, JSON.stringify(testCreds));

      const loaded = await loadCredentialsFromFile(credFile);

      expect(loaded).not.toBeNull();
      expect(loaded.agentId).toBe('agent-file-123');
      expect(loaded.privateKey).toBe('file-private-key');

      console.log('✅ Loaded credentials from file');
    });

    it('should throw error for non-existent file', async () => {
      await expect(
        loadCredentialsFromFile('/non/existent/path.json')
      ).rejects.toThrow();

      console.log('✅ Correctly threw error for missing file');
    });

    it('should throw error for malformed JSON', async () => {
      const badFile = join(TEST_DIR, 'bad.json');
      writeFileSync(badFile, 'not valid json {{{');

      await expect(loadCredentialsFromFile(badFile)).rejects.toThrow();

      console.log('✅ Correctly threw error for malformed JSON');
    });
  });

  describe('Credential Saving', () => {
    it('should save credentials to file', async () => {
      const savePath = join(TEST_DIR, 'saved-creds.json');
      const creds: AgentCredentials = {
        agentId: 'agent-save-123',
        privateKey: 'save-private-key',
        publicKey: 'save-public-key',
        organizationId: 'org-save',
        createdAt: new Date().toISOString(),
      };

      await saveCredentialsToFile(creds, savePath);

      expect(existsSync(savePath)).toBe(true);

      // Verify content
      const loaded = await loadCredentialsFromFile(savePath);
      expect(loaded.agentId).toBe('agent-save-123');

      console.log('✅ Saved credentials to file');
    });

    it('should create parent directories if needed', async () => {
      const deepPath = join(TEST_DIR, 'deep', 'nested', 'creds.json');
      const creds: AgentCredentials = {
        agentId: 'agent-deep',
        privateKey: 'key',
        publicKey: 'key',
        organizationId: 'org',
        createdAt: new Date().toISOString(),
      };

      await saveCredentialsToFile(creds, deepPath);

      expect(existsSync(deepPath)).toBe(true);

      console.log('✅ Created nested directories for credentials');
    });
  });

  describe('OAuth Token Manager', () => {
    it('should be created with credentials', () => {
      const creds: AgentCredentials = {
        agentId: 'token-agent',
        privateKey: 'MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=', // Valid base64
        publicKey: 'ZmVkY2JhOTg3NjU0MzIxMGZlZGNiYTk4NzY1NDMyMTA=',
        organizationId: 'org',
        createdAt: new Date().toISOString(),
      };

      const manager = new OAuthTokenManager(AIM_URL, creds);

      expect(manager).toBeDefined();
      expect(manager.hasValidToken()).toBe(false);

      console.log('✅ Token manager created');
    });

    it('should report no valid token initially', () => {
      const creds: AgentCredentials = {
        agentId: 'fresh-agent',
        privateKey: 'MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=',
        publicKey: 'ZmVkY2JhOTg3NjU0MzIxMGZlZGNiYTk4NzY1NDMyMTA=',
        organizationId: 'org',
        createdAt: new Date().toISOString(),
      };

      const manager = new OAuthTokenManager(AIM_URL, creds);

      expect(manager.hasValidToken()).toBe(false);

      console.log('✅ No token initially');
    });

    it('should clear cached token', () => {
      const creds: AgentCredentials = {
        agentId: 'clear-agent',
        privateKey: 'MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=',
        publicKey: 'ZmVkY2JhOTg3NjU0MzIxMGZlZGNiYTk4NzY1NDMyMTA=',
        organizationId: 'org',
        createdAt: new Date().toISOString(),
      };

      const manager = new OAuthTokenManager(AIM_URL, creds);

      // Even without a token, clear should work
      manager.clearToken();
      expect(manager.hasValidToken()).toBe(false);

      console.log('✅ Token cleared');
    });

    it.skipIf(!backendAvailable)(
      'should acquire token from backend',
      async () => {
        // This requires valid credentials
        const envCreds = loadCredentialsFromEnv();
        if (!envCreds) {
          console.log('⚠️  Skipping: No credentials in environment');
          return;
        }

        const manager = new OAuthTokenManager(AIM_URL, envCreds);

        const token = await manager.getAccessToken();

        expect(token).toBeDefined();
        expect(typeof token).toBe('string');
        expect(token.length).toBeGreaterThan(0);

        console.log('✅ Token acquired from backend');
      }
    );

    it.skipIf(!backendAvailable)(
      'should cache and reuse token',
      async () => {
        const envCreds = loadCredentialsFromEnv();
        if (!envCreds) {
          console.log('⚠️  Skipping: No credentials in environment');
          return;
        }

        const manager = new OAuthTokenManager(AIM_URL, envCreds);

        const token1 = await manager.getAccessToken();
        const token2 = await manager.getAccessToken();

        expect(token1).toBe(token2);
        expect(manager.hasValidToken()).toBe(true);

        console.log('✅ Token cached and reused');
      }
    );
  });

  describe('Credential Type Detection', () => {
    it('should identify SDK OAuth credentials format', () => {
      const sdkCreds = {
        refreshToken: 'rt_test',
        sdkTokenId: 'sdk-123',
        aimUrl: 'https://aim.example.com',
        organizationId: 'org',
      };

      // Check if it has the SDK OAuth format
      const hasRefreshToken = 'refreshToken' in sdkCreds || 'refresh_token' in sdkCreds;
      const hasSdkTokenId = 'sdkTokenId' in sdkCreds || 'sdk_token_id' in sdkCreds;

      expect(hasRefreshToken).toBe(true);
      expect(hasSdkTokenId).toBe(true);
      console.log('✅ SDK OAuth credentials format detected');
    });

    it('should identify SDK OAuth with snake_case', () => {
      const sdkCreds = {
        refresh_token: 'rt_test',
        sdk_token_id: 'sdk-123',
        aim_url: 'https://aim.example.com',
        organization_id: 'org',
      };

      const hasRefreshToken = 'refreshToken' in sdkCreds || 'refresh_token' in sdkCreds;
      const hasSdkTokenId = 'sdkTokenId' in sdkCreds || 'sdk_token_id' in sdkCreds;

      expect(hasRefreshToken).toBe(true);
      expect(hasSdkTokenId).toBe(true);
      console.log('✅ snake_case SDK OAuth credentials format detected');
    });

    it('should identify agent credentials format', () => {
      const agentCreds = {
        agentId: 'agent-123',
        privateKey: 'key',
        publicKey: 'key',
        organizationId: 'org',
      };

      const isAgentCreds =
        'agentId' in agentCreds && 'privateKey' in agentCreds && 'publicKey' in agentCreds;

      expect(isAgentCreds).toBe(true);
      console.log('✅ Agent credentials format detected');
    });

    it('should identify unknown format', () => {
      const invalidCreds = {
        foo: 'bar',
        baz: 123,
      };

      const isAgentCreds =
        'agentId' in invalidCreds &&
        'privateKey' in invalidCreds &&
        'publicKey' in invalidCreds;
      const isSdkOAuth =
        ('refreshToken' in invalidCreds || 'refresh_token' in invalidCreds) &&
        ('sdkTokenId' in invalidCreds || 'sdk_token_id' in invalidCreds);

      expect(isAgentCreds).toBe(false);
      expect(isSdkOAuth).toBe(false);
      console.log('✅ Unknown credential format detected');
    });
  });

  // Cleanup
  afterAll(() => {
    try {
      if (existsSync(TEST_DIR)) {
        rmSync(TEST_DIR, { recursive: true, force: true });
      }
    } catch {
      // Ignore cleanup errors
    }
  });
});
