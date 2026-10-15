package org.opena2a.aim.client;

/**
 * Enum representing supported AI agent types: the agent types the AIM server
 * accepts at registration and filters a list of agents by.
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
    /**
     * Demo agents registered by {@code aim-sdk demo}: visibly a demo everywhere
     * they appear, and excluded from adoption/trust analytics by this one value.
     */
    DEMO("demo"),
    /** Google Gemini-based agent */
    GEMINI("gemini"),
    /** Meta Llama-based agent */
    LLAMA("llama"),
    /** Mistral-based agent */
    MISTRAL("mistral"),
    /** Cohere-based agent */
    COHERE("cohere"),
    /** LlamaIndex-based agent */
    LLAMAINDEX("llamaindex"),
    /** LangGraph-based agent */
    LANGGRAPH("langgraph"),
    /** Haystack-based agent */
    HAYSTACK("haystack"),
    /** Microsoft Semantic Kernel-based agent */
    SEMANTIC_KERNEL("semantic_kernel"),
    /** Code assistant (copilot) agent */
    COPILOT("copilot"),
    /** General-purpose assistant agent */
    ASSISTANT("assistant"),
    /** Conversational chatbot agent */
    CHATBOT("chatbot"),
    /** AutoGPT-based autonomous agent */
    AUTOGPT("autogpt"),
    /** BabyAGI-based autonomous agent */
    BABYAGI("babyagi"),
    /** Generic AI agent */
    AI_AGENT("ai_agent"),
    /**
     * Unknown or unrecognized agent type: {@link #fromString} returns it for a
     * value no other type has. It is sent as {@code ai_agent}, the value of
     * {@link #AI_AGENT}, so registering with it or filtering a list of agents
     * by it uses an agent type the server defines.
     */
    UNKNOWN("ai_agent");

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
     * @return the corresponding AgentType ({@link #AI_AGENT} for {@code ai_agent}),
     *         or UNKNOWN if not recognized
     */
    public static AgentType fromString(String text) {
        for (AgentType type : AgentType.values()) {
            if (type != UNKNOWN && type.value.equalsIgnoreCase(text)) {
                return type;
            }
        }
        return UNKNOWN;
    }
}
