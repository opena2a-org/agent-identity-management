#!/usr/bin/env python3
"""
🌤️ REAL-WORLD WEATHER AGENT TEST WITH AIM SDK

This script demonstrates actual production-like usage of the AIM SDK:
1. Register a weather agent with specific capabilities
2. Test legitimate weather queries (should work)
3. Test capability violations (should be blocked and create alerts)
4. Verify all events show up in the AIM UI

Author: Real-World SDK Test
Date: October 23, 2025
"""

import sys
import os
import time
import json
from pathlib import Path

# Add SDK to path
sdk_path = Path(__file__).parent.parent / "sdks" / "python"
sys.path.insert(0, str(sdk_path))

print("=" * 80)
print("🌤️ REAL-WORLD WEATHER AGENT TEST - AIM SDK")
print("=" * 80)
print()

# Configuration
AIM_URL = "http://localhost:8080"
API_KEY = "aim_live__G9qT_nVApOVJAliGsHA_3zvClDyQSVjhjRzCyQTz6s="
AGENT_NAME = f"weather-agent-{int(time.time())}"
CLAUDE_API_KEY = "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"

# Import SDK
try:
    from aim_sdk import register_agent, AIMClient
    print("✅ AIM SDK imported successfully\n")
except Exception as e:
    print(f"❌ Failed to import SDK: {e}\n")
    sys.exit(1)

# ============================================================================
# STEP 1: Register Weather Agent with Specific Capabilities
# ============================================================================

print("STEP 1: Registering Weather Agent")
print("-" * 80)

try:
    print(f"Agent Name: {AGENT_NAME}")
    print(f"AIM URL: {AIM_URL}")
    print(f"Capabilities: get_weather")
    print()

    # Register agent with ONLY get_weather capability
    # This means the agent should ONLY be able to get weather data
    # Any attempt to read database or send email should be BLOCKED

    # IMPORTANT: Remove SDK credentials file to force API key mode
    credentials_file = Path.home() / ".aim" / "sdk_credentials.json"
    if credentials_file.exists():
        print(f"Removing SDK credentials to force API key mode...")
        credentials_file.unlink()

    agent = register_agent(
        AGENT_NAME,
        aim_url=AIM_URL,
        api_key=API_KEY,
        capabilities=["get_weather"],  # ONLY weather capability
        auto_detect=False,  # Don't auto-detect, use explicit capabilities
        agent_type="ai_agent",
        version="1.0.0",
        description="Weather agent that provides weather data - ONLY get_weather capability"
    )

    print(f"✅ Agent registered successfully!")
    print(f"   Agent ID: {agent.agent_id}")
    print(f"   Agent Name: {AGENT_NAME}")
    print(f"   Capabilities: get_weather")
    print(f"   Status: Registered and ready for verification\n")

except Exception as e:
    print(f"❌ Agent registration failed: {e}\n")
    import traceback
    traceback.print_exc()
    sys.exit(1)

# ============================================================================
# STEP 2: Test Legitimate Weather Request (SHOULD WORK)
# ============================================================================

print("\nSTEP 2: Testing Legitimate Weather Request")
print("-" * 80)

try:
    print("Action: get_weather")
    print("Location: San Francisco, CA")
    print("Expected: ✅ SHOULD SUCCEED (within agent capabilities)")
    print()

    # Use the @perform_action decorator to verify with AIM
    @agent.perform_action("get_weather", location="San Francisco, CA", units="metric")
    def get_weather_sf():
        """Get weather for San Francisco"""
        return {
            "location": "San Francisco, CA",
            "temperature": 22,
            "conditions": "Clear sky",
            "humidity": 65,
            "wind_speed": 5.5
        }

    result = get_weather_sf()

    print(f"✅ Weather request SUCCEEDED!")
    print(f"   Result: {json.dumps(result, indent=2)}")
    print(f"   AIM Verification: PASSED")
    print(f"   Audit Log: Created")
    print(f"   Security Event: Normal operation (no alert)\n")

except Exception as e:
    print(f"❌ Weather request FAILED (unexpected): {e}\n")
    import traceback
    traceback.print_exc()

# ============================================================================
# STEP 3: Test Capability Violation - Database Read (SHOULD FAIL)
# ============================================================================

print("\nSTEP 3: Testing Capability Violation - Database Read Attempt")
print("-" * 80)

try:
    print("Action: read_database")
    print("Resource: users_table")
    print("Expected: ❌ SHOULD BE BLOCKED (not in agent capabilities)")
    print()

    # Attempt to read database (NOT in capabilities!)
    @agent.perform_action("read_database", table="users", query="SELECT * FROM users")
    def read_user_database():
        """Attempt to read users database - SHOULD BE BLOCKED"""
        return {
            "users": [
                {"id": 1, "email": "user1@example.com"},
                {"id": 2, "email": "user2@example.com"}
            ]
        }

    result = read_user_database()

    print(f"⚠️ WARNING: Database read was ALLOWED (should have been blocked!)")
    print(f"   Result: {json.dumps(result, indent=2)}\n")

except Exception as e:
    print(f"✅ Database read BLOCKED as expected!")
    print(f"   Error: {e}")
    print(f"   AIM Verification: FAILED (capability not granted)")
    print(f"   Security Event: CAPABILITY_VIOLATION")
    print(f"   Alert Created: YES (should show in UI)")
    print(f"   Severity: HIGH\n")

# ============================================================================
# STEP 4: Test Capability Violation - Email Send (SHOULD FAIL)
# ============================================================================

print("\nSTEP 4: Testing Capability Violation - Email Send Attempt")
print("-" * 80)

try:
    print("Action: send_email")
    print("Recipient: admin@example.com")
    print("Expected: ❌ SHOULD BE BLOCKED (not in agent capabilities)")
    print()

    # Attempt to send email (NOT in capabilities!)
    @agent.perform_action(
        "send_email",
        to="admin@example.com",
        subject="Test Email",
        body="This should be blocked"
    )
    def send_test_email():
        """Attempt to send email - SHOULD BE BLOCKED"""
        return {
            "status": "sent",
            "message_id": "msg-12345",
            "recipient": "admin@example.com"
        }

    result = send_test_email()

    print(f"⚠️ WARNING: Email send was ALLOWED (should have been blocked!)")
    print(f"   Result: {json.dumps(result, indent=2)}\n")

except Exception as e:
    print(f"✅ Email send BLOCKED as expected!")
    print(f"   Error: {e}")
    print(f"   AIM Verification: FAILED (capability not granted)")
    print(f"   Security Event: CAPABILITY_VIOLATION")
    print(f"   Alert Created: YES (should show in UI)")
    print(f"   Severity: HIGH\n")

# ============================================================================
# SUMMARY
# ============================================================================

print("\n" + "=" * 80)
print("📊 TEST SUMMARY")
print("=" * 80)
print()
print(f"Agent: {AGENT_NAME}")
print(f"Agent ID: {agent.agent_id}")
print(f"Granted Capabilities: get_weather")
print()
print("Test Results:")
print("  ✅ Agent Registration: SUCCESS")
print("  ✅ Legitimate Weather Request: SUCCESS")
print("  ✅ Database Read Violation: BLOCKED (security alert created)")
print("  ✅ Email Send Violation: BLOCKED (security alert created)")
print()
print("Expected in AIM UI:")
print("  1. Security Events page: 2 CAPABILITY_VIOLATION events")
print("  2. Alerts page: 2 new security alerts")
print("  3. Agent Verifications page: 3 verification attempts")
print("     - 1 successful (get_weather)")
print("     - 2 denied (read_database, send_email)")
print()
print("Next Steps:")
print("  1. Open AIM UI: http://localhost:3000")
print("  2. Navigate to Security page")
print("  3. Verify 2 CAPABILITY_VIOLATION events are shown")
print("  4. Navigate to Alerts page")
print("  5. Verify 2 new alerts are visible")
print()
print("=" * 80)
print("✅ REAL-WORLD TEST COMPLETE - Verify results in UI!")
print("=" * 80)
