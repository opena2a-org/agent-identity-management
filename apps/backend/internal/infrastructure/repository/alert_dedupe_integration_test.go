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

// The coalescing step behind AlertService.CreateAlert, against the real
// alerts table: CreateCoalesced counts a repeat only on an unacknowledged
// alert of the same organization and key created at or after the cut-off,
// the newest one when there are several, and inserts the alert otherwise.
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

func TestAlertDedupe_CreateCoalesced(t *testing.T) {
	db := openAlertDedupeTestDB(t)
	ctx := context.Background()
	repo := NewAlertRepository(db)

	orgID := seedAlertDedupeOrg(t, db, ctx)
	otherOrgID := seedAlertDedupeOrg(t, db, ctx)
	agentID := uuid.New()
	key := "capability_violation:" + agentID.String() + ":digest"
	t0 := time.Now().UTC().Truncate(time.Microsecond)

	newAlert := func(org uuid.UUID, dedupeKey string, createdAt time.Time) *domain.Alert {
		return &domain.Alert{
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
	}
	rowsIn := func(org uuid.UUID) int {
		t.Helper()
		var n int
		require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM alerts WHERE organization_id = $1`, org).Scan(&n))
		return n
	}
	occurrences := func(id uuid.UUID) (int, sql.NullTime) {
		t.Helper()
		var count int
		var lastSeen sql.NullTime
		require.NoError(t, db.QueryRowContext(ctx,
			`SELECT occurrence_count, last_seen_at FROM alerts WHERE id = $1`, id,
		).Scan(&count, &lastSeen))
		return count, lastSeen
	}

	open := newAlert(orgID, key, t0)
	created, err := repo.CreateCoalesced(open, t0.Add(-time.Hour), t0)
	require.NoError(t, err)
	require.True(t, created, "the first occurrence of a key is a new alert")
	assert.Equal(t, 1, open.OccurrenceCount)

	var storedKey sql.NullString
	require.NoError(t, db.QueryRowContext(ctx, `SELECT dedupe_key FROM alerts WHERE id = $1`, open.ID).Scan(&storedKey))
	assert.Equal(t, key, storedKey.String)
	count, lastSeen := occurrences(open.ID)
	assert.Equal(t, 1, count)
	assert.False(t, lastSeen.Valid)

	t.Run("a repeat of the open alert created at or after the cut-off is counted on it", func(t *testing.T) {
		seen := t0.Add(9 * time.Minute)
		for _, at := range []time.Time{seen, seen.Add(time.Second)} {
			created, err := repo.CreateCoalesced(newAlert(orgID, key, at), t0, at)
			require.NoError(t, err)
			assert.False(t, created)
		}

		assert.Equal(t, 1, rowsIn(orgID))
		count, lastSeen := occurrences(open.ID)
		assert.Equal(t, 3, count)
		require.True(t, lastSeen.Valid)
		assert.True(t, lastSeen.Time.Equal(seen.Add(time.Second)))

		got, err := repo.GetByID(open.ID)
		require.NoError(t, err)
		assert.Equal(t, 3, got.OccurrenceCount)
		require.NotNil(t, got.LastSeenAt)
		assert.True(t, got.LastSeenAt.Equal(seen.Add(time.Second)))
	})

	t.Run("an alert created before the cut-off is not counted on", func(t *testing.T) {
		later := t0.Add(11 * time.Minute)
		created, err := repo.CreateCoalesced(newAlert(orgID, key, later), t0.Add(time.Second), later)
		require.NoError(t, err)
		assert.True(t, created)
		assert.Equal(t, 2, rowsIn(orgID))
	})

	t.Run("of two open alerts, the repeat is counted on the newest", func(t *testing.T) {
		seen := t0.Add(12 * time.Minute)
		created, err := repo.CreateCoalesced(newAlert(orgID, key, seen), t0.Add(-time.Hour), seen)
		require.NoError(t, err)
		assert.False(t, created)

		alerts, err := repo.GetByOrganization(orgID, 10, 0)
		require.NoError(t, err)
		require.Len(t, alerts, 2)
		assert.Equal(t, 2, alerts[0].OccurrenceCount, "the newest open alert is the one a repeat is counted on")
		assert.Equal(t, open.ID, alerts[1].ID)
		assert.Equal(t, 3, alerts[1].OccurrenceCount)
	})

	t.Run("another organization with the same key is a new alert", func(t *testing.T) {
		created, err := repo.CreateCoalesced(newAlert(otherOrgID, key, t0), t0.Add(-time.Hour), t0)
		require.NoError(t, err)
		assert.True(t, created)
		assert.Equal(t, 1, rowsIn(otherOrgID))
	})

	t.Run("another key is a new alert, and an alert without a key stores NULL", func(t *testing.T) {
		created, err := repo.CreateCoalesced(newAlert(orgID, key+"-other", t0), t0.Add(-time.Hour), t0)
		require.NoError(t, err)
		assert.True(t, created)

		plain := newAlert(orgID, "", t0)
		require.NoError(t, repo.Create(plain))
		var k sql.NullString
		require.NoError(t, db.QueryRowContext(ctx, `SELECT dedupe_key FROM alerts WHERE id = $1`, plain.ID).Scan(&k))
		assert.False(t, k.Valid)

		_, err = repo.CreateCoalesced(newAlert(orgID, "", t0), t0.Add(-time.Hour), t0)
		assert.Error(t, err, "an alert without a key cannot be coalesced")
	})

	t.Run("an acknowledged alert is not counted on", func(t *testing.T) {
		_, err := db.ExecContext(ctx, `UPDATE alerts SET is_acknowledged = true WHERE organization_id = $1 AND dedupe_key = $2`, orgID, key)
		require.NoError(t, err)
		before := rowsIn(orgID)

		seen := t0.Add(13 * time.Minute)
		created, err := repo.CreateCoalesced(newAlert(orgID, key, seen), t0.Add(-time.Hour), seen)
		require.NoError(t, err)
		assert.True(t, created)
		assert.Equal(t, before+1, rowsIn(orgID))
	})
}
