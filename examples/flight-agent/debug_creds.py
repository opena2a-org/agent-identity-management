#!/usr/bin/env python3
"""Debug credentials loading"""

import sys
import os
import json
from pathlib import Path

sys.path.insert(0, os.path.join(os.path.dirname(__file__), 'aim-sdk-python'))

from aim_sdk.oauth import load_sdk_credentials

print("="*80)
print("DEBUGGING CREDENTIALS")
print("="*80 + "\n")

# Check SDK credentials
print("1. SDK Credentials (from .aim/credentials.encrypted):")
sdk_creds = load_sdk_credentials()
if sdk_creds:
    print(f"   ✅ Found SDK credentials")
    print(f"   Keys: {list(sdk_creds.keys())}")
    print(f"   aim_url: {sdk_creds.get('aim_url')}")
    print(f"   Has refresh_token: {'refresh_token' in sdk_creds}")
    print(f"   user_id: {sdk_creds.get('user_id')}")
else:
    print(f"   ❌ No SDK credentials")
print()

# Check agent credentials
print("2. Agent Credentials (from ~/.aim/credentials.json):")
creds_path = Path.home() / ".aim" / "credentials.json"
if creds_path.exists():
    with open(creds_path, 'r') as f:
        all_creds = json.load(f)

    if 'flight-search-agent' in all_creds:
        agent_creds = all_creds['flight-search-agent']
        print(f"   ✅ Found agent credentials")
        print(f"   Keys: {list(agent_creds.keys())}")
        print(f"   agent_id: {agent_creds.get('agent_id')}")
        print(f"   Has private_key: {'private_key' in agent_creds}")
        print(f"   Has public_key: {'public_key' in agent_creds}")
        print(f"   aim_url: {agent_creds.get('aim_url')}")
    else:
        print(f"   ❌ No flight-search-agent credentials")

    # Check root level tokens
    print()
    print("3. Root Level Tokens (from ~/.aim/credentials.json):")
    print(f"   Has refresh_token: {'refresh_token' in all_creds}")
    print(f"   Has sdk_token_id: {'sdk_token_id' in all_creds}")
    if 'refresh_token' in all_creds:
        print(f"   refresh_token: {all_creds['refresh_token'][:50]}...")
else:
    print(f"   ❌ No credentials file at {creds_path}")
print()

# The issue
print("="*80)
print("THE PROBLEM:")
print("="*80)
print()
print("When loading existing agent credentials, the SDK is:")
print("1. ✅ Loading agent-specific keys (private_key, public_key, agent_id)")
print("2. ❌ NOT loading OAuth tokens (refresh_token from root level)")
print()
print("This means AIMClient is created with:")
print("  ✅ private_key")
print("  ✅ public_key  ")
print("  ❌ oauth_token_manager (None, because no refresh_token in agent_creds)")
print()
print("Solution: Merge root-level tokens with agent credentials when loading")
