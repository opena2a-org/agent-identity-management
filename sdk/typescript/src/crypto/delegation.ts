/**
 * Ed25519 delegation chain creation, signing, and cross-engine verification.
 *
 * Implements the interop surface documented in:
 * https://github.com/kanoniv/agent-auth/blob/main/spec/CROSS-ENGINE-VERIFICATION.md
 *
 * Canonical form: JSON.stringify with sorted keys, compact separators (no spaces).
 * DID method: did:key with Ed25519 multicodec prefix 0xed01.
 * Signature/key encoding: base64url.
 */

import * as ed from '@noble/ed25519';
import { type KeyPair } from './ed25519';

// --- Base58btc (used for did:key encoding) ---

const BASE58_ALPHABET = '123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz';
const BASE58N = BigInt(58);
const BIG_ZERO = BigInt(0);
const BIG_FF = BigInt(0xff);
const BIG_8 = BigInt(8);

function base58btcEncode(bytes: Uint8Array): string {
  let leadingZeros = 0;
  for (const b of bytes) {
    if (b === 0) leadingZeros++;
    else break;
  }
  let num = BIG_ZERO;
  for (const b of bytes) {
    num = (num << BIG_8) | BigInt(b);
  }
  let result = '';
  while (num > BIG_ZERO) {
    const remainder = Number(num % BASE58N);
    num = num / BASE58N;
    result = BASE58_ALPHABET[remainder] + result;
  }
  return BASE58_ALPHABET[0].repeat(leadingZeros) + result;
}

function base58btcDecode(str: string): Uint8Array {
  let leadingZeros = 0;
  for (const char of str) {
    if (char === BASE58_ALPHABET[0]) leadingZeros++;
    else break;
  }
  let num = BIG_ZERO;
  for (const char of str) {
    const index = BASE58_ALPHABET.indexOf(char);
    if (index === -1) throw new Error(`Invalid base58 character: ${char}`);
    num = num * BASE58N + BigInt(index);
  }
  const byteArray: number[] = [];
  while (num > BIG_ZERO) {
    byteArray.unshift(Number(num & BIG_FF));
    num = num >> BIG_8;
  }
  const result = new Uint8Array(leadingZeros + byteArray.length);
  for (let i = 0; i < byteArray.length; i++) {
    result[leadingZeros + i] = byteArray[i];
  }
  return result;
}

// --- Base64url encoding ---

function toBase64url(data: Uint8Array): string {
  const b64 = Buffer.from(data).toString('base64');
  return b64.replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}

function fromBase64url(str: string): Uint8Array {
  // Restore standard base64
  let b64 = str.replace(/-/g, '+').replace(/_/g, '/');
  while (b64.length % 4 !== 0) b64 += '=';
  return new Uint8Array(Buffer.from(b64, 'base64'));
}

// --- did:key ---

const ED25519_MULTICODEC_PREFIX = new Uint8Array([0xed, 0x01]);

export function publicKeyToDidKey(publicKey: Uint8Array): string {
  if (publicKey.length !== 32) {
    throw new Error(`Ed25519 public key must be 32 bytes, got ${publicKey.length}`);
  }
  const prefixed = new Uint8Array(2 + 32);
  prefixed.set(ED25519_MULTICODEC_PREFIX, 0);
  prefixed.set(publicKey, 2);
  return `did:key:z${base58btcEncode(prefixed)}`;
}

export function didKeyToPublicKey(did: string): Uint8Array {
  if (!did.startsWith('did:key:z')) {
    throw new Error(`Invalid did:key format: ${did}`);
  }
  const decoded = base58btcDecode(did.slice(9)); // skip "did:key:z" (9 chars)
  if (decoded[0] !== 0xed || decoded[1] !== 0x01) {
    throw new Error('did:key does not use Ed25519 multicodec prefix (0xed01)');
  }
  return decoded.slice(2);
}

// --- Canonical JSON ---

/**
 * Produce canonical JSON: sorted keys, compact separators, no spaces.
 * Matches Python's json.dumps(obj, sort_keys=True, separators=(',', ':'))
 */
export function canonicalJSON(obj: Record<string, unknown>): string {
  return JSON.stringify(obj, Object.keys(obj).sort());
}

/**
 * Deep sort: recursively sorts all object keys for canonical serialization.
 * Arrays are preserved in order (values are not sorted).
 */
function deepSortKeys(value: unknown): unknown {
  if (Array.isArray(value)) {
    return value.map(deepSortKeys);
  }
  if (value !== null && typeof value === 'object') {
    const sorted: Record<string, unknown> = {};
    for (const key of Object.keys(value as Record<string, unknown>).sort()) {
      sorted[key] = deepSortKeys((value as Record<string, unknown>)[key]);
    }
    return sorted;
  }
  return value;
}

/**
 * Produce canonical JSON with deep key sorting.
 * This handles nested objects correctly.
 */
export function canonicalJSONDeep(obj: unknown): string {
  return JSON.stringify(deepSortKeys(obj));
}

// --- Delegation types ---

export interface Delegation {
  delegator: string;
  delegate: string;
  scopes: string[];
  createdAt: string;
  expiresAt: string;
  signature: string;
  publicKey: string;
  parentDelegation?: string;
  trustAttenuation?: number;  // Multiplier per hop (0.0-1.0, default 0.8)
  minDelegatedTrust?: number; // Floor below which chain is invalid (0.0-1.0, default 0.3)
}

/**
 * Fields excluded from the signed payload (per interop spec section 4.3).
 * Trust attenuation fields are metadata about the trust relationship,
 * not part of the signed delegation contract.
 */
const EXCLUDED_FROM_SIGNING = new Set(['signature', 'publicKey', 'trustAttenuation', 'minDelegatedTrust']);

/**
 * Extract the signable payload from a delegation object.
 * Returns the canonical JSON bytes to sign/verify.
 */
export function delegationSignablePayload(delegation: Omit<Delegation, 'signature' | 'publicKey'> & Partial<Pick<Delegation, 'signature' | 'publicKey'>>): Uint8Array {
  const payload: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(delegation)) {
    if (!EXCLUDED_FROM_SIGNING.has(key) && value !== undefined) {
      // Convert to snake_case for interop (the spec uses snake_case)
      const snakeKey = key.replace(/[A-Z]/g, (m) => `_${m.toLowerCase()}`);
      payload[snakeKey] = value;
    }
  }
  const canonical = canonicalJSONDeep(payload);
  return new TextEncoder().encode(canonical);
}

export interface DelegationChainEntry {
  delegation: Delegation;
  label: string;
  effectiveTrust?: number;
}

export interface DelegationChainExport {
  engine: string;
  sdk: string;
  didMethod: string;
  chain: DelegationChainEntry[];
  verification: {
    algorithm: string;
    encoding: string;
    signedPayload: string;
    didResolution: string;
    canonicalForm: string;
  };
}

// --- Signing and verification ---

/**
 * Create a signed delegation.
 */
export async function createDelegation(params: {
  delegatorKeyPair: KeyPair;
  delegatePublicKey: Uint8Array;
  scopes: string[];
  createdAt?: string;
  expiresAt?: string;
  parentDelegation?: string;
  /**
   * The parent delegation's `expiresAt`, supplied when creating a
   * sub-delegation. A child must not outlive its delegator: when this is set,
   * the default child expiry is capped at the parent's, and an explicit
   * `expiresAt` beyond the parent's is rejected. Enforced symmetrically by
   * {@link verifyDelegationChain}.
   */
  parentExpiresAt?: string;
}): Promise<Delegation> {
  const now = new Date();
  const oneWeek = new Date(now.getTime() + 7 * 24 * 60 * 60 * 1000);

  // A child must not outlive its parent. When the parent's expiry is known,
  // cap the default and reject an explicit expiry that exceeds it.
  let expiresAt = params.expiresAt ?? oneWeek.toISOString();
  if (params.parentExpiresAt !== undefined) {
    const parentMs = Date.parse(params.parentExpiresAt);
    if (Number.isNaN(parentMs)) {
      throw new Error('createDelegation: parentExpiresAt is not a valid timestamp');
    }
    const childMs = Date.parse(expiresAt);
    if (Number.isNaN(childMs)) {
      throw new Error('createDelegation: expiresAt is not a valid timestamp');
    }
    if (params.expiresAt === undefined) {
      // Default path: cap the child at the parent's expiry.
      if (parentMs < childMs) expiresAt = params.parentExpiresAt;
    } else if (childMs > parentMs) {
      // Explicit path: a child cannot be asked to outlive its parent.
      throw new Error(
        'createDelegation: expiresAt exceeds parentExpiresAt (a delegation cannot outlive its delegator)',
      );
    }
  }

  const delegation: Omit<Delegation, 'signature' | 'publicKey'> & { publicKey?: string; signature?: string } = {
    delegator: publicKeyToDidKey(params.delegatorKeyPair.publicKey),
    delegate: publicKeyToDidKey(params.delegatePublicKey),
    scopes: params.scopes,
    createdAt: params.createdAt ?? now.toISOString(),
    expiresAt,
  };

  if (params.parentDelegation) {
    delegation.parentDelegation = params.parentDelegation;
  }

  const payload = delegationSignablePayload(delegation);
  const signatureBytes = await ed.signAsync(payload, params.delegatorKeyPair.privateKey);

  return {
    ...delegation,
    signature: toBase64url(signatureBytes),
    publicKey: toBase64url(params.delegatorKeyPair.publicKey),
  } as Delegation;
}

/**
 * Verify a single delegation's Ed25519 signature only.
 *
 * This is the raw cryptographic check: it proves the delegation was signed by
 * the holder of `publicKey` and has not been tampered with. It deliberately
 * ignores temporal validity — an authentic but expired delegation returns true.
 * Prefer {@link verifyDelegation}, which also enforces the signed `expiresAt`
 * window. Use this only when you need signature authenticity independent of
 * time (e.g. archival/audit inspection).
 */
export async function verifyDelegationSignature(delegation: Delegation): Promise<boolean> {
  try {
    const publicKeyBytes = fromBase64url(delegation.publicKey);
    const signatureBytes = fromBase64url(delegation.signature);
    const payload = delegationSignablePayload(delegation);
    return await ed.verifyAsync(signatureBytes, payload, publicKeyBytes);
  } catch {
    return false;
  }
}

/**
 * Options controlling temporal (expiry) evaluation.
 */
export interface DelegationTemporalOptions {
  /**
   * The instant to evaluate temporal validity against. Accepts a Date or an
   * ISO-8601 string; defaults to the current time. Supply it to keep tests
   * deterministic and to support offline / as-of verification. If provided but
   * unparseable, verification fails closed.
   */
  verifyAt?: Date | string;
}

export interface DelegationTemporalResult {
  valid: boolean;
  error?: string;
}

/**
 * Resolve a verifyAt option to epoch milliseconds. Returns NaN when the caller
 * supplied a value that cannot be parsed, so callers can fail closed.
 */
function resolveVerifyAtMs(verifyAt?: Date | string): number {
  if (verifyAt === undefined) return Date.now();
  if (verifyAt instanceof Date) return verifyAt.getTime();
  return Date.parse(verifyAt);
}

/**
 * Evaluate a delegation's signed temporal window (`createdAt`/`expiresAt`)
 * against an evaluation time. Fails closed: a missing or unparseable timestamp,
 * an inverted window (createdAt after expiresAt), or an unparseable evaluation
 * time all return `{ valid: false }`. The expiry bound is exclusive — a
 * delegation is valid strictly before `expiresAt`.
 */
export function checkDelegationTemporalValidity(
  delegation: Pick<Delegation, 'createdAt' | 'expiresAt'>,
  verifyAt?: Date | string,
): DelegationTemporalResult {
  const atMs = resolveVerifyAtMs(verifyAt);
  if (Number.isNaN(atMs)) return { valid: false, error: 'Unparseable verifyAt evaluation time' };
  return temporalAtMs(delegation, atMs);
}

/**
 * Core temporal check against a resolved epoch-ms evaluation time. Kept private
 * so a chain can evaluate every hop against one shared instant.
 */
function temporalAtMs(
  delegation: Pick<Delegation, 'createdAt' | 'expiresAt'>,
  atMs: number,
): DelegationTemporalResult {
  const createdMs = delegation.createdAt === undefined ? NaN : Date.parse(delegation.createdAt);
  const expiresMs = delegation.expiresAt === undefined ? NaN : Date.parse(delegation.expiresAt);
  if (Number.isNaN(createdMs)) return { valid: false, error: 'Missing or unparseable createdAt timestamp' };
  if (Number.isNaN(expiresMs)) return { valid: false, error: 'Missing or unparseable expiresAt timestamp' };
  if (createdMs > expiresMs) return { valid: false, error: 'Invalid window: createdAt is after expiresAt' };
  if (atMs >= expiresMs) return { valid: false, error: 'Delegation has expired' };
  return { valid: true };
}

/**
 * Verify a single delegation: Ed25519 signature AND temporal validity.
 *
 * Returns true only when the signature is authentic and the delegation is
 * within its signed `createdAt`/`expiresAt` window at the evaluation time
 * (default: now). Expired, not-yet-parseable, or malformed-window delegations
 * return false. Pass `options.verifyAt` for deterministic or offline checks.
 */
export async function verifyDelegation(
  delegation: Delegation,
  options?: DelegationTemporalOptions,
): Promise<boolean> {
  if (!(await verifyDelegationSignature(delegation))) return false;
  return checkDelegationTemporalValidity(delegation, options?.verifyAt).valid;
}

/**
 * Verify that the delegation's publicKey matches the delegator DID.
 */
export function verifyDelegatorIdentity(delegation: Delegation): boolean {
  try {
    const publicKeyBytes = fromBase64url(delegation.publicKey);
    const expectedDid = publicKeyToDidKey(publicKeyBytes);
    return delegation.delegator === expectedDid;
  } catch {
    return false;
  }
}

/**
 * Verify scope narrowing: child scopes must be a strict subset of parent scopes.
 */
export function verifyScopeNarrowing(parent: Delegation, child: Delegation): boolean {
  return child.scopes.every((scope) => parent.scopes.includes(scope));
}

export interface DelegationChainVerificationOptions extends DelegationTemporalOptions {
  rootTrustScore?: number; // Trust score of the root delegator (default 1.0)
}

export interface DelegationChainResult {
  index: number;
  label: string;
  signatureValid: boolean;
  identityValid: boolean;
  scopeValid: boolean;
  temporalValid: boolean;
  effectiveTrust: number;
  error?: string;
}

/**
 * Verify a delegation chain (array of delegations from root to leaf).
 * Checks:
 * 1. Each delegation's signature is valid
 * 2. Each delegator's publicKey matches their DID
 * 3. Each sub-delegation's delegator matches the parent's delegate
 * 4. Scope narrowing is enforced
 * 5. Temporal validity: every hop must be within its signed createdAt/expiresAt
 *    window, evaluated against one shared instant for the whole chain, AND each
 *    child's expiresAt must not exceed its parent's (a child cannot outlive its
 *    delegator, even before the parent expires)
 * 6. Trust attenuation: effective trust decays per hop and must stay above minimum
 */
export async function verifyDelegationChain(
  chain: Delegation[],
  options?: DelegationChainVerificationOptions,
): Promise<{
  valid: boolean;
  results: DelegationChainResult[];
}> {
  if (chain.length === 0) {
    return { valid: false, results: [{ index: 0, label: 'root', signatureValid: false, identityValid: false, scopeValid: false, temporalValid: false, effectiveTrust: 0, error: 'Empty delegation chain' }] };
  }

  // Capture ONE evaluation instant for the whole chain so every hop is judged
  // against the same clock. Fail closed if a supplied verifyAt is unparseable.
  const evalAtMs = resolveVerifyAtMs(options?.verifyAt);
  if (Number.isNaN(evalAtMs)) {
    return { valid: false, results: [{ index: 0, label: 'root', signatureValid: false, identityValid: false, scopeValid: false, temporalValid: false, effectiveTrust: 0, error: 'Unparseable verifyAt evaluation time' }] };
  }

  const results: DelegationChainResult[] = [];
  let effectiveTrust = options?.rootTrustScore ?? 1.0;

  for (let i = 0; i < chain.length; i++) {
    const delegation = chain[i];
    const label = i === 0 ? 'root' : `subdelegation-${i}`;

    const signatureValid = await verifyDelegationSignature(delegation);
    const identityValid = verifyDelegatorIdentity(delegation);
    const temporal = temporalAtMs(delegation, evalAtMs);
    let temporalValid = temporal.valid;
    let temporalError = temporal.error;

    // Temporal narrowing: a child must not outlive its parent. Enforced even
    // while the parent is still live, so a child cannot claim authority in time
    // beyond its delegator's. (The per-hop expiry check above already fails the
    // chain once the parent has actually expired; this closes the window before
    // that.) The own-window error, if any, takes precedence in the message.
    if (i > 0) {
      const parentExpMs = Date.parse(chain[i - 1].expiresAt);
      const childExpMs = Date.parse(delegation.expiresAt);
      if (!Number.isNaN(parentExpMs) && !Number.isNaN(childExpMs) && childExpMs > parentExpMs) {
        temporalValid = false;
        if (temporalError === undefined) {
          temporalError = 'Temporal narrowing violated: expiresAt exceeds parent expiresAt';
        }
      }
    }

    let scopeValid = true;
    if (i > 0) {
      scopeValid = verifyScopeNarrowing(chain[i - 1], delegation);
      // Also verify chain linkage: this delegator should be the parent's delegate
      if (delegation.delegator !== chain[i - 1].delegate) {
        results.push({ index: i, label, signatureValid, identityValid, scopeValid: false, temporalValid, effectiveTrust, error: 'Chain broken: delegator does not match parent delegate' });
        continue;
      }

      // Apply trust attenuation from the parent delegation
      const attenuation = chain[i - 1].trustAttenuation ?? 0.8;
      effectiveTrust = effectiveTrust * attenuation;

      // Check minimum trust threshold
      const minTrust = chain[i - 1].minDelegatedTrust ?? 0.3;
      if (effectiveTrust < minTrust) {
        results.push({
          index: i,
          label,
          signatureValid,
          identityValid,
          scopeValid,
          temporalValid,
          effectiveTrust,
          error: `Effective trust ${effectiveTrust.toFixed(4)} is below minimum threshold ${minTrust}`,
        });
        continue;
      }
    }

    // Surface an expiry/window/narrowing failure as the hop error when nothing worse fired.
    const error = temporalValid ? undefined : temporalError;
    results.push({ index: i, label, signatureValid, identityValid, scopeValid, temporalValid, effectiveTrust, error });
  }

  return {
    valid: results.every((r) => r.signatureValid && r.identityValid && r.scopeValid && r.temporalValid && !r.error),
    results,
  };
}

/**
 * Export a delegation chain in the interop format for posting to the verification thread.
 * If verificationResults are provided, effective trust scores are included per entry.
 */
export function exportDelegationChain(
  chain: Delegation[],
  labels?: string[],
  verificationResults?: DelegationChainResult[],
): DelegationChainExport {
  return {
    engine: 'AIM',
    sdk: '@opena2a/aim-sdk (TypeScript)',
    didMethod: 'did:key (Ed25519)',
    chain: chain.map((d, i) => ({
      delegation: d,
      label: labels?.[i] ?? (i === 0 ? 'root' : `subdelegation_${i}`),
      effectiveTrust: verificationResults?.[i]?.effectiveTrust,
    })),
    verification: {
      algorithm: 'Ed25519',
      encoding: 'base64url',
      signedPayload: 'canonical JSON of delegation fields excluding signature and publicKey (sorted keys, compact separators)',
      didResolution: 'did:key multicodec 0xed01 prefix, base58btc encoded',
      canonicalForm: 'JSON.stringify with sorted keys, no spaces — equivalent to Python json.dumps(sort_keys=True, separators=(",",":"))',
    },
  };
}

// Re-export for convenience
export { toBase64url, fromBase64url, base58btcEncode, base58btcDecode };
