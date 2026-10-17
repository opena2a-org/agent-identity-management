package repository

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// Every alert channel query names the organization, so a channel is read,
// changed or deleted only inside the organization it belongs to.

func newAlertChannelRepo(t *testing.T) (*AlertChannelRepository, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, mock.ExpectationsWereMet())
		_ = db.Close()
	})
	return NewAlertChannelRepository(db), mock
}

var alertChannelRowColumns = []string{"id", "organization_id", "name", "kind", "config", "min_severity", "is_active",
	"last_delivery_status", "last_delivery_code", "last_delivery_at", "created_by", "created_at", "updated_at"}

func TestAlertChannelRepository_CreateStoresTheConfigAsJSON(t *testing.T) {
	repo, mock := newAlertChannelRepo(t)
	user := uuid.New()
	channel := &domain.AlertChannel{ID: uuid.New(), OrganizationID: uuid.New(), Name: "ops", Kind: domain.AlertChannelPagerDuty,
		Config: map[string]string{"routingKey": "k"}, MinSeverity: domain.AlertSeverityHigh, IsActive: true, CreatedBy: &user}

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO alert_channels")).
		WithArgs(channel.ID, channel.OrganizationID, "ops", "pagerduty", []byte(`{"routingKey":"k"}`), "high", true, &user,
			sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repo.Create(context.Background(), channel))
	assert.False(t, channel.CreatedAt.IsZero())
}

func TestAlertChannelRepository_ANameInUseIsNameTaken(t *testing.T) {
	repo, mock := newAlertChannelRepo(t)
	channel := &domain.AlertChannel{ID: uuid.New(), OrganizationID: uuid.New(), Name: "ops", Kind: domain.AlertChannelSlack,
		Config: map[string]string{}, MinSeverity: domain.AlertSeverityInfo}

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO alert_channels")).
		WillReturnError(&pq.Error{Code: "23505", Constraint: "alert_channels_organization_id_name_key"})
	assert.ErrorIs(t, repo.Create(context.Background(), channel), domain.ErrAlertChannelNameTaken)

	// Another unique refusal is not a name in use.
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO alert_channels")).
		WillReturnError(&pq.Error{Code: "23505", Constraint: "alert_channels_pkey"})
	err := repo.Create(context.Background(), channel)
	require.Error(t, err)
	assert.False(t, errors.Is(err, domain.ErrAlertChannelNameTaken))

	mock.ExpectExec(regexp.QuoteMeta("UPDATE alert_channels")).
		WillReturnError(&pq.Error{Code: "23505", Constraint: "alert_channels_organization_id_name_key"})
	assert.ErrorIs(t, repo.Update(context.Background(), channel), domain.ErrAlertChannelNameTaken)
}

func TestAlertChannelRepository_GetByIDIsScopedToTheOrganization(t *testing.T) {
	repo, mock := newAlertChannelRepo(t)
	org, id := uuid.New(), uuid.New()
	at := time.Date(2026, 10, 16, 1, 2, 3, 0, time.UTC)

	mock.ExpectQuery(regexp.QuoteMeta("FROM alert_channels WHERE id = $1 AND organization_id = $2")).
		WithArgs(id, org).
		WillReturnRows(sqlmock.NewRows(alertChannelRowColumns).
			AddRow(id, org, "ops", "slack", `{"webhookUrl":"u"}`, "warning", true, "delivered", 200, at, nil, at, at))
	channel, err := repo.GetByID(context.Background(), org, id)
	require.NoError(t, err)
	assert.Equal(t, domain.AlertChannelSlack, channel.Kind)
	assert.Equal(t, domain.AlertSeverityWarning, channel.MinSeverity)
	assert.Equal(t, map[string]string{"webhookUrl": "u"}, channel.Config)
	require.NotNil(t, channel.LastDeliveryStatus)
	assert.Equal(t, "delivered", *channel.LastDeliveryStatus)
	require.NotNil(t, channel.LastDeliveryCode)
	assert.Equal(t, 200, *channel.LastDeliveryCode)
	assert.Nil(t, channel.CreatedBy)

	mock.ExpectQuery(regexp.QuoteMeta("FROM alert_channels WHERE id = $1 AND organization_id = $2")).
		WithArgs(id, org).
		WillReturnError(sql.ErrNoRows)
	_, err = repo.GetByID(context.Background(), org, id)
	assert.ErrorIs(t, err, domain.ErrAlertChannelNotFound)
}

func TestAlertChannelRepository_ListActiveForFloors(t *testing.T) {
	repo, mock := newAlertChannelRepo(t)
	org := uuid.New()

	mock.ExpectQuery(regexp.QuoteMeta("WHERE organization_id = $1 AND is_active AND min_severity = ANY($2)")).
		WithArgs(org, pq.Array([]string{"info", "warning"})).
		WillReturnRows(sqlmock.NewRows(alertChannelRowColumns))
	channels, err := repo.ListActiveForFloors(context.Background(), org, []domain.AlertSeverity{"info", "warning"})
	require.NoError(t, err)
	assert.Empty(t, channels)
	assert.NotNil(t, channels)
}

func TestAlertChannelRepository_WritesAreScopedToTheOrganization(t *testing.T) {
	repo, mock := newAlertChannelRepo(t)
	org, id := uuid.New(), uuid.New()
	at := time.Now().UTC()

	mock.ExpectExec(regexp.QuoteMeta("UPDATE alert_channels SET name = $3, kind = $4, config = $5, min_severity = $6, is_active = $7, updated_at = $8 WHERE id = $1 AND organization_id = $2")).
		WillReturnResult(sqlmock.NewResult(0, 0))
	assert.ErrorIs(t, repo.Update(context.Background(), &domain.AlertChannel{ID: id, OrganizationID: org, Config: map[string]string{}}),
		domain.ErrAlertChannelNotFound)

	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM alert_channels WHERE id = $1 AND organization_id = $2")).
		WithArgs(id, org).
		WillReturnResult(sqlmock.NewResult(0, 0))
	assert.ErrorIs(t, repo.Delete(context.Background(), org, id), domain.ErrAlertChannelNotFound)

	// A delivery after a network error records no code.
	mock.ExpectExec(regexp.QuoteMeta("SET last_delivery_status = $3, last_delivery_code = $4, last_delivery_at = $5 WHERE id = $1 AND organization_id = $2")).
		WithArgs(id, org, "failed", nil, at).
		WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repo.RecordDelivery(context.Background(), org, id, "failed", nil, at))
}
