package handlers

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function for MCP tests that need org/user context
func withMCPOrgContext(handler func(c fiber.Ctx) error) fiber.Handler {
	return func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		c.Locals("user_id", uuid.New())
		return handler(c)
	}
}

// ===========================
// NewMCPHandler Tests
// ===========================

func TestNewMCPHandler_NilDeps(t *testing.T) {
	handler := NewMCPHandler(nil, nil, nil, nil, nil, nil, nil)
	assert.NotNil(t, handler)
}

// ===========================
// MCPHandler.CreateMCPServer Tests
// ===========================

func TestMCPHandler_CreateMCPServer_NoAuthContext(t *testing.T) {
	handler := &MCPHandler{}
	app := fiber.New()
	// Wrap with org context only, not user context
	app.Post("/mcp-servers", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		// No user_id or agent_id set
		return handler.CreateMCPServer(c)
	})

	body := `{"name":"test"}`
	req := httptest.NewRequest("POST", "/mcp-servers", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestMCPHandler_CreateMCPServer_InvalidJSON(t *testing.T) {
	handler := &MCPHandler{}
	app := fiber.New()
	app.Post("/mcp-servers", withMCPOrgContext(handler.CreateMCPServer))

	req := httptest.NewRequest("POST", "/mcp-servers", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// MCPHandler.GetMCPServer Tests
// ===========================

func TestMCPHandler_GetMCPServer_InvalidID(t *testing.T) {
	handler := &MCPHandler{}
	app := fiber.New()
	app.Get("/mcp-servers/:id", withMCPOrgContext(handler.GetMCPServer))

	req := httptest.NewRequest("GET", "/mcp-servers/not-a-uuid", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// MCPHandler.GetMCPServerByName Tests
// ===========================

func TestMCPHandler_GetMCPServerByName_MissingName(t *testing.T) {
	handler := &MCPHandler{}
	app := fiber.New()
	app.Get("/mcp-servers/by-name", withMCPOrgContext(handler.GetMCPServerByName))

	req := httptest.NewRequest("GET", "/mcp-servers/by-name", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// MCPHandler.UpdateMCPServer Tests
// ===========================

func TestMCPHandler_UpdateMCPServer_InvalidID(t *testing.T) {
	handler := &MCPHandler{}
	app := fiber.New()
	app.Put("/mcp-servers/:id", withMCPOrgContext(handler.UpdateMCPServer))

	body := `{"name":"updated"}`
	req := httptest.NewRequest("PUT", "/mcp-servers/not-a-uuid", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestMCPHandler_UpdateMCPServer_InvalidJSON(t *testing.T) {
	handler := &MCPHandler{}
	app := fiber.New()
	app.Put("/mcp-servers/:id", withMCPOrgContext(handler.UpdateMCPServer))

	req := httptest.NewRequest("PUT", "/mcp-servers/"+uuid.New().String(), strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// MCPHandler.DeleteMCPServer Tests
// ===========================

func TestMCPHandler_DeleteMCPServer_InvalidID(t *testing.T) {
	handler := &MCPHandler{}
	app := fiber.New()
	app.Delete("/mcp-servers/:id", withMCPOrgContext(handler.DeleteMCPServer))

	req := httptest.NewRequest("DELETE", "/mcp-servers/not-a-uuid", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// MCPHandler.VerifyMCPServer Tests
// ===========================

func TestMCPHandler_VerifyMCPServer_InvalidID(t *testing.T) {
	handler := &MCPHandler{}
	app := fiber.New()
	app.Post("/mcp-servers/:id/verify", withMCPOrgContext(handler.VerifyMCPServer))

	req := httptest.NewRequest("POST", "/mcp-servers/not-a-uuid/verify", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// MCPHandler.AddPublicKey Tests
// ===========================

func TestMCPHandler_AddPublicKey_InvalidID(t *testing.T) {
	handler := &MCPHandler{}
	app := fiber.New()
	app.Post("/mcp-servers/:id/public-key", withMCPOrgContext(handler.AddPublicKey))

	body := `{"publicKey":"test-key"}`
	req := httptest.NewRequest("POST", "/mcp-servers/not-a-uuid/public-key", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestMCPHandler_AddPublicKey_InvalidJSON(t *testing.T) {
	handler := &MCPHandler{}
	app := fiber.New()
	app.Post("/mcp-servers/:id/public-key", withMCPOrgContext(handler.AddPublicKey))

	req := httptest.NewRequest("POST", "/mcp-servers/"+uuid.New().String()+"/public-key", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// MCPHandler.GetVerificationStatus Tests
// ===========================

func TestMCPHandler_GetVerificationStatus_InvalidID(t *testing.T) {
	handler := &MCPHandler{}
	app := fiber.New()
	app.Get("/mcp-servers/:id/verification-status", withMCPOrgContext(handler.GetVerificationStatus))

	req := httptest.NewRequest("GET", "/mcp-servers/not-a-uuid/verification-status", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// MCPHandler.GetMCPServerCapabilities Tests
// ===========================

func TestMCPHandler_GetMCPServerCapabilities_InvalidID(t *testing.T) {
	handler := &MCPHandler{}
	app := fiber.New()
	app.Get("/mcp-servers/:id/capabilities", withMCPOrgContext(handler.GetMCPServerCapabilities))

	req := httptest.NewRequest("GET", "/mcp-servers/not-a-uuid/capabilities", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// MCPHandler.DetectCapabilities Tests
// ===========================

func TestMCPHandler_DetectCapabilities_InvalidID(t *testing.T) {
	handler := &MCPHandler{}
	app := fiber.New()
	app.Post("/mcp-servers/:id/detect-capabilities", withMCPOrgContext(handler.DetectCapabilities))

	req := httptest.NewRequest("POST", "/mcp-servers/not-a-uuid/detect-capabilities", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// MCPHandler.GetMCPServerAgents Tests
// ===========================

func TestMCPHandler_GetMCPServerAgents_InvalidID(t *testing.T) {
	handler := &MCPHandler{}
	app := fiber.New()
	app.Get("/mcp-servers/:id/agents", withMCPOrgContext(handler.GetMCPServerAgents))

	req := httptest.NewRequest("GET", "/mcp-servers/not-a-uuid/agents", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// MCPHandler.GetMCPVerificationEvents Tests
// ===========================

func TestMCPHandler_GetMCPVerificationEvents_InvalidID(t *testing.T) {
	handler := &MCPHandler{}
	app := fiber.New()
	app.Get("/mcp-servers/:id/verification-events", withMCPOrgContext(handler.GetMCPVerificationEvents))

	req := httptest.NewRequest("GET", "/mcp-servers/not-a-uuid/verification-events", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// MCPHandler.VerifyMCPCapability Tests
// ===========================

func TestMCPHandler_VerifyMCPCapability_InvalidID(t *testing.T) {
	handler := &MCPHandler{}
	app := fiber.New()
	app.Post("/mcp-servers/:id/verify-capability", withMCPOrgContext(handler.VerifyMCPCapability))

	body := `{"capability":"test"}`
	req := httptest.NewRequest("POST", "/mcp-servers/not-a-uuid/verify-capability", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestMCPHandler_VerifyMCPCapability_InvalidJSON(t *testing.T) {
	handler := &MCPHandler{}
	app := fiber.New()
	app.Post("/mcp-servers/:id/verify-capability", withMCPOrgContext(handler.VerifyMCPCapability))

	req := httptest.NewRequest("POST", "/mcp-servers/"+uuid.New().String()+"/verify-capability", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// MCPHandler.GetConnectedAgents Tests
// ===========================

func TestMCPHandler_GetConnectedAgents_InvalidID(t *testing.T) {
	handler := &MCPHandler{}
	app := fiber.New()
	app.Get("/mcp-servers/:id/connected-agents", withMCPOrgContext(handler.GetConnectedAgents))

	req := httptest.NewRequest("GET", "/mcp-servers/not-a-uuid/connected-agents", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// MCPHandler.GetMCPServerAuditLogs Tests
// ===========================

func TestMCPHandler_GetMCPServerAuditLogs_NoOrgContext(t *testing.T) {
	handler := &MCPHandler{}
	app := fiber.New()
	app.Get("/mcp-servers/:id/audit-logs", handler.GetMCPServerAuditLogs)

	req := httptest.NewRequest("GET", "/mcp-servers/"+uuid.New().String()+"/audit-logs", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestMCPHandler_GetMCPServerAuditLogs_InvalidID(t *testing.T) {
	handler := &MCPHandler{}
	app := fiber.New()
	app.Get("/mcp-servers/:id/audit-logs", withMCPOrgContext(handler.GetMCPServerAuditLogs))

	req := httptest.NewRequest("GET", "/mcp-servers/not-a-uuid/audit-logs", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// MCPHandler Helper Function Tests
// ===========================

func TestMCPHandler_buildAuditDescription(t *testing.T) {
	handler := &MCPHandler{}

	tests := []struct {
		name       string
		action     string
		serverName string
		metadata   map[string]interface{}
		contains   string
	}{
		{"create action", "create", "test-server", nil, "registered"},
		{"update action", "update", "test-server", nil, "updated"},
		{"delete action", "delete", "test-server", nil, "deleted"},
		{"verify action", "verify", "test-server", nil, "verification requested"},
		{"attest action", "attest", "test-server", nil, "Attestation submitted"},
		{"revoke with reason", "revoke", "test-server", map[string]interface{}{"reason": "security concern"}, "security concern"},
		{"revoke without reason", "revoke", "test-server", nil, "Attestation revoked"},
		{"default action", "unknown", "test-server", nil, "Action 'unknown' performed on"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.buildAuditDescription(tt.action, tt.serverName, tt.metadata)
			assert.Contains(t, result, tt.contains)
		})
	}
}

func TestMCPHandler_getTimestamp(t *testing.T) {
	handler := &MCPHandler{}
	now := time.Now()

	tests := []struct {
		name     string
		input    interface{}
		expected time.Time
	}{
		{"time.Time", now, now},
		{"*time.Time", &now, now},
		{"nil *time.Time", (*time.Time)(nil), time.Time{}},
		{"valid RFC3339 string", "2024-01-15T10:30:00Z", time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)},
		{"invalid string", "not-a-time", time.Time{}},
		{"nil", nil, time.Time{}},
		{"int (unsupported)", 12345, time.Time{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.getTimestamp(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ===========================
// MCPHandler Service Getter Tests
// ===========================

func TestMCPHandler_getAuditService_NilInterfaceReturnsNil(t *testing.T) {
	// When both auditServicer and auditService are nil, returns nil
	handler := &MCPHandler{}
	result := handler.getAuditService()
	assert.Nil(t, result)
}

func TestMCPHandler_getVerificationEventRepository_NilInterfaceReturnsNil(t *testing.T) {
	// When both verificationEventRepositoryer and verificationEventRepository are nil, returns nil
	handler := &MCPHandler{}
	result := handler.getVerificationEventRepository()
	assert.Nil(t, result)
}

func TestMCPHandler_getAttestationService_NilInterfaceReturnsNil(t *testing.T) {
	// When both attestationServicer and attestationService are nil, returns nil
	handler := &MCPHandler{}
	result := handler.getAttestationService()
	assert.Nil(t, result)
}

func TestMCPHandler_getMCPService_NilInterfaceReturnsNil(t *testing.T) {
	handler := &MCPHandler{}
	result := handler.getMCPService()
	assert.Nil(t, result)
}

func TestMCPHandler_getMCPCapabilityService_NilInterfaceReturnsNil(t *testing.T) {
	handler := &MCPHandler{}
	result := handler.getMCPCapabilityService()
	assert.Nil(t, result)
}

func TestMCPHandler_getAgentRepository_NilInterfaceReturnsNil(t *testing.T) {
	handler := &MCPHandler{}
	result := handler.getAgentRepository()
	assert.Nil(t, result)
}

func TestMCPHandler_getTagService_NilInterfaceReturnsNil(t *testing.T) {
	handler := &MCPHandler{}
	result := handler.getTagService()
	assert.Nil(t, result)
}


