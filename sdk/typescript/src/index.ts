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

// Version
export const VERSION = '1.0.0';
