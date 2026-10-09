package org.opena2a.aim.client;

import okhttp3.HttpUrl;
import okhttp3.mockwebserver.Dispatcher;
import okhttp3.mockwebserver.MockResponse;
import okhttp3.mockwebserver.MockWebServer;
import okhttp3.mockwebserver.RecordedRequest;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.io.IOException;
import java.util.Arrays;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.UUID;
import java.util.concurrent.TimeUnit;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotNull;

/**
 * Checks the query string AIMClient.listAgents sends. A status filter is
 * caller-supplied text, so it has to arrive as the value of the one "status"
 * parameter whatever characters it holds.
 */
class AIMClientListAgentsQueryTest {

    private static final String REFRESH_PATH = "/api/v1/auth/refresh";
    private static final String AGENTS_PATH = "/api/v1/agents";

    private MockWebServer server;

    @BeforeEach
    void setUp() throws IOException {
        server = new MockWebServer();
        server.setDispatcher(new Dispatcher() {
            @Override
            public MockResponse dispatch(RecordedRequest request) {
                if (REFRESH_PATH.equals(request.getRequestUrl().encodedPath())) {
                    return json("{\"accessToken\":\"test-access-token\"}");
                }
                return json("{\"agents\":[],\"total\":0}");
            }
        });
        server.start();
    }

    @AfterEach
    void tearDown() throws IOException {
        server.shutdown();
    }

    @Test
    @DisplayName("a status holding '&', '#', '=', '+' or a space is sent as the value of the status parameter")
    void listAgents_sendsStatusAsOneEncodedValue() throws Exception {
        String status = "verified&agentType=gpt&limit=1000#frag +more=x";

        HttpUrl url = listAgents(25, 5, status, AgentType.LANGCHAIN);

        assertEquals(AGENTS_PATH, url.encodedPath());
        assertEquals(new LinkedHashSet<>(Arrays.asList("limit", "offset", "status", "agentType")),
                url.queryParameterNames(), "query: " + url.encodedQuery());
        assertEquals(List.of(status), url.queryParameterValues("status"));
        assertEquals(List.of("langchain"), url.queryParameterValues("agentType"));
        assertEquals(List.of("25"), url.queryParameterValues("limit"));
        assertEquals(List.of("5"), url.queryParameterValues("offset"));
    }

    @Test
    @DisplayName("without a status or agent type only limit and offset are sent, limit capped at 100")
    void listAgents_withoutFiltersSendsLimitAndOffset() throws Exception {
        HttpUrl url = listAgents(500, 0, "", null);

        assertEquals(AGENTS_PATH, url.encodedPath());
        assertEquals("limit=100&offset=0", url.encodedQuery());
    }

    private HttpUrl listAgents(int limit, int offset, String status, AgentType agentType) throws Exception {
        AIMClient client = new AIMClient.Builder()
                .agentName("list-agents-query-agent")
                .aimUrl(baseUrl())
                .agentId(UUID.randomUUID().toString())
                .refreshToken("test-refresh-token")
                .build();
        try {
            client.listAgents(limit, offset, status, agentType);
        } finally {
            client.close();
        }

        RecordedRequest request = server.takeRequest(5, TimeUnit.SECONDS);
        assertNotNull(request, "the client sent no request");
        if (REFRESH_PATH.equals(request.getRequestUrl().encodedPath())) {
            request = server.takeRequest(5, TimeUnit.SECONDS);
            assertNotNull(request, "the client authenticated and then sent no request");
        }
        assertEquals("GET", request.getMethod());
        return request.getRequestUrl();
    }

    private String baseUrl() {
        String url = server.url("/").toString();
        return url.substring(0, url.length() - 1);
    }

    private static MockResponse json(String body) {
        return new MockResponse()
                .setResponseCode(200)
                .setHeader("Content-Type", "application/json")
                .setBody(body);
    }
}
