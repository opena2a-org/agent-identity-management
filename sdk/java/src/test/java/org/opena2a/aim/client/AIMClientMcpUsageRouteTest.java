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
import java.time.Instant;
import java.util.Map;
import java.util.UUID;
import java.util.concurrent.TimeUnit;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertTrue;

/**
 * Checks AIMClient.useMcpTool against the routes the backend registers.
 *
 * The stand-in server answers the way the backend's router does: a request is
 * served only when its method and path are in the SDK-API route table, read
 * from the backend source by {@link SdkApiRouteTable}, and gets a 404
 * otherwise.
 */
class AIMClientMcpUsageRouteTest {

    private static final String REFRESH_PATH = "/api/v1/auth/refresh";

    private final ObjectMapper objectMapper = new ObjectMapper();

    private MockWebServer server;
    private SdkApiRouteTable registeredRoutes;

    @BeforeEach
    void setUp() throws IOException {
        registeredRoutes = SdkApiRouteTable.read();
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
                        + registeredRoutes.format());
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
                "POST /api/v1/sdk-api/agents/:id/mcp-usage-report was not found in " + SdkApiRouteTable.ROUTE_TABLE
                        + ". Registered routes:\n" + registeredRoutes.format());
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
        return registeredRoutes.isRegistered(method, path);
    }
}
