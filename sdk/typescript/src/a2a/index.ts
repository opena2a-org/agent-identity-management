/**
 * A2A Module - Agent-to-Agent Protocol
 *
 * Provides secure agent-to-agent communication with:
 * - Agent card registration and discovery
 * - Intent-based agent routing
 * - Ed25519 request signing and verification
 * - Trust score management
 * - GDPR/PSD2 compliant consent management
 * - Security policy enforcement
 */

export { A2AClient, createA2AClient, findAgentForIntent } from './A2AClient';
export type {
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
} from './types';
