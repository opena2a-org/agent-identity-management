"""
SDK token management for AIM SDK.

This module handles SDK authentication tokens (refresh/access tokens) for the SDK download mode.
Note: This is NOT OAuth provider authentication (Google/Microsoft/Okta) - that was removed.
This manages the JWT tokens embedded in downloaded SDKs for zero-config authentication.

Handles automatic token refresh with token rotation and secure storage.

Security Note:
- JWT tokens are parsed using PyJWT library for proper structure validation
- Signature verification is NOT performed locally (server uses HS256 with secret key)
- Full token validation occurs server-side when tokens are used for API calls
- Local parsing is only for reading claims (expiry, token ID) for housekeeping
"""

import json
import os
import time
from pathlib import Path
from typing import Optional, Dict, Any

import jwt  # PyJWT for proper JWT parsing
import requests

from .exceptions import AuthenticationError
from .credentials import (
    load_sdk_credentials as _load_sdk_credentials_from_module,
    save_sdk_credentials as _save_sdk_credentials_to_module,
    get_sdk_credentials_path,
    print_sdk_credentials_not_found_error,
    print_token_expired_error,
)
from .security_logging import security_logger, AuthnEventType, CredEventType

# Try to import secure storage (optional dependency)
try:
    from .secure_storage import SecureCredentialStorage
    SECURE_STORAGE_AVAILABLE = True
    SECURE_STORAGE_WARNING = None
except ImportError as e:
    SECURE_STORAGE_AVAILABLE = False
    SECURE_STORAGE_WARNING = (
        "⚠️  SECURITY WARNING: Secure storage packages not installed.\n"
        "   Credentials will be stored in PLAINTEXT without encryption.\n"
        "   For encrypted storage, install: pip install cryptography keyring\n"
    )


# Expected JWT issuers from AIM server
VALID_JWT_ISSUERS = {"agent-identity-management", "agent-identity-management-sdk"}


def decode_jwt_claims(token: str) -> Optional[Dict[str, Any]]:
    """
    Decode JWT token claims using PyJWT library.

    Security Note:
    - Signature verification is NOT performed (server uses HS256 with secret key)
    - Full token validation occurs server-side when tokens are used for API calls
    - This function only reads claims for housekeeping (expiry, token ID)
    - Basic structure validation is performed by PyJWT

    Args:
        token: JWT token string

    Returns:
        Dictionary of claims or None if token is malformed
    """
    try:
        # Decode without signature verification (we don't have the secret)
        # PyJWT still validates JWT structure (3 parts, valid base64, valid JSON)
        claims = jwt.decode(
            token,
            options={
                "verify_signature": False,  # Server uses HS256, we don't have secret
                "verify_exp": False,  # We check expiry manually
                "verify_aud": False,
                "verify_iss": False,  # We check issuer separately for logging
            }
        )

        # Basic sanity check: verify issuer looks like it came from AIM
        issuer = claims.get("iss", "")
        if issuer and issuer not in VALID_JWT_ISSUERS:
            # Log but don't fail - could be a new issuer we don't know about
            import warnings
            warnings.warn(
                f"JWT has unexpected issuer '{issuer}'. Expected one of: {VALID_JWT_ISSUERS}",
                UserWarning,
                stacklevel=2
            )

        return claims

    except jwt.exceptions.DecodeError as e:
        # Token is malformed (not a valid JWT)
        print(f"⚠️  Warning: Failed to decode JWT: {e}")
        return None
    except Exception as e:
        print(f"⚠️  Warning: Unexpected error decoding JWT: {e}")
        return None


class OAuthTokenManager:
    """
    Manages OAuth tokens with automatic refresh and token rotation.

    Security features:
    - Automatic token refresh when expired
    - Token rotation: new refresh token on each refresh
    - Secure encrypted storage (if cryptography + keyring installed)
    - Automatic credential updates when tokens rotate
    """

    def __init__(
        self,
        credentials_path: Optional[str] = None,
        use_secure_storage: bool = True,
        allow_plaintext_fallback: bool = True,
    ):
        """
        Initialize OAuth token manager with intelligent credential discovery.

        Args:
            credentials_path: Path to credentials.json file (default: auto-discover via credentials module)
            use_secure_storage: Use encrypted storage if available (default: True)
            allow_plaintext_fallback: If True (default), fall back to plaintext with warning when
                                     secure storage is unavailable. If False, raise an error instead.
        """
        # Use the centralized credentials module for path discovery
        if credentials_path:
            self.credentials_path = Path(credentials_path)
        else:
            self.credentials_path = get_sdk_credentials_path()

        self.credentials: Optional[Dict[str, Any]] = None
        self.access_token: Optional[str] = None
        self.access_token_expiry: Optional[float] = None

        # Audit #12: the encrypted shadow file (~/.aim/sdk_credentials.encrypted)
        # used to be written alongside the JSON file by SecureCredentialStorage.
        # Every other code path read the JSON only, so the encrypted file was a
        # write-only secondary store that could drift from the JSON and that
        # operators had to delete manually to recover from corruption. The JSON
        # at SDK_CREDENTIALS_FILE (mode 0600) is now the single source of truth.
        # `use_secure_storage` and `allow_plaintext_fallback` are kept for ABI
        # stability and ignored; the storage layer is JSON in all configurations.
        self.use_secure_storage = False
        self.secure_storage = None

        # One-time migration: if the deprecated encrypted shadow file exists,
        # decrypt it into the JSON store and delete the orphan. Subsequent
        # runs find no encrypted file and skip the migration.
        self._migrate_encrypted_shadow_if_present()

        # Load credentials if they exist
        if self._credentials_exist():
            self.load_credentials()

    def _migrate_encrypted_shadow_if_present(self) -> None:
        """Migrate the deprecated encrypted shadow file (audit #12) to JSON.

        Strategy:
          - If no encrypted file exists, no-op.
          - If the secure-storage libs are present, attempt to decrypt and
            save the contents into the JSON store via the centralized module.
          - If decryption fails (missing keyring entry, corrupted file), the
            JSON file is authoritative — log and delete the orphan.
          - Always delete the encrypted file at the end so subsequent runs
            skip this branch entirely.
        """
        encrypted_path = self.credentials_path.with_suffix(".encrypted")
        if not encrypted_path.exists():
            return

        migrated = False
        decrypt_failed = False
        if SECURE_STORAGE_AVAILABLE:
            try:
                legacy_storage = SecureCredentialStorage(str(self.credentials_path))
                creds = legacy_storage.load_credentials()
                if creds:
                    # Persist BEFORE deleting the encrypted file. If the JSON
                    # write fails, the encrypted file is left in place so the
                    # next run can retry — we never delete the only copy of
                    # credentials we still need.
                    _save_sdk_credentials_to_module(creds)
                    print(
                        f"🔄 Migrated credentials from encrypted shadow file to JSON "
                        f"single source of truth at {self.credentials_path}."
                    )
                    migrated = True
            except Exception as e:
                decrypt_failed = True
                print(
                    f"⚠️  Could not migrate legacy encrypted credentials at "
                    f"{encrypted_path} ({e}); leaving the file in place."
                )

        # Only remove the encrypted file when we either migrated successfully
        # or we know we cannot decrypt it at all (SECURE_STORAGE_AVAILABLE is
        # False — the keyring + cryptography libs are gone). If decryption was
        # attempted and failed (decrypt_failed=True), preserve the file so the
        # operator can recover manually.
        should_remove = migrated or (not SECURE_STORAGE_AVAILABLE and not decrypt_failed)
        if should_remove:
            try:
                encrypted_path.unlink()
                if not migrated:
                    print(
                        f"🧹 Removed deprecated encrypted credentials file at "
                        f"{encrypted_path} (single-source-of-truth migration, audit #12)."
                    )
            except Exception:
                pass

    def _credentials_exist(self) -> bool:
        """Check if credentials exist on disk (JSON single source of truth)."""
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
            creds = _load_sdk_credentials_from_module()
            if creds:
                self.credentials = creds
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
            _save_sdk_credentials_to_module(credentials)
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
                            json={"oldRefreshToken": refresh_token},
                            timeout=10
                        )

                        if recovery_response.status_code == 200:
                            recovery_data = recovery_response.json()
                            self.access_token = recovery_data.get('accessToken')
                            new_refresh_token = recovery_data.get('refreshToken')

                            if new_refresh_token:
                                # Save recovered credentials
                                self.credentials['refreshToken'] = new_refresh_token

                                # Update sdkTokenId using PyJWT
                                token_payload = decode_jwt_claims(new_refresh_token)
                                if token_payload:
                                    new_token_id = token_payload.get('jti')
                                    if new_token_id:
                                        self.credentials['sdkTokenId'] = new_token_id

                                self.save_credentials(self.credentials)
                                print("✅ Token recovered automatically! SDK credentials updated.")
                                print("💡 No need to re-download the SDK - everything just works!")

                                # Decode new access token expiry using PyJWT
                                payload = decode_jwt_claims(self.access_token)
                                if payload:
                                    self.access_token_expiry = payload.get('exp')
                                else:
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

                # Also update sdkTokenId if present in the new token (using PyJWT)
                new_token_id_full = None
                token_payload = decode_jwt_claims(new_refresh_token)
                if token_payload:
                    new_token_id_full = token_payload.get('jti')
                    if new_token_id_full:
                        self.credentials['sdkTokenId'] = new_token_id_full

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
                # Audit #12: explicit that the prior refresh token is now revoked
                # server-side so the user understands why an out-of-band copy
                # (other machine, another venv, bundled SDK) would stop working.
                print(
                    "🔄 Token rotated successfully \u2014 old refresh token revoked "
                    f"(new id: {new_token_id_full[:8] + '...' if new_token_id_full else 'unknown'})"
                )

            # Decode token to get expiry using PyJWT
            if self.access_token:
                payload = decode_jwt_claims(self.access_token)
                if payload:
                    self.access_token_expiry = payload.get('exp')
                else:
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
        # Support both camelCase and snake_case for backward compatibility
        has_refresh_token = 'refreshToken' in self.credentials or 'refresh_token' in self.credentials if self.credentials else False
        if not self.credentials or not has_refresh_token:
            return False

        aim_url = self.credentials.get('aimUrl') or self.credentials.get('aim_url', 'http://localhost:8080')
        refresh_token = self.credentials.get('refreshToken') or self.credentials.get('refresh_token')

        try:
            # Call token revocation endpoint (if implemented)
            response = requests.post(
                f"{aim_url.rstrip('/')}/api/v1/auth/revoke",
                json={"refreshToken": refresh_token},
                timeout=10
            )

            # Delete local credentials regardless of server response
            if self.credentials_path.exists():
                self.credentials_path.unlink()

            self.credentials = None
            self.access_token = None
            self.access_token_expiry = None

            print("✅ Token revoked and credentials deleted")
            return True

        except Exception as e:
            print(f"⚠️  Warning: Token revocation failed: {e}")
            # Still delete local credentials for safety
            if self.credentials_path.exists():
                self.credentials_path.unlink()

            return False


def load_sdk_credentials(use_secure_storage: bool = True) -> Optional[Dict[str, Any]]:
    """Load SDK credentials from the JSON single source of truth.

    Audit #12: the encrypted shadow file was removed as a credential store;
    OAuthTokenManager performs a one-time migration from any leftover
    `.encrypted` file. The `use_secure_storage` parameter is preserved for
    ABI stability but ignored.

    Returns SDK OAuth credentials (refreshToken, sdkTokenId) or None.
    For agent credentials, use load_agent_credentials() from the
    credentials module.
    """
    return _load_sdk_credentials_from_module()
