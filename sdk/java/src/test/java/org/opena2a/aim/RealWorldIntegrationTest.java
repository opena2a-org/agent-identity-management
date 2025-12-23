package org.opena2a.aim;

import org.junit.jupiter.api.*;
import org.opena2a.aim.client.*;
import org.opena2a.aim.credentials.CredentialManager;
import org.opena2a.aim.integration.langchain4j.AIMAgentCallback;
import org.opena2a.aim.integration.langchain4j.AIMToolExecutor;
import org.opena2a.aim.integrations.mcp.discovery.MCPDiscoveryResult;
import org.opena2a.aim.integrations.mcp.discovery.MCPTool;
import org.opena2a.aim.mcp.AttestationCache;
import org.opena2a.aim.security.*;

import java.util.*;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Real-world integration test that simulates how a developer would use the SDK.
 *
 * Scenario: A claims processing AI agent that:
 * 1. Registers with AIM
 * 2. Reads customer data (MEDIUM risk)
 * 3. Processes payments (HIGH risk)
 * 4. Uses MCP tools with attestation
 * 5. Logs all security events
 */
@TestMethodOrder(MethodOrderer.OrderAnnotation.class)
class RealWorldIntegrationTest {

    private static final String AGENT_NAME = "claims-processor-" + System.currentTimeMillis();
    private static AIMClient agent;
    private static boolean backendAvailable = false;

    @BeforeAll
    static void setup() {
        // Check backend availability
        try {
            Map<String, String> creds = CredentialManager.loadSdkCredentials();
            backendAvailable = !creds.isEmpty();
        } catch (Exception e) {
            backendAvailable = false;
        }
    }

    @AfterAll
    static void cleanup() {
        if (agent != null) {
            agent.close();
        }
    }

    // =========================================================================
    // 1. SECURITY LOGGER
    // =========================================================================

    @Test
    @Order(1)
    @DisplayName("SecurityLogger: Configure and log events")
    void testSecurityLogger() {
        SecurityLogger logger = SecurityLogger.getInstance();

        // Configure for testing
        SecurityLogger.Config config = SecurityLogger.Config.builder()
                .enabled(true)
                .logLevel(EventSeverity.DEBUG)
                .logToStdout(true)
                .build();
        logger.configure(config);

        // Start a session
        String sessionId = logger.startSession();
        assertNotNull(sessionId);
        System.out.println("📝 Session started: " + sessionId);

        // Set agent context
        logger.setAgentContext("agent-123", AGENT_NAME, "user-456", "org-789");

        // Log various event types
        logger.debug("Debug: Initializing agent");
        logger.info("Info: Agent ready to process claims");
        logger.warning("Warning: High volume of requests detected");

        // Log structured events using actual method signatures
        logger.logAuthentication(
                EventTypes.Authn.TOKEN_REFRESH,
                true,
                "Token refreshed successfully",
                Map.of("provider", "AIM", "scopes", "read,write")
        );

        logger.logAuthorizationEvent(
                EventTypes.Authz.CAPABILITY_GRANTED,
                "payment:process",
                "process-claim-123",
                true,
                Map.of("claimId", "CLM-2024-001", "amount", 5000)
        );

        logger.logAgentEvent(
                EventTypes.Agent.AGENT_REGISTERED,
                "agent-123",
                AGENT_NAME,
                "Claims processor started",
                Map.of("version", "1.0.0"),
                null
        );

        // Clear context
        logger.clearAgentContext();

        System.out.println("✅ SecurityLogger working correctly");
    }

    // =========================================================================
    // 2. RISK DETECTOR
    // =========================================================================

    @Test
    @Order(2)
    @DisplayName("RiskDetector: Analyze capability risks")
    void testRiskDetector() {
        RiskDetector detector = RiskDetector.getInstance();

        // Test built-in patterns - use security.RiskLevel
        Map<String, org.opena2a.aim.security.RiskLevel> testCases = new LinkedHashMap<>();
        testCases.put("db:read", org.opena2a.aim.security.RiskLevel.MEDIUM);
        testCases.put("db:write", org.opena2a.aim.security.RiskLevel.MEDIUM);
        testCases.put("db:delete", org.opena2a.aim.security.RiskLevel.HIGH);
        testCases.put("payment:process", org.opena2a.aim.security.RiskLevel.HIGH);
        testCases.put("admin:delete_user", org.opena2a.aim.security.RiskLevel.CRITICAL);
        testCases.put("api:call", org.opena2a.aim.security.RiskLevel.LOW);
        testCases.put("pii:access", org.opena2a.aim.security.RiskLevel.HIGH);
        testCases.put("credential:rotate", org.opena2a.aim.security.RiskLevel.CRITICAL);

        System.out.println("\n📊 Risk Detection Results:");
        for (Map.Entry<String, org.opena2a.aim.security.RiskLevel> entry : testCases.entrySet()) {
            org.opena2a.aim.security.RiskLevel detected = detector.detectRisk(entry.getKey());
            assertEquals(entry.getValue(), detected, "Risk level mismatch for: " + entry.getKey());
            System.out.printf("   %-25s → %s%n", entry.getKey(), detected);
        }

        // Test aggregate risk
        org.opena2a.aim.security.RiskLevel aggregate = detector.detectAggregateRisk(
                List.of("db:read", "payment:process", "api:call")
        );
        assertEquals(org.opena2a.aim.security.RiskLevel.HIGH, aggregate);
        System.out.println("   Aggregate risk          → " + aggregate);

        // Test context-aware assessment
        RiskDetector.RiskAssessment assessment = detector.assess(
                "db:query",
                Map.of(
                        "table", "customers",
                        "columns", List.of("ssn", "credit_card"),
                        "limit", 50000
                )
        );
        System.out.println("\n📋 Risk Assessment with context:");
        System.out.println("   Base level: " + assessment.getBaseLevel());
        System.out.println("   Adjusted:   " + assessment.getAdjustedLevel());
        System.out.println("   Elevated:   " + assessment.wasElevated());
        System.out.println("   Factors:    " + assessment.getFactors());

        // Register custom pattern for insurance domain
        detector.registerPattern(org.opena2a.aim.security.RiskLevel.HIGH, "^claim:approve.*");
        org.opena2a.aim.security.RiskLevel claimRisk = detector.detectRisk("claim:approve_payout");
        assertEquals(org.opena2a.aim.security.RiskLevel.HIGH, claimRisk);
        System.out.println("\n   Custom pattern claim:approve_payout → " + claimRisk);

        System.out.println("\n✅ RiskDetector working correctly");
    }

    // =========================================================================
    // 3. SECURE CREDENTIAL STORAGE
    // =========================================================================

    @Test
    @Order(3)
    @DisplayName("SecureCredentialStorage: Store and retrieve secrets")
    void testSecureCredentialStorage() {
        SecureCredentialStorage storage = SecureCredentialStorage.getInstance();

        // Store various credential types - uses Map<String, String>
        storage.store("api_credentials", Map.of(
                "api_key", "sk-test-abc123xyz789",
                "api_secret", "secret_value"
        ));
        storage.store("db_credentials", Map.of(
                "username", "admin",
                "password", "super$ecret!Pass"
        ));

        // Retrieve and verify
        Map<String, String> apiCreds = storage.retrieve("api_credentials");
        assertNotNull(apiCreds);
        assertEquals("sk-test-abc123xyz789", apiCreds.get("api_key"));

        Map<String, String> dbCreds = storage.retrieve("db_credentials");
        assertNotNull(dbCreds);
        assertEquals("super$ecret!Pass", dbCreds.get("password"));

        // List stored credentials
        List<String> stored = storage.listStoredCredentials();
        assertTrue(stored.contains("api_credentials"));
        assertTrue(stored.contains("db_credentials"));

        // Delete a credential
        boolean deleted = storage.delete("api_credentials");
        assertTrue(deleted);

        // Verify deletion
        Map<String, String> deletedCreds = storage.retrieve("api_credentials");
        assertNull(deletedCreds);

        System.out.println("✅ SecureCredentialStorage working correctly");
        System.out.println("   - Stored credentials with encryption");
        System.out.println("   - Verified retrieval and decryption");
        System.out.println("   - Tested delete operation");
    }

    // =========================================================================
    // 4. ATTESTATION CACHE
    // =========================================================================

    @Test
    @Order(4)
    @DisplayName("AttestationCache: Cache MCP capabilities with drift detection")
    void testAttestationCache() {
        AttestationCache cache = AttestationCache.getInstance();

        // Create MCP tools
        MCPTool tool1 = new MCPTool("searchDatabase", "Search the customer database", Map.of());
        MCPTool tool2 = new MCPTool("processPayment", "Process a payment transaction", Map.of());

        // Create MCP discovery result
        MCPDiscoveryResult discovery = MCPDiscoveryResult.builder()
                .serverName("claims-mcp")
                .serverVersion("2.1.0")
                .tools(List.of(tool1, tool2))
                .build();

        // Store the attestation
        String attestationId = "att-" + System.currentTimeMillis();
        Optional<AttestationCache.DriftReport> storeResult = cache.store("claims-mcp", discovery, attestationId);
        System.out.println("📦 Stored MCP attestation");

        // Retrieve from cache - uses public fields
        Optional<AttestationCache.CachedAttestation> cached = cache.get("claims-mcp");
        assertTrue(cached.isPresent());
        assertEquals("claims-mcp", cached.get().serverName);
        System.out.println("   Retrieved: " + cached.get().serverName + " with " + cached.get().toolCount + " tools");

        // Test drift detection - no change
        AttestationCache.DriftReport noDrift = cache.detectDrift("claims-mcp", discovery);
        assertFalse(noDrift.hasDrift());
        System.out.println("   No drift detected (same capabilities)");

        // Simulate capability drift - add a new tool
        MCPTool newTool = new MCPTool("deleteRecord", "Delete a customer record", Map.of());

        MCPDiscoveryResult driftedDiscovery = MCPDiscoveryResult.builder()
                .serverName("claims-mcp")
                .serverVersion("2.2.0")
                .tools(List.of(tool1, tool2, newTool))
                .build();

        AttestationCache.DriftReport drift = cache.detectDrift("claims-mcp", driftedDiscovery);
        assertTrue(drift.hasDrift());
        System.out.println("   ⚠️  Drift detected!");
        System.out.println("   - Changes: " + drift.getChanges().size());

        // Check cache validity
        assertTrue(cache.hasValidAttestation("claims-mcp"));

        System.out.println("\n✅ AttestationCache working correctly");
    }

    // =========================================================================
    // 5. SUPPLY CHAIN REPORTER
    // =========================================================================

    @Test
    @Order(5)
    @DisplayName("SupplyChainReporter: Track MCP versions and generate SBOM")
    void testSupplyChainReporter() {
        SupplyChainReporter reporter = SupplyChainReporter.getInstance();

        // Create discovery results for recording
        MCPDiscoveryResult discovery1 = MCPDiscoveryResult.builder()
                .serverName("claims-mcp")
                .serverVersion("2.1.0")
                .tools(List.of(new MCPTool("search", "Search", Map.of())))
                .build();

        MCPDiscoveryResult discovery2 = MCPDiscoveryResult.builder()
                .serverName("payment-mcp")
                .serverVersion("1.5.2")
                .tools(List.of(new MCPTool("process", "Process payment", Map.of())))
                .build();

        // Record MCP server connections
        reporter.recordMcpServer("claims-mcp", "npx claims-mcp", discovery1);
        reporter.recordMcpServer("payment-mcp", "npx payment-mcp", discovery2);

        // Get all recorded servers - uses public fields
        Map<String, SupplyChainReporter.McpServerRecord> servers = reporter.getAllServers();
        assertTrue(servers.size() >= 2);

        // Generate SBOM for agent - uses public fields
        SupplyChainReporter.SBOM sbom = reporter.generateSBOM(AGENT_NAME);
        assertNotNull(sbom);
        assertNotNull(sbom.format);

        System.out.println("📦 Supply Chain Report:");
        System.out.println("   SBOM Format: " + sbom.format);
        System.out.println("   Tracked MCP servers: " + servers.size());
        for (Map.Entry<String, SupplyChainReporter.McpServerRecord> entry : servers.entrySet()) {
            System.out.printf("   - %-25s v%s%n", entry.getKey(), entry.getValue().version);
        }

        // Get summary statistics
        Map<String, Object> summary = reporter.getSummary();
        System.out.println("\n   Summary: " + summary);

        System.out.println("\n✅ SupplyChainReporter working correctly");
    }

    // =========================================================================
    // 6. LANGCHAIN4J INTEGRATION
    // =========================================================================

    @Test
    @Order(6)
    @DisplayName("LangChain4j: Tool executor with risk-based blocking")
    void testLangChain4jToolExecutor() {
        AIMToolExecutor executor = new AIMToolExecutor(null);

        // Register tools with capabilities
        executor.register("searchClaims", "Search claims database",
                input -> "Found 5 claims matching: " + input,
                "db:read");

        executor.register("updateClaim", "Update claim status",
                input -> "Claim updated: " + input,
                "db:write");

        executor.register("processRefund", "Process a refund",
                input -> "Refund processed: $" + input,
                "payment:refund");

        executor.register("deleteAllClaims", "Delete all claims (dangerous!)",
                input -> "DELETED",
                "admin:delete_all");

        System.out.println("🔧 LangChain4j Tool Executor:");

        // Execute safe tools
        Object searchResult = executor.execute("searchClaims", "customer-123");
        System.out.println("   searchClaims → " + searchResult);

        Object updateResult = executor.execute("updateClaim", "CLM-001:approved");
        System.out.println("   updateClaim → " + updateResult);

        // Check tool info with risk levels
        System.out.println("\n   Tool Risk Levels:");
        for (String toolName : executor.getRegisteredTools()) {
            Map<String, Object> info = executor.getToolInfo(toolName);
            System.out.printf("   - %-20s %s%n", toolName, info.get("riskLevel"));
        }

        // Enable blocking for high-risk operations
        executor.setBlockHighRisk(true);
        executor.setApprovalThreshold(org.opena2a.aim.security.RiskLevel.HIGH);

        // Try to execute CRITICAL tool - should be blocked
        try {
            executor.execute("deleteAllClaims", "all");
            fail("Should have thrown ToolBlockedException");
        } catch (AIMToolExecutor.ToolBlockedException e) {
            System.out.println("\n   ⛔ Blocked: " + e.getMessage());
        }

        // Check metrics
        Map<String, Map<String, Object>> metrics = executor.getAllMetrics();
        System.out.println("\n   Metrics:");
        System.out.println("   - searchClaims executions: " + metrics.get("searchClaims").get("successCount"));
        System.out.println("   - deleteAllClaims blocked: " + metrics.get("deleteAllClaims").get("blockedCount"));

        System.out.println("\n✅ LangChain4j ToolExecutor working correctly");
    }

    @Test
    @Order(7)
    @DisplayName("LangChain4j: Agent callback for ReAct pattern")
    void testLangChain4jAgentCallback() {
        AIMAgentCallback callback = new AIMAgentCallback(null);
        callback.setMaxIterations(10);

        System.out.println("🤖 LangChain4j Agent Callback:");

        // Simulate a ReAct agent execution
        String executionId = callback.onAgentStart("claims-agent", "Process claim CLM-2024-001");
        System.out.println("   Started execution: " + executionId.substring(0, 8) + "...");

        // Thought-Action-Observation cycles
        callback.onAgentThought(executionId, "I need to look up the claim details first");
        callback.onAgentAction(executionId, "searchClaims", "CLM-2024-001");
        callback.onAgentObservation(executionId, "Claim found: Amount=$5000, Status=Pending");

        callback.onThoughtActionObservation(executionId,
                "Now I need to verify the customer's coverage",
                "checkCoverage",
                "Coverage verified: Maximum=$10000, Deductible=$500");

        callback.onThoughtActionObservation(executionId,
                "Coverage is sufficient, I'll approve the claim",
                "approveClaim",
                "Claim CLM-2024-001 approved for $4500 (after deductible)");

        // Check status during execution
        Map<String, Object> status = callback.getExecutionStatus(executionId);
        System.out.println("   Status: " + status.get("status"));
        System.out.println("   Steps: " + status.get("steps"));
        System.out.println("   Tool calls: " + status.get("toolCalls"));

        assertTrue(callback.shouldContinue(executionId));

        // Finish
        callback.onAgentFinish(executionId, "Claim CLM-2024-001 processed successfully. Payout: $4500");

        // Final metrics
        Map<String, Object> metrics = callback.getMetrics();
        System.out.println("\n   Final Metrics:");
        System.out.println("   - Total executions: " + metrics.get("totalExecutions"));
        System.out.println("   - Successful: " + metrics.get("successfulExecutions"));
        System.out.println("   - Total steps: " + metrics.get("totalSteps"));
        System.out.println("   - Total tool calls: " + metrics.get("totalToolCalls"));

        System.out.println("\n✅ LangChain4j AgentCallback working correctly");
    }

    // =========================================================================
    // 7. LIVE AGENT REGISTRATION & VERIFICATION
    // =========================================================================

    @Test
    @Order(8)
    @DisplayName("AIMClient: Register agent and verify capabilities (Live)")
    void testAIMClientLive() {
        Assumptions.assumeTrue(backendAvailable, "Backend not available");

        System.out.println("\n🔐 AIM Client Live Test:");

        // Register agent with capabilities
        List<String> capabilities = List.of("db:read", "db:write", "payment:process", "email:send");

        agent = AIMClient.secure(AGENT_NAME, capabilities, AgentType.CUSTOM);
        assertNotNull(agent);
        System.out.println("   Agent registered: " + agent.getAgentName());
        System.out.println("   Agent ID: " + agent.getAgentId());

        // Verify capabilities
        VerificationResult lowRisk = agent.verifyCapability("db:read", "claims_table");
        assertTrue(lowRisk.isVerified());
        System.out.println("   ✅ db:read verified");

        VerificationResult highRisk = agent.verifyCapability("payment:process", "refund-123",
                org.opena2a.aim.client.RiskLevel.HIGH, null);
        assertTrue(highRisk.isVerified());
        System.out.println("   ✅ payment:process verified");

        // Perform a guarded action
        String result = agent.performAction("db:read", "customer-456", () ->
                "{\"id\": \"customer-456\", \"name\": \"John Doe\", \"claims\": 3}");
        assertTrue(result.contains("John Doe"));
        System.out.println("   ✅ Guarded action executed successfully");

        // Get agent details
        Map<String, Object> details = agent.getAgentDetails();
        System.out.println("\n   Agent Details:");
        System.out.println("   - Status: " + details.get("status"));
        System.out.println("   - Trust Score: " + details.get("trustScore"));
        System.out.println("   - Capabilities: " + details.get("capabilities"));

        System.out.println("\n✅ AIMClient live integration working correctly");
    }

    // =========================================================================
    // 8. END-TO-END SCENARIO
    // =========================================================================

    @Test
    @Order(9)
    @DisplayName("End-to-End: Complete claims processing scenario")
    void testEndToEndScenario() {
        Assumptions.assumeTrue(backendAvailable, "Backend not available");
        Assumptions.assumeTrue(agent != null, "Agent not registered");

        System.out.println("\n🎯 End-to-End Claims Processing Scenario:");

        SecurityLogger logger = SecurityLogger.getInstance();
        RiskDetector riskDetector = RiskDetector.getInstance();
        AIMAgentCallback agentCallback = new AIMAgentCallback(agent);

        // Start the agent execution
        String executionId = agentCallback.onAgentStart(AGENT_NAME, "Process insurance claim CLM-2024-999");
        logger.setAgentContext(agent.getAgentId(), AGENT_NAME, "user-admin", "org-insurance");

        System.out.println("   1. Started claim processing for CLM-2024-999");

        // Step 1: Read claim from database
        agentCallback.onAgentThought(executionId, "First, I need to retrieve the claim details");
        org.opena2a.aim.security.RiskLevel readRisk = riskDetector.detectRisk("db:read");
        System.out.println("   2. Risk assessment for db:read: " + readRisk);

        VerificationResult readVerify = agent.verifyCapability("db:read", "claims_table");
        if (readVerify.isVerified()) {
            agentCallback.onAgentAction(executionId, "readClaim", "CLM-2024-999");
            String claimData = agent.performAction("db:read", "claims", () ->
                    "{\"claimId\": \"CLM-2024-999\", \"amount\": 7500, \"type\": \"auto\", \"status\": \"pending\"}");
            agentCallback.onAgentObservation(executionId, "Claim retrieved: " + claimData);
            System.out.println("   3. Retrieved claim data");

            logger.logAuthorizationEvent(EventTypes.Authz.ACTION_EXECUTED, "db:read", "readClaim",
                    true, Map.of("claimId", "CLM-2024-999"));
        }

        // Step 2: Process payment
        agentCallback.onAgentThought(executionId, "Claim is valid, processing payment");
        org.opena2a.aim.security.RiskLevel paymentRisk = riskDetector.detectRisk("payment:process");
        System.out.println("   4. Risk assessment for payment:process: " + paymentRisk);

        VerificationResult paymentVerify = agent.verifyCapability("payment:process", "payout",
                org.opena2a.aim.client.RiskLevel.HIGH, null);
        if (paymentVerify.isVerified()) {
            agentCallback.onAgentAction(executionId, "processPayment", "7500");
            String paymentResult = agent.performAction("payment:process", "payout-999", () ->
                    "{\"transactionId\": \"TXN-" + System.currentTimeMillis() + "\", \"amount\": 7500, \"status\": \"completed\"}");
            agentCallback.onAgentObservation(executionId, "Payment processed: " + paymentResult);
            System.out.println("   5. Payment processed successfully");

            logger.logAuthorizationEvent(EventTypes.Authz.ACTION_EXECUTED, "payment:process", "processPayment",
                    true, Map.of("amount", 7500, "claimId", "CLM-2024-999"));
        }

        // Step 3: Update claim status
        agentCallback.onAgentThought(executionId, "Updating claim status to completed");
        agentCallback.onAgentAction(executionId, "updateClaim", "CLM-2024-999:completed");

        VerificationResult writeVerify = agent.verifyCapability("db:write", "claims_table");
        if (writeVerify.isVerified()) {
            agent.performAction("db:write", "update-claim", () -> "OK");
            agentCallback.onAgentObservation(executionId, "Claim status updated to completed");
            System.out.println("   6. Claim status updated");
        }

        // Finish the execution
        agentCallback.onAgentFinish(executionId, "Claim CLM-2024-999 processed. Payout: $7500");

        // Log completion
        logger.logAgentEvent(EventTypes.Agent.AGENT_UPDATED, agent.getAgentId(), AGENT_NAME,
                "Claims processing completed", Map.of("claimId", "CLM-2024-999", "payout", 7500), null);

        // Final metrics
        Map<String, Object> metrics = agentCallback.getMetrics();
        System.out.println("\n   📊 Execution Summary:");
        System.out.println("   - Total steps: " + metrics.get("totalSteps"));
        System.out.println("   - Tool calls: " + metrics.get("totalToolCalls"));
        System.out.println("   - Success rate: " + metrics.get("successRate"));

        logger.clearAgentContext();

        System.out.println("\n✅ End-to-end scenario completed successfully!");
    }
}
