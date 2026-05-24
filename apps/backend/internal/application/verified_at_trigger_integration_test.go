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

// TestVerifiedAtTrigger_SetsOnStatusTransition is the regression
// test for issue #167's `verified_at` half. Migration 092 installs a
// BEFORE UPDATE trigger on `agents` that sets `verified_at = NOW()`
// when status transitions to 'verified' from any other state and
// `verified_at` is still NULL.
//
// The audit doc § 10 observed agents whose dashboard rendered
// "Verified at: never" because a direct-SQL UPDATE path flipped
// status to 'verified' without setting `verified_at`. With the
// trigger, no direct-SQL path can leave this gap.
//
// Build-tag gated: requires Postgres reachable via TEST_DATABASE_URL
// with the AIM schema already applied (run migrations first).
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run TestVerifiedAtTrigger ./internal/application/...
func TestVerifiedAtTrigger_SetsOnStatusTransition(t *testing.T) {
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
		_, _ = db.ExecContext(ctx, `DELETE FROM agents WHERE id = $1`, agentID)
		_, _ = db.ExecContext(ctx, `DELETE FROM organizations WHERE id = $1`, orgID)
	})

	_, err = db.ExecContext(ctx,
		`INSERT INTO organizations (id, name, created_at, updated_at)
		 VALUES ($1, $2, NOW(), NOW())`,
		orgID, "verified-at-trigger-test-org-"+agentID.String()[:8])
	require.NoError(t, err)

	// Seed an agent in 'pending' state with NULL verified_at — the
	// pre-flip state.
	_, err = db.ExecContext(ctx,
		`INSERT INTO agents (id, organization_id, name, agent_type, status, trust_score,
		                     verified_at, created_at, updated_at)
		 VALUES ($1, $2, $3, 'ai_agent', 'pending', 0.5, NULL, NOW(), NOW())`,
		agentID, orgID, "verified-at-trigger-test-agent-"+agentID.String()[:8])
	require.NoError(t, err)

	// Direct-SQL status flip: this is the path the audit observed
	// leaving verified_at NULL. With the trigger, verified_at must
	// be set even though we did not touch it in the UPDATE.
	beforeFlip := time.Now().UTC()
	_, err = db.ExecContext(ctx,
		`UPDATE agents SET status = 'verified' WHERE id = $1`,
		agentID)
	require.NoError(t, err)

	var verifiedAt sql.NullTime
	err = db.QueryRowContext(ctx,
		`SELECT verified_at FROM agents WHERE id = $1`, agentID,
	).Scan(&verifiedAt)
	require.NoError(t, err)
	require.True(t, verifiedAt.Valid,
		"trigger must set verified_at when status transitions to 'verified'")
	require.WithinDuration(t, beforeFlip, verifiedAt.Time.UTC(), 30*time.Second,
		"trigger should stamp verified_at within the same UPDATE window")
}

// TestVerifiedAtTrigger_DoesNotOverwriteExisting asserts the trigger
// is idempotent: an UPDATE that does not change status, or that
// transitions FROM verified to something else and back, must not
// clobber a verified_at that was already set by the application
// layer (which would falsify the audit trail of when the agent was
// first verified).
func TestVerifiedAtTrigger_DoesNotOverwriteExisting(t *testing.T) {
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
		_, _ = db.ExecContext(ctx, `DELETE FROM agents WHERE id = $1`, agentID)
		_, _ = db.ExecContext(ctx, `DELETE FROM organizations WHERE id = $1`, orgID)
	})

	_, err = db.ExecContext(ctx,
		`INSERT INTO organizations (id, name, created_at, updated_at)
		 VALUES ($1, $2, NOW(), NOW())`,
		orgID, "verified-at-noclobber-test-org-"+agentID.String()[:8])
	require.NoError(t, err)

	// Seed an agent already in 'verified' state with a known
	// verified_at three days ago.
	originalVerifiedAt := time.Now().UTC().Add(-72 * time.Hour).Truncate(time.Second)
	_, err = db.ExecContext(ctx,
		`INSERT INTO agents (id, organization_id, name, agent_type, status, trust_score,
		                     verified_at, created_at, updated_at)
		 VALUES ($1, $2, $3, 'ai_agent', 'verified', 0.8, $4, NOW(), NOW())`,
		agentID, orgID, "verified-at-noclobber-test-agent-"+agentID.String()[:8],
		originalVerifiedAt)
	require.NoError(t, err)

	// UPDATE that does NOT touch status. The trigger fires (BEFORE
	// UPDATE FOR EACH ROW), but the guard `OLD.status <> 'verified'`
	// must skip the assignment, preserving the original timestamp.
	_, err = db.ExecContext(ctx,
		`UPDATE agents SET trust_score = 0.7 WHERE id = $1`,
		agentID)
	require.NoError(t, err)

	var verifiedAt sql.NullTime
	err = db.QueryRowContext(ctx,
		`SELECT verified_at FROM agents WHERE id = $1`, agentID,
	).Scan(&verifiedAt)
	require.NoError(t, err)
	require.True(t, verifiedAt.Valid)
	require.WithinDuration(t, originalVerifiedAt, verifiedAt.Time.UTC(), time.Second,
		"trigger must NOT overwrite verified_at on unrelated UPDATEs")

	// Re-verifying after a suspension: status flips verified -> suspended
	// -> verified. The trigger's NULL guard means the second transition
	// preserves the original verified_at (agent was first verified
	// three days ago; re-verification does not reset that history).
	_, err = db.ExecContext(ctx,
		`UPDATE agents SET status = 'suspended' WHERE id = $1`, agentID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx,
		`UPDATE agents SET status = 'verified' WHERE id = $1`, agentID)
	require.NoError(t, err)

	err = db.QueryRowContext(ctx,
		`SELECT verified_at FROM agents WHERE id = $1`, agentID,
	).Scan(&verifiedAt)
	require.NoError(t, err)
	require.True(t, verifiedAt.Valid)
	require.WithinDuration(t, originalVerifiedAt, verifiedAt.Time.UTC(), time.Second,
		"re-verification must not clobber the first-verified timestamp")
}
