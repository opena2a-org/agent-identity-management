package application

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// expectWebhookInsert expects one webhook insert and records its secret and
// is_active columns.
func expectWebhookInsert(mock sqlmock.Sqlmock) (secret, isActive *columnLog) {
	secret, isActive = &columnLog{}, &columnLog{}
	args := make([]driver.Value, 15)
	for i := range args {
		args[i] = sqlmock.AnyArg()
	}
	args[5], args[6] = secret, isActive
	mock.ExpectExec("INSERT INTO webhooks").WithArgs(args...).WillReturnResult(sqlmock.NewResult(0, 1))
	return secret, isActive
}

func webhookRequest(events ...domain.WebhookEvent) *CreateWebhookRequest {
	return &CreateWebhookRequest{Name: "hook", URL: "https://1.1.1.1/hook", Events: events}
}

// A caller-chosen secret of 32 characters or more is stored as given, and
// the created webhook holds it.
func TestCreateWebhook_StoresACallerChosenSecretAsGiven(t *testing.T) {
	for _, length := range []int{MinWebhookSecretLength, 40, MaxWebhookSecretLength} {
		svc, mock := newMockedWebhookService(t)
		stored, _ := expectWebhookInsert(mock)
		req := webhookRequest(domain.WebhookEventAgentCreated)
		req.Secret = strings.Repeat("c", length)

		webhook, err := svc.CreateWebhook(context.Background(), req, uuid.New(), uuid.New())
		require.NoError(t, err, "a %d-character secret", length)
		assert.Equal(t, req.Secret, webhook.Secret)
		assert.Equal(t, []driver.Value{req.Secret}, stored.all())
		require.NoError(t, mock.ExpectationsWereMet())
	}
}

// With no secret in the request, a 64-character hex secret is generated.
func TestCreateWebhook_GeneratesASecretWhenNoneIsGiven(t *testing.T) {
	svc, mock := newMockedWebhookService(t)
	stored, _ := expectWebhookInsert(mock)

	webhook, err := svc.CreateWebhook(context.Background(), webhookRequest(domain.WebhookEventAgentCreated), uuid.New(), uuid.New())
	require.NoError(t, err)
	assert.Regexp(t, "^[0-9a-f]{64}$", webhook.Secret)
	assert.Equal(t, []driver.Value{webhook.Secret}, stored.all())
	require.NoError(t, mock.ExpectationsWereMet())
}

// A secret shorter than 32 or longer than 255 characters is refused with the
// length it misses, and nothing is stored. Length is counted in characters.
func TestCreateWebhook_RefusesASecretOutsideItsLengths(t *testing.T) {
	cases := map[string]string{
		strings.Repeat("s", MinWebhookSecretLength-1): "secret must be at least 32 characters",
		strings.Repeat("é", MinWebhookSecretLength-1): "secret must be at least 32 characters",
		strings.Repeat("s", MaxWebhookSecretLength+1): "secret must be at most 255 characters",
	}
	for secret, reason := range cases {
		svc, mock := newMockedWebhookService(t)
		req := webhookRequest(domain.WebhookEventAgentCreated)
		req.Secret = secret

		_, err := svc.CreateWebhook(context.Background(), req, uuid.New(), uuid.New())
		var requestErr *WebhookRequestError
		require.True(t, errors.As(err, &requestErr), "a %d-byte secret: %v", len(secret), err)
		assert.Equal(t, reason, requestErr.Reason)
		require.NoError(t, mock.ExpectationsWereMet(), "a refused secret stores nothing")
	}
}

// An event outside domain.AllWebhookEvents is refused by name on create and
// on update; the five accepted names no event is sent for stay accepted.
func TestCreateAndUpdateWebhook_RefuseAnEventTheServerDoesNotKnow(t *testing.T) {
	svc, mock := newMockedWebhookService(t)

	_, err := svc.CreateWebhook(context.Background(),
		webhookRequest(domain.WebhookEventAgentCreated, "auth.failed"), uuid.New(), uuid.New())
	var requestErr *WebhookRequestError
	require.True(t, errors.As(err, &requestErr), "%v", err)
	assert.Equal(t, "unsupported event: auth.failed", requestErr.Reason)

	_, err = svc.UpdateWebhook(context.Background(), uuid.New(), webhookRequest("verification.failed"))
	require.True(t, errors.As(err, &requestErr), "%v", err)
	assert.Equal(t, "unsupported event: verification.failed", requestErr.Reason)
	require.NoError(t, mock.ExpectationsWereMet(), "a refused event reads and stores nothing")

	expectWebhookInsert(mock)
	_, err = svc.CreateWebhook(context.Background(), webhookRequest(
		domain.WebhookEventAPIKeyExpired, domain.WebhookEventSecurityViolation, domain.WebhookEventComplianceViolation,
		domain.WebhookEventCapabilityDrift, domain.WebhookEventDataExfiltration), uuid.New(), uuid.New())
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

// Every event the server defines is one a webhook can subscribe to, and no
// other name is.
func TestIsKnownWebhookEvent_MatchesAllWebhookEvents(t *testing.T) {
	for _, event := range domain.AllWebhookEvents() {
		assert.True(t, domain.IsKnownWebhookEvent(event), "%s", event)
	}
	for _, event := range []domain.WebhookEvent{"", "auth.failed", "agent.reactivated", "trust_score.critical",
		"alert.resolved", "verification.failed", "AGENT.CREATED", "agent.created "} {
		assert.False(t, domain.IsKnownWebhookEvent(event), "%q", event)
	}
}

// An update does not change the secret, and refuses a request carrying one.
func TestUpdateWebhook_RefusesASecret(t *testing.T) {
	svc, mock := newMockedWebhookService(t)
	req := webhookRequest(domain.WebhookEventAgentCreated)
	req.Secret = strings.Repeat("u", 40)

	_, err := svc.UpdateWebhook(context.Background(), uuid.New(), req)
	var requestErr *WebhookRequestError
	require.True(t, errors.As(err, &requestErr), "%v", err)
	assert.Equal(t, "unknown field: secret", requestErr.Reason)
	require.NoError(t, mock.ExpectationsWereMet())
}

// A create that sets isActive to false stores the webhook inactive; without
// it, the webhook is active.
func TestCreateWebhook_StoresIsActive(t *testing.T) {
	inactive := false
	for _, isActive := range []*bool{nil, &inactive} {
		svc, mock := newMockedWebhookService(t)
		_, stored := expectWebhookInsert(mock)
		req := webhookRequest(domain.WebhookEventAgentCreated)
		req.IsActive = isActive

		webhook, err := svc.CreateWebhook(context.Background(), req, uuid.New(), uuid.New())
		require.NoError(t, err)
		assert.Equal(t, isActive == nil, webhook.IsActive)
		assert.Equal(t, []driver.Value{isActive == nil}, stored.all())
	}
}

// Rotation stores a new generated secret on the organization's webhook only,
// and returns it.
func TestRotateWebhookSecret_StoresANewSecretScopedToTheOrganization(t *testing.T) {
	svc, mock := newMockedWebhookService(t)
	id, org := uuid.New(), uuid.New()
	stored := &columnLog{}
	mock.ExpectExec(`UPDATE webhooks SET secret = \$1, updated_at = \$2\s+WHERE id = \$3 AND organization_id = \$4 AND deleted_at IS NULL`).
		WithArgs(stored, sqlmock.AnyArg(), id, org).WillReturnResult(sqlmock.NewResult(0, 1))

	secret, err := svc.RotateWebhookSecret(context.Background(), id, org)
	require.NoError(t, err)
	assert.Regexp(t, "^[0-9a-f]{64}$", secret)
	assert.Equal(t, []driver.Value{secret}, stored.all())
	require.NoError(t, mock.ExpectationsWereMet())
}

// Rotating a webhook the organization does not have changes nothing and
// returns sql.ErrNoRows.
func TestRotateWebhookSecret_ReturnsNoRowsForAnotherOrganizationsWebhook(t *testing.T) {
	svc, mock := newMockedWebhookService(t)
	mock.ExpectExec("UPDATE webhooks SET secret").WillReturnResult(sqlmock.NewResult(0, 0))

	secret, err := svc.RotateWebhookSecret(context.Background(), uuid.New(), uuid.New())
	assert.ErrorIs(t, err, sql.ErrNoRows)
	assert.Empty(t, secret)
	require.NoError(t, mock.ExpectationsWereMet())
}
