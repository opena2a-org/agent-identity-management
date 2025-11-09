#!/usr/bin/env python3
"""
Test SDK verification by creating a new agent and triggering violations
"""

import requests
import json
import base64
import time
from datetime import datetime, timezone
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
from cryptography.hazmat.primitives import serialization

# Configuration
BASE_URL = "http://localhost:8080"

# Generate key pair
private_key = Ed25519PrivateKey.generate()
public_key_bytes = private_key.public_key().public_bytes(
    encoding=serialization.Encoding.Raw,
    format=serialization.PublicFormat.Raw
)
public_key_b64 = base64.b64encode(public_key_bytes).decode()

# Step 1: Create a new agent
print("📝 Step 1: Creating new agent...")
create_response = requests.post(
    f"{BASE_URL}/api/v1/agents",
    json={
        "name": f"test-agent-{int(time.time())}",
        "display_name": "Test Violation Agent",
        "description": "Agent for testing violations",
        "public_key": public_key_b64,
        "agent_type": "test",
        "status": "verified"  # Make it verified so we can test
    }
)

if create_response.status_code not in [200, 201]:
    print(f"❌ Failed to create agent: {create_response.status_code}")
    print(create_response.text)
    exit(1)

agent_data = create_response.json()
agent_id = agent_data["id"]
print(f"✅ Agent created: {agent_id}")

# Wait a moment for agent to be created
time.sleep(1)

def sign_payload(payload_dict, private_key):
    """Sign a payload using Ed25519"""
    message = json.dumps(payload_dict, sort_keys=True, separators=(', ', ': '))
    message_bytes = message.encode()
    signature_bytes = private_key.sign(message_bytes)
    return base64.b64encode(signature_bytes).decode()

# Step 2: Trigger violation with delete_database (high risk, no capability)
print("\n🧪 Step 2: Attempting delete_database (should trigger violation)")
timestamp = datetime.now(timezone.utc).isoformat().replace('+00:00', 'Z')

payload = {
    "agent_id": agent_id,
    "action_type": "delete_database",
    "resource": None,
    "context": {"risk_level": "critical"},
    "timestamp": timestamp
}

signature = sign_payload(payload, private_key)

request_payload = {
    **payload,
    "signature": signature,
    "public_key": public_key_b64
}

verify_response = requests.post(
    f"{BASE_URL}/api/v1/sdk-api/verifications",
    json=request_payload
)

print(f"Status: {verify_response.status_code}")
print(f"Response: {verify_response.json()}")

# Step 3: Check backend logs for violation creation
print("\n📊 Step 3: Checking backend logs...")
time.sleep(1)
result = requests.get(f"{BASE_URL}/api/v1/agents/{agent_id}/violations")
if result.status_code == 200:
    violations_data = result.json()
    print(f"✅ Agent violations: {len(violations_data.get('violations', []))}")
    if len(violations_data.get('violations', [])) > 0:
        print(f"   First violation: {violations_data['violations'][0]}")
else:
    print(f"⚠️  Could not fetch agent violations: {result.status_code}")

# Step 4: Check trust score was updated
trust_response = requests.get(f"{BASE_URL}/api/v1/agents/{agent_id}")
if trust_response.status_code == 200:
    agent_info = trust_response.json()
    print(f"\n🎯 Trust Score: {agent_info.get('trust_score', 0):.2f}%")
else:
    print(f"⚠️  Could not fetch agent info: {trust_response.status_code}")
