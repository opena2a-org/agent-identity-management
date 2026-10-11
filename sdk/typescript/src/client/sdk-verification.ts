/**
 * The request `AIMClient.verifyAction` sends to AIM: POST
 * /api/v1/sdk-api/verifications.
 *
 * The server binds the body into its `VerificationRequest` (agentId,
 * capability, resource, context, timestamp, riskLevel, signature, publicKey)
 * and authenticates it from the body alone: it rebuilds the signed message
 * from agentId, capability, resource, context and timestamp, requires
 * publicKey to be the agent's registered key, and verifies the Ed25519
 * signature over that message. The route is mounted outside the SDK-API
 * group's authentication, so the request carries no Authorization header and
 * needs no token.
 *
 * The answer is `201` for an approved, auto-approved (monitoring mode) or
 * pending (just-in-time request awaiting an administrator) action, and `403`
 * with `status: "denied"` for a denied one. A `403` without a status is a
 * refusal before any decision, for example an agent whose status does not
 * allow it to act.
 */

import * as ed from '@noble/ed25519';
import { decodeAgentPrivateKey } from '../crypto/agent-request';
import { toBase64 } from '../crypto/ed25519';
import type { AgentCredentials } from '../types';

/** The route AIM serves action verification on. */
export const SDK_VERIFICATIONS_PATH = '/api/v1/sdk-api/verifications';

/** The body the route binds, member for member. */
export interface SDKVerificationRequest {
  agentId: string;
  capability: string;
  resource?: string;
  context: Record<string, unknown>;
  timestamp: string;
  riskLevel?: string;
  signature: string;
  publicKey: string;
}

/** The body the route answers with, approved, pending or denied. */
export interface SDKVerificationResponse {
  id?: string;
  status?: string;
  approvedBy?: string;
  expiresAt?: string;
  denialReason?: string;
  trustScore?: number;
  enforcementMode?: string;
  riskLevel?: string;
  riskAutoDetected?: boolean;
}

/** What a verification asks about, before it is signed. */
export interface VerificationFields {
  capability: string;
  resource?: string;
  context?: Record<string, unknown>;
  riskLevel?: string;
}

/** A lone UTF-16 surrogate, which the server reads as U+FFFD. */
const LONE_SURROGATE = /[\uD800-\uDBFF](?![\uDC00-\uDFFF])|(?<![\uD800-\uDBFF])[\uDC00-\uDFFF]/g;

const REPLACEMENT_CHARACTER = String.fromCharCode(0xfffd);
const LINE_SEPARATOR = String.fromCharCode(0x2028);
const PARAGRAPH_SEPARATOR = String.fromCharCode(0x2029);

function wellFormed(value: string): string {
  return value.replace(LONE_SURROGATE, REPLACEMENT_CHARACTER);
}

/** A JSON string as the server's encoder writes it: U+2028 and U+2029 escaped, HTML characters not. */
function jsonString(value: string): string {
  return JSON.stringify(wellFormed(value))
    .split(LINE_SEPARATOR)
    .join('\\u2028')
    .split(PARAGRAPH_SEPARATOR)
    .join('\\u2029');
}

/** Object members in byte order of their UTF-8 names, as the server sorts map keys. */
function byUTF8(a: string, b: string): number {
  return Buffer.compare(Buffer.from(a, 'utf8'), Buffer.from(b, 'utf8'));
}

/**
 * A JSON value written as the server writes the signed message: compact, with
 * one space after each comma and colon outside a string. `value` has already
 * been through JSON, so it holds only objects, arrays, strings, finite
 * numbers, booleans and null.
 */
function canonicalJSON(value: unknown): string {
  if (value === null) return 'null';
  if (typeof value === 'string') return jsonString(value);
  if (typeof value === 'number' || typeof value === 'boolean') return JSON.stringify(value);
  if (Array.isArray(value)) return `[${value.map(canonicalJSON).join(', ')}]`;
  const members = Object.entries(value as Record<string, unknown>)
    .map(([name, member]) => [wellFormed(name), member] as const)
    .sort(([a], [b]) => byUTF8(a, b))
    .map(([name, member]) => `${jsonString(name)}: ${canonicalJSON(member)}`);
  return `{${members.join(', ')}}`;
}

/**
 * The message the server verifies the signature over: an object with the
 * members action_type, agent_id, context, resource and timestamp, written by
 * {@link canonicalJSON}. An absent or empty resource is null.
 */
export function verificationSigningMessage(fields: {
  agentId: string;
  capability: string;
  resource?: string;
  context: Record<string, unknown>;
  timestamp: string;
}): string {
  return canonicalJSON({
    action_type: fields.capability,
    agent_id: fields.agentId,
    context: fields.context,
    resource: fields.resource ? fields.resource : null,
    timestamp: fields.timestamp,
  });
}

/**
 * Build and sign the body for one verification. The context is put through
 * JSON first, so the message is signed over the same values the server reads
 * from the body. The public key sent is the one derived from the private key,
 * which is the key the signature verifies under; the server then requires it
 * to be the agent's registered key. A private key that is not 32 or 64 bytes
 * of standard base64 throws `ConfigurationError`, and nothing is sent.
 */
export async function signVerificationRequest(
  credentials: Pick<AgentCredentials, 'agentId' | 'privateKey'>,
  fields: VerificationFields,
  timestamp: string = new Date().toISOString()
): Promise<SDKVerificationRequest> {
  const { seed, publicKey } = await decodeAgentPrivateKey(credentials.privateKey, 'no request was sent');
  const context = JSON.parse(JSON.stringify(fields.context ?? {})) as Record<string, unknown>;
  const message = verificationSigningMessage({
    agentId: credentials.agentId,
    capability: fields.capability,
    resource: fields.resource,
    context,
    timestamp,
  });
  const signature = await ed.signAsync(new TextEncoder().encode(message), seed);

  return {
    agentId: credentials.agentId,
    capability: fields.capability,
    ...(fields.resource !== undefined ? { resource: fields.resource } : {}),
    context,
    timestamp,
    ...(fields.riskLevel !== undefined ? { riskLevel: fields.riskLevel } : {}),
    signature: toBase64(signature),
    publicKey: toBase64(publicKey),
  };
}
