//go:build integration

package main

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
)

// The nonce cleanup against a real a2a_request_nonces table: a nonce past
// both expires_at + skew and used_at + 2*skew is deleted, by a direct run and
// by the scheduled job within one interval; a nonce a request could still
// replay is kept.
//
// The DELETE is table-wide by design, so it also removes other suites'
// nonces that are past the same bound; nothing reads those rows.
//
// Build-tag gated: requires Postgres reachable via TEST_DATABASE_URL with
// the AIM schema already applied (run migrations first).
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run TestNonceCleanup_ ./cmd/server/...
func TestNonceCleanup_DeletesOnlyNoncesPastTheReplayWindow(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping nonce cleanup test")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, db.Ping())

	ctx := context.Background()
	orgID, userID, agentID := uuid.New(), uuid.New(), uuid.New()
	suffix := orgID.String()[:8]

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM a2a_request_nonces WHERE agent_id = $1`, agentID)
		_, _ = db.ExecContext(ctx, `DELETE FROM agents WHERE id = $1`, agentID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, userID)
		_, _ = db.ExecContext(ctx, `DELETE FROM organizations WHERE id = $1`, orgID)
	})

	_, err = db.ExecContext(ctx,
		`INSERT INTO organizations (id, name, domain, created_at, updated_at)
		 VALUES ($1, $2, $3, NOW(), NOW())`,
		orgID, "nonce-cleanup-org-"+suffix, "nonce-cleanup-"+suffix+".example.com")
	require.NoError(t, err)
	_, err = db.ExecContext(ctx,
		`INSERT INTO users (id, organization_id, email, name, password_hash, role,
		                    provider, provider_id, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, 'x', 'admin', 'local', $5, NOW(), NOW())`,
		userID, orgID, "nonce-cleanup-"+suffix+"@example.com", "nonce-cleanup-user", "local-"+suffix)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx,
		`INSERT INTO agents (id, organization_id, name, display_name, agent_type, status,
		                     created_by, created_at, updated_at)
		 VALUES ($1, $2, $3, 'Nonce Cleanup Agent', 'ai_agent', 'verified', $4, NOW(), NOW())`,
		agentID, orgID, "nonce-cleanup-agent-"+suffix, userID)
	require.NoError(t, err)

	now := time.Now().UTC()
	plant := func(name string, usedAgo, expiresIn time.Duration) string {
		t.Helper()
		nonce := "nonce-cleanup-" + name + "-" + suffix
		_, err := db.ExecContext(ctx,
			`INSERT INTO a2a_request_nonces (nonce, agent_id, used_at, expires_at, request_hash)
			 VALUES ($1, $2, $3, $4, $5)`,
			nonce, agentID, now.Add(-usedAgo), now.Add(expiresIn), "request-hash-"+name)
		require.NoError(t, err)
		return nonce
	}
	exists := func(nonce string) bool {
		t.Helper()
		var found bool
		require.NoError(t, db.QueryRowContext(ctx,
			`SELECT EXISTS(SELECT 1 FROM a2a_request_nonces WHERE nonce = $1)`, nonce).Scan(&found))
		return found
	}

	// Past expires_at + skew and used_at + 2*skew: no request can carry it again.
	stale := plant("stale", 20*time.Minute, -15*time.Minute)
	// Expired two minutes ago: a request timestamped up to five minutes ahead
	// of its first use is still inside the tolerance window.
	withinSkew := plant("within-skew", 7*time.Minute, -2*time.Minute)
	// A one-minute expiry, past expires_at + skew but not used_at + 2*skew.
	shortExpiry := plant("short-expiry", 8*time.Minute, -7*time.Minute)
	live := plant("live", 0, 5*time.Minute)
	for _, n := range []string{stale, withinSkew, shortExpiry, live} {
		require.True(t, exists(n), "planted nonce %s must be visible before the run", n)
	}

	svc := application.NewA2AService(nil, nil, nil, nil, nil, nil,
		repository.NewA2ARequestNonceRepository(db),
		nil, nil, nil, nil, nil, nil, nil, nil)

	deleted, err := svc.CleanupExpiredNonces(ctx)
	require.NoError(t, err)
	require.GreaterOrEqual(t, deleted, 1, "a run that deletes nothing while a planted nonce is past the window is a defect")
	require.False(t, exists(stale), "a nonce past the replay window must be deleted")
	for _, n := range []string{withinSkew, shortExpiry, live} {
		require.True(t, exists(n), "nonce %s can still be replayed and must be kept", n)
	}

	// The scheduled job deletes a planted stale nonce within one interval.
	const interval = 50 * time.Millisecond
	staleForJob := plant("stale-job", 30*time.Minute, -25*time.Minute)
	stop := startNonceCleanupJob(svc, interval)
	defer stop()

	deadline := time.Now().Add(interval + 2*time.Second)
	for exists(staleForJob) {
		if time.Now().After(deadline) {
			t.Fatalf("scheduled job did not delete a stale nonce within one %s interval", interval)
		}
		time.Sleep(10 * time.Millisecond)
	}
	for _, n := range []string{withinSkew, shortExpiry, live} {
		require.True(t, exists(n), "nonce %s can still be replayed and must be kept by the job", n)
	}
}
