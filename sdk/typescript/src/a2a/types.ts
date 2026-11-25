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
 * An A2A card document, as `updateAgentCard` sends it in `cardData`. The server
 * reads the members below and stores the whole document, other A2A members
 * (`provider`, `capabilities`, ...) included.
 */
export interface A2ACardDocument {
  name: string;
  description?: string;
  url: string;
  version: string;
  skills?: A2ACardSkill[];
  securitySchemes?: Record<string, unknown>;
  defaultInputModes?: string[];
  defaultOutputModes?: string[];
  [member: string]: unknown;
}

/**
 * A skill as a card document declares it.
 */
export interface A2ACardSkill {
  id: string;
  name: string;
  description?: string;
  tags?: string[];
  inputModes?: string[];
  outputModes?: string[];
  examples?: string[];
}

/**
 * What `updateAgentCard` sends: the card document itself (`cardData`) or the
 * URL the server fetches it from (`cardUrl`). `agentId` names the agent whose
 * card is replaced, the current agent when left out.
 */
export interface A2AAgentCardUpdate {
  agentId?: string;
  cardUrl?: string;
  cardData?: A2ACardDocument;
}

/**
 * The server's answer to a card registration or replacement: the stored card,
 * the identifiers of the skills it declares, and the expiry of its new
 * attestation.
 */
export interface A2AAgentCardRegistration {
  cardId: string;
  agentId: string;
  cardUrl: string;
  attestationExpires: string;
  skills: string[];
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
 * What `registerSkill` sends to `POST /api/v1/a2a/skills`: the members the
 * route reads. `id` is the skill's identifier, `agentId` the agent it is
 * registered for (the authenticated agent when left out). Each example is a
 * string, as A2A card skills declare them.
 */
export interface A2ASkillRegistration {
  agentId?: string;
  id?: string;
  name: string;
  description?: string;
  tags?: string[];
  inputModes?: string[];
  outputModes?: string[];
  examples?: string[];
}

/**
 * A skill as the server stores and answers it: `id` is the stored row's
 * identifier, `skillId` the skill's own, and the examples are strings.
 */
export interface A2ARegisteredSkill {
  id: string;
  agentId: string;
  skillId: string;
  name: string;
  description?: string;
  tags?: string[];
  inputModes?: string[];
  outputModes?: string[];
  examples?: string[];
  invocationCount: number;
  successCount: number;
  failureCount: number;
  avgDurationMs?: number;
  isVerified: boolean;
  verifiedAt?: string;
  attestationCount: number;
  createdAt: string;
  updatedAt: string;
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
  grantorAgentId: string;
  recipientAgentId: string;
  scope: string[];
  purpose: string;
  dataTypes: string[];
  consentMethod: string;
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
 * A2A Security Settings, with the member names the server reads and answers with.
 *
 * The server replaces every setting with the ones an update carries, and stores a member the
 * update leaves out as false, 0, an empty string or no agent IDs. To change some settings, read
 * them with `getSecuritySettings()`, change them on that object, and pass it to
 * `updateSecuritySettings()`.
 *
 * `id`, `organizationId`, `createdBy`, `updatedBy`, `createdAt` and `updatedAt` describe the
 * stored record.
 */
export interface A2ASecuritySettings {
  id?: string;
  organizationId: string;
  enforcementMode: 'monitor' | 'strict';
  requireVerifiedSkills: boolean;
  minTrustScore: number;
  minAttestationCount: number;
  requireRequestSigning: boolean;
  requireNonceValidation: boolean;
  nonceValiditySeconds: number;
  allowedAgentIds?: string[];
  blockedAgentIds?: string[];
  maxRequestsPerMinute: number;
  maxRequestsPerHour: number;
  createdBy?: string;
  updatedBy?: string;
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

/**
 * A2A Policy Decision - the answer of POST /api/v1/a2a/policies/evaluate
 *
 * `decision` is ALLOW or DENY. `policiesEvaluated` names each active policy of
 * the caller's organization that was walked; `reason` names the policy that
 * denied, when one did.
 */
export interface A2APolicyDecision {
  decision: 'ALLOW' | 'DENY' | string;
  policiesEvaluated: string[];
  reason?: string;
  evaluatedAt: string;
}
