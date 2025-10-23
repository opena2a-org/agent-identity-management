#!/usr/bin/env python3
"""
Test script for AIM SDK protocol auto-detection and MCP auto-discovery.

Tests:
1. Protocol detection with environment variables (MCP, A2A)
2. Protocol detection with explicit override
3. Default protocol detection (no indicators)
4. Runtime MCP call tracking
"""

import os
import sys
from datetime import datetime

# Add SDK to path
sys.path.insert(0, os.path.join(os.path.dirname(__file__), 'aim_sdk'))

from aim_sdk.protocol_detection import ProtocolDetector, auto_detect_protocol
from aim_sdk.detection import MCPDetector, track_mcp_call


def test_mcp_detection_from_env():
    """Test MCP protocol detection from environment variables."""
    print("\n🧪 Test 1: MCP Detection from Environment Variables")
    print("=" * 60)

    # Set MCP environment variable
    os.environ["MCP_SERVER_MODE"] = "true"

    detector = ProtocolDetector()
    protocol = detector.detect_protocol()
    confidence = detector.get_detection_confidence(protocol)
    details = detector.get_protocol_details(protocol)

    print(f"✅ Detected Protocol: {protocol}")
    print(f"✅ Confidence Score: {confidence}%")
    print(f"✅ Indicators Found: {len(details['indicators_found'])}")

    for indicator in details['indicators_found']:
        print(f"   - Type: {indicator['type']}, Indicator: {indicator['indicator']}")

    # Cleanup
    del os.environ["MCP_SERVER_MODE"]

    assert protocol == "mcp", f"Expected 'mcp', got '{protocol}'"
    assert confidence >= 90, f"Expected confidence >= 90%, got {confidence}%"
    print("✅ Test 1 PASSED\n")


def test_a2a_detection_from_env():
    """Test A2A protocol detection from environment variables."""
    print("\n🧪 Test 2: A2A Detection from Environment Variables")
    print("=" * 60)

    # Set A2A environment variable
    os.environ["A2A_AGENT_MODE"] = "client"

    detector = ProtocolDetector()
    protocol = detector.detect_protocol()
    confidence = detector.get_detection_confidence(protocol)
    details = detector.get_protocol_details(protocol)

    print(f"✅ Detected Protocol: {protocol}")
    print(f"✅ Confidence Score: {confidence}%")
    print(f"✅ Indicators Found: {len(details['indicators_found'])}")

    for indicator in details['indicators_found']:
        print(f"   - Type: {indicator['type']}, Indicator: {indicator['indicator']}")

    # Cleanup
    del os.environ["A2A_AGENT_MODE"]

    assert protocol == "a2a", f"Expected 'a2a', got '{protocol}'"
    assert confidence >= 90, f"Expected confidence >= 90%, got {confidence}%"
    print("✅ Test 2 PASSED\n")


def test_explicit_protocol_override():
    """Test explicit protocol override (highest priority)."""
    print("\n🧪 Test 3: Explicit Protocol Override")
    print("=" * 60)

    # Set MCP env var but override with OAuth
    os.environ["MCP_SERVER_MODE"] = "true"

    detector = ProtocolDetector()
    protocol = detector.detect_protocol(explicit_protocol="OAuth")

    print(f"✅ Detected Protocol: {protocol} (overridden)")
    print(f"✅ Environment had MCP indicator but explicit override took precedence")

    # Cleanup
    del os.environ["MCP_SERVER_MODE"]

    assert protocol == "oauth", f"Expected 'oauth', got '{protocol}'"
    print("✅ Test 3 PASSED\n")


def test_default_protocol():
    """Test default protocol when no indicators present."""
    print("\n🧪 Test 4: Default Protocol Detection")
    print("=" * 60)

    # Ensure no protocol env vars are set
    protocol_env_vars = [
        "MCP_SERVER_MODE", "MCP_SERVER_NAME", "A2A_AGENT_MODE",
        "OAUTH_CLIENT_ID", "SAML_IDP_URL", "DID_METHOD", "ACP_AGENT_ID"
    ]
    for var in protocol_env_vars:
        if var in os.environ:
            del os.environ[var]

    detector = ProtocolDetector()
    protocol = detector.detect_protocol()

    print(f"✅ Detected Protocol: {protocol} (default)")
    print(f"✅ No indicators found, defaulted to MCP (most common for AI agents)")

    assert protocol == "mcp", f"Expected default 'mcp', got '{protocol}'"
    print("✅ Test 4 PASSED\n")


def test_runtime_mcp_tracking():
    """Test runtime MCP call tracking."""
    print("\n🧪 Test 5: Runtime MCP Call Tracking")
    print("=" * 60)

    # Clear any existing tracker data
    from aim_sdk.detection import _mcp_call_tracker
    _mcp_call_tracker.clear()

    # Simulate MCP tool calls
    print("📡 Simulating MCP tool calls...")
    track_mcp_call("filesystem", "read_file")
    track_mcp_call("filesystem", "write_file")
    track_mcp_call("github", "create_issue")
    track_mcp_call("github", "list_repos")
    track_mcp_call("filesystem", "read_file")  # Duplicate call

    print(f"✅ Tracked 5 MCP calls across 2 servers")

    # Get runtime detections
    detections = MCPDetector.get_runtime_detections()

    print(f"✅ Generated {len(detections)} detection events")
    print()

    for detection in detections:
        server = detection["mcpServer"]
        method = detection["detectionMethod"]
        confidence = detection["confidence"]
        call_count = detection["details"]["call_count"]
        tools_used = detection["details"]["tools_used"]

        print(f"📊 Server: {server}")
        print(f"   - Detection Method: {method}")
        print(f"   - Confidence: {confidence}%")
        print(f"   - Total Calls: {call_count}")
        print(f"   - Tools Used: {', '.join(tools_used)}")
        print()

    # Assertions
    assert len(detections) == 2, f"Expected 2 detections, got {len(detections)}"

    filesystem_detection = next(d for d in detections if d["mcpServer"] == "filesystem")
    assert filesystem_detection["details"]["call_count"] == 3, "Expected 3 filesystem calls"
    assert len(filesystem_detection["details"]["tools_used"]) == 2, "Expected 2 unique tools"

    github_detection = next(d for d in detections if d["mcpServer"] == "github")
    assert github_detection["details"]["call_count"] == 2, "Expected 2 github calls"

    print("✅ Test 5 PASSED\n")


def test_combined_detection():
    """Test combined static + runtime detection."""
    print("\n🧪 Test 6: Combined Static + Runtime Detection")
    print("=" * 60)

    # Clear tracker
    from aim_sdk.detection import _mcp_call_tracker
    _mcp_call_tracker.clear()

    # Set up MCP environment
    os.environ["MCP_SERVER_NAME"] = "test-server"

    # Track some runtime calls
    track_mcp_call("supabase", "execute_sql")
    track_mcp_call("github", "create_pr")

    # Run combined detection
    detector = MCPDetector()
    all_detections = detector.detect_all_with_runtime()

    print(f"✅ Total Detections: {len(all_detections)}")
    print()

    detection_methods = {}
    for detection in all_detections:
        method = detection["detectionMethod"]
        detection_methods[method] = detection_methods.get(method, 0) + 1

    print("📊 Detection Methods Breakdown:")
    for method, count in detection_methods.items():
        print(f"   - {method}: {count} detection(s)")

    # Cleanup
    del os.environ["MCP_SERVER_NAME"]

    assert len(all_detections) >= 2, f"Expected at least 2 detections (runtime), got {len(all_detections)}"
    print("\n✅ Test 6 PASSED\n")


def test_convenience_function():
    """Test convenience function auto_detect_protocol()."""
    print("\n🧪 Test 7: Convenience Function")
    print("=" * 60)

    # Test with env var
    os.environ["MCP_SERVER_MODE"] = "true"

    protocol = auto_detect_protocol()
    print(f"✅ auto_detect_protocol() returned: {protocol}")

    # Test with explicit override
    protocol_override = auto_detect_protocol(explicit_protocol="SAML")
    print(f"✅ auto_detect_protocol(explicit_protocol='SAML') returned: {protocol_override}")

    # Cleanup
    del os.environ["MCP_SERVER_MODE"]

    assert protocol == "mcp", f"Expected 'mcp', got '{protocol}'"
    assert protocol_override == "saml", f"Expected 'saml', got '{protocol_override}'"
    print("✅ Test 7 PASSED\n")


def run_all_tests():
    """Run all protocol detection tests."""
    print("🚀 AIM SDK Protocol Auto-Detection Test Suite")
    print("=" * 60)
    print(f"⏰ Started at: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
    print()

    tests = [
        test_mcp_detection_from_env,
        test_a2a_detection_from_env,
        test_explicit_protocol_override,
        test_default_protocol,
        test_runtime_mcp_tracking,
        test_combined_detection,
        test_convenience_function
    ]

    passed = 0
    failed = 0

    for test_func in tests:
        try:
            test_func()
            passed += 1
        except AssertionError as e:
            print(f"❌ {test_func.__name__} FAILED: {e}\n")
            failed += 1
        except Exception as e:
            print(f"💥 {test_func.__name__} ERROR: {e}\n")
            failed += 1

    print("=" * 60)
    print(f"📊 Test Results: {passed} passed, {failed} failed")
    print(f"⏰ Completed at: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")

    if failed == 0:
        print("✅ ALL TESTS PASSED! 🎉")
        return 0
    else:
        print(f"❌ {failed} TEST(S) FAILED")
        return 1


if __name__ == "__main__":
    sys.exit(run_all_tests())
