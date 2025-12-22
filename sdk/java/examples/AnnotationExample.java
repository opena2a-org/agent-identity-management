package org.opena2a.aim.examples;

import org.opena2a.aim.annotations.SecureAction;
import org.opena2a.aim.annotations.SecureActionAspect;
import org.opena2a.aim.client.AIMClient;
import org.opena2a.aim.client.RiskLevel;
import org.opena2a.aim.exceptions.ActionDeniedException;

import java.math.BigDecimal;
import java.util.Arrays;

/**
 * Example demonstrating @SecureAction annotation usage.
 *
 * <p>This example shows how to use declarative security with AspectJ.</p>
 *
 * <p>Prerequisites:</p>
 * <ol>
 *   <li>Enable AspectJ weaving (compile-time or load-time)</li>
 *   <li>Configure AIMClient with SecureActionAspect</li>
 * </ol>
 *
 * <p>For Spring Boot, add @EnableAspectJAutoProxy to your config.</p>
 */
public class AnnotationExample {

    public static void main(String[] args) {
        System.out.println("=== AIM @SecureAction Annotation Example ===\n");

        try {
            // Step 1: Register agent
            System.out.println("1. Registering agent...");
            AIMClient agent = AIMClient.secure(
                    "annotation-demo-agent",
                    Arrays.asList("db:read", "db:write", "payment:process", "db:delete"),
                    null
            );
            System.out.println("   Agent registered: " + agent.getAgentName());

            // Step 2: Configure the aspect with the AIM client
            System.out.println("\n2. Configuring SecureActionAspect...");
            SecureActionAspect.setClient(agent);
            System.out.println("   Aspect configured!");

            // Step 3: Create service instance
            UserService userService = new UserService();
            PaymentService paymentService = new PaymentService();

            // Step 4: Call secured methods
            System.out.println("\n3. Calling secured methods...\n");

            // Low-risk read operation
            System.out.println("--- Reading user (low risk) ---");
            try {
                User user = userService.getUserById("user-123");
                System.out.println("User: " + user);
            } catch (ActionDeniedException e) {
                System.out.println("Read denied: " + e.getMessage());
            }

            // Medium-risk write operation
            System.out.println("\n--- Updating user (medium risk) ---");
            try {
                userService.updateUserEmail("user-123", "new@example.com");
                System.out.println("User email updated!");
            } catch (ActionDeniedException e) {
                System.out.println("Update denied: " + e.getMessage());
            }

            // High-risk payment operation
            System.out.println("\n--- Processing payment (high risk) ---");
            try {
                PaymentResult result = paymentService.processPayment(
                        new PaymentRequest("card_123", new BigDecimal("99.99"))
                );
                System.out.println("Payment result: " + result);
            } catch (ActionDeniedException e) {
                System.out.println("Payment denied: " + e.getMessage());
            }

            // Critical operation with JIT access
            System.out.println("\n--- Deleting user (critical, requires approval) ---");
            try {
                userService.deleteUser("user-123");
                System.out.println("User deleted!");
            } catch (ActionDeniedException e) {
                System.out.println("Delete denied (expected for critical action): " + e.getMessage());
            }

            // Clean up
            agent.close();
            System.out.println("\n=== Example completed ===");

        } catch (Exception e) {
            System.err.println("Error: " + e.getMessage());
            e.printStackTrace();
        }
    }

    // ===========================================
    // Service classes with @SecureAction methods
    // ===========================================

    /**
     * User service with secured methods.
     */
    public static class UserService {

        @SecureAction(capability = "db:read", resource = "users_table")
        public User getUserById(String id) {
            System.out.println("   [UserService] Fetching user: " + id);
            return new User(id, "Jane Doe", "jane@example.com");
        }

        @SecureAction(
                capability = "db:write",
                resource = "users_table",
                riskLevel = RiskLevel.MEDIUM
        )
        public void updateUserEmail(String userId, String newEmail) {
            System.out.println("   [UserService] Updating email for " + userId + " to " + newEmail);
            // Database update logic here
        }

        @SecureAction(
                capability = "db:delete",
                resource = "users_table",
                riskLevel = RiskLevel.CRITICAL,
                jitAccess = true,
                jitTimeout = 300
        )
        public void deleteUser(String userId) {
            System.out.println("   [UserService] Deleting user: " + userId);
            // This requires admin approval before execution
        }
    }

    /**
     * Payment service with secured methods.
     */
    public static class PaymentService {

        @SecureAction(
                capability = "payment:process",
                resource = "stripe_api",
                riskLevel = RiskLevel.HIGH
        )
        public PaymentResult processPayment(PaymentRequest request) {
            System.out.println("   [PaymentService] Processing payment: $" + request.amount);
            return new PaymentResult("txn_" + System.currentTimeMillis(), "completed");
        }

        @SecureAction(
                capability = "payment:refund",
                resource = "stripe_api",
                riskLevel = RiskLevel.CRITICAL,
                jitAccess = true,
                jitTimeout = 600
        )
        public PaymentResult processRefund(String transactionId, BigDecimal amount) {
            System.out.println("   [PaymentService] Processing refund: $" + amount);
            return new PaymentResult("ref_" + System.currentTimeMillis(), "refunded");
        }
    }

    // ===========================================
    // Model classes
    // ===========================================

    public record User(String id, String name, String email) {
        @Override
        public String toString() {
            return "User{id='" + id + "', name='" + name + "', email='" + email + "'}";
        }
    }

    public record PaymentRequest(String cardId, BigDecimal amount) {}

    public record PaymentResult(String transactionId, String status) {
        @Override
        public String toString() {
            return "PaymentResult{txnId='" + transactionId + "', status='" + status + "'}";
        }
    }
}
