#!/usr/bin/env python3
"""
Test SDK verification to trigger capability violations
"""

import requests
import json
import base64
import time
from datetime import datetime
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
from cryptography.hazmat.primitives import serialization

# Configuration
BASE_URL = "http://localhost:8080"
AGENT_ID = "c641db38-7101-41bd-9c4d-8e0714711ecf"  # test-agent

# Generate key pair (same as SDK)
private_key = Ed25519PrivateKey.generate()
public_key_bytes = private_key.public_key().public_bytes(
    encoding=serialization.Encoding.Raw,
    format=serialization.PublicFormat.Raw
)
public_key_b64 = base64.b64encode(public_key_bytes).decode()

def sign_payload(payload_dict, private_key):
    """Sign a payload using Ed25519 (same as SDK)"""
    # Create deterministic JSON matching Python's json.dumps(sort_keys=True)
    message = json.dumps(payload_dict, sort_keys=True, separators=(', ', ': '))
    message_bytes = message.encode()

    # Sign
    signature_bytes = private_key.sign(message_bytes)
    signature_b64 = base64.b64encode(signature_bytes).decode()

    return signature_b64

# Test 1: Trigger violation with delete_database (high risk, no capability)
print("🧪 Test 1: Attempting delete_database (should trigger violation)")
timestamp = datetime.utcnow().isoformat() + "Z"

payload = {
    "agent_id": AGENT_ID,
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

response = requests.post(
    f"{BASE_URL}/api/v1/sdk-api/verifications",
    json=request_payload
)

print(f"Status: {response.status_code}")
print(f"Response: {response.json()}")

# Check violations count
time.sleep(1)
violations_response = requests.get(f"{BASE_URL}/api/v1/violations")
if violations_response.status_code == 200:
    violations_data = violations_response.json()
    print(f"\n✅ Total violations: {violations_data.get('total', 0)}")
else:
    print(f"\n❌ Failed to fetch violations: {violations_response.status_code}")
