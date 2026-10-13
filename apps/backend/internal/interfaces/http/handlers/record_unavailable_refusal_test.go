package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record/store"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record/transition"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A delete of an agent is a destruction: it is made only together with its audit record,
// and when the record cannot be written the service refuses it whole with an error that
// wraps transition.ErrRecordUnavailable. The route answers that refusal with 503, its
// reason and Retry-After, so the dashboard can say nothing changed and when to try again.

// recordRefusal is the error AgentService.DeleteAgent returns when the record write fails,
// built the way the recorder builds it, with a cause the answer must not carry.
var recordRefusal = fmt.Errorf("%w (%s): %w", transition.ErrRecordUnavailable,
	transition.TriggerAgentDeleted, errors.New(`pq: relation "authorization_records" chain head locked`))

func deleteAgentApp(t *testing.T, deleteErr error, audited *int) (*fiber.App, string) {
	t.Helper()
	orgID, userID, agentID := uuid.New(), uuid.New(), uuid.New()
	handler := NewAgentHandlerWithInterfaces(
		&MockAgentServiceImpl{
			GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
				return &domain.Agent{ID: agentID, OrganizationID: orgID}, nil
			},
			DeleteAgentFunc: func(ctx context.Context, id uuid.UUID) error { return deleteErr },
		},
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{
			LogActionFunc: func(context.Context, uuid.UUID, uuid.UUID, domain.AuditAction, string, uuid.UUID, string, string, map[string]interface{}) error {
				*audited++
				return nil
			},
		},
		&MockAPIKeyServiceImpl{},
		nil,
		&MockAlertServiceImpl{},
		&MockVerificationEventServiceImpl{},
		&MockCapabilityServiceImpl{},
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		&MockMCPAttestationServiceImpl{},
	)
	app := createTestAppWithAuth(handler, orgID, userID)
	app.Delete("/api/v1/agents/:id", handler.DeleteAgent)
	return app, "/api/v1/agents/" + agentID.String()
}

func TestDeleteAgentRefusedByTheRecordPathAnswers503WithReasonAndRetryAfter(t *testing.T) {
	causes := map[string]error{
		"the recorder's refusal":        recordRefusal,
		"the refusal wrapped once more": fmt.Errorf("delete agent: %w", recordRefusal),
	}
	for name, cause := range causes {
		t.Run(name, func(t *testing.T) {
			audited := 0
			app, path := deleteAgentApp(t, cause, &audited)

			resp, err := app.Test(httptest.NewRequest("DELETE", path, nil))
			require.NoError(t, err)
			defer resp.Body.Close()
			var body map[string]interface{}
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))

			assert.Equal(t, fiber.StatusServiceUnavailable, resp.StatusCode)
			assert.Equal(t, strconv.Itoa(int(store.RecordPathWindow/time.Second)), resp.Header.Get("Retry-After"))
			assert.Equal(t, "300", resp.Header.Get("Retry-After"), "the record path window is 300 s today")
			assert.Equal(t, map[string]interface{}{
				"error":      RecordUnavailableMessage,
				"reasonCode": "recordUnavailable",
				"code":       "recordUnavailable",
			}, body)
			assert.Contains(t, RecordUnavailableMessage, "Try again in 5 minutes.")
			assert.NotContains(t, fmt.Sprint(body), "pq:")
			assert.Zero(t, audited, "a refused delete writes no delete audit row")
		})
	}
}

func TestDeleteAgentOtherFailureKeepsItsStatedLineWithoutRetryAfter(t *testing.T) {
	audited := 0
	app, path := deleteAgentApp(t, repositoryFailure, &audited)

	resp, err := app.Test(httptest.NewRequest("DELETE", path, nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
	assert.Empty(t, resp.Header.Get("Retry-After"))
	assert.Equal(t, map[string]interface{}{"error": agentDeleteFailedMessage}, body)
}

func TestRespondRecordUnavailableWritesNothingForAnotherError(t *testing.T) {
	app := fiber.New()
	app.Get("/probe", func(c fiber.Ctx) error {
		if refused, resp := respondRecordUnavailable(c, errors.New("some other failure")); refused {
			return resp
		}
		return c.SendStatus(fiber.StatusTeapot)
	})
	resp, err := app.Test(httptest.NewRequest("GET", "/probe", nil))
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusTeapot, resp.StatusCode)
	assert.Empty(t, resp.Header.Get("Retry-After"))
}
