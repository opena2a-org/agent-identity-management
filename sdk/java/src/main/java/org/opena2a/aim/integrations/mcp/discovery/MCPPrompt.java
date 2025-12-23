package org.opena2a.aim.integrations.mcp.discovery;

import java.util.List;
import java.util.Map;

/**
 * Represents a prompt template discovered from an MCP server.
 * Prompts are reusable templates that can be invoked with arguments.
 */
public class MCPPrompt {
    private final String name;
    private final String description;
    private final List<Map<String, Object>> arguments;

    public MCPPrompt(String name, String description, List<Map<String, Object>> arguments) {
        this.name = name;
        this.description = description;
        this.arguments = arguments;
    }

    public String getName() {
        return name;
    }

    public String getDescription() {
        return description;
    }

    public List<Map<String, Object>> getArguments() {
        return arguments;
    }

    @Override
    public String toString() {
        return name + (description != null ? " - " + description : "");
    }
}
