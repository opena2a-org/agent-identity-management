package org.opena2a.aim.annotations;

import org.aspectj.lang.ProceedingJoinPoint;
import org.aspectj.lang.annotation.Around;
import org.aspectj.lang.annotation.Aspect;
import org.aspectj.lang.reflect.MethodSignature;
import org.opena2a.aim.client.AIMClient;
import org.opena2a.aim.client.VerificationResult;
import org.opena2a.aim.exceptions.ActionDeniedException;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.lang.reflect.Method;

/**
 * AspectJ aspect for processing {@link SecureAction} annotations.
 *
 * <p>This aspect intercepts methods annotated with {@code @SecureAction}
 * and verifies capabilities before allowing execution.</p>
 *
 * <p>To use this aspect, you need to:</p>
 * <ol>
 *   <li>Include AspectJ runtime and weaver dependencies</li>
 *   <li>Configure AspectJ weaving (compile-time or load-time)</li>
 *   <li>Set the AIMClient instance via {@link #setClient(AIMClient)}</li>
 * </ol>
 *
 * <p>For Spring Boot applications, this aspect will be auto-detected
 * if you enable {@code @EnableAspectJAutoProxy}.</p>
 */
@Aspect
public class SecureActionAspect {

    private static final Logger logger = LoggerFactory.getLogger(SecureActionAspect.class);

    private static AIMClient client;

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
        if (client == null) {
            logger.warn("AIMClient not configured. Skipping verification.");
            return joinPoint.proceed();
        }

        // Get annotation
        MethodSignature signature = (MethodSignature) joinPoint.getSignature();
        Method method = signature.getMethod();
        SecureAction annotation = method.getAnnotation(SecureAction.class);

        String capability = annotation.capability();
        String resource = annotation.resource();

        logger.debug("Verifying capability '{}' on resource '{}' for method {}",
                capability, resource, method.getName());

        // Verify capability
        VerificationResult result = client.verifyCapability(
                capability,
                resource,
                annotation.riskLevel(),
                null
        );

        if (!result.isVerified()) {
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

        // Proceed with the method
        return joinPoint.proceed();
    }
}
