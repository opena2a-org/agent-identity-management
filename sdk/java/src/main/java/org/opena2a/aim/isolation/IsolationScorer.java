package org.opena2a.aim.isolation;

import java.util.Map;

/**
 * Computes isolation scores based on runtime isolation posture.
 */
public class IsolationScorer {

    private static final Map<SandboxType, Double> SANDBOX_SCORES = Map.of(
        SandboxType.FIRECRACKER, 0.40,
        SandboxType.KATA, 0.38,
        SandboxType.VM, 0.35,
        SandboxType.GVISOR, 0.32,
        SandboxType.WASM, 0.28,
        SandboxType.DOCKER, 0.20,
        SandboxType.NONE, 0.0
    );

    private static final Map<NetworkIsolation, Double> NETWORK_SCORES = Map.of(
        NetworkIsolation.AIRGAP, 0.25,
        NetworkIsolation.VPC, 0.20,
        NetworkIsolation.NAMESPACE, 0.15,
        NetworkIsolation.FIREWALL, 0.10,
        NetworkIsolation.NONE, 0.0
    );

    private static final Map<FilesystemIsolation, Double> FILESYSTEM_SCORES = Map.of(
        FilesystemIsolation.READONLY, 0.20,
        FilesystemIsolation.OVERLAY, 0.16,
        FilesystemIsolation.TMPFS, 0.12,
        FilesystemIsolation.CHROOT, 0.08,
        FilesystemIsolation.NONE, 0.0
    );

    private static final Map<ProcessIsolation, Double> PROCESS_SCORES = Map.of(
        ProcessIsolation.FULL, 0.15,
        ProcessIsolation.SELINUX, 0.12,
        ProcessIsolation.APPARMOR, 0.11,
        ProcessIsolation.SECCOMP, 0.10,
        ProcessIsolation.PIDNS, 0.06,
        ProcessIsolation.NONE, 0.0
    );

    /**
     * Compute an isolation score from 0.0 to 1.0 based on posture.
     */
    public static double score(SandboxType sandbox, NetworkIsolation network,
                               FilesystemIsolation filesystem, ProcessIsolation process) {
        double result = SANDBOX_SCORES.getOrDefault(sandbox, 0.0)
            + NETWORK_SCORES.getOrDefault(network, 0.0)
            + FILESYSTEM_SCORES.getOrDefault(filesystem, 0.0)
            + PROCESS_SCORES.getOrDefault(process, 0.0);
        return Math.min(result, 1.0);
    }
}
