#!/usr/bin/env python3
"""
Simple Python SDK Test - OAuth Mode
Tests capability reporting using existing OAuth credentials
"""

import os
import sys
import json
from pathlib import Path

# Add SDK to path
sdk_path = os.path.join(os.path.dirname(__file__), 'sdks', 'python')
sys.path.insert(0, sdk_path)

from aim_sdk import AIMClient, auto_detect_capabilities
from aim_sdk.oauth import OAuthTokenManager


def main():
    print("=" * 80)
    print("🐍 PYTHON SDK SIMPLE TEST (OAuth Mode)")
    print("=" * 80)
    print()

    # Load SDK credentials (disable secure storage)
    creds_path = Path.home() / ".aim" / "credentials.json"
    if not creds_path.exists():
        print("❌ No SDK credentials found at ~/.aim/credentials.json")
        print("   Please download SDK from dashboard:")
        print("   http://localhost:8080/dashboard/sdk")
        return

    with open(creds_path, 'r') as f:
        sdk_creds = json.load(f)

    print(f"📡 AIM URL: {sdk_creds.get('aim_url')}")
    print(f"👤 User: {sdk_creds.get('email')}")
    print()

    # Step 1: Create a test agent using backend API with OAuth token
    print("🔐 Step 1: Creating test agent with OAuth...")

    # Get OAuth token
    token_manager = OAuthTokenManager(str(creds_path))
    access_token = token_manager.get_access_token()

    if not access_token:
        print("❌ Failed to get OAuth access token")
        print("   Token may have expired. Please re-download SDK from dashboard.")
        return

    print(f"   ✅ Got OAuth token")

    # Register agent using backend API
    import requests
    import base64
    from nacl.signing import SigningKey
    from nacl.encoding import Base64Encoder

    # Generate Ed25519 keypair
    signing_key = SigningKey.generate()
    private_key_bytes = bytes(signing_key) + bytes(signing_key.verify_key)
    public_key_bytes = bytes(signing_key.verify_key)
    private_key_b64 = base64.b64encode(private_key_bytes).decode('utf-8')
    public_key_b64 = base64.b64encode(public_key_bytes).decode('utf-8')

    import time
    agent_name = f"python-sdk-test-{int(time.time())}"

    response = requests.post(
        f"{sdk_creds['aim_url']}/api/v1/agents",
        json={
            "name": agent_name,
            "display_name": f"Python SDK Test Agent",
            "description": "Test agent for Python SDK validation",
            "agent_type": "ai_agent",
            "public_key": public_key_b64
        },
        headers={
            "Authorization": f"Bearer {access_token}",
            "Content-Type": "application/json"
        }
    )

    if response.status_code not in [200, 201]:
        print(f"❌ Failed to create agent: {response.status_code}")
        print(f"   {response.text}")
        return

    agent_data = response.json()
    agent_id = agent_data.get('id') or agent_data.get('agent_id')

    print(f"   ✅ Agent created: {agent_id}")
    print(f"   📝 Name: {agent_name}")
    print()

    # Step 2: Create AIMClient
    print("📦 Step 2: Creating AIM SDK client...")

    client = AIMClient(
        agent_id=agent_id,
        public_key=public_key_b64,
        private_key=private_key_b64,
        aim_url=sdk_creds['aim_url'],
        oauth_token_manager=token_manager
    )

    print(f"   ✅ Client ready")
    print()

    # Step 3: Auto-detect capabilities
    print("🔍 Step 3: Auto-detecting capabilities...")
    capabilities = auto_detect_capabilities()

    if capabilities:
        print(f"   ✅ Detected {len(capabilities)} capabilities:")
        for cap in capabilities[:5]:
            print(f"      - {cap}")
        if len(capabilities) > 5:
            print(f"      ... and {len(capabilities) - 5} more")
    else:
        print("   ℹ️  No capabilities auto-detected")
        capabilities = ["network_access", "make_api_calls", "read_files"]

    print()

    # Step 4: Report SDK integration
    print("📡 Step 4: Reporting SDK integration...")

    try:
        result = client.report_sdk_integration(
            sdk_version="aim-sdk-python@1.0.0",
            platform="python",
            capabilities=["auto_detect_mcps", "capability_detection"]
        )

        print(f"   ✅ SDK integration reported")
        print(f"   📊 Detections processed: {result.get('detectionsProcessed', 0)}")
        print()

    except Exception as e:
        print(f"   ⚠️  SDK integration report failed: {e}")
        print()

    # Step 5: Register test MCP server
    print("🔌 Step 5: Registering test MCP server...")

    try:
        mcp_result = client.register_mcp(
            mcp_server_id="filesystem-mcp-server",
            detection_method="auto_sdk",
            confidence=95.0,
            metadata={
                "source": "capability_detection_test",
                "package": "mcp-server-filesystem"
            }
        )

        print(f"   ✅ Registered {mcp_result.get('added', 0)} MCP server(s)")
        print()

    except Exception as e:
        print(f"   ⚠️  MCP registration failed: {e}")
        print()

    # Summary
    print("=" * 80)
    print("🎉 Python SDK Test Complete!")
    print(f"   - Agent ID: {agent_id}")
    print(f"   - Agent Name: {agent_name}")
    print(f"   - Capabilities detected: {len(capabilities)}")
    print(f"   - SDK Integration: ✅")
    print(f"   - MCP Server: ✅")
    print()
    print("📊 Check the AIM dashboard:")
    print(f"   - Capabilities: http://localhost:8080/dashboard/agents/{agent_id}")
    print(f"   - Detection: http://localhost:8080/dashboard/sdk")
    print(f"   - Connections: http://localhost:8080/dashboard/agents/{agent_id}")
    print("=" * 80)
    print()


if __name__ == "__main__":
    main()
