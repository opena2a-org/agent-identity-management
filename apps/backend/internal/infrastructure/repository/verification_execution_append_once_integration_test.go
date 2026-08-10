//go:build integration

package repository

import (
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The execution-report columns (executed, strict_mode, executed_at,
// execution_error) have exactly one writer in the backend: the reporting agent's
// own SDK. They are rendered to an operator as claims about what happened, so an
// agent must not be able to revise its own record after the fact.
//
// UpdateExecutionStatus therefore carries `AND executed IS NULL`. That predicate
// lives in SQL, so a mocked repository cannot exercise it — the mock replaces the
// statement under test. These tests drive the REAL repository against a real
// Postgres.
//
// Build-tag gated; requires the AIM schema:
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run TestUpdateExecutionStatusAppendOnce ./internal/infrastructure/repository/...

func execAppendOnceDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping execution-status append-once integration test")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Ping())
	return db
}

// seedVerificationEvent creates its OWN organization and agent rather than
// borrowing whatever happens to be in the database. Selecting an ambient agent
// makes the test depend on seed data it does not control: it would pass today
// only because a migration happens to insert some, and would hard-fail rather
// than skip if that ever changed. Every other integration test in this package
// creates its own fixture; this follows them.
func seedVerificationEvent(t *testing.T, db *sql.DB) uuid.UUID {
	t.Helper()

	orgID, agentID, eventID := uuid.New(), uuid.New(), uuid.New()
	userID := uuid.New()

	_, err := db.Exec(`
        INSERT INTO organizations
            (id, name, domain, plan_type, max_agents, max_users, is_active,
             created_at, updated_at, max_mcp_servers, enforcement_mode,
             community_intelligence_enabled)
        VALUES ($1, $2, $3, 'free', 100, 100, true, NOW(), NOW(), 100, 'monitoring', false)`,
		orgID, "append-once-"+orgID.String()[:8], "append-once-"+orgID.String()[:8]+".invalid")
	require.NoError(t, err, "seed organization")

	// agents.created_by carries an FK to users, so the fixture needs its own user.
	_, err = db.Exec(`
        INSERT INTO users
            (id, organization_id, email, name, role, provider, provider_id,
             created_at, updated_at, status)
        VALUES ($1, $2, $3, 'Append Once Fixture', 'member', 'local', $4, NOW(), NOW(), 'active')`,
		userID, orgID, "append-once-"+userID.String()[:8]+"@fixture.invalid", userID.String())
	require.NoError(t, err, "seed user")

	_, err = db.Exec(`
        INSERT INTO agents
            (id, organization_id, name, display_name, agent_type, status,
             created_at, updated_at, created_by)
        VALUES ($1, $2, $3, 'Append Once Fixture', 'service', 'active', NOW(), NOW(), $4)`,
		agentID, orgID, "append-once-agent-"+agentID.String()[:8], userID)
	require.NoError(t, err, "seed agent")

	_, err = db.Exec(`
        INSERT INTO verification_events
            (id, organization_id, agent_id, protocol, verification_type, status,
             initiator_type, started_at, created_at)
        VALUES ($1, $2, $3, 'a2a', 'capability', 'pending', 'agent', NOW(), NOW())`,
		eventID, orgID, agentID)
	require.NoError(t, err, "seed verification event")

	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM verification_events WHERE organization_id = $1`, orgID)
		_, _ = db.Exec(`DELETE FROM agents WHERE organization_id = $1`, orgID)
		_, _ = db.Exec(`DELETE FROM users WHERE organization_id = $1`, orgID)
		_, _ = db.Exec(`DELETE FROM organizations WHERE id = $1`, orgID)
	})
	return eventID
}

func TestUpdateExecutionStatusAppendOnce_FirstReportWins(t *testing.T) {
	db := execAppendOnceDB(t)
	repo := &VerificationEventRepositorySimple{db: db}
	id := seedVerificationEvent(t, db)

	// First report: the agent says it did NOT run the action, under strict mode.
	require.NoError(t, repo.UpdateExecutionStatus(id, false, true, time.Now(), nil),
		"the first execution report must be accepted")

	// Second report, same event: the agent tries to revise its own record.
	err := repo.UpdateExecutionStatus(id, true, false, time.Now(), nil)
	require.Error(t, err,
		"a second execution report must be rejected; the record is append-once")

	var executed, strictMode bool
	require.NoError(t, db.QueryRow(
		`SELECT executed, strict_mode FROM verification_events WHERE id = $1`, id).
		Scan(&executed, &strictMode))

	assert.False(t, executed, "the stored report must still be the FIRST one")
	assert.True(t, strictMode, "the second report must not have rewritten strict_mode either")
}

// TestUpdateExecutionStatusAppendOnce_ExecutedTrueIsAlsoSealed pins that the
// predicate keys on NULL, not on falsiness. `executed IS NULL` and
// `executed = false` are different conditions, and a predicate written as
// `AND executed IS NOT TRUE` would leave a first report of false rewritable —
// which is exactly the direction an agent would want to revise.
func TestUpdateExecutionStatusAppendOnce_ExecutedTrueIsAlsoSealed(t *testing.T) {
	db := execAppendOnceDB(t)
	repo := &VerificationEventRepositorySimple{db: db}
	id := seedVerificationEvent(t, db)

	require.NoError(t, repo.UpdateExecutionStatus(id, true, true, time.Now(), nil))
	require.Error(t, repo.UpdateExecutionStatus(id, false, false, time.Now(), nil))

	var executed bool
	require.NoError(t, db.QueryRow(
		`SELECT executed FROM verification_events WHERE id = $1`, id).Scan(&executed))
	assert.True(t, executed, "a true first report is sealed the same as a false one")
}

// TestUpdateExecutionStatusAppendOnce_UnknownEventStillErrors keeps the
// not-found signal distinguishable at the repository seam: adding a predicate to
// a WHERE clause must not turn "no such row" into a silent success.
func TestUpdateExecutionStatusAppendOnce_UnknownEventStillErrors(t *testing.T) {
	db := execAppendOnceDB(t)
	repo := &VerificationEventRepositorySimple{db: db}

	assert.Error(t, repo.UpdateExecutionStatus(uuid.New(), true, false, time.Now(), nil),
		"an unknown id must still error rather than report success")
}
