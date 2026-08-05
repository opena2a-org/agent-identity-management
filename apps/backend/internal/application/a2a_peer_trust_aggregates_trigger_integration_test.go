//go:build integration

package application

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// TestA2APeerTrustAggregatesTrigger_RecomputesOnInsert is the
// regression test for issue #169. Migration 095 installs an
// AFTER INSERT/UPDATE/DELETE trigger on a2a_peer_trust that
// recomputes a2a_trust_scores.peer_trust_average and
// unique_peers_count from the actual peer rows, replacing the
// application-layer dual-write at a2a_repository.go:1700-1727.
//
// Build-tag gated: requires Postgres reachable via TEST_DATABASE_URL.
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run TestA2APeerTrustAggregatesTrigger ./internal/application/...
func TestA2APeerTrustAggregatesTrigger_RecomputesOnInsert(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping trigger regression test")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, db.Ping())

	ctx := context.Background()
	agentID := uuid.New()
	peerA := uuid.New()
	peerB := uuid.New()
	peerC := uuid.New()

	orgID, userID := seedOrgAndUser(t, db, ctx, "a2a-aggr")

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM a2a_peer_trust WHERE agent_id = $1 OR peer_agent_id IN ($2, $3, $4)`,
			agentID, peerA, peerB, peerC)
		_, _ = db.ExecContext(ctx, `DELETE FROM a2a_trust_scores WHERE agent_id = $1`, agentID)
		_, _ = db.ExecContext(ctx, `DELETE FROM agents WHERE id IN ($1, $2, $3, $4)`,
			agentID, peerA, peerB, peerC)
	})

	for _, id := range []uuid.UUID{agentID, peerA, peerB, peerC} {
		_, err = db.ExecContext(ctx,
			`INSERT INTO agents (id, organization_id, name, display_name, agent_type, status, trust_score, created_by, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, 'ai_agent', 'verified', 0.5, $5, NOW(), NOW())`,
			id, orgID, "a2a-aggr-agent-"+id.String()[:8], "A2A Aggr Agent", userID)
		require.NoError(t, err)
	}

	// INSERT three peer rows. The trigger must recompute the
	// aggregates after each insert.
	for i, peer := range []uuid.UUID{peerA, peerB, peerC} {
		_, err = db.ExecContext(ctx,
			`INSERT INTO a2a_peer_trust (agent_id, peer_agent_id, peer_trust_score, created_at, updated_at)
			 VALUES ($1, $2, $3, NOW(), NOW())`,
			agentID, peer, float64(i+1)*0.3)
		require.NoError(t, err)
	}

	var avg sql.NullFloat64
	var count sql.NullInt64
	err = db.QueryRowContext(ctx,
		`SELECT peer_trust_average, unique_peers_count FROM a2a_trust_scores WHERE agent_id = $1`,
		agentID).Scan(&avg, &count)
	require.NoError(t, err)
	require.True(t, avg.Valid)
	// 0.3, 0.6, 0.9 -> avg = 0.6
	require.InDelta(t, 0.6, avg.Float64, 0.001,
		"trigger must compute peer_trust_average from actual peer rows")
	require.True(t, count.Valid)
	require.Equal(t, int64(3), count.Int64,
		"trigger must compute unique_peers_count from actual peer rows")
}

// TestA2APeerTrustAggregatesTrigger_DeleteShrinksAggregates asserts
// that DELETEing a peer row recomputes the aggregates DOWNWARD —
// the audit-doc concern about "deleted row -> aggregate lies."
func TestA2APeerTrustAggregatesTrigger_DeleteShrinksAggregates(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping trigger regression test")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, db.Ping())

	ctx := context.Background()
	agentID := uuid.New()
	peerA := uuid.New()
	peerB := uuid.New()

	orgID, userID := seedOrgAndUser(t, db, ctx, "a2a-del")

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM a2a_peer_trust WHERE agent_id = $1 OR peer_agent_id IN ($2, $3)`,
			agentID, peerA, peerB)
		_, _ = db.ExecContext(ctx, `DELETE FROM a2a_trust_scores WHERE agent_id = $1`, agentID)
		_, _ = db.ExecContext(ctx, `DELETE FROM agents WHERE id IN ($1, $2, $3)`, agentID, peerA, peerB)
	})

	for _, id := range []uuid.UUID{agentID, peerA, peerB} {
		_, err = db.ExecContext(ctx,
			`INSERT INTO agents (id, organization_id, name, display_name, agent_type, status, trust_score, created_by, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, 'ai_agent', 'verified', 0.5, $5, NOW(), NOW())`,
			id, orgID, "a2a-del-agent-"+id.String()[:8], "A2A Del Agent", userID)
		require.NoError(t, err)
	}

	_, err = db.ExecContext(ctx,
		`INSERT INTO a2a_peer_trust (agent_id, peer_agent_id, peer_trust_score, created_at, updated_at)
		 VALUES ($1, $2, 0.8, NOW(), NOW()), ($1, $3, 0.4, NOW(), NOW())`,
		agentID, peerA, peerB)
	require.NoError(t, err)

	var avg sql.NullFloat64
	var count sql.NullInt64
	err = db.QueryRowContext(ctx,
		`SELECT peer_trust_average, unique_peers_count FROM a2a_trust_scores WHERE agent_id = $1`,
		agentID).Scan(&avg, &count)
	require.NoError(t, err)
	require.InDelta(t, 0.6, avg.Float64, 0.001)
	require.Equal(t, int64(2), count.Int64)

	// DELETE the higher-scoring peer row. Aggregate must shrink.
	_, err = db.ExecContext(ctx,
		`DELETE FROM a2a_peer_trust WHERE agent_id = $1 AND peer_agent_id = $2`, agentID, peerA)
	require.NoError(t, err)

	err = db.QueryRowContext(ctx,
		`SELECT peer_trust_average, unique_peers_count FROM a2a_trust_scores WHERE agent_id = $1`,
		agentID).Scan(&avg, &count)
	require.NoError(t, err)
	require.InDelta(t, 0.4, avg.Float64, 0.001,
		"DELETE must recompute peer_trust_average to reflect remaining rows")
	require.Equal(t, int64(1), count.Int64,
		"DELETE must recompute unique_peers_count downward")

	// DELETE the last peer row.
	_, err = db.ExecContext(ctx,
		`DELETE FROM a2a_peer_trust WHERE agent_id = $1 AND peer_agent_id = $2`, agentID, peerB)
	require.NoError(t, err)

	err = db.QueryRowContext(ctx,
		`SELECT peer_trust_average, unique_peers_count FROM a2a_trust_scores WHERE agent_id = $1`,
		agentID).Scan(&avg, &count)
	require.NoError(t, err)
	require.False(t, avg.Valid, "with no peer rows, peer_trust_average should be NULL")
	require.Equal(t, int64(0), count.Int64, "with no peer rows, unique_peers_count should be 0")
}
