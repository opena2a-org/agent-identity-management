//go:build integration

package database_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/database"
)

// The request pool's role against a real database: with role set to a
// NOLOGIN role that lacks SELECT on a table, a SELECT on that table fails
// with SQLSTATE 42501, and with role empty it succeeds. Every connection of
// the role pool runs as the role, and a SET ROLE the server refuses fails
// the connection.
//
// Build-tag gated: requires TEST_DATABASE_URL, a postgres:// URL for a login
// that may create a table and a role.
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run TestOpenPool_ ./internal/infrastructure/database/...
func TestOpenPool_RoleWithoutSelectIsRefusedAndNoRoleIsNot(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping request pool role test")
	}
	ctx := context.Background()
	owner, err := database.OpenMigrationHandle(dsn)
	require.NoError(t, err)
	defer owner.Close()
	require.NoError(t, owner.PingContext(ctx))

	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	role := pq.QuoteIdentifier("aim_pool_role_test_" + suffix)
	table := pq.QuoteIdentifier("aim_pool_role_test_" + suffix)
	t.Cleanup(func() {
		_, _ = owner.ExecContext(ctx, `DROP TABLE IF EXISTS `+table)
		_, _ = owner.ExecContext(ctx, `DROP ROLE IF EXISTS `+role)
	})
	for _, stmt := range []string{
		`CREATE TABLE ` + table + ` (id integer)`,
		`INSERT INTO ` + table + ` (id) VALUES (1)`,
		`CREATE ROLE ` + role + ` NOLOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOREPLICATION`,
		`GRANT ` + role + ` TO CURRENT_USER`,
	} {
		_, err := owner.ExecContext(ctx, stmt)
		require.NoError(t, err, stmt)
	}
	var login string
	require.NoError(t, owner.QueryRowContext(ctx, `SELECT session_user`).Scan(&login))

	pool, err := database.OpenPool(dsn, "aim_pool_role_test_"+suffix)
	require.NoError(t, err)
	defer pool.Close()
	pool.SetMaxOpenConns(2)
	require.NoError(t, pool.PingContext(ctx))

	// Two connections held at once: each runs as the role, on the login.
	first, err := pool.Conn(ctx)
	require.NoError(t, err)
	defer first.Close()
	second, err := pool.Conn(ctx)
	require.NoError(t, err)
	defer second.Close()
	for i, conn := range []*sql.Conn{first, second} {
		var current, session string
		require.NoError(t, conn.QueryRowContext(ctx, `SELECT current_user, session_user`).Scan(&current, &session))
		assert.Equal(t, "aim_pool_role_test_"+suffix, current, "connection %d runs as the pool role", i)
		assert.Equal(t, login, session, "connection %d logs in as the login", i)
	}

	var n int
	err = first.QueryRowContext(ctx, `SELECT count(*) FROM `+table).Scan(&n)
	require.Error(t, err, "the role has no SELECT on the table")
	var pqErr *pq.Error
	require.True(t, errors.As(err, &pqErr), "a database error: %v", err)
	assert.Equal(t, pq.ErrorCode("42501"), pqErr.Code, "insufficient_privilege")

	noRole, err := database.OpenPool(dsn, "")
	require.NoError(t, err)
	defer noRole.Close()
	require.NoError(t, noRole.QueryRowContext(ctx, `SELECT count(*) FROM `+table).Scan(&n))
	assert.Equal(t, 1, n, "with no role the login reads the table")

	refused, err := database.OpenPool(dsn, "aim_pool_role_missing_"+suffix)
	require.NoError(t, err)
	defer refused.Close()
	err = refused.PingContext(ctx)
	require.Error(t, err, "SET ROLE to a role that does not exist is refused")
	require.True(t, errors.As(err, &pqErr), "a database error: %v", err)
	assert.Equal(t, pq.ErrorCode("22023"), pqErr.Code, "invalid_parameter_value: the role does not exist")
	assert.Zero(t, refused.Stats().OpenConnections, "the pool holds no connection after a refused SET ROLE")
}
