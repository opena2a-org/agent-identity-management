/**
 * AIM SDK for TypeScript/Node.js
 *
 * Agent Identity Management SDK for secure AI agent verification.
 *
 * @packageDocumentation
 *
 * @example Basic usage
 * ```typescript
 * import { AIMClient, AgentType } from '@opena2a/aim-sdk';
 *
 * const client = new AIMClient({
 *   baseUrl: 'https://aim.example.com',
 *   apiKey: 'your-api-key',
 * });
 *
 * // Register an agent
 * const agent = await client.registerAgent({
 *   name: 'my-agent',
 *   displayName: 'My AI Agent',
 *   agentType: AgentType.LANGCHAIN,
 * });
 *
 * // Verify an action
 * const result = await client.verifyAction({
 *   action: 'file:read',
 *   resource: '/data/config.json',
 * });
 * ```
 *
 * @example With Express middleware
 * ```typescript
 * import express from 'express';
 * import { createAIMMiddleware, verifyAction } from '@opena2a/aim-sdk/express';
 *
 * const app = express();
 *
 * // Add AIM middleware
 * app.use(createAIMMiddleware({
 *   baseUrl: 'https://aim.example.com',
 *   apiKey: 'your-api-key',
 * }));
 *
 * // Verify specific actions
 * app.post('/api/data',
 *   verifyAction('data:write'),
 *   (req, res) => res.json({ success: true })
 * );
 * ```
 *
 * @example With Fastify plugin
 * ```typescript
 * import Fastify from 'fastify';
 * import { aimPlugin, verifyAction } from '@opena2a/aim-sdk/fastify';
 *
 * const fastify = Fastify();
 *
 * await fastify.register(aimPlugin, {
 *   baseUrl: 'https://aim.example.com',
 *   apiKey: 'your-api-key',
 * });
 *
 * fastify.post('/api/data', {
 *   preHandler: verifyAction('data:write'),
 * }, async () => ({ success: true }));
 * ```
 */

// Core client
export { AIMClient, createClient, registerAgent } from './client/AIMClient';

// Types
export {
  AgentType,
  RiskLevel,
  AgentStatus,
  WebhookEvent,
  type AIMClientConfig,
  type Agent,
  type AgentCredentials,
  type RegisterAgentOptions,
  type VerificationResult,
  type VerifyActionOptions,
  type TokenResponse,
  type APIError,
  type WebhookPayload,
  type MCPServer,
} from './types';

// Exceptions
export {
  AIMError,
  AuthenticationError,
  AuthorizationError,
  ActionDeniedError,
  VerificationError,
  ConfigurationError,
  NetworkError,
  NotFoundError,
  RateLimitError,
  SecretsError,
} from './exceptions';

// Auth utilities
export {
  OAuthTokenManager,
  loadCredentialsFromEnv,
  loadCredentialsFromFile,
  saveCredentialsToFile,
} from './auth/oauth';

// Crypto utilities (Ed25519)
export {
  generateKeyPair,
  sign,
  verify,
  toBase64,
  fromBase64,
  toHex,
  fromHex,
  createRequestSignature,
  type KeyPair,
} from './crypto/ed25519';

// Delegation chain (cross-engine interop)
export {
  publicKeyToDidKey,
  didKeyToPublicKey,
  canonicalJSONDeep,
  createDelegation,
  verifyDelegation,
  verifyDelegationSignature,
  verifyDelegatorIdentity,
  verifyScopeNarrowing,
  verifyDelegationChain,
  checkDelegationTemporalValidity,
  exportDelegationChain,
  delegationSignablePayload,
  type Delegation,
  type DelegationChainEntry,
  type DelegationChainExport,
  type DelegationTemporalOptions,
  type DelegationTemporalResult,
  type DelegationChainResult,
  type DelegationChainVerificationOptions,
} from './crypto/delegation';

// A2A Protocol
export {
  A2AClient,
  createA2AClient,
  findAgentForIntent,
  type A2AAgentCard,
  type A2ASkill,
  type A2ASkillExample,
  type A2AAttestation,
  type A2ATrustScore,
  type A2APeerTrust,
  type A2AConsent,
  type A2ARequestSignature,
  type A2ASecurityCheckResult,
  type A2ASecurityViolationInfo,
  type A2ASecuritySettings,
  type A2ASecurityViolation,
  type CapableAgent,
  type A2ASkillAttestation,
  type A2AConsensusResult,
} from './a2a';

// Post-Quantum Crypto utilities (ML-DSA)
// Requires optional dependency: @noble/post-quantum
export {
  // PQC availability check
  isPQCAvailable,
  // ML-DSA operations
  generateMLDSAKeyPair,
  signMLDSA,
  verifyMLDSA,
  // Hybrid Ed25519+ML-DSA operations
  generateHybridKeyPair,
  hybridSign,
  hybridVerify,
  getHybridPublicKey,
  // Encoding/decoding
  encodeHybridSignature,
  decodeHybridSignature,
  encodeHybridPublicKey,
  decodeHybridPublicKey,
  // Algorithm helpers
  getMLDSAVariant,
  isHybridAlgorithm,
  isPQCAlgorithm,
  validateKeySize,
  // Hybrid request headers
  createHybridRequestHeaders,
  // Constants
  KEY_SIZES,
  // Types
  type Algorithm,
  type MLDSAKeyPair,
  type HybridKeyPair,
  type HybridSignature,
  type HybridPublicKey,
  type EncodedHybridSignature,
  type EncodedHybridPublicKey,
} from './crypto/pqc';

// Isolation attestation
export {
  SandboxType,
  NetworkIsolation,
  FilesystemIsolation,
  ProcessIsolation,
  type IsolationPosture,
  type IsolationAttestationResult,
  scoreIsolation,
  autoDetectIsolation,
} from './isolation/index.js';

// Secrets (identity-native credential management)
export {
  SecretsClient,
  edwardsToMontgomeryPrivate,
  type SecretsClientHost,
  type ResolvedCredential,
  type CreateNamespaceOptions,
  type StoreCredentialOptions,
  type AuditLogOptions,
} from './secrets';

// Local, offline ATX credential verification (spec-compliant local-verify path,
// powered by the shared conformance-locked @opena2a/atx-verify verifier).
export {
  LocalVerifier,
  CrlCache,
  type LocalVerificationConfig,
  type LocalAuthorizationOptions,
  type LocalAuthorizationResult,
  type CrlData,
  type CrlStalePolicy,
  type CrlCacheConfig,
  type CrlCacheStatus,
  type Atx,
  type AtxPublicKey,
  type AtxTrustAnchors,
  type AtxVerificationResult,
  type RejectCategory,
  type ResolutionContext,
} from './local';

// Causal-denial telemetry (correlate why a blocked action happened; off the
// enforcement critical path, best-effort)
export {
  CORRELATION_HEADER,
  CORRELATION_BAGGAGE_KEY,
  mintCorrelationId,
  isCorrelationId,
  correlationHeaders,
  extractCorrelationId,
  TELEMETRY_SCHEMA_VERSION,
  buildCorrelatedRecord,
  assertObservedInvariant,
  toSharedIndicator,
  defaultDataDir,
  writeCorrelatedRecord,
  readCorrelatedRecords,
  CorrelationJoiner,
  isHighConfidence,
  CorrelatedRelay,
  DEFAULT_REGISTRY_URL,
  RUNTIME_TELEMETRY_PATH,
  RELAY_EVENT_TYPE,
  SENTINEL_PACKAGE_NAME,
  openA2AHome,
  deriveSensorToken,
  detectRuntimeEnv,
  daysSinceInstall,
  isValidTechniqueId,
  isTechniqueIdFormat,
  mapAttackClass,
  interimTechniqueFields,
  MATRIX_SNAPSHOT_VERSION,
  type CorrelatedRecord,
  type SharedIndicator,
  type EnforcementFact,
  type IntentInference,
  type DetectionInference,
  type BuildCorrelatedRecordInput,
  type SharedIndicatorContext,
  type TechniqueSource,
  type Completeness,
  type JoinerOptions,
  type EnforcementEvent,
  type IntentEvent,
  type DetectionEvent,
  type RelayConfig,
  type FlushResult,
} from './telemetry';

// Version (single source of truth: src/version.ts)
export { SDK_VERSION as VERSION } from './version';
