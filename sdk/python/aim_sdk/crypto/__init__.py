"""
Post-Quantum Cryptographic Hybrid Signing Module.

This module provides a crypto-agile signing framework that supports:
- Classical algorithms (Ed25519)
- Post-quantum algorithms (ML-DSA-65/Dilithium3)
- Hybrid mode combining both for defense-in-depth

The hybrid approach provides security against both classical and quantum
attackers during the transition period to post-quantum cryptography.
"""

from .signer import (
    Algorithm,
    Signer,
    Verifier,
    KeyPair,
    HybridKeyPair,
    HybridSignature,
    encode_hybrid_signature,
    decode_hybrid_signature,
)
from .ed25519 import Ed25519Signer, Ed25519Verifier
from .mldsa import MLDSASigner, MLDSAVerifier
from .hybrid import HybridSigner, HybridVerifier

__all__ = [
    # Core types
    "Algorithm",
    "Signer",
    "Verifier",
    "KeyPair",
    "HybridKeyPair",
    "HybridSignature",
    # Encoding utilities
    "encode_hybrid_signature",
    "decode_hybrid_signature",
    # Ed25519
    "Ed25519Signer",
    "Ed25519Verifier",
    # ML-DSA (PQC)
    "MLDSASigner",
    "MLDSAVerifier",
    # Hybrid
    "HybridSigner",
    "HybridVerifier",
]
