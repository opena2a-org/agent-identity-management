package org.opena2a.aim;

import okhttp3.mockwebserver.MockResponse;
import okhttp3.mockwebserver.MockWebServer;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import java.io.IOException;
import java.net.InetAddress;
import java.net.ServerSocket;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.*;

/**
 * The live tests run only with a credential to sign in with whose AIM server
 * answers GET /health. The credential of the report, left from a server that
 * no longer ran and holding an empty sdkTokenId, failed two of them.
 */
class LiveAimTest {

    private static final String FAKE_REFRESH_TOKEN = "not-a-real-token";

    private MockWebServer server;

    @BeforeEach
    void setUp() throws IOException {
        server = new MockWebServer();
        server.start(InetAddress.getByName("127.0.0.1"), 0);
    }

    @AfterEach
    void tearDown() throws IOException {
        server.shutdown();
    }

    private String serverUrl() {
        return "http://127.0.0.1:" + server.getPort();
    }

    private static int closedPort() throws IOException {
        try (ServerSocket socket = new ServerSocket(0, 0, InetAddress.getByName("127.0.0.1"))) {
            return socket.getLocalPort();
        }
    }

    @Test
    void aCredentialWhoseServerDoesNotRunCannotSignIn() throws IOException {
        String url = "http://127.0.0.1:" + closedPort();

        assertFalse(LiveAim.canSignIn(Map.of(
                "aimUrl", url, "refreshToken", FAKE_REFRESH_TOKEN, "sdkTokenId", "")));
    }

    @Test
    void aCredentialWhoseServerAnswersHealthCanSignIn() throws InterruptedException {
        server.enqueue(new MockResponse().setResponseCode(200));
        server.enqueue(new MockResponse().setResponseCode(200));

        assertTrue(LiveAim.canSignIn(Map.of("aimUrl", serverUrl(), "refreshToken", FAKE_REFRESH_TOKEN)));
        assertTrue(LiveAim.canSignIn(Map.of(
                "aimUrl", serverUrl(), "clientId", "not-a-real-id", "clientSecret", "not-a-real-secret")));
        assertEquals("/health", server.takeRequest().getPath());
        assertEquals("/health", server.takeRequest().getPath());
    }

    @Test
    void aServerThatAnswersHealthWithAnErrorIsNotLive() {
        server.enqueue(new MockResponse().setResponseCode(503));

        assertFalse(LiveAim.canSignIn(Map.of("aimUrl", serverUrl(), "refreshToken", FAKE_REFRESH_TOKEN)));
    }

    @Test
    void aCredentialWithNothingToSignInWithCannotSignInAndSendsNoRequest() {
        assertFalse(LiveAim.canSignIn(Map.of("aimUrl", serverUrl())));
        assertFalse(LiveAim.canSignIn(Map.of("aimUrl", serverUrl(), "refreshToken", "")));
        assertFalse(LiveAim.canSignIn(Map.of("aimUrl", serverUrl(), "clientId", "not-a-real-id")));
        assertFalse(LiveAim.canSignIn(Map.of("aimUrl", serverUrl(), "clientId", "not-a-real-id", "clientSecret", " ")));
        assertEquals(0, server.getRequestCount());
    }

    @Test
    void headShortensOnlyAValueLongerThanEightCharacters() {
        assertEquals("", LiveAim.head(""));
        assertEquals("abc", LiveAim.head("abc"));
        assertEquals("abcdefgh", LiveAim.head("abcdefgh"));
        assertEquals("abcdefgh...", LiveAim.head("abcdefghi"));
        assertEquals("n/a", LiveAim.head(null));
    }
}
