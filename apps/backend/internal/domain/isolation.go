package domain

import (
	"fmt"
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
	ProcessNone     ProcessIsolation = "none"
	ProcessPIDNS    ProcessIsolation = "pidns"
	ProcessSeccomp  ProcessIsolation = "seccomp"
	ProcessAppArmor ProcessIsolation = "apparmor"
	ProcessSELinux  ProcessIsolation = "selinux"
	ProcessFull     ProcessIsolation = "full" // PID namespace + seccomp + MAC
)

// IsolationAttestationTTL is the hard expiry on a reported posture: an
// attestation whose reported_at is older than this counts for nothing and the
// factor falls back to the no-attestation baseline.
//
// It is uniform for verified and unverified rows on purpose. A posture is a
// statement about a running deployment, and deployments move; a claim made once
// and never renewed describes a machine that may no longer exist. Exempting
// verified rows would let a single verification prop the factor up forever,
// which is the "one-time claim props up the score indefinitely" failure the
// expiry exists to close. 90 days matches the re-attestation cadence an operator
// can reasonably hold without the factor becoming a nag.
const IsolationAttestationTTL = 90 * 24 * time.Hour

// UnverifiedIsolationCeiling is the highest factor-9 score an UNVERIFIED
// self-report may earn on the read side.
//
// It is DERIVED, never a literal: it is exactly the score of the commodity
// container tier — Docker + network namespace + read-only rootfs + seccomp — the
// posture a competent operator gets from an ordinary hardened container. The
// reasoning is that an unverified claim is worth no more than what the claim is
// cheapest to make about; anything above this tier (a VM, a TEE, an airgap) is a
// claim about infrastructure AIM cannot see, so it must be earned with
// verification rather than asserted.
//
// Deriving it from ScoreIsolation means a retune of the posture enum weights
// moves the ceiling with the tier it names instead of stranding a stale
// constant. TestAIM02_UnverifiedCeiling pins today's value (0.65) so the retune
// is a conscious edit rather than a silent drift.
//
// This is a READ-side cap only. The stored attestation Score stays the honest
// posture value: the row is a record of what was reported, not of what it was
// allowed to count for.
func UnverifiedIsolationCeiling() float64 {
	return ScoreIsolation(SandboxDocker, NetworkNamespace, FilesystemReadOnly, ProcessSeccomp)
}

// IsolationAttestation represents an agent's self-reported runtime isolation posture.
// Agents submit this via SDK; the server scores it as a trust factor.
type IsolationAttestation struct {
	ID         uuid.UUID           `json:"id"`
	AgentID    uuid.UUID           `json:"agentId"`
	Sandbox    SandboxType         `json:"sandbox"`
	Network    NetworkIsolation    `json:"network"`
	Filesystem FilesystemIsolation `json:"filesystem"`
	Process    ProcessIsolation    `json:"process"`
	Score      float64             `json:"score"` // Computed 0-1 isolation score
	ReportedAt time.Time           `json:"reportedAt"`
	CreatedAt  time.Time           `json:"createdAt"`

	// Verified reports whether an INDEPENDENT source corroborated this row's
	// posture. It is bound to the row, not to the agent: verification describes
	// one report of one deployment at one moment, so a later re-attestation
	// starts unverified and never carries a predecessor's verification forward.
	//
	// Nothing in the codebase sets this true today. There is no verified write
	// path until an independent source exists (Phase 2, roadmap
	// aim-isolation-verification); the ingest path hard-sets false and the
	// repository INSERT writes the literal FALSE. The evidence classes that may
	// ever set it are TEE attestation and orchestrator/host metadata; an HMA
	// static scan may NOT (it reads the declared surface, not the running one),
	// and the SDK may never (it is the self-report being checked).
	Verified bool `json:"verified"`

	// VerifiedBy names the independent source that corroborated the posture, and
	// VerifiedAt is when it did. Both are nil while Verified is false.
	VerifiedBy *string    `json:"verifiedBy,omitempty"`
	VerifiedAt *time.Time `json:"verifiedAt,omitempty"`
}

// IsExpiredAt reports whether the attestation is past IsolationAttestationTTL
// relative to now. Uniform for verified and unverified rows.
func (a *IsolationAttestation) IsExpiredAt(now time.Time) bool {
	return now.Sub(a.ReportedAt) > IsolationAttestationTTL
}

// IsolationAttestationRepository defines persistence for isolation attestations
type IsolationAttestationRepository interface {
	Create(attestation *IsolationAttestation) error
	GetLatest(agentID uuid.UUID) (*IsolationAttestation, error)
	GetHistory(agentID uuid.UUID, limit int) ([]*IsolationAttestation, error)
}

// IsValid reports whether the sandbox type is a recognized value.
func (s SandboxType) IsValid() bool {
	switch s {
	case SandboxNone, SandboxDocker, SandboxVM, SandboxGVisor, SandboxFirecracker, SandboxWASM, SandboxKataVM:
		return true
	}
	return false
}

// IsValid reports whether the network isolation level is a recognized value.
func (n NetworkIsolation) IsValid() bool {
	switch n {
	case NetworkNone, NetworkFirewall, NetworkNamespace, NetworkVPC, NetworkAirgap:
		return true
	}
	return false
}

// IsValid reports whether the filesystem isolation level is a recognized value.
func (f FilesystemIsolation) IsValid() bool {
	switch f {
	case FilesystemNone, FilesystemChroot, FilesystemTmpfs, FilesystemReadOnly, FilesystemOverlay:
		return true
	}
	return false
}

// IsValid reports whether the process isolation level is a recognized value.
func (p ProcessIsolation) IsValid() bool {
	switch p {
	case ProcessNone, ProcessPIDNS, ProcessSeccomp, ProcessAppArmor, ProcessSELinux, ProcessFull:
		return true
	}
	return false
}

// ValidateIsolationPosture rejects any unrecognized posture value. The ingest
// path computes the score server-side via ScoreIsolation (which treats unknown
// values as zero contribution); validating up front turns a silent zero into an
// explicit error so a typo'd posture is never persisted as a real attestation.
func ValidateIsolationPosture(sandbox SandboxType, network NetworkIsolation, filesystem FilesystemIsolation, process ProcessIsolation) error {
	if !sandbox.IsValid() {
		return fmt.Errorf("invalid sandbox type %q", sandbox)
	}
	if !network.IsValid() {
		return fmt.Errorf("invalid network isolation %q", network)
	}
	if !filesystem.IsValid() {
		return fmt.Errorf("invalid filesystem isolation %q", filesystem)
	}
	if !process.IsValid() {
		return fmt.Errorf("invalid process isolation %q", process)
	}
	return nil
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
