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
 * SECURITY — single-issuer anchor sets only (current limitation). The underlying
 * `@opena2a/atx-verify` verifier tries a credential's signature against EVERY
 * Ed25519 key in `publicKeys` and does not bind a key to the issuer DID that owns
 * it (`AtxPublicKey` carries no issuerDid/keyId). So if `publicKeys` holds keys
 * for more than one issuer, a credential claiming issuer A but signed by issuer
 * B's trusted key would verify and be attributed to A. Until key-to-issuer
 * binding lands upstream, configure anchors for exactly ONE trusted issuer per
 * `LocalVerifier` instance (one entry in `trustedIssuers`, only that issuer's
 * keys in `publicKeys`). Use separate instances for federated multi-issuer setups.
 */
export interface LocalVerificationConfig {
  /**
   * Issuer DIDs the verifier trusts. See the single-issuer security note above:
   * prefer exactly one entry until upstream key-to-issuer binding ships.
   */
  trustedIssuers: string[];
  /** Issuer public keys keyed by algorithm. Ed25519 is verified; ML-DSA-65 presence is recorded. */
  publicKeys: AtxPublicKey[];
  /** Cached federated revocation list. Refresh off the hot path; absence soft-fails open on revocation only. */
  crl?: { entries: Array<{ agentId: string; reason?: string }> };
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
  private verifier: AtxVerifier | null = null;

  constructor(config: LocalVerificationConfig) {
    this.anchors = {
      trustedIssuers: config.trustedIssuers,
      publicKeys: config.publicKeys,
      crl: config.crl,
      now: config.now,
    };
  }

  /**
   * Pre-load the verifier module so the first {@link verifyCredential} is also
   * sub-millisecond. Optional — `verifyCredential`/`authorize` load it lazily.
   */
  async ready(): Promise<void> {
    await this.getVerifier();
  }

  private async getVerifier(): Promise<AtxVerifier> {
    if (!this.verifier) {
      const { LocalAtxVerifier } = await loadAtxVerify();
      this.verifier = new LocalAtxVerifier(this.anchors);
    }
    return this.verifier;
  }

  /** Cryptographically verify an ATX credential offline. No network access. */
  async verifyCredential(atx: Atx): Promise<AtxVerificationResult> {
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
