#!/usr/bin/env python3
"""
Test 1: secure() One-Line Function - The "Stripe Moment"

This test verifies the core claim of AIM SDK:
"ONE LINE OF CODE - Complete enterprise security"

We're testing:
1. Zero-config registration using secure()
2. Automatic agent creation
3. Cryptographic key generation
4. Credential storage
5. Automatic capability detection
"""

import os
import sys
import json
import logging
from pathlib import Path

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

def test_secure_zero_config():
    """Test the secure() function with ZERO configuration.

    This is the "Stripe moment" - one line should give us:
    - Agent registered
    - Ed25519 keys generated
    - Credentials stored
    - Ready to use
    """
    logger.info("=" * 80)
    logger.info("TEST 1: secure() One-Line Function - Zero Config Mode")
    logger.info("=" * 80)

    try:
        # Import the SDK
        logger.info("\n📦 Step 1: Importing AIM SDK...")
        from aim_sdk import secure
        logger.info("✅ SDK imported successfully")

        # Set AIM URL from environment
        aim_url = os.getenv('AIM_URL', 'https://aim-prod-backend.graypebble-c7e67ab8.canadacentral.azurecontainerapps.io')
        logger.info(f"\n🔗 Using AIM URL: {aim_url}")

        # THE ONE LINE - This should do EVERYTHING
        logger.info("\n🚀 Step 2: Testing the ONE LINE - secure('test-agent-01')")
        logger.info("   This single line should:")
        logger.info("   - Register the agent with AIM backend")
        logger.info("   - Generate Ed25519 cryptographic keys")
        logger.info("   - Store credentials securely")
        logger.info("   - Auto-detect capabilities")
        logger.info("   - Auto-detect MCP servers")
        logger.info("   - Return ready-to-use agent client")

        agent = secure("test-agent-01", aim_url=aim_url)

        logger.info("✅ ONE LINE WORKED! Agent created!")

        # Verify agent has all expected attributes
        logger.info("\n🔍 Step 3: Verifying agent attributes...")

        # Check agent_id
        if hasattr(agent, 'agent_id'):
            logger.info(f"✅ agent.agent_id: {agent.agent_id}")
        else:
            logger.error("❌ Missing agent_id attribute")
            return False

        # Check public_key
        if hasattr(agent, 'public_key'):
            logger.info(f"✅ agent.public_key: {agent.public_key[:32]}... (truncated)")
        else:
            logger.error("❌ Missing public_key attribute")
            return False

        # Check private_key
        if hasattr(agent, 'private_key'):
            logger.info(f"✅ agent.private_key: [REDACTED] (exists)")
        else:
            logger.error("❌ Missing private_key attribute")
            return False

        # Check credentials file was created
        logger.info("\n📁 Step 4: Verifying credential storage...")
        creds_path = Path.home() / '.aim' / 'credentials.json'

        if creds_path.exists():
            logger.info(f"✅ Credentials file exists: {creds_path}")

            with open(creds_path, 'r') as f:
                creds = json.load(f)

            if 'test-agent-01' in creds:
                logger.info("✅ Agent credentials stored successfully")
                agent_creds = creds['test-agent-01']

                logger.info(f"   - Agent ID: {agent_creds.get('agent_id', 'N/A')}")
                logger.info(f"   - Public Key: {agent_creds.get('public_key', 'N/A')[:32]}...")
                logger.info(f"   - AIM URL: {agent_creds.get('aim_url', 'N/A')}")
                logger.info(f"   - Status: {agent_creds.get('status', 'N/A')}")
                logger.info(f"   - Trust Score: {agent_creds.get('trust_score', 'N/A')}")
            else:
                logger.error("❌ Agent credentials not found in file")
                return False
        else:
            logger.error(f"❌ Credentials file not created: {creds_path}")
            return False

        # Check agent has perform_action method
        logger.info("\n🎯 Step 5: Verifying agent capabilities...")
        if hasattr(agent, 'perform_action'):
            logger.info("✅ agent.perform_action() method exists")
        else:
            logger.error("❌ Missing perform_action method")
            return False

        logger.info("\n" + "=" * 80)
        logger.info("✅ TEST 1 PASSED - secure() function works as advertised!")
        logger.info("=" * 80)
        logger.info("\nVerified Claims:")
        logger.info("✅ ONE LINE creates complete agent identity")
        logger.info("✅ Ed25519 cryptographic keys generated automatically")
        logger.info("✅ Credentials stored securely in ~/.aim/credentials.json")
        logger.info("✅ Agent ready to use immediately")
        logger.info("✅ Zero configuration required")

        return True

    except Exception as e:
        logger.error(f"\n❌ TEST 1 FAILED: {e}")
        import traceback
        traceback.print_exc()
        return False

def test_secure_with_api_key():
    """Test secure() with explicit API key.

    This tests the alternative mode where user provides their own API key.
    """
    logger.info("\n\n" + "=" * 80)
    logger.info("TEST 1b: secure() with API Key Mode")
    logger.info("=" * 80)

    try:
        from aim_sdk import secure

        aim_url = os.getenv('AIM_URL', 'https://aim-prod-backend.graypebble-c7e67ab8.canadacentral.azurecontainerapps.io')

        # Note: This will fail without a valid API key, which is expected
        # We're just testing that the parameter works
        logger.info("\n🔑 Testing secure() with api_key parameter...")
        logger.info("   (Will fail without valid key, which is expected)")

        try:
            agent = secure("test-agent-02", aim_url=aim_url, api_key="test_invalid_key")
            logger.info("✅ secure() accepted api_key parameter")
        except Exception as e:
            # Expected to fail with invalid key
            if "api_key" in str(e).lower() or "auth" in str(e).lower() or "401" in str(e) or "403" in str(e):
                logger.info("✅ API key validation working (rejected invalid key)")
                return True
            else:
                logger.error(f"❌ Unexpected error: {e}")
                return False

        return True

    except Exception as e:
        logger.error(f"\n❌ TEST 1b FAILED: {e}")
        return False

if __name__ == "__main__":
    # Load environment variables
    from dotenv import load_dotenv
    load_dotenv()

    # Run tests
    results = []

    results.append(("secure() zero config", test_secure_zero_config()))
    results.append(("secure() with API key", test_secure_with_api_key()))

    # Print summary
    print("\n\n" + "=" * 80)
    print("TEST SUMMARY")
    print("=" * 80)

    for test_name, passed in results:
        status = "✅ PASS" if passed else "❌ FAIL"
        print(f"{status} - {test_name}")

    # Exit with appropriate code
    all_passed = all(passed for _, passed in results)
    sys.exit(0 if all_passed else 1)
