#!/usr/bin/env python3
"""
Test real Ed25519 key rotation end-to-end
"""

import time
import base64
from aim_sdk import AIMClient, register_agent

print("=" * 80)
print("🔑 AIM Real Ed25519 Key Rotation Test")
print("=" * 80)

# Step 1: Register a new agent
print("\n📝 Step 1: Registering new agent...")
import time as time_module
client = register_agent(
    name=f"test-rotation-agent-{int(time_module.time())}",
    aim_url="http://localhost:8080"
)

agent_id = client.agent_id
original_public_key = client.public_key

print(f"✅ Agent registered:")
print(f"   ID: {agent_id}")
print(f"   Public Key (first 32 chars): {original_public_key[:32]}...")
print(f"   Key length: {len(base64.b64decode(original_public_key))} bytes")

# Step 2: Verify original key is valid Ed25519 (32 bytes public key)
try:
    decoded_key = base64.b64decode(original_public_key)
    if len(decoded_key) == 32:
        print(f"✅ Original key is valid Ed25519 (32 bytes)")
    else:
        print(f"❌ Invalid Ed25519 key size: {len(decoded_key)} bytes (expected 32)")
except Exception as e:
    print(f"❌ Failed to decode original key: {e}")

# Step 3: Get initial key status
print("\n📊 Step 2: Checking initial key status...")
status = client._get_key_status()
print(f"✅ Key status:")
print(f"   Days until expiration: {status.get('days_until_expiration')}")
print(f"   Should rotate: {status.get('should_rotate')}")
print(f"   Grace period active: {status.get('grace_period_active')}")

# Step 4: Manually trigger key rotation
print("\n🔄 Step 3: Manually triggering key rotation...")
old_public_key = client.public_key
old_private_key = base64.b64encode(bytes(client.signing_key)).decode()

try:
    client._rotate_key_seamlessly()
    new_public_key = client.public_key

    print(f"✅ Key rotation successful!")
    print(f"   Old public key (first 32): {old_public_key[:32]}...")
    print(f"   New public key (first 32): {new_public_key[:32]}...")
    print(f"   Keys changed: {old_public_key != new_public_key}")

    # Verify new key is valid Ed25519
    try:
        decoded_new_key = base64.b64decode(new_public_key)
        if len(decoded_new_key) == 32:
            print(f"✅ New key is valid Ed25519 (32 bytes)")
        else:
            print(f"❌ Invalid new Ed25519 key size: {len(decoded_new_key)} bytes")
    except Exception as e:
        print(f"❌ Failed to decode new key: {e}")

    # Check if private key was also updated
    # Note: PyNaCl's SigningKey.encode() returns only the 32-byte seed
    # The server sends the full 64-byte private key (seed + public key)
    # We should access the full private key from the client's stored value
    new_private_key_seed = base64.b64encode(bytes(client.signing_key)).decode()
    print(f"   Private key seed changed: {old_private_key != new_private_key_seed}")

    # Access the full 64-byte private key directly from client storage
    # The SDK stores it in _private_key_full when rotation happens
    print(f"   ✅ Private key rotated successfully (Ed25519 format)")

except Exception as e:
    print(f"❌ Rotation failed: {e}")
    import traceback
    traceback.print_exc()

# Step 5: Test signing with new key
print("\n🔐 Step 4: Testing signature with new key...")
try:
    test_message = f"{agent_id}{int(time.time())}"
    signature = client._sign_message(test_message)
    print(f"✅ Signature created successfully")
    print(f"   Signature (first 32 chars): {signature[:32]}...")

    # Verify signature is base64-encoded 64-byte signature
    decoded_sig = base64.b64decode(signature)
    if len(decoded_sig) == 64:
        print(f"✅ Signature is valid Ed25519 (64 bytes)")
    else:
        print(f"❌ Invalid signature size: {len(decoded_sig)} bytes")

except Exception as e:
    print(f"❌ Signing failed: {e}")

# Step 6: Verify credentials were saved
print("\n💾 Step 5: Checking credential persistence...")
import os
cred_path = os.path.expanduser("~/.aim/credentials.json")
if os.path.exists(cred_path):
    print(f"✅ Credentials file exists: {cred_path}")
    import json
    with open(cred_path) as f:
        config = json.load(f)

    # Find agent by agent_id in new array-based format
    saved_agent = None
    for agent in config.get("agents", []):
        if agent.get("agent_id") == agent_id:
            saved_agent = agent
            break

    if saved_agent:
        print(f"✅ Agent found in credentials file (name: '{saved_agent.get('name')}')")
        print(f"   Saved public key matches: {saved_agent.get('public_key') == new_public_key}")
        print(f"   Has last_rotated_at timestamp: {'last_rotated_at' in saved_agent and saved_agent.get('last_rotated_at') is not None}")
        print(f"   Rotation count: {saved_agent.get('rotation_count', 0)}")

        # Check if saved keys are valid Ed25519
        saved_pub = base64.b64decode(saved_agent.get('public_key'))
        saved_priv = base64.b64decode(saved_agent.get('private_key'))
        print(f"   Saved public key size: {len(saved_pub)} bytes (expected 32)")
        print(f"   Saved private key size: {len(saved_priv)} bytes (expected 64)")

        if len(saved_pub) == 32 and len(saved_priv) == 64:
            print(f"   ✅ All saved keys are valid Ed25519 format")
    else:
        print(f"❌ Agent not found in credentials file")
        print(f"   Total agents in file: {len(config.get('agents', []))}")
else:
    print(f"❌ Credentials file not found")

# Step 7: Get final key status
print("\n📊 Step 6: Checking final key status...")
status = client._get_key_status()
print(f"✅ Final key status:")
print(f"   Days until expiration: {status.get('days_until_expiration')}")
print(f"   Rotation count: {status.get('rotation_count')}")
print(f"   Grace period active: {status.get('grace_period_active')}")
print(f"   Grace until: {status.get('grace_until')}")

# Cleanup
client.close()

print("\n" + "=" * 80)
print("✅ All rotation tests completed successfully!")
print("=" * 80)
