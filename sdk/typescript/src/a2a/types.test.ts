/**
 * Tests for A2A Type Definitions
 *
 * Tests cover:
 * - Interface structure validation
 * - Type contracts for A2A protocol
 */

import { describe, it, expect } from 'vitest';
import type {
  A2AAgentCard,
  A2ASkill,
  A2ASkillExample,
  A2AAttestation,
  A2ATrustScore,
  A2APeerTrust,
  A2AConsent,
  A2ARequestSignature,
  A2ASecurityCheckResult,
  A2ASecurityViolationInfo,
  A2ASecuritySettings,
  A2ASecurityViolation,
  CapableAgent,
  A2ASkillAttestation,
  A2AConsensusResult,
} from './types';

describe('A2AAgentCard interface', () => {
  it('should accept valid agent card', () => {
    const card: A2AAgentCard = {
      agentId: 'agent-123',
      name: 'test-agent',
      displayName: 'Test Agent',
      description: 'A test agent',
      url: 'https://agent.example.com',
      cardUrl: 'https://agent.example.com/.well-known/agent.json',
      version: '1.0.0',
      capabilities: ['db:read', 'api:call'],
      skills: [],
    };
    expect(card.agentId).toBe('agent-123');
    expect(card.url).toBe('https://agent.example.com');
  });

  it('should support skills array', () => {
    const skill: A2ASkill = {
      id: 'skill-1',
      name: 'data-analysis',
      description: 'Analyzes data',
    };
    const card: A2AAgentCard = {
      agentId: 'agent-123',
      name: 'analyst',
      url: 'https://example.com',
      cardUrl: 'https://example.com/.well-known/agent.json',
      version: '1.0.0',
      skills: [skill],
    };
    expect(card.skills.length).toBe(1);
    expect(card.skills[0].name).toBe('data-analysis');
  });

  it('should support AIM attestation', () => {
    const attestation: A2AAttestation = {
      agentId: 'agent-123',
      attestedAt: '2024-01-01T00:00:00Z',
      expiresAt: '2025-01-01T00:00:00Z',
      trustScore: 0.95,
      signature: 'base64-signature',
      attestationType: 'full',
    };
    const card: A2AAgentCard = {
      agentId: 'agent-123',
      name: 'attested-agent',
      url: 'https://example.com',
      cardUrl: 'https://example.com/.well-known/agent.json',
      version: '1.0.0',
      skills: [],
      aimAttestation: attestation,
    };
    expect(card.aimAttestation?.trustScore).toBe(0.95);
  });
});

describe('A2ASkill interface', () => {
  it('should have required fields', () => {
    const skill: A2ASkill = {
      id: 'skill-123',
      name: 'document-processing',
      description: 'Processes documents',
    };
    expect(skill.id).toBe('skill-123');
    expect(skill.name).toBe('document-processing');
  });

  it('should support input/output schemas', () => {
    const skill: A2ASkill = {
      id: 'skill-123',
      name: 'calculator',
      description: 'Performs calculations',
      inputSchema: {
        type: 'object',
        properties: {
          a: { type: 'number' },
          b: { type: 'number' },
        },
      },
      outputSchema: {
        type: 'object',
        properties: {
          result: { type: 'number' },
        },
      },
    };
    expect(skill.inputSchema).toBeDefined();
    expect(skill.outputSchema).toBeDefined();
  });

  it('should support examples', () => {
    const example: A2ASkillExample = {
      name: 'Addition',
      input: { a: 2, b: 3 },
      output: { result: 5 },
    };
    const skill: A2ASkill = {
      id: 'calc',
      name: 'calculator',
      description: 'Math operations',
      examples: [example],
    };
    expect(skill.examples?.length).toBe(1);
    expect(skill.examples?.[0].output).toEqual({ result: 5 });
  });

  it('should support verification status', () => {
    const skill: A2ASkill = {
      id: 'skill-123',
      name: 'verified-skill',
      description: 'A verified skill',
      isVerified: true,
      verifiedAt: '2024-01-01T00:00:00Z',
      attestationCount: 5,
    };
    expect(skill.isVerified).toBe(true);
    expect(skill.attestationCount).toBe(5);
  });
});

describe('A2ATrustScore interface', () => {
  it('should have required fields', () => {
    const trustScore: A2ATrustScore = {
      evaluatorAgentId: 'agent-1',
      subjectAgentId: 'agent-2',
      score: 0.85,
      confidence: 0.9,
      interactionCount: 100,
      successfulInteractions: 95,
      failedInteractions: 5,
    };
    expect(trustScore.score).toBe(0.85);
    expect(trustScore.confidence).toBe(0.9);
    expect(trustScore.interactionCount).toBe(100);
  });

  it('should support factors breakdown', () => {
    const trustScore: A2ATrustScore = {
      evaluatorAgentId: 'agent-1',
      subjectAgentId: 'agent-2',
      score: 0.8,
      confidence: 0.75,
      interactionCount: 50,
      successfulInteractions: 40,
      failedInteractions: 10,
      factors: {
        reliability: 0.9,
        accuracy: 0.8,
        responseTime: 0.7,
      },
    };
    expect(trustScore.factors?.reliability).toBe(0.9);
  });
});

describe('A2APeerTrust interface', () => {
  it('should have required fields', () => {
    const peerTrust: A2APeerTrust = {
      peerId: 'peer-123',
      peerName: 'Trusted Peer',
      trustScore: 0.92,
      interactionCount: 250,
    };
    expect(peerTrust.peerId).toBe('peer-123');
    expect(peerTrust.trustScore).toBe(0.92);
  });
});

describe('A2AConsent interface', () => {
  it('should have required fields for GDPR/PSD2 compliance', () => {
    const consent: A2AConsent = {
      id: 'consent-123',
      userId: 'user-456',
      grantorAgentId: 'agent-1',
      recipientAgentId: 'agent-2',
      scope: ['read:profile', 'write:transactions'],
      purpose: 'Financial data sharing',
      dataTypes: ['personal_info', 'transaction_history'],
      consentMethod: 'explicit',
      status: 'active',
      grantedAt: '2024-01-01T00:00:00Z',
    };
    expect(consent.status).toBe('active');
    expect(consent.scope).toContain('read:profile');
  });

  it('should support consent status values', () => {
    const activeConsent: A2AConsent = {
      id: 'c1', userId: 'u1', grantorAgentId: 'a1', recipientAgentId: 'a2',
      scope: [], purpose: 'test', dataTypes: [], consentMethod: 'explicit',
      status: 'active', grantedAt: '2024-01-01T00:00:00Z',
    };
    const revokedConsent: A2AConsent = {
      ...activeConsent,
      id: 'c2',
      status: 'revoked',
      revokedAt: '2024-06-01T00:00:00Z',
    };
    const expiredConsent: A2AConsent = {
      ...activeConsent,
      id: 'c3',
      status: 'expired',
      expiresAt: '2024-01-01T00:00:00Z',
    };

    expect(activeConsent.status).toBe('active');
    expect(revokedConsent.status).toBe('revoked');
    expect(expiredConsent.status).toBe('expired');
  });
});

describe('A2ARequestSignature interface', () => {
  it('should have required fields', () => {
    const signature: A2ARequestSignature = {
      signature: 'base64-signature-data',
      timestamp: Date.now(),
      nonce: 'unique-nonce-123',
      algorithm: 'Ed25519',
      agentId: 'agent-123',
      headers: {
        'X-Signature': 'sig-header',
        'X-Timestamp': '1704067200',
      },
    };
    expect(signature.algorithm).toBe('Ed25519');
    expect(signature.nonce).toBeDefined();
  });
});

describe('A2ASecurityCheckResult interface', () => {
  it('should indicate allowed request', () => {
    const result: A2ASecurityCheckResult = {
      allowed: true,
      enforcementMode: 'strict',
      violations: [],
      targetAgentId: 'agent-123',
    };
    expect(result.allowed).toBe(true);
    expect(result.violations.length).toBe(0);
  });

  it('should indicate denied request with violations', () => {
    const violation: A2ASecurityViolationInfo = {
      type: 'TRUST_SCORE_TOO_LOW',
      message: 'Trust score below minimum threshold',
      severity: 'high',
    };
    const result: A2ASecurityCheckResult = {
      allowed: false,
      enforcementMode: 'strict',
      violations: [violation],
      denialReason: 'Security policy violation',
      targetAgentId: 'agent-123',
      requesterTrustScore: 0.3,
    };
    expect(result.allowed).toBe(false);
    expect(result.violations.length).toBe(1);
    expect(result.denialReason).toBeDefined();
  });

  it('should support monitor mode', () => {
    const result: A2ASecurityCheckResult = {
      allowed: true,
      enforcementMode: 'monitor',
      violations: [{ type: 'LOW_TRUST', message: 'Trust below recommended', severity: 'low' }],
      targetAgentId: 'agent-123',
    };
    expect(result.enforcementMode).toBe('monitor');
    expect(result.allowed).toBe(true); // Allowed in monitor mode despite violations
  });
});

describe('A2ASecuritySettings interface', () => {
  it('should have required fields', () => {
    const settings: A2ASecuritySettings = {
      organizationId: 'org-123',
      enforcementMode: 'strict',
      requireVerifiedSkills: true,
      minTrustScore: 0.7,
      requireSignedRequests: true,
      requireNonceValidation: true,
      maxTimestampAge: 300000,
      blockedAgentIds: ['blocked-agent-1'],
      allowedAgentIds: ['allowed-agent-1', 'allowed-agent-2'],
    };
    expect(settings.minTrustScore).toBe(0.7);
    expect(settings.requireSignedRequests).toBe(true);
  });
});

describe('A2ASecurityViolation interface', () => {
  it('should have required fields', () => {
    const violation: A2ASecurityViolation = {
      id: 'violation-123',
      organizationId: 'org-456',
      requestingAgentId: 'agent-1',
      targetAgentId: 'agent-2',
      violationType: 'UNAUTHORIZED_SKILL_ACCESS',
      message: 'Agent attempted to access unauthorized skill',
      severity: 'high',
      enforcementMode: 'strict',
      blocked: true,
      createdAt: '2024-01-01T00:00:00Z',
    };
    expect(violation.blocked).toBe(true);
    expect(violation.severity).toBe('high');
  });

  it('should support severity levels', () => {
    const severities: Array<'low' | 'medium' | 'high' | 'critical'> = ['low', 'medium', 'high', 'critical'];
    for (const severity of severities) {
      const violation: A2ASecurityViolation = {
        id: 'v1', organizationId: 'o1', requestingAgentId: 'a1', targetAgentId: 'a2',
        violationType: 'TEST', message: 'Test', severity, enforcementMode: 'monitor',
        blocked: false, createdAt: '2024-01-01T00:00:00Z',
      };
      expect(violation.severity).toBe(severity);
    }
  });
});

describe('CapableAgent interface', () => {
  it('should have required fields', () => {
    const agent: CapableAgent = {
      agentId: 'agent-123',
      name: 'capable-agent',
      matchingSkills: [
        { id: 'skill-1', name: 'data-analysis', description: 'Analyzes data' },
      ],
    };
    expect(agent.matchingSkills.length).toBe(1);
  });

  it('should support relevance scoring', () => {
    const agent: CapableAgent = {
      agentId: 'agent-123',
      name: 'ranked-agent',
      displayName: 'Ranked Agent',
      trustScore: 0.9,
      matchingSkills: [],
      relevanceScore: 0.95,
    };
    expect(agent.relevanceScore).toBe(0.95);
    expect(agent.trustScore).toBe(0.9);
  });
});

describe('A2ASkillAttestation interface', () => {
  it('should have required fields for multi-agent consensus', () => {
    const attestation: A2ASkillAttestation = {
      id: 'attestation-123',
      attestingAgentId: 'attester-agent',
      attestedAgentId: 'subject-agent',
      skillId: 'skill-456',
      attestationType: 'SKILL_VERIFICATION',
      confidence: 0.85,
      signature: 'base64-signature',
      isRevoked: false,
      createdAt: '2024-01-01T00:00:00Z',
    };
    expect(attestation.attestationType).toBe('SKILL_VERIFICATION');
    expect(attestation.isRevoked).toBe(false);
  });

  it('should support attestation types', () => {
    const types: Array<'SKILL_VERIFICATION' | 'QUALITY' | 'SECURITY'> = ['SKILL_VERIFICATION', 'QUALITY', 'SECURITY'];
    for (const type of types) {
      const attestation: A2ASkillAttestation = {
        id: 'a1', attestingAgentId: 'a1', attestedAgentId: 'a2', skillId: 's1',
        attestationType: type, confidence: 0.9, signature: 'sig', isRevoked: false,
        createdAt: '2024-01-01T00:00:00Z',
      };
      expect(attestation.attestationType).toBe(type);
    }
  });

  it('should support revocation', () => {
    const attestation: A2ASkillAttestation = {
      id: 'attestation-123',
      attestingAgentId: 'attester',
      attestedAgentId: 'subject',
      skillId: 'skill-1',
      attestationType: 'QUALITY',
      confidence: 0.8,
      signature: 'sig',
      isRevoked: true,
      revokedAt: '2024-06-01T00:00:00Z',
      revokedReason: 'Skill quality degraded',
      createdAt: '2024-01-01T00:00:00Z',
    };
    expect(attestation.isRevoked).toBe(true);
    expect(attestation.revokedReason).toBeDefined();
  });
});

describe('A2AConsensusResult interface', () => {
  it('should have required fields', () => {
    const result: A2AConsensusResult = {
      skillId: 'skill-123',
      agentId: 'agent-456',
      attestationCount: 5,
      uniqueAttesters: 5,
      uniqueOwners: 3,
      confidenceScore: 75.5,
      isVerified: true,
    };
    expect(result.isVerified).toBe(true);
    expect(result.uniqueOwners).toBe(3);
  });

  it('should indicate unverified when below thresholds', () => {
    const result: A2AConsensusResult = {
      skillId: 'skill-123',
      agentId: 'agent-456',
      attestationCount: 2,
      uniqueAttesters: 2,
      uniqueOwners: 1,
      confidenceScore: 45.0,
      isVerified: false,
    };
    expect(result.isVerified).toBe(false);
    // Below thresholds: 3+ attesters, 2+ owners, 60+ confidence
  });
});
