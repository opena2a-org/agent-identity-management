//go:build integration

package repository

import (
	"context"
	"database/sql"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// The store against a real registry_atc_revocations table.
//
// A claim takes every due row in the table, and go test runs the packages'
// test binaries at once against one database, so each test here holds the
// advisory lock the application package's restart test takes as well, and
// asserts only on the rows it stored.
//
// Build-tag gated; requires the AIM schema:
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run 'TestAgentRevocationRegistryRequests' ./internal/infrastructure/repository/...

// registryATCRevocationTestLock is the advisory lock key the tests that claim
// from registry_atc_revocations hold. internal/application uses the same one.
const registryATCRevocationTestLock int64 = 7_133_000_133

func registryATCRevocationDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping Registry credential revocation store test")
	}
	db := openRegistryATCRevocationDB(t, dsn)
	ctx := context.Background()
	conn, err := db.Conn(ctx)
	require.NoError(t, err)
	_, err = conn.ExecContext(ctx, `SELECT pg_advisory_lock($1)`, registryATCRevocationTestLock)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = conn.ExecContext(ctx, `SELECT pg_advisory_unlock($1)`, registryATCRevocationTestLock)
		_ = conn.Close()
	})
	return db
}

func openRegistryATCRevocationDB(t *testing.T, dsn string) *sql.DB {
	t.Helper()
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Ping())
	return db
}

// newRegistryATCRevocationAgents returns n agent IDs whose rows are deleted
// when the test ends.
func newRegistryATCRevocationAgents(t *testing.T, db *sql.DB, n int) []uuid.UUID {
	t.Helper()
	ids := make([]uuid.UUID, n)
	for i := range ids {
		ids[i] = uuid.New()
	}
	t.Cleanup(func() {
		for _, id := range ids {
			_, _ = db.Exec(`DELETE FROM registry_atc_revocations WHERE agent_id = $1`, id)
		}
	})
	return ids
}

// claimedFor keeps the claimed requests that name one of agents.
func claimedFor(claimed []domain.RegistryATCRevocation, agents ...uuid.UUID) []domain.RegistryATCRevocation {
	mine := make(map[uuid.UUID]bool, len(agents))
	for _, id := range agents {
		mine[id] = true
	}
	kept := make([]domain.RegistryATCRevocation, 0)
	for _, c := range claimed {
		if mine[c.AgentID] {
			kept = append(kept, c)
		}
	}
	return kept
}

type storedRegistryATCRevocationRow struct {
	requestID       uuid.UUID
	attempts        int
	secondsUntilDue float64
}

// readRegistryATCRevocation reads the agent's row, with how long until it is
// due on the database clock, and whether there is one.
func readRegistryATCRevocation(t *testing.T, db *sql.DB, agentID uuid.UUID) (storedRegistryATCRevocationRow, bool) {
	t.Helper()
	var row storedRegistryATCRevocationRow
	err := db.QueryRow(`
		SELECT request_id, attempts, EXTRACT(EPOCH FROM next_attempt_at - statement_timestamp())::float8
		FROM registry_atc_revocations WHERE agent_id = $1`, agentID).
		Scan(&row.requestID, &row.attempts, &row.secondsUntilDue)
	if err == sql.ErrNoRows {
		return row, false
	}
	require.NoError(t, err)
	return row, true
}

// makeDue moves the agent's next attempt to now, as time passing would.
func makeDue(t *testing.T, db *sql.DB, agentID uuid.UUID) {
	t.Helper()
	_, err := db.Exec(`UPDATE registry_atc_revocations SET next_attempt_at = statement_timestamp() WHERE agent_id = $1`, agentID)
	require.NoError(t, err)
}

func TestAgentRevocationRegistryRequests_ClaimHoldsARequestUntilItIsRetriedOrDeleted(t *testing.T) {
	db := registryATCRevocationDB(t)
	repo := NewRegistryATCRevocationRepository(db)
	ctx := context.Background()
	agentID := newRegistryATCRevocationAgents(t, db, 1)[0]

	require.NoError(t, repo.Store(ctx, agentID))
	stored, ok := readRegistryATCRevocation(t, db, agentID)
	require.True(t, ok)
	assert.Equal(t, 0, stored.attempts)
	assert.LessOrEqual(t, stored.secondsUntilDue, 0.0, "a stored request is due at once")

	claimed, err := repo.Claim(ctx, 100, time.Minute)
	require.NoError(t, err)
	mine := claimedFor(claimed, agentID)
	require.Len(t, mine, 1)
	assert.Equal(t, stored.requestID, mine[0].RequestID)
	assert.Equal(t, 1, mine[0].Attempts)
	held, _ := readRegistryATCRevocation(t, db, agentID)
	assert.InDelta(t, 60, held.secondsUntilDue, 5, "a claimed request is held for the hold")

	claimed, err = repo.Claim(ctx, 100, time.Minute)
	require.NoError(t, err)
	assert.Empty(t, claimedFor(claimed, agentID), "a held request was claimed again")

	require.NoError(t, repo.Retry(ctx, mine[0], 5*time.Minute))
	retried, _ := readRegistryATCRevocation(t, db, agentID)
	assert.InDelta(t, 300, retried.secondsUntilDue, 5)
	claimed, err = repo.Claim(ctx, 100, time.Minute)
	require.NoError(t, err)
	assert.Empty(t, claimedFor(claimed, agentID), "a request was claimed before its retry delay passed")

	makeDue(t, db, agentID)
	claimed, err = repo.Claim(ctx, 100, time.Minute)
	require.NoError(t, err)
	again := claimedFor(claimed, agentID)
	require.Len(t, again, 1)
	assert.Equal(t, 2, again[0].Attempts)
	assert.Equal(t, stored.requestID, again[0].RequestID)

	require.NoError(t, repo.Delete(ctx, again[0]))
	_, ok = readRegistryATCRevocation(t, db, agentID)
	assert.False(t, ok, "the deleted request is still stored")
}

// Storing the request again while a claim holds it replaces it: the answer to
// the claimed request neither deletes nor reschedules the new one.
func TestAgentRevocationRegistryRequests_StoringAgainReplacesTheClaimedRequest(t *testing.T) {
	db := registryATCRevocationDB(t)
	repo := NewRegistryATCRevocationRepository(db)
	ctx := context.Background()
	agentID := newRegistryATCRevocationAgents(t, db, 1)[0]

	require.NoError(t, repo.Store(ctx, agentID))
	claimed, err := repo.Claim(ctx, 100, time.Minute)
	require.NoError(t, err)
	earlier := claimedFor(claimed, agentID)
	require.Len(t, earlier, 1)

	require.NoError(t, repo.Store(ctx, agentID))
	replaced, ok := readRegistryATCRevocation(t, db, agentID)
	require.True(t, ok)
	assert.NotEqual(t, earlier[0].RequestID, replaced.requestID)
	assert.Equal(t, 0, replaced.attempts)
	assert.LessOrEqual(t, replaced.secondsUntilDue, 0.0)

	require.NoError(t, repo.Retry(ctx, earlier[0], time.Hour))
	require.NoError(t, repo.Delete(ctx, earlier[0]))
	kept, ok := readRegistryATCRevocation(t, db, agentID)
	require.True(t, ok, "the answer to the earlier request deleted the later one")
	assert.Equal(t, replaced.requestID, kept.requestID)
	assert.LessOrEqual(t, kept.secondsUntilDue, 0.0, "the answer to the earlier request rescheduled the later one")

	claimed, err = repo.Claim(ctx, 100, time.Minute)
	require.NoError(t, err)
	later := claimedFor(claimed, agentID)
	require.Len(t, later, 1)
	assert.Equal(t, replaced.requestID, later[0].RequestID)
	assert.Equal(t, 1, later[0].Attempts)
}

// Claims made at once take each due request once between them.
func TestAgentRevocationRegistryRequests_ConcurrentClaimsTakeEachRequestOnce(t *testing.T) {
	db := registryATCRevocationDB(t)
	repo := NewRegistryATCRevocationRepository(db)
	ctx := context.Background()
	agents := newRegistryATCRevocationAgents(t, db, 40)
	for _, id := range agents {
		require.NoError(t, repo.Store(ctx, id))
	}

	const claimers = 4
	results := make([][]domain.RegistryATCRevocation, claimers)
	errs := make([]error, claimers)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range claimers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results[i], errs[i] = repo.Claim(ctx, len(agents), time.Minute)
		}()
	}
	close(start)
	wg.Wait()

	taken := make(map[uuid.UUID]int)
	for i := range claimers {
		require.NoError(t, errs[i])
		for _, c := range claimedFor(results[i], agents...) {
			taken[c.AgentID]++
			assert.Equal(t, 1, c.Attempts)
		}
	}
	for _, id := range agents {
		assert.Equal(t, 1, taken[id], "agent %s was claimed %d times", id, taken[id])
	}
}

// A request outlives the server that stored it: a store opened on a new
// database handle, as the next server opens one, claims the request a
// stopped server left waiting for its retry once the retry is due.
func TestAgentRevocationRegistryRequests_ARequestOutlivesTheHandleThatStoredIt(t *testing.T) {
	db := registryATCRevocationDB(t)
	ctx := context.Background()
	agentID := newRegistryATCRevocationAgents(t, db, 1)[0]

	first, err := sql.Open("postgres", os.Getenv("TEST_DATABASE_URL"))
	require.NoError(t, err)
	firstRepo := NewRegistryATCRevocationRepository(first)
	require.NoError(t, firstRepo.Store(ctx, agentID))
	claimed, err := firstRepo.Claim(ctx, 100, time.Minute)
	require.NoError(t, err)
	failed := claimedFor(claimed, agentID)
	require.Len(t, failed, 1)
	require.NoError(t, firstRepo.Retry(ctx, failed[0], 5*time.Second))
	require.NoError(t, first.Close())

	makeDue(t, db, agentID)
	next := NewRegistryATCRevocationRepository(openRegistryATCRevocationDB(t, os.Getenv("TEST_DATABASE_URL")))
	claimed, err = next.Claim(ctx, 100, time.Minute)
	require.NoError(t, err)
	resumed := claimedFor(claimed, agentID)
	require.Len(t, resumed, 1)
	assert.Equal(t, failed[0].RequestID, resumed[0].RequestID)
	assert.Equal(t, 2, resumed[0].Attempts)
	require.NoError(t, next.Delete(ctx, resumed[0]))
}
