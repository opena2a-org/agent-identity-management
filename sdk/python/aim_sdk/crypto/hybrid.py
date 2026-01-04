"""
Hybrid signing combining Ed25519 and ML-DSA-65.

This provides defense-in-depth during the post-quantum transition period:
- If quantum computers never materialize: Ed25519 remains secure
- If quantum computers break Ed25519: ML-DSA-65 remains secure
- Both signatures must verify for the hybrid to be valid
"""

import base64
from dataclasses import dataclass
from typing import Optional

from .signer import (
    Algorithm,
    Signer,
    Verifier,
    HybridKeyPair,
    HybridSignature,
    encode_hybrid_signature,
    decode_hybrid_signature,
)
from .ed25519 import Ed25519Signer, Ed25519Verifier
from .mldsa import MLDSASigner, MLDSAVerifier


@dataclass
class SignatureInfo:
    """Detailed information about signature verification."""

    algorithm: Algorithm
    classical_valid: bool
    pqc_valid: bool
    classical_algorithm: str
    pqc_algorithm: str
    signature_size: int


class HybridSigner(Signer):
    """
    Hybrid signer combining Ed25519 and ML-DSA-65.

    Provides defense-in-depth: both signatures must verify.
    """

    def __init__(self, classical: Ed25519Signer, pqc: MLDSASigner):
        """
        Initialize hybrid signer with both component signers.

        Args:
            classical: Ed25519 signer
            pqc: ML-DSA-65 signer
        """
        self._classical = classical
        self._pqc = pqc

    @classmethod
    def generate(cls) -> "HybridSigner":
        """
        Generate new key pairs for both algorithms.

        Returns:
            New HybridSigner with generated keys
        """
        classical = Ed25519Signer.generate()
        pqc = MLDSASigner.generate()
        return cls(classical, pqc)

    @classmethod
    def from_key_pair(cls, key_pair: HybridKeyPair) -> "HybridSigner":
        """
        Create hybrid signer from an existing key pair.

        Args:
            key_pair: HybridKeyPair with keys for both algorithms

        Returns:
            HybridSigner instance
        """
        classical = Ed25519Signer.from_keys(
            key_pair.classical_public,
            key_pair.classical_private,
        )
        pqc = MLDSASigner.from_keys(
            key_pair.pqc_public,
            key_pair.pqc_private,
        )
        return cls(classical, pqc)

    @property
    def algorithm(self) -> Algorithm:
        return Algorithm.HYBRID

    @property
    def is_simulated(self) -> bool:
        """Return True if PQC component is using placeholder implementation."""
        return self._pqc.is_simulated

    def public_key(self) -> str:
        """
        Return combined public keys as base64-encoded hybrid format.

        Format: base64(classical_pub || pqc_pub)
        """
        combined = self.public_key_bytes()
        return base64.b64encode(combined).decode("utf-8")

    def public_key_bytes(self) -> bytes:
        """Return combined public key bytes."""
        classical = self._classical.public_key_bytes()
        pqc = self._pqc.public_key_bytes()
        return classical + pqc

    def classical_public_key(self) -> str:
        """Return the Ed25519 public key."""
        return self._classical.public_key()

    def pqc_public_key(self) -> str:
        """Return the ML-DSA-65 public key."""
        return self._pqc.public_key()

    def key_pair(self) -> HybridKeyPair:
        """Return the hybrid key pair for storage."""
        return HybridKeyPair(
            algorithm=Algorithm.HYBRID,
            classical_public=self._classical.public_key(),
            classical_private=self._classical.private_key(),
            pqc_public=self._pqc.public_key(),
            pqc_private=self._pqc.private_key(),
        )

    def sign(self, message: bytes) -> str:
        """
        Sign message with both algorithms and return base64-encoded hybrid signature.

        Args:
            message: The message to sign

        Returns:
            Base64-encoded hybrid signature
        """
        sig = self.sign_bytes(message)
        return base64.b64encode(sig).decode("utf-8")

    def sign_bytes(self, message: bytes) -> bytes:
        """
        Sign message with both algorithms and return encoded hybrid signature.

        Args:
            message: The message to sign

        Returns:
            Encoded hybrid signature bytes
        """
        # Sign with classical algorithm
        classical_sig = self._classical.sign_bytes(message)

        # Sign with PQC algorithm
        pqc_sig = self._pqc.sign_bytes(message)

        # Encode as hybrid signature
        hybrid = HybridSignature(classical=classical_sig, pqc=pqc_sig)
        return encode_hybrid_signature(hybrid)

    def verify(self, message: bytes, signature: str) -> bool:
        """
        Verify both signatures in a hybrid signature.

        BOTH must be valid for the hybrid signature to be valid.

        Args:
            message: The original message
            signature: Base64-encoded hybrid signature

        Returns:
            True if BOTH signatures are valid
        """
        try:
            sig_bytes = base64.b64decode(signature)
            return self.verify_bytes(message, sig_bytes)
        except Exception:
            return False

    def verify_bytes(self, message: bytes, signature: bytes) -> bool:
        """
        Verify both signatures in a hybrid signature.

        Args:
            message: The original message
            signature: Encoded hybrid signature bytes

        Returns:
            True if BOTH signatures are valid
        """
        try:
            # Decode hybrid signature
            hybrid = decode_hybrid_signature(signature)

            # Verify classical signature
            if not self._classical.verify_bytes(message, hybrid.classical):
                return False

            # Verify PQC signature
            if not self._pqc.verify_bytes(message, hybrid.pqc):
                return False

            return True
        except Exception:
            return False


class HybridVerifier(Verifier):
    """
    Hybrid verifier without private key access.

    Verifies both Ed25519 and ML-DSA-65 signatures.
    """

    def __init__(self, classical: Ed25519Verifier, pqc: MLDSAVerifier):
        """
        Initialize hybrid verifier with both component verifiers.

        Args:
            classical: Ed25519 verifier
            pqc: ML-DSA-65 verifier
        """
        self._classical = classical
        self._pqc = pqc

    @classmethod
    def from_public_keys(
        cls, classical_public_b64: str, pqc_public_b64: str
    ) -> "HybridVerifier":
        """
        Create hybrid verifier from both public keys.

        Args:
            classical_public_b64: Base64-encoded Ed25519 public key
            pqc_public_b64: Base64-encoded ML-DSA-65 public key

        Returns:
            HybridVerifier instance
        """
        classical = Ed25519Verifier.from_public_key(classical_public_b64)
        pqc = MLDSAVerifier.from_public_key(pqc_public_b64)
        return cls(classical, pqc)

    @property
    def algorithm(self) -> Algorithm:
        return Algorithm.HYBRID

    def verify(self, message: bytes, signature: str) -> bool:
        """
        Verify both signatures in a hybrid signature.

        Args:
            message: The original message
            signature: Base64-encoded hybrid signature

        Returns:
            True if BOTH signatures are valid
        """
        try:
            sig_bytes = base64.b64decode(signature)
            return self.verify_bytes(message, sig_bytes)
        except Exception:
            return False

    def verify_bytes(self, message: bytes, signature: bytes) -> bool:
        """
        Verify both signatures in a hybrid signature.

        Args:
            message: The original message
            signature: Encoded hybrid signature bytes

        Returns:
            True if BOTH signatures are valid
        """
        try:
            # Decode hybrid signature
            hybrid = decode_hybrid_signature(signature)

            # Verify classical signature
            if not self._classical.verify_bytes(message, hybrid.classical):
                return False

            # Verify PQC signature
            if not self._pqc.verify_bytes(message, hybrid.pqc):
                return False

            return True
        except Exception:
            return False

    def verify_with_info(self, message: bytes, signature: str) -> SignatureInfo:
        """
        Verify and return detailed information about the signature.

        Args:
            message: The original message
            signature: Base64-encoded hybrid signature

        Returns:
            SignatureInfo with details about each component
        """
        sig_bytes = base64.b64decode(signature)
        hybrid = decode_hybrid_signature(sig_bytes)

        classical_valid = self._classical.verify_bytes(message, hybrid.classical)
        pqc_valid = self._pqc.verify_bytes(message, hybrid.pqc)

        return SignatureInfo(
            algorithm=Algorithm.HYBRID,
            classical_valid=classical_valid,
            pqc_valid=pqc_valid,
            classical_algorithm=str(Algorithm.ED25519.value),
            pqc_algorithm=str(Algorithm.MLDSA65.value),
            signature_size=len(sig_bytes),
        )
