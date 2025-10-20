#!/usr/bin/env python3
"""
Simple Example: Auto-Detect MCP Servers from Claude Desktop Config

This example shows the simplest way to use the MCP auto-detection feature.
It automatically discovers MCP servers from your Claude Desktop configuration
and registers them with AIM.

Author: AIM SDK Team
License: MIT
"""

from aim_sdk import AIMClient
from aim_sdk.integrations.mcp import detect_mcp_servers_from_config


def main():
    # Step 1: Initialize AIM client with your agent credentials
    # (You get these when you register your agent with AIM)
    aim_client = AIMClient(
        agent_id="550e8400-e29b-41d4-a716-446655440000",
        public_key="your-base64-public-key",
        private_key="your-base64-private-key",
        aim_url="https://aim.example.com"
    )

    # Step 2: Auto-detect and register MCP servers
    # This will:
    # - Read your Claude Desktop config (~/.config/claude/claude_desktop_config.json)
    # - Detect all MCP servers configured there
    # - Automatically register them with AIM
    # - Map them to your agent's "talks_to" list
    result = detect_mcp_servers_from_config(
        aim_client=aim_client,
        agent_id="550e8400-e29b-41d4-a716-446655440000"
    )

    # Step 3: Review the results
    print(f"✅ Detected {len(result['detected_servers'])} MCP servers")
    print(f"✅ Registered {result['registered_count']} new servers")
    print(f"✅ Mapped {result['mapped_count']} servers to agent")

    # List detected servers
    for server in result['detected_servers']:
        print(f"\n📦 {server['name']}")
        print(f"   Command: {server['command']}")
        print(f"   Confidence: {server['confidence']}%")

    # Done! Your agent now has all MCP servers registered and mapped.
    aim_client.close()


if __name__ == "__main__":
    main()
