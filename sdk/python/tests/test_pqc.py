"""
Tests for Post-Quantum Cryptography (PQC) module.

These tests verify the ML-DSA and hybrid Ed25519+ML-DSA implementations.
Tests will be skipped if liboqs-python is not installed.
"""

import base64
import pytest
from unittest.mock import patch, MagicMock

from aim_sdk.crypto.pqc import (
    # Constants
    Algorithm,
    ALGORITHMS,
    KEY_SIZES,
    LIBOQS_ALGORITHM_NAMES,

    # Key generation
    generate_mldsa_keypair,
    generate_hybrid_keypair,

    # Signing
    sign_mldsa,
    sign_hybrid,

    # Verification
    verify_mldsa,
    verify_hybrid,

    # Availability
    is_pqc_available,
    get_pqc_availability_info,

    # Data classes
    MLDSAKeyPair,
    HybridKeyPair,
    HybridSignature,

    # Exceptions
    PQCError,
    PQCNotAvailableError,
    PQCSignatureError,
    PQCKeyError,
)


# =============================================================================
# FIXTURES AND HELPERS
# =============================================================================

# Check if PQC is available for conditional skipping
_pqc_available = None

def check_pqc_installed():
    """Check if liboqs-python is installed and functional."""
    global _pqc_available
    if _pqc_available is None:
        try:
            import oqs
            _pqc_available = "ML-DSA-65" in oqs.get_enabled_sig_mechanisms()
        except (ImportError, SystemExit, RuntimeError, Exception):
            # liboqs-python may raise SystemExit if native library not found
            _pqc_available = False
    return _pqc_available


# Skip marker for tests requiring liboqs
# Note: Call check_pqc_installed() at module load time with error handling
try:
    _pqc_skip = not check_pqc_installed()
except (SystemExit, Exception):
    _pqc_skip = True

requires_pqc = pytest.mark.skipif(
    _pqc_skip,
    reason="liboqs-python not installed or native library unavailable"
)


@pytest.fixture
def mldsa_keypair():
    """Generate ML-DSA-65 keypair for testing."""
    if not check_pqc_installed():
        pytest.skip("liboqs-python not installed")
    return generate_mldsa_keypair(Algorithm.MLDSA_65)


@pytest.fixture
def hybrid_keypair():
    """Generate hybrid Ed25519+ML-DSA-65 keypair for testing."""
    if not check_pqc_installed():
        pytest.skip("liboqs-python not installed")
    return generate_hybrid_keypair(Algorithm.HYBRID_ED25519_MLDSA_65)


# =============================================================================
# ALGORITHM CONSTANTS TESTS
# =============================================================================

class TestAlgorithmConstants:
    """Test algorithm constants and enums."""

    def test_algorithm_enum_values(self):
        """Test Algorithm enum has correct values."""
        assert Algorithm.ED25519.value == "Ed25519"
        assert Algorithm.MLDSA_44.value == "ML-DSA-44"
        assert Algorithm.MLDSA_65.value == "ML-DSA-65"
        assert Algorithm.MLDSA_87.value == "ML-DSA-87"
        assert Algorithm.HYBRID_ED25519_MLDSA_44.value == "Ed25519+ML-DSA-44"
        assert Algorithm.HYBRID_ED25519_MLDSA_65.value == "Ed25519+ML-DSA-65"
        assert Algorithm.HYBRID_ED25519_MLDSA_87.value == "Ed25519+ML-DSA-87"

    def test_algorithms_dict_structure(self):
        """Test ALGORITHMS dictionary structure."""
        assert "classical" in ALGORITHMS
        assert "post_quantum" in ALGORITHMS
        assert "hybrid" in ALGORITHMS

        assert Algorithm.ED25519 in ALGORITHMS["classical"]
        assert Algorithm.MLDSA_44 in ALGORITHMS["post_quantum"]
        assert Algorithm.MLDSA_65 in ALGORITHMS["post_quantum"]
        assert Algorithm.MLDSA_87 in ALGORITHMS["post_quantum"]
        assert Algorithm.HYBRID_ED25519_MLDSA_44 in ALGORITHMS["hybrid"]
        assert Algorithm.HYBRID_ED25519_MLDSA_65 in ALGORITHMS["hybrid"]
        assert Algorithm.HYBRID_ED25519_MLDSA_87 in ALGORITHMS["hybrid"]

    def test_key_sizes(self):
        """Test KEY_SIZES has correct NIST FIPS 204 values."""
        # Ed25519
        assert KEY_SIZES[Algorithm.ED25519]["public_key"] == 32
        assert KEY_SIZES[Algorithm.ED25519]["signature"] == 64

        # ML-DSA-44
        assert KEY_SIZES[Algorithm.MLDSA_44]["public_key"] == 1312
        assert KEY_SIZES[Algorithm.MLDSA_44]["private_key"] == 2560
        assert KEY_SIZES[Algorithm.MLDSA_44]["signature"] == 2420

        # ML-DSA-65 (recommended)
        assert KEY_SIZES[Algorithm.MLDSA_65]["public_key"] == 1952
        assert KEY_SIZES[Algorithm.MLDSA_65]["private_key"] == 4032
        assert KEY_SIZES[Algorithm.MLDSA_65]["signature"] == 3309

        # ML-DSA-87
        assert KEY_SIZES[Algorithm.MLDSA_87]["public_key"] == 2592
        assert KEY_SIZES[Algorithm.MLDSA_87]["private_key"] == 4896
        assert KEY_SIZES[Algorithm.MLDSA_87]["signature"] == 4627

    def test_liboqs_algorithm_names(self):
        """Test liboqs algorithm name mappings."""
        assert LIBOQS_ALGORITHM_NAMES[Algorithm.MLDSA_44] == "ML-DSA-44"
        assert LIBOQS_ALGORITHM_NAMES[Algorithm.MLDSA_65] == "ML-DSA-65"
        assert LIBOQS_ALGORITHM_NAMES[Algorithm.MLDSA_87] == "ML-DSA-87"
        # Hybrid algorithms map to their pure ML-DSA variant
        assert LIBOQS_ALGORITHM_NAMES[Algorithm.HYBRID_ED25519_MLDSA_65] == "ML-DSA-65"


# =============================================================================
# PQC AVAILABILITY TESTS
# =============================================================================

class TestPQCAvailability:
    """Test PQC availability detection."""

    def test_is_pqc_available_returns_bool(self):
        """Test is_pqc_available returns a boolean."""
        result = is_pqc_available()
        assert isinstance(result, bool)

    def test_get_pqc_availability_info_structure(self):
        """Test get_pqc_availability_info returns correct structure."""
        info = get_pqc_availability_info()

        assert "available" in info
        assert "library" in info
        assert "algorithms" in info
        assert "error" in info

        assert isinstance(info["available"], bool)
        assert isinstance(info["algorithms"], list)

    @requires_pqc
    def test_pqc_available_with_liboqs(self):
        """Test availability is True when liboqs is installed."""
        assert is_pqc_available() is True

        info = get_pqc_availability_info()
        assert info["available"] is True
        assert info["library"] == "liboqs-python"
        assert "ML-DSA-65" in info["algorithms"]
        assert info["error"] is None


# =============================================================================
# EXCEPTION TESTS
# =============================================================================

class TestExceptions:
    """Test PQC exception hierarchy."""

    def test_pqc_error_is_base_exception(self):
        """Test PQCError is the base exception."""
        assert issubclass(PQCNotAvailableError, PQCError)
        assert issubclass(PQCSignatureError, PQCError)
        assert issubclass(PQCKeyError, PQCError)

    def test_pqc_not_available_error_message(self):
        """Test PQCNotAvailableError has default message."""
        error = PQCNotAvailableError()
        assert "liboqs-python" in str(error)

    def test_pqc_not_available_error_custom_message(self):
        """Test PQCNotAvailableError accepts custom message."""
        error = PQCNotAvailableError("Custom error message")
        assert str(error) == "Custom error message"


# =============================================================================
# KEY GENERATION TESTS
# =============================================================================

class TestMLDSAKeyGeneration:
    """Test ML-DSA key generation."""

    @requires_pqc
    def test_generate_mldsa44_keypair(self):
        """Test ML-DSA-44 key generation."""
        keypair = generate_mldsa_keypair(Algorithm.MLDSA_44)

        assert isinstance(keypair, MLDSAKeyPair)
        assert keypair.algorithm == Algorithm.MLDSA_44
        assert len(keypair.public_key) == KEY_SIZES[Algorithm.MLDSA_44]["public_key"]
        assert len(keypair.private_key) == KEY_SIZES[Algorithm.MLDSA_44]["private_key"]

    @requires_pqc
    def test_generate_mldsa65_keypair(self):
        """Test ML-DSA-65 key generation (default/recommended)."""
        keypair = generate_mldsa_keypair()  # Default is ML-DSA-65

        assert isinstance(keypair, MLDSAKeyPair)
        assert keypair.algorithm == Algorithm.MLDSA_65
        assert len(keypair.public_key) == KEY_SIZES[Algorithm.MLDSA_65]["public_key"]
        assert len(keypair.private_key) == KEY_SIZES[Algorithm.MLDSA_65]["private_key"]

    @requires_pqc
    def test_generate_mldsa87_keypair(self):
        """Test ML-DSA-87 key generation."""
        keypair = generate_mldsa_keypair(Algorithm.MLDSA_87)

        assert isinstance(keypair, MLDSAKeyPair)
        assert keypair.algorithm == Algorithm.MLDSA_87
        assert len(keypair.public_key) == KEY_SIZES[Algorithm.MLDSA_87]["public_key"]
        assert len(keypair.private_key) == KEY_SIZES[Algorithm.MLDSA_87]["private_key"]

    @requires_pqc
    def test_keypair_base64_encoding(self, mldsa_keypair):
        """Test key pair base64 encoding methods."""
        pub_b64 = mldsa_keypair.public_key_base64()
        priv_b64 = mldsa_keypair.private_key_base64()

        # Verify base64 decodes back to original
        assert base64.b64decode(pub_b64) == mldsa_keypair.public_key
        assert base64.b64decode(priv_b64) == mldsa_keypair.private_key

    @requires_pqc
    def test_keypair_uniqueness(self):
        """Test that each key generation produces unique keys."""
        keypair1 = generate_mldsa_keypair()
        keypair2 = generate_mldsa_keypair()

        assert keypair1.public_key != keypair2.public_key
        assert keypair1.private_key != keypair2.private_key


class TestHybridKeyGeneration:
    """Test hybrid Ed25519+ML-DSA key generation."""

    @requires_pqc
    def test_generate_hybrid_keypair_default(self):
        """Test hybrid key generation with default algorithm."""
        keypair = generate_hybrid_keypair()

        assert isinstance(keypair, HybridKeyPair)
        assert keypair.algorithm == Algorithm.HYBRID_ED25519_MLDSA_65

        # Ed25519 key sizes
        assert len(keypair.ed25519_public_key) == 32
        assert len(keypair.ed25519_private_key) == 32

        # ML-DSA-65 key sizes
        assert len(keypair.mldsa_public_key) == KEY_SIZES[Algorithm.MLDSA_65]["public_key"]
        assert len(keypair.mldsa_private_key) == KEY_SIZES[Algorithm.MLDSA_65]["private_key"]

    @requires_pqc
    def test_generate_hybrid_keypair_mldsa44(self):
        """Test hybrid key generation with ML-DSA-44."""
        keypair = generate_hybrid_keypair(Algorithm.HYBRID_ED25519_MLDSA_44)

        assert keypair.algorithm == Algorithm.HYBRID_ED25519_MLDSA_44
        assert len(keypair.mldsa_public_key) == KEY_SIZES[Algorithm.MLDSA_44]["public_key"]

    @requires_pqc
    def test_generate_hybrid_keypair_mldsa87(self):
        """Test hybrid key generation with ML-DSA-87."""
        keypair = generate_hybrid_keypair(Algorithm.HYBRID_ED25519_MLDSA_87)

        assert keypair.algorithm == Algorithm.HYBRID_ED25519_MLDSA_87
        assert len(keypair.mldsa_public_key) == KEY_SIZES[Algorithm.MLDSA_87]["public_key"]

    @requires_pqc
    def test_hybrid_keypair_base64_encoding(self, hybrid_keypair):
        """Test hybrid key pair base64 encoding methods."""
        ed_pub_b64 = hybrid_keypair.ed25519_public_key_base64()
        ed_priv_b64 = hybrid_keypair.ed25519_private_key_base64()
        mldsa_pub_b64 = hybrid_keypair.mldsa_public_key_base64()
        mldsa_priv_b64 = hybrid_keypair.mldsa_private_key_base64()

        # Verify base64 decodes back to original
        assert base64.b64decode(ed_pub_b64) == hybrid_keypair.ed25519_public_key
        assert base64.b64decode(ed_priv_b64) == hybrid_keypair.ed25519_private_key
        assert base64.b64decode(mldsa_pub_b64) == hybrid_keypair.mldsa_public_key
        assert base64.b64decode(mldsa_priv_b64) == hybrid_keypair.mldsa_private_key


# =============================================================================
# ML-DSA SIGNING AND VERIFICATION TESTS
# =============================================================================

class TestMLDSASigningVerification:
    """Test ML-DSA signing and verification."""

    @requires_pqc
    def test_sign_verify_roundtrip(self, mldsa_keypair):
        """Test ML-DSA sign/verify round-trip."""
        message = b"Hello, Post-Quantum World!"

        signature = sign_mldsa(
            mldsa_keypair.private_key,
            message,
            mldsa_keypair.algorithm
        )

        # Verify signature size
        assert len(signature) == KEY_SIZES[mldsa_keypair.algorithm]["signature"]

        # Verify signature
        is_valid = verify_mldsa(
            mldsa_keypair.public_key,
            message,
            signature,
            mldsa_keypair.algorithm
        )

        assert is_valid is True

    @requires_pqc
    def test_sign_verify_all_variants(self):
        """Test sign/verify for all ML-DSA variants."""
        message = b"Testing all ML-DSA variants"

        for alg in [Algorithm.MLDSA_44, Algorithm.MLDSA_65, Algorithm.MLDSA_87]:
            keypair = generate_mldsa_keypair(alg)
            signature = sign_mldsa(keypair.private_key, message, alg)
            is_valid = verify_mldsa(keypair.public_key, message, signature, alg)

            assert is_valid is True, f"Failed for {alg}"

    @requires_pqc
    def test_verify_tampered_message(self, mldsa_keypair):
        """Test verification fails for tampered message."""
        message = b"Original message"
        signature = sign_mldsa(
            mldsa_keypair.private_key,
            message,
            mldsa_keypair.algorithm
        )

        tampered_message = b"Tampered message"
        is_valid = verify_mldsa(
            mldsa_keypair.public_key,
            tampered_message,
            signature,
            mldsa_keypair.algorithm
        )

        assert is_valid is False

    @requires_pqc
    def test_verify_wrong_key(self, mldsa_keypair):
        """Test verification fails with wrong public key."""
        message = b"Test message"
        signature = sign_mldsa(
            mldsa_keypair.private_key,
            message,
            mldsa_keypair.algorithm
        )

        # Generate different keypair
        other_keypair = generate_mldsa_keypair(mldsa_keypair.algorithm)

        is_valid = verify_mldsa(
            other_keypair.public_key,
            message,
            signature,
            mldsa_keypair.algorithm
        )

        assert is_valid is False

    @requires_pqc
    def test_sign_empty_message(self, mldsa_keypair):
        """Test signing empty message."""
        message = b""

        signature = sign_mldsa(
            mldsa_keypair.private_key,
            message,
            mldsa_keypair.algorithm
        )

        is_valid = verify_mldsa(
            mldsa_keypair.public_key,
            message,
            signature,
            mldsa_keypair.algorithm
        )

        assert is_valid is True

    @requires_pqc
    def test_sign_large_message(self, mldsa_keypair):
        """Test signing large message."""
        message = b"X" * 1_000_000  # 1MB message

        signature = sign_mldsa(
            mldsa_keypair.private_key,
            message,
            mldsa_keypair.algorithm
        )

        is_valid = verify_mldsa(
            mldsa_keypair.public_key,
            message,
            signature,
            mldsa_keypair.algorithm
        )

        assert is_valid is True


# =============================================================================
# HYBRID SIGNING AND VERIFICATION TESTS
# =============================================================================

class TestHybridSigningVerification:
    """Test hybrid Ed25519+ML-DSA signing and verification."""

    @requires_pqc
    def test_sign_verify_roundtrip(self, hybrid_keypair):
        """Test hybrid sign/verify round-trip."""
        message = b"Hybrid signature test message"

        signature = sign_hybrid(
            hybrid_keypair.ed25519_private_key,
            hybrid_keypair.mldsa_private_key,
            message,
            hybrid_keypair.algorithm
        )

        assert isinstance(signature, HybridSignature)
        assert signature.algorithm == hybrid_keypair.algorithm
        assert len(signature.ed25519_signature) == 64  # Ed25519 signature size

        # Verify both signatures
        is_valid, reason = verify_hybrid(
            hybrid_keypair.ed25519_public_key,
            hybrid_keypair.mldsa_public_key,
            message,
            signature.ed25519_signature,
            signature.mldsa_signature,
            hybrid_keypair.algorithm
        )

        assert is_valid is True
        assert "valid" in reason.lower()

    @requires_pqc
    def test_signature_has_timestamp(self, hybrid_keypair):
        """Test hybrid signature includes timestamp."""
        message = b"Test message"

        signature = sign_hybrid(
            hybrid_keypair.ed25519_private_key,
            hybrid_keypair.mldsa_private_key,
            message
        )

        assert isinstance(signature.timestamp, int)
        assert signature.timestamp > 0

    @requires_pqc
    def test_signature_custom_timestamp(self, hybrid_keypair):
        """Test hybrid signature with custom timestamp."""
        message = b"Test message"
        custom_ts = 1700000000

        signature = sign_hybrid(
            hybrid_keypair.ed25519_private_key,
            hybrid_keypair.mldsa_private_key,
            message,
            timestamp=custom_ts
        )

        assert signature.timestamp == custom_ts

    @requires_pqc
    def test_signature_to_dict(self, hybrid_keypair):
        """Test HybridSignature to_dict method."""
        message = b"Test message"

        signature = sign_hybrid(
            hybrid_keypair.ed25519_private_key,
            hybrid_keypair.mldsa_private_key,
            message
        )

        sig_dict = signature.to_dict()

        assert "algorithm" in sig_dict
        assert "ed25519_signature" in sig_dict
        assert "mldsa_signature" in sig_dict
        assert "timestamp" in sig_dict

        # Verify base64 encoded signatures can be decoded
        base64.b64decode(sig_dict["ed25519_signature"])
        base64.b64decode(sig_dict["mldsa_signature"])

    @requires_pqc
    def test_verify_fails_invalid_ed25519(self, hybrid_keypair):
        """Test verification fails when Ed25519 signature is invalid."""
        message = b"Test message"

        signature = sign_hybrid(
            hybrid_keypair.ed25519_private_key,
            hybrid_keypair.mldsa_private_key,
            message,
            hybrid_keypair.algorithm
        )

        # Create invalid Ed25519 signature
        invalid_ed_sig = bytes(64)  # All zeros

        is_valid, reason = verify_hybrid(
            hybrid_keypair.ed25519_public_key,
            hybrid_keypair.mldsa_public_key,
            message,
            invalid_ed_sig,
            signature.mldsa_signature,
            hybrid_keypair.algorithm
        )

        assert is_valid is False
        assert "Ed25519" in reason

    @requires_pqc
    def test_verify_fails_invalid_mldsa(self, hybrid_keypair):
        """Test verification fails when ML-DSA signature is invalid."""
        message = b"Test message"

        signature = sign_hybrid(
            hybrid_keypair.ed25519_private_key,
            hybrid_keypair.mldsa_private_key,
            message,
            hybrid_keypair.algorithm
        )

        # Create invalid ML-DSA signature (wrong size will cause error)
        invalid_mldsa_sig = bytes(len(signature.mldsa_signature))  # All zeros

        is_valid, reason = verify_hybrid(
            hybrid_keypair.ed25519_public_key,
            hybrid_keypair.mldsa_public_key,
            message,
            signature.ed25519_signature,
            invalid_mldsa_sig,
            hybrid_keypair.algorithm
        )

        assert is_valid is False
        assert "ML-DSA" in reason

    @requires_pqc
    def test_verify_fails_tampered_message(self, hybrid_keypair):
        """Test verification fails when message is tampered."""
        original_message = b"Original message"

        signature = sign_hybrid(
            hybrid_keypair.ed25519_private_key,
            hybrid_keypair.mldsa_private_key,
            original_message,
            hybrid_keypair.algorithm
        )

        tampered_message = b"Tampered message"

        is_valid, reason = verify_hybrid(
            hybrid_keypair.ed25519_public_key,
            hybrid_keypair.mldsa_public_key,
            tampered_message,
            signature.ed25519_signature,
            signature.mldsa_signature,
            hybrid_keypair.algorithm
        )

        assert is_valid is False

    @requires_pqc
    def test_both_signatures_required(self, hybrid_keypair):
        """Test that BOTH signatures must be valid (defense-in-depth)."""
        message = b"Security critical message"

        signature = sign_hybrid(
            hybrid_keypair.ed25519_private_key,
            hybrid_keypair.mldsa_private_key,
            message,
            hybrid_keypair.algorithm
        )

        # Verify with correct signatures works
        is_valid, _ = verify_hybrid(
            hybrid_keypair.ed25519_public_key,
            hybrid_keypair.mldsa_public_key,
            message,
            signature.ed25519_signature,
            signature.mldsa_signature,
            hybrid_keypair.algorithm
        )
        assert is_valid is True

        # Tamper with Ed25519 - should fail
        tampered_ed_sig = bytearray(signature.ed25519_signature)
        tampered_ed_sig[0] ^= 0xFF  # Flip bits

        is_valid, reason = verify_hybrid(
            hybrid_keypair.ed25519_public_key,
            hybrid_keypair.mldsa_public_key,
            message,
            bytes(tampered_ed_sig),
            signature.mldsa_signature,
            hybrid_keypair.algorithm
        )
        assert is_valid is False


# =============================================================================
# ERROR HANDLING TESTS
# =============================================================================

class TestErrorHandling:
    """Test error handling and edge cases."""

    def test_unsupported_algorithm_key_generation(self):
        """Test key generation with unsupported algorithm raises error."""
        if not check_pqc_installed():
            pytest.skip("liboqs-python not installed")

        with pytest.raises(PQCKeyError, match="Unsupported algorithm"):
            generate_mldsa_keypair(Algorithm.ED25519)  # Ed25519 is not ML-DSA

    @requires_pqc
    def test_unsupported_algorithm_signing(self, mldsa_keypair):
        """Test signing with unsupported algorithm raises error."""
        with pytest.raises(PQCSignatureError, match="Unsupported algorithm"):
            sign_mldsa(
                mldsa_keypair.private_key,
                b"message",
                Algorithm.ED25519  # Not valid for ML-DSA
            )


# =============================================================================
# DATA CLASS TESTS
# =============================================================================

class TestDataClasses:
    """Test data class functionality."""

    @requires_pqc
    def test_mldsa_keypair_immutable_access(self, mldsa_keypair):
        """Test MLDSAKeyPair fields are accessible."""
        assert mldsa_keypair.algorithm is not None
        assert mldsa_keypair.public_key is not None
        assert mldsa_keypair.private_key is not None

    @requires_pqc
    def test_hybrid_keypair_immutable_access(self, hybrid_keypair):
        """Test HybridKeyPair fields are accessible."""
        assert hybrid_keypair.algorithm is not None
        assert hybrid_keypair.ed25519_public_key is not None
        assert hybrid_keypair.ed25519_private_key is not None
        assert hybrid_keypair.mldsa_public_key is not None
        assert hybrid_keypair.mldsa_private_key is not None

    @requires_pqc
    def test_hybrid_signature_immutable_access(self, hybrid_keypair):
        """Test HybridSignature fields are accessible."""
        signature = sign_hybrid(
            hybrid_keypair.ed25519_private_key,
            hybrid_keypair.mldsa_private_key,
            b"test"
        )

        assert signature.algorithm is not None
        assert signature.ed25519_signature is not None
        assert signature.mldsa_signature is not None
        assert signature.timestamp is not None


# =============================================================================
# BENCHMARK TESTS (Optional)
# =============================================================================

class TestBenchmarks:
    """Performance benchmark tests."""

    @requires_pqc
    @pytest.mark.slow
    def test_key_generation_performance(self):
        """Benchmark key generation performance."""
        import time

        iterations = 10

        for alg in [Algorithm.MLDSA_44, Algorithm.MLDSA_65, Algorithm.MLDSA_87]:
            start = time.perf_counter()
            for _ in range(iterations):
                generate_mldsa_keypair(alg)
            elapsed = (time.perf_counter() - start) / iterations

            # Key generation should be < 50ms per operation
            assert elapsed < 0.05, f"{alg} key generation too slow: {elapsed:.3f}s"

    @requires_pqc
    @pytest.mark.slow
    def test_signing_performance(self, mldsa_keypair):
        """Benchmark signing performance."""
        import time

        message = b"Performance test message"
        iterations = 100

        start = time.perf_counter()
        for _ in range(iterations):
            sign_mldsa(mldsa_keypair.private_key, message, mldsa_keypair.algorithm)
        elapsed = (time.perf_counter() - start) / iterations

        # Signing should be < 1ms per operation
        assert elapsed < 0.001, f"Signing too slow: {elapsed*1000:.3f}ms"

    @requires_pqc
    @pytest.mark.slow
    def test_verification_performance(self, mldsa_keypair):
        """Benchmark verification performance."""
        import time

        message = b"Performance test message"
        signature = sign_mldsa(
            mldsa_keypair.private_key,
            message,
            mldsa_keypair.algorithm
        )

        iterations = 100

        start = time.perf_counter()
        for _ in range(iterations):
            verify_mldsa(
                mldsa_keypair.public_key,
                message,
                signature,
                mldsa_keypair.algorithm
            )
        elapsed = (time.perf_counter() - start) / iterations

        # Verification should be < 0.5ms per operation
        assert elapsed < 0.0005, f"Verification too slow: {elapsed*1000:.3f}ms"


# =============================================================================
# MOCKED TESTS (For CI without liboqs)
# =============================================================================

class TestMockedPQCUnavailable:
    """Test behavior when PQC is not available (for CI environments)."""

    def test_operations_raise_when_unavailable(self):
        """Test that operations raise PQCNotAvailableError when unavailable."""
        # Reset the cached availability check
        import aim_sdk.crypto.pqc as pqc_module
        original_available = pqc_module._pqc_available
        original_sig = pqc_module._liboqs_sig

        try:
            # Force unavailable state
            pqc_module._pqc_available = False
            pqc_module._liboqs_sig = None
            pqc_module._availability_error = "Mocked unavailable"

            with pytest.raises(PQCNotAvailableError):
                generate_mldsa_keypair()

            with pytest.raises(PQCNotAvailableError):
                generate_hybrid_keypair()

            with pytest.raises(PQCNotAvailableError):
                sign_mldsa(b"key", b"msg")

            with pytest.raises(PQCNotAvailableError):
                sign_hybrid(b"ed_key", b"mldsa_key", b"msg")

            with pytest.raises(PQCNotAvailableError):
                verify_mldsa(b"key", b"msg", b"sig")

            with pytest.raises(PQCNotAvailableError):
                verify_hybrid(b"ed_key", b"mldsa_key", b"msg", b"ed_sig", b"mldsa_sig")

        finally:
            # Restore original state
            pqc_module._pqc_available = original_available
            pqc_module._liboqs_sig = original_sig
