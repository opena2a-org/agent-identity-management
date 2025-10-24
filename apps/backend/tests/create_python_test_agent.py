#!/usr/bin/env python3
"""
Create Python SDK Test Agent
Creates a test agent for Python SDK validation, matching Go and JavaScript test agents.
"""

import os
import sys
import json
import requests
from pathlib import Path

# Add SDK to path
sdk_path = os.path.join(os.path.dirname(__file__), 'sdks', 'python')
sys.path.insert(0, sdk_path)

from aim_sdk.oauth import OAuthTokenManager

# Load OAuth credentials and get access token
creds_path = Path.home() / ".aim" / "credentials.json"
if not creds_path.exists():
    print("❌ No credentials found. Please login to AIM dashboard first.")
    sys.exit(1)

with open(creds_path, 'r') as f:
    creds = json.load(f)

aim_url = creds.get('aim_url', 'http://localhost:8080')

# Use OAuth token manager to get access token
token_manager = OAuthTokenManager(str(creds_path))
access_token = token_manager.get_access_token()

if not access_token:
    print("❌ Failed to get OAuth access token. Token may have expired.")
    print("   Please re-download SDK from dashboard: http://localhost:8080/dashboard/sdk")
    sys.exit(1)

# Generate Ed25519 keypair
from nacl.signing import SigningKey
import base64

signing_key = SigningKey.generate()
private_key_bytes = bytes(signing_key) + bytes(signing_key.verify_key)  # 64 bytes
public_key_bytes = bytes(signing_key.verify_key)

private_key_b64 = base64.b64encode(private_key_bytes).decode('utf-8')
public_key_b64 = base64.b64encode(public_key_bytes).decode('utf-8')

# Create agent
agent_data = {
    "name": "python-sdk-test-agent",
    "display_name": "Python SDK Test Agent",
    "description": "Test agent for Python SDK validation and capability detection",
    "agent_type": "ai_agent",
    "version": "1.0.0",
    "public_key": public_key_b64
}

print("Creating Python SDK Test Agent...")
print(f"📡 AIM URL: {aim_url}")

response = requests.post(
    f"{aim_url}/api/v1/agents",
    json=agent_data,
    headers={
        "Authorization": f"Bearer {access_token}",
        "Content-Type": "application/json"
    }
)

if response.status_code not in [200, 201]:
    print(f"❌ Failed to create agent: {response.status_code}")
    print(f"   {response.text}")
    sys.exit(1)

agent = response.json()
agent_id = agent.get('id') or agent.get('agent_id')

print(f"✅ Python SDK Test Agent created successfully!")
print(f"   Agent ID: {agent_id}")
print(f"   Name: {agent.get('name')}")
print(f"   Status: {agent.get('status')}")
print(f"   Trust Score: {agent.get('trust_score', 0)}")
print()

# Generate API key for the agent
print("Generating API key...")
api_key_data = {
    "name": "Python SDK Test Key",
    "agent_id": agent_id,
    "expires_at": None  # No expiration
}

response = requests.post(
    f"{aim_url}/api/v1/api-keys",
    json=api_key_data,
    headers={
        "Authorization": f"Bearer {access_token}",
        "Content-Type": "application/json"
    }
)

if response.status_code not in [200, 201]:
    print(f"⚠️  Warning: Failed to create API key: {response.status_code}")
    print(f"   {response.text}")
else:
    api_key_response = response.json()
    api_key = api_key_response.get('key')

    print(f"✅ API key generated!")
    print(f"   Key: {api_key}")
    print()

# Save credentials for testing
test_creds = {
    "agent_id": agent_id,
    "public_key": public_key_b64,
    "private_key": private_key_b64,
    "api_key": api_key if 'api_key' in locals() else None,
    "aim_url": aim_url,
    "name": "python-sdk-test-agent"
}

test_creds_path = Path(__file__).parent / "python_sdk_test_credentials.json"
with open(test_creds_path, 'w') as f:
    json.dump(test_creds, f, indent=2)

os.chmod(test_creds_path, 0o600)

print(f"✅ Test credentials saved to: {test_creds_path}")
print()
print("🎯 Next steps:")
print(f"   1. View agent in dashboard: {aim_url}/dashboard/agents/{agent_id}")
print(f"   2. Run Python SDK test with this agent")
print()
