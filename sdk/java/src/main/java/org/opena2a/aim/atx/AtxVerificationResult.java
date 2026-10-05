package org.opena2a.aim.atx;

/**
 * Outcome of {@link LocalAtxVerifier#verify}. On success {@code valid} is true and
 * {@code context} is present; on failure {@code rejectCategory} and {@code reason}
 * explain why.
 *
 * @param valid          whether the credential verified
 * @param context        present iff valid — the authorization context
 * @param rejectCategory present iff invalid
 * @param reason         human-readable reason (this verifier's own wording)
 * @param mldsaPresent   whether the accepted credential carried an ML-DSA-65 signature;
 *                       when true, every such signature verified (always false on reject)
 */
public record AtxVerificationResult(
        boolean valid,
        ResolutionContext context,
        RejectCategory rejectCategory,
        String reason,
        boolean mldsaPresent) {

    static AtxVerificationResult reject(RejectCategory category, String reason) {
        return new AtxVerificationResult(false, null, category, reason, false);
    }

    static AtxVerificationResult accept(ResolutionContext context, boolean mldsaPresent) {
        return new AtxVerificationResult(true, context, null, null, mldsaPresent);
    }
}
