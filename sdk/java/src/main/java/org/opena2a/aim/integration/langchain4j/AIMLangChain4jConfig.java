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
     *
     * @return a new Builder instance
     */
    public static Builder builder() {
        return new Builder();
    }

    /**
     * Builder for LangChain4j integration configuration.
     */
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

        /** Creates a new Builder with default settings. */
        public Builder() {}

        /**
         * Sets the agent name.
         *
         * @param agentName the agent name
         * @return this builder
         */
        public Builder agentName(String agentName) {
            this.agentName = agentName;
            return this;
        }

        /**
         * Sets the capabilities.
         *
         * @param capabilities the capability list
         * @return this builder
         */
        public Builder capabilities(List<String> capabilities) {
            this.capabilities = capabilities;
            return this;
        }

        /**
         * Sets the agent type.
         *
         * @param agentType the agent type
         * @return this builder
         */
        public Builder agentType(AgentType agentType) {
            this.agentType = agentType;
            return this;
        }

        /**
         * Sets the MCP servers.
         *
         * @param talksTo the MCP server list
         * @return this builder
         */
        public Builder talksTo(List<String> talksTo) {
            this.talksTo = talksTo;
            return this;
        }

        /**
         * Sets the description.
         *
         * @param description the agent description
         * @return this builder
         */
        public Builder description(String description) {
            this.description = description;
            return this;
        }

        /**
         * Sets the tags.
         *
         * @param tags the tag list
         * @return this builder
         */
        public Builder tags(List<String> tags) {
            this.tags = tags;
            return this;
        }

        /**
         * Sets the metadata.
         *
         * @param metadata the metadata map
         * @return this builder
         */
        public Builder metadata(Map<String, Object> metadata) {
            this.metadata = metadata;
            return this;
        }

        /**
         * Sets the MCP commands.
         *
         * @param mcpCommands the MCP command map
         * @return this builder
         */
        public Builder mcpCommands(Map<String, String> mcpCommands) {
            this.mcpCommands = mcpCommands;
            return this;
        }

        /**
         * Sets whether to log prompts.
         *
         * @param logPrompts true to log prompts
         * @return this builder
         */
        public Builder logPrompts(boolean logPrompts) {
            this.logPrompts = logPrompts;
            return this;
        }

        /**
         * Sets whether to log responses.
         *
         * @param logResponses true to log responses
         * @return this builder
         */
        public Builder logResponses(boolean logResponses) {
            this.logResponses = logResponses;
            return this;
        }

        /**
         * Sets whether to log tool calls.
         *
         * @param logToolCalls true to log tool calls
         * @return this builder
         */
        public Builder logToolCalls(boolean logToolCalls) {
            this.logToolCalls = logToolCalls;
            return this;
        }

        /**
         * Builds only the AIM client.
         *
         * @return the configured AIMClient
         */
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

        /**
         * Builds the complete integration.
         *
         * @return the LangChain4j integration bundle
         */
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

        /**
         * Creates a new integration bundle.
         *
         * @param client the AIM client
         * @param config the listener configuration
         */
        public LangChain4jIntegration(AIMClient client, AIMChatModelListener.Config config) {
            this.client = client;
            this.listener = new AIMChatModelListener(client, config);
            this.toolExecutor = new AIMToolExecutor(client);
            this.agentCallback = new AIMAgentCallback(client);
        }

        /**
         * Gets the AIM client.
         *
         * @return the client
         */
        public AIMClient getClient() { return client; }

        /**
         * Gets the chat model listener.
         *
         * @return the listener
         */
        public AIMChatModelListener getListener() { return listener; }

        /**
         * Gets the tool executor.
         *
         * @return the tool executor
         */
        public AIMToolExecutor getToolExecutor() { return toolExecutor; }

        /**
         * Gets the agent callback.
         *
         * @return the agent callback
         */
        public AIMAgentCallback getAgentCallback() { return agentCallback; }
    }
}
