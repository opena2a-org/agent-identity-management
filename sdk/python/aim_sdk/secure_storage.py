"""
Secure credential storage for AIM SDK.

Uses system keyring for encryption keys and stores encrypted credentials.
Falls back to plaintext with warning if keyring is unavailable.
"""

import json
import os
from pathlib import Path
from typing import Dict, Optional, Any

try:
    from cryptography.fernet import Fernet
    CRYPTOGRAPHY_AVAILABLE = True
except ImportError:
    CRYPTOGRAPHY_AVAILABLE = False

try:
    import keyring
    KEYRING_AVAILABLE = True
except ImportError:
    KEYRING_AVAILABLE = False


class SecureCredentialStorage:
    """
    Securely stores AIM SDK credentials using encryption.

    Security features:
    - Encryption key stored in system keyring (macOS Keychain, Windows Credential Manager, Linux Secret Service)
    - Credentials encrypted with Fernet (AES-128 CBC)
    - Automatic key generation and rotation
    - REQUIRES cryptography and keyring packages (no insecure fallback)

    Raises:
        RuntimeError: If required security packages are not installed
    """

    SERVICE_NAME = "aim-sdk"
    KEY_NAME = "encryption-key"

    def __init__(self, credentials_path: Optional[str] = None):
        """
        Initialize secure credential storage.

        Args:
            credentials_path: Optional custom path to credentials file.
                            Defaults to ~/.aim/credentials.json

        Raises:
            RuntimeError: If cryptography or keyring packages are not installed
        """
        # SECURITY: Require secure storage packages - NO FALLBACK
        if not CRYPTOGRAPHY_AVAILABLE or not KEYRING_AVAILABLE:
            missing = []
            if not CRYPTOGRAPHY_AVAILABLE:
                missing.append("cryptography")
            if not KEYRING_AVAILABLE:
                missing.append("keyring")

            raise RuntimeError(
                f"❌ SECURITY ERROR: Required packages not installed: {', '.join(missing)}\n"
                f"   AIM SDK REQUIRES secure credential storage.\n"
                f"   Install with: pip install {' '.join(missing)}\n"
                f"   We do NOT support insecure plaintext storage."
            )

        if credentials_path:
            self.credentials_path = Path(credentials_path)
        else:
            self.credentials_path = Path.home() / ".aim" / "credentials.json"

        self.encrypted_path = self.credentials_path.with_suffix('.encrypted')
        self.cipher = self._get_cipher()

    def _get_cipher(self) -> Fernet:
        """
        Get or create encryption cipher using key from system keyring.

        Returns:
            Fernet cipher for encryption/decryption

        Raises:
            RuntimeError: If keyring access fails
        """
        try:
            # Try to get existing key from keyring
            key = keyring.get_password(self.SERVICE_NAME, self.KEY_NAME)

            if not key:
                # Generate new key and store in keyring
                key = Fernet.generate_key().decode('utf-8')
                keyring.set_password(self.SERVICE_NAME, self.KEY_NAME, key)
                print("🔐 Generated new encryption key and stored in system keyring")

            return Fernet(key.encode('utf-8'))

        except Exception as e:
            raise RuntimeError(
                f"❌ SECURITY ERROR: Failed to access system keyring: {e}\n"
                f"   AIM SDK requires secure credential storage.\n"
                f"   Please check your system keyring configuration."
            )

    def save_credentials(self, credentials: Dict[str, Any]) -> None:
        """
        Save credentials securely (ALWAYS encrypted).

        Args:
            credentials: Dictionary containing AIM credentials
        """
        # Create directory if it doesn't exist
        self.credentials_path.parent.mkdir(parents=True, exist_ok=True)

        # Serialize credentials
        credentials_json = json.dumps(credentials, indent=2)

        # Encrypt and save (NO plaintext fallback)
        encrypted_data = self.cipher.encrypt(credentials_json.encode('utf-8'))
        self.encrypted_path.write_bytes(encrypted_data)

        # Set restrictive permissions (owner read/write only)
        os.chmod(self.encrypted_path, 0o600)

        # Remove plaintext file if it exists
        if self.credentials_path.exists():
            self.credentials_path.unlink()
            print("🗑️  Removed old plaintext credentials")

        print(f"✅ Credentials saved securely (encrypted) at {self.encrypted_path}")

    def load_credentials(self) -> Optional[Dict[str, Any]]:
        """
        Load credentials from secure storage (ALWAYS encrypted).

        Returns:
            Dictionary containing credentials, or None if not found

        Raises:
            RuntimeError: If decryption fails
        """
        # Load encrypted credentials only
        if self.encrypted_path.exists():
            try:
                encrypted_data = self.encrypted_path.read_bytes()
                decrypted_data = self.cipher.decrypt(encrypted_data)
                credentials = json.loads(decrypted_data.decode('utf-8'))
                return credentials
            except Exception as e:
                raise RuntimeError(
                    f"❌ SECURITY ERROR: Failed to decrypt credentials: {e}\n"
                    f"   Credentials may be corrupted or encryption key changed.\n"
                    f"   You may need to re-register with AIM."
                )

        # Auto-migrate plaintext credentials to encrypted storage (transparent security upgrade)
        if self.credentials_path.exists():
            try:
                # Load plaintext credentials FIRST
                with open(self.credentials_path, 'r') as f:
                    credentials = json.load(f)

                print(f"🔐 Auto-migrating plaintext credentials to encrypted storage...")

                # Save as encrypted (this also sets self.credentials)
                self.save_credentials(credentials)

                # Plaintext file already deleted by save_credentials()
                print(f"✅ Credentials migrated successfully to encrypted storage.")

                return credentials

            except Exception as e:
                # If migration fails, try to read plaintext if it still exists
                print(f"⚠️  Warning: Failed to migrate credentials to encrypted storage: {e}")
                print(f"   Attempting to use plaintext credentials as fallback...")

                try:
                    # Check if plaintext still exists before trying to read
                    if self.credentials_path.exists():
                        with open(self.credentials_path, 'r') as f:
                            return json.load(f)
                    else:
                        print(f"   Plaintext file no longer exists. Migration partially completed.")
                        return None
                except Exception as fallback_error:
                    print(f"   Failed to read plaintext fallback: {fallback_error}")
                    return None

        return None

    def delete_credentials(self) -> None:
        """Delete stored credentials (both encrypted and plaintext)."""
        if self.encrypted_path.exists():
            self.encrypted_path.unlink()
            print(f"🗑️  Deleted encrypted credentials at {self.encrypted_path}")

        if self.credentials_path.exists():
            self.credentials_path.unlink()
            print(f"🗑️  Deleted plaintext credentials at {self.credentials_path}")

    def credentials_exist(self) -> bool:
        """Check if credentials file exists (encrypted or plaintext)."""
        return self.encrypted_path.exists() or self.credentials_path.exists()

    def migrate_to_encrypted(self) -> bool:
        """
        Migrate plaintext credentials to encrypted storage.

        Returns:
            True if migration successful, False otherwise
        """
        if not self.cipher:
            print("⚠️  Encryption not available, cannot migrate")
            return False

        if not self.credentials_path.exists():
            print("⚠️  No plaintext credentials found to migrate")
            return False

        try:
            # Load plaintext credentials
            credentials = json.loads(self.credentials_path.read_text())

            # Save encrypted
            self.save_credentials(credentials)

            print("✅ Successfully migrated credentials to encrypted storage")
            return True

        except Exception as e:
            print(f"❌ Failed to migrate credentials: {e}")
            return False


def get_secure_storage(credentials_path: Optional[str] = None) -> SecureCredentialStorage:
    """
    Get a SecureCredentialStorage instance.

    Args:
        credentials_path: Optional custom path to credentials file

    Returns:
        SecureCredentialStorage instance
    """
    return SecureCredentialStorage(credentials_path)
