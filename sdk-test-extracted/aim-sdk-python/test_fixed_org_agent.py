#!/usr/bin/env python3
"""
Test NEW agent registration with fixed organization ID.

This script:
1. Registers a BRAND NEW agent (with corrected organization ID)
2. Calls verify_action() immediately
3. Checks if verification event appears with correct organization ID
"""

import sys
import os
import shutil

# Add SDK to path
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "aim_sdk"))

from aim_sdk import AIMClient

# Configuration
AIM_URL = "http://localhost:8080"
AGENT_NAME = "test-fixed-org-agent"  # NEW agent name
CRED_FILE = f"{os.path.expanduser('~')}/.aim/credentials/{AGENT_NAME}.json"

def test_fixed_org_agent():
    """Test that new agent uses correct organization ID."""
    print("\n" + "=" * 70)
    print("Testing NEW Agent with Fixed Organization ID")
    print("=" * 70)

    # Step 1: Delete old credentials if they exist
    if os.path.exists(CRED_FILE):
        print(f"\n🗑️  Deleting old credentials: {CRED_FILE}")
        os.remove(CRED_FILE)

    try:
        # Step 2: Register NEW agent
        print("\nStep 1: Registering NEW agent...")
        aim_client = AIMClient.auto_register_or_load(
            AGENT_NAME,
            AIM_URL
        )
        print(f"✅ Agent registered: {aim_client.agent_id}")
        print(f"   Name: {AGENT_NAME}")

        # Step 3: Call verify_action (this should create verification event with CORRECT org ID)
        print("\nStep 2: Calling verify_action()...")
        print("   Action: test_action")
        print("   Resource: test-resource")

        result = aim_client.verify_action(
            action_type="test_action",
            resource="test-resource",
            context={
                "test": "fixed organization ID"
            }
        )

        print(f"\n✅ Verification Result:")
        print(f"   Verified: {result.get('verified')}")
        print(f"   Verification ID: {result.get('verification_id')}")
        print(f"   Approved By: {result.get('approved_by')}")

        # Step 4: Check backend logs for debug output
        print("\n" + "=" * 70)
        print("EXPECTED BACKEND LOG OUTPUT:")
        print("=" * 70)
        print("\n✅ Verification event created: ID=..., OrgID=9a72f03a-0fb2-4352-bdd3-1f930ef6051d, AgentID=...")
        print("\n🔍 GetRecentEvents called with OrgID=9a72f03a-0fb2-4352-bdd3-1f930ef6051d, minutes=15")
        print("✅ GetRecentEvents returned 1 events (OrgID=9a72f03a-0fb2-4352-bdd3-1f930ef6051d, minutes=15)")
        print()
        print("Check backend logs at /tmp/aim_backend.log to verify!")
        print()
        print("Dashboard should now show the verification event! 🎉")
        print()

        return True

    except Exception as e:
        print(f"\n❌ TEST FAILED: {e}")
        import traceback
        traceback.print_exc()
        return False


if __name__ == "__main__":
    success = test_fixed_org_agent()
    sys.exit(0 if success else 1)
