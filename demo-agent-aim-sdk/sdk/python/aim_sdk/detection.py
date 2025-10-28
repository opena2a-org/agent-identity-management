"""
AIM SDK - MCP Server Auto-Detection

This module provides utilities for automatically detecting MCP servers
that an agent uses, through various detection methods:

1. SDK Import Analysis - Scanning Python imports for MCP packages
2. Claude Config Parsing - Reading Claude Desktop configuration files
3. Direct API - Manual reporting of known MCP servers

Detection results can be reported to AIM using client.report_detections()
"""

import json
import os
import pathlib
import sys
from typing import List, Dict, Optional, Any
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


__version__ = "1.0.0"

# Global MCP call tracker for runtime detection
_mcp_call_tracker = {}


class MCPDetector:
    """
    Auto-detector for MCP servers used by an agent.

    This class scans the environment for MCP server usage and generates
    detection events that can be reported to AIM.

    Example:
        from aim_sdk import AIMClient, MCPDetector

        client = AIMClient(...)
        detector = MCPDetector(sdk_version="aim-sdk-python@1.0.0")

        # Detect MCP servers
        detections = detector.detect_all()

        # Report to AIM
        result = client.report_detections(detections)
        print(f"Found {len(detections)} MCP servers")
    """

    def __init__(self, sdk_version: str = f"aim-sdk-python@{__version__}"):
        """
        Initialize the MCP detector.

        Args:
            sdk_version: SDK version string to include in detections
        """
        self.sdk_version = sdk_version
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

        Reads ~/.claude/claude_desktop_config.json and extracts MCP server
        configurations.

        Returns:
            List of detection events with method 'claude_config'
        """
        detections = []
        config_path = self._get_claude_config_path()

        if not config_path or not config_path.exists():
            return detections

        try:
            with open(config_path, 'r') as f:
                config = json.load(f)

            # Extract MCP servers from config
            mcp_servers = config.get("mcpServers", {})

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

        except Exception as e:
            # Silently fail - don't break agent execution
            pass

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

    def _get_claude_config_path(self) -> Optional[pathlib.Path]:
        """Get path to Claude Desktop config file."""
        home = pathlib.Path.home()

        # macOS/Linux path
        config_path = home / ".claude" / "claude_desktop_config.json"
        if config_path.exists():
            return config_path

        # Windows path
        if os.name == 'nt':
            appdata = os.getenv('APPDATA')
            if appdata:
                config_path = pathlib.Path(appdata) / "Claude" / "claude_desktop_config.json"
                if config_path.exists():
                    return config_path

        return None

    def _is_mcp_module(self, module_name: str) -> bool:
        """Check if a module name is MCP-related."""
        mcp_indicators = [
            "mcp",
            "model_context_protocol",
            "modelcontextprotocol"
        ]
        module_lower = module_name.lower()
        return any(indicator in module_lower for indicator in mcp_indicators)

    def _is_mcp_package(self, package_name: str) -> bool:
        """Check if a package name is MCP-related."""
        package_lower = package_name.lower()

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
    def get_runtime_detections(sdk_version: str = f"aim-sdk-python@{__version__}") -> List[Dict[str, Any]]:
        """
        Get MCP detections from runtime tracking.

        Returns MCP servers that were tracked via track_mcp_call().

        Args:
            sdk_version: SDK version string

        Returns:
            List of detection events with method 'sdk_runtime'
        """
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
    sdk_version: str = f"aim-sdk-python@{__version__}"
) -> List[Dict[str, Any]]:
    """
    Convenience function for quick MCP detection.

    This is a helper function that creates an MCPDetector and runs
    all detection methods.

    Args:
        sdk_version: SDK version string

    Returns:
        List of detection events

    Example:
        from aim_sdk import AIMClient, auto_detect_mcps

        client = AIMClient(...)
        detections = auto_detect_mcps()
        result = client.report_detections(detections)
    """
    detector = MCPDetector(sdk_version=sdk_version)
    return detector.detect_all()
