/**
 * A2A Client Tests
 *
 * Tests for the Agent-to-Agent protocol client including:
 * - Agent card operations
 * - Intent-based discovery
 * - Trust score management
 * - Consent management (GDPR/PSD2)
 * - Security policy enforcement
 * - Skill attestation and consensus
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { A2AClient } from './A2AClient';
import type { AIMClient } from '../client/AIMClient';
import type {
  A2AAgentCard,
  A2ATrustScore,
  A2APeerTrust,
  A2AConsent,
  A2ASecurityCheckResult,
  A2ASkillAttestation,
  A2AConsensusResult,
  CapableAgent,
} from './types';

// Store original fetch
const originalFetch = global.fetch;

// Mock fetch responses
let mockFetchResponse: unknown = {};

// Mock AIMClient with proper structure
const createMockAIMClient = () => {
  return {
    config: { baseUrl: 'http://test.example.com' },
    getCredentials: vi.fn().mockReturnValue({ agentId: 'test-agent-id' }),
    credentials: { agentId: 'test-agent-id' },
  } as unknown as AIMClient;
};

// Helper to set mock response
const setMockResponse = (response: unknown) => {
  mockFetchResponse = response;
};

describe('A2AClient', () => {
  let client: A2AClient;
  let mockAIMClient: ReturnType<typeof createMockAIMClient>;

  beforeEach(() => {
    mockAIMClient = createMockAIMClient();
    client = new A2AClient(mockAIMClient);

    // Mock global fetch
    global.fetch = vi.fn().mockImplementation(() =>
      Promise.resolve({
        ok: true,
        json: () => Promise.resolve(mockFetchResponse),
      } as Response)
    );
  });

  afterEach(() => {
    global.fetch = originalFetch;
    vi.resetAllMocks();
  });

  // ==================== Agent Card Tests ====================

  describe('Agent Card Operations', () => {
    it('should register an agent card', async () => {
      const mockCard: A2AAgentCard = {
        id: 'card-123',
        agentId: 'test-agent',
        name: 'Test Agent',
        displayName: 'Test Display Name',
        url: 'https://agent.example.com',
        cardUrl: 'https://agent.example.com/.well-known/agent.json',
        version: '1.0.0',
        skills: [],
      };

      setMockResponse(mockCard);

      const result = await client.registerAgentCard('https://agent.example.com/.well-known/agent.json');

      expect(result).toEqual(mockCard);
      expect(global.fetch).toHaveBeenCalledWith(
        'http://test.example.com/api/v1/a2a/cards',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({ cardUrl: 'https://agent.example.com/.well-known/agent.json' }),
        })
      );
    });

    it('should get an agent card', async () => {
      const mockCard: A2AAgentCard = {
        id: 'card-123',
        agentId: 'test-agent',
        name: 'Test Agent',
        url: 'https://agent.example.com',
        cardUrl: 'https://agent.example.com/.well-known/agent.json',
        version: '1.0.0',
        skills: [],
      };

      setMockResponse(mockCard);

      const result = await client.getAgentCard('agent-123');

      expect(result).toEqual(mockCard);
      expect(global.fetch).toHaveBeenCalledWith(
        'http://test.example.com/api/v1/a2a/cards/agent-123',
        expect.objectContaining({ method: 'GET' })
      );
    });
  });

  // ==================== Intent-Based Discovery Tests ====================

  describe('Intent-Based Discovery', () => {
    it('should route by intent', async () => {
      const mockAgents: CapableAgent[] = [
        {
          agentId: 'capable-agent',
          name: 'Capable Agent',
          matchingSkills: [{ id: 'skill-1', name: 'Code Analysis', description: 'Analyzes code' }],
        },
      ];

      setMockResponse({ agents: mockAgents });

      const result = await client.routeByIntent('analyze code quality');

      expect(result).toEqual(mockAgents);
      expect(global.fetch).toHaveBeenCalledWith(
        'http://test.example.com/api/v1/a2a/discovery/route',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({ intent: 'analyze code quality', minTrustScore: 0 }),
        })
      );
    });

    it('should get capable agents', async () => {
      const mockAgents: CapableAgent[] = [
        {
          agentId: 'agent-1',
          name: 'Agent 1',
          matchingSkills: [{ id: 'skill-1', name: 'Skill 1', description: 'Test skill' }],
        },
        {
          agentId: 'agent-2',
          name: 'Agent 2',
          matchingSkills: [{ id: 'skill-2', name: 'Skill 2', description: 'Another skill' }],
        },
      ];

      setMockResponse({ agents: mockAgents });

      const result = await client.getCapableAgents(['data-processing']);

      expect(result).toEqual(mockAgents);
      expect(global.fetch).toHaveBeenCalledWith(
        'http://test.example.com/api/v1/a2a/discovery/capable',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({ skillIds: ['data-processing'], minTrustScore: 0 }),
        })
      );
    });
  });

  // ==================== Trust Score Tests ====================

  describe('Trust Score Management', () => {
    it('should get A2A trust score', async () => {
      const mockScore: A2ATrustScore = {
        id: 'score-123',
        evaluatorAgentId: 'evaluator',
        subjectAgentId: 'subject',
        score: 0.85,
        confidence: 0.9,
        interactionCount: 100,
        successfulInteractions: 90,
        failedInteractions: 10,
      };

      setMockResponse(mockScore);

      const result = await client.getTrustScore('agent-123');

      expect(result).toEqual(mockScore);
      expect(global.fetch).toHaveBeenCalledWith(
        'http://test.example.com/api/v1/a2a/trust/agent-123',
        expect.objectContaining({ method: 'GET' })
      );
    });

    it('should list peer trusts', async () => {
      const mockPeers: A2APeerTrust[] = [
        {
          peerId: 'peer-agent',
          peerName: 'Peer Agent',
          trustScore: 0.9,
          interactionCount: 50,
        },
      ];

      setMockResponse({ peers: mockPeers });

      const result = await client.listPeerTrusts();

      expect(result).toEqual(mockPeers);
      expect(global.fetch).toHaveBeenCalledWith(
        'http://test.example.com/api/v1/a2a/peers',
        expect.objectContaining({ method: 'GET' })
      );
    });
  });

  // ==================== Consent Management Tests ====================

  describe('Consent Management (GDPR/PSD2)', () => {
    it('should record consent with correct field names', async () => {
      const mockConsent: A2AConsent = {
        id: 'consent-123',
        userId: 'user-123',
        grantorAgentId: 'test-agent-id',
        recipientAgentId: 'recipient-agent',
        scope: ['pii', 'usage_stats'],
        purpose: 'data_processing',
        dataTypes: ['email', 'name'],
        consentMethod: 'explicit_consent',
        status: 'active',
        grantedAt: new Date().toISOString(),
      };

      setMockResponse(mockConsent);

      const result = await client.recordConsent(
        'user-123',
        'recipient-agent',
        ['pii', 'usage_stats'],
        'data_processing',
        ['email', 'name']
      );

      expect(result).toEqual(mockConsent);
      expect(global.fetch).toHaveBeenCalledWith(
        'http://test.example.com/api/v1/a2a/consent',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({
            userId: 'user-123',
            grantorAgentId: 'test-agent-id',
            recipientAgentId: 'recipient-agent',
            scope: ['pii', 'usage_stats'],
            purpose: 'data_processing',
            dataTypes: ['email', 'name'],
            consentMethod: 'explicit_consent',
            expiresInHours: 24,
          }),
        })
      );
    });

    it('should check consent with correct query parameters', async () => {
      const mockResponse = { hasConsent: true };

      setMockResponse(mockResponse);

      const result = await client.checkConsent('user-123', 'recipient-agent', 'pii');

      expect(result.hasConsent).toBe(true);

      // Verify the URL contains the correct query parameters
      const fetchCall = (global.fetch as ReturnType<typeof vi.fn>).mock.calls[0];
      const url = fetchCall[0] as string;
      expect(url).toContain('grantorAgentId=test-agent-id');
      expect(url).toContain('recipientAgentId=recipient-agent');
      expect(url).toContain('scope=pii');
    });

    it('should revoke consent', async () => {
      const mockRevokedConsent: A2AConsent = {
        id: 'consent-123',
        userId: 'user-123',
        grantorAgentId: 'grantor',
        recipientAgentId: 'recipient',
        scope: ['pii'],
        purpose: 'data_processing',
        dataTypes: ['email'],
        consentMethod: 'explicit_consent',
        status: 'revoked',
        grantedAt: new Date().toISOString(),
        revokedAt: new Date().toISOString(),
      };

      setMockResponse(mockRevokedConsent);

      const result = await client.revokeConsent('consent-123', 'User requested revocation');

      expect(result.status).toBe('revoked');
      expect(global.fetch).toHaveBeenCalledWith(
        'http://test.example.com/api/v1/a2a/consent/consent-123/revoke',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({ reason: 'User requested revocation' }),
        })
      );
    });

    it('should list user consents', async () => {
      const mockConsents: A2AConsent[] = [
        {
          id: 'consent-1',
          userId: 'user-123',
          grantorAgentId: 'grantor',
          recipientAgentId: 'recipient-1',
          scope: ['pii'],
          purpose: 'processing',
          dataTypes: ['email'],
          consentMethod: 'explicit_consent',
          status: 'active',
          grantedAt: new Date().toISOString(),
        },
      ];

      setMockResponse({ consents: mockConsents });

      const result = await client.listUserConsents('user-123');

      expect(result).toHaveLength(1);
      expect(result[0].id).toBe('consent-1');
      expect(global.fetch).toHaveBeenCalledWith(
        'http://test.example.com/api/v1/a2a/consent/user/user-123?includeRevoked=false',
        expect.objectContaining({ method: 'GET' })
      );
    });
  });

  // ==================== Security Check Tests ====================

  describe('Security Policy Enforcement', () => {
    it('should check security for A2A communication', async () => {
      const mockResult: A2ASecurityCheckResult = {
        allowed: true,
        enforcementMode: 'strict',
        violations: [],
        targetAgentId: 'target-agent',
      };

      setMockResponse(mockResult);

      const result = await client.checkSecurity('target-agent', 'skill-1');

      expect(result.allowed).toBe(true);
      expect(result.enforcementMode).toBe('strict');
      expect(global.fetch).toHaveBeenCalledWith(
        'http://test.example.com/api/v1/a2a/security/check',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({ targetAgentId: 'target-agent', skillId: 'skill-1' }),
        })
      );
    });

    it('should handle security violations', async () => {
      const mockResult: A2ASecurityCheckResult = {
        allowed: false,
        enforcementMode: 'strict',
        violations: [
          {
            type: 'LOW_TRUST_SCORE',
            message: 'Agent trust score below threshold',
            severity: 'high',
          },
        ],
        denialReason: 'Trust score too low',
        targetAgentId: 'target-agent',
      };

      setMockResponse(mockResult);

      const result = await client.checkSecurity('target-agent');

      expect(result.allowed).toBe(false);
      expect(result.violations).toHaveLength(1);
      expect(result.violations[0].type).toBe('LOW_TRUST_SCORE');
    });
  });

  // ==================== Skill Attestation Tests ====================

  describe('Skill Attestation and Consensus', () => {
    it('should create skill attestation', async () => {
      const mockAttestation: A2ASkillAttestation = {
        id: 'attest-123',
        attestingAgentId: 'test-agent-id',
        attestedAgentId: 'target-agent',
        skillId: 'data-transform',
        attestationType: 'SKILL_VERIFICATION',
        confidence: 0.95,
        signature: 'sig-123',
        isRevoked: false,
        createdAt: new Date().toISOString(),
      };

      setMockResponse(mockAttestation);

      const result = await client.attestSkill(
        'target-agent',
        'data-transform',
        'SKILL_VERIFICATION',
        0.95,
        { testsPassed: true }
      );

      expect(result.id).toBe('attest-123');
      expect(result.confidence).toBe(0.95);
      expect(global.fetch).toHaveBeenCalledWith(
        'http://test.example.com/api/v1/a2a/attestations',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({
            attestedAgentId: 'target-agent',
            skillId: 'data-transform',
            attestationType: 'SKILL_VERIFICATION',
            confidence: 0.95,
            evidence: { testsPassed: true },
          }),
        })
      );
    });

    it('should get consensus status', async () => {
      const mockConsensus: A2AConsensusResult = {
        skillId: 'data-transform',
        agentId: 'target-agent',
        attestationCount: 5,
        uniqueAttesters: 4,
        uniqueOwners: 3,
        confidenceScore: 85.5,
        isVerified: true,
      };

      setMockResponse(mockConsensus);

      const result = await client.getConsensusStatus('target-agent', 'data-transform');

      expect(result.isVerified).toBe(true);
      expect(result.uniqueAttesters).toBe(4);
      expect(result.confidenceScore).toBe(85.5);
      expect(global.fetch).toHaveBeenCalledWith(
        'http://test.example.com/api/v1/a2a/agents/target-agent/skills/data-transform/consensus',
        expect.objectContaining({ method: 'GET' })
      );
    });

    it('should indicate unverified skill below threshold', async () => {
      const mockConsensus: A2AConsensusResult = {
        skillId: 'new-skill',
        agentId: 'target-agent',
        attestationCount: 1,
        uniqueAttesters: 1,
        uniqueOwners: 1,
        confidenceScore: 30.0,
        isVerified: false,
      };

      setMockResponse(mockConsensus);

      const result = await client.getConsensusStatus('target-agent', 'new-skill');

      // Skill not verified: needs 3+ attesters, 2+ owners, 60+ confidence
      expect(result.isVerified).toBe(false);
    });
  });
});

// ==================== Type Tests ====================

describe('A2A Types', () => {
  describe('A2AConsent', () => {
    it('should have correct field names matching API', () => {
      const consent: A2AConsent = {
        id: 'consent-123',
        userId: 'user-123',
        grantorAgentId: 'grantor-agent',
        recipientAgentId: 'recipient-agent',
        scope: ['pii'],
        purpose: 'data_processing',
        dataTypes: ['email'],
        consentMethod: 'explicit_consent',
        status: 'active',
        grantedAt: new Date().toISOString(),
      };

      expect(consent.grantorAgentId).toBe('grantor-agent');
      expect(consent.recipientAgentId).toBe('recipient-agent');
      expect(consent.scope).toContain('pii');
      expect(consent.consentMethod).toBe('explicit_consent');
    });
  });

  describe('A2AConsensusResult', () => {
    it('should represent consensus thresholds', () => {
      const consensus: A2AConsensusResult = {
        skillId: 'test-skill',
        agentId: 'test-agent',
        attestationCount: 5,
        uniqueAttesters: 3,
        uniqueOwners: 2,
        confidenceScore: 75.0,
        isVerified: true,
      };

      // Verification thresholds: 3+ attesters, 2+ owners, 60+ confidence
      expect(consensus.uniqueAttesters).toBeGreaterThanOrEqual(3);
      expect(consensus.uniqueOwners).toBeGreaterThanOrEqual(2);
      expect(consensus.confidenceScore).toBeGreaterThanOrEqual(60);
      expect(consensus.isVerified).toBe(true);
    });
  });
});
