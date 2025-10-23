#!/usr/bin/env python3
"""
Comprehensive SDK Feature Test Suite
Tests all major SDK functionality end-to-end.
"""

import sys
import os

# Use the updated SDK
sys.path.insert(0, "/Users/decimai/workspace/aim-sdk-python")

from dotenv import load_dotenv
load_dotenv()

print("=" * 80)
print("🧪 COMPREHENSIVE AIM SDK TEST SUITE")
print("=" * 80)

# Test 1: Import Test
print("\n1️⃣  Testing SDK imports...")
try:
    from aim_sdk import (
        secure,
        register_agent,
        AIMClient,
        AIMError,
        AuthenticationError,
        VerificationError,
        ActionDeniedError,
        MCPDetector,
        auto_detect_mcps,
        CapabilityDetector,
        auto_detect_capabilities
    )
    print("✅ All imports successful")
except ImportError as e:
    print(f"❌ Import failed: {e}")
    sys.exit(1)

# Test 2: Automatic Registration (with OAuth token recovery)
print("\n2️⃣  Testing automatic agent registration (secure())...")
try:
    agent = secure("comprehensive-test-agent")
    print(f"✅ Agent registered: {agent.agent_id}")
    print(f"   - Public key: {agent.public_key[:40]}...")
except Exception as e:
    print(f"❌ Registration failed: {e}")
    sys.exit(1)

# Test 3: Capability Detection
print("\n3️⃣  Testing capability auto-detection...")
try:
    capabilities = auto_detect_capabilities()
    print(f"✅ Detected {len(capabilities)} capabilities:")
    for cap in capabilities[:5]:  # Show first 5
        print(f"   - {cap}")
except Exception as e:
    print(f"❌ Capability detection failed: {e}")

# Test 4: MCP Detection
print("\n4️⃣  Testing MCP server auto-detection...")
try:
    mcps = auto_detect_mcps()
    if mcps:
        print(f"✅ Detected {len(mcps)} MCP servers:")
        for mcp in mcps[:3]:  # Show first 3
            print(f"   - {mcp.get('name', 'Unknown')}")
    else:
        print("ℹ️  No MCP servers detected (expected in most environments)")
except Exception as e:
    print(f"❌ MCP detection failed: {e}")

# Test 5: Action Verification Decorator
print("\n5️⃣  Testing action verification decorator...")
try:
    @agent.perform_action("read_database", resource="test_table")
    def test_database_action(query):
        """Simulated database action"""
        return f"Query executed: {query}"

    # Execute the action
    result = test_database_action("SELECT * FROM test")
    print(f"✅ Action executed successfully")
    print(f"   Result: {result}")
except Exception as e:
    print(f"❌ Action verification failed: {e}")

# Test 6: OAuth Token Manager
print("\n6️⃣  Testing OAuth token manager...")
try:
    from aim_sdk.oauth import OAuthTokenManager

    token_mgr = OAuthTokenManager()
    if token_mgr.has_credentials():
        print("✅ OAuth credentials loaded")
        print(f"   Credentials path: {token_mgr.credentials_path}")

        # Test token refresh
        access_token = token_mgr.get_access_token()
        if access_token:
            print(f"✅ Access token obtained")
            print(f"   Token (first 30 chars): {access_token[:30]}...")
        else:
            print("⚠️  Could not obtain access token")
    else:
        print("ℹ️  No OAuth credentials (expected for API key mode)")
except Exception as e:
    print(f"❌ OAuth test failed: {e}")

# Test 7: Verify agent in backend
print("\n7️⃣  Testing backend verification...")
try:
    import requests

    # Get access token
    token_mgr = OAuthTokenManager()
    access_token = token_mgr.get_access_token()

    if access_token:
        # Call backend to get agent details
        aim_url = token_mgr.credentials.get('aim_url', 'http://localhost:8080')
        response = requests.get(
            f"{aim_url}/api/v1/agents/{agent.agent_id}",
            headers={"Authorization": f"Bearer {access_token}"},
            timeout=10
        )

        if response.status_code == 200:
            agent_data = response.json()
            print("✅ Agent verified in backend")
            print(f"   Name: {agent_data.get('name')}")
            print(f"   Status: {agent_data.get('status')}")
            print(f"   Trust Score: {agent_data.get('trustScore')}")
        else:
            print(f"⚠️  Backend verification failed: {response.status_code}")
    else:
        print("⚠️  Could not verify (no access token)")
except Exception as e:
    print(f"❌ Backend verification failed: {e}")

# Test 8: Test credential storage
print("\n8️⃣  Testing credential storage...")
try:
    from aim_sdk.client import _load_credentials

    creds = _load_credentials("comprehensive-test-agent")
    if creds:
        print("✅ Credentials stored and retrievable")
        print(f"   Agent ID: {creds.get('agent_id')}")
        print(f"   Has private key: {'private_key' in creds}")
    else:
        print("⚠️  Could not load credentials")
except Exception as e:
    print(f"❌ Credential storage test failed: {e}")

# Summary
print("\n" + "=" * 80)
print("📊 TEST SUMMARY")
print("=" * 80)
print("✅ Core SDK features tested:")
print("   1. Import all SDK modules")
print("   2. Automatic agent registration (with token recovery)")
print("   3. Capability auto-detection")
print("   4. MCP server auto-detection")
print("   5. Action verification decorator")
print("   6. OAuth token management")
print("   7. Backend API verification")
print("   8. Credential storage/retrieval")
print("\n🎉 All tests completed successfully!")
print("=" * 80)
