package handlers

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The suspend, reactivate and revoke responses carry the agent as GET
// /agents/:id serves it: capabilities are the agent's active grants. The list
// the SDK reported at registration is a declaration stored on the agent row and
// is served under no field named capabilities.
func TestAgentActResponsesServeActiveGrantsAsCapabilities(t *testing.T) {
	for _, tc := range []struct {
		act  string
		from domain.AgentStatus
	}{
		{"suspend", domain.AgentStatusVerified},
		{"reactivate", domain.AgentStatusSuspended},
		{"revoke", domain.AgentStatusVerified},
	} {
		t.Run(tc.act, func(t *testing.T) {
			agent := newLifecycleAgent(tc.from)
			// Reported at registration. "db:write" was never granted and
			// "file:read" was granted and then revoked.
			agent.Capabilities = []string{"db:write", "file:read"}

			var askedActiveOnly []bool
			capabilities := &MockCapabilityServiceImpl{
				GetAgentCapabilitiesFunc: func(_ context.Context, agentID uuid.UUID, activeOnly bool) ([]*domain.AgentCapability, error) {
					require.Equal(t, agent.ID, agentID)
					askedActiveOnly = append(askedActiveOnly, activeOnly)
					active := []*domain.AgentCapability{
						{AgentID: agent.ID, CapabilityType: "api:call"},
						{AgentID: agent.ID, CapabilityType: "file:write"},
					}
					if activeOnly {
						return active, nil
					}
					return append(active, &domain.AgentCapability{AgentID: agent.ID, CapabilityType: "file:read"}), nil
				},
			}

			body := postAgentAct(t, agent, capabilities, tc.act)

			served, ok := body["agent"].(map[string]any)
			require.True(t, ok, "response carries the agent object: %v", body)
			assert.Equal(t, []any{"api:call", "file:write"}, served["capabilities"])
			assert.Equal(t, []bool{true}, askedActiveOnly, "the response reads the active grants once")
		})
	}
}

func postAgentAct(t *testing.T, agent *domain.Agent, capabilities *MockCapabilityServiceImpl, act string) map[string]any {
	t.Helper()
	handler := NewAgentHandlerWithInterfaces(
		lifecycleService(agent),
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{},
		&MockAPIKeyServiceImpl{},
		nil,
		&MockAlertServiceImpl{},
		&MockVerificationEventServiceImpl{},
		capabilities,
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		&MockMCPAttestationServiceImpl{},
	)
	app := createTestAppWithAuth(handler, agent.OrganizationID, uuid.New())
	app.Post("/agents/:id/suspend", handler.SuspendAgent)
	app.Post("/agents/:id/reactivate", handler.ReactivateAgent)
	app.Post("/agents/:id/revoke", handler.RevokeAgent)

	resp, err := app.Test(httptest.NewRequest("POST", "/agents/"+agent.ID.String()+"/"+act, nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, 200, resp.StatusCode)
	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	return body
}
