"""
OAuth token management for AIM SDK.

Handles automatic token refresh using embedded refresh tokens.
"""

import os
import json
import time
from pathlib import Path
from typing import Optional, Dict, Any
import requests

from .exceptions import AuthenticationError


class OAuthTokenManager:
    """Manages OAuth tokens with automatic refresh."""

    def __init__(self, credentials_path: Optional[str] = None):
        """
        Initialize OAuth token manager.

        Args:
            credentials_path: Path to credentials.json file (default: ~/.aim/credentials.json)
        """
        if credentials_path:
            self.credentials_path = Path(credentials_path)
        else:
            # Default location for SDK-embedded credentials
            self.credentials_path = Path.home() / ".aim" / "credentials.json"

        self.credentials: Optional[Dict[str, Any]] = None
        self.access_token: Optional[str] = None
        self.access_token_expiry: Optional[float] = None

        # Load credentials if they exist
        if self.credentials_path.exists():
            self.load_credentials()

    def load_credentials(self) -> bool:
        """
        Load credentials from file.

        Returns:
            True if credentials were loaded successfully
        """
        try:
            with open(self.credentials_path, 'r') as f:
                self.credentials = json.load(f)
            return True
        except Exception as e:
            print(f"Warning: Failed to load credentials: {e}")
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

        Returns:
            New access token or None if refresh failed
        """
        if not self.credentials or 'refresh_token' not in self.credentials:
            return None

        aim_url = self.credentials.get('aim_url', 'http://localhost:8080')
        refresh_token = self.credentials['refresh_token']

        try:
            # Call token refresh endpoint
            response = requests.post(
                f"{aim_url.rstrip('/')}/api/v1/auth/refresh",
                json={"refresh_token": refresh_token},
                timeout=10
            )

            if response.status_code != 200:
                print(f"Warning: Token refresh failed with status {response.status_code}")
                return None

            data = response.json()
            self.access_token = data.get('access_token')

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
                    print(f"Warning: Failed to decode token expiry: {e}")
                    # Assume 1 hour expiry if we can't decode
                    self.access_token_expiry = time.time() + 3600

            return self.access_token

        except Exception as e:
            print(f"Warning: Token refresh failed: {e}")
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


def load_sdk_credentials() -> Optional[Dict[str, Any]]:
    """
    Load credentials from SDK-embedded location.

    This function looks for credentials in the default location
    where the SDK download process places them.

    Returns:
        Credentials dict or None if not found
    """
    credentials_path = Path.home() / ".aim" / "credentials.json"

    if not credentials_path.exists():
        return None

    try:
        with open(credentials_path, 'r') as f:
            return json.load(f)
    except Exception as e:
        print(f"Warning: Failed to load SDK credentials: {e}")
        return None
