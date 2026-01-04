"""
Key rotation support for hybrid cryptographic keys.

This module provides secure key rotation capabilities for transitioning
from classical to hybrid (post-quantum) cryptography. It supports:

1. Scheduled rotation: Plan and execute key rotations
2. Grace periods: Keep old keys valid during transition
3. Key versioning: Track key generations for audit
4. Rollback: Revert to previous keys if issues arise

Key rotation follows a three-phase approach:
- Phase 1 (Prepare): Generate new keys, announce upcoming rotation
- Phase 2 (Dual-Sign): Sign with both old and new keys
- Phase 3 (Complete): Deprecate old keys after grace period
"""

import base64
import json
import time
from dataclasses import dataclass, field, asdict
from datetime import datetime, timedelta, timezone
from enum import Enum
from pathlib import Path
from typing import Optional, List, Dict, Any, Tuple

from .signer import Algorithm, HybridKeyPair
from .hybrid import HybridSigner


class RotationPhase(str, Enum):
    """Current phase of key rotation."""

    NONE = "none"  # No rotation in progress
    PREPARE = "prepare"  # New keys generated, not yet active
    DUAL_SIGN = "dual_sign"  # Signing with both old and new keys
    COMPLETE = "complete"  # Rotation complete, old keys deprecated


class RotationStatus(str, Enum):
    """Status of a key rotation operation."""

    PENDING = "pending"  # Rotation scheduled but not started
    IN_PROGRESS = "in_progress"  # Rotation actively underway
    COMPLETED = "completed"  # Rotation finished successfully
    ROLLED_BACK = "rolled_back"  # Rotation was reverted
    FAILED = "failed"  # Rotation failed


@dataclass
class KeyVersion:
    """A versioned key pair with metadata."""

    version: int
    algorithm: Algorithm
    key_pair: HybridKeyPair
    created_at: str  # ISO format
    expires_at: Optional[str] = None  # ISO format, None = never
    deprecated_at: Optional[str] = None  # ISO format
    status: str = "active"  # active, deprecated, revoked

    def to_dict(self) -> Dict[str, Any]:
        """Convert to dictionary for serialization."""
        return {
            "version": self.version,
            "algorithm": self.algorithm.value,
            "key_pair": {
                "algorithm": self.key_pair.algorithm.value,
                "classical_public": self.key_pair.classical_public,
                "classical_private": self.key_pair.classical_private,
                "pqc_public": self.key_pair.pqc_public,
                "pqc_private": self.key_pair.pqc_private,
            },
            "created_at": self.created_at,
            "expires_at": self.expires_at,
            "deprecated_at": self.deprecated_at,
            "status": self.status,
        }

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "KeyVersion":
        """Create from dictionary."""
        kp_data = data["key_pair"]
        key_pair = HybridKeyPair(
            algorithm=Algorithm(kp_data["algorithm"]),
            classical_public=kp_data["classical_public"],
            classical_private=kp_data["classical_private"],
            pqc_public=kp_data["pqc_public"],
            pqc_private=kp_data["pqc_private"],
        )
        return cls(
            version=data["version"],
            algorithm=Algorithm(data["algorithm"]),
            key_pair=key_pair,
            created_at=data["created_at"],
            expires_at=data.get("expires_at"),
            deprecated_at=data.get("deprecated_at"),
            status=data.get("status", "active"),
        )


@dataclass
class RotationRecord:
    """Record of a key rotation event."""

    rotation_id: str
    from_version: int
    to_version: int
    initiated_at: str
    completed_at: Optional[str] = None
    status: RotationStatus = RotationStatus.PENDING
    reason: str = ""
    rollback_version: Optional[int] = None

    def to_dict(self) -> Dict[str, Any]:
        """Convert to dictionary."""
        return {
            "rotation_id": self.rotation_id,
            "from_version": self.from_version,
            "to_version": self.to_version,
            "initiated_at": self.initiated_at,
            "completed_at": self.completed_at,
            "status": self.status.value,
            "reason": self.reason,
            "rollback_version": self.rollback_version,
        }

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "RotationRecord":
        """Create from dictionary."""
        return cls(
            rotation_id=data["rotation_id"],
            from_version=data["from_version"],
            to_version=data["to_version"],
            initiated_at=data["initiated_at"],
            completed_at=data.get("completed_at"),
            status=RotationStatus(data.get("status", "pending")),
            reason=data.get("reason", ""),
            rollback_version=data.get("rollback_version"),
        )


@dataclass
class KeyRotationState:
    """Complete state of key rotation for an agent."""

    agent_id: str
    current_version: int
    phase: RotationPhase
    key_versions: List[KeyVersion] = field(default_factory=list)
    rotation_history: List[RotationRecord] = field(default_factory=list)
    grace_period_days: int = 30  # How long old keys remain valid
    pending_rotation: Optional[RotationRecord] = None

    def to_dict(self) -> Dict[str, Any]:
        """Convert to dictionary for storage."""
        return {
            "agent_id": self.agent_id,
            "current_version": self.current_version,
            "phase": self.phase.value,
            "key_versions": [kv.to_dict() for kv in self.key_versions],
            "rotation_history": [r.to_dict() for r in self.rotation_history],
            "grace_period_days": self.grace_period_days,
            "pending_rotation": self.pending_rotation.to_dict() if self.pending_rotation else None,
        }

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "KeyRotationState":
        """Create from dictionary."""
        return cls(
            agent_id=data["agent_id"],
            current_version=data["current_version"],
            phase=RotationPhase(data.get("phase", "none")),
            key_versions=[KeyVersion.from_dict(kv) for kv in data.get("key_versions", [])],
            rotation_history=[RotationRecord.from_dict(r) for r in data.get("rotation_history", [])],
            grace_period_days=data.get("grace_period_days", 30),
            pending_rotation=RotationRecord.from_dict(data["pending_rotation"]) if data.get("pending_rotation") else None,
        )

    def save(self, path: Path) -> None:
        """Save state to file."""
        with open(path, "w") as f:
            json.dump(self.to_dict(), f, indent=2)

    @classmethod
    def load(cls, path: Path) -> "KeyRotationState":
        """Load state from file."""
        with open(path, "r") as f:
            return cls.from_dict(json.load(f))


class KeyRotationManager:
    """
    Manages key rotation for hybrid cryptographic keys.

    Usage:
        manager = KeyRotationManager(agent_id="my-agent")

        # Initialize with first key
        signer = manager.initialize()

        # Later, rotate keys
        rotation = manager.prepare_rotation(reason="Scheduled rotation")
        manager.begin_dual_signing()
        # ... wait for grace period ...
        manager.complete_rotation()

        # Sign with current active key
        sig = manager.sign(message)

        # Verify with any valid key (current or grace period)
        valid = manager.verify(message, signature)
    """

    def __init__(
        self,
        agent_id: str,
        state_path: Optional[Path] = None,
        grace_period_days: int = 30,
    ):
        """
        Initialize key rotation manager.

        Args:
            agent_id: Unique identifier for the agent
            state_path: Path to persist rotation state
            grace_period_days: How long old keys remain valid after rotation
        """
        self.agent_id = agent_id
        self.state_path = state_path
        self.grace_period_days = grace_period_days

        # Load or create state
        if state_path and state_path.exists():
            self.state = KeyRotationState.load(state_path)
        else:
            self.state = KeyRotationState(
                agent_id=agent_id,
                current_version=0,
                phase=RotationPhase.NONE,
                grace_period_days=grace_period_days,
            )

        # Cache signers for active keys
        self._signers: Dict[int, HybridSigner] = {}

    def _save_state(self) -> None:
        """Persist state to file if path configured."""
        if self.state_path:
            self.state.save(self.state_path)

    def _get_signer(self, version: int) -> Optional[HybridSigner]:
        """Get or create signer for a key version."""
        if version in self._signers:
            return self._signers[version]

        kv = self._get_key_version(version)
        if kv is None:
            return None

        signer = HybridSigner.from_key_pair(kv.key_pair)
        self._signers[version] = signer
        return signer

    def _get_key_version(self, version: int) -> Optional[KeyVersion]:
        """Get key version by version number."""
        for kv in self.state.key_versions:
            if kv.version == version:
                return kv
        return None

    def _generate_rotation_id(self) -> str:
        """Generate unique rotation ID."""
        import hashlib
        data = f"{self.agent_id}:{time.time()}:{len(self.state.rotation_history)}"
        return hashlib.sha256(data.encode()).hexdigest()[:16]

    def initialize(self) -> HybridSigner:
        """
        Initialize with first hybrid key pair.

        Returns:
            HybridSigner for the initial key

        Raises:
            RuntimeError: If already initialized
        """
        if self.state.key_versions:
            raise RuntimeError("Already initialized - use get_current_signer() instead")

        # Generate initial key pair
        signer = HybridSigner.generate()
        key_pair = signer.key_pair()

        # Create version 1
        now = datetime.now(timezone.utc).isoformat().replace("+00:00", "Z")
        kv = KeyVersion(
            version=1,
            algorithm=Algorithm.HYBRID,
            key_pair=key_pair,
            created_at=now,
        )

        self.state.key_versions.append(kv)
        self.state.current_version = 1
        self._signers[1] = signer
        self._save_state()

        return signer

    def get_current_signer(self) -> HybridSigner:
        """Get signer for the current active key version."""
        if not self.state.key_versions:
            raise RuntimeError("Not initialized - call initialize() first")

        signer = self._get_signer(self.state.current_version)
        if signer is None:
            raise RuntimeError(f"Current version {self.state.current_version} not found")
        return signer

    def prepare_rotation(self, reason: str = "") -> RotationRecord:
        """
        Prepare for key rotation by generating new keys.

        This is Phase 1 of rotation. New keys are generated but not yet active.

        Args:
            reason: Reason for rotation (for audit trail)

        Returns:
            RotationRecord tracking this rotation

        Raises:
            RuntimeError: If rotation already in progress
        """
        if self.state.phase != RotationPhase.NONE:
            raise RuntimeError(f"Rotation already in phase: {self.state.phase}")

        # Generate new key pair
        signer = HybridSigner.generate()
        key_pair = signer.key_pair()

        # Create new version
        new_version = self.state.current_version + 1
        now = datetime.now(timezone.utc).isoformat().replace("+00:00", "Z")

        kv = KeyVersion(
            version=new_version,
            algorithm=Algorithm.HYBRID,
            key_pair=key_pair,
            created_at=now,
            status="pending",  # Not yet active
        )

        # Create rotation record
        rotation = RotationRecord(
            rotation_id=self._generate_rotation_id(),
            from_version=self.state.current_version,
            to_version=new_version,
            initiated_at=now,
            status=RotationStatus.PENDING,
            reason=reason,
        )

        self.state.key_versions.append(kv)
        self.state.pending_rotation = rotation
        self.state.phase = RotationPhase.PREPARE
        self._signers[new_version] = signer
        self._save_state()

        return rotation

    def begin_dual_signing(self) -> None:
        """
        Begin Phase 2: dual signing with both old and new keys.

        During this phase, sign() produces signatures from both keys,
        and verify() accepts signatures from either.
        """
        if self.state.phase != RotationPhase.PREPARE:
            raise RuntimeError(f"Cannot begin dual signing from phase: {self.state.phase}")

        if self.state.pending_rotation is None:
            raise RuntimeError("No pending rotation")

        # Activate new key version
        new_version = self.state.pending_rotation.to_version
        for kv in self.state.key_versions:
            if kv.version == new_version:
                kv.status = "active"
                break

        self.state.pending_rotation.status = RotationStatus.IN_PROGRESS
        self.state.phase = RotationPhase.DUAL_SIGN
        self._save_state()

    def complete_rotation(self) -> RotationRecord:
        """
        Complete Phase 3: deprecate old key and make new key primary.

        Returns:
            Completed RotationRecord
        """
        if self.state.phase != RotationPhase.DUAL_SIGN:
            raise RuntimeError(f"Cannot complete rotation from phase: {self.state.phase}")

        if self.state.pending_rotation is None:
            raise RuntimeError("No pending rotation")

        now = datetime.now(timezone.utc).isoformat().replace("+00:00", "Z")
        old_version = self.state.pending_rotation.from_version
        new_version = self.state.pending_rotation.to_version

        # Deprecate old key (still valid during grace period)
        grace_expires = (datetime.now(timezone.utc) + timedelta(days=self.grace_period_days)).isoformat().replace("+00:00", "Z")
        for kv in self.state.key_versions:
            if kv.version == old_version:
                kv.status = "deprecated"
                kv.deprecated_at = now
                kv.expires_at = grace_expires
                break

        # Update rotation record
        self.state.pending_rotation.status = RotationStatus.COMPLETED
        self.state.pending_rotation.completed_at = now

        # Move to history
        self.state.rotation_history.append(self.state.pending_rotation)
        completed = self.state.pending_rotation

        # Update state
        self.state.current_version = new_version
        self.state.pending_rotation = None
        self.state.phase = RotationPhase.NONE
        self._save_state()

        return completed

    def rollback_rotation(self, reason: str = "") -> RotationRecord:
        """
        Rollback a rotation in progress.

        Args:
            reason: Reason for rollback

        Returns:
            Updated RotationRecord
        """
        if self.state.phase == RotationPhase.NONE:
            raise RuntimeError("No rotation in progress to rollback")

        if self.state.pending_rotation is None:
            raise RuntimeError("No pending rotation")

        now = datetime.now(timezone.utc).isoformat().replace("+00:00", "Z")
        new_version = self.state.pending_rotation.to_version

        # Revoke new key
        for kv in self.state.key_versions:
            if kv.version == new_version:
                kv.status = "revoked"
                break

        # Remove from cache
        self._signers.pop(new_version, None)

        # Update rotation record
        self.state.pending_rotation.status = RotationStatus.ROLLED_BACK
        self.state.pending_rotation.completed_at = now
        self.state.pending_rotation.reason += f" [ROLLBACK: {reason}]"
        self.state.pending_rotation.rollback_version = self.state.current_version

        # Move to history
        self.state.rotation_history.append(self.state.pending_rotation)
        rolled_back = self.state.pending_rotation

        # Reset state
        self.state.pending_rotation = None
        self.state.phase = RotationPhase.NONE
        self._save_state()

        return rolled_back

    def get_valid_versions(self) -> List[int]:
        """Get list of key versions that are currently valid for verification."""
        now = datetime.now(timezone.utc)
        valid = []

        for kv in self.state.key_versions:
            if kv.status == "revoked":
                continue

            if kv.status == "deprecated" and kv.expires_at:
                expires = datetime.fromisoformat(kv.expires_at.rstrip("Z")).replace(tzinfo=timezone.utc)
                if now > expires:
                    continue

            if kv.status in ("active", "pending", "deprecated"):
                valid.append(kv.version)

        return valid

    def sign(self, message: bytes) -> Tuple[str, int]:
        """
        Sign a message with the current key.

        Args:
            message: Message to sign

        Returns:
            Tuple of (base64 signature, key version used)
        """
        signer = self.get_current_signer()
        signature = signer.sign(message)
        return signature, self.state.current_version

    def sign_dual(self, message: bytes) -> List[Tuple[str, int]]:
        """
        Sign a message with all active keys (for dual-signing phase).

        Args:
            message: Message to sign

        Returns:
            List of (signature, version) tuples
        """
        results = []

        for version in self.get_valid_versions():
            signer = self._get_signer(version)
            if signer:
                sig = signer.sign(message)
                results.append((sig, version))

        return results

    def verify(self, message: bytes, signature: str, version: Optional[int] = None) -> bool:
        """
        Verify a signature, optionally specifying which key version.

        Args:
            message: Original message
            signature: Base64-encoded signature
            version: Key version to use (None = try all valid versions)

        Returns:
            True if signature is valid
        """
        if version is not None:
            signer = self._get_signer(version)
            if signer is None:
                return False
            return signer.verify(message, signature)

        # Try all valid versions
        for ver in self.get_valid_versions():
            signer = self._get_signer(ver)
            if signer and signer.verify(message, signature):
                return True

        return False

    def get_public_keys(self) -> Dict[int, Dict[str, str]]:
        """
        Get public keys for all valid versions.

        Returns:
            Dict mapping version to {classical, pqc} public keys
        """
        result = {}
        for version in self.get_valid_versions():
            kv = self._get_key_version(version)
            if kv:
                result[version] = {
                    "classical": kv.key_pair.classical_public,
                    "pqc": kv.key_pair.pqc_public,
                }
        return result
