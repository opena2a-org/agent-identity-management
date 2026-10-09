package org.opena2a.aim.integrations.mcp;

import okhttp3.mockwebserver.MockResponse;
import okhttp3.mockwebserver.MockWebServer;
import okhttp3.mockwebserver.RecordedRequest;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.MethodSource;
import org.opena2a.aim.client.AIMClient;
import org.opena2a.aim.exceptions.AIMException;

import java.io.IOException;
import java.util.List;
import java.util.concurrent.TimeUnit;
import java.util.stream.Stream;

import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.Mockito.*;

/**
 * Every MCPIntegration request carries the AIM client's access token, none is
 * sent without one, and attestServer sends nothing, because this class cannot
 * sign with the agent's private key.
 */
class MCPIntegrationCredentialsTest {

    private static final String AGENT_ID = "agent-123";
    private static final String ACCESS_TOKEN = "test-access-token";
    private static final String SERVER_ID = "6f1c2b9e-4d3a-4f7e-9b2d-0c8e5a7f1d24";
    private static final String PUBLIC_KEY = "MCowBQYDK2VwAyEAExamplePublicKeyForDemonstration123456789012345";
    private static final List<String> CAPABILITIES = List.of("read_file", "write_file");

    @FunctionalInterface
    interface Invocation {
        Object invoke(AIMClient client);
    }

    record Call(String name, String path, String responseBody, Invocation invocation) {
        @Override
        public String toString() {
            return name;
        }
    }

    static Stream<Call> calls() {
        return Stream.of(
                new Call("registerServer", "/api/v1/sdk-api/agents/" + AGENT_ID + "/mcp-servers",
                        "{\"id\":\"" + SERVER_ID + "\",\"name\":\"filesystem-mcp\"}",
                        c -> MCPIntegration.registerServer(c, "filesystem-mcp", "http://localhost:3000",
                                PUBLIC_KEY, CAPABILITIES)),
                new Call("listServers", "/api/v1/sdk-api/agents/" + AGENT_ID + "/mcp-servers?limit=50&offset=0",
                        "{\"servers\":[]}",
                        MCPIntegration::listServers),
                new Call("recordToolUsage", "/api/v1/sdk-api/agents/" + AGENT_ID + "/mcp-connections",
                        "{\"success\":true}",
                        c -> MCPIntegration.recordToolUsage(c, SERVER_ID, "read_file")),
                new Call("verifyAction", "/api/v1/mcp-servers/" + SERVER_ID + "/verify-action",
                        "{\"verified\":true}",
                        c -> MCPIntegration.verifyAction(c, SERVER_ID, "read_file"))
        );
    }

    private MockWebServer mockServer;
    private AIMClient client;

    @BeforeEach
    void setUp() throws IOException {
        mockServer = new MockWebServer();
        mockServer.start();
        client = new AIMClient.Builder()
                .agentName("mcp-agent")
                .agentId(AGENT_ID)
                .aimUrl(baseUrl())
                .refreshToken("test-refresh-token")
                .build();
    }

    @AfterEach
    void tearDown() throws IOException {
        client.close();
        mockServer.shutdown();
    }

    private String baseUrl() {
        String url = mockServer.url("/").toString();
        return url.substring(0, url.length() - 1);
    }

    private void respond(String body) {
        mockServer.enqueue(new MockResponse()
                .setResponseCode(200)
                .setHeader("Content-Type", "application/json")
                .setBody(body));
    }

    private AIMClient clientWithToken(String token) {
        AIMClient aim = mock(AIMClient.class);
        when(aim.getAimUrl()).thenReturn(baseUrl());
        when(aim.getAgentId()).thenReturn(AGENT_ID);
        when(aim.getAccessToken()).thenReturn(token);
        return aim;
    }

    @ParameterizedTest(name = "{0}")
    @MethodSource("calls")
    @DisplayName("each request carries the client's access token")
    void sendsTheClientsAccessToken(Call call) throws InterruptedException {
        respond("{\"accessToken\":\"" + ACCESS_TOKEN + "\"}");
        respond(call.responseBody());

        call.invocation().invoke(client);

        RecordedRequest refresh = mockServer.takeRequest(1, TimeUnit.SECONDS);
        assertNotNull(refresh, "the client fetches its access token first");
        assertEquals("/api/v1/auth/refresh", refresh.getPath());
        RecordedRequest request = mockServer.takeRequest(1, TimeUnit.SECONDS);
        assertNotNull(request, "expected a request to " + call.path());
        assertEquals(call.path(), request.getPath());
        assertEquals("Bearer " + ACCESS_TOKEN, request.getHeader("Authorization"));
        assertEquals(2, mockServer.getRequestCount());
    }

    @ParameterizedTest(name = "{0}")
    @MethodSource("calls")
    @DisplayName("a client without an access token sends nothing")
    void sendsNothingWithoutAnAccessToken(Call call) {
        for (String token : new String[] {null, "", " "}) {
            AIMClient aim = clientWithToken(token);
            if (call.name().equals("verifyAction")) {
                assertEquals(false, call.invocation().invoke(aim), "verifyAction with token " + token);
            } else {
                assertThrows(AIMException.class, () -> call.invocation().invoke(aim),
                        call.name() + " with token " + token);
            }
        }
        assertEquals(0, mockServer.getRequestCount());
    }

    @Test
    @DisplayName("attestServer throws before any request is sent")
    void attestServerSendsNothing() {
        AIMException shortForm = assertThrows(AIMException.class, () -> MCPIntegration.attestServer(
                client, SERVER_ID, "http://localhost:3000", "filesystem-mcp", CAPABILITIES));
        assertTrue(shortForm.getMessage().contains("No request was sent"), shortForm.getMessage());

        AIMException longForm = assertThrows(AIMException.class, () -> MCPIntegration.attestServer(
                client, SERVER_ID, "http://localhost:3000", "filesystem-mcp", CAPABILITIES,
                true, true, 45.0));
        assertTrue(longForm.getMessage().contains("No request was sent"), longForm.getMessage());

        assertEquals(0, mockServer.getRequestCount());
    }
}
