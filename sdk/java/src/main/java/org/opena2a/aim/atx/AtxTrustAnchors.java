package org.opena2a.aim.atx;

import java.time.Clock;
import java.util.List;

/**
 * Trust anchors the verifier evaluates against. In production these are fetched
 * once from AIM/the Registry and cached; revocation rides on the short-lived,
 * asynchronously-refreshed CRL.
 *
 * <p>SECURITY — multi-issuer anchor sets are safe only when each key carries a
 * DID-URL {@code keyId}. A key bound to its controller DID may only verify
 * credentials issued by that DID (see {@link AtxPublicKey}). For a multi-issuer
 * anchor set, always set a DID-URL keyId on every key.
 *
 * @param trustedIssuers issuer DIDs the verifier trusts
 * @param publicKeys     issuer public keys (Ed25519 verified; ML-DSA-65 recorded)
 * @param crl            cached revocation list (may be null)
 * @param clock          clock source (injectable for tests); when null, the system UTC clock is used
 */
public record AtxTrustAnchors(
        List<String> trustedIssuers,
        List<AtxPublicKey> publicKeys,
        Crl crl,
        Clock clock) {

    public AtxTrustAnchors(List<String> trustedIssuers, List<AtxPublicKey> publicKeys) {
        this(trustedIssuers, publicKeys, null, null);
    }

    /** A cached, federated certificate revocation list. */
    public record Crl(List<Entry> entries) {
        public record Entry(String agentId, String reason) {}
    }
}
