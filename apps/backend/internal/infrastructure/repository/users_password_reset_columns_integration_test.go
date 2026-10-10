//go:build integration

package repository

import (
	"database/sql"
	"os"
	"testing"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The users table carries one password reset expiry column, the one the reset flow reads.
//
// Migration 002 adds users.password_reset_expires_at, which UserRepository reads and writes.
// Migration 011 later added a second column, users.password_reset_expires, with a partial
// index on it. No code ever read or wrote that column, so on every migrated database it sat
// empty next to the live one, and its index was maintained for nothing. Migration 125 drops
// both. These tests pin the migrated schema, so a reintroduced duplicate fails here rather
// than surviving as a second expiry column that looks authoritative and is not.
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run TestUsersPasswordResetColumns ./internal/infrastructure/repository/...

func usersPasswordResetTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping users password reset schema test")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Ping())
	return db
}

func usersColumnExists(t *testing.T, db *sql.DB, column string) bool {
	t.Helper()

	var exists bool
	err := db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema = current_schema()
			  AND table_name = 'users'
			  AND column_name = $1
		)`, column).Scan(&exists)
	require.NoError(t, err)
	return exists
}

func usersIndexExists(t *testing.T, db *sql.DB, index string) bool {
	t.Helper()

	var exists bool
	err := db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM pg_indexes
			WHERE schemaname = current_schema()
			  AND tablename = 'users'
			  AND indexname = $1
		)`, index).Scan(&exists)
	require.NoError(t, err)
	return exists
}

func TestUsersPasswordResetColumns(t *testing.T) {
	db := usersPasswordResetTestDB(t)

	t.Run("the column the reset flow reads is present", func(t *testing.T) {
		assert.True(t, usersColumnExists(t, db, "password_reset_expires_at"),
			"UserRepository reads and writes users.password_reset_expires_at")
		assert.True(t, usersColumnExists(t, db, "password_reset_token"))
		assert.True(t, usersIndexExists(t, db, "idx_users_password_reset_token"),
			"the reset token lookup index must survive the drop")
	})

	t.Run("the unread duplicate column is gone", func(t *testing.T) {
		assert.False(t, usersColumnExists(t, db, "password_reset_expires"),
			"users.password_reset_expires is read and written by nothing; migration 125 drops it")
	})

	t.Run("the index on the unread column is gone", func(t *testing.T) {
		assert.False(t, usersIndexExists(t, db, "idx_users_password_reset_expires"),
			"idx_users_password_reset_expires indexes a column nothing reads; migration 125 drops it")
	})
}
