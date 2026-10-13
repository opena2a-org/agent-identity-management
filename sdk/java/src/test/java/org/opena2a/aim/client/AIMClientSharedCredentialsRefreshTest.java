package org.opena2a.aim.client;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import okhttp3.mockwebserver.Dispatcher;
import okhttp3.mockwebserver.MockResponse;
import okhttp3.mockwebserver.MockWebServer;
import okhttp3.mockwebserver.RecordedRequest;
import org.junit.jupiter.api.*;
import org.junit.jupiter.api.io.TempDir;
import org.opena2a.aim.credentials.CredentialManager;
import org.opena2a.aim.exceptions.AIMException;

import java.io.BufferedReader;
import java.io.IOException;
import java.io.InputStreamReader;
import java.io.OutputStream;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.security.MessageDigest;
import java.time.Duration;
import java.util.ArrayList;
import java.util.Base64;
import java.util.HashMap;
import java.util.HexFormat;
import java.util.List;
import java.util.Map;
import java.util.concurrent.CopyOnWriteArrayList;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Processes sharing {@code ~/.aim/sdk_credentials.json} refresh it once: the refresh runs under
 * the credentials lock and starts by re-reading the file, so a process never presents a refresh
 * token another process already rotated. Each process here is a separate JVM whose home
 * directory is a temporary one, against a server that accepts each refresh token once.
 */
class AIMClientSharedCredentialsRefreshTest {

    private static final ObjectMapper MAPPER = new ObjectMapper();
    private static final String REFRESH_PATH = "/api/v1/auth/refresh";
    private static final String RESULT = "RESULT ";

    @TempDir
    Path home;

    private MockWebServer server;
    private SingleUseRefreshServer refreshes;

    @BeforeEach
    void setUp() throws IOException {
        refreshes = new SingleUseRefreshServer();
        server = new MockWebServer();
        server.setDispatcher(refreshes);
        server.start();
    }

    @AfterEach
    void tearDown() throws IOException {
        server.shutdown();
    }

    private String baseUrl() {
        String url = server.url("/").toString();
        return url.substring(0, url.length() - 1);
    }

    private Path credentialsPath() {
        return home.resolve(".aim").resolve("sdk_credentials.json");
    }

    private void storeRefreshToken(String refreshToken) {
        Map<String, String> credentials = new HashMap<>();
        credentials.put("aimUrl", baseUrl());
        credentials.put("refreshToken", refreshToken);
        CredentialManager.saveSdkCredentials(credentials, credentialsPath());
    }

    private JsonNode storedCredentials() throws IOException {
        return MAPPER.readTree(Files.readString(credentialsPath()));
    }

    @Test
    @DisplayName("two processes refreshing together send one refresh request and use the same access token")
    void twoProcessesRefreshingTogetherSendOneRefresh() throws Exception {
        storeRefreshToken("not-a-real-refresh-0");
        refreshes.accept("not-a-real-refresh-0");

        List<String> results = new ArrayList<>();
        try (RefreshProcess first = RefreshProcess.start(home, baseUrl(), 1);
             RefreshProcess second = RefreshProcess.start(home, baseUrl(), 1)) {
            first.go();
            second.go();
            results.addAll(first.results());
            results.addAll(second.results());
        }

        assertEquals(List.of("not-a-real-refresh-0"), refreshes.presented(),
                () -> "refresh tokens presented: " + refreshes.presented());
        assertEquals(2, results.size(), () -> "results: " + results);
        assertTrue(results.get(0).startsWith("token "), () -> "results: " + results);
        assertEquals(results.get(0), results.get(1), "the processes use different access tokens");
        assertEquals(List.of(REFRESH_PATH), refreshes.paths(), () -> "requests: " + refreshes.paths());

        JsonNode stored = storedCredentials();
        assertEquals("not-a-real-refresh-1", stored.get("refreshToken").asText());
        JsonNode shared = stored.get("sharedAccessToken");
        assertNotNull(shared, "the access token was not stored for the other process");
        assertEquals(results.get(0).substring("token ".length()), shared.get("token").asText());
        assertEquals(sha256Hex("not-a-real-refresh-1"), shared.get("refreshTokenSha256").asText());
    }

    @Test
    @DisplayName("a refresh after another process rotated the stored token presents the new token")
    void aRefreshAfterAnotherProcessRotatedPresentsTheNewToken() throws Exception {
        storeRefreshToken("not-a-real-refresh-0");

        List<String> results;
        try (RefreshProcess process = RefreshProcess.start(home, baseUrl(), 1)) {
            // Another process rotated the token after this one loaded the file.
            storeRefreshToken("not-a-real-refresh-5");
            refreshes.accept("not-a-real-refresh-5");
            process.go();
            results = process.results();
        }

        assertEquals(List.of("not-a-real-refresh-5"), refreshes.presented(),
                () -> "refresh tokens presented: " + refreshes.presented());
        assertEquals(1, results.size(), () -> "results: " + results);
        assertTrue(results.get(0).startsWith("token "), () -> "results: " + results);
        assertEquals("not-a-real-refresh-6", storedCredentials().get("refreshToken").asText());
    }

    @Test
    @DisplayName("a refresh token the server refused is not presented again by the same process")
    void aRefusedRefreshTokenIsNotPresentedAgain() throws Exception {
        storeRefreshToken("not-a-real-refresh-0");
        refreshes.accept("not-a-real-refresh-other");

        List<String> results;
        try (RefreshProcess process = RefreshProcess.start(home, baseUrl(), 2)) {
            process.go();
            results = process.results();
        }

        assertEquals(List.of("refused", "refused"), results);
        assertEquals(List.of("not-a-real-refresh-0"), refreshes.presented(),
                () -> "refresh tokens presented: " + refreshes.presented());
        assertEquals("not-a-real-refresh-0", storedCredentials().get("refreshToken").asText());
    }

    @Test
    @DisplayName("no refresh request is sent while another process holds the credentials lock")
    void noRefreshIsSentWhileAnotherProcessHoldsTheLock() throws Exception {
        storeRefreshToken("not-a-real-refresh-0");
        refreshes.accept("not-a-real-refresh-0");

        CountDownLatch held = new CountDownLatch(1);
        CountDownLatch release = new CountDownLatch(1);
        Thread holder = new Thread(() -> {
            try {
                CredentialManager.withSdkCredentialsLock(credentialsPath(), Duration.ofSeconds(10), () -> {
                    held.countDown();
                    return release.await(30, TimeUnit.SECONDS);
                });
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
            }
        });

        List<String> results;
        try (RefreshProcess process = RefreshProcess.start(home, baseUrl(), 1)) {
            holder.start();
            try {
                assertTrue(held.await(10, TimeUnit.SECONDS), "the lock was not taken");
                process.go();
                Thread.sleep(1500);
                assertEquals(List.of(), refreshes.presented(), "a refresh was sent while the lock was held");
            } finally {
                // Released before the process is closed, which waits for it to finish.
                release.countDown();
            }
            results = process.results();
        } finally {
            holder.join(10_000);
        }

        assertEquals(1, results.size(), () -> "results: " + results);
        assertTrue(results.get(0).startsWith("token "), () -> "results: " + results);
        assertEquals(List.of("not-a-real-refresh-0"), refreshes.presented());
    }

    private static String sha256Hex(String value) throws Exception {
        return HexFormat.of().formatHex(
                MessageDigest.getInstance("SHA-256").digest(value.getBytes(StandardCharsets.UTF_8)));
    }

    /** An access token the client can read an expiry from, assembled here rather than written out. */
    private static String accessToken(String subject, long expiresAt) {
        Base64.Encoder encoder = Base64.getUrlEncoder().withoutPadding();
        String header = encoder.encodeToString("{\"alg\":\"none\"}".getBytes(StandardCharsets.UTF_8));
        String claims = encoder.encodeToString(
                ("{\"sub\":\"" + subject + "\",\"exp\":" + expiresAt + "}").getBytes(StandardCharsets.UTF_8));
        return header + "." + claims + ".not-a-signature";
    }

    /**
     * Answers the refresh route only, and accepts each refresh token once, as the server does:
     * the accepted token is replaced by the next one in the answer, and any other token is
     * refused with 401. Every other path is 404.
     */
    private static final class SingleUseRefreshServer extends Dispatcher {
        private final List<String> presented = new CopyOnWriteArrayList<>();
        private final List<String> paths = new CopyOnWriteArrayList<>();
        private String accepted;

        synchronized void accept(String refreshToken) {
            accepted = refreshToken;
        }

        List<String> presented() {
            return List.copyOf(presented);
        }

        List<String> paths() {
            return List.copyOf(paths);
        }

        @Override
        public synchronized MockResponse dispatch(RecordedRequest request) throws InterruptedException {
            String path = request.getRequestUrl().encodedPath();
            paths.add(path);
            if (!REFRESH_PATH.equals(path) || !"POST".equals(request.getMethod())) {
                return new MockResponse().setResponseCode(404);
            }
            String token;
            try {
                token = MAPPER.readTree(request.getBody().readUtf8()).path("refreshToken").asText();
            } catch (IOException e) {
                return new MockResponse().setResponseCode(400);
            }
            presented.add(token);
            // Wide enough that a second process refreshing at the same time arrives meanwhile.
            Thread.sleep(300);
            if (!token.equals(accepted)) {
                return new MockResponse().setResponseCode(401)
                        .setHeader("Content-Type", "application/json")
                        .setBody("{\"error\":\"invalid refresh token\"}");
            }
            int next = Integer.parseInt(token.substring(token.lastIndexOf('-') + 1)) + 1;
            accepted = "not-a-real-refresh-" + next;
            String body = "{\"accessToken\":\"" + accessToken("access-" + next, System.currentTimeMillis() / 1000 + 900)
                    + "\",\"refreshToken\":\"" + accepted + "\"}";
            return new MockResponse().setHeader("Content-Type", "application/json").setBody(body);
        }
    }

    /**
     * A second JVM, with {@code home} as its home directory, that builds a client from the
     * credentials file and, told to go, asks it for an access token {@code times} times,
     * printing {@code RESULT token <token>} or {@code RESULT refused} for each.
     */
    static final class RefreshProcess implements AutoCloseable {
        private final Process process;
        private final BufferedReader out;
        private final OutputStream in;

        private RefreshProcess(Process process) {
            this.process = process;
            this.out = new BufferedReader(new InputStreamReader(process.getInputStream(), StandardCharsets.UTF_8));
            this.in = process.getOutputStream();
        }

        static RefreshProcess start(Path home, String aimUrl, int times) throws IOException {
            List<String> command = List.of(
                    Path.of(System.getProperty("java.home"), "bin", "java").toString(),
                    "-Duser.home=" + home,
                    "-Dorg.slf4j.simpleLogger.defaultLogLevel=error",
                    "-cp", System.getProperty("java.class.path"),
                    Main.class.getName(), aimUrl, String.valueOf(times));
            RefreshProcess started = new RefreshProcess(new ProcessBuilder(command)
                    .redirectError(ProcessBuilder.Redirect.INHERIT)
                    .start());
            List<String> lines = started.readResults(1);
            if (!List.of("ready").equals(lines)) {
                started.close();
                throw new IOException("the refreshing process answered " + lines + " instead of ready");
            }
            return started;
        }

        void go() throws IOException {
            in.write("go\n".getBytes(StandardCharsets.UTF_8));
            in.flush();
        }

        /** Every result the process prints until it exits. */
        List<String> results() throws Exception {
            List<String> lines = readResults(Integer.MAX_VALUE);
            assertTrue(process.waitFor(60, TimeUnit.SECONDS), "the refreshing process did not finish");
            assertEquals(0, process.exitValue(), "the refreshing process failed");
            return lines;
        }

        /** Up to {@code limit} result lines, skipping what the client itself prints. */
        private List<String> readResults(int limit) throws IOException {
            List<String> lines = new ArrayList<>();
            String line;
            while (lines.size() < limit && (line = out.readLine()) != null) {
                if (line.startsWith(RESULT)) {
                    lines.add(line.substring(RESULT.length()));
                }
            }
            return lines;
        }

        @Override
        public void close() throws IOException {
            try {
                in.close();
                if (!process.waitFor(30, TimeUnit.SECONDS)) {
                    process.destroyForcibly();
                }
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
                process.destroyForcibly();
            }
        }

        /** Child process entry point. */
        static final class Main {
            public static void main(String[] args) throws IOException {
                Map<String, String> credentials = new HashMap<>(CredentialManager.readSdkCredentialsFile());
                AIMClient client = new AIMClient.Builder()
                        .agentName("shared-credentials-refresh-agent")
                        .aimUrl(args[0])
                        .refreshToken(credentials.get("refreshToken"))
                        .credentials(credentials)
                        .build();
                int times = Integer.parseInt(args[1]);
                System.out.println(RESULT + "ready");
                System.out.flush();
                new BufferedReader(new InputStreamReader(System.in, StandardCharsets.UTF_8)).readLine();
                try {
                    for (int i = 0; i < times; i++) {
                        String result;
                        try {
                            result = "token " + client.getAccessToken();
                        } catch (AIMException e) {
                            result = "refused";
                        }
                        System.out.println(RESULT + result);
                        System.out.flush();
                    }
                } finally {
                    client.close();
                }
            }
        }
    }
}
