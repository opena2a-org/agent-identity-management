package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record/transition"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A reactivation and a verification let an agent act again, an expansion: each is made
// only together with its audit record, and when the record cannot be written the service
// refuses it whole with an error that wraps transition.ErrRecordUnavailable. The routes
// answer that refusal with 503, its reason and Retry-After, as the agent delete does.

// lifecycleRecordRefusal is the error AgentService returns for a status change whose
// record write failed: the recorder's refusal under the service's own prefix, with a
// cause the answer must not carry.
func lifecycleRecordRefusal(prefix string, trigger transition.Trigger) error {
	return fmt.Errorf("%s: %w", prefix, fmt.Errorf("%w (%s): %w", transition.ErrRecordUnavailable,
		trigger, errors.New(`pq: canceling statement due to lock timeout on "record_chains"`)))
}

// postRefusedLifecycleAct posts act for an agent in status, with the service answering
// actErr to all three status acts, and returns the response and how many audit rows the
// route wrote.
func postRefusedLifecycleAct(t *testing.T, status domain.AgentStatus, act string, actErr error) (int, string, map[string]any, int) {
	t.Helper()
	agent := newLifecycleAgent(status)
	refuse := func(context.Context, uuid.UUID) error { return actErr }
	audited := 0
	handler := NewAgentHandlerWithInterfaces(
		&MockAgentServiceImpl{
			GetAgentFunc: func(context.Context, uuid.UUID) (*domain.Agent, error) {
				copied := *agent
				return &copied, nil
			},
			VerifyAgentFunc:     refuse,
			SuspendAgentFunc:    refuse,
			ReactivateAgentFunc: refuse,
		},
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{
			LogActionFunc: func(context.Context, uuid.UUID, uuid.UUID, domain.AuditAction, string, uuid.UUID, string, string, map[string]interface{}) error {
				audited++
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
	app := createTestAppWithAuth(handler, agent.OrganizationID, uuid.New())
	app.Post("/agents/:id/verify", handler.VerifyAgent)
	app.Post("/agents/:id/suspend", handler.SuspendAgent)
	app.Post("/agents/:id/reactivate", handler.ReactivateAgent)

	resp, err := app.Test(httptest.NewRequest("POST", "/agents/"+agent.ID.String()+"/"+act, nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	return resp.StatusCode, resp.Header.Get("Retry-After"), body, audited
}

func TestReactivateAndVerifyRefusedByTheRecordPathAnswer503WithReasonAndRetryAfter(t *testing.T) {
	cases := []struct {
		act     string
		status  domain.AgentStatus
		refusal error
	}{
		{"reactivate", domain.AgentStatusSuspended,
			lifecycleRecordRefusal("failed to reactivate agent", transition.TriggerAgentReactivated)},
		{"verify", domain.AgentStatusPending,
			lifecycleRecordRefusal("failed to verify agent", transition.TriggerAgentVerified)},
	}
	for _, tc := range cases {
		t.Run(tc.act, func(t *testing.T) {
			status, retryAfter, body, audited := postRefusedLifecycleAct(t, tc.status, tc.act, tc.refusal)

			assert.Equal(t, fiber.StatusServiceUnavailable, status)
			assert.Equal(t, "300", retryAfter, "the record path window is 300 s today")
			assert.Equal(t, map[string]any{
				"error":      RecordUnavailableMessage,
				"reasonCode": "recordUnavailable",
				"code":       "recordUnavailable",
			}, body)
			assert.NotContains(t, fmt.Sprint(body), "lock timeout")
			assert.Zero(t, audited, "a refused "+tc.act+" writes no audit row of the act")
		})
	}
}

// Any other failure of a status act keeps the generic server error line and sends no
// Retry-After: only the record path's refusal promises a time to try again.
func TestLifecycleActOtherFailureKeepsTheGenericLineWithoutRetryAfter(t *testing.T) {
	for act, agentStatus := range map[string]domain.AgentStatus{
		"reactivate": domain.AgentStatusSuspended,
		"verify":     domain.AgentStatusPending,
		"suspend":    domain.AgentStatusVerified,
	} {
		t.Run(act, func(t *testing.T) {
			status, retryAfter, body, audited := postRefusedLifecycleAct(t, agentStatus, act, repositoryFailure)

			assert.Equal(t, fiber.StatusInternalServerError, status)
			assert.Empty(t, retryAfter)
			assert.Equal(t, map[string]any{"error": ServerErrorMessage}, body)
			assert.Zero(t, audited)
		})
	}
}
