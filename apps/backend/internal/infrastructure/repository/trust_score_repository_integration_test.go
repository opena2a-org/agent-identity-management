//go:build integration

package repository

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// TestTrustScoreRepository_RoundTripNineFactorsAndExclusions pins the
// persistence contract that was silently broken until migration 103's
// companion repository change: factor 9 (execution_isolation, 10% weight)
// was absent from the INSERT/SELECT column lists, so it was dropped on
// write and read back as 0; ExcludedFactors was API-response-only, so a
// stored AIP-SPEC 6.1 capped/renormalized composite was not reproducible
// from its row.
//
// Build-tag gated: requires Postgres reachable via TEST_DATABASE_URL with
// migrations through 103 applied.
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run TestTrustScoreRepository_RoundTrip ./internal/infrastructure/repository/...
func TestTrustScoreRepository_RoundTripNineFactorsAndExclusions(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping repository round-trip test")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, db.Ping())

	ctx := context.Background()
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM trust_scores WHERE agent_id = $1`, agentID)
		_, _ = db.ExecContext(ctx, `DELETE FROM agents WHERE id = $1`, agentID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, userID)
		_, _ = db.ExecContext(ctx, `DELETE FROM organizations WHERE id = $1`, orgID)
	})

	_, err = db.ExecContext(ctx,
		`INSERT INTO organizations (id, name, domain, created_at, updated_at)
		 VALUES ($1, $2, $3, NOW(), NOW())`,
		orgID, "trust-persistence-test-org-"+agentID.String()[:8],
		"trust-persistence-"+agentID.String()[:8]+".test")
	require.NoError(t, err)
	_, err = db.ExecContext(ctx,
		`INSERT INTO users (id, organization_id, email, name, provider, provider_id, created_at, updated_at)
		 VALUES ($1, $2, $3, 'trust-persistence-test-user', 'test', $4, NOW(), NOW())`,
		userID, orgID, "trust-persistence-"+agentID.String()[:8]+"@test.invalid",
		"trust-persistence-test-"+userID.String()[:8])
	require.NoError(t, err)

	agentName := "trust-persistence-test-agent-" + agentID.String()[:8]
	_, err = db.ExecContext(ctx,
		`INSERT INTO agents (id, organization_id, name, display_name, agent_type, status, trust_score, created_by, created_at, updated_at)
		 VALUES ($1, $2, $3, $3, 'ai_agent', 'verified', 0.500, $4, NOW(), NOW())`,
		agentID, orgID, agentName, userID)
	require.NoError(t, err)

	repo := NewTrustScoreRepository(db)

	written := &domain.TrustScore{
		AgentID: agentID,
		// trust_scores.score is DECIMAL with 3 fractional digits; keep the
		// fixture representable so the round-trip assertion is exact.
		Score: 0.813,
		Factors: domain.TrustScoreFactors{
			VerificationStatus: 1.0,
			Uptime:             0.95,
			SuccessRate:        0.9,
			SecurityAlerts:     1.0,
			Compliance:         0.5, // neutral placeholder for an excluded factor
			Age:                0.4,
			DriftDetection:     1.0,
			UserFeedback:       0.5, // neutral placeholder for an excluded factor
			ExecutionIsolation: 0.75,
		},
		ExcludedFactors: []string{"compliance", "user_feedback"},
		Confidence:      0.77,
		LastCalculated:  time.Now().UTC().Truncate(time.Second),
	}
	require.NoError(t, repo.Create(written))

	read, err := repo.GetLatest(agentID)
	require.NoError(t, err)
	require.NotNil(t, read)

	// The regression this test exists for: factor 9 survives the round trip.
	require.Equal(t, written.Factors.ExecutionIsolation, read.Factors.ExecutionIsolation,
		"execution_isolation (factor 9) must be persisted, not dropped on write")
	require.Equal(t, written.ExcludedFactors, read.ExcludedFactors,
		"excluded_factors must make the stored composite reproducible from its row")
	require.Equal(t, written.Factors, read.Factors)
	require.InDelta(t, written.Score, read.Score, 1e-9)

	// GetHistory takes the same column list.
	history, err := repo.GetHistory(agentID, 5)
	require.NoError(t, err)
	require.Len(t, history, 1)
	require.Equal(t, written.Factors.ExecutionIsolation, history[0].Factors.ExecutionIsolation)
	require.Equal(t, written.ExcludedFactors, history[0].ExcludedFactors)

	// Nothing-excluded scores round-trip as empty (rows predating migration
	// 103 read the same way via the column default).
	clean := &domain.TrustScore{
		AgentID:        agentID,
		Score:          0.9,
		Factors:        written.Factors,
		Confidence:     0.9,
		LastCalculated: time.Now().UTC(),
	}
	require.NoError(t, repo.Create(clean))
	latest, err := repo.GetLatest(agentID)
	require.NoError(t, err)
	require.Empty(t, latest.ExcludedFactors)
}
