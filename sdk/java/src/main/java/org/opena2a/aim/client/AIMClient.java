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
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.time.Instant;
import java.util.*;
import java.util.concurrent.TimeUnit;
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

    private final String agentId;
    private final String agentName;
    private final String aimUrl;
    private final AgentType agentType;
    private final List<String> capabilities;
    private final OkHttpClient httpClient;
    private final ObjectMapper objectMapper;

    private String accessToken;
    private Instant tokenExpiry;
    private byte[] privateKey;
    private byte[] publicKey;

    // OAuth credentials (from SDK download)
    private String clientId;
    private String clientSecret;
    private String organizationId;

    // SDK OAuth credentials (refresh token flow)
    private String refreshToken;
    private String sdkTokenId;
    private Map<String, String> credentials;

    private AIMClient(Builder builder) {
        this.agentName = builder.agentName;
        this.aimUrl = builder.aimUrl;
        this.agentType = builder.agentType;
        this.capabilities = builder.capabilities;
        this.agentId = builder.agentId;
        this.clientId = builder.clientId;
        this.clientSecret = builder.clientSecret;
        this.organizationId = builder.organizationId;
        this.refreshToken = builder.refreshToken;
        this.sdkTokenId = builder.sdkTokenId;
        this.credentials = builder.credentials;

        this.objectMapper = new ObjectMapper();
        this.objectMapper.registerModule(new JavaTimeModule());

        this.httpClient = new OkHttpClient.Builder()
                .connectTimeout(30, TimeUnit.SECONDS)
                .readTimeout(30, TimeUnit.SECONDS)
                .writeTimeout(30, TimeUnit.SECONDS)
                .build();
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
     * Register this agent with AIM.
     */
    private void registerAgent() {
        try {
            // First, authenticate via OAuth
            authenticate();

            // Generate Ed25519 key pair
            generateKeyPair();

            // Register with AIM backend
            ObjectNode payload = objectMapper.createObjectNode();
            payload.put("name", agentName);
            payload.put("publicKey", Base64.getEncoder().encodeToString(publicKey));
            payload.put("agentType", agentType.getValue());

            if (!capabilities.isEmpty()) {
                var capArray = payload.putArray("capabilities");
                for (String cap : capabilities) {
                    capArray.add(cap);
                }
            }

            String response = post("/api/v1/agents/register", payload.toString());
            JsonNode result = objectMapper.readTree(response);

            // Store agent ID if returned
            if (result.has("id")) {
                // Agent registered successfully
                logger.info("Agent registered: {} ({})", agentName, result.get("id").asText());
            }

        } catch (Exception e) {
            throw new AIMException("Failed to register agent: " + e.getMessage(), e);
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

            try (Response response = httpClient.newCall(request).execute()) {
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

            try (Response response = httpClient.newCall(request).execute()) {
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
        return verifyCapability(capability, resource, RiskLevel.MEDIUM, null);
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
        try {
            ensureAuthenticated();

            ObjectNode payload = objectMapper.createObjectNode();
            payload.put("capability", capability);
            payload.put("resource", resource);
            payload.put("riskLevel", riskLevel.getValue());
            payload.put("timestamp", Instant.now().toString());

            // Sign the request
            String signature = sign(payload.toString());
            payload.put("signature", signature);

            if (context != null) {
                payload.set("context", objectMapper.valueToTree(context));
            }

            String response = post("/api/v1/agents/verify", payload.toString());
            JsonNode result = objectMapper.readTree(response);

            boolean verified = result.has("verified") && result.get("verified").asBoolean();
            String status = result.has("status") ? result.get("status").asText() : "unknown";

            return VerificationResult.builder()
                    .verified(verified)
                    .status(status)
                    .verificationId(result.has("verificationId") ? result.get("verificationId").asText() : null)
                    .capability(capability)
                    .resource(resource)
                    .timestamp(Instant.now())
                    .build();

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
        VerificationResult result = verifyCapability(capability, resource, riskLevel, null);

        if (!result.isVerified()) {
            throw new ActionDeniedException(
                    "Action denied: " + capability + " on " + resource,
                    capability,
                    resource
            );
        }

        return action.get();
    }

    /**
     * Get agent details from AIM.
     *
     * @return Map containing agent details
     */
    public Map<String, Object> getAgentDetails() {
        try {
            ensureAuthenticated();
            String response = get("/api/v1/agents/me");
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
            ensureAuthenticated();

            ObjectNode payload = objectMapper.createObjectNode();
            payload.put("capabilityType", capabilityType);
            payload.put("reason", reason);

            String response = post("/api/v1/capabilities/request", payload.toString());
            return objectMapper.readValue(response, new TypeReference<Map<String, Object>>() {});
        } catch (Exception e) {
            throw new AIMException("Failed to request capability: " + e.getMessage(), e);
        }
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
     * Ensure we have a valid access token.
     */
    private void ensureAuthenticated() {
        if (accessToken == null || Instant.now().isAfter(tokenExpiry)) {
            authenticate();
        }
    }

    /**
     * Make a GET request to AIM.
     */
    private String get(String path) throws IOException {
        Request request = new Request.Builder()
                .url(aimUrl + path)
                .header("Authorization", "Bearer " + accessToken)
                .get()
                .build();

        try (Response response = httpClient.newCall(request).execute()) {
            if (!response.isSuccessful()) {
                throw new AIMException("Request failed: " + response.code(), "HTTP_ERROR", response.code());
            }
            return response.body().string();
        }
    }

    /**
     * Make a POST request to AIM.
     */
    private String post(String path, String json) throws IOException {
        RequestBody body = RequestBody.create(json, JSON);

        Request request = new Request.Builder()
                .url(aimUrl + path)
                .header("Authorization", "Bearer " + accessToken)
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

    public String getAgentId() {
        return agentId;
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

    @Override
    public void close() {
        httpClient.dispatcher().executorService().shutdown();
        httpClient.connectionPool().evictAll();
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
