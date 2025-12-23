package org.opena2a.aim.integrations.mcp.discovery;

import com.fasterxml.jackson.core.type.TypeReference;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ObjectNode;

import java.io.*;
import java.util.*;
import java.util.concurrent.*;
import java.util.concurrent.atomic.AtomicInteger;

/**
 * MCP (Model Context Protocol) Client for Java.
 *
 * Implements JSON-RPC 2.0 over stdio to communicate with MCP servers.
 * This allows discovery of tools, resources, and prompts from any MCP server.
 *
 * Usage:
 *   MCPDiscoveryResult result = MCPClient.discover("npx -y @modelcontextprotocol/server-github");
 *   System.out.println("Found " + result.getTools().size() + " tools");
 */
public class MCPClient implements AutoCloseable {

    private static final ObjectMapper objectMapper = new ObjectMapper();
    private static final String MCP_PROTOCOL_VERSION = "2024-11-05";
    private static final int DEFAULT_TIMEOUT_SECONDS = 30;

    private final Process process;
    private final BufferedWriter stdin;
    private final BufferedReader stdout;
    private final BufferedReader stderr;
    private final AtomicInteger requestId = new AtomicInteger(1);
    private final Map<Integer, CompletableFuture<JsonNode>> pendingRequests = new ConcurrentHashMap<>();
    private final ExecutorService readerExecutor;
    private volatile boolean closed = false;
    private String serverName = "unknown";
    private String serverVersion = "unknown";
    private String protocolVersion = "unknown";

    /**
     * Create a new MCP client connected to the given server command.
     *
     * @param command The MCP server command (e.g., "npx -y @modelcontextprotocol/server-github")
     * @throws IOException If the process cannot be started
     */
    public MCPClient(String command) throws IOException {
        this(parseCommand(command));
    }

    /**
     * Create a new MCP client with explicit command and arguments.
     *
     * @param commandParts The command and its arguments
     * @throws IOException If the process cannot be started
     */
    public MCPClient(List<String> commandParts) throws IOException {
        ProcessBuilder pb = new ProcessBuilder(commandParts);
        pb.environment().putAll(getQuietEnv());
        pb.redirectErrorStream(false);

        this.process = pb.start();
        this.stdin = new BufferedWriter(new OutputStreamWriter(process.getOutputStream()));
        this.stdout = new BufferedReader(new InputStreamReader(process.getInputStream()));
        this.stderr = new BufferedReader(new InputStreamReader(process.getErrorStream()));

        // Start reader thread to process incoming messages
        this.readerExecutor = Executors.newSingleThreadExecutor(r -> {
            Thread t = new Thread(r, "mcp-reader");
            t.setDaemon(true);
            return t;
        });
        readerExecutor.submit(this::readLoop);
    }

    /**
     * Discover all capabilities from an MCP server.
     * This is the main entry point for capability discovery.
     *
     * @param mcpCommand The MCP server command (e.g., "npx -y @modelcontextprotocol/server-github")
     * @return MCPDiscoveryResult containing all discovered tools, resources, and prompts
     */
    public static MCPDiscoveryResult discover(String mcpCommand) {
        return discover(mcpCommand, DEFAULT_TIMEOUT_SECONDS);
    }

    /**
     * Discover all capabilities from an MCP server with custom timeout.
     *
     * @param mcpCommand The MCP server command
     * @param timeoutSeconds Maximum time to wait for discovery
     * @return MCPDiscoveryResult containing all discovered tools, resources, and prompts
     */
    public static MCPDiscoveryResult discover(String mcpCommand, int timeoutSeconds) {
        long startTime = System.currentTimeMillis();

        try (MCPClient client = new MCPClient(mcpCommand)) {
            long connectionTime = System.currentTimeMillis();
            long connectionLatencyMs = connectionTime - startTime;

            // Initialize the connection
            client.initialize(timeoutSeconds);

            // Build result
            MCPDiscoveryResult.Builder resultBuilder = MCPDiscoveryResult.builder()
                .serverName(client.serverName)
                .serverVersion(client.serverVersion)
                .protocolVersion(client.protocolVersion)
                .connectionLatencyMs(connectionLatencyMs);

            // Discover tools
            try {
                List<MCPTool> tools = client.listTools(timeoutSeconds);
                resultBuilder.tools(tools);
            } catch (Exception e) {
                // Server may not support tools - continue
            }

            // Discover resources
            try {
                List<MCPResource> resources = client.listResources(timeoutSeconds);
                resultBuilder.resources(resources);
            } catch (Exception e) {
                // Server may not support resources - continue
            }

            // Discover prompts
            try {
                List<MCPPrompt> prompts = client.listPrompts(timeoutSeconds);
                resultBuilder.prompts(prompts);
            } catch (Exception e) {
                // Server may not support prompts - continue
            }

            long endTime = System.currentTimeMillis();
            resultBuilder.discoveryTimeMs(endTime - startTime);

            return resultBuilder.build();

        } catch (Exception e) {
            long endTime = System.currentTimeMillis();
            return MCPDiscoveryResult.builder()
                .discoveryTimeMs(endTime - startTime)
                .error(e.getMessage())
                .build();
        }
    }

    /**
     * Initialize the MCP connection.
     * This must be called before any other operations.
     */
    public void initialize(int timeoutSeconds) throws Exception {
        ObjectNode params = objectMapper.createObjectNode();
        params.put("protocolVersion", MCP_PROTOCOL_VERSION);

        ObjectNode clientInfo = objectMapper.createObjectNode();
        clientInfo.put("name", "aim-java-sdk");
        clientInfo.put("version", "1.0.0");
        params.set("clientInfo", clientInfo);

        ObjectNode capabilities = objectMapper.createObjectNode();
        params.set("capabilities", capabilities);

        JsonNode response = sendRequest("initialize", params, timeoutSeconds);

        // Extract server info
        if (response.has("serverInfo")) {
            JsonNode serverInfo = response.get("serverInfo");
            if (serverInfo.has("name")) {
                this.serverName = serverInfo.get("name").asText();
            }
            if (serverInfo.has("version")) {
                this.serverVersion = serverInfo.get("version").asText();
            }
        }
        if (response.has("protocolVersion")) {
            this.protocolVersion = response.get("protocolVersion").asText();
        }

        // Send initialized notification
        sendNotification("notifications/initialized", objectMapper.createObjectNode());
    }

    /**
     * List all tools available on the MCP server.
     */
    public List<MCPTool> listTools(int timeoutSeconds) throws Exception {
        JsonNode response = sendRequest("tools/list", objectMapper.createObjectNode(), timeoutSeconds);

        List<MCPTool> tools = new ArrayList<>();
        if (response.has("tools") && response.get("tools").isArray()) {
            for (JsonNode toolNode : response.get("tools")) {
                String name = toolNode.has("name") ? toolNode.get("name").asText() : "unknown";
                String description = toolNode.has("description") ? toolNode.get("description").asText() : null;
                Map<String, Object> inputSchema = null;
                if (toolNode.has("inputSchema")) {
                    inputSchema = objectMapper.convertValue(toolNode.get("inputSchema"),
                        new TypeReference<Map<String, Object>>() {});
                }
                tools.add(new MCPTool(name, description, inputSchema));
            }
        }
        return tools;
    }

    /**
     * List all resources available on the MCP server.
     */
    public List<MCPResource> listResources(int timeoutSeconds) throws Exception {
        JsonNode response = sendRequest("resources/list", objectMapper.createObjectNode(), timeoutSeconds);

        List<MCPResource> resources = new ArrayList<>();
        if (response.has("resources") && response.get("resources").isArray()) {
            for (JsonNode resourceNode : response.get("resources")) {
                String uri = resourceNode.has("uri") ? resourceNode.get("uri").asText() : "";
                String name = resourceNode.has("name") ? resourceNode.get("name").asText() : "";
                String description = resourceNode.has("description") ? resourceNode.get("description").asText() : null;
                String mimeType = resourceNode.has("mimeType") ? resourceNode.get("mimeType").asText() : null;
                resources.add(new MCPResource(uri, name, description, mimeType));
            }
        }
        return resources;
    }

    /**
     * List all prompts available on the MCP server.
     */
    public List<MCPPrompt> listPrompts(int timeoutSeconds) throws Exception {
        JsonNode response = sendRequest("prompts/list", objectMapper.createObjectNode(), timeoutSeconds);

        List<MCPPrompt> prompts = new ArrayList<>();
        if (response.has("prompts") && response.get("prompts").isArray()) {
            for (JsonNode promptNode : response.get("prompts")) {
                String name = promptNode.has("name") ? promptNode.get("name").asText() : "unknown";
                String description = promptNode.has("description") ? promptNode.get("description").asText() : null;
                List<Map<String, Object>> arguments = null;
                if (promptNode.has("arguments") && promptNode.get("arguments").isArray()) {
                    arguments = objectMapper.convertValue(promptNode.get("arguments"),
                        new TypeReference<List<Map<String, Object>>>() {});
                }
                prompts.add(new MCPPrompt(name, description, arguments));
            }
        }
        return prompts;
    }

    /**
     * Send a JSON-RPC request and wait for response.
     */
    private JsonNode sendRequest(String method, ObjectNode params, int timeoutSeconds) throws Exception {
        int id = requestId.getAndIncrement();

        ObjectNode request = objectMapper.createObjectNode();
        request.put("jsonrpc", "2.0");
        request.put("id", id);
        request.put("method", method);
        request.set("params", params);

        CompletableFuture<JsonNode> future = new CompletableFuture<>();
        pendingRequests.put(id, future);

        try {
            String json = objectMapper.writeValueAsString(request);
            synchronized (stdin) {
                stdin.write(json);
                stdin.newLine();
                stdin.flush();
            }

            return future.get(timeoutSeconds, TimeUnit.SECONDS);
        } catch (TimeoutException e) {
            throw new Exception("Request timed out: " + method);
        } finally {
            pendingRequests.remove(id);
        }
    }

    /**
     * Send a JSON-RPC notification (no response expected).
     */
    private void sendNotification(String method, ObjectNode params) throws Exception {
        ObjectNode notification = objectMapper.createObjectNode();
        notification.put("jsonrpc", "2.0");
        notification.put("method", method);
        notification.set("params", params);

        String json = objectMapper.writeValueAsString(notification);
        synchronized (stdin) {
            stdin.write(json);
            stdin.newLine();
            stdin.flush();
        }
    }

    /**
     * Background thread that reads responses from the MCP server.
     */
    private void readLoop() {
        try {
            String line;
            while (!closed && (line = stdout.readLine()) != null) {
                if (line.trim().isEmpty()) continue;

                try {
                    JsonNode message = objectMapper.readTree(line);

                    // Handle response (has "id" field)
                    if (message.has("id") && !message.get("id").isNull()) {
                        int id = message.get("id").asInt();
                        CompletableFuture<JsonNode> future = pendingRequests.get(id);
                        if (future != null) {
                            if (message.has("error")) {
                                JsonNode error = message.get("error");
                                String errorMsg = error.has("message") ? error.get("message").asText() : "Unknown error";
                                future.completeExceptionally(new Exception(errorMsg));
                            } else if (message.has("result")) {
                                future.complete(message.get("result"));
                            }
                        }
                    }
                    // Notifications and other messages are ignored

                } catch (Exception e) {
                    // Invalid JSON, skip
                }
            }
        } catch (IOException e) {
            if (!closed) {
                // Connection lost unexpectedly
                for (CompletableFuture<JsonNode> future : pendingRequests.values()) {
                    future.completeExceptionally(new Exception("Connection lost"));
                }
            }
        }
    }

    @Override
    public void close() {
        closed = true;
        readerExecutor.shutdownNow();

        try {
            stdin.close();
        } catch (IOException ignored) {}

        try {
            stdout.close();
        } catch (IOException ignored) {}

        try {
            stderr.close();
        } catch (IOException ignored) {}

        process.destroyForcibly();
        try {
            process.waitFor(5, TimeUnit.SECONDS);
        } catch (InterruptedException ignored) {}
    }

    /**
     * Parse an MCP command string into command parts.
     * Handles quoted arguments properly.
     */
    private static List<String> parseCommand(String command) {
        List<String> parts = new ArrayList<>();
        StringBuilder current = new StringBuilder();
        boolean inQuote = false;
        char quoteChar = 0;

        for (int i = 0; i < command.length(); i++) {
            char c = command.charAt(i);

            if (!inQuote && (c == '"' || c == '\'')) {
                inQuote = true;
                quoteChar = c;
            } else if (inQuote && c == quoteChar) {
                inQuote = false;
                quoteChar = 0;
            } else if (!inQuote && Character.isWhitespace(c)) {
                if (current.length() > 0) {
                    parts.add(current.toString());
                    current = new StringBuilder();
                }
            } else {
                current.append(c);
            }
        }

        if (current.length() > 0) {
            parts.add(current.toString());
        }

        return parts;
    }

    /**
     * Get environment variables that suppress noisy subprocess output.
     */
    private static Map<String, String> getQuietEnv() {
        Map<String, String> env = new HashMap<>(System.getenv());

        // Suppress npm output
        env.put("NPM_CONFIG_LOGLEVEL", "silent");
        env.put("NPM_CONFIG_PROGRESS", "false");
        env.put("NPM_CONFIG_FUND", "false");
        env.put("NPM_CONFIG_AUDIT", "false");
        env.put("NPM_CONFIG_UPDATE_NOTIFIER", "false");

        // Suppress npx output
        env.put("NPX_CONFIG_LOGLEVEL", "silent");

        // Suppress node warnings
        env.put("NODE_NO_WARNINGS", "1");

        return env;
    }
}
