#!/usr/bin/env python3
"""
🚀 One-Line Setup for AI Agent Identity

This example demonstrates how AIM achieves radical simplicity:
enterprise-grade security with just one line of code.

ONE LINE OF CODE. ZERO CONFIGURATION. AUTOMATIC EVERYTHING.
"""

import sys

# ============================================================================
# PART 1: ONE LINE REGISTRATION - RADICAL SIMPLICITY
# ============================================================================

print("=" * 70)
print("🎉 ONE LINE OF CODE FOR ENTERPRISE SECURITY")
print("=" * 70)
print()
print("Complex security infrastructure, simplified to one line:")
print('  agent = register_agent("my-agent")')
print()
print("Behind the scenes: cryptographic verification, trust scoring,")
print("MCP attestation, audit logging, and real-time threat detection.")
print()
print("=" * 70)
print()

# THE MAGIC LINE - Everything auto-detected!
from aim_sdk import register_agent

print("🔍 Testing Zero-Config Registration (SDK Download Mode)...")
print("   (If you downloaded SDK from dashboard)")
print()

try:
    # ATTEMPT 1: Zero-config (requires SDK download)
    agent = register_agent("demo-stripe-moment")

    print("✅ ZERO-CONFIG SUCCESS!")
    print(f"   Agent ID: {agent.agent_id}")
    print(f"   AIM URL: {agent.aim_url}")
    print()
    print("What happened automatically:")
    print("  ✅ OAuth credentials loaded from SDK")
    print("  ✅ Capabilities auto-detected from imports")
    print("  ✅ MCP servers auto-detected")
    print("  ✅ Ed25519 keys generated and saved")
    print("  ✅ Challenge-response verification completed")
    print("  ✅ Agent registered and verified!")
    print()

except Exception as e:
    print("ℹ️  SDK Download Mode not available (expected if using pip install)")
    print(f"   Error: {e}")
    print()
    print("📝 Falling back to Manual Mode (API Key required)...")
    print()

    # ATTEMPT 2: Manual mode with API key
    # (This is still simple - just needs API key)
    API_KEY = "aim_test_key_12345"  # Replace with your actual API key
    AIM_URL = "http://localhost:8080"

    try:
        agent = register_agent(
            "demo-stripe-moment",
            aim_url=AIM_URL,
            api_key=API_KEY
        )

        print("✅ MANUAL MODE SUCCESS!")
        print(f"   Agent ID: {agent.agent_id}")
        print()
        print("What happened automatically:")
        print("  ✅ Capabilities auto-detected from imports")
        print("  ✅ MCP servers auto-detected")
        print("  ✅ Ed25519 keys generated and saved")
        print("  ✅ Challenge-response verification completed")
        print("  ✅ Agent registered and verified!")
        print()

    except Exception as e:
        print(f"❌ Manual mode failed: {e}")
        print()
        print("💡 To run this example:")
        print("   1. Start AIM backend: cd apps/backend && go run cmd/server/main.go")
        print("   2. Get API key from dashboard")
        print("   3. Update API_KEY and AIM_URL in this file")
        print("   4. Run: python example_stripe_moment.py")
        sys.exit(1)

# ============================================================================
# PART 2: AUTOMATIC CAPABILITY DETECTION
# ============================================================================

print("=" * 70)
print("🔍 AUTOMATIC CAPABILITY DETECTION")
print("=" * 70)
print()

from aim_sdk import auto_detect_capabilities

# Import some packages to demonstrate detection
import requests  # → Should detect "make_api_calls"
import smtplib   # → Should detect "send_email"

capabilities = auto_detect_capabilities()

print("Detected capabilities from your imports:")
for cap in capabilities:
    print(f"  ✅ {cap}")
print()

# ============================================================================
# PART 3: AUTOMATIC MCP SERVER DETECTION
# ============================================================================

print("=" * 70)
print("📡 AUTOMATIC MCP SERVER DETECTION")
print("=" * 70)
print()

from aim_sdk import auto_detect_mcps

mcps = auto_detect_mcps()

if mcps:
    print(f"Detected {len(mcps)} MCP servers:")
    for mcp in mcps:
        print(f"  ✅ {mcp['mcpServer']} ({mcp['detectionMethod']}, {mcp['confidence']}% confidence)")
else:
    print("ℹ️  No MCP servers detected (expected if Claude Desktop not configured)")
print()

# ============================================================================
# PART 4: VERIFIED ACTIONS (The Real Power)
# ============================================================================

print("=" * 70)
print("🔐 VERIFIED ACTIONS")
print("=" * 70)
print()

print("Now you can perform actions with automatic verification:")
print()

# Example: Verified database read
@agent.perform_action("read_database", resource="users_table")
def get_user_count():
    """
    This function is automatically verified by AIM before execution.
    AIM:
    - Verifies agent identity (Ed25519 signature)
    - Creates audit log entry
    - Checks trust score
    - Returns cryptographic proof
    """
    print("  📊 Querying database...")
    return {"count": 42, "table": "users"}

# Example: Verified API call
@agent.perform_action("make_api_call", resource="https://api.example.com/data")
def fetch_external_data():
    """High-trust action with full audit trail"""
    print("  🌐 Calling external API...")
    return {"status": "success", "data": [1, 2, 3]}

# Execute verified actions
print("1️⃣  Getting user count (auto-verified)...")
try:
    result = get_user_count()
    print(f"   ✅ Result: {result}")
except Exception as e:
    print(f"   ⚠️  Action requires backend connection: {e}")
print()

print("2️⃣  Fetching external data (auto-verified)...")
try:
    result = fetch_external_data()
    print(f"   ✅ Result: {result}")
except Exception as e:
    print(f"   ⚠️  Action requires backend connection: {e}")
print()

# ============================================================================
# PART 5: COMPARISON - THE "STRIPE MOMENT"
# ============================================================================

print("=" * 70)
print("💡 THE 'STRIPE MOMENT' - BEFORE vs AFTER")
print("=" * 70)
print()

print("BEFORE AIM (Old Way):")
print("  ❌ Manual key generation (openssl genrsa -out private.pem 2048)")
print("  ❌ Manual registration API calls")
print("  ❌ Manual credential storage")
print("  ❌ Manual capability declaration")
print("  ❌ Manual MCP server registration")
print("  ❌ Manual verification on every action")
print("  ❌ Manual audit logging")
print("  ❌ 100+ lines of boilerplate code")
print()

print("AFTER AIM (New Way):")
print("  ✅ ONE LINE: agent = register_agent('my-agent')")
print("  ✅ Everything automatic:")
print("     - Key generation (Ed25519)")
print("     - Registration")
print("     - Credential storage")
print("     - Capability detection")
print("     - MCP server detection")
print("     - Action verification")
print("     - Audit logging")
print("  ✅ 1 line of code vs 100+ lines")
print()

print("=" * 70)
print("🎉 THAT'S THE 'STRIPE MOMENT' FOR AI AGENT IDENTITY!")
print("=" * 70)
print()

print("📝 Next Steps:")
print("   1. Check AIM dashboard for your registered agent")
print("   2. View audit logs for verified actions")
print("   3. See auto-detected capabilities and MCP servers")
print("   4. Integrate into your production agent code")
print()

print("💾 Credentials stored at: ~/.aim/credentials.json")
print("   (Private key never leaves your machine!)")
print()

print("🚀 Ready to deploy? Just add one line to your agent:")
print("   from aim_sdk import register_agent")
print("   agent = register_agent('my-production-agent')")
print()
