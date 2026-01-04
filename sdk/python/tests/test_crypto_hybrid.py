"""
Tests for the hybrid cryptographic signing module.
"""

import pytest
import base64

from aim_sdk.crypto import (
    Algorithm,
    KeyPair,
    HybridKeyPair,
    HybridSignature,
    encode_hybrid_signature,
    decode_hybrid_signature,
    Ed25519Signer,
    Ed25519Verifier,
    MLDSASigner,
    MLDSAVerifier,
    HybridSigner,
    HybridVerifier,
)


class TestEd25519Signer:
    """Tests for Ed25519 signing."""

    def test_generate(self):
        """Test key generation."""
        signer = Ed25519Signer.generate()
        assert signer.algorithm == Algorithm.ED25519
        assert len(signer.public_key_bytes()) == 32
        assert signer.public_key() != ""

    def test_sign_and_verify(self):
        """Test signing and verification."""
        signer = Ed25519Signer.generate()
        message = b"test message for Ed25519"

        signature = signer.sign(message)
        assert signature != ""

        # Verify with same signer
        assert signer.verify(message, signature) is True

    def test_sign_and_verify_bytes(self):
        """Test signing and verification with raw bytes."""
        signer = Ed25519Signer.generate()
        message = b"another test message"

        signature = signer.sign_bytes(message)
        assert len(signature) == 64

        assert signer.verify_bytes(message, signature) is True

    def test_tampered_message_fails(self):
        """Test that tampered messages fail verification."""
        signer = Ed25519Signer.generate()
        message = b"original message"
        tampered = b"tampered message"

        signature = signer.sign_bytes(message)
        assert signer.verify_bytes(tampered, signature) is False

    def test_from_keys(self):
        """Test creating signer from existing keys."""
        original = Ed25519Signer.generate()
        kp = original.key_pair()

        restored = Ed25519Signer.from_keys(kp.public_key, kp.private_key)
        assert restored.public_key() == original.public_key()

        # Sign with original, verify with restored
        message = b"cross-verify message"
        sig = original.sign(message)
        assert restored.verify(message, sig) is True

    def test_verifier_from_public_key(self):
        """Test creating verifier from public key only."""
        signer = Ed25519Signer.generate()
        verifier = Ed25519Verifier.from_public_key(signer.public_key())

        message = b"verifier test message"
        sig = signer.sign(message)

        assert verifier.verify(message, sig) is True
        assert verifier.algorithm == Algorithm.ED25519


class TestMLDSASigner:
    """Tests for ML-DSA-65 signing (simulated mode)."""

    def test_generate(self):
        """Test key generation."""
        signer = MLDSASigner.generate()
        assert signer.algorithm == Algorithm.MLDSA65
        assert signer.is_simulated is True
        assert len(signer.public_key_bytes()) == 1952

    def test_sign_and_verify(self):
        """Test signing and verification."""
        signer = MLDSASigner.generate()
        message = b"test message for ML-DSA"

        signature = signer.sign(message)
        assert signature != ""

        # In simulated mode, verification works with same signer
        assert signer.verify(message, signature) is True

    def test_signature_size(self):
        """Test that signature has correct size."""
        signer = MLDSASigner.generate()
        message = b"size test"

        sig = signer.sign_bytes(message)
        assert len(sig) == 3309  # ML-DSA-65 signature size

    def test_from_keys(self):
        """Test creating signer from existing keys."""
        original = MLDSASigner.generate()
        kp = original.key_pair()

        restored = MLDSASigner.from_keys(kp.public_key, kp.private_key)
        assert restored.public_key() == original.public_key()


class TestHybridSigner:
    """Tests for hybrid Ed25519+ML-DSA-65 signing."""

    def test_generate(self):
        """Test hybrid key generation."""
        signer = HybridSigner.generate()
        assert signer.algorithm == Algorithm.HYBRID
        assert signer.is_simulated is True  # PQC component is simulated

    def test_key_pair(self):
        """Test hybrid key pair extraction."""
        signer = HybridSigner.generate()
        kp = signer.key_pair()

        assert kp.algorithm == Algorithm.HYBRID
        assert kp.classical_public != ""
        assert kp.classical_private != ""
        assert kp.pqc_public != ""
        assert kp.pqc_private != ""

    def test_public_keys(self):
        """Test individual public key access."""
        signer = HybridSigner.generate()

        # Classical (Ed25519) public key should be 32 bytes
        classical_bytes = base64.b64decode(signer.classical_public_key())
        assert len(classical_bytes) == 32

        # PQC (ML-DSA-65) public key should be 1952 bytes
        pqc_bytes = base64.b64decode(signer.pqc_public_key())
        assert len(pqc_bytes) == 1952

        # Combined public key should be the concatenation
        combined_bytes = signer.public_key_bytes()
        assert len(combined_bytes) == 32 + 1952
        assert combined_bytes == classical_bytes + pqc_bytes

    def test_sign_and_verify(self):
        """Test hybrid signing and verification."""
        signer = HybridSigner.generate()
        message = b"test message for hybrid signing"

        signature = signer.sign(message)
        assert signature != ""

        assert signer.verify(message, signature) is True

    def test_sign_and_verify_bytes(self):
        """Test hybrid signing with raw bytes."""
        signer = HybridSigner.generate()
        message = b"another hybrid test"

        sig = signer.sign_bytes(message)

        # Size should be: 4 (length) + 64 (Ed25519) + 3309 (ML-DSA)
        expected_size = 4 + 64 + 3309
        assert len(sig) == expected_size

        assert signer.verify_bytes(message, sig) is True

    def test_tampered_message_fails(self):
        """Test that tampered messages fail verification."""
        signer = HybridSigner.generate()
        message = b"original message"
        tampered = b"tampered message"

        sig = signer.sign_bytes(message)
        assert signer.verify_bytes(tampered, sig) is False

    def test_from_key_pair(self):
        """Test creating signer from existing key pair."""
        original = HybridSigner.generate()
        kp = original.key_pair()

        restored = HybridSigner.from_key_pair(kp)
        assert restored.public_key() == original.public_key()

        # Sign with original, verify with restored
        message = b"cross-verify hybrid message"
        sig = original.sign(message)
        assert restored.verify(message, sig) is True


class TestHybridVerifier:
    """Tests for hybrid signature verification without private keys."""

    def test_from_public_keys(self):
        """Test creating verifier from public keys."""
        signer = HybridSigner.generate()
        verifier = HybridVerifier.from_public_keys(
            signer.classical_public_key(),
            signer.pqc_public_key(),
        )

        assert verifier.algorithm == Algorithm.HYBRID

    def test_verify_classical_component(self):
        """Test that classical (Ed25519) verification works with public key only."""
        signer = HybridSigner.generate()
        verifier = HybridVerifier.from_public_keys(
            signer.classical_public_key(),
            signer.pqc_public_key(),
        )

        message = b"verifier test message"
        sig = signer.sign(message)

        info = verifier.verify_with_info(message, sig)
        # Classical verification should work with public key only
        assert info.classical_valid is True
        # PQC verification requires private key in simulated mode
        # (This is a known limitation of the placeholder implementation)
        # In production with liboqs, pqc_valid would also be True
        assert info.pqc_valid is False  # Expected in simulated mode

    def test_verify_with_info_structure(self):
        """Test verification info structure."""
        signer = HybridSigner.generate()
        verifier = HybridVerifier.from_public_keys(
            signer.classical_public_key(),
            signer.pqc_public_key(),
        )

        message = b"info verification test"
        sig = signer.sign(message)

        info = verifier.verify_with_info(message, sig)
        assert info.algorithm == Algorithm.HYBRID
        assert info.classical_valid is True
        assert info.classical_algorithm == "ed25519"
        assert info.pqc_algorithm == "mldsa65"
        assert info.signature_size == 4 + 64 + 3309

    def test_full_verification_with_signer(self):
        """Test that full verification works when signer is used as verifier."""
        # When we have access to the private key (via signer), both
        # components should verify correctly
        signer = HybridSigner.generate()
        message = b"full verification test"
        sig = signer.sign(message)

        # Using signer's verify (which has private key) should pass
        assert signer.verify(message, sig) is True


class TestHybridSignatureEncoding:
    """Tests for hybrid signature encoding/decoding."""

    def test_encode_decode(self):
        """Test roundtrip encoding."""
        original = HybridSignature(
            classical=b"\x42" * 64,
            pqc=b"\x43" * 3309,
        )

        encoded = encode_hybrid_signature(original)
        expected_size = 4 + 64 + 3309
        assert len(encoded) == expected_size

        decoded = decode_hybrid_signature(encoded)
        assert decoded.classical == original.classical
        assert decoded.pqc == original.pqc

    def test_decode_too_short(self):
        """Test decoding too-short data."""
        with pytest.raises(ValueError, match="too short"):
            decode_hybrid_signature(b"\x00\x01")

    def test_decode_truncated(self):
        """Test decoding truncated data."""
        # Claims 64 bytes of classical sig but only has 4 bytes total
        data = b"\x00\x00\x00\x40"  # Length = 64 in big-endian
        with pytest.raises(ValueError, match="truncated"):
            decode_hybrid_signature(data)

    def test_variable_size_classical(self):
        """Test encoding with different classical signature sizes."""
        # This shouldn't happen in practice, but test the encoding handles it
        sig = HybridSignature(
            classical=b"\x00" * 32,  # Shorter than usual
            pqc=b"\x01" * 100,
        )

        encoded = encode_hybrid_signature(sig)
        decoded = decode_hybrid_signature(encoded)

        assert decoded.classical == sig.classical
        assert decoded.pqc == sig.pqc


class TestCrossCompatibility:
    """Tests for compatibility between Python and Go implementations."""

    def test_signature_format_matches_go(self):
        """Verify signature format matches Go implementation."""
        # The format is: [classical_len:4][classical_sig][pqc_sig]
        # with classical_len as big-endian uint32

        sig = HybridSignature(
            classical=b"\xAA" * 64,
            pqc=b"\xBB" * 3309,
        )

        encoded = encode_hybrid_signature(sig)

        # First 4 bytes should be length of classical sig (64)
        length_bytes = encoded[:4]
        assert length_bytes == b"\x00\x00\x00\x40"  # 64 in big-endian

        # Next 64 bytes should be classical signature
        assert encoded[4:68] == sig.classical

        # Remaining bytes should be PQC signature
        assert encoded[68:] == sig.pqc


if __name__ == "__main__":
    pytest.main([__file__, "-v"])
