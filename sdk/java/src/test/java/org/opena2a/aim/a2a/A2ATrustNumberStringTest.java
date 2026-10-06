package org.opena2a.aim.a2a;

import com.fasterxml.jackson.databind.exc.InvalidFormatException;
import com.fasterxml.jackson.databind.exc.MismatchedInputException;
import okhttp3.mockwebserver.MockResponse;
import okhttp3.mockwebserver.MockWebServer;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.MethodSource;
import org.opena2a.aim.client.AIMClient;

import java.io.IOException;
import java.util.List;
import java.util.function.Function;
import java.util.stream.Stream;

import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.Mockito.*;

/**
 * Every trust or confidence number the A2A client binds from a response refuses a JSON string,
 * whatever its length, and still binds a JSON number.
 *
 * <p>Run against another Jackson release with {@code mvn test -Djackson.version=<version>}; the
 * first check confirms the requested release is the one on the classpath.
 */
@DisplayName("A2A trust numbers refuse a JSON string")
class A2ATrustNumberStringTest {

    /** A string that a regex-based numeric pre-check of some Jackson releases takes quadratic time to reject. */
    private static final String LONG_STRING = "1".repeat(40_000) + "x";
    private static final String SHORT_NUMERIC_STRING = "0.75";
    private static final long TIME_LIMIT_MS = 1_000;

    @FunctionalInterface
    interface Call {
        Object invoke(A2AClient client) throws A2AException;
    }

    record Cell(String name, String template, String field, Call call, Function<Object, Number> read) {
        String body(String jsonValue) {
            return template.replace("VALUE", jsonValue);
        }

        @Override
        public String toString() {
            return name;
        }
    }

    private static Cell trustScore(String name, String template, String field, Function<A2ATrustScore, Number> read) {
        return new Cell("getTrustScore " + name, template, field,
                c -> c.getTrustScore("agent-2"), r -> read.apply((A2ATrustScore) r));
    }

    private static Cell peerTrust(String name, String template, String field, Function<A2APeerTrust, Number> read) {
        return new Cell("getPeerTrust " + name, template, field,
                c -> c.getPeerTrust("agent-2"), r -> read.apply((A2APeerTrust) r));
    }

    static Stream<Cell> cells() {
        return Stream.of(
                trustScore("score", "{\"agentId\":\"agent-2\",\"score\":VALUE}", "score",
                        A2ATrustScore::getScore),
                trustScore("a2aTrustScore", "{\"agentId\":\"agent-2\",\"a2aTrustScore\":VALUE}", "a2aTrustScore",
                        A2ATrustScore::getScore),
                trustScore("peerTrustAverage", "{\"agentId\":\"agent-2\",\"peerTrustAverage\":VALUE}", "peerTrustAverage",
                        A2ATrustScore::getPeerTrustAverage),
                trustScore("confidence", "{\"agentId\":\"agent-2\",\"confidence\":VALUE}", "confidence",
                        A2ATrustScore::getConfidence),
                trustScore("factors", "{\"agentId\":\"agent-2\",\"factors\":{\"verification\":VALUE}}", "factors",
                        t -> t.getFactors().get("verification")),
                peerTrust("peerTrustScore", "{\"peerAgentId\":\"agent-2\",\"peerTrustScore\":VALUE}", "peerTrustScore",
                        A2APeerTrust::getPeerTrustScore),
                peerTrust("successRate", "{\"peerAgentId\":\"agent-2\",\"successRate\":VALUE}", "successRate",
                        A2APeerTrust::getSuccessRate),
                peerTrust("trustScore.score", "{\"peerAgentId\":\"agent-2\",\"trustScore\":{\"score\":VALUE}}", "score",
                        p -> p.getTrustScore().getScore()),
                peerTrust("trustScore.a2aTrustScore",
                        "{\"peerAgentId\":\"agent-2\",\"trustScore\":{\"a2aTrustScore\":VALUE}}", "a2aTrustScore",
                        p -> p.getTrustScore().getScore()),
                peerTrust("trustScore.peerTrustAverage",
                        "{\"peerAgentId\":\"agent-2\",\"trustScore\":{\"peerTrustAverage\":VALUE}}", "peerTrustAverage",
                        p -> p.getTrustScore().getPeerTrustAverage()),
                peerTrust("trustScore.confidence",
                        "{\"peerAgentId\":\"agent-2\",\"trustScore\":{\"confidence\":VALUE}}", "confidence",
                        p -> p.getTrustScore().getConfidence()),
                peerTrust("trustScore.factors",
                        "{\"peerAgentId\":\"agent-2\",\"trustScore\":{\"factors\":{\"verification\":VALUE}}}", "factors",
                        p -> p.getTrustScore().getFactors().get("verification")),
                new Cell("listPeerTrusts peerTrustScore",
                        "{\"peers\":[{\"peerAgentId\":\"agent-2\",\"peerTrustScore\":VALUE}]}", "peerTrustScore",
                        A2AClient::listPeerTrusts,
                        r -> ((List<?>) r).isEmpty() ? null : ((A2APeerTrust) ((List<?>) r).get(0)).getPeerTrustScore()),
                new Cell("checkSecurity requesterTrustScore",
                        "{\"allowed\":true,\"requesterTrustScore\":VALUE}", "requesterTrustScore",
                        c -> c.checkSecurity("agent-2"),
                        r -> ((A2ASecurityCheckResult) r).getRequesterTrustScore()),
                new Cell("getSecuritySettings minTrustScore",
                        "{\"enforcementMode\":\"enforce\",\"minTrustScore\":VALUE}", "minTrustScore",
                        A2AClient::getSecuritySettings,
                        r -> ((A2ASecuritySettings) r).getMinTrustScore()),
                new Cell("getConsensusStatus confidenceScore",
                        "{\"skillId\":\"skill-1\",\"confidenceScore\":VALUE}", "confidenceScore",
                        c -> c.getConsensusStatus("agent-2", "skill-1"),
                        r -> ((A2AConsensusResult) r).getConfidenceScore())
        );
    }

    private MockWebServer server;
    private A2AClient client;

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
        AIMClient aim = mock(AIMClient.class);
        String url = server.url("/").toString();
        when(aim.getAimUrl()).thenReturn(url.substring(0, url.length() - 1));
        when(aim.getAccessToken()).thenReturn("test-token");
        when(aim.getAgentId()).thenReturn("agent-1");
        client = new A2AClient(aim, 10);
    }

    @AfterEach
    void tearDown() throws IOException {
        server.shutdown();
    }

    private void respond(String body) {
        server.enqueue(new MockResponse()
                .setResponseCode(200)
                .setHeader("Content-Type", "application/json")
                .setBody(body));
    }

    private static void assertRefusal(A2AException e, Cell cell, String value, String forbiddenRun) {
        assertInstanceOf(MismatchedInputException.class, e.getCause(), "cause of the refusal");
        assertFalse(e.getCause() instanceof InvalidFormatException,
                "the refusal comes before Jackson's string-to-number conversion");
        String message = e.getMessage();
        assertTrue(message.contains("refused where a number is bound"), message);
        assertTrue(message.contains(cell.field()), "names the field: " + message);
        assertTrue(message.contains(value.length() + " characters"), "states the character count: " + message);
        assertTrue(message.contains("a JSON number is expected"), "states what was expected: " + message);
        assertTrue(message.length() < 512, "message length " + message.length());
        assertFalse(message.contains(forbiddenRun), "message carries the value: " + message);
    }

    @ParameterizedTest(name = "{0}")
    @MethodSource("cells")
    @DisplayName("C1: a long non-numeric string is refused quickly, without the value in the message")
    void longStringIsRefused(Cell cell) {
        respond(cell.body("\"" + LONG_STRING + "\""));
        long start = System.nanoTime();
        A2AException e = assertThrows(A2AException.class, () -> cell.call().invoke(client));
        long elapsedMs = (System.nanoTime() - start) / 1_000_000;
        assertTrue(elapsedMs < TIME_LIMIT_MS, "took " + elapsedMs + " ms");
        assertRefusal(e, cell, LONG_STRING, "1".repeat(32));
    }

    @ParameterizedTest(name = "{0}")
    @MethodSource("cells")
    @DisplayName("C2: a JSON number binds as before")
    void jsonNumberBinds(Cell cell) throws A2AException {
        respond(cell.body("0.75"));
        Number bound = cell.read().apply(cell.call().invoke(client));
        assertNotNull(bound);
        assertEquals(0.75, bound.doubleValue(), 0.0);
    }

    @ParameterizedTest(name = "{0}")
    @MethodSource("cells")
    @DisplayName("C3: a short numeric string is refused")
    void shortNumericStringIsRefused(Cell cell) {
        respond(cell.body("\"" + SHORT_NUMERIC_STRING + "\""));
        A2AException e = assertThrows(A2AException.class, () -> cell.call().invoke(client));
        assertRefusal(e, cell, SHORT_NUMERIC_STRING, SHORT_NUMERIC_STRING);
    }
}
