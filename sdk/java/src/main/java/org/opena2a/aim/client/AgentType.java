package org.opena2a.aim.client;

/**
 * Enum representing supported AI agent types.
 */
public enum AgentType {
    /** LangChain-based agent */
    LANGCHAIN("langchain"),
    /** CrewAI-based agent */
    CREWAI("crewai"),
    /** Microsoft AutoGen-based agent */
    AUTOGEN("autogen"),
    /** OpenAI GPT-based agent */
    OPENAI("gpt"),
    /** Anthropic Claude-based agent */
    ANTHROPIC("claude"),
    /** Custom agent implementation */
    CUSTOM("custom"),
    /** Unknown or unrecognized agent type */
    UNKNOWN("unknown");

    private final String value;

    AgentType(String value) {
        this.value = value;
    }

    /**
     * Gets the string value of this agent type.
     *
     * @return the agent type value
     */
    public String getValue() {
        return value;
    }

    /**
     * Converts a string to an AgentType enum value.
     *
     * @param text the string representation of the agent type
     * @return the corresponding AgentType, or UNKNOWN if not recognized
     */
    public static AgentType fromString(String text) {
        for (AgentType type : AgentType.values()) {
            if (type.value.equalsIgnoreCase(text)) {
                return type;
            }
        }
        return UNKNOWN;
    }
}
