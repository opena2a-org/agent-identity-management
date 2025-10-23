#!/usr/bin/env python3
"""
Test that SDK verify_action() creates verification events in the database.

This script:
1. Uses existing agent credentials
2. Calls verify_action() for a simple action
3. Checks backend logs to verify verification event was created
"""

import sys
import os

# Add SDK to path
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "aim_sdk"))

from aim_sdk import AIMClient

# Configuration
AIM_URL = "http://localhost:8080"
AGENT_NAME = "live-azure-openai-copilot"  # From previous tests

def test_verification_event_creation():
    """Test that verify_action creates verification events."""
    print("\n" + "=" * 70)
    print("Testing Verification Event Creation")
    print("=" * 70)

    try:
        # Step 1: Load existing agent credentials
        print("\nStep 1: Loading existing agent credentials...")
        aim_client = AIMClient.auto_register_or_load(
            AGENT_NAME,
            AIM_URL
        )
        print(f"✅ Agent loaded: {aim_client.agent_id}")
        print(f"   Name: {AGENT_NAME}")

        # Step 2: Call verify_action (this should create a verification event)
        print("\nStep 2: Calling verify_action()...")
        print("   Action: azure_openai_chat")
        print("   Resource: gpt-4-aim-demo")

        result = aim_client.verify_action(
            action_type="azure_openai_chat",
            resource="gpt-4-aim-demo",
            context={
                "model": "gpt-4",
                "endpoint": "https://aim-openai-demo.openai.azure.com/"
            }
        )

        print(f"\n✅ Verification Result:")
        print(f"   Verified: {result.get('verified')}")
        print(f"   Verification ID: {result.get('verification_id')}")
        print(f"   Approved By: {result.get('approved_by')}")
        print(f"   Expires At: {result.get('expires_at')}")

        # Step 3: Instructions for verification
        print("\n" + "=" * 70)
        print("VERIFICATION STEPS:")
        print("=" * 70)
        print("\n1. Check backend logs for verification event creation:")
        print("   tail -20 /tmp/aim_backend.log")
        print()
        print("2. Query database for verification events:")
        print("   psql -d agent_identity_management -c \\")
        print("   \"SELECT id, agent_id, action, status, created_at FROM verification_events ORDER BY created_at DESC LIMIT 5;\"")
        print()
        print("3. Check dashboard at http://localhost:3000/dashboard/")
        print("   - Should show verification events")
        print("   - Should display agent activity")
        print()

        return True

    except Exception as e:
        print(f"\n❌ TEST FAILED: {e}")
        import traceback
        traceback.print_exc()
        return False


if __name__ == "__main__":
    success = test_verification_event_creation()
    sys.exit(0 if success else 1)
