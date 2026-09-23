"""
AIM SDK - MCP Server Auto-Detection

This module provides utilities for automatically detecting MCP servers
that an agent uses, through various detection methods:

1. SDK Import Analysis - Scanning Python imports for MCP packages
2. Claude Config Parsing - Reading Claude Desktop configuration files
3. Direct API - Manual reporting of known MCP servers
4. Dynamic Tool Discovery - Querying MCP servers for their available tools

Detection results can be reported to AIM using client.report_detections()
"""

import json
import logging
import os
import pathlib
import sys
import warnings
from dataclasses import dataclass
from typing import List, Dict, Optional, Any, Tuple
from datetime import datetime, timezone
import importlib.util

# Use importlib.metadata (Python 3.8+) for package detection
try:
    from importlib.metadata import distributions
except ImportError:
    # Fallback for Python < 3.8
    try:
        from importlib_metadata import distributions
    except ImportError:
        distributions = None

__all__ = [
    "MCPDetector",
    "MCPServerMetadata",
    "track_mcp_call",
    "auto_detect_mcps",
    "discover_mcp_capabilities",
    "discover_mcp_metadata",
    "get_mcp_server_config",
]

logger = logging.getLogger(__name__)


# Global MCP call tracker for runtime detection
_mcp_call_tracker = {}


def _default_sdk_version() -> str:
    """Resolve the installed package version at call time.

    Imported lazily because aim_sdk/__init__.py imports this module before
    it assigns the package-level __version__, so a module-scope import (or
    an import-time default) would either fail or freeze a stale version.
    """
    from aim_sdk import __version__
    return f"aim-sdk-python@{__version__}"


def _read_claude_config(config_path: pathlib.Path) -> Optional[Dict[str, Any]]:
    """
    Read and parse a Claude Desktop config, warning instead of swallowing.

    Every reader of the config used to sit behind ``except Exception: pass``,
    so a config file with a trailing comma made auto-detection return an empty
    list that was indistinguishable from "you have no MCP servers configured".
    The read still never raises -- detection must not break agent execution --
    but exactly one warning naming the file and the parse error is emitted, so
    the difference is visible.

    Returns:
        The parsed config dict, or None when it could not be read or parsed.
    """
    try:
        with open(config_path, 'r') as f:
            return json.load(f)
    except json.JSONDecodeError as e:
        _warn_unreadable_config(config_path, f"invalid JSON: {e}")
    except OSError as e:
        _warn_unreadable_config(config_path, f"could not be read: {e}")
    except Exception as e:  # pragma: no cover - defensive
        _warn_unreadable_config(config_path, f"{type(e).__name__}: {e}")
    return None


def _warn_unreadable_config(config_path: pathlib.Path, detail: str) -> None:
    """
    Emit THE one warning for an unusable Claude Desktop config.

    One channel on purpose: a Python warning, which is filterable per call site
    and reaches logging too for anyone who calls
    ``logging.captureWarnings(True)``. Emitting a log record beside it would
    make one unreadable file announce itself twice.
    """
    warnings.warn(
        f"AIM MCP detection skipped the Claude Desktop config at "
        f"{config_path}: {detail}",
        UserWarning,
        stacklevel=3,
    )


class MCPDetector:
    """
    Auto-detector for MCP servers used by an agent.

    This class scans the environment for MCP server usage and generates
    detection events that can be reported to AIM.

    Example:
        from aim_sdk import AIMClient, MCPDetector

        client = AIMClient(...)
        detector = MCPDetector()

        # Detect MCP servers
        detections = detector.detect_all()

        # Report to AIM
        result = client.report_detections(detections)
        print(f"Found {len(detections)} MCP servers")
    """

    def __init__(self, sdk_version: Optional[str] = None):
        """
        Initialize the MCP detector.

        Args:
            sdk_version: SDK version string to include in detections.
                Defaults to the installed package version.

        Raises:
            TypeError: If sdk_version is neither None nor a string. It is
                copied verbatim into the ``sdkVersion`` field of every
                detection this detector reports; a non-string reached the
                server as whatever json.dumps made of it.
            ValueError: If sdk_version is an empty string. An empty
                ``sdkVersion`` is not "use the default", it is a row claiming
                the SDK has no version.
        """
        if sdk_version is not None:
            if not isinstance(sdk_version, str):
                raise TypeError(
                    f"sdk_version must be a string or None -- got "
                    f"{sdk_version!r} ({type(sdk_version).__name__})"
                )
            if not sdk_version.strip():
                raise ValueError(
                    "sdk_version must be a non-empty string; pass None to use "
                    "the installed package version"
                )
        self.sdk_version = sdk_version if sdk_version is not None else _default_sdk_version()
        self._mcp_packages = [
            "@modelcontextprotocol/server-filesystem",
            "@modelcontextprotocol/server-github",
            "@modelcontextprotocol/server-memory",
            "@modelcontextprotocol/server-postgres",
            "@modelcontextprotocol/server-puppeteer",
            "@modelcontextprotocol/server-slack",
            "mcp-server-fetch",
            "mcp-server-git"
        ]

    def detect_all(self) -> List[Dict[str, Any]]:
        """
        Run all detection methods and return combined results.

        Returns:
            List of detection events
        """
        detections = []

        # Detect from Claude config
        config_detections = self.detect_from_claude_config()
        detections.extend(config_detections)

        # Detect from Python imports
        import_detections = self.detect_from_imports()
        detections.extend(import_detections)

        return detections

    def detect_from_claude_config(self) -> List[Dict[str, Any]]:
        """
        Detect MCP servers from Claude Desktop configuration.

        Reads the first of these that exists, in this order, and extracts its
        MCP server configurations -- the same list, in the same order, that
        ``_get_claude_config_path`` searches:

        1. ``~/Library/Application Support/Claude/claude_desktop_config.json``
           (macOS only)
        2. ``%APPDATA%\\Claude\\claude_desktop_config.json`` (Windows only)
        3. ``~/.config/Claude/claude_desktop_config.json``
        4. ``~/.claude/claude_desktop_config.json`` (legacy)

        NOT searched: ``~/.cursor/mcp.json``, ``./.cursor/mcp.json`` and
        ``./mcp.json``. Cursor's MCP configuration is a different file in a
        different format, and project-local config is not read at all -- an
        agent's working directory is not a trustworthy source of server
        commands this SDK may later execute.

        An unreadable or malformed config yields an empty list and one warning
        naming the file and the parse error; it never raises.

        Returns:
            List of detection events with method 'claude_config'
        """
        detections = []
        config_path = self._get_claude_config_path()

        if not config_path or not config_path.exists():
            return detections

        config = _read_claude_config(config_path)
        if config is None:
            return detections

        # Extract MCP servers from config
        mcp_servers = config.get("mcpServers", {}) if isinstance(config, dict) else {}

        for server_name, server_config in mcp_servers.items():
            detection = {
                "mcpServer": server_name,
                "detectionMethod": "claude_config",
                "confidence": 100.0,  # Config file is definitive
                "details": {
                    "configPath": str(config_path),
                    "command": server_config.get("command", ""),
                    "args": server_config.get("args", [])
                },
                "sdkVersion": self.sdk_version,
                "timestamp": datetime.now(timezone.utc).isoformat()
            }
            detections.append(detection)

        return detections

    def detect_from_imports(self) -> List[Dict[str, Any]]:
        """
        Detect MCP servers from Python imports.

        Scans sys.modules and installed packages for MCP-related imports.

        Returns:
            List of detection events with method 'sdk_import'
        """
        detections = []
        detected_packages = set()

        # Check currently loaded modules
        for module_name in sys.modules.keys():
            if self._is_mcp_module(module_name):
                package_name = self._extract_package_name(module_name)
                if package_name and package_name not in detected_packages:
                    detected_packages.add(package_name)

        # Check installed packages
        if distributions:
            try:
                for dist in distributions():
                    package_name = dist.metadata.get('Name', '')
                    if package_name and self._is_mcp_package(package_name):
                        if package_name not in detected_packages:
                            detected_packages.add(package_name)
            except Exception:
                pass

        # Create detection events
        for package_name in detected_packages:
            detection = {
                "mcpServer": package_name,
                "detectionMethod": "sdk_import",
                "confidence": 90.0,  # Import detection is high confidence
                "details": {
                    "packageName": package_name,
                    "detectionSource": "import_scan"
                },
                "sdkVersion": self.sdk_version,
                "timestamp": datetime.now(timezone.utc).isoformat()
            }
            detections.append(detection)

        return detections

    def discover_mcp_tools(
        self,
        server_name: str,
        server_config: Dict[str, Any],
        timeout: float = 30.0
    ) -> Tuple[List[str], Optional[str]]:
        """
        Dynamically discover tools from an MCP server using the official MCP protocol.

        RUNS THE SERVER. This method executes ``server_config["command"]`` with
        its configured arguments as a child process and speaks MCP to it over
        stdio. It is not a config read.

        Args:
            server_name: Name of the MCP server
            server_config: Server configuration from Claude Desktop config
            timeout: Maximum time to wait for response (seconds)

        Returns:
            Tuple of (list of tool names, error message if any). The error
            names the server, the command that was run, and what actually went
            wrong -- never the anyio wrapper text "unhandled errors in a
            TaskGroup", which is what a failed stdio session raises and which
            says nothing about the server that failed.

        Example:
            tools, error = detector.discover_mcp_tools(
                "filesystem",
                {"command": "npx", "args": ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]}
            )
            if not error:
                print(f"Found tools: {tools}")
        """
        command = server_config.get("command", "")
        args = server_config.get("args", [])

        if not command:
            return [], f"No command specified for server {server_name}"

        # Build the MCP command string for discovery
        mcp_command = f"{command} {' '.join(str(a) for a in args)}"

        def _failure(detail: str) -> Tuple[List[str], str]:
            return [], (
                f"MCP server '{server_name}' (command: {mcp_command}): {detail}"
            )

        try:
            # Use the existing MCP integration for discovery
            from aim_sdk.integrations.mcp.discovery import (
                describe_unusable_server_command,
                discover_capabilities,
                is_mcp_sdk_available,
            )

            if not is_mcp_sdk_available():
                # Without the MCP client library this SDK cannot complete a
                # session -- but it can still say whether the configured
                # command is even runnable, which is the answer the caller
                # actually needs when the command is a typo or a dead path.
                problem = describe_unusable_server_command(
                    mcp_command, timeout_seconds=timeout
                )
                if problem:
                    return _failure(problem)
                return _failure(
                    "the MCP client library is not installed, so its tools "
                    "could not be listed. Install with: pip install mcp"
                )

            result = discover_capabilities(mcp_command, timeout_seconds=timeout)

            if result.error:
                # Ask the command itself why, so the two branches of this
                # method answer identically. The MCP client library reports a
                # failed session; it does not report that the command was a
                # typo or that the process exited before saying anything, and
                # those are the cases an operator can act on. The extra start
                # happens only on the failure path and is bounded by the same
                # timeout; when the probe finds nothing wrong with starting
                # the server, the session error stands as the cause.
                problem = describe_unusable_server_command(
                    mcp_command, timeout_seconds=timeout
                )
                return _failure(problem or result.error)

            # Return all discovered capability names (tools + resources + prompts)
            return result.all_capability_names, None

        except Exception as e:
            # The third-party detail belongs in a log the operator can turn on,
            # not on stderr as a traceback beside a detection result.
            logger.debug(
                "MCP discovery for %r failed: %s", mcp_command, e, exc_info=True
            )
            return _failure(f"{type(e).__name__}: {e}")

    def detect_with_tools(
        self,
        discover_tools: bool = True,
        timeout_per_server: float = 30.0
    ) -> List[Dict[str, Any]]:
        """
        Detect MCP servers from Claude config AND discover their actual tools.

        This enhanced detection method not only finds MCP servers but also
        queries each one to discover its available tools/capabilities using
        the official MCP protocol.

        EXECUTES EVERY CONFIGURED SERVER COMMAND when ``discover_tools`` is
        true. The MCP protocol has no way to list a stdio server's tools
        without running it, so each ``command``/``args`` pair in the config is
        launched as a child process, asked for its tools, and shut down. Pass
        ``discover_tools=False`` to read the config without running anything.

        The config file searched is the one ``detect_from_claude_config``
        documents. An unreadable or malformed config yields an empty list and
        one warning; it never raises.

        Args:
            discover_tools: Whether to run and query servers for their tools
            timeout_per_server: Timeout for each server query

        Returns:
            List of detection events with 'capabilities' field populated. On a
            server that could not be queried, ``details.discoveryError`` names
            the server, the command that was run and the real cause.

        Example:
            detector = MCPDetector()
            detections = detector.detect_with_tools()
            for d in detections:
                print(f"{d['mcpServer']}: {len(d['details'].get('capabilities', []))} tools")
        """
        detections = []
        config_path = self._get_claude_config_path()

        if not config_path or not config_path.exists():
            return detections

        config = _read_claude_config(config_path)
        if config is None:
            return detections

        mcp_servers = config.get("mcpServers", {}) if isinstance(config, dict) else {}

        for server_name, server_config in mcp_servers.items():
            capabilities = []
            discovery_error = None

            # Try to discover tools if enabled
            if discover_tools:
                capabilities, discovery_error = self.discover_mcp_tools(
                    server_name,
                    server_config,
                    timeout=timeout_per_server
                )

            detection = {
                "mcpServer": server_name,
                "detectionMethod": "claude_config",
                "confidence": 100.0,
                "details": {
                    "configPath": str(config_path),
                    "command": server_config.get("command", ""),
                    "args": server_config.get("args", []),
                    "capabilities": capabilities,  # Actual discovered tools
                    "toolCount": len(capabilities),
                    "discoveryError": discovery_error
                },
                "sdkVersion": self.sdk_version,
                "timestamp": datetime.now(timezone.utc).isoformat()
            }
            detections.append(detection)

        return detections

    def _get_claude_config_path(self) -> Optional[pathlib.Path]:
        """
        Get path to Claude Desktop config file: the first of the searched
        locations that exists, or None.

        The search order is the list ``detect_from_claude_config``'s docstring
        publishes, and the two must stay in step -- the docstring named only
        ``~/.claude/claude_desktop_config.json`` while the search tried three
        other places, so a user whose config was found somewhere else had no
        way to know which file the SDK had read.

        Platform-native first, then the cross-platform locations, so a machine
        carrying both a real Claude Desktop config and a stale legacy one reads
        the one Claude Desktop actually writes.
        """
        for config_path in self._claude_config_search_paths():
            if config_path.exists():
                return config_path

        return None

    @staticmethod
    def _claude_config_search_paths() -> List[pathlib.Path]:
        """The Claude Desktop config locations searched, in search order."""
        home = pathlib.Path.home()
        paths: List[pathlib.Path] = []

        # macOS (Claude Desktop's own location)
        if sys.platform == 'darwin':
            paths.append(
                home / "Library" / "Application Support" / "Claude"
                / "claude_desktop_config.json"
            )

        # Windows (Claude Desktop's own location)
        if os.name == 'nt':
            appdata = os.getenv('APPDATA')
            if appdata:
                paths.append(
                    pathlib.Path(appdata) / "Claude" / "claude_desktop_config.json"
                )

        # XDG-style config directory (Claude Desktop on Linux)
        paths.append(home / ".config" / "Claude" / "claude_desktop_config.json")

        # Legacy Claude CLI style, searched on every platform
        paths.append(home / ".claude" / "claude_desktop_config.json")

        return paths

    def _is_mcp_module(self, module_name: str) -> bool:
        """Check if a module name is MCP-related."""
        module_lower = module_name.lower()

        # Exclude AIM SDK itself from being detected as an MCP server
        if "aim_sdk" in module_lower or "aimsdk" in module_lower:
            return False

        mcp_indicators = [
            "mcp",
            "model_context_protocol",
            "modelcontextprotocol"
        ]
        return any(indicator in module_lower for indicator in mcp_indicators)

    def _is_mcp_package(self, package_name: str) -> bool:
        """Check if a package name is MCP-related."""
        package_lower = package_name.lower()

        # Exclude AIM SDK itself from being detected as an MCP server
        if "aim_sdk" in package_lower or "aim-sdk" in package_lower or "aimsdk" in package_lower:
            return False

        # Check against known MCP packages
        for known_package in self._mcp_packages:
            if known_package.lower() in package_lower:
                return True

        # Check for common MCP naming patterns
        mcp_patterns = [
            "mcp-server-",
            "mcp_server_",
            "@modelcontextprotocol/",
            "modelcontextprotocol-"
        ]
        return any(pattern in package_lower for pattern in mcp_patterns)

    def _extract_package_name(self, module_name: str) -> Optional[str]:
        """Extract top-level package name from module name."""
        parts = module_name.split('.')
        if parts:
            return parts[0]
        return None

    @staticmethod
    def track_mcp_call(mcp_server: str, tool_name: Optional[str] = None):
        """
        Track a runtime MCP server call for auto-discovery.

        This method should be called whenever your agent invokes an MCP tool.
        The SDK will aggregate these calls and automatically report them to AIM.

        Args:
            mcp_server: Name of the MCP server being called
            tool_name: Optional name of the specific tool/function being invoked

        Example:
            from aim_sdk import MCPDetector

            # Before calling MCP tool
            MCPDetector.track_mcp_call("filesystem", "read_file")

            # Then call your MCP tool
            result = mcp_client.call_tool("filesystem", "read_file", {...})
        """
        if mcp_server not in _mcp_call_tracker:
            _mcp_call_tracker[mcp_server] = {
                "first_call": datetime.now(timezone.utc).isoformat(),
                "call_count": 0,
                "tools_used": set()
            }

        _mcp_call_tracker[mcp_server]["call_count"] += 1
        _mcp_call_tracker[mcp_server]["last_call"] = datetime.now(timezone.utc).isoformat()

        if tool_name:
            _mcp_call_tracker[mcp_server]["tools_used"].add(tool_name)

    @staticmethod
    def get_runtime_detections(sdk_version: Optional[str] = None) -> List[Dict[str, Any]]:
        """
        Get MCP detections from runtime tracking.

        Returns MCP servers that were tracked via track_mcp_call().

        Args:
            sdk_version: SDK version string. Defaults to the installed
                package version.

        Returns:
            List of detection events with method 'sdk_runtime'
        """
        if sdk_version is None:
            sdk_version = _default_sdk_version()

        detections = []

        for mcp_server, stats in _mcp_call_tracker.items():
            # Convert tools_used set to list for JSON serialization
            tools_list = list(stats.get("tools_used", set()))

            detection = {
                "mcpServer": mcp_server,
                "detectionMethod": "sdk_runtime",
                "confidence": 100.0,  # Runtime calls are definitive
                "details": {
                    "call_count": stats.get("call_count", 0),
                    "first_call": stats.get("first_call"),
                    "last_call": stats.get("last_call"),
                    "tools_used": tools_list
                },
                "sdkVersion": sdk_version,
                "timestamp": datetime.now(timezone.utc).isoformat()
            }
            detections.append(detection)

        return detections

    def detect_all_with_runtime(self) -> List[Dict[str, Any]]:
        """
        Run all detection methods INCLUDING runtime tracking.

        This combines static detection (config, imports) with runtime tracking.

        Returns:
            List of detection events from all sources
        """
        detections = []

        # Static detection (config + imports)
        detections.extend(self.detect_from_claude_config())
        detections.extend(self.detect_from_imports())

        # Runtime detection (tracked calls)
        detections.extend(self.get_runtime_detections(self.sdk_version))

        return detections


def track_mcp_call(mcp_server: str, tool_name: Optional[str] = None):
    """
    Track a runtime MCP server call for auto-discovery (convenience function).

    This function should be called whenever your agent invokes an MCP tool.
    The SDK will aggregate these calls and automatically report them to AIM.

    Args:
        mcp_server: Name of the MCP server being called
        tool_name: Optional name of the specific tool/function being invoked

    Example:
        from aim_sdk import track_mcp_call

        # Track before calling MCP tool
        track_mcp_call("filesystem", "read_file")

        # Then call your MCP tool
        result = mcp_client.call_tool("filesystem", "read_file", {...})
    """
    MCPDetector.track_mcp_call(mcp_server, tool_name)


def auto_detect_mcps(
    sdk_version: Optional[str] = None,
    discover_tools: bool = False,
    timeout_per_server: float = 10.0
) -> List[Dict[str, Any]]:
    """
    Convenience function for quick MCP detection.

    This is a helper function that creates an MCPDetector and runs
    all detection methods.

    With ``discover_tools=True`` this EXECUTES EVERY SERVER COMMAND in the
    Claude Desktop config: each configured ``command``/``args`` pair is
    launched as a child process, asked for its tools over stdio, and shut
    down. The default (``discover_tools=False``) reads the config and the
    loaded Python modules and runs nothing.

    Args:
        sdk_version: SDK version string. Defaults to the installed package version.
        discover_tools: If True, run each configured MCP server and query it
            for its tools
        timeout_per_server: Timeout for querying each server (if discover_tools=True)

    Returns:
        List of detection events

    Example:
        from aim_sdk import AIMClient, auto_detect_mcps

        client = AIMClient(...)

        # Quick detection (just server names)
        detections = auto_detect_mcps()

        # Full detection with tool discovery
        detections = auto_detect_mcps(discover_tools=True)
        result = client.report_detections(detections)
    """
    if sdk_version is None:
        sdk_version = _default_sdk_version()

    detector = MCPDetector(sdk_version=sdk_version)

    if discover_tools:
        return detector.detect_with_tools(
            discover_tools=True,
            timeout_per_server=timeout_per_server
        )

    return detector.detect_all()


def _discover_single_server_capabilities(
    detector: 'MCPDetector',
    server_name: str,
    server_config: Dict[str, Any],
    timeout: float
) -> Tuple[str, List[str]]:
    """Discover capabilities for a single MCP server (for parallel execution)."""
    try:
        tools, _ = detector.discover_mcp_tools(server_name, server_config, timeout=timeout)
        return (server_name, tools or [])
    except Exception:
        return (server_name, [])


def discover_mcp_capabilities(
    server_names: Optional[List[str]] = None,
    timeout_per_server: float = 15.0  # 15s per server (runs in parallel)
) -> Dict[str, List[str]]:
    """
    Discover actual capabilities (tools) for MCP servers from Claude Desktop config.

    Uses parallel execution to query multiple servers concurrently for fast discovery.

    Args:
        server_names: List of server names to query. If None, queries all servers in config.
        timeout_per_server: Maximum time to wait for each server (seconds)

    Returns:
        Dict mapping server names to their list of tool names

    Example:
        from aim_sdk import discover_mcp_capabilities

        # Discover tools for specific servers
        caps = discover_mcp_capabilities(["filesystem", "github"])
        print(caps["filesystem"])  # ['read_file', 'write_file', 'list_directory', ...]

        # Discover all servers
        all_caps = discover_mcp_capabilities()
        for name, tools in all_caps.items():
            print(f"{name}: {len(tools)} tools")
    """
    from concurrent.futures import ThreadPoolExecutor, as_completed

    detector = MCPDetector()
    config_path = detector._get_claude_config_path()

    result: Dict[str, List[str]] = {}

    if not config_path or not config_path.exists():
        return result

    config = _read_claude_config(config_path)
    if config is None:
        return result

    try:
        mcp_servers = config.get("mcpServers", {}) if isinstance(config, dict) else {}

        # Filter to requested servers if specified
        if server_names:
            mcp_servers = {
                name: cfg for name, cfg in mcp_servers.items()
                if name in server_names or any(
                    sn.lower() in name.lower() for sn in server_names
                )
            }

        if not mcp_servers:
            return result

        # Use parallel discovery for performance (max 10 concurrent)
        max_workers = min(10, len(mcp_servers))

        with ThreadPoolExecutor(max_workers=max_workers) as executor:
            futures = {
                executor.submit(
                    _discover_single_server_capabilities,
                    detector, server_name, server_config, timeout_per_server
                ): server_name
                for server_name, server_config in mcp_servers.items()
            }

            for future in as_completed(futures, timeout=timeout_per_server * 2):
                try:
                    server_name, tools = future.result(timeout=timeout_per_server)
                    result[server_name] = tools
                except Exception:
                    # Skip failed servers silently
                    pass

    except Exception as e:
        logger.debug("MCP capability discovery failed: %s", e, exc_info=True)

    return result


def get_mcp_server_config(server_name: str) -> Optional[Dict[str, Any]]:
    """
    Get the configuration for a specific MCP server from Claude Desktop config.

    Args:
        server_name: Name of the MCP server

    Returns:
        Server configuration dict or None if not found

    Example:
        config = get_mcp_server_config("filesystem")
        if config:
            print(f"Command: {config.get('command')}")
            print(f"Args: {config.get('args')}")
    """
    detector = MCPDetector()
    config_path = detector._get_claude_config_path()

    if not config_path or not config_path.exists():
        return None

    config = _read_claude_config(config_path)
    if config is None:
        return None

    mcp_servers = config.get("mcpServers", {}) if isinstance(config, dict) else {}

    # Direct match
    if server_name in mcp_servers:
        return mcp_servers[server_name]

    # Partial match (case-insensitive)
    for name, cfg in mcp_servers.items():
        if server_name.lower() in name.lower():
            return cfg

    return None


@dataclass
class MCPServerMetadata:
    """Metadata discovered from an MCP server."""
    name: str
    version: str
    capabilities: List[str]

    def to_dict(self) -> Dict[str, Any]:
        return {
            "name": self.name,
            "version": self.version,
            "capabilities": self.capabilities
        }


def _discover_single_server_metadata(
    detector: 'MCPDetector',
    server_name: str,
    server_config: Dict[str, Any],
    timeout: float
) -> Tuple[str, MCPServerMetadata]:
    """Discover metadata (version + capabilities) for a single MCP server."""
    try:
        command = server_config.get("command", "")
        args = server_config.get("args", [])

        if not command:
            return (server_name, MCPServerMetadata(name=server_name, version="unknown", capabilities=[]))

        mcp_command = f"{command} {' '.join(str(a) for a in args)}"

        from aim_sdk.integrations.mcp.discovery import discover_capabilities
        result = discover_capabilities(mcp_command, timeout_seconds=timeout)

        return (server_name, MCPServerMetadata(
            name=server_name,
            version=result.server_version if result.server_version else "unknown",
            capabilities=result.all_capability_names if not result.error else []
        ))
    except Exception:
        return (server_name, MCPServerMetadata(name=server_name, version="unknown", capabilities=[]))


def discover_mcp_metadata(
    server_names: Optional[List[str]] = None,
    timeout_per_server: float = 15.0
) -> Dict[str, MCPServerMetadata]:
    """
    Discover metadata (version + capabilities) for MCP servers from Claude Desktop config.

    Uses parallel execution to query multiple servers concurrently for fast discovery.

    Args:
        server_names: List of server names to query. If None, queries all servers in config.
        timeout_per_server: Maximum time to wait for each server (seconds)

    Returns:
        Dict mapping server names to their MCPServerMetadata (version + capabilities)

    Example:
        from aim_sdk.detection import discover_mcp_metadata

        # Discover metadata for specific servers
        metadata = discover_mcp_metadata(["github", "filesystem"])
        for name, meta in metadata.items():
            print(f"{name}: v{meta.version}, {len(meta.capabilities)} capabilities")

        # Access individual fields
        github = metadata.get("github")
        if github:
            print(f"GitHub version: {github.version}")
            print(f"GitHub tools: {github.capabilities[:5]}")
    """
    from concurrent.futures import ThreadPoolExecutor, as_completed

    detector = MCPDetector()
    config_path = detector._get_claude_config_path()

    result: Dict[str, MCPServerMetadata] = {}

    if not config_path or not config_path.exists():
        return result

    config = _read_claude_config(config_path)
    if config is None:
        return result

    try:
        mcp_servers = config.get("mcpServers", {}) if isinstance(config, dict) else {}

        # Filter to requested servers if specified
        if server_names:
            mcp_servers = {
                name: cfg for name, cfg in mcp_servers.items()
                if name in server_names or any(
                    sn.lower() in name.lower() for sn in server_names
                )
            }

        if not mcp_servers:
            return result

        # Use parallel discovery for performance (max 10 concurrent)
        max_workers = min(10, len(mcp_servers))

        with ThreadPoolExecutor(max_workers=max_workers) as executor:
            futures = {
                executor.submit(
                    _discover_single_server_metadata,
                    detector, server_name, server_config, timeout_per_server
                ): server_name
                for server_name, server_config in mcp_servers.items()
            }

            for future in as_completed(futures, timeout=timeout_per_server * 2):
                try:
                    server_name, metadata = future.result(timeout=timeout_per_server)
                    result[server_name] = metadata
                except Exception:
                    # Skip failed servers silently
                    pass

    except Exception as e:
        logger.debug("MCP metadata discovery failed: %s", e, exc_info=True)

    return result
