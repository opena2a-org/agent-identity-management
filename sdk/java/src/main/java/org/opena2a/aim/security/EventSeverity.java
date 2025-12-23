package org.opena2a.aim.security;

/**
 * Severity levels aligned with syslog/SIEM standards.
 */
public enum EventSeverity {
    DEBUG("DEBUG"),
    INFO("INFO"),
    WARNING("WARNING"),
    ERROR("ERROR"),
    CRITICAL("CRITICAL");

    private final String value;

    EventSeverity(String value) {
        this.value = value;
    }

    public String getValue() {
        return value;
    }
}
