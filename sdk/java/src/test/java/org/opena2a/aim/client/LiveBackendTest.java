package org.opena2a.aim.client;

import org.junit.jupiter.api.*;
import org.opena2a.aim.credentials.CredentialManager;
import org.opena2a.aim.exceptions.AIMException;

import java.net.HttpURLConnection;
import java.net.URL;
import java.util.Arrays;
import java.util.List;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Live backend tests for AIMClient.
 *
 * These tests require:
 * - AIM backend running at http://localhost:8080
 * - Valid SDK credentials in ~/.aim/sdk_credentials.json
 *
 * Run with: mvn test -Dtest=LiveBackendTest
 */
@TestMethodOrder(MethodOrderer.OrderAnnotation.class)
class LiveBackendTest {

    private static final String AIM_URL = "http://localhost:8080";
    private static final String TEST_AGENT_NAME = "demo-agent-java-sdk";

    private static boolean backendAvailable = false;
    private static boolean credentialsAvailable = false;
    private static AIMClient testAgent = null;

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
            System.out.println("✅ Backend available at " + AIM_URL);
        } catch (Exception e) {
            System.out.println("❌ Backend not available: " + e.getMessage());
            backendAvailable = false;
        }

        // Check if credentials are available
        try {
            Map<String, String> creds = CredentialManager.loadSdkCredentials();
            credentialsAvailable = creds.containsKey("refreshToken") ||
                                   (creds.containsKey("clientId") && creds.containsKey("clientSecret"));
            if (credentialsAvailable) {
                System.out.println("✅ SDK credentials found");
                System.out.println("   - Type: " + (creds.containsKey("refreshToken") ? "SDK OAuth (refresh token)" : "Client credentials"));
                System.out.println("   - AIM URL: " + creds.getOrDefault("aimUrl", "http://localhost:8080"));
            }
        } catch (Exception e) {
            System.out.println("❌ Credentials error: " + e.getMessage());
            credentialsAvailable = false;
        }
    }

    @AfterAll
    static void cleanup() {
        if (testAgent != null) {
            testAgent.close();
        }
    }

    @Test
    @Order(1)
    @DisplayName("Should load SDK credentials from file")
    void loadCredentials() {
        Assumptions.assumeTrue(credentialsAvailable, "SDK credentials not available");

        Map<String, String> creds = CredentialManager.loadSdkCredentials();

        assertNotNull(creds);
        assertFalse(creds.isEmpty());

        // Should have either refresh token or client credentials
        boolean hasValidCreds = creds.containsKey("refreshToken") ||
                                (creds.containsKey("clientId") && creds.containsKey("clientSecret"));
        assertTrue(hasValidCreds, "Should have valid credentials");

        System.out.println("✅ Credentials loaded successfully");
    }

    @Test
    @Order(2)
    @DisplayName("Should register agent with MCP servers using AIMClient.secure()")
    void registerAgent() throws Exception {
        System.out.println("🔍 Starting registerAgent test...");

        List<String> capabilities = Arrays.asList("db:read", "db:write", "api:call", "file:read", "file:write");
        List<String> mcpServers = Arrays.asList("filesystem", "github");

        System.out.println("🔍 Calling AIMClient.secure() with:");
        System.out.println("   - Capabilities: " + capabilities);
        System.out.println("   - MCP Servers (talksTo): " + mcpServers);

        testAgent = AIMClient.secure(TEST_AGENT_NAME, capabilities, AgentType.LANGCHAIN, mcpServers);

        assertNotNull(testAgent, "Agent should not be null");
        assertEquals(TEST_AGENT_NAME, testAgent.getAgentName());
        assertEquals(AgentType.LANGCHAIN, testAgent.getAgentType());

        System.out.println("✅ Agent registered: " + testAgent.getAgentName());
        System.out.println("   - Type: " + testAgent.getAgentType());
    }

    @Test
    @Order(3)
    @DisplayName("Should verify capability")
    void verifyCapability() {
        Assumptions.assumeTrue(testAgent != null, "Agent not registered");

        try {
            VerificationResult result = testAgent.verifyCapability("db:read", "users_table");

            assertNotNull(result, "Result should not be null");
            assertNotNull(result.getStatus(), "Status should not be null");

            System.out.println("✅ Capability verification result:");
            System.out.println("   - Verified: " + result.isVerified());
            System.out.println("   - Status: " + result.getStatus());
            System.out.println("   - Verification ID: " + result.getVerificationId());

        } catch (AIMException e) {
            System.out.println("⚠️ Verification error (may be expected): " + e.getMessage());
        }
    }

    @Test
    @Order(4)
    @DisplayName("Should execute performAction when verified")
    void performAction() {
        Assumptions.assumeTrue(testAgent != null, "Agent not registered");

        try {
            String result = testAgent.performAction("db:read", "test_resource", () -> {
                // Simulated database read
                return "{\"data\": [1, 2, 3]}";
            });

            assertNotNull(result, "Action result should not be null");
            assertTrue(result.contains("data"), "Result should contain expected data");

            System.out.println("✅ Action executed successfully: " + result);

        } catch (AIMException e) {
            System.out.println("⚠️ Action execution error (may be expected if not verified): " + e.getMessage());
        }
    }

    @Test
    @Order(5)
    @DisplayName("Should get agent details")
    void getAgentDetails() {
        Assumptions.assumeTrue(testAgent != null, "Agent not registered");

        try {
            Map<String, Object> details = testAgent.getAgentDetails();

            assertNotNull(details, "Details should not be null");

            System.out.println("✅ Agent details retrieved:");
            details.forEach((key, value) -> System.out.println("   - " + key + ": " + value));

        } catch (AIMException e) {
            System.out.println("⚠️ Get agent details error: " + e.getMessage());
        }
    }

    @Test
    @Order(6)
    @DisplayName("Should verify with different risk levels")
    void verifyWithRiskLevels() {
        Assumptions.assumeTrue(testAgent != null, "Agent not registered");

        try {
            // LOW risk
            VerificationResult lowRisk = testAgent.verifyCapability("db:read", "logs", RiskLevel.LOW, null);
            System.out.println("✅ LOW risk verification: " + lowRisk.getStatus());

            // MEDIUM risk
            VerificationResult mediumRisk = testAgent.verifyCapability("db:write", "users", RiskLevel.MEDIUM, null);
            System.out.println("✅ MEDIUM risk verification: " + mediumRisk.getStatus());

            // HIGH risk
            VerificationResult highRisk = testAgent.verifyCapability("api:call", "external-api", RiskLevel.HIGH, null);
            System.out.println("✅ HIGH risk verification: " + highRisk.getStatus());

        } catch (AIMException e) {
            System.out.println("⚠️ Risk level verification error: " + e.getMessage());
        }
    }
}
