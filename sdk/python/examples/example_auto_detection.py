#!/usr/bin/env python3
"""
🔍 Auto-Detection Example

This example demonstrates AIM's automatic capability and MCP server detection.
NO BACKEND REQUIRED - you can run this right now!
"""

print("=" * 70)
print("🔍 AIM SDK - Auto-Detection Demo")
print("=" * 70)
print()

# ============================================================================
# PART 1: CAPABILITY AUTO-DETECTION
# ============================================================================

print("📋 PART 1: Automatic Capability Detection")
print("-" * 70)
print()

print("AIM automatically detects capabilities from your Python imports!")
print()

# Import some common packages
print("Importing packages...")
import requests      # HTTP client
import smtplib       # Email
import subprocess    # Code execution
import os            # File operations

print("  ✓ requests")
print("  ✓ smtplib")
print("  ✓ subprocess")
print("  ✓ os")
print()

# Now detect capabilities
from aim_sdk import auto_detect_capabilities

capabilities = auto_detect_capabilities()

print(f"🎉 AIM detected {len(capabilities)} capabilities:")
print()
for i, cap in enumerate(capabilities, 1):
    icon = "📡" if "api" in cap else "📧" if "email" in cap else "💾" if "file" in cap else "⚡"
    print(f"  {i}. {icon} {cap}")
print()

print("How it works:")
print("  • requests → make_api_calls")
print("  • smtplib → send_email")
print("  • subprocess → execute_code")
print("  • os + builtins → read_files, write_files")
print()

# ============================================================================
# PART 2: MCP SERVER AUTO-DETECTION
# ============================================================================

print("=" * 70)
print("📡 PART 2: Automatic MCP Server Detection")
print("-" * 70)
print()

from aim_sdk import auto_detect_mcps

print("AIM looks for MCP servers in:")
print("  • Claude Desktop config (~/.claude/claude_desktop_config.json)")
print("  • Python imports (mcp-* packages)")
print()

mcps = auto_detect_mcps()

if mcps:
    print(f"🎉 AIM detected {len(mcps)} MCP servers:")
    print()
    for i, mcp in enumerate(mcps, 1):
        confidence_icon = "🟢" if mcp['confidence'] == 100 else "🟡"
        print(f"  {i}. {mcp['mcpServer']}")
        print(f"     {confidence_icon} Confidence: {mcp['confidence']}%")
        print(f"     📍 Method: {mcp['detectionMethod']}")
        print(f"     🔧 Command: {mcp.get('command', 'N/A')}")
        print()
else:
    print("ℹ️  No MCP servers detected")
    print()
    print("To test MCP detection:")
    print("  1. Install Claude Desktop")
    print("  2. Configure MCP servers in ~/.claude/claude_desktop_config.json")
    print("  3. Run this script again")
    print()

# ============================================================================
# PART 3: COMPLETE DETECTION SUMMARY
# ============================================================================

print("=" * 70)
print("📊 DETECTION SUMMARY")
print("=" * 70)
print()

print(f"✅ Capabilities detected: {len(capabilities)}")
for cap in capabilities:
    print(f"   • {cap}")
print()

print(f"✅ MCP servers detected: {len(mcps)}")
if mcps:
    for mcp in mcps:
        print(f"   • {mcp['mcpServer']} ({mcp['confidence']}%)")
else:
    print("   • None (install Claude Desktop to test)")
print()

# ============================================================================
# PART 4: WHAT HAPPENS DURING REGISTRATION
# ============================================================================

print("=" * 70)
print("🚀 What Happens During register_agent()")
print("=" * 70)
print()

print("When you call register_agent('my-agent'), AIM automatically:")
print()
print("1. 🔍 Auto-detects capabilities (from imports + decorators + config)")
print("2. 📡 Auto-detects MCP servers (from Claude config + imports)")
print("3. 🔐 Generates Ed25519 key pair (cryptographically secure)")
print("4. 📤 Registers agent with AIM backend")
print("5. ✅ Performs challenge-response verification")
print("6. 💾 Saves credentials to ~/.aim/credentials.json")
print("7. 🎉 Returns ready-to-use AIMClient")
print()

print("All of this from ONE LINE:")
print()
print("  from aim_sdk import register_agent")
print("  agent = register_agent('my-agent')")
print()

# ============================================================================
# PART 5: TRY IT YOURSELF
# ============================================================================

print("=" * 70)
print("💡 Try It Yourself")
print("=" * 70)
print()

print("Option 1: Zero-Config (SDK Download from Dashboard)")
print("-" * 70)
print()
print("from aim_sdk import register_agent")
print()
print("# Download SDK from dashboard, then just:")
print("agent = register_agent('my-agent')")
print()
print("# That's it! No API key, no URL, nothing!")
print()

print("Option 2: Manual Mode (download SDK from dashboard)")
print("-" * 70)
print()
print("from aim_sdk import register_agent")
print()
print("agent = register_agent(")
print("    'my-agent',")
print("    aim_url='http://localhost:8080',")
print("    api_key='aim_your_key_here'")
print(")")
print()
print("# Still auto-detects capabilities & MCPs!")
print()

print("Option 3: Power User (Full Control)")
print("-" * 70)
print()
print("from aim_sdk import register_agent")
print()
print("agent = register_agent(")
print("    'my-agent',")
print("    aim_url='http://localhost:8080',")
print("    api_key='aim_your_key_here',")
print("    auto_detect=False,  # Disable auto-detection")
print("    capabilities=['custom_capability'],")
print("    talks_to=['custom-mcp-server']")
print(")")
print()

print("=" * 70)
print("✨ Enterprise security simplified to one line of code!")
print("=" * 70)
print()
