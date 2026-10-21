package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The bodies the Python SDK's register_mcp, report_execution_status and
// _register_single_mcp send live in sdk/python/tests/fixtures/sdk_requests, which a
// Python test also reads, so a member renamed on either side fails one of the two.

func readSDKRequestFixture(t *testing.T, name string) pqcRequestFixture {
	t.Helper()
	path := backendFile(t, filepath.Join("..", "..", "sdk", "python", "tests", "fixtures", "sdk_requests", name+".json"))
	raw, err := os.ReadFile(path) //nolint:gosec // G304: path derived from runtime.Caller, not user input
	require.NoError(t, err, "read the %s fixture", name)
	var fx pqcRequestFixture
	require.NoError(t, json.Unmarshal(raw, &fx))
	return fx
}

func TestAddMCPServersToAgentReadsTheSDKRequestBody(t *testing.T) {
	fx := readSDKRequestFixture(t, "register_mcp")
	var want application.AddMCPServersRequest
	requireBodyIsRequestType(t, fx, &want)
	assert.Equal(t, 87.5, want.Confidence)
	assert.Equal(t, "request-contract-fixture", want.Metadata["source"])

	orgID, agentID := uuid.New(), uuid.New()
	var gotIDs []string
	var gotMethod interface{}
	h := &AgentHandler{
		agentService: &MockAgentServiceImpl{
			GetAgentFunc: func(_ context.Context, id uuid.UUID) (*domain.Agent, error) {
				return &domain.Agent{ID: id, OrganizationID: orgID, Name: "mcp-agent"}, nil
			},
			AddMCPServersFunc: func(_ context.Context, id uuid.UUID, identifiers []string) (*domain.Agent, []string, error) {
				gotIDs = identifiers
				return &domain.Agent{ID: id, OrganizationID: orgID, TalksTo: identifiers}, identifiers, nil
			},
		},
		auditService: &MockAuditServiceImpl{
			LogActionFunc: func(_ context.Context, _, _ uuid.UUID, _ domain.AuditAction, _ string, _ uuid.UUID, _, _ string, metadata map[string]interface{}) error {
				gotMethod = metadata["detected_method"]
				return nil
			},
		},
	}

	status, out := postPQCFixture(t, fx, h.AddMCPServersToAgent, orgID, agentID, fx.Body)
	require.Equal(t, fiber.StatusOK, status, "response: %v", out)
	assert.Equal(t, want.MCPServerIDs, gotIDs, "the identifiers sent are the ones added")
	assert.Equal(t, want.DetectedMethod, gotMethod, "the detection method sent is the one audited")
}

func TestUpdateExecutionStatusReadsTheSDKRequestBody(t *testing.T) {
	fx := readSDKRequestFixture(t, "report_execution_status")
	var want UpdateExecutionStatusRequest
	requireBodyIsRequestType(t, fx, &want)
	require.True(t, want.Executed, "true is not what a member the handler does not read decodes to")
	require.NotNil(t, want.ExecutionError)

	agentID, orgID := uuid.New(), uuid.New()
	event := pendingEvent(agentID, orgID)
	var gotExecuted bool
	var gotAt time.Time
	var gotError *string
	svc := &MockVerificationEventServiceForVerificationImpl{
		GetVerificationEventFunc: func(_ context.Context, id uuid.UUID) (*domain.VerificationEvent, error) {
			if id != event.ID {
				return nil, fmt.Errorf("verification event not found")
			}
			return event, nil
		},
		UpdateExecutionStatusFunc: func(_ context.Context, _ uuid.UUID, executed bool, _ bool, executedAt time.Time, executionError *string) error {
			gotExecuted, gotAt, gotError = executed, executedAt, executionError
			return nil
		},
	}
	app := sdkWriteApp(newHandlerFor(svc, &MockOrganizationRepositoryerImpl{}), agentPrincipal("owning agent", agentID, orgID))

	req := httptest.NewRequest(fx.Method, "/api/v1/sdk-api/verifications/"+event.ID.String()+"/execution-status", bytes.NewReader(fx.Body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	assert.True(t, gotExecuted)
	assert.True(t, gotAt.Equal(time.Date(2026, 10, 21, 12, 0, 0, 250_000_000, time.UTC)), "executedAt read as %s", gotAt)
	require.NotNil(t, gotError)
	assert.Equal(t, *want.ExecutionError, *gotError)
}

// CreateMCPServer calls a concrete MCP service and agent repository, so the body is
// checked where the handler binds it: it decodes into CreateMCPServerRequest with unknown
// members refused, and carries the name and URL the handler refuses a body without.
func TestCreateMCPServerRequestBindsTheSDKRequestBody(t *testing.T) {
	fx := readSDKRequestFixture(t, "register_single_mcp")
	require.Equal(t, "CreateMCPServerRequest", fx.RequestType)

	dec := json.NewDecoder(bytes.NewReader(fx.Body))
	dec.DisallowUnknownFields()
	var got application.CreateMCPServerRequest
	require.NoError(t, dec.Decode(&got), "the fixture body sends a member CreateMCPServerRequest does not read")
	assert.NotEmpty(t, got.Name)
	assert.NotEmpty(t, got.URL)
	_, err := uuid.Parse(got.RegisteredByAgent)
	assert.NoError(t, err, "registeredByAgent is the agent ID")
}
