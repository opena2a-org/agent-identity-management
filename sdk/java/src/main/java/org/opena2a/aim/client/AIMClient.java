package org.opena2a.aim.client;

import com.fasterxml.jackson.core.type.TypeReference;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ObjectNode;
import com.fasterxml.jackson.datatype.jsr310.JavaTimeModule;
import okhttp3.*;
import org.bouncycastle.crypto.params.Ed25519PrivateKeyParameters;
import org.bouncycastle.crypto.params.Ed25519PublicKeyParameters;
import org.bouncycastle.crypto.signers.Ed25519Signer;
import org.opena2a.aim.credentials.CredentialManager;
import org.opena2a.aim.exceptions.*;
import org.opena2a.aim.exceptions.ConfigurationException;
import org.opena2a.aim.integrations.mcp.discovery.MCPDiscoveryResult;
import org.opena2a.aim.integrations.mcp.discovery.MCPDiscoveryService;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.time.Instant;
import java.util.*;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.concurrent.locks.ReentrantLock;
import java.util.function.Supplier;

/**
 * AIM Client for Java - Secure your AI agents with cryptographic identity verification.
 *
 * <p>Quick Start:</p>
 * <pre>{@code
 * // One-line registration
 * AIMClient agent = AIMClient.secure("my-agent");
 *
 * // Verify actions before execution
 * VerificationResult result = agent.verifyCapability("db:read", "users_table");
 * if (result.isVerified()) {
 *     // Perform the action
 * }
 * }</pre>
 */
public class AIMClient implements AutoCloseable {

    private static final Logger logger = LoggerFactory.getLogger(AIMClient.class);
    private static final MediaType JSON = MediaType.get("application/json; charset=utf-8");

    // Retry configuration for enterprise reliability
    private static final int MAX_RETRIES = 3;
    private static final int INITIAL_BACKOFF_MS = 1000;
    private static final int MAX_BACKOFF_MS = 10000;

    private volatile String agentId;  // Can be updated after registration
    private final String agentName;
    private final String aimUrl;
    private final AgentType agentType;
    private final List<String> capabilities;
    private final List<String> talksTo; // MCP servers this agent communicates with
    private final Map<String, String> mcpCommands; // MCP server name to command mapping for discovery
    private final List<String> tags; // Agent tags for categorization
    private final String description;
    private final Map<String, Object> metadata;
    private final OkHttpClient httpClient;
    private final OkHttpClient authClient; // Separate client for auth endpoints (no authenticator)
    private final ObjectMapper objectMapper;

    // Thread-safe token management
    private volatile String accessToken;
    private volatile Instant tokenExpiry;
    private final ReentrantLock tokenLock = new ReentrantLock();
    private final AtomicBoolean isRefreshing = new AtomicBoolean(false);

    private byte[] privateKey;
    private byte[] publicKey;

    // OAuth credentials (from SDK download)
    private String clientId;
    private String clientSecret;
    private String organizationId;

    // SDK OAuth credentials (refresh token flow)
    private volatile String refreshToken;
    private String sdkTokenId;
    private Map<String, String> credentials;

    private AIMClient(Builder builder) {
        this.agentName = builder.agentName;
        this.aimUrl = builder.aimUrl;
        this.agentType = builder.agentType;
        this.capabilities = builder.capabilities;
        this.talksTo = builder.talksTo;
        this.mcpCommands = builder.mcpCommands != null ? builder.mcpCommands : new HashMap<>();
        this.tags = builder.tags;
        this.description = builder.description;
        this.metadata = builder.metadata;
        this.agentId = builder.agentId;
        this.clientId = builder.clientId;
        this.clientSecret = builder.clientSecret;
        this.organizationId = builder.organizationId;
        this.refreshToken = builder.refreshToken;
        this.sdkTokenId = builder.sdkTokenId;
        this.credentials = builder.credentials;

        this.objectMapper = new ObjectMapper();
        this.objectMapper.registerModule(new JavaTimeModule());

        // Build auth client for token refresh (no authenticator to avoid infinite loops)
        this.authClient = new OkHttpClient.Builder()
                .proxy(java.net.Proxy.NO_PROXY)
                .connectTimeout(30, TimeUnit.SECONDS)
                .readTimeout(30, TimeUnit.SECONDS)
                .writeTimeout(30, TimeUnit.SECONDS)
                .build();

        // Build HTTP client with automatic retry and auth refresh
        // Use NO_PROXY to bypass any system proxies for localhost/internal addresses
        this.httpClient = new OkHttpClient.Builder()
                .proxy(java.net.Proxy.NO_PROXY)
                .connectTimeout(30, TimeUnit.SECONDS)
                .readTimeout(30, TimeUnit.SECONDS)
                .writeTimeout(30, TimeUnit.SECONDS)
                .addInterceptor(this::addAuthHeader)
                .addInterceptor(this::retryInterceptor)
                .authenticator(this::handleAuthChallenge)
                .build();
    }

    /**
     * Interceptor to add auth header to all requests.
     */
    private Response addAuthHeader(Interceptor.Chain chain) throws IOException {
        Request original = chain.request();

        // Skip auth header for auth endpoints
        String path = original.url().encodedPath();
        if (path.contains("/auth/refresh") || path.contains("/auth/sdk/recover")) {
            return chain.proceed(original);
        }

        // Proactively refresh token before it expires
        ensureValidToken();

        Request.Builder builder = original.newBuilder();
        if (accessToken != null) {
            builder.header("Authorization", "Bearer " + accessToken);
        }
        if (sdkTokenId != null) {
            builder.header("X-SDK-Token", sdkTokenId);
        }

        return chain.proceed(builder.build());
    }

    /**
     * Interceptor for retry with exponential backoff.
     */
    private Response retryInterceptor(Interceptor.Chain chain) throws IOException {
        Request request = chain.request();
        Response response = null;
        IOException lastException = null;

        for (int attempt = 0; attempt <= MAX_RETRIES; attempt++) {
            try {
                if (response != null) {
                    response.close();
                }
                response = chain.proceed(request);

                // Success or client error (don't retry 4xx except 401/408/429)
                int code = response.code();
                if (code < 500 && code != 408 && code != 429) {
                    return response;
                }

                // Server error or retryable - wait and retry
                if (attempt < MAX_RETRIES) {
                    response.close();
                    int backoff = Math.min(INITIAL_BACKOFF_MS * (1 << attempt), MAX_BACKOFF_MS);
                    logger.debug("Request failed with {}, retrying in {}ms (attempt {}/{})",
                            code, backoff, attempt + 1, MAX_RETRIES);
                    Thread.sleep(backoff);
                }

            } catch (IOException e) {
                lastException = e;
                if (attempt < MAX_RETRIES) {
                    int backoff = Math.min(INITIAL_BACKOFF_MS * (1 << attempt), MAX_BACKOFF_MS);
                    logger.debug("Request failed with {}, retrying in {}ms (attempt {}/{})",
                            e.getClass().getSimpleName(), backoff, attempt + 1, MAX_RETRIES);
                    try {
                        Thread.sleep(backoff);
                    } catch (InterruptedException ie) {
                        Thread.currentThread().interrupt();
                        throw new IOException("Interrupted during retry", ie);
                    }
                }
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
                throw new IOException("Interrupted during retry", e);
            }
        }

        if (response != null) {
            return response;
        }
        throw lastException != null ? lastException : new IOException("Request failed after retries");
    }

    /**
     * Handle 401 by refreshing token and retrying.
     */
    private Request handleAuthChallenge(Route route, Response response) {
        // Only retry on 401
        if (response.code() != 401) {
            return null;
        }

        // Prevent infinite loops
        if (response.request().header("X-Retry-Auth") != null) {
            logger.warn("Auth retry already attempted, not retrying again");
            return null;
        }

        logger.info("Received 401, refreshing authentication...");

        try {
            // Refresh token (synchronized to prevent concurrent refreshes)
            refreshAuthToken();

            // Retry request with new token
            return response.request().newBuilder()
                    .header("Authorization", "Bearer " + accessToken)
                    .header("X-Retry-Auth", "true")
                    .build();

        } catch (Exception e) {
            logger.error("Failed to refresh authentication: {}", e.getMessage());
            return null;
        }
    }

    /**
     * Refresh authentication token with synchronization.
     */
    private void refreshAuthToken() {
        tokenLock.lock();
        try {
            // Double-check if another thread already refreshed
            if (accessToken != null && tokenExpiry != null && Instant.now().isBefore(tokenExpiry)) {
                logger.debug("Token already refreshed by another thread");
                return;
            }

            logger.debug("Refreshing authentication token...");
            authenticate();

        } finally {
            tokenLock.unlock();
        }
    }

    /**
     * Ensure we have a valid token, refreshing proactively if needed.
     */
    private void ensureValidToken() {
        // Quick check without lock
        if (accessToken != null && tokenExpiry != null && Instant.now().isBefore(tokenExpiry)) {
            return;
        }

        // Need to refresh - use synchronized refresh
        refreshAuthToken();
    }

    /**
     * One-line secure registration - the recommended way to use AIM.
     *
     * @param agentName Name for your agent
     * @return Configured AIMClient ready for use
     */
    public static AIMClient secure(String agentName) {
        return secure(agentName, null, null);
    }

    /**
     * Secure registration with capabilities.
     *
     * @param agentName    Name for your agent
     * @param capabilities List of capabilities to register
     * @return Configured AIMClient ready for use
     */
    public static AIMClient secure(String agentName, List<String> capabilities) {
        return secure(agentName, capabilities, null);
    }

    /**
     * Secure registration with capabilities and agent type.
     *
     * @param agentName    Name for your agent
     * @param capabilities List of capabilities to register
     * @param agentType    Type of agent (auto-detected if null)
     * @return Configured AIMClient ready for use
     */
    public static AIMClient secure(String agentName, List<String> capabilities, AgentType agentType) {
        return secure(agentName, capabilities, agentType, null);
    }

    /**
     * Secure registration with capabilities, agent type, and MCP server connections.
     *
     * @param agentName    Name for your agent
     * @param capabilities List of capabilities to register
     * @param agentType    Type of agent (auto-detected if null)
     * @param talksTo      List of MCP servers this agent communicates with
     * @return Configured AIMClient ready for use
     */
    public static AIMClient secure(String agentName, List<String> capabilities, AgentType agentType, List<String> talksTo) {
        return secure(agentName, capabilities, agentType, talksTo, null, null, null);
    }

    /**
     * Secure registration with full configuration including description, tags, and metadata.
     *
     * @param agentName    Name for your agent
     * @param capabilities List of capabilities to register
     * @param agentType    Type of agent (auto-detected if null)
     * @param talksTo      List of MCP servers this agent communicates with
     * @param description  Description of the agent
     * @param tags         List of tags for categorization (e.g., "production", "customer-facing")
     * @param metadata     Additional metadata as key-value pairs
     * @return Configured AIMClient ready for use
     */
    public static AIMClient secure(String agentName, List<String> capabilities, AgentType agentType,
                                    List<String> talksTo, String description, List<String> tags,
                                    Map<String, Object> metadata) {
        return secure(agentName, capabilities, agentType, talksTo, description, tags, metadata, null);
    }

    /**
     * Secure registration with full configuration including MCP discovery commands.
     *
     * @param agentName    Name for your agent
     * @param capabilities List of capabilities to register
     * @param agentType    Type of agent (auto-detected if null)
     * @param talksTo      List of MCP servers this agent communicates with
     * @param description  Description of the agent
     * @param tags         List of tags for categorization (e.g., "production", "customer-facing")
     * @param metadata     Additional metadata as key-value pairs
     * @param mcpCommands  Map of MCP server name to command for capability discovery
     * @return Configured AIMClient ready for use
     */
    public static AIMClient secure(String agentName, List<String> capabilities, AgentType agentType,
                                    List<String> talksTo, String description, List<String> tags,
                                    Map<String, Object> metadata, Map<String, String> mcpCommands) {
        // Load credentials from SDK download (with intelligent discovery)
        Map<String, String> credentials = CredentialManager.loadSdkCredentials();

        String aimUrl = credentials.getOrDefault("aimUrl", "http://localhost:8080");

        if (credentials.isEmpty()) {
            CredentialManager.printSdkCredentialsNotFoundError(aimUrl);
            throw new CredentialException(
                    "No SDK credentials found. Download the SDK from your AIM dashboard (Settings -> SDK Downloads)");
        }

        // Check for SDK OAuth credentials (refresh token flow) - preferred method
        String refreshToken = credentials.get("refreshToken");
        String sdkTokenId = credentials.get("sdkTokenId");

        // Fall back to client credentials if no refresh token
        String clientId = credentials.get("clientId");
        String clientSecret = credentials.get("clientSecret");
        String organizationId = credentials.get("organizationId");

        // Validate we have at least one valid credential type
        boolean hasRefreshToken = refreshToken != null && !refreshToken.isEmpty();
        boolean hasClientCredentials = clientId != null && clientSecret != null;

        if (!hasRefreshToken && !hasClientCredentials) {
            CredentialManager.CredentialType foundType = CredentialManager.detectCredentialType(credentials);
            CredentialManager.printWrongCredentialTypeError(foundType, aimUrl);
            throw new CredentialException(
                    "Invalid SDK credentials. Need either refreshToken or clientId/clientSecret. " +
                    "Re-download from AIM dashboard (Settings -> SDK Downloads).");
        }

        Builder builder = new Builder()
                .agentName(agentName)
                .aimUrl(aimUrl)
                .agentType(agentType != null ? agentType : AgentType.CUSTOM)
                .capabilities(capabilities != null ? capabilities : Collections.emptyList())
                .talksTo(talksTo != null ? talksTo : Collections.emptyList())
                .mcpCommands(mcpCommands != null ? mcpCommands : Collections.emptyMap())
                .description(description)
                .tags(tags != null ? tags : Collections.emptyList())
                .metadata(metadata != null ? metadata : Collections.emptyMap())
                .credentials(credentials);

        if (hasRefreshToken) {
            builder.refreshToken(refreshToken).sdkTokenId(sdkTokenId);
            String tokenIdPreview = sdkTokenId != null && sdkTokenId.length() > 8
                    ? sdkTokenId.substring(0, 8) + "..."
                    : sdkTokenId;
            logger.info("Using SDK OAuth authentication (token: {})", tokenIdPreview);
        } else {
            builder.clientId(clientId).clientSecret(clientSecret).organizationId(organizationId);
            logger.info("Using client credentials authentication");
        }

        AIMClient client = builder.build();

        // Register agent
        client.registerAgent();

        return client;
    }

    /**
     * Register this agent with AIM, or reconnect to existing agent if already registered.
     * Uses the authenticated /api/v1/agents endpoint (Bearer token).
     *
     * Follows the "get or create" pattern:
     * 1. First check if agent with this name already exists
     * 2. If exists: reconnect and update configuration (capabilities, tags, metadata)
     * 3. If not exists: create new agent
     */
    private void registerAgent() {
        try {
            // First, authenticate via OAuth
            authenticate();

            // Generate Ed25519 key pair for signing
            generateKeyPair();

            // FIRST: Check if agent already exists (get or create pattern)
            // Use sdk-api endpoint which accepts both ID and name
            try {
                String response = get("/api/v1/sdk-api/agents/" + agentName);
                // Agent exists - reconnect to it
                logger.info("Agent '{}' already exists, connecting...", agentName);
                handleRegistrationResponse(response, true);

                // Update the agent with new configuration
                if (this.agentId != null) {
                    updateExistingAgent();
                    // Register and attest MCP servers
                    registerAndAttestMcpServers();
                }
                return;
            } catch (AIMException e) {
                // 404 means agent doesn't exist - continue to create
                if (!e.getMessage().contains("404")) {
                    throw e;
                }
            }

            // Agent doesn't exist - create it
            ObjectNode payload = buildAgentPayload();
            String response = post("/api/v1/agents", payload.toString());
            handleRegistrationResponse(response, false);

            // Register and attest MCP servers
            registerAndAttestMcpServers();

        } catch (AIMException e) {
            throw e;
        } catch (Exception e) {
            throw new AIMException("Failed to register agent: " + e.getMessage(), e);
        }
    }

    /**
     * Build the agent registration/update payload.
     */
    private ObjectNode buildAgentPayload() {
        ObjectNode payload = objectMapper.createObjectNode();
        payload.put("name", agentName);
        payload.put("displayName", agentName);
        payload.put("description", description != null ? description : "Agent registered via AIM Java SDK");
        payload.put("publicKey", Base64.getEncoder().encodeToString(publicKey));
        payload.put("agentType", agentType.getValue());
        payload.put("version", "1.0.0");

        if (metadata != null && !metadata.isEmpty()) {
            payload.set("metadata", objectMapper.valueToTree(metadata));
        }

        if (!capabilities.isEmpty()) {
            var capArray = payload.putArray("capabilities");
            for (String cap : capabilities) {
                capArray.add(cap);
            }
        }

        if (talksTo != null && !talksTo.isEmpty()) {
            var talksToArray = payload.putArray("talksTo");
            for (String mcp : talksTo) {
                talksToArray.add(mcp);
            }
        }

        if (tags != null && !tags.isEmpty()) {
            var tagsArray = payload.putArray("tagIds");
            for (String tag : tags) {
                tagsArray.add(tag);
            }
        }

        return payload;
    }

    /**
     * Handle the registration response and extract agent ID.
     */
    private void handleRegistrationResponse(String response, boolean isReconnect) throws Exception {
        JsonNode result = objectMapper.readTree(response);

        // Handle nested response structure from GET /agents/{name}
        JsonNode agentNode = result.has("agent") ? result.get("agent") : result;

        String agentIdField = agentNode.has("agentId") ? "agentId" : "id";
        if (agentNode.has(agentIdField)) {
            this.agentId = agentNode.get(agentIdField).asText();
            if (isReconnect) {
                logger.info("Reconnected to existing agent: {} ({})", agentName, this.agentId);
            } else {
                logger.info("Agent registered: {} ({})", agentName, this.agentId);
            }

            if (!isReconnect && result.has("apiKey") && result.get("apiKey").has("key")) {
                String apiKeyPrefix = result.get("apiKey").get("prefix").asText();
                logger.info("API key auto-generated: {}...", apiKeyPrefix);
            }
        }
    }

    /**
     * Update an existing agent with new public key, capabilities, tags, or metadata.
     * This is called when reconnecting to an existing agent to sync configuration.
     */
    private void updateExistingAgent() {
        // 1. Update public key via dedicated /keys endpoint
        try {
            ObjectNode keyPayload = objectMapper.createObjectNode();
            keyPayload.put("publicKey", Base64.getEncoder().encodeToString(publicKey));
            put("/api/v1/agents/" + agentId + "/keys", keyPayload.toString());
            logger.debug("Updated public key for agent: {}", agentName);
        } catch (Exception e) {
            logger.warn("Could not update agent public key: {}", e.getMessage());
        }

        // 2. Update agent configuration (capabilities, tags, metadata)
        try {
            ObjectNode updatePayload = objectMapper.createObjectNode();
            boolean hasUpdates = false;

            // Update description if provided
            if (description != null) {
                updatePayload.put("description", description);
                hasUpdates = true;
            }

            // Update metadata if provided
            if (metadata != null && !metadata.isEmpty()) {
                updatePayload.set("metadata", objectMapper.valueToTree(metadata));
                hasUpdates = true;
            }

            // Update capabilities if provided
            if (!capabilities.isEmpty()) {
                var capArray = updatePayload.putArray("capabilities");
                for (String cap : capabilities) {
                    capArray.add(cap);
                }
                hasUpdates = true;
            }

            // Update talksTo if provided
            if (talksTo != null && !talksTo.isEmpty()) {
                var talksToArray = updatePayload.putArray("talksTo");
                for (String mcp : talksTo) {
                    talksToArray.add(mcp);
                }
                hasUpdates = true;
            }

            // Update tags if provided
            if (tags != null && !tags.isEmpty()) {
                var tagsArray = updatePayload.putArray("tagIds");
                for (String tag : tags) {
                    tagsArray.add(tag);
                }
                hasUpdates = true;
            }

            if (hasUpdates) {
                put("/api/v1/agents/" + agentId, updatePayload.toString());
                logger.info("Updated agent configuration for: {}", agentName);
            }
        } catch (Exception e) {
            // Log but don't fail - agent is still usable
            logger.warn("Could not update agent configuration: {}", e.getMessage());
        }
    }

    /**
     * Register and attest MCP servers from talksTo list.
     *
     * Production-grade implementation with:
     * - Parallel discovery of multiple MCP servers (4x faster for multiple servers)
     * - File-based caching with 1-hour TTL (instant on subsequent runs)
     * - Automatic retry on transient failures
     * - Graceful degradation if discovery fails
     */
    private void registerAndAttestMcpServers() {
        if (talksTo == null || talksTo.isEmpty()) {
            return;
        }

        System.out.println("  Registering " + talksTo.size() + " MCP server(s)...");

        // Step 1: Build map of servers to discover (only those with commands)
        Map<String, String> serversToDiscover = new LinkedHashMap<>();
        if (mcpCommands != null) {
            for (String mcpName : talksTo) {
                String command = mcpCommands.get(mcpName);
                if (command != null && !command.isEmpty()) {
                    serversToDiscover.put(mcpName, command);
                }
            }
        }

        // Step 2: Discover all MCP servers in PARALLEL (production optimization)
        Map<String, MCPDiscoveryResult> discoveryResults = Collections.emptyMap();
        if (!serversToDiscover.isEmpty()) {
            System.out.println("  → Discovering capabilities from " + serversToDiscover.size() + " server(s) in parallel...");
            long discoveryStart = System.currentTimeMillis();

            discoveryResults = MCPDiscoveryService.getInstance().discoverAll(serversToDiscover);

            long discoveryTime = System.currentTimeMillis() - discoveryStart;
            int totalTools = discoveryResults.values().stream()
                    .filter(MCPDiscoveryResult::isSuccess)
                    .mapToInt(r -> r.getTools().size())
                    .sum();
            int totalResources = discoveryResults.values().stream()
                    .filter(MCPDiscoveryResult::isSuccess)
                    .mapToInt(r -> r.getResources().size())
                    .sum();

            System.out.println("  ✓ Discovery complete: " + totalTools + " tools, " +
                    totalResources + " resources (" + discoveryTime + "ms total)");
        }

        // Step 3: Register and attest each server
        for (String mcpName : talksTo) {
            try {
                // Register the MCP server
                String mcpServerId = registerMcpServerViaSdkEndpoint(mcpName);
                if (mcpServerId == null) {
                    System.out.println("    ⚠️  Could not register MCP server: " + mcpName);
                    continue;
                }

                // Get discovery result for this server (from parallel discovery)
                MCPDiscoveryResult discovery = discoveryResults.get(mcpName);
                List<String> discoveredCapabilities = Collections.emptyList();
                double latencyMs = 0.0;

                if (discovery != null && discovery.isSuccess()) {
                    discoveredCapabilities = discovery.getAllCapabilityNames();
                    latencyMs = discovery.getConnectionLatencyMs();

                    System.out.println("    ✓ " + mcpName + ": " +
                            discovery.getTools().size() + " tools" +
                            (discovery.getResources().size() > 0 ? ", " + discovery.getResources().size() + " resources" : "") +
                            (discovery.getPrompts().size() > 0 ? ", " + discovery.getPrompts().size() + " prompts" : ""));
                } else if (discovery != null && discovery.hasError()) {
                    System.out.println("    ⚠️  " + mcpName + ": discovery failed - " + discovery.getError());
                } else {
                    System.out.println("    ✓ " + mcpName + ": registered (no discovery command)");
                }

                // Attest to the MCP server
                try {
                    attestMcp(mcpServerId, "stdio://" + mcpName, mcpName, discoveredCapabilities, latencyMs);
                } catch (Exception e) {
                    System.out.println("    ⚠️  Attestation failed for " + mcpName + ": " + e.getMessage());
                }

            } catch (Exception e) {
                System.out.println("    ⚠️  Error with " + mcpName + ": " + e.getMessage());
            }
        }
    }

    /**
     * Register an MCP server using the SDK endpoint.
     * Uses Ed25519 signed request for authentication.
     *
     * Endpoint: POST /api/v1/sdk-api/agents/{agentId}/mcp-servers
     *
     * @param mcpName Name of the MCP server
     * @return MCP server ID or null if registration fails
     */
    private String registerMcpServerViaSdkEndpoint(String mcpName) {
        try {
            String mcpUrl = "stdio://" + mcpName;

            // Build MCP server registration data
            ObjectNode mcpData = objectMapper.createObjectNode();
            mcpData.put("name", mcpName);
            mcpData.put("description", "MCP server registered by " + agentName);
            mcpData.put("url", mcpUrl);
            mcpData.put("version", "1.0.0");
            mcpData.putArray("capabilities");  // Empty for now, will be discovered later
            mcpData.put("registeredByAgent", agentId);
            mcpData.put("verificationMethod", "agent_attestation");

            // Use SDK endpoint for MCP registration
            String endpoint = "/api/v1/sdk-api/agents/" + agentId + "/mcp-servers";
            String response = postWithSignature(endpoint, mcpData.toString());

            JsonNode result = objectMapper.readTree(response);
            return result.has("id") ? result.get("id").asText() : null;

        } catch (AIMException e) {
            // 409 Conflict means server already exists - extract ID from response
            if (e.getMessage() != null && e.getMessage().contains("409")) {
                try {
                    // Try to get existing MCP server ID from the error message or list
                    String listEndpoint = "/api/v1/sdk-api/agents/" + agentId + "/mcp-servers";
                    String response = getWithSignature(listEndpoint);
                    JsonNode result = objectMapper.readTree(response);
                    JsonNode servers = result.has("mcpServers") ? result.get("mcpServers") : result;
                    if (servers.isArray()) {
                        for (JsonNode server : servers) {
                            if (mcpName.equals(server.path("name").asText())) {
                                return server.get("id").asText();
                            }
                        }
                    }
                } catch (Exception ex) {
                    logger.debug("Could not get existing MCP server: {}", ex.getMessage());
                }
            }
            logger.debug("MCP registration failed: {}", e.getMessage());
            return null;
        } catch (Exception e) {
            logger.debug("MCP registration error: {}", e.getMessage());
            return null;
        }
    }

    /**
     * POST request with Ed25519 signature.
     */
    private String postWithSignature(String endpoint, String body) {
        try {
            String timestamp = String.valueOf(System.currentTimeMillis() / 1000);
            String message = "POST\n" + endpoint + "\n" + timestamp + "\n" + body;
            String signature = sign(message);

            Request.Builder builder = new Request.Builder()
                .url(aimUrl + endpoint)
                .post(RequestBody.create(body, JSON));

            builder.addHeader("X-Agent-ID", agentId);
            builder.addHeader("X-Signature", signature);
            builder.addHeader("X-Timestamp", timestamp);
            builder.addHeader("X-Public-Key", Base64.getEncoder().encodeToString(publicKey));

            try (Response response = httpClient.newCall(builder.build()).execute()) {
                String responseBody = response.body() != null ? response.body().string() : "";
                if (!response.isSuccessful()) {
                    throw new AIMException("HTTP " + response.code() + ": " + responseBody);
                }
                return responseBody;
            }
        } catch (AIMException e) {
            throw e;
        } catch (Exception e) {
            throw new AIMException("Request failed: " + e.getMessage(), e);
        }
    }

    /**
     * GET request with Ed25519 signature.
     */
    private String getWithSignature(String endpoint) {
        try {
            String timestamp = String.valueOf(System.currentTimeMillis() / 1000);
            String message = "GET\n" + endpoint + "\n" + timestamp + "\n";
            String signature = sign(message);

            Request.Builder builder = new Request.Builder()
                .url(aimUrl + endpoint)
                .get();

            builder.addHeader("X-Agent-ID", agentId);
            builder.addHeader("X-Signature", signature);
            builder.addHeader("X-Timestamp", timestamp);
            builder.addHeader("X-Public-Key", Base64.getEncoder().encodeToString(publicKey));

            try (Response response = httpClient.newCall(builder.build()).execute()) {
                String responseBody = response.body() != null ? response.body().string() : "";
                if (!response.isSuccessful()) {
                    throw new AIMException("HTTP " + response.code() + ": " + responseBody);
                }
                return responseBody;
            }
        } catch (AIMException e) {
            throw e;
        } catch (Exception e) {
            throw new AIMException("Request failed: " + e.getMessage(), e);
        }
    }

    /**
     * Authenticate with AIM using either refresh token or client credentials.
     */
    private void authenticate() {
        if (refreshToken != null && !refreshToken.isEmpty()) {
            authenticateWithRefreshToken();
        } else if (clientId != null && clientSecret != null) {
            authenticateWithClientCredentials();
        } else {
            throw new AuthenticationException("No valid authentication credentials available");
        }
    }

    /**
     * Authenticate using refresh token (SDK OAuth flow).
     * Uses authClient to avoid infinite loops from 401 handling.
     */
    private void authenticateWithRefreshToken() {
        try {
            ObjectNode payload = objectMapper.createObjectNode();
            payload.put("refreshToken", refreshToken);

            RequestBody body = RequestBody.create(payload.toString(), JSON);

            Request request = new Request.Builder()
                    .url(aimUrl + "/api/v1/auth/refresh")
                    .post(body)
                    .build();

            // Use authClient (no authenticator) to avoid infinite loops
            try (Response response = authClient.newCall(request).execute()) {
                if (!response.isSuccessful()) {
                    String errorBody = response.body() != null ? response.body().string() : "";
                    logger.warn("Refresh token auth failed: {} - {}", response.code(), errorBody);

                    // Try token recovery if token was revoked
                    if (errorBody.toLowerCase().contains("revoked") ||
                        errorBody.toLowerCase().contains("invalid") ||
                        errorBody.toLowerCase().contains("expired")) {

                        System.out.println("\n🔄 Token was revoked - attempting automatic recovery...");
                        logger.info("Attempting token recovery...");

                        if (attemptTokenRecovery()) {
                            System.out.println("✅ Token recovered successfully! SDK credentials updated.");
                            return; // Recovery successful
                        }

                        // Recovery failed - show user-friendly error
                        printTokenExpiredError();
                    }

                    throw new AuthenticationException(
                        "SDK authentication failed. Download a fresh SDK from your AIM dashboard.");
                }

                String responseBody = response.body().string();
                JsonNode json = objectMapper.readTree(responseBody);

                this.accessToken = json.has("accessToken") ? json.get("accessToken").asText() : json.get("access_token").asText();

                // Handle token rotation - server may return a new refresh token
                if (json.has("refreshToken")) {
                    String newRefreshToken = json.get("refreshToken").asText();
                    if (!newRefreshToken.equals(this.refreshToken)) {
                        this.refreshToken = newRefreshToken;
                        updateStoredCredentials(newRefreshToken);
                        extractAndSaveTokenId(newRefreshToken);
                        System.out.println("🔄 Token rotated successfully");
                        logger.debug("Refresh token rotated");
                    }
                }

                // Decode token expiry from JWT
                setTokenExpiryFromJWT(this.accessToken);

                logger.debug("Authenticated successfully via refresh token");
            }
        } catch (IOException e) {
            throw new AuthenticationException("Failed to authenticate with refresh token: " + e.getMessage(), e);
        }
    }

    /**
     * Print user-friendly error when token is expired.
     */
    private void printTokenExpiredError() {
        String border = "=".repeat(72);
        System.out.println();
        System.out.println(border);
        System.out.println("SDK TOKEN EXPIRED");
        System.out.println(border);
        System.out.println();
        System.out.println("Your SDK authentication token has expired or been revoked.");
        System.out.println();
        System.out.println("This can happen if:");
        System.out.println("  - The token expired (90 days since last use)");
        System.out.println("  - The token was revoked for security reasons");
        System.out.println("  - Another SDK installation rotated the token");
        System.out.println();
        System.out.println("TO FIX:");
        System.out.println("  1. Visit: " + aimUrl);
        System.out.println("  2. Go to Settings -> SDK Downloads");
        System.out.println("  3. Download a fresh Java SDK");
        System.out.println("  4. Extract and add to your project");
        System.out.println();
        System.out.println("Your agents and data are safe! Only SDK credentials need updating.");
        System.out.println();
        System.out.println("Tip: You can also manage SDK tokens from the dashboard:");
        System.out.println("     Settings -> SDK Tokens");
        System.out.println(border);
        System.out.println();
    }

    /**
     * Extract and save the token ID (JTI) from the refresh token.
     */
    private void extractAndSaveTokenId(String token) {
        try {
            String[] parts = token.split("\\.");
            if (parts.length == 3) {
                String payload = parts[1];
                // Add padding if needed
                int padding = 4 - payload.length() % 4;
                if (padding != 4) {
                    payload = payload + "=".repeat(padding);
                }
                byte[] decoded = Base64.getUrlDecoder().decode(payload);
                JsonNode claims = objectMapper.readTree(decoded);
                if (claims.has("jti") && credentials != null) {
                    credentials.put("sdkTokenId", claims.get("jti").asText());
                }
            }
        } catch (Exception e) {
            logger.debug("Could not extract token ID: {}", e.getMessage());
        }
    }

    /**
     * Set token expiry by decoding the JWT access token.
     */
    private void setTokenExpiryFromJWT(String token) {
        try {
            String[] parts = token.split("\\.");
            if (parts.length == 3) {
                String payload = parts[1];
                // Add padding if needed
                int padding = 4 - payload.length() % 4;
                if (padding != 4) {
                    payload = payload + "=".repeat(padding);
                }
                byte[] decoded = Base64.getUrlDecoder().decode(payload);
                JsonNode claims = objectMapper.readTree(decoded);
                if (claims.has("exp")) {
                    long exp = claims.get("exp").asLong();
                    this.tokenExpiry = Instant.ofEpochSecond(exp).minusSeconds(60); // Refresh 60s early
                    return;
                }
            }
        } catch (Exception e) {
            logger.debug("Could not decode token expiry: {}", e.getMessage());
        }
        // Default: 15 minutes minus 60s buffer
        this.tokenExpiry = Instant.now().plusSeconds(840);
    }

    /**
     * Attempt token recovery when refresh token is revoked.
     * Uses authClient to avoid infinite loops from 401 handling.
     */
    private boolean attemptTokenRecovery() {
        try {
            ObjectNode payload = objectMapper.createObjectNode();
            payload.put("old_refresh_token", refreshToken);

            RequestBody body = RequestBody.create(payload.toString(), JSON);

            Request request = new Request.Builder()
                    .url(aimUrl + "/api/v1/auth/sdk/recover")
                    .post(body)
                    .build();

            // Use authClient (no authenticator) to avoid infinite loops
            try (Response response = authClient.newCall(request).execute()) {
                if (!response.isSuccessful()) {
                    return false;
                }

                String responseBody = response.body().string();
                JsonNode json = objectMapper.readTree(responseBody);

                this.accessToken = json.has("accessToken") ? json.get("accessToken").asText() : null;
                String newRefreshToken = json.has("refreshToken") ? json.get("refreshToken").asText() : null;

                if (accessToken != null && newRefreshToken != null) {
                    this.refreshToken = newRefreshToken;
                    updateStoredCredentials(newRefreshToken);
                    this.tokenExpiry = Instant.now().plusSeconds(840); // 14 minutes
                    logger.info("Token recovered successfully");
                    return true;
                }
            }
        } catch (Exception e) {
            logger.warn("Token recovery failed: {}", e.getMessage());
        }
        return false;
    }

    /**
     * Update stored credentials with new refresh token.
     */
    private void updateStoredCredentials(String newRefreshToken) {
        if (credentials != null) {
            credentials.put("refreshToken", newRefreshToken);
            try {
                CredentialManager.saveSdkCredentials(credentials, null);
            } catch (Exception e) {
                logger.warn("Failed to save updated credentials: {}", e.getMessage());
            }
        }
    }

    /**
     * Authenticate using OAuth client credentials flow.
     */
    private void authenticateWithClientCredentials() {
        try {
            FormBody formBody = new FormBody.Builder()
                    .add("grant_type", "client_credentials")
                    .add("client_id", clientId)
                    .add("client_secret", clientSecret)
                    .build();

            Request request = new Request.Builder()
                    .url(aimUrl + "/oauth/token")
                    .post(formBody)
                    .build();

            try (Response response = httpClient.newCall(request).execute()) {
                if (!response.isSuccessful()) {
                    throw new AuthenticationException("OAuth authentication failed: " + response.code());
                }

                String body = response.body().string();
                JsonNode json = objectMapper.readTree(body);

                this.accessToken = json.get("access_token").asText();
                int expiresIn = json.has("expires_in") ? json.get("expires_in").asInt() : 3600;
                this.tokenExpiry = Instant.now().plusSeconds(expiresIn - 60); // Refresh 60s early
            }
        } catch (IOException e) {
            throw new AuthenticationException("Failed to authenticate: " + e.getMessage(), e);
        }
    }

    /**
     * Generate Ed25519 key pair for signing.
     */
    private void generateKeyPair() {
        try {
            java.security.SecureRandom random = new java.security.SecureRandom();
            Ed25519PrivateKeyParameters privateKeyParams = new Ed25519PrivateKeyParameters(random);
            Ed25519PublicKeyParameters publicKeyParams = privateKeyParams.generatePublicKey();

            this.privateKey = privateKeyParams.getEncoded();
            this.publicKey = publicKeyParams.getEncoded();
        } catch (Exception e) {
            throw new AIMException("Failed to generate key pair: " + e.getMessage(), e);
        }
    }

    /**
     * Verify a capability before performing an action.
     *
     * @param capability The capability to verify (e.g., "db:read")
     * @param resource   The resource being accessed (e.g., "users_table")
     * @return VerificationResult with verification status
     */
    public VerificationResult verifyCapability(String capability, String resource) {
        return verifyCapability(capability, resource, RiskLevel.MEDIUM, null, false, 300);
    }

    /**
     * Verify a capability with risk level and context.
     *
     * @param capability The capability to verify
     * @param resource   The resource being accessed
     * @param riskLevel  Risk level of the action
     * @param context    Additional context for verification
     * @return VerificationResult with verification status
     */
    public VerificationResult verifyCapability(String capability, String resource,
                                                RiskLevel riskLevel, Map<String, Object> context) {
        return verifyCapability(capability, resource, riskLevel, context, false, 300);
    }

    /**
     * Verify a capability with JIT access (requires admin approval).
     *
     * When jitAccess is true, the action will be held for admin approval if the agent
     * doesn't already have the capability. Use this for critical operations that require
     * human oversight.
     *
     * @param capability The capability to verify
     * @param resource   The resource being accessed
     * @param riskLevel  Risk level of the action
     * @param context    Additional context for verification
     * @param jitAccess  If true, requires admin approval for sensitive operations
     * @return VerificationResult with verification status
     */
    public VerificationResult verifyCapability(String capability, String resource,
                                                RiskLevel riskLevel, Map<String, Object> context,
                                                boolean jitAccess) {
        return verifyCapability(capability, resource, riskLevel, context, jitAccess, 300);
    }

    /**
     * Verify a capability with risk level, context, JIT access, and timeout.
     *
     * This method:
     * 1. Creates a verification request with capability details
     * 2. Signs the request with the agent's private key
     * 3. Sends the request to AIM
     * 4. If status is "pending", waits for JIT approval (up to timeoutSeconds)
     * 5. Returns verification result
     *
     * @param capability     The capability to verify
     * @param resource       The resource being accessed
     * @param riskLevel      Risk level of the action
     * @param context        Additional context for verification
     * @param jitAccess      If true, requires admin approval for sensitive operations
     * @param timeoutSeconds Maximum time to wait for JIT approval (default: 300s = 5min)
     * @return VerificationResult with verification status
     */
    public VerificationResult verifyCapability(String capability, String resource,
                                                RiskLevel riskLevel, Map<String, Object> context,
                                                boolean jitAccess, int timeoutSeconds) {
        try {
            // Create timestamp matching backend expected format
            String timestamp = Instant.now().toString();

            // Build merged context with JIT access info
            Map<String, Object> mergedContext = context != null ? new HashMap<>(context) : new HashMap<>();
            if (jitAccess) {
                mergedContext.put("jit_access", true);
                mergedContext.put("risk_level", riskLevel != null ? riskLevel.getValue() : "medium");
                logger.info("JIT access requested for capability '{}' - will wait for admin approval", capability);
            }

            // Create signature payload with sorted keys for deterministic signing
            // Uses format: json.dumps(payload, sort_keys=True, separators=(', ', ': '))
            // Keys must be sorted: action_type, agent_id, context, resource, timestamp
            Map<String, Object> signaturePayload = new TreeMap<>();  // TreeMap for sorted keys
            signaturePayload.put("action_type", capability);
            signaturePayload.put("agent_id", agentId);
            signaturePayload.put("context", mergedContext);
            signaturePayload.put("resource", resource);
            signaturePayload.put("timestamp", timestamp);

            // Create deterministic JSON matching Python format
            // Python: separators=(', ', ': ') means comma+space and colon+space
            String signatureMessage = createDeterministicJson(signaturePayload);
            String signature = sign(signatureMessage);

            // Create request payload (camelCase for backend)
            ObjectNode payload = objectMapper.createObjectNode();
            payload.put("agentId", agentId);
            payload.put("capability", capability);
            payload.put("resource", resource);
            payload.set("context", objectMapper.valueToTree(mergedContext));
            payload.put("timestamp", timestamp);
            payload.put("signature", signature);
            payload.put("publicKey", Base64.getEncoder().encodeToString(publicKey));

            // Use SDK API endpoint (no OAuth auth - uses Ed25519 signature auth)
            String url = aimUrl + "/api/v1/sdk-api/verifications";
            RequestBody body = RequestBody.create(payload.toString(), JSON);
            Request request = new Request.Builder()
                    .url(url)
                    .post(body)
                    .header("Content-Type", "application/json")
                    .header("User-Agent", "AIM-Java-SDK/1.0.0")
                    .build();

            // Use authClient (no auth interceptor) for verification
            try (Response httpResponse = authClient.newCall(request).execute()) {
                if (!httpResponse.isSuccessful()) {
                    String errorBody = httpResponse.body() != null ? httpResponse.body().string() : "";
                    if (httpResponse.code() == 404) {
                        logger.warn("AIM verification endpoint not found (404)");
                        return VerificationResult.builder()
                                .verified(false)
                                .status("pending")
                                .capability(capability)
                                .resource(resource)
                                .timestamp(Instant.now())
                                .build();
                    }
                    throw new VerificationException("Verification failed: " + httpResponse.code() + " - " + errorBody);
                }

                String responseBody = httpResponse.body().string();
                JsonNode result = objectMapper.readTree(responseBody);

                // Get status - "approved" means verified
                String status = result.has("status") ? result.get("status").asText() : "unknown";

                // Get enforcement mode from response
                String enforcementMode = result.has("enforcementMode") ? result.get("enforcementMode").asText() : "monitoring";

                // Get verification ID - field might be "id" or "verificationId"
                String verificationId = null;
                if (result.has("id")) {
                    verificationId = result.get("id").asText();
                } else if (result.has("verificationId")) {
                    verificationId = result.get("verificationId").asText();
                }

                // If approved (or auto-approved in monitoring mode), return immediately
                if ("approved".equalsIgnoreCase(status) || "auto-approved".equalsIgnoreCase(status)) {
                    logger.info("Capability '{}' {}", capability, status);
                    return VerificationResult.builder()
                            .verified(true)
                            .status(status)
                            .verificationId(verificationId)
                            .capability(capability)
                            .resource(resource)
                            .enforcementMode(enforcementMode)
                            .timestamp(Instant.now())
                            .build();
                }

                // If denied, throw exception
                if ("denied".equalsIgnoreCase(status)) {
                    String reason = result.has("denialReason") ? result.get("denialReason").asText() : "Action denied by policy";
                    logger.warn("Capability '{}' denied: {}", capability, reason);
                    throw new ActionDeniedException("Action denied: " + reason, capability, resource);
                }

                // If pending, poll for JIT approval
                if ("pending".equalsIgnoreCase(status)) {
                    logger.info("Capability '{}' pending JIT approval, polling for {} seconds...", capability, timeoutSeconds);
                    return waitForApproval(verificationId, timeoutSeconds, capability, resource);
                }

                // Unknown status - check verified field
                boolean verified = result.has("verified") && result.get("verified").asBoolean();
                return VerificationResult.builder()
                        .verified(verified)
                        .status(status)
                        .verificationId(verificationId)
                        .capability(capability)
                        .resource(resource)
                        .enforcementMode(enforcementMode)
                        .timestamp(Instant.now())
                        .build();
            }

        } catch (VerificationException e) {
            throw e;
        } catch (Exception e) {
            logger.error("Verification failed for {}: {}", capability, e.getMessage());
            throw new VerificationException("Failed to verify capability: " + e.getMessage(), e);
        }
    }

    /**
     * Execute an action with automatic verification.
     *
     * @param capability The capability required
     * @param resource   The resource being accessed
     * @param action     The action to execute if verified
     * @param <T>        Return type of the action
     * @return Result of the action
     * @throws ActionDeniedException if verification fails
     */
    public <T> T performAction(String capability, String resource, Supplier<T> action) {
        return performAction(capability, resource, RiskLevel.MEDIUM, action);
    }

    /**
     * Execute an action with automatic verification and risk level.
     *
     * @param capability The capability required
     * @param resource   The resource being accessed
     * @param riskLevel  Risk level of the action
     * @param action     The action to execute if verified
     * @param <T>        Return type of the action
     * @return Result of the action
     * @throws ActionDeniedException if verification fails
     */
    public <T> T performAction(String capability, String resource, RiskLevel riskLevel, Supplier<T> action) {
        return performAction(capability, resource, riskLevel, false, 300, action);
    }

    /**
     * Execute a critical action with JIT (Just-In-Time) approval.
     *
     * When jitAccess is true, the action will be held for admin approval if the agent
     * doesn't already have the capability. Use this for critical operations that require
     * human oversight.
     *
     * Example - equivalent to Python's @agent.perform_action(jit_access=True):
     * <pre>{@code
     * String result = agent.performAction("payment:refund", "stripe",
     *     RiskLevel.CRITICAL, true, 60,
     *     () -> processRefund(orderId, amount)
     * );
     * }</pre>
     *
     * @param capability     The capability required
     * @param resource       The resource being accessed
     * @param riskLevel      Risk level of the action
     * @param jitAccess      If true, requires admin approval for sensitive operations
     * @param timeoutSeconds Maximum time to wait for JIT approval
     * @param action         The action to execute if verified
     * @param <T>            Return type of the action
     * @return Result of the action
     * @throws ActionDeniedException if verification fails or times out
     */
    public <T> T performAction(String capability, String resource, RiskLevel riskLevel,
                                boolean jitAccess, int timeoutSeconds, Supplier<T> action) {
        VerificationResult result = verifyCapability(capability, resource, riskLevel, null, jitAccess, timeoutSeconds);
        String verificationId = result.getVerificationId();

        if (!result.isVerified()) {
            // Log the denial to Activity Timeline
            logCapabilityResult(verificationId, false, null, "Action denied: capability not authorized");
            throw new ActionDeniedException(
                    "Action denied: " + capability + " on " + resource,
                    capability,
                    resource
            );
        }

        // Execute the action and log the result
        try {
            T actionResult = action.get();
            // Log success to Activity Timeline
            String summary = actionResult != null ? actionResult.toString() : "Action completed successfully";
            if (summary.length() > 200) {
                summary = summary.substring(0, 200) + "...";
            }
            logCapabilityResult(verificationId, true, summary, null);
            return actionResult;
        } catch (Exception e) {
            // Log failure to Activity Timeline
            logCapabilityResult(verificationId, false, null, e.getMessage());
            throw new AIMException("Action execution failed: " + e.getMessage(), e);
        }
    }

    /**
     * Get agent details from AIM.
     *
     * @return Map containing agent details
     */
    public Map<String, Object> getAgentDetails() {
        return getAgentDetails(null);
    }

    /**
     * Get details for a specific agent or the current agent.
     *
     * @param targetAgentId Agent ID to fetch (defaults to current agent)
     * @return Map containing agent details
     */
    public Map<String, Object> getAgentDetails(String targetAgentId) {
        try {
            String id = (targetAgentId != null && !targetAgentId.isEmpty()) ? targetAgentId : agentId;
            if (id == null || id.isEmpty()) {
                throw new AIMException("No agent ID available");
            }
            String response = get("/api/v1/agents/" + id);
            return objectMapper.readValue(response, new TypeReference<Map<String, Object>>() {});
        } catch (Exception e) {
            throw new AIMException("Failed to get agent details: " + e.getMessage(), e);
        }
    }

    /**
     * Request a new capability.
     *
     * @param capabilityType Type of capability to request
     * @param reason         Justification for the request
     * @return Map containing request result
     */
    public Map<String, Object> requestCapability(String capabilityType, String reason) {
        try {
            ObjectNode payload = objectMapper.createObjectNode();
            payload.put("capabilityType", capabilityType);
            payload.put("reason", reason);

            // Use SDK-API endpoint with agent ID
            String url = "/api/v1/sdk-api/agents/" + agentId + "/capability-requests";
            String response = post(url, payload.toString());
            return objectMapper.readValue(response, new TypeReference<Map<String, Object>>() {});
        } catch (Exception e) {
            throw new AIMException("Failed to request capability: " + e.getMessage(), e);
        }
    }

    /**
     * Log the result of a capability execution to AIM.
     * This updates the Activity Timeline and helps build trust scores.
     *
     * @param verificationId ID from verifyCapability response
     * @param success        Whether the execution succeeded
     * @param resultSummary  Brief summary of the result (optional)
     * @param errorMessage   Error message if failed (optional)
     */
    public void logCapabilityResult(String verificationId, boolean success, String resultSummary, String errorMessage) {
        if (verificationId == null || verificationId.isEmpty()) {
            logger.warn("Cannot log capability result: verification ID is null or empty");
            return;
        }

        try {
            ObjectNode payload = objectMapper.createObjectNode();
            payload.put("result", success ? "success" : "failure");
            if (resultSummary != null) {
                payload.put("resultSummary", resultSummary);
            }
            if (errorMessage != null) {
                payload.put("errorMessage", errorMessage);
            }
            payload.put("timestamp", java.time.Instant.now().toString());

            String url = "/api/v1/sdk-api/verifications/" + verificationId + "/result";
            post(url, payload.toString());
            logger.debug("Logged capability result: verification={}, success={}", verificationId, success);
        } catch (Exception e) {
            // Don't fail the action if logging fails - just log the error
            logger.warn("Failed to log capability result: {}", e.getMessage());
        }
    }

    /**
     * Log a successful capability execution result.
     *
     * @param verificationId ID from verifyCapability response
     * @param resultSummary  Brief summary of the result
     */
    public void logSuccess(String verificationId, String resultSummary) {
        logCapabilityResult(verificationId, true, resultSummary, null);
    }

    /**
     * Log a failed capability execution result.
     *
     * @param verificationId ID from verifyCapability response
     * @param errorMessage   Error message describing the failure
     */
    public void logFailure(String verificationId, String errorMessage) {
        logCapabilityResult(verificationId, false, null, errorMessage);
    }

    /**
     * Register a new capability for this agent.
     * In MONITORING mode, capabilities are auto-granted.
     * In STRICT mode, requires admin approval.
     *
     * @param capability  Capability to register (e.g., "db:write")
     * @param description Description of the capability
     * @return Map containing registration result
     */
    public Map<String, Object> registerCapability(String capability, String description) {
        try {
            ObjectNode payload = objectMapper.createObjectNode();
            payload.put("capability", capability);
            payload.put("description", description);

            String response = post("/api/v1/agents/me/capabilities", payload.toString());
            return objectMapper.readValue(response, new TypeReference<Map<String, Object>>() {});
        } catch (Exception e) {
            throw new AIMException("Failed to register capability: " + e.getMessage(), e);
        }
    }

    // ==================== MCP SERVER METHODS ====================

    /**
     * Register an MCP server to this agent's mcp_servers list.
     *
     * @param mcpServerId     MCP server ID or name to register
     * @param detectionMethod How the MCP was detected ("manual", "auto_sdk", "auto_config", "cli")
     * @param confidence      Detection confidence score (0-100, default: 100 for manual)
     * @return Map containing registration result
     */
    public Map<String, Object> registerMcp(String mcpServerId, String detectionMethod, double confidence) {
        try {
            ObjectNode payload = objectMapper.createObjectNode();
            var mcpArray = payload.putArray("mcp_server_ids");
            mcpArray.add(mcpServerId);
            payload.put("detected_method", detectionMethod != null ? detectionMethod : "manual");
            payload.put("confidence", confidence);

            String agentId = getAgentId();
            String response = post("/api/v1/sdk-api/agents/" + agentId + "/mcp-servers", payload.toString());
            logger.info("Registered MCP server: {}", mcpServerId);
            return objectMapper.readValue(response, new TypeReference<Map<String, Object>>() {});
        } catch (Exception e) {
            throw new AIMException("Failed to register MCP server: " + e.getMessage(), e);
        }
    }

    /**
     * Register an MCP server with default detection method and confidence.
     *
     * @param mcpServerId MCP server ID or name to register
     * @return Map containing registration result
     */
    public Map<String, Object> registerMcp(String mcpServerId) {
        return registerMcp(mcpServerId, "manual", 100.0);
    }

    /**
     * Record usage of an MCP tool.
     * This builds the supply chain graph and triggers smart attestation when needed.
     *
     * @param serverId   MCP server ID
     * @param toolName   Name of the tool being used
     * @param mcpUrl     URL of the MCP server (optional, for attestation)
     * @param mcpName    Name of the MCP server (optional, for attestation)
     * @return Map containing usage recording result
     */
    public Map<String, Object> useMcpTool(String serverId, String toolName, String mcpUrl, String mcpName) {
        try {
            ObjectNode payload = objectMapper.createObjectNode();
            payload.put("serverId", serverId);
            payload.put("toolName", toolName);
            if (mcpUrl != null) {
                payload.put("mcpUrl", mcpUrl);
            }
            if (mcpName != null) {
                payload.put("mcpName", mcpName);
            }
            payload.put("timestamp", java.time.Instant.now().toString());

            String agentId = getAgentId();
            String response = post("/api/v1/sdk-api/agents/" + agentId + "/mcp-usage", payload.toString());
            logger.debug("Recorded MCP tool usage: server={}, tool={}", serverId, toolName);
            return objectMapper.readValue(response, new TypeReference<Map<String, Object>>() {});
        } catch (Exception e) {
            // Don't fail if usage recording fails
            logger.warn("Failed to record MCP tool usage: {}", e.getMessage());
            Map<String, Object> result = new HashMap<>();
            result.put("success", false);
            result.put("error", e.getMessage());
            return result;
        }
    }

    /**
     * Submit cryptographic attestation for an MCP server.
     * Uses challenge-response mechanism for proof of key possession.
     *
     * @param mcpServerId       MCP server ID
     * @param mcpUrl            URL of the MCP server
     * @param mcpName           Name of the MCP server
     * @param capabilitiesFound List of capabilities discovered
     * @param latencyMs         Connection latency in milliseconds
     * @return Map containing attestation result
     */
    public Map<String, Object> attestMcp(String mcpServerId, String mcpUrl, String mcpName,
                                         List<String> capabilitiesFound, double latencyMs) {
        try {
            String agentId = getAgentId();
            String timestamp = java.time.Instant.now().toString();

            // Step 1: Get challenge from server (proof of key possession)
            String challenge = null;
            try {
                String challengeUrl = "/api/v1/mcp-servers/" + mcpServerId + "/challenge?agent_id=" + agentId;
                String challengeResponse = get(challengeUrl);
                JsonNode challengeJson = objectMapper.readTree(challengeResponse);
                if (challengeJson.has("challenge")) {
                    challenge = challengeJson.get("challenge").asText();
                    logger.info("Obtained attestation challenge for MCP server: {}", mcpServerId);
                }
            } catch (Exception e) {
                logger.warn("Could not get challenge, proceeding without: {}", e.getMessage());
                // Continue without challenge for backward compatibility
            }

            // Step 2: Build attestation payload with sorted keys (alphabetical order)
            // Backend expects camelCase keys: agentId, capabilitiesFound, challenge, connectionLatencyMs, etc.
            // TreeMap sorts keys alphabetically, matching backend's canonical JSON
            Map<String, Object> attestationData = new TreeMap<>();  // TreeMap for sorted keys
            attestationData.put("agentId", agentId);
            attestationData.put("capabilitiesFound", capabilitiesFound != null ? capabilitiesFound : Collections.emptyList());
            if (challenge != null) {
                attestationData.put("challenge", challenge);  // Include challenge for proof of key possession
            }
            attestationData.put("connectionLatencyMs", latencyMs);
            attestationData.put("connectionSuccessful", true);
            attestationData.put("healthCheckPassed", true);
            attestationData.put("mcpName", mcpName);
            attestationData.put("mcpUrl", mcpUrl);
            attestationData.put("sdkVersion", "1.0.0");  // SDK version for attestation
            attestationData.put("timestamp", timestamp);

            // Step 3: Create canonical JSON matching backend format EXACTLY
            // Backend uses compact JSON with no spaces: {"key1":"value1","key2":"value2"}
            String canonicalJson = createCompactJson(attestationData);
            String signature = sign(canonicalJson);
            logger.debug("Attestation canonical JSON: {}", canonicalJson);

            // Step 4: Build request payload
            ObjectNode request = objectMapper.createObjectNode();
            request.set("attestation", objectMapper.valueToTree(attestationData));
            request.put("signature", signature);

            String response = post("/api/v1/mcp-servers/" + mcpServerId + "/attest", request.toString());
            logger.info("Submitted attestation for MCP server: {} ({})", mcpName, mcpServerId);
            return objectMapper.readValue(response, new TypeReference<Map<String, Object>>() {});
        } catch (Exception e) {
            throw new AIMException("Failed to attest MCP server: " + e.getMessage(), e);
        }
    }

    /**
     * Create compact JSON matching Python's format: json.dumps(data, sort_keys=True, separators=(',', ':'))
     * This is used for attestation signing - NO SPACES after separators.
     */
    private String createCompactJson(Map<String, Object> map) {
        StringBuilder sb = new StringBuilder();
        sb.append("{");
        boolean first = true;
        // TreeMap already provides sorted keys
        for (Map.Entry<String, Object> entry : map.entrySet()) {
            if (!first) {
                sb.append(",");  // Python separator: ',' (no space!)
            }
            first = false;
            sb.append("\"").append(entry.getKey()).append("\":");  // Python separator: ':' (no space!)
            sb.append(valueToCompactJson(entry.getValue()));
        }
        sb.append("}");
        return sb.toString();
    }

    /**
     * Convert a value to compact JSON string format (no spaces).
     */
    @SuppressWarnings("unchecked")
    private String valueToCompactJson(Object value) {
        if (value == null) {
            return "null";
        } else if (value instanceof String) {
            return "\"" + escapeJsonString((String) value) + "\"";
        } else if (value instanceof Number || value instanceof Boolean) {
            return value.toString();
        } else if (value instanceof Map) {
            // Recursively handle nested maps (sort keys)
            Map<String, Object> sortedMap = new TreeMap<>((Map<String, Object>) value);
            return createCompactJson(sortedMap);
        } else if (value instanceof List) {
            List<?> list = (List<?>) value;
            StringBuilder sb = new StringBuilder("[");
            for (int i = 0; i < list.size(); i++) {
                if (i > 0) sb.append(",");  // No space!
                sb.append(valueToCompactJson(list.get(i)));
            }
            sb.append("]");
            return sb.toString();
        } else {
            // Fallback: use ObjectMapper for complex objects
            try {
                return objectMapper.writeValueAsString(value);
            } catch (Exception e) {
                return "\"" + value.toString() + "\"";
            }
        }
    }

    /**
     * List all MCP servers registered for this agent.
     *
     * @param limit Maximum number of servers to return
     * @return List of MCP server details
     */
    public List<Map<String, Object>> listMcpServers(int limit) {
        try {
            String agentId = getAgentId();
            String response = get("/api/v1/agents/" + agentId + "/mcp-servers?limit=" + limit);
            Map<String, Object> result = objectMapper.readValue(response, new TypeReference<Map<String, Object>>() {});
            return (List<Map<String, Object>>) result.getOrDefault("servers", Collections.emptyList());
        } catch (Exception e) {
            throw new AIMException("Failed to list MCP servers: " + e.getMessage(), e);
        }
    }

    /**
     * Get the agent ID (fetches if not cached).
     */
    public String getAgentId() {
        if (agentId != null && !agentId.isEmpty()) {
            return agentId;
        }
        // Try to get from agent details
        try {
            Map<String, Object> details = getAgentDetails();
            String id = (String) details.get("id");
            if (id == null) {
                id = (String) details.get("agentId");
            }
            return id;
        } catch (Exception e) {
            throw new AIMException("Cannot determine agent ID", e);
        }
    }

    /**
     * Create deterministic JSON matching Python's format.
     * Python uses: json.dumps(payload, sort_keys=True, separators=(', ', ': '))
     * This produces: {"key1": value1, "key2": value2} with specific spacing.
     */
    private String createDeterministicJson(Map<String, Object> map) {
        StringBuilder sb = new StringBuilder();
        sb.append("{");
        boolean first = true;
        // TreeMap already provides sorted keys
        for (Map.Entry<String, Object> entry : map.entrySet()) {
            if (!first) {
                sb.append(", ");  // Python separator: ', '
            }
            first = false;
            sb.append("\"").append(entry.getKey()).append("\": ");  // Python separator: ': '
            sb.append(valueToJson(entry.getValue()));
        }
        sb.append("}");
        return sb.toString();
    }

    /**
     * Convert a value to JSON string format.
     */
    @SuppressWarnings("unchecked")
    private String valueToJson(Object value) {
        if (value == null) {
            return "null";
        } else if (value instanceof String) {
            return "\"" + escapeJsonString((String) value) + "\"";
        } else if (value instanceof Number || value instanceof Boolean) {
            return value.toString();
        } else if (value instanceof Map) {
            // Recursively handle nested maps (sort keys)
            Map<String, Object> sortedMap = new TreeMap<>((Map<String, Object>) value);
            return createDeterministicJson(sortedMap);
        } else if (value instanceof List) {
            List<?> list = (List<?>) value;
            StringBuilder sb = new StringBuilder("[");
            for (int i = 0; i < list.size(); i++) {
                if (i > 0) sb.append(", ");
                sb.append(valueToJson(list.get(i)));
            }
            sb.append("]");
            return sb.toString();
        } else {
            // Fallback: use ObjectMapper for complex objects
            try {
                return objectMapper.writeValueAsString(value);
            } catch (Exception e) {
                return "\"" + value.toString() + "\"";
            }
        }
    }

    /**
     * Escape special characters in JSON strings.
     */
    private String escapeJsonString(String s) {
        return s.replace("\\", "\\\\")
                .replace("\"", "\\\"")
                .replace("\n", "\\n")
                .replace("\r", "\\r")
                .replace("\t", "\\t");
    }

    /**
     * Sign data using Ed25519.
     */
    private String sign(String data) {
        try {
            Ed25519PrivateKeyParameters privateKeyParams = new Ed25519PrivateKeyParameters(privateKey, 0);
            Ed25519Signer signer = new Ed25519Signer();
            signer.init(true, privateKeyParams);
            byte[] dataBytes = data.getBytes(StandardCharsets.UTF_8);
            signer.update(dataBytes, 0, dataBytes.length);
            byte[] signature = signer.generateSignature();
            return Base64.getEncoder().encodeToString(signature);
        } catch (Exception e) {
            throw new AIMException("Failed to sign data: " + e.getMessage(), e);
        }
    }

    /**
     * Make a GET request to AIM.
     * Auth header and retry are handled by interceptors automatically.
     */
    private String get(String path) throws IOException {
        Request request = new Request.Builder()
                .url(aimUrl + path)
                .get()
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

    /**
     * Make a POST request to AIM.
     * Auth header and retry are handled by interceptors automatically.
     */
    private String post(String path, String json) throws IOException {
        RequestBody body = RequestBody.create(json, JSON);

        Request request = new Request.Builder()
                .url(aimUrl + path)
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

    public String getAgentName() {
        return agentName;
    }

    public String getAimUrl() {
        return aimUrl;
    }

    public AgentType getAgentType() {
        return agentType;
    }

    // ========================================================================
    // CAPABILITY MANAGEMENT
    // ========================================================================

    /**
     * Request an additional capability for the agent.
     * When an agent needs a capability that wasn't granted during registration,
     * it can request it through this method. The request will be sent to admins for approval.
     *
     * @param capabilityType Type of capability being requested (e.g., "write_database")
     * @param reason Business justification for the capability request (minimum 10 characters)
     * @param metadata Optional additional context for the request
     * @return Map containing request details (id, status, etc.)
     */
    public Map<String, Object> requestCapability(String capabilityType, String reason, Map<String, Object> metadata) {
        if (capabilityType == null || capabilityType.isEmpty()) {
            throw new ConfigurationException("capabilityType must be a non-empty string");
        }
        if (reason == null || reason.length() < 10) {
            throw new ConfigurationException("reason must be at least 10 characters");
        }

        try {
            ObjectNode payload = objectMapper.createObjectNode();
            payload.put("capabilityType", capabilityType);
            payload.put("reason", reason);
            if (metadata != null && !metadata.isEmpty()) {
                payload.set("metadata", objectMapper.valueToTree(metadata));
            }

            String agentId = getAgentId();
            String url = "/api/v1/sdk-api/agents/" + agentId + "/capability-requests";
            String response = post(url, payload.toString());
            return objectMapper.readValue(response, new TypeReference<Map<String, Object>>() {});
        } catch (AIMException e) {
            // Handle 409 Conflict - capability request already exists
            if (e.getMessage().contains("409")) {
                logger.info("Capability request for '{}' already exists", capabilityType);
                Map<String, Object> result = new HashMap<>();
                result.put("status", "already_exists");
                result.put("message", "A capability request for '" + capabilityType + "' already exists");
                result.put("capabilityType", capabilityType);
                return result;
            }
            throw e;
        } catch (Exception e) {
            throw new AIMException("Failed to request capability: " + e.getMessage(), e);
        }
    }

    /**
     * Register a capability that this agent intends to use.
     * Different from requestCapability - declares intended capabilities for visibility/tracking.
     * In MONITORING mode: Capabilities are automatically granted.
     * In STRICT mode: Creates a pending request requiring admin approval.
     *
     * @param capabilityType The capability to register (e.g., "weather:check", "db:read")
     * @param description Optional description of what this capability does
     * @param riskLevel Risk level ("low", "medium", "high", "critical")
     * @return Map containing registration result
     */
    public Map<String, Object> registerCapability(String capabilityType, String description, String riskLevel) {
        if (capabilityType == null || capabilityType.isEmpty()) {
            throw new ConfigurationException("capabilityType must be a non-empty string");
        }
        if (riskLevel == null) {
            riskLevel = "medium";
        }

        try {
            ObjectNode payload = objectMapper.createObjectNode();
            payload.put("capabilityType", capabilityType);
            payload.put("description", description != null ? description : "Registered capability: " + capabilityType);
            payload.put("riskLevel", riskLevel);

            String agentId = getAgentId();
            String url = "/api/v1/sdk-api/agents/" + agentId + "/capabilities/register";
            String response = post(url, payload.toString());
            Map<String, Object> result = objectMapper.readValue(response, new TypeReference<Map<String, Object>>() {});

            // Build response
            Map<String, Object> returnResult = new HashMap<>();
            returnResult.put("success", true);
            returnResult.put("capabilityType", capabilityType);
            returnResult.put("status", result.getOrDefault("status", "registered"));
            returnResult.put("message", result.getOrDefault("message", "Capability '" + capabilityType + "' registered"));
            if (result.containsKey("requestId")) {
                returnResult.put("requestId", result.get("requestId"));
            }
            return returnResult;
        } catch (AIMException e) {
            // Handle 409 or already exists
            if (e.getMessage().contains("409") || e.getMessage().toLowerCase().contains("already")) {
                Map<String, Object> result = new HashMap<>();
                result.put("success", true);
                result.put("capabilityType", capabilityType);
                result.put("status", "already_exists");
                result.put("message", "Capability '" + capabilityType + "' already exists");
                return result;
            }
            // Handle 404 - endpoint not available
            if (e.getMessage().contains("404")) {
                Map<String, Object> result = new HashMap<>();
                result.put("success", true);
                result.put("capabilityType", capabilityType);
                result.put("status", "not_tracked");
                result.put("message", "Capability registration not supported by server");
                return result;
            }
            throw e;
        } catch (Exception e) {
            logger.warn("Failed to register capability '{}': {}", capabilityType, e.getMessage());
            Map<String, Object> result = new HashMap<>();
            result.put("success", false);
            result.put("capabilityType", capabilityType);
            result.put("status", "error");
            result.put("message", e.getMessage());
            return result;
        }
    }

    // ========================================================================
    // JIT (JUST-IN-TIME) APPROVAL
    // ========================================================================

    /**
     * Wait for JIT (Just-in-Time) approval.
     * Polls the AIM server for verification approval.
     *
     * @param verificationId ID of the verification request
     * @param timeoutSeconds Maximum time to wait for approval
     * @return VerificationResult when approved
     * @throws ActionDeniedException if action is denied
     * @throws VerificationException if timeout or polling fails
     */
    public VerificationResult waitForApproval(String verificationId, int timeoutSeconds) {
        return waitForApproval(verificationId, timeoutSeconds, null, null);
    }

    /**
     * Wait for JIT (Just-in-Time) approval with capability/resource info.
     * Polls the AIM server for verification approval.
     *
     * @param verificationId ID of the verification request
     * @param timeoutSeconds Maximum time to wait for approval
     * @param capability     The capability being verified (for result)
     * @param resource       The resource being accessed (for result)
     * @return VerificationResult when approved
     * @throws ActionDeniedException if action is denied
     * @throws VerificationException if timeout or polling fails
     */
    private VerificationResult waitForApproval(String verificationId, int timeoutSeconds,
                                               String capability, String resource) {
        long startTime = System.currentTimeMillis();
        long pollInterval = 2000; // Start with 2 second polls
        long maxPollInterval = 10000; // Max 10 seconds between polls

        while (System.currentTimeMillis() - startTime < timeoutSeconds * 1000L) {
            try {
                String url = "/api/v1/sdk-api/verifications/" + verificationId;
                String response = get(url);
                Map<String, Object> result = objectMapper.readValue(response, new TypeReference<Map<String, Object>>() {});

                String status = (String) result.get("status");
                String enforcementMode = result.containsKey("enforcementMode") ? (String) result.get("enforcementMode") : "monitoring";
                if ("approved".equalsIgnoreCase(status) || "auto-approved".equalsIgnoreCase(status) || "verified".equalsIgnoreCase(status)) {
                    logger.info("JIT approval granted for verification {} (status: {})", verificationId, status);
                    return VerificationResult.builder()
                            .verified(true)
                            .status(status)
                            .verificationId(verificationId)
                            .capability(capability)
                            .resource(resource)
                            .enforcementMode(enforcementMode)
                            .timestamp(Instant.now())
                            .metadata(result.get("constraints") != null ?
                                objectMapper.convertValue(result.get("constraints"), new TypeReference<Map<String, Object>>() {}) : null)
                            .build();
                }

                if ("denied".equalsIgnoreCase(status) || "rejected".equalsIgnoreCase(status)) {
                    String message = (String) result.getOrDefault("message", "Action denied by administrator");
                    logger.warn("JIT approval denied for verification {}: {}", verificationId, message);
                    throw new ActionDeniedException(message, capability, resource);
                }

                // Still pending, continue polling
                logger.debug("Waiting for JIT approval (status: {})", status);
                Thread.sleep(pollInterval);
                pollInterval = Math.min((long) (pollInterval * 1.5), maxPollInterval);

            } catch (ActionDeniedException e) {
                throw e;
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
                throw new VerificationException("Interrupted while waiting for approval", e);
            } catch (Exception e) {
                logger.warn("Error polling verification status: {}", e.getMessage());
                try {
                    Thread.sleep(pollInterval);
                } catch (InterruptedException ie) {
                    Thread.currentThread().interrupt();
                    throw new VerificationException("Interrupted while waiting for approval", ie);
                }
                pollInterval = Math.min((long) (pollInterval * 1.5), maxPollInterval);
            }
        }

        throw new VerificationException("Timeout waiting for JIT approval after " + timeoutSeconds + " seconds");
    }

    // ========================================================================
    // AGENT MANAGEMENT
    // ========================================================================

    /**
     * List agents in the user's organization.
     *
     * @param limit Maximum number of agents to return (default 50, max 100)
     * @param offset Pagination offset
     * @param status Filter by status ("pending", "verified", "denied")
     * @param agentType Filter by agent type
     * @return Map containing agents list and pagination info
     */
    public Map<String, Object> listAgents(int limit, int offset, String status, AgentType agentType) {
        try {
            StringBuilder queryParams = new StringBuilder();
            queryParams.append("limit=").append(Math.min(limit, 100));
            queryParams.append("&offset=").append(offset);
            if (status != null && !status.isEmpty()) {
                queryParams.append("&status=").append(status);
            }
            if (agentType != null) {
                queryParams.append("&agentType=").append(agentType.getValue());
            }

            String url = "/api/v1/agents?" + queryParams;
            String response = get(url);
            return objectMapper.readValue(response, new TypeReference<Map<String, Object>>() {});
        } catch (Exception e) {
            throw new AIMException("Failed to list agents: " + e.getMessage(), e);
        }
    }

    /**
     * List agents with default parameters.
     */
    public Map<String, Object> listAgents() {
        return listAgents(50, 0, null, null);
    }

    /**
     * Update an agent's details.
     *
     * @param targetAgentId Agent ID to update (defaults to current agent)
     * @param displayName New display name
     * @param description New description
     * @param version New version
     * @param repositoryUrl New repository URL
     * @param documentationUrl New documentation URL
     * @return Map containing updated agent details
     */
    public Map<String, Object> updateAgent(String targetAgentId, String displayName, String description,
                                           String version, String repositoryUrl, String documentationUrl) {
        if (targetAgentId == null || targetAgentId.isEmpty()) {
            targetAgentId = getAgentId();
        }

        ObjectNode payload = objectMapper.createObjectNode();
        if (displayName != null) payload.put("displayName", displayName);
        if (description != null) payload.put("description", description);
        if (version != null) payload.put("version", version);
        if (repositoryUrl != null) payload.put("repositoryUrl", repositoryUrl);
        if (documentationUrl != null) payload.put("documentationUrl", documentationUrl);

        if (payload.isEmpty()) {
            throw new ConfigurationException("At least one field must be provided for update");
        }

        try {
            String url = "/api/v1/agents/" + targetAgentId;
            String response = put(url, payload.toString());
            return objectMapper.readValue(response, new TypeReference<Map<String, Object>>() {});
        } catch (Exception e) {
            throw new AIMException("Failed to update agent: " + e.getMessage(), e);
        }
    }

    /**
     * Delete/deactivate an agent (soft delete).
     * WARNING: This action cannot be easily undone.
     *
     * @param targetAgentId Agent ID to delete (cannot delete current agent)
     * @return Map containing success/message
     */
    public Map<String, Object> deleteAgent(String targetAgentId) {
        String currentAgentId = getAgentId();
        if (targetAgentId.equals(currentAgentId)) {
            throw new ConfigurationException("Cannot delete the currently authenticated agent");
        }

        try {
            String url = "/api/v1/agents/" + targetAgentId;
            String response = delete(url);
            return objectMapper.readValue(response, new TypeReference<Map<String, Object>>() {});
        } catch (Exception e) {
            throw new AIMException("Failed to delete agent: " + e.getMessage(), e);
        }
    }

    // ========================================================================
    // ADDITIONAL SDK FEATURES
    // ========================================================================

    /**
     * Create/register a new agent through an authenticated client.
     * This allows an authenticated user to create a new agent programmatically.
     *
     * @param name           Agent name (required, must be unique within organization)
     * @param displayName    Human-readable display name (optional)
     * @param description    Agent description (optional)
     * @param agentType      Type of agent (optional, defaults to CUSTOM)
     * @param version        Agent version (optional)
     * @param capabilities   List of initial capabilities (optional)
     * @param mcpServers     List of MCP servers this agent uses (optional)
     * @return Map containing new agent details including agentId
     */
    public Map<String, Object> createNewAgent(String name, String displayName, String description,
                                              AgentType agentType, String version,
                                              List<String> capabilities, List<String> mcpServers) {
        if (name == null || name.isEmpty()) {
            throw new ConfigurationException("Agent name is required");
        }

        try {
            ObjectNode payload = objectMapper.createObjectNode();
            payload.put("name", name);
            payload.put("displayName", displayName != null ? displayName : name);
            if (description != null) payload.put("description", description);
            payload.put("agentType", agentType != null ? agentType.getValue() : AgentType.CUSTOM.getValue());
            if (version != null) payload.put("version", version);

            if (capabilities != null && !capabilities.isEmpty()) {
                var capsArray = payload.putArray("capabilities");
                for (String cap : capabilities) {
                    capsArray.add(cap);
                }
            }

            if (mcpServers != null && !mcpServers.isEmpty()) {
                var mcpArray = payload.putArray("talksTo");
                for (String mcp : mcpServers) {
                    mcpArray.add(mcp);
                }
            }

            String response = post("/api/v1/agents", payload.toString());
            return objectMapper.readValue(response, new TypeReference<Map<String, Object>>() {});
        } catch (Exception e) {
            throw new AIMException("Failed to create new agent: " + e.getMessage(), e);
        }
    }

    /**
     * Report capabilities to AIM in bulk.
     * Useful for declaring multiple capabilities at once.
     *
     * @param capabilities List of capability types to report
     * @param scope        Optional scope information (source, detectedAt, etc.)
     * @return Map with granted count and total count
     */
    public Map<String, Object> reportCapabilities(List<String> capabilities, Map<String, Object> scope) {
        if (capabilities == null || capabilities.isEmpty()) {
            Map<String, Object> result = new HashMap<>();
            result.put("granted", 0);
            result.put("total", 0);
            return result;
        }

        int grantedCount = 0;
        int totalCount = capabilities.size();

        for (String capabilityType : capabilities) {
            try {
                ObjectNode payload = objectMapper.createObjectNode();
                payload.put("capabilityType", capabilityType);
                if (scope != null) {
                    payload.set("scope", objectMapper.valueToTree(scope));
                } else {
                    ObjectNode defaultScope = objectMapper.createObjectNode();
                    defaultScope.put("source", "java_sdk_auto_detection");
                    defaultScope.put("detectedAt", Instant.now().toString());
                    payload.set("scope", defaultScope);
                }

                String agentId = getAgentId();
                post("/api/v1/sdk-api/agents/" + agentId + "/capabilities", payload.toString());
                grantedCount++;
            } catch (Exception e) {
                // Capability might already exist - count as granted if duplicate
                String errorStr = e.getMessage().toLowerCase();
                if (errorStr.contains("duplicate") || errorStr.contains("already exists") ||
                    errorStr.contains("unique constraint") || errorStr.contains("500")) {
                    grantedCount++;
                }
                // Continue even if one capability fails
            }
        }

        Map<String, Object> result = new HashMap<>();
        result.put("granted", grantedCount);
        result.put("total", totalCount);
        return result;
    }

    /**
     * Report SDK integration status to AIM dashboard.
     * Updates the Detection tab to show SDK is installed and integrated.
     *
     * @param sdkVersion   SDK version string (e.g., "aim-sdk-java@1.0.0")
     * @param platform     Platform/language (e.g., "java", "python", "go")
     * @param capabilities Optional list of SDK capabilities enabled
     * @return Map with success status and message
     */
    public Map<String, Object> reportSdkIntegration(String sdkVersion, String platform,
                                                     List<String> capabilities) {
        try {
            ObjectNode detectionEvent = objectMapper.createObjectNode();
            detectionEvent.put("mcpServer", "aim-sdk-integration");
            detectionEvent.put("detectionMethod", "sdk_integration");
            detectionEvent.put("confidence", 100.0);
            detectionEvent.put("sdkVersion", sdkVersion != null ? sdkVersion : "aim-sdk-java@1.0.0");
            detectionEvent.put("timestamp", Instant.now().toString());

            ObjectNode details = objectMapper.createObjectNode();
            details.put("platform", platform != null ? platform : "java");
            details.put("integrated", true);
            if (capabilities != null) {
                var capsArray = details.putArray("capabilities");
                for (String cap : capabilities) {
                    capsArray.add(cap);
                }
            }
            detectionEvent.set("details", details);

            ObjectNode payload = objectMapper.createObjectNode();
            var detectionsArray = payload.putArray("detections");
            detectionsArray.add(detectionEvent);

            String agentId = getAgentId();
            String response = post("/api/v1/sdk-api/agents/" + agentId + "/detection/report", payload.toString());
            return objectMapper.readValue(response, new TypeReference<Map<String, Object>>() {});
        } catch (Exception e) {
            throw new AIMException("Failed to report SDK integration: " + e.getMessage(), e);
        }
    }

    /**
     * Report MCP server detections to AIM.
     * Used for auto-detection of MCP servers by the SDK.
     *
     * @param detections List of detection events, each containing:
     *                   - mcpServer: Server name or ID
     *                   - detectionMethod: How it was detected
     *                   - confidence: Detection confidence (0-100)
     *                   - sdkVersion: SDK version
     * @return Map with detectionsProcessed count
     */
    public Map<String, Object> reportDetections(List<Map<String, Object>> detections) {
        if (detections == null || detections.isEmpty()) {
            Map<String, Object> result = new HashMap<>();
            result.put("detectionsProcessed", 0);
            return result;
        }

        try {
            // Add timestamps if not present
            for (Map<String, Object> detection : detections) {
                if (!detection.containsKey("timestamp")) {
                    detection.put("timestamp", Instant.now().toString());
                }
                if (!detection.containsKey("sdkVersion")) {
                    detection.put("sdkVersion", "aim-sdk-java@1.0.0");
                }
            }

            ObjectNode payload = objectMapper.createObjectNode();
            payload.set("detections", objectMapper.valueToTree(detections));

            String agentId = getAgentId();
            String response = post("/api/v1/sdk-api/agents/" + agentId + "/detection/report", payload.toString());
            return objectMapper.readValue(response, new TypeReference<Map<String, Object>>() {});
        } catch (Exception e) {
            throw new AIMException("Failed to report detections: " + e.getMessage(), e);
        }
    }

    /**
     * Get the SDK version string.
     *
     * @return SDK version string
     */
    public static String getSdkVersion() {
        return "aim-sdk-java@1.0.0";
    }

    /**
     * Make a PUT request to AIM.
     */
    private String put(String path, String json) throws IOException {
        RequestBody body = RequestBody.create(json, JSON);
        Request request = new Request.Builder()
                .url(aimUrl + path)
                .put(body)
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

    /**
     * Make a DELETE request to AIM.
     */
    private String delete(String path) throws IOException {
        Request request = new Request.Builder()
                .url(aimUrl + path)
                .delete()
                .build();

        try (Response response = httpClient.newCall(request).execute()) {
            if (!response.isSuccessful()) {
                String errorBody = response.body() != null ? response.body().string() : "";
                throw new AIMException("Request failed: " + response.code() + " - " + errorBody,
                        "HTTP_ERROR", response.code());
            }
            return response.body() != null ? response.body().string() : "{\"success\": true}";
        }
    }

    @Override
    public void close() {
        httpClient.dispatcher().executorService().shutdown();
        httpClient.connectionPool().evictAll();
        authClient.dispatcher().executorService().shutdown();
        authClient.connectionPool().evictAll();
    }

    /**
     * Builder for AIMClient.
     */
    public static class Builder {
        private String agentId;
        private String agentName;
        private String aimUrl = "http://localhost:8080";
        private AgentType agentType = AgentType.CUSTOM;
        private List<String> capabilities = new ArrayList<>();
        private List<String> talksTo = new ArrayList<>(); // MCP servers
        private Map<String, String> mcpCommands = new HashMap<>(); // MCP server name to command
        private List<String> tags = new ArrayList<>(); // Tags for categorization
        private String description;
        private Map<String, Object> metadata = new HashMap<>();
        private String clientId;
        private String clientSecret;
        private String organizationId;
        private String refreshToken;
        private String sdkTokenId;
        private Map<String, String> credentials;

        public Builder agentId(String agentId) {
            this.agentId = agentId;
            return this;
        }

        public Builder agentName(String agentName) {
            this.agentName = agentName;
            return this;
        }

        public Builder aimUrl(String aimUrl) {
            this.aimUrl = aimUrl;
            return this;
        }

        public Builder agentType(AgentType agentType) {
            this.agentType = agentType;
            return this;
        }

        public Builder capabilities(List<String> capabilities) {
            this.capabilities = capabilities;
            return this;
        }

        public Builder addCapability(String capability) {
            this.capabilities.add(capability);
            return this;
        }

        public Builder talksTo(List<String> talksTo) {
            this.talksTo = talksTo;
            return this;
        }

        public Builder addTalksTo(String mcpServer) {
            this.talksTo.add(mcpServer);
            return this;
        }

        public Builder mcpCommands(Map<String, String> mcpCommands) {
            this.mcpCommands = new HashMap<>(mcpCommands);
            return this;
        }

        public Builder addMcpCommand(String serverName, String command) {
            this.mcpCommands.put(serverName, command);
            return this;
        }

        public Builder tags(List<String> tags) {
            this.tags = tags;
            return this;
        }

        public Builder addTag(String tag) {
            this.tags.add(tag);
            return this;
        }

        public Builder description(String description) {
            this.description = description;
            return this;
        }

        public Builder metadata(Map<String, Object> metadata) {
            this.metadata = metadata;
            return this;
        }

        public Builder addMetadata(String key, Object value) {
            this.metadata.put(key, value);
            return this;
        }

        public Builder clientId(String clientId) {
            this.clientId = clientId;
            return this;
        }

        public Builder clientSecret(String clientSecret) {
            this.clientSecret = clientSecret;
            return this;
        }

        public Builder organizationId(String organizationId) {
            this.organizationId = organizationId;
            return this;
        }

        public Builder refreshToken(String refreshToken) {
            this.refreshToken = refreshToken;
            return this;
        }

        public Builder sdkTokenId(String sdkTokenId) {
            this.sdkTokenId = sdkTokenId;
            return this;
        }

        public Builder credentials(Map<String, String> credentials) {
            this.credentials = new HashMap<>(credentials);
            return this;
        }

        public AIMClient build() {
            return new AIMClient(this);
        }
    }
}
