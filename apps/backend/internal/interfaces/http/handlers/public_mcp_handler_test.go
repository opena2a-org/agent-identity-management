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

// ===========================
// NewPublicMCPHandler Tests
// ===========================

func TestNewPublicMCPHandler_NilDeps(t *testing.T) {
	handler := NewPublicMCPHandler(nil, nil, nil)
	assert.NotNil(t, handler)
}

// ===========================
// PublicMCPHandler.RegisterMCPServer Tests
// ===========================

func TestPublicMCPHandler_RegisterMCPServer_InvalidJSON(t *testing.T) {
	handler := &PublicMCPHandler{}
	app := fiber.New()
	app.Post("/public/mcp-servers/register", handler.RegisterMCPServer)

	req := httptest.NewRequest("POST", "/public/mcp-servers/register", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestPublicMCPHandler_RegisterMCPServer_MissingAgentID(t *testing.T) {
	handler := &PublicMCPHandler{}
	app := fiber.New()
	app.Post("/public/mcp-servers/register", handler.RegisterMCPServer)

	body := `{"serverName":"test","serverUrl":"http://test.com","publicKey":"abc","capabilities":["read"],"signature":"sig"}`
	req := httptest.NewRequest("POST", "/public/mcp-servers/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestPublicMCPHandler_RegisterMCPServer_MissingServerName(t *testing.T) {
	handler := &PublicMCPHandler{}
	app := fiber.New()
	app.Post("/public/mcp-servers/register", handler.RegisterMCPServer)

	body := `{"agentId":"` + uuid.New().String() + `","serverUrl":"http://test.com","publicKey":"abc","capabilities":["read"],"signature":"sig"}`
	req := httptest.NewRequest("POST", "/public/mcp-servers/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestPublicMCPHandler_RegisterMCPServer_MissingServerURL(t *testing.T) {
	handler := &PublicMCPHandler{}
	app := fiber.New()
	app.Post("/public/mcp-servers/register", handler.RegisterMCPServer)

	body := `{"agentId":"` + uuid.New().String() + `","serverName":"test","publicKey":"abc","capabilities":["read"],"signature":"sig"}`
	req := httptest.NewRequest("POST", "/public/mcp-servers/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestPublicMCPHandler_RegisterMCPServer_MissingPublicKey(t *testing.T) {
	handler := &PublicMCPHandler{}
	app := fiber.New()
	app.Post("/public/mcp-servers/register", handler.RegisterMCPServer)

	body := `{"agentId":"` + uuid.New().String() + `","serverName":"test","serverUrl":"http://test.com","capabilities":["read"],"signature":"sig"}`
	req := httptest.NewRequest("POST", "/public/mcp-servers/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestPublicMCPHandler_RegisterMCPServer_EmptyCapabilities(t *testing.T) {
	handler := &PublicMCPHandler{}
	app := fiber.New()
	app.Post("/public/mcp-servers/register", handler.RegisterMCPServer)

	body := `{"agentId":"` + uuid.New().String() + `","serverName":"test","serverUrl":"http://test.com","publicKey":"abc","capabilities":[],"signature":"sig"}`
	req := httptest.NewRequest("POST", "/public/mcp-servers/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestPublicMCPHandler_RegisterMCPServer_MissingSignature(t *testing.T) {
	handler := &PublicMCPHandler{}
	app := fiber.New()
	app.Post("/public/mcp-servers/register", handler.RegisterMCPServer)

	body := `{"agentId":"` + uuid.New().String() + `","serverName":"test","serverUrl":"http://test.com","publicKey":"abc","capabilities":["read"]}`
	req := httptest.NewRequest("POST", "/public/mcp-servers/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestPublicMCPHandler_RegisterMCPServer_InvalidAgentID(t *testing.T) {
	handler := &PublicMCPHandler{}
	app := fiber.New()
	app.Post("/public/mcp-servers/register", handler.RegisterMCPServer)

	body := `{"agentId":"not-a-uuid","serverName":"test","serverUrl":"http://test.com","publicKey":"abc","capabilities":["read"],"signature":"sig"}`
	req := httptest.NewRequest("POST", "/public/mcp-servers/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// PublicMCPHandler.ListMCPServersForAgent Tests
// ===========================

func TestPublicMCPHandler_ListMCPServersForAgent_InvalidAgentID(t *testing.T) {
	handler := &PublicMCPHandler{}
	app := fiber.New()
	app.Get("/public/mcp-servers/agent/:id", handler.ListMCPServersForAgent)

	req := httptest.NewRequest("GET", "/public/mcp-servers/agent/not-a-uuid?signature=test&timestamp=123", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestPublicMCPHandler_ListMCPServersForAgent_MissingSignature(t *testing.T) {
	handler := &PublicMCPHandler{}
	app := fiber.New()
	app.Get("/public/mcp-servers/agent/:id", handler.ListMCPServersForAgent)

	req := httptest.NewRequest("GET", "/public/mcp-servers/agent/"+uuid.New().String()+"?timestamp=123", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestPublicMCPHandler_ListMCPServersForAgent_MissingTimestamp(t *testing.T) {
	handler := &PublicMCPHandler{}
	app := fiber.New()
	app.Get("/public/mcp-servers/agent/:id", handler.ListMCPServersForAgent)

	req := httptest.NewRequest("GET", "/public/mcp-servers/agent/"+uuid.New().String()+"?signature=test", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// PublicMCPHandler.VerifyMCPAction Tests
// ===========================

func TestPublicMCPHandler_VerifyMCPAction_InvalidServerID(t *testing.T) {
	handler := &PublicMCPHandler{}
	app := fiber.New()
	app.Post("/public/mcp-servers/:id/verify", handler.VerifyMCPAction)

	body := `{"agentId":"` + uuid.New().String() + `","actionType":"read","signature":"test"}`
	req := httptest.NewRequest("POST", "/public/mcp-servers/not-a-uuid/verify", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestPublicMCPHandler_VerifyMCPAction_InvalidJSON(t *testing.T) {
	handler := &PublicMCPHandler{}
	app := fiber.New()
	app.Post("/public/mcp-servers/:id/verify", handler.VerifyMCPAction)

	req := httptest.NewRequest("POST", "/public/mcp-servers/"+uuid.New().String()+"/verify", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestPublicMCPHandler_VerifyMCPAction_MissingRequiredFields(t *testing.T) {
	handler := &PublicMCPHandler{}
	app := fiber.New()
	app.Post("/public/mcp-servers/:id/verify", handler.VerifyMCPAction)

	// Missing agentId, actionType, and signature
	body := `{}`
	req := httptest.NewRequest("POST", "/public/mcp-servers/"+uuid.New().String()+"/verify", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestPublicMCPHandler_VerifyMCPAction_InvalidAgentID(t *testing.T) {
	handler := &PublicMCPHandler{}
	app := fiber.New()
	app.Post("/public/mcp-servers/:id/verify", handler.VerifyMCPAction)

	body := `{"agentId":"not-a-uuid","actionType":"read","signature":"test"}`
	req := httptest.NewRequest("POST", "/public/mcp-servers/"+uuid.New().String()+"/verify", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// Struct Tests
// ===========================

func TestRegisterMCPServerRequest_Struct(t *testing.T) {
	req := RegisterMCPServerRequest{
		AgentID:      "agent-uuid-123",
		ServerName:   "test-mcp-server",
		ServerURL:    "https://mcp.example.com",
		PublicKey:    "base64-public-key",
		Capabilities: []string{"read", "write", "execute"},
		Description:  "A test MCP server",
		Version:      "1.0.0",
		Timestamp:    1704067200,
		Signature:    "base64-signature",
	}

	assert.Equal(t, "agent-uuid-123", req.AgentID)
	assert.Equal(t, "test-mcp-server", req.ServerName)
	assert.Equal(t, "https://mcp.example.com", req.ServerURL)
	assert.Equal(t, "base64-public-key", req.PublicKey)
	assert.Len(t, req.Capabilities, 3)
	assert.Contains(t, req.Capabilities, "read")
	assert.Contains(t, req.Capabilities, "write")
	assert.Contains(t, req.Capabilities, "execute")
	assert.Equal(t, "A test MCP server", req.Description)
	assert.Equal(t, "1.0.0", req.Version)
	assert.Equal(t, int64(1704067200), req.Timestamp)
	assert.Equal(t, "base64-signature", req.Signature)
}

func TestRegisterMCPServerRequest_EmptyStruct(t *testing.T) {
	req := RegisterMCPServerRequest{}

	assert.Empty(t, req.AgentID)
	assert.Empty(t, req.ServerName)
	assert.Empty(t, req.ServerURL)
	assert.Empty(t, req.PublicKey)
	assert.Nil(t, req.Capabilities)
	assert.Empty(t, req.Description)
	assert.Empty(t, req.Version)
	assert.Equal(t, int64(0), req.Timestamp)
	assert.Empty(t, req.Signature)
}

func TestVerifyMCPActionRequest_Struct(t *testing.T) {
	req := VerifyMCPActionRequest{
		AgentID:    "agent-uuid-456",
		ActionType: "file:read",
		Resource:   "/path/to/resource",
		Context:    map[string]interface{}{"key": "value"},
		RiskLevel:  "low",
		Timestamp:  1704067200,
		Signature:  "base64-signature",
	}

	assert.Equal(t, "agent-uuid-456", req.AgentID)
	assert.Equal(t, "file:read", req.ActionType)
	assert.Equal(t, "/path/to/resource", req.Resource)
	assert.NotNil(t, req.Context)
	assert.Equal(t, "low", req.RiskLevel)
	assert.Equal(t, int64(1704067200), req.Timestamp)
	assert.Equal(t, "base64-signature", req.Signature)
}

func TestVerifyMCPActionRequest_EmptyStruct(t *testing.T) {
	req := VerifyMCPActionRequest{}

	assert.Empty(t, req.AgentID)
	assert.Empty(t, req.ActionType)
	assert.Empty(t, req.Resource)
	assert.Nil(t, req.Context)
	assert.Empty(t, req.RiskLevel)
	assert.Equal(t, int64(0), req.Timestamp)
	assert.Empty(t, req.Signature)
}
