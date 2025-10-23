#!/bin/bash
SDK_PATH="/Users/decimai/workspace/aim-sdk-python"

echo "Applying OAuth credential discovery fixes..."

# Fix 1: client.py - Use intelligent credential discovery
sed -i '' '/Initialize OAuth token manager with SDK credentials path/,/token_manager = OAuthTokenManager/c\
    # Initialize OAuth token manager with intelligent credential discovery\
    # NO path argument - let OAuthTokenManager use its _discover_credentials_path() method\
    # This will check: home dir → SDK package dir → current dir (in that order)\
    print(f"[DEBUG] Creating OAuthTokenManager with intelligent credential discovery...")\
    token_manager = OAuthTokenManager()  # ✅ NO path argument!\
    print(f"[DEBUG] OAuthTokenManager created, credentials path: {token_manager.credentials_path}")\
    print(f"[DEBUG] Calling get_access_token()...")
' "$SDK_PATH/aim_sdk/client.py"

echo "✅ Applied client.py fix"
echo "⚠️  oauth.py fix requires manual application (too complex for sed)"
