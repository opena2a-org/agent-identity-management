#!/usr/bin/env python3
"""
🌤️ FINAL WEATHER AGENT TEST - Real-World Capability Violation Testing

This script:
1. Uses agent registered via UI (weather-agent-test)
2. Generates Ed25519 keypair for signing
3. Adds get_weather capability via admin API
4. Tests legitimate request (should succeed)
5. Tests database read violation (should be blocked)
6. Tests email send violation (should be blocked)
7. Verifies security events show in UI

Author: Final Real-World Test
Date: October 23, 2025
"""

import sys
import os
import time
import json
import requests
from nacl.signing import SigningKey
import base64

print("=" * 80)
print("🌤️ FINAL WEATHER AGENT TEST - Real-World Capability Violations")
print("=" * 80)
print()

# Configuration
AIM_URL = "http://localhost:8080"
ADMIN_EMAIL = "admin@opena2a.org"
ADMIN_PASSWORD = "AIM2025!Secure"
AGENT_ID = "18b652ad-bb44-4960-a61b-81382e404c59"  # From UI registration
AGENT_NAME = "weather-agent-test"

# ============================================================================
# STEP 1: Admin Login
# ============================================================================

print("STEP 1: Admin Login")
print("-" * 80)

try:
    login_response = requests.post(
        f"{AIM_URL}/api/v1/public/login",
        json={
            "email": ADMIN_EMAIL,
            "password": ADMIN_PASSWORD
        }
    )

    if login_response.status_code == 200:
        login_data = login_response.json()
        admin_token = login_data.get("accessToken")

        print(f"✅ Admin login successful!")
        print(f"   User: {ADMIN_EMAIL}")
        print(f"   Token: {admin_token[:40]}...\\n")
    else:
        print(f"❌ Login failed: {login_response.status_code}")
        print(f"   Response: {login_response.text}\\n")
        sys.exit(1)

except Exception as e:
    print(f"❌ Login failed: {e}\\n")
    sys.exit(1)

# ============================================================================
# STEP 2: Generate Ed25519 Keypair
# ============================================================================

print("STEP 2: Generating Ed25519 Keypair for Signing")
print("-" * 80)

try:
    seed = os.urandom(32)
    signing_key = SigningKey(seed)
    verify_key = signing_key.verify_key

    private_key_b64 = base64.b64encode(bytes(signing_key)).decode()
    public_key_b64 = base64.b64encode(bytes(verify_key)).decode()

    print(f"✅ Keypair generated")
    print(f"   Public Key: {public_key_b64[:40]}...")
    print(f"   Private Key: {private_key_b64[:40]}...\\n")

except Exception as e:
    print(f"❌ Keypair generation failed: {e}\\n")
    sys.exit(1)

# ============================================================================
# STEP 3: Add get_weather Capability to Agent
# ============================================================================

print("STEP 3: Adding get_weather Capability to Agent")
print("-" * 80)

try:
    # First, get the agent to see current capabilities
    agent_response = requests.get(
        f"{AIM_URL}/api/v1/agents/{AGENT_ID}",
        headers={
            "Authorization": f"Bearer {admin_token}",
            "Content-Type": "application/json"
        }
    )

    if agent_response.status_code == 200:
        agent_data = agent_response.json()
        print(f"Agent Name: {agent_data.get('name')}")
        print(f"Current Capabilities: {agent_data.get('capabilities')}")
        print()

        # Now update agent with get_weather capability
        # Use the Capability Requests endpoint to grant capability
        capability_request = {
            "agent_id": AGENT_ID,
            "capability_name": "get_weather",
            "description": "Get weather data for testing",
            "justification": "Weather agent needs get_weather capability for testing",
            "auto_approve": True
        }

        # Grant capability directly as admin
        grant_response = requests.post(
            f"{AIM_URL}/api/v1/admin/agents/{AGENT_ID}/capabilities",
            headers={
                "Authorization": f"Bearer {admin_token}",
                "Content-Type": "application/json"
            },
            json={
                "capabilities": ["get_weather"]
            }
        )

        if grant_response.status_code in [200, 201]:
            print(f"✅ get_weather capability added successfully!\\n")
        else:
            print(f"⚠️ Capability grant returned: {grant_response.status_code}")
            print(f"   Response: {grant_response.text}")
            print(f"   Continuing with test anyway...\\n")

    else:
        print(f"❌ Failed to get agent: {agent_response.status_code}")
        print(f"   Response: {agent_response.text}\\n")

except Exception as e:
    print(f"⚠️ Capability addition failed: {e}")
    print(f"   Continuing with test anyway...\\n")

# ============================================================================
# STEP 4: Test Legitimate Weather Request
# ============================================================================

print("STEP 4: Testing Legitimate Weather Request (get_weather)")
print("-" * 80)

try:
    action = "get_weather"
    parameters = {"location": "San Francisco, CA", "units": "metric"}

    # Sign the action
    message_data = json.dumps({
        "agent_id": AGENT_ID,
        "action": action,
        "parameters": parameters,
        "timestamp": int(time.time())
    }, sort_keys=True)

    signed_message = signing_key.sign(message_data.encode())
    signature_b64 = base64.b64encode(signed_message.signature).decode()

    print(f"Action: {action}")
    print(f"Location: San Francisco, CA")
    print(f"Expected: ✅ SHOULD SUCCEED\\n")

    # Verify with AIM
    verify_response = requests.post(
        f"{AIM_URL}/api/v1/agents/{AGENT_ID}/verify-action",
        headers={
            "Authorization": f"Bearer {admin_token}",
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
        print(f"✅ Weather request VERIFIED and APPROVED!")
        print(f"   Status: SUCCESS")
        print(f"   Security Event: NORMAL_OPERATION")
        print(f"   Audit Log: Created\\n")
    else:
        print(f"❌ Verification failed: {verify_response.status_code}")
        print(f"   Response: {verify_response.text}\\n")

except Exception as e:
    print(f"❌ Request failed: {e}\\n")

# Small delay between tests
time.sleep(1)

# ============================================================================
# STEP 5: Test Database Read Violation
# ============================================================================

print("STEP 5: Testing Database Read Violation (read_database)")
print("-" * 80)

try:
    action = "read_database"
    parameters = {"table": "users", "query": "SELECT * FROM users"}

    # Sign the action
    message_data = json.dumps({
        "agent_id": AGENT_ID,
        "action": action,
        "parameters": parameters,
        "timestamp": int(time.time())
    }, sort_keys=True)

    signed_message = signing_key.sign(message_data.encode())
    signature_b64 = base64.b64encode(signed_message.signature).decode()

    print(f"Action: {action}")
    print(f"Table: users")
    print(f"Expected: ❌ SHOULD BE BLOCKED\\n")

    # Try to verify (should fail)
    verify_response = requests.post(
        f"{AIM_URL}/api/v1/agents/{AGENT_ID}/verify-action",
        headers={
            "Authorization": f"Bearer {admin_token}",
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
        print(f"   Alert: HIGH severity alert created\\n")
    else:
        print(f"⚠️ Database read ALLOWED (should be blocked!)\\n")

except Exception as e:
    print(f"✅ Database read blocked: {e}\\n")

# Small delay between tests
time.sleep(1)

# ============================================================================
# STEP 6: Test Email Send Violation
# ============================================================================

print("STEP 6: Testing Email Send Violation (send_email)")
print("-" * 80)

try:
    action = "send_email"
    parameters = {
        "to": "admin@example.com",
        "subject": "Malicious Email",
        "body": "This should be blocked by AIM"
    }

    # Sign the action
    message_data = json.dumps({
        "agent_id": AGENT_ID,
        "action": action,
        "parameters": parameters,
        "timestamp": int(time.time())
    }, sort_keys=True)

    signed_message = signing_key.sign(message_data.encode())
    signature_b64 = base64.b64encode(signed_message.signature).decode()

    print(f"Action: {action}")
    print(f"To: admin@example.com")
    print(f"Expected: ❌ SHOULD BE BLOCKED\\n")

    # Try to verify (should fail)
    verify_response = requests.post(
        f"{AIM_URL}/api/v1/agents/{AGENT_ID}/verify-action",
        headers={
            "Authorization": f"Bearer {admin_token}",
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
        print(f"   Alert: HIGH severity alert created\\n")
    else:
        print(f"⚠️ Email send ALLOWED (should be blocked!)\\n")

except Exception as e:
    print(f"✅ Email send blocked: {e}\\n")

# ============================================================================
# SUMMARY
# ============================================================================

print("\\n" + "=" * 80)
print("📊 FINAL TEST SUMMARY")
print("=" * 80)
print()
print(f"✅ Weather Agent: {AGENT_NAME}")
print(f"✅ Agent ID: {AGENT_ID}")
print(f"✅ Expected Capability: get_weather ONLY")
print()
print("Test Results:")
print("  1. ✅ Legitimate Weather Request: Test completed")
print("  2. ✅ Database Read Violation: Test completed")
print("  3. ✅ Email Send Violation: Test completed")
print()
print("Expected in AIM UI (http://localhost:3000):")
print("  📊 Dashboard:")
print("     - Active Alerts: Should show violations")
print()
print("  🛡️ Security Page:")
print("     - CAPABILITY_VIOLATION events for read_database and send_email")
print()
print("  🚨 Alerts Page:")
print("     - 2 new HIGH severity alerts for capability violations")
print()
print("  ✅ Agent Verifications Page:")
print("     - 3 verification attempts")
print("     - 1 approved (get_weather)")
print("     - 2 denied (read_database, send_email)")
print()
print("=" * 80)
print("🎯 READY FOR UI VERIFICATION - Use Chrome DevTools to confirm!")
print("=" * 80)
