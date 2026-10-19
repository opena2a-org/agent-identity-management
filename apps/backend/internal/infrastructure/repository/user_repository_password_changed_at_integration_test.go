//go:build integration

package repository

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Update only moves users.password_changed_at forward, held against PostgreSQL
// rather than against the query's text. A password change records its instant,
// and every session signed in before it is refused on that instant, so a later
// write of a stale copy of the row (one read without the column, or read before
// the change) must leave the instant in place, or the sessions the change ended
// would be admitted again. An older instant leaves it in place too; only a later
// one replaces it, and a first change is recorded over NULL.
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run TestUserRepository_UpdatePasswordChangedAtOnlyMovesForward ./internal/infrastructure/repository/...

func passwordChangedAtTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping users.password_changed_at update test")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Ping())
	return db
}

// seedPasswordChangedAtUser inserts an organization and one local user whose
// password_changed_at is NULL, and removes both when the test ends.
func seedPasswordChangedAtUser(t *testing.T, ctx context.Context, db *sql.DB) uuid.UUID {
	t.Helper()

	orgID, userID := uuid.New(), uuid.New()
	suffix := userID.String()[:8]
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, userID)
		_, _ = db.ExecContext(ctx, `DELETE FROM organizations WHERE id = $1`, orgID)
	})

	_, err := db.ExecContext(ctx,
		`INSERT INTO organizations (id, name, domain, created_at, updated_at)
		 VALUES ($1, $2, $3, NOW(), NOW())`,
		orgID, "pwchanged-org-"+suffix, "pwchanged-"+suffix+".example.com")
	require.NoError(t, err)

	_, err = db.ExecContext(ctx,
		`INSERT INTO users (id, organization_id, email, name, password_hash, role,
		                    provider, provider_id, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, 'x', 'admin', 'local', $5, NOW(), NOW())`,
		userID, orgID, "pwchanged-"+suffix+"@example.com", "pwchanged-user", "local-"+suffix)
	require.NoError(t, err)
	return userID
}

func TestUserRepository_UpdatePasswordChangedAtOnlyMovesForward(t *testing.T) {
	ctx := context.Background()
	db := passwordChangedAtTestDB(t)
	userID := seedPasswordChangedAtUser(t, ctx, db)
	repo := NewUserRepository(db)

	first := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	older := first.Add(-time.Hour)
	later := first.Add(time.Hour)

	// setRecorded puts the row in the state the case starts from.
	setRecorded := func(t *testing.T, at *time.Time) {
		t.Helper()
		_, err := db.ExecContext(ctx, `UPDATE users SET password_changed_at = $1 WHERE id = $2`, at, userID)
		require.NoError(t, err)
	}
	// recorded reads the column back the way the auth middleware does.
	recorded := func(t *testing.T) *time.Time {
		t.Helper()
		state, err := repo.GetSessionState(ctx, userID)
		require.NoError(t, err)
		return state.PasswordChangedAt
	}
	// writeCopy writes the row back through Update from a copy that carries
	// the given change: nil is a copy read without the column or before the
	// change.
	writeCopy := func(t *testing.T, carried *time.Time) {
		t.Helper()
		row, err := repo.GetByID(userID)
		require.NoError(t, err)
		row.PasswordChangedAt = carried
		require.NoError(t, repo.Update(row))
	}

	for _, tc := range []struct {
		name     string
		recorded *time.Time
		carried  *time.Time
		want     *time.Time
	}{
		{"a first change is recorded over NULL", nil, &first, &first},
		{"a copy that carries no change leaves NULL in place", nil, nil, nil},
		{"a copy that carries no change leaves the recorded change in place", &first, nil, &first},
		{"a copy that carries an older change leaves the recorded change in place", &first, &older, &first},
		{"a copy that carries the recorded change leaves it in place", &first, &first, &first},
		{"a later change replaces the recorded one", &first, &later, &later},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setRecorded(t, tc.recorded)
			writeCopy(t, tc.carried)
			got := recorded(t)
			if tc.want == nil {
				assert.Nil(t, got)
				return
			}
			require.NotNil(t, got, "the recorded change was cleared")
			assert.True(t, tc.want.Equal(*got), "recorded %v, want %v", got, tc.want)
		})
	}
}
