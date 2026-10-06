/**
 * agent-request-v1: the Ed25519 request signature AIM verifies for requests
 * made as an agent (PQCAgentMiddleware, with `X-Algorithm` absent).
 *
 * The server rebuilds the signed bytes as UPPER(method), the raw request
 * target, and `X-Timestamp` as sent, joined by `\n`, then `\n` and the raw body
 * when the body is not empty.
 */

import * as ed from '@noble/ed25519';
import { ConfigurationError } from '../exceptions';
import type { AgentCredentials } from '../types';
import { fromBase64, toBase64 } from './ed25519';

/** The methods `createAgentRequestHeaders` signs, in the exact case sent. */
export type AgentRequestMethod = 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE';

/** One request to sign: the same method, url and body are passed to fetch. */
export interface AgentRequest {
  method: AgentRequestMethod;
  /** The absolute http or https URL the request is sent to. */
  url: string;
  /** The body exactly as sent, as a string (sent as UTF-8) or as bytes. */
  body?: string | Uint8Array;
}

/** The four headers that carry an `agent-request-v1` signature. */
export interface AgentRequestHeaders {
  'X-Agent-ID': string;
  'X-Timestamp': string;
  'X-Signature': string;
  'X-Public-Key': string;
}

const AGENT_REQUEST_METHODS: ReadonlySet<string> = new Set([
  'GET',
  'POST',
  'PUT',
  'PATCH',
  'DELETE',
]);

/** Standard base64 with correct padding; canonical form is checked separately. */
const STANDARD_BASE64 = /^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$/;

const NOTHING_SIGNED = 'nothing was signed';

/** The Fix for a key that does not belong to one registered agent. */
const CREDENTIALS_FIX =
  'Load credentials from one `registerAgent` result for the AIM server at `baseUrl`: ' +
  '`AIM_AGENT_ID`, `AIM_PUBLIC_KEY` and `AIM_PRIVATE_KEY` must belong to the same registered agent.';

const METHOD_FIX = 'Pass the method in upper case as one of GET, POST, PUT, PATCH or DELETE.';

const URL_FIX =
  'Pass the absolute http or https URL the request is sent to, with no username or password in it.';

const BODY_FIX = 'Serialize the body once and pass that string or those bytes, exactly as they are sent.';

function refusal(line: string, stem: string, fix: string): ConfigurationError {
  return new ConfigurationError(`${line}; ${stem}\nFix: ${fix}`);
}

function decodeStandardBase64(encoded: string): Uint8Array | null {
  if (!STANDARD_BASE64.test(encoded)) return null;
  const bytes = fromBase64(encoded);
  // Rejects non-zero padding bits, which decode to the same bytes as another
  // string: only the one canonical encoding of the key is accepted.
  return toBase64(bytes) === encoded ? bytes : null;
}

function bytesEqual(a: Uint8Array, b: Uint8Array): boolean {
  if (a.length !== b.length) return false;
  let diff = 0;
  for (let i = 0; i < a.length; i++) diff |= a[i] ^ b[i];
  return diff === 0;
}

/**
 * The one decoder from a configured agent private key to signing bytes.
 *
 * Accepts standard padded base64 of exactly 32 bytes (a seed) or 64 bytes
 * (AIM's issued form: the seed, then its public key). For 64 bytes, the public
 * key derived from the seed must equal the last 32 bytes. Returns the 32-byte
 * seed and the public key derived from it. Any other input throws
 * `ConfigurationError` naming the case; no error carries any part of the key.
 *
 * @internal
 */
export async function decodeAgentPrivateKey(
  encoded: unknown,
  stem: string = NOTHING_SIGNED
): Promise<{ seed: Uint8Array; publicKey: Uint8Array }> {
  if (typeof encoded !== 'string' || encoded.length === 0) {
    throw refusal('no agent private key is configured', stem, CREDENTIALS_FIX);
  }
  const bytes = decodeStandardBase64(encoded);
  if (bytes === null) {
    throw refusal('the agent private key is not standard base64', stem, CREDENTIALS_FIX);
  }
  if (bytes.length !== 32 && bytes.length !== 64) {
    throw refusal(
      `the agent private key is ${bytes.length} bytes long, not 32 or 64`,
      stem,
      CREDENTIALS_FIX
    );
  }
  const seed = bytes.slice(0, 32);
  const publicKey = await ed.getPublicKeyAsync(seed);
  if (bytes.length === 64 && !bytesEqual(publicKey, bytes.slice(32))) {
    throw refusal(
      'the agent private key is 64 bytes long, and its last 32 bytes are not its public key',
      stem,
      CREDENTIALS_FIX
    );
  }
  return { seed, publicKey };
}

/**
 * The request target fetch sends for `url`: the serialized URL from the end of
 * its origin up to, not including, the first `#`. An empty query keeps its `?`,
 * which `pathname + search` would drop.
 */
function requestTarget(url: unknown, stem: string): string {
  if (typeof url !== 'string') {
    throw refusal('the url is not an absolute http or https URL', stem, URL_FIX);
  }
  let parsed: URL;
  try {
    parsed = new URL(url);
  } catch {
    throw refusal('the url is not an absolute http or https URL', stem, URL_FIX);
  }
  if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') {
    throw refusal('the url is not an absolute http or https URL', stem, URL_FIX);
  }
  if (parsed.username !== '' || parsed.password !== '') {
    throw refusal('the url carries a username or a password', stem, URL_FIX);
  }
  const href = parsed.href;
  if (!href.startsWith(parsed.origin)) {
    throw refusal('the url is not an absolute http or https URL', stem, URL_FIX);
  }
  const rest = href.slice(parsed.origin.length);
  const fragment = rest.indexOf('#');
  return fragment === -1 ? rest : rest.slice(0, fragment);
}

function bodyBytes(body: unknown, stem: string): Uint8Array {
  if (body === undefined) return new Uint8Array(0);
  if (typeof body === 'string') return new TextEncoder().encode(body);
  if (body instanceof Uint8Array) return body;
  throw refusal('the body is not a string or bytes', stem, BODY_FIX);
}

/**
 * Signs a request as this agent with `agent-request-v1`, the Ed25519 request
 * signature AIM verifies, and returns the four headers to send with it.
 * Signed bytes: the method, `\n`, the path and query string of `url`, `\n`,
 * `X-Timestamp`, and, when `body` is not empty, `\n` and `body` exactly as
 * sent. The URL's host is not signed.
 * `X-Timestamp` is this host's clock in Unix seconds; AIM accepts the request
 * only within 30 seconds of its own clock, earlier or later.
 * There is no nonce: anyone who obtains the request, including any server it
 * is sent to, can send it to AIM again until AIM's clock is more than 30
 * seconds past `X-Timestamp`. Send these headers only to AIM; do not log them.
 * Ed25519 only. Pass the same method, url and body to fetch, and add no
 * Authorization header (AIM then skips this signature).
 */
export async function createAgentRequestHeaders(
  request: AgentRequest,
  credentials: Pick<AgentCredentials, 'agentId' | 'privateKey' | 'publicKey'>
): Promise<AgentRequestHeaders> {
  const stem = NOTHING_SIGNED;
  const method: unknown = request?.method;
  if (typeof method !== 'string' || !AGENT_REQUEST_METHODS.has(method)) {
    throw refusal(
      'the method is not one of GET, POST, PUT, PATCH or DELETE in upper case',
      stem,
      METHOD_FIX
    );
  }
  const target = requestTarget(request.url, stem);
  const body = bodyBytes(request.body, stem);

  const agentId: unknown = credentials?.agentId;
  if (typeof agentId !== 'string' || agentId.length === 0) {
    throw refusal('no agent ID is configured', stem, CREDENTIALS_FIX);
  }
  const { seed, publicKey } = await decodeAgentPrivateKey(credentials.privateKey, stem);
  const publicKeyB64 = toBase64(publicKey);
  if (publicKeyB64 !== credentials.publicKey) {
    throw refusal(
      'the configured agent public key is not the public key of the agent private key',
      stem,
      CREDENTIALS_FIX
    );
  }

  // The host clock only; never a time taken from a response or the network.
  const timestamp = String(Math.floor(Date.now() / 1000));
  const head = new TextEncoder().encode(`${method}\n${target}\n${timestamp}`);
  let message = head;
  if (body.length > 0) {
    message = new Uint8Array(head.length + 1 + body.length);
    message.set(head, 0);
    message[head.length] = 0x0a;
    message.set(body, head.length + 1);
  }
  const signature = await ed.signAsync(message, seed);

  return {
    'X-Agent-ID': agentId,
    'X-Timestamp': timestamp,
    'X-Signature': toBase64(signature),
    'X-Public-Key': publicKeyB64,
  };
}
