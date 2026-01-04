"""
Ed25519 signing implementation using PyNaCl.
"""

import base64
from nacl.signing import SigningKey, VerifyKey
from nacl.encoding import RawEncoder
from nacl.exceptions import BadSignatureError

from .signer import Algorithm, Signer, Verifier, KeyPair


class Ed25519Signer(Signer):
    """Ed25519 signer implementation using PyNaCl."""

    def __init__(self, signing_key: SigningKey):
        """
        Initialize with a PyNaCl SigningKey.

        Args:
            signing_key: PyNaCl SigningKey instance
        """
        self._signing_key = signing_key
        self._verify_key = signing_key.verify_key

    @classmethod
    def generate(cls) -> "Ed25519Signer":
        """
        Generate a new Ed25519 key pair.

        Returns:
            New Ed25519Signer with generated keys
        """
        signing_key = SigningKey.generate()
        return cls(signing_key)

    @classmethod
    def from_keys(cls, public_key_b64: str, private_key_b64: str) -> "Ed25519Signer":
        """
        Create signer from base64-encoded keys.

        Args:
            public_key_b64: Base64-encoded public key (32 bytes)
            private_key_b64: Base64-encoded private key (32 or 64 bytes)

        Returns:
            Ed25519Signer instance

        Note:
            The private key may be 32 bytes (seed only) or 64 bytes
            (seed + public key, as used by Go's Ed25519).
        """
        private_key_bytes = base64.b64decode(private_key_b64)

        # Handle both 32-byte seed and 64-byte full private key
        if len(private_key_bytes) == 64:
            # Go format: seed (32) + public key (32)
            seed = private_key_bytes[:32]
        elif len(private_key_bytes) == 32:
            seed = private_key_bytes
        else:
            raise ValueError(
                f"Invalid private key size: expected 32 or 64, got {len(private_key_bytes)}"
            )

        signing_key = SigningKey(seed)
        return cls(signing_key)

    @classmethod
    def from_seed(cls, seed: bytes) -> "Ed25519Signer":
        """
        Create signer from a 32-byte seed.

        Args:
            seed: 32-byte seed

        Returns:
            Ed25519Signer instance
        """
        if len(seed) != 32:
            raise ValueError(f"Seed must be 32 bytes, got {len(seed)}")
        signing_key = SigningKey(seed)
        return cls(signing_key)

    @property
    def algorithm(self) -> Algorithm:
        return Algorithm.ED25519

    def public_key(self) -> str:
        """Return base64-encoded public key."""
        return base64.b64encode(bytes(self._verify_key)).decode("utf-8")

    def public_key_bytes(self) -> bytes:
        """Return raw public key bytes."""
        return bytes(self._verify_key)

    def private_key(self) -> str:
        """
        Return base64-encoded private key (64-byte Go format).

        Format: seed (32 bytes) + public key (32 bytes)
        """
        # Create 64-byte format for Go compatibility
        seed = bytes(self._signing_key)[:32]
        public = bytes(self._verify_key)
        return base64.b64encode(seed + public).decode("utf-8")

    def key_pair(self) -> KeyPair:
        """Return the key pair for storage."""
        return KeyPair(
            algorithm=Algorithm.ED25519,
            public_key=self.public_key(),
            private_key=self.private_key(),
        )

    def sign(self, message: bytes) -> str:
        """Sign message and return base64-encoded signature."""
        sig = self.sign_bytes(message)
        return base64.b64encode(sig).decode("utf-8")

    def sign_bytes(self, message: bytes) -> bytes:
        """Sign message and return raw signature bytes (64 bytes)."""
        signed = self._signing_key.sign(message, encoder=RawEncoder)
        return signed.signature

    def verify(self, message: bytes, signature: str) -> bool:
        """Verify base64-encoded signature."""
        try:
            sig_bytes = base64.b64decode(signature)
            return self.verify_bytes(message, sig_bytes)
        except Exception:
            return False

    def verify_bytes(self, message: bytes, signature: bytes) -> bool:
        """Verify raw signature bytes."""
        if len(signature) != 64:
            return False
        try:
            self._verify_key.verify(message, signature)
            return True
        except BadSignatureError:
            return False


class Ed25519Verifier(Verifier):
    """Ed25519 verifier (public key only, no signing capability)."""

    def __init__(self, verify_key: VerifyKey):
        """
        Initialize with a PyNaCl VerifyKey.

        Args:
            verify_key: PyNaCl VerifyKey instance
        """
        self._verify_key = verify_key

    @classmethod
    def from_public_key(cls, public_key_b64: str) -> "Ed25519Verifier":
        """
        Create verifier from base64-encoded public key.

        Args:
            public_key_b64: Base64-encoded 32-byte public key

        Returns:
            Ed25519Verifier instance
        """
        public_key_bytes = base64.b64decode(public_key_b64)
        if len(public_key_bytes) != 32:
            raise ValueError(
                f"Invalid public key size: expected 32, got {len(public_key_bytes)}"
            )
        verify_key = VerifyKey(public_key_bytes)
        return cls(verify_key)

    @property
    def algorithm(self) -> Algorithm:
        return Algorithm.ED25519

    def verify(self, message: bytes, signature: str) -> bool:
        """Verify base64-encoded signature."""
        try:
            sig_bytes = base64.b64decode(signature)
            return self.verify_bytes(message, sig_bytes)
        except Exception:
            return False

    def verify_bytes(self, message: bytes, signature: bytes) -> bool:
        """Verify raw signature bytes."""
        if len(signature) != 64:
            return False
        try:
            self._verify_key.verify(message, signature)
            return True
        except BadSignatureError:
            return False
