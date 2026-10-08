package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOnboardingRepository_TimeToFirstAgentSamplesScansOrgsWithoutAgents(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewOnboardingEventRepository(db)

	withAgent, without := uuid.New(), uuid.New()
	created := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	first := created.Add(4 * time.Minute)
	mock.ExpectQuery(regexp.QuoteMeta("FROM organizations o")).
		WithArgs(nil).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "min"}).
			AddRow(withAgent, created, first).
			AddRow(without, created, nil))

	samples, err := repo.TimeToFirstAgentSamples(context.Background(), nil)
	require.NoError(t, err)
	require.Len(t, samples, 2)
	require.NotNil(t, samples[0].FirstAgentAt)
	assert.Equal(t, first, *samples[0].FirstAgentAt)
	assert.Nil(t, samples[1].FirstAgentAt)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestOnboardingRepository_TimeToFirstAgentSamplesPassesTheWindowStart(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewOnboardingEventRepository(db)

	since := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta("FROM organizations o")).
		WithArgs(since).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "min"}))

	samples, err := repo.TimeToFirstAgentSamples(context.Background(), &since)
	require.NoError(t, err)
	assert.NotNil(t, samples, "an empty result is an empty slice, never null")
	assert.Empty(t, samples)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestOnboardingRepository_RecordFirstAgentUsesTheAgentsTimestampOnce(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewOnboardingEventRepository(db)
	orgID := uuid.New()

	mock.ExpectExec(regexp.QuoteMeta(recordFirstAgentQuery)).
		WithArgs(orgID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	require.NoError(t, repo.RecordFirstAgent(context.Background(), orgID))
	require.NoError(t, mock.ExpectationsWereMet())
	assert.Contains(t, recordFirstAgentQuery, "MIN(created_at)")
	assert.Contains(t, recordFirstAgentQuery, "ON CONFLICT (organization_id) WHERE event = 'first_agent_registered' DO NOTHING")
}

func TestOnboardingRepository_RecordWritesNoIdentityColumns(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewOnboardingEventRepository(db)
	orgID := uuid.New()
	tab := "python"
	at := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO onboarding_events (organization_id, event, tab, occurred_at)")).
		WithArgs(orgID, "tab_selected", &tab, at).
		WillReturnResult(sqlmock.NewResult(0, 1))

	require.NoError(t, repo.Record(context.Background(), &domain.OnboardingEvent{
		OrganizationID: orgID, Event: domain.OnboardingEventTabSelected, Tab: &tab, OccurredAt: at,
	}))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestOnboardingRepository_RecordCappedReportsWhetherItInserted(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewOnboardingEventRepository(db)
	orgID := uuid.New()
	at := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	since := at.Add(-24 * time.Hour)
	e := &domain.OnboardingEvent{OrganizationID: orgID, Event: domain.OnboardingEventViewed, OccurredAt: at}

	mock.ExpectExec(regexp.QuoteMeta(recordCappedQuery)).
		WithArgs(orgID, "onboarding_viewed", (*string)(nil), at, since, 20).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(recordCappedQuery)).
		WithArgs(orgID, "onboarding_viewed", (*string)(nil), at, since, 20).
		WillReturnResult(sqlmock.NewResult(0, 0))

	inserted, err := repo.RecordCapped(context.Background(), e, since, 20)
	require.NoError(t, err)
	assert.True(t, inserted)
	inserted, err = repo.RecordCapped(context.Background(), e, since, 20)
	require.NoError(t, err)
	assert.False(t, inserted, "no row inserted: the organization is at the cap")
	require.NoError(t, mock.ExpectationsWereMet())

	// The count and the insert are one statement, keyed on the same
	// organization, event and tab as the row being inserted.
	assert.Contains(t, recordCappedQuery, "tab IS NOT DISTINCT FROM $3::varchar")
	assert.Contains(t, recordCappedQuery, ") < $6::int")
}
