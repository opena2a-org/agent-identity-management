"""
AIM MCP Server Discovery

Automatic discovery of MCP server capabilities using the official MCP protocol.
This module connects to MCP servers and enumerates all available tools, resources,
and prompts using the JSON-RPC 2.0 protocol.

This is the proper way to discover MCP capabilities - by directly querying the
server using the official MCP protocol rather than relying on manual capability lists.
"""

import asyncio
import shlex
import time
from dataclasses import dataclass, field
from typing import Any, Dict, List, Optional, Tuple

# Check if MCP SDK is available
try:
    from mcp import ClientSession, StdioServerParameters
    from mcp.client.stdio import stdio_client
    MCP_SDK_AVAILABLE = True
except ImportError:
    MCP_SDK_AVAILABLE = False
    ClientSession = None
    StdioServerParameters = None
    stdio_client = None


@dataclass
class MCPTool:
    """Represents a tool discovered from an MCP server."""
    name: str
    description: Optional[str] = None
    input_schema: Optional[Dict[str, Any]] = None

    def to_dict(self) -> Dict[str, Any]:
        """Convert to dictionary for serialization."""
        return {
            "name": self.name,
            "description": self.description,
            "inputSchema": self.input_schema
        }


@dataclass
class MCPResource:
    """Represents a resource discovered from an MCP server."""
    uri: str
    name: str
    description: Optional[str] = None
    mime_type: Optional[str] = None

    def to_dict(self) -> Dict[str, Any]:
        """Convert to dictionary for serialization."""
        return {
            "uri": self.uri,
            "name": self.name,
            "description": self.description,
            "mimeType": self.mime_type
        }


@dataclass
class MCPPrompt:
    """Represents a prompt template discovered from an MCP server."""
    name: str
    description: Optional[str] = None
    arguments: Optional[List[Dict[str, Any]]] = None

    def to_dict(self) -> Dict[str, Any]:
        """Convert to dictionary for serialization."""
        return {
            "name": self.name,
            "description": self.description,
            "arguments": self.arguments
        }


@dataclass
class MCPDiscoveryResult:
    """Result of MCP server capability discovery."""
    server_name: str
    server_version: str
    protocol_version: str
    tools: List[MCPTool] = field(default_factory=list)
    resources: List[MCPResource] = field(default_factory=list)
    prompts: List[MCPPrompt] = field(default_factory=list)
    connection_latency_ms: float = 0.0
    discovery_time_ms: float = 0.0
    error: Optional[str] = None

    @property
    def tool_names(self) -> List[str]:
        """Get list of tool names for easy access."""
        return [tool.name for tool in self.tools]

    @property
    def resource_uris(self) -> List[str]:
        """Get list of resource URIs for easy access."""
        return [resource.uri for resource in self.resources]

    @property
    def prompt_names(self) -> List[str]:
        """Get list of prompt names for easy access."""
        return [prompt.name for prompt in self.prompts]

    @property
    def all_capability_names(self) -> List[str]:
        """Get all capability names (tools + resources + prompts) for attestation."""
        return self.tool_names + self.resource_uris + self.prompt_names

    def to_dict(self) -> Dict[str, Any]:
        """Convert to dictionary for serialization."""
        return {
            "serverName": self.server_name,
            "serverVersion": self.server_version,
            "protocolVersion": self.protocol_version,
            "tools": [tool.to_dict() for tool in self.tools],
            "resources": [resource.to_dict() for resource in self.resources],
            "prompts": [prompt.to_dict() for prompt in self.prompts],
            "connectionLatencyMs": self.connection_latency_ms,
            "discoveryTimeMs": self.discovery_time_ms,
            "error": self.error,
            "summary": {
                "toolCount": len(self.tools),
                "resourceCount": len(self.resources),
                "promptCount": len(self.prompts),
                "totalCapabilities": len(self.all_capability_names)
            }
        }


def parse_mcp_command(mcp_url: str) -> Tuple[str, List[str]]:
    """
    Parse an MCP server command string into command and arguments.

    Args:
        mcp_url: MCP server command (e.g., "npx -y @modelcontextprotocol/server-filesystem /tmp")

    Returns:
        Tuple of (command, args) for StdioServerParameters

    Examples:
        >>> parse_mcp_command("npx -y @modelcontextprotocol/server-filesystem /tmp")
        ('npx', ['-y', '@modelcontextprotocol/server-filesystem', '/tmp'])

        >>> parse_mcp_command("python -m my_mcp_server --config /etc/config.json")
        ('python', ['-m', 'my_mcp_server', '--config', '/etc/config.json'])
    """
    parts = shlex.split(mcp_url)
    if not parts:
        raise ValueError(f"Invalid MCP command: {mcp_url}")

    command = parts[0]
    args = parts[1:] if len(parts) > 1 else []
    return command, args


async def _discover_capabilities_async(
    mcp_url: str,
    timeout_seconds: float = 30.0
) -> MCPDiscoveryResult:
    """
    Asynchronously discover all capabilities from an MCP server.

    This function uses the official MCP protocol to query the server for:
    - Tools (via tools/list)
    - Resources (via resources/list)
    - Prompts (via prompts/list)

    Args:
        mcp_url: MCP server command (e.g., "npx -y @modelcontextprotocol/server-filesystem /tmp")
        timeout_seconds: Maximum time to wait for discovery (default: 30s)

    Returns:
        MCPDiscoveryResult containing all discovered capabilities
    """
    if not MCP_SDK_AVAILABLE:
        return MCPDiscoveryResult(
            server_name="unknown",
            server_version="unknown",
            protocol_version="unknown",
            error="MCP SDK not installed. Install with: pip install mcp"
        )

    start_time = time.time()

    try:
        # Parse the MCP command
        command, args = parse_mcp_command(mcp_url)

        # Create server parameters
        server_params = StdioServerParameters(
            command=command,
            args=args
        )

        # Connect to MCP server via stdio
        async with stdio_client(server_params) as (read_stream, write_stream):
            connection_time = time.time()
            connection_latency_ms = (connection_time - start_time) * 1000

            async with ClientSession(read_stream, write_stream) as session:
                # Initialize the session (required first step)
                init_result = await session.initialize()

                server_name = init_result.serverInfo.name if hasattr(init_result.serverInfo, 'name') else "unknown"
                server_version = init_result.serverInfo.version if hasattr(init_result.serverInfo, 'version') else "unknown"
                protocol_version = init_result.protocolVersion if hasattr(init_result, 'protocolVersion') else "unknown"

                # Initialize result
                result = MCPDiscoveryResult(
                    server_name=server_name,
                    server_version=server_version,
                    protocol_version=protocol_version,
                    connection_latency_ms=connection_latency_ms
                )

                # Discover tools
                try:
                    tools_result = await session.list_tools()
                    for tool in tools_result.tools:
                        result.tools.append(MCPTool(
                            name=tool.name,
                            description=getattr(tool, 'description', None),
                            input_schema=getattr(tool, 'inputSchema', None)
                        ))
                except Exception as e:
                    # Server may not support tools
                    pass

                # Discover resources
                try:
                    resources_result = await session.list_resources()
                    for resource in resources_result.resources:
                        result.resources.append(MCPResource(
                            uri=resource.uri,
                            name=resource.name,
                            description=getattr(resource, 'description', None),
                            mime_type=getattr(resource, 'mimeType', None)
                        ))
                except Exception as e:
                    # Server may not support resources
                    pass

                # Discover prompts
                try:
                    prompts_result = await session.list_prompts()
                    for prompt in prompts_result.prompts:
                        result.prompts.append(MCPPrompt(
                            name=prompt.name,
                            description=getattr(prompt, 'description', None),
                            arguments=getattr(prompt, 'arguments', None)
                        ))
                except Exception as e:
                    # Server may not support prompts
                    pass

                # Calculate total discovery time
                end_time = time.time()
                result.discovery_time_ms = (end_time - start_time) * 1000

                return result

    except Exception as e:
        end_time = time.time()
        return MCPDiscoveryResult(
            server_name="unknown",
            server_version="unknown",
            protocol_version="unknown",
            discovery_time_ms=(end_time - start_time) * 1000,
            error=str(e)
        )


def discover_capabilities(
    mcp_url: str,
    timeout_seconds: float = 30.0
) -> MCPDiscoveryResult:
    """
    Discover all capabilities from an MCP server.

    This is the main entry point for capability discovery. It connects to the
    MCP server using the official MCP protocol and enumerates all available
    tools, resources, and prompts.

    Args:
        mcp_url: MCP server command (e.g., "npx -y @modelcontextprotocol/server-filesystem /tmp")
        timeout_seconds: Maximum time to wait for discovery (default: 30s)

    Returns:
        MCPDiscoveryResult containing all discovered capabilities

    Example:
        from aim_sdk.integrations.mcp import discover_capabilities

        # Discover filesystem MCP server capabilities
        result = discover_capabilities("npx -y @modelcontextprotocol/server-filesystem /tmp")

        print(f"Server: {result.server_name} v{result.server_version}")
        print(f"Found {len(result.tools)} tools:")
        for tool in result.tools:
            print(f"  - {tool.name}: {tool.description}")

        # Get tool names for attestation
        tool_names = result.tool_names
        print(f"Tool names: {tool_names}")
    """
    if not MCP_SDK_AVAILABLE:
        return MCPDiscoveryResult(
            server_name="unknown",
            server_version="unknown",
            protocol_version="unknown",
            error="MCP SDK not installed. Install with: pip install mcp"
        )

    # Run the async discovery function
    try:
        # Try to get existing event loop
        loop = asyncio.get_event_loop()
        if loop.is_running():
            # If we're already in an async context, create a new thread
            import concurrent.futures
            with concurrent.futures.ThreadPoolExecutor() as executor:
                future = executor.submit(
                    asyncio.run,
                    _discover_capabilities_async(mcp_url, timeout_seconds)
                )
                return future.result(timeout=timeout_seconds)
        else:
            return loop.run_until_complete(
                _discover_capabilities_async(mcp_url, timeout_seconds)
            )
    except RuntimeError:
        # No event loop, create new one
        return asyncio.run(
            _discover_capabilities_async(mcp_url, timeout_seconds)
        )


def auto_discover_capabilities(
    mcp_url: str,
    fallback_capabilities: Optional[List[str]] = None,
    timeout_seconds: float = 30.0
) -> Tuple[List[str], MCPDiscoveryResult]:
    """
    Auto-discover MCP server capabilities with optional fallback.

    This is a convenience function that attempts to discover capabilities
    automatically, but falls back to a provided list if discovery fails.

    Args:
        mcp_url: MCP server command (e.g., "npx -y @modelcontextprotocol/server-filesystem /tmp")
        fallback_capabilities: Optional list of capabilities to use if discovery fails
        timeout_seconds: Maximum time to wait for discovery (default: 30s)

    Returns:
        Tuple of (capability_names, discovery_result)
        - capability_names: List of discovered tool/resource/prompt names
        - discovery_result: Full discovery result with details

    Example:
        from aim_sdk.integrations.mcp import auto_discover_capabilities

        # Auto-discover with fallback
        capabilities, result = auto_discover_capabilities(
            "npx -y @modelcontextprotocol/server-filesystem /tmp",
            fallback_capabilities=["read_file", "write_file"]  # Used if discovery fails
        )

        if result.error:
            print(f"Discovery failed: {result.error}, using fallback")
        else:
            print(f"Discovered {len(capabilities)} capabilities")
    """
    result = discover_capabilities(mcp_url, timeout_seconds)

    if result.error:
        # Discovery failed, use fallback
        capabilities = fallback_capabilities or []
        return capabilities, result

    # Return tool names (primary capabilities) plus any resources/prompts
    return result.all_capability_names, result


# Export check function for SDK availability
def is_mcp_sdk_available() -> bool:
    """Check if the MCP SDK is installed and available."""
    return MCP_SDK_AVAILABLE
