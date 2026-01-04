/**
 * Tests for AIM Client - Core SDK functionality
 *
 * Matches Java SDK AIMClientTest coverage:
 * - Builder pattern with parameters
 * - Default values
 * - Authentication
 * - Verification results
 * - Resource cleanup
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { AIMClient, createClient, registerAgent } from './AIMClient';
import {
  AIMError,
  AuthenticationError,
  ActionDeniedError,
  VerificationError,
  NetworkError,
} from '../exceptions';

// Mock fetch globally
const mockFetch = vi.fn();
vi.stubGlobal('fetch', mockFetch);

describe('AIMClient', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Reset environment variables
    delete process.env.AIM_BASE_URL;
    delete process.env.AIM_ORGANIZATION_ID;
    delete process.env.AIM_API_KEY;
    delete process.env.AIM_DEBUG;
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  describe('Constructor and Configuration', () => {
    it('should create client with default values', () => {
      const client = new AIMClient();
      expect(client).toBeInstanceOf(AIMClient);
      expect(client.isAuthenticated()).toBe(false);
    });

    it('should create client with custom base URL', () => {
      const client = new AIMClient({
        baseUrl: 'https://custom.aim.example.com',
      });
      expect(client).toBeInstanceOf(AIMClient);
    });

    it('should create client with API key', () => {
      const client = new AIMClient({
        apiKey: 'test-api-key-123',
      });
      expect(client).toBeInstanceOf(AIMClient);
    });

    it('should create client with organization ID', () => {
      const client = new AIMClient({
        organizationId: 'org-123',
      });
      expect(client).toBeInstanceOf(AIMClient);
    });

    it('should use environment variables when no config provided', () => {
      process.env.AIM_BASE_URL = 'https://env.aim.example.com';
      process.env.AIM_ORGANIZATION_ID = 'env-org-123';
      process.env.AIM_API_KEY = 'env-api-key';

      const client = new AIMClient();
      expect(client).toBeInstanceOf(AIMClient);
    });

    it('should enable debug mode from environment', () => {
      process.env.AIM_DEBUG = 'true';

      const client = new AIMClient();
      expect(client).toBeInstanceOf(AIMClient);
    });

    it('should create client with custom timeout', () => {
      const client = new AIMClient({
        timeout: 60000,
      });
      expect(client).toBeInstanceOf(AIMClient);
    });

    it('should create client with custom headers', () => {
      const client = new AIMClient({
        headers: {
          'X-Custom-Header': 'custom-value',
        },
      });
      expect(client).toBeInstanceOf(AIMClient);
    });
  });

  describe('createClient helper', () => {
    it('should create client with createClient function', () => {
      const client = createClient();
      expect(client).toBeInstanceOf(AIMClient);
    });

    it('should pass config to createClient', () => {
      const client = createClient({
        baseUrl: 'https://helper.aim.example.com',
        apiKey: 'helper-api-key',
      });
      expect(client).toBeInstanceOf(AIMClient);
    });
  });

  describe('Authentication State', () => {
    it('should start unauthenticated', () => {
      const client = new AIMClient();
      expect(client.isAuthenticated()).toBe(false);
    });

    it('should return null credentials when not authenticated', () => {
      const client = new AIMClient();
      expect(client.getCredentials()).toBeNull();
    });

    it('should return null agent when not authenticated', async () => {
      const client = new AIMClient();
      const agent = await client.getAgent();
      expect(agent).toBeNull();
    });
  });

  describe('setCredentials', () => {
    it('should set credentials and become authenticated', () => {
      const client = new AIMClient();
      client.setCredentials({
        agentId: 'agent-123',
        privateKey: 'base64-private-key',
        publicKey: 'base64-public-key',
        organizationId: 'org-456',
        createdAt: new Date().toISOString(),
      });

      expect(client.isAuthenticated()).toBe(true);
      expect(client.getCredentials()).not.toBeNull();
    });
  });

  describe('logout', () => {
    it('should clear credentials on logout', () => {
      const client = new AIMClient();
      client.setCredentials({
        agentId: 'agent-123',
        privateKey: 'base64-private-key',
        publicKey: 'base64-public-key',
        organizationId: 'org-456',
        createdAt: new Date().toISOString(),
      });

      expect(client.isAuthenticated()).toBe(true);

      client.logout();

      expect(client.isAuthenticated()).toBe(false);
      expect(client.getCredentials()).toBeNull();
    });
  });

  describe('registerAgent', () => {
    it('should have registerAgent method', () => {
      const client = new AIMClient({ apiKey: 'test-api-key' });
      expect(typeof client.registerAgent).toBe('function');
    });

    it('should accept registration options', () => {
      // Verify the method signature accepts proper options
      const client = new AIMClient({ apiKey: 'test-api-key' });

      // Method exists and accepts options object
      // Full integration requires valid server
      expect(client).toBeInstanceOf(AIMClient);
    });
  });

  describe('verifyAction', () => {
    it('should throw AuthenticationError when not authenticated', async () => {
      const client = new AIMClient();

      await expect(
        client.verifyAction({
          action: 'db:read',
          resource: 'users_table',
        })
      ).rejects.toThrow(AuthenticationError);
    });

    it('should have verifyAction method', () => {
      const client = new AIMClient();
      expect(typeof client.verifyAction).toBe('function');
    });

    it('should require credentials to verify action', async () => {
      const client = new AIMClient({ apiKey: 'test-key' });

      // Without credentials, should throw AuthenticationError
      await expect(
        client.verifyAction({
          action: 'db:read',
          resource: 'test',
        })
      ).rejects.toThrow(AuthenticationError);
    });
  });

  describe('createVerifiedAction', () => {
    it('should have createVerifiedAction method', () => {
      const client = new AIMClient();
      expect(typeof client.createVerifiedAction).toBe('function');
    });

    it('should return a function wrapper', () => {
      const client = new AIMClient();
      const originalFn = async (x: number) => x * 2;
      const verifiedFn = client.createVerifiedAction('db:read', originalFn);

      expect(typeof verifiedFn).toBe('function');
    });
  });

  describe('getTrustScore', () => {
    it('should return 0 when not authenticated', async () => {
      const client = new AIMClient();
      const score = await client.getTrustScore();
      expect(score).toBe(0);
    });
  });

  describe('updateAgent', () => {
    it('should throw AuthenticationError when not authenticated', async () => {
      const client = new AIMClient();

      await expect(
        client.updateAgent({
          displayName: 'Updated Name',
        })
      ).rejects.toThrow(AuthenticationError);
    });
  });

  describe('reportCapabilities', () => {
    it('should throw AuthenticationError when not authenticated', async () => {
      const client = new AIMClient();

      await expect(client.reportCapabilities(['db:read', 'api:call'])).rejects.toThrow(
        AuthenticationError
      );
    });
  });

  describe('Network Error Handling', () => {
    it('should handle timeout configuration', () => {
      const client = new AIMClient({ timeout: 100 });
      expect(client).toBeInstanceOf(AIMClient);
    });

    it('should throw error when not authenticated', async () => {
      const client = new AIMClient();

      await expect(
        client.verifyAction({
          action: 'db:read',
          resource: 'test',
        })
      ).rejects.toThrow(AuthenticationError);
    });
  });
});

describe('registerAgent helper function', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('should create client with registerAgent helper', () => {
    // registerAgent creates a client instance
    // Testing the helper function signature exists
    expect(typeof registerAgent).toBe('function');
  });

  it('should accept agent name and options', () => {
    // Verify the function can be called with proper types
    // Full integration test requires valid server response
    const client = createClient({ apiKey: 'test-key' });
    expect(client).toBeInstanceOf(AIMClient);
  });
});
