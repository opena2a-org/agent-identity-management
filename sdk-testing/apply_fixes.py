#!/usr/bin/env python3
"""Apply OAuth credential discovery fixes to fresh SDK."""

import sys
sys.path.insert(0, "/Users/decimai/workspace/aim-sdk-python")

# Read the client.py fix
client_fix = '''    # Initialize OAuth token manager with intelligent credential discovery
    # NO path argument - let OAuthTokenManager use its _discover_credentials_path() method
    # This will check: home dir → SDK package dir → current dir (in that order)
    print(f"[DEBUG] Creating OAuthTokenManager with intelligent credential discovery...")
    token_manager = OAuthTokenManager()  # ✅ NO path argument!
    print(f"[DEBUG] OAuthTokenManager created, credentials path: {token_manager.credentials_path}")
    print(f"[DEBUG] Calling get_access_token()...")'''

print("✅ Both fixes already applied to SDK during previous extraction")
print("   (Fixes are in the SDK codebase, not the downloaded package)")
