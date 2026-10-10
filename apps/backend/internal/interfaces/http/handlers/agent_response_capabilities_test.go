package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// getAgentResponse answers GET /agents/:id for agent with grants read from capabilities.
func getAgentResponse(t *testing.T, agent *domain.Agent, capabilities *MockCapabilityServiceImpl) (int, map[string]any) {
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
	app.Get("/agents/:id", handler.GetAgent)

	resp, err := app.Test(httptest.NewRequest("GET", "/agents/"+agent.ID.String(), nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	return resp.StatusCode, body
}

// GET /agents/:id and the suspend, reactivate and revoke responses list an
// agent's capabilities as its trust credential, agent card and A2A verification
// result do: no honeytoken grant, each capability once, sorted ascending.
func TestAgentResponsesListCapabilitiesAsTheCredentialDoes(t *testing.T) {
	agent := newLifecycleAgent(domain.AgentStatusVerified)
	decoy := &domain.AgentCapability{AgentID: agent.ID, CapabilityType: "secrets:read", Honeytoken: true}
	capabilities := &MockCapabilityServiceImpl{
		GetAgentCapabilitiesFunc: func(_ context.Context, _ uuid.UUID, _ bool) ([]*domain.AgentCapability, error) {
			return []*domain.AgentCapability{
				{AgentID: agent.ID, CapabilityType: "db:read"},
				decoy,
				{AgentID: agent.ID, CapabilityType: "api:call"},
				{AgentID: agent.ID, CapabilityType: "db:read"},
			}, nil
		},
	}
	want := []any{"api:call", "db:read"}

	status, body := getAgentResponse(t, agent, capabilities)
	require.Equal(t, fiber.StatusOK, status)
	assert.Equal(t, want, body["capabilities"], "GET /agents/:id")

	for _, act := range []string{"suspend", "reactivate", "revoke"} {
		served, ok := postAgentAct(t, agent, capabilities, act)["agent"].(map[string]any)
		require.True(t, ok, "the %s response carries the agent object", act)
		assert.Equal(t, want, served["capabilities"], act)
	}
}

// An agent with no active grant is served an empty capabilities array, never
// null and never a missing field: the dashboard iterates the list, and a
// missing field means the grants could not be read.
func TestAgentResponsesListNoGrantAsAnEmptyArray(t *testing.T) {
	agent := newLifecycleAgent(domain.AgentStatusVerified)
	agent.Capabilities = []string{"file:read"}
	capabilities := &MockCapabilityServiceImpl{
		GetAgentCapabilitiesFunc: func(context.Context, uuid.UUID, bool) ([]*domain.AgentCapability, error) {
			return nil, nil
		},
	}

	status, body := getAgentResponse(t, agent, capabilities)
	require.Equal(t, fiber.StatusOK, status)
	require.Contains(t, body, "capabilities", "GET /agents/:id")
	assert.Equal(t, []any{}, body["capabilities"], "GET /agents/:id")

	for _, act := range []string{"suspend", "reactivate", "revoke"} {
		served, ok := postAgentAct(t, agent, capabilities, act)["agent"].(map[string]any)
		require.True(t, ok, "the %s response carries the agent object", act)
		require.Contains(t, served, "capabilities", act)
		assert.Equal(t, []any{}, served["capabilities"], act)
	}
}

// GET /agents/:id serves the API key the agent was created with, which the
// agent detail page links to.
func TestGetAgentServesTheAPIKeyTheAgentWasCreatedWith(t *testing.T) {
	agent := newLifecycleAgent(domain.AgentStatusVerified)
	keyID := uuid.New()
	agent.CreatedByAPIKeyID = &keyID

	status, body := getAgentResponse(t, agent, &MockCapabilityServiceImpl{})

	require.Equal(t, fiber.StatusOK, status)
	assert.Equal(t, keyID.String(), body["createdByApiKeyId"])
}

// When the grant table cannot be read, GET /agents/:id fails instead of serving
// an empty capability list, which would say the agent holds no grant.
func TestGetAgentFailsWhenGrantsCannotBeRead(t *testing.T) {
	agent := newLifecycleAgent(domain.AgentStatusVerified)
	agent.Capabilities = []string{"file:read"}
	capabilities := &MockCapabilityServiceImpl{
		GetAgentCapabilitiesFunc: func(context.Context, uuid.UUID, bool) ([]*domain.AgentCapability, error) {
			return nil, errors.New("agent_capabilities: connection reset")
		},
	}

	status, body := getAgentResponse(t, agent, capabilities)

	assert.Equal(t, fiber.StatusInternalServerError, status)
	assert.Equal(t, map[string]any{"error": ServerErrorMessage}, body)
}

// The suspend, reactivate and revoke acts are applied before the response reads
// the grants, so a failed read does not fail them: the response reports the act
// and carries the agent without a capabilities field, never an empty list.
func TestAgentActResponsesLeaveCapabilitiesOutWhenGrantsCannotBeRead(t *testing.T) {
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
			agent.Capabilities = []string{"file:read"}
			capabilities := &MockCapabilityServiceImpl{
				GetAgentCapabilitiesFunc: func(context.Context, uuid.UUID, bool) ([]*domain.AgentCapability, error) {
					return nil, errors.New("agent_capabilities: connection reset")
				},
			}

			body := postAgentAct(t, agent, capabilities, tc.act)

			assert.Equal(t, true, body["success"])
			served, ok := body["agent"].(map[string]any)
			require.True(t, ok, "response carries the agent object: %v", body)
			assert.Equal(t, agent.ID.String(), served["id"])
			assert.NotContains(t, served, "capabilities")
		})
	}
}
