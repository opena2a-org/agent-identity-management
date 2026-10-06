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
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The coalescing lookup behind AlertService.CreateAlert, against the real
// alerts table: FindOpenByDedupeKey matches only an unacknowledged alert of
// the same organization and key created at or after the cut-off, and
// IncrementOccurrence counts on the row it is given.
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run TestAlertDedupe ./internal/infrastructure/repository/...

func openAlertDedupeTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping alert dedupe integration test")
	}

	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Ping())

	return db
}

func seedAlertDedupeOrg(t *testing.T, db *sql.DB, ctx context.Context) uuid.UUID {
	t.Helper()

	orgID := uuid.New()
	suffix := orgID.String()[:8]
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM alerts WHERE organization_id = $1`, orgID)
		_, _ = db.ExecContext(ctx, `DELETE FROM organizations WHERE id = $1`, orgID)
	})

	_, err := db.ExecContext(ctx,
		`INSERT INTO organizations (id, name, domain, created_at, updated_at)
		 VALUES ($1, $2, $3, NOW(), NOW())`,
		orgID, "alertdedupe-org-"+suffix, "alertdedupe-"+suffix+".example.com")
	require.NoError(t, err)
	return orgID
}

func TestAlertDedupe_FindOpenByDedupeKeyAndIncrementOccurrence(t *testing.T) {
	db := openAlertDedupeTestDB(t)
	ctx := context.Background()
	repo := NewAlertRepository(db)

	orgID := seedAlertDedupeOrg(t, db, ctx)
	otherOrgID := seedAlertDedupeOrg(t, db, ctx)
	agentID := uuid.New()
	key := "capability_violation:" + agentID.String() + ":digest"
	t0 := time.Now().UTC().Truncate(time.Microsecond)

	newAlert := func(org uuid.UUID, dedupeKey string, createdAt time.Time) *domain.Alert {
		a := &domain.Alert{
			OrganizationID: org,
			AlertType:      domain.AlertSecurityBreach,
			Severity:       domain.AlertSeverityHigh,
			Title:          "Capability Violation",
			Description:    "dedupe integration",
			ResourceType:   "agent",
			ResourceID:     agentID,
			DedupeKey:      dedupeKey,
			CreatedAt:      createdAt,
		}
		require.NoError(t, repo.Create(a))
		return a
	}

	open := newAlert(orgID, key, t0)

	var storedKey sql.NullString
	var count int
	var lastSeen sql.NullTime
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT dedupe_key, occurrence_count, last_seen_at FROM alerts WHERE id = $1`, open.ID,
	).Scan(&storedKey, &count, &lastSeen))
	assert.Equal(t, key, storedKey.String)
	assert.Equal(t, 1, count)
	assert.False(t, lastSeen.Valid)

	t.Run("matches the open alert created at or after the cut-off", func(t *testing.T) {
		got, err := repo.FindOpenByDedupeKey(orgID, key, t0)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, open.ID, got.ID)
		assert.Equal(t, key, got.DedupeKey)
	})

	t.Run("an alert created before the cut-off does not match", func(t *testing.T) {
		got, err := repo.FindOpenByDedupeKey(orgID, key, t0.Add(time.Second))
		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("another organization with the same key does not match", func(t *testing.T) {
		got, err := repo.FindOpenByDedupeKey(otherOrgID, key, t0.Add(-time.Hour))
		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("another key does not match, and an alert without a key stores NULL", func(t *testing.T) {
		plain := newAlert(orgID, "", t0)
		var k sql.NullString
		require.NoError(t, db.QueryRowContext(ctx, `SELECT dedupe_key FROM alerts WHERE id = $1`, plain.ID).Scan(&k))
		assert.False(t, k.Valid)

		got, err := repo.FindOpenByDedupeKey(orgID, key+"-other", t0.Add(-time.Hour))
		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("IncrementOccurrence counts and records when it was seen", func(t *testing.T) {
		seen := t0.Add(9 * time.Minute)
		require.NoError(t, repo.IncrementOccurrence(open.ID, seen))
		require.NoError(t, repo.IncrementOccurrence(open.ID, seen.Add(time.Second)))

		require.NoError(t, db.QueryRowContext(ctx,
			`SELECT occurrence_count, last_seen_at FROM alerts WHERE id = $1`, open.ID,
		).Scan(&count, &lastSeen))
		assert.Equal(t, 3, count)
		require.True(t, lastSeen.Valid)
		assert.True(t, lastSeen.Time.Equal(seen.Add(time.Second)))
	})

	t.Run("an acknowledged alert does not match, and the newest open one does", func(t *testing.T) {
		newer := newAlert(orgID, key, t0.Add(time.Minute))
		got, err := repo.FindOpenByDedupeKey(orgID, key, t0.Add(-time.Hour))
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, newer.ID, got.ID, "the newest open alert is the one a repeat is counted on")

		_, err = db.ExecContext(ctx, `UPDATE alerts SET is_acknowledged = true WHERE organization_id = $1 AND dedupe_key = $2`, orgID, key)
		require.NoError(t, err)
		got, err = repo.FindOpenByDedupeKey(orgID, key, t0.Add(-time.Hour))
		require.NoError(t, err)
		assert.Nil(t, got)
	})
}
