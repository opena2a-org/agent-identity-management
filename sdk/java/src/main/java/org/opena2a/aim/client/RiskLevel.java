package org.opena2a.aim.client;

/**
 * Risk levels for agent actions.
 *
 * <p>Used with {@code @SecureAction} annotation to specify the risk level
 * of an action. Can be set to {@code AUTO} to enable automatic detection
 * based on capability patterns using the {@code RiskDetector}.</p>
 *
 * <h2>Usage:</h2>
 * <pre>{@code
 * // Explicit risk level
 * @SecureAction(capability = "payment:process", resource = "stripe", riskLevel = RiskLevel.HIGH)
 * public void processPayment() { ... }
 *
 * // Auto-detected risk level (recommended)
 * @SecureAction(capability = "payment:process", resource = "stripe", riskLevel = RiskLevel.AUTO)
 * public void processPayment() { ... }
 * // AUTO will detect as HIGH based on "payment:" pattern
 * }</pre>
 */
public enum RiskLevel {
    /**
     * Auto-detect risk level from capability pattern.
     * Uses {@code RiskDetector} to analyze the capability string.
     */
    AUTO("auto"),

    /**
     * Low risk - read-only operations, non-sensitive APIs.
     */
    LOW("low"),

    /**
     * Medium risk - data access, configuration changes.
     */
    MEDIUM("medium"),

    /**
     * High risk - financial operations, user management, data export.
     */
    HIGH("high"),

    /**
     * Critical risk - system administration, data destruction.
     */
    CRITICAL("critical");

    private final String value;

    RiskLevel(String value) {
        this.value = value;
    }

    public String getValue() {
        return value;
    }

    /**
     * Parse risk level from string.
     */
    public static RiskLevel fromString(String text) {
        if (text == null) return MEDIUM;
        for (RiskLevel level : RiskLevel.values()) {
            if (level.value.equalsIgnoreCase(text)) {
                return level;
            }
        }
        return MEDIUM;
    }

    /**
     * Convert from security.RiskLevel to client.RiskLevel.
     */
    public static RiskLevel fromSecurityRiskLevel(org.opena2a.aim.security.RiskLevel secLevel) {
        if (secLevel == null) return MEDIUM;
        switch (secLevel) {
            case CRITICAL: return CRITICAL;
            case HIGH: return HIGH;
            case MEDIUM: return MEDIUM;
            case LOW: return LOW;
            default: return MEDIUM;
        }
    }

    /**
     * Convert to security.RiskLevel.
     */
    public org.opena2a.aim.security.RiskLevel toSecurityRiskLevel() {
        switch (this) {
            case CRITICAL: return org.opena2a.aim.security.RiskLevel.CRITICAL;
            case HIGH: return org.opena2a.aim.security.RiskLevel.HIGH;
            case MEDIUM: return org.opena2a.aim.security.RiskLevel.MEDIUM;
            case LOW: return org.opena2a.aim.security.RiskLevel.LOW;
            case AUTO: return org.opena2a.aim.security.RiskLevel.MEDIUM; // fallback
            default: return org.opena2a.aim.security.RiskLevel.MEDIUM;
        }
    }
}
