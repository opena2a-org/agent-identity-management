package org.opena2a.aim.integrations.mcp;

import com.fasterxml.jackson.core.type.TypeReference;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ObjectNode;
import okhttp3.*;
import org.bouncycastle.crypto.params.Ed25519PrivateKeyParameters;
import org.bouncycastle.crypto.signers.Ed25519Signer;
import org.opena2a.aim.client.AIMClient;
import org.opena2a.aim.exceptions.AIMException;
import org.opena2a.aim.json.SdkObjectMappers;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.io.IOException;
import java.time.Instant;
import java.util.*;
import java.util.concurrent.TimeUnit;

/**
 * MCP (Model Context Protocol) Integration for AIM.
 *
 * <p>This class provides methods for registering and managing MCP servers
 * with the AIM backend. MCP servers enable AI agents to interact with
 * external tools, resources, and prompts in a standardized way. Each request
 * carries the AIM client's access token.</p>
 *
 * <p>Example usage:</p>
 * <pre>{@code
 * AIMClient client = AIMClient.secure("my-agent");
 *
 * // Register an MCP server
 * MCPServerInfo server = MCPIntegration.registerServer(
 *     client,
 *     "filesystem-mcp",
 *     "http://localhost:3000",
 *     publicKey,
 *     Arrays.asList("read_file", "write_file", "list_directory")
 * );
 *
 * // Record tool usage
 * MCPIntegration.recordToolUsage(client, server.getId(), "read_file");
 * }</pre>
 */
public class MCPIntegration {

    private static final Logger logger = LoggerFactory.getLogger(MCPIntegration.class);

    /**
     * Private constructor to prevent instantiation.
     * This is a utility class with only static methods.
     */
    private MCPIntegration() {
        // Utility class
    }
    private static final MediaType JSON = MediaType.get("application/json; charset=utf-8");
    private static final ObjectMapper objectMapper = SdkObjectMappers.create();

    // The server accepts an attestation only with an Ed25519 signature made
    // with the agent's private key. This class cannot reach that key, so
    // attestServer() refuses before sending anything.
    private static final String ATTESTATION_UNSIGNABLE =
            "MCPIntegration.attestServer cannot sign an attestation: the signature "
            + "must be made with the agent's Ed25519 private key, which this class "
            + "cannot reach. No request was sent.";

    private static final OkHttpClient httpClient = new OkHttpClient.Builder()
            .connectTimeout(30, TimeUnit.SECONDS)
            .readTimeout(30, TimeUnit.SECONDS)
            .writeTimeout(30, TimeUnit.SECONDS)
            .build();

    /**
     * Register an MCP server with the AIM backend.
     *
     * @param client       AIMClient instance for authentication
     * @param serverName   Name of the MCP server
     * @param serverUrl    Base URL of the MCP server
     * @param publicKey    Ed25519 public key for cryptographic verification
     * @param capabilities List of server capabilities
     * @return MCPServerInfo with registration details
     */
    public static MCPServerInfo registerServer(
            AIMClient client,
            String serverName,
            String serverUrl,
            String publicKey,
            List<String> capabilities
    ) {
        return registerServer(client, serverName, serverUrl, publicKey, capabilities, null, "1.0.0");
    }

    /**
     * Register an MCP server with the AIM backend.
     *
     * @param client       AIMClient instance for authentication
     * @param serverName   Name of the MCP server
     * @param serverUrl    Base URL of the MCP server
     * @param publicKey    Ed25519 public key for cryptographic verification
     * @param capabilities List of server capabilities
     * @param description  Optional description of the MCP server
     * @param version      Server version (default: "1.0.0")
     * @return MCPServerInfo with registration details
     */
    public static MCPServerInfo registerServer(
            AIMClient client,
            String serverName,
            String serverUrl,
            String publicKey,
            List<String> capabilities,
            String description,
            String version
    ) {
        validateRegistrationParams(serverName, publicKey, capabilities);

        try {
            ObjectNode payload = objectMapper.createObjectNode();
            payload.put("name", serverName.trim());
            payload.put("description", description != null ? description.trim() : "MCP Server: " + serverName);
            payload.put("url", serverUrl.trim());
            payload.put("version", version);
            payload.put("publicKey", publicKey);
            var capArray = payload.putArray("capabilities");
            for (String cap : capabilities) {
                capArray.add(cap);
            }

            String response = post(client, "/api/v1/sdk-api/agents/" + client.getAgentId() + "/mcp-servers", payload.toString());
            JsonNode json = objectMapper.readTree(response);

            return MCPServerInfo.builder()
                    .id(json.has("id") ? json.get("id").asText() : null)
                    .name(json.has("name") ? json.get("name").asText() : serverName)
                    .url(json.has("url") ? json.get("url").asText() : serverUrl)
                    .status(json.has("status") ? json.get("status").asText() : "pending")
                    .trustScore(json.has("trustScore") ? json.get("trustScore").asDouble() : 50.0)
                    .capabilities(capabilities)
                    .publicKey(publicKey)
                    .version(version)
                    .createdAt(Instant.now())
                    .build();

        } catch (IOException e) {
            throw new AIMException("Failed to register MCP server: " + e.getMessage(), e);
        }
    }

    /**
     * List all MCP servers registered with AIM for the current organization.
     *
     * @param client AIMClient instance for authentication
     * @return List of MCP server information
     */
    public static List<MCPServerInfo> listServers(AIMClient client) {
        return listServers(client, 50, 0);
    }

    /**
     * List MCP servers with pagination.
     *
     * @param client AIMClient instance for authentication
     * @param limit  Maximum number of servers to return
     * @param offset Number of servers to skip
     * @return List of MCP server information
     */
    public static List<MCPServerInfo> listServers(AIMClient client, int limit, int offset) {
        try {
            String response = get(client, "/api/v1/sdk-api/agents/" + client.getAgentId() +
                    "/mcp-servers?limit=" + limit + "&offset=" + offset);
            JsonNode json = objectMapper.readTree(response);

            List<MCPServerInfo> servers = new ArrayList<>();
            JsonNode serversArray = json.isArray() ? json : json.get("servers");

            if (serversArray != null && serversArray.isArray()) {
                for (JsonNode serverNode : serversArray) {
                    servers.add(parseServerInfo(serverNode));
                }
            }

            return servers;
        } catch (IOException e) {
            throw new AIMException("Failed to list MCP servers: " + e.getMessage(), e);
        }
    }

    /**
     * Not available: throws {@link AIMException} before any request is sent.
     *
     * <p>The server accepts an attestation only with an Ed25519 signature made
     * with the agent's private key, and this class cannot reach that key.</p>
     *
     * @param client           AIMClient instance
     * @param serverId         UUID of the MCP server to attest
     * @param mcpUrl           URL/command of the MCP server
     * @param mcpName          Name of the MCP server
     * @param capabilitiesFound List of capabilities detected on the MCP server
     * @return never returns normally
     * @throws AIMException always, before any request is sent
     */
    public static AttestationResult attestServer(
            AIMClient client,
            String serverId,
            String mcpUrl,
            String mcpName,
            List<String> capabilitiesFound
    ) {
        return attestServer(client, serverId, mcpUrl, mcpName, capabilitiesFound, true, true, 0.0);
    }

    /**
     * Not available: throws {@link AIMException} before any request is sent.
     *
     * <p>The server accepts an attestation only with an Ed25519 signature made
     * with the agent's private key, and this class cannot reach that key.</p>
     *
     * @param client               AIMClient instance
     * @param serverId             UUID of the MCP server to attest
     * @param mcpUrl               URL/command of the MCP server
     * @param mcpName              Name of the MCP server
     * @param capabilitiesFound    List of capabilities detected on the MCP server
     * @param connectionSuccessful Whether connection to MCP was successful
     * @param healthCheckPassed    Whether health check passed
     * @param connectionLatencyMs  Connection latency in milliseconds
     * @return never returns normally
     * @throws AIMException always, before any request is sent
     */
    public static AttestationResult attestServer(
            AIMClient client,
            String serverId,
            String mcpUrl,
            String mcpName,
            List<String> capabilitiesFound,
            boolean connectionSuccessful,
            boolean healthCheckPassed,
            double connectionLatencyMs
    ) {
        throw new AIMException(ATTESTATION_UNSIGNABLE);
    }

    /**
     * Record MCP tool usage for supply chain analytics.
     *
     * @param client   AIMClient instance for authentication
     * @param serverId UUID of the MCP server being used
     * @param toolName Name of the tool being used
     * @return Map containing connection response
     */
    public static Map<String, Object> recordToolUsage(
            AIMClient client,
            String serverId,
            String toolName
    ) {
        return recordToolUsage(client, serverId, toolName, null, null);
    }

    /**
     * Record MCP tool usage for supply chain analytics.
     *
     * @param client   AIMClient instance for authentication
     * @param serverId UUID of the MCP server being used
     * @param toolName Name of the tool being used
     * @param mcpUrl   URL of the MCP server
     * @param mcpName  Name of the MCP server
     * @return Map containing connection response
     */
    public static Map<String, Object> recordToolUsage(
            AIMClient client,
            String serverId,
            String toolName,
            String mcpUrl,
            String mcpName
    ) {
        if (serverId == null || serverId.isBlank()) {
            throw new IllegalArgumentException("serverId cannot be empty");
        }

        if (toolName == null || toolName.isBlank()) {
            throw new IllegalArgumentException("toolName cannot be empty");
        }

        try {
            ObjectNode payload = objectMapper.createObjectNode();
            payload.put("mcp_server_id", serverId);
            payload.put("tool_name", toolName);
            if (mcpUrl != null) {
                payload.put("mcp_url", mcpUrl);
            }
            if (mcpName != null) {
                payload.put("mcp_name", mcpName);
            }
            payload.put("connection_type", "attested");

            String response = post(client, "/api/v1/sdk-api/agents/" + client.getAgentId() + "/mcp-connections", payload.toString());
            return objectMapper.readValue(response, new TypeReference<Map<String, Object>>() {});

        } catch (IOException e) {
            throw new AIMException("Failed to record MCP tool usage: " + e.getMessage(), e);
        }
    }

    /**
     * Verify an MCP action before execution.
     *
     * @param client   AIMClient instance for authentication
     * @param serverId UUID of the MCP server
     * @param toolName Name of the tool being used
     * @return true if the action is verified
     */
    public static boolean verifyAction(AIMClient client, String serverId, String toolName) {
        try {
            ObjectNode payload = objectMapper.createObjectNode();
            payload.put("mcpServerId", serverId);
            payload.put("toolName", toolName);
            payload.put("timestamp", Instant.now().toString());

            String response = post(client, "/api/v1/mcp-servers/" + serverId + "/verify-action", payload.toString());
            JsonNode json = objectMapper.readTree(response);

            return json.has("verified") && json.get("verified").asBoolean();
        } catch (Exception e) {
            logger.error("Failed to verify MCP action: {}", e.getMessage());
            return false;
        }
    }

    // Helper methods

    private static void validateRegistrationParams(String serverName, String publicKey, List<String> capabilities) {
        if (serverName == null || serverName.isBlank()) {
            throw new IllegalArgumentException("serverName cannot be empty");
        }
        if (publicKey == null || publicKey.length() < 32) {
            throw new IllegalArgumentException("publicKey must be a valid Ed25519 public key");
        }
        if (capabilities == null || capabilities.isEmpty()) {
            throw new IllegalArgumentException("capabilities list cannot be empty");
        }
    }

    private static MCPServerInfo parseServerInfo(JsonNode node) {
        List<String> capabilities = new ArrayList<>();
        if (node.has("capabilities") && node.get("capabilities").isArray()) {
            for (JsonNode cap : node.get("capabilities")) {
                capabilities.add(cap.asText());
            }
        }

        return MCPServerInfo.builder()
                .id(node.has("id") ? node.get("id").asText() : null)
                .name(node.has("name") ? node.get("name").asText() : null)
                .url(node.has("url") ? node.get("url").asText() : null)
                .description(node.has("description") ? node.get("description").asText() : null)
                .version(node.has("version") ? node.get("version").asText() : null)
                .publicKey(node.has("publicKey") ? node.get("publicKey").asText() : null)
                .capabilities(capabilities)
                .status(node.has("status") ? node.get("status").asText() : null)
                .trustScore(node.has("trustScore") ? node.get("trustScore").asDouble() : 0.0)
                .build();
    }

    private static String get(AIMClient client, String path) throws IOException {
        Request request = new Request.Builder()
                .url(client.getAimUrl() + path)
                .header("Authorization", "Bearer " + getAccessToken(client))
                .get()
                .build();

        try (Response response = httpClient.newCall(request).execute()) {
            if (!response.isSuccessful()) {
                throw new AIMException("Request failed: " + response.code(), "HTTP_ERROR", response.code());
            }
            return response.body().string();
        }
    }

    private static String post(AIMClient client, String path, String json) throws IOException {
        RequestBody body = RequestBody.create(json, JSON);

        Request request = new Request.Builder()
                .url(client.getAimUrl() + path)
                .header("Authorization", "Bearer " + getAccessToken(client))
                .post(body)
                .build();

        try (Response response = httpClient.newCall(request).execute()) {
            if (!response.isSuccessful()) {
                String errorBody = response.body() != null ? response.body().string() : "";
                throw new AIMException("Request failed: " + response.code() + " - " + errorBody,
                        "HTTP_ERROR", response.code());
            }
            return response.body().string();
        }
    }

    private static String getAccessToken(AIMClient client) {
        String token = client.getAccessToken();
        if (token == null || token.isBlank()) {
            throw new AIMException("The AIM client has no access token to send. No request was sent.");
        }
        return token;
    }
}
