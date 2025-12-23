package org.opena2a.aim.integration.langchain4j;

import org.opena2a.aim.client.AIMClient;
import org.opena2a.aim.security.EventTypes;
import org.opena2a.aim.security.SecurityLogger;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.time.Duration;
import java.time.Instant;
import java.util.*;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.concurrent.atomic.AtomicLong;

/**
 * Agent callback handler for LangChain4j with AIM security integration.
 *
 * <p>Provides comprehensive observability for LangChain4j agent executions:</p>
 * <ul>
 *   <li>Agent lifecycle tracking (start, step, complete)</li>
 *   <li>Thought/action/observation logging</li>
 *   <li>Tool call monitoring</li>
 *   <li>Iteration tracking and limits</li>
 *   <li>Error and timeout handling</li>
 * </ul>
 *
 * <h2>Usage:</h2>
 * <pre>{@code
 * AIMAgentCallback callback = new AIMAgentCallback(aimClient);
 *
 * // Start agent execution
 * String executionId = callback.onAgentStart("process-claim", userInput);
 *
 * // Track each step
 * callback.onAgentThought(executionId, "I need to look up the claim details");
 * callback.onAgentAction(executionId, "searchClaims", claimId);
 * callback.onAgentObservation(executionId, claimDetails);
 *
 * // Complete
 * callback.onAgentFinish(executionId, finalAnswer);
 * }</pre>
 *
 * <h2>ReAct Pattern Integration:</h2>
 * <pre>{@code
 * // Integrates with ReAct agents
 * callback.onThoughtActionObservation(executionId,
 *     "I should check the database for claims",
 *     "searchDatabase",
 *     "Found 3 claims matching criteria");
 * }</pre>
 */
public class AIMAgentCallback {

    private static final Logger log = LoggerFactory.getLogger(AIMAgentCallback.class);

    private final AIMClient client;
    private final SecurityLogger securityLogger;

    // Active executions
    private final Map<String, AgentExecution> executions = new ConcurrentHashMap<>();

    // Global metrics
    private final AtomicLong totalExecutions = new AtomicLong(0);
    private final AtomicLong successfulExecutions = new AtomicLong(0);
    private final AtomicLong failedExecutions = new AtomicLong(0);
    private final AtomicLong totalSteps = new AtomicLong(0);
    private final AtomicLong totalToolCalls = new AtomicLong(0);

    // Configuration
    private int maxIterations = 10;
    private long maxDurationMs = 60000; // 1 minute

    public AIMAgentCallback(AIMClient client) {
        this.client = client;
        this.securityLogger = SecurityLogger.getInstance();
    }

    /**
     * Called when an agent starts execution.
     *
     * @param agentName Name of the agent
     * @param input Initial input to the agent
     * @return Execution ID for tracking
     */
    public String onAgentStart(String agentName, String input) {
        String executionId = UUID.randomUUID().toString();

        AgentExecution execution = new AgentExecution();
        execution.executionId = executionId;
        execution.agentName = agentName;
        execution.startTime = Instant.now();
        execution.input = input;
        executions.put(executionId, execution);

        totalExecutions.incrementAndGet();

        Map<String, Object> details = new LinkedHashMap<>();
        details.put("executionId", executionId);
        details.put("agent", agentName);
        details.put("inputLength", input != null ? input.length() : 0);

        securityLogger.logAgentEvent(
                EventTypes.Agent.AGENT_LOADED,
                agentName,
                "Agent execution started",
                details,
                true
        );

        log.info("Agent started: executionId={}, agent={}", executionId, agentName);
        return executionId;
    }

    /**
     * Called when the agent produces a thought.
     */
    public void onAgentThought(String executionId, String thought) {
        AgentExecution execution = executions.get(executionId);
        if (execution == null) return;

        execution.stepCount.incrementAndGet();
        totalSteps.incrementAndGet();

        AgentStep step = new AgentStep();
        step.type = "thought";
        step.content = thought;
        step.timestamp = Instant.now();
        execution.steps.add(step);

        log.debug("Agent thought: executionId={}, thought={}",
                executionId, truncate(thought, 100));
    }

    /**
     * Called when the agent takes an action (calls a tool).
     */
    public void onAgentAction(String executionId, String action, Object input) {
        AgentExecution execution = executions.get(executionId);
        if (execution == null) return;

        execution.toolCalls.incrementAndGet();
        totalToolCalls.incrementAndGet();

        AgentStep step = new AgentStep();
        step.type = "action";
        step.action = action;
        step.actionInput = input != null ? input.toString() : null;
        step.timestamp = Instant.now();
        execution.steps.add(step);

        Map<String, Object> details = new LinkedHashMap<>();
        details.put("executionId", executionId);
        details.put("action", action);
        details.put("stepNumber", execution.stepCount.get());

        securityLogger.logMcpEvent(
                EventTypes.Mcp.MCP_TOOL_USED,
                action,
                "Agent action taken",
                details,
                true
        );

        log.debug("Agent action: executionId={}, action={}", executionId, action);
    }

    /**
     * Called when the agent receives an observation (tool result).
     */
    public void onAgentObservation(String executionId, String observation) {
        AgentExecution execution = executions.get(executionId);
        if (execution == null) return;

        AgentStep step = new AgentStep();
        step.type = "observation";
        step.content = observation;
        step.timestamp = Instant.now();
        execution.steps.add(step);

        log.debug("Agent observation: executionId={}, observation={}",
                executionId, truncate(observation, 100));
    }

    /**
     * Combined thought-action-observation for ReAct pattern.
     */
    public void onThoughtActionObservation(String executionId, String thought,
                                            String action, String observation) {
        onAgentThought(executionId, thought);
        onAgentAction(executionId, action, null);
        onAgentObservation(executionId, observation);
    }

    /**
     * Called when the agent finishes successfully.
     */
    public void onAgentFinish(String executionId, String output) {
        AgentExecution execution = executions.remove(executionId);
        if (execution == null) return;

        execution.endTime = Instant.now();
        execution.output = output;
        execution.success = true;
        successfulExecutions.incrementAndGet();

        long durationMs = Duration.between(execution.startTime, execution.endTime).toMillis();

        Map<String, Object> details = new LinkedHashMap<>();
        details.put("executionId", executionId);
        details.put("agent", execution.agentName);
        details.put("durationMs", durationMs);
        details.put("steps", execution.stepCount.get());
        details.put("toolCalls", execution.toolCalls.get());
        details.put("outputLength", output != null ? output.length() : 0);

        securityLogger.logAgentEvent(
                EventTypes.Agent.AGENT_UPDATED,
                execution.agentName,
                "Agent execution completed",
                details,
                true
        );

        log.info("Agent finished: executionId={}, duration={}ms, steps={}, tools={}",
                executionId, durationMs, execution.stepCount.get(), execution.toolCalls.get());
    }

    /**
     * Called when the agent fails.
     */
    public void onAgentError(String executionId, Throwable error) {
        AgentExecution execution = executions.remove(executionId);
        if (execution == null) return;

        execution.endTime = Instant.now();
        execution.error = error.getMessage();
        execution.success = false;
        failedExecutions.incrementAndGet();

        long durationMs = Duration.between(execution.startTime, execution.endTime).toMillis();

        Map<String, Object> details = new LinkedHashMap<>();
        details.put("executionId", executionId);
        details.put("agent", execution.agentName);
        details.put("durationMs", durationMs);
        details.put("steps", execution.stepCount.get());
        details.put("errorType", error.getClass().getSimpleName());
        details.put("errorMessage", error.getMessage());

        securityLogger.logAgentEvent(
                EventTypes.Agent.AGENT_SUSPENDED,
                execution.agentName,
                "Agent execution failed",
                details,
                false
        );

        log.error("Agent failed: executionId={}, error={}", executionId, error.getMessage());
    }

    /**
     * Check if execution should continue (iteration/time limits).
     */
    public boolean shouldContinue(String executionId) {
        AgentExecution execution = executions.get(executionId);
        if (execution == null) return false;

        // Check iteration limit
        if (execution.stepCount.get() >= maxIterations) {
            log.warn("Agent reached max iterations: executionId={}, iterations={}",
                    executionId, execution.stepCount.get());
            return false;
        }

        // Check time limit
        long durationMs = Duration.between(execution.startTime, Instant.now()).toMillis();
        if (durationMs >= maxDurationMs) {
            log.warn("Agent reached max duration: executionId={}, duration={}ms",
                    executionId, durationMs);
            return false;
        }

        return true;
    }

    /**
     * Get execution status.
     */
    public Map<String, Object> getExecutionStatus(String executionId) {
        AgentExecution execution = executions.get(executionId);
        if (execution == null) {
            return Collections.singletonMap("status", "not_found");
        }

        Map<String, Object> status = new LinkedHashMap<>();
        status.put("executionId", executionId);
        status.put("agent", execution.agentName);
        status.put("status", "running");
        status.put("steps", execution.stepCount.get());
        status.put("toolCalls", execution.toolCalls.get());
        status.put("durationMs", Duration.between(execution.startTime, Instant.now()).toMillis());
        return status;
    }

    /**
     * Get global metrics.
     */
    public Map<String, Object> getMetrics() {
        Map<String, Object> metrics = new LinkedHashMap<>();
        metrics.put("totalExecutions", totalExecutions.get());
        metrics.put("successfulExecutions", successfulExecutions.get());
        metrics.put("failedExecutions", failedExecutions.get());
        metrics.put("activeExecutions", executions.size());
        metrics.put("totalSteps", totalSteps.get());
        metrics.put("totalToolCalls", totalToolCalls.get());

        if (totalExecutions.get() > 0) {
            metrics.put("successRate",
                    (double) successfulExecutions.get() / totalExecutions.get());
            metrics.put("avgStepsPerExecution",
                    (double) totalSteps.get() / totalExecutions.get());
            metrics.put("avgToolCallsPerExecution",
                    (double) totalToolCalls.get() / totalExecutions.get());
        }

        return metrics;
    }

    /**
     * Configure max iterations.
     */
    public void setMaxIterations(int maxIterations) {
        this.maxIterations = maxIterations;
    }

    /**
     * Configure max duration.
     */
    public void setMaxDurationMs(long maxDurationMs) {
        this.maxDurationMs = maxDurationMs;
    }

    private String truncate(String s, int maxLen) {
        if (s == null) return null;
        return s.length() > maxLen ? s.substring(0, maxLen) + "..." : s;
    }

    // Internal classes

    private static class AgentExecution {
        String executionId;
        String agentName;
        Instant startTime;
        Instant endTime;
        String input;
        String output;
        String error;
        boolean success;
        final AtomicInteger stepCount = new AtomicInteger(0);
        final AtomicInteger toolCalls = new AtomicInteger(0);
        final List<AgentStep> steps = Collections.synchronizedList(new ArrayList<>());
    }

    private static class AgentStep {
        String type; // thought, action, observation
        String content;
        String action;
        String actionInput;
        Instant timestamp;
    }
}
