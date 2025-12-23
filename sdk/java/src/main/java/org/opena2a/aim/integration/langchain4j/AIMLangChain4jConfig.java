package org.opena2a.aim.integration.langchain4j;

import org.opena2a.aim.client.AIMClient;
import org.opena2a.aim.client.AgentType;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.util.List;
import java.util.Map;

/**
 * LangChain4j integration configuration for AIM SDK.
 *
 * <p>Provides integration components for LangChain4j-based AI applications:</p>
 * <ul>
 *   <li>Chat model listener for security logging</li>
 *   <li>Tool execution decorator for capability tracking</li>
 *   <li>Memory handler for audit trail</li>
 *   <li>Agent callback handler for full observability</li>
 * </ul>
 *
 * <h2>Usage with LangChain4j:</h2>
 * <pre>{@code
 * // Create AIM-integrated chat model
 * AIMClient aimClient = AIMLangChain4jConfig.builder()
 *     .agentName("my-langchain4j-agent")
 *     .capabilities(List.of("chat:complete", "tool:execute"))
 *     .build();
 *
 * // Wrap your chat model
 * ChatLanguageModel wrappedModel = AIMLangChain4jConfig.wrap(
 *     openAiChatModel, aimClient);
 *
 * // Use with tools
 * AIMToolExecutor executor = AIMLangChain4jConfig.createToolExecutor(aimClient);
 * executor.register("searchDatabase", tool, "database:search");
 * }</pre>
 *
 * <h2>AI Services Integration:</h2>
 * <pre>{@code
 * interface Assistant {
 *     String chat(String userMessage);
 * }
 *
 * AIMAgentCallback callback = new AIMAgentCallback(aimClient);
 *
 * Assistant assistant = AiServices.builder(Assistant.class)
 *     .chatLanguageModel(model)
 *     .tools(tools)
 *     .chatMemory(memoryWithAudit)
 *     .build();
 * }</pre>
 */
public class AIMLangChain4jConfig {

    private static final Logger log = LoggerFactory.getLogger(AIMLangChain4jConfig.class);

    private AIMLangChain4jConfig() {}

    /**
     * Create a chat model listener for security logging.
     *
     * @param client AIMClient to use for logging
     * @return Chat model listener
     */
    public static AIMChatModelListener createListener(AIMClient client) {
        return new AIMChatModelListener(client);
    }

    /**
     * Create a chat model listener with custom configuration.
     *
     * @param client AIMClient to use
     * @param config Listener configuration
     * @return Configured listener
     */
    public static AIMChatModelListener createListener(
            AIMClient client, AIMChatModelListener.Config config) {
        return new AIMChatModelListener(client, config);
    }

    /**
     * Create a tool executor with security wrapper.
     *
     * @param client AIMClient to use
     * @return Tool executor
     */
    public static AIMToolExecutor createToolExecutor(AIMClient client) {
        return new AIMToolExecutor(client);
    }

    /**
     * Create an agent callback handler.
     *
     * @param client AIMClient to use
     * @return Agent callback handler
     */
    public static AIMAgentCallback createAgentCallback(AIMClient client) {
        return new AIMAgentCallback(client);
    }

    /**
     * Builder for LangChain4j configuration.
     */
    public static Builder builder() {
        return new Builder();
    }

    public static class Builder {
        private String agentName = "langchain4j-agent";
        private List<String> capabilities;
        private AgentType agentType = AgentType.LANGCHAIN;
        private List<String> talksTo;
        private String description;
        private List<String> tags;
        private Map<String, Object> metadata;
        private Map<String, String> mcpCommands;

        // Listener config
        private boolean logPrompts = false;
        private boolean logResponses = false;
        private boolean logToolCalls = true;

        public Builder agentName(String agentName) {
            this.agentName = agentName;
            return this;
        }

        public Builder capabilities(List<String> capabilities) {
            this.capabilities = capabilities;
            return this;
        }

        public Builder agentType(AgentType agentType) {
            this.agentType = agentType;
            return this;
        }

        public Builder talksTo(List<String> talksTo) {
            this.talksTo = talksTo;
            return this;
        }

        public Builder description(String description) {
            this.description = description;
            return this;
        }

        public Builder tags(List<String> tags) {
            this.tags = tags;
            return this;
        }

        public Builder metadata(Map<String, Object> metadata) {
            this.metadata = metadata;
            return this;
        }

        public Builder mcpCommands(Map<String, String> mcpCommands) {
            this.mcpCommands = mcpCommands;
            return this;
        }

        public Builder logPrompts(boolean logPrompts) {
            this.logPrompts = logPrompts;
            return this;
        }

        public Builder logResponses(boolean logResponses) {
            this.logResponses = logResponses;
            return this;
        }

        public Builder logToolCalls(boolean logToolCalls) {
            this.logToolCalls = logToolCalls;
            return this;
        }

        public AIMClient buildClient() {
            log.info("Creating AIMClient for LangChain4j: {}", agentName);
            return AIMClient.secure(
                    agentName,
                    capabilities,
                    agentType,
                    talksTo,
                    description,
                    tags,
                    metadata,
                    mcpCommands
            );
        }

        public LangChain4jIntegration build() {
            AIMClient client = buildClient();
            AIMChatModelListener.Config listenerConfig = new AIMChatModelListener.Config()
                    .logPrompts(logPrompts)
                    .logResponses(logResponses);

            return new LangChain4jIntegration(client, listenerConfig);
        }
    }

    /**
     * Complete LangChain4j integration bundle.
     */
    public static class LangChain4jIntegration {
        private final AIMClient client;
        private final AIMChatModelListener listener;
        private final AIMToolExecutor toolExecutor;
        private final AIMAgentCallback agentCallback;

        public LangChain4jIntegration(AIMClient client, AIMChatModelListener.Config config) {
            this.client = client;
            this.listener = new AIMChatModelListener(client, config);
            this.toolExecutor = new AIMToolExecutor(client);
            this.agentCallback = new AIMAgentCallback(client);
        }

        public AIMClient getClient() { return client; }
        public AIMChatModelListener getListener() { return listener; }
        public AIMToolExecutor getToolExecutor() { return toolExecutor; }
        public AIMAgentCallback getAgentCallback() { return agentCallback; }
    }
}
