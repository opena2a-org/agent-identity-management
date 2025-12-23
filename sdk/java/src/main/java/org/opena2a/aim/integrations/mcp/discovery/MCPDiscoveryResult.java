package org.opena2a.aim.integrations.mcp.discovery;

import java.util.ArrayList;
import java.util.List;
import java.util.stream.Collectors;
import java.util.stream.Stream;

/**
 * Result of MCP server capability discovery.
 * Contains all tools, resources, and prompts discovered from an MCP server.
 */
public class MCPDiscoveryResult {
    private final String serverName;
    private final String serverVersion;
    private final String protocolVersion;
    private final List<MCPTool> tools;
    private final List<MCPResource> resources;
    private final List<MCPPrompt> prompts;
    private final long connectionLatencyMs;
    private final long discoveryTimeMs;
    private final String error;

    private MCPDiscoveryResult(Builder builder) {
        this.serverName = builder.serverName;
        this.serverVersion = builder.serverVersion;
        this.protocolVersion = builder.protocolVersion;
        this.tools = builder.tools != null ? builder.tools : new ArrayList<>();
        this.resources = builder.resources != null ? builder.resources : new ArrayList<>();
        this.prompts = builder.prompts != null ? builder.prompts : new ArrayList<>();
        this.connectionLatencyMs = builder.connectionLatencyMs;
        this.discoveryTimeMs = builder.discoveryTimeMs;
        this.error = builder.error;
    }

    public String getServerName() {
        return serverName;
    }

    public String getServerVersion() {
        return serverVersion;
    }

    public String getProtocolVersion() {
        return protocolVersion;
    }

    public List<MCPTool> getTools() {
        return tools;
    }

    public List<MCPResource> getResources() {
        return resources;
    }

    public List<MCPPrompt> getPrompts() {
        return prompts;
    }

    public long getConnectionLatencyMs() {
        return connectionLatencyMs;
    }

    public long getDiscoveryTimeMs() {
        return discoveryTimeMs;
    }

    public String getError() {
        return error;
    }

    public boolean hasError() {
        return error != null && !error.isEmpty();
    }

    public boolean isSuccess() {
        return !hasError();
    }

    /**
     * Get list of tool names for attestation.
     */
    public List<String> getToolNames() {
        return tools.stream().map(MCPTool::getName).collect(Collectors.toList());
    }

    /**
     * Get list of resource URIs for attestation.
     */
    public List<String> getResourceUris() {
        return resources.stream().map(MCPResource::getUri).collect(Collectors.toList());
    }

    /**
     * Get list of prompt names for attestation.
     */
    public List<String> getPromptNames() {
        return prompts.stream().map(MCPPrompt::getName).collect(Collectors.toList());
    }

    /**
     * Get all capability names (tools + resources + prompts) for attestation.
     */
    public List<String> getAllCapabilityNames() {
        return Stream.concat(
            Stream.concat(getToolNames().stream(), getResourceUris().stream()),
            getPromptNames().stream()
        ).collect(Collectors.toList());
    }

    /**
     * Get total count of all capabilities.
     */
    public int getTotalCapabilities() {
        return tools.size() + resources.size() + prompts.size();
    }

    public static Builder builder() {
        return new Builder();
    }

    public static class Builder {
        private String serverName = "unknown";
        private String serverVersion = "unknown";
        private String protocolVersion = "unknown";
        private List<MCPTool> tools = new ArrayList<>();
        private List<MCPResource> resources = new ArrayList<>();
        private List<MCPPrompt> prompts = new ArrayList<>();
        private long connectionLatencyMs = 0;
        private long discoveryTimeMs = 0;
        private String error;

        public Builder serverName(String serverName) {
            this.serverName = serverName;
            return this;
        }

        public Builder serverVersion(String serverVersion) {
            this.serverVersion = serverVersion;
            return this;
        }

        public Builder protocolVersion(String protocolVersion) {
            this.protocolVersion = protocolVersion;
            return this;
        }

        public Builder tools(List<MCPTool> tools) {
            this.tools = tools;
            return this;
        }

        public Builder addTool(MCPTool tool) {
            this.tools.add(tool);
            return this;
        }

        public Builder resources(List<MCPResource> resources) {
            this.resources = resources;
            return this;
        }

        public Builder addResource(MCPResource resource) {
            this.resources.add(resource);
            return this;
        }

        public Builder prompts(List<MCPPrompt> prompts) {
            this.prompts = prompts;
            return this;
        }

        public Builder addPrompt(MCPPrompt prompt) {
            this.prompts.add(prompt);
            return this;
        }

        public Builder connectionLatencyMs(long connectionLatencyMs) {
            this.connectionLatencyMs = connectionLatencyMs;
            return this;
        }

        public Builder discoveryTimeMs(long discoveryTimeMs) {
            this.discoveryTimeMs = discoveryTimeMs;
            return this;
        }

        public Builder error(String error) {
            this.error = error;
            return this;
        }

        public MCPDiscoveryResult build() {
            return new MCPDiscoveryResult(this);
        }
    }
}
