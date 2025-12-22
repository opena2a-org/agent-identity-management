package org.opena2a.aim.examples;

import org.opena2a.aim.client.AIMClient;
import org.opena2a.aim.integrations.mcp.AttestationResult;
import org.opena2a.aim.integrations.mcp.MCPIntegration;
import org.opena2a.aim.integrations.mcp.MCPServerInfo;

import java.util.Arrays;
import java.util.List;
import java.util.Map;

/**
 * Example demonstrating MCP (Model Context Protocol) integration.
 *
 * <p>This example shows how to:</p>
 * <ul>
 *   <li>Register MCP servers with AIM</li>
 *   <li>Attest to server capabilities</li>
 *   <li>Record tool usage for supply chain analytics</li>
 *   <li>List registered MCP servers</li>
 * </ul>
 */
public class MCPIntegrationExample {

    // Example Ed25519 public key (base64 encoded)
    // In production, this would be the actual public key of the MCP server
    private static final String EXAMPLE_PUBLIC_KEY =
            "MCowBQYDK2VwAyEAExamplePublicKeyForDemonstration123456789012345";

    public static void main(String[] args) {
        System.out.println("=== AIM MCP Integration Example ===\n");

        try {
            // Step 1: Register the agent
            System.out.println("1. Registering agent...");
            AIMClient agent = AIMClient.secure(
                    "mcp-demo-agent",
                    Arrays.asList("mcp:register", "mcp:attest", "mcp:use"),
                    null
            );
            System.out.println("   Agent registered: " + agent.getAgentName());

            // Step 2: Register an MCP server
            System.out.println("\n2. Registering MCP server...");
            List<String> capabilities = Arrays.asList(
                    "read_file",
                    "write_file",
                    "list_directory",
                    "create_directory",
                    "delete_file"
            );

            MCPServerInfo server = MCPIntegration.registerServer(
                    agent,
                    "filesystem-mcp",
                    "http://localhost:3000",
                    EXAMPLE_PUBLIC_KEY,
                    capabilities,
                    "Local filesystem MCP server for file operations",
                    "1.0.0"
            );

            System.out.println("   Server registered!");
            System.out.println("   Server ID: " + server.getId());
            System.out.println("   Server name: " + server.getName());
            System.out.println("   Trust score: " + server.getTrustScore());
            System.out.println("   Status: " + server.getStatus());
            System.out.println("   Capabilities: " + server.getCapabilities());

            // Step 3: Attest to the server's capabilities
            System.out.println("\n3. Submitting attestation...");
            AttestationResult attestation = MCPIntegration.attestServer(
                    agent,
                    server.getId(),
                    "npx -y @modelcontextprotocol/server-filesystem /tmp",
                    "filesystem-mcp",
                    capabilities,
                    true,   // connection successful
                    true,   // health check passed
                    45.0    // connection latency in ms
            );

            System.out.println("   Attestation submitted!");
            System.out.println("   Attestation ID: " + attestation.getAttestationId());
            System.out.println("   Confidence score: " + attestation.getConfidenceScore() + "%");
            System.out.println("   Attestation count: " + attestation.getAttestationCount());

            // Step 4: Record tool usage
            System.out.println("\n4. Recording tool usage...");

            // Simulate using the read_file tool
            Map<String, Object> usage1 = MCPIntegration.recordToolUsage(
                    agent,
                    server.getId(),
                    "read_file",
                    "npx -y @modelcontextprotocol/server-filesystem /tmp",
                    "filesystem-mcp"
            );
            System.out.println("   Recorded: read_file");

            // Simulate using the list_directory tool
            Map<String, Object> usage2 = MCPIntegration.recordToolUsage(
                    agent,
                    server.getId(),
                    "list_directory"
            );
            System.out.println("   Recorded: list_directory");

            // Simulate using the write_file tool
            Map<String, Object> usage3 = MCPIntegration.recordToolUsage(
                    agent,
                    server.getId(),
                    "write_file"
            );
            System.out.println("   Recorded: write_file");

            // Step 5: List all registered MCP servers
            System.out.println("\n5. Listing registered MCP servers...");
            List<MCPServerInfo> servers = MCPIntegration.listServers(agent);

            for (MCPServerInfo s : servers) {
                System.out.println("   - " + s.getName() +
                        " (trust: " + s.getTrustScore() + ", status: " + s.getStatus() + ")");
            }

            // Step 6: Verify an MCP action
            System.out.println("\n6. Verifying MCP action...");
            boolean verified = MCPIntegration.verifyAction(
                    agent,
                    server.getId(),
                    "read_file"
            );
            System.out.println("   Action verified: " + verified);

            // Clean up
            agent.close();
            System.out.println("\n=== Example completed ===");

        } catch (Exception e) {
            System.err.println("Error: " + e.getMessage());
            e.printStackTrace();
        }
    }
}
