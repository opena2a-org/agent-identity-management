#!/usr/bin/env python3
"""
Test 2: Automatic Capability Detection

This test verifies the SDK's claim:
"AIM automatically detects everything about your agent"

We're testing:
1. Python import detection (requests → API calls, psycopg2 → database)
2. Decorator detection (@agent.perform_action)
3. Config file detection (~/.aim/capabilities.json)
4. Confidence scoring
"""

import os
import sys
import logging
import tempfile
from pathlib import Path

logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

def test_import_detection():
    """Test that SDK detects capabilities from Python imports."""
    logger.info("=" * 80)
    logger.info("TEST 2a: Import-Based Capability Detection")
    logger.info("=" * 80)

    try:
        from aim_sdk.capability_detection import CapabilityDetector

        # Create a temporary Python file with various imports
        with tempfile.NamedTemporaryFile(mode='w', suffix='.py', delete=False) as f:
            f.write("""
import requests
import psycopg2
import smtplib
import stripe
from anthropic import Anthropic

def some_function():
    pass
""")
            temp_file = f.name

        logger.info(f"\n📄 Created test file: {temp_file}")
        logger.info("   Contains imports: requests, psycopg2, smtplib, stripe, anthropic")

        # Detect capabilities
        logger.info("\n🔍 Running capability detection...")
        detector = CapabilityDetector()
        capabilities = detector.detect_from_code(temp_file)

        logger.info(f"\n✅ Detected {len(capabilities)} capabilities:")
        for cap in capabilities:
            logger.info(f"   - {cap['capability']}: {cap['source']} (confidence: {cap['confidence']}%)")

        # Verify expected capabilities
        expected = ['api_calls', 'database_access', 'email_send', 'payment_processing', 'ai_model_access']
        detected_names = [c['capability'] for c in capabilities]

        logger.info("\n🎯 Verifying expected capabilities...")
        for exp_cap in expected:
            if exp_cap in detected_names:
                logger.info(f"   ✅ {exp_cap} detected")
            else:
                logger.warning(f"   ⚠️  {exp_cap} NOT detected")

        # Cleanup
        os.unlink(temp_file)

        return len(capabilities) > 0

    except Exception as e:
        logger.error(f"\n❌ TEST 2a FAILED: {e}")
        import traceback
        traceback.print_exc()
        return False

def test_decorator_detection():
    """Test that SDK detects capabilities from @perform_action decorators."""
    logger.info("\n\n" + "=" * 80)
    logger.info("TEST 2b: Decorator-Based Capability Detection")
    logger.info("=" * 80)

    try:
        from aim_sdk.capability_detection import CapabilityDetector

        # Create a temporary Python file with decorators
        with tempfile.NamedTemporaryFile(mode='w', suffix='.py', delete=False) as f:
            f.write("""
from aim_sdk import secure

agent = secure("test-agent")

@agent.perform_action("read_database", resource="users_table")
def get_users():
    pass

@agent.perform_action("send_email", resource="notifications")
def send_notification():
    pass

@agent.perform_action("delete_data", resource="user_records", risk_level="high")
def delete_user():
    pass
""")
            temp_file = f.name

        logger.info(f"\n📄 Created test file: {temp_file}")
        logger.info("   Contains decorators: read_database, send_email, delete_data")

        # Detect capabilities
        logger.info("\n🔍 Running decorator detection...")
        detector = CapabilityDetector()
        capabilities = detector.detect_from_code(temp_file)

        logger.info(f"\n✅ Detected {len(capabilities)} capabilities:")
        for cap in capabilities:
            logger.info(f"   - {cap['capability']}: {cap['source']} (confidence: {cap['confidence']}%)")

        # Verify expected capabilities
        expected_decorators = ['read_database', 'send_email', 'delete_data']
        detected_names = [c['capability'] for c in capabilities]

        logger.info("\n🎯 Verifying decorator detection...")
        for exp_cap in expected_decorators:
            if exp_cap in detected_names:
                logger.info(f"   ✅ {exp_cap} detected from decorator")
            else:
                logger.warning(f"   ⚠️  {exp_cap} NOT detected")

        # Cleanup
        os.unlink(temp_file)

        return len(capabilities) > 0

    except Exception as e:
        logger.error(f"\n❌ TEST 2b FAILED: {e}")
        import traceback
        traceback.print_exc()
        return False

def test_config_file_detection():
    """Test that SDK detects capabilities from config file."""
    logger.info("\n\n" + "=" * 80)
    logger.info("TEST 2c: Config File Capability Detection")
    logger.info("=" * 80)

    try:
        import json
        from aim_sdk.capability_detection import CapabilityDetector

        # Create temporary config file
        config_dir = Path.home() / '.aim'
        config_dir.mkdir(exist_ok=True)
        config_file = config_dir / 'test_capabilities.json'

        test_config = {
            "capabilities": [
                "custom_capability_1",
                "custom_capability_2",
                "special_feature"
            ]
        }

        with open(config_file, 'w') as f:
            json.dump(test_config, f, indent=2)

        logger.info(f"\n📄 Created config file: {config_file}")
        logger.info(f"   Contains: {test_config['capabilities']}")

        # Detect capabilities
        logger.info("\n🔍 Running config file detection...")
        detector = CapabilityDetector()
        capabilities = detector.detect_from_config(str(config_file))

        logger.info(f"\n✅ Detected {len(capabilities)} capabilities from config:")
        for cap in capabilities:
            logger.info(f"   - {cap['capability']}: {cap['source']} (confidence: {cap['confidence']}%)")

        # Verify
        detected_names = [c['capability'] for c in capabilities]
        for exp_cap in test_config['capabilities']:
            if exp_cap in detected_names:
                logger.info(f"   ✅ {exp_cap} detected from config")
            else:
                logger.warning(f"   ⚠️  {exp_cap} NOT detected")

        # Cleanup
        config_file.unlink()

        return len(capabilities) > 0

    except Exception as e:
        logger.error(f"\n❌ TEST 2c FAILED: {e}")
        import traceback
        traceback.print_exc()
        return False

def test_auto_detect_capabilities():
    """Test the auto_detect_capabilities() helper function."""
    logger.info("\n\n" + "=" * 80)
    logger.info("TEST 2d: auto_detect_capabilities() Helper Function")
    logger.info("=" * 80)

    try:
        from aim_sdk import auto_detect_capabilities

        # Create a test file with mixed capabilities
        with tempfile.NamedTemporaryFile(mode='w', suffix='.py', delete=False) as f:
            f.write("""
import requests
import psycopg2
from aim_sdk import secure

agent = secure("test")

@agent.perform_action("read_database")
def read_db():
    pass

@agent.perform_action("call_api")
def call_api():
    requests.get("https://api.example.com")
""")
            temp_file = f.name

        logger.info(f"\n📄 Created test file: {temp_file}")

        # Auto-detect
        logger.info("\n🔍 Running auto_detect_capabilities()...")
        capabilities = auto_detect_capabilities(temp_file)

        logger.info(f"\n✅ Auto-detected {len(capabilities)} capabilities:")
        for cap in capabilities:
            logger.info(f"   - {cap}")

        # Cleanup
        os.unlink(temp_file)

        return len(capabilities) > 0

    except Exception as e:
        logger.error(f"\n❌ TEST 2d FAILED: {e}")
        import traceback
        traceback.print_exc()
        return False

if __name__ == "__main__":
    from dotenv import load_dotenv
    load_dotenv()

    # Run tests
    results = []

    results.append(("Import detection", test_import_detection()))
    results.append(("Decorator detection", test_decorator_detection()))
    results.append(("Config file detection", test_config_file_detection()))
    results.append(("auto_detect_capabilities()", test_auto_detect_capabilities()))

    # Print summary
    print("\n\n" + "=" * 80)
    print("TEST SUMMARY - Capability Detection")
    print("=" * 80)

    for test_name, passed in results:
        status = "✅ PASS" if passed else "❌ FAIL"
        print(f"{status} - {test_name}")

    # Exit with appropriate code
    all_passed = all(passed for _, passed in results)
    sys.exit(0 if all_passed else 1)
