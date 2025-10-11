"""
OAuth token management for AIM SDK.

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
        Initialize OAuth token manager.

        Args:
            credentials_path: Path to credentials.json file (default: ~/.aim/credentials.json)
            use_secure_storage: Use encrypted storage if available (default: True)
        """
        if credentials_path:
            self.credentials_path = Path(credentials_path)
        else:
            # Default location for SDK-embedded credentials
            self.credentials_path = Path.home() / ".aim" / "credentials.json"

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
            response = requests.post(
                f"{aim_url.rstrip('/')}/api/v1/auth/refresh",
                json={"refresh_token": refresh_token},
                timeout=10
            )

            if response.status_code != 200:
                print(f"⚠️  Warning: Token refresh failed with status {response.status_code}")
                return None

            data = response.json()
            self.access_token = data.get('access_token')

            # Check if server returned new refresh token (token rotation)
            new_refresh_token = data.get('refresh_token')
            if new_refresh_token and new_refresh_token != refresh_token:
                # Token rotation: save new refresh token
                self.credentials['refresh_token'] = new_refresh_token
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

    This function looks for credentials in the default location
    where the SDK download process places them.

    Args:
        use_secure_storage: Try encrypted storage first (default: True)

    Returns:
        Credentials dict or None if not found
    """
    credentials_path = Path.home() / ".aim" / "credentials.json"

    # Try secure storage first
    if use_secure_storage and SECURE_STORAGE_AVAILABLE:
        try:
            storage = SecureCredentialStorage(str(credentials_path))
            credentials = storage.load_credentials()
            if credentials:
                return credentials
        except Exception as e:
            print(f"⚠️  Warning: Failed to load from secure storage: {e}")

    # Fall back to plaintext
    if not credentials_path.exists():
        return None

    try:
        with open(credentials_path, 'r') as f:
            return json.load(f)
    except Exception as e:
        print(f"⚠️  Warning: Failed to load SDK credentials: {e}")
        return None
