package org.opena2a.aim.client;

import com.fasterxml.jackson.core.type.TypeReference;
import com.fasterxml.jackson.databind.ObjectMapper;
import okhttp3.mockwebserver.Dispatcher;
import okhttp3.mockwebserver.MockResponse;
import okhttp3.mockwebserver.MockWebServer;
import okhttp3.mockwebserver.RecordedRequest;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.opena2a.aim.exceptions.AIMException;
import org.opena2a.aim.exceptions.ConfigurationException;

import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.io.PrintStream;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Collections;
import java.util.HashMap;
import java.util.HashSet;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.UUID;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

/**
 * Checks capability registration and reporting against the routes the backend
 * registers and the outcomes its registration handler answers with.
 *
 * The stand-in server serves only the SDK-API route table read from the
 * backend source. The registration route answers the way RegisterCapability
 * does: 201 granted in monitoring mode, 202 pending with a request id in strict
 * mode, and 409 already_exists or pending on a repeat. The legacy grant route
 * answers the way GrantCapability does: 201 with the granted capability,
 * whatever the enforcement mode.
 */
class AIMClientCapabilityRegistrationTest {

    private static final String REFRESH_PATH = "/api/v1/auth/refresh";
    private static final String REGISTER_ROUTE = "/api/v1/sdk-api/agents/:id/capabilities/register";
    private static final String GRANT_ROUTE = "/api/v1/sdk-api/agents/:id/capabilities";

    private final ObjectMapper objectMapper = new ObjectMapper();

    private SdkApiRouteTable routes;
    private MockWebServer server;
    private AIMClient client;
    private String agentId;

    private volatile String mode = "monitoring";
    private final Set<String> granted = Collections.synchronizedSet(new HashSet<>());
    private final Set<String> pending = Collections.synchronizedSet(new HashSet<>());
    private final Map<String, MockResponse> planted = Collections.synchronizedMap(new HashMap<>());
    /** Every non-refresh request: method, path and parsed body. */
    private final List<Map<String, Object>> requests = Collections.synchronizedList(new ArrayList<>());

    @BeforeEach
    void setUp() throws IOException {
        routes = SdkApiRouteTable.read();
        server = new MockWebServer();
        server.setDispatcher(new Dispatcher() {
            @Override
            public MockResponse dispatch(RecordedRequest request) {
                return answer(request);
            }
        });
        server.start();

        agentId = UUID.randomUUID().toString();
        String url = server.url("/").toString();
        client = new AIMClient.Builder()
                .agentName("capability-registration-agent")
                .aimUrl(url.substring(0, url.length() - 1))
                .agentId(agentId)
                .refreshToken("test-refresh-token")
                .build();
    }

    @AfterEach
    void tearDown() throws IOException {
        client.close();
        server.shutdown();
    }

    @Test
    @DisplayName("the route table read from the backend source has the registration route")
    void routeTable_hasRegistrationRoute() {
        assertEquals(REGISTER_ROUTE, routes.routeFor("POST", registerPath()),
                "POST " + REGISTER_ROUTE + " was not found. Registered routes:\n" + routes.format());
    }

    @Test
    @DisplayName("reportCapabilities registers each distinct capability once, in order, and monitoring mode grants it")
    void reportCapabilities_monitoringModeGrants() {
        Map<String, Object> result = client.reportCapabilities(Arrays.asList("db:read", "api:call", "db:read"));

        assertEquals(Arrays.asList(
                request("POST", registerPath(), "db:read"),
                request("POST", registerPath(), "api:call")), requests);
        assertEquals(2, result.get("granted"));
        assertEquals(0, result.get("pending"));
        assertEquals(2, result.get("total"));
        assertEquals(Arrays.asList("granted", "granted"), statuses(result));
    }

    @Test
    @DisplayName("in strict mode reportCapabilities reports pending with its request id and grants nothing")
    void reportCapabilities_strictModePending() {
        mode = "strict";

        Map<String, Object> result = client.reportCapabilities(Collections.singletonList("db:write"));

        assertTrue(granted.isEmpty(), "a strict-mode report must not grant a capability");
        assertEquals(0, result.get("granted"));
        assertEquals(1, result.get("pending"));
        assertEquals(1, result.get("total"));
        Map<String, Object> outcome = results(result).get(0);
        assertEquals("db:write", outcome.get("capabilityType"));
        assertEquals("pending", outcome.get("status"));
        assertEquals("req-db:write", outcome.get("requestId"));
    }

    @Test
    @DisplayName("a second reportCapabilities call reads both 409 outcomes")
    void reportCapabilities_secondCallReadsConflicts() {
        mode = "strict";
        granted.add("file:read");

        client.reportCapabilities(Collections.singletonList("db:write"));
        Map<String, Object> result = client.reportCapabilities(Arrays.asList("file:read", "db:write"));

        assertEquals(Arrays.asList("already_exists", "pending"), statuses(result));
        assertNull(results(result).get(1).get("requestId"));
        assertEquals(1, result.get("granted"));
        assertEquals(1, result.get("pending"));
        assertEquals(2, result.get("total"));
    }

    @Test
    @DisplayName("a 503 from reportCapabilities raises")
    void reportCapabilities_503Raises() {
        planted.put("db:read", json(503, "{\"error\":\"Service unavailable\"}"));

        AIMException e = assertThrows(AIMException.class,
                () -> client.reportCapabilities(Collections.singletonList("db:read")));
        assertEquals(503, e.getStatusCode());
    }

    @Test
    @DisplayName("a 500 from reportCapabilities raises, is not counted as a grant, and stops the report")
    void reportCapabilities_500RaisesAndIsNotCounted() {
        planted.put("api:call", json(500, "{\"error\":\"duplicate key value violates unique constraint\"}"));

        AIMException e = assertThrows(AIMException.class,
                () -> client.reportCapabilities(Arrays.asList("db:read", "api:call", "file:read")));
        assertEquals(500, e.getStatusCode());
        for (Map<String, Object> request : requests) {
            assertFalse("file:read".equals(request.get("capabilityType")),
                    "the capability after a failure must not be sent");
        }
    }

    @Test
    @DisplayName("a refusal from reportCapabilities raises")
    void reportCapabilities_refusalRaises() {
        planted.put("db:read", json(404, "{\"error\":\"Agent not found\"}"));

        AIMException e = assertThrows(AIMException.class,
                () -> client.reportCapabilities(Collections.singletonList("db:read")));
        assertEquals(404, e.getStatusCode());
    }

    @Test
    @DisplayName("an answer without a known status raises")
    void reportCapabilities_unknownStatusRaises() {
        planted.put("db:read", json(200, "{\"success\":true}"));

        assertThrows(AIMException.class,
                () -> client.reportCapabilities(Collections.singletonList("db:read")));
    }

    @Test
    @DisplayName("scope is accepted with a deprecation warning and is not sent")
    @SuppressWarnings("deprecation")
    void reportCapabilities_scopeIsDeprecated() {
        Map<String, Object> scope = new HashMap<>();
        scope.put("source", "test");

        PrintStream original = System.err;
        ByteArrayOutputStream captured = new ByteArrayOutputStream();
        Map<String, Object> result;
        System.setErr(new PrintStream(captured, true));
        try {
            result = client.reportCapabilities(Collections.singletonList("db:read"), scope);
        } finally {
            System.setErr(original);
        }

        assertEquals(Collections.singletonList(request("POST", registerPath(), "db:read")), requests);
        assertEquals(1, result.get("granted"));
        String log = new String(captured.toByteArray(), StandardCharsets.UTF_8);
        assertTrue(log.contains("scope") && log.contains("deprecated"),
                "passing scope must log a deprecation warning, logged:\n" + log);
    }

    @Test
    @DisplayName("an empty capability is refused before anything is sent")
    void reportCapabilities_emptyCapabilityRefused() {
        assertThrows(ConfigurationException.class,
                () -> client.reportCapabilities(Arrays.asList("db:read", "")));
        assertTrue(requests.isEmpty());
    }

    @Test
    @DisplayName("an empty list sends nothing")
    void reportCapabilities_emptyList() {
        Map<String, Object> result = client.reportCapabilities(Collections.emptyList());

        assertEquals(0, result.get("granted"));
        assertEquals(0, result.get("pending"));
        assertEquals(0, result.get("total"));
        assertTrue(results(result).isEmpty());
        assertTrue(requests.isEmpty());
    }

    @Test
    @DisplayName("registerCapability(capability, description) sends capabilityType")
    void registerCapability_twoArgumentsSendsCapabilityType() {
        Map<String, Object> result = client.registerCapability("db:read", "Read the orders table");

        assertEquals(1, requests.size());
        assertEquals(registerPath(), requests.get(0).get("path"));
        assertEquals("db:read", requests.get(0).get("capabilityType"));
        assertEquals("granted", result.get("status"));
    }

    @Test
    @DisplayName("registerCapability(type, description, risk) reads a pending 409 from the body")
    void registerCapability_threeArgumentsReadsConflictStatus() {
        mode = "strict";
        pending.add("db:write");

        Map<String, Object> result = client.registerCapability("db:write", "Write orders", "high");

        assertEquals("pending", result.get("status"));
        assertEquals(Boolean.TRUE, result.get("success"));
    }

    @Test
    @DisplayName("registerCapability(type, description, risk) raises on a 404")
    void registerCapability_threeArgumentsRaisesOn404() {
        planted.put("db:read", json(404, "{\"error\":\"Agent not found\"}"));

        AIMException e = assertThrows(AIMException.class,
                () -> client.registerCapability("db:read", "Read orders", "low"));
        assertEquals(404, e.getStatusCode());
    }

    // ------------------------------------------------------------------------

    private MockResponse answer(RecordedRequest request) {
        String path = request.getRequestUrl().encodedPath();
        if (REFRESH_PATH.equals(path)) {
            return json(200, "{\"accessToken\":\"test-access-token\"}");
        }
        Map<String, Object> body;
        try {
            String raw = request.getBody().readUtf8();
            body = raw.isEmpty() ? new HashMap<>()
                    : objectMapper.readValue(raw, new TypeReference<Map<String, Object>>() {});
        } catch (IOException e) {
            return json(400, "{\"error\":\"Invalid request body\"}");
        }
        Map<String, Object> recorded = new LinkedHashMap<>(body);
        recorded.put("method", request.getMethod());
        recorded.put("path", path);
        requests.add(recorded);

        String route = routes.routeFor(request.getMethod(), path);
        if (route == null) {
            return json(404, "{\"error\":\"Cannot " + request.getMethod() + " " + path + "\"}");
        }
        if (REGISTER_ROUTE.equals(route)) {
            return register(body);
        }
        if (GRANT_ROUTE.equals(route)) {
            String cap = String.valueOf(body.get("capabilityType"));
            granted.add(cap);
            return json(201, "{\"id\":\"" + UUID.randomUUID() + "\",\"capabilityType\":\"" + cap + "\"}");
        }
        return json(200, "{}");
    }

    private MockResponse register(Map<String, Object> body) {
        Object value = body.get("capabilityType");
        String cap = value instanceof String ? (String) value : "";
        MockResponse plantedResponse = planted.get(cap);
        if (plantedResponse != null) {
            return plantedResponse;
        }
        if (cap.isEmpty()) {
            return json(400, "{\"error\":\"capabilityType is required\"}");
        }
        if (granted.contains(cap)) {
            return outcome(409, cap, "already_exists", null);
        }
        if ("monitoring".equals(mode)) {
            granted.add(cap);
            return outcome(201, cap, "granted", null);
        }
        if (pending.contains(cap)) {
            return outcome(409, cap, "pending", null);
        }
        pending.add(cap);
        return outcome(202, cap, "pending", "req-" + cap);
    }

    private MockResponse outcome(int code, String cap, String status, String requestId) {
        Map<String, Object> body = new LinkedHashMap<>();
        body.put("success", true);
        body.put("capabilityType", cap);
        body.put("status", status);
        body.put("message", status);
        if (requestId != null) {
            body.put("requestId", requestId);
        }
        try {
            return json(code, objectMapper.writeValueAsString(body));
        } catch (IOException e) {
            throw new IllegalStateException(e);
        }
    }

    private static MockResponse json(int status, String body) {
        return new MockResponse()
                .setResponseCode(status)
                .setHeader("Content-Type", "application/json")
                .setBody(body);
    }

    private String registerPath() {
        return "/api/v1/sdk-api/agents/" + agentId + "/capabilities/register";
    }

    /** The recorded form of a request whose body is exactly {capabilityType}. */
    private static Map<String, Object> request(String method, String path, String capabilityType) {
        Map<String, Object> recorded = new LinkedHashMap<>();
        recorded.put("capabilityType", capabilityType);
        recorded.put("method", method);
        recorded.put("path", path);
        return recorded;
    }

    @SuppressWarnings("unchecked")
    private static List<Map<String, Object>> results(Map<String, Object> result) {
        return (List<Map<String, Object>>) result.get("results");
    }

    private static List<String> statuses(Map<String, Object> result) {
        List<String> out = new ArrayList<>();
        for (Map<String, Object> outcome : results(result)) {
            out.add(String.valueOf(outcome.get("status")));
        }
        return out;
    }
}
