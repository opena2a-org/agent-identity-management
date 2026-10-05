package org.opena2a.aim.client;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import okhttp3.mockwebserver.MockResponse;
import okhttp3.mockwebserver.MockWebServer;
import okhttp3.mockwebserver.RecordedRequest;
import org.junit.jupiter.api.*;
import org.opena2a.aim.crypto.pqc.Algorithm;
import org.opena2a.aim.crypto.pqc.HybridKeyPair;
import org.opena2a.aim.crypto.pqc.PQCOperations;
import org.opena2a.aim.exceptions.ConfigurationException;

import java.io.IOException;
import java.util.concurrent.TimeUnit;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Hybrid mode marks an agent as requiring both an Ed25519 and an ML-DSA
 * signature, and no AIM SDK signs requests in that form. These tests pin that
 * the client refuses to turn it on before sending anything, and that the
 * calls that work still send what the server reads.
 */
class PQCHybridModeTest {

    private static final String AGENT_ID = "agent-123";
    private static final String MLDSA_KEY = "bWxkc2EtcHVibGljLWtleQ==";

    private MockWebServer mockServer;
    private AIMClient client;
    private final ObjectMapper objectMapper = new ObjectMapper();

    @BeforeEach
    void setUp() throws IOException {
        mockServer = new MockWebServer();
        mockServer.start();
        String baseUrl = mockServer.url("/").toString();
        client = new AIMClient.Builder()
                .agentName("pqc-agent")
                .agentId(AGENT_ID)
                .aimUrl(baseUrl.substring(0, baseUrl.length() - 1))
                .refreshToken("test-refresh-token")
                .build();
    }

    @AfterEach
    void tearDown() throws IOException {
        client.close();
        mockServer.shutdown();
    }

    private void enqueueTokenAndOk(String body) {
        mockServer.enqueue(new MockResponse()
                .setResponseCode(200)
                .setHeader("Content-Type", "application/json")
                .setBody("{\"accessToken\":\"test-token\"}"));
        mockServer.enqueue(new MockResponse()
                .setResponseCode(200)
                .setHeader("Content-Type", "application/json")
                .setBody(body));
    }

    private RecordedRequest takeRequestTo(String path) throws InterruptedException {
        for (int i = 0; i < 2; i++) {
            RecordedRequest request = mockServer.takeRequest(1, TimeUnit.SECONDS);
            assertNotNull(request, "expected a request to " + path);
            if (path.equals(request.getPath())) {
                return request;
            }
        }
        fail("no request to " + path);
        return null;
    }

    @Test
    @DisplayName("registerPQCKey(key, algorithm, true) throws before any request is sent")
    void registerPQCKey_hybridTrue_sendsNothing() {
        enqueueTokenAndOk("{\"hybridModeEnabled\":true}");

        ConfigurationException e = assertThrows(ConfigurationException.class,
                () -> client.registerPQCKey(MLDSA_KEY, Algorithm.ML_DSA_65, true));

        assertEquals(0, mockServer.getRequestCount());
        assertTrue(e.getMessage().contains("no AIM SDK signs requests in that form"), e.getMessage());
        assertTrue(e.getMessage().contains("registerPQCKey(pqcPublicKey, algorithm, false)"), e.getMessage());
    }

    @Test
    @DisplayName("registerPQCKey(keyPair, true) throws before any request is sent")
    void registerPQCKey_keyPairHybridTrue_sendsNothing() {
        enqueueTokenAndOk("{\"hybridModeEnabled\":true}");
        HybridKeyPair keyPair = PQCOperations.generateHybridKeyPair(Algorithm.ED25519_ML_DSA_65);

        assertThrows(ConfigurationException.class, () -> client.registerPQCKey(keyPair, true));

        assertEquals(0, mockServer.getRequestCount());
    }

    @Test
    @DisplayName("setHybridMode(true) throws before any request is sent")
    void setHybridMode_true_sendsNothing() {
        enqueueTokenAndOk("{\"hybridModeEnabled\":true}");

        ConfigurationException e = assertThrows(ConfigurationException.class,
                () -> client.setHybridMode(true));

        assertEquals(0, mockServer.getRequestCount());
        assertTrue(e.getMessage().contains("registerPQCKey(pqcPublicKey, algorithm, false)"), e.getMessage());
    }

    @Test
    @DisplayName("registerPQCKey(key, algorithm, false) posts the members the server reads")
    void registerPQCKey_hybridFalse_postsKey() throws Exception {
        enqueueTokenAndOk("{\"agentId\":\"agent-123\",\"pqcKeyAlgorithm\":\"ML-DSA-65\",\"hybridModeEnabled\":false}");

        client.registerPQCKey(MLDSA_KEY, Algorithm.ML_DSA_65, false);

        RecordedRequest request = takeRequestTo("/api/v1/agents/" + AGENT_ID + "/pqc-key");
        assertEquals("POST", request.getMethod());
        JsonNode body = objectMapper.readTree(request.getBody().readUtf8());
        assertEquals(MLDSA_KEY, body.get("publicKey").asText());
        assertEquals("ML-DSA-65", body.get("algorithm").asText());
        assertFalse(body.get("enableHybrid").asBoolean());
    }

    @Test
    @DisplayName("setHybridMode(false) posts enable=false")
    void setHybridMode_false_postsDisable() throws Exception {
        enqueueTokenAndOk("{\"agentId\":\"agent-123\",\"hybridModeEnabled\":false}");

        client.setHybridMode(false);

        RecordedRequest request = takeRequestTo("/api/v1/agents/" + AGENT_ID + "/hybrid-mode");
        assertEquals("POST", request.getMethod());
        JsonNode body = objectMapper.readTree(request.getBody().readUtf8());
        assertFalse(body.get("enable").asBoolean());
    }

    @Test
    @DisplayName("createHybridRequestHeaders is deprecated on AIMClient and PQCOperations")
    void createHybridRequestHeaders_isDeprecated() throws NoSuchMethodException {
        assertNotNull(AIMClient.class
                .getMethod("createHybridRequestHeaders", HybridKeyPair.class, String.class, String.class, String.class)
                .getAnnotation(Deprecated.class));
        assertNotNull(PQCOperations.class
                .getMethod("createHybridRequestHeaders", HybridKeyPair.class, String.class, String.class, String.class, Long.class)
                .getAnnotation(Deprecated.class));
    }
}
