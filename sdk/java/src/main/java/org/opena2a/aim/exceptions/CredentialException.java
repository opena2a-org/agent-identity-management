package org.opena2a.aim.exceptions;

/**
 * Exception thrown when there are issues with credentials.
 */
public class CredentialException extends AIMException {

    /**
     * Creates a new CredentialException with a message.
     *
     * @param message the error message
     */
    public CredentialException(String message) {
        super(message, "CREDENTIAL_ERROR", 401);
    }

    /**
     * Creates a new CredentialException with a message and cause.
     *
     * @param message the error message
     * @param cause the underlying cause of the exception
     */
    public CredentialException(String message, Throwable cause) {
        super(message, "CREDENTIAL_ERROR", 401, cause);
    }
}
