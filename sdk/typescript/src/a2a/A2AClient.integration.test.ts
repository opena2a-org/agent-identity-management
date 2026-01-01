/**
 * Integration tests for A2A Client against a live or mock AIM backend.
 *
 * Tests the Agent-to-Agent protocol including:
 * - Agent card operations
 * - Skill discovery and routing
 * - Trust score management
 * - Consent management (GDPR/PSD2)
 * - Security policy enforcement
 * - Skill attestation and consensus
 */

import { describe, it, expect, beforeAll, beforeEach } from 'vitest';
import { A2AClient } from './A2AClient';
import { AIMClient, createClient } from '../client/AIMClient';
import { loadCredentialsFromEnv, loadCredentialsFromFile } from '../auth/oauth';
import type {
  A2AAgentCard,
  A2ASkill,
  A2ATrustScore,
  A2APeerTrust,
  A2AConsent,
  A2ASecurityCheckResult,
  A2ASecuritySettings,
  A2ASkillAttestation,
  A2AConsensusResult,
  CapableAgent,
} from './types';

const AIM_URL = process.env.AIM_BASE_URL ?? 'http://localhost:8080';
const TEST_AGENT_NAME = `ts-a2a-test-${Date.now()}`;

let backendAvailable = false;
let credentialsAvailable = false;

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

function checkCredentials(): boolean {
  const envCreds = loadCredentialsFromEnv();
  if (envCreds) return true;

  try {
    const fileCreds = loadCredentialsFromFile();
    return fileCreds !== null;
  } catch {
    return false;
  }
}

describe('A2AClient Integration Tests', () => {
  let aimClient: AIMClient;
  let a2aClient: A2AClient;

  beforeAll(async () => {
    backendAvailable = await checkBackendHealth();
    credentialsAvailable = checkCredentials();

    if (!backendAvailable) {
      console.log(`⚠️  A2A integration tests skipped: Backend not available at ${AIM_URL}`);
    }
    if (!credentialsAvailable) {
      console.log('⚠️  A2A integration tests skipped: No SDK credentials found');
    }
  });

  beforeEach(async () => {
    if (!backendAvailable || !credentialsAvailable) return;

    aimClient = createClient({
      baseUrl: AIM_URL,
      apiKey: process.env.AIM_API_KEY,
    });

    await aimClient.registerAgent({
      name: `${TEST_AGENT_NAME}-${Date.now()}`,
      displayName: 'A2A Test Agent',
      capabilities: ['data-analysis', 'reporting'],
    });

    a2aClient = new A2AClient(aimClient);
  });

  describe('Agent Card Operations', () => {
    it.skipIf(!backendAvailable || !credentialsAvailable)(
      'should publish and retrieve agent card',
      async () => {
        const skills: A2ASkill[] = [
          {
            id: 'data-analysis',
            name: 'Data Analysis',
            description: 'Analyzes data sets and provides insights',
            inputSchema: {
              type: 'object',
              properties: {
                data: { type: 'array' },
              },
            },
            outputSchema: {
              type: 'object',
              properties: {
                insights: { type: 'array' },
              },
            },
          },
        ];

        // Publish card
        const card = await a2aClient.publishAgentCard({
          url: 'https://agent.example.com',
          skills,
        });

        expect(card).toBeDefined();
        expect(card.cardUrl).toContain('.well-known/agent.json');

        console.log(`✅ Published agent card: ${card.cardUrl}`);

        // Retrieve card
        const credentials = aimClient.getCredentials();
        const retrievedCard = await a2aClient.getAgentCard(credentials!.agentId);

        expect(retrievedCard).toBeDefined();
        expect(retrievedCard.skills).toHaveLength(skills.length);

        console.log('✅ Retrieved agent card');
      }
    );

    it.skipIf(!backendAvailable || !credentialsAvailable)(
      'should update agent card with new skills',
      async () => {
        const initialSkills: A2ASkill[] = [
          {
            id: 'initial-skill',
            name: 'Initial Skill',
            description: 'Initial skill description',
          },
        ];

        await a2aClient.publishAgentCard({
          url: 'https://agent.example.com',
          skills: initialSkills,
        });

        const newSkills: A2ASkill[] = [
          ...initialSkills,
          {
            id: 'new-skill',
            name: 'New Skill',
            description: 'Added new capability',
          },
        ];

        const updatedCard = await a2aClient.updateAgentCard({
          skills: newSkills,
        });

        expect(updatedCard.skills).toHaveLength(2);
        console.log('✅ Updated agent card with new skills');
      }
    );
  });

  describe('Skill Discovery', () => {
    it.skipIf(!backendAvailable || !credentialsAvailable)(
      'should route by intent',
      async () => {
        const agents = await a2aClient.routeByIntent('analyze data and generate report');

        expect(Array.isArray(agents)).toBe(true);
        // May return empty if no matching agents
        console.log(`✅ Found ${agents.length} agents matching intent`);
      }
    );

    it.skipIf(!backendAvailable || !credentialsAvailable)(
      'should get capable agents by skill',
      async () => {
        const agents = await a2aClient.getCapableAgents(['data-analysis']);

        expect(Array.isArray(agents)).toBe(true);
        if (agents.length > 0) {
          expect(agents[0].matchingSkills).toBeDefined();
        }
        console.log(`✅ Found ${agents.length} capable agents`);
      }
    );

    it.skipIf(!backendAvailable || !credentialsAvailable)(
      'should filter by minimum trust score',
      async () => {
        const highTrustAgents = await a2aClient.getCapableAgents(['data-analysis'], 0.8);

        expect(Array.isArray(highTrustAgents)).toBe(true);
        console.log(`✅ Found ${highTrustAgents.length} high-trust agents`);
      }
    );
  });

  describe('Trust Score Management', () => {
    it.skipIf(!backendAvailable || !credentialsAvailable)(
      'should get trust score for agent',
      async () => {
        const credentials = aimClient.getCredentials();
        const trustScore = await a2aClient.getTrustScore(credentials!.agentId);

        expect(trustScore).toBeDefined();
        expect(typeof trustScore.score).toBe('number');
        expect(trustScore.score).toBeGreaterThanOrEqual(0);
        expect(trustScore.score).toBeLessThanOrEqual(1);

        console.log(`✅ Trust score: ${trustScore.score}`);
      }
    );

    it.skipIf(!backendAvailable || !credentialsAvailable)(
      'should list peer trusts',
      async () => {
        const peers = await a2aClient.listPeerTrusts();

        expect(Array.isArray(peers)).toBe(true);
        console.log(`✅ Found ${peers.length} trusted peers`);
      }
    );

    it.skipIf(!backendAvailable || !credentialsAvailable)(
      'should update peer trust after interaction',
      async () => {
        // First, need another agent to track
        const peerClient = createClient({
          baseUrl: AIM_URL,
          apiKey: process.env.AIM_API_KEY,
        });

        const peerAgent = await peerClient.registerAgent({
          name: `${TEST_AGENT_NAME}-peer-${Date.now()}`,
          capabilities: ['peer-capability'],
        });

        // Update trust
        const updatedTrust = await a2aClient.updatePeerTrust(
          peerAgent.id,
          0.75,
          1,
          true
        );

        expect(updatedTrust.trustScore).toBe(0.75);
        console.log('✅ Peer trust updated');
      }
    );
  });

  describe('Consent Management (GDPR/PSD2)', () => {
    let recipientAgentId: string;

    beforeEach(async () => {
      if (!backendAvailable || !credentialsAvailable) return;

      const recipientClient = createClient({
        baseUrl: AIM_URL,
        apiKey: process.env.AIM_API_KEY,
      });

      const recipient = await recipientClient.registerAgent({
        name: `${TEST_AGENT_NAME}-recipient-${Date.now()}`,
        capabilities: ['data-processing'],
      });

      recipientAgentId = recipient.id;
    });

    it.skipIf(!backendAvailable || !credentialsAvailable)(
      'should record consent',
      async () => {
        const consent = await a2aClient.recordConsent(
          'user-123',
          recipientAgentId,
          ['pii', 'usage_stats'],
          'data_processing',
          ['email', 'name']
        );

        expect(consent).toBeDefined();
        expect(consent.status).toBe('active');
        expect(consent.scope).toContain('pii');
        expect(consent.grantorAgentId).toBeDefined();
        expect(consent.recipientAgentId).toBe(recipientAgentId);

        console.log(`✅ Consent recorded: ${consent.id}`);
      }
    );

    it.skipIf(!backendAvailable || !credentialsAvailable)(
      'should check consent exists',
      async () => {
        // First record consent
        await a2aClient.recordConsent(
          'user-456',
          recipientAgentId,
          ['read:profile'],
          'profile_access',
          ['profile_data']
        );

        const result = await a2aClient.checkConsent('user-456', recipientAgentId, 'read:profile');

        expect(result.hasConsent).toBe(true);
        console.log('✅ Consent check passed');
      }
    );

    it.skipIf(!backendAvailable || !credentialsAvailable)(
      'should revoke consent',
      async () => {
        const consent = await a2aClient.recordConsent(
          'user-789',
          recipientAgentId,
          ['write:data'],
          'data_sync',
          ['user_data']
        );

        const revokedConsent = await a2aClient.revokeConsent(consent.id, 'User requested');

        expect(revokedConsent.status).toBe('revoked');
        expect(revokedConsent.revokedAt).toBeDefined();

        console.log('✅ Consent revoked');
      }
    );

    it.skipIf(!backendAvailable || !credentialsAvailable)(
      'should list user consents',
      async () => {
        // Record some consents
        await a2aClient.recordConsent(
          'user-list-test',
          recipientAgentId,
          ['scope1'],
          'purpose1',
          ['data1']
        );

        const consents = await a2aClient.listUserConsents('user-list-test');

        expect(Array.isArray(consents)).toBe(true);
        expect(consents.length).toBeGreaterThan(0);

        console.log(`✅ Found ${consents.length} user consents`);
      }
    );
  });

  describe('Security Policy Enforcement', () => {
    it.skipIf(!backendAvailable || !credentialsAvailable)(
      'should check security for A2A communication',
      async () => {
        const targetClient = createClient({
          baseUrl: AIM_URL,
          apiKey: process.env.AIM_API_KEY,
        });

        const target = await targetClient.registerAgent({
          name: `${TEST_AGENT_NAME}-target-${Date.now()}`,
          capabilities: ['secure-skill'],
        });

        const result = await a2aClient.checkSecurity(target.id, 'secure-skill');

        expect(result).toBeDefined();
        expect(typeof result.allowed).toBe('boolean');
        expect(result.enforcementMode).toMatch(/strict|monitor/);
        expect(Array.isArray(result.violations)).toBe(true);

        console.log(`✅ Security check: allowed=${result.allowed}`);
      }
    );

    it.skipIf(!backendAvailable || !credentialsAvailable)(
      'should get security settings',
      async () => {
        const settings = await a2aClient.getSecuritySettings();

        expect(settings).toBeDefined();
        expect(settings.enforcementMode).toMatch(/strict|monitor/);
        expect(typeof settings.minTrustScore).toBe('number');

        console.log(`✅ Security settings: mode=${settings.enforcementMode}`);
      }
    );

    it.skipIf(!backendAvailable || !credentialsAvailable)(
      'should update security settings',
      async () => {
        const updatedSettings = await a2aClient.updateSecuritySettings({
          minTrustScore: 0.6,
          requireVerifiedSkills: true,
        });

        expect(updatedSettings.minTrustScore).toBe(0.6);
        expect(updatedSettings.requireVerifiedSkills).toBe(true);

        console.log('✅ Security settings updated');
      }
    );
  });

  describe('Skill Attestation and Consensus', () => {
    let targetAgentId: string;

    beforeEach(async () => {
      if (!backendAvailable || !credentialsAvailable) return;

      const targetClient = createClient({
        baseUrl: AIM_URL,
        apiKey: process.env.AIM_API_KEY,
      });

      const target = await targetClient.registerAgent({
        name: `${TEST_AGENT_NAME}-attested-${Date.now()}`,
        capabilities: ['attestable-skill'],
      });

      targetAgentId = target.id;
    });

    it.skipIf(!backendAvailable || !credentialsAvailable)(
      'should create skill attestation',
      async () => {
        const attestation = await a2aClient.attestSkill(
          targetAgentId,
          'attestable-skill',
          'SKILL_VERIFICATION',
          0.9,
          { testsPassed: 10, testsTotal: 10 }
        );

        expect(attestation).toBeDefined();
        expect(attestation.attestedAgentId).toBe(targetAgentId);
        expect(attestation.skillId).toBe('attestable-skill');
        expect(attestation.confidence).toBe(0.9);
        expect(attestation.signature).toBeDefined();

        console.log(`✅ Skill attestation created: ${attestation.id}`);
      }
    );

    it.skipIf(!backendAvailable || !credentialsAvailable)(
      'should get skill attestations',
      async () => {
        // Create attestation first
        await a2aClient.attestSkill(
          targetAgentId,
          'attestable-skill',
          'SKILL_VERIFICATION',
          0.85
        );

        const attestations = await a2aClient.getSkillAttestations(
          targetAgentId,
          'attestable-skill'
        );

        expect(Array.isArray(attestations)).toBe(true);
        expect(attestations.length).toBeGreaterThan(0);

        console.log(`✅ Found ${attestations.length} attestations`);
      }
    );

    it.skipIf(!backendAvailable || !credentialsAvailable)(
      'should get consensus status',
      async () => {
        // Create multiple attestations for consensus
        await a2aClient.attestSkill(
          targetAgentId,
          'consensus-skill',
          'SKILL_VERIFICATION',
          0.9
        );

        const consensus = await a2aClient.getConsensusStatus(
          targetAgentId,
          'consensus-skill'
        );

        expect(consensus).toBeDefined();
        expect(typeof consensus.isVerified).toBe('boolean');
        expect(typeof consensus.attestationCount).toBe('number');
        expect(typeof consensus.confidenceScore).toBe('number');

        console.log(`✅ Consensus: verified=${consensus.isVerified}, count=${consensus.attestationCount}`);
      }
    );

    it.skipIf(!backendAvailable || !credentialsAvailable)(
      'should revoke attestation',
      async () => {
        const attestation = await a2aClient.attestSkill(
          targetAgentId,
          'revokable-skill',
          'SKILL_VERIFICATION',
          0.8
        );

        const revoked = await a2aClient.revokeAttestation(
          attestation.id,
          'No longer valid'
        );

        expect(revoked.isRevoked).toBe(true);
        console.log('✅ Attestation revoked');
      }
    );
  });

  describe('Error Handling', () => {
    it.skipIf(!backendAvailable)(
      'should handle non-existent agent card',
      async () => {
        await expect(
          a2aClient.getAgentCard('non-existent-agent-id')
        ).rejects.toThrow();
      }
    );

    it.skipIf(!backendAvailable)(
      'should handle invalid consent check',
      async () => {
        // Check consent for non-existent user should return no consent
        const result = await a2aClient.checkConsent(
          'non-existent-user',
          'non-existent-agent',
          'invalid-scope'
        );

        expect(result.hasConsent).toBe(false);
      }
    );
  });
});
