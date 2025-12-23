package org.opena2a.aim.exceptions;

/**
 * Base exception for all AIM SDK errors.
 */
public class AIMException extends RuntimeException {

    private final String errorCode;
    private final int statusCode;

    /**
     * Creates a new AIMException with a message.
     *
     * @param message the error message
     */
    public AIMException(String message) {
        super(message);
        this.errorCode = "AIM_ERROR";
        this.statusCode = 500;
    }

    /**
     * Creates a new AIMException with a message and cause.
     *
     * @param message the error message
     * @param cause the underlying cause of the exception
     */
    public AIMException(String message, Throwable cause) {
        super(message, cause);
        this.errorCode = "AIM_ERROR";
        this.statusCode = 500;
    }

    /**
     * Creates a new AIMException with full details.
     *
     * @param message the error message
     * @param errorCode the error code identifying the type of error
     * @param statusCode the HTTP status code associated with the error
     */
    public AIMException(String message, String errorCode, int statusCode) {
        super(message);
        this.errorCode = errorCode;
        this.statusCode = statusCode;
    }

    /**
     * Creates a new AIMException with full details and a cause.
     *
     * @param message the error message
     * @param errorCode the error code identifying the type of error
     * @param statusCode the HTTP status code associated with the error
     * @param cause the underlying cause of the exception
     */
    public AIMException(String message, String errorCode, int statusCode, Throwable cause) {
        super(message, cause);
        this.errorCode = errorCode;
        this.statusCode = statusCode;
    }

    /**
     * Gets the error code identifying the type of error.
     *
     * @return the error code
     */
    public String getErrorCode() {
        return errorCode;
    }

    /**
     * Gets the HTTP status code associated with the error.
     *
     * @return the HTTP status code
     */
    public int getStatusCode() {
        return statusCode;
    }
}
