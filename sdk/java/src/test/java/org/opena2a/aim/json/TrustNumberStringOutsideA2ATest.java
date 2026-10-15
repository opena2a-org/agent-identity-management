package org.opena2a.aim.json;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import okhttp3.mockwebserver.MockResponse;
import okhttp3.mockwebserver.MockWebServer;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.MethodSource;
import org.opena2a.aim.atx.AtxPublicKey;
import org.opena2a.aim.atx.AtxTrustAnchors;
import org.opena2a.aim.atx.AtxVerificationResult;
import org.opena2a.aim.atx.LocalAtxVerifier;
import org.opena2a.aim.atx.RejectCategory;
import org.opena2a.aim.client.AIMClient;
import org.opena2a.aim.exceptions.AIMException;
import org.opena2a.aim.integrations.mcp.MCPIntegration;
import org.opena2a.aim.integrations.mcp.MCPServerInfo;

import java.io.IOException;
import java.io.InputStream;
import java.time.Clock;
import java.time.Instant;
import java.time.ZoneOffset;
import java.util.ArrayList;
import java.util.List;
import java.util.Locale;
import java.util.function.Function;
import java.util.stream.Stream;

import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.Mockito.*;

/**
 * The trust numbers the SDK reads from JSON outside the A2A client: the ATX credential's
 * {@code trustScore}, which the SDK mapper binds, and the MCP server {@code trustScore} that
 * {@link MCPIntegration} reads from the JSON tree.
 *
 * <p>A long string in any of them is answered within the same one-second bound as the A2A
 * fields. A field the SDK mapper binds refuses every JSON string, a short numeric one included.
 * A tree read may return or refuse; either way the value never appears in a message. A JSON
 * number binds as before, and a valid v1.1 credential still verifies, so the {@code "%.6f"}
 * string of its signing form is never parsed back into a bound field.
 *
 * <p>Run against another Jackson release with {@code mvn test -Djackson.version=<version>}; the
 * first check confirms the requested release is the one on the classpath.
 */
@DisplayName("Trust numbers outside the A2A client")
class TrustNumberStringOutsideA2ATest {

    private static final String LONG_STRING = "1".repeat(40_000) + "x";
    private static final long TIME_LIMIT_MS = 1_000;
    private static final ObjectMapper FIXTURE_MAPPER = new ObjectMapper();

    private static final String ATX_FIXTURE = "/atx-fixtures/v1_1-baseline-valid.json";
    private static final String ATX_TRUST_SCORE = "\"trustScore\":87.5";
    private static final String SERVER_ID = "6f1c2b9e-4d3a-4f7e-9b2d-0c8e5a7f1d24";
    /** registerServer only checks that the key is at least 32 characters; the stub never reads it. */
    private static final String SERVER_PUBLIC_KEY = "fake-ed25519-public-key-for-tests-only";

    /** What a probe saw: the number it bound, or the message it refused with. */
    record Outcome(Number bound, String refusal) {
        static Outcome bound(Number n) {
            return new Outcome(n, null);
        }

        static Outcome refused(String message) {
            return new Outcome(null, message);
        }
    }

    @FunctionalInterface
    interface Probe {
        Outcome run(TrustNumberStringOutsideA2ATest test, String jsonValue) throws Exception;
    }

    /**
     * @param field         the JSON member, as the refusal names it
     * @param mapperBound   true where the SDK mapper binds the field, so every JSON string is refused
     * @param numberLiteral a JSON number that binds, and the value it binds to
     */
    record Cell(String name, String field, boolean mapperBound, String numberLiteral, double numberValue,
                Probe probe) {
        Outcome run(TrustNumberStringOutsideA2ATest test, String jsonValue) throws Exception {
            return probe.run(test, jsonValue);
        }

        @Override
        public String toString() {
            return name;
        }
    }

    static Stream<Cell> cells() {
        return Stream.of(
                new Cell("LocalAtxVerifier.verify trustScore", "trustScore", true, "87.5", 87.5,
                        (t, v) -> t.verifyAtx(v)),
                new Cell("MCPIntegration.registerServer trustScore", "trustScore", false, "0.75", 0.75,
                        (t, v) -> t.mcp("{\"id\":\"" + SERVER_ID + "\",\"name\":\"filesystem-mcp\",\"trustScore\":"
                                + v + "}", c -> MCPIntegration.registerServer(c, "filesystem-mcp",
                                "http://localhost:3000", SERVER_PUBLIC_KEY, List.of("read_file")))),
                new Cell("MCPIntegration.listServers trustScore", "trustScore", false, "0.75", 0.75,
                        (t, v) -> t.mcpList("{\"mcpServers\":[{\"id\":\"" + SERVER_ID
                                + "\",\"name\":\"filesystem-mcp\",\"trustScore\":" + v + "}]}"))
        );
    }

    private MockWebServer server;
    private AIMClient client;

    @BeforeAll
    static void loadedJacksonIsTheRequestedOne() {
        String requested = System.getProperty("aim.test.jacksonVersion");
        if (requested == null || requested.isEmpty()) {
            return;
        }
        assertEquals(requested, com.fasterxml.jackson.core.json.PackageVersion.VERSION.toString(),
                "jackson-core on the test classpath");
        assertEquals(requested, com.fasterxml.jackson.databind.cfg.PackageVersion.VERSION.toString(),
                "jackson-databind on the test classpath");
    }

    @BeforeEach
    void setUp() throws IOException {
        server = new MockWebServer();
        server.start();
        client = mock(AIMClient.class);
        String url = server.url("/").toString();
        when(client.getAimUrl()).thenReturn(url.substring(0, url.length() - 1));
        when(client.getAccessToken()).thenReturn("test-access-token");
        when(client.getAgentId()).thenReturn("agent-123");
    }

    @AfterEach
    void tearDown() throws IOException {
        server.shutdown();
    }

    // --- probes ---

    private Outcome verifyAtx(String jsonValue) throws IOException {
        JsonNode fixture;
        try (InputStream in = getClass().getResourceAsStream(ATX_FIXTURE)) {
            assertNotNull(in, "fixture not found: " + ATX_FIXTURE);
            fixture = FIXTURE_MAPPER.readTree(in);
        }
        String credential = FIXTURE_MAPPER.writeValueAsString(fixture.get("atx"));
        int at = credential.indexOf(ATX_TRUST_SCORE);
        assertTrue(at >= 0 && credential.indexOf(ATX_TRUST_SCORE, at + 1) < 0,
                "the fixture carries one " + ATX_TRUST_SCORE);
        credential = credential.replace(ATX_TRUST_SCORE, "\"trustScore\":" + jsonValue);

        AtxVerificationResult result = new LocalAtxVerifier(anchors(fixture.get("verifierState"))).verify(credential);
        if (result.valid()) {
            return Outcome.bound(result.context().trustScore());
        }
        assertEquals(RejectCategory.MALFORMED, result.rejectCategory(), "reject category: " + result.reason());
        return Outcome.refused(result.reason());
    }

    private Outcome mcp(String body, Function<AIMClient, MCPServerInfo> call) {
        respond(body);
        try {
            return Outcome.bound(call.apply(client).getTrustScore());
        } catch (AIMException e) {
            return Outcome.refused(e.getMessage());
        }
    }

    private Outcome mcpList(String body) {
        respond(body);
        try {
            List<MCPServerInfo> servers = MCPIntegration.listServers(client);
            assertEquals(1, servers.size(), "servers listed");
            return Outcome.bound(servers.get(0).getTrustScore());
        } catch (AIMException e) {
            return Outcome.refused(e.getMessage());
        }
    }

    private void respond(String body) {
        server.enqueue(new MockResponse()
                .setResponseCode(200)
                .setHeader("Content-Type", "application/json")
                .setBody(body));
    }

    private static AtxTrustAnchors anchors(JsonNode verifierState) {
        Clock clock = Clock.fixed(Instant.parse(verifierState.get("clockRfc3339").asText()), ZoneOffset.UTC);
        List<String> issuers = new ArrayList<>();
        verifierState.get("trustedIssuers").forEach(n -> issuers.add(n.asText()));
        List<AtxPublicKey> keys = new ArrayList<>();
        for (JsonNode k : verifierState.get("publicKeys")) {
            keys.add(new AtxPublicKey(k.get("algorithm").asText(), k.get("publicKeyHex").asText(),
                    k.hasNonNull("keyId") ? k.get("keyId").asText() : null));
        }
        return new AtxTrustAnchors(issuers, keys, null, clock);
    }

    private static void assertGuardRefusal(Cell cell, String message, String value, String forbiddenRun) {
        assertNotNull(message, cell + " bound the string instead of refusing it");
        assertTrue(message.contains("refused where a number is bound"), message);
        assertTrue(message.contains(cell.field()), "names the field: " + message);
        assertTrue(message.contains(value.length() + " characters"), "states the character count: " + message);
        assertTrue(message.contains("a JSON number is expected"), "states what was expected: " + message);
        assertTrue(message.length() < 512, "message length " + message.length());
        assertFalse(message.contains(forbiddenRun), "message carries the value: " + message);
    }

    // --- cells ---

    @ParameterizedTest(name = "{0}")
    @MethodSource("cells")
    @DisplayName("C6: a long non-numeric string returns or is refused within the time bound")
    void longStringIsAnsweredQuickly(Cell cell) throws Exception {
        long start = System.nanoTime();
        Outcome outcome = cell.run(this, "\"" + LONG_STRING + "\"");
        long elapsedMs = (System.nanoTime() - start) / 1_000_000;
        assertTrue(elapsedMs < TIME_LIMIT_MS, "took " + elapsedMs + " ms");
        if (cell.mapperBound()) {
            assertGuardRefusal(cell, outcome.refusal(), LONG_STRING, "1".repeat(32));
        } else if (outcome.refusal() != null) {
            assertTrue(outcome.refusal().length() < 512, "message length " + outcome.refusal().length());
            assertFalse(outcome.refusal().contains("1".repeat(32)), "message carries the value");
        }
    }

    @ParameterizedTest(name = "{0}")
    @MethodSource("cells")
    @DisplayName("C2: a JSON number binds as before, and a valid v1.1 credential verifies")
    void jsonNumberBinds(Cell cell) throws Exception {
        Outcome outcome = cell.run(this, cell.numberLiteral());
        assertNull(outcome.refusal(), cell + " refused a JSON number: " + outcome.refusal());
        assertEquals(cell.numberValue(), outcome.bound().doubleValue(), 0.0);
    }

    @ParameterizedTest(name = "{0}")
    @MethodSource("cells")
    @DisplayName("C3: where the SDK mapper binds the field, a short numeric string is refused")
    void shortNumericStringIsRefusedWhereTheMapperBinds(Cell cell) throws Exception {
        if (!cell.mapperBound()) {
            return;
        }
        // The ATX signing form writes trustScore as a "%.6f" string; on the wire it is a number.
        String numeric = String.format(Locale.ROOT, "%.6f", cell.numberValue());
        Outcome outcome = cell.run(this, "\"" + numeric + "\"");
        assertGuardRefusal(cell, outcome.refusal(), numeric, numeric);
    }
}
