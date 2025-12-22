package org.opena2a.aim.client;

/**
 * Enum representing supported AI agent types.
 */
public enum AgentType {
    LANGCHAIN("langchain"),
    CREWAI("crewai"),
    AUTOGEN("autogen"),
    OPENAI("gpt"),
    ANTHROPIC("claude"),
    CUSTOM("custom"),
    UNKNOWN("unknown");

    private final String value;

    AgentType(String value) {
        this.value = value;
    }

    public String getValue() {
        return value;
    }

    public static AgentType fromString(String text) {
        for (AgentType type : AgentType.values()) {
            if (type.value.equalsIgnoreCase(text)) {
                return type;
            }
        }
        return UNKNOWN;
    }
}
