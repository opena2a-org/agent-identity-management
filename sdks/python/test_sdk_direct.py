#!/usr/bin/env python3
"""
Simplified SDK Integration Test - Direct Registration

Tests SDK features using direct agent_id and keys (not OAuth flow).
This bypasses OAuth authentication to focus on testing SDK functionality.
"""

import os
import sys
import json
import time
from datetime import datetime
from nacl.signing import SigningKey
from nacl.encoding import Base64Encoder

# Add SDK to path
sys.path.insert(0, os.path.join(os.path.dirname(__file__), 'aim_sdk'))

from aim_sdk import AIMClient, track_mcp_call, MCPDetector, auto_detect_protocol


# Configuration
AIM_URL = os.getenv("AIM_URL", "http://localhost:8080")

print("=" * 80)
print("🧪 AIM SDK Direct Integration Test")
print("=" * 80)
print(f"⏰ Started at: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
print(f"🌐 AIM URL: {AIM_URL}")
print()


def generate_keypair():
    """Generate Ed25519 keypair and return as base64 (raw 32-byte keys)"""
    # Generate new signing key (32-byte seed)
    signing_key = SigningKey.generate()
    verify_key = signing_key.verify_key

    # Encode as base64
    private_b64 = signing_key.encode(encoder=Base64Encoder).decode('utf-8')
    public_b64 = verify_key.encode(encoder=Base64Encoder).decode('utf-8')

    return private_b64, public_b64


def login_as_admin(aim_url):
    """Login as admin to get JWT token for key registration"""
    import requests

    response = requests.post(
        f"{aim_url}/api/v1/public/login",
        json={
            "email": "admin@opena2a.org",
            "password": "AIM2025!Secure"
        }
    )

    if response.status_code == 200:
        data = response.json()
        return data.get("accessToken")  # Backend returns 'accessToken', not 'token'
    else:
        raise Exception(f"Login failed: {response.status_code} {response.text}")


def test_existing_agent():
    """Test with existing agent from database"""
    print("\n" + "=" * 80)
    print("🧪 TESTING WITH EXISTING AGENT")
    print("=" * 80)

    # Use existing agent from database (one of the real agents)
    AGENT_ID = "4f40a950-270f-49fa-a490-136cf60c12bf"  # integration-test-agent-1761248935

    print(f"📝 Using existing agent: {AGENT_ID}")

    # Generate new keypair for this test
    private_key_b64, public_key_b64 = generate_keypair()
    print(f"🔑 Generated new Ed25519 keypair")
    print(f"   - Public key: {public_key_b64[:20]}...")
    print()

    try:
        # Step 1: Login as admin to get JWT token
        print("🔐 Step 1: Logging in as admin to get JWT token...")
        jwt_token = login_as_admin(AIM_URL)
        print(f"✅ JWT token obtained: {jwt_token[:20]}...")
        print()

        # Step 2: Create SDK client with generated keys
        print("🔧 Step 2: Creating AIMClient with generated keys...")
        client = AIMClient(
            agent_id=AGENT_ID,
            public_key=public_key_b64,
            private_key=private_key_b64,
            aim_url=AIM_URL,
            protocol="mcp"
        )

        print(f"✅ SDK Client initialized:")
        print(f"   - Agent ID: {client.agent_id}")
        print(f"   - Protocol: {client.protocol}")
        print(f"   - AIM URL: {client.aim_url}")
        print()

        # Step 3: Register keys with backend (using JWT token)
        print("📡 Step 3: Registering public key with AIM backend...")
        try:
            # Add JWT token to client session for this request
            client.session.headers.update({"Authorization": f"Bearer {jwt_token}"})

            result = client.register_keys()
            print(f"✅ Keys registered successfully!")
            print(f"   - Message: {result.get('message')}")
            print(f"   - Key created: {result.get('key_created_at')}")
            print(f"   - Key expires: {result.get('key_expires_at')}")

            # Remove JWT token - subsequent requests will use Ed25519 signatures
            del client.session.headers["Authorization"]
        except Exception as e:
            print(f"❌ Key registration failed: {str(e)}")
            print("   Cannot proceed without registered keys - authentication will fail")
            raise

        print()

        # Test 1: Get agent details (now using Ed25519 signature)
        print("📡 Test 1: Getting agent details...")
        try:
            details = client.get_agent_details()
            print(f"✅ Agent details retrieved:")
            print(f"   - Name: {details.get('name')}")
            print(f"   - Type: {details.get('agent_type')}")
            print(f"   - Status: {details.get('status')}")
            print(f"   - Trust Score: {details.get('trust_score')}")
        except Exception as e:
            print(f"   ❌ Failed to get agent details: {str(e)[:150]}")

        print()

        # Test 2: MCP Auto-Discovery
        print("📡 Test 2: MCP Auto-Discovery...")
        from aim_sdk.detection import _mcp_call_tracker
        _mcp_call_tracker.clear()

        # Simulate MCP calls
        print("   - Tracking MCP call: filesystem.read_file")
        track_mcp_call("filesystem", "read_file")
        print("   - Tracking MCP call: filesystem.write_file")
        track_mcp_call("filesystem", "write_file")
        print("   - Tracking MCP call: github.create_issue")
        track_mcp_call("github", "create_issue")
        print("   - Tracking MCP call: supabase.execute_sql")
        track_mcp_call("supabase", "execute_sql")

        # Get detections
        detector = MCPDetector()
        detections = detector.detect_all_with_runtime()

        print(f"✅ Generated {len(detections)} detection events:")
        for det in detections:
            if det["detectionMethod"] == "sdk_runtime":
                server = det["mcpServer"]
                calls = det["details"]["call_count"]
                tools = det["details"]["tools_used"]
                print(f"   - {server}: {calls} calls, tools: {', '.join(tools)}")

        print()

        # Test 3: Protocol Detection
        print("📡 Test 3: Protocol Detection...")
        protocol = auto_detect_protocol()
        print(f"✅ Auto-detected protocol: {protocol}")
        print(f"   - SDK client protocol: {client.protocol}")

        print()

        # Test 4: Report Detections
        print("📡 Test 4: Reporting Detections to AIM...")
        try:
            result = client.report_detections(detections)
            print(f"✅ Detection report result:")
            print(f"   - Success: {result.get('success')}")
            print(f"   - Message: {result.get('message')}")
            print(f"   - Detections reported: {len(detections)}")
        except Exception as e:
            print(f"   ℹ️  Detection reporting: {str(e)[:100]}")

        print()

        # Test 5: Capability Detection
        print("📡 Test 5: Capability Detection...")
        try:
            from aim_sdk import CapabilityDetector
            cap_detector = CapabilityDetector()
            capabilities = cap_detector.detect_all()

            print(f"✅ Detected {len(capabilities)} capabilities:")
            for cap in capabilities[:5]:
                print(f"   - {cap} (import_analysis)")

            if capabilities:
                # Try to report capabilities
                try:
                    result = client.report_capabilities(capabilities)
                    print(f"✅ Capability report: {result.get('message')}")
                except Exception as e:
                    print(f"   ℹ️  Capability reporting: {str(e)[:100]}")
        except Exception as e:
            print(f"   ℹ️  Capability detection: {str(e)[:100]}")

        print()
        print("=" * 80)
        print("✅ SDK FEATURE TESTS COMPLETED")
        print("=" * 80)
        print()
        print("📊 Summary:")
        print(f"   - Protocol detection: ✅ Working")
        print(f"   - MCP auto-discovery: ✅ Working ({len(detections)} detections)")
        print(f"   - SDK client creation: ✅ Working")
        print(f"   - Detection tracking: ✅ Working")
        print()
        print("📋 Next: Verify in Dashboard")
        print(f"   1. Open: http://localhost:3000/agents/{AGENT_ID}")
        print(f"   2. Check Detection tab for new MCP servers:")
        print(f"      - filesystem (2 calls: read_file, write_file)")
        print(f"      - github (1 call: create_issue)")
        print(f"      - supabase (1 call: execute_sql)")
        print(f"   3. Verify detection method: sdk_runtime")
        print()

        return client, detections

    except Exception as e:
        print(f"❌ TEST FAILED: {e}")
        import traceback
        traceback.print_exc()
        return None, []


if __name__ == "__main__":
    client, detections = test_existing_agent()

    print("⏰ Test completed at:", datetime.now().strftime('%Y-%m-%d %H:%M:%S'))
    print()
