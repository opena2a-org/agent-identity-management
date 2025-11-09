"""
AIM SDK - MCP (Model Context Protocol) Integration

Seamless integration between AIM (Agent Identity Management) and MCP servers
for automatic verification and registration of AI agent context sources.

Available integrations:
- register_mcp_server: Register MCP servers with AIM backend
- AIMVerifiedMCPClient: Client for connecting to AIM-verified MCP servers
- verify_mcp_action: Verify MCP tool/resource/prompt usage

Usage:
    from aim_sdk.integrations.mcp import register_mcp_server

    # Register MCP server with AIM
    server_info = register_mcp_server(
        aim_client=aim_client,
        server_name="my-mcp-server",
        server_url="http://localhost:3000",
        public_key="ed25519_public_key",
        capabilities=["tools", "resources", "prompts"]
    )
"""

from aim_sdk.integrations.mcp.registration import register_mcp_server, list_mcp_servers, attest_mcp_server, use_mcp_tool
from aim_sdk.integrations.mcp.verification import verify_mcp_action

__all__ = [
    "register_mcp_server",
    "list_mcp_servers",
    "attest_mcp_server",
    "use_mcp_tool",
    "verify_mcp_action",
]
