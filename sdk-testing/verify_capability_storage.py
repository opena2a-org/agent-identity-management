#!/usr/bin/env python3
"""
Verification script to test that SDK-detected capabilities are now stored in the database.

This script will:
1. Create a new test agent with capabilities via SDK
2. Verify capabilities are stored in database
3. Report results
"""

import requests
import json
import sys

def main():
    print("\n" + "="*60)
    print("🔍 CAPABILITY STORAGE VERIFICATION TEST")
    print("="*60 + "\n")

    # Load credentials
    with open('/Users/decimai/.aim/credentials.json', 'r') as f:
        creds = json.load(f)

    base_url = creds['aim_url']
    token = creds['refresh_token']
    headers = {'Authorization': f'Bearer {token}'}

    # Test 1: Create agent with capabilities
    print("📝 Test 1: Creating agent with capabilities...")

    agent_payload = {
        "name": f"capability-test-agent-{int(__import__('time').time())}",
        "display_name": "Capability Test Agent",
        "description": "Testing capability storage fix",
        "agent_type": "ai_agent",
        "capabilities": [
            "execute_code",
            "read_files",
            "write_files",
            "make_api_calls",
            "send_email"
        ]
    }

    try:
        response = requests.post(
            f'{base_url}/api/v1/agents',
            headers=headers,
            json=agent_payload,
            timeout=10
        )

        if response.status_code == 201:
            agent = response.json()
            agent_id = agent['id']
            print(f"✅ Agent created: {agent_id}")

            # Test 2: Verify capabilities in database
            print("\n📊 Test 2: Verifying capabilities in database...")

            get_response = requests.get(
                f'{base_url}/api/v1/agents/{agent_id}',
                headers=headers,
                timeout=10
            )

            if get_response.status_code == 200:
                stored_agent = get_response.json()
                stored_capabilities = stored_agent.get('capabilities')

                print(f"\n📦 Stored capabilities: {stored_capabilities}")
                print(f"📦 Expected capabilities: {agent_payload['capabilities']}")

                if stored_capabilities == agent_payload['capabilities']:
                    print("\n✅ ✅ ✅ SUCCESS! Capabilities are correctly stored in database!")
                    print("\n🎉 FIX VERIFIED: agent_service.go now stores capabilities")
                    return 0
                elif stored_capabilities is None:
                    print("\n❌ FAILED: Capabilities are NULL in database")
                    print("⚠️  The fix did not work - capabilities field is still null")
                    return 1
                else:
                    print(f"\n⚠️  PARTIAL: Capabilities stored but don't match")
                    print(f"   Expected: {agent_payload['capabilities']}")
                    print(f"   Got: {stored_capabilities}")
                    return 1
            else:
                print(f"\n❌ Failed to retrieve agent: {get_response.status_code}")
                print(f"Response: {get_response.text}")
                return 1

        elif response.status_code == 500:
            error_text = response.text
            if "duplicate key" in error_text:
                print("⚠️  Agent name already exists, trying with different name...")
                # Could retry with different name
            print(f"\n❌ Server error: {error_text}")
            return 1
        else:
            print(f"\n❌ Failed to create agent: {response.status_code}")
            print(f"Response: {response.text}")
            return 1

    except Exception as e:
        print(f"\n❌ Error: {e}")
        return 1

if __name__ == "__main__":
    sys.exit(main())
