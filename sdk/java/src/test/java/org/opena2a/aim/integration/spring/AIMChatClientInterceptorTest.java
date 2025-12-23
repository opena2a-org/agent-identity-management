package org.opena2a.aim.integration.spring;

import org.junit.jupiter.api.*;
import org.opena2a.aim.client.AIMClient;

import java.util.Map;

import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.Mockito.*;

/**
 * Tests for AIMChatClientInterceptor - Spring AI chat interception.
 */
class AIMChatClientInterceptorTest {

    private AIMClient mockClient;
    private AIMChatClientInterceptor interceptor;

    @BeforeEach
    void setUp() {
        mockClient = mock(AIMClient.class);
        interceptor = new AIMChatClientInterceptor(mockClient);
    }

    // =========================================================================
    // BASIC INTERCEPTION
    // =========================================================================

    @Test
    @DisplayName("beforeCall returns correlation ID")
    void testBeforeCallReturnsCorrelationId() {
        String correlationId = interceptor.beforeCall("Hello world", "gpt-4");

        assertNotNull(correlationId);
        assertFalse(correlationId.isEmpty());
    }

    @Test
    @DisplayName("afterCall updates metrics")
    void testAfterCallUpdatesMetrics() {
        String correlationId = interceptor.beforeCall("prompt", "model");

        interceptor.afterCall(correlationId, "response", 100, 500);

        Map<String, Object> metrics = interceptor.getMetrics();
        assertEquals(1L, metrics.get("totalCalls"));
        assertEquals(100L, metrics.get("totalTokens"));
        assertEquals(500L, metrics.get("totalLatencyMs"));
    }

    @Test
    @DisplayName("onError increments error count")
    void testOnErrorIncrementsErrorCount() {
        String correlationId = interceptor.beforeCall("prompt", "model");

        interceptor.onError(correlationId, new RuntimeException("Test error"));

        Map<String, Object> metrics = interceptor.getMetrics();
        assertEquals(1L, metrics.get("errorCount"));
    }

    // =========================================================================
    // METRICS
    // =========================================================================

    @Test
    @DisplayName("Metrics track multiple calls")
    void testMetricsMultipleCalls() {
        for (int i = 0; i < 5; i++) {
            String correlationId = interceptor.beforeCall("prompt " + i, "model");
            interceptor.afterCall(correlationId, "response " + i, 50, 100);
        }

        Map<String, Object> metrics = interceptor.getMetrics();

        assertEquals(5L, metrics.get("totalCalls"));
        assertEquals(250L, metrics.get("totalTokens"));
        assertEquals(500L, metrics.get("totalLatencyMs"));
        assertEquals(50L, metrics.get("avgTokensPerCall"));
        assertEquals(100L, metrics.get("avgLatencyMs"));
    }

    @Test
    @DisplayName("Metrics calculate error rate")
    void testMetricsErrorRate() {
        // 4 successful calls
        for (int i = 0; i < 4; i++) {
            String correlationId = interceptor.beforeCall("prompt", "model");
            interceptor.afterCall(correlationId, "response", 10, 10);
        }

        // 1 failed call
        String failedCorrelationId = interceptor.beforeCall("prompt", "model");
        interceptor.onError(failedCorrelationId, new RuntimeException("error"));

        Map<String, Object> metrics = interceptor.getMetrics();

        assertEquals(4L, metrics.get("totalCalls")); // Only successful calls counted
        assertEquals(1L, metrics.get("errorCount"));
        // Error rate would be calculated based on total attempts if needed
    }

    @Test
    @DisplayName("Reset metrics clears all counters")
    void testResetMetrics() {
        String correlationId = interceptor.beforeCall("prompt", "model");
        interceptor.afterCall(correlationId, "response", 100, 500);

        interceptor.resetMetrics();

        Map<String, Object> metrics = interceptor.getMetrics();
        assertEquals(0L, metrics.get("totalCalls"));
        assertEquals(0L, metrics.get("totalTokens"));
        assertEquals(0L, metrics.get("totalLatencyMs"));
        assertEquals(0L, metrics.get("errorCount"));
    }

    // =========================================================================
    // WRAP METHOD
    // =========================================================================

    @Test
    @DisplayName("wrap executes action and returns result")
    void testWrapExecutesAction() {
        String result = interceptor.wrap("prompt", "model", () -> "Hello World");

        assertEquals("Hello World", result);
    }

    @Test
    @DisplayName("wrap records metrics automatically")
    void testWrapRecordsMetrics() {
        interceptor.wrap("prompt", "model", () -> "result");

        Map<String, Object> metrics = interceptor.getMetrics();
        assertEquals(1L, metrics.get("totalCalls"));
        assertTrue((Long) metrics.get("totalLatencyMs") >= 0);
    }

    @Test
    @DisplayName("wrap handles exceptions and records error")
    void testWrapHandlesExceptions() {
        RuntimeException expectedException = new RuntimeException("Test exception");

        assertThrows(RuntimeException.class, () -> {
            interceptor.wrap("prompt", "model", () -> {
                throw expectedException;
            });
        });

        Map<String, Object> metrics = interceptor.getMetrics();
        assertEquals(1L, metrics.get("errorCount"));
    }

    // =========================================================================
    // CONFIGURATION
    // =========================================================================

    @Test
    @DisplayName("Config builder works")
    void testConfigBuilder() {
        AIMChatClientInterceptor.Config config = new AIMChatClientInterceptor.Config()
                .logPrompts(true)
                .logResponses(true)
                .maxPromptLogLength(1000)
                .maxResponseLogLength(1000)
                .enableMetrics(true);

        AIMChatClientInterceptor configuredInterceptor = new AIMChatClientInterceptor(mockClient, config);

        // Should work without errors
        String correlationId = configuredInterceptor.beforeCall("test prompt", "model");
        assertNotNull(correlationId);
    }

    @Test
    @DisplayName("Config with logging disabled")
    void testConfigLoggingDisabled() {
        AIMChatClientInterceptor.Config config = new AIMChatClientInterceptor.Config(false, false);

        AIMChatClientInterceptor configuredInterceptor = new AIMChatClientInterceptor(mockClient, config);

        // Should work without errors
        String correlationId = configuredInterceptor.beforeCall("test prompt", "model");
        configuredInterceptor.afterCall(correlationId, "response", 10, 10);

        assertNotNull(correlationId);
    }

    // =========================================================================
    // EDGE CASES
    // =========================================================================

    @Test
    @DisplayName("Handle null prompt")
    void testNullPrompt() {
        assertDoesNotThrow(() -> {
            String correlationId = interceptor.beforeCall(null, "model");
            interceptor.afterCall(correlationId, "response", 10, 10);
        });
    }

    @Test
    @DisplayName("Handle null response")
    void testNullResponse() {
        assertDoesNotThrow(() -> {
            String correlationId = interceptor.beforeCall("prompt", "model");
            interceptor.afterCall(correlationId, null, 10, 10);
        });
    }

    @Test
    @DisplayName("Handle null model")
    void testNullModel() {
        assertDoesNotThrow(() -> {
            String correlationId = interceptor.beforeCall("prompt", null);
            interceptor.afterCall(correlationId, "response", 10, 10);
        });
    }

    @Test
    @DisplayName("Very long prompt is handled")
    void testLongPrompt() {
        String longPrompt = "x".repeat(10000);

        assertDoesNotThrow(() -> {
            String correlationId = interceptor.beforeCall(longPrompt, "model");
            interceptor.afterCall(correlationId, "response", 2500, 100);
        });
    }
}
