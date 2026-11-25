package org.opena2a.aim.a2a;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import okhttp3.mockwebserver.MockResponse;
import okhttp3.mockwebserver.MockWebServer;
import okhttp3.mockwebserver.RecordedRequest;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.opena2a.aim.client.AIMClient;

import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.util.ArrayList;
import java.util.Collections;
import java.util.Iterator;
import java.util.List;
import java.util.Map;
import java.util.concurrent.TimeUnit;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.Mockito.*;

/**
 * {@code A2AClient.evaluatePolicy} sends the request the server reads (#652).
 *
 * <p>{@code POST /api/v1/a2a/policies/evaluate} binds {@code EvaluateA2APolicyRequest}
 * ({@code event}, {@code clientAgentId}, {@code remoteAgentId}, {@code skillId}, and an
 * {@code organizationId} that may only name the caller's own organization), read here from the
 * backend source. The client sent {@code targetAgentId}, {@code action}, {@code resource} and
 * {@code context}, none of which the route reads. The request is pinned in
 * {@code sdk/typescript/tests/fixtures/sdk_requests/a2a_evaluate_policy.json}, which the
 * backend's handler test POSTs to that route and the TypeScript SDK's tests hold its client
 * to.</p>
 */
@DisplayName("evaluatePolicy sends the request the server reads")
class A2AEvaluatePolicyWireTest {

    private static final String SERVICE = "apps/backend/internal/application/a2a_service.go";
    private static final String FIXTURE = "sdk/typescript/tests/fixtures/sdk_requests/a2a_evaluate_policy.json";
    private static final Pattern REQUEST_STRUCT =
            Pattern.compile("type EvaluateA2APolicyRequest struct \\{(.*?)\\n\\}", Pattern.DOTALL);
    private static final Pattern JSON_NAME = Pattern.compile("`json:\"(\\w+)[,\"]");

    private static final String DECISION = "{"
            + "\"decision\":\"DENY\","
            + "\"policiesEvaluated\":[\"skill-policy\"],"
            + "\"reason\":\"Trust score below threshold (client: 0.00, remote: 0.00, required: 0.30)\","
            + "\"evaluatedAt\":\"2026-11-24T12:00:00Z\""
            + "}";

    private final ObjectMapper objectMapper = new ObjectMapper();

    private MockWebServer server;
    private JsonNode fixture;
    private A2AClient client;

    @BeforeEach
    void setUp() throws IOException {
        fixture = objectMapper.readTree(read(FIXTURE));
        server = new MockWebServer();
        server.start();
        AIMClient aim = mock(AIMClient.class);
        String url = server.url("/").toString();
        when(aim.getAimUrl()).thenReturn(url.substring(0, url.length() - 1));
        when(aim.getAccessToken()).thenReturn("test-token");
        when(aim.getAgentId()).thenReturn(fixture.get("body").get("clientAgentId").asText());
        client = new A2AClient(aim, 10);
    }

    @AfterEach
    void tearDown() throws IOException {
        server.shutdown();
    }

    @Test
    @DisplayName("the server binds event, clientAgentId, remoteAgentId and skillId")
    void theServerBindsTheMembersThePinnedRequestCarries() throws Exception {
        Matcher declared = REQUEST_STRUCT.matcher(read(SERVICE));
        assertTrue(declared.find(), "EvaluateA2APolicyRequest is not declared in " + SERVICE);
        List<String> members = new ArrayList<>();
        Matcher name = JSON_NAME.matcher(declared.group(1));
        while (name.find()) {
            members.add(name.group(1));
        }
        Collections.sort(members);
        assertEquals(List.of("clientAgentId", "event", "organizationId", "remoteAgentId", "skillId"), members);
        for (Iterator<String> it = fixture.get("body").fieldNames(); it.hasNext(); ) {
            String member = it.next();
            assertTrue(members.contains(member), "the pinned request sends " + member + ", which the server does not read");
        }
    }

    @Test
    @DisplayName("an event that invokes a skill is sent as the pinned request, and the decision is read")
    void anEventWithASkillIsSentAsThePinnedRequest() throws Exception {
        JsonNode pinned = fixture.get("body");
        respond(DECISION);

        Map<String, Object> decision = client.evaluatePolicy(
                pinned.get("event").asText(), pinned.get("remoteAgentId").asText(), pinned.get("skillId").asText());

        RecordedRequest request = take();
        assertEquals(fixture.get("method").asText(), request.getMethod());
        assertEquals(fixture.get("path").asText(), request.getRequestUrl().encodedPath());
        assertEquals(pinned, objectMapper.readTree(request.getBody().readUtf8()));
        assertEquals("DENY", decision.get("decision"));
        assertEquals(List.of("skill-policy"), decision.get("policiesEvaluated"));
    }

    @Test
    @DisplayName("an event that invokes no skill sends no skillId")
    void anEventWithNoSkillSendsNoSkillId() throws Exception {
        JsonNode pinned = fixture.get("body");
        respond("{\"decision\":\"ALLOW\",\"policiesEvaluated\":[],\"evaluatedAt\":\"2026-11-24T12:00:00Z\"}");

        client.evaluatePolicy("task.received", pinned.get("remoteAgentId").asText());

        JsonNode body = objectMapper.readTree(take().getBody().readUtf8());
        assertEquals(objectMapper.readTree("{"
                + "\"event\":\"task.received\","
                + "\"clientAgentId\":\"" + pinned.get("clientAgentId").asText() + "\","
                + "\"remoteAgentId\":\"" + pinned.get("remoteAgentId").asText() + "\""
                + "}"), body);
    }

    @Test
    @DisplayName("a refusal by the server fails the call")
    void aRefusalFailsTheCall() {
        server.enqueue(new MockResponse().setResponseCode(400).setBody("{\"error\":\"Invalid request body\"}"));

        A2AException e = assertThrows(A2AException.class,
                () -> client.evaluatePolicy("task.create", "not-a-uuid"));
        assertTrue(e.getMessage().contains("400"), e.getMessage());
    }

    private void respond(String body) {
        server.enqueue(new MockResponse()
                .setResponseCode(200)
                .setHeader("Content-Type", "application/json")
                .setBody(body));
    }

    private RecordedRequest take() throws InterruptedException {
        RecordedRequest request = server.takeRequest(5, TimeUnit.SECONDS);
        assertNotNull(request, "no request reached the stand-in server");
        return request;
    }

    private static String read(String relative) throws IOException {
        return new String(Files.readAllBytes(locate(relative)), StandardCharsets.UTF_8);
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
