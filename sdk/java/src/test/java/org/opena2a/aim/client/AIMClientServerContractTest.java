package org.opena2a.aim.client;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import okhttp3.mockwebserver.Dispatcher;
import okhttp3.mockwebserver.MockResponse;
import okhttp3.mockwebserver.MockWebServer;
import okhttp3.mockwebserver.RecordedRequest;
import org.bouncycastle.crypto.params.Ed25519PrivateKeyParameters;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.opena2a.aim.exceptions.AIMException;
import org.opena2a.aim.exceptions.ActionDeniedException;
import org.opena2a.aim.exceptions.AuthenticationException;

import java.io.IOException;
import java.lang.reflect.Field;
import java.lang.reflect.InvocationTargetException;
import java.lang.reflect.Method;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Collections;
import java.util.HashMap;
import java.util.Iterator;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.TreeSet;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.TimeUnit;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.junit.jupiter.api.Assertions.fail;

/**
 * Checks AIMClient's requests against the routes the backend registers and the
 * members its handlers bind and answer with, all read from the backend source.
 *
 * The stand-in server answers only the requests a test lists and 404s every
 * other one, as the backend's router does for a path it does not register.
 */
class AIMClientServerContractTest {

    private static final String CLIENT_SOURCE = "sdk/java/src/main/java/org/opena2a/aim/client/AIMClient.java";
    private static final String ROUTES = "apps/backend/cmd/server/main.go";
    private static final String SDK_ROUTES = "apps/backend/cmd/server/sdk_api_routes.go";
    private static final String AGENT_DOMAIN = "apps/backend/internal/domain/agent.go";
    private static final String AGENT_SERVICE = "apps/backend/internal/application/agent_service.go";
    private static final String MCP_SERVICE = "apps/backend/internal/application/mcp_service.go";
    private static final String HANDLERS = "apps/backend/internal/interfaces/http/handlers/";
    private static final String AGENT_HANDLER = HANDLERS + "agent_handler.go";
    private static final String MCP_HANDLER = HANDLERS + "mcp_handler.go";
    private static final String CAPABILITY_REQUEST_HANDLER = HANDLERS + "capability_request_handler.go";
    private static final String VERIFICATION_HANDLER = HANDLERS + "verification_handler.go";
    private static final String PQC_HANDLER = HANDLERS + "pqc_endpoints.go";
    private static final String OAUTH_TOKEN_HANDLER = HANDLERS + "oauth_token_handler.go";

    private static final String AGENT_ID = "6f1c2e8a-3b4d-4c5e-9f60-718293a4b5c6";
    private static final String OTHER_AGENT_ID = "2d7e9a1b-5c4f-4e3a-8b2d-1f0e9c8b7a65";
    private static final String VERIFICATION_ID = "0b6d2f3e-4a5b-4c6d-8e7f-9a0b1c2d3e4f";
    private static final String VERIFICATIONS_PATH = "/api/v1/sdk-api/verifications";
    private static final String VERIFICATION_PATH = VERIFICATIONS_PATH + "/" + VERIFICATION_ID;
    private static final String SDK_AGENT_PATH = "/api/v1/sdk-api/agents/" + AGENT_ID;
    private static final String AGENT_PATH = "/api/v1/agents/" + AGENT_ID;

    private static final Pattern JSON_MEMBER = Pattern.compile("`json:\"([^\",`]+)");
    private static final Pattern FIELD_AND_MEMBER =
            Pattern.compile("^\\s*(\\w+)\\s+[^`\\n]+`json:\"([^\",`]+)", Pattern.MULTILINE);
    private static final Pattern QUOTED = Pattern.compile("\"([^\"]+)\"");

    private final ObjectMapper objectMapper = new ObjectMapper();
    private final Map<String, MockResponse> answers = new ConcurrentHashMap<>();

    private MockWebServer server;

    @BeforeEach
    void setUp() throws IOException {
        server = new MockWebServer();
        server.setDispatcher(new Dispatcher() {
            @Override
            public MockResponse dispatch(RecordedRequest request) {
                String path = request.getRequestUrl().encodedPath();
                MockResponse answer = answers.get(request.getMethod() + " " + path);
                if (answer != null) {
                    return answer;
                }
                return json(404, "{\"error\":\"Cannot " + request.getMethod() + " " + path + "\"}");
            }
        });
        server.start();
    }

    @AfterEach
    void tearDown() throws IOException {
        server.shutdown();
    }

    // ------------------------------------------------------------------------
    // Verification results
    // ------------------------------------------------------------------------

    @Test
    @DisplayName("logCapabilityResult, logSuccess and logFailure send nothing: the result route is withdrawn")
    @SuppressWarnings("deprecation")
    void resultReportingSendsNothing() throws Exception {
        assertTrue(Pattern.compile("Path: \"/verifications/:id/result\"[^\\n]*Note: \"withdrawn")
                        .matcher(read(SDK_ROUTES)).find(),
                "the backend's route table no longer marks /verifications/:id/result withdrawn; "
                        + "this test and the client's deprecation need revisiting");

        AIMClient client = registeredClient();
        try {
            client.logCapabilityResult(VERIFICATION_ID, true, "3 rows", null);
            client.logCapabilityResult(VERIFICATION_ID, false, null, "connection reset");
            client.logSuccess(VERIFICATION_ID, "3 rows");
            client.logFailure(VERIFICATION_ID, "connection reset");
        } finally {
            client.close();
        }

        assertEquals(Collections.emptyList(), paths(drain()));
    }

    @Test
    @DisplayName("performAction sends the verification request and no result report")
    void performActionSendsNoResultReport() throws Exception {
        answers.put("POST " + VERIFICATIONS_PATH, json(201,
                "{\"id\":\"" + VERIFICATION_ID + "\",\"status\":\"approved\",\"trustScore\":0.8,"
                        + "\"enforcementMode\":\"monitoring\",\"riskLevel\":\"low\",\"riskAutoDetected\":true}"));

        AIMClient client = registeredClient();
        String result;
        try {
            result = client.performAction("db:read", "users", () -> "3 rows");
            assertThrows(AIMException.class,
                    () -> client.performAction("db:read", "users", () -> {
                        throw new IllegalStateException("connection reset");
                    }));
        } finally {
            client.close();
        }

        assertEquals("3 rows", result);
        assertEquals(Arrays.asList("POST " + VERIFICATIONS_PATH, "POST " + VERIFICATIONS_PATH), paths(drain()));
    }

    // ------------------------------------------------------------------------
    // Approval poll
    // ------------------------------------------------------------------------

    @Test
    @DisplayName("a denied approval carries the denial reason the server answers with")
    void deniedApprovalCarriesTheDenialReason() throws Exception {
        Set<String> answered = jsonMembers(structBody(read(VERIFICATION_HANDLER), "VerificationResponse"));
        assertTrue(answered.contains("denialReason"), "VerificationResponse members: " + answered);
        assertFalse(answered.contains("message"), "VerificationResponse members: " + answered);

        answers.put("GET " + VERIFICATION_PATH, json(200,
                "{\"id\":\"" + VERIFICATION_ID + "\",\"status\":\"denied\","
                        + "\"denialReason\":\"Outside the change window\",\"trustScore\":0.4,"
                        + "\"enforcementMode\":\"strict\",\"riskLevel\":\"\",\"riskAutoDetected\":false}"));

        AIMClient client = registeredClient();
        ActionDeniedException denied;
        try {
            denied = assertThrows(ActionDeniedException.class, () -> client.waitForApproval(VERIFICATION_ID, 5));
        } finally {
            client.close();
        }

        assertTrue(denied.getMessage().contains("Outside the change window"), denied.getMessage());
        assertEquals(1, drain().size());
    }

    @Test
    @DisplayName("an expired approval ends the wait at the first answer")
    void expiredApprovalEndsTheWait() throws Exception {
        String writer = functionBody(read(VERIFICATION_HANDLER), "func (h *VerificationHandler) writeVerificationResponse(");
        assertTrue(writer.contains("response.Status = \"expired\""),
                "writeVerificationResponse no longer answers status \"expired\"");

        answers.put("GET " + VERIFICATION_PATH, json(200,
                "{\"id\":\"" + VERIFICATION_ID + "\",\"status\":\"expired\",\"trustScore\":0.4,"
                        + "\"enforcementMode\":\"strict\",\"riskLevel\":\"\",\"riskAutoDetected\":false}"));

        AIMClient client = registeredClient();
        ActionDeniedException expired;
        try {
            expired = assertThrows(ActionDeniedException.class, () -> client.waitForApproval(VERIFICATION_ID, 6));
        } finally {
            client.close();
        }

        assertTrue(expired.getMessage().contains("expired"), expired.getMessage());
        assertEquals(1, drain().size(), "the client polled again after the verification expired");
    }

    // ------------------------------------------------------------------------
    // Agents
    // ------------------------------------------------------------------------

    @Test
    @DisplayName("listAgents documents exactly the statuses the server defines")
    void listAgentsDocumentsTheServerStatuses() throws IOException {
        Set<String> serverStatuses = new TreeSet<>();
        Matcher constant = Pattern.compile("AgentStatus\\w+\\s+AgentStatus\\s*=\\s*\"([^\"]*)\"")
                .matcher(read(AGENT_DOMAIN));
        while (constant.find()) {
            serverStatuses.add(constant.group(1));
        }
        assertFalse(serverStatuses.isEmpty(), "no AgentStatus constants found in " + AGENT_DOMAIN);

        String source = read(CLIENT_SOURCE);
        int method = source.indexOf("public Map<String, Object> listAgents(int limit");
        assertTrue(method > 0, "listAgents(int, int, String, AgentType) was not found in " + CLIENT_SOURCE);
        int javadocStart = source.lastIndexOf("/**", method);
        int javadocEnd = source.indexOf("*/", javadocStart);
        assertTrue(javadocStart >= 0 && javadocEnd > javadocStart
                        && source.substring(javadocEnd + 2, method).trim().isEmpty(),
                "listAgents(int, int, String, AgentType) has no javadoc in " + CLIENT_SOURCE);
        String javadoc = source.substring(javadocStart, javadocEnd);
        Matcher statusLine = Pattern.compile("@param status([^@]*)").matcher(javadoc);
        assertTrue(statusLine.find(), "listAgents documents no status parameter");

        Set<String> documented = new TreeSet<>();
        Matcher quoted = QUOTED.matcher(statusLine.group(1));
        while (quoted.find()) {
            documented.add(quoted.group(1));
        }
        assertEquals(serverStatuses, documented);
    }

    @Test
    @DisplayName("reconnecting sends only the members UpdateAgent reads")
    void reconnectUpdateSendsOnlyMembersUpdateAgentReads() throws Exception {
        Set<String> read = updateAgentReadMembers();

        answers.put("PUT " + AGENT_PATH + "/keys", json(200, "{\"id\":\"" + AGENT_ID + "\"}"));
        answers.put("PUT " + AGENT_PATH, json(200, "{\"id\":\"" + AGENT_ID + "\"}"));

        AIMClient client = registeredClient(new AIMClient.Builder()
                .description("Nightly export agent")
                .addCapability("db:read")
                .addTalksTo("filesystem")
                .tags(Arrays.asList("production", "team-a"))
                .metadata(Map.of("owner", "data-team")));
        try {
            invoke(client, "updateExistingAgent", new Class<?>[0]);
        } finally {
            client.close();
        }

        RecordedRequest update = null;
        for (RecordedRequest request : drain()) {
            if ("PUT".equals(request.getMethod()) && AGENT_PATH.equals(request.getRequestUrl().encodedPath())) {
                update = request;
            }
        }
        assertNotNull(update, "the client sent no PUT " + AGENT_PATH);
        Set<String> sent = members(objectMapper.readTree(update.getBody().readUtf8()));
        sent.removeAll(read);
        assertEquals(Collections.emptySet(), sent, "members UpdateAgent does not read; it reads " + read);
    }

    @Test
    @DisplayName("deleteAgent accepts the 204 the server answers a deletion with")
    void deleteAgentAcceptsNoContent() throws Exception {
        String handler = functionBody(read(AGENT_HANDLER), "func (h *AgentHandler) DeleteAgent(");
        assertTrue(handler.contains("return c.SendStatus(fiber.StatusNoContent)"),
                "DeleteAgent no longer answers 204 No Content");

        answers.put("DELETE /api/v1/agents/" + OTHER_AGENT_ID, new MockResponse().setResponseCode(204));

        AIMClient client = registeredClient();
        Map<String, Object> result;
        try {
            result = client.deleteAgent(OTHER_AGENT_ID);
        } finally {
            client.close();
        }

        assertEquals(Boolean.TRUE, result.get("success"));
        assertEquals(Collections.singletonList("DELETE /api/v1/agents/" + OTHER_AGENT_ID), paths(drain()));
    }

    // ------------------------------------------------------------------------
    // Capability requests
    // ------------------------------------------------------------------------

    @Test
    @DisplayName("requestCapability sends only the members the capability request route binds")
    void requestCapabilitySendsOnlyBoundMembers() throws Exception {
        String handler = functionBody(read(CAPABILITY_REQUEST_HANDLER),
                "func (h *CapabilityRequestHandlers) CreateCapabilityRequest(");
        Set<String> bound = jsonMembers(structBody(handler, "RequestBody"));

        answers.put("POST " + SDK_AGENT_PATH + "/capability-requests",
                json(201, "{\"id\":\"" + VERIFICATION_ID + "\",\"status\":\"pending\"}"));

        AIMClient client = registeredClient();
        try {
            client.requestCapability("db:write", "Needed for the nightly export",
                    Map.of("targetTable", "users"));
            client.requestCapability("db:write", "Needed for the nightly export");
        } finally {
            client.close();
        }

        List<RecordedRequest> requests = drain();
        assertEquals(2, requests.size(), "requests: " + paths(requests));
        for (RecordedRequest request : requests) {
            JsonNode body = objectMapper.readTree(request.getBody().readUtf8());
            Set<String> unbound = members(body);
            unbound.removeAll(bound);
            assertEquals(Collections.emptySet(), unbound, "members the route does not bind; it binds " + bound);
            assertEquals("db:write", body.path("capabilityType").asText());
        }
    }

    // ------------------------------------------------------------------------
    // MCP servers
    // ------------------------------------------------------------------------

    @Test
    @DisplayName("automatic MCP registration sends only the members CreateMCPServerRequest binds")
    void mcpServerCreationSendsOnlyBoundMembers() throws Exception {
        Set<String> bound = jsonMembers(structBody(read(MCP_SERVICE), "CreateMCPServerRequest"));
        String mcpServerId = "7c9e6679-7425-40de-944b-e07fc1f90ae7";

        answers.put("POST " + SDK_AGENT_PATH + "/mcp-servers",
                json(201, "{\"id\":\"" + mcpServerId + "\",\"name\":\"filesystem\"}"));

        AIMClient client = registeredClient();
        Object id;
        try {
            id = invoke(client, "registerMcpServerViaSdkEndpoint", new Class<?>[]{String.class, List.class},
                    "filesystem", Arrays.asList("read_file", "write_file"));
        } finally {
            client.close();
        }

        assertEquals(mcpServerId, id);
        List<RecordedRequest> requests = drain();
        assertEquals(1, requests.size(), "requests: " + paths(requests));
        Set<String> unbound = members(objectMapper.readTree(requests.get(0).getBody().readUtf8()));
        unbound.removeAll(bound);
        assertEquals(Collections.emptySet(), unbound, "members CreateMCPServerRequest does not bind; it binds " + bound);
    }

    @Test
    @DisplayName("listMcpServers returns the servers the route answers under mcpServers, at most limit of them")
    void listMcpServersReadsTheAnsweredServers() throws Exception {
        String handler = functionBody(read(MCP_HANDLER), "func (h *MCPHandler) ListMCPServers(");
        assertTrue(handler.contains("\"mcpServers\":"), "ListMCPServers no longer answers an mcpServers member");
        assertFalse(handler.contains("Query("), "ListMCPServers now reads a query parameter; send it");

        answers.put("GET " + SDK_AGENT_PATH + "/mcp-servers", json(200,
                "{\"mcpServers\":["
                        + "{\"id\":\"a1\",\"name\":\"filesystem\",\"status\":\"verified\"},"
                        + "{\"id\":\"a2\",\"name\":\"github\",\"status\":\"pending\"},"
                        + "{\"id\":\"a3\",\"name\":\"postgres\",\"status\":\"verified\"}],\"total\":3}"));

        AIMClient client = registeredClient();
        List<Map<String, Object>> two;
        List<Map<String, Object>> all;
        try {
            two = client.listMcpServers(2);
            all = client.listMcpServers(20);
        } finally {
            client.close();
        }

        assertEquals(Arrays.asList("filesystem", "github"), names(two));
        assertEquals(Arrays.asList("filesystem", "github", "postgres"), names(all));
        for (RecordedRequest request : drain()) {
            assertNull(request.getRequestUrl().encodedQuery(), "the route reads no query parameter");
        }
    }

    @Test
    @DisplayName("listMcpServers refuses a negative limit before sending, and an answer without mcpServers")
    void listMcpServersRefusesANegativeLimitAndAnAnswerWithoutServers() throws Exception {
        answers.put("GET " + SDK_AGENT_PATH + "/mcp-servers", json(200, "{\"servers\":[],\"total\":0}"));

        AIMClient client = registeredClient();
        AIMException noServers;
        try {
            assertThrows(IllegalArgumentException.class, () -> client.listMcpServers(-1));
            assertEquals(0, server.getRequestCount(), "a negative limit was sent");
            noServers = assertThrows(AIMException.class, () -> client.listMcpServers(20));
        } finally {
            client.close();
        }

        assertTrue(noServers.getMessage().contains("mcpServers"), noServers.getMessage());
    }

    // ------------------------------------------------------------------------
    // Algorithms and sign-in
    // ------------------------------------------------------------------------

    @Test
    @DisplayName("getSupportedAlgorithms reads the route that lists the algorithms, and returns every entry")
    void supportedAlgorithmsReadTheRegisteredRoute() throws Exception {
        String routes = read(ROUTES);
        Matcher route = Pattern.compile("(\\w+)\\.Get\\(\"([^\"]+)\",\\s*h\\.Agent\\.GetSupportedAlgorithms\\)")
                .matcher(routes);
        assertTrue(route.find(), "GetSupportedAlgorithms is not registered in " + ROUTES);
        Matcher group = Pattern.compile(Pattern.quote(route.group(1)) + " := v1\\.Group\\(\"([^\"]+)\"\\)")
                .matcher(routes);
        assertTrue(group.find(), "the group of GetSupportedAlgorithms is not a v1 group in " + ROUTES);
        assertTrue(routes.contains("v1 := app.Group(\"/api/v1\")"), "v1 is no longer /api/v1 in " + ROUTES);
        String path = "/api/v1" + group.group(1) + route.group(2);

        String handler = functionBody(read(PQC_HANDLER), "func (h *AgentHandler) GetSupportedAlgorithms(");
        for (String list : Arrays.asList("classical", "postQuantum", "hybrid")) {
            assertTrue(handler.contains("\"" + list + "\": []fiber.Map{"),
                    "GetSupportedAlgorithms no longer answers a " + list + " list");
        }

        answers.put("GET " + path, json(200,
                "{\"classical\":[{\"algorithm\":\"Ed25519\",\"type\":\"classical\"}],"
                        + "\"postQuantum\":[{\"algorithm\":\"ML-DSA-44\",\"type\":\"post-quantum\"},"
                        + "{\"algorithm\":\"ML-DSA-65\",\"type\":\"post-quantum\"}],"
                        + "\"hybrid\":[{\"algorithm\":\"Ed25519+ML-DSA-65\",\"type\":\"hybrid\"}],"
                        + "\"recommended\":{\"classical\":\"Ed25519\",\"postQuantum\":\"ML-DSA-65\","
                        + "\"hybrid\":\"Ed25519+ML-DSA-65\"}}"));

        AIMClient client = registeredClient();
        List<Map<String, Object>> algorithms;
        try {
            algorithms = client.getSupportedAlgorithms();
        } finally {
            client.close();
        }

        List<String> named = new ArrayList<>();
        for (Map<String, Object> algorithm : algorithms) {
            named.add(algorithm.get("algorithm") + "/" + algorithm.get("type"));
        }
        assertEquals(Arrays.asList("Ed25519/classical", "ML-DSA-44/post-quantum", "ML-DSA-65/post-quantum",
                "Ed25519+ML-DSA-65/hybrid"), named);
        assertEquals(Collections.singletonList("GET " + path), paths(drain()));
    }

    @Test
    @DisplayName("a client ID and secret are refused before anything is sent: the token route takes no secret")
    @SuppressWarnings("deprecation")
    void clientSecretSignInSendsNothing() {
        String token = functionBody(read(OAUTH_TOKEN_HANDLER), "func (h *OAuthTokenHandler) processTokenRequest(");
        assertTrue(token.contains("if grantType != \"urn:ietf:params:oauth:grant-type:jwt-bearer\" {"),
                "the token route accepts a grant other than jwt-bearer; revisit the client-secret refusal");

        AIMClient client = new AIMClient.Builder()
                .agentName("client-secret-agent")
                .aimUrl(baseUrl())
                .clientId("client-id-placeholder")
                .clientSecret("not-a-real-secret")
                .build();
        AuthenticationException refused;
        try {
            refused = assertThrows(AuthenticationException.class, client::getAccessToken);
        } finally {
            client.close();
        }

        assertTrue(refused.getMessage().contains("refresh token"), refused.getMessage());
        assertEquals(0, server.getRequestCount());
    }

    // ------------------------------------------------------------------------
    // Helpers
    // ------------------------------------------------------------------------

    private AIMClient registeredClient() throws Exception {
        return registeredClient(new AIMClient.Builder());
    }

    /** A client in the state registration leaves it in: agent ID, key pair and a bearer token. */
    private AIMClient registeredClient(AIMClient.Builder builder) throws Exception {
        AIMClient client = builder
                .agentName("contract-test-agent")
                .aimUrl(baseUrl())
                .agentId(AGENT_ID)
                .build();
        byte[] seed = new byte[32];
        for (int i = 0; i < seed.length; i++) {
            seed[i] = (byte) (i + 7);
        }
        Ed25519PrivateKeyParameters privateKey = new Ed25519PrivateKeyParameters(seed, 0);
        setField(client, "privateKey", privateKey.getEncoded());
        setField(client, "publicKey", privateKey.generatePublicKey().getEncoded());
        setField(client, "accessToken", "test-access-token");
        setField(client, "tokenExpiry", Instant.now().plusSeconds(3600));
        return client;
    }

    /** The JSON members of the fields UpdateAgent reads from its CreateAgentRequest. */
    private static Set<String> updateAgentReadMembers() {
        Map<String, String> memberOf = new HashMap<>();
        Matcher field = FIELD_AND_MEMBER.matcher(structBody(read(AGENT_SERVICE), "CreateAgentRequest"));
        while (field.find()) {
            memberOf.put(field.group(1), field.group(2));
        }
        String body = functionBody(read(AGENT_SERVICE), "func (s *AgentService) UpdateAgent(");
        Set<String> read = new TreeSet<>();
        Matcher use = Pattern.compile("req\\.(\\w+)").matcher(body);
        while (use.find()) {
            String member = memberOf.get(use.group(1));
            assertNotNull(member, "UpdateAgent reads req." + use.group(1) + ", which CreateAgentRequest does not bind");
            read.add(member);
        }
        assertFalse(read.isEmpty(), "UpdateAgent reads no request field");
        return read;
    }

    private static Object invoke(AIMClient client, String name, Class<?>[] types, Object... args) throws Exception {
        Method method = AIMClient.class.getDeclaredMethod(name, types);
        method.setAccessible(true);
        try {
            return method.invoke(client, args);
        } catch (InvocationTargetException e) {
            if (e.getCause() instanceof Exception) {
                throw (Exception) e.getCause();
            }
            throw e;
        }
    }

    private static void setField(AIMClient client, String name, Object value) throws Exception {
        Field field = AIMClient.class.getDeclaredField(name);
        field.setAccessible(true);
        field.set(client, value);
    }

    private List<RecordedRequest> drain() throws InterruptedException {
        List<RecordedRequest> requests = new ArrayList<>();
        RecordedRequest request;
        while ((request = server.takeRequest(200, TimeUnit.MILLISECONDS)) != null) {
            requests.add(request);
        }
        return requests;
    }

    private static List<String> paths(List<RecordedRequest> requests) {
        List<String> paths = new ArrayList<>();
        for (RecordedRequest request : requests) {
            paths.add(request.getMethod() + " " + request.getRequestUrl().encodedPath());
        }
        return paths;
    }

    private static List<Object> names(List<Map<String, Object>> servers) {
        List<Object> names = new ArrayList<>();
        for (Map<String, Object> server : servers) {
            names.add(server.get("name"));
        }
        return names;
    }

    private static Set<String> members(JsonNode body) {
        Set<String> names = new TreeSet<>();
        Iterator<String> fields = body.fieldNames();
        while (fields.hasNext()) {
            names.add(fields.next());
        }
        return names;
    }

    private static Set<String> jsonMembers(String structBody) {
        Set<String> members = new TreeSet<>();
        Matcher member = JSON_MEMBER.matcher(structBody);
        while (member.find()) {
            members.add(member.group(1));
        }
        assertFalse(members.isEmpty(), "no json members found in " + structBody);
        return members;
    }

    private static String structBody(String source, String typeName) {
        Matcher struct = Pattern.compile("type " + Pattern.quote(typeName) + " struct \\{(.*?)\\n\\s*\\}",
                Pattern.DOTALL).matcher(source);
        if (!struct.find()) {
            fail("type " + typeName + " struct was not found");
        }
        return struct.group(1);
    }

    private static String functionBody(String source, String signature) {
        Matcher function = Pattern.compile(Pattern.quote(signature) + ".*?\\n}\\n", Pattern.DOTALL).matcher(source);
        if (!function.find()) {
            fail(signature + " was not found");
        }
        return function.group();
    }

    private static String read(String repoRelative) {
        Path start = Paths.get("").toAbsolutePath();
        for (Path dir = start; dir != null; dir = dir.getParent()) {
            Path candidate = dir.resolve(repoRelative);
            if (Files.isRegularFile(candidate)) {
                try {
                    return new String(Files.readAllBytes(candidate), StandardCharsets.UTF_8);
                } catch (IOException e) {
                    return fail("could not read " + candidate + ": " + e.getMessage());
                }
            }
        }
        return fail(repoRelative + " was not found above " + start
                + ". This test reads the backend source and has to run inside the repository.");
    }

    private String baseUrl() {
        String url = server.url("/").toString();
        return url.substring(0, url.length() - 1);
    }

    private static MockResponse json(int status, String body) {
        return new MockResponse()
                .setResponseCode(status)
                .setHeader("Content-Type", "application/json")
                .setBody(body);
    }
}
