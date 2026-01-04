"""
ML-DSA-65 (FIPS 204) signing implementation using liboqs.

ML-DSA-65 is the NIST-standardized post-quantum signature algorithm
designed to be secure against both classical and quantum computers.

Key sizes (FIPS 204):
- Public key: 1952 bytes
- Private key: 4032 bytes
- Signature: 3309 bytes

This implementation uses liboqs-python for real post-quantum cryptography.
If liboqs is not available, it falls back to a simulated implementation
for development purposes.
"""

import base64
import hashlib
import os
import warnings
from typing import Tuple, Optional

from .signer import Algorithm, Signer, Verifier, KeyPair

# ML-DSA-65 constants (from FIPS 204)
MLDSA_PUBLIC_KEY_SIZE = 1952
MLDSA_PRIVATE_KEY_SIZE = 4032
MLDSA_SIGNATURE_SIZE = 3309

# Try to import liboqs
_LIBOQS_AVAILABLE = False
_oqs = None

try:
    import oqs as _oqs
    # Verify ML-DSA-65 is available
    if "ML-DSA-65" in _oqs.get_enabled_sig_mechanisms():
        _LIBOQS_AVAILABLE = True
except ImportError:
    pass
except Exception as e:
    warnings.warn(f"liboqs available but ML-DSA-65 check failed: {e}")


def is_liboqs_available() -> bool:
    """Check if liboqs is available for real PQC operations."""
    return _LIBOQS_AVAILABLE


class MLDSASigner(Signer):
    """
    ML-DSA-65 signer implementation using liboqs.

    Uses real NIST-standardized ML-DSA-65 (FIPS 204) when liboqs is available.
    Falls back to deterministic simulation if liboqs is not installed.
    """

    def __init__(
        self,
        public_key: bytes,
        private_key: bytes,
        simulated: bool = False,
        _oqs_signer: Optional[object] = None,
    ):
        """
        Initialize ML-DSA signer.

        Args:
            public_key: Public key bytes (1952 bytes)
            private_key: Private key bytes (4032 bytes)
            simulated: Whether this is a simulated implementation
            _oqs_signer: Internal liboqs Signature object (for signing)
        """
        if len(public_key) != MLDSA_PUBLIC_KEY_SIZE:
            raise ValueError(
                f"Invalid public key size: expected {MLDSA_PUBLIC_KEY_SIZE}, "
                f"got {len(public_key)}"
            )
        if len(private_key) != MLDSA_PRIVATE_KEY_SIZE:
            raise ValueError(
                f"Invalid private key size: expected {MLDSA_PRIVATE_KEY_SIZE}, "
                f"got {len(private_key)}"
            )

        self._public_key = public_key
        self._private_key = private_key
        self._simulated = simulated
        self._oqs_signer = _oqs_signer

    @classmethod
    def generate(cls) -> "MLDSASigner":
        """
        Generate a new ML-DSA-65 key pair.

        Uses liboqs for real PQC if available, otherwise falls back to simulation.

        Returns:
            New MLDSASigner with generated keys
        """
        if _LIBOQS_AVAILABLE:
            return cls._generate_real()
        else:
            return cls._generate_simulated()

    @classmethod
    def _generate_real(cls) -> "MLDSASigner":
        """Generate real ML-DSA-65 keys using liboqs."""
        signer = _oqs.Signature("ML-DSA-65")
        public_key = signer.generate_keypair()
        private_key = signer.export_secret_key()

        return cls(
            public_key=public_key,
            private_key=private_key,
            simulated=False,
            _oqs_signer=signer,
        )

    @classmethod
    def _generate_simulated(cls) -> "MLDSASigner":
        """Generate simulated ML-DSA-65 keys for development."""
        warnings.warn(
            "liboqs not available - using simulated ML-DSA-65. "
            "Install liboqs-python for real post-quantum security.",
            UserWarning,
        )
        seed = os.urandom(32)
        public_key, private_key = cls._simulate_keygen(seed)
        return cls(public_key, private_key, simulated=True)

    @classmethod
    def from_keys(cls, public_key_b64: str, private_key_b64: str) -> "MLDSASigner":
        """
        Create signer from base64-encoded keys.

        Args:
            public_key_b64: Base64-encoded public key
            private_key_b64: Base64-encoded private key

        Returns:
            MLDSASigner instance
        """
        public_key = base64.b64decode(public_key_b64)
        private_key = base64.b64decode(private_key_b64)

        # If liboqs is available, create an oqs signer with these keys
        oqs_signer = None
        simulated = True

        if _LIBOQS_AVAILABLE:
            try:
                # Create signer and import the secret key
                oqs_signer = _oqs.Signature("ML-DSA-65", private_key)
                simulated = False
            except Exception:
                # Fall back to simulated mode
                pass

        return cls(public_key, private_key, simulated=simulated, _oqs_signer=oqs_signer)

    @classmethod
    def _simulate_keygen(cls, seed: bytes) -> Tuple[bytes, bytes]:
        """
        Generate simulated ML-DSA keys from a seed.

        This creates deterministic placeholder keys for development.
        """
        hasher = hashlib.shake_256()
        hasher.update(b"MLDSA65-KEYGEN-SIMULATION-" + seed)
        key_material = hasher.digest(MLDSA_PUBLIC_KEY_SIZE + MLDSA_PRIVATE_KEY_SIZE)

        public_key = key_material[:MLDSA_PUBLIC_KEY_SIZE]
        private_key = key_material[MLDSA_PUBLIC_KEY_SIZE:]

        return public_key, private_key

    @property
    def algorithm(self) -> Algorithm:
        return Algorithm.MLDSA65

    @property
    def is_simulated(self) -> bool:
        """Return True if using placeholder implementation."""
        return self._simulated

    def public_key(self) -> str:
        """Return base64-encoded public key."""
        return base64.b64encode(self._public_key).decode("utf-8")

    def public_key_bytes(self) -> bytes:
        """Return raw public key bytes."""
        return self._public_key

    def private_key(self) -> str:
        """Return base64-encoded private key."""
        return base64.b64encode(self._private_key).decode("utf-8")

    def key_pair(self) -> KeyPair:
        """Return the key pair for storage."""
        return KeyPair(
            algorithm=Algorithm.MLDSA65,
            public_key=self.public_key(),
            private_key=self.private_key(),
        )

    def sign(self, message: bytes) -> str:
        """Sign message and return base64-encoded signature."""
        sig = self.sign_bytes(message)
        return base64.b64encode(sig).decode("utf-8")

    def sign_bytes(self, message: bytes) -> bytes:
        """
        Sign message and return raw signature bytes.

        Uses liboqs for real signatures if available.
        """
        if self._oqs_signer is not None:
            return self._oqs_signer.sign(message)
        else:
            return self._sign_simulated(message)

    def _sign_simulated(self, message: bytes) -> bytes:
        """Simulated signing using SHAKE256."""
        hasher = hashlib.shake_256()
        hasher.update(b"MLDSA65-SIGN-SIMULATION-")
        hasher.update(self._private_key[:64])
        hasher.update(message)
        return hasher.digest(MLDSA_SIGNATURE_SIZE)

    def verify(self, message: bytes, signature: str) -> bool:
        """Verify base64-encoded signature."""
        try:
            sig_bytes = base64.b64decode(signature)
            return self.verify_bytes(message, sig_bytes)
        except Exception:
            return False

    def verify_bytes(self, message: bytes, signature: bytes) -> bool:
        """
        Verify raw signature bytes.

        Uses liboqs for real verification if available.
        """
        if len(signature) != MLDSA_SIGNATURE_SIZE:
            return False

        if _LIBOQS_AVAILABLE:
            try:
                verifier = _oqs.Signature("ML-DSA-65")
                return verifier.verify(message, signature, self._public_key)
            except Exception:
                pass

        # Simulated verification (requires private key)
        if self._simulated:
            expected = self._sign_simulated(message)
            return signature == expected

        return False


class MLDSAVerifier(Verifier):
    """
    ML-DSA-65 verifier (public key only).

    Uses real NIST-standardized ML-DSA-65 verification when liboqs is available.
    """

    def __init__(self, public_key: bytes, simulated: bool = False):
        """
        Initialize ML-DSA verifier.

        Args:
            public_key: Public key bytes (1952 bytes)
            simulated: Whether this is a simulated implementation
        """
        if len(public_key) != MLDSA_PUBLIC_KEY_SIZE:
            raise ValueError(
                f"Invalid public key size: expected {MLDSA_PUBLIC_KEY_SIZE}, "
                f"got {len(public_key)}"
            )
        self._public_key = public_key
        self._simulated = simulated

    @classmethod
    def from_public_key(cls, public_key_b64: str) -> "MLDSAVerifier":
        """
        Create verifier from base64-encoded public key.

        Args:
            public_key_b64: Base64-encoded public key

        Returns:
            MLDSAVerifier instance
        """
        public_key = base64.b64decode(public_key_b64)
        simulated = not _LIBOQS_AVAILABLE
        return cls(public_key, simulated=simulated)

    @property
    def algorithm(self) -> Algorithm:
        return Algorithm.MLDSA65

    @property
    def is_simulated(self) -> bool:
        """Return True if using placeholder implementation."""
        return self._simulated

    def verify(self, message: bytes, signature: str) -> bool:
        """Verify base64-encoded signature."""
        try:
            sig_bytes = base64.b64decode(signature)
            return self.verify_bytes(message, sig_bytes)
        except Exception:
            return False

    def verify_bytes(self, message: bytes, signature: bytes) -> bool:
        """
        Verify raw signature bytes.

        Uses liboqs for real verification if available.
        In simulated mode without liboqs, verification always fails
        since it requires the private key.
        """
        if len(signature) != MLDSA_SIGNATURE_SIZE:
            return False

        if _LIBOQS_AVAILABLE:
            try:
                verifier = _oqs.Signature("ML-DSA-65")
                return verifier.verify(message, signature, self._public_key)
            except Exception:
                return False

        # Simulated mode cannot verify without private key
        return False
