package org.opena2a.aim.exceptions;

/**
 * Exception thrown when authentication fails.
 */
public class AuthenticationException extends AIMException {

    /**
     * Creates a new AuthenticationException with a message.
     *
     * @param message the error message
     */
    public AuthenticationException(String message) {
        super(message, "AUTH_ERROR", 401);
    }

    /**
     * Creates a new AuthenticationException with a message and cause.
     *
     * @param message the error message
     * @param cause the underlying cause of the exception
     */
    public AuthenticationException(String message, Throwable cause) {
        super(message, "AUTH_ERROR", 401, cause);
    }
}
