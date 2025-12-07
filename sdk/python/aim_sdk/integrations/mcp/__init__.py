"""
AIM SDK - MCP (Model Context Protocol) Integration

Seamless integration between AIM (Agent Identity Management) and MCP servers
for automatic verification and registration of AI agent context sources.

Available integrations:
- register_mcp_server: Register MCP servers with AIM backend
- attest_mcp_server: Submit cryptographic attestations with optional auto-discovery
- discover_capabilities: Auto-discover MCP server capabilities via MCP protocol
- verify_mcp_action: Verify MCP tool/resource/prompt usage

Capability Auto-Discovery:
    The SDK can automatically discover MCP server capabilities using the official
    MCP protocol (tools/list). This is lightweight and on-demand - no background
    processes, no polling, just a single query at attestation time.

    # Manual capability list (default, no overhead)
    attest_mcp_server(..., capabilities_found=["read_file", "write_file"])

    # OR auto-discover capabilities (opt-in)
    attest_mcp_server(..., auto_discover=True)

Usage:
    from aim_sdk.integrations.mcp import register_mcp_server, discover_capabilities

    # Register MCP server with AIM
    server_info = register_mcp_server(
        aim_client=aim_client,
        server_name="my-mcp-server",
        server_url="http://localhost:3000",
        public_key="ed25519_public_key",
        capabilities=["tools", "resources", "prompts"]
    )

    # Discover capabilities from MCP server (optional)
    result = discover_capabilities("npx -y @modelcontextprotocol/server-filesystem /tmp")
    print(f"Found {len(result.tools)} tools")
"""

from aim_sdk.integrations.mcp.registration import (
    register_mcp_server,
    list_mcp_servers,
    attest_mcp_server,
    use_mcp_tool,
    get_attestation_challenge,  # For proof of private key possession
)
from aim_sdk.integrations.mcp.verification import verify_mcp_action
from aim_sdk.integrations.mcp.discovery import (
    discover_capabilities,
    auto_discover_capabilities,
    is_mcp_sdk_available,
    MCPDiscoveryResult,
    MCPTool,
    MCPResource,
    MCPPrompt,
)

__all__ = [
    # Registration functions
    "register_mcp_server",
    "list_mcp_servers",
    "attest_mcp_server",
    "use_mcp_tool",
    "get_attestation_challenge",
    # Verification
    "verify_mcp_action",
    # Discovery (opt-in feature)
    "discover_capabilities",
    "auto_discover_capabilities",
    "is_mcp_sdk_available",
    "MCPDiscoveryResult",
    "MCPTool",
    "MCPResource",
    "MCPPrompt",
]
