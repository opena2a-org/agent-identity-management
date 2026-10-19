//go:build integration

package repository

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// The account deletion purge deletes an organization it leaves with no user,
// held against PostgreSQL with every migration applied. Migration 140 gives the
// three references to organizations(id) that had no ON DELETE rule a cascade, so
// the organization's delete is not refused by them.
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run 'TestOrganizationDelete|TestUserDeletionPurge' ./internal/infrastructure/repository/...

const organizationDeleteMigration = "140_organization_references_allow_organization_delete.sql"

// The references migration 140 gave ON DELETE CASCADE.
var organizationDeleteCascades = []string{
	"a2a_consent_records.organization_id",
	"community_intelligence_reports.organization_id",
	"device_codes.organization_id",
}

// The references to organizations(id) that keep ON DELETE RESTRICT: a signed
// audit record chain is not removed by cascade (migrations 118 and 119).
var organizationDeleteRestricted = map[string]bool{
	"record_chains.organization_id": true,
	"record_debts.organization_id":  true,
}

// Every reference to organizations(id) lets the organization it names be
// deleted: its ON DELETE rule is CASCADE or SET NULL, except the two references
// of the record chain tables, which keep RESTRICT. A reference added with no
// rule (NO ACTION, the default) fails here. The three references migration 140
// changed cascade.
func TestOrganizationDeleteEveryReferenceToOrganizationsHasADeleteRule(t *testing.T) {
	db := userDeleteTestDB(t)
	ctx := context.Background()

	rows, err := db.QueryContext(ctx, `
		SELECT c.conrelid::regclass::text, a.attname::text, c.confdeltype::text
		FROM pg_constraint c
		JOIN pg_attribute a
		  ON a.attrelid = c.conrelid
		 AND a.attnum = ANY (c.conkey)
		WHERE c.contype = 'f'
		  AND c.confrelid = 'organizations'::regclass
		ORDER BY 1, 2`)
	require.NoError(t, err)
	defer rows.Close()

	rules := make(map[string]string)
	var refusing []string
	for rows.Next() {
		var table, column, rule string
		require.NoError(t, rows.Scan(&table, &column, &rule))
		ref := table + "." + column
		rules[ref] = rule
		if !organizationDeleteRestricted[ref] && rule != "c" && rule != "n" {
			refusing = append(refusing, fmt.Sprintf("%s (%s)", ref, userDeleteRuleName(rule)))
		}
	}
	require.NoError(t, rows.Err())
	require.Contains(t, rules, "users.organization_id",
		"the catalog must list the references to organizations(id), or this test proves nothing")

	assert.Empty(t, refusing,
		"each of these references to organizations(id) refuses the delete of an organization it names; give it ON DELETE CASCADE or ON DELETE SET NULL")
	for ref := range organizationDeleteRestricted {
		assert.Equal(t, "RESTRICT", userDeleteRuleName(rules[ref]), "the ON DELETE rule of %s", ref)
	}
	for _, ref := range organizationDeleteCascades {
		assert.Equal(t, "CASCADE", userDeleteRuleName(rules[ref]), "the ON DELETE rule of %s", ref)
	}
}

// Migration 140 runs again on a migrated database: each of the three columns
// keeps one foreign key to organizations(id), with ON DELETE CASCADE and the
// name the migration gives it. It runs in a transaction that is rolled back, so
// the shared database is left as it was.
func TestOrganizationDeleteMigrationRunsAgain(t *testing.T) {
	db := userDeleteTestDB(t)
	ctx := context.Background()

	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, userDeleteReadMigration(t, organizationDeleteMigration))
	require.NoError(t, err, "the migration runs again")

	rows, err := tx.QueryContext(ctx, `
		SELECT c.conrelid::regclass::text || '.' || a.attname::text, c.conname::text, c.confdeltype::text
		FROM pg_constraint c
		JOIN pg_attribute a
		  ON a.attrelid = c.conrelid
		 AND a.attnum = ANY (c.conkey)
		WHERE c.contype = 'f'
		  AND c.confrelid = 'organizations'::regclass
		  AND c.conrelid IN ('a2a_consent_records'::regclass,
		                     'community_intelligence_reports'::regclass,
		                     'device_codes'::regclass)
		ORDER BY 1`)
	require.NoError(t, err)
	defer rows.Close()

	var got []string
	for rows.Next() {
		var ref, name, rule string
		require.NoError(t, rows.Scan(&ref, &name, &rule))
		got = append(got, ref+" "+name+" "+userDeleteRuleName(rule))
	}
	require.NoError(t, rows.Err())
	assert.Equal(t, []string{
		"a2a_consent_records.organization_id a2a_consent_records_organization_id_fkey CASCADE",
		"community_intelligence_reports.organization_id community_intelligence_reports_organization_id_fkey CASCADE",
		"device_codes.organization_id device_codes_organization_id_fkey CASCADE",
	}, got)
}

// The purge deletes the only user of an organization and then the organization,
// with the rows that reference it: its agent, MCP server and API key, and an A2A
// consent record and a community intelligence report, the two rows whose
// references refused the delete before migration 140. The two accounts of an
// organization whose two users were due in the same batch both carry
// OrganizationDeleted. An organization with a user left is kept.
func TestUserDeletionPurgeDeletesTheOrganizationItLeavesWithNoUser(t *testing.T) {
	db := userDeleteTestDB(t)
	ctx := context.Background()
	repo := NewUserRepository(db)
	at := time.Now().UTC().Add(-50 * 24 * time.Hour).Truncate(time.Microsecond)

	alone := seedTenant(t, db, ctx, "deletion-org-alone")
	name, email := userDeleteNameAndEmail(t, db, ctx, alone.userID)
	serverID := userDeleteCopiesServer(t, db, ctx, NewMCPServerRepository(db), alone.orgID, alone.userID, name, email)
	keyID := userDeleteInsertAPIKey(t, db, ctx, alone, alone.userID)
	consentID := organizationDeleteInsertConsent(t, db, ctx, alone)
	reportID := organizationDeleteInsertReport(t, db, ctx, alone.orgID)
	_, err := repo.RequestDeletion(ctx, alone.userID, at)
	require.NoError(t, err)

	pair := seedTenant(t, db, ctx, "deletion-org-pair")
	second := userDeletionSeedUser(t, db, ctx, pair.orgID, "member", "active")
	_, err = repo.RequestDeletion(ctx, second, at)
	require.NoError(t, err)
	_, err = repo.RequestDeletion(ctx, pair.userID, at)
	require.NoError(t, err, "the admin asks once the other user is deactivated")

	stays := seedTenant(t, db, ctx, "deletion-org-stays")
	member := userDeletionSeedUser(t, db, ctx, stays.orgID, "member", "active")
	_, err = repo.RequestDeletion(ctx, member, at)
	require.NoError(t, err)

	due, err := repo.PurgeDeletionRequests(ctx, at, domain.AccountDeletionPurgeBatch)
	require.NoError(t, err)

	mine := userDeletionsOfOrg(due, alone.orgID)
	require.Len(t, mine, 1)
	assert.Equal(t, alone.userID, mine[0].UserID)
	assert.True(t, mine[0].OrganizationDeleted, "the organization had no user left")
	for _, c := range []struct {
		what, query string
		id          uuid.UUID
	}{
		{"the organization", `SELECT COUNT(*) FROM organizations WHERE id = $1`, alone.orgID},
		{"the agent", `SELECT COUNT(*) FROM agents WHERE id = $1`, alone.agentID},
		{"the MCP server", `SELECT COUNT(*) FROM mcp_servers WHERE id = $1`, serverID},
		{"the API key", `SELECT COUNT(*) FROM api_keys WHERE id = $1`, keyID},
		{"the consent record", `SELECT COUNT(*) FROM a2a_consent_records WHERE id = $1`, consentID},
		{"the intelligence report", `SELECT COUNT(*) FROM community_intelligence_reports WHERE id = $1`, reportID},
	} {
		assert.Equal(t, 0, userDeletionCount(t, db, ctx, c.query, c.id), "%s is deleted", c.what)
	}

	both := userDeletionsOfOrg(due, pair.orgID)
	assert.ElementsMatch(t, []uuid.UUID{second, pair.userID}, userDeletionIDs(both))
	for _, d := range both {
		assert.True(t, d.OrganizationDeleted, "user %s was one of the organization's last two users", d.UserID)
	}
	assert.Equal(t, 0, userDeletionCount(t, db, ctx, `SELECT COUNT(*) FROM organizations WHERE id = $1`, pair.orgID))

	kept := userDeletionsOfOrg(due, stays.orgID)
	require.Len(t, kept, 1)
	assert.Equal(t, member, kept[0].UserID)
	assert.False(t, kept[0].OrganizationDeleted, "the admin is left")
	assert.Equal(t, 1, userDeletionCount(t, db, ctx, `SELECT COUNT(*) FROM organizations WHERE id = $1`, stays.orgID))
	assert.Equal(t, 1, userDeletionCount(t, db, ctx, `SELECT COUNT(*) FROM agents WHERE id = $1`, stays.agentID))
}

// An organization whose delete a reference refuses is kept: here it has a
// signed audit record chain, whose reference has ON DELETE RESTRICT. The purge
// deletes its account all the same, without OrganizationDeleted, logs the
// refusal, and still deletes the other organization of the batch that it leaves
// with no user.
func TestUserDeletionPurgeKeepsAnOrganizationWhoseDeleteIsRefused(t *testing.T) {
	db := userDeleteTestDB(t)
	ctx := context.Background()
	repo := NewUserRepository(db)
	at := time.Now().UTC().Add(-60 * 24 * time.Hour).Truncate(time.Microsecond)

	chained := seedTenant(t, db, ctx, "deletion-org-chained")
	organizationDeleteInsertChain(t, db, ctx, chained.orgID)
	_, err := repo.RequestDeletion(ctx, chained.userID, at)
	require.NoError(t, err)

	plain := seedTenant(t, db, ctx, "deletion-org-plain")
	_, err = repo.RequestDeletion(ctx, plain.userID, at)
	require.NoError(t, err)

	logs := organizationDeleteCaptureLog(t)
	due, err := repo.PurgeDeletionRequests(ctx, at, domain.AccountDeletionPurgeBatch)
	require.NoError(t, err, "a refused organization delete does not fail the purge")

	refused := userDeletionsOfOrg(due, chained.orgID)
	require.Len(t, refused, 1)
	assert.Equal(t, chained.userID, refused[0].UserID)
	assert.False(t, refused[0].OrganizationDeleted)
	assert.Equal(t, 0, userDeletionCount(t, db, ctx, `SELECT COUNT(*) FROM users WHERE id = $1`, chained.userID),
		"the account is deleted")
	assert.Equal(t, 1, userDeletionCount(t, db, ctx, `SELECT COUNT(*) FROM organizations WHERE id = $1`, chained.orgID),
		"the organization is kept")
	assert.Equal(t, 1, userDeletionCount(t, db, ctx, `SELECT COUNT(*) FROM agents WHERE id = $1`, chained.agentID),
		"the organization's agent is kept with it")
	assert.Contains(t, logs.String(),
		"Account deletion purge: organization "+chained.orgID.String()+" has no user left and is kept: its delete was refused")
	assert.Contains(t, logs.String(), "record_chains_organization_id_fkey")

	deleted := userDeletionsOfOrg(due, plain.orgID)
	require.Len(t, deleted, 1)
	assert.True(t, deleted[0].OrganizationDeleted)
	assert.Equal(t, 0, userDeletionCount(t, db, ctx, `SELECT COUNT(*) FROM organizations WHERE id = $1`, plain.orgID))
	assert.NotContains(t, logs.String(), plain.orgID.String())
}

// A user that another transaction adds to the organization while the purge
// runs keeps the organization. The transaction is still open when the purge has
// deleted the organization's last due user; the purge waits for it, and once it
// commits, counts its user. Neither the organization nor that user is deleted.
func TestUserDeletionPurgeKeepsAnOrganizationThatGainsAUserMeanwhile(t *testing.T) {
	db := userDeleteTestDB(t)
	ctx := context.Background()
	repo := NewUserRepository(db)
	at := time.Now().UTC().Add(-70 * 24 * time.Hour).Truncate(time.Microsecond)

	f := seedTenant(t, db, ctx, "deletion-org-newcomer")
	_, err := repo.RequestDeletion(ctx, f.userID, at)
	require.NoError(t, err)

	conn, err := db.Conn(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	adding, err := conn.BeginTx(ctx, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = adding.Rollback() })
	var addingPID int
	require.NoError(t, adding.QueryRowContext(ctx, `SELECT pg_backend_pid()`).Scan(&addingPID))
	newcomer := uuid.New()
	suffix := newcomer.String()[:8]
	_, err = adding.ExecContext(ctx,
		`INSERT INTO users (id, organization_id, email, name, password_hash, role, provider, provider_id)
		 VALUES ($1, $2, $3, 'deletion newcomer', 'x', 'member', 'local', $4)`,
		newcomer, f.orgID, "deletion-newcomer-"+suffix+"@example.com", "local-"+suffix)
	require.NoError(t, err)

	type purged struct {
		deleted []domain.AccountDeletion
		err     error
	}
	done := make(chan purged, 1)
	go func() {
		deleted, err := repo.PurgeDeletionRequests(ctx, at, domain.AccountDeletionPurgeBatch)
		done <- purged{deleted, err}
	}()

	require.Eventually(t, func() bool {
		var waits bool
		err := db.QueryRowContext(ctx,
			`SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE $1 = ANY (pg_blocking_pids(pid)))`, addingPID).Scan(&waits)
		return err == nil && waits
	}, 10*time.Second, 20*time.Millisecond, "the purge waits for the transaction that adds a user")
	require.NoError(t, adding.Commit())

	var r purged
	select {
	case r = <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("the purge did not finish once the user was added")
	}
	require.NoError(t, r.err)
	mine := userDeletionsOfOrg(r.deleted, f.orgID)
	require.Len(t, mine, 1)
	assert.Equal(t, f.userID, mine[0].UserID)
	assert.False(t, mine[0].OrganizationDeleted, "the organization has a user again")
	assert.Equal(t, 0, userDeletionCount(t, db, ctx, `SELECT COUNT(*) FROM users WHERE id = $1`, f.userID))
	assert.Equal(t, 1, userDeletionCount(t, db, ctx, `SELECT COUNT(*) FROM organizations WHERE id = $1`, f.orgID))
	assert.Equal(t, 1, userDeletionCount(t, db, ctx, `SELECT COUNT(*) FROM users WHERE id = $1`, newcomer),
		"the added user is not deleted")
}

// A failure of an organization's delete that is not a refusal by a reference
// rolls back the purge's whole batch: here the purge is cancelled while it
// waits for the organization's row, which another transaction holds. The
// account is not deleted and keeps its request, so the next purge deletes it,
// and the organization with it.
func TestUserDeletionPurgeRollsBackTheBatchWhenAnOrganizationDeleteFails(t *testing.T) {
	db := userDeleteTestDB(t)
	ctx := context.Background()
	repo := NewUserRepository(db)
	at := time.Now().UTC().Add(-80 * 24 * time.Hour).Truncate(time.Microsecond)

	f := seedTenant(t, db, ctx, "deletion-org-rollback")
	_, err := repo.RequestDeletion(ctx, f.userID, at)
	require.NoError(t, err)

	conn, err := db.Conn(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	holding, err := conn.BeginTx(ctx, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = holding.Rollback() })
	var holdingPID int
	require.NoError(t, holding.QueryRowContext(ctx, `SELECT pg_backend_pid()`).Scan(&holdingPID))
	_, err = holding.ExecContext(ctx, `SELECT 1 FROM organizations WHERE id = $1 FOR KEY SHARE`, f.orgID)
	require.NoError(t, err)

	purgeCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	type purged struct {
		deleted []domain.AccountDeletion
		err     error
	}
	done := make(chan purged, 1)
	go func() {
		deleted, err := repo.PurgeDeletionRequests(purgeCtx, at, domain.AccountDeletionPurgeBatch)
		done <- purged{deleted, err}
	}()
	require.Eventually(t, func() bool {
		var waits bool
		err := db.QueryRowContext(ctx,
			`SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE $1 = ANY (pg_blocking_pids(pid)))`, holdingPID).Scan(&waits)
		return err == nil && waits
	}, 10*time.Second, 20*time.Millisecond, "the purge waits for the organization's row")
	cancel()

	var r purged
	select {
	case r = <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("the cancelled purge did not return")
	}
	require.Error(t, r.err)
	assert.Empty(t, r.deleted, "a batch that is rolled back deletes no account")
	require.NoError(t, holding.Rollback())

	status, requestedAt := userDeletionState(t, db, ctx, f.userID)
	assert.Equal(t, "deactivated", status, "the account is not deleted")
	assert.True(t, requestedAt.Valid && requestedAt.Time.Equal(at), "the account keeps its request")
	assert.Equal(t, 1, userDeletionCount(t, db, ctx, `SELECT COUNT(*) FROM organizations WHERE id = $1`, f.orgID))

	again, err := repo.PurgeDeletionRequests(ctx, at, domain.AccountDeletionPurgeBatch)
	require.NoError(t, err)
	mine := userDeletionsOfOrg(again, f.orgID)
	require.Len(t, mine, 1, "the next purge deletes the account")
	assert.True(t, mine[0].OrganizationDeleted)
	assert.Equal(t, 0, userDeletionCount(t, db, ctx, `SELECT COUNT(*) FROM organizations WHERE id = $1`, f.orgID))
}

// organizationDeleteInsertConsent adds an A2A consent record of f's
// organization, between f's agent and itself.
func organizationDeleteInsertConsent(t *testing.T, db *sql.DB, ctx context.Context, f tenantFixture) uuid.UUID {
	t.Helper()

	return userDeleteInsert(t, db, ctx, "a2a_consent_records", "id",
		`INSERT INTO a2a_consent_records (user_id, organization_id, grantor_agent_id, recipient_agent_id,
		                                  scope, purpose, data_types, consent_method)
		 VALUES ('organization-delete-test-user', $1, $2, $2, '["pii"]', 'organization delete test',
		         '["email"]', 'api')
		 RETURNING id`,
		f.orgID, f.agentID)
}

// organizationDeleteInsertReport adds a community intelligence report of orgID.
func organizationDeleteInsertReport(t *testing.T, db *sql.DB, ctx context.Context, orgID uuid.UUID) uuid.UUID {
	t.Helper()

	return userDeleteInsert(t, db, ctx, "community_intelligence_reports", "id",
		`INSERT INTO community_intelligence_reports (organization_id, report_type, agent_count, data_points)
		 VALUES ($1, 'trust_factor_distribution', 1, '{}')
		 RETURNING id`,
		orgID)
}

// organizationDeleteInsertChain gives orgID a record chain row, whose
// reference to the organization has ON DELETE RESTRICT. The row is removed when
// the test ends, before seedTenant's cleanup deletes the organization.
func organizationDeleteInsertChain(t *testing.T, db *sql.DB, ctx context.Context, orgID uuid.UUID) {
	t.Helper()

	id := uuid.New()
	_, err := db.ExecContext(ctx,
		`INSERT INTO record_chains (id, organization_id, key_id, head_seq, head_hash)
		 VALUES ($1, $2, repeat('0', 64), 0, repeat('0', 64))`, id, orgID)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, `DELETE FROM record_chains WHERE id = $1`, id) })
}

// organizationDeleteCaptureLog sends the standard logger's output to a buffer
// until the test ends.
func organizationDeleteCaptureLog(t *testing.T) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer
	prevOut, prevFlags := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(prevOut)
		log.SetFlags(prevFlags)
	})
	return &buf
}
