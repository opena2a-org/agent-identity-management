#!/usr/bin/env python3
"""Debug verification authentication"""

import sys
import os

sys.path.insert(0, os.path.join(os.path.dirname(__file__), 'aim-sdk-python'))

from aim_sdk import secure
import json

print("="*80)
print("DEBUGGING VERIFICATION AUTHENTICATION")
print("="*80 + "\n")

# Load agent
agent = secure('flight-search-agent')
print(f"✅ Agent loaded: {agent.agent_id}")
print(f"   API URL: {agent.aim_url}")
print()

# Check credentials
print("Checking credentials...")
if hasattr(agent, '_oauth_manager'):
    print(f"✅ OAuth manager exists")
    try:
        token = agent._oauth_manager.get_access_token()
        print(f"✅ Access token obtained: {token[:50]}...")
    except Exception as e:
        print(f"❌ Failed to get access token: {e}")
else:
    print(f"❌ No OAuth manager")
print()

# Check signing
print("Checking cryptographic signing...")
if hasattr(agent, 'private_key'):
    print(f"✅ Private key exists")
else:
    print(f"❌ No private key")

if hasattr(agent, 'public_key'):
    print(f"✅ Public key exists")
else:
    print(f"❌ No public key")
print()

# Try verification with detailed error
print("Attempting verification...")
try:
    result = agent.verify_action(
        action_type='test_action',
        resource='test_resource',
        context={'test': True}
    )
    print(f"✅ Verification succeeded!")
    print(f"   Result: {json.dumps(result, indent=2)}")
except Exception as e:
    print(f"❌ Verification failed: {e}")
    print()
    print("Full traceback:")
    import traceback
    traceback.print_exc()
    print()

    # Try to get more details
    if hasattr(e, '__dict__'):
        print("Error details:")
        print(json.dumps(e.__dict__, indent=2, default=str))
