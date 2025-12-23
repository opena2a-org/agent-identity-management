package org.opena2a.aim.exceptions;

/**
 * Exception thrown for configuration errors (invalid parameters, missing fields, etc.).
 */
public class ConfigurationException extends AIMException {

    /**
     * Creates a new ConfigurationException with a message.
     *
     * @param message the error message
     */
    public ConfigurationException(String message) {
        super(message, "CONFIGURATION_ERROR", 400);
    }

    /**
     * Creates a new ConfigurationException with a message and cause.
     *
     * @param message the error message
     * @param cause the underlying cause of the exception
     */
    public ConfigurationException(String message, Throwable cause) {
        super(message, cause);
    }
}
