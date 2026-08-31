package application

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// AIM-02 — factor 9 (execution isolation) integrity gates.
//
// Factor 9 is an agent's self-report about its own runtime isolation, carrying 10%
// of the trust composite. Before this change an agent that typed
// firecracker + airgap + readonly + full scored 1.0 on it, worth roughly +0.07 over
// the 0.3 no-attestation baseline, for the cost of four strings — and a claim made
// once counted forever.
//
// Two read-side gates close that, per the CDS ruling of 2026-08-29:
//
//	AC1 ceiling — an UNVERIFIED report is clipped to the commodity-container tier.
//	AC2 expiry  — a report older than 90 days stops counting, verified or not.
//	AC3 no write path — nothing can produce a verified row, so the ceiling is the
//	                    effective maximum for every agent today.
//
// Both gates are read-side: the stored row keeps the honest posture score, because
// the table records what was claimed and the scorer decides what a claim is worth.

// memIsolationRepo is an in-memory IsolationAttestationRepository that resolves
// GetLatest the way the SQL one does — newest reported_at wins. A scripted mock
// would let a test assert "the newer row supersedes" while returning whatever it
// was told to; this makes the ordering itself the thing under test.
type memIsolationRepo struct {
	rows []*domain.IsolationAttestation
}

// Create mirrors the SQL INSERT, which writes the literal FALSE for verified
// regardless of the struct field. The fake must not be more permissive than the
// repository it stands in for, or a test could pass here and fail in production.
func (m *memIsolationRepo) Create(a *domain.IsolationAttestation) error {
	stored := *a
	stored.Verified = false
	stored.VerifiedBy = nil
	stored.VerifiedAt = nil
	m.rows = append(m.rows, &stored)
	return nil
}

func (m *memIsolationRepo) GetLatest(agentID uuid.UUID) (*domain.IsolationAttestation, error) {
	var latest *domain.IsolationAttestation
	for _, r := range m.rows {
		if r.AgentID != agentID {
			continue
		}
		if latest == nil || r.ReportedAt.After(latest.ReportedAt) {
			latest = r
		}
	}
	return latest, nil // nil, nil is "no attestation", as the SQL repo returns
}

func (m *memIsolationRepo) GetHistory(agentID uuid.UUID, limit int) ([]*domain.IsolationAttestation, error) {
	out := make([]*domain.IsolationAttestation, 0, len(m.rows))
	for _, r := range m.rows {
		if r.AgentID == agentID {
			out = append(out, r)
		}
	}
	return out, nil
}

// aim02Calculator wires a calculator to an in-memory isolation repo seeded with
// the given rows, all re-pointed at one fresh agent.
func aim02Calculator(t *testing.T, rows ...*domain.IsolationAttestation) (*TrustCalculator, *domain.Agent, *memIsolationRepo) {
	t.Helper()
	agent := &domain.Agent{ID: uuid.New()}
	repo := &memIsolationRepo{}
	for _, r := range rows {
		r.AgentID = agent.ID
		repo.rows = append(repo.rows, r)
	}
	calc := &TrustCalculator{}
	calc.SetIsolationRepo(repo)
	return calc, agent, repo
}

// maxPostureScore is what a hostile agent claims: every enum at its top value.
const maxPostureScore = 1.0

func TestAIM02_UnverifiedCeiling(t *testing.T) {
	t.Run("AIM-02.AC1 ceiling is derived from the commodity-container tier", func(t *testing.T) {
		// Identity, not equality-of-current-value: the ceiling IS the score of
		// docker + namespace + readonly + seccomp. Retuning the enum weights
		// moves the ceiling with the tier it names.
		assert.Equal(t,
			domain.ScoreIsolation(domain.SandboxDocker, domain.NetworkNamespace,
				domain.FilesystemReadOnly, domain.ProcessSeccomp),
			domain.UnverifiedIsolationCeiling(),
			"the ceiling must be the commodity-container score itself, not a copy of its value")

		// And pin today's number. This line failing means the tier moved; the
		// point is that someone has to look at it and decide, rather than the
		// ceiling drifting silently with an unrelated enum retune.
		assert.InDelta(t, 0.65, domain.UnverifiedIsolationCeiling(), 1e-9,
			"docker(0.20) + namespace(0.15) + readonly(0.20) + seccomp(0.10) = 0.65")
	})

	t.Run("AIM-02.AC1 an unverified maximal self-report is clipped to the ceiling", func(t *testing.T) {
		// The gaming vector: claim the strongest posture in the enum and collect
		// 1.0. It now earns exactly what an honest hardened container earns.
		calc, agent, _ := aim02Calculator(t, &domain.IsolationAttestation{
			ID:         uuid.New(),
			Sandbox:    domain.SandboxFirecracker,
			Network:    domain.NetworkAirgap,
			Filesystem: domain.FilesystemReadOnly,
			Process:    domain.ProcessFull,
			Score:      maxPostureScore,
			ReportedAt: time.Now(),
			Verified:   false,
		})

		score, reason := calc.calculateExecutionIsolation(agent)

		assert.InDelta(t, domain.UnverifiedIsolationCeiling(), score, 1e-9,
			"an unverified claim of airgapped firecracker must not outscore a hardened container")
		assert.Less(t, score, maxPostureScore, "the lie must buy strictly less than it claims")
		assert.Empty(t, reason, "a capped factor is scored, not excluded")
	})

	t.Run("AIM-02.AC1 a verified fresh attestation passes through uncapped", func(t *testing.T) {
		verifier := "orchestrator-metadata"
		verifiedAt := time.Now()
		calc, agent, _ := aim02Calculator(t, &domain.IsolationAttestation{
			ID:         uuid.New(),
			Sandbox:    domain.SandboxFirecracker,
			Network:    domain.NetworkAirgap,
			Filesystem: domain.FilesystemReadOnly,
			Process:    domain.ProcessFull,
			Score:      maxPostureScore,
			ReportedAt: time.Now(),
			Verified:   true,
			VerifiedBy: &verifier,
			VerifiedAt: &verifiedAt,
		})

		score, reason := calc.calculateExecutionIsolation(agent)

		assert.InDelta(t, maxPostureScore, score, 1e-9,
			"corroborated posture counts in full — the ceiling is what verification buys past")
		assert.Empty(t, reason)
	})

	t.Run("AIM-02.AC1 an against-interest all-none report passes through unchanged", func(t *testing.T) {
		// min() clips only downward. An agent honest enough to report no
		// isolation at all must keep its honest 0.0 — a cap that raised it to
		// 0.65 would reward the worst posture in the enum for reporting itself,
		// and would score it above the 0.3 no-attestation baseline.
		calc, agent, _ := aim02Calculator(t, &domain.IsolationAttestation{
			ID:         uuid.New(),
			Sandbox:    domain.SandboxNone,
			Network:    domain.NetworkNone,
			Filesystem: domain.FilesystemNone,
			Process:    domain.ProcessNone,
			Score:      0.0,
			ReportedAt: time.Now(),
			Verified:   false,
		})

		score, reason := calc.calculateExecutionIsolation(agent)

		assert.InDelta(t, 0.0, score, 1e-9, "the cap must never raise a score")
		assert.Empty(t, reason)
	})

	t.Run("AIM-02.AC1 an unverified report below the ceiling is untouched", func(t *testing.T) {
		// docker + firewall + tmpfs + seccomp = 0.52, under the ceiling: the cap
		// is a no-op and must not quantize the score to the ceiling.
		below := domain.ScoreIsolation(domain.SandboxDocker, domain.NetworkFirewall,
			domain.FilesystemTmpfs, domain.ProcessSeccomp)
		require.Less(t, below, domain.UnverifiedIsolationCeiling())

		calc, agent, _ := aim02Calculator(t, &domain.IsolationAttestation{
			ID:         uuid.New(),
			Score:      below,
			ReportedAt: time.Now(),
			Verified:   false,
		})

		score, _ := calc.calculateExecutionIsolation(agent)
		assert.InDelta(t, below, score, 1e-9)
	})

	t.Run("AIM-02.AC1 a nil repository still excludes the factor at the baseline", func(t *testing.T) {
		calc := &TrustCalculator{}
		score, reason := calc.calculateExecutionIsolation(&domain.Agent{ID: uuid.New()})

		assert.Equal(t, 0.3, score)
		assert.Equal(t, exclReasonNotWired, reason, "an un-wired repo is a deployment defect, still excluded")
	})

	t.Run("AIM-02.AC1 no attestation still scores the 0.3 baseline in the composite", func(t *testing.T) {
		calc, agent, _ := aim02Calculator(t)
		score, reason := calc.calculateExecutionIsolation(agent)

		assert.Equal(t, 0.3, score)
		assert.Empty(t, reason, "the 0.3 incentive baseline keeps the factor IN the composite")
	})

	t.Run("AIM-02.AC1 the cap is read-side only and the stored row keeps the honest score", func(t *testing.T) {
		calc, agent, repo := aim02Calculator(t)

		att, err := calc.RecordIsolationAttestation(context.Background(), agent.ID,
			domain.SandboxFirecracker, domain.NetworkAirgap,
			domain.FilesystemReadOnly, domain.ProcessFull)
		require.NoError(t, err)

		// What was written: the posture's true score, uncapped.
		assert.InDelta(t, maxPostureScore, att.Score, 1e-9,
			"the returned attestation carries the honest posture score")
		require.Len(t, repo.rows, 1)
		assert.InDelta(t, maxPostureScore, repo.rows[0].Score, 1e-9,
			"the stored row records what was claimed, not what it was allowed to count for")

		// What it counts for: the ceiling.
		score, _ := calc.calculateExecutionIsolation(agent)
		assert.InDelta(t, domain.UnverifiedIsolationCeiling(), score, 1e-9)
		assert.NotEqual(t, repo.rows[0].Score, score,
			"stored posture and scored contribution must be able to differ")
	})
}

func TestAIM02_AttestationExpiry(t *testing.T) {
	// One day past the TTL: the smallest interval that is unambiguously expired.
	pastTTL := func() time.Time {
		return time.Now().Add(-(domain.IsolationAttestationTTL + 24*time.Hour))
	}

	t.Run("AIM-02.AC2 the TTL is 90 days", func(t *testing.T) {
		assert.Equal(t, 90*24*time.Hour, domain.IsolationAttestationTTL)
	})

	t.Run("AIM-02.AC2 an attestation one day past the TTL scores the baseline", func(t *testing.T) {
		calc, agent, _ := aim02Calculator(t, &domain.IsolationAttestation{
			ID:         uuid.New(),
			Score:      maxPostureScore,
			ReportedAt: pastTTL(),
			Verified:   false,
		})

		score, reason := calc.calculateExecutionIsolation(agent)

		assert.Equal(t, 0.3, score, "a claim about a deployment that may no longer exist counts for nothing")
		assert.Empty(t, reason,
			"staleness is a scoring choice, not missing data: a non-empty reason would join the "+
				"AIP §6.1 renormalization set and hand back the weight the agent let rot")
	})

	t.Run("AIM-02.AC2 a stale VERIFIED attestation is also the baseline", func(t *testing.T) {
		// The expiry is uniform. Exempting verified rows would let one
		// verification prop the factor up forever, which is the failure the
		// expiry exists to close.
		verifier := "tee-attestation"
		verifiedAt := pastTTL()
		calc, agent, _ := aim02Calculator(t, &domain.IsolationAttestation{
			ID:         uuid.New(),
			Score:      maxPostureScore,
			ReportedAt: pastTTL(),
			Verified:   true,
			VerifiedBy: &verifier,
			VerifiedAt: &verifiedAt,
		})

		score, reason := calc.calculateExecutionIsolation(agent)

		assert.Equal(t, 0.3, score, "verification does not exempt a row from the expiry")
		assert.Empty(t, reason)
	})

	t.Run("AIM-02.AC2 an attestation just inside the TTL still counts", func(t *testing.T) {
		// Bounds the gate from the other side: "older than 90 days" is strict,
		// so a report at 89 days is live and the expiry is not eating fresh data.
		calc, agent, _ := aim02Calculator(t, &domain.IsolationAttestation{
			ID:         uuid.New(),
			Score:      maxPostureScore,
			ReportedAt: time.Now().Add(-(domain.IsolationAttestationTTL - 24*time.Hour)),
			Verified:   false,
		})

		score, _ := calc.calculateExecutionIsolation(agent)
		assert.InDelta(t, domain.UnverifiedIsolationCeiling(), score, 1e-9,
			"a report inside the TTL is scored (at the unverified ceiling), not expired")
	})

	t.Run("AIM-02.AC2 a fresh re-attestation restores the score", func(t *testing.T) {
		// Expiry is recoverable by doing the thing the factor asks for: report again.
		calc, agent, _ := aim02Calculator(t, &domain.IsolationAttestation{
			ID:         uuid.New(),
			Score:      maxPostureScore,
			ReportedAt: pastTTL(),
		})

		stale, _ := calc.calculateExecutionIsolation(agent)
		require.Equal(t, 0.3, stale, "precondition: the agent starts stale")

		_, err := calc.RecordIsolationAttestation(context.Background(), agent.ID,
			domain.SandboxDocker, domain.NetworkNamespace,
			domain.FilesystemReadOnly, domain.ProcessSeccomp)
		require.NoError(t, err)

		restored, reason := calc.calculateExecutionIsolation(agent)
		assert.InDelta(t, domain.UnverifiedIsolationCeiling(), restored, 1e-9,
			"re-attesting restores the factor")
		assert.Greater(t, restored, stale)
		assert.Empty(t, reason)
	})

	t.Run("AIM-02.AC2 a stale attestation does not enter the exclusion set", func(t *testing.T) {
		// The empty reason has to survive all the way to the published score:
		// execution_isolation must NOT appear in ExcludedFactors, so its 10%
		// weight is not redistributed across the agent's other factors.
		calculator, agent := compositionTestAgent(t)
		repo := &memIsolationRepo{rows: []*domain.IsolationAttestation{{
			ID:         uuid.New(),
			AgentID:    agent.ID,
			Score:      maxPostureScore,
			ReportedAt: pastTTL(),
		}}}
		calculator.SetIsolationRepo(repo)

		score, err := calculator.Calculate(agent)
		require.NoError(t, err)

		assert.NotContains(t, score.ExcludedFactors, "execution_isolation",
			"a stale attestation is scored at the baseline, never renormalized away")
		assert.Equal(t, 0.3, score.Factors.ExecutionIsolation)
	})
}

func TestAIM02_NoVerifiedWritePath(t *testing.T) {
	t.Run("AIM-02.AC3 the ingest path hard-sets verified false", func(t *testing.T) {
		calc, agent, repo := aim02Calculator(t)

		att, err := calc.RecordIsolationAttestation(context.Background(), agent.ID,
			domain.SandboxFirecracker, domain.NetworkAirgap,
			domain.FilesystemReadOnly, domain.ProcessFull)
		require.NoError(t, err)

		assert.False(t, att.Verified, "a self-report is never its own verification")
		assert.Nil(t, att.VerifiedBy)
		assert.Nil(t, att.VerifiedAt)
		require.Len(t, repo.rows, 1)
		assert.False(t, repo.rows[0].Verified, "the persisted row is unverified")
		assert.Nil(t, repo.rows[0].VerifiedBy)
		assert.Nil(t, repo.rows[0].VerifiedAt)
	})

	t.Run("AIM-02.AC3 a re-attestation supersedes a verified row and starts unverified", func(t *testing.T) {
		// No carry-forward: verification is bound to the row, not the agent. An
		// agent verified in a hardened deployment must not be able to move to a
		// bare host, re-attest a maximal posture, and keep the old row's credit.
		verifier := "tee-attestation"
		verifiedAt := time.Now().Add(-time.Hour)
		calc, agent, repo := aim02Calculator(t, &domain.IsolationAttestation{
			ID:         uuid.New(),
			Score:      maxPostureScore,
			ReportedAt: time.Now().Add(-time.Hour),
			Verified:   true,
			VerifiedBy: &verifier,
			VerifiedAt: &verifiedAt,
		})

		before, _ := calc.calculateExecutionIsolation(agent)
		require.InDelta(t, maxPostureScore, before, 1e-9,
			"precondition: the verified row scores in full")

		_, err := calc.RecordIsolationAttestation(context.Background(), agent.ID,
			domain.SandboxFirecracker, domain.NetworkAirgap,
			domain.FilesystemReadOnly, domain.ProcessFull)
		require.NoError(t, err)

		latest, err := repo.GetLatest(agent.ID)
		require.NoError(t, err)
		assert.False(t, latest.Verified, "the newest row is a fresh self-report, unverified")
		assert.Nil(t, latest.VerifiedBy)

		after, reason := calc.calculateExecutionIsolation(agent)
		assert.InDelta(t, domain.UnverifiedIsolationCeiling(), after, 1e-9,
			"the same posture now scores at the ceiling — the predecessor's verification is gone")
		assert.Less(t, after, before, "re-attesting cannot preserve a superseded verification")
		assert.Empty(t, reason)
	})

	t.Run("AIM-02.AC3 the ingest signature carries no verification input", func(t *testing.T) {
		// Structural, not behavioural: RecordIsolationAttestation takes posture
		// only. There is no argument through which a caller — handler, SDK, or
		// anything downstream of a request body — could ask for verification, so
		// the invariant does not depend on any caller behaving.
		var _ func(context.Context, uuid.UUID, domain.SandboxType, domain.NetworkIsolation,
			domain.FilesystemIsolation, domain.ProcessIsolation) (*domain.IsolationAttestation, error) =
			(&TrustCalculator{}).RecordIsolationAttestation
	})
}
