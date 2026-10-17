package org.opena2a.aim.integration;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import okhttp3.HttpUrl;
import okhttp3.MediaType;
import okhttp3.OkHttpClient;
import okhttp3.Request;
import okhttp3.RequestBody;
import okhttp3.Response;
import org.junit.jupiter.api.AfterAll;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;
import org.opena2a.aim.client.AIMClient;
import org.opena2a.aim.client.VerificationResult;
import org.opena2a.aim.integration.langchain4j.AIMChatModelListener;
import org.opena2a.aim.integration.langchain4j.AIMToolExecutor;
import org.opena2a.aim.integration.spring.AIMFunctionCallingAdvisor;

import java.io.ByteArrayInputStream;
import java.io.IOException;
import java.net.Proxy;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.attribute.PosixFilePermissions;
import java.time.Duration;
import java.util.ArrayList;
import java.util.Collections;
import java.util.List;
import java.util.UUID;
import java.util.function.Function;
import java.util.zip.ZipEntry;
import java.util.zip.ZipInputStream;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.junit.jupiter.api.Assertions.fail;
import static org.junit.jupiter.api.Assumptions.assumeTrue;

/**
 * Drives AIMToolExecutor, AIMFunctionCallingAdvisor and AIMChatModelListener
 * the way their javadoc shows, each with an AIMClient registered against the
 * backend at AIM_URL, and reads from the backend whether the call recorded a
 * verification.
 *
 * <p>The backend's answer is read through {@code GET
 * /api/v1/verification-events/agent/{id}} with the seeded admin's token, never
 * from what a helper returns. Each test then has the same client call
 * {@link AIMClient#verifyCapability(String, String)} and reads one new event,
 * which shows the read would have seen a verification the helper recorded.</p>
 *
 * <p>Skipped when AIM_URL is unset. scripts/test-sdk-java-helpers-integration.sh
 * starts a backend, sets AIM_URL, AIM_TEST_ADMIN_EMAIL and
 * AIM_TEST_ADMIN_PASSWORD, and runs this class. The SDK credentials come from
 * {@code GET /api/v1/sdk/download?sdk=java}, as an application's do, and are
 * written under this test's temporary directory, which is the JVM's
 * {@code user.home} while the class runs.</p>
 */
class HelpersAgainstBackendIntegrationTest {

    private static final MediaType JSON = MediaType.get("application/json; charset=utf-8");
    private static final ObjectMapper MAPPER = new ObjectMapper();
    private static final String CREDENTIALS_ENTRY = ".aim/sdk_credentials.json";

    private static String aimUrl;
    private static String adminToken;
    private static OkHttpClient http;
    private static String previousUserHome;

    @BeforeAll
    static void signInAndInstallSdkCredentials(@TempDir Path home) throws IOException {
        aimUrl = System.getenv("AIM_URL");
        assumeTrue(aimUrl != null && !aimUrl.isEmpty(),
                "AIM_URL is not set; scripts/test-sdk-java-helpers-integration.sh starts a backend and sets it");
        aimUrl = aimUrl.replaceAll("/+$", "");

        // The SDK reads and writes its stored sign-in under user.home. Keep
        // both inside this test's directory, whatever the JVM was started with.
        previousUserHome = System.getProperty("user.home");
        System.setProperty("user.home", home.toString());

        http = new OkHttpClient.Builder()
                .proxy(Proxy.NO_PROXY)
                .callTimeout(Duration.ofSeconds(60))
                .build();

        String password = System.getenv("AIM_TEST_ADMIN_PASSWORD");
        if (password == null || password.isEmpty()) {
            fail("AIM_TEST_ADMIN_PASSWORD is not set; scripts/test-sdk-java-helpers-integration.sh sets it");
        }
        String email = System.getenv().getOrDefault("AIM_TEST_ADMIN_EMAIL", "admin@opena2a.org");
        String login = MAPPER.createObjectNode().put("email", email).put("password", password).toString();
        try (Response response = http.newCall(new Request.Builder()
                .url(aimUrl + "/api/v1/auth/login/local")
                .post(RequestBody.create(login, JSON))
                .build()).execute()) {
            assertEquals(200, response.code(),
                    "admin sign-in answered " + response.code() + "; AIM_URL must name a backend this run bootstrapped");
            adminToken = MAPPER.readTree(response.body().string()).path("accessToken").asText();
        }
        assertTrue(!adminToken.isEmpty(), "admin sign-in returned no accessToken");

        byte[] credentials = null;
        try (Response response = http.newCall(new Request.Builder()
                .url(aimUrl + "/api/v1/sdk/download?sdk=java")
                .header("Authorization", "Bearer " + adminToken)
                .get()
                .build()).execute()) {
            assertEquals(200, response.code(), "the Java SDK download answered " + response.code());
            try (ZipInputStream zip = new ZipInputStream(new ByteArrayInputStream(response.body().bytes()))) {
                for (ZipEntry entry; (entry = zip.getNextEntry()) != null; ) {
                    if (entry.getName().endsWith("/" + CREDENTIALS_ENTRY)) {
                        credentials = zip.readAllBytes();
                        break;
                    }
                }
            }
        }
        assertNotNull(credentials, "the Java SDK download holds no " + CREDENTIALS_ENTRY);

        Path target = home.resolve(CREDENTIALS_ENTRY);
        Files.createDirectories(target.getParent());
        Files.write(target, credentials);
        Files.setPosixFilePermissions(target, PosixFilePermissions.fromString("rw-------"));
    }

    @AfterAll
    static void restoreUserHome() {
        if (previousUserHome != null) {
            System.setProperty("user.home", previousUserHome);
        }
    }

    @Test
    void toolExecutorRecordsNoVerificationOnTheBackend() throws Exception {
        try (AIMClient client = register("java-tool-executor")) {
            List<String> before = recordedActions(client.getAgentId());

            AIMToolExecutor executor = new AIMToolExecutor(client);
            executor.register("searchDatabase", input -> "rows for " + input, "database:search");
            Object registered = executor.execute("searchDatabase", "open orders");
            String inline = executor.execute("getWeather", "Denver", () -> "sunny in Denver");

            assertEquals("rows for open orders", registered);
            assertEquals("sunny in Denver", inline);
            assertEquals(before, recordedActions(client.getAgentId()),
                    "AIMToolExecutor.execute recorded a verification on the backend");

            assertClientRecordsOneVerification(client, before, "database:search");
        }
    }

    @Test
    void functionCallingAdvisorRecordsNoVerificationOnTheBackend() throws Exception {
        try (AIMClient client = register("java-function-advisor")) {
            List<String> before = recordedActions(client.getAgentId());

            AIMFunctionCallingAdvisor advisor = new AIMFunctionCallingAdvisor(client);
            Function<String, String> processPayment = order -> "paid " + order;
            advisor.register("processPayment", processPayment, "payment:process");
            String paid = advisor.execute("processPayment", "order-7");

            // The javadoc's risk-based execution: a function at or above the
            // threshold is refused without an approval.
            Function<String, String> deleteUser = user -> "deleted " + user;
            advisor.register("deleteUser", deleteUser, "user:delete");
            advisor.requireApprovalFor(advisor.getRiskLevel("deleteUser"));
            advisor.setBlockWithoutApproval(true);
            assertThrows(AIMFunctionCallingAdvisor.ApprovalRequiredException.class,
                    () -> advisor.execute("deleteUser", "user-9"));

            assertEquals("paid order-7", paid);
            assertEquals(before, recordedActions(client.getAgentId()),
                    "AIMFunctionCallingAdvisor.execute recorded a verification on the backend");

            assertClientRecordsOneVerification(client, before, "payment:process");
        }
    }

    @Test
    void chatModelListenerRecordsNoVerificationOnTheBackend() throws Exception {
        try (AIMClient client = register("java-chat-listener")) {
            List<String> before = recordedActions(client.getAgentId());

            AIMChatModelListener listener = new AIMChatModelListener(client);
            String requestId = listener.onRequest("What is the order status?", "test-model");
            listener.onResponse(requestId, "The order shipped.", 6, 4);
            String wrapped = listener.wrap("test-model", "Summarize the order", () -> "One order, shipped.");

            assertEquals("One order, shipped.", wrapped);
            assertEquals(before, recordedActions(client.getAgentId()),
                    "AIMChatModelListener recorded a verification on the backend");

            assertClientRecordsOneVerification(client, before, "chat:request");
        }
    }

    /** Registers a new agent through AIMClient.secure, as an application does. */
    private static AIMClient register(String prefix) {
        AIMClient client = AIMClient.secure(prefix + "-" + UUID.randomUUID().toString().substring(0, 8));
        assertNotNull(client.getAgentId(), "AIMClient.secure registered no agent");
        return client;
    }

    /**
     * The client the helper holds records a verification when asked directly,
     * and the backend read sees it.
     */
    private static void assertClientRecordsOneVerification(AIMClient client, List<String> before, String capability)
            throws IOException {
        VerificationResult result = client.verifyCapability(capability, "integration-test");
        assertNotNull(result.getVerificationId(), "verifyCapability returned no verification id");

        List<String> expected = new ArrayList<>(before);
        expected.add(capability);
        Collections.sort(expected);
        assertEquals(expected, recordedActions(client.getAgentId()),
                "the backend did not record the client's own verifyCapability call");
    }

    /** The action of every verification event the backend holds for the agent, sorted. */
    private static List<String> recordedActions(String agentId) throws IOException {
        HttpUrl url = HttpUrl.get(aimUrl + "/api/v1/verification-events/agent/" + agentId).newBuilder()
                .addQueryParameter("limit", "100")
                .build();
        try (Response response = http.newCall(new Request.Builder()
                .url(url)
                .header("Authorization", "Bearer " + adminToken)
                .get()
                .build()).execute()) {
            assertEquals(200, response.code(), "reading verification events answered " + response.code());
            JsonNode events = MAPPER.readTree(response.body().string()).path("events");
            List<String> actions = new ArrayList<>();
            for (JsonNode event : events) {
                assertEquals(agentId, event.path("agentId").asText(), "an event of another agent was returned");
                actions.add(event.path("action").asText());
            }
            Collections.sort(actions);
            return actions;
        }
    }
}
