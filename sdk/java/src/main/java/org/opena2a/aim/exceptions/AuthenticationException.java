package org.opena2a.aim.exceptions;

/**
 * Exception thrown when authentication fails.
 */
public class AuthenticationException extends AIMException {

    public AuthenticationException(String message) {
        super(message, "AUTH_ERROR", 401);
    }

    public AuthenticationException(String message, Throwable cause) {
        super(message, "AUTH_ERROR", 401, cause);
    }
}
