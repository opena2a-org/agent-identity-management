"""
SDK token management for AIM SDK.

This module handles SDK authentication tokens (refresh/access tokens) for the SDK download mode.
Note: This is NOT OAuth provider authentication (Google/Microsoft/Okta) - that was removed.
This manages the JWT tokens embedded in downloaded SDKs for zero-config authentication.

Handles automatic token refresh with token rotation and secure storage.
"""

import base64
import json
import os
import time
from pathlib import Path
from typing import Optional, Dict, Any

import requests

from .exceptions import AuthenticationError
from .credentials import (
    load_sdk_credentials as _load_sdk_credentials_from_module,
    save_sdk_credentials as _save_sdk_credentials_to_module,
    get_sdk_credentials_path,
    print_sdk_credentials_not_found_error,
    print_wrong_credential_type_error,
    print_token_expired_error,
    detect_credential_type,
    CredentialType,
    SDK_CREDENTIALS_FILE,
)
from .security_logging import security_logger, AuthnEventType, CredEventType

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
            credentials_path: Path to credentials.json file (default: auto-discover via credentials module)
            use_secure_storage: Use encrypted storage if available (default: True)
        """
        # Use the centralized credentials module for path discovery
        if credentials_path:
            self.credentials_path = Path(credentials_path)
        else:
            self.credentials_path = get_sdk_credentials_path()

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
        Load SDK credentials with proper type validation.

        Uses the centralized credentials module which:
        - Checks the correct SDK credentials path
        - Migrates from legacy locations if needed
        - Validates credential type (SDK vs Agent)

        Returns:
            True if SDK credentials were loaded successfully
        """
        try:
            # Try secure storage first
            if self.secure_storage:
                creds = self.secure_storage.load_credentials()
                if creds:
                    # Validate it's SDK credentials, not agent credentials
                    cred_type = detect_credential_type(creds)
                    if cred_type == CredentialType.SDK_OAUTH:
                        self.credentials = creds
                        return True
                    elif cred_type == CredentialType.AGENT:
                        # Wrong credential type in secure storage - this shouldn't happen
                        # but handle it gracefully
                        print_wrong_credential_type_error(cred_type)
                        return False

            # Use centralized credentials module for discovery and migration
            creds = _load_sdk_credentials_from_module()
            if creds:
                self.credentials = creds
                # Migrate to secure storage if available
                if self.secure_storage:
                    try:
                        self.secure_storage.save_credentials(creds)
                    except Exception:
                        pass  # Continue even if secure storage fails
                return True

            return False

        except Exception as e:
            print(f"⚠️  Warning: Failed to load credentials: {e}")
            return False

    def save_credentials(self, credentials: Dict[str, Any]) -> bool:
        """
        Save SDK credentials securely.

        Uses the centralized credentials module to ensure proper
        file location and schema versioning.

        Args:
            credentials: SDK credentials dictionary to save

        Returns:
            True if saved successfully
        """
        try:
            # Use centralized module for proper path and schema
            _save_sdk_credentials_to_module(credentials)

            # Also save to secure storage if available
            if self.secure_storage:
                try:
                    self.secure_storage.save_credentials(credentials)
                except Exception:
                    pass  # Continue even if secure storage fails

            self.credentials = credentials
            return True

        except Exception as e:
            print(f"⚠️  Warning: Failed to save credentials: {e}")
            return False

    def has_credentials(self) -> bool:
        """Check if credentials are available."""
        return self.credentials is not None

    def get_access_token(self, suppress_errors: bool = False) -> Optional[str]:
        """
        Get a valid access token, refreshing if necessary.

        Args:
            suppress_errors: If True, suppress error messages when refresh fails.
                           Use this when you have a fallback authentication mechanism.

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
        return self._refresh_token(suppress_errors=suppress_errors)

    def _refresh_token(self, suppress_errors: bool = False) -> Optional[str]:
        """
        Refresh access token using refresh token.

        Implements token rotation:
        - Server returns new access_token AND new refresh_token
        - Old refresh token is invalidated
        - New refresh token is saved to credentials

        Args:
            suppress_errors: If True, suppress error messages when refresh fails.
                           Use this when you have a fallback authentication mechanism.

        Returns:
            New access token or None if refresh failed
        """
        # Support both camelCase and snake_case for backward compatibility
        has_refresh_token = 'refreshToken' in self.credentials or 'refresh_token' in self.credentials
        if not self.credentials or not has_refresh_token:
            security_logger.log_authentication(
                AuthnEventType.TOKEN_REFRESH_FAILED,
                success=False,
                error="No refresh token available"
            )
            return None

        aim_url = self.credentials.get('aimUrl') or self.credentials.get('aim_url', 'http://localhost:8080')
        refresh_token = self.credentials.get('refreshToken') or self.credentials.get('refresh_token')
        token_id = self.credentials.get('sdkTokenId', '')[:8] + "..." if self.credentials.get('sdkTokenId') else None

        try:
            # Call token refresh endpoint (with rotation support)
            refresh_url = f"{aim_url.rstrip('/')}/api/v1/auth/refresh"

            response = requests.post(
                refresh_url,
                json={"refreshToken": refresh_token},
                timeout=10
            )

            if response.status_code != 200:
                error_data = response.json() if response.headers.get('content-type', '').startswith('application/json') else {}
                error_msg = error_data.get('error', response.text)

                # Check if token was revoked/expired - try automatic recovery
                if 'revoked' in error_msg.lower() or 'invalid' in error_msg.lower():
                    security_logger.log_authentication(
                        AuthnEventType.TOKEN_REVOKED,
                        success=False,
                        details={"token_id": token_id, "attempting_recovery": True}
                    )
                    if not suppress_errors:
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
                            self.access_token = recovery_data.get('accessToken')
                            new_refresh_token = recovery_data.get('refreshToken')

                            if new_refresh_token:
                                # Save recovered credentials
                                self.credentials['refreshToken'] = new_refresh_token

                                # Update sdkTokenId
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
                                            self.credentials['sdkTokenId'] = new_token_id
                                except Exception:
                                    pass

                                self.save_credentials(self.credentials)
                                print("✅ Token recovered automatically! SDK credentials updated.")
                                print("💡 No need to re-download the SDK - everything just works!")

                                # Decode new access token expiry
                                try:
                                    payload_part = self.access_token.split('.')[1]
                                    padding = 4 - len(payload_part) % 4
                                    if padding != 4:
                                        payload_part += '=' * padding
                                    payload = json.loads(base64.b64decode(payload_part))
                                    self.access_token_expiry = payload.get('exp')
                                except Exception:
                                    self.access_token_expiry = time.time() + 3600

                                security_logger.log_authentication(
                                    AuthnEventType.TOKEN_RECOVERED,
                                    success=True,
                                    details={"token_id": token_id, "recovery": "automatic"}
                                )
                                return self.access_token

                    except Exception as recovery_error:
                        # Recovery failed - fall back to manual instructions
                        security_logger.log_authentication(
                            AuthnEventType.TOKEN_REFRESH_FAILED,
                            success=False,
                            error=f"Recovery failed: {recovery_error}",
                            details={"token_id": token_id}
                        )

                    # If recovery failed, show manual instructions using centralized error
                    # Only show if suppress_errors is False (i.e., no fallback auth available)
                    security_logger.log_authentication(
                        AuthnEventType.TOKEN_EXPIRED,
                        success=False,
                        error="Token expired and recovery failed",
                        details={"token_id": token_id}
                    )
                    if not suppress_errors:
                        print_token_expired_error(aim_url)
                else:
                    security_logger.log_authentication(
                        AuthnEventType.TOKEN_REFRESH_FAILED,
                        success=False,
                        error=error_msg,
                        details={"token_id": token_id, "http_status": response.status_code}
                    )
                    if not suppress_errors:
                        print(f"⚠️  Token refresh failed with status {response.status_code}: {error_msg}")

                return None

            data = response.json()
            self.access_token = data.get('accessToken')

            # Check if server returned new refresh token (token rotation)
            new_refresh_token = data.get('refreshToken')
            if new_refresh_token and new_refresh_token != refresh_token:
                # Token rotation: save new refresh token
                self.credentials['refreshToken'] = new_refresh_token

                # Also update sdkTokenId if present in the new token
                new_token_id_full = None
                try:
                    # Decode new refresh token to get JTI
                    token_parts = new_refresh_token.split('.')
                    if len(token_parts) == 3:
                        payload_part = token_parts[1]
                        padding = 4 - len(payload_part) % 4
                        if padding != 4:
                            payload_part += '=' * padding
                        token_payload = json.loads(base64.b64decode(payload_part))
                        new_token_id_full = token_payload.get('jti')
                        if new_token_id_full:
                            self.credentials['sdkTokenId'] = new_token_id_full
                except Exception:
                    pass  # Continue even if JTI extraction fails

                self.save_credentials(self.credentials)
                security_logger.log_credential_event(
                    CredEventType.CREDENTIAL_ROTATED,
                    credential_type="sdk_oauth",
                    success=True,
                    details={
                        "old_token_id": token_id,
                        "new_token_id": new_token_id_full[:8] + "..." if new_token_id_full else None
                    }
                )
                print("🔄 Token rotated successfully")

            # Decode token to get expiry (JWT format)
            if self.access_token:
                try:
                    # JWT tokens are base64url encoded: header.payload.signature
                    # Note: base64 here is for decoding JWT claims, NOT for security
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

            security_logger.log_authentication(
                AuthnEventType.TOKEN_REFRESH,
                success=True,
                details={"token_id": token_id}
            )
            return self.access_token

        except Exception as e:
            # token_id is defined earlier in the function, so it should always be available
            # Use getattr on locals() as a safer alternative to dir() check
            local_token_id = locals().get('token_id')
            security_logger.log_authentication(
                AuthnEventType.TOKEN_REFRESH_FAILED,
                success=False,
                error=str(e),
                details={"token_id": local_token_id}
            )
            if not suppress_errors:
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
                json={"refreshToken": refresh_token},
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
    Load SDK credentials with intelligent discovery and migration.

    This function uses the centralized credentials module which:
    1. Checks ~/.aim/sdk_credentials.json (new location)
    2. Migrates from ~/.aim/credentials.json if it contains SDK credentials
    3. Installs from SDK package if found (fresh download)

    Note: This only returns SDK OAuth credentials (refreshToken, sdkTokenId).
    For agent credentials, use load_agent_credentials() from the credentials module.

    Args:
        use_secure_storage: Try encrypted storage first (default: True)

    Returns:
        SDK credentials dict or None if not found
    """
    # Try secure storage first
    if use_secure_storage and SECURE_STORAGE_AVAILABLE:
        try:
            storage = SecureCredentialStorage(str(SDK_CREDENTIALS_FILE))
            credentials = storage.load_credentials()
            if credentials:
                cred_type = detect_credential_type(credentials)
                if cred_type == CredentialType.SDK_OAUTH:
                    return credentials
        except Exception:
            pass

    # Fall back to centralized credentials module
    return _load_sdk_credentials_from_module()
