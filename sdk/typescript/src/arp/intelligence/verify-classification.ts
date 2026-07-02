/**
 * NanoMind-Guard classification verifier (AIComply P1, producer side).
 *
 * This module is the producer-side companion to the L0-comply gate on
 * `IntelligenceCoordinator`. NanoMind-Guard produces classification labels
 * for runtime events; every result it emits is hybrid-signed with
 * Ed25519+ML-DSA-44 (the high-throughput variant, deliberately chosen over
 * ML-DSA-65 to keep per-event signing cost bounded). Before the coordinator
 * can trust a classification, the signature must verify and the class must
 * pass a tier-keyed rejection matrix that bounds what a given capability
 * tier can ever emit, regardless of what the per-manifest permitted list
 * says.
 *
 * The order of checks is:
 *
 *   1. Schema sanity of the result object and signature block.
 *   2. Algorithm gate: the signature must advertise Ed25519+ML-DSA-44.
 *      Anything else is refused before any crypto runs, so an attacker
 *      cannot downgrade the Guard to a weaker algorithm.
 *   3. Key and signature byte-size checks after base64 decode. Node's
 *      `Buffer.from(_, 'base64')` is permissive, so malformed inputs are
 *      caught here as KEY_FORMAT_ERROR rather than falling through to a
 *      SIGNATURE_INVALID that hides a structural problem.
 *   4. Hybrid signature verification over the canonical signed payload.
 *      Both halves must pass; the underlying `hybridVerify` is
 *      non-short-circuit for timing reasons.
 *   5. Freshness: the signed timestamp must be within `maxAgeMs` of now and
 *      must not be more than `futureSkewMs` in the future.
 *   6. Rejection matrix keyed on `manifest.tier`. Any classification on the
 *      absolute deny list is rejected regardless of tier. Any classification
 *      whose minimum required tier exceeds the manifest's tier is rejected.
 *      Any classification not in the registry is rejected as unknown
 *      (parse-to-deny).
 *
 * A verified result returns `{valid: true, classification}`; the caller
 * assigns `event.data.classification = result.classification` before
 * handing the event to the coordinator, where the session 26 comply gate
 * takes over and enforces the per-manifest permitted/prohibited lists. This
 * is a defense-in-depth composition: the rejection matrix here is a hard
 * ceiling that even a permissive manifest cannot override; the per-manifest
 * lists are a narrower policy on top of that ceiling.
 *
 * Parse-to-deny (CR-001) applies to every failure path. The function never
 * throws; all rejection branches return a typed `{valid: false}` object.
 * Callers must treat `valid === false` as authoritative and refuse to
 * populate `event.data.classification` on failure.
 */

import type {
  CapabilityTier,
  NanoMindGuardResult,
  NanoMindGuardVerifyErrorCode,
  NanoMindGuardVerifyOptions,
  NanoMindGuardVerifyResult,
} from '../types';
import {
  decodeHybridPublicKey,
  decodeHybridSignature,
  hybridVerify,
  validateKeySize,
} from '../crypto/hybrid-signing';
import type {
  EncodedHybridPublicKey,
  EncodedHybridSignature,
  HybridAlgorithm,
} from '../crypto/types';

/**
 * Only hybrid algorithm accepted for NanoMind-Guard classification results.
 * ML-DSA-44 is the FIPS 204 high-throughput variant. Do not accept ML-DSA-65
 * here: the manifest loader uses ML-DSA-65 for long-lived capability
 * manifests, and keeping the two algorithm lanes disjoint prevents a
 * manifest key from being mistakenly used to mint per-event classifications.
 */
export const GUARD_SIGNATURE_ALGORITHM: HybridAlgorithm =
  'Ed25519+ML-DSA-44';

/**
 * Default freshness window for a Guard signature. Per-event signing is
 * expected to complete well under a second, so 5 minutes is a generous
 * upper bound that still closes the replay window.
 */
export const DEFAULT_MAX_AGE_MS = 5 * 60 * 1000;

/**
 * Default tolerance for signatures that appear to be timestamped in the
 * future. Accounts for small clock skew between the Guard and the
 * coordinator; anything beyond this is a bug or an attack.
 */
export const DEFAULT_FUTURE_SKEW_MS = 30 * 1000;

/**
 * Total order on capability tiers. A classification requiring tier T is
 * permitted only when the manifest's tier is at least T. Synchronized with
 * `CapabilityTier` in `../types.ts`.
 */
const TIER_LEVEL: Record<CapabilityTier, number> = {
  minimal: 0,
  read: 1,
  execute: 2,
  mutate: 3,
  privileged: 4,
};

/**
 * Minimum capability tier required to act on a given classification. A
 * classification not in this registry is treated as unknown and denied
 * outright (parse-to-deny, CR-001). Callers that want to allow a new class
 * must add it here first; silent acceptance is a vulnerability.
 *
 * The exact values are policy, not physics: they are the floor below which
 * ARP will not execute the class regardless of the per-manifest envelope.
 * A more permissive tier still has to pass the per-manifest comply gate
 * inside the coordinator before the class actually runs.
 */
export const CLASSIFICATION_MIN_TIER: Readonly<Record<string, CapabilityTier>> = {
  // Tier 0: minimal -- read-only, no side effects
  'documentation': 'minimal',
  // Tier 1: read -- observe state, no mutation
  'code-analysis': 'read',
  'data-read': 'read',
  // Tier 2: execute -- sandboxed side effects and controlled egress
  'code-generation': 'execute',
  'network-egress': 'execute',
  'process-spawn': 'execute',
  // Tier 3: mutate -- write user-visible state
  'data-write': 'mutate',
  'file-write': 'mutate',
  // Tier 4: privileged -- irreversible or cross-user effects
  'file-delete': 'privileged',
  'system-mutation': 'privileged',
  'privileged-operation': 'privileged',
};

/**
 * Classifications that are denied at every tier, including privileged.
 * Raw credential access is always routed through the credential broker, so
 * a Guard result labelled `credential-access` or `credential-exfiltration`
 * is by construction a signal that something is wrong. Parse-to-deny.
 */
export const ABSOLUTE_DENY_CLASSES: ReadonlySet<string> = new Set([
  'credential-access',
  'credential-exfiltration',
]);

/**
 * Produce canonical signed-payload bytes for a NanoMind-Guard result.
 * Exported so out-of-repo signers can produce byte-identical payloads. Any
 * drift between signer and verifier defeats verification.
 *
 * Canonicalization rules:
 *   1. Remove the `signature` field if present.
 *   2. Serialize as JSON with recursively sorted object keys and no
 *      whitespace between tokens.
 *   3. Encode as UTF-8.
 */
export function canonicalizeGuardResultPayload(
  payload: Record<string, unknown>,
): Uint8Array {
  const stripped: Record<string, unknown> = { ...payload };
  delete stripped.signature;
  return new TextEncoder().encode(stableStringify(stripped));
}

function stableStringify(value: unknown): string {
  if (value === null || typeof value !== 'object') return JSON.stringify(value);
  if (Array.isArray(value)) {
    return `[${value.map((v) => stableStringify(v)).join(',')}]`;
  }
  const obj = value as Record<string, unknown>;
  const keys = Object.keys(obj).sort();
  const pairs = keys.map(
    (k) => `${JSON.stringify(k)}:${stableStringify(obj[k])}`,
  );
  return `{${pairs.join(',')}}`;
}

/**
 * Verify a NanoMind-Guard classification result and clear it against the
 * capability manifest's tier. Never throws; every failure path returns a
 * typed `{valid: false, code, reason}` the caller routes on.
 *
 * On success, the returned `classification` is the cleared label the caller
 * should write into `event.data.classification` before handing the event to
 * the coordinator. The per-manifest permitted/prohibited envelope is NOT
 * applied here; the coordinator's L0-comply gate handles that layer.
 */
export async function verifyClassification(
  result: NanoMindGuardResult,
  options: NanoMindGuardVerifyOptions,
): Promise<NanoMindGuardVerifyResult> {
  // 1. Schema sanity. We refuse to crypto-verify a malformed object.
  const schemaError = validateResultSchema(result);
  if (schemaError !== null) {
    return invalid('SCHEMA_ERROR', schemaError);
  }

  // 2. Algorithm gate. The Guard MUST advertise Ed25519+ML-DSA-44. Refusing
  // anything else prevents algorithm downgrade, and prevents a manifest
  // key (ML-DSA-65) from being replayed as a classification signing key.
  const sig = result.signature;
  if (sig.alg !== GUARD_SIGNATURE_ALGORITHM) {
    return invalid(
      'ALGORITHM_UNSUPPORTED',
      `signature algorithm must be ${GUARD_SIGNATURE_ALGORITHM}, got ${JSON.stringify(sig.alg)}`,
    );
  }
  if (options.guardPublicKey.algorithm !== GUARD_SIGNATURE_ALGORITHM) {
    return invalid(
      'ALGORITHM_UNSUPPORTED',
      `guard public key algorithm must be ${GUARD_SIGNATURE_ALGORITHM}, got ${JSON.stringify(options.guardPublicKey.algorithm)}`,
    );
  }
  if (options.guardPublicKey.mldsaVariant !== 'ML-DSA-44') {
    return invalid(
      'ALGORITHM_UNSUPPORTED',
      `guard public key mldsaVariant must be ML-DSA-44, got ${JSON.stringify(options.guardPublicKey.mldsaVariant)}`,
    );
  }

  // 3. Decode keys and signature. Base64 decode followed by FIPS 204 size
  // checks; any failure here is KEY_FORMAT_ERROR so callers can tell
  // "structurally malformed" apart from "cryptographically wrong".
  let publicKeyBytes;
  let signatureBytes;
  try {
    const encodedPk: EncodedHybridPublicKey = options.guardPublicKey;
    publicKeyBytes = decodeHybridPublicKey(encodedPk);

    const encodedSig: EncodedHybridSignature = {
      alg: GUARD_SIGNATURE_ALGORITHM,
      ed25519Sig: sig.ed25519Sig,
      mldsaSig: sig.mldsaSig,
      ts: sig.ts,
    };
    signatureBytes = decodeHybridSignature(encodedSig);
  } catch (err) {
    return invalid(
      'KEY_FORMAT_ERROR',
      `failed to decode hybrid key or signature bytes: ${(err as Error).message}`,
    );
  }

  if (!validateKeySize('Ed25519', 'publicKey', publicKeyBytes.ed25519PublicKey)) {
    return invalid(
      'KEY_FORMAT_ERROR',
      `ed25519 public key has wrong length: ${publicKeyBytes.ed25519PublicKey.length}`,
    );
  }
  if (!validateKeySize('ML-DSA-44', 'publicKey', publicKeyBytes.mldsaPublicKey)) {
    return invalid(
      'KEY_FORMAT_ERROR',
      `ml-dsa-44 public key has wrong length: ${publicKeyBytes.mldsaPublicKey.length}`,
    );
  }
  if (!validateKeySize('Ed25519', 'signature', signatureBytes.ed25519Sig)) {
    return invalid(
      'KEY_FORMAT_ERROR',
      `ed25519 signature has wrong length: ${signatureBytes.ed25519Sig.length}`,
    );
  }
  if (!validateKeySize('ML-DSA-44', 'signature', signatureBytes.mldsaSig)) {
    return invalid(
      'KEY_FORMAT_ERROR',
      `ml-dsa-44 signature has wrong length: ${signatureBytes.mldsaSig.length}`,
    );
  }

  // 4. Canonicalize the signed payload (result minus signature block) and
  // run the non-short-circuit hybrid verifier. Both halves are evaluated on
  // every call.
  const canonical = canonicalizeGuardResultPayload(
    result as unknown as Record<string, unknown>,
  );
  const verifyResult = await hybridVerify(
    canonical,
    signatureBytes,
    publicKeyBytes,
  );
  if (!verifyResult.valid) {
    return invalid(
      'SIGNATURE_INVALID',
      verifyResult.reason ?? 'hybrid signature rejected by verifier',
    );
  }

  // 5. Freshness check. Runs AFTER signature verification so that an
  // attacker who manipulates the timestamp cannot distinguish "signature
  // broken" from "timestamp too old" via timing or error code. A valid
  // signature over a stale timestamp is still STALE.
  const now = (options.now ?? Date.now)();
  const maxAgeMs = options.maxAgeMs ?? DEFAULT_MAX_AGE_MS;
  const futureSkewMs = options.futureSkewMs ?? DEFAULT_FUTURE_SKEW_MS;
  const age = now - result.timestamp;
  if (age > maxAgeMs) {
    return invalid(
      'STALE',
      `signature timestamp is ${age} ms old, exceeds maxAgeMs=${maxAgeMs}`,
    );
  }
  if (-age > futureSkewMs) {
    return invalid(
      'FUTURE_DATED',
      `signature timestamp is ${-age} ms in the future, exceeds futureSkewMs=${futureSkewMs}`,
    );
  }

  // 6. Tier rejection matrix. The absolute deny list is checked first and
  // applies at every tier. Unknown classifications are parse-to-deny.
  if (ABSOLUTE_DENY_CLASSES.has(result.classification)) {
    return invalid(
      'ABSOLUTE_DENY',
      `classification ${JSON.stringify(result.classification)} is on the absolute deny list`,
    );
  }
  const requiredTier = CLASSIFICATION_MIN_TIER[result.classification];
  if (requiredTier === undefined) {
    return invalid(
      'UNKNOWN_CLASSIFICATION',
      `classification ${JSON.stringify(result.classification)} is not in the tier registry; parse-to-deny`,
    );
  }
  const manifestLevel = TIER_LEVEL[options.manifest.tier];
  const requiredLevel = TIER_LEVEL[requiredTier];
  if (manifestLevel < requiredLevel) {
    return invalid(
      'TIER_REJECTED',
      `classification ${JSON.stringify(result.classification)} requires tier ${requiredTier}, manifest tier is ${options.manifest.tier}`,
    );
  }

  return {
    valid: true,
    classification: result.classification,
    tier: options.manifest.tier,
    confidence: result.confidence,
  };
}

/**
 * Validate the runtime shape of a `NanoMindGuardResult`. Returns a
 * human-readable error string on failure, or `null` if the result passes.
 * Kept as a string-returning helper so the caller can wrap the result in a
 * typed `invalid()` without mixing exception and value control flow.
 */
function validateResultSchema(result: NanoMindGuardResult): string | null {
  if (result === null || typeof result !== 'object') {
    return 'result must be an object';
  }
  if (typeof result.classification !== 'string' || result.classification.length === 0) {
    return 'classification must be a non-empty string';
  }
  if (
    typeof result.confidence !== 'number' ||
    !Number.isFinite(result.confidence) ||
    result.confidence < 0 ||
    result.confidence > 1
  ) {
    return 'confidence must be a finite number in [0, 1]';
  }
  if (typeof result.modelVersion !== 'string' || result.modelVersion.length === 0) {
    return 'modelVersion must be a non-empty string';
  }
  if (
    typeof result.contentHash !== 'string' ||
    !/^[0-9a-fA-F]{64}$/.test(result.contentHash)
  ) {
    return 'contentHash must be a 64-character hex string (SHA-256)';
  }
  if (typeof result.timestamp !== 'number' || !Number.isFinite(result.timestamp)) {
    return 'timestamp must be a finite number (Unix ms)';
  }
  const sig = result.signature;
  if (sig === null || typeof sig !== 'object') {
    return 'signature must be a mapping';
  }
  if (typeof sig.alg !== 'string') {
    return 'signature.alg must be a string';
  }
  if (typeof sig.ed25519Sig !== 'string' || sig.ed25519Sig.length === 0) {
    return 'signature.ed25519Sig must be a non-empty base64 string';
  }
  if (typeof sig.mldsaSig !== 'string' || sig.mldsaSig.length === 0) {
    return 'signature.mldsaSig must be a non-empty base64 string';
  }
  if (typeof sig.ts !== 'number' || !Number.isFinite(sig.ts)) {
    return 'signature.ts must be a finite number';
  }
  return null;
}

/**
 * Ensure the structurally identical invalid-result shape across every
 * failure branch. Kept as a tiny helper so the TypeScript discriminant on
 * `valid` narrows correctly in callers.
 */
function invalid(
  code: NanoMindGuardVerifyErrorCode,
  reason: string,
): NanoMindGuardVerifyResult {
  return { valid: false, code, reason };
}

/**
 * Convenience wrapper: verify a Guard result and, on success, write the
 * cleared classification into `event.data.classification` so the
 * coordinator's L0-comply gate can consume it. Returns the verification
 * result regardless of outcome so callers can route on the reason without
 * re-running verification. On rejection, `event.data.classification` is
 * LEFT UNCHANGED -- a failed verification must never plant a classification.
 *
 * The `event.data` argument is typed as `Record<string, unknown>` because
 * that matches `ARPEvent.data`; the caller passes `event.data` directly.
 */
export async function applyVerifiedClassification(
  eventData: Record<string, unknown>,
  result: NanoMindGuardResult,
  options: NanoMindGuardVerifyOptions,
): Promise<NanoMindGuardVerifyResult> {
  const verifyResult = await verifyClassification(result, options);
  if (verifyResult.valid) {
    eventData.classification = verifyResult.classification;
  }
  return verifyResult;
}
