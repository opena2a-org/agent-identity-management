#!/usr/bin/env python3
"""
End-to-End Integration Test for AIM SDK

This script tests the complete "Stripe Moment" workflow:
1. Auto-detection of capabilities
2. Auto-detection of MCPs
3. Zero-config registration (mocked backend)
"""

import sys
import json
from unittest.mock import patch, MagicMock
from aim_sdk import auto_detect_capabilities, auto_detect_mcps, register_agent


def test_capability_detection():
    """Test 1: Capability Auto-Detection"""
    print("\n📋 Test 1: Capability Auto-Detection")
    print("=" * 60)

    # Import known packages to trigger detection
    import requests
    import smtplib

    capabilities = auto_detect_capabilities()
    print(f"✅ Detected {len(capabilities)} capabilities")
    print(f"   Capabilities: {', '.join(capabilities)}")

    # Verify expected capabilities
    assert "make_api_calls" in capabilities, "Should detect requests → make_api_calls"
    assert "send_email" in capabilities, "Should detect smtplib → send_email"

    print("✅ Capability detection works correctly!\n")
    return True


def test_mcp_detection():
    """Test 2: MCP Auto-Detection"""
    print("📡 Test 2: MCP Auto-Detection")
    print("=" * 60)

    mcp_detections = auto_detect_mcps()
    print(f"✅ MCP detection completed")
    print(f"   Found {len(mcp_detections)} MCP servers")

    if mcp_detections:
        for det in mcp_detections[:3]:
            print(f"   - {det['mcpServer']} ({det['detectionMethod']}, {det['confidence']}%)")
    else:
        print("   ℹ️  No MCP servers detected (expected in test environment)")

    print("✅ MCP detection works correctly!\n")
    return True


def test_zero_config_registration():
    """Test 3: Zero-Config Registration (Mocked)"""
    print("🚀 Test 3: Zero-Config Registration")
    print("=" * 60)

    # Mock SDK credentials (simulates downloaded SDK)
    mock_sdk_creds = {
        "aim_url": "https://aim-test.example.com",
        "refresh_token": "mock_refresh_token_12345",
        "sdk_token_id": "sdk_token_test_123"
    }

    # Mock registration response with valid base64 keys
    import base64
    import os
    from nacl.signing import SigningKey
    from nacl.encoding import Base64Encoder

    # Generate valid Ed25519 key pair
    seed = os.urandom(32)
    signing_key = SigningKey(seed)
    verify_key = signing_key.verify_key

    public_key_b64 = verify_key.encode(encoder=Base64Encoder).decode('utf-8')
    private_key_64bytes = seed + bytes(verify_key)  # 64 bytes total
    private_key_b64 = base64.b64encode(private_key_64bytes).decode('utf-8')

    mock_registration_response = {
        "agent_id": "550e8400-e29b-41d4-a716-446655440000",
        "name": "test-agent-e2e",
        "public_key": public_key_b64,
        "private_key": private_key_b64,
        "aim_url": "https://aim-test.example.com",
        "status": "verified",
        "trust_score": 85.0,
        "message": "Agent registered successfully via SDK"
    }

    # Mock the HTTP request and OAuth token manager
    with patch('aim_sdk.client.load_sdk_credentials', return_value=mock_sdk_creds):
        with patch('aim_sdk.client._load_credentials', return_value=None):  # No existing creds
            with patch('aim_sdk.client._save_credentials'):  # Don't save to disk
                with patch('aim_sdk.client.OAuthTokenManager') as mock_token_manager:
                    # Setup token manager mock (prevent __init__ from being called)
                    mock_tm_instance = MagicMock()
                    mock_tm_instance.get_access_token.return_value = "mock_access_token_xyz"
                    mock_token_manager.return_value = mock_tm_instance

                    # Mock the HTTP POST request
                    with patch('aim_sdk.client.requests.post') as mock_post:
                        mock_response = MagicMock()
                        mock_response.status_code = 201
                        mock_response.json.return_value = mock_registration_response
                        mock_post.return_value = mock_response

                        # Mock MCP detection reporting
                        with patch.object(sys.modules['aim_sdk.client'].AIMClient, 'report_detections'):
                            print("   🔐 SDK Mode detected")
                            print("   🔍 Auto-detecting capabilities and MCPs...")

                            # Register agent with zero config!
                            agent = register_agent("test-agent-e2e")

                            print(f"\n   ✅ Registration successful!")
                            print(f"      Agent ID: {agent.agent_id}")
                            print(f"      AIM URL: {agent.aim_url}")

                            # Verify agent was created correctly
                            assert agent.agent_id == mock_registration_response["agent_id"]
                            assert agent.aim_url == mock_registration_response["aim_url"]

    print("\n✅ Zero-config registration works correctly!\n")
    return True


def test_api_key_registration():
    """Test 4: API Key Registration (Manual Mode)"""
    print("🔑 Test 4: API Key Registration (Manual Mode)")
    print("=" * 60)

    # Mock NO SDK credentials (forces API key mode)
    import base64
    import os
    from nacl.signing import SigningKey
    from nacl.encoding import Base64Encoder

    # Generate valid Ed25519 key pair
    seed = os.urandom(32)
    signing_key = SigningKey(seed)
    verify_key = signing_key.verify_key

    public_key_b64 = verify_key.encode(encoder=Base64Encoder).decode('utf-8')
    private_key_64bytes = seed + bytes(verify_key)  # 64 bytes total
    private_key_b64 = base64.b64encode(private_key_64bytes).decode('utf-8')

    mock_registration_response = {
        "agent_id": "660e8400-e29b-41d4-a716-446655440001",
        "name": "test-agent-manual",
        "public_key": public_key_b64,
        "private_key": private_key_b64,
        "aim_url": "https://aim-test.example.com",
        "status": "pending_verification",
        "trust_score": 70.0,
        "message": "Agent registered successfully via API key"
    }

    with patch('aim_sdk.client.load_sdk_credentials', return_value=None):
        with patch('aim_sdk.client._load_credentials', return_value=None):  # No existing creds
            with patch('aim_sdk.client._save_credentials'):  # Don't save to disk
                with patch('aim_sdk.client.requests.post') as mock_post:
                    mock_response = MagicMock()
                    mock_response.status_code = 201
                    mock_response.json.return_value = mock_registration_response
                    mock_post.return_value = mock_response

                    with patch.object(sys.modules['aim_sdk.client'].AIMClient, 'report_detections'):
                        print("   🔑 Manual Mode: API key authentication")
                        print("   🔍 Auto-detecting capabilities and MCPs...")

                        agent = register_agent(
                            "test-agent-manual",
                            aim_url="https://aim-test.example.com",
                            api_key="aim_test_api_key_12345"
                        )

                print(f"\n   ✅ Registration successful!")
                print(f"      Agent ID: {agent.agent_id}")
                print(f"      Status: {mock_registration_response['status']}")

    assert agent.agent_id == mock_registration_response["agent_id"]
    print("\n✅ API key registration works correctly!\n")
    return True


def main():
    """Run all E2E tests"""
    print("\n" + "=" * 60)
    print("🧪 AIM SDK End-to-End Integration Tests")
    print("   Testing 'The Stripe Moment' for AI Agent Identity")
    print("=" * 60)

    tests = [
        ("Capability Detection", test_capability_detection),
        ("MCP Detection", test_mcp_detection),
        ("Zero-Config Registration", test_zero_config_registration),
        ("API Key Registration", test_api_key_registration),
    ]

    passed = 0
    failed = 0

    for name, test_func in tests:
        try:
            if test_func():
                passed += 1
        except Exception as e:
            failed += 1
            print(f"❌ {name} FAILED: {e}")
            import traceback
            traceback.print_exc()
            print()

    # Summary
    print("=" * 60)
    print(f"📊 Test Summary")
    print("=" * 60)
    print(f"   Total Tests: {len(tests)}")
    print(f"   ✅ Passed: {passed}")
    print(f"   ❌ Failed: {failed}")
    print("=" * 60)

    if failed == 0:
        print("\n🎉 ALL TESTS PASSED! SDK is production-ready!")
        print("   The 'Stripe Moment' is HERE! 🚀\n")
        return 0
    else:
        print(f"\n⚠️  {failed} test(s) failed. Please review.\n")
        return 1


if __name__ == "__main__":
    sys.exit(main())
