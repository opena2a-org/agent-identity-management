package org.opena2a.aim.client;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ArrayNode;
import com.fasterxml.jackson.databind.node.ObjectNode;
import okhttp3.mockwebserver.Dispatcher;
import okhttp3.mockwebserver.MockResponse;
import okhttp3.mockwebserver.MockWebServer;
import okhttp3.mockwebserver.RecordedRequest;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.opena2a.aim.exceptions.AIMException;

import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.util.ArrayList;
import java.util.Iterator;
import java.util.List;
import java.util.Map;
import java.util.TreeSet;
import java.util.concurrent.TimeUnit;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.junit.jupiter.api.Assertions.fail;

/**
 * Checks the request {@link AIMClient#registerMcp} sends against the backend
 * route that adds MCP servers to an agent's list.
 *
 * {@code PUT /api/v1/agents/{id}/mcp-servers} ({@code AddMCPServersToAgent})
 * binds {@code AddMCPServersRequest}, whose JSON members are read here from the
 * backend source, and sits behind the member role gate, read from the backend's
 * route registration. The client posted {@code mcp_server_ids} and
 * {@code detected_method} to {@code POST /api/v1/sdk-api/agents/{id}/mcp-servers},
 * the route that creates an MCP server and reads neither key, so the given
 * server was never added to the agent's list.
 *
 * The stand-in server answers both routes the way the backend does: the attach
 * route reads only the bound members and admits only a member's token, and the
 * create route creates a server from {@code name} and {@code url}.
 *
 * The {@code registerMcp} example in the SDK documentation runs against the same
 * stand-in, and every member it prints has to be one the backend answers with.
 */
class AIMClientRegisterMcpRouteTest {

    private static final String ROUTES = "apps/backend/cmd/server/main.go";
    private static final String REQUEST_TYPE = "apps/backend/internal/application/agent_service.go";
    private static final String HANDLER = "apps/backend/internal/interfaces/http/handlers/agent_handler.go";
    private static final String DOCUMENT = "docs/sdk/java.md";

    private static final String AGENT_ID = "550e8400-e29b-41d4-a716-446655440000";
    private static final String ATTACH_PATH = "/api/v1/agents/" + AGENT_ID + "/mcp-servers";
    private static final String CREATE_PATH = "/api/v1/sdk-api/agents/" + AGENT_ID + "/mcp-servers";
    private static final String REFRESH_PATH = "/api/v1/auth/refresh";

    private static final String MCP_SERVER_ID = "7c9e6679-7425-40de-944b-e07fc1f90ae7";
    private static final String MEMBER_TOKEN = "member-token-not-real";
    private static final String VIEWER_TOKEN = "viewer-token-not-real";

    private static final Pattern ATTACH_ROUTE = Pattern.compile(
            "\\.Put\\(\"/:id/mcp-servers\",\\s*middleware\\.MemberMiddleware\\(\\),\\s*h\\.Agent\\.AddMCPServersToAgent\\)");
    private static final Pattern REQUEST_STRUCT =
            Pattern.compile("type AddMCPServersRequest struct \\{(.*?)\\n\\}", Pattern.DOTALL);
    private static final Pattern JSON_MEMBER = Pattern.compile("`json:\"([^\",`]+)");
    private static final Pattern HANDLER_FUNC = Pattern.compile(
            "func \\(h \\*AgentHandler\\) AddMCPServersToAgent\\(.*?\\n}\\n", Pattern.DOTALL);
    private static final Pattern ANSWER_MAP =
            Pattern.compile("return c\\.JSON\\(fiber\\.Map\\{(.*?)\\}\\)", Pattern.DOTALL);
    private static final Pattern MAP_KEY = Pattern.compile("\"([^\"]+)\"\\s*:");
    private static final Pattern DOCUMENTED_CALL = Pattern.compile(
            "Map<String, Object> (\\w+) = agent\\.registerMcp\\((.*?)\\);", Pattern.DOTALL);
    private static final Pattern LINE_COMMENT = Pattern.compile("//[^\\n]*");
    private static final Pattern QUOTED = Pattern.compile("\"([^\"]+)\"");
    private static final Pattern SCALE = Pattern.compile("\\((\\d+)-(\\d+)\\)");

    private final ObjectMapper objectMapper = new ObjectMapper();
    private final List<String> talksTo = new ArrayList<>();

    private MockWebServer server;
    private List<String> boundMembers;
    private volatile String signedInToken;

    @BeforeEach
    void setUp() throws IOException {
        boundMembers = readBoundMembers();
        server = new MockWebServer();
        server.setDispatcher(new Dispatcher() {
            @Override
            public MockResponse dispatch(RecordedRequest request) {
                String path = request.getRequestUrl().encodedPath();
                if (REFRESH_PATH.equals(path)) {
                    return json(200, "{\"accessToken\":\"" + signedInToken + "\",\"tokenType\":\"Bearer\"}");
                }
                if ("PUT".equals(request.getMethod()) && ATTACH_PATH.equals(path)) {
                    return attach(request);
                }
                if ("POST".equals(request.getMethod()) && CREATE_PATH.equals(path)) {
                    return create(request);
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

    @Test
    @DisplayName("registerMcp adds the server to the agent's list with the members the backend binds")
    void registerMcp_addsTheServerWithTheBoundMembers() throws Exception {
        signedInToken = MEMBER_TOKEN;

        Map<String, Object> result;
        try (AIMClient client = client()) {
            result = client.registerMcp(MCP_SERVER_ID, "auto_sdk", 85.0);
        }

        List<RecordedRequest> requests = drainRequests();
        RecordedRequest attach = only(requests);
        assertEquals("PUT", attach.getMethod(), "requests: " + paths(requests));
        assertEquals(ATTACH_PATH, attach.getRequestUrl().encodedPath(), "requests: " + paths(requests));
        assertEquals("Bearer " + MEMBER_TOKEN, attach.getHeader("Authorization"));

        JsonNode body = objectMapper.readTree(attach.getBody().readUtf8());
        TreeSet<String> sent = new TreeSet<>();
        body.fieldNames().forEachRemaining(sent::add);
        assertTrue(boundMembers.containsAll(sent),
                "the body carries members AddMCPServersRequest does not declare in " + REQUEST_TYPE
                        + ": sent " + sent + ", bound " + boundMembers);
        assertEquals(MCP_SERVER_ID, body.path("mcpServerIds").path(0).asText(), "body: " + body);
        assertEquals(1, body.path("mcpServerIds").size(), "body: " + body);
        assertEquals("auto_sdk", body.path("detectedMethod").asText(), "body: " + body);
        assertEquals(85.0, body.path("confidence").asDouble(), "body: " + body);

        assertEquals(List.of(MCP_SERVER_ID), result.get("talksTo"), "result: " + result);
        assertEquals(List.of(MCP_SERVER_ID), result.get("added_servers"), "result: " + result);
        assertEquals(1, ((Number) result.get("total_count")).intValue(), "result: " + result);
    }

    @Test
    @DisplayName("the one-argument registerMcp sends detectedMethod manual and confidence 100")
    void registerMcp_defaults() throws Exception {
        signedInToken = MEMBER_TOKEN;

        try (AIMClient client = client()) {
            client.registerMcp(MCP_SERVER_ID);
        }

        JsonNode body = objectMapper.readTree(only(drainRequests()).getBody().readUtf8());
        assertEquals("manual", body.path("detectedMethod").asText(), "body: " + body);
        assertEquals(100.0, body.path("confidence").asDouble(), "body: " + body);
    }

    @Test
    @DisplayName("a token below the member role is refused, and registerMcp raises")
    void registerMcp_belowMember_raises() throws Exception {
        signedInToken = VIEWER_TOKEN;

        try (AIMClient client = client()) {
            AIMException refused = assertThrows(AIMException.class,
                    () -> client.registerMcp(MCP_SERVER_ID));
            assertTrue(refused.getMessage().contains("403"), refused.getMessage());
        }
        assertTrue(talksTo.isEmpty(), "a refused call added a server: " + talksTo);
    }

    @Test
    @DisplayName("the backend registers the attach route behind the member role gate")
    void backend_registersTheAttachRouteBehindTheMemberGate() throws IOException {
        // Guards the stand-in: it answers this path with this method and admits
        // only a member's token, because the backend registers the route so.
        String source = new String(Files.readAllBytes(locate(ROUTES)), StandardCharsets.UTF_8);
        assertTrue(ATTACH_ROUTE.matcher(source).find(),
                "PUT /:id/mcp-servers is no longer registered for AddMCPServersToAgent behind MemberMiddleware in "
                        + ROUTES);
        assertTrue(boundMembers.contains("mcpServerIds"), "bound members: " + boundMembers);
    }

    @Test
    @DisplayName("the documented registerMcp example sends what the route reads and prints what it answers")
    void documentedExample_matchesTheRoute() throws Exception {
        String document = new String(Files.readAllBytes(locate(DOCUMENT)), StandardCharsets.UTF_8);
        Matcher call = DOCUMENTED_CALL.matcher(document);
        assertTrue(call.find(), "no `Map<String, Object> result = agent.registerMcp(...)` example in " + DOCUMENT);
        String resultName = call.group(1);
        String[] arguments = LINE_COMMENT.matcher(call.group(2)).replaceAll("").split(",");
        assertEquals(3, arguments.length, "documented call: " + call.group());
        String serverId = unquote(arguments[0]);
        String detectionMethod = unquote(arguments[1]);
        double confidence = Double.parseDouble(arguments[2].trim());

        List<String> methods = new ArrayList<>();
        Matcher method = QUOTED.matcher(memberComment("detectedMethod"));
        while (method.find()) {
            methods.add(method.group(1));
        }
        assertTrue(methods.contains(detectionMethod), "the example's detection method \"" + detectionMethod
                + "\" is not one AddMCPServersRequest in " + REQUEST_TYPE + " lists: " + methods);
        Matcher scale = SCALE.matcher(memberComment("confidence"));
        assertTrue(scale.find(), "no confidence scale on AddMCPServersRequest in " + REQUEST_TYPE);
        assertTrue(confidence >= Double.parseDouble(scale.group(1)) && confidence <= Double.parseDouble(scale.group(2)),
                "the example's confidence " + confidence + " is outside " + scale.group());

        int blockEnd = document.indexOf("```", call.end());
        String block = document.substring(call.start(), blockEnd < 0 ? document.length() : blockEnd);
        Matcher read = Pattern.compile(Pattern.quote(resultName) + "\\.get(?:OrDefault)?\\(\"([^\"]+)\"")
                .matcher(block);
        TreeSet<String> reads = new TreeSet<>();
        while (read.find()) {
            reads.add(read.group(1));
        }
        assertTrue(!reads.isEmpty(), "the example prints nothing from " + resultName + ": " + block);
        TreeSet<String> answered = handlerAnswerMembers();
        assertTrue(answered.containsAll(reads), "the example prints members AddMCPServersToAgent in " + HANDLER
                + " does not answer with: prints " + reads + ", answered " + answered);

        signedInToken = MEMBER_TOKEN;
        Map<String, Object> result;
        try (AIMClient client = client()) {
            result = client.registerMcp(serverId, detectionMethod, confidence);
        }
        for (String member : reads) {
            assertNotNull(result.get(member),
                    "the example prints " + resultName + ".get(\"" + member + "\"), which is null in " + result);
        }
    }

    private AIMClient client() {
        return new AIMClient.Builder()
                .agentName("register-mcp-agent")
                .aimUrl(baseUrl())
                .agentId(AGENT_ID)
                .refreshToken("sdk-refresh-token-not-real")
                .build();
    }

    private MockResponse attach(RecordedRequest request) {
        String authorization = request.getHeader("Authorization");
        if (("Bearer " + VIEWER_TOKEN).equals(authorization)) {
            return json(403, "{\"error\":\"Member access required (viewers cannot perform this action)\"}");
        }
        if (!("Bearer " + MEMBER_TOKEN).equals(authorization)) {
            return json(401, "{\"error\":\"Authentication required\"}");
        }
        JsonNode body;
        try {
            body = objectMapper.readTree(request.getBody().clone().readUtf8());
        } catch (IOException e) {
            return json(400, "{\"error\":\"Invalid request body\"}");
        }
        // Members the request type does not declare are not read.
        List<String> identifiers = new ArrayList<>();
        JsonNode ids = boundMembers.contains("mcpServerIds") ? body.path("mcpServerIds") : null;
        if (ids != null && ids.isArray()) {
            for (Iterator<JsonNode> it = ids.elements(); it.hasNext(); ) {
                identifiers.add(it.next().asText());
            }
        }
        if (identifiers.isEmpty()) {
            return json(400, "{\"error\":\"mcp_server_ids is required and must not be empty\"}");
        }
        List<String> added = new ArrayList<>();
        for (String id : identifiers) {
            if (!talksTo.contains(id)) {
                talksTo.add(id);
                added.add(id);
            }
        }
        ObjectNode answer = objectMapper.createObjectNode();
        answer.put("message", "Successfully added " + added.size() + " MCP server(s)");
        ArrayNode talks = answer.putArray("talksTo");
        talksTo.forEach(talks::add);
        ArrayNode addedNode = answer.putArray("added_servers");
        added.forEach(addedNode::add);
        answer.put("total_count", talksTo.size());
        return json(200, answer.toString());
    }

    private MockResponse create(RecordedRequest request) {
        // CreateMCPServer reads name and url, and none of the attach members.
        JsonNode body;
        try {
            body = objectMapper.readTree(request.getBody().clone().readUtf8());
        } catch (IOException e) {
            return json(400, "{\"error\":\"Invalid request body\"}");
        }
        ObjectNode created = objectMapper.createObjectNode();
        created.put("id", "0b8a3c1e-2f4d-4a6b-9c8d-7e6f5a4b3c2d");
        created.put("name", body.path("name").asText(""));
        created.put("url", body.path("url").asText(""));
        return json(201, created.toString());
    }

    /** The one request after the sign-in refresh. */
    private static RecordedRequest only(List<RecordedRequest> requests) {
        List<RecordedRequest> calls = new ArrayList<>();
        for (RecordedRequest request : requests) {
            if (!REFRESH_PATH.equals(request.getRequestUrl().encodedPath())) {
                calls.add(request);
            }
        }
        assertEquals(1, calls.size(), "requests: " + paths(requests));
        return calls.get(0);
    }

    private List<RecordedRequest> drainRequests() throws InterruptedException {
        List<RecordedRequest> requests = new ArrayList<>();
        RecordedRequest request;
        while ((request = server.takeRequest(1, TimeUnit.SECONDS)) != null) {
            requests.add(request);
        }
        return requests;
    }

    private static List<String> paths(List<RecordedRequest> requests) {
        List<String> out = new ArrayList<>();
        for (RecordedRequest request : requests) {
            out.add(request.getMethod() + " " + request.getRequestUrl().encodedPath());
        }
        return out;
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

    /** The JSON members of the request type the attach route binds. */
    private static List<String> readBoundMembers() throws IOException {
        String source = new String(Files.readAllBytes(locate(REQUEST_TYPE)), StandardCharsets.UTF_8);
        Matcher declared = REQUEST_STRUCT.matcher(source);
        assertTrue(declared.find(), "AddMCPServersRequest is not declared in " + REQUEST_TYPE);
        List<String> members = new ArrayList<>();
        Matcher member = JSON_MEMBER.matcher(declared.group(1));
        while (member.find()) {
            members.add(member.group(1));
        }
        assertNotNull(members);
        assertTrue(!members.isEmpty(), "AddMCPServersRequest has no JSON members in " + REQUEST_TYPE);
        return members;
    }

    /** The line comment on one JSON member of the request type the attach route binds. */
    private static String memberComment(String member) throws IOException {
        String source = new String(Files.readAllBytes(locate(REQUEST_TYPE)), StandardCharsets.UTF_8);
        Matcher declared = REQUEST_STRUCT.matcher(source);
        assertTrue(declared.find(), "AddMCPServersRequest is not declared in " + REQUEST_TYPE);
        Matcher comment = Pattern.compile("`json:\"" + Pattern.quote(member) + "\"`[^\\n]*?//([^\\n]*)")
                .matcher(declared.group(1));
        assertTrue(comment.find(), "AddMCPServersRequest declares no commented " + member + " in " + REQUEST_TYPE);
        return comment.group(1);
    }

    /** The members AddMCPServersToAgent answers with. */
    private static TreeSet<String> handlerAnswerMembers() throws IOException {
        String source = new String(Files.readAllBytes(locate(HANDLER)), StandardCharsets.UTF_8);
        Matcher handler = HANDLER_FUNC.matcher(source);
        assertTrue(handler.find(), "AddMCPServersToAgent is not declared in " + HANDLER);
        Matcher answer = ANSWER_MAP.matcher(handler.group());
        assertTrue(answer.find(), "AddMCPServersToAgent answers no fiber.Map in " + HANDLER);
        TreeSet<String> members = new TreeSet<>();
        Matcher key = MAP_KEY.matcher(answer.group(1));
        while (key.find()) {
            members.add(key.group(1));
        }
        return members;
    }

    private static String unquote(String argument) {
        Matcher literal = QUOTED.matcher(argument.trim());
        assertTrue(literal.matches(), "not a string literal: " + argument);
        return literal.group(1);
    }

    private static Path locate(String relative) {
        Path start = Paths.get("").toAbsolutePath();
        for (Path dir = start; dir != null; dir = dir.getParent()) {
            Path candidate = dir.resolve(relative);
            if (Files.isRegularFile(candidate)) {
                return candidate;
            }
        }
        return fail(relative + " was not found above " + start
                + ". This test reads the backend source and has to run inside the repository.");
    }
}
