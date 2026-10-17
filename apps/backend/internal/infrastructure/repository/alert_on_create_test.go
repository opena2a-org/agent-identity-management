package repository

import (
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// OnCreate is how the server forwards alerts to a SIEM: each observer it adds
// is given each alert the repository inserts, and a repeat counted on an open
// alert is given to none.

func newAlertOnCreateRepo(t *testing.T) (*AlertRepository, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return NewAlertRepository(db), mock
}

// observed records, in order, the observer that was given each alert.
type observed struct {
	calls []string
	given []*domain.Alert
}

func (o *observed) observer(name string) func(*domain.Alert) {
	return func(alert *domain.Alert) {
		o.calls = append(o.calls, name)
		o.given = append(o.given, alert)
	}
}

func coalescedAlert() *domain.Alert {
	return &domain.Alert{
		OrganizationID: uuid.New(),
		AlertType:      domain.AlertSecurityBreach,
		Severity:       domain.AlertSeverityHigh,
		ResourceType:   "agent",
		ResourceID:     uuid.New(),
		DedupeKey:      "capability_violation:key",
	}
}

func TestAlertCreate_GivesTheInsertedAlertToEveryObserverInOrder(t *testing.T) {
	repo, mock := newAlertOnCreateRepo(t)
	var o observed
	repo.OnCreate(o.observer("first"))
	repo.OnCreate(o.observer("second"))

	mock.ExpectExec(`INSERT INTO alerts`).WillReturnResult(sqlmock.NewResult(0, 1))
	alert := &domain.Alert{OrganizationID: uuid.New(), AlertType: domain.AlertSecurityBreach, Severity: domain.AlertSeverityHigh}
	require.NoError(t, repo.Create(alert))

	assert.Equal(t, []string{"first", "second"}, o.calls)
	require.Len(t, o.given, 2)
	assert.Same(t, alert, o.given[0])
	assert.Same(t, alert, o.given[1])
	assert.NotEqual(t, uuid.Nil, alert.ID, "the alert is given with the id it was written with")
	assert.False(t, alert.CreatedAt.IsZero(), "and the time it was written with")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAlertCreate_DoesNotGiveAFailedWriteToObservers(t *testing.T) {
	repo, mock := newAlertOnCreateRepo(t)
	var o observed
	repo.OnCreate(o.observer("first"))

	mock.ExpectExec(`INSERT INTO alerts`).WillReturnError(errors.New("connection refused"))
	require.Error(t, repo.Create(&domain.Alert{OrganizationID: uuid.New()}))

	assert.Empty(t, o.calls)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAlertCreate_WithoutObservers(t *testing.T) {
	repo, mock := newAlertOnCreateRepo(t)

	mock.ExpectExec(`INSERT INTO alerts`).WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repo.Create(&domain.Alert{OrganizationID: uuid.New()}))
	require.NoError(t, mock.ExpectationsWereMet())
}

// A panic in one observer is recovered: the observers after it run and the
// write succeeds.
func TestAlertCreate_RecoversAPanickingObserver(t *testing.T) {
	repo, mock := newAlertOnCreateRepo(t)
	var o observed
	repo.OnCreate(func(*domain.Alert) { panic("observer failed") })
	repo.OnCreate(o.observer("second"))

	mock.ExpectExec(`INSERT INTO alerts`).WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repo.Create(&domain.Alert{OrganizationID: uuid.New()}))

	assert.Equal(t, []string{"second"}, o.calls)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAlertCreateCoalesced_GivesAnInsertedAlertToObservers(t *testing.T) {
	repo, mock := newAlertOnCreateRepo(t)
	var o observed
	repo.OnCreate(o.observer("first"))
	repo.OnCreate(o.observer("second"))

	mock.ExpectBegin()
	mock.ExpectExec(`SELECT pg_advisory_xact_lock`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`SELECT id\s+FROM alerts`).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectExec(`INSERT INTO alerts`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	alert := coalescedAlert()
	now := time.Now()
	inserted, err := repo.CreateCoalesced(alert, now.Add(-10*time.Minute), now)
	require.NoError(t, err)
	require.True(t, inserted)

	assert.Equal(t, []string{"first", "second"}, o.calls)
	assert.Same(t, alert, o.given[0])
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAlertCreateCoalesced_DoesNotGiveACountedRepeatToObservers(t *testing.T) {
	repo, mock := newAlertOnCreateRepo(t)
	var o observed
	repo.OnCreate(o.observer("first"))

	mock.ExpectBegin()
	mock.ExpectExec(`SELECT pg_advisory_xact_lock`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`SELECT id\s+FROM alerts`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
	mock.ExpectExec(`UPDATE alerts\s+SET occurrence_count`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	now := time.Now()
	inserted, err := repo.CreateCoalesced(coalescedAlert(), now.Add(-10*time.Minute), now)
	require.NoError(t, err)
	require.False(t, inserted)

	assert.Empty(t, o.calls, "a repeat counted on an open alert is given to no observer")
	require.NoError(t, mock.ExpectationsWereMet())
}

// The open alert was acknowledged before the increment ran, so the repeat is
// inserted as a new alert, and that alert is given to the observers.
func TestAlertCreateCoalesced_GivesARepeatInsertedAfterAnAcknowledgementToObservers(t *testing.T) {
	repo, mock := newAlertOnCreateRepo(t)
	var o observed
	repo.OnCreate(o.observer("first"))

	mock.ExpectBegin()
	mock.ExpectExec(`SELECT pg_advisory_xact_lock`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`SELECT id\s+FROM alerts`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
	mock.ExpectExec(`UPDATE alerts\s+SET occurrence_count`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`INSERT INTO alerts`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	now := time.Now()
	inserted, err := repo.CreateCoalesced(coalescedAlert(), now.Add(-10*time.Minute), now)
	require.NoError(t, err)
	require.True(t, inserted)

	assert.Equal(t, []string{"first"}, o.calls)
	require.NoError(t, mock.ExpectationsWereMet())
}

// An insert whose transaction does not commit wrote no alert.
func TestAlertCreateCoalesced_DoesNotGiveAnUncommittedInsertToObservers(t *testing.T) {
	repo, mock := newAlertOnCreateRepo(t)
	var o observed
	repo.OnCreate(o.observer("first"))

	mock.ExpectBegin()
	mock.ExpectExec(`SELECT pg_advisory_xact_lock`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`SELECT id\s+FROM alerts`).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectExec(`INSERT INTO alerts`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit().WillReturnError(errors.New("connection reset"))

	now := time.Now()
	inserted, err := repo.CreateCoalesced(coalescedAlert(), now.Add(-10*time.Minute), now)
	require.Error(t, err)
	assert.False(t, inserted)

	assert.Empty(t, o.calls)
	require.NoError(t, mock.ExpectationsWereMet())
}
