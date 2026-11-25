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
import java.time.Instant;
import java.util.ArrayList;
import java.util.List;
import java.util.concurrent.TimeUnit;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.Mockito.*;

/**
 * {@code A2AClient.updateAgentCard} sends the card document the server reads (#652).
 *
 * <p>{@code PUT /api/v1/a2a/cards/:id} reads {@code cardUrl} or {@code cardData} and replaces the
 * agent's card with that document; it answers the identifiers of the card's skills as strings
 * ({@code RegisterAgentCardResponse.Skills []string}, read here from the backend source). The
 * client sent the card itself, which the route does not read and answered 400, and could not read
 * the skills of the answer. The request is pinned in
 * {@code sdk/typescript/tests/fixtures/sdk_requests/a2a_update_agent_card.json}, which the
 * backend's handler test PUTs to that route and the TypeScript SDK's tests hold its client to.</p>
 */
@DisplayName("updateAgentCard sends the card document the server reads")
class A2AAgentCardUpdateWireTest {

    private static final String SERVICE = "apps/backend/internal/application/a2a_service.go";
    private static final String FIXTURE = "sdk/typescript/tests/fixtures/sdk_requests/a2a_update_agent_card.json";
    private static final Pattern RESPONSE_STRUCT =
            Pattern.compile("type RegisterAgentCardResponse struct \\{(.*?)\\n\\}", Pattern.DOTALL);

    private static final String AGENT_ID = "550e8400-e29b-41d4-a716-446655440000";
    private static final String CARD_ID = "6ba7b810-9dad-11d1-80b4-00c04fd430c8";

    /** The backend's answer to the pinned request. */
    private static final String ANSWER = "{"
            + "\"cardId\":\"" + CARD_ID + "\","
            + "\"agentId\":\"" + AGENT_ID + "\","
            + "\"cardUrl\":\"inline://" + AGENT_ID + "\","
            + "\"attestationExpires\":\"2026-11-25T12:00:00Z\","
            + "\"skills\":[\"summarize\"]"
            + "}";

    private final ObjectMapper objectMapper = new ObjectMapper();

    private MockWebServer server;
    private A2AClient client;

    @BeforeEach
    void setUp() throws IOException {
        server = new MockWebServer();
        server.start();
        AIMClient aim = mock(AIMClient.class);
        String url = server.url("/").toString();
        when(aim.getAimUrl()).thenReturn(url.substring(0, url.length() - 1));
        when(aim.getAccessToken()).thenReturn("test-token");
        when(aim.getAgentId()).thenReturn(AGENT_ID);
        client = new A2AClient(aim, 10);
    }

    @AfterEach
    void tearDown() throws IOException {
        server.shutdown();
    }

    @Test
    @DisplayName("the server answers a card's skills as strings")
    void theServerAnswersSkillsAsStrings() throws Exception {
        Matcher declared = RESPONSE_STRUCT.matcher(read(SERVICE));
        assertTrue(declared.find(), "RegisterAgentCardResponse is not declared in " + SERVICE);
        assertTrue(Pattern.compile("Skills\\s+\\[\\]string\\s+`json:\"skills\"`").matcher(declared.group(1)).find(),
                "RegisterAgentCardResponse: " + declared.group(1));
    }

    @Test
    @DisplayName("a card with a skill is sent as the pinned request, and the answer's skills are read")
    void aCardWithASkillIsSentAsThePinnedRequest() throws Exception {
        JsonNode fixture = objectMapper.readTree(read(FIXTURE));
        respond(ANSWER);

        A2AAgentCard.Skill skill = new A2AAgentCard.Skill();
        skill.setAgentId(AGENT_ID);
        skill.setId("summarize");
        skill.setName("Summarize");
        skill.setDescription("Summarizes a report");
        skill.setTags(List.of("text"));
        A2AAgentCard stored = client.updateAgentCard(A2AAgentCard.builder()
                .id("3f2504e0-4f89-41d3-9a0c-0305e82c3301")
                .agentId(AGENT_ID)
                .name("Report Writer")
                .description("Writes the nightly report")
                .url("https://agent.example.com")
                .version("1.1.0")
                .skills(List.of(skill))
                .aimAttestation("previous-attestation")
                .publicKey("previous-key")
                .verified(true)
                .build());

        RecordedRequest update = take();
        assertEquals(fixture.get("method").asText(), update.getMethod());
        assertEquals(fixture.get("path").asText().replace("{agentId}", AGENT_ID), update.getRequestUrl().encodedPath());
        assertEquals(fixture.get("body"), objectMapper.readTree(update.getBody().readUtf8()));

        assertEquals(CARD_ID, stored.getId());
        assertEquals(AGENT_ID, stored.getAgentId());
        assertEquals(Instant.parse("2026-11-25T12:00:00Z"), stored.getAimAttestationExpiresAt());
        List<String> skills = new ArrayList<>();
        for (A2AAgentCard.Skill s : stored.getSkills()) {
            skills.add(s.getId());
        }
        assertEquals(List.of("summarize"), skills);
    }

    @Test
    @DisplayName("a card with no agent ID replaces the current agent's card")
    void aCardWithNoAgentIdReplacesTheCurrentAgentsCard() throws Exception {
        respond(ANSWER);

        client.updateAgentCard(A2AAgentCard.builder()
                .name("Report Writer")
                .url("https://agent.example.com")
                .version("1.1.0")
                .build());

        RecordedRequest update = take();
        assertEquals("/api/v1/a2a/cards/" + AGENT_ID, update.getRequestUrl().encodedPath());
        JsonNode body = objectMapper.readTree(update.getBody().readUtf8());
        assertEquals(objectMapper.readTree(
                "{\"cardData\":{\"name\":\"Report Writer\",\"url\":\"https://agent.example.com\",\"version\":\"1.1.0\"}}"),
                body);
    }

    @Test
    @DisplayName("a refusal by the server fails the call")
    void aRefusalFailsTheCall() {
        server.enqueue(new MockResponse().setResponseCode(400).setBody("{\"error\":\"refused\"}"));

        A2AException e = assertThrows(A2AException.class,
                () -> client.updateAgentCard(A2AAgentCard.builder().agentId(AGENT_ID).name("x").build()));
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
