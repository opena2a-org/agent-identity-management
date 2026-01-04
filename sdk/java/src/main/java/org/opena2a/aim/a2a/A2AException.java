package org.opena2a.aim.a2a;

import org.opena2a.aim.exceptions.AIMException;

/**
 * Exception for A2A (Agent-to-Agent) protocol operations.
 */
public class A2AException extends AIMException {

    /**
     * Creates a new A2AException with a message.
     *
     * @param message the error message
     */
    public A2AException(String message) {
        super(message, "A2A_ERROR", 500);
    }

    /**
     * Creates a new A2AException with a message and error code.
     *
     * @param message the error message
     * @param errorCode the specific error code
     */
    public A2AException(String message, String errorCode) {
        super(message, errorCode, 500);
    }

    /**
     * Creates a new A2AException with full details.
     *
     * @param message the error message
     * @param errorCode the specific error code
     * @param statusCode the HTTP status code
     */
    public A2AException(String message, String errorCode, int statusCode) {
        super(message, errorCode, statusCode);
    }

    /**
     * Creates a new A2AException with a message and cause.
     *
     * @param message the error message
     * @param cause the underlying cause
     */
    public A2AException(String message, Throwable cause) {
        super(message, "A2A_ERROR", 500, cause);
    }

    /**
     * Creates a new A2AException with full details and a cause.
     *
     * @param message the error message
     * @param errorCode the specific error code
     * @param cause the underlying cause
     */
    public A2AException(String message, String errorCode, Throwable cause) {
        super(message, errorCode, 500, cause);
    }
}
