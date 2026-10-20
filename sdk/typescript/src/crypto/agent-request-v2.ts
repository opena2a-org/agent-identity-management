/**
 * agent-request-v2: a request signed as a statement about itself, the form
 * AIM's AgentRequestV2Middleware verifies.
 *
 * The statement is a JSON object whose members name the request: `method`,
 * `target` (the path and query string fetch sends), `audience` (the URL's
 * origin), `body_sha256` (the unpadded base64url SHA-256 of the body, present
 * exactly when the body is not empty), `agent_id`, `timestamp` (RFC 3339 in
 * UTC) and `nonce` (16 bytes, unpadded base64url). It is sent as unpadded
 * base64url in `X-Agent-Request`. Each signature covers the DSSE v1
 * pre-authentication encoding of the payload type and the statement, and is
 * sent in `X-Agent-Request-Signature` as its algorithm, a colon and the
 * unpadded base64url signature. docs/specs/agent-request-v2-vectors.json in
 * the AIM repository holds requests signed this way and the answer AIM's
 * verifier gives each.
 *
 * The package entry point does not export this module, and no SDK code calls
 * it.
 */

import * as ed from '@noble/ed25519';
import { createHash, randomBytes } from 'crypto';
import type { AgentCredentials } from '../types';
import { checkAgentRequest, NOTHING_SIGNED, refusal, type AgentRequest } from './agent-request';
import { KEY_SIZES, signMLDSA } from './pqc';

/** The DSSE payload type of a statement. */
const PAYLOAD_TYPE = 'application/vnd.opena2a.agent-request.v2+json';

/** The length of a statement's nonce, in bytes. */
const NONCE_BYTES = 16;

/** The ML-DSA parameter sets a signature may name. */
export type AgentRequestMLDSAAlgorithm = 'ML-DSA-44' | 'ML-DSA-65' | 'ML-DSA-87';

/** An ML-DSA private key of the agent and its parameter set. */
export interface AgentRequestMLDSAKey {
  algorithm: AgentRequestMLDSAAlgorithm;
  /** The private key, as `generateMLDSAKeyPair` returns it. */
  privateKey: Uint8Array;
}

export interface AgentRequestV2Options {
  /**
   * The agent's ML-DSA key. Given, it signs the statement as well as the
   * Ed25519 key of the credentials: AIM requires both signatures of an agent
   * in hybrid mode, and refuses two signatures from any other agent.
   */
  mlDsa?: AgentRequestMLDSAKey;
}

/** The two headers that carry a statement and its signatures. */
export interface AgentRequestV2Headers {
  'X-Agent-Request': string;
  'X-Agent-Request-Signature': string;
}

const ML_DSA_ALGORITHMS: ReadonlySet<string> = new Set(['ML-DSA-44', 'ML-DSA-65', 'ML-DSA-87']);

const ML_DSA_FIX =
  "Pass the private key of the agent's registered ML-DSA key, as `generateMLDSAKeyPair` returns it, " +
  'and its algorithm: ML-DSA-44, ML-DSA-65 or ML-DSA-87.';

function checkMLDSAKey(key: unknown): AgentRequestMLDSAKey | undefined {
  if (key === undefined) return undefined;
  const stem = NOTHING_SIGNED;
  const algorithm: unknown = (key as { algorithm?: unknown } | null)?.algorithm;
  if (typeof algorithm !== 'string' || !ML_DSA_ALGORITHMS.has(algorithm)) {
    throw refusal('the ML-DSA algorithm is not ML-DSA-44, ML-DSA-65 or ML-DSA-87', stem, ML_DSA_FIX);
  }
  const privateKey: unknown = (key as { privateKey?: unknown }).privateKey;
  if (!(privateKey instanceof Uint8Array)) {
    throw refusal('the ML-DSA private key is not bytes', stem, ML_DSA_FIX);
  }
  const expected = KEY_SIZES[algorithm as AgentRequestMLDSAAlgorithm].privateKey;
  if (privateKey.length !== expected) {
    throw refusal(
      `the ML-DSA private key is ${privateKey.length} bytes long, not ${expected} as ${algorithm} requires`,
      stem,
      ML_DSA_FIX
    );
  }
  return { algorithm: algorithm as AgentRequestMLDSAAlgorithm, privateKey };
}

function base64url(bytes: Uint8Array): string {
  return Buffer.from(bytes).toString('base64url');
}

/**
 * The DSSE v1 pre-authentication encoding of the payload type and `payload`:
 * "DSSEv1", the byte length of the type, the type, the byte length of the
 * payload and the payload, separated by single spaces.
 */
function preAuthenticationEncoding(payload: Uint8Array): Uint8Array {
  const head = new TextEncoder().encode(
    `DSSEv1 ${PAYLOAD_TYPE.length} ${PAYLOAD_TYPE} ${payload.length} `
  );
  const message = new Uint8Array(head.length + payload.length);
  message.set(head, 0);
  message.set(payload, head.length);
  return message;
}

/**
 * Signs a request as this agent with `agent-request-v2` and returns the two
 * headers to send with it. The statement names the method, the path and query
 * string of `url`, the origin of `url` as the audience, the SHA-256 of `body`
 * when it is not empty, the agent ID, this host's clock in whole seconds and a
 * random 16-byte nonce drawn for this call.
 * AIM's verifier refuses a statement whose nonce it has already admitted for
 * the agent, and one whose time is more than 30 seconds from its database
 * clock, earlier or later. It compares the audience byte for byte with the
 * origin it is configured with.
 * The Ed25519 key of `credentials` signs every statement. Given
 * `options.mlDsa`, for an agent in hybrid mode, the ML-DSA key signs it too,
 * and `X-Agent-Request-Signature` carries the two signatures separated by a
 * comma.
 * The request and the credentials are checked as `createAgentRequestHeaders`
 * checks them, and a refusal throws `ConfigurationError` with nothing signed.
 * Pass the same method, url and body to fetch. Send no Authorization,
 * X-API-Key or agent-request-v1 header with these headers: AIM refuses a
 * statement sent beside another credential. Do not log them.
 */
export async function createAgentRequestV2Headers(
  request: AgentRequest,
  credentials: Pick<AgentCredentials, 'agentId' | 'privateKey' | 'publicKey'>,
  options: AgentRequestV2Options = {}
): Promise<AgentRequestV2Headers> {
  // The host clock only; never a time taken from a response or the network.
  const signedAt = Math.floor(Date.now() / 1000);
  return signAgentRequestV2(request, credentials, options, signedAt, randomBytes(NONCE_BYTES));
}

/**
 * `createAgentRequestV2Headers` with the signing time, in Unix seconds, and
 * the nonce given, which the conformance vectors fix. A signer takes them from
 * the host clock and a random source, as `createAgentRequestV2Headers` does.
 *
 * @internal
 */
export async function signAgentRequestV2(
  request: AgentRequest,
  credentials: Pick<AgentCredentials, 'agentId' | 'privateKey' | 'publicKey'>,
  options: AgentRequestV2Options,
  signedAt: number,
  nonce: Uint8Array
): Promise<AgentRequestV2Headers> {
  const checked = await checkAgentRequest(request, credentials);
  const mlDsa = checkMLDSAKey(options?.mlDsa);

  // The members in the order the format lists them.
  const statement: Record<string, string> = {
    method: checked.method,
    target: checked.target,
    audience: checked.origin,
  };
  if (checked.body.length > 0) {
    statement.body_sha256 = base64url(createHash('sha256').update(checked.body).digest());
  }
  statement.agent_id = checked.agentId;
  statement.timestamp = new Date(signedAt * 1000).toISOString().slice(0, 19) + 'Z';
  statement.nonce = base64url(nonce);

  const payload = new TextEncoder().encode(JSON.stringify(statement));
  const message = preAuthenticationEncoding(payload);
  const signatures = [`Ed25519:${base64url(await ed.signAsync(message, checked.seed))}`];
  if (mlDsa) {
    const signature = await signMLDSA(mlDsa.algorithm, mlDsa.privateKey, message);
    signatures.push(`${mlDsa.algorithm}:${base64url(signature)}`);
  }
  return {
    'X-Agent-Request': base64url(payload),
    'X-Agent-Request-Signature': signatures.join(', '),
  };
}
