"""
Post-Quantum Cryptography (PQC) module for AIM SDK.

This module implements NIST FIPS 204 compliant ML-DSA (formerly Dilithium)
digital signatures for post-quantum security, as well as hybrid Ed25519+ML-DSA
signatures for defense-in-depth during the cryptographic transition period.

Supported Algorithms:
- ML-DSA-44: NIST Security Level 2 (128-bit classical equivalent)
- ML-DSA-65: NIST Security Level 3 (192-bit classical equivalent) - RECOMMENDED
- ML-DSA-87: NIST Security Level 5 (256-bit classical equivalent)
- Ed25519+ML-DSA-65: Hybrid mode combining classical and post-quantum (RECOMMENDED)

Dependencies:
- liboqs-python: Open Quantum Safe library bindings

Installation:
    pip install liboqs-python

Or install AIM SDK with PQC extras:
    pip install aim-sdk[pqc]
"""

import base64
from dataclasses import dataclass
from enum import Enum
from typing import Optional, Tuple, Dict, Any
import time


# =============================================================================
# EXCEPTIONS
# =============================================================================

class PQCError(Exception):
    """Base exception for PQC operations."""
    pass


class PQCNotAvailableError(PQCError):
    """Raised when PQC library is not available."""
    def __init__(self, message: str = "PQC support not available. Install: pip install liboqs-python"):
        super().__init__(message)


class PQCSignatureError(PQCError):
    """Raised when signature verification fails."""
    pass


class PQCKeyError(PQCError):
    """Raised when key operations fail."""
    pass


# =============================================================================
# ALGORITHM CONSTANTS
# =============================================================================

class Algorithm(str, Enum):
    """Supported cryptographic algorithms."""
    ED25519 = "Ed25519"
    MLDSA_44 = "ML-DSA-44"
    MLDSA_65 = "ML-DSA-65"
    MLDSA_87 = "ML-DSA-87"
    HYBRID_ED25519_MLDSA_44 = "Ed25519+ML-DSA-44"
    HYBRID_ED25519_MLDSA_65 = "Ed25519+ML-DSA-65"
    HYBRID_ED25519_MLDSA_87 = "Ed25519+ML-DSA-87"


# Algorithm name mappings for liboqs
LIBOQS_ALGORITHM_NAMES = {
    Algorithm.MLDSA_44: "ML-DSA-44",
    Algorithm.MLDSA_65: "ML-DSA-65",
    Algorithm.MLDSA_87: "ML-DSA-87",
    Algorithm.HYBRID_ED25519_MLDSA_44: "ML-DSA-44",
    Algorithm.HYBRID_ED25519_MLDSA_65: "ML-DSA-65",
    Algorithm.HYBRID_ED25519_MLDSA_87: "ML-DSA-87",
}

# Key and signature sizes for each algorithm
KEY_SIZES = {
    Algorithm.ED25519: {
        "public_key": 32,
        "private_key": 32,  # Seed only (not the full 64-byte key)
        "signature": 64,
    },
    Algorithm.MLDSA_44: {
        "public_key": 1312,
        "private_key": 2560,
        "signature": 2420,
    },
    Algorithm.MLDSA_65: {
        "public_key": 1952,
        "private_key": 4032,
        "signature": 3309,
    },
    Algorithm.MLDSA_87: {
        "public_key": 2592,
        "private_key": 4896,
        "signature": 4627,
    },
}

# All supported algorithms
ALGORITHMS = {
    "classical": [Algorithm.ED25519],
    "post_quantum": [Algorithm.MLDSA_44, Algorithm.MLDSA_65, Algorithm.MLDSA_87],
    "hybrid": [
        Algorithm.HYBRID_ED25519_MLDSA_44,
        Algorithm.HYBRID_ED25519_MLDSA_65,
        Algorithm.HYBRID_ED25519_MLDSA_87,
    ],
}


# =============================================================================
# PQC AVAILABILITY DETECTION
# =============================================================================

_pqc_available: Optional[bool] = None
_liboqs_sig = None
_availability_error: Optional[str] = None


def _check_pqc_availability() -> bool:
    """Check if liboqs-python is available and functional."""
    global _pqc_available, _liboqs_sig, _availability_error

    if _pqc_available is not None:
        return _pqc_available

    try:
        import oqs
        _liboqs_sig = oqs.Signature

        # Verify ML-DSA-65 is available (most common variant)
        supported = oqs.get_enabled_sig_mechanisms()
        if "ML-DSA-65" not in supported:
            _availability_error = "ML-DSA-65 not enabled in liboqs"
            _pqc_available = False
            return False

        _pqc_available = True
        return True

    except ImportError as e:
        _availability_error = f"liboqs-python not installed: {e}"
        _pqc_available = False
        return False
    except SystemExit:
        # liboqs-python raises SystemExit if native library not found
        _availability_error = "liboqs native library not found"
        _pqc_available = False
        return False
    except Exception as e:
        _availability_error = f"liboqs initialization failed: {e}"
        _pqc_available = False
        return False


def is_pqc_available() -> bool:
    """
    Check if post-quantum cryptography is available.

    Returns:
        True if liboqs-python is installed and functional, False otherwise.
    """
    return _check_pqc_availability()


def get_pqc_availability_info() -> Dict[str, Any]:
    """
    Get detailed information about PQC availability.

    Returns:
        Dictionary with availability status and details.
    """
    available = _check_pqc_availability()

    info = {
        "available": available,
        "library": "liboqs-python" if available else None,
        "algorithms": [],
        "error": None if available else _availability_error,
    }

    if available:
        try:
            import oqs
            info["version"] = getattr(oqs, "__version__", "unknown")
            info["algorithms"] = [
                alg for alg in oqs.get_enabled_sig_mechanisms()
                if alg.startswith("ML-DSA")
            ]
        except Exception:
            pass

    return info


def _require_pqc():
    """Raise exception if PQC is not available."""
    if not _check_pqc_availability():
        raise PQCNotAvailableError(_availability_error or "PQC not available")


# =============================================================================
# KEY GENERATION
# =============================================================================

@dataclass
class MLDSAKeyPair:
    """ML-DSA key pair."""
    algorithm: Algorithm
    public_key: bytes
    private_key: bytes

    def public_key_base64(self) -> str:
        """Return base64-encoded public key."""
        return base64.b64encode(self.public_key).decode('ascii')

    def private_key_base64(self) -> str:
        """Return base64-encoded private key."""
        return base64.b64encode(self.private_key).decode('ascii')


@dataclass
class HybridKeyPair:
    """Hybrid Ed25519+ML-DSA key pair."""
    algorithm: Algorithm
    ed25519_public_key: bytes
    ed25519_private_key: bytes
    mldsa_public_key: bytes
    mldsa_private_key: bytes

    def ed25519_public_key_base64(self) -> str:
        return base64.b64encode(self.ed25519_public_key).decode('ascii')

    def ed25519_private_key_base64(self) -> str:
        return base64.b64encode(self.ed25519_private_key).decode('ascii')

    def mldsa_public_key_base64(self) -> str:
        return base64.b64encode(self.mldsa_public_key).decode('ascii')

    def mldsa_private_key_base64(self) -> str:
        return base64.b64encode(self.mldsa_private_key).decode('ascii')


def generate_mldsa_keypair(
    algorithm: Algorithm = Algorithm.MLDSA_65
) -> MLDSAKeyPair:
    """
    Generate an ML-DSA key pair.

    Args:
        algorithm: ML-DSA variant (MLDSA_44, MLDSA_65, or MLDSA_87).
                   Default is MLDSA_65 (NIST Level 3, recommended).

    Returns:
        MLDSAKeyPair containing public and private keys.

    Raises:
        PQCNotAvailableError: If liboqs is not installed.
        PQCKeyError: If key generation fails.
    """
    _require_pqc()

    if algorithm not in LIBOQS_ALGORITHM_NAMES:
        raise PQCKeyError(f"Unsupported algorithm: {algorithm}")

    liboqs_name = LIBOQS_ALGORITHM_NAMES[algorithm]

    try:
        import oqs
        sig = oqs.Signature(liboqs_name)
        public_key = sig.generate_keypair()
        private_key = sig.export_secret_key()

        return MLDSAKeyPair(
            algorithm=algorithm,
            public_key=public_key,
            private_key=private_key,
        )

    except Exception as e:
        raise PQCKeyError(f"ML-DSA key generation failed: {e}") from e


def generate_hybrid_keypair(
    algorithm: Algorithm = Algorithm.HYBRID_ED25519_MLDSA_65
) -> HybridKeyPair:
    """
    Generate a hybrid Ed25519+ML-DSA key pair.

    Args:
        algorithm: Hybrid algorithm variant.
                   Default is Ed25519+ML-DSA-65 (recommended).

    Returns:
        HybridKeyPair containing both Ed25519 and ML-DSA keys.

    Raises:
        PQCNotAvailableError: If liboqs is not installed.
        PQCKeyError: If key generation fails.
    """
    _require_pqc()

    # Generate Ed25519 keys using PyNaCl
    try:
        from nacl.signing import SigningKey
        ed_signing_key = SigningKey.generate()
        ed25519_private = bytes(ed_signing_key)
        ed25519_public = bytes(ed_signing_key.verify_key)
    except ImportError:
        raise PQCKeyError("PyNaCl required for Ed25519: pip install PyNaCl")
    except Exception as e:
        raise PQCKeyError(f"Ed25519 key generation failed: {e}") from e

    # Generate ML-DSA keys
    mldsa_keypair = generate_mldsa_keypair(
        _get_pure_algorithm(algorithm)
    )

    return HybridKeyPair(
        algorithm=algorithm,
        ed25519_public_key=ed25519_public,
        ed25519_private_key=ed25519_private,
        mldsa_public_key=mldsa_keypair.public_key,
        mldsa_private_key=mldsa_keypair.private_key,
    )


def _get_pure_algorithm(hybrid_alg: Algorithm) -> Algorithm:
    """Extract the pure ML-DSA algorithm from a hybrid algorithm."""
    mapping = {
        Algorithm.HYBRID_ED25519_MLDSA_44: Algorithm.MLDSA_44,
        Algorithm.HYBRID_ED25519_MLDSA_65: Algorithm.MLDSA_65,
        Algorithm.HYBRID_ED25519_MLDSA_87: Algorithm.MLDSA_87,
    }
    return mapping.get(hybrid_alg, hybrid_alg)


# =============================================================================
# SIGNING
# =============================================================================

def sign_mldsa(
    private_key: bytes,
    message: bytes,
    algorithm: Algorithm = Algorithm.MLDSA_65
) -> bytes:
    """
    Sign a message using ML-DSA.

    Args:
        private_key: ML-DSA private key bytes.
        message: Message to sign.
        algorithm: ML-DSA variant matching the key.

    Returns:
        ML-DSA signature bytes.

    Raises:
        PQCNotAvailableError: If liboqs is not installed.
        PQCSignatureError: If signing fails.
    """
    _require_pqc()

    liboqs_name = LIBOQS_ALGORITHM_NAMES.get(algorithm)
    if not liboqs_name:
        raise PQCSignatureError(f"Unsupported algorithm: {algorithm}")

    try:
        import oqs
        sig = oqs.Signature(liboqs_name, private_key)
        signature = sig.sign(message)
        return signature

    except Exception as e:
        raise PQCSignatureError(f"ML-DSA signing failed: {e}") from e


@dataclass
class HybridSignature:
    """Hybrid Ed25519+ML-DSA signature."""
    algorithm: Algorithm
    ed25519_signature: bytes
    mldsa_signature: bytes
    timestamp: int  # Unix timestamp

    def ed25519_signature_base64(self) -> str:
        return base64.b64encode(self.ed25519_signature).decode('ascii')

    def mldsa_signature_base64(self) -> str:
        return base64.b64encode(self.mldsa_signature).decode('ascii')

    def to_dict(self) -> Dict[str, Any]:
        """Convert to dictionary for JSON serialization."""
        return {
            "algorithm": self.algorithm.value,
            "ed25519_signature": self.ed25519_signature_base64(),
            "mldsa_signature": self.mldsa_signature_base64(),
            "timestamp": self.timestamp,
        }


def sign_hybrid(
    ed25519_private_key: bytes,
    mldsa_private_key: bytes,
    message: bytes,
    algorithm: Algorithm = Algorithm.HYBRID_ED25519_MLDSA_65,
    timestamp: Optional[int] = None,
) -> HybridSignature:
    """
    Sign a message using hybrid Ed25519+ML-DSA.

    BOTH signatures must be valid for verification to succeed.
    This provides defense-in-depth during the quantum transition.

    Args:
        ed25519_private_key: Ed25519 private key bytes (32-byte seed).
        mldsa_private_key: ML-DSA private key bytes.
        message: Message to sign.
        algorithm: Hybrid algorithm variant.
        timestamp: Optional Unix timestamp (defaults to current time).

    Returns:
        HybridSignature containing both signatures.

    Raises:
        PQCNotAvailableError: If liboqs is not installed.
        PQCSignatureError: If signing fails.
    """
    _require_pqc()

    ts = timestamp or int(time.time())

    # Sign with Ed25519
    try:
        from nacl.signing import SigningKey
        ed_signing_key = SigningKey(ed25519_private_key)
        ed_signed = ed_signing_key.sign(message)
        ed25519_sig = ed_signed.signature
    except Exception as e:
        raise PQCSignatureError(f"Ed25519 signing failed: {e}") from e

    # Sign with ML-DSA
    pure_alg = _get_pure_algorithm(algorithm)
    mldsa_sig = sign_mldsa(mldsa_private_key, message, pure_alg)

    return HybridSignature(
        algorithm=algorithm,
        ed25519_signature=ed25519_sig,
        mldsa_signature=mldsa_sig,
        timestamp=ts,
    )


# =============================================================================
# VERIFICATION
# =============================================================================

def verify_mldsa(
    public_key: bytes,
    message: bytes,
    signature: bytes,
    algorithm: Algorithm = Algorithm.MLDSA_65
) -> bool:
    """
    Verify an ML-DSA signature.

    Args:
        public_key: ML-DSA public key bytes.
        message: Original message.
        signature: Signature to verify.
        algorithm: ML-DSA variant matching the key.

    Returns:
        True if signature is valid.

    Raises:
        PQCNotAvailableError: If liboqs is not installed.
        PQCSignatureError: If verification encounters an error.
    """
    _require_pqc()

    liboqs_name = LIBOQS_ALGORITHM_NAMES.get(algorithm)
    if not liboqs_name:
        raise PQCSignatureError(f"Unsupported algorithm: {algorithm}")

    try:
        import oqs
        sig = oqs.Signature(liboqs_name)
        return sig.verify(message, signature, public_key)

    except Exception as e:
        # Verification failure is not an exception, it returns False
        # Only raise on actual errors
        raise PQCSignatureError(f"ML-DSA verification error: {e}") from e


def verify_hybrid(
    ed25519_public_key: bytes,
    mldsa_public_key: bytes,
    message: bytes,
    ed25519_signature: bytes,
    mldsa_signature: bytes,
    algorithm: Algorithm = Algorithm.HYBRID_ED25519_MLDSA_65
) -> Tuple[bool, str]:
    """
    Verify a hybrid Ed25519+ML-DSA signature.

    BOTH signatures must be valid for verification to succeed.
    This is a security-critical requirement for hybrid mode.

    Args:
        ed25519_public_key: Ed25519 public key bytes.
        mldsa_public_key: ML-DSA public key bytes.
        message: Original message.
        ed25519_signature: Ed25519 signature bytes.
        mldsa_signature: ML-DSA signature bytes.
        algorithm: Hybrid algorithm variant.

    Returns:
        Tuple of (is_valid: bool, reason: str).
        Both signatures must be valid for is_valid to be True.

    Raises:
        PQCNotAvailableError: If liboqs is not installed.
    """
    _require_pqc()

    # Import nacl exceptions at function scope (before try block)
    from nacl.signing import VerifyKey
    from nacl.exceptions import BadSignatureError

    # Verify Ed25519 first (faster)
    try:
        verify_key = VerifyKey(ed25519_public_key)
        verify_key.verify(message, ed25519_signature)
    except BadSignatureError:
        return False, "Ed25519 signature invalid"
    except Exception as e:
        return False, f"Ed25519 verification error: {e}"

    # Verify ML-DSA
    try:
        pure_alg = _get_pure_algorithm(algorithm)
        mldsa_valid = verify_mldsa(
            mldsa_public_key, message, mldsa_signature, pure_alg
        )
        if not mldsa_valid:
            return False, "ML-DSA signature invalid"
    except PQCSignatureError as e:
        return False, f"ML-DSA verification error: {e}"

    # BOTH must be valid
    return True, "Both signatures valid"
