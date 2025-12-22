package org.opena2a.aim.exceptions;

/**
 * Exception thrown when there are issues with credentials.
 */
public class CredentialException extends AIMException {

    public CredentialException(String message) {
        super(message, "CREDENTIAL_ERROR", 401);
    }

    public CredentialException(String message, Throwable cause) {
        super(message, "CREDENTIAL_ERROR", 401, cause);
    }
}
