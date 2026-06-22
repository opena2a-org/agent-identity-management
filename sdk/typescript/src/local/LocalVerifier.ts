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
 * No verifier is reimplemented here — duplicating it would reintroduce exactly the
 * drift the cross-language conformance gate exists to prevent. The package is
 * loaded via a cached dynamic `import()` so this works identically from both the
 * CJS and ESM builds of the SDK on every supported Node (the package is ESM-only).
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
  AtxVerifier,
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
   * Issuer public keys keyed by algorithm. Ed25519 is verified; ML-DSA-65
   * presence is recorded. Set each key's `keyId` to a DID-URL to bind it to its
   * controller (required to be safe with a multi-issuer anchor set).
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
  /** Whether an ML-DSA-65 signature was present (delegated, not silently skipped). */
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
  private verifier: AtxVerifier | null = null;

  constructor(config: LocalVerificationConfig) {
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

  private async getVerifier(): Promise<AtxVerifier> {
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

  /** Cryptographically verify an ATX credential offline. No network access. */
  async verifyCredential(atx: Atx): Promise<AtxVerificationResult> {
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
    return verifier.verify(atx);
  }

  /**
   * Verify the credential, then evaluate the local broker policy for `action`.
   * Fails closed: a credential that does not verify denies the action.
   */
  async authorize(atx: Atx, options: LocalAuthorizationOptions): Promise<LocalAuthorizationResult> {
    const result = await this.verifyCredential(atx);

    if (!result.valid || !result.context) {
      return {
        verified: false,
        actionAllowed: false,
        agentId: atx.agentId ?? '',
        agentDid: atx.agentDid ?? '',
        issuerDid: atx.issuerDid ?? '',
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
