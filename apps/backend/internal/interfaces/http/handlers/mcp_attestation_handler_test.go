package handlers

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// Helper function for MCP attestation tests that need org/user context
func withMCPAttestationContext(handler func(c fiber.Ctx) error) fiber.Handler {
	return func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		c.Locals("user_id", uuid.New())
		return handler(c)
	}
}

// ===========================
// NewMCPAttestationHandler Tests
// ===========================

func TestNewMCPAttestationHandler_NilDeps(t *testing.T) {
	handler := NewMCPAttestationHandler(nil, nil, nil, nil)
	assert.NotNil(t, handler)
}

// ===========================
// MCPAttestationHandler.GetAttestationChallenge Tests
// ===========================

func TestMCPAttestationHandler_GetAttestationChallenge_InvalidMCPID(t *testing.T) {
	handler := &MCPAttestationHandler{}
	app := fiber.New()
	app.Get("/mcp-servers/:id/challenge", handler.GetAttestationChallenge)

	req := httptest.NewRequest("GET", "/mcp-servers/not-a-uuid/challenge?agent_id="+uuid.New().String(), nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestMCPAttestationHandler_GetAttestationChallenge_MissingAgentID(t *testing.T) {
	handler := &MCPAttestationHandler{}
	app := fiber.New()
	app.Get("/mcp-servers/:id/challenge", handler.GetAttestationChallenge)

	req := httptest.NewRequest("GET", "/mcp-servers/"+uuid.New().String()+"/challenge", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestMCPAttestationHandler_GetAttestationChallenge_InvalidAgentID(t *testing.T) {
	handler := &MCPAttestationHandler{}
	app := fiber.New()
	app.Get("/mcp-servers/:id/challenge", handler.GetAttestationChallenge)

	req := httptest.NewRequest("GET", "/mcp-servers/"+uuid.New().String()+"/challenge?agent_id=not-a-uuid", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// MCPAttestationHandler.AttestMCP Tests
// ===========================

func TestMCPAttestationHandler_AttestMCP_InvalidMCPID(t *testing.T) {
	handler := &MCPAttestationHandler{}
	app := fiber.New()
	app.Post("/mcp-servers/:id/attest", handler.AttestMCP)

	body := `{"agentId":"test","challenge":"test","signature":"test"}`
	req := httptest.NewRequest("POST", "/mcp-servers/not-a-uuid/attest", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestMCPAttestationHandler_AttestMCP_InvalidJSON(t *testing.T) {
	handler := &MCPAttestationHandler{}
	app := fiber.New()
	app.Post("/mcp-servers/:id/attest", handler.AttestMCP)

	req := httptest.NewRequest("POST", "/mcp-servers/"+uuid.New().String()+"/attest", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// MCPAttestationHandler.GetMCPAttestations Tests
// ===========================

func TestMCPAttestationHandler_GetMCPAttestations_InvalidMCPID(t *testing.T) {
	handler := &MCPAttestationHandler{}
	app := fiber.New()
	app.Get("/mcp-servers/:id/attestations", handler.GetMCPAttestations)

	req := httptest.NewRequest("GET", "/mcp-servers/not-a-uuid/attestations", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// MCPAttestationHandler.GetConnectedAgents Tests
// ===========================

func TestMCPAttestationHandler_GetConnectedAgents_InvalidMCPID(t *testing.T) {
	handler := &MCPAttestationHandler{}
	app := fiber.New()
	app.Get("/mcp-servers/:id/connected-agents", handler.GetConnectedAgents)

	req := httptest.NewRequest("GET", "/mcp-servers/not-a-uuid/connected-agents", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// MCPAttestationHandler.GetAgentMCPServers Tests
// ===========================

func TestMCPAttestationHandler_GetAgentMCPServers_InvalidAgentID(t *testing.T) {
	handler := &MCPAttestationHandler{}
	app := fiber.New()
	app.Get("/agents/:agent_id/mcp-servers", handler.GetAgentMCPServers)

	req := httptest.NewRequest("GET", "/agents/not-a-uuid/mcp-servers", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// MCPAttestationHandler.ManualAttestMCP Tests
// ===========================

func TestMCPAttestationHandler_ManualAttestMCP_NoUserContext(t *testing.T) {
	handler := &MCPAttestationHandler{}
	app := fiber.New()
	app.Post("/mcp-servers/:id/manual-attest", handler.ManualAttestMCP)

	body := `{"notes":"test"}`
	req := httptest.NewRequest("POST", "/mcp-servers/"+uuid.New().String()+"/manual-attest", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestMCPAttestationHandler_ManualAttestMCP_NoOrgContext(t *testing.T) {
	handler := &MCPAttestationHandler{}
	app := fiber.New()
	app.Post("/mcp-servers/:id/manual-attest", func(c fiber.Ctx) error {
		c.Locals("user_id", uuid.New())
		// No organization_id set
		return handler.ManualAttestMCP(c)
	})

	body := `{"notes":"test"}`
	req := httptest.NewRequest("POST", "/mcp-servers/"+uuid.New().String()+"/manual-attest", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestMCPAttestationHandler_ManualAttestMCP_InvalidMCPID(t *testing.T) {
	handler := &MCPAttestationHandler{}
	app := fiber.New()
	app.Post("/mcp-servers/:id/manual-attest", withMCPAttestationContext(handler.ManualAttestMCP))

	body := `{"agentId":"` + uuid.New().String() + `"}`
	req := httptest.NewRequest("POST", "/mcp-servers/not-a-uuid/manual-attest", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestMCPAttestationHandler_ManualAttestMCP_InvalidJSON(t *testing.T) {
	handler := &MCPAttestationHandler{}
	app := fiber.New()
	app.Post("/mcp-servers/:id/manual-attest", withMCPAttestationContext(handler.ManualAttestMCP))

	req := httptest.NewRequest("POST", "/mcp-servers/"+uuid.New().String()+"/manual-attest", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// Note: ManualAttestMCP doesn't use agentId - it uses user_id from context.
// Tests for MissingAgentID and InvalidAgentID are not applicable for this endpoint.

// ===========================
// MCPAttestationHandler.RevokeAttestation Tests
// ===========================

func TestMCPAttestationHandler_RevokeAttestation_InvalidAttestationID(t *testing.T) {
	handler := &MCPAttestationHandler{}
	app := fiber.New()
	app.Delete("/mcp-servers/:mcp_id/attestations/:attestation_id", withMCPAttestationContext(handler.RevokeAttestation))

	req := httptest.NewRequest("DELETE", "/mcp-servers/"+uuid.New().String()+"/attestations/not-a-uuid", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestMCPAttestationHandler_RevokeAttestation_NoUserContext(t *testing.T) {
	handler := &MCPAttestationHandler{}
	app := fiber.New()
	app.Delete("/attestations/:attestation_id/revoke", handler.RevokeAttestation)

	body := `{"reason":"test reason"}`
	req := httptest.NewRequest("DELETE", "/attestations/"+uuid.New().String()+"/revoke", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestMCPAttestationHandler_RevokeAttestation_NoOrgContext(t *testing.T) {
	handler := &MCPAttestationHandler{}
	app := fiber.New()
	app.Delete("/attestations/:attestation_id/revoke", func(c fiber.Ctx) error {
		c.Locals("user_id", uuid.New())
		// No organization_id set
		return handler.RevokeAttestation(c)
	})

	body := `{"reason":"test reason"}`
	req := httptest.NewRequest("DELETE", "/attestations/"+uuid.New().String()+"/revoke", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestMCPAttestationHandler_RevokeAttestation_InvalidJSON(t *testing.T) {
	handler := &MCPAttestationHandler{}
	app := fiber.New()
	app.Delete("/attestations/:attestation_id/revoke", withMCPAttestationContext(handler.RevokeAttestation))

	req := httptest.NewRequest("DELETE", "/attestations/"+uuid.New().String()+"/revoke", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestMCPAttestationHandler_RevokeAttestation_MissingReason(t *testing.T) {
	handler := &MCPAttestationHandler{}
	app := fiber.New()
	app.Delete("/attestations/:attestation_id/revoke", withMCPAttestationContext(handler.RevokeAttestation))

	body := `{}`
	req := httptest.NewRequest("DELETE", "/attestations/"+uuid.New().String()+"/revoke", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// MCPAttestationHandler.RevokeAllAttestationsByAgent Tests
// ===========================

func TestMCPAttestationHandler_RevokeAllAttestationsByAgent_InvalidAgentID(t *testing.T) {
	handler := &MCPAttestationHandler{}
	app := fiber.New()
	app.Delete("/agents/:agent_id/attestations", withMCPAttestationContext(handler.RevokeAllAttestationsByAgent))

	req := httptest.NewRequest("DELETE", "/agents/not-a-uuid/attestations", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestMCPAttestationHandler_RevokeAllAttestationsByAgent_NoUserContext(t *testing.T) {
	handler := &MCPAttestationHandler{}
	app := fiber.New()
	app.Delete("/agents/:agent_id/attestations/revoke-all", handler.RevokeAllAttestationsByAgent)

	body := `{"reason":"test reason"}`
	req := httptest.NewRequest("DELETE", "/agents/"+uuid.New().String()+"/attestations/revoke-all", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestMCPAttestationHandler_RevokeAllAttestationsByAgent_NoOrgContext(t *testing.T) {
	handler := &MCPAttestationHandler{}
	app := fiber.New()
	app.Delete("/agents/:agent_id/attestations/revoke-all", func(c fiber.Ctx) error {
		c.Locals("user_id", uuid.New())
		// No organization_id set
		return handler.RevokeAllAttestationsByAgent(c)
	})

	body := `{"reason":"test reason"}`
	req := httptest.NewRequest("DELETE", "/agents/"+uuid.New().String()+"/attestations/revoke-all", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestMCPAttestationHandler_RevokeAllAttestationsByAgent_InvalidJSON(t *testing.T) {
	handler := &MCPAttestationHandler{}
	app := fiber.New()
	app.Delete("/agents/:agent_id/attestations/revoke-all", withMCPAttestationContext(handler.RevokeAllAttestationsByAgent))

	req := httptest.NewRequest("DELETE", "/agents/"+uuid.New().String()+"/attestations/revoke-all", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestMCPAttestationHandler_RevokeAllAttestationsByAgent_MissingReason(t *testing.T) {
	handler := &MCPAttestationHandler{}
	app := fiber.New()
	app.Delete("/agents/:agent_id/attestations/revoke-all", withMCPAttestationContext(handler.RevokeAllAttestationsByAgent))

	body := `{}`
	req := httptest.NewRequest("DELETE", "/agents/"+uuid.New().String()+"/attestations/revoke-all", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// MCPAttestationHandler.RecordMCPConnection Tests
// ===========================

func TestMCPAttestationHandler_RecordMCPConnection_InvalidJSON(t *testing.T) {
	handler := &MCPAttestationHandler{}
	app := fiber.New()
	app.Post("/mcp-servers/connection", handler.RecordMCPConnection)

	req := httptest.NewRequest("POST", "/mcp-servers/connection", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestMCPAttestationHandler_RecordMCPConnection_InvalidAgentID(t *testing.T) {
	handler := &MCPAttestationHandler{}
	app := fiber.New()
	app.Post("/agents/:agent_id/mcp-connections", handler.RecordMCPConnection)

	body := `{"mcpServerId":"` + uuid.New().String() + `","toolName":"test"}`
	req := httptest.NewRequest("POST", "/agents/not-a-uuid/mcp-connections", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestMCPAttestationHandler_RecordMCPConnection_MissingMCPServerID(t *testing.T) {
	handler := &MCPAttestationHandler{}
	app := fiber.New()
	app.Post("/agents/:agent_id/mcp-connections", handler.RecordMCPConnection)

	body := `{"toolName":"test"}`
	req := httptest.NewRequest("POST", "/agents/"+uuid.New().String()+"/mcp-connections", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestMCPAttestationHandler_RecordMCPConnection_MissingToolName(t *testing.T) {
	handler := &MCPAttestationHandler{}
	app := fiber.New()
	app.Post("/agents/:agent_id/mcp-connections", handler.RecordMCPConnection)

	body := `{"mcpServerId":"` + uuid.New().String() + `"}`
	req := httptest.NewRequest("POST", "/agents/"+uuid.New().String()+"/mcp-connections", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestMCPAttestationHandler_RecordMCPConnection_InvalidMCPServerIDFormat(t *testing.T) {
	handler := &MCPAttestationHandler{}
	app := fiber.New()
	app.Post("/agents/:agent_id/mcp-connections", handler.RecordMCPConnection)

	body := `{"mcpServerId":"not-a-uuid","toolName":"test"}`
	req := httptest.NewRequest("POST", "/agents/"+uuid.New().String()+"/mcp-connections", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// SECURITY (defect #19, body-class #2): RecordMCPConnection accepts a
// body-supplied mcpServerId. PR #185 closed the URL path's agent_id
// cross-org class but the body-supplied mcpServerId remained an IDOR:
// an attacker holding an SDK token for org A could record a fictitious
// connection between A's own agent and ANY tenant's MCP server by
// guessing the UUID, poisoning the victim org's connection-graph
// analytics. The fix wraps mcpServerId in LoadOwned against
// mcpServerRepo; the captured-flag test confirms the service is never
// reached (attestationService is nil; a bypass would 500-panic
// rather than returning the asserted 404).
func TestMCPAttestationHandler_RecordMCPConnection_CrossOrgMCPServerID_Returns404(t *testing.T) {
	callerOrgID := uuid.New()
	victimOrgID := uuid.New()
	agentID := uuid.New()
	mcpServerID := uuid.New()

	handler := &MCPAttestationHandler{
		agentRepo: &MockAgentRepositoryerImpl{
			GetByIDFunc: func(id uuid.UUID) (*domain.Agent, error) {
				return &domain.Agent{ID: agentID, OrganizationID: callerOrgID}, nil
			},
		},
		mcpServerRepo: &MockMCPServerRepositoryerImpl{
			GetByIDFunc: func(id uuid.UUID) (*domain.MCPServer, error) {
				return &domain.MCPServer{ID: mcpServerID, OrganizationID: victimOrgID}, nil
			},
		},
	}

	app := fiber.New()
	app.Post("/agents/:agent_id/mcp-connections", func(c fiber.Ctx) error {
		c.Locals("organization_id", callerOrgID)
		return handler.RecordMCPConnection(c)
	})

	body := `{"mcpServerId":"` + mcpServerID.String() + `","toolName":"x"}`
	req := httptest.NewRequest("POST", "/agents/"+agentID.String()+"/mcp-connections", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode, "cross-org body mcpServerId must return 404")
	bodyBytes, _ := io.ReadAll(resp.Body)
	bodyStr := string(bodyBytes)
	assert.NotContains(t, bodyStr, mcpServerID.String(), "response must not echo the supplied mcpServerId")
	assert.NotContains(t, bodyStr, victimOrgID.String(), "response must not echo the cross-org organization UUID")
}

// ===========================
// MCPAttestationHandler.RecordMCPUsageReport Tests
// ===========================

func TestMCPAttestationHandler_RecordMCPUsageReport_InvalidJSON(t *testing.T) {
	handler := &MCPAttestationHandler{}
	app := fiber.New()
	app.Post("/mcp-servers/usage", handler.RecordMCPUsageReport)

	req := httptest.NewRequest("POST", "/mcp-servers/usage", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestMCPAttestationHandler_RecordMCPUsageReport_InvalidAgentID(t *testing.T) {
	handler := &MCPAttestationHandler{}
	app := fiber.New()
	app.Post("/agents/:id/mcp-usage-report", handler.RecordMCPUsageReport)

	body := `{"agentId":"test","mcpServers":{},"reportedAt":"2024-01-01T00:00:00Z"}`
	req := httptest.NewRequest("POST", "/agents/not-a-uuid/mcp-usage-report", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// SECURITY (defect #19b, body-class #2): RecordMCPUsageReport accepts
// a body containing a map of mcpServerId → usage stats. Any cross-org
// UUID anywhere in the batch must abort the entire request with 404
// before any service call — without the pre-validation pass, an
// attacker could plant a fictitious usage report against a victim
// org's MCP, poisoning their invocation counters / capability rollups.
//
// Captured-flag pattern: attestationService is nil. A bypass that
// reached the service would surface as a panic / 500, observably
// distinct from the 404 the test asserts. The first cross-org UUID
// encountered during the pre-validation pass triggers the abort,
// regardless of where it appears in the map.
func TestMCPAttestationHandler_RecordMCPUsageReport_CrossOrgMCPServerID_Returns404(t *testing.T) {
	callerOrgID := uuid.New()
	victimOrgID := uuid.New()
	agentID := uuid.New()
	sameOrgMCP := uuid.New()
	crossOrgMCP := uuid.New()

	handler := &MCPAttestationHandler{
		agentRepo: &MockAgentRepositoryerImpl{
			GetByIDFunc: func(id uuid.UUID) (*domain.Agent, error) {
				return &domain.Agent{ID: agentID, OrganizationID: callerOrgID}, nil
			},
		},
		mcpServerRepo: &MockMCPServerRepositoryerImpl{
			GetByIDFunc: func(id uuid.UUID) (*domain.MCPServer, error) {
				if id == sameOrgMCP {
					return &domain.MCPServer{ID: id, OrganizationID: callerOrgID}, nil
				}
				return &domain.MCPServer{ID: id, OrganizationID: victimOrgID}, nil
			},
		},
	}

	app := fiber.New()
	app.Post("/agents/:id/mcp-usage-report", func(c fiber.Ctx) error {
		c.Locals("organization_id", callerOrgID)
		return handler.RecordMCPUsageReport(c)
	})

	// Batch with one same-org and one cross-org server. The whole
	// request must abort with 404 — partial-acceptance would still
	// poison the victim's analytics for the same-org entries while
	// the lying-by-omission about the cross-org entry hides the
	// probe.
	body := `{"agentId":"` + agentID.String() + `","mcpServers":{` +
		`"` + sameOrgMCP.String() + `":{"toolUsage":{},"capabilitiesAttested":[]},` +
		`"` + crossOrgMCP.String() + `":{"toolUsage":{},"capabilitiesAttested":[]}` +
		`},"reportedAt":"2024-01-01T00:00:00Z"}`
	req := httptest.NewRequest("POST", "/agents/"+agentID.String()+"/mcp-usage-report", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode, "any cross-org body mcpServerId must abort the whole batch with 404")
	bodyBytes, _ := io.ReadAll(resp.Body)
	bodyStr := string(bodyBytes)
	assert.NotContains(t, bodyStr, crossOrgMCP.String(), "response must not echo the supplied cross-org mcpServerId")
	assert.NotContains(t, bodyStr, victimOrgID.String(), "response must not echo the cross-org organization UUID")
}

// ===========================
// MCPAttestationHandler.GetConsensusStatus Tests
// ===========================

func TestMCPAttestationHandler_GetConsensusStatus_InvalidMCPID(t *testing.T) {
	handler := &MCPAttestationHandler{}
	app := fiber.New()
	app.Get("/mcp-servers/:id/consensus", handler.GetConsensusStatus)

	req := httptest.NewRequest("GET", "/mcp-servers/not-a-uuid/consensus", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// A3d-vi: cross-tenant access on all 8 path-id MCPAttestationHandler
// routes must return 404 with existence-secrecy body. The LoadOwned
// guards wired in this PR short-circuit BEFORE any service dispatch;
// attestationService is nil, so any bypass that reached the service
// would panic and fiber would emit 500 — observably distinct from the
// 404 we assert. Mirrors A3d-i (TagHandler) and PR #150's panic-proof
// pattern. Never use t.Fatalf inside fiber app.Test mock goroutines:
// app.Test runs the handler on a separate goroutine and runtime.Goexit
// from a non-test goroutine does not fail the test (memory:
// feedback_fiber_app_test_goroutine_t_fatalf_race).
// ===========================

// stubMCPServerByIDRepo satisfies mcpServerByIDLookup for cross-tenant
// tests. The lookup returns an MCPServer owned by a foreign org so that
// LoadOwned in the handler short-circuits to 404.
type stubMCPServerByIDRepo struct {
	getByID func(id uuid.UUID) (*domain.MCPServer, error)
}

func (s *stubMCPServerByIDRepo) GetByID(id uuid.UUID) (*domain.MCPServer, error) {
	return s.getByID(id)
}

func TestMCPAttestationHandler_CrossOrgReturns404(t *testing.T) {
	callerOrgID := uuid.New()
	callerUserID := uuid.New()
	differentOrgID := uuid.New()
	pathID := uuid.New()

	mcpServerRepo := &stubMCPServerByIDRepo{
		getByID: func(id uuid.UUID) (*domain.MCPServer, error) {
			return &domain.MCPServer{
				ID:             id,
				OrganizationID: differentOrgID,
				Name:           "victim-mcp",
			}, nil
		},
	}
	agentRepo := &MockAgentRepositoryerImpl{
		GetByIDFunc: func(id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{
				ID:             id,
				OrganizationID: differentOrgID,
				Name:           "victim-agent",
			}, nil
		},
	}

	// attestationService and auditService are intentionally nil — the
	// LoadOwned gates must short-circuit before any service dispatch.
	// Reaching a nil service would panic (which fiber maps to 500 — a
	// distinct status from the 404 we assert).
	handler := &MCPAttestationHandler{
		attestationService: nil,
		auditService:       nil,
		agentRepo:          agentRepo,
		mcpServerRepo:      mcpServerRepo,
	}

	setLocals := func(c fiber.Ctx) {
		c.Locals("organization_id", callerOrgID)
		c.Locals("user_id", callerUserID)
	}

	cases := []struct {
		name        string
		method      string
		mount       func(app *fiber.App)
		requestPath string
		body        string
	}{
		{
			name:   "GetAttestationChallenge_CrossOrgMCP",
			method: "GET",
			mount: func(app *fiber.App) {
				app.Get("/mcp-servers/:id/challenge", func(c fiber.Ctx) error {
					setLocals(c)
					return handler.GetAttestationChallenge(c)
				})
			},
			requestPath: "/mcp-servers/" + pathID.String() + "/challenge?agent_id=" + uuid.New().String(),
		},
		{
			name:   "AttestMCP_CrossOrgMCP",
			method: "POST",
			mount: func(app *fiber.App) {
				app.Post("/mcp-servers/:id/attest", func(c fiber.Ctx) error {
					setLocals(c)
					return handler.AttestMCP(c)
				})
			},
			requestPath: "/mcp-servers/" + pathID.String() + "/attest",
			body:        `{"attestation":{"agentId":"` + uuid.New().String() + `","challenge":"x","capabilitiesFound":[]},"signature":"x"}`,
		},
		{
			name:   "GetMCPAttestations_CrossOrgMCP",
			method: "GET",
			mount: func(app *fiber.App) {
				app.Get("/mcp-servers/:id/attestations", func(c fiber.Ctx) error {
					setLocals(c)
					return handler.GetMCPAttestations(c)
				})
			},
			requestPath: "/mcp-servers/" + pathID.String() + "/attestations",
		},
		{
			name:   "GetConnectedAgents_CrossOrgMCP",
			method: "GET",
			mount: func(app *fiber.App) {
				app.Get("/mcp-servers/:id/connected-agents", func(c fiber.Ctx) error {
					setLocals(c)
					return handler.GetConnectedAgents(c)
				})
			},
			requestPath: "/mcp-servers/" + pathID.String() + "/connected-agents",
		},
		{
			name:   "GetAgentMCPServers_CrossOrgAgent",
			method: "GET",
			mount: func(app *fiber.App) {
				app.Get("/agents/:id/mcp-servers", func(c fiber.Ctx) error {
					setLocals(c)
					return handler.GetAgentMCPServers(c)
				})
			},
			requestPath: "/agents/" + pathID.String() + "/mcp-servers",
		},
		{
			name:   "ManualAttestMCP_CrossOrgMCP",
			method: "POST",
			mount: func(app *fiber.App) {
				app.Post("/mcp-servers/:id/manual-attest", func(c fiber.Ctx) error {
					setLocals(c)
					return handler.ManualAttestMCP(c)
				})
			},
			requestPath: "/mcp-servers/" + pathID.String() + "/manual-attest",
			body:        `{"notes":"x"}`,
		},
		{
			name:   "RevokeAllAttestationsByAgent_CrossOrgAgent",
			method: "POST",
			mount: func(app *fiber.App) {
				app.Post("/agents/:agent_id/attestations/revoke-all", func(c fiber.Ctx) error {
					setLocals(c)
					return handler.RevokeAllAttestationsByAgent(c)
				})
			},
			requestPath: "/agents/" + pathID.String() + "/attestations/revoke-all",
			body:        `{"reason":"x"}`,
		},
		{
			name:   "GetConsensusStatus_CrossOrgMCP",
			method: "GET",
			mount: func(app *fiber.App) {
				app.Get("/mcp-servers/:id/consensus-status", func(c fiber.Ctx) error {
					setLocals(c)
					return handler.GetConsensusStatus(c)
				})
			},
			requestPath: "/mcp-servers/" + pathID.String() + "/consensus-status",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := fiber.New()
			tc.mount(app)

			r := httptest.NewRequest(tc.method, tc.requestPath, strings.NewReader(tc.body))
			if tc.body != "" {
				r.Header.Set("Content-Type", "application/json")
			}

			resp, err := app.Test(r)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, fiber.StatusNotFound, resp.StatusCode,
				"cross-org request must return 404 (existence-secrecy); 500 would mean LoadOwned was bypassed and the nil service was reached")
			body, _ := io.ReadAll(resp.Body)
			assert.JSONEq(t, `{"error":"not found"}`, string(body),
				"cross-org body must be the standard not-found shape; any other body means the response was rewritten downstream")
		})
	}
}

// TestMCPAttestationHandler_GetAttestationChallenge_CrossOrgQueryAgent
// closes class-#1 lint blindspot (c.Query() tenant UUID): the path MCP
// is in the caller's org but the query agent_id points to a victim org.
// The handler must short-circuit on the second LoadOwned to 404.
func TestMCPAttestationHandler_GetAttestationChallenge_CrossOrgQueryAgent(t *testing.T) {
	callerOrgID := uuid.New()
	callerUserID := uuid.New()
	differentOrgID := uuid.New()
	mcpID := uuid.New()
	queryAgentID := uuid.New()

	// Path MCP is owned by caller (passes the first LoadOwned).
	mcpServerRepo := &stubMCPServerByIDRepo{
		getByID: func(id uuid.UUID) (*domain.MCPServer, error) {
			return &domain.MCPServer{ID: id, OrganizationID: callerOrgID, Name: "own-mcp"}, nil
		},
	}
	// Query agent is owned by a victim org (must fail the second LoadOwned).
	agentRepo := &MockAgentRepositoryerImpl{
		GetByIDFunc: func(id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{ID: id, OrganizationID: differentOrgID, Name: "victim-agent"}, nil
		},
	}
	handler := &MCPAttestationHandler{
		attestationService: nil,
		auditService:       nil,
		agentRepo:          agentRepo,
		mcpServerRepo:      mcpServerRepo,
	}

	app := fiber.New()
	app.Get("/mcp-servers/:id/challenge", func(c fiber.Ctx) error {
		c.Locals("organization_id", callerOrgID)
		c.Locals("user_id", callerUserID)
		return handler.GetAttestationChallenge(c)
	})

	r := httptest.NewRequest("GET",
		"/mcp-servers/"+mcpID.String()+"/challenge?agent_id="+queryAgentID.String(), nil)
	resp, err := app.Test(r)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode,
		"cross-org query-supplied agent_id must yield 404")
	body, _ := io.ReadAll(resp.Body)
	assert.JSONEq(t, `{"error":"not found"}`, string(body))
}
