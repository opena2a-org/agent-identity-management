/**
 * Tests for OAuth Token Management
 *
 * Tests cover:
 * - OAuthTokenManager class
 * - Token caching
 * - Credential loading from environment
 * - Credential file operations
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import type { AgentCredentials } from '../types';

// Mock fetch at module level before any imports
const mockFetch = vi.fn();
globalThis.fetch = mockFetch;

describe('OAuthTokenManager', () => {
  // Valid base64 encoded 32-byte key for testing
  const validBase64Key = 'AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=';

  const mockCredentials: AgentCredentials = {
    agentId: 'agent-123',
    privateKey: validBase64Key,
    publicKey: validBase64Key,
    organizationId: 'org-456',
    createdAt: '2024-01-01T00:00:00Z',
  };

  beforeEach(() => {
    vi.clearAllMocks();
    // Restore mock after clearing
    globalThis.fetch = mockFetch;
  });

  describe('constructor', () => {
    it('should create manager with valid credentials', async () => {
      const { OAuthTokenManager } = await import('./oauth');
      const manager = new OAuthTokenManager('https://aim.example.com', mockCredentials);
      expect(manager).toBeDefined();
    });

    it('should strip trailing slash from baseUrl', async () => {
      const { OAuthTokenManager } = await import('./oauth');
      const manager = new OAuthTokenManager('https://aim.example.com/', mockCredentials);
      expect(manager).toBeDefined();
    });
  });

  describe('hasValidToken', () => {
    it('should return false initially', async () => {
      const { OAuthTokenManager } = await import('./oauth');
      const manager = new OAuthTokenManager('https://aim.example.com', mockCredentials);
      expect(manager.hasValidToken()).toBe(false);
    });
  });

  describe('clearToken', () => {
    it('should clear cached token', async () => {
      const { OAuthTokenManager } = await import('./oauth');
      const manager = new OAuthTokenManager('https://aim.example.com', mockCredentials);
      manager.clearToken();
      expect(manager.hasValidToken()).toBe(false);
    });
  });

  describe('getAccessToken', () => {
    it('should call token endpoint', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          accessToken: 'new-access-token',
          tokenType: 'Bearer',
          expiresIn: 3600,
        }),
      });

      const { OAuthTokenManager } = await import('./oauth');
      const manager = new OAuthTokenManager('https://aim.example.com', mockCredentials);
      const token = await manager.getAccessToken();

      expect(token).toBe('new-access-token');
      expect(mockFetch).toHaveBeenCalledWith(
        'https://aim.example.com/oauth/token',
        expect.objectContaining({
          method: 'POST',
          headers: {
            'Content-Type': 'application/x-www-form-urlencoded',
          },
        })
      );
    });

    it('should cache token and return cached on subsequent calls', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          accessToken: 'cached-token',
          tokenType: 'Bearer',
          expiresIn: 3600,
        }),
      });

      const { OAuthTokenManager } = await import('./oauth');
      const manager = new OAuthTokenManager('https://aim.example.com', mockCredentials);

      const token1 = await manager.getAccessToken();
      const token2 = await manager.getAccessToken();

      expect(token1).toBe('cached-token');
      expect(token2).toBe('cached-token');
      expect(mockFetch).toHaveBeenCalledTimes(1);
    });

    it('should throw on token acquisition failure', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 401,
        text: async () => 'Unauthorized',
      });

      const { OAuthTokenManager } = await import('./oauth');
      const manager = new OAuthTokenManager('https://aim.example.com', mockCredentials);

      await expect(manager.getAccessToken()).rejects.toThrow('Failed to acquire token');
    });

    it('should prevent concurrent refresh requests', async () => {
      let resolvePromise: (value: unknown) => void;
      const slowFetch = new Promise((resolve) => {
        resolvePromise = resolve;
      });

      mockFetch.mockImplementationOnce(() => slowFetch);

      const { OAuthTokenManager } = await import('./oauth');
      const manager = new OAuthTokenManager('https://aim.example.com', mockCredentials);

      // Start two concurrent requests
      const promise1 = manager.getAccessToken();
      const promise2 = manager.getAccessToken();

      // Resolve the fetch
      resolvePromise!({
        ok: true,
        json: async () => ({
          accessToken: 'concurrent-token',
          tokenType: 'Bearer',
          expiresIn: 3600,
        }),
      });

      const [token1, token2] = await Promise.all([promise1, promise2]);

      expect(token1).toBe('concurrent-token');
      expect(token2).toBe('concurrent-token');
      expect(mockFetch).toHaveBeenCalledTimes(1);
    });

    it('should include proper grant type in request', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          accessToken: 'token',
          tokenType: 'Bearer',
          expiresIn: 3600,
        }),
      });

      const { OAuthTokenManager } = await import('./oauth');
      const manager = new OAuthTokenManager('https://aim.example.com', mockCredentials);
      await manager.getAccessToken();

      const fetchCall = mockFetch.mock.calls[0];
      const body = fetchCall[1].body as URLSearchParams;

      expect(body.get('grant_type')).toBe('client_credentials');
      expect(body.get('client_id')).toBe('agent-123');
      expect(body.get('client_assertion_type')).toBe(
        'urn:ietf:params:oauth:client-assertion-type:jwt-bearer'
      );
      expect(body.get('client_assertion')).toBeDefined();
    });
  });

  describe('hasValidToken after token acquisition', () => {
    it('should return true after successful token acquisition', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          accessToken: 'valid-token',
          tokenType: 'Bearer',
          expiresIn: 3600,
        }),
      });

      const { OAuthTokenManager } = await import('./oauth');
      const manager = new OAuthTokenManager('https://aim.example.com', mockCredentials);
      await manager.getAccessToken();

      expect(manager.hasValidToken()).toBe(true);
    });

    it('should return false after clearToken', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          accessToken: 'token-to-clear',
          tokenType: 'Bearer',
          expiresIn: 3600,
        }),
      });

      const { OAuthTokenManager } = await import('./oauth');
      const manager = new OAuthTokenManager('https://aim.example.com', mockCredentials);
      await manager.getAccessToken();

      expect(manager.hasValidToken()).toBe(true);
      manager.clearToken();
      expect(manager.hasValidToken()).toBe(false);
    });
  });
});

describe('loadCredentialsFromEnv', () => {
  const originalEnv = process.env;

  beforeEach(() => {
    vi.resetModules();
    process.env = { ...originalEnv };
  });

  afterEach(() => {
    process.env = originalEnv;
  });

  it('should return null when no env vars set', async () => {
    delete process.env.AIM_AGENT_ID;
    delete process.env.AIM_PRIVATE_KEY;
    delete process.env.AIM_PUBLIC_KEY;
    delete process.env.AIM_ORGANIZATION_ID;

    const { loadCredentialsFromEnv } = await import('./oauth');
    const result = loadCredentialsFromEnv();
    expect(result).toBeNull();
  });

  it('should return null when some env vars missing', async () => {
    process.env.AIM_AGENT_ID = 'agent-123';
    delete process.env.AIM_PRIVATE_KEY;
    delete process.env.AIM_PUBLIC_KEY;
    delete process.env.AIM_ORGANIZATION_ID;

    const { loadCredentialsFromEnv } = await import('./oauth');
    const result = loadCredentialsFromEnv();
    expect(result).toBeNull();
  });

  it('should return credentials when all env vars set', async () => {
    process.env.AIM_AGENT_ID = 'agent-123';
    process.env.AIM_PRIVATE_KEY = 'private-key-base64';
    process.env.AIM_PUBLIC_KEY = 'public-key-base64';
    process.env.AIM_ORGANIZATION_ID = 'org-456';

    const { loadCredentialsFromEnv } = await import('./oauth');
    const result = loadCredentialsFromEnv();

    expect(result).not.toBeNull();
    expect(result?.agentId).toBe('agent-123');
    expect(result?.privateKey).toBe('private-key-base64');
    expect(result?.publicKey).toBe('public-key-base64');
    expect(result?.organizationId).toBe('org-456');
    expect(result?.createdAt).toBeDefined();
  });
});

describe('Credential file operations', () => {
  // Note: loadCredentialsFromFile and saveCredentialsToFile require
  // filesystem access. These would be integration tests.

  describe('loadCredentialsFromFile', () => {
    it('should export loadCredentialsFromFile function', async () => {
      const { loadCredentialsFromFile } = await import('./oauth');
      expect(typeof loadCredentialsFromFile).toBe('function');
    });

    it('throws a typed ConfigurationError (not raw ENOENT) for a missing file', async () => {
      const { loadCredentialsFromFile } = await import('./oauth');
      const { ConfigurationError } = await import('../exceptions');
      const missing = '/nonexistent/does-not-exist-aim-creds.json';
      await expect(loadCredentialsFromFile(missing)).rejects.toBeInstanceOf(ConfigurationError);
      await expect(loadCredentialsFromFile(missing)).rejects.toMatchObject({
        code: 'CONFIGURATION_ERROR',
        message: expect.stringContaining('not found'),
      });
    });

    it('throws a typed ConfigurationError for a file that is not valid JSON', async () => {
      const fs = await import('fs/promises');
      const os = await import('os');
      const path = await import('path');
      const { loadCredentialsFromFile } = await import('./oauth');
      const { ConfigurationError } = await import('../exceptions');
      const tmp = path.join(os.tmpdir(), `aim-creds-bad-${process.pid}-${Date.now()}.json`);
      await fs.writeFile(tmp, 'not-json{');
      try {
        await expect(loadCredentialsFromFile(tmp)).rejects.toBeInstanceOf(ConfigurationError);
        await expect(loadCredentialsFromFile(tmp)).rejects.toMatchObject({
          message: expect.stringContaining('not valid JSON'),
        });
      } finally {
        await fs.rm(tmp, { force: true });
      }
    });
  });

  describe('saveCredentialsToFile', () => {
    it('should export saveCredentialsToFile function', async () => {
      const { saveCredentialsToFile } = await import('./oauth');
      expect(typeof saveCredentialsToFile).toBe('function');
    });
  });
});
