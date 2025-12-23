package org.opena2a.aim.integration.spring;

import org.opena2a.aim.client.AgentType;

import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;

/**
 * Configuration properties for AIM Spring AI integration.
 *
 * <p>When used with Spring Boot, these can be configured via application.properties
 * or application.yml with the prefix "aim".</p>
 *
 * <h2>Example application.yml:</h2>
 * <pre>
 * aim:
 *   agent-name: my-spring-ai-agent
 *   capabilities:
 *     - chat:complete
 *     - embedding:create
 *     - function:call
 *   agent-type: CUSTOM
 *   description: Spring AI powered customer service agent
 *   tags:
 *     - spring-ai
 *     - customer-service
 *   auto-register: true
 *   security:
 *     log-prompts: false
 *     log-responses: false
 *     risk-check-enabled: true
 *   mcp-commands:
 *     github: npx -y @modelcontextprotocol/server-github
 *     filesystem: npx -y @modelcontextprotocol/server-filesystem /data
 * </pre>
 */
public class AIMSpringAIProperties {

    private String agentName = "spring-ai-agent";
    private List<String> capabilities = new ArrayList<>();
    private AgentType agentType = AgentType.CUSTOM;
    private List<String> talksTo = new ArrayList<>();
    private String description;
    private List<String> tags = new ArrayList<>();
    private Map<String, Object> metadata = new HashMap<>();
    private Map<String, String> mcpCommands = new HashMap<>();
    private boolean autoRegister = true;
    private SecurityConfig securityConfig = new SecurityConfig();

    /** Creates default properties. */
    public AIMSpringAIProperties() {}

    /**
     * Gets the agent name.
     *
     * @return the agent name
     */
    public String getAgentName() { return agentName; }

    /**
     * Sets the agent name.
     *
     * @param agentName the agent name
     */
    public void setAgentName(String agentName) { this.agentName = agentName; }

    /**
     * Gets the capabilities.
     *
     * @return the capability list
     */
    public List<String> getCapabilities() { return capabilities; }

    /**
     * Sets the capabilities.
     *
     * @param capabilities the capability list
     */
    public void setCapabilities(List<String> capabilities) { this.capabilities = capabilities; }

    /**
     * Gets the agent type.
     *
     * @return the agent type
     */
    public AgentType getAgentType() { return agentType; }

    /**
     * Sets the agent type.
     *
     * @param agentType the agent type
     */
    public void setAgentType(AgentType agentType) { this.agentType = agentType; }

    /**
     * Gets the MCP servers.
     *
     * @return the MCP server list
     */
    public List<String> getTalksTo() { return talksTo; }

    /**
     * Sets the MCP servers.
     *
     * @param talksTo the MCP server list
     */
    public void setTalksTo(List<String> talksTo) { this.talksTo = talksTo; }

    /**
     * Gets the description.
     *
     * @return the description
     */
    public String getDescription() { return description; }

    /**
     * Sets the description.
     *
     * @param description the description
     */
    public void setDescription(String description) { this.description = description; }

    /**
     * Gets the tags.
     *
     * @return the tag list
     */
    public List<String> getTags() { return tags; }

    /**
     * Sets the tags.
     *
     * @param tags the tag list
     */
    public void setTags(List<String> tags) { this.tags = tags; }

    /**
     * Gets the metadata.
     *
     * @return the metadata map
     */
    public Map<String, Object> getMetadata() { return metadata; }

    /**
     * Sets the metadata.
     *
     * @param metadata the metadata map
     */
    public void setMetadata(Map<String, Object> metadata) { this.metadata = metadata; }

    /**
     * Gets the MCP commands.
     *
     * @return the MCP command map
     */
    public Map<String, String> getMcpCommands() { return mcpCommands; }

    /**
     * Sets the MCP commands.
     *
     * @param mcpCommands the MCP command map
     */
    public void setMcpCommands(Map<String, String> mcpCommands) { this.mcpCommands = mcpCommands; }

    /**
     * Checks if auto-register is enabled.
     *
     * @return true if enabled
     */
    public boolean isAutoRegister() { return autoRegister; }

    /**
     * Sets auto-register.
     *
     * @param autoRegister true to enable
     */
    public void setAutoRegister(boolean autoRegister) { this.autoRegister = autoRegister; }

    /**
     * Gets the security config.
     *
     * @return the security config
     */
    public SecurityConfig getSecurityConfig() { return securityConfig; }

    /**
     * Sets the security config.
     *
     * @param securityConfig the security config
     */
    public void setSecurityConfig(SecurityConfig securityConfig) { this.securityConfig = securityConfig; }

    /**
     * Security-specific configuration options.
     */
    public static class SecurityConfig {
        private boolean logPrompts = false;
        private boolean logResponses = false;
        private boolean riskCheckEnabled = true;
        private String approvalThreshold = "HIGH";
        private boolean blockWithoutApproval = false;

        /** Creates default security config. */
        public SecurityConfig() {}

        /**
         * Checks if prompts are logged.
         *
         * @return true if enabled
         */
        public boolean isLogPrompts() { return logPrompts; }

        /**
         * Sets prompt logging.
         *
         * @param logPrompts true to enable
         */
        public void setLogPrompts(boolean logPrompts) { this.logPrompts = logPrompts; }

        /**
         * Checks if responses are logged.
         *
         * @return true if enabled
         */
        public boolean isLogResponses() { return logResponses; }

        /**
         * Sets response logging.
         *
         * @param logResponses true to enable
         */
        public void setLogResponses(boolean logResponses) { this.logResponses = logResponses; }

        /**
         * Checks if risk checking is enabled.
         *
         * @return true if enabled
         */
        public boolean isRiskCheckEnabled() { return riskCheckEnabled; }

        /**
         * Sets risk checking.
         *
         * @param riskCheckEnabled true to enable
         */
        public void setRiskCheckEnabled(boolean riskCheckEnabled) { this.riskCheckEnabled = riskCheckEnabled; }

        /**
         * Gets the approval threshold.
         *
         * @return the threshold
         */
        public String getApprovalThreshold() { return approvalThreshold; }

        /**
         * Sets the approval threshold.
         *
         * @param approvalThreshold the threshold
         */
        public void setApprovalThreshold(String approvalThreshold) { this.approvalThreshold = approvalThreshold; }

        /**
         * Checks if blocking without approval is enabled.
         *
         * @return true if enabled
         */
        public boolean isBlockWithoutApproval() { return blockWithoutApproval; }

        /**
         * Sets blocking without approval.
         *
         * @param blockWithoutApproval true to enable
         */
        public void setBlockWithoutApproval(boolean blockWithoutApproval) { this.blockWithoutApproval = blockWithoutApproval; }
    }
}
