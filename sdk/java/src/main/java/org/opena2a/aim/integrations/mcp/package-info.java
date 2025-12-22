/**
 * AIM SDK - MCP (Model Context Protocol) Integration.
 *
 * <p>This package provides integration between AIM (Agent Identity Management)
 * and MCP servers for automatic verification and registration of AI agent
 * context sources.</p>
 *
 * <p>Key classes:</p>
 * <ul>
 *   <li>{@link org.opena2a.aim.integrations.mcp.MCPIntegration} - Main integration class</li>
 *   <li>{@link org.opena2a.aim.integrations.mcp.MCPServerInfo} - MCP server information</li>
 *   <li>{@link org.opena2a.aim.integrations.mcp.AttestationResult} - Attestation result</li>
 * </ul>
 *
 * <p>Example usage:</p>
 * <pre>{@code
 * import org.opena2a.aim.client.AIMClient;
 * import org.opena2a.aim.integrations.mcp.MCPIntegration;
 *
 * AIMClient client = AIMClient.secure("my-agent");
 *
 * // Register an MCP server
 * MCPServerInfo server = MCPIntegration.registerServer(
 *     client,
 *     "filesystem-mcp",
 *     "http://localhost:3000",
 *     publicKey,
 *     Arrays.asList("read_file", "write_file")
 * );
 *
 * // Record tool usage
 * MCPIntegration.recordToolUsage(client, server.getId(), "read_file");
 * }</pre>
 */
package org.opena2a.aim.integrations.mcp;
