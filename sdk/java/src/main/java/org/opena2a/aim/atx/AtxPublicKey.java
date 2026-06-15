package org.opena2a.aim.atx;

/**
 * A public key the verifier trusts, keyed by algorithm.
 *
 * @param algorithm    "Ed25519" (verified) or "ML-DSA-65" (presence recorded)
 * @param publicKeyHex hex-encoded raw public key (32 bytes for Ed25519)
 * @param keyId        optional DID-URL identifying the key and its controller, e.g.
 *                     {@code did:opena2a:authority:opena2a.org#key-1}. When present
 *                     (contains '#'), the key is BOUND to its controller DID and may
 *                     only verify credentials issued by that DID (or, for v1.1, a
 *                     signed issuerChain authority). A key without a '#' fragment is
 *                     unbound and eligible for any issuer (single-issuer back-compat).
 */
public record AtxPublicKey(String algorithm, String publicKeyHex, String keyId) {

    /** Convenience for an unbound key (no controller binding). */
    public AtxPublicKey(String algorithm, String publicKeyHex) {
        this(algorithm, publicKeyHex, null);
    }
}
