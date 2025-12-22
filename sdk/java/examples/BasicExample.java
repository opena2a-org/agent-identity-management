package org.opena2a.aim.examples;

import org.opena2a.aim.client.AIMClient;
import org.opena2a.aim.client.RiskLevel;
import org.opena2a.aim.client.VerificationResult;
import org.opena2a.aim.exceptions.ActionDeniedException;

import java.util.Arrays;
import java.util.Map;

/**
 * Basic AIM SDK usage example.
 *
 * <p>This example demonstrates:</p>
 * <ul>
 *   <li>One-line agent registration</li>
 *   <li>Capability verification</li>
 *   <li>Action execution with automatic verification</li>
 *   <li>Exception handling</li>
 * </ul>
 *
 * <p>Prerequisites:</p>
 * <ol>
 *   <li>Download SDK credentials from AIM dashboard</li>
 *   <li>Place credentials at ~/.aim/sdk_credentials.json</li>
 *   <li>Ensure AIM backend is running</li>
 * </ol>
 */
public class BasicExample {

    public static void main(String[] args) {
        System.out.println("=== AIM Java SDK Basic Example ===\n");

        try {
            // Step 1: One-line agent registration
            System.out.println("1. Registering agent...");
            AIMClient agent = AIMClient.secure(
                    "java-demo-agent",
                    Arrays.asList("db:read", "db:write", "api:call"),
                    null  // Auto-detect agent type
            );
            System.out.println("   Agent registered: " + agent.getAgentName());
            System.out.println("   Agent type: " + agent.getAgentType());

            // Step 2: Get agent details
            System.out.println("\n2. Fetching agent details...");
            Map<String, Object> details = agent.getAgentDetails();
            System.out.println("   Trust score: " + details.get("trustScore"));
            System.out.println("   Status: " + details.get("status"));

            // Step 3: Verify a capability before action
            System.out.println("\n3. Verifying capability...");
            VerificationResult result = agent.verifyCapability("db:read", "users_table");

            if (result.isVerified()) {
                System.out.println("   Verification successful!");
                System.out.println("   Verification ID: " + result.getVerificationId());
                System.out.println("   Status: " + result.getStatus());

                // Safe to perform the action
                String userData = fetchUserData("user-123");
                System.out.println("   Fetched user data: " + userData);
            } else {
                System.out.println("   Verification failed: " + result.getStatus());
            }

            // Step 4: Execute action with automatic verification
            System.out.println("\n4. Executing action with performAction...");
            try {
                String apiResult = agent.performAction("api:call", "weather_api", () -> {
                    // This code only runs if verification passes
                    return callWeatherApi("New York");
                });
                System.out.println("   API result: " + apiResult);
            } catch (ActionDeniedException e) {
                System.out.println("   Action denied: " + e.getMessage());
            }

            // Step 5: Try a high-risk action
            System.out.println("\n5. Attempting high-risk action...");
            try {
                agent.performAction("db:delete", "users_table", RiskLevel.HIGH, () -> {
                    System.out.println("   Deleting records... (simulated)");
                    return "Deleted 0 records";
                });
            } catch (ActionDeniedException e) {
                System.out.println("   High-risk action denied (expected): " + e.getMessage());
            }

            // Step 6: Request a new capability
            System.out.println("\n6. Requesting new capability...");
            Map<String, Object> capResult = agent.requestCapability(
                    "email:send",
                    "Need to send notification emails to users"
            );
            System.out.println("   Request status: " + capResult.get("status"));

            // Clean up
            agent.close();
            System.out.println("\n=== Example completed successfully ===");

        } catch (Exception e) {
            System.err.println("Error: " + e.getMessage());
            e.printStackTrace();
        }
    }

    // Simulated database fetch
    private static String fetchUserData(String userId) {
        return "{\"id\": \"" + userId + "\", \"name\": \"Jane Doe\", \"email\": \"jane@example.com\"}";
    }

    // Simulated API call
    private static String callWeatherApi(String city) {
        return "{\"city\": \"" + city + "\", \"temperature\": 72, \"conditions\": \"Sunny\"}";
    }
}
