#!/usr/bin/env python3
"""
Simple test to register an MCP server and verify it shows in the dashboard.
"""

import sys
import os

# Add SDK to path
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "aim_sdk"))

from aim_sdk import AIMClient
from aim_sdk.integrations.mcp import register_mcp_server, list_mcp_servers

AIM_URL = "http://localhost:8080"


def main():
    print("\n" + "="*70)
    print("SIMPLE MCP REGISTRATION TEST")
    print("="*70)

    try:
        # Step 1: Register/load AIM agent
        print("\n[1/3] Registering AIM agent...")
        aim_client = AIMClient.auto_register_or_load(
            "test-mcp-dashboard-agent",
            AIM_URL
        )
        print(f"✅ Agent ID: {aim_client.agent_id}")

        # Step 2: Register an MCP server
        print("\n[2/3] Registering MCP server...")
        server_info = register_mcp_server(
            aim_client=aim_client,
            server_name="Filesystem MCP Server",
            server_url="http://localhost:3100",
            public_key="ed25519_filesystem_public_key_abc123",
            capabilities=["read_file", "write_file", "list_directory"],
            description="MCP server for file system operations",
            version="1.0.0"
        )
        print(f"✅ MCP Server registered: {server_info['id']}")
        print(f"   Name: {server_info['name']}")
        print(f"   URL: {server_info['url']}")
        print(f"   Status: {server_info.get('status', 'N/A')}")

        # Step 3: List all MCP servers
        print("\n[3/3] Listing all MCP servers...")
        servers = list_mcp_servers(aim_client, limit=10)
        print(f"✅ Total servers: {len(servers)}")
        for i, server in enumerate(servers, 1):
            print(f"   {i}. {server['name']} - {server.get('status', 'unknown')}")

        print("\n" + "="*70)
        print("🎉 SUCCESS! Check the dashboard at http://localhost:3000/dashboard/mcp")
        print("="*70)

    except Exception as e:
        print(f"\n❌ ERROR: {e}")
        import traceback
        traceback.print_exc()
        return 1

    return 0


if __name__ == "__main__":
    sys.exit(main())
