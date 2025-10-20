#!/usr/bin/env python3
"""
AIM SDK - Manual vs Auto Registration Examples

This example demonstrates the flexibility spectrum of the AIM SDK:
1. EASY MODE: Full auto-detection (recommended for most users)
2. BALANCED MODE: Auto-detect + manual additions
3. EXPERT MODE: Full manual control

The AIM SDK philosophy:
- Auto-detection makes the platform as easy as possible to use
- Manual declaration gives experts full control
- You can mix both approaches seamlessly
"""

import sys
import os

# Add SDK to path
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "aim_sdk"))

from aim_sdk import register_agent, AIMClient
from aim_sdk.integrations.mcp import (
    register_mcp_server,
    detect_mcp_servers_from_config,
    auto_detect_capabilities
)

AIM_URL = "http://localhost:8080"
API_KEY = "aim_test_key_123"  # Get from AIM dashboard


# ============================================================================
# MODE 1: EASY MODE - Full Auto-Detection (Recommended)
# ============================================================================

def example_easy_mode():
    """
    EASY MODE: Full auto-detection - zero configuration required.

    This is the recommended approach for most users. The SDK automatically:
    - Detects MCP servers from Claude Desktop config
    - Detects capabilities from MCP tools
    - Registers everything with AIM backend

    Perfect for: Quick start, prototyping, developers who want it to "just work"
    """
    print("\n" + "=" * 70)
    print("MODE 1: EASY MODE - Full Auto-Detection")
    print("=" * 70)

    # Step 1: Register agent with auto-detection enabled (default)
    # No need to specify capabilities or talks_to - SDK detects automatically
    agent = register_agent(
        name="my-easy-agent",
        aim_url=AIM_URL,
        api_key=API_KEY,
        description="Agent using full auto-detection",
        # talks_to: NOT specified - will be auto-detected
        # capabilities: NOT specified - will be auto-detected
    )

    print(f"\n✅ Agent registered: {agent.agent_id}")
    print("   Using full auto-detection mode")

    # Step 2: Auto-detect MCP servers from Claude Desktop config
    # This scans ~/.config/claude/claude_desktop_config.json
    try:
        detection_result = detect_mcp_servers_from_config(
            aim_client=agent,
            agent_id=agent.agent_id,
            auto_register=True  # Automatically register new MCP servers
        )

        print(f"\n✅ Auto-detected MCP servers:")
        print(f"   Detected: {len(detection_result['detected_servers'])} servers")
        print(f"   Registered: {detection_result['registered_count']} new servers")
        print(f"   Mapped: {detection_result['mapped_count']} to agent")

        for server in detection_result['detected_servers']:
            print(f"   - {server['name']} (confidence: {server['confidence']}%)")

    except FileNotFoundError:
        print("\n⚠️  Claude Desktop config not found - skipping MCP detection")
        print("   Install Claude Desktop or use manual mode")

    # Step 3: Auto-detect capabilities from MCP tools
    try:
        capability_result = auto_detect_capabilities(
            aim_client=agent,
            agent_id=agent.agent_id,
            auto_detect_from_mcp=True  # Detect from registered MCP servers
        )

        print(f"\n✅ Auto-detected capabilities:")
        print(f"   Reported: {capability_result['capabilities_reported']} capabilities")
        print(f"   Risk Level: {capability_result['risk_assessment']['risk_level']}")
        print(f"   Risk Score: {capability_result['risk_assessment']['overall_risk_score']}")
        print(f"   Trust Impact: {capability_result['risk_assessment']['trust_score_impact']}")

        # Show security alerts
        for alert in capability_result['risk_assessment']['alerts']:
            if alert['severity'] in ['HIGH', 'CRITICAL']:
                print(f"   ⚠️  {alert['severity']}: {alert['message']}")

    except Exception as e:
        print(f"\n⚠️  Capability auto-detection failed: {e}")

    print("\n🎉 EASY MODE COMPLETE - Zero configuration required!")
    print("   AIM automatically detected and secured everything")


# ============================================================================
# MODE 2: BALANCED MODE - Auto-Detect + Manual Additions
# ============================================================================

def example_balanced_mode():
    """
    BALANCED MODE: Combine auto-detection with manual declarations.

    This approach gives you the best of both worlds:
    - Auto-detect common MCP servers and capabilities
    - Manually declare specific/custom capabilities
    - Add MCP servers that aren't in Claude Desktop config

    Perfect for: Production agents, custom integrations, specific security requirements
    """
    print("\n" + "=" * 70)
    print("MODE 2: BALANCED MODE - Auto-Detect + Manual Additions")
    print("=" * 70)

    # Step 1: Register agent with some manual declarations
    agent = register_agent(
        name="my-balanced-agent",
        aim_url=AIM_URL,
        api_key=API_KEY,
        description="Agent using balanced auto-detect + manual mode",

        # MANUAL: Explicitly declare some MCP servers
        talks_to=[
            "custom-database-mcp",  # Custom MCP not in Claude Desktop
            "internal-api-mcp"       # Internal company MCP
        ],

        # MANUAL: Explicitly declare some capabilities
        capabilities=[
            "execute_sql_queries",   # Database access
            "call_internal_api",     # Internal API access
            "send_notifications"     # Email/Slack notifications
        ]
    )

    print(f"\n✅ Agent registered: {agent.agent_id}")
    print("   Using balanced mode (manual + auto-detect)")
    print(f"   Manual MCP servers: {2}")
    print(f"   Manual capabilities: {3}")

    # Step 2: Auto-detect ADDITIONAL MCP servers from Claude Desktop
    # This adds to the manual declarations above
    try:
        detection_result = detect_mcp_servers_from_config(
            aim_client=agent,
            agent_id=agent.agent_id,
            auto_register=True
        )

        print(f"\n✅ Auto-detected ADDITIONAL MCP servers:")
        print(f"   Detected: {len(detection_result['detected_servers'])} servers")
        print(f"   Total talks_to: {detection_result['total_talks_to']} servers")
        print("   (Manual + Auto-detected)")

    except FileNotFoundError:
        print("\n⚠️  Claude Desktop config not found - using manual only")

    # Step 3: Manually register a custom MCP server
    # This is for MCP servers not in Claude Desktop config
    try:
        custom_mcp = register_mcp_server(
            aim_client=agent,
            server_name="custom-database-mcp",
            server_url="http://localhost:5000",
            public_key="ed25519_custom_key_here",
            capabilities=["database", "query", "transactions"],
            description="Custom PostgreSQL MCP server for production database",
            version="2.1.0"
        )

        print(f"\n✅ Manually registered custom MCP server:")
        print(f"   Name: {custom_mcp.get('name')}")
        print(f"   Status: {custom_mcp.get('status')}")
        print(f"   Trust Score: {custom_mcp.get('trust_score')}")

    except Exception as e:
        print(f"\n⚠️  Custom MCP registration failed: {e}")

    # Step 4: Report additional capabilities manually
    # Combine with auto-detected capabilities
    try:
        manual_capabilities = [
            {
                "capability_type": "database_write",
                "capability_scope": {
                    "database": "postgres://prod-db:5432/main",
                    "tables": ["users", "transactions", "audit_log"]
                },
                "risk_level": "HIGH",
                "detected_via": "manual_declaration"
            },
            {
                "capability_type": "external_api",
                "capability_scope": {
                    "api": "https://internal-api.company.com",
                    "endpoints": ["/users", "/orders", "/payments"]
                },
                "risk_level": "MEDIUM",
                "detected_via": "manual_declaration"
            }
        ]

        capability_result = auto_detect_capabilities(
            aim_client=agent,
            agent_id=agent.agent_id,
            detected_capabilities=manual_capabilities,
            auto_detect_from_mcp=True  # ALSO auto-detect from MCP
        )

        print(f"\n✅ Combined capability detection:")
        print(f"   Manual capabilities: {len(manual_capabilities)}")
        print(f"   Total reported: {capability_result['capabilities_reported']}")
        print(f"   Risk Level: {capability_result['risk_assessment']['risk_level']}")

    except Exception as e:
        print(f"\n⚠️  Capability detection failed: {e}")

    print("\n🎉 BALANCED MODE COMPLETE - Best of both worlds!")
    print("   Manual declarations + Auto-detection working together")


# ============================================================================
# MODE 3: EXPERT MODE - Full Manual Control
# ============================================================================

def example_expert_mode():
    """
    EXPERT MODE: Full manual control - zero auto-detection.

    This approach gives you complete control:
    - Explicitly declare all MCP servers
    - Explicitly declare all capabilities
    - No auto-detection whatsoever

    Perfect for: Security-critical applications, compliance requirements,
                 custom agents that don't use standard MCP servers
    """
    print("\n" + "=" * 70)
    print("MODE 3: EXPERT MODE - Full Manual Control")
    print("=" * 70)

    # Step 1: Register agent with complete manual declarations
    agent = register_agent(
        name="my-expert-agent",
        aim_url=AIM_URL,
        api_key=API_KEY,
        description="Agent using full manual control (no auto-detection)",
        agent_type="ai_agent",
        version="1.0.0",

        # MANUAL: Exhaustively list ALL MCP servers
        talks_to=[
            "filesystem-mcp-v2",
            "database-mcp-postgres",
            "github-mcp-enterprise",
            "slack-mcp-notifications",
            "email-mcp-sendgrid"
        ],

        # MANUAL: Exhaustively list ALL capabilities
        capabilities=[
            "read_files",
            "write_files",
            "execute_code",
            "database_read",
            "database_write",
            "git_operations",
            "send_notifications",
            "call_webhooks",
            "process_payments"
        ]
    )

    print(f"\n✅ Agent registered: {agent.agent_id}")
    print("   Using expert mode (100% manual control)")
    print(f"   Manual MCP servers: {5}")
    print(f"   Manual capabilities: {9}")
    print("   Auto-detection: DISABLED")

    # Step 2: Manually register each MCP server with precise configuration
    mcp_servers = [
        {
            "name": "filesystem-mcp-v2",
            "url": "http://localhost:3001",
            "public_key": "ed25519_filesystem_key",
            "capabilities": ["read", "write", "list"],
            "description": "File system access (read/write)",
            "version": "2.0.0"
        },
        {
            "name": "database-mcp-postgres",
            "url": "http://localhost:3002",
            "public_key": "ed25519_database_key",
            "capabilities": ["query", "transactions", "migrations"],
            "description": "PostgreSQL database access",
            "version": "1.5.0"
        },
        {
            "name": "github-mcp-enterprise",
            "url": "http://localhost:3003",
            "public_key": "ed25519_github_key",
            "capabilities": ["repositories", "pull_requests", "issues"],
            "description": "GitHub Enterprise integration",
            "version": "3.0.0"
        }
    ]

    print("\n✅ Manually registering MCP servers:")
    for mcp_config in mcp_servers:
        try:
            mcp = register_mcp_server(
                aim_client=agent,
                server_name=mcp_config["name"],
                server_url=mcp_config["url"],
                public_key=mcp_config["public_key"],
                capabilities=mcp_config["capabilities"],
                description=mcp_config["description"],
                version=mcp_config["version"]
            )
            print(f"   ✅ {mcp_config['name']} registered")

        except Exception as e:
            print(f"   ⚠️  {mcp_config['name']} failed: {e}")

    # Step 3: Manually report all capabilities with precise risk levels
    manual_capabilities = [
        {
            "capability_type": "file_read",
            "capability_scope": {
                "paths": ["/home/user/workspace"],
                "permissions": "read"
            },
            "risk_level": "LOW",
            "detected_via": "manual_declaration"
        },
        {
            "capability_type": "file_write",
            "capability_scope": {
                "paths": ["/home/user/workspace"],
                "permissions": "write"
            },
            "risk_level": "MEDIUM",
            "detected_via": "manual_declaration"
        },
        {
            "capability_type": "code_execution",
            "capability_scope": {
                "languages": ["python", "bash"],
                "restrictions": "sandboxed"
            },
            "risk_level": "HIGH",
            "detected_via": "manual_declaration"
        },
        {
            "capability_type": "database_write",
            "capability_scope": {
                "database": "postgres://prod-db:5432/main",
                "tables": ["users", "orders"],
                "operations": ["INSERT", "UPDATE", "DELETE"]
            },
            "risk_level": "CRITICAL",
            "detected_via": "manual_declaration"
        },
        {
            "capability_type": "external_api",
            "capability_scope": {
                "api": "https://api.github.com",
                "endpoints": ["/repos", "/issues", "/pulls"],
                "auth": "token"
            },
            "risk_level": "MEDIUM",
            "detected_via": "manual_declaration"
        }
    ]

    try:
        capability_result = auto_detect_capabilities(
            aim_client=agent,
            agent_id=agent.agent_id,
            detected_capabilities=manual_capabilities,
            auto_detect_from_mcp=False  # DISABLE auto-detection
        )

        print(f"\n✅ Manually reported capabilities:")
        print(f"   Total: {capability_result['capabilities_reported']}")
        print(f"   Risk Level: {capability_result['risk_assessment']['risk_level']}")
        print(f"   Risk Score: {capability_result['risk_assessment']['overall_risk_score']}")
        print(f"   Trust Impact: {capability_result['risk_assessment']['trust_score_impact']}")

        # Show all security alerts
        alerts = capability_result['risk_assessment']['alerts']
        if alerts:
            print(f"\n   Security Alerts ({len(alerts)}):")
            for alert in alerts:
                print(f"   [{alert['severity']}] {alert['message']}")

    except Exception as e:
        print(f"\n⚠️  Capability reporting failed: {e}")

    print("\n🎉 EXPERT MODE COMPLETE - Full manual control!")
    print("   Every capability and MCP server explicitly declared")
    print("   Zero auto-detection, maximum security control")


# ============================================================================
# MODE COMPARISON SUMMARY
# ============================================================================

def show_mode_comparison():
    """Show a comparison table of all three modes"""
    print("\n" + "=" * 70)
    print("MODE COMPARISON SUMMARY")
    print("=" * 70)

    comparison = """
┌─────────────────┬──────────────┬─────────────────┬──────────────┐
│ Feature         │ Easy Mode    │ Balanced Mode   │ Expert Mode  │
├─────────────────┼──────────────┼─────────────────┼──────────────┤
│ Setup Time      │ 1 minute     │ 5-10 minutes    │ 20+ minutes  │
│ Code Required   │ 3 lines      │ 10-20 lines     │ 50+ lines    │
│ Auto-Detection  │ 100% Auto    │ Hybrid          │ 0% Auto      │
│ Manual Control  │ None         │ Partial         │ 100%         │
│ Security Level  │ Good         │ Better          │ Best         │
│ Flexibility     │ Low          │ High            │ Maximum      │
│ Recommended For │ Quick Start  │ Production      │ Compliance   │
│                 │ Prototyping  │ Most Agents     │ Critical Ops │
└─────────────────┴──────────────┴─────────────────┴──────────────┘

EASY MODE:
  ✅ Perfect for getting started quickly
  ✅ Automatically detects everything
  ✅ Zero configuration required
  ⚠️  Less control over what's registered

BALANCED MODE (RECOMMENDED):
  ✅ Best of both worlds
  ✅ Auto-detect common components
  ✅ Manual control for critical parts
  ✅ Production-ready

EXPERT MODE:
  ✅ Maximum security and control
  ✅ Perfect for compliance requirements
  ✅ Explicit declaration of everything
  ⚠️  Requires more setup time
  ⚠️  No auto-detection convenience
"""

    print(comparison)

    print("\nCHOOSING THE RIGHT MODE:")
    print("  • Start with EASY MODE to learn the platform")
    print("  • Move to BALANCED MODE for production agents")
    print("  • Use EXPERT MODE for security-critical applications")
    print("\nREMEMBER: You can mix and match approaches!")
    print("  • Start with auto-detection, add manual declarations later")
    print("  • Use auto-detection during development, manual in production")
    print("  • Different agents can use different modes\n")


# ============================================================================
# MAIN - Run All Examples
# ============================================================================

def main():
    """Run all mode examples"""
    print("=" * 70)
    print("AIM SDK - Manual vs Auto-Detection Examples")
    print("=" * 70)
    print("Demonstrates the flexibility spectrum:")
    print("  1. EASY MODE: Full auto-detection")
    print("  2. BALANCED MODE: Auto-detect + manual additions")
    print("  3. EXPERT MODE: Full manual control")
    print("=" * 70)

    try:
        # Example 1: Easy Mode
        example_easy_mode()

        # Example 2: Balanced Mode
        example_balanced_mode()

        # Example 3: Expert Mode
        example_expert_mode()

        # Show comparison
        show_mode_comparison()

        print("\n" + "=" * 70)
        print("🎉 ALL EXAMPLES COMPLETED SUCCESSFULLY!")
        print("=" * 70)
        print("\nNEXT STEPS:")
        print("  1. Choose the mode that fits your use case")
        print("  2. Copy the relevant example code")
        print("  3. Customize for your specific agent")
        print("  4. Test with your AIM backend")
        print("\nDOCUMENTATION:")
        print("  • API Reference: https://docs.aim.example.com/api")
        print("  • SDK Guide: https://docs.aim.example.com/sdk")
        print("  • Security Best Practices: https://docs.aim.example.com/security")
        print()

        return 0

    except KeyboardInterrupt:
        print("\n\n⚠️  Examples interrupted by user")
        return 1

    except Exception as e:
        print(f"\n\n❌ Examples failed: {e}")
        import traceback
        traceback.print_exc()
        return 1


if __name__ == "__main__":
    sys.exit(main())
