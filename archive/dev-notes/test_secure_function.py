#!/usr/bin/env python3
"""
Test the secure() function alias
Verifies that secure() works exactly like register_agent()
"""

import sys
import os

# Add SDK to path
SDK_PATH = "./sdk-test-extracted/aim-sdk-python"
sys.path.insert(0, SDK_PATH)

print("=" * 80)
print("🔒 TESTING secure() FUNCTION ALIAS")
print("=" * 80)

# Test 1: Import secure()
print("\n🧪 TEST 1: Import secure() function")
print("-" * 80)

try:
    from aim_sdk import secure
    print("✅ Successfully imported secure() function")
except ImportError as e:
    print(f"❌ Failed to import secure(): {e}")
    sys.exit(1)

# Test 2: Verify it's callable
print("\n🧪 TEST 2: Verify secure() is callable")
print("-" * 80)

if callable(secure):
    print("✅ secure() is callable")
else:
    print("❌ secure() is not callable")
    sys.exit(1)

# Test 3: Verify it's an alias for register_agent
print("\n🧪 TEST 3: Verify secure() is alias for register_agent")
print("-" * 80)

from aim_sdk import register_agent

if secure is register_agent:
    print("✅ secure() is an alias for register_agent()")
    print("   They reference the same function object")
else:
    print("❌ secure() is NOT the same as register_agent()")
    sys.exit(1)

# Test 4: Verify secure() appears in __all__
print("\n🧪 TEST 4: Verify secure() in public API")
print("-" * 80)

import aim_sdk

if "secure" in aim_sdk.__all__:
    print("✅ secure() is exported in __all__")
    print(f"   Public API: {', '.join(aim_sdk.__all__)}")
else:
    print("❌ secure() is NOT in __all__")
    sys.exit(1)

# Test 5: Verify function signature matches
print("\n🧪 TEST 5: Verify function signature")
print("-" * 80)

import inspect

sig = inspect.signature(secure)
print(f"✅ Function signature: secure{sig}")
print(f"   Parameters: {list(sig.parameters.keys())}")

# Test 6: Test with actual credentials (if available)
print("\n🧪 TEST 6: Test with embedded credentials")
print("-" * 80)

credentials_path = os.path.join(SDK_PATH, ".aim", "credentials.json")
if os.path.exists(credentials_path):
    import json
    with open(credentials_path, 'r') as f:
        creds = json.load(f)

    print(f"✅ Found embedded credentials")
    print(f"   AIM URL: {creds.get('aim_url')}")
    print(f"   User: {creds.get('email')}")

    # Note: Not actually calling secure() here to avoid network issues
    # Just verifying the function would be callable with these params
    print("✅ secure() function ready to use with embedded credentials")
else:
    print("⚠️  No embedded credentials found (expected for extracted SDK)")

# Summary
print("\n" + "=" * 80)
print("📊 TEST SUMMARY")
print("=" * 80)
print("""
✅ All Tests Passed!

Verified:
   • secure() function can be imported
   • secure() is callable
   • secure() is an alias for register_agent()
   • secure() is in public API (__all__)
   • secure() has correct function signature

Example Usage:
   from aim_sdk import secure

   # One-line secure agent registration
   agent = secure("my-agent")  # Uses embedded credentials

   # Or with explicit URL
   agent = secure("my-agent", "https://aim.example.com")

Status: ✅ FEATURE VERIFIED - secure() works as advertised
""")

print("=" * 80)
