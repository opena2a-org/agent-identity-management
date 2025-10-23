#!/bin/bash
SDK_PATH="/Users/decimai/workspace/aim-sdk-python"

echo "Applying OAuth fixes to fresh SDK..."

# Fix 1: client.py
sed -i '' '/Initialize OAuth token manager with SDK credentials path/,/token_manager = OAuthTokenManager/c\
    # Initialize OAuth token manager with intelligent credential discovery\
    # NO path argument - let OAuthTokenManager use its _discover_credentials_path() method\
    # This will check: home dir → SDK package dir → current dir (in that order)\
    print(f"[DEBUG] Creating OAuthTokenManager with intelligent credential discovery...")\
    token_manager = OAuthTokenManager()  # ✅ NO path argument!\
    print(f"[DEBUG] OAuthTokenManager created, credentials path: {token_manager.credentials_path}")\
    print(f"[DEBUG] Calling get_access_token()...")
' "$SDK_PATH/aim_sdk/client.py"

echo "✅ Client.py fixed"

# Fix 2: oauth.py - Add intelligent credential discovery
python << 'PYTHON'
import sys
sys.path.insert(0, "/Users/decimai/workspace/aim-sdk-python")
from pathlib import Path

oauth_file = Path("/Users/decimai/workspace/aim-sdk-python/aim_sdk/oauth.py")
content = oauth_file.read_text()

# Add _discover_credentials_path method if not present
if "_discover_credentials_path" not in content:
    # Find the __init__ method and update it
    updated = content.replace(
        '    def __init__(self, credentials_path: Optional[str] = None, use_secure_storage: bool = True):',
        '''    def __init__(self, credentials_path: Optional[str] = None, use_secure_storage: bool = True):
        """
        Initialize OAuth token manager with intelligent credential discovery.

        Args:
            credentials_path: Path to credentials.json file (default: auto-discover)
            use_secure_storage: Use encrypted storage if available (default: True)
        """'''
    )
    
    # Update the credentials_path assignment
    updated = updated.replace(
        '''        self.credentials_path = Path(credentials_path) if credentials_path else Path.home() / ".aim" / "credentials.json"''',
        '''        if credentials_path:
            self.credentials_path = Path(credentials_path)
        else:
            # Intelligent credential discovery
            self.credentials_path = self._discover_credentials_path()'''
    )
    
    # Add the _discover_credentials_path method after __init__
    init_end = updated.find('        # Load credentials if they exist')
    if init_end > 0:
        method_code = '''

    def _discover_credentials_path(self) -> Path:
        """Intelligently discover credentials location with auto-copy for downloaded SDKs."""
        import shutil
        home_creds = Path.home() / ".aim" / "credentials.json"
        if home_creds.exists():
            return home_creds
        try:
            import aim_sdk
            sdk_package_root = Path(aim_sdk.__file__).parent.parent
            sdk_creds = sdk_package_root / ".aim" / "credentials.json"
            if sdk_creds.exists():
                try:
                    home_creds.parent.mkdir(parents=True, exist_ok=True)
                    shutil.copy(sdk_creds, home_creds)
                    import os
                    os.chmod(home_creds, 0o600)
                    print(f"✅ SDK credentials installed to {home_creds}")
                    return home_creds
                except Exception as e:
                    return sdk_creds
        except:
            pass
        cwd_creds = Path.cwd() / ".aim" / "credentials.json"
        if cwd_creds.exists():
            return cwd_creds
        return home_creds
'''
        updated = updated[:init_end] + method_code + '\n    ' + updated[init_end:]
    
    oauth_file.write_text(updated)
    print("✅ OAuth.py updated with intelligent credential discovery")
else:
    print("ℹ️  OAuth.py already has intelligent credential discovery")

PYTHON

echo "✅ All fixes applied to fresh SDK"
