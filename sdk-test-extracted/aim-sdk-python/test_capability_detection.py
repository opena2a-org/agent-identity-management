#!/usr/bin/env python3
"""
Test script for auto-capability detection feature in AIM SDK.

This script demonstrates how to use the auto_detect_capabilities function
to report agent capabilities to the AIM backend for risk assessment.
"""

from aim_sdk.integrations.mcp import auto_detect_capabilities, get_agent_capabilities
from aim_sdk.client import AIMClient

# Mock AIM client for testing (replace with actual credentials)
# In production, use: AIMClient.auto_register_or_load()
def test_auto_detect_capabilities():
    """Test capability auto-detection feature."""

    # Example 1: Auto-detect from MCP servers
    print("=" * 60)
    print("Example 1: Auto-detect capabilities from MCP servers")
    print("=" * 60)

    try:
        # This would connect to AIM backend and auto-detect capabilities
        # from registered MCP servers
        result = auto_detect_capabilities(
            aim_client=aim_client,
            agent_id="550e8400-e29b-41d4-a716-446655440000",
            auto_detect_from_mcp=True
        )

        print(f"✅ Capabilities reported: {result['capabilities_reported']}")
        print(f"📊 Risk Level: {result['risk_assessment']['risk_level']}")
        print(f"📈 Risk Score: {result['risk_assessment']['overall_risk_score']}")
        print(f"🎯 Trust Score Impact: {result['risk_assessment']['trust_score_impact']}")

        if result['risk_assessment']['alerts']:
            print("\n⚠️ Security Alerts:")
            for alert in result['risk_assessment']['alerts']:
                print(f"  - [{alert['severity']}] {alert['message']}")

    except Exception as e:
        print(f"❌ Error: {e}")

    # Example 2: Manually report specific capabilities
    print("\n" + "=" * 60)
    print("Example 2: Manually report specific capabilities")
    print("=" * 60)

    try:
        capabilities = [
            {
                "capability_type": "file_read",
                "capability_scope": {
                    "paths": ["/etc/hosts", "/var/log"],
                    "permissions": "read"
                },
                "risk_level": "MEDIUM",
                "detected_via": "static_analysis"
            },
            {
                "capability_type": "database_write",
                "capability_scope": {
                    "database": "postgres://prod-db",
                    "tables": ["users", "transactions"]
                },
                "risk_level": "HIGH",
                "detected_via": "runtime"
            },
            {
                "capability_type": "code_execution",
                "capability_scope": {
                    "languages": ["python", "javascript"],
                    "restricted": False
                },
                "risk_level": "CRITICAL",
                "detected_via": "mcp_tool"
            }
        ]

        result = auto_detect_capabilities(
            aim_client=aim_client,
            agent_id="550e8400-e29b-41d4-a716-446655440000",
            detected_capabilities=capabilities,
            auto_detect_from_mcp=False
        )

        print(f"✅ Capabilities reported: {result['capabilities_reported']}")
        print(f"📊 Risk Level: {result['risk_assessment']['risk_level']}")
        print(f"📈 Risk Score: {result['risk_assessment']['overall_risk_score']}")
        print(f"🎯 Trust Score Impact: {result['risk_assessment']['trust_score_impact']}")
        print(f"🆕 New capabilities: {result['new_capabilities']}")
        print(f"📋 Existing capabilities: {result['existing_capabilities']}")

        if result['risk_assessment']['alerts']:
            print("\n⚠️ Security Alerts:")
            for alert in result['risk_assessment']['alerts']:
                severity_emoji = {
                    'CRITICAL': '🔴',
                    'HIGH': '🟠',
                    'MEDIUM': '🟡',
                    'LOW': '🟢'
                }.get(alert['severity'], '⚪')

                print(f"  {severity_emoji} [{alert['severity']}] {alert['message']}")
                print(f"     Capability: {alert.get('capability_type', 'N/A')}")

    except Exception as e:
        print(f"❌ Error: {e}")

    # Example 3: Get agent capabilities
    print("\n" + "=" * 60)
    print("Example 3: Get all agent capabilities")
    print("=" * 60)

    try:
        capabilities = get_agent_capabilities(
            aim_client=aim_client,
            agent_id="550e8400-e29b-41d4-a716-446655440000"
        )

        print(f"✅ Total capabilities: {capabilities['total']}")

        if capabilities['capabilities']:
            print("\n📋 Capability List:")
            for cap in capabilities['capabilities']:
                print(f"  - {cap['capability_type']}")
                print(f"    Risk Level: {cap.get('risk_level', 'UNKNOWN')}")
                print(f"    Granted At: {cap.get('granted_at', 'N/A')}")
                if cap.get('capability_scope'):
                    print(f"    Scope: {cap['capability_scope']}")

    except Exception as e:
        print(f"❌ Error: {e}")


if __name__ == "__main__":
    print("\n🚀 AIM SDK - Auto-Capability Detection Test\n")

    # Note: This is a test script demonstrating the API
    # In production, replace with actual AIM client initialization:
    #
    # aim_client = AIMClient.auto_register_or_load(
    #     agent_name="my-agent",
    #     aim_url="http://localhost:8080"
    # )

    print("⚠️ This is a test script demonstrating the API interface.")
    print("⚠️ To run actual tests, initialize AIMClient with valid credentials.\n")

    # Uncomment to run actual tests:
    # test_auto_detect_capabilities()

    print("\n✅ API Interface:")
    print("   - auto_detect_capabilities(aim_client, agent_id, detected_capabilities, auto_detect_from_mcp)")
    print("   - get_agent_capabilities(aim_client, agent_id)")
    print("\n✅ Endpoint: POST /api/v1/detection/agents/:id/capabilities/report")
    print("✅ Backend Handler: DetectionHandler.ReportCapabilities")
    print("✅ Backend Service: CapabilityService.AutoDetectCapabilities")
