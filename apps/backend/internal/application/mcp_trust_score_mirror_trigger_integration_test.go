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

// seedOrgAndUser creates the organization and user rows that `mcp_servers`
// requires via NOT NULL / foreign key. Returns both IDs and registers cleanup.
func seedOrgAndUser(t *testing.T, db *sql.DB, ctx context.Context, prefix string) (uuid.UUID, uuid.UUID) {
	t.Helper()
	orgID := uuid.New()
	userID := uuid.New()
	suffix := orgID.String()[:8]

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, userID)
	})

	_, err := db.ExecContext(ctx,
		`INSERT INTO organizations (id, name, domain, created_at, updated_at)
		 VALUES ($1, $2, $3, NOW(), NOW())`,
		orgID, prefix+"-org-"+suffix, prefix+"-"+suffix+".example.com")
	require.NoError(t, err)

	_, err = db.ExecContext(ctx,
		`INSERT INTO users
		   (id, organization_id, email, name, password_hash, role, provider,
		    provider_id, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, 'x', 'admin', 'local', $5, NOW(), NOW())`,
		userID, orgID, prefix+"-"+suffix+"@example.com", prefix+"-user", "local-"+suffix)
	require.NoError(t, err)

	return orgID, userID
}

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
	serverID := uuid.New()
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM mcp_trust_scores WHERE mcp_server_id = $1`, serverID)
		_, _ = db.ExecContext(ctx, `DELETE FROM mcp_servers WHERE id = $1`, serverID)
	})

	orgID, userID := seedOrgAndUser(t, db, ctx, "mcp-mirror")

	// Seed MCP server with a stale cache value.
	_, err = db.ExecContext(ctx,
		`INSERT INTO mcp_servers
		   (id, organization_id, name, url, version, public_key, trust_score,
		    created_by, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, '1.0.0', 'test-key', 0.50, $5, NOW(), NOW())`,
		serverID, orgID,
		"mcp-mirror-test-server-"+serverID.String()[:8],
		"https://mcp-mirror-test-"+serverID.String()[:8]+".example.com", userID)
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
	// Both columns are DECIMAL(5,4) since migration 104, so the cache holds
	// the source value exactly. This assertion previously tolerated 0.8234
	// arriving as 0.82, because `mcp_servers.trust_score` was DECIMAL(5,2)
	// and silently rounded every mirrored write — a cache that disagreed
	// with its source on every score, which is the #170 defect this trigger
	// exists to prevent.
	require.InDelta(t, 0.8234, cached, 0.00001,
		"trigger must mirror mcp_trust_scores.score onto mcp_servers.trust_score exactly")
}

// TestMCPServerTrustScore_RejectsOutOfScaleValue is the structural guard for
// the [CHIEF-CDS] decision of 2026-08-04: an MCP trust score is a [0,1] value
// produced by the 8-factor calculator, and the schema — not reviewer
// attention — is what keeps a literal on some other scale out of the column.
//
// Before migration 104 `mcp_servers.trust_score` was DECIMAL(5,2) with no
// CHECK constraint, so the application's hardcoded 75.0 stored cleanly and
// then sat above every [0,1] `MinTrustScore` policy threshold.
func TestMCPServerTrustScore_RejectsOutOfScaleValue(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping constraint regression test")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, db.Ping())

	ctx := context.Background()
	serverID := uuid.New()
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM mcp_servers WHERE id = $1`, serverID)
	})

	orgID, userID := seedOrgAndUser(t, db, ctx, "mcp-scale")

	insert := func(score string) error {
		_, err := db.ExecContext(ctx,
			`INSERT INTO mcp_servers
			   (id, organization_id, name, url, version, public_key, trust_score,
			    created_by, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, '1.0.0', 'test-key', `+score+`, $5, NOW(), NOW())`,
			serverID, orgID,
			"mcp-scale-test-server-"+serverID.String()[:8],
			"https://mcp-scale-test-"+serverID.String()[:8]+".example.com", userID)
		return err
	}

	// The exact literal the application used to write.
	require.Error(t, insert("75.0"),
		"75.0 must be rejected: it is the fabricated 0-100 literal")
	require.Error(t, insert("1.5"),
		"any value above 1.0 must be rejected")
	require.Error(t, insert("-0.1"),
		"any value below 0.0 must be rejected")

	// A legitimate calculated score must still store, at full precision.
	require.NoError(t, insert("0.8234"),
		"a calculated [0,1] score must be storable")

	var stored float64
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT trust_score FROM mcp_servers WHERE id = $1`, serverID).Scan(&stored))
	require.InDelta(t, 0.8234, stored, 0.00001,
		"the column must retain 4 decimal places to match mcp_trust_scores.score")
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
	serverID := uuid.New()
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM mcp_trust_scores WHERE mcp_server_id = $1`, serverID)
		_, _ = db.ExecContext(ctx, `DELETE FROM mcp_servers WHERE id = $1`, serverID)
	})

	orgID, userID := seedOrgAndUser(t, db, ctx, "mcp-update")
	_, err = db.ExecContext(ctx,
		`INSERT INTO mcp_servers
		   (id, organization_id, name, url, version, public_key, trust_score,
		    created_by, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, '1.0.0', 'test-key', 0.50, $5, NOW(), NOW())`,
		serverID, orgID,
		"mcp-update-test-server-"+serverID.String()[:8],
		"https://mcp-update-test-"+serverID.String()[:8]+".example.com", userID)
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
