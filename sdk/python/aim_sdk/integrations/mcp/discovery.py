"""
AIM MCP Server Discovery

Automatic discovery of MCP server capabilities using the official MCP protocol.
This module connects to MCP servers and enumerates all available tools, resources,
and prompts using the JSON-RPC 2.0 protocol.

This is the proper way to discover MCP capabilities - by directly querying the
server using the official MCP protocol rather than relying on manual capability lists.
"""

import asyncio
import json
import logging
import os
import selectors
import shlex
import subprocess
import sys
import time
from contextlib import contextmanager
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

logger = logging.getLogger(__name__)

# The anyio task group the MCP stdio client runs in collects whatever went
# wrong into an exception group whose str() is this sentence. It is the shape
# of the plumbing, not the failure: reported as-is it told an operator whose
# server command was a typo that there were "unhandled errors in a TaskGroup".
_TASKGROUP_WRAPPER_TEXT = "unhandled errors in a TaskGroup"


def _get_quiet_env() -> Dict[str, str]:
    """
    Get environment variables that suppress noisy subprocess output.

    This provides a cleaner UX during MCP capability discovery by hiding
    noisy output from MCP server initialization (e.g., npm warnings).
    """
    env = os.environ.copy()

    # Suppress npm output
    env["NPM_CONFIG_LOGLEVEL"] = "silent"
    env["NPM_CONFIG_PROGRESS"] = "false"
    env["NPM_CONFIG_FUND"] = "false"
    env["NPM_CONFIG_AUDIT"] = "false"
    env["NPM_CONFIG_UPDATE_NOTIFIER"] = "false"

    # Suppress npx output
    env["NPX_CONFIG_LOGLEVEL"] = "silent"

    # Suppress node warnings
    env["NODE_NO_WARNINGS"] = "1"
    env["NODE_OPTIONS"] = "--no-warnings"

    # Python-based MCP servers
    env["PYTHONWARNINGS"] = "ignore"

    return env


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


def _leaf_exceptions(exc: BaseException, depth: int = 0) -> List[BaseException]:
    """
    Flatten an exception to the leaves that actually describe the failure.

    A failed stdio session surfaces as an anyio/`BaseExceptionGroup` wrapper
    around the real error (a FileNotFoundError for a command that is not on
    PATH, a broken pipe for a process that exited). The wrapper's ``str()``
    names neither, so it is walked through rather than reported.
    """
    if exc is None or depth > 12:
        return []

    nested = getattr(exc, "exceptions", None)
    if isinstance(nested, (list, tuple)) and nested:
        leaves: List[BaseException] = []
        for sub in nested:
            leaves.extend(_leaf_exceptions(sub, depth + 1))
        if leaves:
            return leaves

    if _TASKGROUP_WRAPPER_TEXT in str(exc):
        chained = exc.__cause__ or exc.__context__
        if chained is not None:
            return _leaf_exceptions(chained, depth + 1)

    return [exc]


def describe_exception(exc: BaseException) -> str:
    """Render an exception (or exception group) as its underlying cause(s)."""
    described: List[str] = []
    for leaf in _leaf_exceptions(exc):
        text = str(leaf).strip()
        rendered = f"{type(leaf).__name__}: {text}" if text else type(leaf).__name__
        if rendered not in described:
            described.append(rendered)

    description = "; ".join(described)
    if not description or _TASKGROUP_WRAPPER_TEXT in description:
        # The wrapper carried no sub-exception we can read. Say that, rather
        # than repeating a sentence about a TaskGroup the caller never asked for.
        return (
            f"{type(exc).__name__} while running the MCP session "
            f"(no underlying error detail available)"
        )
    return description


def _last_informative_line(raw: bytes) -> Optional[str]:
    """The most specific line of a child's stderr, if it has one."""
    if not raw:
        return None
    lines = [line.strip() for line in raw.decode("utf-8", "replace").splitlines()]
    for line in reversed(lines):
        if not line:
            continue
        if "Traceback" in line:
            # A child's traceback header says nothing the reader needs and is
            # the one string this SDK promises not to put in front of them.
            return None
        return line[:200]
    return None


def _describe_early_exit(
    proc: "subprocess.Popen", command: str, deadline: float
) -> str:
    """Why a server process stopped talking before it answered."""
    returncode = proc.poll()
    if returncode is None:
        try:
            returncode = proc.wait(timeout=max(deadline - time.monotonic(), 0.2))
        except subprocess.TimeoutExpired:
            returncode = None

    detail = ""
    if returncode is not None and proc.stderr is not None:
        try:
            detail = _last_informative_line(proc.stderr.read()) or ""
        except Exception:
            detail = ""
    suffix = f" (stderr: {detail})" if detail else ""

    if returncode is None:
        return (
            f"command {command!r} closed its output without answering an MCP "
            f"initialize request{suffix}"
        )
    return (
        f"command {command!r} exited with status {returncode} without answering "
        f"an MCP initialize request{suffix}"
    )


def _shut_down(proc: "subprocess.Popen") -> None:
    """Stop a probed server process and release its pipes."""
    try:
        if proc.poll() is None:
            proc.terminate()
            try:
                proc.wait(timeout=2)
            except subprocess.TimeoutExpired:
                proc.kill()
                try:
                    proc.wait(timeout=2)
                except subprocess.TimeoutExpired:
                    pass
    except Exception:
        pass
    for stream in (proc.stdin, proc.stdout, proc.stderr):
        try:
            if stream is not None:
                stream.close()
        except Exception:
            pass


def describe_unusable_server_command(
    mcp_url: str,
    timeout_seconds: float = 10.0
) -> Optional[str]:
    """
    Run a configured MCP server command and report why it cannot be used.

    RUNS THE COMMAND. It is started as a child process, sent one MCP
    ``initialize`` request on stdin, and shut down again.

    This exists so that "why did discovery fail for this server?" has an answer
    even when the MCP client library is not installed: without it the SDK
    reported "MCP SDK not installed" for a command that was a typo, a dead path
    or a process that exits immediately, and the operator had no way to tell
    those apart.

    Returns:
        A description of the failure, or None when the command started and
        answered -- which is as far as this probe goes; enumerating the tools
        is the MCP client library's job.
    """
    try:
        command, args = parse_mcp_command(mcp_url)
    except ValueError as exc:
        return str(exc)

    budget = max(float(timeout_seconds), 0.1)
    deadline = time.monotonic() + budget

    try:
        proc = subprocess.Popen(
            [command] + [str(a) for a in args],
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            env=_get_quiet_env(),
        )
    except OSError as exc:
        return f"command {command!r} could not be executed: {exc}"

    try:
        request = json.dumps({
            "jsonrpc": "2.0",
            "id": 0,
            "method": "initialize",
            "params": {
                "protocolVersion": "2024-11-05",
                "capabilities": {},
                "clientInfo": {"name": "aim-sdk", "version": "probe"},
            },
        }) + "\n"
        try:
            proc.stdin.write(request.encode("utf-8"))
            proc.stdin.flush()
        except OSError:
            # Already gone; the read below reports the exit status.
            pass

        selector = selectors.DefaultSelector()
        selector.register(proc.stdout, selectors.EVENT_READ)
        try:
            while True:
                remaining = deadline - time.monotonic()
                if remaining <= 0:
                    return (
                        f"command {command!r} did not answer an MCP initialize "
                        f"request within {budget:g}s"
                    )
                if not selector.select(timeout=remaining):
                    continue
                # os.read, not readline: a server that writes a partial line
                # and stalls must not block this probe past its deadline.
                chunk = os.read(proc.stdout.fileno(), 4096)
                if chunk:
                    return None
                return _describe_early_exit(proc, command, deadline)
        finally:
            selector.close()
    except Exception as exc:  # pragma: no cover - defensive
        logger.debug("MCP command probe failed for %r", mcp_url, exc_info=True)
        return f"command {command!r} could not be probed: {type(exc).__name__}: {exc}"
    finally:
        _shut_down(proc)


async def _discover_capabilities_async(
    mcp_url: str,
    timeout_seconds: float = 30.0,
    quiet: bool = True
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
        quiet: If True, suppress subprocess output for cleaner UX (default: True)

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

        # Create server parameters with optional quiet mode env
        env = _get_quiet_env() if quiet else None
        server_params = StdioServerParameters(
            command=command,
            args=args,
            env=env
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
        # The full chain, with its traceback, goes to an aim_sdk logger at DEBUG
        # -- not to stderr. A detection run over a config with a broken server
        # used to print a third-party traceback in the middle of an agent's
        # startup output; the caller gets the cause as a string instead.
        logger.debug("MCP discovery failed for %r", mcp_url, exc_info=True)
        return MCPDiscoveryResult(
            server_name="unknown",
            server_version="unknown",
            protocol_version="unknown",
            discovery_time_ms=(end_time - start_time) * 1000,
            error=describe_exception(e)
        )


def discover_capabilities(
    mcp_url: str,
    timeout_seconds: float = 30.0,
    quiet: bool = True
) -> MCPDiscoveryResult:
    """
    Discover all capabilities from an MCP server.

    This is the main entry point for capability discovery. It connects to the
    MCP server using the official MCP protocol and enumerates all available
    tools, resources, and prompts.

    Args:
        mcp_url: MCP server command (e.g., "npx -y @modelcontextprotocol/server-filesystem /tmp")
        timeout_seconds: Maximum time to wait for discovery (default: 30s)
        quiet: If True, suppress subprocess output for cleaner UX (default: True)

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
        try:
            # Try to get existing event loop
            loop = asyncio.get_event_loop()
            if loop.is_running():
                # If we're already in an async context, create a new thread
                import concurrent.futures
                with concurrent.futures.ThreadPoolExecutor() as executor:
                    future = executor.submit(
                        asyncio.run,
                        _discover_capabilities_async(mcp_url, timeout_seconds, quiet)
                    )
                    return future.result(timeout=timeout_seconds)
            else:
                return loop.run_until_complete(
                    _discover_capabilities_async(mcp_url, timeout_seconds, quiet)
                )
        except RuntimeError:
            # No event loop, create new one
            return asyncio.run(
                _discover_capabilities_async(mcp_url, timeout_seconds, quiet)
            )
    except Exception as e:
        # Whatever failed outside the async body (loop setup, the executor's own
        # timeout) is reported in the same shape as a failure inside it, so a
        # caller never has to catch here to find out what happened.
        logger.debug("MCP discovery could not run for %r", mcp_url, exc_info=True)
        return MCPDiscoveryResult(
            server_name="unknown",
            server_version="unknown",
            protocol_version="unknown",
            error=describe_exception(e)
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
