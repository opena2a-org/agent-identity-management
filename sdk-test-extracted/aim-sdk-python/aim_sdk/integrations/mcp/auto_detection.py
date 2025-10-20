"""
AIM MCP Server Auto-Detection

Automatically detect and register MCP servers from Claude Desktop configuration files.
This module provides seamless integration for users who already have MCP servers
configured in Claude Desktop.
"""

import os
from typing import Any, Dict, List, Optional
import requests

from aim_sdk.client import AIMClient


def detect_mcp_servers_from_config(
    aim_client: AIMClient,
    agent_id: str,
    config_path: str = "~/.config/claude/claude_desktop_config.json",
    auto_register: bool = True,
    dry_run: bool = False
) -> Dict[str, Any]:
    """
    Auto-detect MCP servers from Claude Desktop configuration file.

    This function scans the Claude Desktop configuration file to discover MCP servers
    that are already configured locally. It can optionally register these servers
    automatically with AIM and associate them with the specified agent.

    The function performs the following steps:
    1. Reads the Claude Desktop config file (usually ~/.config/claude/claude_desktop_config.json)
    2. Parses the mcpServers section to extract server configurations
    3. Optionally registers new MCP servers with AIM (if auto_register=True)
    4. Maps the detected servers to the agent's "talks_to" list

    Args:
        aim_client: AIMClient instance for authentication and API communication
        agent_id: UUID of the agent to associate detected servers with
        config_path: Path to Claude Desktop config file
                     Default: "~/.config/claude/claude_desktop_config.json"
                     Also supports: "~/Library/Application Support/Claude/claude_desktop_config.json" (macOS)
        auto_register: If True, automatically registers new MCP servers with AIM (default: True)
                      If False, only maps existing registered servers to agent
        dry_run: If True, preview detected servers without making any changes (default: False)

    Returns:
        Dictionary containing detection results:
        {
            "detected_servers": [
                {
                    "name": "filesystem",
                    "command": "npx",
                    "args": ["-y", "@modelcontextprotocol/server-filesystem", "/path"],
                    "env": {"VAR": "value"},
                    "confidence": 100.0,
                    "source": "claude_desktop_config",
                    "metadata": {...}
                }
            ],
            "registered_count": 3,      # Number of newly registered servers
            "mapped_count": 3,           # Number of servers mapped to agent
            "total_talks_to": 5,         # Total MCP servers agent talks to
            "dry_run": False,            # Whether this was a dry run
            "errors_encountered": []     # Any errors during processing
        }

    Raises:
        FileNotFoundError: If config file doesn't exist
        ValueError: If agent_id is invalid or config file is malformed
        requests.exceptions.RequestException: If API request fails
        PermissionError: If authentication fails

    Examples:
        # Basic usage - auto-detect and register servers
        from aim_sdk import AIMClient
        from aim_sdk.integrations.mcp import detect_mcp_servers_from_config

        aim_client = AIMClient(
            agent_id="550e8400-e29b-41d4-a716-446655440000",
            public_key="base64-public-key",
            private_key="base64-private-key",
            aim_url="https://aim.example.com"
        )

        result = detect_mcp_servers_from_config(
            aim_client=aim_client,
            agent_id="550e8400-e29b-41d4-a716-446655440000"
        )

        print(f"Detected {len(result['detected_servers'])} MCP servers")
        print(f"Registered {result['registered_count']} new servers")
        print(f"Mapped {result['mapped_count']} servers to agent")

        # Preview detected servers without registering (dry run)
        result = detect_mcp_servers_from_config(
            aim_client=aim_client,
            agent_id="550e8400-e29b-41d4-a716-446655440000",
            dry_run=True
        )

        for server in result['detected_servers']:
            print(f"Found: {server['name']} (confidence: {server['confidence']}%)")

        # Detect from custom config path (macOS Library)
        result = detect_mcp_servers_from_config(
            aim_client=aim_client,
            agent_id="550e8400-e29b-41d4-a716-446655440000",
            config_path="~/Library/Application Support/Claude/claude_desktop_config.json"
        )

        # Only map existing servers without registering new ones
        result = detect_mcp_servers_from_config(
            aim_client=aim_client,
            agent_id="550e8400-e29b-41d4-a716-446655440000",
            auto_register=False
        )

    Notes:
        - The function expands tilde (~) in config_path to the user's home directory
        - Detection confidence is based on config file structure validation
        - Servers are identified by their "name" field in the config
        - If auto_register=True, duplicate servers are handled gracefully
        - Errors during registration don't block mapping of other servers
    """
    # Validate inputs
    if not agent_id or not agent_id.strip():
        raise ValueError("agent_id cannot be empty")

    if not config_path or not config_path.strip():
        raise ValueError("config_path cannot be empty")

    # Expand tilde (~) in path
    expanded_path = os.path.expanduser(config_path.strip())

    # Check if file exists
    if not os.path.exists(expanded_path):
        raise FileNotFoundError(
            f"Claude Desktop config file not found at: {expanded_path}\n"
            f"Please ensure Claude Desktop is installed and configured."
        )

    # Prepare request payload
    payload = {
        "config_path": expanded_path,
        "auto_register": auto_register,
        "dry_run": dry_run
    }

    # Make API request to auto-detection endpoint
    try:
        response = aim_client._make_request(
            method="POST",
            endpoint=f"/api/v1/agents/{agent_id.strip()}/mcp-servers/detect",
            data=payload
        )
        return response

    except requests.exceptions.RequestException as e:
        raise requests.exceptions.RequestException(
            f"Failed to detect MCP servers: {e}"
        )


def get_default_config_paths() -> List[str]:
    """
    Get list of default Claude Desktop config file paths based on OS.

    Returns:
        List of potential config file paths, in order of preference

    Example:
        from aim_sdk.integrations.mcp import get_default_config_paths

        for config_path in get_default_config_paths():
            if os.path.exists(os.path.expanduser(config_path)):
                print(f"Found config at: {config_path}")
                break
    """
    import platform

    os_name = platform.system()

    if os_name == "Darwin":  # macOS
        return [
            "~/Library/Application Support/Claude/claude_desktop_config.json",
            "~/.config/claude/claude_desktop_config.json"
        ]
    elif os_name == "Windows":
        return [
            "~/AppData/Roaming/Claude/claude_desktop_config.json",
            "~/.config/claude/claude_desktop_config.json"
        ]
    else:  # Linux and others
        return [
            "~/.config/claude/claude_desktop_config.json"
        ]


def find_claude_config() -> Optional[str]:
    """
    Automatically find Claude Desktop config file on the system.

    Searches common locations based on the operating system.

    Returns:
        Path to config file if found, None otherwise

    Example:
        from aim_sdk.integrations.mcp import find_claude_config

        config_path = find_claude_config()
        if config_path:
            print(f"Found Claude config at: {config_path}")
        else:
            print("Claude Desktop config not found")
    """
    for config_path in get_default_config_paths():
        expanded_path = os.path.expanduser(config_path)
        if os.path.exists(expanded_path):
            return config_path

    return None
