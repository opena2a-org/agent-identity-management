"""
Hybrid cryptography integration for agent registration.

This module provides a bridge between the existing Ed25519-based registration
and the new hybrid (Ed25519 + ML-DSA-65) cryptographic system. It enables:

1. Gradual migration from classical to hybrid crypto
2. Backward compatibility with existing Ed25519-only backends
3. Future-proofing with post-quantum signatures

Usage:
    from aim_sdk.crypto.integration import HybridKeyGenerator, HybridCredentials

    # Generate hybrid keys for registration
    generator = HybridKeyGenerator()
    credentials = generator.generate()

    # Get classical key for current backend compatibility
    public_key = credentials.classical_public_key

    # Store full hybrid credentials for future use
    full_creds = credentials.to_dict()
"""

import base64
from dataclasses import dataclass, field
from typing import Dict, Any, Optional
from pathlib import Path
import json

from .hybrid import HybridSigner
from .signer import HybridKeyPair


@dataclass
class HybridCredentials:
    """
    Hybrid cryptographic credentials for an agent.

    Contains both classical (Ed25519) and post-quantum (ML-DSA-65) keys.
    The classical key can be used with existing backends, while the full
    hybrid key pair is stored for future post-quantum verification.
    """

    # Classical (Ed25519) keys - compatible with current backend
    classical_public_key: str  # Base64-encoded (32 bytes)
    classical_private_key: str  # Base64-encoded (64 bytes: seed + public, Go format)

    # Post-quantum (ML-DSA-65) keys - for future use
    pqc_public_key: str  # Base64-encoded (1952 bytes)
    pqc_private_key: str  # Base64-encoded (4032 bytes)

    # Full hybrid key pair for signing
    hybrid_key_pair: HybridKeyPair = field(repr=False)

    # Metadata
    algorithm: str = "hybrid-ed25519-mldsa65"
    is_hybrid: bool = True

    @property
    def ed25519_public_key_bytes(self) -> bytes:
        """Get Ed25519 public key as bytes (32 bytes)."""
        return base64.b64decode(self.classical_public_key)

    @property
    def ed25519_private_key_bytes(self) -> bytes:
        """
        Get Ed25519 private key seed as bytes (32 bytes).

        Extracts the seed from the 64-byte Go-compatible format.
        """
        full_key = base64.b64decode(self.classical_private_key)
        # Go format is seed (32 bytes) + public (32 bytes)
        return full_key[:32]

    @property
    def ed25519_private_key_full(self) -> bytes:
        """
        Get Ed25519 private key in Go-compatible format (64 bytes).

        Go expects: seed (32 bytes) + public key (32 bytes)
        """
        return base64.b64decode(self.classical_private_key)

    @property
    def ed25519_private_key_full_b64(self) -> str:
        """Get Go-compatible Ed25519 private key as base64."""
        return self.classical_private_key  # Already in correct format

    def to_dict(self) -> Dict[str, Any]:
        """
        Convert credentials to dictionary for storage.

        The classical keys are stored in the format expected by the current
        AIM backend and client, while hybrid keys are stored separately
        for future use.
        """
        return {
            # Primary keys (classical) - compatible with current system
            "public_key": self.classical_public_key,
            "private_key": self.classical_private_key,  # Already 64-byte Go format

            # Hybrid extension - for future post-quantum support
            "hybrid": {
                "algorithm": self.algorithm,
                "classical_public": self.classical_public_key,
                "classical_private": self.classical_private_key,  # 64-byte Go format
                "pqc_public": self.pqc_public_key,
                "pqc_private": self.pqc_private_key,
            },

            # Flag indicating hybrid mode
            "is_hybrid": True,
        }

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "HybridCredentials":
        """Create credentials from stored dictionary."""
        hybrid_data = data.get("hybrid", {})

        # Extract keys - prefer hybrid section, fall back to top-level
        classical_public = hybrid_data.get("classical_public", data.get("public_key"))
        classical_private = hybrid_data.get("classical_private", data.get("private_key"))

        pqc_public = hybrid_data.get("pqc_public", "")
        pqc_private = hybrid_data.get("pqc_private", "")

        # Reconstruct hybrid key pair
        from .signer import Algorithm
        key_pair = HybridKeyPair(
            algorithm=Algorithm.HYBRID,
            classical_public=classical_public,
            classical_private=classical_private,
            pqc_public=pqc_public,
            pqc_private=pqc_private,
        )

        return cls(
            classical_public_key=classical_public,
            classical_private_key=classical_private,
            pqc_public_key=pqc_public,
            pqc_private_key=pqc_private,
            hybrid_key_pair=key_pair,
            algorithm=hybrid_data.get("algorithm", "hybrid-ed25519-mldsa65"),
        )

    def get_signer(self) -> HybridSigner:
        """Get a HybridSigner instance for signing operations."""
        return HybridSigner.from_key_pair(self.hybrid_key_pair)


class HybridKeyGenerator:
    """
    Generator for hybrid cryptographic keys.

    Creates key pairs that contain both classical (Ed25519) and
    post-quantum (ML-DSA-65) components.
    """

    def generate(self) -> HybridCredentials:
        """
        Generate a new hybrid key pair.

        Returns:
            HybridCredentials with both classical and PQC keys
        """
        # Generate hybrid signer (Ed25519 + ML-DSA-65)
        signer = HybridSigner.generate()
        key_pair = signer.key_pair()

        return HybridCredentials(
            classical_public_key=key_pair.classical_public,
            classical_private_key=key_pair.classical_private,
            pqc_public_key=key_pair.pqc_public,
            pqc_private_key=key_pair.pqc_private,
            hybrid_key_pair=key_pair,
        )


def generate_hybrid_credentials() -> HybridCredentials:
    """
    Convenience function to generate hybrid credentials.

    Returns:
        HybridCredentials ready for agent registration
    """
    generator = HybridKeyGenerator()
    return generator.generate()


def upgrade_credentials_to_hybrid(
    credentials: Dict[str, Any]
) -> HybridCredentials:
    """
    Upgrade existing Ed25519-only credentials to hybrid.

    This keeps the existing Ed25519 key pair and adds a new
    ML-DSA-65 key pair for post-quantum security.

    Note: This creates a NEW PQC key - the agent's public key
    on the server will need to be updated to include the PQC
    component.

    Args:
        credentials: Existing credentials dict with public_key/private_key

    Returns:
        HybridCredentials with existing Ed25519 + new ML-DSA-65
    """
    from .mldsa import MLDSASigner

    # Extract existing Ed25519 keys
    public_key_b64 = credentials.get("public_key", "")
    private_key_b64 = credentials.get("private_key", "")

    # Handle 64-byte Go format private key
    private_bytes = base64.b64decode(private_key_b64)
    if len(private_bytes) == 64:
        # Extract 32-byte seed
        classical_private = base64.b64encode(private_bytes[:32]).decode('utf-8')
    else:
        classical_private = private_key_b64

    # Generate new ML-DSA-65 key pair
    mldsa_signer = MLDSASigner.generate()
    mldsa_keypair = mldsa_signer.key_pair()

    # Create hybrid key pair
    from .signer import Algorithm
    key_pair = HybridKeyPair(
        algorithm=Algorithm.HYBRID,
        classical_public=public_key_b64,
        classical_private=classical_private,
        pqc_public=mldsa_keypair.public_key,
        pqc_private=mldsa_keypair.private_key,
    )

    return HybridCredentials(
        classical_public_key=public_key_b64,
        classical_private_key=classical_private,
        pqc_public_key=mldsa_keypair.public_key,
        pqc_private_key=mldsa_keypair.private_key,
        hybrid_key_pair=key_pair,
    )


def is_hybrid_credentials(credentials: Dict[str, Any]) -> bool:
    """Check if credentials contain hybrid keys."""
    return credentials.get("is_hybrid", False) or "hybrid" in credentials
