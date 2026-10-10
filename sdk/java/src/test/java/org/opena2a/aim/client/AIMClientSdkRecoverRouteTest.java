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
import java.util.Base64;
import java.util.List;
import java.util.Map;
import java.util.UUID;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.junit.jupiter.api.Assertions.fail;

/**
 * Checks the token recovery AIMClient sends after the server refuses a refresh
 * against what the backend's recovery route requires.
 *
 * The stand-in server answers the recovery route the way the backend does. The
 * route is mounted behind the auth middleware, read here from the backend's
 * route registration, so a request without the bearer of the token's owner is
 * answered 401 before its body is read. The handler then reads the old refresh
 * token from the JSON key named in its request struct, read here from the
 * backend source, and refuses a body without that key with 400. A test that
 * chose its own key, or answered without a bearer, would agree with the client
 * while the real server refused the call.
 */
class AIMClientSdkRecoverRouteTest {

    /** Repo-relative location of the backend's SDK token recovery handler. */
    private static final String RECOVERY_HANDLER =
            "apps/backend/internal/interfaces/http/handlers/sdk_token_recovery_handler.go";

    /** Repo-relative location of the backend's route registration. */
    private static final String ROUTES = "apps/backend/cmd/server/main.go";

    private static final String REFRESH_PATH = "/api/v1/auth/refresh";
    private static final String RECOVER_PATH = "/api/v1/auth/sdk/recover";

    private static final String REFUSED_REFRESH_TOKEN = "refused-refresh-token";
    private static final String RECOVERED_ACCESS_TOKEN = "recovered-access-token";

    private static final Pattern OLD_REFRESH_TOKEN_FIELD =
            Pattern.compile("OldRefreshToken\\s+string\\s+`json:\"([^\",]+)");
    private static final Pattern AUTH_PROTECTED_GROUP = Pattern.compile(
            "authProtected\\s*:=\\s*v1\\.Group\\(\"/auth\"\\)\\s*\\n\\s*authProtected\\.Use\\(middleware\\.AuthMiddleware\\(");
    private static final Pattern RECOVER_ROUTE = Pattern.compile(
            "authProtected\\.Post\\(\"/sdk/recover\",\\s*h\\.SDKTokenRecovery\\.RecoverRevokedToken\\)");

    private final ObjectMapper objectMapper = new ObjectMapper();
    private final AtomicInteger refreshes = new AtomicInteger();

    private MockWebServer server;
    private String recoveryKey;
    /** The access token the first refresh hands out; null when every refresh is refused. */
    private volatile String heldAccessToken;

    @BeforeEach
    void setUp() throws IOException {
        recoveryKey = readRecoveryKey();
        server = new MockWebServer();
        server.setDispatcher(new Dispatcher() {
            @Override
            public MockResponse dispatch(RecordedRequest request) {
                String path = request.getRequestUrl().encodedPath();
                if (REFRESH_PATH.equals(path)) {
                    if (refreshes.incrementAndGet() == 1 && heldAccessToken != null) {
                        return json(200, "{\"accessToken\":\"" + heldAccessToken + "\",\"tokenType\":\"Bearer\"}");
                    }
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
    @DisplayName("a refused refresh is recovered with the held access token and the old refresh token under the key the handler reads")
    void refusedRefresh_recoversWithTheHeldBearerAndTheHandlersKey() throws Exception {
        // The first refresh hands out an access token in its last minute, so
        // the next call refreshes again while the token is still valid, and
        // that refresh is refused.
        heldAccessToken = jwtExpiringIn(30);
        String agentId = UUID.randomUUID().toString();

        AIMClient client = new AIMClient.Builder()
                .agentName("recover-route-agent")
                .aimUrl(baseUrl())
                .agentId(agentId)
                .refreshToken(REFUSED_REFRESH_TOKEN)
                .build();

        Map<String, Object> first;
        Map<String, Object> result;
        try {
            first = client.useMcpTool(UUID.randomUUID().toString(), "read_file",
                    "http://localhost:3001", "filesystem-mcp");
            result = client.useMcpTool(UUID.randomUUID().toString(), "read_file",
                    "http://localhost:3001", "filesystem-mcp");
        } finally {
            client.close();
        }
        assertEquals(Boolean.TRUE, first.get("success"), "the call before recovery failed: " + first.get("error"));

        List<RecordedRequest> requests = drainRequests();
        RecordedRequest recover = find(requests, RECOVER_PATH);
        assertNotNull(recover, "the client did not attempt recovery after the refused refresh; requests: "
                + paths(requests));
        assertEquals("POST", recover.getMethod());
        assertEquals("Bearer " + heldAccessToken, recover.getHeader("Authorization"),
                "the recovery request must carry the access token the client holds; " + ROUTES
                        + " mounts the route behind the auth middleware");

        JsonNode body = objectMapper.readTree(recover.getBody().readUtf8());
        assertEquals(REFUSED_REFRESH_TOKEN, body.path(recoveryKey).asText(),
                "the recovery body must carry the old refresh token under \"" + recoveryKey
                        + "\", the key " + RECOVERY_HANDLER + " reads; got " + body);

        // The recovered access token authorizes the call that needed it.
        String usagePath = "/api/v1/sdk-api/agents/" + agentId + "/mcp-usage-report";
        RecordedRequest usage = findLast(requests, usagePath);
        assertNotNull(usage, "the client sent no request after recovering; requests: " + paths(requests));
        assertEquals("Bearer " + RECOVERED_ACCESS_TOKEN, usage.getHeader("Authorization"));
        assertEquals(Boolean.TRUE, result.get("success"),
                "the call after recovery failed: " + result.get("error"));
    }

    @Test
    @DisplayName("a client that holds no access token sends no recovery request the route would refuse")
    void refusedRefresh_withoutAnAccessToken_sendsNoRecoveryRequest() throws Exception {
        heldAccessToken = null;
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
        assertNotNull(find(requests, REFRESH_PATH), "the client did not refresh; requests: " + paths(requests));
        assertNull(find(requests, RECOVER_PATH),
                "the client sent a recovery request without a bearer; requests: " + paths(requests));
        assertNull(find(requests, "/api/v1/sdk-api/agents/" + agentId + "/mcp-usage-report"),
                "the client sent the call without credentials; requests: " + paths(requests));
        assertEquals(Boolean.FALSE, result.get("success"));
    }

    @Test
    @DisplayName("the recovery key is read from the backend handler source")
    void recoveryKey_isReadFromBackendSource() {
        // Guards the reader itself: a pattern that stopped matching the handler
        // would make every recovery fail for the wrong reason.
        assertFalse(recoveryKey.isEmpty());
        assertEquals("oldRefreshToken", recoveryKey);
    }

    @Test
    @DisplayName("the backend mounts the recovery route behind the auth middleware")
    void recoveryRoute_isMountedBehindTheAuthMiddleware() throws IOException {
        // Guards the stand-in: it answers a recovery request without the
        // owner's bearer with 401, because the backend mounts the route so.
        String source = new String(Files.readAllBytes(locate(ROUTES)), StandardCharsets.UTF_8);
        assertTrue(AUTH_PROTECTED_GROUP.matcher(source).find(),
                "the authProtected /auth group no longer applies AuthMiddleware first in " + ROUTES);
        assertTrue(RECOVER_ROUTE.matcher(source).find(),
                "POST /sdk/recover is no longer registered on the authProtected group in " + ROUTES);
    }

    private MockResponse recover(RecordedRequest request) {
        String authorization = request.getHeader("Authorization");
        if (heldAccessToken == null || !("Bearer " + heldAccessToken).equals(authorization)) {
            return json(401, "{\"error\":\"No authentication token provided\"}");
        }
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

    private static RecordedRequest findLast(List<RecordedRequest> requests, String path) {
        RecordedRequest last = null;
        for (RecordedRequest request : requests) {
            if (path.equals(request.getRequestUrl().encodedPath())) {
                last = request;
            }
        }
        return last;
    }

    /** An unsigned JWT-shaped token whose exp claim is the given seconds from now. */
    private static String jwtExpiringIn(long seconds) {
        Base64.Encoder encoder = Base64.getUrlEncoder().withoutPadding();
        String header = encoder.encodeToString("{\"alg\":\"none\",\"typ\":\"JWT\"}".getBytes(StandardCharsets.UTF_8));
        String claims = "{\"exp\":" + (Instant.now().getEpochSecond() + seconds) + "}";
        return header + "." + encoder.encodeToString(claims.getBytes(StandardCharsets.UTF_8)) + ".held";
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
