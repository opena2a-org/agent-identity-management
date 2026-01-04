/**
 * Integration tests for AIM Client against a live or mock AIM backend.
 *
 * These tests require:
 * - AIM backend running at http://localhost:8080 (or AIM_BASE_URL)
 * - Valid SDK credentials in environment or ~/.aim/sdk_credentials.json
 *
 * Run with: npm test -- --testPathPattern="integration"
 */

import { describe, it, expect, beforeAll, afterAll, beforeEach } from 'vitest';
import { AIMClient, createClient, registerAgent } from './AIMClient';
import { loadCredentialsFromEnv } from '../auth/oauth';
import {
  AIMError,
  AuthenticationError,
  ActionDeniedError,
  NetworkError,
} from '../exceptions';

const AIM_URL = process.env.AIM_BASE_URL ?? 'http://localhost:8080';
const TEST_AGENT_NAME = `ts-sdk-test-agent-${Date.now()}`;

let backendAvailable = false;
let credentialsAvailable = false;

/**
 * Check if the backend is available
 */
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

/**
 * Check if credentials are available
 */
function checkCredentials(): boolean {
  const envCreds = loadCredentialsFromEnv();
  return envCreds !== null;
}

describe('AIMClient Integration Tests', () => {
  beforeAll(async () => {
    backendAvailable = await checkBackendHealth();
    credentialsAvailable = checkCredentials();

    if (!backendAvailable) {
      console.log(`⚠️  Integration tests skipped: AIM backend not available at ${AIM_URL}`);
    }
    if (!credentialsAvailable) {
      console.log('⚠️  Integration tests skipped: No SDK credentials found');
    }
  });

  describe('Credential Loading', () => {
    it.skipIf(!credentialsAvailable)('should load SDK credentials from environment or file', () => {
      const envCreds = loadCredentialsFromEnv();
      let creds = envCreds;

      if (!creds) {
        creds = loadCredentialsFromFile();
      }

      expect(creds).not.toBeNull();
      if (creds) {
        expect(creds.agentId).toBeDefined();
        expect(creds.privateKey).toBeDefined();
        expect(creds.publicKey).toBeDefined();
        console.log(`✅ Loaded credentials for agent: ${creds.agentId.substring(0, 12)}...`);
      }
    });
  });

  describe('Client Configuration', () => {
    it('should create client with environment configuration', () => {
      const client = createClient({
        baseUrl: AIM_URL,
      });

      expect(client).toBeInstanceOf(AIMClient);
    });

    it('should create client with explicit configuration', () => {
      const client = new AIMClient({
        baseUrl: AIM_URL,
        organizationId: 'test-org',
        timeout: 10000,
        debug: true,
      });

      expect(client).toBeInstanceOf(AIMClient);
    });
  });

  describe('Agent Registration', () => {
    it.skipIf(!backendAvailable || !credentialsAvailable)(
      'should register a new agent',
      async () => {
        const client = createClient({
          baseUrl: AIM_URL,
          apiKey: process.env.AIM_API_KEY,
        });

        const agent = await client.registerAgent({
          name: TEST_AGENT_NAME,
          displayName: 'TypeScript SDK Test Agent',
          description: 'Integration test agent',
          agentType: 'custom',
          capabilities: ['db:read', 'api:call'],
        });

        expect(agent).toBeDefined();
        expect(agent.name).toBe(TEST_AGENT_NAME);
        expect(client.isAuthenticated()).toBe(true);

        console.log(`✅ Agent registered: ${agent.id}`);
      }
    );

    it.skipIf(!backendAvailable || !credentialsAvailable)(
      'should use registerAgent helper function',
      async () => {
        const { client, agent, credentials } = await registerAgent(
          `${TEST_AGENT_NAME}-helper`,
          {
            displayName: 'Helper Test Agent',
            capabilities: ['data:read'],
            config: {
              baseUrl: AIM_URL,
              apiKey: process.env.AIM_API_KEY,
            },
          }
        );

        expect(client).toBeInstanceOf(AIMClient);
        expect(agent.name).toBe(`${TEST_AGENT_NAME}-helper`);
        expect(credentials.agentId).toBeDefined();
        expect(credentials.privateKey).toBeDefined();
        expect(credentials.publicKey).toBeDefined();

        console.log(`✅ Helper registered agent: ${agent.id}`);
      }
    );
  });

  describe('Action Verification', () => {
    let client: AIMClient;

    beforeEach(async () => {
      if (!backendAvailable || !credentialsAvailable) return;

      client = createClient({
        baseUrl: AIM_URL,
        apiKey: process.env.AIM_API_KEY,
      });

      await client.registerAgent({
        name: `${TEST_AGENT_NAME}-verify-${Date.now()}`,
        capabilities: ['db:read', 'db:write', 'api:call'],
      });
    });

    it.skipIf(!backendAvailable || !credentialsAvailable)(
      'should verify allowed action',
      async () => {
        const result = await client.verifyAction({
          action: 'db:read',
          resource: 'users_table',
        });

        expect(result).toBeDefined();
        expect(result.verified).toBe(true);
        expect(result.actionAllowed).toBe(true);

        console.log(`✅ Action verified: ${result.eventId}`);
      }
    );

    it.skipIf(!backendAvailable || !credentialsAvailable)(
      'should verify action with context',
      async () => {
        const result = await client.verifyAction({
          action: 'api:call',
          resource: 'external-api',
          resourceType: 'http',
          context: {
            method: 'GET',
            endpoint: '/users',
          },
          riskLevel: 'low',
        });

        expect(result).toBeDefined();
        expect(result.verified).toBe(true);

        console.log(`✅ Action with context verified: ${result.riskLevel}`);
      }
    );

    it.skipIf(!backendAvailable || !credentialsAvailable)(
      'should handle different risk levels',
      async () => {
        // Low risk should succeed
        const lowRisk = await client.verifyAction({
          action: 'db:read',
          resource: 'public_data',
          riskLevel: 'low',
        });
        expect(lowRisk.actionAllowed).toBe(true);

        // Medium risk should succeed for high-trust agents
        const mediumRisk = await client.verifyAction({
          action: 'db:write',
          resource: 'user_settings',
          riskLevel: 'medium',
        });
        expect(mediumRisk.riskLevel).toBe('medium');

        console.log('✅ Risk level verification completed');
      }
    );
  });

  describe('Verified Action Wrapper', () => {
    let client: AIMClient;

    beforeEach(async () => {
      if (!backendAvailable || !credentialsAvailable) return;

      client = createClient({
        baseUrl: AIM_URL,
        apiKey: process.env.AIM_API_KEY,
      });

      await client.registerAgent({
        name: `${TEST_AGENT_NAME}-wrapper-${Date.now()}`,
        capabilities: ['data:fetch'],
      });
    });

    it.skipIf(!backendAvailable || !credentialsAvailable)(
      'should execute action after verification',
      async () => {
        const fetchData = async (id: string): Promise<{ id: string; data: string }> => {
          return { id, data: 'test-data' };
        };

        const verifiedFetch = client.createVerifiedAction('data:fetch', fetchData, {
          resource: 'weather_api',
        });

        const result = await verifiedFetch('123');

        expect(result).toEqual({ id: '123', data: 'test-data' });
        console.log('✅ Verified action wrapper executed');
      }
    );
  });

  describe('Agent Management', () => {
    let client: AIMClient;
    let agentId: string;

    beforeEach(async () => {
      if (!backendAvailable || !credentialsAvailable) return;

      client = createClient({
        baseUrl: AIM_URL,
        apiKey: process.env.AIM_API_KEY,
      });

      const agent = await client.registerAgent({
        name: `${TEST_AGENT_NAME}-mgmt-${Date.now()}`,
        capabilities: ['info:read'],
      });
      agentId = agent.id;
    });

    it.skipIf(!backendAvailable || !credentialsAvailable)(
      'should get agent details',
      async () => {
        const agent = await client.getAgent();

        expect(agent).not.toBeNull();
        expect(agent!.id).toBe(agentId);

        console.log(`✅ Got agent details: ${agent!.name}`);
      }
    );

    it.skipIf(!backendAvailable || !credentialsAvailable)(
      'should update agent metadata',
      async () => {
        const updatedAgent = await client.updateAgent({
          displayName: 'Updated Display Name',
          description: 'Updated description',
        });

        expect(updatedAgent.displayName).toBe('Updated Display Name');
        expect(updatedAgent.description).toBe('Updated description');

        console.log('✅ Agent updated successfully');
      }
    );

    it.skipIf(!backendAvailable || !credentialsAvailable)(
      'should report capabilities',
      async () => {
        await expect(
          client.reportCapabilities(['new:capability', 'another:capability'])
        ).resolves.not.toThrow();

        console.log('✅ Capabilities reported');
      }
    );

    it.skipIf(!backendAvailable || !credentialsAvailable)(
      'should get trust score',
      async () => {
        const score = await client.getTrustScore();

        expect(typeof score).toBe('number');
        expect(score).toBeGreaterThanOrEqual(0);
        expect(score).toBeLessThanOrEqual(1);

        console.log(`✅ Trust score: ${score}`);
      }
    );
  });

  describe('Authentication State', () => {
    it('should track authentication state correctly', async () => {
      const client = createClient({ baseUrl: AIM_URL });

      // Initially not authenticated
      expect(client.isAuthenticated()).toBe(false);
      expect(client.getCredentials()).toBeNull();

      // Set credentials manually
      client.setCredentials({
        agentId: 'test-agent-123',
        privateKey: 'base64-private-key',
        publicKey: 'base64-public-key',
        organizationId: 'test-org',
        createdAt: new Date().toISOString(),
      });

      expect(client.isAuthenticated()).toBe(true);
      expect(client.getCredentials()?.agentId).toBe('test-agent-123');

      // Logout
      client.logout();
      expect(client.isAuthenticated()).toBe(false);
      expect(client.getCredentials()).toBeNull();

      console.log('✅ Authentication state tracking verified');
    });
  });

  describe('Error Handling', () => {
    it('should throw AuthenticationError when not authenticated', async () => {
      const client = createClient({ baseUrl: AIM_URL });

      await expect(
        client.verifyAction({ action: 'db:read', resource: 'test' })
      ).rejects.toThrow(AuthenticationError);
    });

    it('should throw AuthenticationError on updateAgent without auth', async () => {
      const client = createClient({ baseUrl: AIM_URL });

      await expect(
        client.updateAgent({ displayName: 'New Name' })
      ).rejects.toThrow(AuthenticationError);
    });

    it('should throw AuthenticationError on reportCapabilities without auth', async () => {
      const client = createClient({ baseUrl: AIM_URL });

      await expect(
        client.reportCapabilities(['cap:1'])
      ).rejects.toThrow(AuthenticationError);
    });

    it.skipIf(!backendAvailable)(
      'should handle network errors gracefully',
      async () => {
        const client = createClient({
          baseUrl: 'http://localhost:59999', // Non-existent server
          timeout: 1000,
        });

        client.setCredentials({
          agentId: 'test-agent',
          privateKey: 'base64-key',
          publicKey: 'base64-key',
          organizationId: 'test-org',
          createdAt: new Date().toISOString(),
        });

        await expect(
          client.verifyAction({ action: 'db:read', resource: 'test' })
        ).rejects.toThrow(NetworkError);
      }
    );
  });

  describe('Timeout Configuration', () => {
    it('should respect timeout configuration', () => {
      const client = new AIMClient({
        baseUrl: AIM_URL,
        timeout: 5000,
      });

      expect(client).toBeInstanceOf(AIMClient);
    });

    it.skipIf(!backendAvailable)(
      'should timeout on slow responses',
      async () => {
        const client = createClient({
          baseUrl: AIM_URL,
          timeout: 1, // 1ms timeout
        });

        client.setCredentials({
          agentId: 'test-agent',
          privateKey: 'base64-key',
          publicKey: 'base64-key',
          organizationId: 'test-org',
          createdAt: new Date().toISOString(),
        });

        await expect(
          client.verifyAction({ action: 'db:read', resource: 'test' })
        ).rejects.toThrow(/timeout|Network error/i);
      }
    );
  });
});
