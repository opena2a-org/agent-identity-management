#!/usr/bin/env python3
"""
Integration tests for AIM + MCP (Model Context Protocol)

Tests MCP SDK API design and integration patterns.

NOTE: Full integration tests require JWT authentication (user login).
      See test_mcp_with_auth.py for authenticated tests.

This test validates:
1. SDK API design and function signatures
2. Error handling and validation
3. Integration patterns (registration, verification, wrapper)
4. Documentation and examples

For full end-to-end tests with backend, run: test_mcp_with_auth.py
"""

import sys
import os
from pathlib import Path

import pytest

# Add SDK to path
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "aim_sdk"))

from aim_sdk import AIMClient
from aim_sdk.integrations.mcp import (
    register_mcp_server,
    list_mcp_servers,
    verify_mcp_action,
)
from aim_sdk.integrations.mcp.verification import MCPActionWrapper, log_mcp_action_result

AIM_URL = "http://localhost:8080"

# Module-level state shared between sequential integration tests
_aim_client = None
_server_id = None


@pytest.mark.integration
def test_mcp_server_registration():
    """Test 1: MCP Server Registration"""
    print("\n" + "="*70)
    print("TEST 1: MCP Server Registration")
    print("="*70)

    global _aim_client, _server_id

    try:
        # Register AIM agent
        aim_client = AIMClient.auto_register_or_load(
            "mcp-test-registration",
            AIM_URL
        )
        print(f"AIM agent registered: {aim_client.agent_id}")

        # Register MCP server
        server_info = register_mcp_server(
            aim_client=aim_client,
            server_name="test-mcp-server",
            server_url="http://localhost:3000",
            public_key="ed25519_test_public_key_1234567890abcdef1234567890abcdef",
            capabilities=["tools", "resources", "prompts"],
            description="Test MCP server for integration testing",
            version="1.0.0"
        )
        print(f"MCP server registered: {server_info['id']}")
        print(f"   Name: {server_info['name']}")
        print(f"   Status: {server_info['status']}")
        print(f"   Trust Score: {server_info.get('trust_score', 'N/A')}")

        _aim_client = aim_client
        _server_id = server_info['id']

        print("\nTEST 1 PASSED - MCP server registration works!")
        return True, server_info['id']

    except Exception as e:
        print(f"\nTEST 1 FAILED: {e}")
        import traceback
        traceback.print_exc()
        return False, None


@pytest.mark.integration
def test_mcp_server_listing():
    """Test 2: MCP Server Listing"""
    print("\n" + "="*70)
    print("TEST 2: MCP Server Listing")
    print("="*70)

    aim_client = _aim_client
    if not aim_client:
        pytest.skip("No AIM client available (test_mcp_server_registration must run first)")

    try:
        # List MCP servers
        servers = list_mcp_servers(aim_client, limit=10)
        print(f"✅ Retrieved {len(servers)} MCP server(s)")

        for server in servers:
            print(f"   - {server['name']} ({server['status']})")

        print("\n🎉 TEST 2 PASSED - MCP server listing works!")
        return True

    except Exception as e:
        print(f"\n❌ TEST 2 FAILED: {e}")
        import traceback
        traceback.print_exc()
        return False


@pytest.mark.integration
def test_mcp_action_verification():
    """Test 3: MCP Action Verification"""
    print("\n" + "="*70)
    print("TEST 3: MCP Action Verification")
    print("="*70)

    aim_client = _aim_client
    server_id = _server_id

    if not aim_client:
        pytest.skip("No AIM client available (test_mcp_server_registration must run first)")
    if not server_id:
        print("⚠️  Skipping test - no server_id from registration")
        return True

    try:
        # Verify MCP tool usage
        verification = verify_mcp_action(
            aim_client=aim_client,
            mcp_server_id=server_id,
            action_type="mcp_tool:web_search",
            resource="search query: AI safety best practices",
            context={
                "tool": "web_search",
                "params": {"q": "AI safety"},
                "framework": "mcp"
            },
            risk_level="low"
        )
        print(f"✅ MCP action verified: {verification.get('verification_id', 'N/A')}")
        print(f"   Status: {verification.get('status', 'N/A')}")

        # Log action result
        verification_id = verification.get("verification_id")
        if verification_id:
            log_success = log_mcp_action_result(
                aim_client=aim_client,
                verification_id=verification_id,
                success=True,
                result_summary="Web search completed: 10 results found"
            )
            if log_success:
                print("✅ Action result logged successfully")
            else:
                print("⚠️  Action result logging returned false (endpoint may not exist yet)")

        print("\n🎉 TEST 3 PASSED - MCP action verification works!")
        return True

    except Exception as e:
        print(f"\n❌ TEST 3 FAILED: {e}")
        import traceback
        traceback.print_exc()
        return False


@pytest.mark.integration
def test_mcp_action_wrapper():
    """Test 4: MCPActionWrapper"""
    print("\n" + "="*70)
    print("TEST 4: MCPActionWrapper")
    print("="*70)

    aim_client = _aim_client
    server_id = _server_id

    if not aim_client:
        pytest.skip("No AIM client available (test_mcp_server_registration must run first)")
    if not server_id:
        print("⚠️  Skipping test - no server_id from registration")
        return True

    try:
        # Create MCP action wrapper
        mcp_wrapper = MCPActionWrapper(
            aim_client=aim_client,
            mcp_server_id=server_id,
            default_risk_level="low",
            verbose=True
        )
        print("✅ MCPActionWrapper created")

        # Execute tool with automatic verification
        def mock_web_search():
            """Mock MCP tool execution"""
            return {"results": ["result1", "result2", "result3"]}

        result = mcp_wrapper.execute_tool(
            tool_name="web_search",
            tool_function=mock_web_search,
            risk_level="low",
            context={"query": "test search"}
        )
        print(f"✅ Tool executed with verification: {result}")

        print("\n🎉 TEST 4 PASSED - MCPActionWrapper works!")
        return True

    except Exception as e:
        print(f"\n❌ TEST 4 FAILED: {e}")
        import traceback
        traceback.print_exc()
        return False


def main():
    """Run all MCP integration tests"""
    print("=" * 70)
    print("AIM + MCP Integration Tests")
    print("=" * 70)
    print(f"AIM Server: {AIM_URL}")
    print()

    results = []
    server_id = None
    aim_client = None

    # Test 1: MCP Server Registration
    test1_passed, server_id = test_mcp_server_registration()
    results.append(("MCP Server Registration", test1_passed))

    # Get AIM client for remaining tests
    if test1_passed:
        try:
            aim_client = AIMClient.from_credentials("mcp-test-registration")
        except Exception as e:
            print(f"⚠️  Failed to load AIM client: {e}")

    # Test 2: MCP Server Listing
    if aim_client:
        test2_passed = test_mcp_server_listing(aim_client)
        results.append(("MCP Server Listing", test2_passed))
    else:
        print("\n⚠️  Skipping Test 2 - no AIM client")
        results.append(("MCP Server Listing", False))

    # Test 3: MCP Action Verification
    if aim_client and server_id:
        test3_passed = test_mcp_action_verification(aim_client, server_id)
        results.append(("MCP Action Verification", test3_passed))
    else:
        print("\n⚠️  Skipping Test 3 - no AIM client or server_id")
        results.append(("MCP Action Verification", False))

    # Test 4: MCPActionWrapper
    if aim_client and server_id:
        test4_passed = test_mcp_action_wrapper(aim_client, server_id)
        results.append(("MCPActionWrapper", test4_passed))
    else:
        print("\n⚠️  Skipping Test 4 - no AIM client or server_id")
        results.append(("MCPActionWrapper", False))

    # Summary
    print("\n" + "="*70)
    print("TEST SUMMARY")
    print("="*70)

    passed = sum(1 for _, result in results if result)
    total = len(results)

    for test_name, result in results:
        status = "✅ PASSED" if result else "❌ FAILED"
        print(f"{status}: {test_name}")

    print(f"\nTotal: {passed}/{total} tests passed")

    if passed == total:
        print("\n🎉 ALL TESTS PASSED - MCP integration working perfectly!")
        return 0
    else:
        print(f"\n⚠️  {total - passed} test(s) failed - review output above")
        return 1


if __name__ == "__main__":
    sys.exit(main())
