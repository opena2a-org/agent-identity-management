"""Execution Isolation attestation types for AIM SDK."""

from enum import Enum
from typing import Optional


class SandboxType(str, Enum):
    NONE = "none"
    DOCKER = "docker"
    VM = "vm"
    GVISOR = "gvisor"
    FIRECRACKER = "firecracker"
    WASM = "wasm"
    KATA = "kata"


class NetworkIsolation(str, Enum):
    NONE = "none"
    FIREWALL = "firewall"
    NAMESPACE = "namespace"
    VPC = "vpc"
    AIRGAP = "airgap"


class FilesystemIsolation(str, Enum):
    NONE = "none"
    CHROOT = "chroot"
    TMPFS = "tmpfs"
    READONLY = "readonly"
    OVERLAY = "overlay"


class ProcessIsolation(str, Enum):
    NONE = "none"
    PIDNS = "pidns"
    SECCOMP = "seccomp"
    APPARMOR = "apparmor"
    SELINUX = "selinux"
    FULL = "full"


def score_isolation(
    sandbox: SandboxType = SandboxType.NONE,
    network: NetworkIsolation = NetworkIsolation.NONE,
    filesystem: FilesystemIsolation = FilesystemIsolation.NONE,
    process: ProcessIsolation = ProcessIsolation.NONE,
) -> float:
    """Compute an isolation score from 0.0 to 1.0 based on posture."""
    score = 0.0

    sandbox_scores = {
        SandboxType.FIRECRACKER: 0.40,
        SandboxType.KATA: 0.38,
        SandboxType.VM: 0.35,
        SandboxType.GVISOR: 0.32,
        SandboxType.WASM: 0.28,
        SandboxType.DOCKER: 0.20,
        SandboxType.NONE: 0.0,
    }
    score += sandbox_scores.get(sandbox, 0.0)

    network_scores = {
        NetworkIsolation.AIRGAP: 0.25,
        NetworkIsolation.VPC: 0.20,
        NetworkIsolation.NAMESPACE: 0.15,
        NetworkIsolation.FIREWALL: 0.10,
        NetworkIsolation.NONE: 0.0,
    }
    score += network_scores.get(network, 0.0)

    filesystem_scores = {
        FilesystemIsolation.READONLY: 0.20,
        FilesystemIsolation.OVERLAY: 0.16,
        FilesystemIsolation.TMPFS: 0.12,
        FilesystemIsolation.CHROOT: 0.08,
        FilesystemIsolation.NONE: 0.0,
    }
    score += filesystem_scores.get(filesystem, 0.0)

    process_scores = {
        ProcessIsolation.FULL: 0.15,
        ProcessIsolation.SELINUX: 0.12,
        ProcessIsolation.APPARMOR: 0.11,
        ProcessIsolation.SECCOMP: 0.10,
        ProcessIsolation.PIDNS: 0.06,
        ProcessIsolation.NONE: 0.0,
    }
    score += process_scores.get(process, 0.0)

    return min(score, 1.0)


def auto_detect_isolation() -> dict:
    """Auto-detect the current runtime isolation posture.

    Returns a dict with sandbox, network, filesystem, process keys.
    Best-effort detection; falls back to 'none' for unknown environments.
    """
    import os
    import platform

    result = {
        "sandbox": SandboxType.NONE,
        "network": NetworkIsolation.NONE,
        "filesystem": FilesystemIsolation.NONE,
        "process": ProcessIsolation.NONE,
    }

    # Detect Docker/container
    if os.path.exists("/.dockerenv") or os.path.exists("/run/.containerenv"):
        result["sandbox"] = SandboxType.DOCKER

    # Detect gVisor (runsc)
    try:
        with open("/proc/version", "r") as f:
            version = f.read()
            if "gvisor" in version.lower():
                result["sandbox"] = SandboxType.GVISOR
    except (FileNotFoundError, PermissionError):
        pass

    # Detect PID namespace (PID 1 in container)
    if os.getpid() == 1 or os.path.exists("/proc/1/ns/pid"):
        try:
            host_ns = os.readlink("/proc/1/ns/pid")
            self_ns = os.readlink(f"/proc/{os.getpid()}/ns/pid")
            if host_ns != self_ns:
                result["process"] = ProcessIsolation.PIDNS
        except (FileNotFoundError, PermissionError):
            pass

    # Detect read-only filesystem
    if os.path.exists("/proc/mounts"):
        try:
            with open("/proc/mounts", "r") as f:
                for line in f:
                    parts = line.split()
                    if len(parts) >= 4 and parts[1] == "/":
                        if "ro" in parts[3].split(","):
                            result["filesystem"] = FilesystemIsolation.READONLY
                        break
        except (FileNotFoundError, PermissionError):
            pass

    # Detect seccomp
    if os.path.exists("/proc/self/status"):
        try:
            with open("/proc/self/status", "r") as f:
                for line in f:
                    if line.startswith("Seccomp:"):
                        mode = line.split(":")[1].strip()
                        if mode in ("1", "2"):
                            if result["process"] == ProcessIsolation.PIDNS:
                                result["process"] = ProcessIsolation.FULL
                            else:
                                result["process"] = ProcessIsolation.SECCOMP
                        break
        except (FileNotFoundError, PermissionError):
            pass

    return result
