//go:build integration

package repository

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// Deleting a user clears the copies of the user's name and email that agents and MCP
// servers hold. agents and mcp_servers copy the creator's name and email into
// created_by_name and created_by_email, and have updated_by, updated_by_name and
// updated_by_email for the user who last changed the row. With migration 135 alone,
// deleting the user set created_by to NULL and kept the copies, so the agent and the
// MCP server read back with the deleted user's name and email, and updated_by, which
// had no foreign key, kept the deleted user's id. Migration 136 clears each copy with
// the reference it copies.
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run 'TestUserDelete(Trigger|Backfill)' ./internal/infrastructure/repository/...

const userDeleteCopiesMigration = "136_user_delete_clears_name_and_email_copies.sql"

// Against every migration applied: the agent and the MCP server the deleted user
// created and last changed read back with no creator, no updater and no copy of
// either; an agent another user created keeps its creator's copies and loses only the
// deleted user's; an MCP server another user created and changed keeps every copy.
func TestUserDeleteTriggerClearsTheUsersNameAndEmailFromAgentsAndMCPServers(t *testing.T) {
	db := userDeleteTestDB(t)
	ctx := context.Background()

	f := seedTenant(t, db, ctx, "user-delete-copies")
	leaving := f.userID
	staying := userDeleteSeedUser(t, db, ctx, f.orgID, nil)
	leavingName, leavingEmail := userDeleteNameAndEmail(t, db, ctx, leaving)
	stayingName, stayingEmail := userDeleteNameAndEmail(t, db, ctx, staying)

	agents := NewAgentRepository(db)
	servers := NewMCPServerRepository(db)

	agentOfLeaving := userDeleteCopiesAgent(t, db, ctx, agents, f.orgID, leaving, leavingName, leavingEmail)
	userDeleteSetUpdater(t, db, ctx, "agents", agentOfLeaving, leaving, leavingName, leavingEmail)
	agentOfStaying := userDeleteCopiesAgent(t, db, ctx, agents, f.orgID, staying, stayingName, stayingEmail)
	userDeleteSetUpdater(t, db, ctx, "agents", agentOfStaying, leaving, leavingName, leavingEmail)

	serverOfLeaving := userDeleteCopiesServer(t, db, ctx, servers, f.orgID, leaving, leavingName, leavingEmail)
	userDeleteSetUpdater(t, db, ctx, "mcp_servers", serverOfLeaving, leaving, leavingName, leavingEmail)
	serverOfStaying := userDeleteCopiesServer(t, db, ctx, servers, f.orgID, staying, stayingName, stayingEmail)
	userDeleteSetUpdater(t, db, ctx, "mcp_servers", serverOfStaying, staying, stayingName, stayingEmail)

	// Control: before the delete each row reads back with the copies it was given.
	before, err := agents.GetByID(agentOfLeaving)
	require.NoError(t, err)
	require.Equal(t, leavingName, before.CreatedByName, "control: the agent must hold its creator's name before the delete")
	require.Equal(t, leavingEmail, before.UpdatedByEmail, "control: the agent must hold its updater's email before the delete")

	require.NoError(t, NewUserRepository(db).Delete(leaving), "deleting the user must succeed")

	agent, err := agents.GetByID(agentOfLeaving)
	require.NoError(t, err, "the agent the deleted user created must be kept")
	assert.Equal(t, uuid.Nil, agent.CreatedBy)
	assert.Equal(t, "", agent.CreatedByName, "the agent must not return the deleted user's name")
	assert.Equal(t, "", agent.CreatedByEmail, "the agent must not return the deleted user's email")
	assert.Nil(t, agent.UpdatedBy, "the agent must not name the deleted user as its updater")
	assert.Equal(t, "", agent.UpdatedByName)
	assert.Equal(t, "", agent.UpdatedByEmail)
	userDeleteAssertNoCopies(t, db, ctx, "agents", agentOfLeaving, true)

	server, err := servers.GetByID(serverOfLeaving)
	require.NoError(t, err, "the MCP server the deleted user created must be kept")
	assert.Equal(t, uuid.Nil, server.CreatedBy)
	assert.Equal(t, "", server.CreatedByName, "the MCP server must not return the deleted user's name")
	assert.Equal(t, "", server.CreatedByEmail, "the MCP server must not return the deleted user's email")
	assert.Nil(t, server.UpdatedBy, "the MCP server must not name the deleted user as its updater")
	assert.Equal(t, "", server.UpdatedByName)
	assert.Equal(t, "", server.UpdatedByEmail)
	userDeleteAssertNoCopies(t, db, ctx, "mcp_servers", serverOfLeaving, true)

	kept, err := agents.GetByID(agentOfStaying)
	require.NoError(t, err)
	assert.Equal(t, staying, kept.CreatedBy, "another user's agent keeps its creator")
	assert.Equal(t, stayingName, kept.CreatedByName, "another user's agent keeps its creator's name")
	assert.Equal(t, stayingEmail, kept.CreatedByEmail, "another user's agent keeps its creator's email")
	assert.Nil(t, kept.UpdatedBy, "the deleted user is no longer the agent's updater")
	assert.Equal(t, "", kept.UpdatedByName)
	assert.Equal(t, "", kept.UpdatedByEmail)
	userDeleteAssertNoCopies(t, db, ctx, "agents", agentOfStaying, false)

	keptServer, err := servers.GetByID(serverOfStaying)
	require.NoError(t, err)
	assert.Equal(t, staying, keptServer.CreatedBy)
	assert.Equal(t, stayingName, keptServer.CreatedByName)
	assert.Equal(t, stayingEmail, keptServer.CreatedByEmail)
	if assert.NotNil(t, keptServer.UpdatedBy, "another user's MCP server keeps its updater") {
		assert.Equal(t, staying, *keptServer.UpdatedBy)
	}
	assert.Equal(t, stayingName, keptServer.UpdatedByName)
	assert.Equal(t, stayingEmail, keptServer.UpdatedByEmail)
}

// Migration 136 clears the copies a database already holds without their user: on a
// row whose creator was deleted under migration 135, and on a row whose updated_by
// names no user. The schema stands in for a database with every migration through
// 135 applied: users, and agents and mcp_servers with the columns migration 136 reads
// and writes, created_by referencing users with ON DELETE SET NULL as migration 135
// left it. The migration runs from the file the repository ships, twice; then a
// delete of the remaining user clears that user's copies through the trigger and the
// foreign key the migration added.
func TestUserDeleteBackfillClearsCopiesThatOutlivedTheirUser(t *testing.T) {
	db := userDeleteStandInSchema(t)

	_, err := db.Exec(`CREATE TABLE users (id UUID PRIMARY KEY, name VARCHAR(255), email VARCHAR(255))`)
	require.NoError(t, err)
	tables := []string{"agents", "mcp_servers"}
	for _, table := range tables {
		_, err := db.Exec(`CREATE TABLE ` + table + ` (
			id UUID PRIMARY KEY,
			created_by UUID REFERENCES users(id) ON DELETE SET NULL,
			created_by_name VARCHAR(255),
			created_by_email VARCHAR(255),
			updated_by UUID,
			updated_by_name VARCHAR(255),
			updated_by_email VARCHAR(255)
		)`)
		require.NoError(t, err)
	}

	user := uuid.New()
	_, err = db.Exec(`INSERT INTO users (id, name, email) VALUES ($1, 'Staying User', 'staying@example.com')`, user)
	require.NoError(t, err)
	gone := uuid.New() // an id no user has: the user it named was deleted

	type seededRows struct{ orphaned, dangling, live uuid.UUID }
	seeded := make(map[string]seededRows, len(tables))
	for _, table := range tables {
		r := seededRows{orphaned: uuid.New(), dangling: uuid.New(), live: uuid.New()}
		_, err := db.Exec(`INSERT INTO `+table+` VALUES
			($1, NULL, 'Deleted User', 'deleted@example.com', NULL, 'Deleted User', 'deleted@example.com'),
			($2, $4, 'Staying User', 'staying@example.com', $5, 'Deleted User', 'deleted@example.com'),
			($3, $4, 'Staying User', 'staying@example.com', $4, 'Staying User', 'staying@example.com')`,
			r.orphaned, r.dangling, r.live, user, gone)
		require.NoError(t, err)
		seeded[table] = r

		// Control: the rows hold the copies before the migration runs.
		assert.Equal(t, [4]string{"Deleted User", "deleted@example.com", "Deleted User", "deleted@example.com"},
			userDeleteStoredCopies(t, db, table, r.orphaned), "%s: control, the orphaned row before the migration", table)
	}

	migration := userDeleteReadMigration(t, userDeleteCopiesMigration)
	_, err = db.Exec(migration)
	require.NoError(t, err, "the migration must apply")
	_, err = db.Exec(migration)
	require.NoError(t, err, "the migration must apply again")

	for _, table := range tables {
		r := seeded[table]
		assert.Equal(t, [4]string{"NULL", "NULL", "NULL", "NULL"}, userDeleteStoredCopies(t, db, table, r.orphaned),
			"%s: a row whose creator was deleted keeps no copy of the creator's name or email", table)
		assert.Equal(t, [4]string{"Staying User", "staying@example.com", "NULL", "NULL"}, userDeleteStoredCopies(t, db, table, r.dangling),
			"%s: a row whose updated_by names no user keeps its creator's copies and loses the updater's", table)
		var updater sql.NullString
		require.NoError(t, db.QueryRow(`SELECT updated_by::text FROM `+table+` WHERE id = $1`, r.dangling).Scan(&updater))
		assert.False(t, updater.Valid, "%s: an updated_by that names no user must be NULL", table)
		assert.Equal(t, [4]string{"Staying User", "staying@example.com", "Staying User", "staying@example.com"},
			userDeleteStoredCopies(t, db, table, r.live), "%s: a row whose users exist keeps every copy", table)

		var rule string
		require.NoError(t, db.QueryRow(`
			SELECT c.confdeltype::text
			FROM pg_constraint c
			JOIN pg_attribute a ON a.attrelid = c.conrelid AND a.attnum = ANY (c.conkey)
			WHERE c.contype = 'f' AND c.conrelid = $1::regclass AND c.confrelid = 'users'::regclass
			  AND a.attname = 'updated_by'`, table).Scan(&rule))
		assert.Equal(t, "SET NULL", userDeleteRuleName(rule), "%s.updated_by must reference users(id) with ON DELETE SET NULL", table)
	}

	_, err = db.Exec(`DELETE FROM users WHERE id = $1`, user)
	require.NoError(t, err, "deleting the remaining user must succeed")
	for _, table := range tables {
		r := seeded[table]
		assert.Equal(t, [4]string{"NULL", "NULL", "NULL", "NULL"}, userDeleteStoredCopies(t, db, table, r.live),
			"%s: once its user is deleted the row keeps no copy of the user's name or email", table)
		assert.Equal(t, [4]string{"NULL", "NULL", "NULL", "NULL"}, userDeleteStoredCopies(t, db, table, r.dangling), "%s", table)
	}
}

// userDeleteNameAndEmail reads a user's name and email.
func userDeleteNameAndEmail(t *testing.T, db *sql.DB, ctx context.Context, id uuid.UUID) (string, string) {
	t.Helper()

	var name, email string
	require.NoError(t, db.QueryRowContext(ctx, `SELECT name, email FROM users WHERE id = $1`, id).Scan(&name, &email))
	require.NotEmpty(t, name)
	require.NotEmpty(t, email)
	return name, email
}

// userDeleteCopiesAgent stores an agent through AgentRepository.Create with the
// creator's name and email copied into it, as AgentService.CreateAgent copies them from
// the user.
func userDeleteCopiesAgent(t *testing.T, db *sql.DB, ctx context.Context, repo *AgentRepository, orgID, createdBy uuid.UUID, name, email string) uuid.UUID {
	t.Helper()

	agent := &domain.Agent{
		OrganizationID: orgID,
		Name:           "user-delete-copies-agent-" + uuid.NewString()[:8],
		DisplayName:    "User Delete Copies Agent",
		AgentType:      domain.AgentTypeAI,
		CreatedBy:      createdBy,
		CreatedByName:  name,
		CreatedByEmail: email,
	}
	require.NoError(t, repo.Create(agent))
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, `DELETE FROM agents WHERE id = $1`, agent.ID) })
	return agent.ID
}

// userDeleteCopiesServer stores an MCP server through MCPServerRepository.Create with
// the creator's name and email copied into it, as MCPService.CreateMCPServer copies them
// from the user.
func userDeleteCopiesServer(t *testing.T, db *sql.DB, ctx context.Context, repo *MCPServerRepository, orgID, createdBy uuid.UUID, name, email string) uuid.UUID {
	t.Helper()

	marker := uuid.NewString()[:8]
	server := &domain.MCPServer{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Name:           "user-delete-copies-mcp-" + marker,
		URL:            "https://mcp-copies-" + marker + ".example.com",
		Status:         domain.MCPServerStatusPending,
		Capabilities:   []string{},
		CreatedBy:      createdBy,
		CreatedByName:  name,
		CreatedByEmail: email,
	}
	require.NoError(t, repo.Create(server))
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, `DELETE FROM mcp_servers WHERE id = $1`, server.ID) })
	return server.ID
}

// userDeleteSetUpdater names a user as the row's last updater with copies of the
// user's name and email. No repository writes these columns; the API reads them back
// as updatedBy, updatedByName and updatedByEmail. table is always a literal from this
// file.
func userDeleteSetUpdater(t *testing.T, db *sql.DB, ctx context.Context, table string, id, updatedBy uuid.UUID, name, email string) {
	t.Helper()

	res, err := db.ExecContext(ctx,
		`UPDATE `+table+` SET updated_by = $2, updated_by_name = $3, updated_by_email = $4 WHERE id = $1`,
		id, updatedBy, name, email)
	require.NoError(t, err)
	n, err := res.RowsAffected()
	require.NoError(t, err)
	require.Equal(t, int64(1), n)
}

// userDeleteAssertNoCopies checks that the row stores NULL, rather than an empty
// string the reads would also return, in the copies of the deleted user's name and
// email and in updated_by; with creatorDeleted, also in the creator's copies.
func userDeleteAssertNoCopies(t *testing.T, db *sql.DB, ctx context.Context, table string, id uuid.UUID, creatorDeleted bool) {
	t.Helper()

	var createdNull, updatedNull bool
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT created_by_name IS NULL AND created_by_email IS NULL,
		        updated_by IS NULL AND updated_by_name IS NULL AND updated_by_email IS NULL
		 FROM `+table+` WHERE id = $1`, id).Scan(&createdNull, &updatedNull))
	assert.True(t, updatedNull, "%s must store NULL in updated_by, updated_by_name and updated_by_email", table)
	if creatorDeleted {
		assert.True(t, createdNull, "%s must store NULL in created_by_name and created_by_email", table)
	}
}

// userDeleteStoredCopies reads created_by_name, created_by_email, updated_by_name and
// updated_by_email of a row, with "NULL" for NULL. table is always a literal from this
// file.
func userDeleteStoredCopies(t *testing.T, db *sql.DB, table string, id uuid.UUID) [4]string {
	t.Helper()

	var got [4]sql.NullString
	require.NoError(t, db.QueryRow(
		`SELECT created_by_name, created_by_email, updated_by_name, updated_by_email FROM `+table+` WHERE id = $1`,
		id).Scan(&got[0], &got[1], &got[2], &got[3]))
	var out [4]string
	for i, v := range got {
		out[i] = "NULL"
		if v.Valid {
			out[i] = v.String
		}
	}
	return out
}

// userDeleteReadMigration returns the text of a migration the repository ships.
func userDeleteReadMigration(t *testing.T, name string) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "..", "migrations", name))
	require.NoError(t, err)
	return string(raw)
}

// userDeleteStandInSchema creates a schema of its own and returns a connection that
// resolves names in it, so the tables a test builds there stand in for a database at
// an earlier migration without touching the migrated one. The schema is dropped when
// the test ends.
func userDeleteStandInSchema(t *testing.T) *sql.DB {
	t.Helper()

	owner := userDeleteTestDB(t)
	schema := "user_delete_copies_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	_, err := owner.Exec(`CREATE SCHEMA ` + pq.QuoteIdentifier(schema))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := owner.Exec(`DROP SCHEMA IF EXISTS ` + pq.QuoteIdentifier(schema) + ` CASCADE`)
		assert.NoError(t, err, "the test schema %s was not dropped", schema)
	})

	u, err := url.Parse(os.Getenv("TEST_DATABASE_URL"))
	require.NoError(t, err)
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, err := sql.Open("postgres", u.String())
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Ping())

	var current string
	require.NoError(t, db.QueryRow(`SELECT current_schema()`).Scan(&current))
	require.Equal(t, schema, current, "the connection does not resolve names in the test schema")
	return db
}
