#!/usr/bin/env python3
"""
Test script for new credential management features (Option 1 enhancements)
Tests:
1. from_credentials() - Load existing credentials
2. auto_register_or_load() - Smart registration/loading
"""

import os
import sys
import json
from pathlib import Path

# Add SDK to path
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "aim_sdk"))

from aim_sdk import AIMClient, register_agent

AIM_URL = "http://localhost:8080"

def cleanup_test_credentials(agent_name: str):
    """Remove test credentials if they exist"""
    creds_path = Path.home() / ".aim" / "credentials.json"
    if creds_path.exists():
        with open(creds_path) as f:
            creds = json.load(f)

        # Remove test agent from credentials
        if creds.get("name") == agent_name:
            creds_path.unlink()
            print(f"✅ Cleaned up credentials for '{agent_name}'")

def test_from_credentials_nonexistent():
    """Test 1: from_credentials() with non-existent agent should raise FileNotFoundError"""
    print("\n=== Test 1: from_credentials() with non-existent agent ===")

    cleanup_test_credentials("nonexistent-agent")

    try:
        client = AIMClient.from_credentials("nonexistent-agent", AIM_URL)
        print("❌ FAILED: Should have raised FileNotFoundError")
        return False
    except FileNotFoundError as e:
        print(f"✅ PASSED: Correctly raised FileNotFoundError")
        print(f"   Error message: {str(e)[:100]}...")
        return True

def test_auto_register_or_load_first_run():
    """Test 2: auto_register_or_load() on first run should register"""
    print("\n=== Test 2: auto_register_or_load() first run (registration) ===")

    agent_name = "test-credential-agent-1"
    cleanup_test_credentials(agent_name)

    try:
        client = AIMClient.auto_register_or_load(
            name=agent_name,
            aim_url=AIM_URL,
            display_name="Test Credential Agent",
            description="Testing credential management",
            agent_type="ai_agent",
            version="1.0.0"
        )

        print(f"✅ PASSED: Agent registered successfully")
        print(f"   Agent ID: {client.agent_id}")
        print(f"   Public Key: {client.public_key[:32]}...")

        # Verify credentials file exists
        creds_path = Path.home() / ".aim" / "credentials.json"
        if creds_path.exists():
            print(f"✅ Credentials file created at {creds_path}")

            # Verify agent is in the credentials file
            with open(creds_path) as f:
                creds_data = json.load(f)

            found = False
            for agent in creds_data.get("agents", []):
                if agent["name"] == agent_name:
                    found = True
                    print(f"✅ Agent '{agent_name}' found in credentials file")
                    break

            if not found:
                print(f"❌ WARNING: Agent '{agent_name}' not found in credentials file")
        else:
            print(f"❌ WARNING: Credentials file not found")

        return True
    except Exception as e:
        print(f"❌ FAILED: {e}")
        return False

def test_auto_register_or_load_second_run():
    """Test 3: auto_register_or_load() on second run should load from file"""
    print("\n=== Test 3: auto_register_or_load() second run (loading) ===")

    agent_name = "test-credential-agent-1"

    # Read original credentials from the multi-agent format
    creds_path = Path.home() / ".aim" / "credentials.json"
    with open(creds_path) as f:
        creds_data = json.load(f)

    # Find the agent in the agents array
    original_agent_id = None
    for agent in creds_data.get("agents", []):
        if agent["name"] == agent_name:
            original_agent_id = agent["agent_id"]
            break

    if not original_agent_id:
        print(f"❌ SETUP FAILED: Could not find agent '{agent_name}' in credentials")
        return False

    try:
        # This should load from file, NOT register again
        client = AIMClient.auto_register_or_load(
            name=agent_name,
            aim_url=AIM_URL
        )

        if client.agent_id == original_agent_id:
            print(f"✅ PASSED: Loaded existing credentials")
            print(f"   Agent ID: {client.agent_id} (same as original)")
            return True
        else:
            print(f"❌ FAILED: Got different agent ID (registered new instead of loading)")
            print(f"   Original: {original_agent_id}")
            print(f"   New: {client.agent_id}")
            return False

    except Exception as e:
        print(f"❌ FAILED: {e}")
        return False

def test_from_credentials_after_registration():
    """Test 4: from_credentials() after registration should work"""
    print("\n=== Test 4: from_credentials() after registration ===")

    agent_name = "test-credential-agent-1"

    try:
        client = AIMClient.from_credentials(agent_name, AIM_URL)
        print(f"✅ PASSED: Successfully loaded credentials")
        print(f"   Agent ID: {client.agent_id}")
        return True
    except Exception as e:
        print(f"❌ FAILED: {e}")
        return False

def test_force_register():
    """Test 5: auto_register_or_load() with force_register=True should register new agent"""
    print("\n=== Test 5: auto_register_or_load() with force_register=True ===")

    import time
    agent_name = f"test-credential-force-{int(time.time())}"
    cleanup_test_credentials(agent_name)

    try:
        # First: Create credentials file to simulate existing agent
        creds_path = Path.home() / ".aim" / "credentials.json"
        if creds_path.exists():
            with open(creds_path) as f:
                existing_data = json.load(f)
        else:
            existing_data = {"version": "1.0", "default_agent": None, "agents": []}

        # Add fake credentials to test force_register bypass
        # Use valid Ed25519 keys (both 32 bytes)
        fake_creds = {
            "name": agent_name,
            "agent_id": "00000000-0000-0000-0000-000000000000",
            "public_key": "2UlKiu19oIAlLQ3EjJcZmgstEG38WF2wm8BP170jrHs=",  # Valid Ed25519 public key (32 bytes)
            "private_key": "iJA6V1nCF/yaZdEJjsYSDiIGDXm57gadTZTQZLLhiRQ=",  # Valid Ed25519 private key (32 bytes)
            "aim_url": AIM_URL,
            "status": "pending",
            "trust_score": 50,
            "registered_at": "2025-01-01T00:00:00+00:00",
            "last_rotated_at": None,
            "rotation_count": 0
        }
        existing_data["agents"].append(fake_creds)

        with open(creds_path, 'w') as f:
            json.dump(existing_data, f, indent=2)

        # Without force_register: should load fake credentials
        client1 = AIMClient.auto_register_or_load(
            name=agent_name,
            aim_url=AIM_URL
        )

        if client1.agent_id == "00000000-0000-0000-0000-000000000000":
            print(f"✅ Part 1 PASSED: Loaded existing fake credentials")
            print(f"   Agent ID: {client1.agent_id}")
        else:
            print(f"❌ Part 1 FAILED: Should have loaded fake credentials")
            cleanup_test_credentials(agent_name)
            return False

        # With force_register: should bypass and register new
        client2 = AIMClient.auto_register_or_load(
            name=agent_name,
            aim_url=AIM_URL,
            display_name="Force Register Test",
            force_register=True
        )

        if client2.agent_id != "00000000-0000-0000-0000-000000000000":
            print(f"✅ Part 2 PASSED: Force registration bypassed credentials and registered new agent")
            print(f"   Old (fake) ID: 00000000-0000-0000-0000-000000000000")
            print(f"   New (real) ID: {client2.agent_id}")
            cleanup_test_credentials(agent_name)
            return True
        else:
            print(f"❌ Part 2 FAILED: Got fake ID (should have registered new)")
            cleanup_test_credentials(agent_name)
            return False

    except Exception as e:
        print(f"❌ FAILED: {e}")
        cleanup_test_credentials(agent_name)
        return False

def main():
    """Run all credential management tests"""
    print("=" * 70)
    print("AIM SDK - Credential Management Tests (Option 1 Enhancements)")
    print("=" * 70)

    results = []

    # Test 1: Non-existent credentials
    results.append(("from_credentials() with non-existent agent", test_from_credentials_nonexistent()))

    # Test 2: First run (registration)
    results.append(("auto_register_or_load() first run", test_auto_register_or_load_first_run()))

    # Test 3: Second run (loading)
    results.append(("auto_register_or_load() second run", test_auto_register_or_load_second_run()))

    # Test 4: from_credentials() after registration
    results.append(("from_credentials() after registration", test_from_credentials_after_registration()))

    # Test 5: Force re-registration
    results.append(("auto_register_or_load() with force_register", test_force_register()))

    # Cleanup
    cleanup_test_credentials("test-credential-agent-1")

    # Summary
    print("\n" + "=" * 70)
    print("TEST SUMMARY")
    print("=" * 70)

    passed = sum(1 for _, result in results if result)
    total = len(results)

    for test_name, result in results:
        status = "✅ PASSED" if result else "❌ FAILED"
        print(f"{status}: {test_name}")

    print(f"\nTotal: {passed}/{total} tests passed")

    if passed == total:
        print("\n🎉 ALL TESTS PASSED - Credential management working correctly!")
        return 0
    else:
        print(f"\n⚠️  {total - passed} test(s) failed - review output above")
        return 1

if __name__ == "__main__":
    sys.exit(main())
