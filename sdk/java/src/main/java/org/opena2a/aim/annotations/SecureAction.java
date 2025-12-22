package org.opena2a.aim.annotations;

import org.opena2a.aim.client.RiskLevel;

import java.lang.annotation.ElementType;
import java.lang.annotation.Retention;
import java.lang.annotation.RetentionPolicy;
import java.lang.annotation.Target;

/**
 * Annotation to secure a method with AIM capability verification.
 *
 * <p>Example usage:</p>
 * <pre>{@code
 * @SecureAction(capability = "db:read", resource = "users_table")
 * public User getUserById(String id) {
 *     return userRepository.findById(id);
 * }
 *
 * @SecureAction(capability = "payment:process", resource = "stripe", riskLevel = RiskLevel.HIGH)
 * public PaymentResult processPayment(PaymentRequest request) {
 *     return paymentService.process(request);
 * }
 * }</pre>
 *
 * <p>Note: This annotation requires AspectJ or Spring AOP to be enabled.
 * For non-AOP usage, use {@link org.opena2a.aim.client.AIMClient#performAction}.</p>
 */
@Target(ElementType.METHOD)
@Retention(RetentionPolicy.RUNTIME)
public @interface SecureAction {

    /**
     * The capability required to execute this action.
     * Examples: "db:read", "api:call", "file:write"
     */
    String capability();

    /**
     * The resource being accessed.
     * Examples: "users_table", "payment_api", "/etc/config"
     */
    String resource();

    /**
     * Risk level of the action.
     * Default is MEDIUM.
     */
    RiskLevel riskLevel() default RiskLevel.MEDIUM;

    /**
     * Whether this action requires Just-In-Time (JIT) approval.
     * Default is false.
     */
    boolean jitAccess() default false;

    /**
     * Timeout in seconds for JIT approval.
     * Only applicable if jitAccess is true.
     */
    int jitTimeout() default 60;
}
