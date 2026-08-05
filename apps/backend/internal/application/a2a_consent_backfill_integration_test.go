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

// TestA2AConsentBackfill_ConstraintEnforced is the regression test
// for issue #179. Migration 097 backfilled
// a2a_consent_records.organization_id from grantor agent and added
// a NOT NULL constraint. After migration, inserting a consent row
// without organization_id must be rejected.
//
// Build-tag gated: requires Postgres reachable via TEST_DATABASE_URL
// with migration 097 already applied.
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run TestA2AConsentBackfill ./internal/application/...
func TestA2AConsentBackfill_ConstraintEnforced(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping constraint regression test")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, db.Ping())

	ctx := context.Background()
	grantorID := uuid.New()
	recipientID := uuid.New()

	// Seeded before the cleanup below is registered, so that cleanup (agents,
	// consent records) runs first under t.Cleanup's LIFO order and the org/user
	// rows it depends on are removed last.
	orgID, userID := seedOrgAndUser(t, db, ctx, "consent-backfill")

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx,
			`DELETE FROM a2a_consent_records WHERE grantor_agent_id IN ($1, $2)`,
			grantorID, recipientID)
		_, _ = db.ExecContext(ctx, `DELETE FROM agents WHERE id IN ($1, $2)`,
			grantorID, recipientID)
	})

	for _, id := range []uuid.UUID{grantorID, recipientID} {
		_, err = db.ExecContext(ctx,
			`INSERT INTO agents (id, organization_id, name, display_name, agent_type, status, trust_score, created_by, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, 'ai_agent', 'verified', 0.5, $5, NOW(), NOW())`,
			id, orgID, "consent-backfill-agent-"+id.String()[:8],
			"Consent Backfill Agent", userID)
		require.NoError(t, err)
	}

	// Attempt to INSERT a consent record WITHOUT organization_id.
	// Must fail with a not-null violation.
	_, err = db.ExecContext(ctx,
		`INSERT INTO a2a_consent_records
		   (user_id, grantor_agent_id, recipient_agent_id,
		    scope, purpose, data_types, consent_method)
		 VALUES ($1, $2, $3, '[]'::jsonb, $4, '[]'::jsonb, 'api')`,
		"test-user-"+grantorID.String()[:8], grantorID, recipientID,
		"backfill constraint test")
	require.Error(t, err,
		"INSERT without organization_id must fail post-migration; got nil error")
	require.Contains(t, err.Error(), "organization_id",
		"error must reference the not-null column; got: %v", err)

	// Sanity check: same INSERT WITH organization_id succeeds.
	_, err = db.ExecContext(ctx,
		`INSERT INTO a2a_consent_records
		   (user_id, organization_id, grantor_agent_id, recipient_agent_id,
		    scope, purpose, data_types, consent_method)
		 VALUES ($1, $2, $3, $4, '[]'::jsonb, $5, '[]'::jsonb, 'api')`,
		"test-user-"+grantorID.String()[:8], orgID, grantorID, recipientID,
		"backfill positive test")
	require.NoError(t, err)
}

// TestA2AConsentBackfill_OrgIDDerivesFromGrantor asserts the
// backfill semantic: an existing NULL-org row's organization_id
// gets stamped with the grantor agent's organization_id. This is
// done by simulating the pre-migration state (insert with NULL,
// then re-run the backfill SQL inline since the actual migration
// is one-shot at migrate-time).
func TestA2AConsentBackfill_OrgIDDerivesFromGrantor(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping backfill semantic test")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, db.Ping())

	ctx := context.Background()
	grantorID := uuid.New()
	recipientID := uuid.New()
	consentID := uuid.New()

	orgID, userID := seedOrgAndUser(t, db, ctx, "consent-derive")

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM a2a_consent_records WHERE id = $1`, consentID)
		_, _ = db.ExecContext(ctx, `DELETE FROM agents WHERE id IN ($1, $2)`,
			grantorID, recipientID)
	})

	for _, id := range []uuid.UUID{grantorID, recipientID} {
		_, err = db.ExecContext(ctx,
			`INSERT INTO agents (id, organization_id, name, display_name, agent_type, status, trust_score, created_by, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, 'ai_agent', 'verified', 0.5, $5, NOW(), NOW())`,
			id, orgID, "consent-derive-agent-"+id.String()[:8],
			"Consent Derive Agent", userID)
		require.NoError(t, err)
	}

	// Simulate the pre-migration shape: temporarily drop the NOT
	// NULL constraint, insert a NULL-org row, then re-apply the
	// constraint to reset state. We test only the backfill SELECT;
	// the constraint itself is covered by the previous test.
	_, err = db.ExecContext(ctx,
		`ALTER TABLE a2a_consent_records ALTER COLUMN organization_id DROP NOT NULL`)
	require.NoError(t, err)
	defer func() {
		_, _ = db.ExecContext(ctx,
			`ALTER TABLE a2a_consent_records ALTER COLUMN organization_id SET NOT NULL`)
	}()

	_, err = db.ExecContext(ctx,
		`INSERT INTO a2a_consent_records
		   (id, user_id, organization_id, grantor_agent_id, recipient_agent_id,
		    scope, purpose, data_types, consent_method)
		 VALUES ($1, $2, NULL, $3, $4, '[]'::jsonb, $5, '[]'::jsonb, 'api')`,
		consentID, "test-user-"+grantorID.String()[:8], grantorID, recipientID,
		"pre-migration NULL-org row")
	require.NoError(t, err)

	// Run the same UPDATE shape that migration 097 ships.
	_, err = db.ExecContext(ctx, `
		UPDATE a2a_consent_records c
		SET organization_id = a.organization_id,
		    updated_at = NOW()
		FROM agents a
		WHERE c.grantor_agent_id = a.id
		  AND c.id = $1
		  AND c.organization_id IS NULL`, consentID)
	require.NoError(t, err)

	var derived uuid.UUID
	err = db.QueryRowContext(ctx,
		`SELECT organization_id FROM a2a_consent_records WHERE id = $1`, consentID,
	).Scan(&derived)
	require.NoError(t, err)
	require.Equal(t, orgID, derived,
		"backfill must derive organization_id from the grantor agent")
}
