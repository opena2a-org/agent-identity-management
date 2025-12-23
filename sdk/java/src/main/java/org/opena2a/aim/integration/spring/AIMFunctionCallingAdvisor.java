package org.opena2a.aim.integration.spring;

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
 * Function calling advisor for Spring AI with AIM integration.
 *
 * <p>Provides security controls and logging for AI function/tool calls:</p>
 * <ul>
 *   <li>Pre-execution risk assessment</li>
 *   <li>Capability verification</li>
 *   <li>Execution logging for audit trail</li>
 *   <li>Post-execution result tracking</li>
 * </ul>
 *
 * <h2>Usage:</h2>
 * <pre>{@code
 * AIMFunctionCallingAdvisor advisor = new AIMFunctionCallingAdvisor(aimClient);
 *
 * // Register a function with security wrapper
 * advisor.register("processPayment", this::processPayment, "payment:process");
 *
 * // Execute through advisor
 * Object result = advisor.execute("processPayment", args);
 * }</pre>
 *
 * <h2>Risk-Based Execution:</h2>
 * <pre>{@code
 * // Configure approval requirements
 * advisor.requireApprovalFor(RiskLevel.HIGH);
 *
 * // Function will be blocked if HIGH risk and no approval
 * try {
 *     advisor.execute("deleteUser", args);
 * } catch (ApprovalRequiredException e) {
 *     // Handle approval workflow
 * }
 * }</pre>
 */
public class AIMFunctionCallingAdvisor {

    private static final Logger log = LoggerFactory.getLogger(AIMFunctionCallingAdvisor.class);

    private final AIMClient client;
    private final RiskDetector riskDetector;
    private final SecurityLogger securityLogger;

    // Registered functions
    private final Map<String, FunctionRegistration<?>> functions = new ConcurrentHashMap<>();

    // Metrics per function
    private final Map<String, FunctionMetrics> metrics = new ConcurrentHashMap<>();

    // Configuration
    private RiskLevel approvalThreshold = RiskLevel.CRITICAL;
    private boolean blockWithoutApproval = false;
    private Set<String> approvedExecutions = ConcurrentHashMap.newKeySet();

    public AIMFunctionCallingAdvisor(AIMClient client) {
        this.client = client;
        this.riskDetector = RiskDetector.getInstance();
        this.securityLogger = SecurityLogger.getInstance();
    }

    /**
     * Register a function with security wrapper.
     *
     * @param name Function name
     * @param function The function implementation
     * @param capability The capability string for risk assessment
     * @param <I> Input type
     * @param <O> Output type
     */
    public <I, O> void register(String name, Function<I, O> function, String capability) {
        FunctionRegistration<O> registration = new FunctionRegistration<>();
        registration.name = name;
        registration.function = (Function<Object, O>) function;
        registration.capability = capability;
        registration.riskLevel = riskDetector.detectRisk(capability);

        functions.put(name, registration);
        metrics.put(name, new FunctionMetrics());

        log.info("Registered function '{}' with capability '{}' (risk: {})",
                name, capability, registration.riskLevel);
    }

    /**
     * Execute a registered function with security checks.
     *
     * @param functionName The function name
     * @param input The input to the function
     * @return The function result
     * @throws FunctionNotRegisteredException if function not found
     * @throws ApprovalRequiredException if approval required but not provided
     */
    @SuppressWarnings("unchecked")
    public <O> O execute(String functionName, Object input) {
        FunctionRegistration<O> registration = (FunctionRegistration<O>) functions.get(functionName);
        if (registration == null) {
            throw new FunctionNotRegisteredException(functionName);
        }

        FunctionMetrics funcMetrics = metrics.get(functionName);

        // Pre-execution: risk check
        if (registration.riskLevel.isHigherThan(approvalThreshold) ||
                registration.riskLevel == approvalThreshold) {
            if (blockWithoutApproval && !isApproved(functionName, input)) {
                funcMetrics.blockedCount.incrementAndGet();

                securityLogger.logAuthorizationEvent(
                        EventTypes.Authz.ACTION_DENIED,
                        registration.capability,
                        functionName,
                        false,
                        Map.of("reason", "approval_required", "riskLevel", registration.riskLevel.getName())
                );

                throw new ApprovalRequiredException(functionName, registration.riskLevel);
            }
        }

        // Pre-execution logging
        String executionId = UUID.randomUUID().toString();
        Map<String, Object> preDetails = new LinkedHashMap<>();
        preDetails.put("executionId", executionId);
        preDetails.put("function", functionName);
        preDetails.put("capability", registration.capability);
        preDetails.put("riskLevel", registration.riskLevel.getName());

        securityLogger.logAuthorizationEvent(
                EventTypes.Authz.ACTION_EXECUTED,
                registration.capability,
                functionName,
                true,
                preDetails
        );

        // Execute
        long startTime = System.currentTimeMillis();
        try {
            O result = registration.function.apply(input);

            // Post-execution: success
            long latencyMs = System.currentTimeMillis() - startTime;
            funcMetrics.successCount.incrementAndGet();
            funcMetrics.totalLatencyMs.addAndGet(latencyMs);

            Map<String, Object> postDetails = new LinkedHashMap<>();
            postDetails.put("executionId", executionId);
            postDetails.put("latencyMs", latencyMs);
            postDetails.put("success", true);

            securityLogger.logAuthorizationEvent(
                    EventTypes.Authz.ACTION_EXECUTED,
                    registration.capability + ":complete",
                    functionName,
                    true,
                    postDetails
            );

            return result;

        } catch (Exception e) {
            // Post-execution: failure
            long latencyMs = System.currentTimeMillis() - startTime;
            funcMetrics.errorCount.incrementAndGet();

            Map<String, Object> errorDetails = new LinkedHashMap<>();
            errorDetails.put("executionId", executionId);
            errorDetails.put("latencyMs", latencyMs);
            errorDetails.put("errorType", e.getClass().getSimpleName());
            errorDetails.put("errorMessage", e.getMessage());

            securityLogger.logAuthorizationEvent(
                    EventTypes.Authz.ACTION_DENIED,
                    registration.capability + ":error",
                    functionName,
                    false,
                    errorDetails
            );

            throw e instanceof RuntimeException ? (RuntimeException) e : new RuntimeException(e);
        }
    }

    /**
     * Pre-approve an execution for high-risk functions.
     *
     * @param functionName Function name
     * @param approvalToken Approval token from workflow
     */
    public void approve(String functionName, String approvalToken) {
        approvedExecutions.add(functionName + ":" + approvalToken);

        securityLogger.logAuthorizationEvent(
                EventTypes.Authz.JIT_APPROVED,
                "function:approve",
                functionName,
                true,
                Map.of("approvalToken", approvalToken)
        );
    }

    /**
     * Check if a function execution is approved.
     */
    private boolean isApproved(String functionName, Object input) {
        // Simple check - in production this would check approval tokens, JIT workflows, etc.
        return approvedExecutions.stream()
                .anyMatch(approval -> approval.startsWith(functionName + ":"));
    }

    /**
     * Configure the risk level that requires approval.
     */
    public void requireApprovalFor(RiskLevel threshold) {
        this.approvalThreshold = threshold;
    }

    /**
     * Enable/disable blocking without approval.
     */
    public void setBlockWithoutApproval(boolean block) {
        this.blockWithoutApproval = block;
    }

    /**
     * Get all registered function names.
     */
    public Set<String> getRegisteredFunctions() {
        return Collections.unmodifiableSet(functions.keySet());
    }

    /**
     * Get risk level for a function.
     */
    public RiskLevel getRiskLevel(String functionName) {
        FunctionRegistration<?> reg = functions.get(functionName);
        return reg != null ? reg.riskLevel : RiskLevel.LOW;
    }

    /**
     * Get metrics for all functions.
     */
    public Map<String, Map<String, Object>> getAllMetrics() {
        Map<String, Map<String, Object>> result = new LinkedHashMap<>();
        for (Map.Entry<String, FunctionMetrics> entry : metrics.entrySet()) {
            result.put(entry.getKey(), entry.getValue().toMap());
        }
        return result;
    }

    /**
     * Get metrics for a specific function.
     */
    public Map<String, Object> getMetrics(String functionName) {
        FunctionMetrics m = metrics.get(functionName);
        return m != null ? m.toMap() : Collections.emptyMap();
    }

    // Internal classes

    private static class FunctionRegistration<O> {
        String name;
        Function<Object, O> function;
        String capability;
        RiskLevel riskLevel;
    }

    private static class FunctionMetrics {
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
            map.put("totalCalls", total);
            if (total > 0) {
                map.put("avgLatencyMs", totalLatencyMs.get() / total);
                map.put("successRate", (double) successCount.get() / total);
            }
            return map;
        }
    }

    /**
     * Exception thrown when a function is not registered.
     */
    public static class FunctionNotRegisteredException extends RuntimeException {
        public FunctionNotRegisteredException(String functionName) {
            super("Function not registered: " + functionName);
        }
    }

    /**
     * Exception thrown when approval is required but not provided.
     */
    public static class ApprovalRequiredException extends RuntimeException {
        private final String functionName;
        private final RiskLevel riskLevel;

        public ApprovalRequiredException(String functionName, RiskLevel riskLevel) {
            super(String.format("Approval required for function '%s' (risk level: %s)",
                    functionName, riskLevel.getName()));
            this.functionName = functionName;
            this.riskLevel = riskLevel;
        }

        public String getFunctionName() { return functionName; }
        public RiskLevel getRiskLevel() { return riskLevel; }
    }
}
