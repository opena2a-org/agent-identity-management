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
import java.util.ArrayList;
import java.util.List;
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
 * Checks the token recovery AIMClient sends after the server refuses a refresh
 * against the request body the backend's recovery handler reads.
 *
 * The stand-in server answers the recovery route the way the handler does: the
 * old refresh token is read from the JSON key named in the handler's request
 * struct, read here from the backend source, and a body without that key is
 * refused with 400. A test that chose its own key would agree with the client
 * while the real server refused the call.
 */
class AIMClientSdkRecoverRouteTest {

    /** Repo-relative location of the backend's SDK token recovery handler. */
    private static final String RECOVERY_HANDLER =
            "apps/backend/internal/interfaces/http/handlers/sdk_token_recovery_handler.go";

    private static final String REFRESH_PATH = "/api/v1/auth/refresh";
    private static final String RECOVER_PATH = "/api/v1/auth/sdk/recover";

    private static final String REFUSED_REFRESH_TOKEN = "refused-refresh-token";
    private static final String RECOVERED_ACCESS_TOKEN = "recovered-access-token";

    private static final Pattern OLD_REFRESH_TOKEN_FIELD =
            Pattern.compile("OldRefreshToken\\s+string\\s+`json:\"([^\",]+)");

    private final ObjectMapper objectMapper = new ObjectMapper();

    private MockWebServer server;
    private String recoveryKey;

    @BeforeEach
    void setUp() throws IOException {
        recoveryKey = readRecoveryKey();
        server = new MockWebServer();
        server.setDispatcher(new Dispatcher() {
            @Override
            public MockResponse dispatch(RecordedRequest request) {
                String path = request.getRequestUrl().encodedPath();
                if (REFRESH_PATH.equals(path)) {
                    return json(401, "{\"error\":\"Refresh token has been revoked\"}");
                }
                if (RECOVER_PATH.equals(path)) {
                    return recover(request);
                }
                return json(200, "{\"success\":true}");
            }
        });
        server.start();
    }

    @AfterEach
    void tearDown() throws IOException {
        server.shutdown();
    }

    @Test
    @DisplayName("a refused refresh is recovered with the old refresh token under the key the handler reads")
    void refusedRefresh_recoversWithTheHandlersKey() throws Exception {
        String agentId = UUID.randomUUID().toString();

        AIMClient client = new AIMClient.Builder()
                .agentName("recover-route-agent")
                .aimUrl(baseUrl())
                .agentId(agentId)
                .refreshToken(REFUSED_REFRESH_TOKEN)
                .build();

        Map<String, Object> result;
        try {
            result = client.useMcpTool(UUID.randomUUID().toString(), "read_file",
                    "http://localhost:3001", "filesystem-mcp");
        } finally {
            client.close();
        }

        List<RecordedRequest> requests = drainRequests();
        RecordedRequest recover = find(requests, RECOVER_PATH);
        assertNotNull(recover, "the client did not attempt recovery after the refused refresh; requests: "
                + paths(requests));
        assertEquals("POST", recover.getMethod());

        JsonNode body = objectMapper.readTree(recover.getBody().readUtf8());
        assertEquals(REFUSED_REFRESH_TOKEN, body.path(recoveryKey).asText(),
                "the recovery body must carry the old refresh token under \"" + recoveryKey
                        + "\", the key " + RECOVERY_HANDLER + " reads; got " + body);

        // The recovered access token authorizes the call that needed it.
        RecordedRequest usage = find(requests, "/api/v1/sdk-api/agents/" + agentId + "/mcp-usage-report");
        assertNotNull(usage, "the client sent no request after recovering; requests: " + paths(requests));
        assertEquals("Bearer " + RECOVERED_ACCESS_TOKEN, usage.getHeader("Authorization"));
        assertEquals(Boolean.TRUE, result.get("success"),
                "the call after recovery failed: " + result.get("error"));
    }

    @Test
    @DisplayName("the recovery key is read from the backend handler source")
    void recoveryKey_isReadFromBackendSource() {
        // Guards the reader itself: a pattern that stopped matching the handler
        // would make every recovery fail for the wrong reason.
        assertFalse(recoveryKey.isEmpty());
        assertEquals("oldRefreshToken", recoveryKey);
    }

    private MockResponse recover(RecordedRequest request) {
        JsonNode body;
        try {
            body = objectMapper.readTree(request.getBody().clone().readUtf8());
        } catch (IOException e) {
            return json(400, "{\"error\":\"Invalid request format\"}");
        }
        if (!body.path(recoveryKey).isTextual() || body.path(recoveryKey).asText().isEmpty()) {
            return json(400, "{\"error\":\"Invalid request format\"}");
        }
        return json(200, "{\"accessToken\":\"" + RECOVERED_ACCESS_TOKEN + "\","
                + "\"refreshToken\":\"recovered-refresh-token\",\"tokenType\":\"Bearer\",\"expiresIn\":900}");
    }

    private List<RecordedRequest> drainRequests() throws InterruptedException {
        List<RecordedRequest> requests = new ArrayList<>();
        RecordedRequest request;
        while ((request = server.takeRequest(1, TimeUnit.SECONDS)) != null) {
            requests.add(request);
        }
        return requests;
    }

    private static RecordedRequest find(List<RecordedRequest> requests, String path) {
        for (RecordedRequest request : requests) {
            if (path.equals(request.getRequestUrl().encodedPath())) {
                return request;
            }
        }
        return null;
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

    /** Reads the JSON key of RecoverTokenRequest.OldRefreshToken from the handler source. */
    private static String readRecoveryKey() throws IOException {
        String source = new String(Files.readAllBytes(locate(RECOVERY_HANDLER)), StandardCharsets.UTF_8);
        Matcher field = OLD_REFRESH_TOKEN_FIELD.matcher(source);
        assertTrue(field.find(), "RecoverTokenRequest.OldRefreshToken has no json tag in " + RECOVERY_HANDLER);
        return field.group(1);
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
                + ". This test reads the backend's recovery handler and has to run inside the repository.");
    }
}
