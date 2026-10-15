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
import java.util.Iterator;
import java.util.List;
import java.util.Set;
import java.util.TreeSet;
import java.util.concurrent.TimeUnit;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.Mockito.*;

/**
 * {@code A2AClient.updateSecuritySettings} sends the members the server reads (#652).
 *
 * <p>{@code PUT /api/v1/a2a/security/settings} binds the backend's {@code A2ASecuritySettings},
 * whose JSON members are read here from the backend source. The settings named request signing
 * {@code requireSignedRequests} and the nonce window {@code maxTimestampAge}, which the server
 * does not read. The request is pinned in
 * {@code sdk/typescript/tests/fixtures/sdk_requests/a2a_update_security_settings.json}, which the
 * backend's handler tests post to that route and the TypeScript SDK's tests hold its client to.</p>
 */
@DisplayName("A2A security settings are sent with the members the server reads")
class A2ASecuritySettingsWireTest {

    private static final String SERVER_TYPE = "apps/backend/internal/domain/a2a.go";
    private static final String FIXTURE = "sdk/typescript/tests/fixtures/sdk_requests/a2a_update_security_settings.json";
    private static final Pattern SETTINGS_STRUCT =
            Pattern.compile("type A2ASecuritySettings struct \\{(.*?)\\n\\}", Pattern.DOTALL);
    private static final Pattern JSON_MEMBER = Pattern.compile("`json:\"([^\",`]+)");

    /** The members that describe the stored record: read from the answer, not sent. */
    private static final Set<String> RECORD_MEMBERS =
            Set.of("id", "organizationId", "createdBy", "updatedBy", "createdAt", "updatedAt");

    private static final String ALLOWED_AGENT = "7c9e6679-7425-40de-944b-e07fc1f90ae7";
    private static final String BLOCKED_AGENT = "550e8400-e29b-41d4-a716-446655440000";

    /** The backend's answer for an organization with stored settings. */
    private static final String STORED_ANSWER = "{"
            + "\"id\":\"3f2504e0-4f89-41d3-9a0c-0305e82c3301\","
            + "\"organizationId\":\"9b2f4c1e-6a3d-4e8b-8f5a-2c7d1e0b9a64\","
            + "\"enforcementMode\":\"monitor\","
            + "\"requireVerifiedSkills\":false,"
            + "\"minTrustScore\":0.25,"
            + "\"minAttestationCount\":2,"
            + "\"requireRequestSigning\":false,"
            + "\"requireNonceValidation\":true,"
            + "\"nonceValiditySeconds\":120,"
            + "\"allowedAgentIds\":[\"" + ALLOWED_AGENT + "\"],"
            + "\"blockedAgentIds\":[\"" + BLOCKED_AGENT + "\"],"
            + "\"maxRequestsPerMinute\":30,"
            + "\"maxRequestsPerHour\":500,"
            + "\"createdBy\":\"0b6e8a52-1c4f-4d7e-9a3b-5f2c8d1e7a90\","
            + "\"updatedBy\":\"0b6e8a52-1c4f-4d7e-9a3b-5f2c8d1e7a90\","
            + "\"createdAt\":\"2026-10-01T09:30:00Z\","
            + "\"updatedAt\":\"2026-10-02T09:30:00Z\""
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
        when(aim.getAgentId()).thenReturn("agent-1");
        client = new A2AClient(aim, 10);
    }

    @AfterEach
    void tearDown() throws IOException {
        server.shutdown();
    }

    @Test
    @DisplayName("the update body is the pinned request: every settings member the server reads, and no other")
    void builtSettingsAreSentAsThePinnedBody() throws Exception {
        JsonNode fixture = objectMapper.readTree(read(FIXTURE));
        respond(STORED_ANSWER);

        client.updateSecuritySettings(A2ASecuritySettings.builder()
                .id("3f2504e0-4f89-41d3-9a0c-0305e82c3301")
                .organizationId("9b2f4c1e-6a3d-4e8b-8f5a-2c7d1e0b9a64")
                .enforcementMode("strict")
                .requireVerifiedSkills(true)
                .minTrustScore(0.7)
                .minAttestationCount(3)
                .requireRequestSigning(true)
                .requireNonceValidation(true)
                .nonceValiditySeconds(300)
                .allowedAgentIds(List.of(ALLOWED_AGENT))
                .blockedAgentIds(List.of(BLOCKED_AGENT))
                .maxRequestsPerMinute(60)
                .maxRequestsPerHour(1000)
                .build());

        RecordedRequest update = take();
        assertEquals(fixture.get("method").asText(), update.getMethod());
        assertEquals(fixture.get("path").asText(), update.getRequestUrl().encodedPath());
        assertEquals(fixture.get("body"), objectMapper.readTree(update.getBody().readUtf8()));

        TreeSet<String> settingsMembers = new TreeSet<>(serverMembers());
        settingsMembers.removeAll(RECORD_MEMBERS);
        assertEquals(settingsMembers, names(fixture.get("body")),
                "the pinned body carries every settings member of the backend's A2ASecuritySettings");
    }

    @Test
    @DisplayName("settings read from the server and changed are sent with every setting it answered")
    void readSettingsAreSentBackWithTheChange() throws Exception {
        respond(STORED_ANSWER);
        respond(STORED_ANSWER);

        A2ASecuritySettings settings = client.getSecuritySettings();
        take();
        assertFalse(settings.isRequireRequestSigning());
        assertEquals(120, settings.getNonceValiditySeconds());
        assertEquals(2, settings.getMinAttestationCount());
        assertEquals(30, settings.getMaxRequestsPerMinute());
        assertEquals(500, settings.getMaxRequestsPerHour());
        assertEquals("9b2f4c1e-6a3d-4e8b-8f5a-2c7d1e0b9a64", settings.getOrganizationId());
        assertEquals("0b6e8a52-1c4f-4d7e-9a3b-5f2c8d1e7a90", settings.getUpdatedBy());
        assertNotNull(settings.getCreatedAt());

        settings.setRequireRequestSigning(true);
        client.updateSecuritySettings(settings);

        JsonNode sent = objectMapper.readTree(take().getBody().readUtf8());
        JsonNode expected = objectMapper.readTree(STORED_ANSWER);
        TreeSet<String> settingsMembers = new TreeSet<>(serverMembers());
        settingsMembers.removeAll(RECORD_MEMBERS);
        assertEquals(settingsMembers, names(sent), "sent: " + sent);
        for (String member : settingsMembers) {
            if (!member.equals("requireRequestSigning")) {
                assertEquals(expected.get(member), sent.get(member), member + " is sent as the server answered it");
            }
        }
        assertTrue(sent.get("requireRequestSigning").asBoolean(), "the change is sent");
    }

    @Test
    @DisplayName("the server's members are the model's: every member the backend declares is read from its answer")
    void everyServerMemberIsBoundFromTheAnswer() throws Exception {
        respond(STORED_ANSWER);
        A2ASecuritySettings settings = client.getSecuritySettings();
        JsonNode answer = objectMapper.readTree(STORED_ANSWER);

        assertEquals(new TreeSet<>(serverMembers()), names(answer),
                "the stand-in answer carries every member the backend's A2ASecuritySettings declares");
        assertEquals("monitor", settings.getEnforcementMode());
        assertFalse(settings.isRequireVerifiedSkills());
        assertEquals(0.25, settings.getMinTrustScore(), 0.0);
        assertTrue(settings.isRequireNonceValidation());
        assertEquals(List.of(ALLOWED_AGENT), settings.getAllowedAgentIds());
        assertEquals(List.of(BLOCKED_AGENT), settings.getBlockedAgentIds());
        assertEquals("3f2504e0-4f89-41d3-9a0c-0305e82c3301", settings.getId());
        assertEquals("0b6e8a52-1c4f-4d7e-9a3b-5f2c8d1e7a90", settings.getCreatedBy());
        assertNotNull(settings.getUpdatedAt());
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

    private static TreeSet<String> names(JsonNode object) {
        TreeSet<String> names = new TreeSet<>();
        for (Iterator<String> it = object.fieldNames(); it.hasNext(); ) {
            names.add(it.next());
        }
        return names;
    }

    /** The JSON members of the backend's A2ASecuritySettings. */
    private static List<String> serverMembers() throws IOException {
        Matcher declared = SETTINGS_STRUCT.matcher(read(SERVER_TYPE));
        assertTrue(declared.find(), "A2ASecuritySettings is not declared in " + SERVER_TYPE);
        List<String> members = new java.util.ArrayList<>();
        Matcher member = JSON_MEMBER.matcher(declared.group(1));
        while (member.find()) {
            members.add(member.group(1));
        }
        assertFalse(members.isEmpty(), "A2ASecuritySettings has no JSON members in " + SERVER_TYPE);
        assertTrue(members.containsAll(RECORD_MEMBERS), "record members: " + members);
        return members;
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
