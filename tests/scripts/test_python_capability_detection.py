#!/usr/bin/env python3
"""
Python SDK Capability Detection Test
Tests capability detection and reporting similar to Go and JavaScript SDK tests.
"""

import requests
import json
from datetime import datetime, timezone

# Test configuration
AGENT_ID = "e237d89d-d366-43e5-808e-32c2ab64de6b"
API_KEY = "aim_live_dw4shT8Ng6fyM7OTO9XLVA71NP09KVeBqmJhlQe_cJw="
AIM_URL = "http://localhost:8080"

def test_grant_capabilities():
    """Grant capabilities to the Python SDK test agent."""
    print("=" * 80)
    print("🐍 PYTHON SDK CAPABILITY DETECTION TEST")
    print("=" * 80)
    print()
    print(f"Agent ID: {AGENT_ID}")
    print(f"AIM URL: {AIM_URL}")
    print()

    # Capabilities to grant (detected from common Python imports)
    capabilities_to_grant = [
        "network_access",    # From requests import
        "make_api_calls",    # From requests import
        "read_files",        # From os/pathlib import
    ]

    print(f"📋 Granting {len(capabilities_to_grant)} capabilities...")
    print()

    headers = {
        "Content-Type": "application/json",
        "X-AIM-API-Key": API_KEY
    }

    granted_count = 0
    for capability_type in capabilities_to_grant:
        url = f"{AIM_URL}/api/v1/sdk-api/agents/{AGENT_ID}/capabilities"

        payload = {
            "capabilityType": capability_type,
            "scope": {
                "source": "python_sdk_auto_detection",
                "detectedAt": datetime.now(timezone.utc).isoformat()
            }
        }

        try:
            response = requests.post(url, headers=headers, json=payload)

            if response.status_code in [200, 201]:
                granted_count += 1
                result = response.json()
                print(f"   ✅ Granted: {capability_type}")
                print(f"      Capability ID: {result.get('id', 'N/A')}")
            elif response.status_code == 409:
                # Capability already exists
                print(f"   ℹ️  Already exists: {capability_type}")
                granted_count += 1
            else:
                print(f"   ❌ Failed to grant {capability_type}: {response.status_code}")
                print(f"      Error: {response.text}")

        except Exception as e:
            print(f"   ❌ Exception granting {capability_type}: {e}")

    print()
    print(f"📊 Results: {granted_count}/{len(capabilities_to_grant)} capabilities granted/verified")
    print()

    return granted_count == len(capabilities_to_grant)


def test_sdk_integration_report():
    """Report SDK integration to show in Detection tab."""
    print("📡 Reporting SDK integration...")
    print()

    headers = {
        "Content-Type": "application/json",
        "X-AIM-API-Key": API_KEY
    }

    detection_event = {
        "detections": [{
            "mcpServer": "aim-sdk-integration",
            "detectionMethod": "sdk_integration",
            "confidence": 100.0,
            "details": {
                "platform": "python",
                "capabilities": ["auto_detect_mcps", "capability_detection"],
                "integrated": True
            },
            "sdkVersion": "aim-sdk-python@1.0.0",
            "timestamp": datetime.now(timezone.utc).isoformat()
        }]
    }

    url = f"{AIM_URL}/api/v1/detection/agents/{AGENT_ID}/report"

    try:
        response = requests.post(url, headers=headers, json=detection_event)

        if response.status_code in [200, 201]:
            result = response.json()
            print(f"   ✅ SDK integration reported successfully")
            print(f"      Detections processed: {result.get('detectionsProcessed', 0)}")
            print()
            return True
        else:
            print(f"   ❌ Failed to report SDK integration: {response.status_code}")
            print(f"      Error: {response.text}")
            print()
            return False

    except Exception as e:
        print(f"   ❌ Exception reporting SDK integration: {e}")
        print()
        return False


def main():
    """Run all tests."""
    # Test 1: Grant capabilities
    capabilities_ok = test_grant_capabilities()

    # Test 2: Report SDK integration
    integration_ok = test_sdk_integration_report()

    # Summary
    print("=" * 80)
    print("📊 TEST SUMMARY")
    print("=" * 80)
    print()
    print(f"   Capabilities: {'✅ PASS' if capabilities_ok else '❌ FAIL'}")
    print(f"   SDK Integration: {'✅ PASS' if integration_ok else '❌ FAIL'}")
    print()

    if capabilities_ok and integration_ok:
        print("🎉 All tests passed!")
        print()
        print("🔍 Next: Validate dashboard tabs")
        print(f"   URL: http://localhost:3000/dashboard/agents/{AGENT_ID}")
        print()
        print("Expected results:")
        print("   - Capabilities tab: Shows 3 capabilities (network_access, make_api_calls, read_files)")
        print("   - Detection tab: Shows aim-sdk-python@1.0.0 integration")
        print("   - Connections tab: Shows aim-sdk-integration MCP server")
    else:
        print("❌ Some tests failed")
        print()

    print("=" * 80)
    print()


if __name__ == "__main__":
    main()
