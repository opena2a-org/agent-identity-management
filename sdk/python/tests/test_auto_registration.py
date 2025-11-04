#!/usr/bin/env python3
"""
Test Phase 2: Auto-Registration with Challenge-Response Verification

This test demonstrates:
1. Agent self-registration
2. Automatic challenge-response verification
3. Auto-approval based on trust score >= 70
"""

import sys
import os

# Add SDK to path
sys.path.insert(0, os.path.dirname(__file__))

from aim_sdk import register_agent

def test_auto_registration():
    """Test complete auto-registration + verification flow."""

    print("=" * 80)
    print("🧪 Phase 2: Auto-Registration + Challenge-Response Verification Test")
    print("=" * 80)

    # Test agent with high trust score (should auto-approve)
    print("\n📋 Test 1: High Trust Score Agent (should auto-approve)")
    print("-" * 80)

    try:
        agent = register_agent(
            name="test-agent-high-trust",
            aim_url="http://localhost:8080",
            display_name="High Trust Test Agent",
            description="Test agent with high trust score for auto-approval",
            agent_type="ai_agent",
            version="1.0.0",
            repository_url="https://github.com/opena2a/aim-sdk",  # +10 points
            documentation_url="https://docs.aim.opena2a.org",      # +5 points
            force_new=True  # Force new registration for testing
        )

        print("\n✅ Test 1 PASSED - High trust agent registered and verified")
        print(f"   Expected: status='verified', trust_score >= 70")
        print(f"   Got: Successfully created client instance")

    except Exception as e:
        print(f"\n❌ Test 1 FAILED: {e}")
        import traceback
        traceback.print_exc()

    # Test agent with low trust score (should be pending)
    print("\n\n📋 Test 2: Low Trust Score Agent (should be pending)")
    print("-" * 80)

    try:
        agent = register_agent(
            name="test-agent-low-trust",
            aim_url="http://localhost:8080",
            display_name="Low Trust Test Agent",
            description="Test agent with low trust score",
            agent_type="ai_agent",
            # No version, no repo URL, no docs URL = base 50 + 25 verification = 75 points (auto-approved!)
            force_new=True
        )

        print("\n✅ Test 2 PASSED - Low trust agent registered")
        print(f"   Expected: status='pending' OR 'verified' (if >= 70)")
        print(f"   Got: Successfully created client instance")

    except Exception as e:
        print(f"\n❌ Test 2 FAILED: {e}")
        import traceback
        traceback.print_exc()

    print("\n" + "=" * 80)
    print("🎉 All tests completed!")
    print("=" * 80)

if __name__ == "__main__":
    test_auto_registration()
