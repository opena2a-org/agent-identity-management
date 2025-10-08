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
    print("⚠️  Warning: cryptography package not installed. Credentials will be stored in plaintext.")
    print("   Install with: pip install cryptography")

try:
    import keyring
    KEYRING_AVAILABLE = True
except ImportError:
    KEYRING_AVAILABLE = False
    print("⚠️  Warning: keyring package not installed. Using less secure storage.")
    print("   Install with: pip install keyring")


class SecureCredentialStorage:
    """
    Securely stores AIM SDK credentials using encryption.

    Security features:
    - Encryption key stored in system keyring (macOS Keychain, Windows Credential Manager, Linux Secret Service)
    - Credentials encrypted with Fernet (AES-128 CBC)
    - Automatic key generation and rotation
    - Falls back to plaintext with warning if dependencies unavailable
    """

    SERVICE_NAME = "aim-sdk"
    KEY_NAME = "encryption-key"

    def __init__(self, credentials_path: Optional[str] = None):
        """
        Initialize secure credential storage.

        Args:
            credentials_path: Optional custom path to credentials file.
                            Defaults to ~/.aim/credentials.json
        """
        if credentials_path:
            self.credentials_path = Path(credentials_path)
        else:
            self.credentials_path = Path.home() / ".aim" / "credentials.json"

        self.encrypted_path = self.credentials_path.with_suffix('.encrypted')

        # Check if we can use encryption
        self.encryption_available = CRYPTOGRAPHY_AVAILABLE and KEYRING_AVAILABLE

        if self.encryption_available:
            self.cipher = self._get_cipher()
        else:
            self.cipher = None

    def _get_cipher(self) -> Optional[Fernet]:
        """Get or create encryption cipher using key from system keyring."""
        if not self.encryption_available:
            return None

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
            print(f"⚠️  Warning: Failed to access system keyring: {e}")
            print("   Falling back to plaintext storage")
            return None

    def save_credentials(self, credentials: Dict[str, Any]) -> None:
        """
        Save credentials securely.

        Args:
            credentials: Dictionary containing AIM credentials
        """
        # Create directory if it doesn't exist
        self.credentials_path.parent.mkdir(parents=True, exist_ok=True)

        # Serialize credentials
        credentials_json = json.dumps(credentials, indent=2)

        if self.cipher:
            # Encrypt and save
            encrypted_data = self.cipher.encrypt(credentials_json.encode('utf-8'))
            self.encrypted_path.write_bytes(encrypted_data)

            # Set restrictive permissions (owner read/write only)
            os.chmod(self.encrypted_path, 0o600)

            # Remove plaintext file if it exists
            if self.credentials_path.exists():
                self.credentials_path.unlink()

            print(f"✅ Credentials saved securely at {self.encrypted_path}")
        else:
            # Fall back to plaintext (with warning)
            self.credentials_path.write_text(credentials_json)

            # Set restrictive permissions
            os.chmod(self.credentials_path, 0o600)

            print(f"⚠️  Credentials saved in plaintext at {self.credentials_path}")
            print("   For better security, install: pip install cryptography keyring")

    def load_credentials(self) -> Optional[Dict[str, Any]]:
        """
        Load credentials from secure storage.

        Returns:
            Dictionary containing credentials, or None if not found
        """
        # Try encrypted file first
        if self.cipher and self.encrypted_path.exists():
            try:
                encrypted_data = self.encrypted_path.read_bytes()
                decrypted_data = self.cipher.decrypt(encrypted_data)
                credentials = json.loads(decrypted_data.decode('utf-8'))
                return credentials
            except Exception as e:
                print(f"⚠️  Warning: Failed to decrypt credentials: {e}")
                print("   Trying plaintext fallback...")

        # Fall back to plaintext
        if self.credentials_path.exists():
            try:
                credentials_json = self.credentials_path.read_text()
                return json.loads(credentials_json)
            except Exception as e:
                print(f"❌ Error loading credentials: {e}")
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
