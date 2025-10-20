"""
AIM SDK - MCP (Model Context Protocol) Integration

Seamless integration between AIM (Agent Identity Management) and MCP servers
for automatic verification and registration of AI agent context sources.

Available integrations:
- register_mcp_server: Register MCP servers with AIM backend
- list_mcp_servers: List all registered MCP servers
- detect_mcp_servers_from_config: Auto-detect MCP servers from Claude Desktop config
- find_claude_config: Automatically find Claude Desktop config file
- get_default_config_paths: Get list of default config file paths for current OS
- verify_mcp_action: Verify MCP tool/resource/prompt usage (manual)
- auto_detect_capabilities: Auto-detect agent capabilities for risk assessment
- get_agent_capabilities: Get all capabilities registered for an agent
- aim_mcp_tool: Decorator for automatic MCP tool verification
- aim_mcp_session: Context manager for MCP sessions with automatic verification
- MCPProtocolInterceptor: Protocol-level interceptor for MCP calls

Usage - Manual verification:
    from aim_sdk.integrations.mcp import register_mcp_server, verify_mcp_action

    # Register MCP server with AIM
    server_info = register_mcp_server(
        aim_client=aim_client,
        server_name="my-mcp-server",
        server_url="http://localhost:3000",
        public_key="ed25519_public_key",
        capabilities=["tools", "resources", "prompts"]
    )

    # Manual verification
    verification = verify_mcp_action(
        aim_client=aim_client,
        mcp_server_id=server_info["id"],
        action_type="mcp_tool:web_search",
        resource="search query",
        risk_level="low"
    )

Usage - Automatic verification (decorator):
    from aim_sdk.integrations.mcp import aim_mcp_tool

    @aim_mcp_tool(aim_client=client, mcp_server_id=server_id, risk_level="low")
    def web_search(query: str):
        '''Search the web via MCP'''
        return mcp_client.call_tool("web_search", {"query": query})

    # Verification happens automatically
    results = web_search("AI safety")

Usage - Automatic verification (context manager):
    from aim_sdk.integrations.mcp import aim_mcp_session, aim_mcp_tool

    with aim_mcp_session(aim_client, server_id, verbose=True):
        # All tools in this block are automatically verified
        @aim_mcp_tool(risk_level="low")
        def search(query: str):
            return mcp_client.call_tool("search", {"query": query})

        results = search("quantum computing")

Usage - Automatic verification (protocol interceptor):
    from aim_sdk.integrations.mcp import MCPProtocolInterceptor

    # Wrap MCP client for automatic verification
    verified_mcp = MCPProtocolInterceptor(
        mcp_client=mcp_client,
        aim_client=aim_client,
        mcp_server_id=server_id,
        auto_verify=True
    )

    # All calls automatically verified
    results = verified_mcp.call_tool("web_search", {"query": "AI safety"})

Usage - Auto-detect MCP servers:
    from aim_sdk.integrations.mcp import detect_mcp_servers_from_config

    # Auto-detect and register MCP servers from Claude Desktop
    result = detect_mcp_servers_from_config(
        aim_client=aim_client,
        agent_id="550e8400-e29b-41d4-a716-446655440000"
    )
    print(f"Detected {len(result['detected_servers'])} MCP servers")
"""

from aim_sdk.integrations.mcp.registration import register_mcp_server, list_mcp_servers
from aim_sdk.integrations.mcp.verification import verify_mcp_action, MCPActionWrapper
from aim_sdk.integrations.mcp.auto_detection import (
    detect_mcp_servers_from_config,
    find_claude_config,
    get_default_config_paths
)
from aim_sdk.integrations.mcp.capabilities import (
    auto_detect_capabilities,
    get_agent_capabilities
)
from aim_sdk.integrations.mcp.auto_detect import (
    aim_mcp_tool,
    aim_mcp_session,
    MCPProtocolInterceptor,
    MCPSessionContext
)

__all__ = [
    # Registration & Discovery
    "register_mcp_server",
    "list_mcp_servers",
    "detect_mcp_servers_from_config",
    "find_claude_config",
    "get_default_config_paths",

    # Capabilities
    "auto_detect_capabilities",
    "get_agent_capabilities",

    # Manual Verification
    "verify_mcp_action",
    "MCPActionWrapper",

    # Automatic Verification
    "aim_mcp_tool",
    "aim_mcp_session",
    "MCPProtocolInterceptor",
    "MCPSessionContext",
]
