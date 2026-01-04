"""
Tests for key rotation module.
"""

import pytest
import tempfile
from pathlib import Path

from aim_sdk.crypto import (
    RotationPhase,
    RotationStatus,
    KeyRotationManager,
    KeyRotationState,
)


class TestKeyRotationManager:
    """Tests for KeyRotationManager."""

    def test_initialize(self):
        """Test initial key generation."""
        manager = KeyRotationManager(agent_id="test-agent")
        signer = manager.initialize()

        assert manager.state.current_version == 1
        assert len(manager.state.key_versions) == 1
        assert manager.state.phase == RotationPhase.NONE

        # Should be able to sign
        message = b"test message"
        sig, version = manager.sign(message)
        assert version == 1
        assert manager.verify(message, sig)

    def test_initialize_twice_fails(self):
        """Test that initializing twice raises error."""
        manager = KeyRotationManager(agent_id="test-agent")
        manager.initialize()

        with pytest.raises(RuntimeError, match="Already initialized"):
            manager.initialize()

    def test_get_current_signer(self):
        """Test getting current signer."""
        manager = KeyRotationManager(agent_id="test-agent")
        manager.initialize()

        signer = manager.get_current_signer()
        assert signer is not None
        assert signer.algorithm.value == "hybrid-ed25519-mldsa65"

    def test_prepare_rotation(self):
        """Test preparing for key rotation."""
        manager = KeyRotationManager(agent_id="test-agent")
        manager.initialize()

        rotation = manager.prepare_rotation(reason="Scheduled rotation")

        assert rotation.from_version == 1
        assert rotation.to_version == 2
        assert rotation.status == RotationStatus.PENDING
        assert manager.state.phase == RotationPhase.PREPARE
        assert len(manager.state.key_versions) == 2

    def test_begin_dual_signing(self):
        """Test beginning dual signing phase."""
        manager = KeyRotationManager(agent_id="test-agent")
        manager.initialize()
        manager.prepare_rotation()
        manager.begin_dual_signing()

        assert manager.state.phase == RotationPhase.DUAL_SIGN
        assert manager.state.pending_rotation.status == RotationStatus.IN_PROGRESS

        # Both versions should be valid
        valid = manager.get_valid_versions()
        assert 1 in valid
        assert 2 in valid

    def test_dual_signing(self):
        """Test signing with both keys during dual sign phase."""
        manager = KeyRotationManager(agent_id="test-agent")
        manager.initialize()
        manager.prepare_rotation()
        manager.begin_dual_signing()

        message = b"dual sign test"
        signatures = manager.sign_dual(message)

        assert len(signatures) == 2
        for sig, version in signatures:
            assert manager.verify(message, sig, version=version)

    def test_complete_rotation(self):
        """Test completing key rotation."""
        manager = KeyRotationManager(agent_id="test-agent")
        manager.initialize()
        manager.prepare_rotation()
        manager.begin_dual_signing()
        completed = manager.complete_rotation()

        assert completed.status == RotationStatus.COMPLETED
        assert manager.state.current_version == 2
        assert manager.state.phase == RotationPhase.NONE
        assert len(manager.state.rotation_history) == 1

    def test_rollback_rotation(self):
        """Test rolling back a rotation."""
        manager = KeyRotationManager(agent_id="test-agent")
        manager.initialize()
        manager.prepare_rotation()
        manager.begin_dual_signing()

        rolled_back = manager.rollback_rotation(reason="Test rollback")

        assert rolled_back.status == RotationStatus.ROLLED_BACK
        assert manager.state.current_version == 1
        assert manager.state.phase == RotationPhase.NONE
        assert len(manager.state.rotation_history) == 1

    def test_signature_validity_across_rotation(self):
        """Test that old signatures remain valid during grace period."""
        manager = KeyRotationManager(agent_id="test-agent", grace_period_days=30)
        manager.initialize()

        # Sign with version 1
        message = b"old signature"
        sig_v1, _ = manager.sign(message)

        # Rotate keys
        manager.prepare_rotation()
        manager.begin_dual_signing()
        manager.complete_rotation()

        # Old signature should still verify (grace period)
        assert manager.verify(message, sig_v1)

        # New signature should also work
        sig_v2, _ = manager.sign(message)
        assert manager.verify(message, sig_v2)

    def test_state_persistence(self):
        """Test state persistence to file."""
        with tempfile.TemporaryDirectory() as tmpdir:
            state_path = Path(tmpdir) / "rotation_state.json"

            # Create manager and initialize
            manager1 = KeyRotationManager(
                agent_id="test-agent",
                state_path=state_path,
            )
            manager1.initialize()
            manager1.prepare_rotation(reason="Test")

            # Sign a message
            message = b"persistence test"
            sig, version = manager1.sign(message)

            # Load in new manager
            manager2 = KeyRotationManager(
                agent_id="test-agent",
                state_path=state_path,
            )

            # State should be preserved
            assert manager2.state.current_version == 1
            assert manager2.state.phase == RotationPhase.PREPARE
            assert len(manager2.state.key_versions) == 2

            # Should be able to verify signature
            assert manager2.verify(message, sig)

    def test_get_public_keys(self):
        """Test getting public keys for valid versions."""
        manager = KeyRotationManager(agent_id="test-agent")
        manager.initialize()
        manager.prepare_rotation()
        manager.begin_dual_signing()

        public_keys = manager.get_public_keys()

        assert len(public_keys) == 2
        assert 1 in public_keys
        assert 2 in public_keys
        for version, keys in public_keys.items():
            assert "classical" in keys
            assert "pqc" in keys
            assert keys["classical"]  # Non-empty
            assert keys["pqc"]  # Non-empty

    def test_multiple_rotations(self):
        """Test multiple sequential rotations."""
        manager = KeyRotationManager(agent_id="test-agent")
        manager.initialize()

        # First rotation
        manager.prepare_rotation(reason="First rotation")
        manager.begin_dual_signing()
        manager.complete_rotation()
        assert manager.state.current_version == 2

        # Second rotation
        manager.prepare_rotation(reason="Second rotation")
        manager.begin_dual_signing()
        manager.complete_rotation()
        assert manager.state.current_version == 3

        # History should have both rotations
        assert len(manager.state.rotation_history) == 2

    def test_cannot_rotate_during_rotation(self):
        """Test that starting new rotation during one fails."""
        manager = KeyRotationManager(agent_id="test-agent")
        manager.initialize()
        manager.prepare_rotation()

        with pytest.raises(RuntimeError, match="already in phase"):
            manager.prepare_rotation()


class TestKeyRotationState:
    """Tests for KeyRotationState serialization."""

    def test_to_dict_from_dict(self):
        """Test roundtrip serialization."""
        manager = KeyRotationManager(agent_id="test-agent")
        manager.initialize()
        manager.prepare_rotation(reason="Test")

        # Convert to dict
        state_dict = manager.state.to_dict()

        # Recreate from dict
        loaded = KeyRotationState.from_dict(state_dict)

        assert loaded.agent_id == manager.state.agent_id
        assert loaded.current_version == manager.state.current_version
        assert loaded.phase == manager.state.phase
        assert len(loaded.key_versions) == len(manager.state.key_versions)


if __name__ == "__main__":
    pytest.main([__file__, "-v"])
