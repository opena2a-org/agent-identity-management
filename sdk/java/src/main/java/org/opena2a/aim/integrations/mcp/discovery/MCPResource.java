package org.opena2a.aim.integrations.mcp.discovery;

/**
 * Represents a resource discovered from an MCP server.
 * Resources are data endpoints that can be read by clients.
 */
public class MCPResource {
    private final String uri;
    private final String name;
    private final String description;
    private final String mimeType;

    public MCPResource(String uri, String name, String description, String mimeType) {
        this.uri = uri;
        this.name = name;
        this.description = description;
        this.mimeType = mimeType;
    }

    public String getUri() {
        return uri;
    }

    public String getName() {
        return name;
    }

    public String getDescription() {
        return description;
    }

    public String getMimeType() {
        return mimeType;
    }

    @Override
    public String toString() {
        return uri + (name != null ? " (" + name + ")" : "");
    }
}
