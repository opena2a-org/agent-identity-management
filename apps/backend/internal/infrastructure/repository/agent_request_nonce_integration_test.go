//go:build integration

package repository

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
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

// The admission statement and the purge against a real agent_request_nonces
// table. The window cells compute T in SQL from the admission statement's own
// database reading, because the database clock cannot be injected: that is
// the only way to place T exactly 30 or 31 seconds from it.
//
// Build-tag gated; requires the AIM schema:
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run 'TestActionRequest' ./internal/infrastructure/repository/...

func actionRequestNonceDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping action-request nonce integration test")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Ping())
	return db
}

// seedNonceAgent creates an organization, a user and an agent, removed again
// (with their nonces, by cascade) when the test ends.
func seedNonceAgent(t *testing.T, db *sql.DB) (orgID, agentID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	orgID, userID, agentID := uuid.New(), uuid.New(), uuid.New()
	suffix := agentID.String()[:8]
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM agents WHERE id = $1`, agentID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, userID)
		_, _ = db.ExecContext(ctx, `DELETE FROM organizations WHERE id = $1`, orgID)
	})
	_, err := db.ExecContext(ctx,
		`INSERT INTO organizations (id, name, domain, created_at, updated_at)
		 VALUES ($1, $2, $3, NOW(), NOW())`,
		orgID, "action-request-org-"+suffix, "action-request-"+suffix+".example.com")
	require.NoError(t, err)
	_, err = db.ExecContext(ctx,
		`INSERT INTO users (id, organization_id, email, name, password_hash, role,
		                    provider, provider_id, created_at, updated_at)
		 VALUES ($1, $2, $3, 'action-request-user', 'x', 'admin', 'local', $4, NOW(), NOW())`,
		userID, orgID, "action-request-"+suffix+"@example.com", "local-"+suffix)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx,
		`INSERT INTO agents (id, organization_id, name, display_name, agent_type, status,
		                     created_by, created_at, updated_at)
		 VALUES ($1, $2, $3, 'Action Request Agent', 'ai_agent', 'verified', $4, NOW(), NOW())`,
		agentID, orgID, "action-request-agent-"+suffix, userID)
	require.NoError(t, err)
	return orgID, agentID
}

func newNonceBytes(t *testing.T) []byte {
	t.Helper()
	b := make([]byte, domain.ActionRequestNonceBytes)
	_, err := rand.Read(b)
	require.NoError(t, err)
	return b
}

// admitAtOffset admits with T = (the admission statement's reading) + offset.
func admitAtOffset(t *testing.T, repo *AgentRequestNonceRepository, orgID, agentID uuid.UUID, nonce []byte, offsetSeconds float64) domain.ActionRequestAdmission {
	t.Helper()
	outcome, err := repo.admit(context.Background(),
		admissionStatement("db_now + make_interval(secs => $4)"), agentID, orgID, nonce, offsetSeconds)
	require.NoError(t, err)
	return outcome
}

func nonceRows(t *testing.T, db *sql.DB, agentID uuid.UUID) int {
	t.Helper()
	var n int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM agent_request_nonces WHERE agent_id = $1`, agentID).Scan(&n))
	return n
}

func TestActionRequestAdmission_WindowIsThirtySecondsInclusiveOnTheDatabaseClock(t *testing.T) {
	db := actionRequestNonceDB(t)
	repo := NewAgentRequestNonceRepository(db)
	orgID, agentID := seedNonceAgent(t, db)

	for _, tc := range []struct {
		offset float64
		want   domain.ActionRequestAdmission
	}{
		{-30, domain.ActionRequestAdmitted},
		{30, domain.ActionRequestAdmitted},
		{-31, domain.ActionRequestBehindClock},
		{31, domain.ActionRequestAheadOfClock},
		{-30.000001, domain.ActionRequestBehindClock},
		{30.000001, domain.ActionRequestAheadOfClock},
	} {
		before := nonceRows(t, db, agentID)
		got := admitAtOffset(t, repo, orgID, agentID, newNonceBytes(t), tc.offset)
		assert.Equal(t, tc.want, got, "offset %v", tc.offset)
		if tc.want == domain.ActionRequestAdmitted {
			assert.Equal(t, before+1, nonceRows(t, db, agentID))
		} else {
			assert.Equal(t, before, nonceRows(t, db, agentID), "a refused timestamp writes no row")
		}
	}
}

// The row keeps T + window as expires_at and the statement's own reading as
// admitted_at, so |admitted_at - T| <= window on every row.
func TestActionRequestAdmission_RowCarriesTheReadingAndTheExpiry(t *testing.T) {
	db := actionRequestNonceDB(t)
	repo := NewAgentRequestNonceRepository(db)
	orgID, agentID := seedNonceAgent(t, db)

	nonce := newNonceBytes(t)
	require.Equal(t, domain.ActionRequestAdmitted, admitAtOffset(t, repo, orgID, agentID, nonce, -12.5))
	var lifetime float64
	var rowOrg uuid.UUID
	require.NoError(t, db.QueryRow(
		`SELECT EXTRACT(EPOCH FROM expires_at - admitted_at), organization_id
		 FROM agent_request_nonces WHERE agent_id = $1 AND nonce = $2`, agentID, nonce).Scan(&lifetime, &rowOrg))
	assert.InDelta(t, 17.5, lifetime, 1e-6, "expires_at = T + 30 s with T = reading - 12.5 s")
	assert.Equal(t, orgID, rowOrg)

	// Through the production statement with a client clock.
	signedAt := time.Now()
	require.Equal(t, domain.ActionRequestAdmitted, mustAdmit(t, repo, orgID, agentID, newNonceBytes(t), signedAt))
	var outside int
	require.NoError(t, db.QueryRow(
		`SELECT count(*) FROM agent_request_nonces
		 WHERE agent_id = $1
		   AND abs(EXTRACT(EPOCH FROM admitted_at - (expires_at - make_interval(secs => $2)))) > $2`,
		agentID, domain.ActionRequestWindowSeconds).Scan(&outside))
	assert.Zero(t, outside)
}

func mustAdmit(t *testing.T, repo *AgentRequestNonceRepository, orgID, agentID uuid.UUID, nonce []byte, signedAt time.Time) domain.ActionRequestAdmission {
	t.Helper()
	outcome, err := repo.Admit(context.Background(), agentID, orgID, nonce, signedAt)
	require.NoError(t, err)
	return outcome
}

func TestActionRequestAdmission_ANonceIsAdmittedOncePerAgent(t *testing.T) {
	db := actionRequestNonceDB(t)
	repo := NewAgentRequestNonceRepository(db)
	orgID, agentID := seedNonceAgent(t, db)
	otherOrg, otherAgent := seedNonceAgent(t, db)

	nonce := newNonceBytes(t)
	assert.Equal(t, domain.ActionRequestAdmitted, mustAdmit(t, repo, orgID, agentID, nonce, time.Now()))
	assert.Equal(t, domain.ActionRequestNonceReused, mustAdmit(t, repo, orgID, agentID, nonce, time.Now()))
	assert.Equal(t, 1, nonceRows(t, db, agentID))

	// The key is (agent_id, nonce): another agent's identical nonce is its own.
	assert.Equal(t, domain.ActionRequestAdmitted, mustAdmit(t, repo, otherOrg, otherAgent, nonce, time.Now()))

	// A stale replay is refused as stale, before the nonce is looked at.
	assert.Equal(t, domain.ActionRequestBehindClock,
		mustAdmit(t, repo, orgID, agentID, nonce, time.Now().Add(-time.Minute)))
}

func TestActionRequestAdmission_ConcurrentIdenticalRequestsAdmitExactlyOne(t *testing.T) {
	db := actionRequestNonceDB(t)
	repo := NewAgentRequestNonceRepository(db)
	orgID, agentID := seedNonceAgent(t, db)

	nonce := newNonceBytes(t)
	signedAt := time.Now()
	const n = 12
	outcomes := make([]domain.ActionRequestAdmission, n)
	errs := make([]error, n)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			outcomes[i], errs[i] = repo.Admit(context.Background(), agentID, orgID, nonce, signedAt)
		}(i)
	}
	close(start)
	wg.Wait()

	admitted, reused := 0, 0
	for i := range outcomes {
		require.NoError(t, errs[i])
		switch outcomes[i] {
		case domain.ActionRequestAdmitted:
			admitted++
		case domain.ActionRequestNonceReused:
			reused++
		}
	}
	assert.Equal(t, 1, admitted)
	assert.Equal(t, n-1, reused)
	assert.Equal(t, 1, nonceRows(t, db, agentID))
}

// An admission blocked behind another transaction's uncommitted insert of
// the same key fails at the statement timeout, below K, and writes nothing.
func TestActionRequestAdmission_BlockedStatementTimesOutBelowK(t *testing.T) {
	db := actionRequestNonceDB(t)
	repo := NewAgentRequestNonceRepository(db)
	orgID, agentID := seedNonceAgent(t, db)
	nonce := newNonceBytes(t)

	blocker, err := db.Begin()
	require.NoError(t, err)
	defer func() { _ = blocker.Rollback() }()
	_, err = blocker.Exec(`INSERT INTO agent_request_nonces (agent_id, organization_id, nonce, expires_at, admitted_at)
		VALUES ($1, $2, $3, statement_timestamp() + interval '30 seconds', statement_timestamp())`, agentID, orgID, nonce)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	started := time.Now()
	_, err = repo.Admit(ctx, agentID, orgID, nonce, time.Now())
	elapsed := time.Since(started)
	assert.True(t, errors.Is(err, domain.ErrActionRequestAdmissionTimedOut), "got %v", err)
	assert.GreaterOrEqual(t, elapsed, domain.ActionRequestAdmissionTimeout-100*time.Millisecond)
	assert.Less(t, elapsed, time.Duration(domain.ActionRequestNonceRetentionSeconds)*time.Second)

	require.NoError(t, blocker.Rollback())
	assert.Equal(t, 0, nonceRows(t, db, agentID))
}

func TestActionRequestAdmission_StoreErrorsAdmitNothing(t *testing.T) {
	db := actionRequestNonceDB(t)
	orgID, agentID := seedNonceAgent(t, db)

	// The table refuses anything but 16 bytes.
	_, err := NewAgentRequestNonceRepository(db).Admit(context.Background(), agentID, orgID, make([]byte, 15), time.Now())
	assert.Error(t, err)
	assert.Equal(t, 0, nonceRows(t, db, agentID))

	// A closed connection pool.
	closed, err := sql.Open("postgres", os.Getenv("TEST_DATABASE_URL"))
	require.NoError(t, err)
	require.NoError(t, closed.Close())
	_, err = NewAgentRequestNonceRepository(closed).Admit(context.Background(), agentID, orgID, newNonceBytes(t), time.Now())
	assert.Error(t, err)
	assert.Equal(t, 0, nonceRows(t, db, agentID))
}

// plantNonce inserts a row whose expires_at is the given number of seconds
// before the insert's database reading.
func plantNonce(t *testing.T, db *sql.DB, orgID, agentID uuid.UUID, secondsPastExpiry float64) []byte {
	t.Helper()
	nonce := newNonceBytes(t)
	_, err := db.Exec(`INSERT INTO agent_request_nonces (agent_id, organization_id, nonce, expires_at, admitted_at)
		VALUES ($1, $2, $3, statement_timestamp() - make_interval(secs => $4),
		        statement_timestamp() - make_interval(secs => $4 + 30))`,
		agentID, orgID, nonce, secondsPastExpiry)
	require.NoError(t, err)
	return nonce
}

func nonceExists(t *testing.T, db *sql.DB, agentID uuid.UUID, nonce []byte) bool {
	t.Helper()
	var n int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM agent_request_nonces WHERE agent_id = $1 AND nonce = $2`,
		agentID, nonce).Scan(&n))
	return n == 1
}

// A row at expires_at + 30 s survives a purge; a row at expires_at + 31 s
// does not. A purge for one organization deletes no row of another.
func TestActionRequestNoncePurge_KeepsRowsUntilExpiresAtPlusK(t *testing.T) {
	db := actionRequestNonceDB(t)
	repo := NewAgentRequestNonceRepository(db)
	orgA, agentA := seedNonceAgent(t, db)
	orgB, agentB := seedNonceAgent(t, db)

	live := plantNonce(t, db, orgA, agentA, -5)
	at30 := plantNonce(t, db, orgA, agentA, 30)
	at31 := plantNonce(t, db, orgA, agentA, 31)
	old := plantNonce(t, db, orgA, agentA, 3600)
	otherOrgsOld := plantNonce(t, db, orgB, agentB, 3600)
	time.Sleep(5 * time.Millisecond)

	deleted, err := repo.PurgeOrganization(context.Background(), orgA)
	require.NoError(t, err)
	assert.Equal(t, int64(2), deleted)
	assert.True(t, nonceExists(t, db, agentA, live))
	assert.True(t, nonceExists(t, db, agentA, at30), "a row at expires_at + 30 s survives")
	assert.False(t, nonceExists(t, db, agentA, at31), "a row at expires_at + 31 s is deleted")
	assert.False(t, nonceExists(t, db, agentA, old))
	assert.True(t, nonceExists(t, db, agentB, otherOrgsOld), "another organization's rows are untouched")

	deleted, err = repo.PurgeOrganization(context.Background(), orgB)
	require.NoError(t, err)
	assert.Equal(t, int64(1), deleted)
	assert.False(t, nonceExists(t, db, agentB, otherOrgsOld))

	ids, err := repo.ListOrganizationIDs(context.Background())
	require.NoError(t, err)
	assert.Contains(t, ids, orgA)
	assert.Contains(t, ids, orgB)
}
