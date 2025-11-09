#!/usr/bin/env python3
"""
Comprehensive SDK Integration Test
Tests the complete flow: agent registration, MCP registration, verification
"""

import sys
import os
import json

# Add SDK to path
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "sdk/python"))

from aim_sdk import register_agent
from aim_sdk.integrations.mcp.registration import register_mcp_server

AIM_URL = "http://localhost:8080"

def test_1_agent_registration():
    """Test 1: Register an agent"""
    print("\n" + "="*70)
    print("TEST 1: AGENT REGISTRATION")
    print("="*70)

    try:
        # Register agent (will auto-generate keys)
        agent_name = "test-sdk-agent"
        print(f"Registering agent: {agent_name}")

        agent = register_agent(
            agent_name,
            aim_url=AIM_URL,
            description="Test agent for comprehensive SDK testing"
        )

        print(f"✅ Agent registered successfully!")
        print(f"   Agent ID: {agent.agent_id}")
        print(f"   Name: {agent_name}")
        print(f"   Public Key: {agent.public_key[:20]}...")

        return agent

    except Exception as e:
        print(f"❌ Agent registration failed: {e}")
        import traceback
        traceback.print_exc()
        return None


def test_2_mcp_registration(agent):
    """Test 2: Register MCP servers"""
    print("\n" + "="*70)
    print("TEST 2: MCP SERVER REGISTRATION")
    print("="*70)

    if not agent:
        print("⏭️  Skipping (agent registration failed)")
        return None

    try:
        # Register multiple MCP servers
        servers = [
            {
                "name": "Filesystem MCP",
                "url": "http://localhost:3100",
                "capabilities": ["read_file", "write_file", "list_directory"],
                "description": "File system operations"
            },
            {
                "name": "Weather MCP",
                "url": "http://localhost:3101",
                "capabilities": ["get_weather", "get_forecast"],
                "description": "Weather data provider"
            },
            {
                "name": "Database MCP",
                "url": "http://localhost:3102",
                "capabilities": ["query_db", "execute_sql"],
                "description": "Database access"
            }
        ]

        registered_servers = []
        for server_config in servers:
            print(f"\nRegistering: {server_config['name']}")

            server_info = register_mcp_server(
                aim_client=agent,
                server_name=server_config["name"],
                server_url=server_config["url"],
                public_key=f"ed25519_{server_config['name'].replace(' ', '_')}_key",
                capabilities=server_config["capabilities"],
                description=server_config["description"],
                version="1.0.0"
            )

            print(f"   ✅ ID: {server_info.get('id', 'N/A')}")
            print(f"   Status: {server_info.get('status', 'N/A')}")
            registered_servers.append(server_info)

        print(f"\n✅ Registered {len(registered_servers)} MCP servers")
        return registered_servers

    except Exception as e:
        print(f"❌ MCP registration failed: {e}")
        import traceback
        traceback.print_exc()
        return None


def test_3_verification_requests(agent):
    """Test 3: Send verification requests"""
    print("\n" + "="*70)
    print("TEST 3: VERIFICATION REQUESTS")
    print("="*70)

    if not agent:
        print("⏭️  Skipping (agent registration failed)")
        return

    try:
        # Test different types of actions
        actions = [
            {"action": "read_database", "resource": "users_table", "expected": "approved"},
            {"action": "delete_database", "resource": "users_table", "expected": "denied"},
            {"action": "send_email", "resource": "user@example.com", "expected": "approved"},
        ]

        for action_config in actions:
            print(f"\nTesting: {action_config['action']}")

            result = agent.verify_action(
                action_type=action_config["action"],
                resource=action_config["resource"]
            )

            print(f"   Status: {result.get('status', 'N/A')}")
            print(f"   Trust Score: {result.get('trust_score', 0):.2f}")

            if result.get('status') != action_config['expected']:
                print(f"   ⚠️  Expected {action_config['expected']}, got {result.get('status')}")

        print(f"\n✅ Verification requests completed")

    except Exception as e:
        print(f"❌ Verification failed: {e}")
        import traceback
        traceback.print_exc()


def main():
    """Run all tests"""
    print("\n" + "="*70)
    print("🧪 COMPREHENSIVE SDK INTEGRATION TEST")
    print("   Testing against: " + AIM_URL)
    print("="*70)

    # Run tests in sequence
    agent = test_1_agent_registration()
    mcp_servers = test_2_mcp_registration(agent)
    test_3_verification_requests(agent)

    # Summary
    print("\n" + "="*70)
    print("📊 TEST SUMMARY")
    print("="*70)
    print(f"✅ Agent registered: {agent is not None}")
    print(f"✅ MCP servers registered: {mcp_servers is not None and len(mcp_servers) > 0}")
    print("\n🎯 Next Steps:")
    print("   1. Check agents at: http://localhost:3000/dashboard/agents")
    print("   2. Check MCPs at: http://localhost:3000/dashboard/mcp")
    print("   3. Check violations at: [agent detail page] → Violations tab")
    print("="*70 + "\n")

    return 0 if (agent and mcp_servers) else 1


if __name__ == "__main__":
    sys.exit(main())
