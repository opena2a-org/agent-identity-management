package org.opena2a.aim.crypto.pqc;

/**
 * Exception thrown for PQC-related errors.
 */
public class PQCException extends RuntimeException {

    public PQCException(String message) {
        super(message);
    }

    public PQCException(String message, Throwable cause) {
        super(message, cause);
    }

    /**
     * Exception thrown when PQC is not available
     */
    public static class PQCNotAvailableException extends PQCException {
        public PQCNotAvailableException() {
            super("PQC support requires BouncyCastle with ML-DSA support (version 1.75+)");
        }
    }

    /**
     * Exception thrown for signature verification failures
     */
    public static class SignatureVerificationException extends PQCException {
        public SignatureVerificationException(String message) {
            super(message);
        }
    }

    /**
     * Exception thrown for key generation failures
     */
    public static class KeyGenerationException extends PQCException {
        public KeyGenerationException(String message, Throwable cause) {
            super(message, cause);
        }
    }
}
