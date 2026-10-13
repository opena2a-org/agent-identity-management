package handlers

import (
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record/store"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record/transition"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An approval grants a capability, an expansion: it is made only together with its audit
// record, and the service refuses it whole when the record path cannot be used. Here the
// path fails where an approval first reaches it, the read of the request's own record, and
// the service refuses with an error that wraps transition.ErrRecordUnavailable. The route
// answers that refusal with 503, its reason and Retry-After.
func approveCapabilityRequestApp(t *testing.T) (*fiber.App, string, sqlmock.Sqlmock) {
	t.Helper()
	orgID, userID, agentID, requestID := uuid.New(), uuid.New(), uuid.New(), uuid.New()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	mock.ExpectQuery(`FROM audit_records`).
		WillReturnError(errors.New(`pq: canceling statement due to lock timeout on "record_chains"`))
	recorder, err := transition.NewRecorder(transition.Config{
		Writer: &store.Writer{},
		DB:     db,
		Issuer: "urn:uuid:" + uuid.NewString(),
	})
	require.NoError(t, err)

	agents := &capRequestTestAgentRepo{
		getByID: func(id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{ID: agentID, OrganizationID: orgID, Name: "billing-bot"}, nil
		},
	}
	requests := &capRequestTestRequestRepo{
		getByID: func(id uuid.UUID) (*domain.CapabilityRequestWithDetails, error) {
			return &domain.CapabilityRequestWithDetails{CapabilityRequest: domain.CapabilityRequest{
				ID:             requestID,
				AgentID:        agentID,
				CapabilityType: "file:write",
				Status:         domain.CapabilityRequestStatusPending,
			}}, nil
		},
		updateStatusCh: make(chan struct{}, 1),
	}
	service := application.NewCapabilityRequestService(requests, nil, agents, nil)
	service.SetTransitionRecorder(recorder)
	handler := NewCapabilityRequestHandlers(service, agents)

	app := fiber.New()
	app.Post("/api/v1/admin/capability-requests/:id/approve", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		return handler.ApproveCapabilityRequest(c)
	})
	return app, "/api/v1/admin/capability-requests/" + requestID.String() + "/approve", mock
}

func TestApproveCapabilityRequestRefusedByTheRecordPathAnswers503WithReasonAndRetryAfter(t *testing.T) {
	app, path, mock := approveCapabilityRequestApp(t)

	resp, err := app.Test(httptest.NewRequest("POST", path, nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))

	assert.Equal(t, fiber.StatusServiceUnavailable, resp.StatusCode)
	assert.Equal(t, "300", resp.Header.Get("Retry-After"))
	assert.Equal(t, map[string]interface{}{
		"error":      RecordUnavailableMessage,
		"reasonCode": "recordUnavailable",
		"code":       "recordUnavailable",
	}, body)
	assert.NotContains(t, RecordUnavailableMessage, "lock timeout")
	assert.NoError(t, mock.ExpectationsWereMet(), "the approval reached the record path")
}
