package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record/transition"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Adding MCP servers to an agent's talks_to list widens what the agent may reach, an
// expansion: it is made only together with its audit record, and when the record cannot
// be written the service refuses it whole with an error that wraps
// transition.ErrRecordUnavailable. The route answers that refusal with 503, its reason and
// Retry-After, as the agent delete does.

// talksToAddRecordRefusal is the error AgentService.AddMCPServers returns for an addition
// whose record write failed: the recorder's refusal under the service's own prefix, with
// a cause the answer must not carry.
var talksToAddRecordRefusal = fmt.Errorf("failed to update agent: %w", fmt.Errorf("%w (%s): %w",
	transition.ErrRecordUnavailable, transition.TriggerTalksToAdded,
	errors.New(`pq: canceling statement due to lock timeout on "record_chains"`)))

// putRefusedTalksToAdd adds two MCP servers to an agent, with the service answering
// addErr, and returns the response and how many audit rows the route wrote.
func putRefusedTalksToAdd(t *testing.T, addErr error) (int, string, map[string]any, int) {
	t.Helper()
	agent := &domain.Agent{ID: uuid.New(), OrganizationID: uuid.New(), Name: "talks-to-agent"}
	audited := 0
	handler := NewAgentHandlerWithInterfaces(
		&MockAgentServiceImpl{
			GetAgentFunc: func(context.Context, uuid.UUID) (*domain.Agent, error) {
				copied := *agent
				return &copied, nil
			},
			AddMCPServersFunc: func(context.Context, uuid.UUID, []string) (*domain.Agent, []string, error) {
				return nil, nil, addErr
			},
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
	app.Put("/agents/:id/mcp-servers", handler.AddMCPServersToAgent)

	req := httptest.NewRequest("PUT", "/agents/"+agent.ID.String()+"/mcp-servers",
		strings.NewReader(`{"mcpServerIds":["memory","github"]}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	return resp.StatusCode, resp.Header.Get("Retry-After"), body, audited
}

func TestTalksToAddRefusedByTheRecordPathAnswers503WithReasonAndRetryAfter(t *testing.T) {
	status, retryAfter, body, audited := putRefusedTalksToAdd(t, talksToAddRecordRefusal)

	assert.Equal(t, fiber.StatusServiceUnavailable, status)
	assert.Equal(t, "300", retryAfter, "the record path window is 300 s today")
	assert.Equal(t, map[string]any{
		"error":      RecordUnavailableMessage,
		"reasonCode": "recordUnavailable",
		"code":       "recordUnavailable",
	}, body)
	assert.NotContains(t, fmt.Sprint(body), "lock timeout")
	assert.Zero(t, audited, "a refused addition writes no audit row of the addition")
}

// Any other failure of the addition keeps the generic server error line and sends no
// Retry-After: only the record path's refusal promises a time to try again.
func TestTalksToAddOtherFailureKeepsTheGenericLineWithoutRetryAfter(t *testing.T) {
	status, retryAfter, body, audited := putRefusedTalksToAdd(t, repositoryFailure)

	assert.Equal(t, fiber.StatusInternalServerError, status)
	assert.Empty(t, retryAfter)
	assert.Equal(t, map[string]any{"error": ServerErrorMessage}, body)
	assert.Zero(t, audited)
}
