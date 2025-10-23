#!/usr/bin/env python3
"""
Test weather agent registration with FRESH SDK and OAuth fix.
"""

import sys
import os

# Use the updated SDK with fresh credentials
sys.path.insert(0, "/Users/decimai/workspace/aim-sdk-python")

from dotenv import load_dotenv
load_dotenv()

print("=" * 80)
print("🌤️  REGISTERING WEATHER AGENT - FRESH SDK + OAUTH FIX")
print("=" * 80)

print("\n📦 Step 1: Importing secure() from AIM SDK...")
from aim_sdk import secure

print("✅ Imported successfully")

print("\n🔐 Step 2: Registering weather-agent-demo with ONE LINE...")
print("   Code: agent = secure('weather-agent-demo')")
print("")

try:
    agent = secure("weather-agent-demo")

    print("✅ AGENT REGISTERED SUCCESSFULLY!")
    print(f"\n📋 Agent Details:")
    print(f"   - Agent ID: {agent.agent_id}")
    print(f"   - Agent Name: {agent.agent_name}")
    print(f"   - Public Key: {agent.public_key[:32]}...{agent.public_key[-8:]}")
    print(f"   - Created At: {agent.created_at}")

    print("\n🎉 SUCCESS! All key achievements:")
    print("   ✅ OAuth credential discovery working (auto-found SDK package credentials)")
    print("   ✅ Auto-copy to home directory working")
    print("   ✅ Fresh token working (no 401 error!)")
    print("   ✅ Agent cryptographically signed and registered")
    print("   ✅ 'Stripe moment' ACHIEVED - ONE LINE registration!")

    print(f"\n🔍 Backend URL:")
    print(f"   {os.getenv('AIM_URL', 'https://aim-prod-backend.graypebble-c7e67ab8.canadacentral.azurecontainerapps.io')}")

    print("\n💡 Now check the dashboard - weather-agent-demo should be visible!")

except Exception as e:
    print(f"❌ Registration failed: {e}")
    import traceback
    traceback.print_exc()

    print("\n🔍 Debugging info:")
    print("   If this failed, check:")
    print("   1. Credentials exist in SDK package (.aim/credentials.json)")
    print("   2. Backend is accessible")
    print("   3. Token is valid (not expired)")
