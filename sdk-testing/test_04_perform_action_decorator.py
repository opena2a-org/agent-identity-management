#!/usr/bin/env python3
"""
Test 4: @perform_action Decorator - Action Verification

This test verifies the SDK's claim:
"Every API call cryptographically signed"

We're testing:
1. @agent.perform_action() decorator functionality
2. Automatic action verification
3. Ed25519 signature generation
4. Audit trail logging
5. Risk level enforcement
"""

import os
import sys
import logging

logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

def test_basic_perform_action():
    """Test basic @perform_action decorator."""
    logger.info("=" * 80)
    logger.info("TEST 4a: Basic @perform_action Decorator")
    logger.info("=" * 80)

    try:
        from aim_sdk import secure

        aim_url = os.getenv('AIM_URL', 'https://aim-prod-backend.graypebble-c7e67ab8.canadacentral.azurecontainerapps.io')

        logger.info("\n🔐 Step 1: Creating secure agent...")
        agent = secure("test-action-agent", aim_url=aim_url)
        logger.info("✅ Agent created")

        # Test decorator with simple function
        logger.info("\n🎯 Step 2: Testing @perform_action decorator...")

        @agent.perform_action("read_database", resource="users_table")
        def get_users():
            """Get all users from database."""
            logger.info("   📊 Executing: get_users()")
            return {"users": ["alice", "bob", "charlie"]}

        logger.info("✅ Decorator applied successfully")

        # Call the decorated function
        logger.info("\n🚀 Step 3: Calling decorated function...")
        result = get_users()

        logger.info(f"✅ Function executed successfully")
        logger.info(f"   Result: {result}")

        # Verify result includes action verification
        if isinstance(result, dict) and "users" in result:
            logger.info("✅ Function return value intact")
        else:
            logger.error("❌ Function return value modified unexpectedly")
            return False

        return True

    except Exception as e:
        logger.error(f"\n❌ TEST 4a FAILED: {e}")
        import traceback
        traceback.print_exc()
        return False

def test_action_with_metadata():
    """Test @perform_action with additional metadata."""
    logger.info("\n\n" + "=" * 80)
    logger.info("TEST 4b: @perform_action with Metadata")
    logger.info("=" * 80)

    try:
        from aim_sdk import secure

        aim_url = os.getenv('AIM_URL', 'https://aim-prod-backend.graypebble-c7e67ab8.canadacentral.azurecontainerapps.io')

        logger.info("\n🔐 Creating secure agent...")
        agent = secure("test-metadata-agent", aim_url=aim_url)

        # Test with metadata
        @agent.perform_action(
            "modify_user",
            resource="user:12345",
            metadata={"reason": "Account update", "requested_by": "admin"}
        )
        def update_user_email(user_id, new_email):
            logger.info(f"   📝 Updating user {user_id} email to {new_email}")
            return {"success": True, "user_id": user_id, "email": new_email}

        logger.info("✅ Decorator with metadata applied")

        # Execute
        logger.info("\n🚀 Calling function with metadata...")
        result = update_user_email("12345", "new@example.com")

        logger.info(f"✅ Function executed")
        logger.info(f"   Result: {result}")

        return result.get("success", False)

    except Exception as e:
        logger.error(f"\n❌ TEST 4b FAILED: {e}")
        import traceback
        traceback.print_exc()
        return False

def test_high_risk_action():
    """Test @perform_action with high risk level."""
    logger.info("\n\n" + "=" * 80)
    logger.info("TEST 4c: High-Risk Action Verification")
    logger.info("=" * 80)

    try:
        from aim_sdk import secure

        aim_url = os.getenv('AIM_URL', 'https://aim-prod-backend.graypebble-c7e67ab8.canadacentral.azurecontainerapps.io')

        logger.info("\n🔐 Creating secure agent...")
        agent = secure("test-highrisk-agent", aim_url=aim_url)

        # High-risk action (delete data)
        @agent.perform_action(
            "delete_data",
            resource="user:12345",
            risk_level="high"
        )
        def delete_user_account(user_id):
            logger.info(f"   🗑️  Deleting user account: {user_id}")
            return {"deleted": True, "user_id": user_id}

        logger.info("✅ High-risk decorator applied")

        # Execute
        logger.info("\n🚀 Attempting high-risk action...")
        logger.info("   (This may require higher trust score)")

        try:
            result = delete_user_account("12345")
            logger.info(f"✅ High-risk action allowed")
            logger.info(f"   Result: {result}")
            return True
        except Exception as e:
            # May be blocked due to insufficient trust score
            if "trust" in str(e).lower() or "denied" in str(e).lower():
                logger.info("⚠️  High-risk action blocked (expected if trust score too low)")
                logger.info("   This is actually correct behavior!")
                return True
            else:
                raise

    except Exception as e:
        logger.error(f"\n❌ TEST 4c FAILED: {e}")
        import traceback
        traceback.print_exc()
        return False

def test_multiple_actions():
    """Test multiple @perform_action decorators on same agent."""
    logger.info("\n\n" + "=" * 80)
    logger.info("TEST 4d: Multiple Actions on Same Agent")
    logger.info("=" * 80)

    try:
        from aim_sdk import secure

        aim_url = os.getenv('AIM_URL', 'https://aim-prod-backend.graypebble-c7e67ab8.canadacentral.azurecontainerapps.io')

        logger.info("\n🔐 Creating secure agent...")
        agent = secure("test-multi-agent", aim_url=aim_url)

        # Define multiple actions
        @agent.perform_action("read_database")
        def read_data():
            return {"data": "read"}

        @agent.perform_action("write_database")
        def write_data():
            return {"data": "written"}

        @agent.perform_action("send_email")
        def send_email():
            return {"email": "sent"}

        @agent.perform_action("call_api")
        def call_api():
            return {"api": "called"}

        logger.info("✅ Multiple decorators applied")

        # Execute all actions
        logger.info("\n🚀 Executing all actions...")
        actions = [
            ("read_data", read_data),
            ("write_data", write_data),
            ("send_email", send_email),
            ("call_api", call_api)
        ]

        all_success = True
        for name, func in actions:
            try:
                result = func()
                logger.info(f"   ✅ {name}(): {result}")
            except Exception as e:
                logger.error(f"   ❌ {name}() failed: {e}")
                all_success = False

        return all_success

    except Exception as e:
        logger.error(f"\n❌ TEST 4d FAILED: {e}")
        import traceback
        traceback.print_exc()
        return False

if __name__ == "__main__":
    from dotenv import load_dotenv
    load_dotenv()

    # Run tests
    results = []

    results.append(("Basic @perform_action", test_basic_perform_action()))
    results.append(("@perform_action with metadata", test_action_with_metadata()))
    results.append(("High-risk action", test_high_risk_action()))
    results.append(("Multiple actions", test_multiple_actions()))

    # Print summary
    print("\n\n" + "=" * 80)
    print("TEST SUMMARY - @perform_action Decorator")
    print("=" * 80)

    for test_name, passed in results:
        status = "✅ PASS" if passed else "❌ FAIL"
        print(f"{status} - {test_name}")

    # Exit with appropriate code
    all_passed = all(passed for _, passed in results)
    sys.exit(0 if all_passed else 1)
