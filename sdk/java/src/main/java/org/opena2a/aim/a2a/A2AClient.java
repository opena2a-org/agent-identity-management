package org.opena2a.aim.a2a;

import com.fasterxml.jackson.core.type.TypeReference;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ObjectNode;
import com.fasterxml.jackson.datatype.jsr310.JavaTimeModule;
import okhttp3.*;
import org.bouncycastle.crypto.params.Ed25519PrivateKeyParameters;
import org.bouncycastle.crypto.signers.Ed25519Signer;
import org.opena2a.aim.client.AIMClient;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.security.SecureRandom;
import java.time.Instant;
import java.util.*;
import java.util.concurrent.TimeUnit;

/**
 * A2A Client for Agent-to-Agent protocol operations.
 *
 * <p>Provides secure agent-to-agent communication with:
 * <ul>
 *   <li>Agent card registration and discovery</li>
 *   <li>Ed25519 request signing and verification</li>
 *   <li>Trust score management</li>
 *   <li>GDPR/PSD2 compliant consent management</li>
 *   <li>Cross-agent policy evaluation</li>
 * </ul>
 *
 * <p>Example usage:</p>
 * <pre>{@code
 * AIMClient aimClient = AIMClient.secure("my-agent");
 * A2AClient a2a = new A2AClient(aimClient);
 *
 * // Register agent card
 * A2AAgentCard card = a2a.registerAgentCard("https://myagent.example.com/.well-known/agent.json");
 *
 * // Sign requests for outbound A2A calls
 * A2ARequestSignature sig = a2a.signRequest("POST", "/tasks/send", requestBody);
 *
 * // Verify incoming A2A requests
 * boolean valid = a2a.verifyRequest(signature, publicKey, timestamp, nonce, method, path, body);
 *
 * // Get trust score for another agent
 * A2ATrustScore trust = a2a.getTrustScore(targetAgentId);
 * }</pre>
 */
public class A2AClient {

    private static final Logger logger = LoggerFactory.getLogger(A2AClient.class);
    private static final MediaType JSON = MediaType.get("application/json; charset=utf-8");
    private static final String A2A_BASE_PATH = "/api/v1/a2a";

    private final AIMClient aimClient;
    private final OkHttpClient httpClient;
    private final ObjectMapper objectMapper;
    private final int timeoutSeconds;

    /**
     * Create a new A2AClient using an existing AIMClient.
     *
     * @param aimClient The AIMClient to use for authentication
     */
    public A2AClient(AIMClient aimClient) {
        this(aimClient, 30);
    }

    /**
     * Create a new A2AClient with custom timeout.
     *
     * @param aimClient The AIMClient to use for authentication
     * @param timeoutSeconds Request timeout in seconds
     */
    public A2AClient(AIMClient aimClient, int timeoutSeconds) {
        this.aimClient = aimClient;
        this.timeoutSeconds = timeoutSeconds;

        this.objectMapper = new ObjectMapper();
        this.objectMapper.registerModule(new JavaTimeModule());

        this.httpClient = new OkHttpClient.Builder()
                .proxy(java.net.Proxy.NO_PROXY)
                .connectTimeout(timeoutSeconds, TimeUnit.SECONDS)
                .readTimeout(timeoutSeconds, TimeUnit.SECONDS)
                .writeTimeout(timeoutSeconds, TimeUnit.SECONDS)
                .build();
    }

    // ==================== Agent Card Operations ====================

    /**
     * Register an A2A agent card for the current agent.
     *
     * @param cardUrl URL to the agent's .well-known/agent.json
     * @return The registered agent card
     * @throws A2AException if registration fails
     */
    public A2AAgentCard registerAgentCard(String cardUrl) throws A2AException {
        try {
            ObjectNode body = objectMapper.createObjectNode();
            body.put("cardUrl", cardUrl);

            String response = post(A2A_BASE_PATH + "/cards", objectMapper.writeValueAsString(body));
            return objectMapper.readValue(response, A2AAgentCard.class);
        } catch (Exception e) {
            throw new A2AException("Failed to register agent card: " + e.getMessage(), e);
        }
    }

    /**
     * Get an agent card by agent ID.
     *
     * @param agentId The agent ID
     * @return The agent card
     * @throws A2AException if retrieval fails
     */
    public A2AAgentCard getAgentCard(String agentId) throws A2AException {
        try {
            String response = get(A2A_BASE_PATH + "/cards/" + agentId);
            return objectMapper.readValue(response, A2AAgentCard.class);
        } catch (Exception e) {
            throw new A2AException("Failed to get agent card: " + e.getMessage(), e);
        }
    }

    /**
     * Update the current agent's card.
     *
     * @param card The updated agent card
     * @return The updated agent card
     * @throws A2AException if update fails
     */
    public A2AAgentCard updateAgentCard(A2AAgentCard card) throws A2AException {
        try {
            String response = put(A2A_BASE_PATH + "/cards/" + card.getAgentId(),
                                  objectMapper.writeValueAsString(card));
            return objectMapper.readValue(response, A2AAgentCard.class);
        } catch (Exception e) {
            throw new A2AException("Failed to update agent card: " + e.getMessage(), e);
        }
    }

    /**
     * Refresh AIM attestation for an agent card.
     *
     * @param agentId The agent ID
     * @return The refreshed agent card with new attestation
     * @throws A2AException if refresh fails
     */
    public A2AAgentCard refreshAttestation(String agentId) throws A2AException {
        try {
            String response = post(A2A_BASE_PATH + "/cards/" + agentId + "/attestation", "{}");
            return objectMapper.readValue(response, A2AAgentCard.class);
        } catch (Exception e) {
            throw new A2AException("Failed to refresh attestation: " + e.getMessage(), e);
        }
    }

    // ==================== Multi-Agent Skill Attestation ====================

    /**
     * Attest to another agent's skill capability.
     *
     * <p>Creates a signed attestation that contributes to multi-agent consensus
     * verification. Skills become "verified" when they reach the consensus
     * threshold (3+ unique attesters, 2+ unique owners).</p>
     *
     * @param targetAgentId Agent whose skill is being attested
     * @param skillId ID of the skill being attested
     * @param attestationType Type of attestation (SKILL_VERIFICATION, QUALITY, SECURITY)
     * @param confidence Confidence level (0.0 to 1.0)
     * @param evidence Optional evidence supporting the attestation
     * @return The created attestation record
     * @throws A2AException if attestation fails
     */
    public Map<String, Object> attestSkill(String targetAgentId, String skillId,
                                            String attestationType, double confidence,
                                            Map<String, Object> evidence) throws A2AException {
        try {
            ObjectNode body = objectMapper.createObjectNode();
            body.put("attestedAgentId", targetAgentId);
            body.put("skillId", skillId);
            body.put("attestationType", attestationType);
            body.put("confidence", confidence);
            if (evidence != null) {
                body.set("evidence", objectMapper.valueToTree(evidence));
            }

            String response = post(A2A_BASE_PATH + "/attestations", objectMapper.writeValueAsString(body));
            return objectMapper.readValue(response, new TypeReference<Map<String, Object>>() {});
        } catch (Exception e) {
            throw new A2AException("Failed to attest skill: " + e.getMessage(), e);
        }
    }

    /**
     * Attest to another agent's skill with default parameters.
     */
    public Map<String, Object> attestSkill(String targetAgentId, String skillId) throws A2AException {
        return attestSkill(targetAgentId, skillId, "SKILL_VERIFICATION", 1.0, null);
    }

    /**
     * Get attestations for an agent's skill(s).
     *
     * @param agentId Agent to get attestations for
     * @param skillId Optional specific skill (null for all skills)
     * @return List of attestation records
     * @throws A2AException if retrieval fails
     */
    public List<Map<String, Object>> getSkillAttestations(String agentId, String skillId) throws A2AException {
        try {
            String path = A2A_BASE_PATH + "/agents/" + agentId + "/attestations";
            if (skillId != null && !skillId.isEmpty()) {
                path += "?skillId=" + skillId;
            }
            String response = get(path);
            JsonNode root = objectMapper.readTree(response);
            JsonNode attestationsNode = root.has("attestations") ? root.get("attestations") : root;
            return objectMapper.convertValue(attestationsNode, new TypeReference<List<Map<String, Object>>>() {});
        } catch (Exception e) {
            throw new A2AException("Failed to get skill attestations: " + e.getMessage(), e);
        }
    }

    /**
     * Get all attestations for an agent.
     */
    public List<Map<String, Object>> getSkillAttestations(String agentId) throws A2AException {
        return getSkillAttestations(agentId, null);
    }

    /**
     * Get the consensus verification status for a skill.
     *
     * <p>Returns whether a skill has reached the verification threshold
     * through multi-agent consensus.</p>
     *
     * @param agentId Agent that owns the skill
     * @param skillId Skill to check
     * @return Consensus status including isVerified, attestationCount, uniqueAttesters, etc.
     * @throws A2AException if retrieval fails
     */
    public A2AConsensusResult getConsensusStatus(String agentId, String skillId) throws A2AException {
        try {
            String response = get(A2A_BASE_PATH + "/agents/" + agentId + "/skills/" + skillId + "/consensus");
            return objectMapper.readValue(response, A2AConsensusResult.class);
        } catch (Exception e) {
            throw new A2AException("Failed to get consensus status: " + e.getMessage(), e);
        }
    }

    /**
     * Revoke a previously made attestation.
     *
     * @param attestationId ID of the attestation to revoke
     * @param reason Reason for revocation
     * @return The revoked attestation record
     * @throws A2AException if revocation fails
     */
    public Map<String, Object> revokeAttestation(String attestationId, String reason) throws A2AException {
        try {
            ObjectNode body = objectMapper.createObjectNode();
            body.put("reason", reason);

            String response = post(A2A_BASE_PATH + "/attestations/" + attestationId + "/revoke",
                                   objectMapper.writeValueAsString(body));
            return objectMapper.readValue(response, new TypeReference<Map<String, Object>>() {});
        } catch (Exception e) {
            throw new A2AException("Failed to revoke attestation: " + e.getMessage(), e);
        }
    }

    /**
     * List all agent cards.
     *
     * @return List of agent cards
     * @throws A2AException if listing fails
     */
    public List<A2AAgentCard> listAgentCards() throws A2AException {
        return listAgentCards(100, 0);
    }

    /**
     * List agent cards with pagination.
     *
     * @param limit Maximum number of cards to return
     * @param offset Number of cards to skip
     * @return List of agent cards
     * @throws A2AException if listing fails
     */
    public List<A2AAgentCard> listAgentCards(int limit, int offset) throws A2AException {
        try {
            String response = get(A2A_BASE_PATH + "/cards?limit=" + limit + "&offset=" + offset);
            JsonNode root = objectMapper.readTree(response);
            JsonNode cardsNode = root.has("cards") ? root.get("cards") : root;
            return objectMapper.convertValue(cardsNode, new TypeReference<List<A2AAgentCard>>() {});
        } catch (Exception e) {
            throw new A2AException("Failed to list agent cards: " + e.getMessage(), e);
        }
    }

    // ==================== Request Signing & Verification ====================

    /**
     * Sign an outbound A2A request using Ed25519.
     *
     * @param method HTTP method
     * @param path Request path
     * @param body Request body (can be null or empty for GET requests)
     * @return The request signature
     * @throws A2AException if signing fails
     */
    public A2ARequestSignature signRequest(String method, String path, String body) throws A2AException {
        try {
            ObjectNode reqBody = objectMapper.createObjectNode();
            reqBody.put("method", method);
            reqBody.put("path", path);
            if (body != null && !body.isEmpty()) {
                reqBody.put("body", body);
            }

            String response = post(A2A_BASE_PATH + "/sign", objectMapper.writeValueAsString(reqBody));
            return objectMapper.readValue(response, A2ARequestSignature.class);
        } catch (Exception e) {
            throw new A2AException("Failed to sign request: " + e.getMessage(), e);
        }
    }

    /**
     * Create a local request signature without calling the backend.
     * Requires the agent's private key.
     *
     * @param privateKey Ed25519 private key bytes
     * @param method HTTP method
     * @param path Request path
     * @param body Request body
     * @return The request signature
     * @throws A2AException if signing fails
     */
    public A2ARequestSignature signRequestLocal(byte[] privateKey, String method, String path, String body)
            throws A2AException {
        try {
            Instant timestamp = Instant.now();
            String nonce = generateNonce();

            // Build message to sign
            String message = String.format("%s\n%s\n%s\n%s\n%s",
                    method.toUpperCase(),
                    path,
                    timestamp.toEpochMilli(),
                    nonce,
                    body != null ? body : "");

            // Sign with Ed25519
            Ed25519PrivateKeyParameters privateKeyParams = new Ed25519PrivateKeyParameters(privateKey, 0);
            Ed25519Signer signer = new Ed25519Signer();
            signer.init(true, privateKeyParams);
            byte[] messageBytes = message.getBytes(StandardCharsets.UTF_8);
            signer.update(messageBytes, 0, messageBytes.length);
            byte[] signatureBytes = signer.generateSignature();

            String signature = Base64.getEncoder().encodeToString(signatureBytes);
            String publicKey = Base64.getEncoder().encodeToString(
                    privateKeyParams.generatePublicKey().getEncoded());

            // Build headers map
            Map<String, String> headers = new HashMap<>();
            headers.put("X-A2A-Signature", signature);
            headers.put("X-A2A-Timestamp", String.valueOf(timestamp.toEpochMilli()));
            headers.put("X-A2A-Nonce", nonce);
            headers.put("X-A2A-Algorithm", "ed25519");
            headers.put("X-A2A-Agent-ID", aimClient.getAgentId());
            headers.put("X-A2A-Public-Key", publicKey);

            return A2ARequestSignature.builder()
                    .signature(signature)
                    .timestamp(timestamp)
                    .nonce(nonce)
                    .algorithm("ed25519")
                    .agentId(aimClient.getAgentId())
                    .publicKey(publicKey)
                    .headers(headers)
                    .build();
        } catch (Exception e) {
            throw new A2AException("Failed to sign request locally: " + e.getMessage(), e);
        }
    }

    /**
     * Verify an incoming A2A request signature.
     *
     * @param signature The signature to verify
     * @param publicKey The signer's public key (base64)
     * @param timestamp Request timestamp (epoch millis)
     * @param nonce Request nonce
     * @param method HTTP method
     * @param path Request path
     * @param body Request body
     * @return Verification result containing valid status and details
     * @throws A2AException if verification fails
     */
    public Map<String, Object> verifyRequest(String signature, String publicKey, long timestamp,
                                              String nonce, String method, String path, String body)
            throws A2AException {
        try {
            ObjectNode reqBody = objectMapper.createObjectNode();
            reqBody.put("signature", signature);
            reqBody.put("publicKey", publicKey);
            reqBody.put("timestamp", timestamp);
            reqBody.put("nonce", nonce);
            reqBody.put("method", method);
            reqBody.put("path", path);
            if (body != null) {
                reqBody.put("body", body);
            }

            String response = post("/api/v1/a2a/verify", objectMapper.writeValueAsString(reqBody));
            return objectMapper.readValue(response, new TypeReference<Map<String, Object>>() {});
        } catch (Exception e) {
            throw new A2AException("Failed to verify request: " + e.getMessage(), e);
        }
    }

    // ==================== Trust Score Operations ====================

    /**
     * Get trust score for another agent.
     *
     * @param targetAgentId The target agent ID
     * @return The trust score
     * @throws A2AException if retrieval fails
     */
    public A2ATrustScore getTrustScore(String targetAgentId) throws A2AException {
        try {
            String response = get(A2A_BASE_PATH + "/trust/" + targetAgentId);
            return objectMapper.readValue(response, A2ATrustScore.class);
        } catch (Exception e) {
            throw new A2AException("Failed to get trust score: " + e.getMessage(), e);
        }
    }

    /**
     * Update trust score for another agent.
     *
     * @param targetAgentId The target agent ID
     * @param score New trust score (0.0 to 1.0)
     * @param confidence Confidence in the score (0.0 to 1.0)
     * @param factors Trust factors and their weights
     * @return Updated trust score
     * @throws A2AException if update fails
     */
    public A2ATrustScore updateTrustScore(String targetAgentId, double score, double confidence,
                                           Map<String, Double> factors) throws A2AException {
        try {
            ObjectNode body = objectMapper.createObjectNode();
            body.put("score", score);
            body.put("confidence", confidence);
            if (factors != null) {
                ObjectNode factorsNode = objectMapper.createObjectNode();
                factors.forEach(factorsNode::put);
                body.set("factors", factorsNode);
            }

            String response = put(A2A_BASE_PATH + "/trust/" + targetAgentId,
                                  objectMapper.writeValueAsString(body));
            return objectMapper.readValue(response, A2ATrustScore.class);
        } catch (Exception e) {
            throw new A2AException("Failed to update trust score: " + e.getMessage(), e);
        }
    }

    /**
     * Record a successful interaction with another agent.
     *
     * @param targetAgentId The target agent ID
     * @param taskType Type of task completed
     * @param durationMs Duration of the interaction in milliseconds
     * @return Updated trust score
     * @throws A2AException if recording fails
     */
    public A2ATrustScore recordSuccessfulInteraction(String targetAgentId, String taskType, long durationMs)
            throws A2AException {
        try {
            ObjectNode body = objectMapper.createObjectNode();
            body.put("success", true);
            body.put("taskType", taskType);
            body.put("durationMs", durationMs);

            String response = post(A2A_BASE_PATH + "/trust/" + targetAgentId + "/interaction",
                                   objectMapper.writeValueAsString(body));
            return objectMapper.readValue(response, A2ATrustScore.class);
        } catch (Exception e) {
            throw new A2AException("Failed to record interaction: " + e.getMessage(), e);
        }
    }

    /**
     * Record a failed interaction with another agent.
     *
     * @param targetAgentId The target agent ID
     * @param taskType Type of task that failed
     * @param errorReason Reason for failure
     * @return Updated trust score
     * @throws A2AException if recording fails
     */
    public A2ATrustScore recordFailedInteraction(String targetAgentId, String taskType, String errorReason)
            throws A2AException {
        try {
            ObjectNode body = objectMapper.createObjectNode();
            body.put("success", false);
            body.put("taskType", taskType);
            body.put("errorReason", errorReason);

            String response = post(A2A_BASE_PATH + "/trust/" + targetAgentId + "/interaction",
                                   objectMapper.writeValueAsString(body));
            return objectMapper.readValue(response, A2ATrustScore.class);
        } catch (Exception e) {
            throw new A2AException("Failed to record interaction: " + e.getMessage(), e);
        }
    }

    /**
     * List all peer trust relationships.
     *
     * @return List of peer trusts
     * @throws A2AException if listing fails
     */
    public List<A2APeerTrust> listPeerTrusts() throws A2AException {
        try {
            String response = get(A2A_BASE_PATH + "/peers");
            JsonNode root = objectMapper.readTree(response);
            JsonNode peersNode = root.has("peers") ? root.get("peers") : root;
            return objectMapper.convertValue(peersNode, new TypeReference<List<A2APeerTrust>>() {});
        } catch (Exception e) {
            throw new A2AException("Failed to list peer trusts: " + e.getMessage(), e);
        }
    }

    /**
     * Get trust relationship with a specific peer agent.
     *
     * @param peerAgentId The peer agent ID
     * @return The peer trust relationship
     * @throws A2AException if retrieval fails
     */
    public A2APeerTrust getPeerTrust(String peerAgentId) throws A2AException {
        try {
            String response = get(A2A_BASE_PATH + "/agents/" + aimClient.getAgentId() +
                                  "/peers/" + peerAgentId + "/trust");
            return objectMapper.readValue(response, A2APeerTrust.class);
        } catch (Exception e) {
            throw new A2AException("Failed to get peer trust: " + e.getMessage(), e);
        }
    }

    /**
     * Compute and update the A2A trust score for this agent.
     *
     * @return The computed trust score
     * @throws A2AException if computation fails
     */
    public A2ATrustScore computeTrustScore() throws A2AException {
        try {
            String response = post(A2A_BASE_PATH + "/agents/" + aimClient.getAgentId() +
                                   "/trust-score/compute", "{}");
            return objectMapper.readValue(response, A2ATrustScore.class);
        } catch (Exception e) {
            throw new A2AException("Failed to compute trust score: " + e.getMessage(), e);
        }
    }

    // ==================== Consent Management (GDPR/PSD2) ====================

    /**
     * Record consent for cross-agent data sharing.
     *
     * @param userId User ID granting consent
     * @param targetAgentId Target agent that will receive data
     * @param purpose Purpose of data sharing
     * @param dataTypes Types of data being shared
     * @param legalBasis Legal basis for processing (e.g., "consent", "contract")
     * @param expiresAt When consent expires (can be null for indefinite)
     * @return The recorded consent
     * @throws A2AException if recording fails
     */
    public A2AConsent recordConsent(String userId, String targetAgentId, String purpose,
                                     List<String> dataTypes, String legalBasis, Instant expiresAt)
            throws A2AException {
        try {
            ObjectNode body = objectMapper.createObjectNode();
            body.put("userId", userId);
            body.put("targetAgentId", targetAgentId);
            body.put("purpose", purpose);
            body.set("dataTypes", objectMapper.valueToTree(dataTypes));
            body.put("legalBasis", legalBasis);
            if (expiresAt != null) {
                body.put("expiresAt", expiresAt.toString());
            }

            String response = post(A2A_BASE_PATH + "/consent", objectMapper.writeValueAsString(body));
            return objectMapper.readValue(response, A2AConsent.class);
        } catch (Exception e) {
            throw new A2AException("Failed to record consent: " + e.getMessage(), e);
        }
    }

    /**
     * Check if consent exists for a specific operation.
     *
     * @param userId User ID
     * @param targetAgentId Target agent
     * @param purpose Purpose of data sharing
     * @param dataType Type of data being accessed
     * @return Consent status with details
     * @throws A2AException if check fails
     */
    public Map<String, Object> checkConsent(String userId, String targetAgentId,
                                             String purpose, String dataType) throws A2AException {
        try {
            String query = String.format("userId=%s&targetAgentId=%s&purpose=%s&dataType=%s",
                    userId, targetAgentId, purpose, dataType);
            String response = get(A2A_BASE_PATH + "/consent/check?" + query);
            return objectMapper.readValue(response, new TypeReference<Map<String, Object>>() {});
        } catch (Exception e) {
            throw new A2AException("Failed to check consent: " + e.getMessage(), e);
        }
    }

    /**
     * Revoke consent for cross-agent data sharing.
     *
     * @param consentId The consent ID to revoke
     * @param reason Reason for revocation
     * @return The revoked consent
     * @throws A2AException if revocation fails
     */
    public A2AConsent revokeConsent(String consentId, String reason) throws A2AException {
        try {
            ObjectNode body = objectMapper.createObjectNode();
            body.put("reason", reason);

            String response = post(A2A_BASE_PATH + "/consent/" + consentId + "/revoke",
                                   objectMapper.writeValueAsString(body));
            return objectMapper.readValue(response, A2AConsent.class);
        } catch (Exception e) {
            throw new A2AException("Failed to revoke consent: " + e.getMessage(), e);
        }
    }

    /**
     * List all consents for a user.
     *
     * @param userId The user ID
     * @return List of consents
     * @throws A2AException if listing fails
     */
    public List<A2AConsent> listUserConsents(String userId) throws A2AException {
        try {
            String response = get(A2A_BASE_PATH + "/consent?userId=" + userId);
            JsonNode root = objectMapper.readTree(response);
            JsonNode consentsNode = root.has("consents") ? root.get("consents") : root;
            return objectMapper.convertValue(consentsNode, new TypeReference<List<A2AConsent>>() {});
        } catch (Exception e) {
            throw new A2AException("Failed to list user consents: " + e.getMessage(), e);
        }
    }

    // ==================== Policy Evaluation ====================

    /**
     * Evaluate cross-agent policy for an operation.
     *
     * @param targetAgentId Target agent for the operation
     * @param action The action being requested
     * @param resource The resource being accessed
     * @param context Additional context for policy evaluation
     * @return Policy evaluation result
     * @throws A2AException if evaluation fails
     */
    public Map<String, Object> evaluatePolicy(String targetAgentId, String action, String resource,
                                               Map<String, Object> context) throws A2AException {
        try {
            ObjectNode body = objectMapper.createObjectNode();
            body.put("targetAgentId", targetAgentId);
            body.put("action", action);
            body.put("resource", resource);
            if (context != null) {
                body.set("context", objectMapper.valueToTree(context));
            }

            String response = post(A2A_BASE_PATH + "/policies/evaluate", objectMapper.writeValueAsString(body));
            return objectMapper.readValue(response, new TypeReference<Map<String, Object>>() {});
        } catch (Exception e) {
            throw new A2AException("Failed to evaluate policy: " + e.getMessage(), e);
        }
    }

    // ==================== Task Operations ====================

    /**
     * Log a task sent to another agent.
     *
     * @param targetAgentId Target agent ID
     * @param taskId Task ID
     * @param taskType Type of task
     * @param status Task status
     * @return Task log entry
     * @throws A2AException if logging fails
     */
    public Map<String, Object> logTask(String targetAgentId, String taskId, String taskType, String status)
            throws A2AException {
        try {
            ObjectNode body = objectMapper.createObjectNode();
            body.put("targetAgentId", targetAgentId);
            body.put("taskId", taskId);
            body.put("taskType", taskType);
            body.put("status", status);

            String response = post(A2A_BASE_PATH + "/tasks", objectMapper.writeValueAsString(body));
            return objectMapper.readValue(response, new TypeReference<Map<String, Object>>() {});
        } catch (Exception e) {
            throw new A2AException("Failed to log task: " + e.getMessage(), e);
        }
    }

    /**
     * Get task history between agents.
     *
     * @param targetAgentId Target agent ID
     * @param limit Maximum number of tasks to return
     * @return List of task entries
     * @throws A2AException if retrieval fails
     */
    public List<Map<String, Object>> getTaskHistory(String targetAgentId, int limit) throws A2AException {
        try {
            String response = get(A2A_BASE_PATH + "/tasks?targetAgentId=" + targetAgentId + "&limit=" + limit);
            JsonNode root = objectMapper.readTree(response);
            JsonNode tasksNode = root.has("tasks") ? root.get("tasks") : root;
            return objectMapper.convertValue(tasksNode, new TypeReference<List<Map<String, Object>>>() {});
        } catch (Exception e) {
            throw new A2AException("Failed to get task history: " + e.getMessage(), e);
        }
    }

    /**
     * Update the state of an A2A task.
     *
     * @param taskId Task ID to update
     * @param state New state (SUBMITTED, WORKING, INPUT_NEEDED, COMPLETED, FAILED, CANCELLED)
     * @param errorCode Optional error code if failed
     * @param errorMessage Optional error message if failed
     * @return Update confirmation
     * @throws A2AException if update fails
     */
    public Map<String, Object> updateTaskState(String taskId, String state, String errorCode, String errorMessage)
            throws A2AException {
        try {
            ObjectNode body = objectMapper.createObjectNode();
            body.put("state", state);
            if (errorCode != null && !errorCode.isEmpty()) {
                body.put("errorCode", errorCode);
            }
            if (errorMessage != null && !errorMessage.isEmpty()) {
                body.put("errorMessage", errorMessage);
            }

            String response = put(A2A_BASE_PATH + "/tasks/" + taskId + "/state",
                                  objectMapper.writeValueAsString(body));
            return objectMapper.readValue(response, new TypeReference<Map<String, Object>>() {});
        } catch (Exception e) {
            throw new A2AException("Failed to update task state: " + e.getMessage(), e);
        }
    }

    /**
     * Update the state of an A2A task (without error details).
     *
     * @param taskId Task ID to update
     * @param state New state
     * @return Update confirmation
     * @throws A2AException if update fails
     */
    public Map<String, Object> updateTaskState(String taskId, String state) throws A2AException {
        return updateTaskState(taskId, state, null, null);
    }

    // ==================== Skill Operations ====================

    /**
     * Register a skill for the current agent.
     *
     * @param skill The skill definition
     * @return Registered skill
     * @throws A2AException if registration fails
     */
    public A2AAgentCard.Skill registerSkill(A2AAgentCard.Skill skill) throws A2AException {
        try {
            String response = post(A2A_BASE_PATH + "/skills", objectMapper.writeValueAsString(skill));
            return objectMapper.readValue(response, A2AAgentCard.Skill.class);
        } catch (Exception e) {
            throw new A2AException("Failed to register skill: " + e.getMessage(), e);
        }
    }

    /**
     * List all skills for the current agent.
     *
     * @return List of skills
     * @throws A2AException if listing fails
     */
    public List<A2AAgentCard.Skill> listSkills() throws A2AException {
        try {
            String response = get(A2A_BASE_PATH + "/skills");
            JsonNode root = objectMapper.readTree(response);
            JsonNode skillsNode = root.has("skills") ? root.get("skills") : root;
            return objectMapper.convertValue(skillsNode, new TypeReference<List<A2AAgentCard.Skill>>() {});
        } catch (Exception e) {
            throw new A2AException("Failed to list skills: " + e.getMessage(), e);
        }
    }

    /**
     * Get skills for a specific agent.
     *
     * @param agentId The agent ID to get skills for
     * @return List of skills for the agent
     * @throws A2AException if retrieval fails
     */
    public List<A2AAgentCard.Skill> getSkills(String agentId) throws A2AException {
        try {
            String response = get(A2A_BASE_PATH + "/agents/" + agentId + "/skills");
            JsonNode root = objectMapper.readTree(response);
            JsonNode skillsNode = root.has("skills") ? root.get("skills") : root;
            return objectMapper.convertValue(skillsNode, new TypeReference<List<A2AAgentCard.Skill>>() {});
        } catch (Exception e) {
            throw new A2AException("Failed to get skills for agent: " + e.getMessage(), e);
        }
    }

    /**
     * Search for skills across all agents.
     *
     * @param query Search query
     * @param limit Maximum number of results
     * @return List of matching skills
     * @throws A2AException if search fails
     */
    public List<Map<String, Object>> searchSkills(String query, int limit) throws A2AException {
        try {
            String response = get(A2A_BASE_PATH + "/skills/search?q=" + query + "&limit=" + limit);
            JsonNode root = objectMapper.readTree(response);
            JsonNode skillsNode = root.has("skills") ? root.get("skills") : root;
            return objectMapper.convertValue(skillsNode, new TypeReference<List<Map<String, Object>>>() {});
        } catch (Exception e) {
            throw new A2AException("Failed to search skills: " + e.getMessage(), e);
        }
    }

    /**
     * Search for skills with default limit.
     *
     * @param query Search query
     * @return List of matching skills
     * @throws A2AException if search fails
     */
    public List<Map<String, Object>> searchSkills(String query) throws A2AException {
        return searchSkills(query, 20);
    }

    /**
     * Delete a skill.
     *
     * @param skillId The skill ID to delete
     * @throws A2AException if deletion fails
     */
    public void deleteSkill(String skillId) throws A2AException {
        try {
            delete(A2A_BASE_PATH + "/skills/" + skillId);
        } catch (Exception e) {
            throw new A2AException("Failed to delete skill: " + e.getMessage(), e);
        }
    }

    // ==================== Intent-Based Discovery ====================

    /**
     * Route a natural language intent to capable agents.
     *
     * <p>Uses full-text search to find agents with skills matching the intent.
     * Returns agents ranked by relevance and trust score.</p>
     *
     * @param intent Natural language description of what you need (e.g., "analyze code quality")
     * @param minTrustScore Minimum trust score required (0.0 to 1.0)
     * @return List of capable agents with their matching skills
     * @throws A2AException if routing fails
     */
    public List<Map<String, Object>> routeByIntent(String intent, double minTrustScore) throws A2AException {
        try {
            ObjectNode body = objectMapper.createObjectNode();
            body.put("intent", intent);
            body.put("minTrustScore", minTrustScore);

            String response = post(A2A_BASE_PATH + "/discovery/route", objectMapper.writeValueAsString(body));
            JsonNode root = objectMapper.readTree(response);
            JsonNode agentsNode = root.has("agents") ? root.get("agents") : root;
            return objectMapper.convertValue(agentsNode, new TypeReference<List<Map<String, Object>>>() {});
        } catch (Exception e) {
            throw new A2AException("Failed to route intent: " + e.getMessage(), e);
        }
    }

    /**
     * Route a natural language intent with default trust score.
     *
     * @param intent Natural language description of what you need
     * @return List of capable agents
     * @throws A2AException if routing fails
     */
    public List<Map<String, Object>> routeByIntent(String intent) throws A2AException {
        return routeByIntent(intent, 0.0);
    }

    /**
     * Find agents capable of handling specific skills.
     *
     * @param skillIds List of skill IDs to search for
     * @param minTrustScore Minimum trust score required
     * @return List of capable agents
     * @throws A2AException if search fails
     */
    public List<Map<String, Object>> getCapableAgents(List<String> skillIds, double minTrustScore)
            throws A2AException {
        try {
            ObjectNode body = objectMapper.createObjectNode();
            body.set("skillIds", objectMapper.valueToTree(skillIds));
            body.put("minTrustScore", minTrustScore);

            String response = post(A2A_BASE_PATH + "/discovery/capable", objectMapper.writeValueAsString(body));
            JsonNode root = objectMapper.readTree(response);
            JsonNode agentsNode = root.has("agents") ? root.get("agents") : root;
            return objectMapper.convertValue(agentsNode, new TypeReference<List<Map<String, Object>>>() {});
        } catch (Exception e) {
            throw new A2AException("Failed to find capable agents: " + e.getMessage(), e);
        }
    }

    // ==================== Security Policy Operations ====================

    /**
     * Check security policy before making an A2A call.
     *
     * <p>Evaluates security settings to determine if an A2A call is allowed.
     * In "monitor" mode, violations are logged but allowed.
     * In "strict" mode, violations block the request.</p>
     *
     * @param targetAgentId Agent being called
     * @param skillId Skill being invoked (optional)
     * @return Security check result
     * @throws A2AException if check fails
     */
    public A2ASecurityCheckResult checkSecurity(String targetAgentId, String skillId) throws A2AException {
        try {
            ObjectNode body = objectMapper.createObjectNode();
            body.put("targetAgentId", targetAgentId);
            if (skillId != null && !skillId.isEmpty()) {
                body.put("skillId", skillId);
            }

            String response = post(A2A_BASE_PATH + "/security/check", objectMapper.writeValueAsString(body));
            return objectMapper.readValue(response, A2ASecurityCheckResult.class);
        } catch (Exception e) {
            throw new A2AException("Failed to check security: " + e.getMessage(), e);
        }
    }

    /**
     * Check security policy before making an A2A call (without skill).
     *
     * @param targetAgentId Agent being called
     * @return Security check result
     * @throws A2AException if check fails
     */
    public A2ASecurityCheckResult checkSecurity(String targetAgentId) throws A2AException {
        return checkSecurity(targetAgentId, null);
    }

    /**
     * Get organization's A2A security settings.
     *
     * @return Security settings
     * @throws A2AException if retrieval fails
     */
    public A2ASecuritySettings getSecuritySettings() throws A2AException {
        try {
            String response = get(A2A_BASE_PATH + "/security/settings");
            return objectMapper.readValue(response, A2ASecuritySettings.class);
        } catch (Exception e) {
            throw new A2AException("Failed to get security settings: " + e.getMessage(), e);
        }
    }

    /**
     * Update organization's A2A security settings.
     *
     * @param settings Updated security settings
     * @return Updated settings
     * @throws A2AException if update fails
     */
    public A2ASecuritySettings updateSecuritySettings(A2ASecuritySettings settings) throws A2AException {
        try {
            String response = put(A2A_BASE_PATH + "/security/settings",
                                  objectMapper.writeValueAsString(settings));
            return objectMapper.readValue(response, A2ASecuritySettings.class);
        } catch (Exception e) {
            throw new A2AException("Failed to update security settings: " + e.getMessage(), e);
        }
    }

    /**
     * Get recent security violations.
     *
     * @param limit Maximum number of violations to return
     * @return List of security violations
     * @throws A2AException if retrieval fails
     */
    public List<A2ASecurityViolation> getSecurityViolations(int limit) throws A2AException {
        try {
            String response = get(A2A_BASE_PATH + "/security/violations?limit=" + limit);
            JsonNode root = objectMapper.readTree(response);
            JsonNode violationsNode = root.has("violations") ? root.get("violations") : root;
            return objectMapper.convertValue(violationsNode, new TypeReference<List<A2ASecurityViolation>>() {});
        } catch (Exception e) {
            throw new A2AException("Failed to get security violations: " + e.getMessage(), e);
        }
    }

    /**
     * Get security violations with default limit.
     *
     * @return List of security violations
     * @throws A2AException if retrieval fails
     */
    public List<A2ASecurityViolation> getSecurityViolations() throws A2AException {
        return getSecurityViolations(100);
    }

    // ==================== URL-Based Discovery ====================

    /**
     * Discover agent by URL (fetch and register their agent card).
     *
     * @param agentUrl Base URL of the agent
     * @return The discovered agent card
     * @throws A2AException if discovery fails
     */
    public A2AAgentCard discoverAgent(String agentUrl) throws A2AException {
        String cardUrl = agentUrl.endsWith("/")
                ? agentUrl + ".well-known/agent.json"
                : agentUrl + "/.well-known/agent.json";
        return registerAgentCard(cardUrl);
    }

    /**
     * Get the public agent card (unauthenticated discovery endpoint).
     *
     * @param aimBaseUrl AIM server base URL
     * @param agentId Agent ID to discover
     * @return The public agent card
     * @throws A2AException if discovery fails
     */
    public A2AAgentCard getPublicAgentCard(String aimBaseUrl, String agentId) throws A2AException {
        try {
            Request request = new Request.Builder()
                    .url(aimBaseUrl + "/api/v1/agents/" + agentId + "/.well-known/agent.json")
                    .get()
                    .build();

            try (Response response = httpClient.newCall(request).execute()) {
                if (!response.isSuccessful()) {
                    throw new A2AException("Failed to get public agent card: HTTP " + response.code());
                }
                String body = response.body() != null ? response.body().string() : "";
                return objectMapper.readValue(body, A2AAgentCard.class);
            }
        } catch (A2AException e) {
            throw e;
        } catch (Exception e) {
            throw new A2AException("Failed to get public agent card: " + e.getMessage(), e);
        }
    }

    // ==================== Helper Methods ====================

    private String generateNonce() {
        byte[] nonceBytes = new byte[16];
        new SecureRandom().nextBytes(nonceBytes);
        return Base64.getUrlEncoder().withoutPadding().encodeToString(nonceBytes);
    }

    private String get(String path) throws IOException {
        Request request = new Request.Builder()
                .url(aimClient.getAimUrl() + path)
                .header("Authorization", "Bearer " + aimClient.getAccessToken())
                .get()
                .build();

        try (Response response = httpClient.newCall(request).execute()) {
            String body = response.body() != null ? response.body().string() : "";
            if (!response.isSuccessful()) {
                throw new IOException("HTTP " + response.code() + ": " + body);
            }
            return body;
        }
    }

    private String post(String path, String json) throws IOException {
        RequestBody body = RequestBody.create(json, JSON);
        Request request = new Request.Builder()
                .url(aimClient.getAimUrl() + path)
                .header("Authorization", "Bearer " + aimClient.getAccessToken())
                .post(body)
                .build();

        try (Response response = httpClient.newCall(request).execute()) {
            String responseBody = response.body() != null ? response.body().string() : "";
            if (!response.isSuccessful()) {
                throw new IOException("HTTP " + response.code() + ": " + responseBody);
            }
            return responseBody;
        }
    }

    private String put(String path, String json) throws IOException {
        RequestBody body = RequestBody.create(json, JSON);
        Request request = new Request.Builder()
                .url(aimClient.getAimUrl() + path)
                .header("Authorization", "Bearer " + aimClient.getAccessToken())
                .put(body)
                .build();

        try (Response response = httpClient.newCall(request).execute()) {
            String responseBody = response.body() != null ? response.body().string() : "";
            if (!response.isSuccessful()) {
                throw new IOException("HTTP " + response.code() + ": " + responseBody);
            }
            return responseBody;
        }
    }

    private void delete(String path) throws IOException {
        Request request = new Request.Builder()
                .url(aimClient.getAimUrl() + path)
                .header("Authorization", "Bearer " + aimClient.getAccessToken())
                .delete()
                .build();

        try (Response response = httpClient.newCall(request).execute()) {
            if (!response.isSuccessful()) {
                String body = response.body() != null ? response.body().string() : "";
                throw new IOException("HTTP " + response.code() + ": " + body);
            }
        }
    }
}
