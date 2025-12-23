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

    /**
     * Agent name for registration with AIM server.
     */
    private String agentName = "spring-ai-agent";

    /**
     * List of capabilities this agent has.
     */
    private List<String> capabilities = new ArrayList<>();

    /**
     * Type of agent (CUSTOM, LANGCHAIN, etc.).
     */
    private AgentType agentType = AgentType.CUSTOM;

    /**
     * List of other agents this agent can communicate with.
     */
    private List<String> talksTo = new ArrayList<>();

    /**
     * Human-readable description of the agent.
     */
    private String description;

    /**
     * Tags for categorization and search.
     */
    private List<String> tags = new ArrayList<>();

    /**
     * Additional metadata for the agent.
     */
    private Map<String, Object> metadata = new HashMap<>();

    /**
     * MCP server commands (server name -> command).
     */
    private Map<String, String> mcpCommands = new HashMap<>();

    /**
     * Whether to automatically register with AIM server on startup.
     */
    private boolean autoRegister = true;

    /**
     * Security-specific configuration.
     */
    private SecurityConfig securityConfig = new SecurityConfig();

    // Getters and setters

    public String getAgentName() {
        return agentName;
    }

    public void setAgentName(String agentName) {
        this.agentName = agentName;
    }

    public List<String> getCapabilities() {
        return capabilities;
    }

    public void setCapabilities(List<String> capabilities) {
        this.capabilities = capabilities;
    }

    public AgentType getAgentType() {
        return agentType;
    }

    public void setAgentType(AgentType agentType) {
        this.agentType = agentType;
    }

    public List<String> getTalksTo() {
        return talksTo;
    }

    public void setTalksTo(List<String> talksTo) {
        this.talksTo = talksTo;
    }

    public String getDescription() {
        return description;
    }

    public void setDescription(String description) {
        this.description = description;
    }

    public List<String> getTags() {
        return tags;
    }

    public void setTags(List<String> tags) {
        this.tags = tags;
    }

    public Map<String, Object> getMetadata() {
        return metadata;
    }

    public void setMetadata(Map<String, Object> metadata) {
        this.metadata = metadata;
    }

    public Map<String, String> getMcpCommands() {
        return mcpCommands;
    }

    public void setMcpCommands(Map<String, String> mcpCommands) {
        this.mcpCommands = mcpCommands;
    }

    public boolean isAutoRegister() {
        return autoRegister;
    }

    public void setAutoRegister(boolean autoRegister) {
        this.autoRegister = autoRegister;
    }

    public SecurityConfig getSecurityConfig() {
        return securityConfig;
    }

    public void setSecurityConfig(SecurityConfig securityConfig) {
        this.securityConfig = securityConfig;
    }

    /**
     * Security-specific configuration options.
     */
    public static class SecurityConfig {
        /**
         * Whether to log prompts (may contain PII).
         */
        private boolean logPrompts = false;

        /**
         * Whether to log model responses (may contain PII).
         */
        private boolean logResponses = false;

        /**
         * Whether to perform risk checks on function calls.
         */
        private boolean riskCheckEnabled = true;

        /**
         * Minimum risk level that requires approval.
         */
        private String approvalThreshold = "HIGH";

        /**
         * Whether to block high-risk actions without approval.
         */
        private boolean blockWithoutApproval = false;

        public boolean isLogPrompts() {
            return logPrompts;
        }

        public void setLogPrompts(boolean logPrompts) {
            this.logPrompts = logPrompts;
        }

        public boolean isLogResponses() {
            return logResponses;
        }

        public void setLogResponses(boolean logResponses) {
            this.logResponses = logResponses;
        }

        public boolean isRiskCheckEnabled() {
            return riskCheckEnabled;
        }

        public void setRiskCheckEnabled(boolean riskCheckEnabled) {
            this.riskCheckEnabled = riskCheckEnabled;
        }

        public String getApprovalThreshold() {
            return approvalThreshold;
        }

        public void setApprovalThreshold(String approvalThreshold) {
            this.approvalThreshold = approvalThreshold;
        }

        public boolean isBlockWithoutApproval() {
            return blockWithoutApproval;
        }

        public void setBlockWithoutApproval(boolean blockWithoutApproval) {
            this.blockWithoutApproval = blockWithoutApproval;
        }
    }
}
