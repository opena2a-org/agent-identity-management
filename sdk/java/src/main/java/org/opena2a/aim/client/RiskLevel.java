package org.opena2a.aim.client;

/**
 * Risk levels for agent actions.
 */
public enum RiskLevel {
    LOW("low"),
    MEDIUM("medium"),
    HIGH("high"),
    CRITICAL("critical");

    private final String value;

    RiskLevel(String value) {
        this.value = value;
    }

    public String getValue() {
        return value;
    }

    public static RiskLevel fromString(String text) {
        for (RiskLevel level : RiskLevel.values()) {
            if (level.value.equalsIgnoreCase(text)) {
                return level;
            }
        }
        return MEDIUM;
    }
}
