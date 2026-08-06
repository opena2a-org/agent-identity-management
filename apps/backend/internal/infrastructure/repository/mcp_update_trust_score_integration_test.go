//go:build integration

package repository

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMCPServerRepository_UpdateDoesNotWriteTrustScore is the regression test for
// `MCPServerRepository.Update` writing `mcp_servers.trust_score` unconditionally.
//
// `trust_score` is a denormalized cache of `mcp_trust_scores.score`, maintained by
// the migration 094 trigger. While `Update` carried it in its SET list, every
// caller that patched an unrelated field wrote back whatever `TrustScore` its
// in-memory snapshot happened to hold. When that snapshot was stale the write did
// two things:
//
//  1. reverted the calculated cache to an old value, and
//  2. fired `mcp_trust_score_change_trigger` (migration 051), inserting a
//     `mcp_trust_score_history` row with `change_reason = 'automated_recalculation'`
//     describing a recalculation that never ran.
//
// (2) is the reason this is filed as a data-integrity defect rather than a cache
// bug: it fabricates a security event inside an audit trail. Migration 104's header
// names exactly that fabrication as worse than the defect it was fixing.
//
// The staleness has to be real for this test to mean anything. `UpdateMCPServer`
// calls `GetByID` immediately before `Update`, so its snapshot is fresh, the
// trigger's `IF OLD.trust_score IS DISTINCT FROM NEW.trust_score` guard suppresses
// the insert, and a test written that way passes on the unfixed code. So the
// sequence below deliberately opens the window: take a snapshot, move the cache
// underneath it through the source of truth, and only then save the snapshot.
//
// Build-tag gated: requires Postgres reachable via TEST_DATABASE_URL with
// migrations through 104 applied.
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run TestMCPServerRepository_UpdateDoesNotWriteTrustScore \
//	  ./internal/infrastructure/repository/...
func TestMCPServerRepository_UpdateDoesNotWriteTrustScore(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping MCP update write-path test")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, db.Ping())

	ctx := context.Background()
	orgID := uuid.New()
	userID := uuid.New()
	serverID := uuid.New()
	suffix := serverID.String()[:8]

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM mcp_trust_score_history WHERE mcp_server_id = $1`, serverID)
		_, _ = db.ExecContext(ctx, `DELETE FROM mcp_trust_scores WHERE mcp_server_id = $1`, serverID)
		_, _ = db.ExecContext(ctx, `DELETE FROM mcp_servers WHERE id = $1`, serverID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, userID)
		_, _ = db.ExecContext(ctx, `DELETE FROM organizations WHERE id = $1`, orgID)
	})

	_, err = db.ExecContext(ctx,
		`INSERT INTO organizations (id, name, domain, created_at, updated_at)
		 VALUES ($1, $2, $3, NOW(), NOW())`,
		orgID, "mcp-update-writepath-org-"+suffix, "mcp-update-writepath-"+suffix+".test")
	require.NoError(t, err)

	_, err = db.ExecContext(ctx,
		`INSERT INTO users (id, organization_id, email, name, provider, provider_id, created_at, updated_at)
		 VALUES ($1, $2, $3, 'mcp-update-writepath-user', 'test', $4, NOW(), NOW())`,
		userID, orgID, "mcp-update-writepath-"+suffix+"@test.invalid",
		"mcp-update-writepath-"+suffix)
	require.NoError(t, err)

	// `verification_method` is scanned into a plain string by GetByID, so seed it
	// explicitly rather than relying on a column default.
	const staleScore = 0.1000
	_, err = db.ExecContext(ctx,
		`INSERT INTO mcp_servers
		   (id, organization_id, name, description, url, version, public_key,
		    status, is_verified, verification_method, trust_score,
		    created_by, created_at, updated_at)
		 VALUES ($1, $2, $3, 'seeded description', $4, '1.0.0', 'test-key',
		         'pending', false, 'manual', $5, $6, NOW(), NOW())`,
		serverID, orgID,
		"mcp-update-writepath-server-"+suffix,
		"https://mcp-update-writepath-"+suffix+".example.com",
		staleScore, userID)
	require.NoError(t, err)

	repo := NewMCPServerRepository(db)

	// 1. Take the snapshot BEFORE the score moves. This is the in-memory value an
	//    ordinary caller holds while it edits an unrelated field.
	snapshot, err := repo.GetByID(serverID)
	require.NoError(t, err)
	require.InDelta(t, staleScore, snapshot.TrustScore, 0.00001,
		"precondition: the snapshot must start at the pre-calculation score")

	historyBefore := countTrustScoreHistory(t, db, ctx, serverID)

	// 2. The calculator produces a real score. Inserting into `mcp_trust_scores`
	//    fires the migration 094 mirror, which moves `mcp_servers.trust_score` and
	//    legitimately records the change in history. The snapshot is now stale.
	const calculatedScore = 0.8234
	_, err = db.ExecContext(ctx,
		`INSERT INTO mcp_trust_scores
		   (id, mcp_server_id, score, factors, confidence, last_calculated, created_at)
		 VALUES (gen_random_uuid(), $1, $2, '{}'::jsonb, 0.95, NOW(), NOW())`,
		serverID, calculatedScore)
	require.NoError(t, err)

	require.InDelta(t, calculatedScore, readCachedTrustScore(t, db, ctx, serverID), 0.00001,
		"precondition: the migration 094 mirror must have moved the cache")

	historyAfterCalculation := countTrustScoreHistory(t, db, ctx, serverID)
	require.Equal(t, historyBefore+1, historyAfterCalculation,
		"precondition: a genuine recalculation must still record exactly one history row — "+
			"if this fails the audit trigger is not firing and the rest of this test is vacuous")

	// 3. Save the stale snapshot with an unrelated field changed. This is the whole
	//    defect: a description edit must not be able to touch a trust score.
	const patchedDescription = "description patched by an unrelated caller"
	snapshot.Description = patchedDescription
	require.NoError(t, repo.Update(snapshot))

	// The write must actually have landed. Without this the assertions below would
	// also pass for an Update that silently did nothing.
	var storedDescription string
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT description FROM mcp_servers WHERE id = $1`, serverID,
	).Scan(&storedDescription))
	require.Equal(t, patchedDescription, storedDescription,
		"the unrelated field must still be written; otherwise this test proves nothing")

	// 4a and 4b are deliberately `assert`, not `require`: they describe two
	// distinct consequences of the same write, and on unfixed code BOTH fail. A
	// fatal 4a would abort before 4b was ever evaluated, which would leave the
	// audit-trail assertion — the one that actually motivates this fix — unproven
	// against the defect it was written for.

	// 4a. The cache must still hold the calculated score, not the snapshot's.
	assert.InDelta(t, calculatedScore, readCachedTrustScore(t, db, ctx, serverID), 0.00001,
		"Update must not revert mcp_servers.trust_score to the caller's stale snapshot value")

	// 4b. And no audit row may have been invented. This is the assertion that
	//     matters most: a reverted cache is a bug, but a fabricated
	//     'automated_recalculation' row is a false security event in an audit trail.
	assert.Equal(t, historyAfterCalculation, countTrustScoreHistory(t, db, ctx, serverID),
		"Update must not fire mcp_trust_score_change_trigger — a fabricated "+
			"'automated_recalculation' row describes a recalculation that never ran")
}

func readCachedTrustScore(t *testing.T, db *sql.DB, ctx context.Context, serverID uuid.UUID) float64 {
	t.Helper()
	var score float64
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT trust_score FROM mcp_servers WHERE id = $1`, serverID,
	).Scan(&score))
	return score
}

func countTrustScoreHistory(t *testing.T, db *sql.DB, ctx context.Context, serverID uuid.UUID) int {
	t.Helper()
	var n int
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM mcp_trust_score_history WHERE mcp_server_id = $1`, serverID,
	).Scan(&n))
	return n
}
