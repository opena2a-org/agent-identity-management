#!/usr/bin/env python3
"""
Test script for MCP Auto-Detection functionality

This script demonstrates how to use the detect_mcp_servers_from_config function
to automatically discover and register MCP servers from Claude Desktop configuration.

Usage:
    python test_mcp_auto_detection.py

Requirements:
    - AIM backend running at http://localhost:8080
    - Valid agent credentials (agent_id, public_key, private_key)
    - Claude Desktop config file at ~/.config/claude/claude_desktop_config.json
"""

import os
import sys

# Add parent directory to path for local development
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from aim_sdk import AIMClient
from aim_sdk.integrations.mcp import (
    detect_mcp_servers_from_config,
    find_claude_config,
    get_default_config_paths
)


def test_find_config():
    """Test finding Claude Desktop config file."""
    print("\n" + "="*80)
    print("TEST 1: Finding Claude Desktop Config File")
    print("="*80)

    # List all possible config paths
    print("\nDefault config paths for this OS:")
    for path in get_default_config_paths():
        expanded = os.path.expanduser(path)
        exists = "✓" if os.path.exists(expanded) else "✗"
        print(f"  {exists} {path}")

    # Try to find config automatically
    config_path = find_claude_config()
    if config_path:
        print(f"\n✅ Found Claude config at: {config_path}")
        expanded_path = os.path.expanduser(config_path)
        file_size = os.path.getsize(expanded_path)
        print(f"   File size: {file_size} bytes")
        return config_path
    else:
        print("\n❌ Claude Desktop config not found on this system")
        print("   Please install Claude Desktop or specify custom path")
        return None


def test_dry_run_detection(aim_client, agent_id, config_path):
    """Test dry run detection (preview without changes)."""
    print("\n" + "="*80)
    print("TEST 2: Dry Run Detection (Preview)")
    print("="*80)

    try:
        result = detect_mcp_servers_from_config(
            aim_client=aim_client,
            agent_id=agent_id,
            config_path=config_path,
            dry_run=True
        )

        print(f"\n✅ Detected {len(result['detected_servers'])} MCP servers (dry run)")
        print("\nDetected servers:")

        for i, server in enumerate(result['detected_servers'], 1):
            print(f"\n  {i}. {server['name']}")
            print(f"     Command: {server['command']}")
            if server.get('args'):
                print(f"     Args: {' '.join(server['args'])}")
            if server.get('env'):
                print(f"     Environment variables: {len(server['env'])}")
            print(f"     Confidence: {server['confidence']}%")
            print(f"     Source: {server['source']}")

        return result

    except FileNotFoundError as e:
        print(f"\n❌ Config file not found: {e}")
        return None
    except Exception as e:
        print(f"\n❌ Detection failed: {type(e).__name__}: {e}")
        return None


def test_auto_detection_with_registration(aim_client, agent_id, config_path):
    """Test auto-detection with automatic registration."""
    print("\n" + "="*80)
    print("TEST 3: Auto-Detection with Registration")
    print("="*80)

    try:
        result = detect_mcp_servers_from_config(
            aim_client=aim_client,
            agent_id=agent_id,
            config_path=config_path,
            auto_register=True
        )

        print(f"\n✅ Detection complete!")
        print(f"   Detected: {len(result['detected_servers'])} MCP servers")
        print(f"   Registered: {result['registered_count']} new servers")
        print(f"   Mapped: {result['mapped_count']} servers to agent")
        print(f"   Total talks_to: {result['total_talks_to']} servers")

        if result.get('errors_encountered'):
            print(f"\n⚠️  Encountered {len(result['errors_encountered'])} errors:")
            for error in result['errors_encountered']:
                print(f"   - {error}")

        return result

    except Exception as e:
        print(f"\n❌ Auto-detection failed: {type(e).__name__}: {e}")
        import traceback
        traceback.print_exc()
        return None


def test_detection_without_registration(aim_client, agent_id, config_path):
    """Test detection that only maps existing servers (no registration)."""
    print("\n" + "="*80)
    print("TEST 4: Detection Without Auto-Registration")
    print("="*80)

    try:
        result = detect_mcp_servers_from_config(
            aim_client=aim_client,
            agent_id=agent_id,
            config_path=config_path,
            auto_register=False
        )

        print(f"\n✅ Detection complete (no registration)!")
        print(f"   Detected: {len(result['detected_servers'])} MCP servers")
        print(f"   Registered: {result['registered_count']} new servers")
        print(f"   Mapped: {result['mapped_count']} existing servers to agent")
        print(f"   Total talks_to: {result['total_talks_to']} servers")

        return result

    except Exception as e:
        print(f"\n❌ Detection failed: {type(e).__name__}: {e}")
        return None


def main():
    """Run all tests."""
    print("\n" + "="*80)
    print("MCP AUTO-DETECTION TEST SUITE")
    print("="*80)

    # Configuration (update these values for your environment)
    AIM_URL = os.getenv("AIM_URL", "http://localhost:8080")
    AGENT_ID = os.getenv("AGENT_ID", "")
    PUBLIC_KEY = os.getenv("PUBLIC_KEY", "")
    PRIVATE_KEY = os.getenv("PRIVATE_KEY", "")

    # Test 1: Find config file
    config_path = test_find_config()
    if not config_path:
        print("\n⚠️  Skipping remaining tests (no config file found)")
        print("   To test with a custom path, set CLAUDE_CONFIG_PATH environment variable")
        custom_path = os.getenv("CLAUDE_CONFIG_PATH")
        if custom_path:
            config_path = custom_path
        else:
            return

    # Check if we have agent credentials
    if not all([AGENT_ID, PUBLIC_KEY, PRIVATE_KEY]):
        print("\n" + "="*80)
        print("⚠️  CREDENTIALS NOT CONFIGURED")
        print("="*80)
        print("\nTo run full tests, set these environment variables:")
        print("  export AIM_URL='http://localhost:8080'")
        print("  export AGENT_ID='your-agent-id'")
        print("  export PUBLIC_KEY='your-public-key'")
        print("  export PRIVATE_KEY='your-private-key'")
        print("\nAlternatively, you can modify the script to hardcode these values.")
        print("Tests requiring AIM connection will be skipped.\n")
        return

    # Create AIM client
    try:
        print("\n" + "="*80)
        print("Initializing AIM Client")
        print("="*80)
        print(f"AIM URL: {AIM_URL}")
        print(f"Agent ID: {AGENT_ID}")

        aim_client = AIMClient(
            agent_id=AGENT_ID,
            public_key=PUBLIC_KEY,
            private_key=PRIVATE_KEY,
            aim_url=AIM_URL
        )
        print("✅ AIM Client initialized successfully")

    except Exception as e:
        print(f"❌ Failed to initialize AIM Client: {e}")
        import traceback
        traceback.print_exc()
        return

    # Run tests
    try:
        # Test 2: Dry run (preview)
        dry_run_result = test_dry_run_detection(aim_client, AGENT_ID, config_path)

        # Test 3: Auto-detection with registration
        if dry_run_result and len(dry_run_result.get('detected_servers', [])) > 0:
            auto_result = test_auto_detection_with_registration(
                aim_client, AGENT_ID, config_path
            )

            # Test 4: Detection without registration (only mapping)
            if auto_result:
                no_reg_result = test_detection_without_registration(
                    aim_client, AGENT_ID, config_path
                )

    finally:
        aim_client.close()

    print("\n" + "="*80)
    print("TEST SUITE COMPLETE")
    print("="*80)


if __name__ == "__main__":
    main()
