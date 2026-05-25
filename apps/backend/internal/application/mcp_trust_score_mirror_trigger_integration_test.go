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

// TestMCPTrustScoreMirrorTrigger_InsertSyncsServerCache is the
// regression test for issue #170. Migration 094 installs an
// AFTER INSERT OR UPDATE trigger on mcp_trust_scores that mirrors
// NEW.score onto mcp_servers.trust_score.
//
// Build-tag gated: requires Postgres reachable via TEST_DATABASE_URL.
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run TestMCPTrustScoreMirrorTrigger ./internal/application/...
func TestMCPTrustScoreMirrorTrigger_InsertSyncsServerCache(t *testing.T) {
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
	serverID := uuid.New()
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM mcp_trust_scores WHERE mcp_server_id = $1`, serverID)
		_, _ = db.ExecContext(ctx, `DELETE FROM mcp_servers WHERE id = $1`, serverID)
		_, _ = db.ExecContext(ctx, `DELETE FROM organizations WHERE id = $1`, orgID)
	})

	_, err = db.ExecContext(ctx,
		`INSERT INTO organizations (id, name, created_at, updated_at)
		 VALUES ($1, $2, NOW(), NOW())`,
		orgID, "mcp-mirror-test-org-"+serverID.String()[:8])
	require.NoError(t, err)

	// Seed MCP server with a stale cache value.
	_, err = db.ExecContext(ctx,
		`INSERT INTO mcp_servers
		   (id, organization_id, name, url, version, trust_score, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, '1.0.0', 0.50, NOW(), NOW())`,
		serverID, orgID,
		"mcp-mirror-test-server-"+serverID.String()[:8],
		"https://mcp-mirror-test-"+serverID.String()[:8]+".example.com")
	require.NoError(t, err)

	// INSERT a new score row. Trigger must mirror to mcp_servers.trust_score.
	_, err = db.ExecContext(ctx,
		`INSERT INTO mcp_trust_scores
		   (id, mcp_server_id, score, factors, confidence, last_calculated, created_at)
		 VALUES (gen_random_uuid(), $1, 0.8234, '{}'::jsonb, 0.95, NOW(), NOW())`,
		serverID)
	require.NoError(t, err)

	var cached float64
	err = db.QueryRowContext(ctx,
		`SELECT trust_score FROM mcp_servers WHERE id = $1`, serverID,
	).Scan(&cached)
	require.NoError(t, err)
	// mcp_servers.trust_score is DECIMAL(5,2); mcp_trust_scores.score is
	// DECIMAL(5,4). The mirrored value gets rounded to 2 decimals on
	// storage, so 0.8234 -> 0.82. Tolerance reflects that.
	require.InDelta(t, 0.82, cached, 0.01,
		"trigger must mirror mcp_trust_scores.score onto mcp_servers.trust_score on INSERT")
}

// TestMCPTrustScoreMirrorTrigger_UpdateSyncsLatestOnly asserts the
// guard: an UPDATE on a historical (non-latest) row must NOT
// clobber the cache.
func TestMCPTrustScoreMirrorTrigger_UpdateSyncsLatestOnly(t *testing.T) {
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
	serverID := uuid.New()
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM mcp_trust_scores WHERE mcp_server_id = $1`, serverID)
		_, _ = db.ExecContext(ctx, `DELETE FROM mcp_servers WHERE id = $1`, serverID)
		_, _ = db.ExecContext(ctx, `DELETE FROM organizations WHERE id = $1`, orgID)
	})

	_, err = db.ExecContext(ctx,
		`INSERT INTO organizations (id, name, created_at, updated_at)
		 VALUES ($1, $2, NOW(), NOW())`,
		orgID, "mcp-update-test-org-"+serverID.String()[:8])
	require.NoError(t, err)
	_, err = db.ExecContext(ctx,
		`INSERT INTO mcp_servers
		   (id, organization_id, name, url, version, trust_score, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, '1.0.0', 0.50, NOW(), NOW())`,
		serverID, orgID,
		"mcp-update-test-server-"+serverID.String()[:8],
		"https://mcp-update-test-"+serverID.String()[:8]+".example.com")
	require.NoError(t, err)

	olderID := uuid.New()
	newerID := uuid.New()
	olderCreatedAt := time.Now().UTC().Add(-1 * time.Hour)

	_, err = db.ExecContext(ctx,
		`INSERT INTO mcp_trust_scores
		   (id, mcp_server_id, score, factors, confidence, last_calculated, created_at)
		 VALUES ($1, $2, 0.6000, '{}'::jsonb, 0.9, $3, $3)`,
		olderID, serverID, olderCreatedAt)
	require.NoError(t, err)

	// Newer row inserts; cache must move to 0.90.
	_, err = db.ExecContext(ctx,
		`INSERT INTO mcp_trust_scores
		   (id, mcp_server_id, score, factors, confidence, last_calculated, created_at)
		 VALUES ($1, $2, 0.9000, '{}'::jsonb, 0.95, NOW(), NOW())`,
		newerID, serverID)
	require.NoError(t, err)

	var cached float64
	err = db.QueryRowContext(ctx,
		`SELECT trust_score FROM mcp_servers WHERE id = $1`, serverID).Scan(&cached)
	require.NoError(t, err)
	require.InDelta(t, 0.90, cached, 0.01,
		"newer INSERT must move the cache")

	// UPDATE the older row: cache MUST NOT clobber.
	_, err = db.ExecContext(ctx,
		`UPDATE mcp_trust_scores SET score = 0.1000 WHERE id = $1`, olderID)
	require.NoError(t, err)
	err = db.QueryRowContext(ctx,
		`SELECT trust_score FROM mcp_servers WHERE id = $1`, serverID).Scan(&cached)
	require.NoError(t, err)
	require.InDelta(t, 0.90, cached, 0.01,
		"UPDATE on historical row must NOT clobber the cache")

	// UPDATE the latest row: cache must mirror.
	_, err = db.ExecContext(ctx,
		`UPDATE mcp_trust_scores SET score = 0.7500 WHERE id = $1`, newerID)
	require.NoError(t, err)
	err = db.QueryRowContext(ctx,
		`SELECT trust_score FROM mcp_servers WHERE id = $1`, serverID).Scan(&cached)
	require.NoError(t, err)
	require.InDelta(t, 0.75, cached, 0.01,
		"UPDATE on the latest row must mirror to the cache")
}
