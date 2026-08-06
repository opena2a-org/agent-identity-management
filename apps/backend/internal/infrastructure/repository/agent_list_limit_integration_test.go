//go:build integration

package repository

import (
	"context"
	"database/sql"
	"os"
	"testing"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// AgentRepository.List and the meaning of a non-positive limit.
//
// This file exists because `List` and its sibling `GetByOrganizationPaged` held two
// contradictory conventions for the same parameter in the same file. The sibling
// documents "limit <= 0 returns all agents" and implements it by omitting the clause.
// `List` appended `LIMIT $1 OFFSET $2` unconditionally, so a zero limit reached Postgres
// as `LIMIT 0` and returned nothing. Both of `List`'s callers were written to the
// sibling's convention — one of them says `// Get all agents` — and both therefore
// silently processed zero rows. One of the two is the federated revocation list, which
// consequently told every verifier that no agent had ever been revoked.
//
// The repair is neither convention. Returning everything on a zero limit would swap a
// silent-empty bug for a silent-unbounded one, so `List` now rejects the input outright.
// These tests pin that, and they pin it against a real database rather than a mock,
// because the defect lived in SQL that a mock replaces.
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run TestAgentRepositoryList ./internal/infrastructure/repository/...

func agentListTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping agent list limit test")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Ping())
	return db
}

// A non-positive limit is an error, not an interpretation. Both directions are covered:
// zero (what both callers actually passed) and negative.
func TestAgentRepositoryListRejectsNonPositiveLimit(t *testing.T) {
	repo := NewAgentRepository(agentListTestDB(t))

	for _, limit := range []int{0, -1} {
		agents, err := repo.List(limit, 0)

		require.Error(t, err, "List(%d, 0) returned no error", limit)
		assert.ErrorIs(t, err, ErrNonPositiveLimit)
		assert.Nil(t, agents,
			"List(%d, 0) returned rows alongside its error; a caller ignoring err would "+
				"act on them", limit)
	}
}

// The control. A positive limit still returns rows, so the test above is rejecting the
// input rather than the method being broken outright.
//
// It seeds a row first. Asserting only `len(agents) <= 1` would pass against an empty
// table, and therefore against a method that returns nothing at all — the defect under
// repair.
func TestAgentRepositoryListAcceptsPositiveLimit(t *testing.T) {
	ctx := context.Background()
	db := agentListTestDB(t)
	repo := NewAgentRepository(db)

	seedNullDescriptionAgent(t, db, ctx)

	agents, err := repo.List(1, 0)

	require.NoError(t, err)
	require.NotEmpty(t, agents, "List(1, 0) returned no rows despite a seeded agent")
	assert.Len(t, agents, 1, "List(1, 0) did not honour the limit")
}
