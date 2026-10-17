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
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.ValueSource;

import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.io.PrintStream;
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
 * Checks what AIMClient does after the server refuses a refresh against what
 * the backend's recovery route requires.
 *
 * The stand-in server answers the recovery route the way the backend does. The
 * route is mounted behind the auth middleware, read here from the backend's
 * route registration, so a request without the bearer of the token's owner is
 * answered 401 before its body is read. The handler then refuses, with 401, an
 * access token that carries a token family and records no sign-in: the access
 * token the refresh route mints for a downloaded SDK's token, which is the
 * only kind this client obtains. Both are read here from the backend source.
 * So the client presents a refused token to the refresh route only, and prints
 * a banner that names how to replace the credential, including the server's
 * maximum age for a download (SDK_TOKEN_FAMILY_MAX_AGE).
 */
class AIMClientSdkRecoverRouteTest {

    /** Repo-relative location of the backend's SDK token recovery handler. */
    private static final String RECOVERY_HANDLER =
            "apps/backend/internal/interfaces/http/handlers/sdk_token_recovery_handler.go";

    /** Repo-relative location of the backend's route registration. */
    private static final String ROUTES = "apps/backend/cmd/server/main.go";

    /** Repo-relative location of the backend's refresh route handler. */
    private static final String REFRESH_HANDLER =
            "apps/backend/internal/interfaces/http/handlers/auth_refresh_handler.go";

    /** Repo-relative location of the guard the credential-minting routes share. */
    private static final String MINT_GUARD =
            "apps/backend/internal/interfaces/http/handlers/credential_mint_family.go";

    /** Repo-relative location of the backend's JWT service. */
    private static final String JWT_SERVICE = "apps/backend/internal/infrastructure/auth/jwt.go";

    /** Repo-relative location of the dashboard's section tabs. */
    private static final String HUB_TABS = "apps/web/lib/hub-tabs.ts";

    private static final String REFRESH_PATH = "/api/v1/auth/refresh";
    private static final String RECOVER_PATH = "/api/v1/auth/sdk/recover";

    private static final String REFUSED_REFRESH_TOKEN = "refused-refresh-token";
    private static final String RECOVERED_ACCESS_TOKEN = "recovered-access-token";

    /** The family of the SDK download whose token the client refreshes. */
    private static final String DOWNLOAD_FAMILY = "download-family-1";

    /** The refresh route's answer to a token past its family's maximum age. */
    private static final String MAX_AGE_REFUSAL = "Invalid or expired refresh token";
    /** The refresh route's answer to a revoked token. */
    private static final String REVOKED_REFUSAL = "Token has been revoked or is invalid";

    private static final String MAX_AGE_CAUSE = "The SDK download reached the server's maximum age "
            + "(SDK_TOKEN_FAMILY_MAX_AGE, 365 days by default). Refreshing does not extend it; "
            + "a fresh download starts a new one.";

    private static final Pattern OLD_REFRESH_TOKEN_FIELD =
            Pattern.compile("OldRefreshToken\\s+string\\s+`json:\"([^\",]+)");
    private static final Pattern AUTH_PROTECTED_GROUP = Pattern.compile(
            "authProtected\\s*:=\\s*v1\\.Group\\(\"/auth\"\\)\\s*\\n\\s*authProtected\\.Use\\(middleware\\.AuthMiddleware\\(");
    private static final Pattern RECOVER_ROUTE = Pattern.compile(
            "authProtected\\.Post\\(\"/sdk/recover\",\\s*h\\.SDKTokenRecovery\\.RecoverRevokedToken\\)");
    private static final Pattern MAX_AGE_REFUSAL_BODY = Pattern.compile(
            "if h\\.familyPastMaxAge\\([^)]*\\) \\{\\s*return c\\.Status\\(fiber\\.StatusUnauthorized\\)"
                    + "\\.JSON\\(fiber\\.Map\\{\\s*\"error\": \"([^\"]+)\"");
    private static final Pattern NO_SIGN_IN_REFUSED = Pattern.compile(
            "if signedIn, _ := c\\.Locals\\(\"auth_time\"\\)\\.\\(time\\.Time\\); signedIn\\.IsZero\\(\\) \\{"
                    + "\\s*recordMintRefused\\([^)]*\\)\\s*return refuseMint\\(c\\)");
    private static final Pattern SDK_ACCESS_HAS_NO_SIGN_IN = Pattern.compile(
            "signedIn := time\\.Time\\{\\}\\s*if !isSDKToken \\{\\s*signedIn = claims\\.SignedInAt\\(\\)\\s*\\}");
    private static final Pattern DEVELOPERS_TABS = Pattern.compile(
            "developers: \\[\\s*\\{ name: \"Guide\"[^\\n]*\\n\\s*\\{ name: \"SDK & docs\", href: \"/dashboard/sdk\""
                    + "[^\\n]*\\n(?:\\s*//[^\\n]*\\n)*\\s*\\{ name: \"Credentials\", href: \"/dashboard/credentials\"");

    private final ObjectMapper objectMapper = new ObjectMapper();
    private final AtomicInteger refreshes = new AtomicInteger();

    private MockWebServer server;
    private String recoveryKey;
    /** The access token the first refresh hands out; null when every refresh is refused. */
    private volatile String heldAccessToken;
    /** The body every refresh after the first is refused with. */
    private volatile String refusalBody = "{\"error\":\"Refresh token has been revoked\"}";

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
                    return json(401, refusalBody);
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

    @ParameterizedTest(name = "refused with \"{0}\"")
    @ValueSource(strings = {MAX_AGE_REFUSAL, REVOKED_REFUSAL})
    @DisplayName("a refused refresh token is presented to the refresh route only, and the banner names the download's maximum age")
    void refusedRefresh_isPresentedOnceAndThePrintedBannerNamesTheMaximumAge(String refusal) throws Exception {
        assertTrue(source(REFRESH_HANDLER).contains("\"error\": \"" + refusal + "\""),
                REFRESH_HANDLER + " no longer answers a refused refresh with \"" + refusal + "\"");
        refusalBody = "{\"error\":\"" + refusal + "\"}";
        // The first refresh hands out an access token in its last minute, so
        // the next call refreshes again while the token is still valid, and
        // that refresh is refused. The held token is the one the refresh route
        // mints for a downloaded SDK's token: the download's family, no
        // sign-in time.
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
        ByteArrayOutputStream printed = new ByteArrayOutputStream();
        PrintStream stdout = System.out;
        try {
            first = client.useMcpTool(UUID.randomUUID().toString(), "read_file",
                    "http://localhost:3001", "filesystem-mcp");
            System.setOut(new PrintStream(printed, true, StandardCharsets.UTF_8));
            result = client.useMcpTool(UUID.randomUUID().toString(), "read_file",
                    "http://localhost:3001", "filesystem-mcp");
        } finally {
            System.setOut(stdout);
            client.close();
        }
        assertEquals(Boolean.TRUE, first.get("success"), "the call before the refusal failed: " + first.get("error"));

        List<RecordedRequest> requests = drainRequests();
        assertEquals(2, count(requests, REFRESH_PATH),
                "the client did not refresh twice; requests: " + paths(requests));
        assertNull(find(requests, RECOVER_PATH),
                "the client presented the refused token to the recovery route, which refuses the access token it "
                        + "holds; requests: " + paths(requests));
        assertEquals(1, count(requests, "/api/v1/sdk-api/agents/" + agentId + "/mcp-usage-report"),
                "the client sent the call after the refusal; requests: " + paths(requests));
        assertEquals(Boolean.FALSE, result.get("success"));

        String out = printed.toString(StandardCharsets.UTF_8);
        String flat = out.replaceAll("\\s+", " ");
        assertEquals(1, out.split("SDK TOKEN EXPIRED", -1).length - 1, "the banner is printed once: " + out);
        assertTrue(flat.contains(MAX_AGE_CAUSE), "the banner does not name the download's maximum age: " + out);
        assertTrue(flat.contains("Go to Developers, then SDK & docs"), out);
        assertTrue(flat.contains("Developers, then Credentials"), out);
        assertFalse(out.contains("Settings"), "the banner names a dashboard path that does not exist: " + out);
        assertFalse(out.contains("recovery"), "the banner is all the refusal prints: " + out);
    }

    @Test
    @DisplayName("the recovery route refuses the access token the refresh route mints for a downloaded SDK's token")
    void recoveryRoute_refusesAnAccessTokenMintedFromADownloadedSdksToken() throws IOException {
        // Guards the stand-in's sign-in check and why the client sends no
        // recovery request: the route refuses an access token that carries a
        // family and no sign-in time before it reads the old token, and the
        // JWT service's refresh gives an SDK token's access token no sign-in
        // time.
        String handler = source(RECOVERY_HANDLER);
        int guard = handler.indexOf("refuseUnlessLiveSignIn(c, h.jwtService, h.audit, \"sdk_recover\")");
        int read = handler.indexOf("GetTokenID(req.OldRefreshToken)");
        assertTrue(guard >= 0, RECOVERY_HANDLER + " no longer refuses an access token without a sign-in");
        assertTrue(read > guard, RECOVERY_HANDLER + " reads the old token before it checks the sign-in");
        assertTrue(NO_SIGN_IN_REFUSED.matcher(source(MINT_GUARD)).find(),
                MINT_GUARD + " no longer refuses an access token whose session records no sign-in");
        assertTrue(SDK_ACCESS_HAS_NO_SIGN_IN.matcher(source(JWT_SERVICE)).find(),
                JWT_SERVICE + " no longer mints an SDK token's access token without a sign-in time");
    }

    @Test
    @DisplayName("the banner's maximum age, its default and its dashboard steps are the server's and the dashboard's")
    void banner_citesTheServersMaximumAgeAndTheDashboardsTabs() throws IOException {
        String jwt = source(JWT_SERVICE);
        assertTrue(jwt.contains("getEnv(\"SDK_TOKEN_FAMILY_MAX_AGE\", \"8760h\")"),
                JWT_SERVICE + " no longer reads SDK_TOKEN_FAMILY_MAX_AGE with the default 8760h");
        assertTrue(jwt.contains("defaultSDKTokenFamilyMaxAge = 365 * 24 * time.Hour"),
                JWT_SERVICE + " no longer defaults the download's maximum age to 365 days");
        Matcher refusal = MAX_AGE_REFUSAL_BODY.matcher(source(REFRESH_HANDLER));
        assertTrue(refusal.find(), "the maximum-age refusal's body was not found in " + REFRESH_HANDLER);
        assertEquals(MAX_AGE_REFUSAL, refusal.group(1));
        assertTrue(DEVELOPERS_TABS.matcher(source(HUB_TABS)).find(),
                HUB_TABS + " no longer lists Guide, SDK & docs and Credentials under Developers");
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
    @DisplayName("a client idle past its access token's expiry sends no recovery request the auth middleware would refuse")
    void refusedRefresh_afterTheHeldAccessTokenExpired_sendsNoRecoveryRequest() throws Exception {
        // The first refresh hands out an access token that expires a second
        // later. The client is idle past that expiry, so the next call
        // refreshes, the refresh is refused, and the only bearer the client
        // holds is one the auth middleware refuses.
        heldAccessToken = jwtExpiringIn(1);
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
            waitPast(expiryOf(heldAccessToken));
            result = client.useMcpTool(UUID.randomUUID().toString(), "read_file",
                    "http://localhost:3001", "filesystem-mcp");
        } finally {
            client.close();
        }
        assertEquals(Boolean.TRUE, first.get("success"), "the call before the idle period failed: " + first.get("error"));

        List<RecordedRequest> requests = drainRequests();
        assertEquals(2, count(requests, REFRESH_PATH),
                "the client did not refresh after the idle period; requests: " + paths(requests));
        assertNull(find(requests, RECOVER_PATH),
                "the client sent a recovery request with an expired bearer; requests: " + paths(requests));
        assertEquals(1, count(requests, "/api/v1/sdk-api/agents/" + agentId + "/mcp-usage-report"),
                "the client sent the call after the idle period without a valid access token; requests: "
                        + paths(requests));
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
        // The auth middleware refuses an access token whose exp has passed.
        if (!Instant.now().isBefore(expiryOf(heldAccessToken))) {
            return json(401, "{\"error\":\"Invalid or expired token\"}");
        }
        JsonNode body;
        try {
            body = objectMapper.readTree(request.getBody().clone().readUtf8());
        } catch (IOException e) {
            return json(400, "{\"error\":\"Invalid request format\"}");
        }
        // The route refuses an access token that carries a family and records
        // no sign-in (refuseUnlessLiveSignIn), before it reads the old token.
        JsonNode held = claimsOf(heldAccessToken);
        if (!held.path("sid").asText().isEmpty() && !held.has("auth_time")) {
            return json(401, "{\"error\":\"" + REVOKED_REFUSAL + "\"}");
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

    private static int count(List<RecordedRequest> requests, String path) {
        int n = 0;
        for (RecordedRequest request : requests) {
            if (path.equals(request.getRequestUrl().encodedPath())) {
                n++;
            }
        }
        return n;
    }

    /**
     * An unsigned JWT-shaped access token whose exp claim is the given seconds
     * from now, shaped as the refresh route mints one for a downloaded SDK's
     * token: it carries the download's family as sid and no auth_time.
     */
    private static String jwtExpiringIn(long seconds) {
        Base64.Encoder encoder = Base64.getUrlEncoder().withoutPadding();
        String header = encoder.encodeToString("{\"alg\":\"none\",\"typ\":\"JWT\"}".getBytes(StandardCharsets.UTF_8));
        String claims = "{\"typ\":\"access\",\"sid\":\"" + DOWNLOAD_FAMILY + "\",\"exp\":"
                + (Instant.now().getEpochSecond() + seconds) + "}";
        return header + "." + encoder.encodeToString(claims.getBytes(StandardCharsets.UTF_8)) + ".held";
    }

    /** The exp claim of a token made by {@link #jwtExpiringIn(long)}. */
    private Instant expiryOf(String token) {
        return Instant.ofEpochSecond(claimsOf(token).path("exp").asLong());
    }

    /** The claims of a token made by {@link #jwtExpiringIn(long)}. */
    private JsonNode claimsOf(String token) {
        try {
            return objectMapper.readTree(Base64.getUrlDecoder().decode(token.split("\\.")[1]));
        } catch (IOException e) {
            throw new IllegalStateException("the held access token has no readable claims", e);
        }
    }

    /** Sleeps until the given instant has passed. */
    private static void waitPast(Instant instant) throws InterruptedException {
        Instant until = instant.plusMillis(200);
        while (Instant.now().isBefore(until)) {
            Thread.sleep(Math.max(1, until.toEpochMilli() - Instant.now().toEpochMilli()));
        }
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

    /** The text of a repository file, found above the working directory. */
    private static String source(String relative) throws IOException {
        return new String(Files.readAllBytes(locate(relative)), StandardCharsets.UTF_8);
    }

    /** Reads the JSON key of RecoverTokenRequest.OldRefreshToken from the handler source. */
    private static String readRecoveryKey() throws IOException {
        String source = source(RECOVERY_HANDLER);
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
