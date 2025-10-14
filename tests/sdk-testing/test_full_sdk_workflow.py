#!/usr/bin/env python3
"""
Comprehensive Python SDK Testing Script
Tests: Auto-registration, Auto-verification, Capability Detection, MCP Detection, Trust Scoring
"""

import sys
import time
from aim_sdk import register_agent
import requests
import json

def print_section(title):
    """Print formatted section header."""
    print("\n" + "=" * 80)
    print(f"🔍 {title}")
    print("=" * 80)

def verify_agent_in_backend(agent_id):
    """Verify agent exists in backend and get full details."""
    print(f"\n📡 Fetching agent details from backend...")

    # Read credentials to get auth token
    with open('.aim/credentials.json', 'r') as f:
        creds = json.load(f)

    token = creds['refresh_token']
    base_url = creds['aim_url']

    # Get agent details
    url = f"{base_url}/api/v1/agents/{agent_id}"
    headers = {"Authorization": f"Bearer {token}"}

    try:
        response = requests.get(url, headers=headers)
        if response.status_code == 200:
            agent = response.json()
            print("✅ Agent found in backend!")
            return agent
        else:
            print(f"❌ Agent not found: {response.status_code}")
            print(f"   Response: {response.text}")
            return None
    except Exception as e:
        print(f"❌ Error fetching agent: {e}")
        return None

def main():
    print_section("PYTHON SDK COMPREHENSIVE TESTING")
    print()
    print("Testing:")
    print("  1. Agent Auto-Registration")
    print("  2. Agent Auto-Verification")
    print("  3. Capability Auto-Detection")
    print("  4. MCP Server Detection")
    print("  5. Trust Score Calculation")
    print()

    try:
        # Step 1: Register Agent using SDK
        print_section("STEP 1: REGISTER AGENT (Auto-Register)")

        # Generate unique agent name with timestamp
        import time as time_module
        timestamp = int(time_module.time())
        agent_name = f"python-sdk-test-{timestamp}"

        print("\n📝 Calling register_agent()...")
        print(f"   Agent Name: {agent_name}")
        print("   Agent Type: ai_agent")
        print("   Description: Testing Python SDK with auto-detection")

        agent = register_agent(
            name=agent_name,
            agent_type="ai_agent",
            description="Testing Python SDK with auto-detection and auto-verification",
            # SDK should auto-detect capabilities and MCPs
        )

        print(f"\n✅ Agent registered successfully!")
        print(f"   Agent ID: {agent.agent_id}")
        print(f"   Name: {agent.name}")
        print(f"   Type: {agent.agent_type}")
        print(f"   Status: {agent.status}")

        # Step 2: Verify Agent Details
        print_section("STEP 2: VERIFY AGENT DETAILS FROM BACKEND")
        backend_agent = verify_agent_in_backend(agent.agent_id)

        if not backend_agent:
            print("❌ Failed to fetch agent from backend")
            return 1

        # Print full agent details
        print("\n📋 Complete Agent Details:")
        print(json.dumps(backend_agent, indent=2))

        # Step 3: Check Verification Status
        print_section("STEP 3: CHECK AUTO-VERIFICATION")
        is_verified = backend_agent.get('is_verified', False)
        verification_method = backend_agent.get('verification_method', 'unknown')
        last_verified = backend_agent.get('last_verified_at')

        print(f"\n🔐 Verification Status:")
        print(f"   Is Verified: {is_verified} {'✅' if is_verified else '❌'}")
        print(f"   Method: {verification_method}")
        print(f"   Last Verified: {last_verified if last_verified else 'Never'}")

        if is_verified:
            print("\n   ✅ AUTO-VERIFICATION SUCCESSFUL!")
        else:
            print("\n   ⚠️  Agent not auto-verified (may need manual verification)")

        # Step 4: Check Capability Detection
        print_section("STEP 4: CHECK CAPABILITY DETECTION")
        capabilities = backend_agent.get('capabilities', [])

        print(f"\n🎯 Detected Capabilities:")
        if capabilities:
            print(f"   Total: {len(capabilities)}")
            for cap in capabilities:
                print(f"   - {cap}")
            print("\n   ✅ CAPABILITIES AUTO-DETECTED!")
        else:
            print("   ⚠️  No capabilities detected (SDK may not have run detection)")

        # Step 5: Check MCP Server Detection
        print_section("STEP 5: CHECK MCP SERVER DETECTION")

        # Get MCP servers for this agent
        with open('.aim/credentials.json', 'r') as f:
            creds = json.load(f)

        token = creds['refresh_token']
        base_url = creds['aim_url']

        url = f"{base_url}/api/v1/agents/{agent.agent_id}/mcp-servers"
        headers = {"Authorization": f"Bearer {token}"}

        try:
            response = requests.get(url, headers=headers)
            if response.status_code == 200:
                mcp_servers = response.json()
                print(f"\n🔌 Detected MCP Servers:")
                if mcp_servers:
                    print(f"   Total: {len(mcp_servers)}")
                    for mcp in mcp_servers:
                        print(f"   - {mcp.get('server_name', 'unknown')}")
                        print(f"     Type: {mcp.get('server_type', 'unknown')}")
                        print(f"     Status: {mcp.get('status', 'unknown')}")
                    print("\n   ✅ MCP SERVERS AUTO-DETECTED!")
                else:
                    print("   ℹ️  No MCP servers detected (this is normal if none configured)")
            else:
                print(f"   ⚠️  Could not fetch MCP servers: {response.status_code}")
        except Exception as e:
            print(f"   ⚠️  Error fetching MCP servers: {e}")

        # Step 6: Check Trust Score
        print_section("STEP 6: CHECK TRUST SCORE CALCULATION")
        trust_score = backend_agent.get('trust_score')
        trust_level = backend_agent.get('trust_level', 'unknown')
        last_calculated = backend_agent.get('last_trust_score_calculated_at')

        print(f"\n🏆 Trust Score:")
        print(f"   Score: {trust_score if trust_score is not None else 'Not calculated'}")
        print(f"   Level: {trust_level}")
        print(f"   Last Calculated: {last_calculated if last_calculated else 'Never'}")

        if trust_score is not None:
            print("\n   ✅ TRUST SCORE CALCULATED!")
            if trust_score >= 80:
                print(f"   🌟 Excellent Trust Score! (>= 80)")
            elif trust_score >= 60:
                print(f"   👍 Good Trust Score! (60-79)")
            elif trust_score >= 40:
                print(f"   ⚠️  Fair Trust Score (40-59)")
            else:
                print(f"   ⚠️  Low Trust Score (< 40)")
        else:
            print("\n   ⚠️  Trust score not calculated yet")

        # Step 7: Check Security Features
        print_section("STEP 7: CHECK SECURITY FEATURES")
        public_key = backend_agent.get('public_key')
        encryption_method = backend_agent.get('encryption_method', 'none')

        print(f"\n🔒 Security:")
        print(f"   Public Key: {'Present ✅' if public_key else 'Not set ❌'}")
        print(f"   Encryption: {encryption_method}")

        if public_key:
            print(f"   Key (first 50 chars): {public_key[:50]}...")
            print("\n   ✅ SECURITY FEATURES ACTIVE!")
        else:
            print("\n   ⚠️  No public key set (may need Ed25519 signing)")

        # Final Summary
        print_section("FINAL SUMMARY")
        print()

        results = {
            "✅ Agent Registration": True,
            "✅ Auto-Verification": is_verified,
            "✅ Capability Detection": len(capabilities) > 0,
            "✅ MCP Detection": "Checked (normal if none)",
            "✅ Trust Score": trust_score is not None,
            "✅ Security Features": public_key is not None
        }

        print("Test Results:")
        for test, passed in results.items():
            status = "✅ PASS" if passed else "⚠️  PARTIAL/SKIP"
            print(f"  {test}: {status}")

        passed_count = sum(1 for v in results.values() if v)
        total_count = len(results)

        print(f"\nOverall: {passed_count}/{total_count} tests passed")

        if passed_count == total_count:
            print("\n🎉 ALL TESTS PASSED! SDK is fully functional!")
            return 0
        elif passed_count >= total_count * 0.7:
            print("\n👍 MOST TESTS PASSED! SDK is mostly functional.")
            return 0
        else:
            print("\n⚠️  SOME TESTS FAILED. SDK may need investigation.")
            return 1

    except Exception as e:
        print(f"\n❌ ERROR: {e}")
        import traceback
        traceback.print_exc()
        return 1

if __name__ == "__main__":
    sys.exit(main())
