//go:build integration

package main

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// The role provisionAppRole leaves for the server, against a real database:
// it is not a superuser, does not bypass row-level security and owns no
// relation; it reads and writes the AIM tables, cannot create a table, and
// passes the server's startup migration step with every migration applied.
// A second run keeps a privilege revoked from the role after it was created.
//
// Build-tag gated: requires TEST_DATABASE_URL, a postgres:// URL for the role
// that owns the AIM schema, with the schema already applied (run migrations
// first), on a server that accepts the new role's password or trusts it.
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run TestMigrateCommand_ ./cmd/server/...
func TestMigrateCommand_AppRoleIsNotSuperuserAndOwnsNoRelation(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping application role test")
	}
	owner, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer owner.Close()
	require.NoError(t, owner.Ping())

	ctx := context.Background()
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	name := "aim_app_test_" + suffix
	password := uuid.NewString()
	t.Cleanup(func() {
		_, _ = owner.ExecContext(ctx, `DROP OWNED BY `+pq.QuoteIdentifier(name))
		_, _ = owner.ExecContext(ctx, `DROP ROLE IF EXISTS `+pq.QuoteIdentifier(name))
	})

	var connectedAs string
	require.NoError(t, owner.QueryRowContext(ctx, `SELECT current_user`).Scan(&connectedAs))
	require.Error(t, provisionAppRole(ctx, owner, connectedAs, password),
		"provisioning the role the command connects as must be refused")

	require.NoError(t, provisionAppRole(ctx, owner, name, password))

	superuser, bypassRLS, owned, err := appRoleState(ctx, owner, name)
	require.NoError(t, err)
	require.False(t, superuser, "the server's role is a superuser")
	require.False(t, bypassRLS, "the server's role bypasses row-level security")
	require.Zero(t, owned, "the server's role owns relations")

	u, err := url.Parse(dsn)
	require.NoError(t, err)
	u.User = url.UserPassword(name, password)
	app, err := sql.Open("postgres", u.String())
	require.NoError(t, err)
	defer app.Close()
	require.NoError(t, app.Ping())

	var appUser string
	require.NoError(t, app.QueryRowContext(ctx, `SELECT current_user`).Scan(&appUser))
	require.Equal(t, name, appUser)

	orgID := uuid.New()
	t.Cleanup(func() {
		_, _ = owner.ExecContext(ctx, `DELETE FROM organizations WHERE id = $1`, orgID)
	})
	_, err = app.ExecContext(ctx,
		`INSERT INTO organizations (id, name, domain, created_at, updated_at)
		 VALUES ($1, $2, $3, NOW(), NOW())`,
		orgID, "app-role-org-"+suffix, "app-role-"+suffix+".example.com")
	require.NoError(t, err, "the server's role cannot insert")
	_, err = app.ExecContext(ctx, `UPDATE organizations SET updated_at = NOW() WHERE id = $1`, orgID)
	require.NoError(t, err, "the server's role cannot update")
	var count int
	require.NoError(t, app.QueryRowContext(ctx, `SELECT count(*) FROM organizations WHERE id = $1`, orgID).Scan(&count))
	require.Equal(t, 1, count)
	_, err = app.ExecContext(ctx, `DELETE FROM organizations WHERE id = $1`, orgID)
	require.NoError(t, err, "the server's role cannot delete")

	_, err = app.ExecContext(ctx, `CREATE TABLE app_role_probe_`+suffix+` (id int)`)
	require.Error(t, err, "the server's role can create a table, and so come to own one")
	require.Contains(t, err.Error(), "permission denied")

	// The startup migration step reads migrations/ relative to apps/backend.
	t.Chdir(filepath.Join("..", ".."))
	require.NoError(t, runMigrations(app), "the server cannot start as its role with every migration applied")

	_, err = owner.ExecContext(ctx, `REVOKE UPDATE, DELETE ON organizations FROM `+pq.QuoteIdentifier(name))
	require.NoError(t, err)
	require.NoError(t, provisionAppRole(ctx, owner, name, password))
	_, err = app.ExecContext(ctx, `DELETE FROM organizations WHERE id = $1`, uuid.New())
	require.Error(t, err, "a second provisioning run granted back a privilege revoked from the role")
	require.Contains(t, err.Error(), "permission denied")
}
