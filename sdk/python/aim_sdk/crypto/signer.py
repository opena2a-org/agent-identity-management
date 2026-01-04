"""
Base interfaces and types for cryptographic signing.

This module defines the core abstractions for the crypto-agile signing framework.
"""

import struct
import base64
from abc import ABC, abstractmethod
from dataclasses import dataclass
from enum import Enum
from typing import Tuple, Optional


class Algorithm(str, Enum):
    """Cryptographic signing algorithm identifiers."""

    ED25519 = "ed25519"
    """Classical Ed25519 signature algorithm."""

    MLDSA65 = "mldsa65"
    """Post-quantum ML-DSA-65 (Dilithium3) algorithm. NIST FIPS 204."""

    HYBRID = "hybrid-ed25519-mldsa65"
    """Hybrid combining Ed25519 and ML-DSA-65 for defense-in-depth."""


@dataclass
class KeyPair:
    """Key pair for a single signing algorithm."""

    algorithm: Algorithm
    public_key: str  # Base64 encoded
    private_key: str  # Base64 encoded


@dataclass
class HybridKeyPair:
    """Key pair for hybrid signing (both classical and PQC)."""

    algorithm: Algorithm
    classical_public: str  # Base64 encoded Ed25519 public key
    classical_private: str  # Base64 encoded Ed25519 private key
    pqc_public: str  # Base64 encoded ML-DSA-65 public key
    pqc_private: str  # Base64 encoded ML-DSA-65 private key


@dataclass
class HybridSignature:
    """Container for signatures from both algorithms."""

    classical: bytes  # Ed25519 signature (64 bytes)
    pqc: bytes  # ML-DSA-65 signature (~3309 bytes)


def encode_hybrid_signature(sig: HybridSignature) -> bytes:
    """
    Encode a hybrid signature to bytes.

    Format: [classical_len:4][classical_sig][pqc_sig]

    Args:
        sig: HybridSignature to encode

    Returns:
        Encoded bytes
    """
    # Pack classical length as 4-byte big-endian
    classical_len = struct.pack(">I", len(sig.classical))
    return classical_len + sig.classical + sig.pqc


def decode_hybrid_signature(data: bytes) -> HybridSignature:
    """
    Decode bytes to a hybrid signature.

    Args:
        data: Encoded signature bytes

    Returns:
        HybridSignature

    Raises:
        ValueError: If data is malformed
    """
    if len(data) < 4:
        raise ValueError("Hybrid signature too short")

    classical_len = struct.unpack(">I", data[:4])[0]

    if len(data) < 4 + classical_len:
        raise ValueError("Hybrid signature truncated")

    return HybridSignature(
        classical=data[4:4 + classical_len],
        pqc=data[4 + classical_len:]
    )


class Signer(ABC):
    """
    Abstract base class for cryptographic signers.

    Implementations must be thread-safe.
    """

    @property
    @abstractmethod
    def algorithm(self) -> Algorithm:
        """Return the algorithm identifier."""
        pass

    @abstractmethod
    def public_key(self) -> str:
        """Return the base64-encoded public key."""
        pass

    @abstractmethod
    def public_key_bytes(self) -> bytes:
        """Return the raw public key bytes."""
        pass

    @abstractmethod
    def sign(self, message: bytes) -> str:
        """
        Sign a message and return base64-encoded signature.

        Args:
            message: The message to sign

        Returns:
            Base64-encoded signature
        """
        pass

    @abstractmethod
    def sign_bytes(self, message: bytes) -> bytes:
        """
        Sign a message and return raw signature bytes.

        Args:
            message: The message to sign

        Returns:
            Raw signature bytes
        """
        pass

    @abstractmethod
    def verify(self, message: bytes, signature: str) -> bool:
        """
        Verify a base64-encoded signature.

        Args:
            message: The original message
            signature: Base64-encoded signature

        Returns:
            True if signature is valid
        """
        pass

    @abstractmethod
    def verify_bytes(self, message: bytes, signature: bytes) -> bool:
        """
        Verify raw signature bytes.

        Args:
            message: The original message
            signature: Raw signature bytes

        Returns:
            True if signature is valid
        """
        pass


class Verifier(ABC):
    """
    Abstract base class for signature verification without private key access.
    """

    @property
    @abstractmethod
    def algorithm(self) -> Algorithm:
        """Return the algorithm identifier."""
        pass

    @abstractmethod
    def verify(self, message: bytes, signature: str) -> bool:
        """
        Verify a base64-encoded signature.

        Args:
            message: The original message
            signature: Base64-encoded signature

        Returns:
            True if signature is valid
        """
        pass

    @abstractmethod
    def verify_bytes(self, message: bytes, signature: bytes) -> bool:
        """
        Verify raw signature bytes.

        Args:
            message: The original message
            signature: Raw signature bytes

        Returns:
            True if signature is valid
        """
        pass
