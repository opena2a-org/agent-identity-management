package org.opena2a.aim.security;

/**
 * Security event categories for SOC/SIEM classification.
 */
public enum EventCategory {
    /** Authentication events (token refresh, credential load) */
    AUTHN("AUTHN"),
    /** Authorization events (capability verification, action execution) */
    AUTHZ("AUTHZ"),
    /** Agent lifecycle events (registration, update, deletion) */
    AGENT("AGENT"),
    /** Credential management events (save, load, migrate) */
    CRED("CRED"),
    /** MCP integration events (connection, attestation) */
    MCP("MCP"),
    /** Security-specific events (anomalies, policy violations) */
    SECURITY("SECURITY"),
    /** Audit trail events */
    AUDIT("AUDIT"),
    /** Configuration changes */
    CONFIG("CONFIG");

    private final String value;

    EventCategory(String value) {
        this.value = value;
    }

    /**
     * Get the string value of this category.
     *
     * @return the category string
     */
    public String getValue() {
        return value;
    }
}
