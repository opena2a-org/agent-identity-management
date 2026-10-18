//go:build integration

package application

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/registry"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
)

// A revocation request that is waiting for a retry when the server stops, end
// to end against a real database and a local Registry: RevokeAgent revokes a
// real agent row and stores the request, the first server's Registry answers
// 503, and the first server stops. The next server, on a database handle of
// its own, sends the request once the retry is due.
//
// Build-tag gated; requires the AIM schema:
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run 'TestAgentRevocationRegistryRequest' ./internal/application/...

// registryATCRevocationTestLock is the advisory lock key the tests that claim
// from registry_atc_revocations hold, as a claim takes every due row in the
// table. internal/infrastructure/repository uses the same one.
const registryATCRevocationTestLock int64 = 7_133_000_133

func openRevocationRestartDB(t *testing.T, dsn string) *sql.DB {
	t.Helper()
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Ping())
	return db
}

type registryRevocationReceived struct {
	method, path, auth string
	body               map[string]any
}

// localRegistry answers every request with status and reports it on the
// returned channel.
func localRegistry(t *testing.T, status int) (*httptest.Server, chan registryRevocationReceived) {
	t.Helper()
	received := make(chan registryRevocationReceived, 8)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := registryRevocationReceived{method: r.Method, path: r.URL.EscapedPath(), auth: r.Header.Get("Authorization")}
		_ = json.NewDecoder(r.Body).Decode(&got.body)
		received <- got
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)
	return srv, received
}

func TestAgentRevocationRegistryRequestWaitingForARetryIsSentAfterARestart(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping Registry revocation restart test")
	}
	ctx := context.Background()
	db := openRevocationRestartDB(t, dsn)
	lock, err := db.Conn(ctx)
	require.NoError(t, err)
	_, err = lock.ExecContext(ctx, `SELECT pg_advisory_lock($1)`, registryATCRevocationTestLock)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = lock.ExecContext(ctx, `SELECT pg_advisory_unlock($1)`, registryATCRevocationTestLock)
		_ = lock.Close()
	})

	orgID, userID := seedOrgAndUser(t, db, ctx, "registry-revocation")
	agentID := uuid.New()
	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM registry_atc_revocations WHERE agent_id = $1`, agentID)
		_, _ = db.Exec(`DELETE FROM agents WHERE id = $1`, agentID)
	})
	_, err = db.Exec(`
		INSERT INTO agents (id, organization_id, name, display_name, agent_type, status, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, 'Registry Revocation Agent', 'ai_agent', 'verified', $4, NOW(), NOW())`,
		agentID, orgID, "registry-revocation-agent-"+agentID.String()[:8], userID)
	require.NoError(t, err)

	// The first server: its Registry answers 503.
	unavailable, unavailableReceived := localRegistry(t, http.StatusServiceUnavailable)
	firstDB := openRevocationRestartDB(t, dsn)
	calc := new(AgentServiceMockTrustScoreCalculator)
	calc.On("Calculate", mock.AnythingOfType("*domain.Agent")).Return(nil, errors.New("not scored in this test"))
	agents := NewAgentService(repository.NewAgentRepository(firstDB), calc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	first := NewRegistryATCRevocations(repository.NewRegistryATCRevocationRepository(firstDB),
		registry.NewATCClient(unavailable.URL, "not-a-real-token", unavailable.Client()))
	// A retry an hour away, so the first server cannot send it before it
	// stops, however slow the machine.
	first.delays = []time.Duration{time.Hour}
	agents.SetRegistryATCRevocations(first)
	stopFirst := first.Start()

	require.NoError(t, agents.RevokeAgent(ctx, agentID))
	select {
	case <-unavailableReceived:
	case <-time.After(registryRevocationWait):
		t.Fatalf("the first server's Registry received no revocation within %s", registryRevocationWait)
	}
	// The failed request's retry is stored, due after the retry delay and not
	// after the hold a request in flight has.
	require.Eventually(t, func() bool {
		var attempts int
		var secondsUntilDue float64
		err := db.QueryRow(`
			SELECT attempts, EXTRACT(EPOCH FROM next_attempt_at - statement_timestamp())::float8
			FROM registry_atc_revocations WHERE agent_id = $1`, agentID).Scan(&attempts, &secondsUntilDue)
		return err == nil && attempts == 1 && secondsUntilDue > registryATCRevocationHold.Seconds()+60
	}, registryRevocationWait, 10*time.Millisecond, "the retry of the failed request was not stored")
	stopFirst()
	require.NoError(t, firstDB.Close())
	assert.Empty(t, unavailableReceived, "the first server sent the retry before it stopped")

	// The retry falls due while no server runs.
	_, err = db.Exec(`UPDATE registry_atc_revocations SET next_attempt_at = statement_timestamp() WHERE agent_id = $1`, agentID)
	require.NoError(t, err)

	// The next server: a database handle of its own, and a Registry that answers 200.
	available, availableReceived := localRegistry(t, http.StatusOK)
	next := NewRegistryATCRevocations(repository.NewRegistryATCRevocationRepository(openRevocationRestartDB(t, dsn)),
		registry.NewATCClient(available.URL, "not-a-real-token", available.Client()))
	stopNext := next.Start()
	defer stopNext()

	select {
	case got := <-availableReceived:
		assert.Equal(t, http.MethodPost, got.method)
		assert.Equal(t, "/internal/atc/"+agentID.String()+"/revoke", got.path)
		assert.Equal(t, "Bearer not-a-real-token", got.auth)
		assert.Equal(t, map[string]any{"reason": "revoked"}, got.body)
	case <-time.After(registryRevocationWait):
		t.Fatalf("the next server's Registry received no revocation within %s", registryRevocationWait)
	}
	require.Eventually(t, func() bool {
		var n int
		return db.QueryRow(`SELECT count(*) FROM registry_atc_revocations WHERE agent_id = $1`, agentID).Scan(&n) == nil && n == 0
	}, registryRevocationWait, 10*time.Millisecond, "the request the Registry answered is still stored")

	var status string
	require.NoError(t, db.QueryRow(`SELECT status FROM agents WHERE id = $1`, agentID).Scan(&status))
	assert.Equal(t, "revoked", status)
}
