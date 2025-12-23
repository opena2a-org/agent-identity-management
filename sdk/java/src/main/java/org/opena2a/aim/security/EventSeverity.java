package org.opena2a.aim.security;

/**
 * Severity levels aligned with syslog/SIEM standards.
 */
public enum EventSeverity {
    /** Debug level - detailed information for debugging. */
    DEBUG("DEBUG"),
    /** Info level - general informational messages. */
    INFO("INFO"),
    /** Warning level - potential issues that should be monitored. */
    WARNING("WARNING"),
    /** Error level - errors that need attention. */
    ERROR("ERROR"),
    /** Critical level - severe issues requiring immediate action. */
    CRITICAL("CRITICAL");

    private final String value;

    EventSeverity(String value) {
        this.value = value;
    }

    /**
     * Get the string value of this severity level.
     *
     * @return the severity level string
     */
    public String getValue() {
        return value;
    }
}
