//go:build integration

package main

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A database that applied 108_add_isolation_attestation_verification.sql before
// its column comment was edited keeps the old comment on
// isolation_attestations.verified, because both runners skip a recorded
// migration. After 128 runs it carries the comment 108 sets today, the same
// text a new database gets.
//
// The test runs in one transaction that it rolls back, on a database with every
// migration applied: it sets a stand-in for the old comment, runs 128 as
// shipped, reads the comment, then runs 108 as shipped and reads the comment a
// new database would get.
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run TestMigration128_ReissuesTheVerifiedColumnComment ./cmd/server/...
func TestMigration128_ReissuesTheVerifiedColumnComment(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping migration 128 comment test")
	}

	readMigration := func(name string) string {
		t.Helper()
		content, err := os.ReadFile(filepath.Join("..", "..", "migrations", name))
		require.NoError(t, err)
		return string(content)
	}
	reissue := readMigration("128_reissue_isolation_attestation_verified_comment.sql")
	original := readMigration("108_add_isolation_attestation_verification.sql")

	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	tx, err := db.Begin()
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	comment := func() string {
		t.Helper()
		var got sql.NullString
		require.NoError(t, tx.QueryRow(
			`SELECT col_description('isolation_attestations'::regclass, attnum)
			 FROM pg_attribute
			 WHERE attrelid = 'isolation_attestations'::regclass AND attname = 'verified'`,
		).Scan(&got))
		return got.String
	}

	const stale = "the comment 108 set before it was edited"
	_, err = tx.Exec(`COMMENT ON COLUMN isolation_attestations.verified IS '` + stale + `'`)
	require.NoError(t, err)
	require.Equal(t, stale, comment())

	_, err = tx.Exec(reissue)
	require.NoError(t, err)
	upgraded := comment()

	_, err = tx.Exec(original)
	require.NoError(t, err)
	fresh := comment()

	assert.NotEqual(t, stale, upgraded, "128 left the old comment in place")
	assert.Equal(t, fresh, upgraded, "an upgraded database and a new one carry different comments")
}
