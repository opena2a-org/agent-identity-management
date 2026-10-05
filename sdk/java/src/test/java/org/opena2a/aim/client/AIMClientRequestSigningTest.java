package org.opena2a.aim.client;

import okhttp3.mockwebserver.MockResponse;
import okhttp3.mockwebserver.MockWebServer;
import okhttp3.mockwebserver.RecordedRequest;
import org.bouncycastle.crypto.params.Ed25519PrivateKeyParameters;
import org.bouncycastle.crypto.params.Ed25519PublicKeyParameters;
import org.bouncycastle.crypto.signers.Ed25519Signer;
import org.junit.jupiter.api.*;
import org.opena2a.aim.exceptions.VerificationException;

import java.io.IOException;
import java.lang.reflect.Field;
import java.nio.charset.StandardCharsets;
import java.time.Instant;
import java.util.Base64;
import java.util.concurrent.TimeUnit;

import static org.junit.jupiter.api.Assertions.*;

/**
 * The messages the client signs, checked against the form the platform
 * rebuilds before it verifies:
 *
 * <ul>
 *   <li>X-Signature requests: METHOD, path and timestamp joined by newlines,
 *       with the body as a fourth line only when the request has one;</li>
 *   <li>a read of the agent's own verification:
 *       {@code GET\n/api/v1/sdk-api/verifications/<id>\n<agent id>\n<timestamp>},
 *       both IDs in canonical lowercase, sent as the X-AIM-* headers.</li>
 * </ul>
 *
 * Each expected message is written out here rather than taken from the client.
 */
class AIMClientRequestSigningTest {

    private static final String AGENT_ID = "6f1c2e8a-3b4d-4c5e-9f60-718293a4b5c6";
    private static final String VERIFICATION_ID = "0b6d2f3e-4a5b-4c6d-8e7f-9a0b1c2d3e4f";
    private static final String MCP_SERVERS_PATH = "/api/v1/sdk-api/agents/" + AGENT_ID + "/mcp-servers";

    private MockWebServer server;
    private Ed25519PublicKeyParameters publicKey;

    @BeforeEach
    void setUp() throws IOException {
        server = new MockWebServer();
        server.start();
    }

    @AfterEach
    void tearDown() throws IOException {
        server.shutdown();
    }

    @Test
    @DisplayName("A signed GET signs method, path and timestamp with no trailing newline")
    void signedGetHasNoTrailingNewline() throws Exception {
        AIMClient client = registeredClient(AGENT_ID);
        server.enqueue(json("[]"));

        client.getWithSignature(MCP_SERVERS_PATH);

        RecordedRequest request = take();
        String timestamp = request.getHeader("X-Timestamp");
        assertNotNull(timestamp, "X-Timestamp header");
        assertEquals("GET", request.getMethod());
        assertEquals(MCP_SERVERS_PATH, request.getPath());
        assertEquals(Base64.getEncoder().encodeToString(publicKey.getEncoded()), request.getHeader("X-Public-Key"));
        assertSignedOver("GET\n" + MCP_SERVERS_PATH + "\n" + timestamp, request.getHeader("X-Signature"));
        client.close();
    }

    @Test
    @DisplayName("A signed POST with a body signs the body as a fourth line")
    void signedPostSignsBodyLine() throws Exception {
        AIMClient client = registeredClient(AGENT_ID);
        server.enqueue(json("{\"id\":\"mcp-1\"}"));
        String body = "{\"name\":\"filesystem\",\"url\":\"stdio://filesystem\"}";

        client.postWithSignature(MCP_SERVERS_PATH, body);

        RecordedRequest request = take();
        String timestamp = request.getHeader("X-Timestamp");
        assertEquals(body, request.getBody().readUtf8());
        assertSignedOver("POST\n" + MCP_SERVERS_PATH + "\n" + timestamp + "\n" + body, request.getHeader("X-Signature"));
        client.close();
    }

    @Test
    @DisplayName("A signed POST with an empty body appends no body line")
    void signedPostWithEmptyBodyAppendsNoBodyLine() throws Exception {
        AIMClient client = registeredClient(AGENT_ID);
        server.enqueue(json("{}"));

        client.postWithSignature(MCP_SERVERS_PATH, "");

        RecordedRequest request = take();
        String timestamp = request.getHeader("X-Timestamp");
        assertEquals(0, request.getBodySize());
        assertSignedOver("POST\n" + MCP_SERVERS_PATH + "\n" + timestamp, request.getHeader("X-Signature"));
        client.close();
    }

    @Test
    @DisplayName("The approval poll reads the verification with the X-AIM-* signed headers")
    void approvalPollSignsVerificationRead() throws Exception {
        // Upper-case IDs: the platform signs over the canonical lowercase form.
        AIMClient client = registeredClient(AGENT_ID.toUpperCase());
        server.enqueue(json("{\"id\":\"" + VERIFICATION_ID + "\",\"status\":\"approved\",\"enforcementMode\":\"strict\"}"));

        VerificationResult result = client.waitForApproval(VERIFICATION_ID.toUpperCase(), 5);

        assertTrue(result.isVerified());
        RecordedRequest request = take();
        String path = "/api/v1/sdk-api/verifications/" + VERIFICATION_ID;
        String timestamp = request.getHeader("X-AIM-Timestamp");
        assertEquals("GET", request.getMethod());
        assertEquals(path, request.getPath());
        assertEquals(AGENT_ID, request.getHeader("X-AIM-Agent-ID"));
        assertNotNull(timestamp, "X-AIM-Timestamp header");
        assertSignedOver("GET\n" + path + "\n" + AGENT_ID + "\n" + timestamp, request.getHeader("X-AIM-Signature"));
        client.close();
    }

    @Test
    @DisplayName("The approval poll refuses before sending when the agent has no signing key")
    void approvalPollRefusesWithoutSigningKey() {
        AIMClient client = new AIMClient.Builder()
                .agentName("unregistered-agent")
                .aimUrl(baseUrl())
                .agentId(AGENT_ID)
                .build();

        VerificationException error = assertThrows(VerificationException.class,
                () -> client.waitForApproval(VERIFICATION_ID, 1));

        assertTrue(error.getMessage().contains("signing key"), error.getMessage());
        assertEquals(0, server.getRequestCount());
        client.close();
    }

    @Test
    @DisplayName("The approval poll refuses a verification ID that is not a full UUID")
    void approvalPollRefusesShortenedVerificationId() throws Exception {
        AIMClient client = registeredClient(AGENT_ID);

        // UUID.fromString would expand this to 00000001-0002-0003-0004-000000000005.
        VerificationException error = assertThrows(VerificationException.class,
                () -> client.waitForApproval("1-2-3-4-5", 1));

        assertTrue(error.getMessage().contains("not a UUID"), error.getMessage());
        assertEquals(0, server.getRequestCount());
        client.close();
    }

    /** A client in the state registration leaves it in: agent ID, key pair and a bearer token. */
    private AIMClient registeredClient(String agentId) throws Exception {
        AIMClient client = new AIMClient.Builder()
                .agentName("signing-test-agent")
                .aimUrl(baseUrl())
                .agentId(agentId)
                .build();
        byte[] seed = new byte[32];
        for (int i = 0; i < seed.length; i++) {
            seed[i] = (byte) (i + 1);
        }
        Ed25519PrivateKeyParameters privateKey = new Ed25519PrivateKeyParameters(seed, 0);
        publicKey = privateKey.generatePublicKey();
        setField(client, "privateKey", privateKey.getEncoded());
        setField(client, "publicKey", publicKey.getEncoded());
        setField(client, "accessToken", "test-access-token");
        setField(client, "tokenExpiry", Instant.now().plusSeconds(3600));
        return client;
    }

    private void assertSignedOver(String expectedMessage, String signatureB64) {
        assertNotNull(signatureB64, "signature header");
        byte[] message = expectedMessage.getBytes(StandardCharsets.UTF_8);
        Ed25519Signer verifier = new Ed25519Signer();
        verifier.init(false, publicKey);
        verifier.update(message, 0, message.length);
        assertTrue(verifier.verifySignature(Base64.getDecoder().decode(signatureB64)),
                "signature does not verify over " + expectedMessage.replace("\n", "\\n"));
    }

    private RecordedRequest take() throws InterruptedException {
        RecordedRequest request = server.takeRequest(5, TimeUnit.SECONDS);
        assertNotNull(request, "no request reached the server");
        return request;
    }

    private String baseUrl() {
        String url = server.url("/").toString();
        return url.substring(0, url.length() - 1);
    }

    private static MockResponse json(String body) {
        return new MockResponse()
                .setResponseCode(200)
                .setHeader("Content-Type", "application/json")
                .setBody(body);
    }

    private static void setField(AIMClient client, String name, Object value) throws Exception {
        Field field = AIMClient.class.getDeclaredField(name);
        field.setAccessible(true);
        field.set(client, value);
    }
}
