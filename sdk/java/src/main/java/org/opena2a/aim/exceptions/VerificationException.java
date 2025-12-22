package org.opena2a.aim.exceptions;

/**
 * Exception thrown when verification of an action fails.
 */
public class VerificationException extends AIMException {

    public VerificationException(String message) {
        super(message, "VERIFICATION_ERROR", 403);
    }

    public VerificationException(String message, Throwable cause) {
        super(message, "VERIFICATION_ERROR", 403, cause);
    }
}
