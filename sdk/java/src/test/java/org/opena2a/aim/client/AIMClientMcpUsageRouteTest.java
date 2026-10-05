package org.opena2a.aim.client;

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

import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.time.Instant;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Locale;
import java.util.Map;
import java.util.UUID;
import java.util.concurrent.TimeUnit;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.junit.jupiter.api.Assertions.fail;

/**
 * Checks AIMClient.useMcpTool against the routes the backend registers.
 *
 * The stand-in server answers the way the backend's router does: a request is
 * served only when its method and path are in the SDK-API route table, read
 * here from the backend source, and gets a 404 otherwise. A test that chose its
 * own path would agree with the client while the real server refused the call.
 */
class AIMClientMcpUsageRouteTest {

    /** Repo-relative location of the backend's SDK-API route table. */
    private static final String ROUTE_TABLE = "apps/backend/cmd/server/sdk_api_routes.go";

    private static final String REFRESH_PATH = "/api/v1/auth/refresh";

    private static final Pattern STRING_CONSTANT = Pattern.compile("(\\w+)\\s*=\\s*\"([^\"]*)\"");
    private static final Pattern ROUTE_ENTRY =
            Pattern.compile("Method:\\s*http\\.Method(\\w+),\\s*Path:\\s*(?:\"([^\"]+)\"|(\\w+))");

    private final ObjectMapper objectMapper = new ObjectMapper();

    private MockWebServer server;
    private List<Route> registeredRoutes;

    @BeforeEach
    void setUp() throws IOException {
        registeredRoutes = readRegisteredRoutes();
        server = new MockWebServer();
        server.setDispatcher(new Dispatcher() {
            @Override
            public MockResponse dispatch(RecordedRequest request) {
                String path = request.getRequestUrl().encodedPath();
                if (REFRESH_PATH.equals(path)) {
                    return json(200, "{\"accessToken\":\"test-access-token\"}");
                }
                if (isRegistered(request.getMethod(), path)) {
                    return json(200, "{\"success\":true,\"serversReported\":1,\"totalInvocations\":1}");
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
    @DisplayName("useMcpTool posts a usage report to a route the backend registers")
    void useMcpTool_postsUsageReportToRegisteredRoute() throws Exception {
        String agentId = UUID.randomUUID().toString();
        String serverId = UUID.randomUUID().toString();

        AIMClient client = new AIMClient.Builder()
                .agentName("usage-route-agent")
                .aimUrl(baseUrl())
                .agentId(agentId)
                .refreshToken("test-refresh-token")
                .build();

        Map<String, Object> result;
        try {
            result = client.useMcpTool(serverId, "read_file", "http://localhost:3001", "filesystem-mcp");
        } finally {
            client.close();
        }

        RecordedRequest usage = takeRequestAfterRefresh();
        String path = usage.getRequestUrl().encodedPath();

        assertEquals("POST", usage.getMethod());
        assertTrue(isRegistered("POST", path),
                "useMcpTool posted to " + path + ", which the backend does not register. Registered routes:\n"
                        + formatRoutes());
        assertEquals("/api/v1/sdk-api/agents/" + agentId + "/mcp-usage-report", path);
        assertEquals(Boolean.TRUE, result.get("success"),
                "the server answered the usage report with an error: " + result.get("error"));

        // The body is the report the route's handler reads: usage keyed by MCP
        // server id, then by tool name.
        JsonNode body = objectMapper.readTree(usage.getBody().readUtf8());
        assertEquals(agentId, body.path("agentId").asText());
        assertTimestamp(body, "reportedAt");
        assertEquals(1, body.path("mcpServers").size(), "one call reports one MCP server");

        JsonNode toolUsage = body.path("mcpServers").path(serverId).path("toolUsage");
        assertEquals(1, toolUsage.size(), "one call reports one tool");

        JsonNode entry = toolUsage.path("read_file");
        assertTrue(entry.path("count").isInt(), "count must be a JSON number, got " + entry.path("count"));
        assertEquals(1, entry.path("count").asInt());
        assertTimestamp(entry, "firstUsed");
        assertTimestamp(entry, "lastUsed");
    }

    @Test
    @DisplayName("the route table read from the backend source has the MCP usage report route")
    void routeTable_isReadFromBackendSource() {
        // Guards the reader itself: a pattern that stopped matching the table
        // would turn every request into a 404 for the wrong reason.
        assertTrue(isRegistered("POST", "/api/v1/sdk-api/agents/" + UUID.randomUUID() + "/mcp-usage-report"),
                "POST /api/v1/sdk-api/agents/:id/mcp-usage-report was not found in " + ROUTE_TABLE
                        + ". Registered routes:\n" + formatRoutes());
        assertFalse(isRegistered("POST", "/api/v1/sdk-api/agents/" + UUID.randomUUID() + "/not-a-route"));
    }

    private RecordedRequest takeRequestAfterRefresh() throws InterruptedException {
        RecordedRequest request = server.takeRequest(5, TimeUnit.SECONDS);
        assertNotNull(request, "the client sent no request");
        if (REFRESH_PATH.equals(request.getRequestUrl().encodedPath())) {
            request = server.takeRequest(5, TimeUnit.SECONDS);
            assertNotNull(request, "the client authenticated and then sent no request");
        }
        return request;
    }

    private static void assertTimestamp(JsonNode node, String field) {
        String value = node.path(field).asText();
        assertFalse(value.isEmpty(), field + " is missing");
        Instant.parse(value);
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

    private boolean isRegistered(String method, String path) {
        for (Route route : registeredRoutes) {
            if (route.method.equals(method) && route.matcher.matcher(path).matches()) {
                return true;
            }
        }
        return false;
    }

    private String formatRoutes() {
        StringBuilder out = new StringBuilder();
        for (Route route : registeredRoutes) {
            out.append("  ").append(route.method).append(' ').append(route.pattern).append('\n');
        }
        return out.toString();
    }

    /**
     * Reads every route of the backend's SDK-API route table: the method, and
     * the full path with its base prefix. A path written as a named constant is
     * resolved from the same file.
     */
    private static List<Route> readRegisteredRoutes() throws IOException {
        String source = new String(Files.readAllBytes(locateRouteTable()), StandardCharsets.UTF_8);

        Map<String, String> constants = new HashMap<>();
        Matcher constant = STRING_CONSTANT.matcher(source);
        while (constant.find()) {
            constants.put(constant.group(1), constant.group(2));
        }
        String basePath = constants.get("sdkAPIBasePath");
        assertNotNull(basePath, "sdkAPIBasePath is not declared in " + ROUTE_TABLE);

        List<Route> routes = new ArrayList<>();
        Matcher entry = ROUTE_ENTRY.matcher(source);
        while (entry.find()) {
            String relative = entry.group(2) != null ? entry.group(2) : constants.get(entry.group(3));
            assertNotNull(relative, "route path " + entry.group(3) + " is not a string constant in " + ROUTE_TABLE);
            routes.add(new Route(entry.group(1).toUpperCase(Locale.ROOT), basePath + relative));
        }
        assertFalse(routes.isEmpty(), "no route entries found in " + ROUTE_TABLE);
        return routes;
    }

    private static Path locateRouteTable() {
        Path start = Paths.get("").toAbsolutePath();
        for (Path dir = start; dir != null; dir = dir.getParent()) {
            Path candidate = dir.resolve(ROUTE_TABLE);
            if (Files.isRegularFile(candidate)) {
                return candidate;
            }
        }
        return fail(ROUTE_TABLE + " was not found above " + start
                + ". This test reads the backend's route table and has to run inside the repository.");
    }

    /** One registered route; a ":name" path segment matches any single segment. */
    private static final class Route {
        final String method;
        final String pattern;
        final Pattern matcher;

        Route(String method, String pattern) {
            this.method = method;
            this.pattern = pattern;

            StringBuilder regex = new StringBuilder();
            for (String segment : pattern.split("/")) {
                if (segment.isEmpty()) {
                    continue;
                }
                regex.append('/').append(segment.startsWith(":") ? "[^/]+" : Pattern.quote(segment));
            }
            this.matcher = Pattern.compile(regex.toString());
        }
    }
}
