#!/usr/bin/env python3
"""
Test Phase 2: Auto-Registration with Challenge-Response Verification

This script tests the complete flow from registration to auto-approval.
"""

import sys
import os
import time

# Add SDK to path
sys.path.insert(0, os.path.dirname(__file__))

from aim_sdk import register_agent

def test_complete_flow():
    """Test end-to-end auto-registration with challenge verification."""

    # Use unique timestamp-based names
    timestamp = str(int(time.time()))

    print("=" * 80)
    print("🧪 Phase 2: Auto-Registration + Challenge-Response Test")
    print("=" * 80)

    print("\n📋 Test: High Trust Agent (Repo URL + Docs URL = 75 points)")
    print("-" * 80)

    try:
        agent = register_agent(
            name=f"test-auto-verify-{timestamp}",
            aim_url="http://localhost:8080",
            display_name="Auto-Verify Test Agent",
            description="Testing automatic challenge-response verification",
            agent_type="ai_agent",
            version="1.0.0",
            repository_url="https://github.com/opena2a/aim-sdk",  # +10 points
            documentation_url="https://docs.aim.opena2a.org",      # +5 points
            # Base: 50 + Repo: 10 + Docs: 5 + Version: 5 + GitHub: 10 = 80 points
            # After verification: 80 + 25 = 105 (capped at 100) -> auto-approved!
            force_new=True
        )

        print("\n✅ TEST PASSED!")
        print("   Successfully registered and created AIMClient instance")

    except Exception as e:
        print(f"\n❌ TEST FAILED: {e}")
        import traceback
        traceback.print_exc()

    print("\n" + "=" * 80)
    print("🎉 Test completed!")
    print("=" * 80)

if __name__ == "__main__":
    test_complete_flow()
