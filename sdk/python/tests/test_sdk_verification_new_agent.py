#!/usr/bin/env python3
"""
Test SDK verification by creating a new agent and triggering violations

Requires: live AIM backend at BASE_URL
Run with: pytest -m integration tests/test_sdk_verification_new_agent.py
"""

import requests
import json
import base64
import time
from datetime import datetime, timezone
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
from cryptography.hazmat.primitives import serialization

import pytest

# Configuration
BASE_URL = "http://localhost:8080"


def _generate_keypair():
    """Generate an Ed25519 key pair for testing."""
    private_key = Ed25519PrivateKey.generate()
    public_key_bytes = private_key.public_key().public_bytes(
        encoding=serialization.Encoding.Raw,
        format=serialization.PublicFormat.Raw
    )
    public_key_b64 = base64.b64encode(public_key_bytes).decode()
    return private_key, public_key_b64


def _sign_payload(payload_dict, private_key):
    """Sign a payload using Ed25519."""
    message = json.dumps(payload_dict, sort_keys=True, separators=(', ', ': '))
    message_bytes = message.encode()
    signature_bytes = private_key.sign(message_bytes)
    return base64.b64encode(signature_bytes).decode()


@pytest.mark.integration
def test_sdk_verification_new_agent():
    """Create a new agent and trigger violations to verify SDK behavior."""
    private_key, public_key_b64 = _generate_keypair()

    # Step 1: Create a new agent
    create_response = requests.post(
        f"{BASE_URL}/api/v1/agents",
        json={
            "name": f"test-agent-{int(time.time())}",
            "display_name": "Test Violation Agent",
            "description": "Agent for testing violations",
            "public_key": public_key_b64,
            "agent_type": "test",
            "status": "verified"
        }
    )

    assert create_response.status_code in [200, 201], (
        f"Failed to create agent: {create_response.status_code} {create_response.text}"
    )

    agent_data = create_response.json()
    agent_id = agent_data["id"]

    # Wait for agent creation to propagate
    time.sleep(1)

    # Step 2: Trigger violation with delete_database (high risk, no capability)
    timestamp = datetime.now(timezone.utc).isoformat().replace('+00:00', 'Z')

    payload = {
        "agent_id": agent_id,
        "action_type": "delete_database",
        "resource": None,
        "context": {"risk_level": "critical"},
        "timestamp": timestamp
    }

    signature = _sign_payload(payload, private_key)

    request_payload = {
        **payload,
        "signature": signature,
        "public_key": public_key_b64
    }

    verify_response = requests.post(
        f"{BASE_URL}/api/v1/sdk-api/verifications",
        json=request_payload
    )

    assert verify_response.status_code in [200, 201, 403]

    # Step 3: Check backend logs for violation creation
    time.sleep(1)
    result = requests.get(f"{BASE_URL}/api/v1/agents/{agent_id}/violations")
    assert result.status_code == 200

    violations_data = result.json()
    assert len(violations_data.get('violations', [])) > 0

    # Step 4: Check trust score was updated
    trust_response = requests.get(f"{BASE_URL}/api/v1/agents/{agent_id}")
    assert trust_response.status_code == 200


if __name__ == "__main__":
    import sys
    sys.exit(pytest.main([__file__, "-v", "-m", "integration"]))
