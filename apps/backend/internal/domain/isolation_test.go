package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// ============================================================================
// TEST: ScoreIsolation() - Domain scoring function
// ============================================================================

func TestScoreIsolation_MaxIsolation(t *testing.T) {
	// Firecracker (0.40) + Airgap (0.25) + ReadOnly (0.20) + Full (0.15) = 1.0
	score := ScoreIsolation(SandboxFirecracker, NetworkAirgap, FilesystemReadOnly, ProcessFull)
	assert.Equal(t, 1.0, score, "Maximum isolation posture should produce score of 1.0")
}

func TestScoreIsolation_NoIsolation(t *testing.T) {
	score := ScoreIsolation(SandboxNone, NetworkNone, FilesystemNone, ProcessNone)
	assert.Equal(t, 0.0, score, "No isolation should produce score of 0.0")
}

func TestScoreIsolation_DockerFirewallTmpfsSeccomp(t *testing.T) {
	// Docker (0.20) + Firewall (0.10) + Tmpfs (0.12) + Seccomp (0.10) = 0.52
	score := ScoreIsolation(SandboxDocker, NetworkFirewall, FilesystemTmpfs, ProcessSeccomp)
	assert.InDelta(t, 0.52, score, 0.001, "Docker+Firewall+Tmpfs+Seccomp should produce ~0.52")
}

func TestScoreIsolation_AllSandboxTypes(t *testing.T) {
	tests := []struct {
		name     string
		sandbox  SandboxType
		expected float64
	}{
		{"Firecracker", SandboxFirecracker, 0.40},
		{"KataVM", SandboxKataVM, 0.38},
		{"VM", SandboxVM, 0.35},
		{"GVisor", SandboxGVisor, 0.32},
		{"WASM", SandboxWASM, 0.28},
		{"Docker", SandboxDocker, 0.20},
		{"None", SandboxNone, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := ScoreIsolation(tt.sandbox, NetworkNone, FilesystemNone, ProcessNone)
			assert.InDelta(t, tt.expected, score, 0.001,
				"Sandbox %s with no other isolation should score %f", tt.name, tt.expected)
		})
	}
}

func TestScoreIsolation_AllNetworkTypes(t *testing.T) {
	tests := []struct {
		name     string
		network  NetworkIsolation
		expected float64
	}{
		{"Airgap", NetworkAirgap, 0.25},
		{"VPC", NetworkVPC, 0.20},
		{"Namespace", NetworkNamespace, 0.15},
		{"Firewall", NetworkFirewall, 0.10},
		{"None", NetworkNone, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := ScoreIsolation(SandboxNone, tt.network, FilesystemNone, ProcessNone)
			assert.InDelta(t, tt.expected, score, 0.001)
		})
	}
}

func TestScoreIsolation_AllFilesystemTypes(t *testing.T) {
	tests := []struct {
		name       string
		filesystem FilesystemIsolation
		expected   float64
	}{
		{"ReadOnly", FilesystemReadOnly, 0.20},
		{"Overlay", FilesystemOverlay, 0.16},
		{"Tmpfs", FilesystemTmpfs, 0.12},
		{"Chroot", FilesystemChroot, 0.08},
		{"None", FilesystemNone, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := ScoreIsolation(SandboxNone, NetworkNone, tt.filesystem, ProcessNone)
			assert.InDelta(t, tt.expected, score, 0.001)
		})
	}
}

func TestScoreIsolation_AllProcessTypes(t *testing.T) {
	tests := []struct {
		name     string
		process  ProcessIsolation
		expected float64
	}{
		{"Full", ProcessFull, 0.15},
		{"SELinux", ProcessSELinux, 0.12},
		{"AppArmor", ProcessAppArmor, 0.11},
		{"Seccomp", ProcessSeccomp, 0.10},
		{"PidNS", ProcessPIDNS, 0.06},
		{"None", ProcessNone, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := ScoreIsolation(SandboxNone, NetworkNone, FilesystemNone, tt.process)
			assert.InDelta(t, tt.expected, score, 0.001)
		})
	}
}

func TestScoreIsolation_ClampedToMax(t *testing.T) {
	// The maximum possible sum is exactly 1.0 (0.40+0.25+0.20+0.15),
	// so no combination of valid inputs exceeds 1.0.
	// Verify the clamp logic exists by confirming max does not exceed 1.0.
	score := ScoreIsolation(SandboxFirecracker, NetworkAirgap, FilesystemReadOnly, ProcessFull)
	assert.LessOrEqual(t, score, 1.0, "Score must be clamped to 1.0 maximum")
	assert.GreaterOrEqual(t, score, 0.0, "Score must be non-negative")
}

func TestScoreIsolation_ScoreAlwaysInRange(t *testing.T) {
	// Exhaustive check: every valid combination produces score in [0, 1]
	sandboxes := []SandboxType{SandboxNone, SandboxDocker, SandboxVM, SandboxGVisor, SandboxFirecracker, SandboxWASM, SandboxKataVM}
	networks := []NetworkIsolation{NetworkNone, NetworkFirewall, NetworkNamespace, NetworkVPC, NetworkAirgap}
	filesystems := []FilesystemIsolation{FilesystemNone, FilesystemChroot, FilesystemTmpfs, FilesystemReadOnly, FilesystemOverlay}
	processes := []ProcessIsolation{ProcessNone, ProcessPIDNS, ProcessSeccomp, ProcessAppArmor, ProcessSELinux, ProcessFull}

	for _, s := range sandboxes {
		for _, n := range networks {
			for _, f := range filesystems {
				for _, p := range processes {
					score := ScoreIsolation(s, n, f, p)
					assert.GreaterOrEqual(t, score, 0.0, "Score must be >= 0 for %s/%s/%s/%s", s, n, f, p)
					assert.LessOrEqual(t, score, 1.0, "Score must be <= 1 for %s/%s/%s/%s", s, n, f, p)
				}
			}
		}
	}
}

func TestScoreIsolation_UnknownTypesScoreZero(t *testing.T) {
	// Unknown enum values should contribute 0 (fall through switch)
	score := ScoreIsolation("unknown", "unknown", "unknown", "unknown")
	assert.Equal(t, 0.0, score, "Unknown isolation types should score 0.0")
}

func TestScoreIsolation_PartialIsolation(t *testing.T) {
	// VM (0.35) + VPC (0.20) + None + None = 0.55
	score := ScoreIsolation(SandboxVM, NetworkVPC, FilesystemNone, ProcessNone)
	assert.InDelta(t, 0.55, score, 0.001)

	// None + Namespace (0.15) + Overlay (0.16) + AppArmor (0.11) = 0.42
	score2 := ScoreIsolation(SandboxNone, NetworkNamespace, FilesystemOverlay, ProcessAppArmor)
	assert.InDelta(t, 0.42, score2, 0.001)
}

// ============================================================================
// TEST: posture validation (ingest guard)
// ============================================================================

func TestValidateIsolationPosture_AllValid(t *testing.T) {
	err := ValidateIsolationPosture(SandboxFirecracker, NetworkAirgap, FilesystemReadOnly, ProcessFull)
	assert.NoError(t, err)

	// "none" across the board is a legitimate posture (no isolation), not invalid.
	err = ValidateIsolationPosture(SandboxNone, NetworkNone, FilesystemNone, ProcessNone)
	assert.NoError(t, err)
}

func TestValidateIsolationPosture_RejectsUnknownValues(t *testing.T) {
	cases := []struct {
		name       string
		sandbox    SandboxType
		network    NetworkIsolation
		filesystem FilesystemIsolation
		process    ProcessIsolation
	}{
		{"bad sandbox", SandboxType("xen"), NetworkNone, FilesystemNone, ProcessNone},
		{"bad network", SandboxDocker, NetworkIsolation("mesh"), FilesystemNone, ProcessNone},
		{"bad filesystem", SandboxDocker, NetworkNone, FilesystemIsolation("zfs"), ProcessNone},
		{"bad process", SandboxDocker, NetworkNone, FilesystemNone, ProcessIsolation("nacl")},
		{"empty sandbox", SandboxType(""), NetworkNone, FilesystemNone, ProcessNone},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Error(t, ValidateIsolationPosture(tc.sandbox, tc.network, tc.filesystem, tc.process))
		})
	}
}

func TestIsolationPosture_IsValid(t *testing.T) {
	assert.True(t, SandboxKataVM.IsValid())
	assert.True(t, NetworkVPC.IsValid())
	assert.True(t, FilesystemOverlay.IsValid())
	assert.True(t, ProcessSELinux.IsValid())
	assert.False(t, SandboxType("bogus").IsValid())
	assert.False(t, NetworkIsolation("").IsValid())
}

// TestUnverifiedIsolationCeiling pins today's ceiling and proves it is DERIVED
// from the commodity-container tier, not a stranded constant: a retune of the
// posture-enum weights must move both sides of this equality together.
func TestUnverifiedIsolationCeiling(t *testing.T) {
	got := UnverifiedIsolationCeiling()
	assert.Equal(t, ScoreIsolation(SandboxDocker, NetworkNamespace, FilesystemReadOnly, ProcessSeccomp), got,
		"the ceiling must equal the docker/namespace/readonly/seccomp tier it is derived from")
	assert.InDelta(t, 0.65, got, 0.001, "today's ceiling is 0.65; a change here is a conscious retune")
	// A maximal posture must not out-score the ceiling once it is clipped to it,
	// and the ceiling itself must sit strictly below a fully-hardened score.
	assert.Less(t, got, ScoreIsolation(SandboxKataVM, NetworkAirgap, FilesystemReadOnly, ProcessSeccomp),
		"an unverified claim must earn strictly less than a fully-hardened posture")
}

// TestIsolationAttestation_IsExpiredAt exercises both sides of the 90-day TTL
// boundary plus the exact edge, uniform for verified and unverified rows.
func TestIsolationAttestation_IsExpiredAt(t *testing.T) {
	now := time.Now()
	mk := func(age time.Duration, verified bool) *IsolationAttestation {
		return &IsolationAttestation{
			ID:         uuid.New(),
			AgentID:    uuid.New(),
			ReportedAt: now.Add(-age),
			Verified:   verified,
		}
	}
	assert.False(t, mk(89*24*time.Hour, false).IsExpiredAt(now), "89 days is inside the TTL")
	assert.False(t, mk(IsolationAttestationTTL, false).IsExpiredAt(now), "exactly at the TTL is not yet past it")
	assert.True(t, mk(IsolationAttestationTTL+time.Second, false).IsExpiredAt(now), "one second past the TTL is expired")
	assert.True(t, mk(91*24*time.Hour, false).IsExpiredAt(now), "91 days is expired")
	// Expiry does not depend on verification.
	assert.True(t, mk(91*24*time.Hour, true).IsExpiredAt(now), "a verified row expires on the same clock")
	assert.False(t, mk(1*time.Hour, true).IsExpiredAt(now), "a fresh verified row is not expired")
}
