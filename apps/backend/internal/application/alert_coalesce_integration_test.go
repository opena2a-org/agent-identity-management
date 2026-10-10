//go:build integration

package application

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"sync"
	"testing"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
)

// Alert coalescing through AlertService and the real alert repository.
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run TestAlertCoalesce ./internal/application/...

func openAlertCoalesceTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping alert coalescing integration test")
	}

	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Ping())

	return db
}

func seedAlertCoalesceOrg(t *testing.T, db *sql.DB) uuid.UUID {
	t.Helper()

	ctx := context.Background()
	orgID := uuid.New()
	suffix := orgID.String()[:8]
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM alerts WHERE organization_id = $1`, orgID)
		_, _ = db.ExecContext(ctx, `DELETE FROM organizations WHERE id = $1`, orgID)
	})

	_, err := db.ExecContext(ctx,
		`INSERT INTO organizations (id, name, domain, created_at, updated_at)
		 VALUES ($1, $2, $3, NOW(), NOW())`,
		orgID, "alertcoalesce-org-"+suffix, "alertcoalesce-"+suffix+".example.com")
	require.NoError(t, err)
	return orgID
}

// Two denied verifications of the same capability arriving together on an
// empty alerts table are one violation: one alert, counted twice. Without a
// guard each request finds no open alert and each inserts one, and later
// repeats coalesce onto the newest row only.
func TestAlertCoalesce_ConcurrentFirstOccurrencesCreateOneAlert(t *testing.T) {
	db := openAlertCoalesceTestDB(t)
	db.SetMaxOpenConns(32)
	service := NewAlertService(repository.NewAlertRepository(db), nil, nil)

	const rounds, parallel = 5, 8
	for round := 0; round < rounds; round++ {
		orgID := seedAlertCoalesceOrg(t, db)
		agentID := uuid.New()

		start := make(chan struct{})
		errs := make(chan error, parallel)
		var wg sync.WaitGroup
		for i := 0; i < parallel; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				errs <- service.CreateAlert(context.Background(), capabilityViolationAlert(orgID, agentID, "file:write", "/etc/passwd"))
			}()
		}
		close(start)
		wg.Wait()
		close(errs)
		for err := range errs {
			require.NoError(t, err)
		}

		var rows, occurrences int
		require.NoError(t, db.QueryRow(
			`SELECT COUNT(*), COALESCE(SUM(occurrence_count), 0) FROM alerts WHERE organization_id = $1`, orgID,
		).Scan(&rows, &occurrences))
		assert.Equal(t, 1, rows, "round %d: concurrent first occurrences of one key must create one alert", round)
		assert.Equal(t, parallel, occurrences, "round %d: every occurrence is counted", round)
	}
}

// GET /api/v1/alerts serializes the alerts GetAlerts returns. A repeat
// coalesced onto an alert is visible there as occurrenceCount and lastSeenAt.
func TestAlertCoalesce_RepeatCountIsInTheAlertsResponse(t *testing.T) {
	db := openAlertCoalesceTestDB(t)
	service := NewAlertService(repository.NewAlertRepository(db), nil, nil)
	ctx := context.Background()

	orgID := seedAlertCoalesceOrg(t, db)
	agentID := uuid.New()
	for i := 0; i < 2; i++ {
		require.NoError(t, service.CreateAlert(ctx, capabilityViolationAlert(orgID, agentID, "file:write", "/etc/passwd")))
	}
	require.NoError(t, service.CreateAlert(ctx, capabilityViolationAlert(orgID, agentID, "file:write", "/etc/shadow")))

	alerts, total, err := service.GetAlerts(ctx, orgID, "", "", 10, 0)
	require.NoError(t, err)
	require.Equal(t, 2, total)

	body, err := json.Marshal(alerts)
	require.NoError(t, err)
	var got []map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &got))

	byDescription := make(map[string]map[string]interface{}, len(got))
	for _, a := range got {
		byDescription[a["description"].(string)] = a
	}

	repeated := byDescription["Agent attempted capability file:write on resource /etc/passwd"]
	require.NotNil(t, repeated)
	assert.EqualValues(t, 2, repeated["occurrenceCount"], "the repeat is counted in the response")
	assert.NotEmpty(t, repeated["lastSeenAt"], "the response says when the repeat arrived")

	single := byDescription["Agent attempted capability file:write on resource /etc/shadow"]
	require.NotNil(t, single)
	assert.EqualValues(t, 1, single["occurrenceCount"])
	assert.Contains(t, single, "lastSeenAt")
	assert.Nil(t, single["lastSeenAt"], "an alert that was never repeated has no last-seen time")

	byID, err := repository.NewAlertRepository(db).GetByID(uuid.MustParse(repeated["id"].(string)))
	require.NoError(t, err)
	body, err = json.Marshal(byID)
	require.NoError(t, err)
	var one map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &one))
	assert.EqualValues(t, 2, one["occurrenceCount"], "a single alert read by ID carries the count too")
}
