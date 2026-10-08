//go:build integration

package repository

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// OnboardingEventRepository and migration 116 against PostgreSQL. The
// once-per-organization rule, the backfill and the baseline query are SQL,
// which a mock replaces, so they are pinned here.
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run TestOnboarding ./internal/infrastructure/repository/...

// onboardingSeedOrg inserts an organization created at createdAt and a user in it.
func onboardingSeedOrg(t *testing.T, db *sql.DB, createdAt time.Time) (orgID, userID uuid.UUID) {
	t.Helper()
	orgID, userID = uuid.New(), uuid.New()
	suffix := orgID.String()[:8]
	_, err := db.Exec(`INSERT INTO organizations (id, name, domain, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $4)`, orgID, "onboarding-org-"+suffix, "onboarding-"+suffix+".example.invalid", createdAt)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO users (id, organization_id, email, name, password_hash, role,
			provider, provider_id, created_at, updated_at)
		VALUES ($1, $2, $3, 'onboarding user', 'x', 'admin', 'local', $4, NOW(), NOW())`,
		userID, orgID, "onboarding-"+suffix+"@example.invalid", "local-"+suffix)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM organizations WHERE id = $1`, orgID)
	})
	return orgID, userID
}

func onboardingSeedAgent(t *testing.T, db *sql.DB, orgID, userID uuid.UUID, createdAt time.Time) {
	t.Helper()
	id := uuid.New()
	_, err := db.Exec(`INSERT INTO agents (id, organization_id, name, display_name, agent_type, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $3, 'custom', $4, $5, $5)`, id, orgID, "agent-"+id.String()[:8], userID, createdAt)
	require.NoError(t, err)
}

func onboardingFirstAgentEvents(t *testing.T, db *sql.DB, orgID uuid.UUID) []time.Time {
	t.Helper()
	rows, err := db.Query(`SELECT occurred_at FROM onboarding_events
		WHERE organization_id = $1 AND event = 'first_agent_registered'`, orgID)
	require.NoError(t, err)
	defer rows.Close()
	var out []time.Time
	for rows.Next() {
		var at time.Time
		require.NoError(t, rows.Scan(&at))
		out = append(out, at.UTC())
	}
	require.NoError(t, rows.Err())
	return out
}

func migration116(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "..", "migrations", "116_create_onboarding_events.sql"))
	require.NoError(t, err)
	return string(raw)
}

// The migration backfills first_agent_registered from agents.created_at, once
// per organization, and is safe to run again.
func TestOnboardingMigration_BackfillsFirstAgentFromExistingColumns(t *testing.T) {
	db := bootstrapTestDB(t)
	orgCreated := time.Date(2025, 3, 1, 9, 0, 0, 0, time.UTC)

	withAgents, user := onboardingSeedOrg(t, db, orgCreated)
	onboardingSeedAgent(t, db, withAgents, user, orgCreated.Add(2*time.Hour))
	onboardingSeedAgent(t, db, withAgents, user, orgCreated.Add(7*time.Minute)) // earliest
	onboardingSeedAgent(t, db, withAgents, user, orgCreated.Add(30*24*time.Hour))
	noAgents, _ := onboardingSeedOrg(t, db, orgCreated)

	sqlText := migration116(t)
	_, err := db.Exec(sqlText)
	require.NoError(t, err)
	_, err = db.Exec(sqlText) // a second run adds nothing
	require.NoError(t, err)

	assert.Equal(t, []time.Time{orgCreated.Add(7 * time.Minute)}, onboardingFirstAgentEvents(t, db, withAgents))
	assert.Empty(t, onboardingFirstAgentEvents(t, db, noAgents))
}

func TestOnboardingRepository_RecordFirstAgentIsOncePerOrgAndStampedWithTheAgent(t *testing.T) {
	db := bootstrapTestDB(t)
	repo := NewOnboardingEventRepository(db)
	ctx := context.Background()
	orgCreated := time.Date(2025, 4, 1, 9, 0, 0, 0, time.UTC)
	orgID, user := onboardingSeedOrg(t, db, orgCreated)

	require.NoError(t, repo.RecordFirstAgent(ctx, orgID), "no agent yet: records nothing, no error")
	assert.Empty(t, onboardingFirstAgentEvents(t, db, orgID))

	first := orgCreated.Add(45 * time.Second)
	onboardingSeedAgent(t, db, orgID, user, first)
	require.NoError(t, repo.RecordFirstAgent(ctx, orgID))
	onboardingSeedAgent(t, db, orgID, user, first.Add(time.Hour))
	require.NoError(t, repo.RecordFirstAgent(ctx, orgID))

	assert.Equal(t, []time.Time{first}, onboardingFirstAgentEvents(t, db, orgID))
}

func TestOnboardingRepository_RecordAndCountEvents(t *testing.T) {
	db := bootstrapTestDB(t)
	repo := NewOnboardingEventRepository(db)
	ctx := context.Background()
	orgA, _ := onboardingSeedOrg(t, db, time.Now().UTC())
	orgB, _ := onboardingSeedOrg(t, db, time.Now().UTC())
	// A window start far in the future isolates this test's rows from any
	// other rows in a shared database.
	at := time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)
	python := "python"

	require.NoError(t, repo.Record(ctx, &domain.OnboardingEvent{OrganizationID: orgA, Event: domain.OnboardingEventViewed, OccurredAt: at}))
	require.NoError(t, repo.Record(ctx, &domain.OnboardingEvent{OrganizationID: orgA, Event: domain.OnboardingEventViewed, OccurredAt: at.Add(time.Minute)}))
	require.NoError(t, repo.Record(ctx, &domain.OnboardingEvent{OrganizationID: orgB, Event: domain.OnboardingEventViewed, OccurredAt: at}))
	require.NoError(t, repo.Record(ctx, &domain.OnboardingEvent{OrganizationID: orgA, Event: domain.OnboardingEventTabSelected, Tab: &python, OccurredAt: at}))
	require.NoError(t, repo.Record(ctx, &domain.OnboardingEvent{OrganizationID: orgA, Event: domain.OnboardingEventSkipped, OccurredAt: at.Add(-time.Hour)})) // before the window

	// The schema refuses what the service refuses.
	bad := "someone@example.com"
	assert.Error(t, repo.Record(ctx, &domain.OnboardingEvent{OrganizationID: orgA, Event: domain.OnboardingEventTabSelected, Tab: &bad, OccurredAt: at}))
	assert.Error(t, repo.Record(ctx, &domain.OnboardingEvent{OrganizationID: orgA, Event: domain.OnboardingEventViewed, Tab: &python, OccurredAt: at}))
	assert.Error(t, repo.Record(ctx, &domain.OnboardingEvent{OrganizationID: orgA, Event: "agent_deleted", OccurredAt: at}))

	counts, err := repo.CountEventsSince(ctx, at)
	require.NoError(t, err)
	assert.ElementsMatch(t, []domain.OnboardingEventCount{
		{Event: domain.OnboardingEventViewed, Organizations: 2, Total: 3},
		{Event: domain.OnboardingEventTabSelected, Organizations: 1, Total: 1},
	}, counts)

	_, _ = db.Exec(`DELETE FROM onboarding_events WHERE organization_id IN ($1, $2)`, orgA, orgB)
}

// The baseline query reads organizations.created_at and agents.created_at, so
// it measures organizations no event was ever recorded for.
func TestOnboardingRepository_TimeToFirstAgentSamplesFromExistingColumns(t *testing.T) {
	db := bootstrapTestDB(t)
	repo := NewOnboardingEventRepository(db)
	ctx := context.Background()
	old := time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2001, 6, 1, 0, 0, 0, 0, time.UTC)

	oldOrg, oldUser := onboardingSeedOrg(t, db, old)
	onboardingSeedAgent(t, db, oldOrg, oldUser, old.Add(10*time.Minute))
	onboardingSeedAgent(t, db, oldOrg, oldUser, old.Add(3*time.Minute))
	newOrg, _ := onboardingSeedOrg(t, db, newer)
	// Remove any event the agent hook or a backfill wrote: the query must not need one.
	_, _ = db.Exec(`DELETE FROM onboarding_events WHERE organization_id IN ($1, $2)`, oldOrg, newOrg)

	byOrg := func(samples []domain.TimeToFirstAgentSample) map[uuid.UUID]domain.TimeToFirstAgentSample {
		out := make(map[uuid.UUID]domain.TimeToFirstAgentSample)
		for _, s := range samples {
			if s.OrganizationID == oldOrg || s.OrganizationID == newOrg {
				out[s.OrganizationID] = s
			}
		}
		return out
	}

	all, err := repo.TimeToFirstAgentSamples(ctx, nil)
	require.NoError(t, err)
	got := byOrg(all)
	require.Len(t, got, 2)
	require.NotNil(t, got[oldOrg].FirstAgentAt)
	assert.Equal(t, 3*time.Minute, got[oldOrg].FirstAgentAt.Sub(got[oldOrg].OrgCreatedAt))
	assert.Nil(t, got[newOrg].FirstAgentAt)

	since := time.Date(2001, 3, 1, 0, 0, 0, 0, time.UTC)
	recent, err := repo.TimeToFirstAgentSamples(ctx, &since)
	require.NoError(t, err)
	got = byOrg(recent)
	assert.Len(t, got, 1)
	assert.Contains(t, got, newOrg)
}
