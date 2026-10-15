package org.opena2a.aim.integrations.mcp;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import okhttp3.mockwebserver.Dispatcher;
import okhttp3.mockwebserver.MockResponse;
import okhttp3.mockwebserver.MockWebServer;
import okhttp3.mockwebserver.RecordedRequest;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.NullSource;
import org.junit.jupiter.params.provider.ValueSource;
import org.opena2a.aim.client.AIMClient;
import org.opena2a.aim.exceptions.AIMException;

import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.util.HashMap;
import java.util.Iterator;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.TreeSet;
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
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

/**
 * Checks what {@link MCPIntegration} sends, and what it reads back, against the
 * backend source: the route each call reaches, the JSON members its handler
 * binds and refuses to go without, and the members its handler answers with.
 *
 * The stand-in server answers those routes the way their handlers do and 404s
 * every other path, as the backend's router does.
 */
class MCPIntegrationServerContractTest {

    private static final String SDK_ROUTES = "apps/backend/cmd/server/sdk_api_routes.go";
    private static final String ROUTES = "apps/backend/cmd/server/main.go";
    private static final String ATTESTATION_HANDLER =
            "apps/backend/internal/interfaces/http/handlers/mcp_attestation_handler.go";
    private static final String MCP_HANDLER = "apps/backend/internal/interfaces/http/handlers/mcp_handler.go";

    private static final String AGENT_ID = "550e8400-e29b-41d4-a716-446655440000";
    private static final String SERVER_ID = "7c9e6679-7425-40de-944b-e07fc1f90ae7";
    private static final String ACCESS_TOKEN = "not-a-real-token";

    private static final Pattern SDK_BASE_PATH = Pattern.compile("const sdkAPIBasePath = \"([^\"]+)\"");
    private static final Pattern V1_GROUP = Pattern.compile("v1 := app\\.Group\\(\"([^\"]+)\"\\)");
    private static final Pattern JSON_FIELD = Pattern.compile("(\\w+)\\s+[^`\\n]+`json:\"([^\",`]+)");
    private static final Pattern REQUIRED_FIELD = Pattern.compile("if req\\.(\\w+) == \"\"");
    private static final Pattern MAP_KEY = Pattern.compile("\"(\\w+)\":");

    private final ObjectMapper objectMapper = new ObjectMapper();

    private MockWebServer server;
    private String connectionsPath;
    private String listPath;
    private String verifyPath;
    private Map<String, String> connectionFields;
    private Set<String> connectionRequired;
    private String listMember;
    private Set<String> serverMembers;
    private Set<String> verifyMembers;
    private volatile boolean actionAllowed = true;
    private volatile String listAnswer;

    @BeforeEach
    void setUp() throws IOException {
        String sdkRoutes = read(SDK_ROUTES);
        String routes = read(ROUTES);
        String attestationHandler = read(ATTESTATION_HANDLER);
        String mcpHandler = read(MCP_HANDLER);

        String sdkBase = single(SDK_BASE_PATH, sdkRoutes, "sdkAPIBasePath in " + SDK_ROUTES);
        connectionsPath = sdkBase + sdkRoute(sdkRoutes, routes, "Post", "RecordMCPConnection")
                .replace(":id", AGENT_ID);
        listPath = sdkBase + sdkRoute(sdkRoutes, routes, "Get", "ListMCPServers").replace(":id", AGENT_ID);
        verifyPath = bearerRoute(routes, "Post", "h.MCP.VerifyMCPCapability").replace(":id", SERVER_ID);

        String record = function(attestationHandler, "MCPAttestationHandler", "RecordMCPConnection");
        connectionFields = jsonFields(block(record, "type RecordConnectionRequest struct {"));
        connectionRequired = new TreeSet<>();
        Matcher required = REQUIRED_FIELD.matcher(record);
        while (required.find()) {
            String tag = connectionFields.get(required.group(1));
            assertNotNull(tag, "RecordMCPConnection requires req." + required.group(1) + ", which it does not bind");
            connectionRequired.add(tag);
        }
        assertFalse(connectionRequired.isEmpty(), "no required member found in RecordMCPConnection");

        String list = function(mcpHandler, "MCPHandler", "ListMCPServers");
        assertFalse(list.contains("c.Query("), "ListMCPServers reads a query parameter; listServers does not send one");
        listMember = single(Pattern.compile("\"(\\w+)\":\\s+enriched,"), list, "the list member of ListMCPServers");
        serverMembers = mapKeys(block(mcpHandler, "func mcpServerResponse("));

        String verify = function(mcpHandler, "MCPHandler", "VerifyMCPCapability");
        verifyMembers = new TreeSet<>(jsonFields(block(verify, "var req struct {")).values());
        assertTrue(Pattern.compile("\"allowed\":\\s+true").matcher(verify).find(),
                "VerifyMCPCapability does not answer \"allowed\": true");
        assertTrue(Pattern.compile("\"allowed\":\\s+false").matcher(verify).find(),
                "VerifyMCPCapability does not answer \"allowed\": false");

        server = new MockWebServer();
        server.setDispatcher(new Dispatcher() {
            @Override
            public MockResponse dispatch(RecordedRequest request) {
                String path = request.getRequestUrl().encodedPath();
                String method = request.getMethod();
                if ("POST".equals(method) && connectionsPath.equals(path)) {
                    return recordConnection(request);
                }
                if ("GET".equals(method) && listPath.equals(path)) {
                    return json(200, listAnswer);
                }
                if ("POST".equals(method) && verifyPath.equals(path)) {
                    return actionAllowed
                            ? json(200, "{\"allowed\":true,\"reason\":\"MCP server is verified and authorized\"}")
                            : json(403, "{\"allowed\":false,\"reason\":\"MCP server not verified\"}");
                }
                return json(404, "{\"error\":\"Cannot " + method + " " + path + "\"}");
            }
        });
        server.start();
        listAnswer = "{\"" + listMember + "\":[" + serverJson(1) + "," + serverJson(2) + "," + serverJson(3)
                + "],\"total\":3}";
    }

    @AfterEach
    void tearDown() throws IOException {
        server.shutdown();
    }

    @Test
    @DisplayName("recordToolUsage sends only members the handler binds, its required ones included")
    void recordToolUsage_sendsTheBoundMembers() throws Exception {
        Map<String, Object> result = MCPIntegration.recordToolUsage(
                client(), SERVER_ID, "read_file", "http://localhost:3000", "filesystem-mcp");

        assertEquals(Boolean.TRUE, result.get("success"), "answer: " + result);
        RecordedRequest request = only();
        JsonNode body = objectMapper.readTree(request.getBody().readUtf8());
        Set<String> sent = names(body);
        Set<String> unbound = new TreeSet<>(sent);
        unbound.removeAll(connectionFields.values());
        assertTrue(unbound.isEmpty(), "members RecordMCPConnection does not bind: " + unbound);
        assertTrue(sent.containsAll(connectionRequired), "sent " + sent + ", required " + connectionRequired);
        assertEquals(SERVER_ID, body.path("mcpServerId").asText());
        assertEquals("read_file", body.path("toolName").asText());
        assertEquals("http://localhost:3000", body.path("mcpUrl").asText());
        assertEquals("filesystem-mcp", body.path("mcpName").asText());
    }

    @Test
    @DisplayName("listServers reads the list from the member ListMCPServers answers with")
    void listServers_readsTheAnsweredList() throws Exception {
        for (String member : List.of("id", "name", "url", "description", "version", "publicKey",
                "capabilities", "status", "trustScore")) {
            assertTrue(serverMembers.contains(member), "mcpServerResponse has no " + member + ": " + serverMembers);
        }

        List<MCPServerInfo> servers = MCPIntegration.listServers(client());

        assertEquals(3, servers.size());
        MCPServerInfo first = servers.get(0);
        assertEquals(serverId(1), first.getId());
        assertEquals("server-1", first.getName());
        assertEquals("http://localhost:3001", first.getUrl());
        assertEquals("MCP server 1", first.getDescription());
        assertEquals("1.0.1", first.getVersion());
        assertEquals("FAKE-PUBLIC-KEY-1", first.getPublicKey());
        assertEquals(List.of("read_file"), first.getCapabilities());
        assertEquals("verified", first.getStatus());
        assertEquals(0.81, first.getTrustScore(), 1e-9);

        RecordedRequest request = only();
        assertEquals(listPath, request.getRequestUrl().encodedPath());
        assertNull(request.getRequestUrl().encodedQuery(), "ListMCPServers reads no query parameter");
    }

    @Test
    @DisplayName("listServers with a limit and an offset returns that slice of the answer")
    void listServers_slicesTheAnswer() {
        List<MCPServerInfo> middle = MCPIntegration.listServers(client(), 1, 1);
        assertEquals(1, middle.size());
        assertEquals(serverId(2), middle.get(0).getId());

        assertEquals(2, MCPIntegration.listServers(client(), 5, 1).size());
        assertEquals(0, MCPIntegration.listServers(client(), 5, 3).size());
        assertEquals(0, MCPIntegration.listServers(client(), 0, 0).size());
        assertEquals(3, MCPIntegration.listServers(client(), Integer.MAX_VALUE, 0).size());
    }

    @Test
    @DisplayName("listServers refuses a negative limit or offset before sending anything")
    void listServers_refusesNegativeArguments() {
        assertThrows(IllegalArgumentException.class, () -> MCPIntegration.listServers(client(), -1, 0));
        assertThrows(IllegalArgumentException.class, () -> MCPIntegration.listServers(client(), 1, -1));
        assertEquals(0, server.getRequestCount());
    }

    @Test
    @DisplayName("listServers throws when the answer carries no list member")
    void listServers_throwsWithoutTheListMember() {
        listAnswer = "{\"servers\":[" + serverJson(1) + "],\"total\":1}";

        AIMException thrown = assertThrows(AIMException.class, () -> MCPIntegration.listServers(client()));
        assertTrue(thrown.getMessage().contains(listMember), thrown.getMessage());
    }

    @Test
    @DisplayName("verifyAction posts the bound members to the registered verify-capability route")
    void verifyAction_reachesTheRegisteredRoute() throws Exception {
        assertTrue(MCPIntegration.verifyAction(client(), SERVER_ID, "read_file"));

        RecordedRequest request = only();
        assertEquals("POST", request.getMethod());
        assertEquals(verifyPath, request.getRequestUrl().encodedPath());
        assertEquals("Bearer " + ACCESS_TOKEN, request.getHeader("Authorization"));
        JsonNode body = objectMapper.readTree(request.getBody().readUtf8());
        Set<String> unbound = names(body);
        unbound.removeAll(verifyMembers);
        assertTrue(unbound.isEmpty(), "members VerifyMCPCapability does not bind: " + unbound);
        assertEquals("read_file", body.path("capability").asText());
    }

    @Test
    @DisplayName("verifyAction returns false when the server refuses the action")
    void verifyAction_returnsFalseOnRefusal() {
        actionAllowed = false;

        assertFalse(MCPIntegration.verifyAction(client(), SERVER_ID, "read_file"));
        assertEquals(1, server.getRequestCount());
    }

    @ParameterizedTest(name = "[{index}] serverId={0}")
    @NullSource
    @ValueSource(strings = {"", " ", "not-a-uuid", "../agents/" + AGENT_ID, SERVER_ID + "/../x",
            SERVER_ID + "?x=1", SERVER_ID + "#x", "{" + SERVER_ID + "}"})
    @DisplayName("verifyAction refuses a server ID that is not a UUID before sending anything")
    void verifyAction_refusesAServerIdThatIsNotAUuid(String serverId) {
        assertThrows(IllegalArgumentException.class, () -> MCPIntegration.verifyAction(client(), serverId, "read_file"));
        assertEquals(0, server.getRequestCount());
    }

    @ParameterizedTest(name = "[{index}] toolName={0}")
    @NullSource
    @ValueSource(strings = {"", " "})
    @DisplayName("verifyAction refuses an empty tool name before sending anything")
    void verifyAction_refusesAnEmptyToolName(String toolName) {
        assertThrows(IllegalArgumentException.class, () -> MCPIntegration.verifyAction(client(), SERVER_ID, toolName));
        assertEquals(0, server.getRequestCount());
    }

    @Test
    @DisplayName("attestServer's refusal names the AIMClient method that signs an attestation")
    void attestServer_namesTheMethodThatSigns() throws Exception {
        AIMClient.class.getMethod("attestMcp", String.class, String.class, String.class, List.class, double.class);

        AIMException thrown = assertThrows(AIMException.class, () -> MCPIntegration.attestServer(
                client(), SERVER_ID, "http://localhost:3000", "filesystem-mcp", List.of("read_file")));
        assertTrue(thrown.getMessage().contains("AIMClient.attestMcp"), thrown.getMessage());
        assertEquals(0, server.getRequestCount());
    }

    private MockResponse recordConnection(RecordedRequest request) {
        JsonNode body;
        try {
            // A clone, so the test can still read the body it recorded.
            body = objectMapper.readTree(request.getBody().clone().readUtf8());
        } catch (IOException e) {
            return json(400, "{\"error\":\"Invalid request body\"}");
        }
        for (String member : connectionRequired) {
            if (body.path(member).asText("").isEmpty()) {
                return json(400, "{\"error\":\"" + member + " is required\"}");
            }
        }
        return json(200, "{\"success\":true,\"message\":\"MCP connection recorded successfully\"}");
    }

    private AIMClient client() {
        AIMClient aim = mock(AIMClient.class);
        String url = server.url("/").toString();
        when(aim.getAimUrl()).thenReturn(url.substring(0, url.length() - 1));
        when(aim.getAgentId()).thenReturn(AGENT_ID);
        when(aim.getAccessToken()).thenReturn(ACCESS_TOKEN);
        return aim;
    }

    private RecordedRequest only() throws InterruptedException {
        RecordedRequest request = server.takeRequest(1, TimeUnit.SECONDS);
        assertNotNull(request, "no request was sent");
        assertEquals(1, server.getRequestCount(), "expected exactly one request");
        return request;
    }

    private static String serverId(int n) {
        return "00000000-0000-4000-8000-00000000000" + n;
    }

    private static String serverJson(int n) {
        return "{\"id\":\"" + serverId(n) + "\",\"name\":\"server-" + n + "\",\"url\":\"http://localhost:300" + n
                + "\",\"description\":\"MCP server " + n + "\",\"version\":\"1.0." + n
                + "\",\"publicKey\":\"FAKE-PUBLIC-KEY-" + n + "\",\"capabilities\":[\"read_file\"]"
                + ",\"status\":\"verified\",\"trustScore\":0.8" + n + "}";
    }

    private static MockResponse json(int status, String body) {
        return new MockResponse()
                .setResponseCode(status)
                .setHeader("Content-Type", "application/json")
                .setBody(body);
    }

    private static Set<String> names(JsonNode body) {
        Set<String> names = new TreeSet<>();
        for (Iterator<String> it = body.fieldNames(); it.hasNext(); ) {
            names.add(it.next());
        }
        return names;
    }

    /**
     * The path of an SDK-API route, relative to sdkAPIBasePath, whose handler
     * the server wires to the named handler method.
     */
    private static String sdkRoute(String sdkRoutes, String routes, String method, String handler) {
        assertTrue(Pattern.compile("\\b" + handler + ":\\s+h\\.\\w+\\." + handler + ",").matcher(routes).find(),
                ROUTES + " does not wire " + handler);
        return single(Pattern.compile("Method:\\s*http\\.Method" + method + ",\\s*Path:\\s*\"([^\"]+)\",\\s*Handler:\\s*h\\."
                + handler + "\\b"), sdkRoutes, method + " route of " + handler + " in " + SDK_ROUTES);
    }

    /**
     * The full path of a route registered on a group that authenticates a
     * bearer token, with no role gate between the group and the handler.
     */
    private static String bearerRoute(String routes, String method, String handler) {
        Matcher route = Pattern.compile("(\\w+)\\." + method + "\\(\"([^\"]+)\",\\s*" + Pattern.quote(handler) + "\\)")
                .matcher(routes);
        assertTrue(route.find(), ROUTES + " registers no " + method + " route for " + handler);
        String group = route.group(1);
        String prefix = single(Pattern.compile(group + " := v1\\.Group\\(\"([^\"]+)\"\\)"), routes,
                "the prefix of the " + group + " group");
        assertTrue(routes.contains(group + ".Use(middleware.AuthMiddleware(jwtService))"),
                "the " + group + " group does not authenticate a bearer token");
        return single(V1_GROUP, routes, "the v1 group") + prefix + route.group(2);
    }

    private static Map<String, String> jsonFields(String struct) {
        Map<String, String> fields = new HashMap<>();
        Matcher field = JSON_FIELD.matcher(struct);
        while (field.find()) {
            fields.put(field.group(1), field.group(2));
        }
        assertFalse(fields.isEmpty(), "no JSON member found in " + struct);
        return fields;
    }

    private static Set<String> mapKeys(String source) {
        Set<String> keys = new TreeSet<>();
        Matcher key = MAP_KEY.matcher(source);
        while (key.find()) {
            keys.add(key.group(1));
        }
        return keys;
    }

    private static String function(String source, String receiver, String name) {
        return block(source, "func (h *" + receiver + ") " + name + "(");
    }

    /** From the opening line to the first line that is a lone closing brace at the opening's indent. */
    private static String block(String source, String opening) {
        int start = source.indexOf(opening);
        assertTrue(start >= 0, "not found: " + opening);
        int lineStart = source.lastIndexOf('\n', start) + 1;
        String indent = source.substring(lineStart, start);
        int end = source.indexOf("\n" + indent + "}", start);
        assertTrue(end >= 0, "no end found for: " + opening);
        return source.substring(start, end);
    }

    private static String single(Pattern pattern, String source, String what) {
        Matcher matcher = pattern.matcher(source);
        assertTrue(matcher.find(), what + " not found");
        String found = matcher.group(1);
        assertFalse(matcher.find(), what + " found more than once");
        return found;
    }

    private static String read(String repoRelative) throws IOException {
        Path start = Paths.get("").toAbsolutePath();
        for (Path dir = start; dir != null; dir = dir.getParent()) {
            Path candidate = dir.resolve(repoRelative);
            if (Files.isRegularFile(candidate)) {
                return new String(Files.readAllBytes(candidate), StandardCharsets.UTF_8);
            }
        }
        return fail(repoRelative + " was not found above " + start
                + ". This test reads the backend source and has to run inside the repository.");
    }
}
