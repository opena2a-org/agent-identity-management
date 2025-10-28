"""
SDK token management for AIM SDK.

This module handles SDK authentication tokens (refresh/access tokens) for the SDK download mode.
Note: This is NOT OAuth provider authentication (Google/Microsoft/Okta) - that was removed.
This manages the JWT tokens embedded in downloaded SDKs for zero-config authentication.

Handles automatic token refresh with token rotation and secure storage.
"""

import os
import json
import time
from pathlib import Path
from typing import Optional, Dict, Any
import requests

from .exceptions import AuthenticationError

# Try to import secure storage (optional dependency)
try:
    from .secure_storage import SecureCredentialStorage
    SECURE_STORAGE_AVAILABLE = True
except ImportError:
    SECURE_STORAGE_AVAILABLE = False


class OAuthTokenManager:
    """
    Manages OAuth tokens with automatic refresh and token rotation.

    Security features:
    - Automatic token refresh when expired
    - Token rotation: new refresh token on each refresh
    - Secure encrypted storage (if cryptography + keyring installed)
    - Automatic credential updates when tokens rotate
    """

    def __init__(self, credentials_path: Optional[str] = None, use_secure_storage: bool = True):
        """
        Initialize OAuth token manager with intelligent credential discovery.

        Args:
            credentials_path: Path to credentials.json file (default: auto-discover)
            use_secure_storage: Use encrypted storage if available (default: True)
        """
        if credentials_path:
            self.credentials_path = Path(credentials_path)
        else:
            # Intelligent credential discovery
            self.credentials_path = self._discover_credentials_path()

        self.credentials: Optional[Dict[str, Any]] = None
        self.access_token: Optional[str] = None
        self.access_token_expiry: Optional[float] = None

        # Use secure storage if available and requested
        self.use_secure_storage = use_secure_storage and SECURE_STORAGE_AVAILABLE
        if self.use_secure_storage:
            self.secure_storage = SecureCredentialStorage(str(self.credentials_path))
        else:
            self.secure_storage = None

        # Load credentials if they exist
        if self._credentials_exist():
            self.load_credentials()

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

    def _credentials_exist(self) -> bool:
        """Check if credentials exist (encrypted or plaintext)."""
        if self.secure_storage:
            return self.secure_storage.credentials_exist()
        return self.credentials_path.exists()

    def load_credentials(self) -> bool:
        """
        Load credentials from file (encrypted or plaintext).

        Returns:
            True if credentials were loaded successfully
        """
        try:
            if self.secure_storage:
                # Try secure storage first
                self.credentials = self.secure_storage.load_credentials()
                if self.credentials:
                    return True

            # Fall back to plaintext
            if self.credentials_path.exists():
                with open(self.credentials_path, 'r') as f:
                    self.credentials = json.load(f)
                return True

            return False

        except Exception as e:
            print(f"⚠️  Warning: Failed to load credentials: {e}")
            return False

    def save_credentials(self, credentials: Dict[str, Any]) -> bool:
        """
        Save credentials securely.

        Args:
            credentials: Credentials dictionary to save

        Returns:
            True if saved successfully
        """
        try:
            if self.secure_storage:
                self.secure_storage.save_credentials(credentials)
            else:
                # Fall back to plaintext
                self.credentials_path.parent.mkdir(parents=True, exist_ok=True)
                with open(self.credentials_path, 'w') as f:
                    json.dump(credentials, f, indent=2)
                # Set restrictive permissions
                os.chmod(self.credentials_path, 0o600)

            self.credentials = credentials
            return True

        except Exception as e:
            print(f"⚠️  Warning: Failed to save credentials: {e}")
            return False

    def has_credentials(self) -> bool:
        """Check if credentials are available."""
        return self.credentials is not None

    def get_access_token(self) -> Optional[str]:
        """
        Get a valid access token, refreshing if necessary.

        Returns:
            Valid access token or None if not available
        """
        if not self.credentials:
            return None

        # Check if current token is still valid (with 60s buffer)
        if self.access_token and self.access_token_expiry:
            if time.time() < (self.access_token_expiry - 60):
                return self.access_token

        # Need to refresh token
        return self._refresh_token()

    def _refresh_token(self) -> Optional[str]:
        """
        Refresh access token using refresh token.

        Implements token rotation:
        - Server returns new access_token AND new refresh_token
        - Old refresh token is invalidated
        - New refresh token is saved to credentials

        Returns:
            New access token or None if refresh failed
        """
        if not self.credentials or 'refresh_token' not in self.credentials:
            return None

        aim_url = self.credentials.get('aim_url', 'http://localhost:8080')
        refresh_token = self.credentials['refresh_token']

        try:
            # Call token refresh endpoint (with rotation support)
            refresh_url = f"{aim_url.rstrip('/')}/api/v1/auth/refresh"

            response = requests.post(
                refresh_url,
                json={"refresh_token": refresh_token},
                timeout=10
            )

            if response.status_code != 200:
                error_data = response.json() if response.headers.get('content-type', '').startswith('application/json') else {}
                error_msg = error_data.get('error', response.text)

                # Check if token was revoked/expired - try automatic recovery
                if 'revoked' in error_msg.lower() or 'invalid' in error_msg.lower():
                    print("🔄 Token was revoked - attempting automatic recovery...")

                    # Try token recovery endpoint (new feature - zero downtime!)
                    recovery_url = f"{aim_url.rstrip('/')}/api/v1/auth/sdk/recover"
                    try:
                        recovery_response = requests.post(
                            recovery_url,
                            json={"old_refresh_token": refresh_token},
                            timeout=10
                        )

                        if recovery_response.status_code == 200:
                            recovery_data = recovery_response.json()
                            self.access_token = recovery_data.get('access_token')
                            new_refresh_token = recovery_data.get('refresh_token')

                            if new_refresh_token:
                                # Save recovered credentials
                                self.credentials['refresh_token'] = new_refresh_token

                                # Update sdk_token_id
                                import base64
                                try:
                                    token_parts = new_refresh_token.split('.')
                                    if len(token_parts) == 3:
                                        payload_part = token_parts[1]
                                        padding = 4 - len(payload_part) % 4
                                        if padding != 4:
                                            payload_part += '=' * padding
                                        token_payload = json.loads(base64.b64decode(payload_part))
                                        new_token_id = token_payload.get('jti')
                                        if new_token_id:
                                            self.credentials['sdk_token_id'] = new_token_id
                                except Exception:
                                    pass

                                self.save_credentials(self.credentials)
                                print("✅ Token recovered automatically! SDK credentials updated.")
                                print("💡 No need to re-download the SDK - everything just works!")

                                # Decode new access token expiry
                                try:
                                    import base64
                                    payload_part = self.access_token.split('.')[1]
                                    padding = 4 - len(payload_part) % 4
                                    if padding != 4:
                                        payload_part += '=' * padding
                                    payload = json.loads(base64.b64decode(payload_part))
                                    self.access_token_expiry = payload.get('exp')
                                except Exception:
                                    self.access_token_expiry = time.time() + 3600

                                return self.access_token

                    except Exception as recovery_error:
                        # Recovery failed - fall back to manual instructions
                        pass

                    # If recovery failed, show manual instructions
                    print("\n" + "=" * 80)
                    print("⚠️  SDK TOKEN EXPIRED OR REVOKED")
                    print("=" * 80)
                    print("\nYour SDK refresh token is no longer valid. This can happen if:")
                    print("  • The token expired (90 days since last use)")
                    print("  • The token was revoked for security reasons")
                    print("  • Another SDK/tool rotated your token")
                    print("\n📥 TO FIX: Download a fresh SDK from the AIM dashboard")
                    print(f"   1. Visit: {aim_url}")
                    print("   2. Go to Settings → SDK Downloads")
                    print("   3. Click 'Download Python SDK'")
                    print("   4. Extract and run your code again")
                    print("\n💡 TIP: Your agents and data are safe! Only the SDK credentials need updating.")
                    print("=" * 80 + "\n")
                else:
                    print(f"⚠️  Token refresh failed with status {response.status_code}: {error_msg}")

                return None

            data = response.json()
            self.access_token = data.get('access_token')

            # Check if server returned new refresh token (token rotation)
            new_refresh_token = data.get('refresh_token')
            if new_refresh_token and new_refresh_token != refresh_token:
                # Token rotation: save new refresh token
                self.credentials['refresh_token'] = new_refresh_token

                # Also update sdk_token_id if present in the new token
                import base64
                try:
                    # Decode new refresh token to get JTI
                    token_parts = new_refresh_token.split('.')
                    if len(token_parts) == 3:
                        payload_part = token_parts[1]
                        padding = 4 - len(payload_part) % 4
                        if padding != 4:
                            payload_part += '=' * padding
                        token_payload = json.loads(base64.b64decode(payload_part))
                        new_token_id = token_payload.get('jti')
                        if new_token_id:
                            self.credentials['sdk_token_id'] = new_token_id
                except Exception:
                    pass  # Continue even if JTI extraction fails

                self.save_credentials(self.credentials)
                print("🔄 Token rotated successfully")

            # Decode token to get expiry (JWT format)
            if self.access_token:
                try:
                    # JWT tokens are base64 encoded: header.payload.signature
                    import base64
                    payload_part = self.access_token.split('.')[1]
                    # Add padding if needed
                    padding = 4 - len(payload_part) % 4
                    if padding != 4:
                        payload_part += '=' * padding

                    payload = json.loads(base64.b64decode(payload_part))
                    self.access_token_expiry = payload.get('exp')
                except Exception as e:
                    print(f"⚠️  Warning: Failed to decode token expiry: {e}")
                    # Assume 1 hour expiry if we can't decode
                    self.access_token_expiry = time.time() + 3600

            return self.access_token

        except Exception as e:
            print(f"⚠️  Warning: Token refresh failed: {e}")
            return None

    def get_auth_header(self) -> Dict[str, str]:
        """
        Get authorization header with current access token.

        Returns:
            Dictionary with Authorization header or empty dict
        """
        token = self.get_access_token()
        if token:
            return {"Authorization": f"Bearer {token}"}
        return {}

    def revoke_token(self) -> bool:
        """
        Revoke the current refresh token.

        This should be called when the user wants to log out
        or revoke SDK access.

        Returns:
            True if revocation successful
        """
        if not self.credentials or 'refresh_token' not in self.credentials:
            return False

        aim_url = self.credentials.get('aim_url', 'http://localhost:8080')
        refresh_token = self.credentials['refresh_token']

        try:
            # Call token revocation endpoint (if implemented)
            response = requests.post(
                f"{aim_url.rstrip('/')}/api/v1/auth/revoke",
                json={"refresh_token": refresh_token},
                timeout=10
            )

            # Delete local credentials regardless of server response
            if self.secure_storage:
                self.secure_storage.delete_credentials()
            elif self.credentials_path.exists():
                self.credentials_path.unlink()

            self.credentials = None
            self.access_token = None
            self.access_token_expiry = None

            print("✅ Token revoked and credentials deleted")
            return True

        except Exception as e:
            print(f"⚠️  Warning: Token revocation failed: {e}")
            # Still delete local credentials for safety
            if self.secure_storage:
                self.secure_storage.delete_credentials()
            elif self.credentials_path.exists():
                self.credentials_path.unlink()

            return False


def load_sdk_credentials(use_secure_storage: bool = True) -> Optional[Dict[str, Any]]:
    """
    Load credentials from SDK-embedded location.

    This function looks for credentials in multiple locations:
    1. Current directory (./.aim/credentials.json) - for SDK download
    2. Home directory (~/.aim/credentials.json) - for installed SDK

    Args:
        use_secure_storage: Try encrypted storage first (default: True)

    Returns:
        Credentials dict or None if not found
    """
    # Check current directory first (SDK download location)
    credentials_paths = [
        Path.cwd() / ".aim" / "credentials.json",
        Path.home() / ".aim" / "credentials.json"
    ]

    for credentials_path in credentials_paths:
        # Try secure storage first
        if use_secure_storage and SECURE_STORAGE_AVAILABLE:
            try:
                storage = SecureCredentialStorage(str(credentials_path))
                credentials = storage.load_credentials()
                if credentials:
                    return credentials
            except Exception as e:
                # Continue to next path
                pass

        # Fall back to plaintext
        if credentials_path.exists():
            try:
                with open(credentials_path, 'r') as f:
                    return json.load(f)
            except Exception as e:
                # Continue to next path
                pass

    # No credentials found in any location
    return None
