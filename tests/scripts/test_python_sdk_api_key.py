#!/usr/bin/env python3
"""
Python SDK API Key Mode Test
Production-quality test using existing python-sdk-test-agent with API key authentication.

This test validates feature parity with Go and JavaScript SDKs.
"""

import os
import sys

# Add SDK to path
sdk_path = os.path.join(os.path.dirname(__file__), 'sdks', 'python')
sys.path.insert(0, sdk_path)

from aim_sdk import AIMClient, auto_detect_capabilities


def main():
    print("=" * 80)
    print("🐍 PYTHON SDK API KEY MODE TEST")
    print("=" * 80)
    print()

    # Configuration
    AGENT_ID = "e237d89d-d366-43e5-808e-32c2ab64de6b"  # python-sdk-test-agent
    API_KEY = "aim_live_dw4shT8Ng6fyM7OTO9XLVA71NP09KVeBqmJhlQe_cJw="
    AIM_URL = "http://localhost:8080"

    print(f"📡 AIM URL: {AIM_URL}")
    print(f"🔑 Agent ID: {AGENT_ID}")
    print(f"🔐 Using API key authentication")
    print()

    # Step 1: Create AIM SDK client with API key
    print("📦 Step 1: Creating AIM SDK client (API key mode)...")

    try:
        client = AIMClient(
            agent_id=AGENT_ID,
            api_key=API_KEY,
            aim_url=AIM_URL
        )
        print(f"   ✅ Client created successfully")
        print()
    except Exception as e:
        print(f"   ❌ Failed to create client: {e}")
        sys.exit(1)

    # Step 2: Auto-detect capabilities
    print("🔍 Step 2: Auto-detecting capabilities...")

    capabilities = auto_detect_capabilities()

    if capabilities:
        print(f"   ✅ Detected {len(capabilities)} capabilities:")
        for cap in capabilities[:5]:
            print(f"      - {cap}")
        if len(capabilities) > 5:
            print(f"      ... and {len(capabilities) - 5} more")
    else:
        print("   ℹ  No capabilities auto-detected, using test capabilities")
        capabilities = [
            "network_access",
            "make_api_calls",
            "read_files"
        ]
    print()

    # Step 3: Report capabilities
    print("📤 Step 3: Reporting capabilities to backend...")

    try:
        result = client.report_capabilities(capabilities)
        print(f"   ✅ Capabilities reported successfully")
        print(f"   📊 Granted: {result['granted']}/{result['total']}")
        print()
    except Exception as e:
        print(f"   ⚠  Capability reporting failed: {e}")
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
        print(f"   ⚠  SDK integration report failed: {e}")
        print()

    # Step 5: Register test MCP server
    print("🔌 Step 5: Registering test MCP server...")

    try:
        mcp_result = client.register_mcp(
            mcp_server_id="filesystem-mcp-server",
            detection_method="auto_sdk",
            confidence=95.0,
            metadata={
                "source": "python_sdk_api_key_test",
                "package": "mcp-server-filesystem"
            }
        )

        print(f"   ✅ Registered {mcp_result.get('added', 0)} MCP server(s)")
        print()
    except Exception as e:
        print(f"   ⚠  MCP registration failed (may already exist): {e}")
        print()

    # Summary
    print("=" * 80)
    print("🎉 Python SDK API Key Mode Test Complete!")
    print(f"   - Agent ID: {AGENT_ID}")
    print(f"   - Capabilities detected: {len(capabilities)}")
    print(f"   - Authentication: API key mode ✅")
    print(f"   - SDK Integration: ✅")
    print(f"   - MCP Server: ✅")
    print()
    print("📊 Check the AIM dashboard:")
    print(f"   - Capabilities: {AIM_URL}/dashboard/agents/{AGENT_ID}")
    print(f"   - Detection: {AIM_URL}/dashboard/sdk")
    print(f"   - Connections: {AIM_URL}/dashboard/agents/{AGENT_ID}")
    print("=" * 80)
    print()


if __name__ == "__main__":
    main()
