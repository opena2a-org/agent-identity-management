package handlers

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	handler := NewMCPAttestationHandler(nil, nil)
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
