#!/usr/bin/env python3
"""
Integration tests for AIM MCP Call Interception

Tests all three approaches for automatic MCP tool call detection and verification:
1. Decorator-based (@aim_mcp_tool)
2. Context manager (with aim_mcp_session)
3. Protocol interceptor (MCPProtocolInterceptor)

Run this test to validate automatic MCP tool call interception and verification.
"""

import sys
import os
from pathlib import Path

# Add SDK to path
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "aim_sdk"))

from aim_sdk import AIMClient
from aim_sdk.integrations.mcp import (
    register_mcp_server,
    aim_mcp_tool,
    aim_mcp_session,
    MCPProtocolInterceptor
)

AIM_URL = "http://localhost:8080"


# ============================================================================
# MOCK MCP CLIENT (for testing without real MCP server)
# ============================================================================

class MockMCPClient:
    """Mock MCP client for testing call interception"""

    def __init__(self, name="mock-mcp"):
        self.name = name
        self.calls = []

    def call_tool(self, tool_name: str, arguments: dict) -> dict:
        """Mock MCP tool call"""
        self.calls.append({"type": "tool", "name": tool_name, "args": arguments})
        return {
            "status": "success",
            "tool": tool_name,
            "result": f"Mock result for {tool_name}",
            "data": arguments
        }

    def read_resource(self, resource_uri: str) -> dict:
        """Mock MCP resource read"""
        self.calls.append({"type": "resource", "uri": resource_uri})
        return {
            "status": "success",
            "uri": resource_uri,
            "content": f"Mock content for {resource_uri}"
        }

    def get_prompt(self, prompt_name: str, arguments: dict) -> dict:
        """Mock MCP prompt get"""
        self.calls.append({"type": "prompt", "name": prompt_name, "args": arguments})
        return {
            "status": "success",
            "prompt": prompt_name,
            "messages": [{"role": "user", "content": f"Mock prompt: {prompt_name}"}]
        }


# ============================================================================
# TEST 1: DECORATOR-BASED AUTO-DETECTION
# ============================================================================

def test_decorator_based_detection():
    """Test 1: Decorator-based auto-detection with @aim_mcp_tool"""
    print("\n" + "="*70)
    print("TEST 1: Decorator-Based Auto-Detection (@aim_mcp_tool)")
    print("="*70)

    try:
        # Setup
        aim_client = AIMClient.auto_register_or_load(
            "mcp-auto-detect-decorator",
            AIM_URL
        )
        print(f"✅ AIM agent registered: {aim_client.agent_id}")

        # Register MCP server
        server_info = register_mcp_server(
            aim_client=aim_client,
            server_name="test-decorator-mcp",
            server_url="http://localhost:3000",
            public_key="ed25519_test_decorator_key_1234567890abcdef1234567890abcdef",
            capabilities=["tools"],
            description="Test MCP server for decorator-based auto-detection"
        )
        print(f"✅ MCP server registered: {server_info['id']}")

        # Create mock MCP client
        mcp_client = MockMCPClient("test-decorator-mcp")

        # Decorate MCP tool functions
        @aim_mcp_tool(
            aim_client=aim_client,
            mcp_server_id=server_info["id"],
            risk_level="low",
            verbose=True
        )
        def web_search(query: str) -> dict:
            """Search the web via MCP (automatically verified by AIM)"""
            return mcp_client.call_tool("web_search", {"query": query})

        @aim_mcp_tool(
            aim_client=aim_client,
            mcp_server_id=server_info["id"],
            risk_level="medium",
            verbose=True
        )
        def database_query(sql: str) -> dict:
            """Execute database query via MCP (automatically verified by AIM)"""
            return mcp_client.call_tool("database_query", {"sql": sql})

        # Execute tools - verification happens automatically
        print("\n--- Executing web_search (auto-verified) ---")
        search_result = web_search("AI safety best practices")
        print(f"✅ Search result: {search_result}")

        print("\n--- Executing database_query (auto-verified) ---")
        query_result = database_query("SELECT * FROM users LIMIT 5")
        print(f"✅ Query result: {query_result}")

        # Verify MCP client received calls
        print(f"\n✅ Mock MCP client received {len(mcp_client.calls)} calls:")
        for call in mcp_client.calls:
            print(f"   - {call['type']}: {call.get('name') or call.get('uri')}")

        print("\n🎉 TEST 1 PASSED - Decorator-based auto-detection works!")
        return True

    except Exception as e:
        print(f"\n❌ TEST 1 FAILED: {e}")
        import traceback
        traceback.print_exc()
        return False


# ============================================================================
# TEST 2: CONTEXT MANAGER AUTO-DETECTION
# ============================================================================

def test_context_manager_detection():
    """Test 2: Context manager auto-detection with aim_mcp_session"""
    print("\n" + "="*70)
    print("TEST 2: Context Manager Auto-Detection (aim_mcp_session)")
    print("="*70)

    try:
        # Setup
        aim_client = AIMClient.auto_register_or_load(
            "mcp-auto-detect-session",
            AIM_URL
        )
        print(f"✅ AIM agent registered: {aim_client.agent_id}")

        # Register MCP server
        server_info = register_mcp_server(
            aim_client=aim_client,
            server_name="test-session-mcp",
            server_url="http://localhost:3001",
            public_key="ed25519_test_session_key_1234567890abcdef1234567890abcdef",
            capabilities=["tools", "resources"],
            description="Test MCP server for session-based auto-detection"
        )
        print(f"✅ MCP server registered: {server_info['id']}")

        # Create mock MCP client
        mcp_client = MockMCPClient("test-session-mcp")

        # Use context manager for automatic server context
        print("\n--- Starting MCP session ---")
        with aim_mcp_session(
            aim_client=aim_client,
            mcp_server_id=server_info["id"],
            session_name="research_pipeline",
            verbose=True
        ) as session:
            # Tools defined inside session automatically use session's MCP server
            @aim_mcp_tool(risk_level="low", verbose=True)
            def search_papers(topic: str) -> dict:
                """Search academic papers (uses session's MCP server)"""
                return mcp_client.call_tool("search_papers", {"topic": topic})

            @aim_mcp_tool(risk_level="medium", verbose=True)
            def analyze_papers(papers: list) -> dict:
                """Analyze papers (uses session's MCP server)"""
                return mcp_client.call_tool("analyze_papers", {"papers": papers})

            # Execute tools - server context from session
            print("\n--- Executing search_papers (session context) ---")
            papers = search_papers("quantum computing")
            session.log(f"Found papers: {papers['result']}")

            print("\n--- Executing analyze_papers (session context) ---")
            analysis = analyze_papers(["paper1", "paper2"])
            session.log(f"Generated analysis: {analysis['result']}")

            # Get session stats
            stats = session.get_stats()
            print(f"\n✅ Session stats: {stats['total_calls']} calls, "
                  f"{stats['successful_calls']} successful")

        print("\n✅ MCP session completed")
        print(f"✅ Mock MCP client received {len(mcp_client.calls)} calls")

        print("\n🎉 TEST 2 PASSED - Context manager auto-detection works!")
        return True

    except Exception as e:
        print(f"\n❌ TEST 2 FAILED: {e}")
        import traceback
        traceback.print_exc()
        return False


# ============================================================================
# TEST 3: PROTOCOL INTERCEPTOR AUTO-DETECTION
# ============================================================================

def test_protocol_interceptor_detection():
    """Test 3: Protocol-level interceptor with MCPProtocolInterceptor"""
    print("\n" + "="*70)
    print("TEST 3: Protocol Interceptor Auto-Detection (MCPProtocolInterceptor)")
    print("="*70)

    try:
        # Setup
        aim_client = AIMClient.auto_register_or_load(
            "mcp-auto-detect-interceptor",
            AIM_URL
        )
        print(f"✅ AIM agent registered: {aim_client.agent_id}")

        # Register MCP server
        server_info = register_mcp_server(
            aim_client=aim_client,
            server_name="test-interceptor-mcp",
            server_url="http://localhost:3002",
            public_key="ed25519_test_interceptor_key_1234567890abcdef1234567890ab",
            capabilities=["tools", "resources", "prompts"],
            description="Test MCP server for protocol-level interception"
        )
        print(f"✅ MCP server registered: {server_info['id']}")

        # Create mock MCP client
        mcp_client = MockMCPClient("test-interceptor-mcp")

        # Wrap MCP client with interceptor
        verified_mcp = MCPProtocolInterceptor(
            mcp_client=mcp_client,
            aim_client=aim_client,
            mcp_server_id=server_info["id"],
            auto_verify=True,
            default_risk_level="medium",
            verbose=True
        )
        print("✅ MCP client wrapped with protocol interceptor")

        # Execute MCP calls through interceptor - automatic verification
        print("\n--- Calling tool: web_search (auto-verified) ---")
        search_result = verified_mcp.call_tool(
            "web_search",
            {"query": "AI safety"},
            risk_level="low"
        )
        print(f"✅ Search result: {search_result}")

        print("\n--- Reading resource: config.json (auto-verified) ---")
        config_data = verified_mcp.read_resource(
            "config.json",
            risk_level="low"
        )
        print(f"✅ Config data: {config_data}")

        print("\n--- Getting prompt: system_prompt (auto-verified) ---")
        prompt_data = verified_mcp.get_prompt(
            "system_prompt",
            {"role": "assistant"},
            risk_level="low"
        )
        print(f"✅ Prompt data: {prompt_data}")

        # Test selective verification
        print("\n--- Testing selective verification ---")
        # Low-risk call without verification
        unverified_result = verified_mcp.call_tool(
            "read_status",
            {},
            verify=False  # Skip verification
        )
        print(f"✅ Unverified call result: {unverified_result}")

        # Get interceptor stats
        stats = verified_mcp.get_stats()
        print(f"\n✅ Interceptor stats:")
        print(f"   Total calls: {stats['total_calls']}")
        print(f"   Verified calls: {stats['verified_calls']}")
        print(f"   Unverified calls: {stats['unverified_calls']}")

        print(f"\n✅ Mock MCP client received {len(mcp_client.calls)} calls")

        print("\n🎉 TEST 3 PASSED - Protocol interceptor auto-detection works!")
        return True

    except Exception as e:
        print(f"\n❌ TEST 3 FAILED: {e}")
        import traceback
        traceback.print_exc()
        return False


# ============================================================================
# TEST 4: GRACEFUL FALLBACK (no AIM configured)
# ============================================================================

def test_graceful_fallback():
    """Test 4: Graceful fallback when AIM is not configured"""
    print("\n" + "="*70)
    print("TEST 4: Graceful Fallback (no AIM configured)")
    print("="*70)

    try:
        # Create mock MCP client
        mcp_client = MockMCPClient("test-fallback-mcp")

        # Decorator with graceful fallback (no AIM client)
        @aim_mcp_tool(
            graceful_fallback=True,
            verbose=True
        )
        def read_file(path: str) -> dict:
            """Read file via MCP (runs without verification if AIM not configured)"""
            return mcp_client.call_tool("read_file", {"path": path})

        # Execute - should run without verification
        print("\n--- Executing read_file (no AIM, graceful fallback) ---")
        result = read_file("/tmp/test.txt")
        print(f"✅ File read result: {result}")

        print(f"\n✅ Mock MCP client received {len(mcp_client.calls)} calls (without AIM verification)")

        print("\n🎉 TEST 4 PASSED - Graceful fallback works!")
        return True

    except Exception as e:
        print(f"\n❌ TEST 4 FAILED: {e}")
        import traceback
        traceback.print_exc()
        return False


# ============================================================================
# MAIN TEST RUNNER
# ============================================================================

def main():
    """Run all MCP call interception tests"""
    print("=" * 70)
    print("AIM MCP Call Interception Tests")
    print("=" * 70)
    print(f"AIM Server: {AIM_URL}")
    print()

    results = []

    # Test 1: Decorator-based
    test1_passed = test_decorator_based_detection()
    results.append(("Decorator-Based Auto-Detection", test1_passed))

    # Test 2: Context manager
    test2_passed = test_context_manager_detection()
    results.append(("Context Manager Auto-Detection", test2_passed))

    # Test 3: Protocol interceptor
    test3_passed = test_protocol_interceptor_detection()
    results.append(("Protocol Interceptor Auto-Detection", test3_passed))

    # Test 4: Graceful fallback
    test4_passed = test_graceful_fallback()
    results.append(("Graceful Fallback", test4_passed))

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
        print("\n🎉 ALL TESTS PASSED - MCP call interception working perfectly!")
        return 0
    else:
        print(f"\n⚠️  {total - passed} test(s) failed - review output above")
        return 1


if __name__ == "__main__":
    sys.exit(main())
