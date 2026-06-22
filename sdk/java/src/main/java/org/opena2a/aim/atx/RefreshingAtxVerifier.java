package org.opena2a.aim.atx;

import java.time.Clock;
import java.util.List;
import java.util.Objects;

/**
 * A {@link LocalAtxVerifier} fronted by an asynchronously-refreshed {@link CrlCache}.
 *
 * <p>The trusted issuers, public keys and clock are fixed at construction (they change
 * rarely and are fetched once); the revocation list rides on the cache. At each
 * {@link #verify(Atx)} the current cached CRL is read (never a network call) and handed
 * to a {@link LocalAtxVerifier} — the conformance-locked verifier is not modified, only
 * fed the current list. Building the verifier per call is allocation-only and negligible
 * next to the Ed25519 verification.
 *
 * <p>Stale-cache handling follows the cache's {@link CrlCache.StalePolicy}: {@code SOFT_OPEN}
 * (default) verifies without revocation once the list is beyond its TTL — signature, expiry
 * and issuer-trust stay hard; {@code REJECT} fails the credential closed (reported as
 * {@link RejectCategory#REVOKED} with a reason that distinguishes it from a real revocation).
 */
public final class RefreshingAtxVerifier {

    private final List<String> trustedIssuers;
    private final List<AtxPublicKey> publicKeys;
    private final Clock clock;
    private final CrlCache crlCache;

    /**
     * @param trustedIssuers issuer DIDs the verifier trusts
     * @param publicKeys     issuer public keys (Ed25519 verified; ML-DSA-65 recorded)
     * @param clock          clock source (nullable; system UTC when null)
     * @param crlCache       async-refreshed revocation cache (required; call {@link CrlCache#start()} to begin refreshing)
     */
    public RefreshingAtxVerifier(
            List<String> trustedIssuers, List<AtxPublicKey> publicKeys, Clock clock, CrlCache crlCache) {
        this.trustedIssuers = Objects.requireNonNull(trustedIssuers, "trustedIssuers");
        this.publicKeys = Objects.requireNonNull(publicKeys, "publicKeys");
        this.clock = clock;
        this.crlCache = Objects.requireNonNull(crlCache, "crlCache");
    }

    /** Verify a credential against the fixed anchors and the cache's current revocation list. */
    public AtxVerificationResult verify(Atx atx) {
        // Fail-closed stale policy: when the list is beyond TTL we cannot confirm the
        // credential is unrevoked, so deny rather than soft-open. Reported as REVOKED
        // (closest spec category) with a reason distinguishing it from a real revocation.
        if (crlCache.onStale() == CrlCache.StalePolicy.REJECT && !crlCache.isFresh()) {
            return AtxVerificationResult.reject(
                    RejectCategory.REVOKED,
                    "revocation list is stale (beyond TTL) and stale-policy is REJECT; failing closed");
        }
        AtxTrustAnchors anchors = new AtxTrustAnchors(trustedIssuers, publicKeys, crlCache.current(), clock);
        return new LocalAtxVerifier(anchors).verify(atx);
    }

    /** The revocation cache backing this verifier. */
    public CrlCache crlCache() {
        return crlCache;
    }
}
