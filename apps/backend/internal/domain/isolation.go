package domain

import (
	"time"

	"github.com/google/uuid"
)

// SandboxType describes the agent's runtime sandbox environment
type SandboxType string

const (
	SandboxNone        SandboxType = "none"
	SandboxDocker      SandboxType = "docker"
	SandboxVM          SandboxType = "vm"
	SandboxGVisor      SandboxType = "gvisor"
	SandboxFirecracker SandboxType = "firecracker"
	SandboxWASM        SandboxType = "wasm"
	SandboxKataVM      SandboxType = "kata"
)

// NetworkIsolation describes the agent's network isolation posture
type NetworkIsolation string

const (
	NetworkNone      NetworkIsolation = "none"
	NetworkFirewall  NetworkIsolation = "firewall"
	NetworkNamespace NetworkIsolation = "namespace"
	NetworkVPC       NetworkIsolation = "vpc"
	NetworkAirgap    NetworkIsolation = "airgap"
)

// FilesystemIsolation describes the agent's filesystem isolation
type FilesystemIsolation string

const (
	FilesystemNone     FilesystemIsolation = "none"
	FilesystemChroot   FilesystemIsolation = "chroot"
	FilesystemTmpfs    FilesystemIsolation = "tmpfs"
	FilesystemReadOnly FilesystemIsolation = "readonly"
	FilesystemOverlay  FilesystemIsolation = "overlay"
)

// ProcessIsolation describes the agent's process-level isolation
type ProcessIsolation string

const (
	ProcessNone      ProcessIsolation = "none"
	ProcessPIDNS     ProcessIsolation = "pidns"
	ProcessSeccomp   ProcessIsolation = "seccomp"
	ProcessAppArmor  ProcessIsolation = "apparmor"
	ProcessSELinux   ProcessIsolation = "selinux"
	ProcessFull      ProcessIsolation = "full" // PID namespace + seccomp + MAC
)

// IsolationAttestation represents an agent's self-reported runtime isolation posture.
// Agents submit this via SDK; the server scores it as a trust factor.
type IsolationAttestation struct {
	ID           uuid.UUID           `json:"id"`
	AgentID      uuid.UUID           `json:"agentId"`
	Sandbox      SandboxType         `json:"sandbox"`
	Network      NetworkIsolation    `json:"network"`
	Filesystem   FilesystemIsolation `json:"filesystem"`
	Process      ProcessIsolation    `json:"process"`
	Score        float64             `json:"score"`     // Computed 0-1 isolation score
	ReportedAt   time.Time           `json:"reportedAt"`
	CreatedAt    time.Time           `json:"createdAt"`
}

// IsolationAttestationRepository defines persistence for isolation attestations
type IsolationAttestationRepository interface {
	Create(attestation *IsolationAttestation) error
	GetLatest(agentID uuid.UUID) (*IsolationAttestation, error)
	GetHistory(agentID uuid.UUID, limit int) ([]*IsolationAttestation, error)
}

// ScoreIsolation computes an isolation score from 0.0 to 1.0 based on posture.
// Higher isolation = higher score. No isolation = 0.0.
func ScoreIsolation(sandbox SandboxType, network NetworkIsolation, filesystem FilesystemIsolation, process ProcessIsolation) float64 {
	score := 0.0

	// Sandbox (40% of isolation score)
	switch sandbox {
	case SandboxFirecracker:
		score += 0.40
	case SandboxKataVM:
		score += 0.38
	case SandboxVM:
		score += 0.35
	case SandboxGVisor:
		score += 0.32
	case SandboxWASM:
		score += 0.28
	case SandboxDocker:
		score += 0.20
	case SandboxNone:
		score += 0.0
	}

	// Network (25% of isolation score)
	switch network {
	case NetworkAirgap:
		score += 0.25
	case NetworkVPC:
		score += 0.20
	case NetworkNamespace:
		score += 0.15
	case NetworkFirewall:
		score += 0.10
	case NetworkNone:
		score += 0.0
	}

	// Filesystem (20% of isolation score)
	switch filesystem {
	case FilesystemReadOnly:
		score += 0.20
	case FilesystemOverlay:
		score += 0.16
	case FilesystemTmpfs:
		score += 0.12
	case FilesystemChroot:
		score += 0.08
	case FilesystemNone:
		score += 0.0
	}

	// Process (15% of isolation score)
	switch process {
	case ProcessFull:
		score += 0.15
	case ProcessSELinux:
		score += 0.12
	case ProcessAppArmor:
		score += 0.11
	case ProcessSeccomp:
		score += 0.10
	case ProcessPIDNS:
		score += 0.06
	case ProcessNone:
		score += 0.0
	}

	// Clamp to [0, 1]
	if score > 1.0 {
		score = 1.0
	}
	return score
}
