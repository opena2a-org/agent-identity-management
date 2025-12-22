package org.opena2a.aim.exceptions;

/**
 * Exception thrown for configuration errors (invalid parameters, missing fields, etc.).
 */
public class ConfigurationException extends AIMException {

    public ConfigurationException(String message) {
        super(message, "CONFIGURATION_ERROR", 400);
    }

    public ConfigurationException(String message, Throwable cause) {
        super(message, cause);
    }
}
