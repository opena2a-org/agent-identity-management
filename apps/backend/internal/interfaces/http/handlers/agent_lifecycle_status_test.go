package handlers

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// lifecycleService holds one agent and applies domain.AgentStatusTransition the way
// AgentService does, so these tests read the status the route leaves behind.
func lifecycleService(agent *domain.Agent) *MockAgentServiceImpl {
	apply := func(act domain.AgentStatusAct) func(context.Context, uuid.UUID) error {
		return func(context.Context, uuid.UUID) error {
			to, write, err := domain.AgentStatusTransition(act, agent.Status)
			if err == nil && write {
				agent.Status = to
			}
			return err
		}
	}
	return &MockAgentServiceImpl{
		GetAgentFunc: func(context.Context, uuid.UUID) (*domain.Agent, error) {
			copied := *agent
			return &copied, nil
		},
		VerifyAgentFunc:     apply(domain.AgentStatusActVerify),
		SuspendAgentFunc:    apply(domain.AgentStatusActSuspend),
		ReactivateAgentFunc: apply(domain.AgentStatusActReactivate),
	}
}

func postLifecycleAct(t *testing.T, agent *domain.Agent, act string) (int, map[string]any) {
	t.Helper()
	userID := uuid.New()
	handler := NewAgentHandlerWithInterfaces(
		lifecycleService(agent),
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{},
		&MockAPIKeyServiceImpl{},
		nil,
		&MockAlertServiceImpl{},
		&MockVerificationEventServiceImpl{},
		&MockCapabilityServiceImpl{},
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		&MockMCPAttestationServiceImpl{},
	)
	app := createTestAppWithAuth(handler, agent.OrganizationID, userID)
	app.Post("/agents/:id/verify", handler.VerifyAgent)
	app.Post("/agents/:id/suspend", handler.SuspendAgent)
	app.Post("/agents/:id/reactivate", handler.ReactivateAgent)

	resp, err := app.Test(httptest.NewRequest("POST", "/agents/"+agent.ID.String()+"/"+act, nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	return resp.StatusCode, body
}

func newLifecycleAgent(status domain.AgentStatus) *domain.Agent {
	return &domain.Agent{ID: uuid.New(), OrganizationID: uuid.New(), Name: "lifecycle-agent", Status: status}
}

// POST /agents/:id/reactivate on a revoked agent is refused with a stated line and leaves
// the agent revoked. Before, it answered 200 and set the agent to verified with the key it
// held when it was revoked.
func TestReactivateRouteRefusesARevokedAgent(t *testing.T) {
	agent := newLifecycleAgent(domain.AgentStatusRevoked)

	status, body := postLifecycleAct(t, agent, "reactivate")

	assert.Equal(t, fiber.StatusConflict, status)
	assert.Equal(t, map[string]any{
		"error":      "This agent is revoked, and a revoked agent cannot be reactivated: register a new agent to replace it",
		"reasonCode": "agentStatusTransitionRefused",
		"code":       "agentStatusTransitionRefused",
	}, body)
	assert.Equal(t, domain.AgentStatusRevoked, agent.Status)
}

func TestLifecycleRoutesRefuseActsTheStatusDoesNotAllow(t *testing.T) {
	for _, tc := range []struct {
		act  string
		from domain.AgentStatus
		line string
	}{
		{"verify", domain.AgentStatusRevoked, "This agent is revoked, and a revoked agent cannot be verified: register a new agent to replace it"},
		{"suspend", domain.AgentStatusRevoked, "This agent is revoked, and a revoked agent cannot be suspended"},
		{"verify", domain.AgentStatusSuspended, "This agent is suspended: reactivate it instead"},
		{"reactivate", domain.AgentStatusPending, "This agent is pending: verify it instead"},
	} {
		t.Run(tc.act+"/"+string(tc.from), func(t *testing.T) {
			agent := newLifecycleAgent(tc.from)

			status, body := postLifecycleAct(t, agent, tc.act)

			assert.Equal(t, fiber.StatusConflict, status)
			assert.Equal(t, tc.line, body["error"])
			assertReasonCode(t, body, "agentStatusTransitionRefused")
			assert.Equal(t, tc.from, agent.Status, "a refused act leaves the status as it was")
		})
	}
}

// A 200 says whether the act wrote anything: changed is false when the agent was already
// in the act's target status.
func TestLifecycleRoutesStateWhetherTheyChangedTheAgent(t *testing.T) {
	for _, tc := range []struct {
		act     string
		from    domain.AgentStatus
		to      domain.AgentStatus
		changed bool
	}{
		{"reactivate", domain.AgentStatusSuspended, domain.AgentStatusVerified, true},
		{"reactivate", domain.AgentStatusVerified, domain.AgentStatusVerified, false},
		{"suspend", domain.AgentStatusVerified, domain.AgentStatusSuspended, true},
		{"suspend", domain.AgentStatusSuspended, domain.AgentStatusSuspended, false},
		{"verify", domain.AgentStatusPending, domain.AgentStatusVerified, true},
		{"verify", domain.AgentStatusVerified, domain.AgentStatusVerified, false},
	} {
		t.Run(tc.act+"/"+string(tc.from), func(t *testing.T) {
			agent := newLifecycleAgent(tc.from)

			status, body := postLifecycleAct(t, agent, tc.act)

			assert.Equal(t, fiber.StatusOK, status)
			assert.Equal(t, tc.changed, body["changed"])
			assert.Equal(t, tc.to, agent.Status)
		})
	}
}
