package domain

import (
	"testing"

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
