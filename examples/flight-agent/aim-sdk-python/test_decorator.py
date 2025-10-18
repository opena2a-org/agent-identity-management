#!/usr/bin/env python3
"""
Test the @aim_verify universal decorator

Demonstrates how developers can use @aim_verify on ANY Python function
to automatically verify actions with the AIM backend.
"""

import sys
import os
import time
from pathlib import Path

# Add SDK to path
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "aim_sdk"))

from aim_sdk import AIMClient, aim_verify, aim_verify_database, aim_verify_api_call

AIM_URL = "http://localhost:8080"


def test_decorator_with_explicit_client():
    """Test 1: Using decorator with explicit AIM client"""
    print("\n" + "=" * 70)
    print("TEST 1: Decorator with Explicit Client")
    print("=" * 70)

    try:
        # Register/load agent
        aim_client = AIMClient.auto_register_or_load("decorator-test-agent", AIM_URL)
        print(f"✅ AIM client initialized: {aim_client.agent_id}")

        # Define a function with AIM verification
        @aim_verify(aim_client, action_type="database_query", risk_level="high")
        def delete_user(user_id: str):
            """Simulates deleting a user from database"""
            print(f"   💾 Executing: DELETE FROM users WHERE id = '{user_id}'")
            return {"deleted": True, "user_id": user_id}

        # Call the function - verification happens automatically
        print("\n🔍 Calling delete_user('user123')...")
        result = delete_user("user123")
        print(f"✅ Function executed successfully: {result}")

        print("\n🎉 TEST 1 PASSED - Decorator with explicit client works!")
        return True

    except Exception as e:
        print(f"\n❌ TEST 1 FAILED: {e}")
        import traceback
        traceback.print_exc()
        return False


def test_decorator_with_auto_init():
    """Test 2: Using decorator with auto-initialization from environment"""
    print("\n" + "=" * 70)
    print("TEST 2: Decorator with Auto-Initialization")
    print("=" * 70)

    try:
        # Set environment variables
        os.environ["AIM_AGENT_NAME"] = "decorator-test-agent"
        os.environ["AIM_URL"] = AIM_URL
        os.environ["AIM_AUTO_REGISTER"] = "true"

        print(f"✅ Environment configured:")
        print(f"   AIM_AGENT_NAME={os.environ['AIM_AGENT_NAME']}")
        print(f"   AIM_URL={os.environ['AIM_URL']}")

        # Define function with auto-init (no explicit client needed)
        @aim_verify(auto_init=True, action_type="api_call", risk_level="medium")
        def call_external_api(endpoint: str):
            """Simulates calling an external API"""
            print(f"   🌐 Executing: GET {endpoint}")
            return {"status": "success", "endpoint": endpoint}

        # Call the function - client auto-initializes and verifies
        print("\n🔍 Calling call_external_api('/users/profile')...")
        result = call_external_api("/users/profile")
        print(f"✅ Function executed successfully: {result}")

        print("\n🎉 TEST 2 PASSED - Auto-initialization works!")
        return True

    except Exception as e:
        print(f"\n❌ TEST 2 FAILED: {e}")
        import traceback
        traceback.print_exc()
        return False


def test_convenience_decorators():
    """Test 3: Using convenience decorators"""
    print("\n" + "=" * 70)
    print("TEST 3: Convenience Decorators")
    print("=" * 70)

    try:
        aim_client = AIMClient.auto_register_or_load("decorator-test-agent", AIM_URL)
        print(f"✅ AIM client initialized: {aim_client.agent_id}")

        # Test database decorator
        @aim_verify_database(aim_client)
        def query_users():
            print("   💾 Executing: SELECT * FROM users")
            return [{"id": "1", "name": "Alice"}, {"id": "2", "name": "Bob"}]

        # Test API call decorator
        @aim_verify_api_call(aim_client)
        def fetch_weather(city: str):
            print(f"   🌐 Executing: GET /weather?city={city}")
            return {"city": city, "temp": 72, "condition": "sunny"}

        print("\n🔍 Calling query_users()...")
        users = query_users()
        print(f"✅ Database query executed: {len(users)} users returned")

        print("\n🔍 Calling fetch_weather('San Francisco')...")
        weather = fetch_weather("San Francisco")
        print(f"✅ API call executed: {weather}")

        print("\n🎉 TEST 3 PASSED - Convenience decorators work!")
        return True

    except Exception as e:
        print(f"\n❌ TEST 3 FAILED: {e}")
        import traceback
        traceback.print_exc()
        return False


def test_decorator_preserves_metadata():
    """Test 4: Verify decorator preserves function metadata"""
    print("\n" + "=" * 70)
    print("TEST 4: Function Metadata Preservation")
    print("=" * 70)

    try:
        aim_client = AIMClient.auto_register_or_load("decorator-test-agent", AIM_URL)

        @aim_verify(aim_client)
        def example_function(x: int, y: int) -> int:
            """Adds two numbers together"""
            return x + y

        # Check metadata
        assert example_function.__name__ == "example_function", "Function name not preserved"
        assert "Adds two numbers" in example_function.__doc__, "Docstring not preserved"
        print(f"✅ Function name preserved: {example_function.__name__}")
        print(f"✅ Docstring preserved: {example_function.__doc__}")

        # Test execution
        result = example_function(5, 3)
        assert result == 8, f"Expected 8, got {result}"
        print(f"✅ Function executes correctly: example_function(5, 3) = {result}")

        print("\n🎉 TEST 4 PASSED - Metadata preservation works!")
        return True

    except Exception as e:
        print(f"\n❌ TEST 4 FAILED: {e}")
        import traceback
        traceback.print_exc()
        return False


def main():
    """Run all decorator tests"""
    print("=" * 70)
    print("AIM Universal Decorator Tests")
    print("=" * 70)
    print(f"AIM Server: {AIM_URL}")
    print()

    results = []

    # Test 1: Explicit client
    test1_passed = test_decorator_with_explicit_client()
    results.append(("Explicit Client", test1_passed))

    # Test 2: Auto-initialization
    test2_passed = test_decorator_with_auto_init()
    results.append(("Auto-Initialization", test2_passed))

    # Test 3: Convenience decorators
    test3_passed = test_convenience_decorators()
    results.append(("Convenience Decorators", test3_passed))

    # Test 4: Metadata preservation
    test4_passed = test_decorator_preserves_metadata()
    results.append(("Metadata Preservation", test4_passed))

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
        print("\n🎉 ALL TESTS PASSED - Universal decorator working perfectly!")
        return 0
    else:
        print(f"\n⚠️  {total - passed} test(s) failed - review output above")
        return 1


if __name__ == "__main__":
    sys.exit(main())
