package org.opena2a.aim.atx;

/**
 * Why an ATX credential was rejected. Mirrors the {@code RejectCategory} union in
 * {@code @opena2a/atx-verify} (TS) and the reject reasons in the Go/Python
 * reference verifiers so the structured outcome is interoperable across SDKs.
 */
public enum RejectCategory {
    UNSUPPORTED_VERSION,
    EXPIRED,
    REVOKED,
    UNTRUSTED_ISSUER,
    SIGNATURE_INVALID,
    MALFORMED
}
