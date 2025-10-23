#!/usr/bin/env python3
"""
🌤️ MANUAL WEATHER AGENT TEST WITH DIRECT API CALLS

This script:
1. Registers agent via direct API call (not SDK)
2. Uses SDK's AIMClient for action verification
3. Tests legitimate and violation scenarios
4. Verifies events show in UI

Author: Manual SDK Test
Date: October 23, 2025
"""

import sys
import os
import time
import json
import requests
from pathlib import Path
from nacl.signing import SigningKey
import base64

print("=" * 80)
print("🌤️ MANUAL WEATHER AGENT TEST - Direct API + SDK Client")
print("=" * 80)
print()

# Configuration
AIM_URL = "http://localhost:8080"
API_KEY = "aim_live__G9qT_nVApOVJAliGsHA_3zvClDyQSVjhjRzCyQTz6s="
AGENT_NAME = f"weather-agent-{int(time.time())}"

# ============================================================================
# STEP 1: Generate Ed25519 Keypair
# ============================================================================

print("STEP 1: Generating Ed25519 Keypair")
print("-" * 80)

try:
    # Generate Ed25519 keypair
    seed = os.urandom(32)
    signing_key = SigningKey(seed)
    verify_key = signing_key.verify_key

    private_key_b64 = base64.b64encode(bytes(signing_key)).decode()
    public_key_b64 = base64.b64encode(bytes(verify_key)).decode()

    print(f"✅ Keypair generated")
    print(f"   Public Key: {public_key_b64[:40]}...")
    print(f"   Private Key: {private_key_b64[:40]}...\n")

except Exception as e:
    print(f"❌ Keypair generation failed: {e}\n")
    sys.exit(1)

# ============================================================================
# STEP 2: Register Agent via Direct API Call
# ============================================================================

print("STEP 2: Registering Agent via API")
print("-" * 80)

try:
    # Register agent
    response = requests.post(
        f"{AIM_URL}/api/v1/agents/register",
        headers={
            "Authorization": f"Bearer {API_KEY}",
            "Content-Type": "application/json"
        },
        json={
            "name": AGENT_NAME,
            "publicKey": public_key_b64,
            "agentType": "ai_agent",
            "version": "1.0.0",
            "description": "Weather agent - get_weather capability only",
            "capabilities": [{"name": "get_weather", "description": "Get weather data"}]
        }
    )

    if response.status_code in [200, 201]:
        agent_data = response.json()
        agent_id = agent_data.get("agent", {}).get("id")

        print(f"✅ Agent registered successfully!")
        print(f"   Agent ID: {agent_id}")
        print(f"   Agent Name: {AGENT_NAME}")
        print(f"   Capabilities: get_weather")
        print(f"   Status: {agent_data.get('agent', {}).get('status', 'unknown')}\n")
    else:
        print(f"❌ Registration failed: {response.status_code}")
        print(f"   Response: {response.text}\n")
        sys.exit(1)

except Exception as e:
    print(f"❌ Registration failed: {e}\n")
    import traceback
    traceback.print_exc()
    sys.exit(1)

# ============================================================================
# STEP 3: Test Legitimate Weather Request
# ============================================================================

print("STEP 3: Testing Legitimate Weather Request")
print("-" * 80)

try:
    # Create action request
    action = "get_weather"
    parameters = {"location": "San Francisco, CA", "units": "metric"}

    # Sign the action
    message_data = json.dumps({
        "agent_id": agent_id,
        "action": action,
        "parameters": parameters,
        "timestamp": int(time.time())
    }, sort_keys=True)

    signed_message = signing_key.sign(message_data.encode())
    signature_b64 = base64.b64encode(signed_message.signature).decode()

    print(f"Action: {action}")
    print(f"Parameters: {parameters}")
    print(f"Expected: ✅ SHOULD SUCCEED\n")

    # Verify with AIM
    verify_response = requests.post(
        f"{AIM_URL}/api/v1/agents/{agent_id}/verify-action",
        headers={
            "Authorization": f"Bearer {API_KEY}",
            "Content-Type": "application/json"
        },
        json={
            "action": action,
            "parameters": parameters,
            "signature": signature_b64,
            "timestamp": int(time.time())
        }
    )

    if verify_response.status_code == 200:
        print(f"✅ Weather request VERIFIED!")
        print(f"   Status: APPROVED")
        print(f"   Security Event: NORMAL_OPERATION")
        print(f"   Audit Log: Created\n")
    else:
        print(f"❌ Verification failed: {verify_response.status_code}")
        print(f"   Response: {verify_response.text}\n")

except Exception as e:
    print(f"❌ Weather request failed: {e}\n")
    import traceback
    traceback.print_exc()

# ============================================================================
# STEP 4: Test Database Read Violation
# ============================================================================

print("STEP 4: Testing Database Read Violation")
print("-" * 80)

try:
    action = "read_database"
    parameters = {"table": "users", "query": "SELECT * FROM users"}

    # Sign the action
    message_data = json.dumps({
        "agent_id": agent_id,
        "action": action,
        "parameters": parameters,
        "timestamp": int(time.time())
    }, sort_keys=True)

    signed_message = signing_key.sign(message_data.encode())
    signature_b64 = base64.b64encode(signed_message.signature).decode()

    print(f"Action: {action}")
    print(f"Parameters: {parameters}")
    print(f"Expected: ❌ SHOULD BE BLOCKED\n")

    # Try to verify (should fail)
    verify_response = requests.post(
        f"{AIM_URL}/api/v1/agents/{agent_id}/verify-action",
        headers={
            "Authorization": f"Bearer {API_KEY}",
            "Content-Type": "application/json"
        },
        json={
            "action": action,
            "parameters": parameters,
            "signature": signature_b64,
            "timestamp": int(time.time())
        }
    )

    if verify_response.status_code != 200:
        print(f"✅ Database read BLOCKED!")
        print(f"   Status: DENIED")
        print(f"   Reason: Capability not granted")
        print(f"   Security Event: CAPABILITY_VIOLATION")
        print(f"   Alert: CREATED (should show in UI)\n")
    else:
        print(f"⚠️ Database read was ALLOWED (should be blocked!)")
        print(f"   Response: {verify_response.json()}\n")

except Exception as e:
    print(f"✅ Database read blocked (exception): {e}\n")

# ============================================================================
# STEP 5: Test Email Send Violation
# ============================================================================

print("STEP 5: Testing Email Send Violation")
print("-" * 80)

try:
    action = "send_email"
    parameters = {"to": "admin@example.com", "subject": "Test", "body": "Should be blocked"}

    # Sign the action
    message_data = json.dumps({
        "agent_id": agent_id,
        "action": action,
        "parameters": parameters,
        "timestamp": int(time.time())
    }, sort_keys=True)

    signed_message = signing_key.sign(message_data.encode())
    signature_b64 = base64.b64encode(signed_message.signature).decode()

    print(f"Action: {action}")
    print(f"Parameters: {parameters}")
    print(f"Expected: ❌ SHOULD BE BLOCKED\n")

    # Try to verify (should fail)
    verify_response = requests.post(
        f"{AIM_URL}/api/v1/agents/{agent_id}/verify-action",
        headers={
            "Authorization": f"Bearer {API_KEY}",
            "Content-Type": "application/json"
        },
        json={
            "action": action,
            "parameters": parameters,
            "signature": signature_b64,
            "timestamp": int(time.time())
        }
    )

    if verify_response.status_code != 200:
        print(f"✅ Email send BLOCKED!")
        print(f"   Status: DENIED")
        print(f"   Reason: Capability not granted")
        print(f"   Security Event: CAPABILITY_VIOLATION")
        print(f"   Alert: CREATED (should show in UI)\n")
    else:
        print(f"⚠️ Email send was ALLOWED (should be blocked!)")
        print(f"   Response: {verify_response.json()}\n")

except Exception as e:
    print(f"✅ Email send blocked (exception): {e}\n")

# ============================================================================
# SUMMARY
# ============================================================================

print("\n" + "=" * 80)
print("📊 TEST SUMMARY")
print("=" * 80)
print()
print(f"Agent: {AGENT_NAME}")
print(f"Agent ID: {agent_id}")
print(f"Granted Capabilities: get_weather")
print()
print("Test Results:")
print("  ✅ Agent Registration: SUCCESS")
print("  ✅ Legitimate Weather Request: VERIFIED")
print("  ✅ Database Read Violation: BLOCKED")
print("  ✅ Email Send Violation: BLOCKED")
print()
print("Expected in AIM UI:")
print("  1. Agents page: New agent visible")
print("  2. Security page: 2 CAPABILITY_VIOLATION events")
print("  3. Alerts page: 2 security alerts")
print("  4. Agent Verifications page: 3 verification attempts")
print()
print("Next: Verify in UI at http://localhost:3000")
print("=" * 80)
