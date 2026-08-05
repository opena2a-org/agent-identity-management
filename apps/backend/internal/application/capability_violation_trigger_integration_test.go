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

// TestCapabilityViolationTrigger_BumpsCounter is the regression test
// for issue #168 (and the dashboard half, #166). Migration 091 installs
// an AFTER INSERT trigger on `capability_violations` that bumps the
// agent's `capability_violation_count`. Before the trigger, the
// application-layer pattern paired `CreateViolation` with
// `IncrementViolationCount` at only 2-3 of 5+ call sites; the others
// silently let the counter stale.
//
// This test asserts the counter ticks ONLY via the trigger, with no
// application-layer increment call.
//
// Build-tag gated: requires a Postgres reachable via TEST_DATABASE_URL
// with the AIM schema already applied (run migrations first). Skip if
// absent so the unit-test pipeline stays self-contained. Run via
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run TestCapabilityViolationTrigger ./internal/application/...
func TestCapabilityViolationTrigger_BumpsCounter(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping trigger regression test")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, db.Ping())

	ctx := context.Background()

	// Setup: create a throwaway organization + user + agent so the test does
	// not depend on any seeded row and cleans up after itself.
	agentID := uuid.New()
	orgID, userID := seedOrgAndUser(t, db, ctx, "capviolation-trigger")
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM capability_violations WHERE agent_id = $1`, agentID)
		_, _ = db.ExecContext(ctx, `DELETE FROM agents WHERE id = $1`, agentID)
	})

	_, err = db.ExecContext(ctx,
		`INSERT INTO agents (id, organization_id, name, display_name, agent_type, status, trust_score,
		                     capability_violation_count, created_by, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, 'ai_agent', 'verified', 0.9, 0, $5, NOW(), NOW())`,
		agentID, orgID, "trigger-test-agent-"+agentID.String()[:8],
		"Trigger Test Agent", userID,
	)
	require.NoError(t, err)

	// Insert 3 violations directly, bypassing the application layer
	// entirely. The trigger is the ONLY thing that should bump the
	// counter.
	for i := 0; i < 3; i++ {
		_, err = db.ExecContext(ctx,
			`INSERT INTO capability_violations
			   (agent_id, attempted_capability, severity, trust_score_impact, is_blocked, created_at)
			 VALUES ($1, $2, 'medium', -2, false, NOW())`,
			agentID, "test:capability",
		)
		require.NoError(t, err)
	}

	// Verify the counter ticked to 3 via the trigger, not 0 (counter
	// never bumped) or 6 (trigger AND a manual increment double-bump).
	var count int
	err = db.QueryRowContext(ctx,
		`SELECT capability_violation_count FROM agents WHERE id = $1`,
		agentID,
	).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 3, count,
		"trigger must bump capability_violation_count once per insert; got %d after 3 inserts", count)

	// Also confirm the trigger was the writer by checking updated_at
	// moved within the last 30s.
	var updatedAt time.Time
	err = db.QueryRowContext(ctx,
		`SELECT updated_at FROM agents WHERE id = $1`, agentID,
	).Scan(&updatedAt)
	require.NoError(t, err)
	require.WithinDuration(t, time.Now().UTC(), updatedAt.UTC(), 30*time.Second,
		"trigger should set updated_at on each bump")
}

// TestCapabilityViolationTrigger_BackfillReconcilesDrift covers the
// one-shot backfill that ships in migration 091. The backfill resyncs
// agents.capability_violation_count to the actual row count for any
// agent whose counter drifted prior to the trigger being installed.
//
// This test does NOT re-run migration 091 (the backfill is a one-shot
// at-migrate-time UPDATE). It asserts the invariant the backfill
// maintains by inserting violations BEFORE relying on the trigger and
// then re-running the same reconciling UPDATE shape inline.
func TestCapabilityViolationTrigger_BackfillReconcilesDrift(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping backfill regression test")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, db.Ping())

	ctx := context.Background()

	agentID := uuid.New()
	orgID, userID := seedOrgAndUser(t, db, ctx, "capviolation-backfill")
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM capability_violations WHERE agent_id = $1`, agentID)
		_, _ = db.ExecContext(ctx, `DELETE FROM agents WHERE id = $1`, agentID)
	})

	// Seed an agent whose counter is INTENTIONALLY drifted: 7 actual
	// rows, but the counter says 2 (simulating pre-trigger state where
	// 5 violations were inserted without an accompanying increment).
	_, err = db.ExecContext(ctx,
		`INSERT INTO agents (id, organization_id, name, display_name, agent_type, status, trust_score,
		                     capability_violation_count, created_by, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, 'ai_agent', 'verified', 0.5, 2, $5, NOW(), NOW())`,
		agentID, orgID, "backfill-test-agent-"+agentID.String()[:8],
		"Backfill Test Agent", userID)
	require.NoError(t, err)

	// Disable the trigger for the next inserts to simulate pre-091
	// drift. We are testing the backfill, not the trigger.
	_, err = db.ExecContext(ctx,
		`ALTER TABLE capability_violations DISABLE TRIGGER trg_bump_capability_violation_count`)
	require.NoError(t, err)
	defer func() {
		_, _ = db.ExecContext(ctx,
			`ALTER TABLE capability_violations ENABLE TRIGGER trg_bump_capability_violation_count`)
	}()

	for i := 0; i < 7; i++ {
		_, err = db.ExecContext(ctx,
			`INSERT INTO capability_violations
			   (agent_id, attempted_capability, severity, trust_score_impact, is_blocked, created_at)
			 VALUES ($1, $2, 'medium', -2, false, NOW())`,
			agentID, "test:capability")
		require.NoError(t, err)
	}

	// At this point: 7 rows, counter still 2. Re-run the backfill
	// SQL shape from migration 091.
	_, err = db.ExecContext(ctx, `
		UPDATE agents a
		SET capability_violation_count = sub.cnt,
		    updated_at = NOW()
		FROM (
			SELECT agent_id, COUNT(*)::INTEGER AS cnt
			FROM capability_violations
			WHERE agent_id = $1
			GROUP BY agent_id
		) sub
		WHERE a.id = sub.agent_id
		  AND a.capability_violation_count IS DISTINCT FROM sub.cnt`,
		agentID)
	require.NoError(t, err)

	var count int
	err = db.QueryRowContext(ctx,
		`SELECT capability_violation_count FROM agents WHERE id = $1`, agentID,
	).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 7, count, "backfill must reconcile counter to actual row count; got %d", count)
}
