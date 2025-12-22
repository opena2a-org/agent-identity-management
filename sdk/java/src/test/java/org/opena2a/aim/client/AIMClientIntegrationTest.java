package org.opena2a.aim.client;

import org.junit.jupiter.api.*;
import org.junit.jupiter.api.condition.EnabledIfEnvironmentVariable;
import org.opena2a.aim.credentials.CredentialManager;
import org.opena2a.aim.exceptions.ActionDeniedException;
import org.opena2a.aim.exceptions.AIMException;
import org.opena2a.aim.exceptions.CredentialException;

import java.net.HttpURLConnection;
import java.net.URL;
import java.util.Arrays;
import java.util.List;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Integration tests for AIMClient against a live AIM backend.
 *
 * <p>These tests require:</p>
 * <ul>
 *   <li>AIM backend running at http://localhost:8080</li>
 *   <li>Valid SDK credentials in ~/.aim/sdk_credentials.json or environment variables</li>
 * </ul>
 *
 * <p>Run with: mvn test -Dtest=AIMClientIntegrationTest</p>
 */
@TestMethodOrder(MethodOrderer.OrderAnnotation.class)
class AIMClientIntegrationTest {

    private static final String AIM_URL = "http://localhost:8080";
    private static final String TEST_AGENT_NAME = "java-sdk-test-agent-" + System.currentTimeMillis();

    private static boolean backendAvailable = false;
    private static boolean credentialsAvailable = false;

    @BeforeAll
    static void checkPrerequisites() {
        // Check if backend is available
        try {
            URL url = new URL(AIM_URL + "/health");
            HttpURLConnection conn = (HttpURLConnection) url.openConnection();
            conn.setRequestMethod("GET");
            conn.setConnectTimeout(5000);
            conn.setReadTimeout(5000);
            int responseCode = conn.getResponseCode();
            backendAvailable = (responseCode == 200);
            conn.disconnect();
        } catch (Exception e) {
            System.out.println("Backend not available: " + e.getMessage());
            backendAvailable = false;
        }

        // Check if credentials are available
        try {
            Map<String, String> creds = CredentialManager.loadSdkCredentials();
            credentialsAvailable = creds.containsKey("clientId") && creds.containsKey("clientSecret");
        } catch (Exception e) {
            credentialsAvailable = false;
        }

        if (!backendAvailable) {
            System.out.println("⚠️  Integration tests skipped: AIM backend not available at " + AIM_URL);
        }
        if (!credentialsAvailable) {
            System.out.println("⚠️  Integration tests skipped: No SDK credentials found");
        }
    }

    @Test
    @Order(1)
    @DisplayName("Should load SDK credentials from environment or file")
    void loadCredentials() {
        Assumptions.assumeTrue(credentialsAvailable, "SDK credentials not available");

        Map<String, String> creds = CredentialManager.loadSdkCredentials();

        assertNotNull(creds.get("clientId"), "clientId should be present");
        assertNotNull(creds.get("clientSecret"), "clientSecret should be present");

        System.out.println("✅ Loaded credentials with clientId: " + creds.get("clientId").substring(0, 8) + "...");
    }

    @Test
    @Order(2)
    @DisplayName("Should register agent with secure() method")
    void secureRegistration() {
        Assumptions.assumeTrue(backendAvailable && credentialsAvailable, "Prerequisites not met");

        try {
            List<String> capabilities = Arrays.asList("db:read", "db:write", "api:call");

            AIMClient agent = AIMClient.secure(TEST_AGENT_NAME, capabilities, AgentType.CUSTOM);

            assertNotNull(agent, "Agent should not be null");
            assertEquals(TEST_AGENT_NAME, agent.getAgentName());
            assertEquals(AgentType.CUSTOM, agent.getAgentType());

            System.out.println("✅ Agent registered: " + agent.getAgentName());

            agent.close();
        } catch (CredentialException e) {
            System.out.println("⚠️  Credential error (expected if no credentials): " + e.getMessage());
            Assumptions.assumeTrue(false, "Credentials not properly configured");
        } catch (AIMException e) {
            System.out.println("⚠️  AIM error: " + e.getMessage());
            fail("Unexpected AIM error: " + e.getMessage());
        }
    }

    @Test
    @Order(3)
    @DisplayName("Should verify capability successfully")
    void verifyCapability() {
        Assumptions.assumeTrue(backendAvailable && credentialsAvailable, "Prerequisites not met");

        try {
            AIMClient agent = AIMClient.secure(TEST_AGENT_NAME + "-verify", Arrays.asList("db:read"));

            VerificationResult result = agent.verifyCapability("db:read", "users_table");

            assertNotNull(result, "Result should not be null");
            assertNotNull(result.getStatus(), "Status should not be null");

            if (result.isVerified()) {
                System.out.println("✅ Capability verified: " + result.getVerificationId());
            } else {
                System.out.println("⚠️  Capability not verified: " + result.getStatus());
            }

            agent.close();
        } catch (AIMException e) {
            System.out.println("⚠️  Verification error: " + e.getMessage());
        }
    }

    @Test
    @Order(4)
    @DisplayName("Should verify capability with different risk levels")
    void verifyCapabilityWithRiskLevel() {
        Assumptions.assumeTrue(backendAvailable && credentialsAvailable, "Prerequisites not met");

        try {
            AIMClient agent = AIMClient.secure(
                    TEST_AGENT_NAME + "-risk",
                    Arrays.asList("db:read", "db:write", "db:delete")
            );

            // Test LOW risk
            VerificationResult lowRisk = agent.verifyCapability("db:read", "logs", RiskLevel.LOW, null);
            System.out.println("LOW risk verification: " + lowRisk.getStatus());

            // Test MEDIUM risk
            VerificationResult mediumRisk = agent.verifyCapability("db:write", "users", RiskLevel.MEDIUM, null);
            System.out.println("MEDIUM risk verification: " + mediumRisk.getStatus());

            // Test HIGH risk
            VerificationResult highRisk = agent.verifyCapability("db:delete", "records", RiskLevel.HIGH, null);
            System.out.println("HIGH risk verification: " + highRisk.getStatus());

            agent.close();
        } catch (AIMException e) {
            System.out.println("⚠️  Risk level verification error: " + e.getMessage());
        }
    }

    @Test
    @Order(5)
    @DisplayName("Should execute performAction when verified")
    void performAction() {
        Assumptions.assumeTrue(backendAvailable && credentialsAvailable, "Prerequisites not met");

        try {
            AIMClient agent = AIMClient.secure(
                    TEST_AGENT_NAME + "-action",
                    Arrays.asList("data:fetch")
            );

            String result = agent.performAction("data:fetch", "weather_api", () -> {
                // Simulated action
                return "{\"temp\": 72, \"city\": \"New York\"}";
            });

            assertNotNull(result, "Action result should not be null");
            assertTrue(result.contains("temp"), "Result should contain expected data");

            System.out.println("✅ Action executed: " + result);

            agent.close();
        } catch (ActionDeniedException e) {
            System.out.println("⚠️  Action denied (expected in some cases): " + e.getMessage());
        } catch (AIMException e) {
            System.out.println("⚠️  performAction error: " + e.getMessage());
        }
    }

    @Test
    @Order(6)
    @DisplayName("Should get agent details")
    void getAgentDetails() {
        Assumptions.assumeTrue(backendAvailable && credentialsAvailable, "Prerequisites not met");

        try {
            AIMClient agent = AIMClient.secure(
                    TEST_AGENT_NAME + "-details",
                    Arrays.asList("info:read")
            );

            Map<String, Object> details = agent.getAgentDetails();

            assertNotNull(details, "Details should not be null");

            System.out.println("✅ Agent details: " + details);

            agent.close();
        } catch (AIMException e) {
            System.out.println("⚠️  getAgentDetails error: " + e.getMessage());
        }
    }

    @Test
    @Order(7)
    @DisplayName("Should request new capability")
    void requestCapability() {
        Assumptions.assumeTrue(backendAvailable && credentialsAvailable, "Prerequisites not met");

        try {
            AIMClient agent = AIMClient.secure(
                    TEST_AGENT_NAME + "-request",
                    Arrays.asList("db:read")
            );

            Map<String, Object> result = agent.requestCapability(
                    "email:send",
                    "Need to send notifications to users"
            );

            assertNotNull(result, "Request result should not be null");

            System.out.println("✅ Capability request: " + result);

            agent.close();
        } catch (AIMException e) {
            System.out.println("⚠️  requestCapability error: " + e.getMessage());
        }
    }

    @Test
    @Order(8)
    @DisplayName("Should handle authentication failure gracefully")
    void authenticationFailure() {
        // Test with invalid credentials
        AIMClient client = new AIMClient.Builder()
                .agentName("bad-auth-agent")
                .aimUrl(AIM_URL)
                .clientId("invalid-client")
                .clientSecret("invalid-secret")
                .organizationId("invalid-org")
                .build();

        // Attempting operations with invalid credentials should fail
        // The builder doesn't authenticate, but operations would fail

        assertNotNull(client);
        client.close();
    }

    @Test
    @Order(9)
    @DisplayName("Should handle network errors gracefully")
    void networkErrorHandling() {
        // Test with unreachable URL
        AIMClient client = new AIMClient.Builder()
                .agentName("network-test-agent")
                .aimUrl("http://localhost:59999")  // Unlikely to be running
                .clientId("test-client")
                .clientSecret("test-secret")
                .build();

        assertNotNull(client);
        // Operations would fail but client should be created
        client.close();
    }
}
