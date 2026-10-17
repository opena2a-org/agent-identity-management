package org.opena2a.aim.security;

import com.fasterxml.jackson.databind.DeserializationFeature;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.MappingIterator;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ObjectNode;
import okhttp3.MediaType;
import okhttp3.OkHttpClient;
import okhttp3.Request;
import okhttp3.RequestBody;
import okhttp3.Response;
import okhttp3.mockwebserver.Dispatcher;
import okhttp3.mockwebserver.MockResponse;
import okhttp3.mockwebserver.MockWebServer;
import okhttp3.mockwebserver.RecordedRequest;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;

import java.io.IOException;
import java.math.BigDecimal;
import java.math.RoundingMode;
import java.net.InetAddress;
import java.net.Proxy;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Collections;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.TreeSet;
import java.util.UUID;
import java.util.concurrent.CopyOnWriteArrayList;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;

/**
 * Measures SecurityLogger's file output against Splunk HTTP Event Collector's
 * published event format.
 *
 * <p>Every public {@code log*} method of SecurityLogger logs one event to a log
 * file in this test's temporary directory. Each line the file gains must be one
 * JSON object, and each line, sent unchanged as the {@code event} of one HEC
 * event, must be accepted by {@link HecReceiver}.</p>
 *
 * <p>{@link HecReceiver} follows two Splunk Enterprise 10.6 pages, "Format events
 * for HTTP Event Collector" and "Troubleshoot HTTP Event Collector", and nothing
 * in SecurityLogger: {@code POST /services/collector/event},
 * {@code Authorization: Splunk <token>}, a body of one or more JSON event objects
 * (concatenated, or in a JSON array), each carrying {@code event}, with the
 * optional {@code time} (epoch seconds), {@code host}, {@code source},
 * {@code sourcetype}, {@code index} and a flat {@code fields} object. Its
 * refusals use the status codes and messages of the troubleshooting page's
 * table. No request leaves the loopback interface.</p>
 */
class SecurityLoggerHecFormatTest {

    private static final MediaType JSON = MediaType.get("application/json; charset=utf-8");

    // SecurityLogger writes java.time.Instant#toString: ISO-8601 in UTC, with
    // 0, 3, 6 or 9 fractional digits.
    private static final Pattern ISO_8601_UTC =
            Pattern.compile("^\\d{4}-\\d{2}-\\d{2}T\\d{2}:\\d{2}:\\d{2}(\\.(\\d{3}|\\d{6}|\\d{9}))?Z$");

    private static final ObjectMapper STRICT = new ObjectMapper()
            .enable(DeserializationFeature.FAIL_ON_TRAILING_TOKENS);

    @TempDir
    Path tempDir;

    private HecReceiver receiver;
    private OkHttpClient http;
    private String token;

    @BeforeEach
    void startReceiver() throws IOException {
        token = UUID.randomUUID().toString();
        receiver = new HecReceiver(token, Set.of("main"));
        http = new OkHttpClient.Builder().proxy(Proxy.NO_PROXY).build();
    }

    @AfterEach
    void stopReceiver() throws IOException {
        receiver.close();
        // Restore the singleton's default level and stderr output for the
        // tests that run after this one in the same JVM.
        SecurityLogger.getInstance().configure(SecurityLogger.Config.builder().build());
        SecurityLogger.getInstance().clearAgentContext();
    }

    @Test
    void everyLogMethodWritesOneJsonObjectLineThatHecAcceptsAsTheEventField() throws Exception {
        Path logFile = tempDir.resolve("logs").resolve("security.jsonl");
        SecurityLogger logger = SecurityLogger.getInstance();
        logger.configure(SecurityLogger.Config.builder()
                .logFile(logFile.toString())
                .logLevel(EventSeverity.DEBUG)
                .logToStderr(false)
                .build());
        logger.setAgentContext("agent-hec-format", "hec-format-agent", "user-hec-format", "org-hec-format");

        Map<String, String> details = new LinkedHashMap<>();
        details.put("note", "a value with \"quotes\" and a\nnewline");
        Map<String, Object> objectDetails = new LinkedHashMap<>(details);
        objectDetails.put("nested", Map.of("count", 2, "flags", List.of("a", "b")));

        Map<String, Runnable> calls = new LinkedHashMap<>();
        calls.put("logAuthentication(4)", () -> logger.logAuthentication(
                EventTypes.Authn.TOKEN_REFRESH, true, "Token refreshed", objectDetails));
        calls.put("logAuthentication(6)", () -> logger.logAuthentication(
                EventTypes.Authn.TOKEN_REVOKED, false, "Token revoked", "agent-other", objectDetails, "revoked"));
        calls.put("logAuthorization(4)", () -> logger.logAuthorization(
                EventTypes.Authz.CAPABILITY_GRANTED, "db:read", "users_table", true));
        calls.put("logAuthorization(7)", () -> logger.logAuthorization(
                EventTypes.Authz.CAPABILITY_ESCALATION, "db:write", null, false, "agent-other", objectDetails, "denied"));
        calls.put("logAuthorizationEvent(5)", () -> logger.logAuthorizationEvent(
                EventTypes.Authz.ACTION_EXECUTED, "chat:request", "model-a", true, objectDetails));
        calls.put("logAgentEvent(4)", () -> logger.logAgentEvent(
                EventTypes.Agent.AGENT_REGISTERED, "agent-other", "other-agent", null));
        calls.put("logAgentEvent(6)", () -> logger.logAgentEvent(
                EventTypes.Agent.AGENT_REGISTRATION_FAILED, null, null, "Registration failed", objectDetails, "conflict"));
        calls.put("logAgentEvent(5)", () -> logger.logAgentEvent(
                EventTypes.Agent.AGENT_UPDATED, "other-agent", "Agent updated", objectDetails, false));
        calls.put("logCredentialEvent(3)", () -> logger.logCredentialEvent(
                EventTypes.Cred.CREDENTIAL_LOADED, "sdk_oauth", true));
        calls.put("logCredentialEvent(6)", () -> logger.logCredentialEvent(
                EventTypes.Cred.CREDENTIAL_TYPE_MISMATCH, "agent", false, "other-agent", objectDetails, "mismatch"));
        calls.put("logMcpEvent(4)", () -> logger.logMcpEvent(
                EventTypes.Mcp.MCP_SERVER_CONNECTED, "server-1", "filesystem", true));
        calls.put("logMcpEvent(5)", () -> logger.logMcpEvent(
                EventTypes.Mcp.MCP_TOOL_USED, "filesystem", "Tool execution started", objectDetails, true));
        calls.put("logMcpEvent(8)", () -> logger.logMcpEvent(
                EventTypes.Mcp.MCP_ATTESTATION_FAILED, "server-1", "filesystem", "read_file", "agent-other",
                false, objectDetails, "attestation failed"));
        calls.put("logSecurityEvent(3)", () -> logger.logSecurityEvent(
                EventTypes.Security.ANOMALY_DETECTED, "A message on\ntwo lines", EventSeverity.WARNING));
        calls.put("logSecurityEvent(7)", () -> logger.logSecurityEvent(
                EventTypes.Security.POLICY_VIOLATION, "Policy violated", EventSeverity.CRITICAL,
                "agent-other", "192.0.2.10", objectDetails, "violation"));

        // One call adds exactly one line.
        List<String> lines = new ArrayList<>();
        for (Map.Entry<String, Runnable> call : calls.entrySet()) {
            int before = lineCount(logFile);
            call.getValue().run();
            List<String> now = Files.readAllLines(logFile, StandardCharsets.UTF_8);
            assertEquals(before + 1, now.size(), call.getKey() + " must add exactly one line to the log file");
            lines.add(now.get(now.size() - 1));
        }

        Map<String, String> refusals = new LinkedHashMap<>();
        Set<String> timestampFormats = new TreeSet<>();
        int index = 0;
        for (Map.Entry<String, Runnable> call : calls.entrySet()) {
            String name = call.getKey();
            String line = lines.get(index++);

            JsonNode parsed = STRICT.readTree(line);
            assertTrue(parsed.isObject(), name + " must write one JSON object per line, wrote: " + parsed.getNodeType());

            // The line names its time in "timestamp", an ISO-8601 UTC string.
            // HEC's "time" is epoch seconds, so a sender converts it.
            assertFalse(parsed.has("time"), name + ": a SecurityLogger line carries no HEC \"time\" member");
            JsonNode timestamp = parsed.get("timestamp");
            assertTrue(timestamp != null && timestamp.isTextual(), name + " must carry a \"timestamp\" string");
            Matcher matcher = ISO_8601_UTC.matcher(timestamp.asText());
            assertTrue(matcher.matches(), name + ": \"timestamp\" is not ISO-8601 UTC: " + timestamp.asText());
            Instant instant = Instant.parse(timestamp.asText());
            timestampFormats.add("ISO-8601 UTC, " + (matcher.group(2) == null ? 0 : matcher.group(2).length())
                    + " fractional digits");

            ObjectNode metadata = STRICT.createObjectNode();
            metadata.put("time", epochSeconds(instant));
            metadata.put("host", parsed.path("hostname").asText("unknown"));
            metadata.put("source", "aim-sdk-java");
            metadata.put("sourcetype", "_json");
            metadata.put("index", "main");
            ObjectNode fields = metadata.putObject("fields");
            fields.put("category", parsed.path("category").asText());
            fields.put("severity", parsed.path("severity").asText());
            String metadataJson = STRICT.writeValueAsString(metadata);
            // The line is sent byte for byte as the value of "event".
            String body = metadataJson.substring(0, metadataJson.length() - 1) + ",\"event\":" + line + "}";

            HecAnswer answer = send(body, "Splunk " + token);
            if (answer.status != 200 || answer.code != 0) {
                refusals.put(name, answer.toString());
                continue;
            }
            assertEquals(parsed, receiver.lastAccepted().get("event"),
                    name + ": the receiver must hold the line as the event's \"event\" member");
        }

        System.out.println("SecurityLogger HEC format: " + lines.size() + " lines, "
                + (lines.size() - refusals.size()) + " accepted, " + refusals.size() + " refused; "
                + "timestamp field \"timestamp\": " + timestampFormats);
        assertTrue(refusals.isEmpty(), "the HEC receiver refused: " + refusals);
        assertEquals(calls.size(), receiver.accepted().size());
    }

    @Test
    void receiverRefusesWhatThePublishedFormatRefuses() throws Exception {
        String auth = "Splunk " + token;

        assertEquals(new HecAnswer(401, 2, "Token is required"), send("{\"event\":\"x\"}", null));
        assertEquals(new HecAnswer(401, 3, "Invalid authorization"), send("{\"event\":\"x\"}", "Bearer " + token));
        assertEquals(new HecAnswer(403, 4, "Invalid token"), send("{\"event\":\"x\"}", "Splunk " + UUID.randomUUID()));
        assertEquals(new HecAnswer(400, 5, "No data"), send("", auth));
        assertEquals(new HecAnswer(400, 6, "Invalid data format"), send("{\"event\":", auth));
        assertEquals(new HecAnswer(400, 6, "Invalid data format"), send("[\"x\"]", auth));
        assertEquals(new HecAnswer(400, 6, "Invalid data format"), send("{\"event\":\"x\",\"time\":\"yesterday\"}", auth));
        assertEquals(new HecAnswer(400, 7, "Incorrect index"), send("{\"event\":\"x\",\"index\":\"other\"}", auth));
        assertEquals(new HecAnswer(400, 12, "Event field is required"), send("{\"time\":1433188255.5,\"host\":\"h\"}", auth));
        assertEquals(new HecAnswer(400, 13, "Event field cannot be blank"), send("{\"event\":\"\"}", auth));
        assertEquals(new HecAnswer(400, 15, "Error in handling indexed fields"),
                send("{\"event\":\"x\",\"fields\":{\"a\":{\"b\":1}}}", auth));
        assertTrue(receiver.accepted().isEmpty(), "a refused request must record no event");

        // The page accepts a batch as concatenated objects or as a JSON array.
        assertEquals(new HecAnswer(200, 0, "Success"), send("{\"event\":\"a\"}{\"event\":{\"b\":1}}", auth));
        assertEquals(new HecAnswer(200, 0, "Success"), send("[{\"event\":\"c\",\"time\":1433188255}]", auth));
        assertEquals(3, receiver.accepted().size());
    }

    private HecAnswer send(String body, String authorization) throws IOException {
        Request.Builder request = new Request.Builder()
                .url(receiver.url())
                .post(RequestBody.create(body, JSON));
        if (authorization != null) {
            request.header("Authorization", authorization);
        }
        try (Response response = http.newCall(request.build()).execute()) {
            String text = response.body() != null ? response.body().string() : "";
            JsonNode answer = STRICT.readTree(text);
            return new HecAnswer(response.code(), answer.path("code").asInt(-1), answer.path("text").asText());
        }
    }

    private static int lineCount(Path file) throws IOException {
        return Files.exists(file) ? Files.readAllLines(file, StandardCharsets.UTF_8).size() : 0;
    }

    private static BigDecimal epochSeconds(Instant instant) {
        return BigDecimal.valueOf(instant.getEpochSecond())
                .add(BigDecimal.valueOf(instant.getNano(), 9))
                .setScale(3, RoundingMode.DOWN);
    }

    private static final class HecAnswer {
        final int status;
        final int code;
        final String text;

        HecAnswer(int status, int code, String text) {
            this.status = status;
            this.code = code;
            this.text = text;
        }

        @Override
        public boolean equals(Object other) {
            if (!(other instanceof HecAnswer)) {
                return false;
            }
            HecAnswer that = (HecAnswer) other;
            return status == that.status && code == that.code && text.equals(that.text);
        }

        @Override
        public int hashCode() {
            return 31 * (31 * status + code) + text.hashCode();
        }

        @Override
        public String toString() {
            return "HTTP " + status + " {\"text\":\"" + text + "\",\"code\":" + code + "}";
        }
    }

    /**
     * A loopback HTTP Event Collector endpoint written from Splunk's two HEC
     * pages. It answers {@code {"text":<status message>,"code":<status code>}}
     * with the HTTP status the troubleshooting page's table gives that code.
     */
    static final class HecReceiver implements AutoCloseable {
        static final String EVENT_PATH = "/services/collector/event";
        private static final Pattern SPLUNK_AUTHORIZATION = Pattern.compile("^Splunk (\\S+)$");
        private static final Set<String> STRING_METADATA = Set.of("host", "source", "sourcetype", "index");

        private final String token;
        private final Set<String> indexes;
        private final MockWebServer server = new MockWebServer();
        private final List<JsonNode> accepted = new CopyOnWriteArrayList<>();
        private final ObjectMapper mapper = new ObjectMapper();

        HecReceiver(String token, Set<String> indexes) throws IOException {
            this.token = token;
            this.indexes = indexes;
            server.setDispatcher(new Dispatcher() {
                @Override
                public MockResponse dispatch(RecordedRequest request) {
                    return receive(request);
                }
            });
            server.start(InetAddress.getLoopbackAddress(), 0);
        }

        String url() {
            return server.url(EVENT_PATH).toString();
        }

        List<JsonNode> accepted() {
            return Collections.unmodifiableList(accepted);
        }

        JsonNode lastAccepted() {
            return accepted.get(accepted.size() - 1);
        }

        private MockResponse receive(RecordedRequest request) {
            if (!EVENT_PATH.equals(request.getRequestUrl().encodedPath()) || !"POST".equals(request.getMethod())) {
                return new MockResponse().setResponseCode(404);
            }
            String authorization = request.getHeader("Authorization");
            if (authorization == null || authorization.isEmpty()) {
                return answer(401, 2, "Token is required");
            }
            Matcher matcher = SPLUNK_AUTHORIZATION.matcher(authorization);
            if (!matcher.matches()) {
                return answer(401, 3, "Invalid authorization");
            }
            if (!token.equals(matcher.group(1))) {
                return answer(403, 4, "Invalid token");
            }

            String body = request.getBody().readUtf8();
            if (body.trim().isEmpty()) {
                return answer(400, 5, "No data");
            }
            List<JsonNode> events = new ArrayList<>();
            try (MappingIterator<JsonNode> values = mapper.readerFor(JsonNode.class).readValues(body)) {
                while (values.hasNextValue()) {
                    events.add(values.nextValue());
                }
            } catch (IOException | RuntimeException e) {
                return answer(400, 6, "Invalid data format");
            }
            if (events.isEmpty()) {
                return answer(400, 5, "No data");
            }
            for (JsonNode event : events) {
                MockResponse refusal = refusalOf(event);
                if (refusal != null) {
                    return refusal;
                }
            }
            accepted.addAll(events);
            return answer(200, 0, "Success");
        }

        private MockResponse refusalOf(JsonNode event) {
            if (!event.isObject()) {
                return answer(400, 6, "Invalid data format");
            }
            JsonNode time = event.get("time");
            if (time != null && !time.isNumber() && !(time.isTextual() && isNumber(time.asText()))) {
                return answer(400, 6, "Invalid data format");
            }
            for (String key : STRING_METADATA) {
                JsonNode value = event.get(key);
                if (value != null && !value.isTextual()) {
                    return answer(400, 6, "Invalid data format");
                }
            }
            JsonNode index = event.get("index");
            if (index != null && !indexes.contains(index.asText())) {
                return answer(400, 7, "Incorrect index");
            }
            JsonNode fields = event.get("fields");
            if (fields != null) {
                if (!fields.isObject()) {
                    return answer(400, 15, "Error in handling indexed fields");
                }
                for (JsonNode value : fields) {
                    if (value.isObject()) {
                        return answer(400, 15, "Error in handling indexed fields");
                    }
                }
            }
            if (!event.has("event")) {
                return answer(400, 12, "Event field is required");
            }
            JsonNode value = event.get("event");
            if (value.isNull() || (value.isTextual() && value.asText().isEmpty())) {
                return answer(400, 13, "Event field cannot be blank");
            }
            return null;
        }

        private static boolean isNumber(String text) {
            try {
                new BigDecimal(text);
                return true;
            } catch (NumberFormatException e) {
                return false;
            }
        }

        private MockResponse answer(int status, int code, String text) {
            ObjectNode body = mapper.createObjectNode();
            body.put("text", text);
            body.put("code", code);
            return new MockResponse()
                    .setResponseCode(status)
                    .setHeader("Content-Type", "application/json")
                    .setBody(body.toString());
        }

        @Override
        public void close() throws IOException {
            server.shutdown();
        }
    }
}
