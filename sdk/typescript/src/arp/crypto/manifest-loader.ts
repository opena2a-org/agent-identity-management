/**
 * Capability manifest YAML loader with hybrid Ed25519+ML-DSA-65 signature
 * verification.
 *
 * The loader implements parse-to-deny (CR-001): any failure at any step --
 * I/O, oversized file, YAML parse error, schema mismatch, missing signature,
 * wrong algorithm, malformed public key, signature rejection, or expiry --
 * raises a `CapabilityManifestError`. Callers must treat the thrown error as
 * an authoritative signal to refuse loading the manifest. There is no
 * partial-success path.
 *
 * Wire format (YAML on disk):
 *
 *   version: "1.0.0"
 *   agentId: "example-agent"
 *   tier: "execute"
 *   comply:
 *     permitted_classes: ["class-a", "class-b"]
 *     prohibited_classes: ["class-c"]
 *     on_violation: "deny"
 *     enforce: true            # optional; default false = detection-only
 *   issuedAt: "2026-04-13T00:00:00.000Z"
 *   expiresAt: "2027-04-13T00:00:00.000Z"
 *   ed25519PublicKey: "<base64>"
 *   mldsa65PublicKey: "<base64>"
 *   signature:
 *     alg: "Ed25519+ML-DSA-65"
 *     ed25519Sig: "<base64>"
 *     mldsaSig: "<base64>"
 *     ts: 1712880000000
 *
 * The signed payload is the manifest object with the `signature` field
 * stripped, serialized to canonical JSON (sorted keys, no whitespace). Signer
 * and verifier must produce byte-identical canonical output or round-tripping
 * will not work. The closed schema (scalars + string arrays + one nested
 * object) is simple enough that full RFC 8785 JCS is unnecessary; the
 * `stableStringify` helper below is deterministic for the shapes we accept.
 *
 * Integration points (deferred to follow-up sessions, not wired yet):
 *   - `src/arp/proxy/server.ts`: load manifest on agent registration, fail
 *     closed if verification raises.
 *   - `src/arp/intelligence/coordinator.ts`: consult the `comply` envelope
 *     when an event's classification lands outside permitted classes.
 */

import { promises as fs } from 'fs';

import yaml from 'js-yaml';

import type {
  CapabilityManifest,
  CapabilityTier,
  ComplyOnViolation,
} from '../types';
import {
  decodeHybridPublicKey,
  decodeHybridSignature,
  hybridVerify,
  validateKeySize,
} from './hybrid-signing';
import type {
  EncodedHybridPublicKey,
  EncodedHybridSignature,
  HybridAlgorithm,
} from './types';

/** Only wire format version currently accepted. */
export const MANIFEST_VERSION = '1.0.0' as const;

/**
 * Only hybrid algorithm permitted for capability manifests. ML-DSA-65 is the
 * NIST category 3 parameter set, matching the AIM SDK default for manifests.
 * ML-DSA-44 is reserved for high-throughput identity signals; ML-DSA-87 for
 * root keys. Manifests that claim a different algorithm are refused.
 */
export const MANIFEST_SIGNATURE_ALGORITHM: HybridAlgorithm =
  'Ed25519+ML-DSA-65';

/**
 * Hard cap on the manifest file size. Anything larger is rejected before
 * YAML parsing to bound worst-case memory and CPU for a malicious file. 64
 * KiB is ample: a realistic manifest with a few hundred permitted classes
 * and base64 public keys sits well under 8 KiB.
 */
export const MAX_MANIFEST_SIZE_BYTES = 64 * 1024;

/** Allowed capability tiers (kept in sync with `CapabilityTier` in ../types). */
const ALLOWED_TIERS: ReadonlySet<CapabilityTier> = new Set<CapabilityTier>([
  'minimal',
  'read',
  'execute',
  'mutate',
  'privileged',
]);

/** Allowed on_violation actions (kept in sync with `ComplyOnViolation`). */
const ALLOWED_ON_VIOLATION: ReadonlySet<ComplyOnViolation> =
  new Set<ComplyOnViolation>(['log', 'alert', 'pause', 'kill', 'deny']);

/**
 * Discrete error codes so callers can switch on the reason without parsing
 * string messages. Keep this in sync with `CapabilityManifestError.code`.
 */
export type CapabilityManifestErrorCode =
  | 'IO_ERROR'
  | 'SIZE_EXCEEDED'
  | 'PARSE_ERROR'
  | 'SCHEMA_ERROR'
  | 'VERSION_UNSUPPORTED'
  | 'SIGNATURE_MISSING'
  | 'ALGORITHM_UNSUPPORTED'
  | 'KEY_FORMAT_ERROR'
  | 'SIGNATURE_INVALID'
  | 'EXPIRED';

/**
 * Error raised on any loader failure. Callers MUST treat a thrown
 * `CapabilityManifestError` as an authoritative parse-to-deny signal. Do not
 * fall through to a "load without verification" path under any circumstance.
 */
export class CapabilityManifestError extends Error {
  constructor(
    public readonly code: CapabilityManifestErrorCode,
    message: string,
    public readonly details?: { reason?: string; cause?: unknown },
  ) {
    super(`[arp/crypto/manifest] ${code}: ${message}`);
    this.name = 'CapabilityManifestError';
  }
}

/**
 * Produce canonical signed-payload bytes for a parsed manifest object.
 *
 * Canonicalization:
 *   1. Remove the `signature` field if present.
 *   2. Serialize as JSON with recursively sorted object keys and no
 *      whitespace between tokens.
 *   3. Encode the result as UTF-8.
 *
 * Exported because fixture generators and out-of-repo signers need to
 * produce the exact same bytes as the verifier. Duplicating this logic in a
 * signer is a recipe for drift.
 */
export function canonicalizeManifestPayload(
  payload: Record<string, unknown>,
): Uint8Array {
  const stripped: Record<string, unknown> = { ...payload };
  delete stripped.signature;
  return new TextEncoder().encode(stableStringify(stripped));
}

/**
 * Deterministic JSON serializer: sorted object keys, no whitespace. Accepts
 * the closed set of JSON-compatible values that manifest payloads contain
 * (string, number, boolean, null, array, plain object). Any other value
 * type is serialized via `JSON.stringify`, which will produce `undefined`
 * for functions or symbols -- the schema guards prevent those from reaching
 * this helper in practice.
 */
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
 * Load and verify a capability manifest from a YAML file on disk.
 *
 * On success, returns the verified `CapabilityManifest` shape (without the
 * signature block, which is not part of the runtime type). On any failure
 * throws a `CapabilityManifestError` -- the caller fails closed.
 *
 * The file is read once, size-checked, then parsed and verified. I/O errors
 * are reported with `code: 'IO_ERROR'` and the underlying error exposed via
 * `details.cause` so callers can log the root cause without re-throwing.
 */
export async function loadCapabilityManifest(
  filePath: string,
): Promise<CapabilityManifest> {
  let raw: string;
  try {
    const stat = await fs.stat(filePath);
    if (stat.size > MAX_MANIFEST_SIZE_BYTES) {
      throw new CapabilityManifestError(
        'SIZE_EXCEEDED',
        `manifest file exceeds ${MAX_MANIFEST_SIZE_BYTES} bytes`,
        { reason: `size=${stat.size}` },
      );
    }
    raw = await fs.readFile(filePath, 'utf8');
  } catch (err) {
    if (err instanceof CapabilityManifestError) throw err;
    throw new CapabilityManifestError(
      'IO_ERROR',
      `unable to read manifest at ${filePath}`,
      { reason: (err as Error).message, cause: err },
    );
  }
  return parseCapabilityManifest(raw);
}

/**
 * Parse and verify a capability manifest from YAML text.
 *
 * Separated from `loadCapabilityManifest` so callers that already have the
 * text in memory (pulled from a database, stdin, IPC channel) can verify
 * without a filesystem round trip.
 */
export async function parseCapabilityManifest(
  yamlText: string,
): Promise<CapabilityManifest> {
  if (typeof yamlText !== 'string') {
    throw new CapabilityManifestError(
      'PARSE_ERROR',
      'manifest text must be a string',
    );
  }
  if (Buffer.byteLength(yamlText, 'utf8') > MAX_MANIFEST_SIZE_BYTES) {
    throw new CapabilityManifestError(
      'SIZE_EXCEEDED',
      `manifest text exceeds ${MAX_MANIFEST_SIZE_BYTES} bytes`,
    );
  }

  let parsed: unknown;
  try {
    parsed = yaml.load(yamlText, {
      // FAILSAFE_SCHEMA would reject base64 strings that look like YAML
      // booleans. CORE_SCHEMA accepts the scalars we use (string, number,
      // bool, null) without executing YAML type coercions like !!js/function.
      schema: yaml.CORE_SCHEMA,
      filename: 'capability-manifest.yaml',
    });
  } catch (err) {
    throw new CapabilityManifestError(
      'PARSE_ERROR',
      'YAML parse failed',
      { reason: (err as Error).message, cause: err },
    );
  }

  if (parsed === null || typeof parsed !== 'object' || Array.isArray(parsed)) {
    throw new CapabilityManifestError(
      'SCHEMA_ERROR',
      'manifest root must be a YAML mapping',
    );
  }

  const obj = parsed as Record<string, unknown>;

  // Version gate first. Rejecting unsupported versions before touching any
  // other field lets us evolve the wire format without leaking half-parsed
  // data from a newer schema into a validator that does not understand it.
  const version = obj.version;
  if (version !== MANIFEST_VERSION) {
    throw new CapabilityManifestError(
      'VERSION_UNSUPPORTED',
      `unsupported manifest version`,
      { reason: `got=${JSON.stringify(version)} expected=${MANIFEST_VERSION}` },
    );
  }

  // Signature block presence check comes before schema validation so that a
  // manifest missing a signature fails with the more specific error code.
  // An attacker who strips the signature block should see the same rejection
  // regardless of whether the remaining schema is well-formed.
  const sigRaw = obj.signature;
  if (sigRaw === undefined || sigRaw === null) {
    throw new CapabilityManifestError(
      'SIGNATURE_MISSING',
      'manifest has no signature block',
    );
  }
  if (typeof sigRaw !== 'object' || Array.isArray(sigRaw)) {
    throw new CapabilityManifestError(
      'SIGNATURE_MISSING',
      'signature block must be a mapping',
    );
  }

  // Schema validation of the payload itself. Narrow order: every required
  // field must be present and of the correct shape before we run any crypto.
  const schemaError = validateManifestSchema(obj);
  if (schemaError) {
    throw new CapabilityManifestError('SCHEMA_ERROR', schemaError);
  }

  // Structural signature checks. Algorithm gate is strict: anything other
  // than the required manifest algorithm is refused outright rather than
  // downgraded.
  const sig = sigRaw as Record<string, unknown>;
  const alg = sig.alg;
  if (alg !== MANIFEST_SIGNATURE_ALGORITHM) {
    throw new CapabilityManifestError(
      'ALGORITHM_UNSUPPORTED',
      `signature algorithm must be ${MANIFEST_SIGNATURE_ALGORITHM}`,
      { reason: `got=${JSON.stringify(alg)}` },
    );
  }
  if (
    typeof sig.ed25519Sig !== 'string' ||
    typeof sig.mldsaSig !== 'string' ||
    typeof sig.ts !== 'number'
  ) {
    throw new CapabilityManifestError(
      'SCHEMA_ERROR',
      'signature block fields malformed',
      {
        reason: `ed25519Sig=${typeof sig.ed25519Sig} mldsaSig=${typeof sig.mldsaSig} ts=${typeof sig.ts}`,
      },
    );
  }

  // Decode keys and signature into raw bytes. Any failure here (bad base64,
  // wrong size after decode) becomes a KEY_FORMAT_ERROR so callers can
  // distinguish "signature cryptographically wrong" from "signature bytes
  // structurally invalid".
  let publicKeyBytes;
  let signatureBytes;
  try {
    const encodedPk: EncodedHybridPublicKey = {
      algorithm: MANIFEST_SIGNATURE_ALGORITHM,
      ed25519PublicKey: obj.ed25519PublicKey as string,
      mldsaPublicKey: obj.mldsa65PublicKey as string,
      mldsaVariant: 'ML-DSA-65',
    };
    publicKeyBytes = decodeHybridPublicKey(encodedPk);

    const encodedSig: EncodedHybridSignature = {
      alg: MANIFEST_SIGNATURE_ALGORITHM,
      ed25519Sig: sig.ed25519Sig,
      mldsaSig: sig.mldsaSig,
      ts: sig.ts,
    };
    signatureBytes = decodeHybridSignature(encodedSig);
  } catch (err) {
    throw new CapabilityManifestError(
      'KEY_FORMAT_ERROR',
      'failed to decode hybrid key or signature bytes',
      { reason: (err as Error).message, cause: err },
    );
  }

  // Node's `Buffer.from(_, 'base64')` is permissive: invalid characters are
  // silently dropped and the result is whatever byte length falls out. Catch
  // size mismatches here so the caller sees KEY_FORMAT_ERROR rather than a
  // downstream SIGNATURE_INVALID that hides a structurally malformed key.
  if (!validateKeySize('Ed25519', 'publicKey', publicKeyBytes.ed25519PublicKey)) {
    throw new CapabilityManifestError(
      'KEY_FORMAT_ERROR',
      'ed25519 public key has wrong length after base64 decode',
      { reason: `length=${publicKeyBytes.ed25519PublicKey.length}` },
    );
  }
  if (!validateKeySize('ML-DSA-65', 'publicKey', publicKeyBytes.mldsaPublicKey)) {
    throw new CapabilityManifestError(
      'KEY_FORMAT_ERROR',
      'ml-dsa-65 public key has wrong length after base64 decode',
      { reason: `length=${publicKeyBytes.mldsaPublicKey.length}` },
    );
  }
  if (!validateKeySize('Ed25519', 'signature', signatureBytes.ed25519Sig)) {
    throw new CapabilityManifestError(
      'KEY_FORMAT_ERROR',
      'ed25519 signature has wrong length after base64 decode',
      { reason: `length=${signatureBytes.ed25519Sig.length}` },
    );
  }
  if (!validateKeySize('ML-DSA-65', 'signature', signatureBytes.mldsaSig)) {
    throw new CapabilityManifestError(
      'KEY_FORMAT_ERROR',
      'ml-dsa-65 signature has wrong length after base64 decode',
      { reason: `length=${signatureBytes.mldsaSig.length}` },
    );
  }

  // Canonicalize the signed payload (the original parsed object minus the
  // signature block) and run the non-short-circuit hybrid verifier. Both
  // halves are evaluated on every call; a structured `{valid: false, reason}`
  // result comes back on any mismatch and becomes SIGNATURE_INVALID here.
  const canonical = canonicalizeManifestPayload(obj);
  const result = await hybridVerify(canonical, signatureBytes, publicKeyBytes);
  if (!result.valid) {
    throw new CapabilityManifestError(
      'SIGNATURE_INVALID',
      'hybrid signature rejected',
      { reason: result.reason ?? 'no reason provided by verifier' },
    );
  }

  // Expiry check runs LAST, after signature verification, so an attacker who
  // manipulates expiresAt cannot force a cheap rejection path that reveals
  // anything about the signature state. Even a valid signature over an
  // expired manifest is denied.
  if (typeof obj.expiresAt === 'string') {
    const expiresAtMs = Date.parse(obj.expiresAt);
    if (!Number.isFinite(expiresAtMs)) {
      throw new CapabilityManifestError(
        'SCHEMA_ERROR',
        'expiresAt is not a valid ISO timestamp',
        { reason: `got=${obj.expiresAt}` },
      );
    }
    if (expiresAtMs <= Date.now()) {
      throw new CapabilityManifestError(
        'EXPIRED',
        'manifest expiresAt has passed',
        { reason: `expiresAt=${obj.expiresAt}` },
      );
    }
  }

  // The runtime shape: everything except the signature block.
  const manifest: CapabilityManifest = {
    version: MANIFEST_VERSION,
    agentId: obj.agentId as string,
    tier: obj.tier as CapabilityTier,
    comply: {
      permitted_classes: ((obj.comply as Record<string, unknown>)
        .permitted_classes as string[]).slice(),
      prohibited_classes: ((obj.comply as Record<string, unknown>)
        .prohibited_classes as string[]).slice(),
      on_violation: (obj.comply as Record<string, unknown>)
        .on_violation as ComplyOnViolation,
      ...(typeof (obj.comply as Record<string, unknown>).enforce === 'boolean'
        ? { enforce: (obj.comply as Record<string, unknown>).enforce as boolean }
        : {}),
    },
    issuedAt: obj.issuedAt as string,
    ...(typeof obj.expiresAt === 'string'
      ? { expiresAt: obj.expiresAt }
      : {}),
    ed25519PublicKey: obj.ed25519PublicKey as string,
    mldsa65PublicKey: obj.mldsa65PublicKey as string,
  };
  return manifest;
}

/**
 * Schema validation for the non-signature fields. Returns a human-readable
 * error string if validation fails, or `null` on success. Kept as a
 * string-returning helper (rather than throwing) so the caller can wrap the
 * result in a single typed error with the correct code.
 */
function validateManifestSchema(obj: Record<string, unknown>): string | null {
  if (typeof obj.agentId !== 'string' || obj.agentId.length === 0) {
    return 'agentId must be a non-empty string';
  }
  if (typeof obj.tier !== 'string' || !ALLOWED_TIERS.has(obj.tier as CapabilityTier)) {
    return `tier must be one of ${Array.from(ALLOWED_TIERS).join(', ')}`;
  }
  if (typeof obj.issuedAt !== 'string' || !Number.isFinite(Date.parse(obj.issuedAt))) {
    return 'issuedAt must be an ISO timestamp string';
  }
  if (
    obj.expiresAt !== undefined &&
    obj.expiresAt !== null &&
    typeof obj.expiresAt !== 'string'
  ) {
    return 'expiresAt must be an ISO timestamp string or omitted';
  }
  if (typeof obj.ed25519PublicKey !== 'string' || obj.ed25519PublicKey.length === 0) {
    return 'ed25519PublicKey must be a non-empty base64 string';
  }
  if (typeof obj.mldsa65PublicKey !== 'string' || obj.mldsa65PublicKey.length === 0) {
    return 'mldsa65PublicKey must be a non-empty base64 string';
  }

  const comply = obj.comply;
  if (comply === null || typeof comply !== 'object' || Array.isArray(comply)) {
    return 'comply must be a mapping';
  }
  const c = comply as Record<string, unknown>;
  if (
    !Array.isArray(c.permitted_classes) ||
    !c.permitted_classes.every((v) => typeof v === 'string')
  ) {
    return 'comply.permitted_classes must be an array of strings';
  }
  if (
    !Array.isArray(c.prohibited_classes) ||
    !c.prohibited_classes.every((v) => typeof v === 'string')
  ) {
    return 'comply.prohibited_classes must be an array of strings';
  }
  if (
    typeof c.on_violation !== 'string' ||
    !ALLOWED_ON_VIOLATION.has(c.on_violation as ComplyOnViolation)
  ) {
    return `comply.on_violation must be one of ${Array.from(ALLOWED_ON_VIOLATION).join(', ')}`;
  }
  if (c.enforce !== undefined && typeof c.enforce !== 'boolean') {
    return 'comply.enforce must be a boolean or omitted';
  }

  return null;
}
