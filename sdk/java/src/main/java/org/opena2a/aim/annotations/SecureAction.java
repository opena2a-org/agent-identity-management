package org.opena2a.aim.annotations;

import org.opena2a.aim.client.RiskLevel;

import java.lang.annotation.ElementType;
import java.lang.annotation.Retention;
import java.lang.annotation.RetentionPolicy;
import java.lang.annotation.Target;

/**
 * Annotation to secure a method with AIM capability verification.
 *
 * <p>Provides comprehensive security controls for method-level access:</p>
 * <ul>
 *   <li>Capability verification before execution</li>
 *   <li>Automatic risk level detection from capability patterns</li>
 *   <li>Just-In-Time (JIT) approval workflows</li>
 *   <li>Security event logging for audit trail</li>
 *   <li>Integration with RiskDetector for pattern-based risk assessment</li>
 * </ul>
 *
 * <h2>Basic Usage:</h2>
 * <pre>{@code
 * // Simple read operation with auto-detected risk
 * @SecureAction(capability = "db:read", resource = "users_table")
 * public User getUserById(String id) {
 *     return userRepository.findById(id);
 * }
 *
 * // Payment operation - AUTO detects as HIGH risk from "payment:" pattern
 * @SecureAction(capability = "payment:process", resource = "stripe")
 * public PaymentResult processPayment(PaymentRequest request) {
 *     return paymentService.process(request);
 * }
 *
 * // Explicit risk level override
 * @SecureAction(capability = "admin:delete", resource = "users", riskLevel = RiskLevel.CRITICAL)
 * public void deleteUser(String userId) {
 *     userService.delete(userId);
 * }
 * }</pre>
 *
 * <h2>JIT (Just-In-Time) Access:</h2>
 * <pre>{@code
 * // Requires approval before execution
 * @SecureAction(
 *     capability = "data:export",
 *     resource = "customer_pii",
 *     jitAccess = true,
 *     jitTimeout = 300  // 5 minute approval window
 * )
 * public File exportCustomerData() {
 *     return dataExporter.export();
 * }
 * }</pre>
 *
 * <h2>Risk Auto-Detection Patterns:</h2>
 * <ul>
 *   <li>CRITICAL: admin:*, root:*, credential:*, system:delete</li>
 *   <li>HIGH: payment:*, pii:*, phi:*, user:delete, data:export</li>
 *   <li>MEDIUM: db:read, file:write, config:update</li>
 *   <li>LOW: api:call, cache:read, status:check</li>
 * </ul>
 *
 * <p><b>Note:</b> This annotation requires AspectJ or Spring AOP to be enabled.
 * For non-AOP usage, use {@link org.opena2a.aim.client.AIMClient#performAction}.</p>
 *
 * @see org.opena2a.aim.security.RiskDetector
 * @see org.opena2a.aim.client.RiskLevel
 */
@Target(ElementType.METHOD)
@Retention(RetentionPolicy.RUNTIME)
public @interface SecureAction {

    /**
     * The capability required to execute this action.
     *
     * <p>Format: "domain:operation" or "domain:operation:qualifier"</p>
     * <p>Examples: "db:read", "api:call", "file:write", "payment:process:stripe"</p>
     *
     * <p>This capability string is used for:</p>
     * <ul>
     *   <li>Verification against agent's registered capabilities</li>
     *   <li>Auto-detection of risk level (when riskLevel = AUTO)</li>
     *   <li>Security event logging</li>
     * </ul>
     */
    String capability();

    /**
     * The resource being accessed.
     *
     * <p>Examples: "users_table", "payment_api", "/etc/config", "s3://bucket"</p>
     *
     * <p>Used for:</p>
     * <ul>
     *   <li>Fine-grained access control</li>
     *   <li>Audit logging</li>
     *   <li>Resource-specific policies</li>
     * </ul>
     */
    String resource();

    /**
     * Risk level of the action.
     *
     * <p>Default is AUTO, which uses the RiskDetector to analyze
     * the capability pattern and determine appropriate risk level.</p>
     *
     * <p>Set explicitly to override auto-detection:</p>
     * <ul>
     *   <li>LOW - Auto-approved, minimal logging</li>
     *   <li>MEDIUM - Standard approval workflow</li>
     *   <li>HIGH - Requires manager approval, detailed logging</li>
     *   <li>CRITICAL - Requires executive approval, full audit trail</li>
     * </ul>
     */
    RiskLevel riskLevel() default RiskLevel.AUTO;

    /**
     * Whether this action requires Just-In-Time (JIT) approval.
     *
     * <p>When enabled, the action will block until approval is received
     * from an authorized approver. Useful for high-risk operations that
     * need real-time human oversight.</p>
     */
    boolean jitAccess() default false;

    /**
     * Timeout in seconds for JIT approval.
     *
     * <p>Only applicable if jitAccess is true. The action will fail
     * with a timeout exception if approval is not received within
     * this window.</p>
     *
     * <p>Default: 60 seconds</p>
     */
    int jitTimeout() default 60;

    /**
     * Whether to log the method arguments.
     *
     * <p>Set to false for methods that receive sensitive data (PII, credentials)
     * that should not appear in audit logs.</p>
     *
     * <p>Default: false (arguments not logged)</p>
     */
    boolean logArguments() default false;

    /**
     * Whether to log the method return value.
     *
     * <p>Set to false for methods that return sensitive data.</p>
     *
     * <p>Default: false (return value not logged)</p>
     */
    boolean logResult() default false;

    /**
     * Custom description for audit logs.
     *
     * <p>If not specified, a default description is generated from
     * the method name and capability.</p>
     */
    String description() default "";
}
