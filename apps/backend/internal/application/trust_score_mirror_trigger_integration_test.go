//go:build integration

package application

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// TestTrustScoreMirrorTrigger_InsertSyncsAgentCache asserts that
// INSERTing into `trust_scores` automatically mirrors the score
// onto the `agents.trust_score` denormalized cache. Closes #163.
//
// Build-tag gated: requires Postgres reachable via TEST_DATABASE_URL.
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run TestTrustScoreMirrorTrigger ./internal/application/...
func TestTrustScoreMirrorTrigger_InsertSyncsAgentCache(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping trigger regression test")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, db.Ping())

	ctx := context.Background()
	orgID := uuid.New()
	agentID := uuid.New()
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM trust_scores WHERE agent_id = $1`, agentID)
		_, _ = db.ExecContext(ctx, `DELETE FROM agents WHERE id = $1`, agentID)
		_, _ = db.ExecContext(ctx, `DELETE FROM organizations WHERE id = $1`, orgID)
	})

	_, err = db.ExecContext(ctx,
		`INSERT INTO organizations (id, name, domain, created_at, updated_at)
		 VALUES ($1, $2, $3, NOW(), NOW())`,
		orgID, "trust-score-mirror-test-org-"+agentID.String()[:8],
		"trust-score-mirror-test-"+agentID.String()[:8]+".test")
	require.NoError(t, err)
	userID := uuid.New()
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, userID) })
	_, err = db.ExecContext(ctx,
		`INSERT INTO users (id, organization_id, email, name, provider, provider_id, created_at, updated_at)
		 VALUES ($1, $2, $3, 'trust-score-mirror-test-user', 'test', $4, NOW(), NOW())`,
		userID, orgID, "trust-score-mirror-test-"+agentID.String()[:8]+"@test.invalid",
		"trust-score-mirror-test-"+userID.String()[:8])
	require.NoError(t, err)

	// Agent starts with a stale cache value of 0.500.
	_, err = db.ExecContext(ctx,
		`INSERT INTO agents (id, organization_id, name, display_name, agent_type, status, trust_score, created_by, created_at, updated_at)
		 VALUES ($1, $2, $3, $3, 'ai_agent', 'verified', 0.500, $4, NOW(), NOW())`,
		agentID, orgID, "trust-score-mirror-test-agent-"+agentID.String()[:8], userID)
	require.NoError(t, err)

	// INSERT a new score row. The trigger must mirror to agents.trust_score.
	_, err = db.ExecContext(ctx,
		`INSERT INTO trust_scores
		   (id, agent_id, score,
		    verification_status, uptime, success_rate, security_alerts,
		    compliance, age, drift_detection, user_feedback,
		    confidence, last_calculated, created_at)
		 VALUES (gen_random_uuid(), $1, 0.820,
		         0.9, 0.8, 0.7, 0.6, 0.85, 0.9, 0.7, 0.8, 0.95,
		         NOW(), NOW())`,
		agentID)
	require.NoError(t, err)

	var agentScore float64
	err = db.QueryRowContext(ctx,
		`SELECT trust_score FROM agents WHERE id = $1`, agentID,
	).Scan(&agentScore)
	require.NoError(t, err)
	// agents.trust_score carries two fractional digits; keep the fixture
	// representable so the mirrored value compares exactly.
	require.InDelta(t, 0.820, agentScore, 0.001,
		"trigger must mirror trust_scores.score onto agents.trust_score on INSERT")
}

// TestTrustScoreMirrorTrigger_UpdateSyncsLatestOnly asserts the
// guard: an UPDATE on the LATEST trust_scores row mirrors, but an
// UPDATE on an OLDER historical row does NOT clobber the cache
// (the cache should reflect the most recent measurement).
func TestTrustScoreMirrorTrigger_UpdateSyncsLatestOnly(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping trigger regression test")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, db.Ping())

	ctx := context.Background()
	orgID := uuid.New()
	agentID := uuid.New()
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM trust_scores WHERE agent_id = $1`, agentID)
		_, _ = db.ExecContext(ctx, `DELETE FROM agents WHERE id = $1`, agentID)
		_, _ = db.ExecContext(ctx, `DELETE FROM organizations WHERE id = $1`, orgID)
	})

	_, err = db.ExecContext(ctx,
		`INSERT INTO organizations (id, name, domain, created_at, updated_at)
		 VALUES ($1, $2, $3, NOW(), NOW())`,
		orgID, "trust-score-update-test-org-"+agentID.String()[:8],
		"trust-score-update-test-"+agentID.String()[:8]+".test")
	require.NoError(t, err)
	userID := uuid.New()
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, userID) })
	_, err = db.ExecContext(ctx,
		`INSERT INTO users (id, organization_id, email, name, provider, provider_id, created_at, updated_at)
		 VALUES ($1, $2, $3, 'trust-score-update-test-user', 'test', $4, NOW(), NOW())`,
		userID, orgID, "trust-score-update-test-"+agentID.String()[:8]+"@test.invalid",
		"trust-score-update-test-"+userID.String()[:8])
	require.NoError(t, err)
	_, err = db.ExecContext(ctx,
		`INSERT INTO agents (id, organization_id, name, display_name, agent_type, status, trust_score, created_by, created_at, updated_at)
		 VALUES ($1, $2, $3, $3, 'ai_agent', 'verified', 0.500, $4, NOW(), NOW())`,
		agentID, orgID, "trust-score-update-test-agent-"+agentID.String()[:8], userID)
	require.NoError(t, err)

	// Seed two trust_scores rows: an older one and a newer one.
	olderID := uuid.New()
	newerID := uuid.New()
	olderCreatedAt := time.Now().UTC().Add(-1 * time.Hour)
	_, err = db.ExecContext(ctx,
		`INSERT INTO trust_scores
		   (id, agent_id, score, verification_status, uptime, success_rate,
		    security_alerts, compliance, age, drift_detection, user_feedback,
		    confidence, last_calculated, created_at)
		 VALUES ($1, $2, 0.600, 0.7, 0.7, 0.7, 0.7, 0.7, 0.7, 0.7, 0.7, 0.9, $3, $3)`,
		olderID, agentID, olderCreatedAt)
	require.NoError(t, err)

	// Newer row inserts; trigger fires; cache moves to 0.900.
	_, err = db.ExecContext(ctx,
		`INSERT INTO trust_scores
		   (id, agent_id, score, verification_status, uptime, success_rate,
		    security_alerts, compliance, age, drift_detection, user_feedback,
		    confidence, last_calculated, created_at)
		 VALUES ($1, $2, 0.900, 0.9, 0.9, 0.9, 0.9, 0.9, 0.9, 0.9, 0.9, 0.95, NOW(), NOW())`,
		newerID, agentID)
	require.NoError(t, err)

	var cached float64
	err = db.QueryRowContext(ctx,
		`SELECT trust_score FROM agents WHERE id = $1`, agentID).Scan(&cached)
	require.NoError(t, err)
	require.InDelta(t, 0.900, cached, 0.001,
		"after the newer INSERT, cache should reflect the newer score")

	// Now UPDATE the OLDER historical row. The trigger must NOT
	// clobber the cache, because the older row is not the latest.
	_, err = db.ExecContext(ctx,
		`UPDATE trust_scores SET score = 0.100 WHERE id = $1`, olderID)
	require.NoError(t, err)

	err = db.QueryRowContext(ctx,
		`SELECT trust_score FROM agents WHERE id = $1`, agentID).Scan(&cached)
	require.NoError(t, err)
	require.InDelta(t, 0.900, cached, 0.001,
		"UPDATE on a historical (non-latest) row must NOT clobber the cache")

	// UPDATE on the LATEST row must mirror.
	_, err = db.ExecContext(ctx,
		`UPDATE trust_scores SET score = 0.750 WHERE id = $1`, newerID)
	require.NoError(t, err)

	err = db.QueryRowContext(ctx,
		`SELECT trust_score FROM agents WHERE id = $1`, agentID).Scan(&cached)
	require.NoError(t, err)
	require.InDelta(t, 0.750, cached, 0.001,
		"UPDATE on the latest row must mirror to the cache")
}
