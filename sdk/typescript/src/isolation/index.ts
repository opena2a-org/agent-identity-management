/**
 * Execution Isolation attestation types for AIM SDK.
 */

export enum SandboxType {
  None = "none",
  Docker = "docker",
  VM = "vm",
  GVisor = "gvisor",
  Firecracker = "firecracker",
  WASM = "wasm",
  Kata = "kata",
}

export enum NetworkIsolation {
  None = "none",
  Firewall = "firewall",
  Namespace = "namespace",
  VPC = "vpc",
  Airgap = "airgap",
}

export enum FilesystemIsolation {
  None = "none",
  Chroot = "chroot",
  Tmpfs = "tmpfs",
  ReadOnly = "readonly",
  Overlay = "overlay",
}

export enum ProcessIsolation {
  None = "none",
  PidNS = "pidns",
  Seccomp = "seccomp",
  AppArmor = "apparmor",
  SELinux = "selinux",
  Full = "full",
}

export interface IsolationPosture {
  sandbox: SandboxType;
  network: NetworkIsolation;
  filesystem: FilesystemIsolation;
  process: ProcessIsolation;
}

export interface IsolationAttestationResult {
  id: string;
  agentId: string;
  score: number;
  sandbox: string;
  network: string;
  filesystem: string;
  process: string;
  reportedAt: string;
}

const SANDBOX_SCORES: Record<SandboxType, number> = {
  [SandboxType.Firecracker]: 0.40,
  [SandboxType.Kata]: 0.38,
  [SandboxType.VM]: 0.35,
  [SandboxType.GVisor]: 0.32,
  [SandboxType.WASM]: 0.28,
  [SandboxType.Docker]: 0.20,
  [SandboxType.None]: 0.0,
};

const NETWORK_SCORES: Record<NetworkIsolation, number> = {
  [NetworkIsolation.Airgap]: 0.25,
  [NetworkIsolation.VPC]: 0.20,
  [NetworkIsolation.Namespace]: 0.15,
  [NetworkIsolation.Firewall]: 0.10,
  [NetworkIsolation.None]: 0.0,
};

const FILESYSTEM_SCORES: Record<FilesystemIsolation, number> = {
  [FilesystemIsolation.ReadOnly]: 0.20,
  [FilesystemIsolation.Overlay]: 0.16,
  [FilesystemIsolation.Tmpfs]: 0.12,
  [FilesystemIsolation.Chroot]: 0.08,
  [FilesystemIsolation.None]: 0.0,
};

const PROCESS_SCORES: Record<ProcessIsolation, number> = {
  [ProcessIsolation.Full]: 0.15,
  [ProcessIsolation.SELinux]: 0.12,
  [ProcessIsolation.AppArmor]: 0.11,
  [ProcessIsolation.Seccomp]: 0.10,
  [ProcessIsolation.PidNS]: 0.06,
  [ProcessIsolation.None]: 0.0,
};

/**
 * Compute an isolation score from 0.0 to 1.0 based on posture.
 */
export function scoreIsolation(posture: IsolationPosture): number {
  const score =
    (SANDBOX_SCORES[posture.sandbox] ?? 0) +
    (NETWORK_SCORES[posture.network] ?? 0) +
    (FILESYSTEM_SCORES[posture.filesystem] ?? 0) +
    (PROCESS_SCORES[posture.process] ?? 0);
  return Math.min(score, 1.0);
}

/**
 * Auto-detect the current runtime isolation posture.
 * Works in Node.js environments. Returns defaults for browser/unknown.
 */
export async function autoDetectIsolation(): Promise<IsolationPosture> {
  const posture: IsolationPosture = {
    sandbox: SandboxType.None,
    network: NetworkIsolation.None,
    filesystem: FilesystemIsolation.None,
    process: ProcessIsolation.None,
  };

  try {
    const fs = await import("fs");
    const { readFileSync, existsSync } = fs;

    // Detect Docker
    if (existsSync("/.dockerenv") || existsSync("/run/.containerenv")) {
      posture.sandbox = SandboxType.Docker;
    }

    // Detect gVisor
    try {
      const version = readFileSync("/proc/version", "utf-8");
      if (version.toLowerCase().includes("gvisor")) {
        posture.sandbox = SandboxType.GVisor;
      }
    } catch {}

    // Detect seccomp
    try {
      const status = readFileSync("/proc/self/status", "utf-8");
      const seccompLine = status.split("\n").find((l) => l.startsWith("Seccomp:"));
      if (seccompLine) {
        const mode = seccompLine.split(":")[1].trim();
        if (mode === "1" || mode === "2") {
          posture.process = ProcessIsolation.Seccomp;
        }
      }
    } catch {}

    // Detect read-only root filesystem
    try {
      const mounts = readFileSync("/proc/mounts", "utf-8");
      const rootLine = mounts.split("\n").find((l) => l.split(" ")[1] === "/");
      if (rootLine && rootLine.split(" ")[3]?.split(",").includes("ro")) {
        posture.filesystem = FilesystemIsolation.ReadOnly;
      }
    } catch {}
  } catch {
    // Not in Node.js or no /proc access
  }

  return posture;
}
