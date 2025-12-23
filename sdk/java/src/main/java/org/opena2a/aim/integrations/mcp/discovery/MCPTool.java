package org.opena2a.aim.integrations.mcp.discovery;

import java.util.Map;

/**
 * Represents a tool discovered from an MCP server.
 * Tools are the primary capabilities that MCP servers expose.
 */
public class MCPTool {
    private final String name;
    private final String description;
    private final Map<String, Object> inputSchema;

    public MCPTool(String name, String description, Map<String, Object> inputSchema) {
        this.name = name;
        this.description = description;
        this.inputSchema = inputSchema;
    }

    public String getName() {
        return name;
    }

    public String getDescription() {
        return description;
    }

    public Map<String, Object> getInputSchema() {
        return inputSchema;
    }

    @Override
    public String toString() {
        return name + (description != null ? " - " + description : "");
    }
}
