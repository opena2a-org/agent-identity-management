/**
 * Local, offline ATX credential verification for the AIM agent SDK.
 *
 * The ATX spec (core §9, AAP-SPEC §3.3) mandates that credential verification is
 * local: the issuing node is never on the hot path. This module brings that model
 * to the agent-side SDK by wrapping the shared, conformance-locked verifier
 * `@opena2a/atx-verify` (the SAME `LocalAtxVerifier` the secretless broker and the
 * Go/Python reference verifiers agree with byte-for-byte). It then evaluates a
 * minimal local broker policy over the credential's *signed* claims.
 *
 * The shared verifier's parse, canonicalization and Ed25519 check are not
 * reimplemented here: duplicating them would reintroduce exactly the drift the
 * cross-language conformance gate exists to prevent. Two checks it leaves out are
 * added after it accepts, over the same canonical payload, so the SDK returns the
 * conformance suite's verdict on every vendored fixture as the Java SDK's
 * verifier does: every declared ML-DSA-65 signature must verify and an unknown
 * signature algorithm rejects, and a v1.1 issuerChain authority lends its bound
 * keys only when it is itself a trusted issuer. The package is loaded via a
 * cached dynamic `import()` so this works identically from both the CJS and ESM
 * builds of the SDK on every supported Node (the package is ESM-only).
 *
 * Network is reserved for credential *resolution* (the AAP broker hands the agent
 * its ATX) and the periodic CRL refresh — never for a per-action decision.
 */

// Type-only imports are erased at build time, so they never force a runtime
// `require()` of the ESM-only package. Values are loaded via dynamic import below.
import type {
  Atx,
  AtxPublicKey,
  AtxTrustAnchors,
  AtxVerificationResult,
  LocalAtxVerifier,
  RejectCategory,
  ResolutionContext,
} from '@opena2a/atx-verify';

// Re-export the credential types so SDK consumers get them without a second
// dependency on @opena2a/atx-verify.
export type {
  Atx,
  AtxPublicKey,
  AtxTrustAnchors,
  AtxVerificationResult,
  RejectCategory,
  ResolutionContext,
} from '@opena2a/atx-verify';

import { CrlCache } from './CrlCache';
import { ConfigurationError } from '../exceptions';
import { verifyMLDSA } from '../crypto/pqc';
export { CrlCache } from './CrlCache';
export type { CrlData, CrlStalePolicy, CrlCacheConfig, CrlCacheStatus } from './CrlCache';

type AtxVerifyModule = typeof import('@opena2a/atx-verify');

// One module load per process, shared across every LocalVerifier instance.
let modulePromise: Promise<AtxVerifyModule> | null = null;
function loadAtxVerify(): Promise<AtxVerifyModule> {
  if (!modulePromise) {
    modulePromise = import('@opena2a/atx-verify');
  }
  return modulePromise;
}

/**
 * Trust anchors and clock for local verification. In production these are fetched
 * once from AIM/the Registry and cached; revocation rides on the short-lived,
 * asynchronously-refreshed CRL (soft-fail) per AAP §6.
 *
 * SECURITY — multi-issuer anchor sets are safe when each key carries a DID-URL
 * `keyId`. `@opena2a/atx-verify` (>= 0.2.0) binds a key to its controller DID:
 * a key whose `keyId` is a DID-URL (e.g. `did:…:opena2a.org#key-1`) may only
 * verify credentials issued by that DID (or, for v1.1, a signed issuerChain
 * authority), so one trusted issuer's key cannot satisfy a credential issued
 * under a different issuer's DID. Always set a DID-URL `keyId` on each key when
 * `publicKeys` holds keys for more than one issuer. A key without a `keyId`
 * fragment is unbound and eligible for any issuer — fine for a single-issuer
 * anchor set, unsafe for a multi-issuer one.
 */
export interface LocalVerificationConfig {
  /** Issuer DIDs the verifier trusts. */
  trustedIssuers: string[];
  /**
   * Issuer public keys keyed by algorithm: `Ed25519` (32-byte raw key, hex) and
   * `ML-DSA-65` (1952-byte raw FIPS 204 key, hex). Every signature the credential
   * declares must verify against an eligible key of its algorithm, so an issuer
   * that signs with both needs both keys here. Set each key's `keyId` to a
   * DID-URL to bind it to its controller (required to be safe with a
   * multi-issuer anchor set).
   */
  publicKeys: AtxPublicKey[];
  /**
   * Static cached federated revocation list. Use this for a list you refresh yourself;
   * for background-refreshed revocation prefer {@link LocalVerificationConfig.crlCache}.
   * Absence soft-fails open on revocation only.
   */
  crl?: { entries: Array<{ agentId: string; reason?: string }> };
  /**
   * Asynchronously-refreshed revocation cache. When set, the verifier reads the
   * cache's current list at each verification (never on a network call) and applies
   * the cache's stale policy: `soft-open` skips revocation once the list is beyond
   * its TTL, `reject` fails the credential closed. Takes precedence over {@link crl}.
   * Call {@link CrlCache.start} once to begin background refresh.
   */
  crlCache?: CrlCache;
  /** Injectable clock (tests). Defaults to the wall clock. */
  now?: () => Date;
}

/** Inputs to the local broker policy for a single action. */
export interface LocalAuthorizationOptions {
  /** The capability/action the agent is attempting; matched against the credential's capabilities. */
  action: string;
  /**
   * Require the credential to be v1.1+ (capabilities covered by the signature)
   * before authorizing on capabilities. Default `true`: a v1.0 ATX's capabilities
   * are forgeable by the holder and MUST NOT be trusted for authorization.
   *
   * WARNING: setting this to `false` authorizes on holder-forgeable capabilities.
   * Any holder of a validly-signed v1.0 credential can edit its (unsigned)
   * `capabilities` to grant themselves anything — including `"*"`. Only enable
   * this for a closed, trusted v1.0 deployment where the holder is not the
   * adversary; never for credentials that cross a trust boundary.
   */
  requireSignedCapabilities?: boolean;
}

/** The outcome of a local verify-then-authorize. Never throws; inspect the fields. */
export interface LocalAuthorizationResult {
  /** Credential is cryptographically valid (signature, expiry, revocation, issuer trust). */
  verified: boolean;
  /** Local broker-policy verdict for the requested action. */
  actionAllowed: boolean;
  agentId: string;
  agentDid: string;
  issuerDid: string;
  trustLevel: number;
  trustScore: number;
  capabilities: string[];
  /** True iff capabilities are covered by the signature (v1.1). */
  signedCapabilities: boolean;
  /** Present when verification failed. */
  rejectCategory?: RejectCategory;
  /** Present when `verified` is false or `actionAllowed` is false. */
  denialReason?: string;
  /**
   * Whether the credential declared an ML-DSA-65 signature. On a verified
   * credential, every ML-DSA-65 signature it declares verified.
   */
  mldsaPresent?: boolean;
  /** Always `'local'` — distinguishes this from the remote PDP path. */
  source: 'local';
}

/**
 * Verify ATX credentials offline against cached trust anchors and evaluate a
 * local broker policy. Reusable on its own or via {@link AIMClient.verifyActionLocally}.
 */
export class LocalVerifier {
  private readonly anchors: AtxTrustAnchors;
  private readonly crlCache?: CrlCache;
  private verifier: LocalAtxVerifier | null = null;

  constructor(config: LocalVerificationConfig) {
    // Trust anchors of the wrong shape must fail loudly here, not later as a
    // denial whose reason points at the credential. From JavaScript (or a
    // mis-cast config) a string where an array belongs was stored silently and
    // every verification failed closed with a misleading reject.
    if (
      !Array.isArray(config.trustedIssuers) ||
      config.trustedIssuers.some((issuer) => typeof issuer !== 'string')
    ) {
      throw new ConfigurationError(
        'localVerification.trustedIssuers must be an array of issuer DID strings',
      );
    }
    if (
      !Array.isArray(config.publicKeys) ||
      config.publicKeys.some((key) => typeof key !== 'object' || key === null)
    ) {
      throw new ConfigurationError(
        'localVerification.publicKeys must be an array of AtxPublicKey objects',
      );
    }
    // A malformed ML-DSA-65 key is unusable. Dropped at verification, it made
    // every hybrid credential reject with "no eligible ML-DSA-65 trust anchor".
    for (const key of config.publicKeys) {
      if (key.algorithm === 'ML-DSA-65' && mldsa65KeyFromHex(key.publicKeyHex) === null) {
        const length = typeof key.publicKeyHex === 'string' ? `${key.publicKeyHex.length} characters` : 'not a string';
        throw new ConfigurationError(
          `localVerification.publicKeys: ML-DSA-65 key ${key.keyId ?? '(no keyId)'} must be 3904 hex characters ` +
            `(a 1952-byte raw FIPS 204 key); got ${length}`,
        );
      }
    }
    this.anchors = {
      trustedIssuers: config.trustedIssuers,
      publicKeys: config.publicKeys,
      crl: config.crl,
      now: config.now,
    };
    this.crlCache = config.crlCache;
  }

  /**
   * Pre-load the verifier module so the first {@link verifyCredential} is also
   * sub-millisecond. Optional — `verifyCredential`/`authorize` load it lazily.
   */
  async ready(): Promise<void> {
    await this.getVerifier();
  }

  private async getVerifier(): Promise<LocalAtxVerifier> {
    if (this.crlCache) {
      // The cached CRL changes over time, so build the verifier with the current
      // list at each call. Construction is allocation-only (it just stores the
      // anchors); the cost is negligible next to the Ed25519 verify. `current()`
      // is null when the list is stale — soft-open then means revocation is not
      // enforced (signature/expiry/issuer stay hard); the `reject` policy is
      // handled in verifyCredential before we get here.
      const { LocalAtxVerifier } = await loadAtxVerify();
      const crl = this.crlCache.current() ?? undefined;
      return new LocalAtxVerifier({ ...this.anchors, crl });
    }
    // No CRL cache: the verifier is immutable, so build it once and reuse it (the
    // module is loaded only on that first build, not on every call).
    if (!this.verifier) {
      const { LocalAtxVerifier } = await loadAtxVerify();
      this.verifier = new LocalAtxVerifier(this.anchors);
    }
    return this.verifier;
  }

  /**
   * Cryptographically verify an ATX credential offline. No network access.
   *
   * Pass the credential as it arrived on the wire — its raw JSON text or bytes —
   * whenever you have it. Raw input is strict-parsed before any field is read: a
   * credential carrying a duplicate member at any depth, including two names that
   * differ only in case, rejects as `MALFORMED` (the conformance suite's
   * `PARSE_ERROR`). A parsed object cannot get that check, because `JSON.parse`
   * has already kept one of the duplicates and dropped the other.
   */
  async verifyCredential(credential: Atx | string | Uint8Array): Promise<AtxVerificationResult> {
    // Fail-closed stale policy: when a revocation cache is configured with
    // onStale='reject' and its list is beyond TTL, we cannot confirm the
    // credential is unrevoked, so deny rather than soft-open. Treated as REVOKED
    // (the closest spec category) with a reason that distinguishes it from a real
    // revocation. soft-open caches fall through and simply verify without a CRL.
    if (this.crlCache && this.crlCache.onStale === 'reject' && !this.crlCache.isFresh()) {
      return {
        valid: false,
        rejectCategory: 'REVOKED' as RejectCategory,
        reason:
          'revocation list is stale (beyond TTL) and the cache stale-policy is "reject"; failing closed',
      } as AtxVerificationResult;
    }
    const verifier = await this.getVerifier();
    const result = isRawCredential(credential)
      ? verifier.verifyCredential(credential)
      : verifier.verify(credential);
    if (!result.valid) {
      return result;
    }
    // The shared verifier accepted: raw input passed the strict parse, so parsing
    // it again here reads the same credential it read.
    const atx = isRawCredential(credential) ? parseStrictlyParsed(credential) : credential;
    const rejection = await this.checkSignaturesBeyondSharedVerifier(atx);
    return rejection ? { ...rejection, mldsaPresent: result.mldsaPresent } : result;
  }

  /**
   * The signature checks `@opena2a/atx-verify` leaves out, run on a credential it
   * accepted. It verifies Ed25519 only, recording an ML-DSA-65 entry without
   * checking it and passing over an algorithm it does not know; and for v1.1 it
   * lets the keys bound to any issuerChain DID verify, trusted issuer or not, so
   * a signer could name itself in the chain and sign for a trusted issuer
   * (atx-spec core.md section 1.3 step 4). Returns the rejection, or null.
   */
  private async checkSignaturesBeyondSharedVerifier(atx: Atx): Promise<AtxVerificationResult | null> {
    const atxVerify = await loadAtxVerify();
    const isV11 = atx.atcVersion === atxVerify.SUPPORTED_ATX_VERSION_V11;
    const authorities = new Set([atx.issuerDid]);
    const chain = isV11 && Array.isArray(atx.issuerChain) ? atx.issuerChain : [];
    for (const did of chain) {
      if (this.anchors.trustedIssuers.includes(did)) {
        authorities.add(did);
      }
    }
    const eligibleKeys = this.anchors.publicKeys.filter((k) => keyEligible(k.keyId, authorities));

    // Ed25519 again, without the keys of chain DIDs that are not trusted issuers.
    // Only needed when the chain names such a DID; otherwise the eligible set is
    // the one the shared verifier already used.
    if (chain.some((did) => !authorities.has(did))) {
      const strict = new atxVerify.LocalAtxVerifier({ ...this.anchors, publicKeys: eligibleKeys }).verify(atx);
      if (!strict.valid) {
        return strict;
      }
    }

    const mldsaKeys = eligibleKeys
      .filter((k) => k.algorithm === 'ML-DSA-65')
      .map((k) => mldsa65KeyFromHex(k.publicKeyHex))
      .filter((k): k is Uint8Array => k !== null);
    let payload: Uint8Array | null = null;
    for (const sig of Array.isArray(atx.signatures) ? atx.signatures : []) {
      if (sig.algorithm === 'Ed25519') {
        continue;
      }
      if (sig.algorithm !== 'ML-DSA-65') {
        // ATX defines Ed25519 and ML-DSA-65 only. Rejecting, not skipping, keeps a
        // misspelled or look-alike algorithm name from carrying an unchecked entry.
        return signatureInvalid(`unsupported signature algorithm: ${String(sig.algorithm)}`);
      }
      if (mldsaKeys.length === 0) {
        return signatureInvalid(
          `no eligible ML-DSA-65 trust anchor for this credential (issuer ${atx.issuerDid})`,
        );
      }
      payload ??= isV11 ? atxVerify.canonicalPayloadV11(atx) : atxVerify.canonicalPayload(atx);
      const sigBytes = typeof sig.value === 'string' ? Buffer.from(sig.value, 'base64') : null;
      if (!sigBytes || !(await anyMldsa65KeyVerifies(mldsaKeys, payload, sigBytes))) {
        return signatureInvalid(`ML-DSA-65 signature ${sig.keyId ?? ''} did not verify`);
      }
    }
    return null;
  }

  /**
   * Verify the credential, then evaluate the local broker policy for `action`.
   * Fails closed: a credential that does not verify denies the action.
   *
   * Accepts the credential as a parsed object or as its raw JSON text or bytes;
   * see {@link verifyCredential}. When raw input fails verification, the identity
   * fields of the result are empty: a credential that did not verify is not
   * parsed to fill them.
   */
  async authorize(
    credential: Atx | string | Uint8Array,
    options: LocalAuthorizationOptions,
  ): Promise<LocalAuthorizationResult> {
    const result = await this.verifyCredential(credential);

    if (!result.valid || !result.context) {
      const claimed = isRawCredential(credential) ? undefined : credential;
      return {
        verified: false,
        actionAllowed: false,
        agentId: claimed?.agentId ?? '',
        agentDid: claimed?.agentDid ?? '',
        issuerDid: claimed?.issuerDid ?? '',
        trustLevel: 0,
        trustScore: 0,
        capabilities: [],
        signedCapabilities: false,
        rejectCategory: result.rejectCategory,
        denialReason: result.reason ?? 'credential verification failed',
        mldsaPresent: result.mldsaPresent,
        source: 'local',
      };
    }

    const ctx = result.context;
    const requireSigned = options.requireSignedCapabilities !== false;
    const decision = evaluateBrokerPolicy(ctx, options.action, requireSigned);

    return {
      verified: true,
      actionAllowed: decision.allowed,
      agentId: ctx.agentId,
      agentDid: ctx.agentDid,
      issuerDid: ctx.issuerDid,
      trustLevel: ctx.trustLevel,
      trustScore: ctx.trustScore,
      capabilities: ctx.capabilities,
      signedCapabilities: ctx.signedCapabilities,
      denialReason: decision.allowed ? undefined : decision.reason,
      mldsaPresent: result.mldsaPresent,
      source: 'local',
    };
  }
}

/**
 * True for a credential given as raw JSON text or bytes. `ArrayBuffer.isView`
 * reads an internal slot rather than the prototype chain, so it cannot throw;
 * a view other than a Uint8Array is passed on and rejected as MALFORMED.
 */
function isRawCredential(credential: Atx | string | Uint8Array): credential is string | Uint8Array {
  return typeof credential === 'string' || ArrayBuffer.isView(credential);
}

/**
 * Parses raw input the shared verifier has accepted, decoding bytes as it does
 * (fatal UTF-8, BOM kept), so the result is the credential it verified.
 */
function parseStrictlyParsed(credential: string | Uint8Array): Atx {
  const text =
    typeof credential === 'string'
      ? credential
      : new TextDecoder('utf-8', { fatal: true, ignoreBOM: true }).decode(credential);
  return JSON.parse(text) as Atx;
}

/**
 * Whether a key may verify a signature for one of `authorities`, by the shared
 * verifier's rule: a key whose keyId is a DID-URL (contains '#') is bound to the
 * controller DID before the '#'; a key with no fragment is unbound and eligible.
 */
function keyEligible(keyId: string | undefined, authorities: Set<string>): boolean {
  if (!keyId || !keyId.includes('#')) {
    return true;
  }
  return authorities.has(keyId.slice(0, keyId.indexOf('#')));
}

/**
 * A raw 1952-byte ML-DSA-65 public key from hex, or null when the hex is
 * malformed or the wrong length. The constructor refuses such a key; at
 * verification a null key is still left out, so a declared ML-DSA-65 signature
 * with no usable key rejects instead of passing.
 */
function mldsa65KeyFromHex(hex: string): Uint8Array | null {
  if (typeof hex !== 'string' || !/^[0-9a-fA-F]{3904}$/.test(hex)) {
    return null;
  }
  return Uint8Array.from(Buffer.from(hex, 'hex'));
}

async function anyMldsa65KeyVerifies(
  keys: Uint8Array[],
  payload: Uint8Array,
  signature: Uint8Array,
): Promise<boolean> {
  for (const key of keys) {
    if (await verifyMLDSA('ML-DSA-65', key, payload, signature)) {
      return true;
    }
  }
  return false;
}

function signatureInvalid(reason: string): AtxVerificationResult {
  return { valid: false, rejectCategory: 'SIGNATURE_INVALID' as RejectCategory, reason };
}

/**
 * Minimal local broker policy: an action is allowed iff a *signed* capability
 * grants it. A richer policy (resource scoping, role mapping) plugs in here.
 */
function evaluateBrokerPolicy(
  ctx: ResolutionContext,
  action: string,
  requireSigned: boolean,
): { allowed: boolean; reason?: string } {
  // v1.0 capabilities are not under the signature (forgeable by the holder), so by
  // default refuse to authorize on them. The security note in @opena2a/atx-verify
  // is the source of this rule.
  if (requireSigned && !ctx.signedCapabilities) {
    return {
      allowed: false,
      reason:
        'capabilities are not covered by the signature (credential is v1.0); refusing capability-based authorization',
    };
  }
  if (ctx.capabilities.includes('*') || ctx.capabilities.includes(action)) {
    return { allowed: true };
  }
  return { allowed: false, reason: `action "${action}" is not in the credential's granted capabilities` };
}
