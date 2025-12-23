package org.opena2a.aim.integration.langchain4j;

import org.opena2a.aim.client.AIMClient;
import org.opena2a.aim.security.EventTypes;
import org.opena2a.aim.security.RiskDetector;
import org.opena2a.aim.security.RiskLevel;
import org.opena2a.aim.security.SecurityLogger;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.util.*;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.atomic.AtomicLong;
import java.util.function.Function;

/**
 * Tool executor for LangChain4j with AIM security integration.
 *
 * <p>Wraps LangChain4j tool executions with:</p>
 * <ul>
 *   <li>Risk assessment before execution</li>
 *   <li>Capability-based access control</li>
 *   <li>Execution logging for audit trail</li>
 *   <li>Metrics and monitoring</li>
 * </ul>
 *
 * <h2>Usage:</h2>
 * <pre>{@code
 * AIMToolExecutor executor = new AIMToolExecutor(aimClient);
 *
 * // Register tools with capabilities
 * executor.register("getWeather", this::getWeather, "api:weather");
 * executor.register("searchDatabase", this::searchDatabase, "database:search");
 * executor.register("processPayment", this::processPayment, "payment:process");
 *
 * // Execute with security checks
 * Object result = executor.execute("searchDatabase", args);
 * }</pre>
 *
 * <h2>Integration with LangChain4j Tools:</h2>
 * <pre>{@code
 * @Tool("Search the database")
 * public String searchDatabase(@P("query") String query) {
 *     return executor.execute("searchDatabase", query,
 *         () -> actualSearchDatabase(query));
 * }
 * }</pre>
 */
public class AIMToolExecutor {

    private static final Logger log = LoggerFactory.getLogger(AIMToolExecutor.class);

    private final AIMClient client;
    private final RiskDetector riskDetector;
    private final SecurityLogger securityLogger;

    // Registered tools
    private final Map<String, ToolRegistration> tools = new ConcurrentHashMap<>();

    // Metrics
    private final Map<String, ToolMetrics> metrics = new ConcurrentHashMap<>();

    // Configuration
    private RiskLevel approvalThreshold = RiskLevel.CRITICAL;
    private boolean blockHighRisk = false;

    /**
     * Creates a new tool executor with the specified AIM client.
     *
     * @param client the AIM client for security logging
     */
    public AIMToolExecutor(AIMClient client) {
        this.client = client;
        this.riskDetector = RiskDetector.getInstance();
        this.securityLogger = SecurityLogger.getInstance();
    }

    /**
     * Register a tool with its capability.
     *
     * @param name Tool name
     * @param tool Tool function
     * @param capability Capability string for risk assessment
     */
    public void register(String name, Function<Object, Object> tool, String capability) {
        ToolRegistration reg = new ToolRegistration();
        reg.name = name;
        reg.tool = tool;
        reg.capability = capability;
        reg.riskLevel = riskDetector.detectRisk(capability);

        tools.put(name, reg);
        metrics.put(name, new ToolMetrics());

        log.info("Registered tool '{}' with capability '{}' (risk: {})",
                name, capability, reg.riskLevel);
    }

    /**
     * Register a tool with description.
     *
     * @param name Tool name
     * @param description Tool description
     * @param tool Tool function
     * @param capability Capability string for risk assessment
     */
    public void register(String name, String description, Function<Object, Object> tool, String capability) {
        ToolRegistration reg = new ToolRegistration();
        reg.name = name;
        reg.description = description;
        reg.tool = tool;
        reg.capability = capability;
        reg.riskLevel = riskDetector.detectRisk(capability);

        tools.put(name, reg);
        metrics.put(name, new ToolMetrics());

        log.info("Registered tool '{}' ({}) with capability '{}' (risk: {})",
                name, description, capability, reg.riskLevel);
    }

    /**
     * Execute a registered tool.
     *
     * @param toolName Tool name
     * @param input Input to the tool
     * @return Tool output
     */
    public Object execute(String toolName, Object input) {
        ToolRegistration reg = tools.get(toolName);
        if (reg == null) {
            throw new ToolNotRegisteredException(toolName);
        }

        return executeInternal(reg, input);
    }

    /**
     * Execute with inline tool function (for @Tool annotation integration).
     *
     * @param <T> the return type of the tool action
     * @param toolName Tool name (for logging)
     * @param input Input value
     * @param action The actual tool action
     * @return Tool output
     */
    public <T> T execute(String toolName, Object input, ToolAction<T> action) {
        ToolRegistration reg = tools.get(toolName);
        String capability = reg != null ? reg.capability : "tool:" + toolName;
        RiskLevel riskLevel = reg != null ? reg.riskLevel : riskDetector.detectRisk(capability);

        ToolMetrics toolMetrics = metrics.computeIfAbsent(toolName, k -> new ToolMetrics());

        // Risk check
        if (blockHighRisk && riskLevel.isHigherThan(approvalThreshold)) {
            toolMetrics.blockedCount.incrementAndGet();
            logToolEvent(toolName, capability, riskLevel, input, null, false, "blocked_high_risk");
            throw new ToolBlockedException(toolName, riskLevel);
        }

        // Pre-execution log
        String executionId = UUID.randomUUID().toString();
        logToolStart(executionId, toolName, capability, riskLevel, input);

        long startTime = System.currentTimeMillis();
        try {
            T result = action.execute();

            long latencyMs = System.currentTimeMillis() - startTime;
            toolMetrics.successCount.incrementAndGet();
            toolMetrics.totalLatencyMs.addAndGet(latencyMs);

            logToolComplete(executionId, toolName, capability, latencyMs, result, true, null);
            return result;

        } catch (Exception e) {
            long latencyMs = System.currentTimeMillis() - startTime;
            toolMetrics.errorCount.incrementAndGet();

            logToolComplete(executionId, toolName, capability, latencyMs, null, false, e.getMessage());
            throw e instanceof RuntimeException ? (RuntimeException) e : new RuntimeException(e);
        }
    }

    /**
     * Internal execution with full tracking.
     */
    private Object executeInternal(ToolRegistration reg, Object input) {
        ToolMetrics toolMetrics = metrics.get(reg.name);

        // Risk check
        if (blockHighRisk && reg.riskLevel.isHigherThan(approvalThreshold)) {
            toolMetrics.blockedCount.incrementAndGet();
            logToolEvent(reg.name, reg.capability, reg.riskLevel, input, null, false, "blocked_high_risk");
            throw new ToolBlockedException(reg.name, reg.riskLevel);
        }

        // Pre-execution log
        String executionId = UUID.randomUUID().toString();
        logToolStart(executionId, reg.name, reg.capability, reg.riskLevel, input);

        long startTime = System.currentTimeMillis();
        try {
            Object result = reg.tool.apply(input);

            long latencyMs = System.currentTimeMillis() - startTime;
            toolMetrics.successCount.incrementAndGet();
            toolMetrics.totalLatencyMs.addAndGet(latencyMs);

            logToolComplete(executionId, reg.name, reg.capability, latencyMs, result, true, null);
            return result;

        } catch (Exception e) {
            long latencyMs = System.currentTimeMillis() - startTime;
            toolMetrics.errorCount.incrementAndGet();

            logToolComplete(executionId, reg.name, reg.capability, latencyMs, null, false, e.getMessage());
            throw e instanceof RuntimeException ? (RuntimeException) e : new RuntimeException(e);
        }
    }

    private void logToolStart(String executionId, String toolName, String capability,
                               RiskLevel riskLevel, Object input) {
        Map<String, Object> details = new LinkedHashMap<>();
        details.put("executionId", executionId);
        details.put("tool", toolName);
        details.put("capability", capability);
        details.put("riskLevel", riskLevel.getName());
        if (input != null) {
            details.put("inputType", input.getClass().getSimpleName());
        }

        securityLogger.logMcpEvent(
                EventTypes.Mcp.MCP_TOOL_USED,
                toolName,
                "Tool execution started",
                details,
                true
        );
    }

    private void logToolComplete(String executionId, String toolName, String capability,
                                  long latencyMs, Object result, boolean success, String error) {
        Map<String, Object> details = new LinkedHashMap<>();
        details.put("executionId", executionId);
        details.put("tool", toolName);
        details.put("capability", capability);
        details.put("latencyMs", latencyMs);
        details.put("success", success);
        if (error != null) {
            details.put("error", error);
        }
        if (result != null) {
            details.put("resultType", result.getClass().getSimpleName());
        }

        securityLogger.logMcpEvent(
                EventTypes.Mcp.MCP_TOOL_USED,
                toolName,
                success ? "Tool execution completed" : "Tool execution failed",
                details,
                success
        );
    }

    private void logToolEvent(String toolName, String capability, RiskLevel riskLevel,
                               Object input, Object result, boolean success, String reason) {
        Map<String, Object> details = new LinkedHashMap<>();
        details.put("tool", toolName);
        details.put("capability", capability);
        details.put("riskLevel", riskLevel.getName());
        details.put("reason", reason);

        securityLogger.logAuthorizationEvent(
                success ? EventTypes.Authz.ACTION_EXECUTED : EventTypes.Authz.ACTION_DENIED,
                capability,
                toolName,
                success,
                details
        );
    }

    /**
     * Get all registered tool names.
     *
     * @return unmodifiable set of tool names
     */
    public Set<String> getRegisteredTools() {
        return Collections.unmodifiableSet(tools.keySet());
    }

    /**
     * Get tool info including risk level.
     *
     * @param toolName the tool name
     * @return map of tool information
     */
    public Map<String, Object> getToolInfo(String toolName) {
        ToolRegistration reg = tools.get(toolName);
        if (reg == null) {
            return Collections.emptyMap();
        }

        Map<String, Object> info = new LinkedHashMap<>();
        info.put("name", reg.name);
        info.put("description", reg.description);
        info.put("capability", reg.capability);
        info.put("riskLevel", reg.riskLevel.getName());
        return info;
    }

    /**
     * Get metrics for all tools.
     *
     * @return map of tool names to their metrics
     */
    public Map<String, Map<String, Object>> getAllMetrics() {
        Map<String, Map<String, Object>> result = new LinkedHashMap<>();
        for (Map.Entry<String, ToolMetrics> entry : metrics.entrySet()) {
            result.put(entry.getKey(), entry.getValue().toMap());
        }
        return result;
    }

    /**
     * Configure approval threshold.
     *
     * @param threshold the risk level threshold for approval
     */
    public void setApprovalThreshold(RiskLevel threshold) {
        this.approvalThreshold = threshold;
    }

    /**
     * Enable/disable blocking high-risk tools.
     *
     * @param block true to block high-risk tools
     */
    public void setBlockHighRisk(boolean block) {
        this.blockHighRisk = block;
    }

    // Internal classes

    private static class ToolRegistration {
        String name;
        String description;
        Function<Object, Object> tool;
        String capability;
        RiskLevel riskLevel;
    }

    private static class ToolMetrics {
        final AtomicLong successCount = new AtomicLong(0);
        final AtomicLong errorCount = new AtomicLong(0);
        final AtomicLong blockedCount = new AtomicLong(0);
        final AtomicLong totalLatencyMs = new AtomicLong(0);

        Map<String, Object> toMap() {
            Map<String, Object> map = new LinkedHashMap<>();
            long total = successCount.get() + errorCount.get();
            map.put("successCount", successCount.get());
            map.put("errorCount", errorCount.get());
            map.put("blockedCount", blockedCount.get());
            map.put("totalExecutions", total);
            if (total > 0) {
                map.put("avgLatencyMs", totalLatencyMs.get() / total);
                map.put("successRate", (double) successCount.get() / total);
            }
            return map;
        }
    }

    /**
     * Functional interface for tool actions.
     *
     * @param <T> the return type of the action
     */
    @FunctionalInterface
    public interface ToolAction<T> {
        /**
         * Executes the tool action.
         *
         * @return the result of the action
         * @throws Exception if an error occurs
         */
        T execute() throws Exception;
    }

    /**
     * Exception for unregistered tools.
     */
    public static class ToolNotRegisteredException extends RuntimeException {
        /**
         * Creates a new exception for an unregistered tool.
         *
         * @param toolName the tool name that was not found
         */
        public ToolNotRegisteredException(String toolName) {
            super("Tool not registered: " + toolName);
        }
    }

    /**
     * Exception for blocked high-risk tools.
     */
    public static class ToolBlockedException extends RuntimeException {
        private final String toolName;
        private final RiskLevel riskLevel;

        /**
         * Creates a new exception for a blocked tool.
         *
         * @param toolName the blocked tool name
         * @param riskLevel the risk level that caused the block
         */
        public ToolBlockedException(String toolName, RiskLevel riskLevel) {
            super(String.format("Tool '%s' blocked due to high risk (%s)", toolName, riskLevel.getName()));
            this.toolName = toolName;
            this.riskLevel = riskLevel;
        }

        /**
         * Gets the tool name.
         *
         * @return the tool name
         */
        public String getToolName() { return toolName; }

        /**
         * Gets the risk level.
         *
         * @return the risk level
         */
        public RiskLevel getRiskLevel() { return riskLevel; }
    }
}
