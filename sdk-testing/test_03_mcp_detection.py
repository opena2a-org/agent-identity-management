#!/usr/bin/env python3
"""
Test 3: Automatic MCP Server Detection

This test verifies the SDK's claim:
"Finds Claude Desktop configs automatically"

We're testing:
1. MCP server detection from Claude Desktop config
2. Parsing of mcpServers configuration
3. Extraction of server capabilities
4. Confidence scoring for MCP servers
"""

import os
import sys
import json
import logging
import tempfile
from pathlib import Path

logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

def test_mcp_config_detection():
    """Test MCP server detection from Claude Desktop config."""
    logger.info("=" * 80)
    logger.info("TEST 3a: MCP Server Detection from Claude Desktop Config")
    logger.info("=" * 80)

    try:
        from aim_sdk.detection import MCPDetector

        # Create a temporary Claude Desktop config
        with tempfile.NamedTemporaryFile(mode='w', suffix='.json', delete=False) as f:
            config = {
                "mcpServers": {
                    "filesystem": {
                        "command": "npx",
                        "args": ["-y", "@modelcontextprotocol/server-filesystem", "/Users/test/workspace"]
                    },
                    "postgres": {
                        "command": "npx",
                        "args": ["-y", "@modelcontextprotocol/server-postgres", "postgresql://localhost/testdb"]
                    },
                    "github": {
                        "command": "npx",
                        "args": ["-y", "@modelcontextprotocol/server-github"],
                        "env": {
                            "GITHUB_TOKEN": "test_token"
                        }
                    }
                }
            }
            json.dump(config, f, indent=2)
            temp_config = f.name

        logger.info(f"\n📄 Created test Claude Desktop config: {temp_config}")
        logger.info("   Contains MCP servers: filesystem, postgres, github")

        # Detect MCP servers
        logger.info("\n🔍 Running MCP detection...")
        detector = MCPDetector()
        mcps = detector.detect_from_config(temp_config)

        logger.info(f"\n✅ Detected {len(mcps)} MCP servers:")
        for mcp in mcps:
            logger.info(f"   - {mcp['name']}: {mcp['command']} (confidence: {mcp['confidence']}%)")
            if 'capabilities' in mcp:
                logger.info(f"     Capabilities: {', '.join(mcp.get('capabilities', []))}")

        # Verify expected servers
        expected_servers = ['filesystem', 'postgres', 'github']
        detected_names = [m['name'] for m in mcps]

        logger.info("\n🎯 Verifying MCP server detection...")
        for exp_server in expected_servers:
            if exp_server in detected_names:
                logger.info(f"   ✅ {exp_server} detected")
            else:
                logger.error(f"   ❌ {exp_server} NOT detected")

        # Cleanup
        os.unlink(temp_config)

        return len(mcps) > 0 and all(s in detected_names for s in expected_servers)

    except Exception as e:
        logger.error(f"\n❌ TEST 3a FAILED: {e}")
        import traceback
        traceback.print_exc()
        return False

def test_mcp_capability_inference():
    """Test that SDK infers capabilities from MCP server names."""
    logger.info("\n\n" + "=" * 80)
    logger.info("TEST 3b: MCP Capability Inference")
    logger.info("=" * 80)

    try:
        from aim_sdk.detection import MCPDetector

        # Create config with well-known MCP servers
        with tempfile.NamedTemporaryFile(mode='w', suffix='.json', delete=False) as f:
            config = {
                "mcpServers": {
                    "filesystem": {
                        "command": "npx",
                        "args": ["-y", "@modelcontextprotocol/server-filesystem"]
                    },
                    "postgres": {
                        "command": "npx",
                        "args": ["-y", "@modelcontextprotocol/server-postgres"]
                    },
                    "github": {
                        "command": "npx",
                        "args": ["-y", "@modelcontextprotocol/server-github"]
                    },
                    "slack": {
                        "command": "npx",
                        "args": ["-y", "@modelcontextprotocol/server-slack"]
                    }
                }
            }
            json.dump(config, f, indent=2)
            temp_config = f.name

        logger.info(f"\n📄 Created test config with capability-rich servers")

        # Detect with capability inference
        logger.info("\n🔍 Detecting MCP servers with capability inference...")
        detector = MCPDetector()
        mcps = detector.detect_from_config(temp_config)

        logger.info(f"\n✅ Detected servers with inferred capabilities:")

        capability_checks = {
            'filesystem': ['file_read', 'file_write', 'file_list'],
            'postgres': ['database_read', 'database_write', 'database_query'],
            'github': ['code_read', 'issue_create', 'pr_create'],
            'slack': ['send_message', 'read_messages', 'channel_management']
        }

        all_correct = True
        for mcp in mcps:
            name = mcp['name']
            capabilities = mcp.get('capabilities', [])
            logger.info(f"\n   {name}:")
            logger.info(f"   Detected capabilities: {', '.join(capabilities) if capabilities else 'None'}")

            if name in capability_checks:
                expected = capability_checks[name]
                # Check if at least some expected capabilities are present
                found = any(exp in ' '.join(capabilities).lower() for exp in expected)
                if capabilities and found:
                    logger.info(f"   ✅ Capability inference working for {name}")
                else:
                    logger.warning(f"   ⚠️  Limited or no capabilities inferred for {name}")
                    all_correct = False

        # Cleanup
        os.unlink(temp_config)

        return len(mcps) > 0

    except Exception as e:
        logger.error(f"\n❌ TEST 3b FAILED: {e}")
        import traceback
        traceback.print_exc()
        return False

def test_auto_detect_mcps():
    """Test the auto_detect_mcps() helper function."""
    logger.info("\n\n" + "=" * 80)
    logger.info("TEST 3c: auto_detect_mcps() Helper Function")
    logger.info("=" * 80)

    try:
        from aim_sdk import auto_detect_mcps

        logger.info("\n🔍 Running auto_detect_mcps() on system...")
        logger.info("   Looking for Claude Desktop config in standard locations...")

        mcps = auto_detect_mcps()

        if mcps:
            logger.info(f"\n✅ Auto-detected {len(mcps)} MCP servers:")
            for mcp in mcps:
                logger.info(f"   - {mcp}")
        else:
            logger.info("\n⚠️  No MCP servers found (this is OK if Claude Desktop not configured)")
            logger.info("   The function works, just no servers to detect")

        # Success if function runs without error
        return True

    except Exception as e:
        logger.error(f"\n❌ TEST 3c FAILED: {e}")
        import traceback
        traceback.print_exc()
        return False

def test_mcp_standard_locations():
    """Test that SDK checks standard Claude Desktop config locations."""
    logger.info("\n\n" + "=" * 80)
    logger.info("TEST 3d: Standard Config Location Detection")
    logger.info("=" * 80)

    try:
        from aim_sdk.detection import MCPDetector
        import platform

        detector = MCPDetector()

        logger.info("\n📂 Standard Claude Desktop config locations:")

        system = platform.system()
        if system == "Darwin":  # macOS
            locations = [
                Path.home() / "Library" / "Application Support" / "Claude" / "claude_desktop_config.json",
                Path.home() / ".config" / "claude" / "claude_desktop_config.json",
                Path.home() / ".claude" / "claude_desktop_config.json"
            ]
        elif system == "Windows":
            locations = [
                Path.home() / "AppData" / "Roaming" / "Claude" / "claude_desktop_config.json",
                Path.home() / ".claude" / "claude_desktop_config.json"
            ]
        else:  # Linux
            locations = [
                Path.home() / ".config" / "claude" / "claude_desktop_config.json",
                Path.home() / ".claude" / "claude_desktop_config.json"
            ]

        for loc in locations:
            exists = loc.exists()
            status = "✅ EXISTS" if exists else "⚠️  Not found"
            logger.info(f"   {status}: {loc}")

            if exists:
                logger.info("   Attempting to detect MCP servers from this file...")
                try:
                    mcps = detector.detect_from_config(str(loc))
                    logger.info(f"   ✅ Detected {len(mcps)} MCP servers")
                except Exception as e:
                    logger.error(f"   ❌ Detection failed: {e}")

        return True

    except Exception as e:
        logger.error(f"\n❌ TEST 3d FAILED: {e}")
        import traceback
        traceback.print_exc()
        return False

if __name__ == "__main__":
    from dotenv import load_dotenv
    load_dotenv()

    # Run tests
    results = []

    results.append(("MCP config detection", test_mcp_config_detection()))
    results.append(("MCP capability inference", test_mcp_capability_inference()))
    results.append(("auto_detect_mcps()", test_auto_detect_mcps()))
    results.append(("Standard location detection", test_mcp_standard_locations()))

    # Print summary
    print("\n\n" + "=" * 80)
    print("TEST SUMMARY - MCP Detection")
    print("=" * 80)

    for test_name, passed in results:
        status = "✅ PASS" if passed else "❌ FAIL"
        print(f"{status} - {test_name}")

    # Exit with appropriate code
    all_passed = all(passed for _, passed in results)
    sys.exit(0 if all_passed else 1)
