package org.opena2a.aim.integration.langchain4j;

import org.junit.jupiter.api.*;
import org.opena2a.aim.client.AIMClient;
import org.opena2a.aim.security.RiskLevel;

import java.util.Map;
import java.util.Set;

import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.Mockito.*;

/**
 * Tests for AIMToolExecutor - LangChain4j tool execution with security.
 */
class AIMToolExecutorTest {

    private AIMClient mockClient;
    private AIMToolExecutor executor;

    @BeforeEach
    void setUp() {
        mockClient = mock(AIMClient.class);
        executor = new AIMToolExecutor(mockClient);
    }

    // =========================================================================
    // TOOL REGISTRATION
    // =========================================================================

    @Test
    @DisplayName("Register tool with capability")
    void testRegisterTool() {
        executor.register("read_file", input -> "file contents", "file:read");

        Set<String> tools = executor.getRegisteredTools();
        assertTrue(tools.contains("read_file"));
    }

    @Test
    @DisplayName("Register tool with description")
    void testRegisterToolWithDescription() {
        executor.register("read_file", "Reads a file from disk",
                input -> "contents", "file:read");

        Map<String, Object> info = executor.getToolInfo("read_file");
        assertEquals("read_file", info.get("name"));
        assertEquals("Reads a file from disk", info.get("description"));
        assertEquals("file:read", info.get("capability"));
    }

    @Test
    @DisplayName("Register multiple tools")
    void testRegisterMultipleTools() {
        executor.register("tool1", input -> "result1", "cap:tool1");
        executor.register("tool2", input -> "result2", "cap:tool2");
        executor.register("tool3", input -> "result3", "cap:tool3");

        assertEquals(3, executor.getRegisteredTools().size());
    }

    // =========================================================================
    // TOOL EXECUTION
    // =========================================================================

    @Test
    @DisplayName("Execute registered tool")
    void testExecuteTool() {
        executor.register("echo", input -> "Echo: " + input, "tool:echo");

        Object result = executor.execute("echo", "Hello");

        assertEquals("Echo: Hello", result);
    }

    @Test
    @DisplayName("Execute tool with inline action")
    void testExecuteWithInlineAction() {
        executor.register("add", input -> null, "math:add");

        Integer result = executor.execute("add", 5, () -> 5 + 10);

        assertEquals(15, result);
    }

    @Test
    @DisplayName("Unregistered tool throws exception")
    void testUnregisteredToolThrows() {
        assertThrows(AIMToolExecutor.ToolNotRegisteredException.class, () -> {
            executor.execute("unknown_tool", "input");
        });
    }

    // =========================================================================
    // RISK DETECTION
    // =========================================================================

    @Test
    @DisplayName("Tool risk level is detected from capability")
    void testRiskDetection() {
        executor.register("read_data", input -> "data", "db:read");
        executor.register("delete_data", input -> null, "db:delete");
        executor.register("process_payment", input -> null, "payment:process");
        executor.register("admin_action", input -> null, "admin:delete_user");

        Map<String, Object> readInfo = executor.getToolInfo("read_data");
        Map<String, Object> deleteInfo = executor.getToolInfo("delete_data");
        Map<String, Object> paymentInfo = executor.getToolInfo("process_payment");
        Map<String, Object> adminInfo = executor.getToolInfo("admin_action");

        // db:read matches MEDIUM pattern
        assertEquals("MEDIUM", readInfo.get("riskLevel"));
        // db:delete falls to keyword "delete" which is HIGH
        assertEquals("HIGH", deleteInfo.get("riskLevel"));
        // payment:* matches HIGH pattern (not CRITICAL)
        assertEquals("HIGH", paymentInfo.get("riskLevel"));
        // admin:* matches CRITICAL pattern
        assertEquals("CRITICAL", adminInfo.get("riskLevel"));
    }

    // =========================================================================
    // HIGH RISK BLOCKING
    // =========================================================================

    @Test
    @DisplayName("Critical risk tools blocked when threshold is HIGH")
    void testCriticalRiskBlocking() {
        executor.setBlockHighRisk(true);
        // Threshold HIGH means CRITICAL (which is higher) gets blocked
        executor.setApprovalThreshold(RiskLevel.HIGH);

        // admin:* is CRITICAL risk level
        executor.register("admin_action", input -> null, "admin:delete_user");

        assertThrows(AIMToolExecutor.ToolBlockedException.class, () -> {
            executor.execute("admin_action", "user_id");
        });
    }

    @Test
    @DisplayName("Critical risk tools allowed when blocking disabled")
    void testCriticalRiskAllowedWhenNotBlocking() {
        executor.setBlockHighRisk(false);

        // admin:* is CRITICAL but should be allowed when blocking is disabled
        executor.register("admin_action", input -> "completed", "admin:delete_user");

        Object result = executor.execute("admin_action", "user_id");
        assertEquals("completed", result);
    }

    @Test
    @DisplayName("Tools below threshold are allowed")
    void testToolsBelowThresholdAllowed() {
        executor.setBlockHighRisk(true);
        executor.setApprovalThreshold(RiskLevel.CRITICAL);

        // HIGH risk should be allowed when threshold is CRITICAL
        executor.register("delete", input -> "deleted", "db:delete");

        Object result = executor.execute("delete", "id");
        assertEquals("deleted", result);
    }

    // =========================================================================
    // METRICS
    // =========================================================================

    @Test
    @DisplayName("Metrics track successful executions")
    void testMetricsSuccess() {
        executor.register("tool", input -> "result", "test:tool");

        executor.execute("tool", "input1");
        executor.execute("tool", "input2");
        executor.execute("tool", "input3");

        Map<String, Map<String, Object>> metrics = executor.getAllMetrics();
        Map<String, Object> toolMetrics = metrics.get("tool");

        assertEquals(3L, toolMetrics.get("successCount"));
        assertEquals(0L, toolMetrics.get("errorCount"));
        assertEquals(3L, toolMetrics.get("totalExecutions"));
    }

    @Test
    @DisplayName("Metrics track errors")
    void testMetricsErrors() {
        executor.register("failing_tool", input -> {
            throw new RuntimeException("Tool failed");
        }, "test:fail");

        for (int i = 0; i < 3; i++) {
            try {
                executor.execute("failing_tool", "input");
            } catch (RuntimeException ignored) {}
        }

        Map<String, Map<String, Object>> metrics = executor.getAllMetrics();
        Map<String, Object> toolMetrics = metrics.get("failing_tool");

        assertEquals(0L, toolMetrics.get("successCount"));
        assertEquals(3L, toolMetrics.get("errorCount"));
    }

    @Test
    @DisplayName("Metrics track blocked executions")
    void testMetricsBlocked() {
        executor.setBlockHighRisk(true);
        // Threshold MEDIUM means HIGH tools get blocked (HIGH > MEDIUM)
        executor.setApprovalThreshold(RiskLevel.MEDIUM);

        // payment:process is HIGH risk
        executor.register("high_risk_tool", input -> null, "payment:process");

        for (int i = 0; i < 3; i++) {
            try {
                executor.execute("high_risk_tool", "input");
            } catch (AIMToolExecutor.ToolBlockedException ignored) {}
        }

        Map<String, Map<String, Object>> metrics = executor.getAllMetrics();
        Map<String, Object> toolMetrics = metrics.get("high_risk_tool");

        assertEquals(3L, toolMetrics.get("blockedCount"));
    }

    @Test
    @DisplayName("Metrics track latency")
    void testMetricsLatency() {
        executor.register("slow_tool", input -> {
            try { Thread.sleep(50); } catch (InterruptedException ignored) {}
            return "done";
        }, "test:slow");

        executor.execute("slow_tool", "input");

        Map<String, Map<String, Object>> metrics = executor.getAllMetrics();
        Map<String, Object> toolMetrics = metrics.get("slow_tool");

        assertTrue((Long) toolMetrics.get("avgLatencyMs") >= 50);
    }

    // =========================================================================
    // TOOL INFO
    // =========================================================================

    @Test
    @DisplayName("Get tool info for non-existent tool returns empty")
    void testToolInfoNonExistent() {
        Map<String, Object> info = executor.getToolInfo("unknown");
        assertTrue(info.isEmpty());
    }

    // =========================================================================
    // EXCEPTION TYPES
    // =========================================================================

    @Test
    @DisplayName("ToolNotRegisteredException has correct message")
    void testToolNotRegisteredExceptionMessage() {
        AIMToolExecutor.ToolNotRegisteredException ex =
                new AIMToolExecutor.ToolNotRegisteredException("my_tool");

        assertTrue(ex.getMessage().contains("my_tool"));
    }

    @Test
    @DisplayName("ToolBlockedException contains tool info")
    void testToolBlockedExceptionInfo() {
        AIMToolExecutor.ToolBlockedException ex =
                new AIMToolExecutor.ToolBlockedException("payment_tool", RiskLevel.CRITICAL);

        assertEquals("payment_tool", ex.getToolName());
        assertEquals(RiskLevel.CRITICAL, ex.getRiskLevel());
        assertTrue(ex.getMessage().contains("payment_tool"));
        assertTrue(ex.getMessage().contains("CRITICAL"));
    }

    // =========================================================================
    // CONFIGURATION
    // =========================================================================

    @Test
    @DisplayName("Set approval threshold")
    void testSetApprovalThreshold() {
        executor.setApprovalThreshold(RiskLevel.MEDIUM);
        executor.setBlockHighRisk(true);

        executor.register("high_risk", input -> null, "db:delete");

        // HIGH risk should be blocked when threshold is MEDIUM
        assertThrows(AIMToolExecutor.ToolBlockedException.class, () -> {
            executor.execute("high_risk", "input");
        });
    }
}
