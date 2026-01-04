"""
Tests for the hybrid cryptography integration module.
"""

import pytest
import base64

from aim_sdk.crypto import (
    HybridCredentials,
    HybridKeyGenerator,
    generate_hybrid_credentials,
    upgrade_credentials_to_hybrid,
    is_hybrid_credentials,
    HybridSigner,
)


class TestHybridKeyGenerator:
    """Tests for HybridKeyGenerator."""

    def test_generate(self):
        """Test generating hybrid credentials."""
        generator = HybridKeyGenerator()
        credentials = generator.generate()

        assert isinstance(credentials, HybridCredentials)
        assert credentials.is_hybrid is True
        assert credentials.algorithm == "hybrid-ed25519-mldsa65"

    def test_classical_keys_present(self):
        """Test that classical Ed25519 keys are generated."""
        generator = HybridKeyGenerator()
        credentials = generator.generate()

        # Ed25519 public key should be 32 bytes
        public_bytes = credentials.ed25519_public_key_bytes
        assert len(public_bytes) == 32

        # Ed25519 private key (seed) should be 32 bytes
        private_bytes = credentials.ed25519_private_key_bytes
        assert len(private_bytes) == 32

    def test_pqc_keys_present(self):
        """Test that ML-DSA-65 keys are generated."""
        generator = HybridKeyGenerator()
        credentials = generator.generate()

        # ML-DSA-65 public key should be 1952 bytes
        pqc_public = base64.b64decode(credentials.pqc_public_key)
        assert len(pqc_public) == 1952

        # ML-DSA-65 private key should be 4032 bytes
        pqc_private = base64.b64decode(credentials.pqc_private_key)
        assert len(pqc_private) == 4032

    def test_go_compatible_private_key(self):
        """Test that private key is Go-compatible (64 bytes)."""
        generator = HybridKeyGenerator()
        credentials = generator.generate()

        # Full private key should be 64 bytes (seed + public)
        full_key = credentials.ed25519_private_key_full
        assert len(full_key) == 64

        # First 32 bytes should be seed, last 32 should be public key
        assert full_key[32:] == credentials.ed25519_public_key_bytes


class TestHybridCredentials:
    """Tests for HybridCredentials."""

    def test_to_dict_from_dict(self):
        """Test roundtrip serialization."""
        original = generate_hybrid_credentials()
        data = original.to_dict()

        # Verify structure
        assert "public_key" in data
        assert "private_key" in data
        assert "hybrid" in data
        assert data["is_hybrid"] is True

        # Roundtrip
        restored = HybridCredentials.from_dict(data)

        assert restored.classical_public_key == original.classical_public_key
        assert restored.pqc_public_key == original.pqc_public_key
        assert restored.algorithm == original.algorithm

    def test_to_dict_backend_compatible(self):
        """Test that to_dict produces backend-compatible format."""
        credentials = generate_hybrid_credentials()
        data = credentials.to_dict()

        # public_key should be base64 Ed25519 public key
        public_bytes = base64.b64decode(data["public_key"])
        assert len(public_bytes) == 32

        # private_key should be 64-byte Go format
        private_bytes = base64.b64decode(data["private_key"])
        assert len(private_bytes) == 64

    def test_get_signer(self):
        """Test getting a signer from credentials."""
        credentials = generate_hybrid_credentials()
        signer = credentials.get_signer()

        assert isinstance(signer, HybridSigner)

        # Test signing
        message = b"test message"
        signature = signer.sign(message)
        assert signature != ""

        # Test verification
        assert signer.verify(message, signature) is True


class TestGenerateHybridCredentials:
    """Tests for generate_hybrid_credentials function."""

    def test_convenience_function(self):
        """Test the convenience function works."""
        credentials = generate_hybrid_credentials()

        assert isinstance(credentials, HybridCredentials)
        assert credentials.is_hybrid is True


class TestUpgradeCredentials:
    """Tests for upgrading Ed25519 credentials to hybrid."""

    def test_upgrade_from_32_byte_key(self):
        """Test upgrading credentials with 32-byte private key."""
        # Simulate existing Ed25519 credentials (32-byte seed)
        from nacl.signing import SigningKey
        signing_key = SigningKey.generate()
        seed = bytes(signing_key)
        public = bytes(signing_key.verify_key)

        existing = {
            "public_key": base64.b64encode(public).decode('utf-8'),
            "private_key": base64.b64encode(seed).decode('utf-8'),
        }

        upgraded = upgrade_credentials_to_hybrid(existing)

        # Classical keys should match original
        assert upgraded.ed25519_public_key_bytes == public

        # Should have PQC keys now
        assert upgraded.pqc_public_key != ""
        assert upgraded.pqc_private_key != ""

    def test_upgrade_from_64_byte_key(self):
        """Test upgrading credentials with 64-byte Go-format private key."""
        # Simulate existing Ed25519 credentials (64-byte Go format)
        from nacl.signing import SigningKey
        signing_key = SigningKey.generate()
        seed = bytes(signing_key)
        public = bytes(signing_key.verify_key)
        full_private = seed + public

        existing = {
            "public_key": base64.b64encode(public).decode('utf-8'),
            "private_key": base64.b64encode(full_private).decode('utf-8'),
        }

        upgraded = upgrade_credentials_to_hybrid(existing)

        # Classical keys should match original
        assert upgraded.ed25519_public_key_bytes == public

        # Should have PQC keys now
        pqc_public = base64.b64decode(upgraded.pqc_public_key)
        assert len(pqc_public) == 1952  # ML-DSA-65 public key size


class TestIsHybridCredentials:
    """Tests for is_hybrid_credentials function."""

    def test_detect_hybrid(self):
        """Test detecting hybrid credentials."""
        credentials = generate_hybrid_credentials()
        data = credentials.to_dict()

        assert is_hybrid_credentials(data) is True

    def test_detect_non_hybrid(self):
        """Test detecting non-hybrid credentials."""
        non_hybrid = {
            "public_key": "some_key",
            "private_key": "some_private",
        }

        assert is_hybrid_credentials(non_hybrid) is False

    def test_detect_by_is_hybrid_flag(self):
        """Test detection by is_hybrid flag."""
        with_flag = {"is_hybrid": True}
        assert is_hybrid_credentials(with_flag) is True

    def test_detect_by_hybrid_section(self):
        """Test detection by hybrid section."""
        with_section = {"hybrid": {"algorithm": "hybrid-ed25519-mldsa65"}}
        assert is_hybrid_credentials(with_section) is True


class TestSigningCompatibility:
    """Tests for signing compatibility with existing system."""

    def test_classical_signature_compatible(self):
        """Test that classical component can be verified independently."""
        from nacl.signing import VerifyKey
        from nacl.encoding import RawEncoder

        credentials = generate_hybrid_credentials()
        signer = credentials.get_signer()

        # Sign a message
        message = b"compatibility test"
        signature = signer.sign(message)

        # The hybrid signature contains the Ed25519 component
        # Verify with standard nacl
        verify_key = VerifyKey(credentials.ed25519_public_key_bytes, encoder=RawEncoder)

        # Get the classical signature from the hybrid
        sig_bytes = base64.b64decode(signature)
        # Format: [length:4][classical:64][pqc:3309]
        classical_len = int.from_bytes(sig_bytes[:4], 'big')
        classical_sig = sig_bytes[4:4+classical_len]

        # Verify with nacl
        try:
            verify_key.verify(message, classical_sig)
            verified = True
        except Exception:
            verified = False

        assert verified is True


if __name__ == "__main__":
    pytest.main([__file__, "-v"])
