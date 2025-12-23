package org.opena2a.aim.integration.langchain4j;

import org.junit.jupiter.api.*;
import org.opena2a.aim.client.AIMClient;

import java.util.Map;

import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.Mockito.*;

/**
 * Tests for AIMAgentCallback - LangChain4j agent lifecycle tracking.
 */
class AIMAgentCallbackTest {

    private AIMClient mockClient;
    private AIMAgentCallback callback;

    @BeforeEach
    void setUp() {
        mockClient = mock(AIMClient.class);
        callback = new AIMAgentCallback(mockClient);
    }

    // =========================================================================
    // AGENT LIFECYCLE
    // =========================================================================

    @Test
    @DisplayName("onAgentStart returns execution ID")
    void testOnAgentStartReturnsId() {
        String executionId = callback.onAgentStart("test-agent", "User input");

        assertNotNull(executionId);
        assertFalse(executionId.isEmpty());
    }

    @Test
    @DisplayName("Full agent lifecycle")
    void testFullAgentLifecycle() {
        String executionId = callback.onAgentStart("claims-processor", "Process claim #123");

        callback.onAgentThought(executionId, "I need to look up the claim");
        callback.onAgentAction(executionId, "searchClaims", "123");
        callback.onAgentObservation(executionId, "Found claim: Status=Pending");

        callback.onAgentThought(executionId, "Now I'll process the claim");
        callback.onAgentAction(executionId, "processClaim", "123");
        callback.onAgentObservation(executionId, "Claim processed successfully");

        callback.onAgentFinish(executionId, "Claim #123 has been processed");

        // Should not throw
        assertTrue(true);
    }

    @Test
    @DisplayName("onAgentError records failure")
    void testOnAgentError() {
        String executionId = callback.onAgentStart("test-agent", "input");

        callback.onAgentError(executionId, new RuntimeException("Database connection failed"));

        Map<String, Object> metrics = callback.getMetrics();
        assertEquals(1L, metrics.get("failedExecutions"));
    }

    // =========================================================================
    // REACT PATTERN
    // =========================================================================

    @Test
    @DisplayName("ReAct pattern helper method")
    void testThoughtActionObservation() {
        String executionId = callback.onAgentStart("react-agent", "Question");

        callback.onThoughtActionObservation(executionId,
                "I should search the database",
                "searchDatabase",
                "Found 5 results");

        Map<String, Object> status = callback.getExecutionStatus(executionId);
        assertEquals("running", status.get("status"));
        assertTrue((Integer) status.get("steps") >= 1);
    }

    // =========================================================================
    // ITERATION LIMITS
    // =========================================================================

    @Test
    @DisplayName("shouldContinue respects max iterations")
    void testMaxIterations() {
        callback.setMaxIterations(3);

        String executionId = callback.onAgentStart("test-agent", "input");

        // Simulate 3 steps
        for (int i = 0; i < 3; i++) {
            callback.onAgentThought(executionId, "Step " + i);
        }

        assertFalse(callback.shouldContinue(executionId));
    }

    @Test
    @DisplayName("shouldContinue allows execution within limits")
    void testWithinIterationLimits() {
        callback.setMaxIterations(10);

        String executionId = callback.onAgentStart("test-agent", "input");

        callback.onAgentThought(executionId, "First thought");
        callback.onAgentThought(executionId, "Second thought");

        assertTrue(callback.shouldContinue(executionId));
    }

    @Test
    @DisplayName("shouldContinue returns false for unknown execution")
    void testShouldContinueUnknownExecution() {
        assertFalse(callback.shouldContinue("unknown-id"));
    }

    // =========================================================================
    // EXECUTION STATUS
    // =========================================================================

    @Test
    @DisplayName("Get execution status")
    void testGetExecutionStatus() {
        String executionId = callback.onAgentStart("my-agent", "input");

        callback.onAgentThought(executionId, "thinking");
        callback.onAgentAction(executionId, "someAction", "arg");

        Map<String, Object> status = callback.getExecutionStatus(executionId);

        assertEquals(executionId, status.get("executionId"));
        assertEquals("my-agent", status.get("agent"));
        assertEquals("running", status.get("status"));
        assertEquals(1, status.get("steps"));
        assertEquals(1, status.get("toolCalls"));
    }

    @Test
    @DisplayName("Get status for unknown execution")
    void testGetStatusUnknownExecution() {
        Map<String, Object> status = callback.getExecutionStatus("unknown");
        assertEquals("not_found", status.get("status"));
    }

    // =========================================================================
    // METRICS
    // =========================================================================

    @Test
    @DisplayName("Metrics track total executions")
    void testMetricsTotalExecutions() {
        for (int i = 0; i < 5; i++) {
            String id = callback.onAgentStart("agent-" + i, "input");
            callback.onAgentFinish(id, "done");
        }

        Map<String, Object> metrics = callback.getMetrics();
        assertEquals(5L, metrics.get("totalExecutions"));
        assertEquals(5L, metrics.get("successfulExecutions"));
    }

    @Test
    @DisplayName("Metrics track success rate")
    void testMetricsSuccessRate() {
        // 3 successful
        for (int i = 0; i < 3; i++) {
            String id = callback.onAgentStart("agent", "input");
            callback.onAgentFinish(id, "done");
        }

        // 1 failed
        String failedId = callback.onAgentStart("agent", "input");
        callback.onAgentError(failedId, new RuntimeException("error"));

        Map<String, Object> metrics = callback.getMetrics();
        assertEquals(4L, metrics.get("totalExecutions"));
        assertEquals(3L, metrics.get("successfulExecutions"));
        assertEquals(1L, metrics.get("failedExecutions"));
        assertEquals(0.75, (Double) metrics.get("successRate"), 0.01);
    }

    @Test
    @DisplayName("Metrics track total steps and tool calls")
    void testMetricsStepsAndToolCalls() {
        String id1 = callback.onAgentStart("agent1", "input");
        callback.onAgentThought(id1, "thought 1");
        callback.onAgentAction(id1, "tool1", "arg");
        callback.onAgentThought(id1, "thought 2");
        callback.onAgentAction(id1, "tool2", "arg");
        callback.onAgentFinish(id1, "done");

        String id2 = callback.onAgentStart("agent2", "input");
        callback.onAgentThought(id2, "thought");
        callback.onAgentAction(id2, "tool", "arg");
        callback.onAgentFinish(id2, "done");

        Map<String, Object> metrics = callback.getMetrics();
        assertEquals(3L, metrics.get("totalSteps"));
        assertEquals(3L, metrics.get("totalToolCalls"));
    }

    @Test
    @DisplayName("Metrics track active executions")
    void testMetricsActiveExecutions() {
        String id1 = callback.onAgentStart("agent1", "input");
        String id2 = callback.onAgentStart("agent2", "input");

        Map<String, Object> metrics = callback.getMetrics();
        assertEquals(2, metrics.get("activeExecutions"));

        callback.onAgentFinish(id1, "done");

        metrics = callback.getMetrics();
        assertEquals(1, metrics.get("activeExecutions"));
    }

    // =========================================================================
    // CONFIGURATION
    // =========================================================================

    @Test
    @DisplayName("Configure max iterations")
    void testConfigureMaxIterations() {
        callback.setMaxIterations(5);

        String id = callback.onAgentStart("agent", "input");

        for (int i = 0; i < 4; i++) {
            callback.onAgentThought(id, "thought " + i);
            assertTrue(callback.shouldContinue(id));
        }

        callback.onAgentThought(id, "thought 5");
        assertFalse(callback.shouldContinue(id));
    }

    @Test
    @DisplayName("Configure max duration")
    void testConfigureMaxDuration() throws Exception {
        callback.setMaxDurationMs(100);

        String id = callback.onAgentStart("agent", "input");

        assertTrue(callback.shouldContinue(id));

        Thread.sleep(150);

        assertFalse(callback.shouldContinue(id));
    }

    // =========================================================================
    // EDGE CASES
    // =========================================================================

    @Test
    @DisplayName("Handle null observation gracefully")
    void testNullObservation() {
        String id = callback.onAgentStart("agent", "input");

        assertDoesNotThrow(() -> callback.onAgentObservation(id, null));
    }

    @Test
    @DisplayName("Handle operations on finished execution")
    void testOperationsOnFinishedExecution() {
        String id = callback.onAgentStart("agent", "input");
        callback.onAgentFinish(id, "done");

        // These should not throw, just be no-ops
        assertDoesNotThrow(() -> callback.onAgentThought(id, "thought"));
        assertDoesNotThrow(() -> callback.onAgentAction(id, "action", "arg"));
        assertDoesNotThrow(() -> callback.onAgentObservation(id, "observation"));
    }

    @Test
    @DisplayName("Multiple agents can run concurrently")
    void testConcurrentAgents() {
        String id1 = callback.onAgentStart("agent1", "input1");
        String id2 = callback.onAgentStart("agent2", "input2");
        String id3 = callback.onAgentStart("agent3", "input3");

        callback.onAgentThought(id1, "agent1 thinking");
        callback.onAgentThought(id2, "agent2 thinking");
        callback.onAgentThought(id3, "agent3 thinking");

        callback.onAgentFinish(id2, "agent2 done");
        callback.onAgentFinish(id1, "agent1 done");
        callback.onAgentFinish(id3, "agent3 done");

        Map<String, Object> metrics = callback.getMetrics();
        assertEquals(3L, metrics.get("totalExecutions"));
        assertEquals(3L, metrics.get("successfulExecutions"));
    }
}
