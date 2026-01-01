"""
Cryptography module for AIM SDK.

This module provides cryptographic operations including:
- Ed25519 (classical) signatures
- ML-DSA (post-quantum) signatures via NIST FIPS 204
- Hybrid Ed25519+ML-DSA signatures for defense-in-depth

Post-quantum cryptography requires the 'liboqs-python' package:
    pip install liboqs-python

Or install with PQC extras:
    pip install aim-sdk[pqc]
"""

from .pqc import (
    # Constants
    Algorithm,
    ALGORITHMS,
    KEY_SIZES,

    # Key generation
    generate_mldsa_keypair,
    generate_hybrid_keypair,

    # Signing
    sign_mldsa,
    sign_hybrid,

    # Verification
    verify_mldsa,
    verify_hybrid,

    # Availability checks
    is_pqc_available,
    get_pqc_availability_info,

    # Exceptions
    PQCError,
    PQCNotAvailableError,
    PQCSignatureError,
    PQCKeyError,
)

__all__ = [
    # Constants
    'Algorithm',
    'ALGORITHMS',
    'KEY_SIZES',

    # Key generation
    'generate_mldsa_keypair',
    'generate_hybrid_keypair',

    # Signing
    'sign_mldsa',
    'sign_hybrid',

    # Verification
    'verify_mldsa',
    'verify_hybrid',

    # Availability
    'is_pqc_available',
    'get_pqc_availability_info',

    # Exceptions
    'PQCError',
    'PQCNotAvailableError',
    'PQCSignatureError',
    'PQCKeyError',
]
