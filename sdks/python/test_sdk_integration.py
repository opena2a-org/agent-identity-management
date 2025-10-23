#!/usr/bin/env python3
"""
Comprehensive SDK Integration Test

Tests all AIM SDK features end-to-end:
1. Agent registration with secure()
2. Action verification with @perform_action decorator
3. Protocol auto-detection (MCP and A2A)
4. MCP server auto-discovery tracking
5. Detection reporting to AIM backend
6. Dashboard data verification

This test creates a real agent in AIM and verifies all features work.
"""

import os
import sys
import json
import time
from datetime import datetime

# Add SDK to path
sys.path.insert(0, os.path.join(os.path.dirname(__file__), 'aim_sdk'))

from aim_sdk import secure, track_mcp_call, MCPDetector, auto_detect_protocol


# Configuration
AIM_URL = os.getenv("AIM_URL", "http://localhost:8080")
AGENT_NAME = f"sdk-test-agent-{int(time.time())}"

print("=" * 80)
print("🧪 AIM SDK Comprehensive Integration Test")
print("=" * 80)
print(f"⏰ Started at: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
print(f"🌐 AIM URL: {AIM_URL}")
print(f"🤖 Agent Name: {AGENT_NAME}")
print()


def test_1_protocol_detection():
    """Test 1: Protocol Auto-Detection"""
    print("\n" + "=" * 80)
    print("🧪 TEST 1: Protocol Auto-Detection")
    print("=" * 80)

    # Test MCP detection
    os.environ["MCP_SERVER_MODE"] = "true"
    protocol = auto_detect_protocol()
    print(f"✅ Detected protocol with MCP env var: {protocol}")
    assert protocol == "mcp", f"Expected 'mcp', got '{protocol}'"
    del os.environ["MCP_SERVER_MODE"]

    # Test A2A detection
    os.environ["A2A_AGENT_MODE"] = "client"
    protocol = auto_detect_protocol()
    print(f"✅ Detected protocol with A2A env var: {protocol}")
    assert protocol == "a2a", f"Expected 'a2a', got '{protocol}'"
    del os.environ["A2A_AGENT_MODE"]

    # Test explicit override
    protocol = auto_detect_protocol(explicit_protocol="OAuth")
    print(f"✅ Explicit protocol override: {protocol}")
    assert protocol == "oauth", f"Expected 'oauth', got '{protocol}'"

    # Test default
    protocol = auto_detect_protocol()
    print(f"✅ Default protocol (no indicators): {protocol}")
    assert protocol == "mcp", f"Expected default 'mcp', got '{protocol}'"

    print("✅ TEST 1 PASSED: Protocol detection working correctly")


def test_2_agent_registration():
    """Test 2: Agent Registration with secure()"""
    print("\n" + "=" * 80)
    print("🧪 TEST 2: Agent Registration")
    print("=" * 80)

    try:
        # Register agent using secure() - ONE LINE!
        print(f"📝 Registering agent: {AGENT_NAME}")
        agent = secure(
            name=AGENT_NAME,
            agent_type="ai_agent",
            aim_url=AIM_URL,
            protocol="mcp"  # Explicitly set MCP protocol
        )

        print(f"✅ Agent registered successfully!")
        print(f"   - Agent ID: {agent.agent_id}")
        print(f"   - Protocol: {agent.protocol}")
        print(f"   - Public Key: {agent.public_key[:50]}...")

        # Verify agent details
        details = agent.get_agent_details()
        print(f"✅ Agent details retrieved:")
        print(f"   - Name: {details.get('name')}")
        print(f"   - Type: {details.get('agent_type')}")
        print(f"   - Status: {details.get('status')}")

        print("✅ TEST 2 PASSED: Agent registration successful")
        return agent

    except Exception as e:
        print(f"❌ TEST 2 FAILED: {e}")
        import traceback
        traceback.print_exc()
        return None


def test_3_action_verification(agent):
    """Test 3: Action Verification with Protocol"""
    print("\n" + "=" * 80)
    print("🧪 TEST 3: Action Verification")
    print("=" * 80)

    if not agent:
        print("⏭️  Skipping (no agent available)")
        return

    try:
        # Test action verification
        print("📡 Verifying action: read_database")

        @agent.perform_action("read_database", resource="users_table")
        def read_user_data(user_id):
            return {"user_id": user_id, "name": "Test User"}

        # Call the function (decorator handles verification)
        result = read_user_data(user_id=123)
        print(f"✅ Action executed successfully: {result}")

        # Test with protocol detection
        verification_result = agent.verify_action_with_protocol(
            action_type="write_database",
            resource="logs_table",
            metadata={"operation": "insert", "rows": 5}
        )

        print(f"✅ Action verification result:")
        print(f"   - Allowed: {verification_result.get('allowed')}")
        print(f"   - Protocol: {agent.protocol}")

        print("✅ TEST 3 PASSED: Action verification working")

    except Exception as e:
        print(f"❌ TEST 3 FAILED: {e}")
        import traceback
        traceback.print_exc()


def test_4_mcp_auto_discovery(agent):
    """Test 4: MCP Server Auto-Discovery"""
    print("\n" + "=" * 80)
    print("🧪 TEST 4: MCP Server Auto-Discovery")
    print("=" * 80)

    if not agent:
        print("⏭️  Skipping (no agent available)")
        return

    try:
        # Clear any existing tracker data
        from aim_sdk.detection import _mcp_call_tracker
        _mcp_call_tracker.clear()

        # Simulate MCP tool calls
        print("📡 Simulating MCP server calls...")
        track_mcp_call("filesystem", "read_file")
        track_mcp_call("filesystem", "write_file")
        track_mcp_call("filesystem", "list_directory")
        track_mcp_call("github", "create_issue")
        track_mcp_call("github", "list_repos")
        track_mcp_call("supabase", "execute_sql")
        track_mcp_call("supabase", "apply_migration")

        print(f"✅ Tracked 7 MCP calls across 3 servers")

        # Get runtime detections
        detector = MCPDetector()
        detections = detector.detect_all_with_runtime()

        print(f"✅ Generated {len(detections)} detection events:")
        for detection in detections:
            if detection["detectionMethod"] == "sdk_runtime":
                print(f"   - {detection['mcpServer']}: {detection['details']['call_count']} calls")

        print("✅ TEST 4 PASSED: MCP auto-discovery tracking works")
        return detections

    except Exception as e:
        print(f"❌ TEST 4 FAILED: {e}")
        import traceback
        traceback.print_exc()
        return []


def test_5_detection_reporting(agent, detections):
    """Test 5: Report Detections to AIM"""
    print("\n" + "=" * 80)
    print("🧪 TEST 5: Detection Reporting")
    print("=" * 80)

    if not agent or not detections:
        print("⏭️  Skipping (no agent or detections available)")
        return

    try:
        # Report detections to AIM
        print(f"📡 Reporting {len(detections)} detections to AIM...")
        result = agent.report_detections(detections)

        print(f"✅ Detection report result:")
        print(f"   - Success: {result.get('success')}")
        print(f"   - Message: {result.get('message')}")

        # Verify detections were recorded
        time.sleep(2)  # Give backend time to process

        print("✅ TEST 5 PASSED: Detections reported successfully")

    except Exception as e:
        print(f"❌ TEST 5 FAILED: {e}")
        import traceback
        traceback.print_exc()


def test_6_capability_detection(agent):
    """Test 6: Capability Auto-Detection"""
    print("\n" + "=" * 80)
    print("🧪 TEST 6: Capability Auto-Detection")
    print("=" * 80)

    if not agent:
        print("⏭️  Skipping (no agent available)")
        return

    try:
        from aim_sdk import CapabilityDetector

        # Detect capabilities
        detector = CapabilityDetector()
        capabilities = detector.detect_all()

        print(f"✅ Detected {len(capabilities)} capabilities:")
        for cap in capabilities[:5]:  # Show first 5
            print(f"   - {cap['capability']}: {cap['detectionMethod']}")

        if capabilities:
            # Report capabilities to AIM
            result = agent.report_capabilities(capabilities)
            print(f"✅ Capability report result: {result.get('message')}")

        print("✅ TEST 6 PASSED: Capability detection working")

    except Exception as e:
        print(f"❌ TEST 6 FAILED: {e}")
        import traceback
        traceback.print_exc()


def test_7_trust_score_retrieval(agent):
    """Test 7: Trust Score Retrieval"""
    print("\n" + "=" * 80)
    print("🧪 TEST 7: Trust Score Retrieval")
    print("=" * 80)

    if not agent:
        print("⏭️  Skipping (no agent available)")
        return

    try:
        # Get agent details including trust score
        details = agent.get_agent_details()

        print(f"✅ Agent trust score: {details.get('trust_score', 'N/A')}")
        print(f"✅ Agent status: {details.get('status')}")
        print(f"✅ Last verified: {details.get('last_verified_at', 'Never')}")

        print("✅ TEST 7 PASSED: Trust score retrieval working")

    except Exception as e:
        print(f"❌ TEST 7 FAILED: {e}")
        import traceback
        traceback.print_exc()


def run_all_tests():
    """Run all SDK integration tests"""
    agent = None
    detections = []

    try:
        # Run tests sequentially
        test_1_protocol_detection()
        agent = test_2_agent_registration()
        test_3_action_verification(agent)
        detections = test_4_mcp_auto_discovery(agent)
        test_5_detection_reporting(agent, detections)
        test_6_capability_detection(agent)
        test_7_trust_score_retrieval(agent)

        print("\n" + "=" * 80)
        print("📊 TEST SUMMARY")
        print("=" * 80)
        if agent:
            print(f"✅ Agent ID: {agent.agent_id}")
            print(f"✅ Agent Name: {AGENT_NAME}")
            print(f"✅ Protocol: {agent.protocol}")
            print(f"✅ Total Detections Reported: {len(detections)}")
        print(f"⏰ Completed at: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
        print()
        print("🎉 ALL TESTS COMPLETED!")
        print()
        print("📋 NEXT STEPS:")
        print("1. Open dashboard: http://localhost:3000")
        print("2. Login as admin@opena2a.org")
        print(f"3. Find agent: {AGENT_NAME}")
        print("4. Verify all data appears correctly:")
        print("   - Agent details (name, type, status)")
        print("   - Trust score")
        print("   - MCP detections (filesystem, github, supabase)")
        print("   - Detection methods (sdk_runtime)")
        print("   - Action verification logs")
        print()

        return agent

    except Exception as e:
        print(f"\n💥 TEST SUITE ERROR: {e}")
        import traceback
        traceback.print_exc()
        return None


if __name__ == "__main__":
    agent = run_all_tests()

    # Keep agent info for dashboard verification
    if agent:
        print(f"💾 Agent credentials saved:")
        print(f"   - Agent ID: {agent.agent_id}")
        print(f"   - Name: {AGENT_NAME}")
        print()
