//go:build integration

package main

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// One tick of the five-minute job against a real users table: a token past
// its expiry and a token stored without an expiry are cleared, a token that
// can still be redeemed is kept.
//
// Build-tag gated: requires Postgres reachable via TEST_DATABASE_URL with
// the AIM schema already applied (run migrations first).
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run TestExpirationCleanup_PasswordResetTokens ./cmd/server/...
func TestExpirationCleanup_PasswordResetTokens(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping password reset sweep test")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, db.Ping())

	ctx := context.Background()
	orgID := uuid.New()
	suffix := orgID.String()[:8]
	expiredID, noExpiryID, liveID := uuid.New(), uuid.New(), uuid.New()
	userIDs := []uuid.UUID{expiredID, noExpiryID, liveID}

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE organization_id = $1`, orgID)
		_, _ = db.ExecContext(ctx, `DELETE FROM organizations WHERE id = $1`, orgID)
	})

	_, err = db.ExecContext(ctx,
		`INSERT INTO organizations (id, name, domain, created_at, updated_at)
		 VALUES ($1, $2, $3, NOW(), NOW())`,
		orgID, "reset-sweep-org-"+suffix, "reset-sweep-"+suffix+".example.com")
	require.NoError(t, err)

	liveExpiry := time.Now().Add(time.Hour).UTC().Truncate(time.Microsecond)
	seed := []struct {
		id      uuid.UUID
		name    string
		expires any
	}{
		{expiredID, "expired", time.Now().Add(-time.Minute)},
		{noExpiryID, "no-expiry", nil},
		{liveID, "live", liveExpiry},
	}
	for _, u := range seed {
		_, err = db.ExecContext(ctx,
			`INSERT INTO users
			   (id, organization_id, email, name, password_hash, role, provider,
			    provider_id, password_reset_token, password_reset_expires_at,
			    created_at, updated_at)
			 VALUES ($1, $2, $3, $4, 'x', 'member', 'local', $5, $6, $7, NOW(), NOW())`,
			u.id, orgID, u.name+"-"+suffix+"@example.com", u.name, "local-"+u.name+"-"+suffix,
			"reset-token-"+u.name+"-"+suffix, u.expires)
		require.NoError(t, err)
	}

	// Positive control: the seeded rows are visible to this connection, so an
	// all-NULL result below means cleared, not unreadable.
	var visible int
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT count(*) FROM users WHERE id = ANY($1) AND password_reset_token IS NOT NULL`,
		pq.Array(userIDs)).Scan(&visible))
	require.Equal(t, 3, visible)

	runExpirationCleanup(db)

	read := func(id uuid.UUID) (sql.NullString, sql.NullTime) {
		var token sql.NullString
		var expires sql.NullTime
		require.NoError(t, db.QueryRowContext(ctx,
			`SELECT password_reset_token, password_reset_expires_at FROM users WHERE id = $1`, id,
		).Scan(&token, &expires))
		return token, expires
	}

	for _, id := range []uuid.UUID{expiredID, noExpiryID} {
		token, expires := read(id)
		require.False(t, token.Valid, "reset token on %s must be cleared", id)
		require.False(t, expires.Valid, "reset expiry on %s must be cleared", id)
	}

	token, expires := read(liveID)
	require.Equal(t, "reset-token-live-"+suffix, token.String, "a live reset token must be kept")
	require.True(t, expires.Valid, "a live reset expiry must be kept")
	require.True(t, liveExpiry.Equal(expires.Time.UTC()), "a live reset expiry must be unchanged")
}
