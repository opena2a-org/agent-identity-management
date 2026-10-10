//go:build integration

package main

import (
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

// renumberedMigrationOldName is the file name 125_drop_unread_password_reset_expires.sql
// was released under, beside 112_trust_score_low_threshold_key_and_scale.sql.
const renumberedMigrationOldName = "112_drop_unread_password_reset_expires.sql"

// A database migrated before 125_drop_unread_password_reset_expires.sql was
// renumbered from 112 recorded it in schema_migrations under the old name.
// After the server's start-up migration step that row is gone, and the row of
// the other 112 migration is still there.
//
// The database stands in for one with every migration through 124 applied: a
// schema holding a users table with the unread column, and schema_migrations
// holding both 112 rows. The step runs over 125 and every migration that names
// the old file, copied from the migrations directory, so the test reads what
// the repository ships rather than SQL of its own.
//
// Build-tag gated: requires TEST_DATABASE_URL, a postgres:// URL for a role
// that may create a schema.
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run TestRunMigrations_RenumberedMigration ./cmd/server/...
func TestRunMigrations_RenumberedMigrationLeavesNoRowUnderItsOldName(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping renumbered migration row test")
	}

	shipped, err := filepath.Abs(filepath.Join("..", "..", "migrations"))
	require.NoError(t, err)
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "migrations"), 0o755))
	entries, err := os.ReadDir(shipped)
	require.NoError(t, err)
	var copied []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".sql") {
			continue
		}
		content, err := os.ReadFile(filepath.Join(shipped, name))
		require.NoError(t, err)
		if name != "125_drop_unread_password_reset_expires.sql" && !strings.Contains(string(content), renumberedMigrationOldName) {
			continue
		}
		require.NoError(t, os.WriteFile(filepath.Join(dir, "migrations", name), content, 0o644))
		copied = append(copied, name)
	}
	require.Contains(t, copied, "125_drop_unread_password_reset_expires.sql",
		"125_drop_unread_password_reset_expires.sql is not in %s", shipped)

	owner, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer owner.Close()
	require.NoError(t, owner.Ping())

	schema := "migrations_renumbered_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	_, err = owner.Exec(`CREATE SCHEMA ` + pq.QuoteIdentifier(schema))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = owner.Exec(`DROP SCHEMA IF EXISTS ` + pq.QuoteIdentifier(schema) + ` CASCADE`)
	})

	u, err := url.Parse(dsn)
	require.NoError(t, err)
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, err := sql.Open("postgres", u.String())
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, db.Ping())

	var current string
	require.NoError(t, db.QueryRow(`SELECT current_schema()`).Scan(&current))
	require.Equal(t, schema, current, "the connection does not resolve names in the test schema")

	_, err = db.Exec(`CREATE TABLE users (
		id UUID PRIMARY KEY,
		password_reset_expires_at TIMESTAMPTZ,
		password_reset_expires TIMESTAMPTZ
	)`)
	require.NoError(t, err)
	require.NoError(t, createMigrationsTable(db))
	_, err = db.Exec(`INSERT INTO schema_migrations (version) VALUES ($1), ($2)`,
		"112_trust_score_low_threshold_key_and_scale.sql", renumberedMigrationOldName)
	require.NoError(t, err)

	t.Chdir(dir)
	require.NoError(t, runMigrations(db))

	rows, err := db.Query(`SELECT version FROM schema_migrations ORDER BY version`)
	require.NoError(t, err)
	defer rows.Close()
	var recorded []string
	for rows.Next() {
		var version string
		require.NoError(t, rows.Scan(&version))
		recorded = append(recorded, version)
	}
	require.NoError(t, rows.Err())

	require.NotContains(t, recorded, renumberedMigrationOldName,
		"schema_migrations still records a migration this repository no longer ships; recorded: %v", recorded)
	require.Contains(t, recorded, "112_trust_score_low_threshold_key_and_scale.sql",
		"the row of the other 112 migration was removed; recorded: %v", recorded)
	require.Contains(t, recorded, "125_drop_unread_password_reset_expires.sql")
	for _, name := range copied {
		require.Contains(t, recorded, name, "a copied migration was not recorded; recorded: %v", recorded)
	}

	var unread bool
	require.NoError(t, db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema = $1 AND table_name = 'users' AND column_name = 'password_reset_expires'
		)`, schema).Scan(&unread))
	require.False(t, unread, "125 did not drop users.password_reset_expires")
}
