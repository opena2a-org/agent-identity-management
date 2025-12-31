/**
 * A2A Type Definitions
 */

/**
 * A2A Agent Card - Describes an agent's capabilities and how to communicate with it
 */
export interface A2AAgentCard {
  id?: string;
  agentId: string;
  name: string;
  displayName?: string;
  description?: string;
  url: string;
  cardUrl: string;
  version: string;
  capabilities?: string[];
  skills: A2ASkill[];
  publicKey?: string;
  aimAttestation?: A2AAttestation;
  createdAt?: string;
  updatedAt?: string;
}

/**
 * A2A Skill - A capability an agent can perform
 */
export interface A2ASkill {
  id: string;
  name: string;
  description: string;
  inputSchema?: Record<string, unknown>;
  outputSchema?: Record<string, unknown>;
  examples?: A2ASkillExample[];
  tags?: string[];
  isVerified?: boolean;
  verifiedAt?: string;
  attestationCount?: number;
}

/**
 * A2A Skill Example
 */
export interface A2ASkillExample {
  name: string;
  input: Record<string, unknown>;
  output?: Record<string, unknown>;
}

/**
 * A2A Attestation - AIM verification of an agent
 */
export interface A2AAttestation {
  agentId: string;
  attestedAt: string;
  expiresAt: string;
  trustScore: number;
  signature: string;
  attestationType: string;
}

/**
 * A2A Trust Score
 */
export interface A2ATrustScore {
  id?: string;
  evaluatorAgentId: string;
  subjectAgentId: string;
  score: number;
  confidence: number;
  interactionCount: number;
  successfulInteractions: number;
  failedInteractions: number;
  lastInteractionAt?: string;
  factors?: Record<string, number>;
  createdAt?: string;
  updatedAt?: string;
}

/**
 * A2A Peer Trust
 */
export interface A2APeerTrust {
  peerId: string;
  peerName: string;
  trustScore: number;
  interactionCount: number;
  lastInteraction?: string;
}

/**
 * A2A Consent - GDPR/PSD2 compliant consent record
 */
export interface A2AConsent {
  id: string;
  userId: string;
  sourceAgentId: string;
  targetAgentId: string;
  purpose: string;
  dataTypes: string[];
  legalBasis: string;
  status: 'active' | 'revoked' | 'expired';
  grantedAt: string;
  expiresAt?: string;
  revokedAt?: string;
}

/**
 * A2A Request Signature
 */
export interface A2ARequestSignature {
  signature: string;
  timestamp: number;
  nonce: string;
  algorithm: string;
  agentId: string;
  publicKey?: string;
  headers: Record<string, string>;
}

/**
 * A2A Security Check Result
 */
export interface A2ASecurityCheckResult {
  allowed: boolean;
  enforcementMode: 'monitor' | 'strict';
  violations: A2ASecurityViolationInfo[];
  denialReason?: string;
  targetAgentId: string;
  skillId?: string;
  requesterTrustScore?: number;
}

/**
 * A2A Security Violation Info (inline)
 */
export interface A2ASecurityViolationInfo {
  type: string;
  message: string;
  severity: 'low' | 'medium' | 'high' | 'critical';
}

/**
 * A2A Security Settings
 */
export interface A2ASecuritySettings {
  id?: string;
  organizationId: string;
  enforcementMode: 'monitor' | 'strict';
  requireVerifiedSkills: boolean;
  minTrustScore: number;
  requireSignedRequests: boolean;
  requireNonceValidation: boolean;
  maxTimestampAge: number;
  blockedAgentIds: string[];
  allowedAgentIds: string[];
  createdAt?: string;
  updatedAt?: string;
}

/**
 * A2A Security Violation Record
 */
export interface A2ASecurityViolation {
  id: string;
  organizationId: string;
  requestingAgentId: string;
  targetAgentId: string;
  skillId?: string;
  violationType: string;
  message: string;
  severity: 'low' | 'medium' | 'high' | 'critical';
  enforcementMode: 'monitor' | 'strict';
  blocked: boolean;
  context?: string;
  createdAt: string;
}

/**
 * Capable Agent Result
 */
export interface CapableAgent {
  agentId: string;
  name: string;
  displayName?: string;
  trustScore?: number;
  matchingSkills: A2ASkill[];
  relevanceScore?: number;
}

/**
 * A2A Skill Attestation - Multi-agent consensus verification
 */
export interface A2ASkillAttestation {
  id: string;
  attestingAgentId: string;
  attestingAgentName?: string;
  attestedAgentId: string;
  skillId: string;
  attestationType: 'SKILL_VERIFICATION' | 'QUALITY' | 'SECURITY';
  confidence: number;
  evidence?: Record<string, unknown>;
  signature: string;
  isRevoked: boolean;
  revokedAt?: string;
  revokedReason?: string;
  createdAt: string;
  expiresAt?: string;
}

/**
 * A2A Consensus Result - Multi-agent verification status
 *
 * Skills become "verified" when they meet thresholds:
 * - 3+ unique attesting agents
 * - 2+ unique organization owners
 * - 60.0+ confidence score
 */
export interface A2AConsensusResult {
  skillId: string;
  agentId: string;
  attestationCount: number;
  uniqueAttesters: number;
  uniqueOwners: number;
  confidenceScore: number;
  isVerified: boolean;
}
