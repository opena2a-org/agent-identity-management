package org.opena2a.aim.annotations;

import org.aspectj.lang.ProceedingJoinPoint;
import org.aspectj.lang.annotation.Around;
import org.aspectj.lang.annotation.Aspect;
import org.aspectj.lang.reflect.MethodSignature;
import org.opena2a.aim.client.AIMClient;
import org.opena2a.aim.client.RiskLevel;
import org.opena2a.aim.client.VerificationResult;
import org.opena2a.aim.exceptions.ActionDeniedException;
import org.opena2a.aim.security.EventTypes;
import org.opena2a.aim.security.RiskDetector;
import org.opena2a.aim.security.SecurityLogger;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.lang.reflect.Method;
import java.time.Duration;
import java.time.Instant;
import java.util.Arrays;
import java.util.LinkedHashMap;
import java.util.Map;
import java.util.UUID;

/**
 * AspectJ aspect for processing {@link SecureAction} annotations.
 *
 * <p>This aspect intercepts methods annotated with {@code @SecureAction}
 * and provides comprehensive security controls:</p>
 * <ul>
 *   <li>Capability verification before execution</li>
 *   <li>Automatic risk level detection from capability patterns</li>
 *   <li>Security event logging for SOC/SIEM integration</li>
 *   <li>Execution timing and metrics</li>
 *   <li>JIT approval workflow support</li>
 * </ul>
 *
 * <h2>Setup:</h2>
 * <ol>
 *   <li>Include AspectJ runtime and weaver dependencies</li>
 *   <li>Configure AspectJ weaving (compile-time or load-time)</li>
 *   <li>Set the AIMClient instance via {@link #setClient(AIMClient)}</li>
 * </ol>
 *
 * <h2>Spring Boot Integration:</h2>
 * <pre>{@code
 * @Configuration
 * @EnableAspectJAutoProxy
 * public class AIMConfig {
 *     @Bean
 *     public SecureActionAspect secureActionAspect(AIMClient client) {
 *         SecureActionAspect.setClient(client);
 *         return new SecureActionAspect();
 *     }
 * }
 * }</pre>
 */
@Aspect
public class SecureActionAspect {

    private static final Logger logger = LoggerFactory.getLogger(SecureActionAspect.class);

    private static AIMClient client;
    private static final RiskDetector riskDetector = RiskDetector.getInstance();
    private static final SecurityLogger securityLogger = SecurityLogger.getInstance();

    /**
     * Set the AIMClient to use for verification.
     * This must be called before any secured methods are invoked.
     *
     * @param aimClient The AIMClient instance
     */
    public static void setClient(AIMClient aimClient) {
        client = aimClient;
    }

    /**
     * Get the current AIMClient.
     *
     * @return The AIMClient instance, or null if not set
     */
    public static AIMClient getClient() {
        return client;
    }

    /**
     * Around advice for methods annotated with @SecureAction.
     */
    @Around("@annotation(org.opena2a.aim.annotations.SecureAction)")
    public Object verifyAction(ProceedingJoinPoint joinPoint) throws Throwable {
        // Get annotation and method info
        MethodSignature signature = (MethodSignature) joinPoint.getSignature();
        Method method = signature.getMethod();
        SecureAction annotation = method.getAnnotation(SecureAction.class);

        String capability = annotation.capability();
        String resource = annotation.resource();
        String executionId = UUID.randomUUID().toString();
        Instant startTime = Instant.now();

        // Determine risk level (auto-detect if AUTO)
        RiskLevel riskLevel = annotation.riskLevel();
        org.opena2a.aim.security.RiskLevel detectedRisk = null;

        if (riskLevel == RiskLevel.AUTO) {
            detectedRisk = riskDetector.detectRisk(capability);
            riskLevel = RiskLevel.fromSecurityRiskLevel(detectedRisk);
            logger.debug("Auto-detected risk level for '{}': {}", capability, riskLevel);
        }

        // Build description
        String description = annotation.description().isEmpty() ?
                String.format("Execute %s.%s", method.getDeclaringClass().getSimpleName(), method.getName()) :
                annotation.description();

        // Log pre-execution event
        Map<String, Object> preDetails = buildPreExecutionDetails(
                executionId, method, annotation, riskLevel, detectedRisk, joinPoint.getArgs());

        securityLogger.logAuthorizationEvent(
                EventTypes.Authz.ACTION_EXECUTED,
                capability,
                resource,
                true,
                preDetails
        );

        logger.debug("Verifying capability '{}' on resource '{}' for method {} (risk: {})",
                capability, resource, method.getName(), riskLevel);

        // Verify capability if client is configured
        if (client != null) {
            VerificationResult result = client.verifyCapability(
                    capability,
                    resource,
                    riskLevel,
                    null
            );

            if (!result.isVerified()) {
                long latencyMs = Duration.between(startTime, Instant.now()).toMillis();

                // Log denial
                Map<String, Object> denyDetails = new LinkedHashMap<>();
                denyDetails.put("executionId", executionId);
                denyDetails.put("status", result.getStatus());
                denyDetails.put("latencyMs", latencyMs);

                securityLogger.logAuthorizationEvent(
                        EventTypes.Authz.ACTION_DENIED,
                        capability,
                        resource,
                        false,
                        denyDetails
                );

                logger.warn("Action denied: {} on {} (status: {})",
                        capability, resource, result.getStatus());

                throw new ActionDeniedException(
                        "Action denied: " + capability + " on " + resource,
                        capability,
                        resource
                );
            }

            logger.debug("Action verified: {} on {} (id: {})",
                    capability, resource, result.getVerificationId());
        } else {
            logger.warn("AIMClient not configured. Proceeding without verification.");
        }

        // Execute the method
        Object returnValue;
        try {
            returnValue = joinPoint.proceed();
        } catch (Throwable t) {
            // Log error
            long latencyMs = Duration.between(startTime, Instant.now()).toMillis();
            Map<String, Object> errorDetails = new LinkedHashMap<>();
            errorDetails.put("executionId", executionId);
            errorDetails.put("latencyMs", latencyMs);
            errorDetails.put("errorType", t.getClass().getSimpleName());
            errorDetails.put("errorMessage", t.getMessage());

            securityLogger.logAuthorizationEvent(
                    EventTypes.Authz.ACTION_DENIED,
                    capability + ":error",
                    resource,
                    false,
                    errorDetails
            );

            throw t;
        }

        // Log successful execution
        long latencyMs = Duration.between(startTime, Instant.now()).toMillis();
        Map<String, Object> postDetails = buildPostExecutionDetails(
                executionId, latencyMs, annotation, returnValue);

        securityLogger.logAuthorizationEvent(
                EventTypes.Authz.ACTION_EXECUTED,
                capability + ":complete",
                resource,
                true,
                postDetails
        );

        logger.debug("Action completed: {} on {} ({}ms)", capability, resource, latencyMs);

        return returnValue;
    }

    /**
     * Build details map for pre-execution logging.
     */
    private Map<String, Object> buildPreExecutionDetails(
            String executionId, Method method, SecureAction annotation,
            RiskLevel riskLevel, org.opena2a.aim.security.RiskLevel detectedRisk,
            Object[] args) {

        Map<String, Object> details = new LinkedHashMap<>();
        details.put("executionId", executionId);
        details.put("method", method.getDeclaringClass().getSimpleName() + "." + method.getName());
        details.put("riskLevel", riskLevel.getValue());

        if (detectedRisk != null) {
            details.put("riskAutoDetected", true);
            details.put("detectedRisk", detectedRisk.getName());
        }

        if (annotation.jitAccess()) {
            details.put("jitRequired", true);
            details.put("jitTimeout", annotation.jitTimeout());
        }

        if (annotation.logArguments() && args != null && args.length > 0) {
            details.put("arguments", summarizeArgs(args));
        }

        return details;
    }

    /**
     * Build details map for post-execution logging.
     */
    private Map<String, Object> buildPostExecutionDetails(
            String executionId, long latencyMs,
            SecureAction annotation, Object returnValue) {

        Map<String, Object> details = new LinkedHashMap<>();
        details.put("executionId", executionId);
        details.put("latencyMs", latencyMs);
        details.put("success", true);

        if (annotation.logResult() && returnValue != null) {
            details.put("resultType", returnValue.getClass().getSimpleName());
            details.put("result", summarizeResult(returnValue));
        }

        return details;
    }

    /**
     * Summarize arguments for logging (avoiding sensitive data exposure).
     */
    private String summarizeArgs(Object[] args) {
        if (args == null || args.length == 0) return "[]";

        StringBuilder sb = new StringBuilder("[");
        for (int i = 0; i < args.length; i++) {
            if (i > 0) sb.append(", ");
            if (args[i] == null) {
                sb.append("null");
            } else {
                String type = args[i].getClass().getSimpleName();
                sb.append(type);
            }
        }
        sb.append("]");
        return sb.toString();
    }

    /**
     * Summarize return value for logging.
     */
    private String summarizeResult(Object result) {
        if (result == null) return "null";

        String type = result.getClass().getSimpleName();
        if (result instanceof String) {
            String s = (String) result;
            return s.length() > 50 ? s.substring(0, 50) + "..." : s;
        }
        return type;
    }
}
