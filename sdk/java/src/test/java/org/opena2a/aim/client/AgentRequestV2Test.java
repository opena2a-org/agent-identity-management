package org.opena2a.aim.client;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import okhttp3.MediaType;
import okhttp3.OkHttpClient;
import okhttp3.Request;
import okhttp3.RequestBody;
import okhttp3.Response;
import okhttp3.mockwebserver.MockResponse;
import okhttp3.mockwebserver.MockWebServer;
import okhttp3.mockwebserver.RecordedRequest;
import org.bouncycastle.crypto.params.Ed25519PrivateKeyParameters;
import org.bouncycastle.crypto.params.MLDSAParameters;
import org.bouncycastle.crypto.params.MLDSAPrivateKeyParameters;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.DynamicTest;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestFactory;
import org.junit.jupiter.api.function.Executable;
import org.opena2a.aim.crypto.pqc.Algorithm;
import org.opena2a.aim.crypto.pqc.MLDSAKeyPair;
import org.opena2a.aim.crypto.pqc.PQCOperations;
import org.opena2a.aim.exceptions.ConfigurationException;

import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.io.PrintStream;
import java.io.UncheckedIOException;
import java.lang.reflect.Modifier;
import java.net.InetAddress;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.security.GeneralSecurityException;
import java.security.KeyFactory;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.security.Signature;
import java.security.spec.X509EncodedKeySpec;
import java.time.Instant;
import java.time.ZoneOffset;
import java.time.ZonedDateTime;
import java.time.format.DateTimeFormatter;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Base64;
import java.util.HashSet;
import java.util.HexFormat;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.concurrent.TimeUnit;
import java.util.regex.Pattern;
import java.util.stream.Collectors;
import java.util.stream.Stream;

import static org.junit.jupiter.api.Assertions.*;

/**
 * The agent-request-v2 signer against the conformance vectors and against the server's rule on
 * the wire.
 *
 * <p>The vector tests sign the requests of docs/specs/agent-request-v2-vectors.json that AIM's
 * verifier accepts, with the keys derived from the file's labels and each vector's time and
 * nonce, and compare the two headers with the vector's statement and signature lines byte for
 * byte. The wire tests send signed requests with OkHttp to a MockWebServer on 127.0.0.1, check
 * that the statement it received names that request the way AIM's verifier compares it, and
 * verify the Ed25519 signature with the JDK's SunEC provider, an implementation independent of
 * the SDK's.
 */
class AgentRequestV2Test {

    private static final String PAYLOAD_TYPE = "application/vnd.opena2a.agent-request.v2+json";
    private static final List<String> MEMBERS =
            List.of("method", "target", "audience", "body_sha256", "agent_id", "timestamp", "nonce");
    private static final Pattern TIMESTAMP =
            Pattern.compile("^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$");
    private static final Pattern BASE64URL = Pattern.compile("^[A-Za-z0-9_-]+$");
    private static final String NOTHING_SIGNED = "; nothing was signed\nFix: ";

    private static final ObjectMapper JSON = new ObjectMapper();
    private static final Base64.Encoder B64URL = Base64.getUrlEncoder().withoutPadding();

    // The conformance vectors.

    private static final JsonNode FILE = readJson(locate("docs/specs/agent-request-v2-vectors.json"));

    /** The accepted vectors whose form the signer does not write, and why. */
    private static final Map<String, String> NOT_WRITTEN = Map.of(
            "ML-DSA-65 alone, from an agent not in hybrid mode",
            "the Ed25519 key signs the statement whenever the signer signs",
            "statement with line breaks, indentation and its members in another order",
            "the signer writes compact JSON with the members in the order of the format");

    private static List<JsonNode> accepted() {
        List<JsonNode> vectors = new ArrayList<>();
        for (JsonNode v : FILE.get("vectors")) {
            if (v.at("/want/status").asInt() == 200) {
                vectors.add(v);
            }
        }
        return vectors;
    }

    private static List<JsonNode> written() {
        return accepted().stream()
                .filter(v -> !NOT_WRITTEN.containsKey(v.get("name").asText()))
                .collect(Collectors.toList());
    }

    private static JsonNode vectorAgent(String name) {
        for (JsonNode agent : FILE.get("agents")) {
            if (agent.get("name").asText().equals(name)) {
                return agent;
            }
        }
        return fail("agent " + name + " is not in the vectors file");
    }

    /** The vector's signature header lines, joined as one line. */
    private static String signatureLine(JsonNode v) {
        List<String> lines = new ArrayList<>();
        v.at("/request/signatures").forEach(line -> lines.add(line.asText()));
        return String.join(", ", lines);
    }

    /**
     * Signs vector v's request with its signer's keys, at the vector's time and with its nonce
     * unless others are given, and with the signer's ML-DSA-65 key as well when the vector carries
     * an ML-DSA-65 signature.
     */
    private static Map<String, String> signVector(JsonNode v, Long signedAt, byte[] nonce) {
        JsonNode agent = vectorAgent(v.get("signer").asText());
        JsonNode statement = readJson(v.at("/request/statement").asText().getBytes(StandardCharsets.UTF_8));
        MLDSAKeyPair mlDsa = null;
        if (signatureLine(v).contains("ML-DSA-65:")) {
            mlDsa = new MLDSAKeyPair(Algorithm.ML_DSA_65,
                    Base64.getDecoder().decode(agent.get("mlDsa65").asText()),
                    sha256(agent.get("mlDsa65Label").asText()));
        }
        String body = v.at("/request/body").asText();
        return AgentRequestV2.sign(
                v.at("/request/method").asText(),
                FILE.get("audience").asText() + v.at("/request/target").asText(),
                body.isEmpty() ? null : body.getBytes(StandardCharsets.UTF_8),
                agent.get("agentId").asText(),
                sha256(agent.get("ed25519Label").asText()),
                mlDsa,
                signedAt != null ? signedAt : Instant.parse(statement.get("timestamp").asText()).getEpochSecond(),
                nonce != null ? nonce : Base64.getUrlDecoder().decode(statement.get("nonce").asText()));
    }

    @Test
    @DisplayName("The file holds agent-request-v2 vectors, and each accepted vector is one the signer writes or a form it does not write")
    void eachAcceptedVectorIsPlaced() {
        assertEquals("agent-request-v2", FILE.get("scheme").asText());
        assertEquals(PAYLOAD_TYPE, FILE.get("payloadType").asText());
        Set<String> acceptedNotWritten = accepted().stream()
                .map(v -> v.get("name").asText())
                .filter(NOT_WRITTEN::containsKey)
                .collect(Collectors.toSet());
        assertEquals(new HashSet<>(NOT_WRITTEN.keySet()), acceptedNotWritten);
        assertEquals(List.of(
                        "Ed25519, a GET with a query",
                        "Ed25519, a POST with a body",
                        "hybrid mode, Ed25519 and ML-DSA-65 on two header lines",
                        "hybrid mode, Ed25519 and ML-DSA-65 on one comma-separated line",
                        "timestamp 30 seconds behind the server clock",
                        "timestamp 30 seconds ahead of the server clock"),
                written().stream().map(v -> v.get("name").asText()).collect(Collectors.toList()));
    }

    @Test
    @DisplayName("The keys derived from the labels are the agents' registered keys")
    void labelKeysAreTheRegisteredKeys() {
        for (JsonNode agent : FILE.get("agents")) {
            String name = agent.get("name").asText();
            byte[] ed25519 = new Ed25519PrivateKeyParameters(sha256(agent.get("ed25519Label").asText()), 0)
                    .generatePublicKey().getEncoded();
            assertEquals(agent.get("ed25519").asText(), Base64.getEncoder().encodeToString(ed25519), name);
            if (agent.has("mlDsa65Label")) {
                byte[] mlDsa65 = new MLDSAPrivateKeyParameters(MLDSAParameters.ml_dsa_65,
                        sha256(agent.get("mlDsa65Label").asText())).getPublicKeyParameters().getEncoded();
                assertEquals(agent.get("mlDsa65").asText(), Base64.getEncoder().encodeToString(mlDsa65), name);
            }
        }
    }

    @TestFactory
    @DisplayName("Each written vector: the statement, its encoding and the signature line, byte for byte")
    Stream<DynamicTest> writtenVectors() {
        return written().stream().map(v -> DynamicTest.dynamicTest(v.get("name").asText(), () -> {
            JsonNode agent = vectorAgent(v.get("signer").asText());
            Map<String, String> headers = signVector(v, null, null);

            byte[] payload = statementBytes(headers.get("X-Agent-Request"));
            assertEquals(v.at("/request/statement").asText(), new String(payload, StandardCharsets.UTF_8));
            assertEquals(v.get("signedText").asText(), new String(pae(payload), StandardCharsets.UTF_8));
            assertEquals(signatureLine(v), headers.get("X-Agent-Request-Signature"));

            String ed25519 = headers.get("X-Agent-Request-Signature").split(", ")[0];
            assertTrue(ed25519.startsWith("Ed25519:"), ed25519);
            assertTrue(jdkVerifiesEd25519(Base64.getDecoder().decode(agent.get("ed25519").asText()), pae(payload),
                    Base64.getUrlDecoder().decode(ed25519.substring("Ed25519:".length()))));
        }));
    }

    @Test
    @DisplayName("Control: each written vector signed one second later, or with one nonce bit flipped, is not that vector")
    void changedTimeOrNonceIsNotTheVector() {
        for (JsonNode v : written()) {
            JsonNode statement = readJson(v.at("/request/statement").asText().getBytes(StandardCharsets.UTF_8));
            byte[] nonce = Base64.getUrlDecoder().decode(statement.get("nonce").asText());
            nonce[0] ^= 0x01;
            long later = Instant.parse(statement.get("timestamp").asText()).getEpochSecond() + 1;
            for (Map<String, String> headers : List.of(signVector(v, later, null), signVector(v, null, nonce))) {
                String name = v.get("name").asText();
                assertNotEquals(v.at("/request/statement").asText(),
                        new String(statementBytes(headers.get("X-Agent-Request")), StandardCharsets.UTF_8), name);
                assertNotEquals(signatureLine(v), headers.get("X-Agent-Request-Signature"), name);
            }
        }
    }

    // The wire.

    private static final String AGENT_ID = "3f8e2b6a-1c4d-4e7f-9a0b-5d6c7e8f9a01";
    private static final byte[][] KEYS = PQCOperations.generateEd25519KeyPair();
    private static final byte[] PUBLIC_KEY = KEYS[0];
    private static final byte[] PRIVATE_KEY = KEYS[1];
    private static final MLDSAKeyPair ML_DSA = PQCOperations.generateMLDSAKeyPair(Algorithm.ML_DSA_65);
    private static final OkHttpClient CLIENT = new OkHttpClient();

    private MockWebServer server;
    private String base;

    @BeforeEach
    void startServer() throws IOException {
        server = new MockWebServer();
        server.start(InetAddress.getByName("127.0.0.1"), 0);
        base = "http://127.0.0.1:" + server.getPort();
    }

    @AfterEach
    void stopServer() throws IOException {
        server.shutdown();
    }

    /** What the loopback server received of one request. */
    private record Received(String method, String target, List<String> statements, List<String> signatures,
                            byte[] body) {
        static Received of(RecordedRequest r) {
            return new Received(r.getMethod(), r.getPath(), r.getHeaders().values("X-Agent-Request"),
                    r.getHeaders().values("X-Agent-Request-Signature"), r.getBody().readByteArray());
        }

        Received withMethod(String method) {
            return new Received(method, target, statements, signatures, body);
        }

        Received withTarget(String target) {
            return new Received(method, target, statements, signatures, body);
        }

        Received withBody(byte[] body) {
            return new Received(method, target, statements, signatures, body);
        }

        JsonNode statement() {
            assertEquals(1, statements.size(), "X-Agent-Request lines");
            return readJson(statementBytes(statements.get(0)));
        }
    }

    private Received signAndSend(String method, String pathAndQuery, byte[] body, MLDSAKeyPair mlDsa)
            throws Exception {
        String url = base + pathAndQuery;
        Map<String, String> headers = AgentRequestV2.headers(method, url, body, AGENT_ID, PRIVATE_KEY, mlDsa);
        Request.Builder builder = new Request.Builder().url(url);
        headers.forEach(builder::addHeader);
        builder.method(method, body == null ? null : RequestBody.create(body, MediaType.get("application/json")));
        // A server clock 600 s away from this host's: the signer must never take its time from a
        // response.
        server.enqueue(new MockResponse()
                .setHeader("Date", DateTimeFormatter.RFC_1123_DATE_TIME.format(
                        ZonedDateTime.ofInstant(Instant.now().plusSeconds(600), ZoneOffset.UTC)))
                .setBody("{}"));
        try (Response response = CLIENT.newCall(builder.build()).execute()) {
            assertEquals(200, response.code());
        }
        RecordedRequest recorded = server.takeRequest(5, TimeUnit.SECONDS);
        assertNotNull(recorded, "the server received no request");
        return Received.of(recorded);
    }

    /**
     * What AIM's verifier compares, applied to what the loopback server received: the members and
     * signatures that do not hold, none when it would accept the statement. The members must be in
     * the format's order, and the time in whole seconds, which is what the signer writes.
     */
    private List<String> serverMismatches(Received r, MLDSAKeyPair mlDsa) {
        assertEquals(1, r.statements().size(), "X-Agent-Request lines");
        byte[] payload = statementBytes(r.statements().get(0));
        JsonNode statement = readJson(payload);
        List<String> mismatches = new ArrayList<>();

        List<String> members = new ArrayList<>();
        statement.fieldNames().forEachRemaining(members::add);
        List<String> expected = MEMBERS.stream()
                .filter(m -> !m.equals("body_sha256") || r.body().length > 0)
                .collect(Collectors.toList());
        if (!members.equals(expected)) mismatches.add("members");
        if (!statement.path("method").asText().equals(r.method())) mismatches.add("method");
        if (!statement.path("target").asText().equals(r.target())) mismatches.add("target");
        if (!statement.path("audience").asText().equals(base)) mismatches.add("audience");
        if (r.body().length > 0
                && !statement.path("body_sha256").asText().equals(B64URL.encodeToString(sha256(r.body())))) {
            mismatches.add("body_sha256");
        }
        if (!statement.path("agent_id").asText().equals(AGENT_ID)) mismatches.add("agent_id");
        if (!TIMESTAMP.matcher(statement.path("timestamp").asText()).matches()) mismatches.add("timestamp");
        if (decodedLength(statement.path("nonce").asText()) != 16) mismatches.add("nonce");

        byte[] message = pae(payload);
        List<String> values = new ArrayList<>();
        for (String line : r.signatures()) {
            for (String value : line.split(",")) {
                values.add(value.trim());
            }
        }
        if (values.size() != (mlDsa == null ? 1 : 2)) mismatches.add("signature count");
        String ed25519 = values.stream().filter(v -> v.startsWith("Ed25519:")).findFirst().orElse(null);
        if (ed25519 == null || !jdkVerifiesEd25519(PUBLIC_KEY, message, decode(ed25519.substring("Ed25519:".length())))) {
            mismatches.add("Ed25519 signature");
        }
        if (mlDsa != null) {
            String prefix = mlDsa.getAlgorithm().getAlgorithmId() + ":";
            String value = values.stream().filter(v -> v.startsWith(prefix)).findFirst().orElse(null);
            if (value == null || !PQCOperations.verifyMLDSA(mlDsa.getAlgorithm(), mlDsa.getPublicKey(), message,
                    decode(value.substring(prefix.length())))) {
                mismatches.add("ML-DSA signature");
            }
        }
        return mismatches;
    }

    @Test
    @DisplayName("A GET with a query, which carries no body_sha256")
    void getWithQuery() throws Exception {
        Received r = signAndSend("GET", "/api/v1/agents?limit=10&offset=0", null, null);
        assertEquals("/api/v1/agents?limit=10&offset=0", r.target());
        assertFalse(r.statement().has("body_sha256"));
        assertEquals(List.of(), serverMismatches(r, null));
    }

    @Test
    @DisplayName("A POST whose body is UTF-8 JSON, and one whose body is bytes that are not UTF-8")
    void postWithBody() throws Exception {
        byte[] text = "{\"zeta\":1,\"alpha\":{\"y\":2,\"b\":\"é\"}}".getBytes(StandardCharsets.UTF_8);
        Received r = signAndSend("POST", "/api/v1/x", text, null);
        assertArrayEquals(text, r.body());
        assertEquals(List.of(), serverMismatches(r, null));

        byte[] bytes = {(byte) 0xff, 0x00, (byte) 0xfe, 0x7f, (byte) 0xc3};
        Received s = signAndSend("POST", "/api/v1/x", bytes, null);
        assertArrayEquals(bytes, s.body());
        assertEquals(List.of(), serverMismatches(s, null));
    }

    @Test
    @DisplayName("A POST with an empty body, which carries no body_sha256")
    void postWithEmptyBody() throws Exception {
        Received r = signAndSend("POST", "/api/v1/x", new byte[0], null);
        assertEquals(0, r.body().length);
        assertFalse(r.statement().has("body_sha256"));
        assertEquals(List.of(), serverMismatches(r, null));
    }

    @Test
    @DisplayName("PUT, PATCH and DELETE")
    void putPatchDelete() throws Exception {
        for (String method : List.of("PUT", "PATCH", "DELETE")) {
            byte[] body = method.equals("DELETE") ? null : "{\"a\":1}".getBytes(StandardCharsets.UTF_8);
            Received r = signAndSend(method, "/api/v1/x?y=1", body, null);
            assertEquals(method, r.method());
            assertEquals(List.of(), serverMismatches(r, null), method);
        }
    }

    @Test
    @DisplayName("A percent-encoded query is signed as sent, and a fragment is neither sent nor signed")
    void percentEncodedQueryAndFragment() throws Exception {
        Received r = signAndSend("GET", "/api/v1/x?b=a%20b&c=%2F#frag", null, null);
        assertEquals("/api/v1/x?b=a%20b&c=%2F", r.target());
        assertEquals(List.of(), serverMismatches(r, null));
    }

    @Test
    @DisplayName("A path and query OkHttp encodes before sending are signed as OkHttp sends them, an empty query included")
    void targetIsWhatOkHttpSends() throws Exception {
        Map<String, String> cases = new LinkedHashMap<>();
        cases.put("/p/é?q=é d", "/p/%C3%A9?q=%C3%A9%20d");
        cases.put("/p?q=\"x\"", "/p?q=%22x%22");
        cases.put("/p?", "/p?");
        for (Map.Entry<String, String> c : cases.entrySet()) {
            Received r = signAndSend("GET", c.getKey(), null, null);
            assertEquals(c.getValue(), r.target(), c.getKey());
            assertEquals(List.of(), serverMismatches(r, null), c.getKey());
        }
    }

    @Test
    @DisplayName("With an ML-DSA-44, ML-DSA-65 or ML-DSA-87 key pair: one line with an Ed25519 and an ML-DSA signature, both over the statement")
    void withMLDSA() throws Exception {
        Map<Algorithm, MLDSAKeyPair> keyPairs = new LinkedHashMap<>();
        keyPairs.put(Algorithm.ML_DSA_44, PQCOperations.generateMLDSAKeyPair(Algorithm.ML_DSA_44));
        keyPairs.put(Algorithm.ML_DSA_65, ML_DSA);
        keyPairs.put(Algorithm.ML_DSA_87, PQCOperations.generateMLDSAKeyPair(Algorithm.ML_DSA_87));
        for (Map.Entry<Algorithm, MLDSAKeyPair> k : keyPairs.entrySet()) {
            String algorithm = k.getKey().getAlgorithmId();
            Received r = signAndSend("POST", "/api/v1/x", "{\"a\":1}".getBytes(StandardCharsets.UTF_8), k.getValue());
            assertEquals(1, r.signatures().size(), "X-Agent-Request-Signature lines");
            assertTrue(Pattern.matches("^Ed25519:[A-Za-z0-9_-]{86}, " + algorithm + ":[A-Za-z0-9_-]+$",
                    r.signatures().get(0)), algorithm);
            assertEquals(List.of(), serverMismatches(r, k.getValue()), algorithm);
        }
    }

    @Test
    @DisplayName("The server rule refuses the statement once the body, target or method differs from what was signed")
    void ruleRefusesAChangedRequest() throws Exception {
        Received r = signAndSend("POST", "/api/v1/x?a=1", "{\"a\":1}".getBytes(StandardCharsets.UTF_8), null);
        assertEquals(List.of(), serverMismatches(r, null));
        assertEquals(List.of("body_sha256"),
                serverMismatches(r.withBody("{\"a\":2}".getBytes(StandardCharsets.UTF_8)), null));
        assertEquals(List.of("members"), serverMismatches(r.withBody(new byte[0]), null));
        assertEquals(List.of("target"), serverMismatches(r.withTarget("/api/v1/x?a=2"), null));
        assertEquals(List.of("method"), serverMismatches(r.withMethod("PUT"), null));
    }

    @Test
    @DisplayName("The time is this host's clock in whole seconds, never a response Date")
    void timeIsTheHostClock() throws Exception {
        long before = Instant.now().getEpochSecond();
        // The first response carries a Date 600 s ahead; the next statement still takes the
        // host clock.
        signAndSend("GET", "/first", null, null);
        Received r = signAndSend("GET", "/second", null, null);
        long after = Instant.now().getEpochSecond();
        String timestamp = r.statement().path("timestamp").asText();
        assertTrue(TIMESTAMP.matcher(timestamp).matches(), timestamp);
        long signedAt = Instant.parse(timestamp).getEpochSecond();
        assertTrue(signedAt >= before && signedAt <= after, timestamp + " is not between " + before + " and " + after);
    }

    @Test
    @DisplayName("Each statement carries a new 16-byte nonce")
    void eachStatementHasANewNonce() {
        Set<String> nonces = new HashSet<>();
        for (int i = 0; i < 32; i++) {
            Map<String, String> headers = AgentRequestV2.headers("GET", base + "/p", null, AGENT_ID, PRIVATE_KEY, null);
            String nonce = readJson(statementBytes(headers.get("X-Agent-Request"))).path("nonce").asText();
            assertEquals(16, decodedLength(nonce), nonce);
            nonces.add(nonce);
        }
        assertEquals(32, nonces.size());
    }

    @Test
    @DisplayName("The audience is the origin of the url: scheme, host and a port other than the scheme's default")
    void audienceIsTheOrigin() {
        Map<String, String> cases = new LinkedHashMap<>();
        cases.put("https://AIM.Example.TEST/p", "https://aim.example.test");
        cases.put("https://aim.example.test:443/p", "https://aim.example.test");
        cases.put("http://aim.example.test:80/p", "http://aim.example.test");
        cases.put("https://aim.example.test:8443/base/p?q=1", "https://aim.example.test:8443");
        cases.put("http://[::1]:8080/p", "http://[::1]:8080");
        for (Map.Entry<String, String> c : cases.entrySet()) {
            Map<String, String> headers = AgentRequestV2.headers("GET", c.getKey(), null, AGENT_ID, PRIVATE_KEY, null);
            assertEquals(c.getValue(), readJson(statementBytes(headers.get("X-Agent-Request"))).path("audience").asText(),
                    c.getKey());
        }
    }

    @Test
    @DisplayName("Returns exactly the two headers, with one Ed25519 signature when no ML-DSA key pair is given")
    void returnsTheTwoHeaders() {
        Map<String, String> headers = AgentRequestV2.headers("GET", base + "/p", null, AGENT_ID, PRIVATE_KEY, null);
        assertEquals(List.of("X-Agent-Request", "X-Agent-Request-Signature"), new ArrayList<>(headers.keySet()));
        assertTrue(Pattern.matches("^Ed25519:[A-Za-z0-9_-]{86}$", headers.get("X-Agent-Request-Signature")));
    }

    // The refusals.

    private static ConfigurationException refusalOf(Executable call) {
        return assertThrows(ConfigurationException.class, call);
    }

    @Test
    @DisplayName("Refuses a method, url, agent ID or key it cannot sign for, before signing, with a Fix line")
    void refusals() {
        String url = "http://127.0.0.1:9/p";
        String method = "the method is not one of GET, POST, PUT, PATCH or DELETE in upper case";
        String notAbsolute = "the url is not an absolute http or https URL";
        String noAgentId = "no agent ID is configured";
        String notUuid = "the agent ID is not a UUID in its 36-character form";
        byte[] key = PRIVATE_KEY;
        Map<Executable, String> cases = new LinkedHashMap<>();
        cases.put(() -> AgentRequestV2.headers("get", url, null, AGENT_ID, key, null), method);
        cases.put(() -> AgentRequestV2.headers("HEAD", url, null, AGENT_ID, key, null), method);
        cases.put(() -> AgentRequestV2.headers(null, url, null, AGENT_ID, key, null), method);
        cases.put(() -> AgentRequestV2.headers("GET", null, null, AGENT_ID, key, null), notAbsolute);
        cases.put(() -> AgentRequestV2.headers("GET", "/api/v1/x", null, AGENT_ID, key, null), notAbsolute);
        cases.put(() -> AgentRequestV2.headers("GET", "ftp://127.0.0.1/p", null, AGENT_ID, key, null), notAbsolute);
        cases.put(() -> AgentRequestV2.headers("GET", "http://someone@127.0.0.1/p", null, AGENT_ID, key, null),
                "the url carries a username or a password");
        cases.put(() -> AgentRequestV2.headers("GET", url, null, null, key, null), noAgentId);
        cases.put(() -> AgentRequestV2.headers("GET", url, null, "", key, null), noAgentId);
        cases.put(() -> AgentRequestV2.headers("GET", url, null, "agent-1", key, null), notUuid);
        cases.put(() -> AgentRequestV2.headers("GET", url, null, "{" + AGENT_ID + "}", key, null), notUuid);
        cases.put(() -> AgentRequestV2.headers("GET", url, null, AGENT_ID.replace('-', 'x'), key, null), notUuid);
        cases.put(() -> AgentRequestV2.headers("GET", url, null, AGENT_ID, null, null),
                "no agent private key is configured");
        cases.put(() -> AgentRequestV2.headers("GET", url, null, AGENT_ID, Arrays.copyOf(key, 31), null),
                "the agent private key is 31 bytes long, not 32, the length of an Ed25519 seed");
        cases.put(() -> AgentRequestV2.headers("GET", url, null, AGENT_ID, Arrays.copyOf(key, 64), null),
                "the agent private key is 64 bytes long, not 32, the length of an Ed25519 seed");
        cases.put(() -> AgentRequestV2.headers("GET", url, null, AGENT_ID, key,
                        new MLDSAKeyPair(Algorithm.ML_DSA_65, ML_DSA.getPublicKey(), new byte[31])),
                "the ML-DSA private key is 31 bytes long, not 32, the length of an ML-DSA seed");
        cases.put(() -> AgentRequestV2.headers("GET", url, null, AGENT_ID, key,
                        new MLDSAKeyPair(Algorithm.ML_DSA_65, ML_DSA.getPublicKey(), new byte[4032])),
                "the ML-DSA private key is 4032 bytes long, not 32, the length of an ML-DSA seed");
        for (Map.Entry<Executable, String> c : cases.entrySet()) {
            String message = refusalOf(c.getKey()).getMessage();
            assertTrue(message.startsWith(c.getValue() + NOTHING_SIGNED), message);
        }
        assertEquals(32, ML_DSA.getPrivateKey().length);
        // Control: the same request with a key of each kind signs.
        assertEquals(2, AgentRequestV2.headers("GET", url, null, AGENT_ID.toUpperCase(), key, ML_DSA).size());
    }

    // In the SDK.

    private static final Pattern NAMES_IT = Pattern.compile("\\bAgentRequestV2\\b");

    @Test
    @DisplayName("The class is not public, and no SDK source outside its file names it")
    void notPublicAndNotCalled() throws IOException {
        assertFalse(Modifier.isPublic(AgentRequestV2.class.getModifiers()));
        Path base = Paths.get(System.getProperty("basedir", "."));
        List<String> naming;
        try (Stream<Path> files = Files.walk(base.resolve("src/main/java"))) {
            naming = files.filter(p -> p.toString().endsWith(".java"))
                    .filter(p -> !p.getFileName().toString().equals("AgentRequestV2.java"))
                    .filter(p -> NAMES_IT.matcher(read(p)).find())
                    .map(Path::toString)
                    .collect(Collectors.toList());
        }
        assertEquals(List.of(), naming);
        // Control: the same reading finds this test file.
        assertTrue(NAMES_IT.matcher(read(base.resolve(
                "src/test/java/org/opena2a/aim/client/AgentRequestV2Test.java"))).find());
    }

    @Test
    @DisplayName("Writes nothing to System.out or System.err when it signs or refuses")
    void writesNothing() {
        PrintStream out = System.out;
        PrintStream err = System.err;
        ByteArrayOutputStream captured = new ByteArrayOutputStream();
        PrintStream capture = new PrintStream(captured, true, StandardCharsets.UTF_8);
        try {
            System.setOut(capture);
            System.setErr(capture);
            AgentRequestV2.headers("GET", base + "/p", null, AGENT_ID, PRIVATE_KEY, ML_DSA);
            refusalOf(() -> AgentRequestV2.headers("get", base + "/p", null, AGENT_ID, PRIVATE_KEY, null));
        } finally {
            System.setOut(out);
            System.setErr(err);
        }
        assertEquals("", captured.toString(StandardCharsets.UTF_8));
    }

    // Helpers.

    /** The DSSE v1 pre-authentication encoding, written from the format's text. */
    private static byte[] pae(byte[] payload) {
        byte[] head = ("DSSEv1 " + PAYLOAD_TYPE.getBytes(StandardCharsets.UTF_8).length + " " + PAYLOAD_TYPE + " "
                + payload.length + " ").getBytes(StandardCharsets.UTF_8);
        byte[] message = Arrays.copyOf(head, head.length + payload.length);
        System.arraycopy(payload, 0, message, head.length, payload.length);
        return message;
    }

    /** The statement a header carries: strict unpadded base64url of one JSON object. */
    private static byte[] statementBytes(String header) {
        assertNotNull(header, "X-Agent-Request");
        assertTrue(BASE64URL.matcher(header).matches(), "X-Agent-Request is not unpadded base64url");
        byte[] payload = Base64.getUrlDecoder().decode(header);
        assertEquals(header, B64URL.encodeToString(payload), "X-Agent-Request is not in its one encoding");
        return payload;
    }

    private static int decodedLength(String base64url) {
        try {
            return BASE64URL.matcher(base64url).matches() ? Base64.getUrlDecoder().decode(base64url).length : -1;
        } catch (IllegalArgumentException e) {
            return -1;
        }
    }

    private static byte[] decode(String base64url) {
        try {
            return Base64.getUrlDecoder().decode(base64url);
        } catch (IllegalArgumentException e) {
            return new byte[0];
        }
    }

    /** Verifies an Ed25519 signature with the JDK's SunEC provider, from the raw 32-byte public key. */
    private static boolean jdkVerifiesEd25519(byte[] rawPublicKey, byte[] message, byte[] signature) {
        // The DER header of an Ed25519 SubjectPublicKeyInfo (RFC 8410), then the raw key.
        byte[] header = HexFormat.of().parseHex("302a300506032b6570032100");
        byte[] spki = Arrays.copyOf(header, header.length + rawPublicKey.length);
        System.arraycopy(rawPublicKey, 0, spki, header.length, rawPublicKey.length);
        try {
            Signature verifier = Signature.getInstance("Ed25519", "SunEC");
            verifier.initVerify(KeyFactory.getInstance("Ed25519", "SunEC").generatePublic(new X509EncodedKeySpec(spki)));
            verifier.update(message);
            return verifier.verify(signature);
        } catch (GeneralSecurityException e) {
            return false;
        }
    }

    private static byte[] sha256(String text) {
        return sha256(text.getBytes(StandardCharsets.UTF_8));
    }

    private static byte[] sha256(byte[] data) {
        try {
            return MessageDigest.getInstance("SHA-256").digest(data);
        } catch (NoSuchAlgorithmException e) {
            throw new IllegalStateException(e);
        }
    }

    private static JsonNode readJson(byte[] json) {
        try {
            return JSON.readTree(json);
        } catch (IOException e) {
            throw new UncheckedIOException(e);
        }
    }

    private static JsonNode readJson(Path path) {
        try {
            return JSON.readTree(Files.readAllBytes(path));
        } catch (IOException e) {
            throw new UncheckedIOException(e);
        }
    }

    private static String read(Path path) {
        try {
            return Files.readString(path, StandardCharsets.UTF_8);
        } catch (IOException e) {
            throw new UncheckedIOException(e);
        }
    }

    private static Path locate(String relative) {
        Path start = Paths.get("").toAbsolutePath();
        for (Path dir = start; dir != null; dir = dir.getParent()) {
            Path candidate = dir.resolve(relative);
            if (Files.isRegularFile(candidate)) {
                return candidate;
            }
        }
        throw new IllegalStateException(relative + " was not found above " + start
                + ". This test reads the conformance vectors and has to run inside the repository.");
    }
}
