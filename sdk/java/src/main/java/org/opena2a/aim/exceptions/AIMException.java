package org.opena2a.aim.exceptions;

/**
 * Base exception for all AIM SDK errors.
 */
public class AIMException extends RuntimeException {

    private final String errorCode;
    private final int statusCode;

    public AIMException(String message) {
        super(message);
        this.errorCode = "AIM_ERROR";
        this.statusCode = 500;
    }

    public AIMException(String message, Throwable cause) {
        super(message, cause);
        this.errorCode = "AIM_ERROR";
        this.statusCode = 500;
    }

    public AIMException(String message, String errorCode, int statusCode) {
        super(message);
        this.errorCode = errorCode;
        this.statusCode = statusCode;
    }

    public AIMException(String message, String errorCode, int statusCode, Throwable cause) {
        super(message, cause);
        this.errorCode = errorCode;
        this.statusCode = statusCode;
    }

    public String getErrorCode() {
        return errorCode;
    }

    public int getStatusCode() {
        return statusCode;
    }
}
