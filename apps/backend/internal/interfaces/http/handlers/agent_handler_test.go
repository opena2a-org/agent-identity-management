package handlers

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testErrorHandler is a custom error handler that preserves already-sent responses
// and only returns 500 for truly unexpected errors
func testErrorHandler(c fiber.Ctx, err error) error {
	// If response is already committed or error is ErrUnauthorized, don't override
	if errors.Is(err, ErrUnauthorized) {
		return nil
	}
	// For other errors, return 500
	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
		"error": err.Error(),
	})
}

// newTestApp creates a Fiber app with the test error handler
func newTestApp() *fiber.App {
	return fiber.New(fiber.Config{
		ErrorHandler: testErrorHandler,
	})
}

// ===========================
// AgentHandler.ListAgents Tests
// ===========================

func TestAgentHandler_ListAgents_NoOrgContext(t *testing.T) {
	handler := &AgentHandler{}
	app := newTestApp()
	app.Get("/agents", handler.ListAgents)

	req := httptest.NewRequest("GET", "/agents", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

// ===========================
// AgentHandler.CreateAgent Tests
// ===========================

func TestAgentHandler_CreateAgent_NoOrgContext(t *testing.T) {
	handler := &AgentHandler{}
	app := newTestApp()
	app.Post("/agents", handler.CreateAgent)

	body := `{"name":"test-agent","agentType":"claude"}`
	req := httptest.NewRequest("POST", "/agents", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAgentHandler_CreateAgent_NoUserContext(t *testing.T) {
	handler := &AgentHandler{}
	app := newTestApp()
	app.Post("/agents", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		return handler.CreateAgent(c)
	})

	body := `{"name":"test-agent","agentType":"claude"}`
	req := httptest.NewRequest("POST", "/agents", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAgentHandler_CreateAgent_InvalidJSON(t *testing.T) {
	handler := &AgentHandler{}
	app := fiber.New()
	app.Post("/agents", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		c.Locals("user_id", uuid.New())
		return handler.CreateAgent(c)
	})

	req := httptest.NewRequest("POST", "/agents", strings.NewReader("not valid json"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// AgentHandler.GetAgent Tests
// ===========================

func TestAgentHandler_GetAgent_NoOrgContext(t *testing.T) {
	handler := &AgentHandler{}
	app := newTestApp()
	app.Get("/agents/:id", handler.GetAgent)

	req := httptest.NewRequest("GET", "/agents/"+uuid.New().String(), nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAgentHandler_GetAgent_InvalidID(t *testing.T) {
	handler := &AgentHandler{}
	app := fiber.New()
	app.Get("/agents/:id", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		return handler.GetAgent(c)
	})

	req := httptest.NewRequest("GET", "/agents/not-a-uuid", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// AgentHandler.UpdateAgent Tests
// ===========================

func TestAgentHandler_UpdateAgent_NoOrgContext(t *testing.T) {
	handler := &AgentHandler{}
	app := newTestApp()
	app.Put("/agents/:id", handler.UpdateAgent)

	body := `{"name":"updated-agent"}`
	req := httptest.NewRequest("PUT", "/agents/"+uuid.New().String(), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAgentHandler_UpdateAgent_NoUserContext(t *testing.T) {
	handler := &AgentHandler{}
	app := newTestApp()
	app.Put("/agents/:id", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		return handler.UpdateAgent(c)
	})

	body := `{"name":"updated-agent"}`
	req := httptest.NewRequest("PUT", "/agents/"+uuid.New().String(), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAgentHandler_UpdateAgent_InvalidID(t *testing.T) {
	handler := &AgentHandler{}
	app := fiber.New()
	app.Put("/agents/:id", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		c.Locals("user_id", uuid.New())
		return handler.UpdateAgent(c)
	})

	body := `{"name":"updated-agent"}`
	req := httptest.NewRequest("PUT", "/agents/not-a-uuid", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestAgentHandler_UpdateAgent_InvalidJSON(t *testing.T) {
	handler := &AgentHandler{}
	app := fiber.New()
	app.Put("/agents/:id", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		c.Locals("user_id", uuid.New())
		return handler.UpdateAgent(c)
	})

	req := httptest.NewRequest("PUT", "/agents/"+uuid.New().String(), strings.NewReader("not valid json"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// AgentHandler.DeleteAgent Tests
// ===========================

func TestAgentHandler_DeleteAgent_NoOrgContext(t *testing.T) {
	handler := &AgentHandler{}
	app := newTestApp()
	app.Delete("/agents/:id", handler.DeleteAgent)

	req := httptest.NewRequest("DELETE", "/agents/"+uuid.New().String(), nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAgentHandler_DeleteAgent_NoUserContext(t *testing.T) {
	handler := &AgentHandler{}
	app := newTestApp()
	app.Delete("/agents/:id", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		return handler.DeleteAgent(c)
	})

	req := httptest.NewRequest("DELETE", "/agents/"+uuid.New().String(), nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAgentHandler_DeleteAgent_InvalidID(t *testing.T) {
	handler := &AgentHandler{}
	app := fiber.New()
	app.Delete("/agents/:id", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		c.Locals("user_id", uuid.New())
		return handler.DeleteAgent(c)
	})

	req := httptest.NewRequest("DELETE", "/agents/not-a-uuid", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// AgentHandler.GetAgentAlerts Tests
// ===========================

func TestAgentHandler_GetAgentAlerts_NoOrgContext(t *testing.T) {
	handler := &AgentHandler{}
	app := newTestApp()
	app.Get("/agents/:id/alerts", handler.GetAgentAlerts)

	req := httptest.NewRequest("GET", "/agents/"+uuid.New().String()+"/alerts", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAgentHandler_GetAgentAlerts_InvalidID(t *testing.T) {
	handler := &AgentHandler{}
	app := fiber.New()
	app.Get("/agents/:id/alerts", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		return handler.GetAgentAlerts(c)
	})

	req := httptest.NewRequest("GET", "/agents/not-a-uuid/alerts", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// AgentHandler.GetAgentMCPServers Tests
// ===========================

func TestAgentHandler_GetAgentMCPServers_NoOrgContext(t *testing.T) {
	handler := &AgentHandler{}
	app := newTestApp()
	app.Get("/agents/:id/mcp-servers", handler.GetAgentMCPServers)

	req := httptest.NewRequest("GET", "/agents/"+uuid.New().String()+"/mcp-servers", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAgentHandler_GetAgentMCPServers_InvalidID(t *testing.T) {
	handler := &AgentHandler{}
	app := fiber.New()
	app.Get("/agents/:id/mcp-servers", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		return handler.GetAgentMCPServers(c)
	})

	req := httptest.NewRequest("GET", "/agents/not-a-uuid/mcp-servers", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// AgentHandler.GetAgentActivity Tests
// ===========================

func TestAgentHandler_GetAgentActivity_NoOrgContext(t *testing.T) {
	handler := &AgentHandler{}
	app := newTestApp()
	app.Get("/agents/:id/activity", handler.GetAgentActivity)

	req := httptest.NewRequest("GET", "/agents/"+uuid.New().String()+"/activity", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAgentHandler_GetAgentActivity_InvalidID(t *testing.T) {
	handler := &AgentHandler{}
	app := fiber.New()
	app.Get("/agents/:id/activity", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		return handler.GetAgentActivity(c)
	})

	req := httptest.NewRequest("GET", "/agents/not-a-uuid/activity", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// AgentHandler.RotateCredentials Tests
// ===========================

func TestAgentHandler_RotateCredentials_NoOrgContext(t *testing.T) {
	handler := &AgentHandler{}
	app := newTestApp()
	app.Post("/agents/:id/rotate-credentials", handler.RotateCredentials)

	req := httptest.NewRequest("POST", "/agents/"+uuid.New().String()+"/rotate-credentials", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAgentHandler_RotateCredentials_InvalidID(t *testing.T) {
	handler := &AgentHandler{}
	app := fiber.New()
	app.Post("/agents/:id/rotate-credentials", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		c.Locals("user_id", uuid.New())
		return handler.RotateCredentials(c)
	})

	req := httptest.NewRequest("POST", "/agents/not-a-uuid/rotate-credentials", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// Note: GetAgentTrustScore and GetAgentTrustScoreHistory delegate directly to TrustScoreHandler
// Those tests are in trust_score_handler_test.go

// ===========================
// AgentHandler.SuspendAgent Tests
// ===========================

func TestAgentHandler_SuspendAgent_NoOrgContext(t *testing.T) {
	handler := &AgentHandler{}
	app := newTestApp()
	app.Post("/agents/:id/suspend", handler.SuspendAgent)

	req := httptest.NewRequest("POST", "/agents/"+uuid.New().String()+"/suspend", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAgentHandler_SuspendAgent_InvalidID(t *testing.T) {
	handler := &AgentHandler{}
	app := fiber.New()
	app.Post("/agents/:id/suspend", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		c.Locals("user_id", uuid.New())
		return handler.SuspendAgent(c)
	})

	req := httptest.NewRequest("POST", "/agents/not-a-uuid/suspend", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// AgentHandler.ReactivateAgent Tests
// ===========================

func TestAgentHandler_ReactivateAgent_NoOrgContext(t *testing.T) {
	handler := &AgentHandler{}
	app := newTestApp()
	app.Post("/agents/:id/reactivate", handler.ReactivateAgent)

	req := httptest.NewRequest("POST", "/agents/"+uuid.New().String()+"/reactivate", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAgentHandler_ReactivateAgent_InvalidID(t *testing.T) {
	handler := &AgentHandler{}
	app := fiber.New()
	app.Post("/agents/:id/reactivate", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		c.Locals("user_id", uuid.New())
		return handler.ReactivateAgent(c)
	})

	req := httptest.NewRequest("POST", "/agents/not-a-uuid/reactivate", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// AgentHandler.VerifyAgent Tests
// ===========================

func TestAgentHandler_VerifyAgent_NoOrgContext(t *testing.T) {
	handler := &AgentHandler{}
	app := newTestApp()
	app.Post("/agents/verify", handler.VerifyAgent)

	body := `{"agentName":"test","signature":"sig"}`
	req := httptest.NewRequest("POST", "/agents/verify", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAgentHandler_VerifyAgent_InvalidID(t *testing.T) {
	handler := &AgentHandler{}
	app := newTestApp()
	app.Post("/agents/:id/verify", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		c.Locals("user_id", uuid.New())
		return handler.VerifyAgent(c)
	})

	req := httptest.NewRequest("POST", "/agents/not-a-uuid/verify", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// AgentHandler.VerifyCapability Tests
// ===========================

func TestAgentHandler_VerifyCapability_InvalidAgentID(t *testing.T) {
	handler := &AgentHandler{}
	app := newTestApp()
	app.Post("/agents/:id/verify-capability", handler.VerifyCapability)

	body := `{"capability":"file:read","resource":"/data/file.csv"}`
	req := httptest.NewRequest("POST", "/agents/not-a-uuid/verify-capability", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestAgentHandler_VerifyCapability_InvalidJSON(t *testing.T) {
	handler := &AgentHandler{}
	app := newTestApp()
	app.Post("/agents/:id/verify-capability", handler.VerifyCapability)

	req := httptest.NewRequest("POST", "/agents/"+uuid.New().String()+"/verify-capability", strings.NewReader("not valid json"))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// AgentHandler.LogCapabilityResult Tests
// ===========================

func TestAgentHandler_LogCapabilityResult_InvalidAgentID(t *testing.T) {
	handler := &AgentHandler{}
	app := newTestApp()
	app.Post("/agents/:id/log-capability/:audit_id", handler.LogCapabilityResult)

	body := `{"success":true}`
	req := httptest.NewRequest("POST", "/agents/not-a-uuid/log-capability/"+uuid.New().String(), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestAgentHandler_LogCapabilityResult_InvalidAuditID(t *testing.T) {
	handler := &AgentHandler{}
	app := newTestApp()
	app.Post("/agents/:id/log-capability/:audit_id", handler.LogCapabilityResult)

	body := `{"success":true}`
	req := httptest.NewRequest("POST", "/agents/"+uuid.New().String()+"/log-capability/not-a-uuid", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestAgentHandler_LogCapabilityResult_InvalidJSON(t *testing.T) {
	handler := &AgentHandler{}
	app := newTestApp()
	app.Post("/agents/:id/log-capability/:audit_id", handler.LogCapabilityResult)

	req := httptest.NewRequest("POST", "/agents/"+uuid.New().String()+"/log-capability/"+uuid.New().String(), strings.NewReader("not valid json"))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// AgentHandler.AddMCPServersToAgent Tests
// ===========================

func TestAgentHandler_AddMCPServersToAgent_NoOrgContext(t *testing.T) {
	handler := &AgentHandler{}
	app := newTestApp()
	app.Post("/agents/:id/mcp-servers", handler.AddMCPServersToAgent)

	body := `{"mcpServerIds":["` + uuid.New().String() + `"]}`
	req := httptest.NewRequest("POST", "/agents/"+uuid.New().String()+"/mcp-servers", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAgentHandler_AddMCPServersToAgent_InvalidID(t *testing.T) {
	handler := &AgentHandler{}
	app := fiber.New()
	app.Post("/agents/:id/mcp-servers", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		c.Locals("user_id", uuid.New())
		return handler.AddMCPServersToAgent(c)
	})

	body := `{"mcpServerIds":["` + uuid.New().String() + `"]}`
	req := httptest.NewRequest("POST", "/agents/not-a-uuid/mcp-servers", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// AgentHandler.RemoveMCPServerFromAgent Tests
// ===========================

func TestAgentHandler_RemoveMCPServerFromAgent_NoOrgContext(t *testing.T) {
	handler := &AgentHandler{}
	app := newTestApp()
	app.Delete("/agents/:id/mcp-servers/:serverId", handler.RemoveMCPServerFromAgent)

	req := httptest.NewRequest("DELETE", "/agents/"+uuid.New().String()+"/mcp-servers/"+uuid.New().String(), nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAgentHandler_RemoveMCPServerFromAgent_InvalidAgentID(t *testing.T) {
	handler := &AgentHandler{}
	app := fiber.New()
	app.Delete("/agents/:id/mcp-servers/:serverId", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		c.Locals("user_id", uuid.New())
		return handler.RemoveMCPServerFromAgent(c)
	})

	req := httptest.NewRequest("DELETE", "/agents/not-a-uuid/mcp-servers/"+uuid.New().String(), nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestAgentHandler_RemoveMCPServerFromAgent_InvalidServerID(t *testing.T) {
	handler := &AgentHandler{}
	app := fiber.New()
	app.Delete("/agents/:id/mcp-servers/:serverId", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		c.Locals("user_id", uuid.New())
		return handler.RemoveMCPServerFromAgent(c)
	})

	req := httptest.NewRequest("DELETE", "/agents/"+uuid.New().String()+"/mcp-servers/not-a-uuid", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// AgentHandler.GetCredentials Tests
// ===========================

func TestAgentHandler_GetCredentials_NoOrgContext(t *testing.T) {
	handler := &AgentHandler{}
	app := newTestApp()
	app.Get("/agents/:id/credentials", handler.GetCredentials)

	req := httptest.NewRequest("GET", "/agents/"+uuid.New().String()+"/credentials", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAgentHandler_GetCredentials_InvalidID(t *testing.T) {
	handler := &AgentHandler{}
	app := newTestApp()
	app.Get("/agents/:id/credentials", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		c.Locals("user_id", uuid.New())
		return handler.GetCredentials(c)
	})

	req := httptest.NewRequest("GET", "/agents/not-a-uuid/credentials", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// AgentHandler.GetAgentByIdentifier Tests
// ===========================

func TestAgentHandler_GetAgentByIdentifier_NoOrgContext(t *testing.T) {
	handler := &AgentHandler{}
	app := newTestApp()
	app.Get("/agents/by-identifier/:identifier", handler.GetAgentByIdentifier)

	req := httptest.NewRequest("GET", "/agents/by-identifier/my-agent", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAgentHandler_GetAgentByIdentifier_EmptyIdentifier(t *testing.T) {
	handler := &AgentHandler{}
	app := fiber.New()
	app.Get("/agents/by-identifier/:identifier", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		return handler.GetAgentByIdentifier(c)
	})

	req := httptest.NewRequest("GET", "/agents/by-identifier/", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Route won't match without identifier
	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

// ===========================
// NewAgentHandler Tests
// ===========================

func TestNewAgentHandler_AllNilDeps(t *testing.T) {
	handler := NewAgentHandler(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	assert.NotNil(t, handler)
}

// ===========================
// AgentHandler.UpdateAgentTrustScore Tests
// ===========================

func TestAgentHandler_UpdateAgentTrustScore_NoOrgContext(t *testing.T) {
	handler := &AgentHandler{}
	app := newTestApp()
	app.Put("/agents/:id/trust-score", handler.UpdateAgentTrustScore)

	body := `{"score":0.85,"reason":"test"}`
	req := httptest.NewRequest("PUT", "/agents/"+uuid.New().String()+"/trust-score", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAgentHandler_UpdateAgentTrustScore_NoUserContext(t *testing.T) {
	handler := &AgentHandler{}
	app := newTestApp()
	app.Put("/agents/:id/trust-score", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		// No user_id
		return handler.UpdateAgentTrustScore(c)
	})

	body := `{"score":0.85,"reason":"test"}`
	req := httptest.NewRequest("PUT", "/agents/"+uuid.New().String()+"/trust-score", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAgentHandler_UpdateAgentTrustScore_InvalidAgentID(t *testing.T) {
	handler := &AgentHandler{}
	app := fiber.New()
	app.Put("/agents/:id/trust-score", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		c.Locals("user_id", uuid.New())
		return handler.UpdateAgentTrustScore(c)
	})

	body := `{"score":0.85,"reason":"test"}`
	req := httptest.NewRequest("PUT", "/agents/not-a-uuid/trust-score", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestAgentHandler_UpdateAgentTrustScore_InvalidJSON(t *testing.T) {
	handler := &AgentHandler{}
	app := fiber.New()
	app.Put("/agents/:id/trust-score", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		c.Locals("user_id", uuid.New())
		return handler.UpdateAgentTrustScore(c)
	})

	req := httptest.NewRequest("PUT", "/agents/"+uuid.New().String()+"/trust-score", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestAgentHandler_UpdateAgentTrustScore_InvalidScoreRange(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"negative score", `{"score":-0.1,"reason":"test"}`},
		{"score too high", `{"score":100.1,"reason":"test"}`},
		{"score way too high", `{"score":500.0,"reason":"test"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := &AgentHandler{}
			app := fiber.New()
			app.Put("/agents/:id/trust-score", func(c fiber.Ctx) error {
				c.Locals("organization_id", uuid.New())
				c.Locals("user_id", uuid.New())
				return handler.UpdateAgentTrustScore(c)
			})

			req := httptest.NewRequest("PUT", "/agents/"+uuid.New().String()+"/trust-score", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")

			resp, err := app.Test(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
		})
	}
}

// ===========================
// AgentHandler.DetectAndMapMCPServers Tests
// ===========================

func TestAgentHandler_DetectAndMapMCPServers_NoOrgContext(t *testing.T) {
	handler := &AgentHandler{}
	app := newTestApp()
	app.Post("/agents/:id/detect-mcp", handler.DetectAndMapMCPServers)

	req := httptest.NewRequest("POST", "/agents/"+uuid.New().String()+"/detect-mcp", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAgentHandler_DetectAndMapMCPServers_NoUserContext(t *testing.T) {
	handler := &AgentHandler{}
	app := newTestApp()
	app.Post("/agents/:id/detect-mcp", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		// No user_id
		return handler.DetectAndMapMCPServers(c)
	})

	req := httptest.NewRequest("POST", "/agents/"+uuid.New().String()+"/detect-mcp", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAgentHandler_DetectAndMapMCPServers_InvalidAgentID(t *testing.T) {
	handler := &AgentHandler{}
	app := fiber.New()
	app.Post("/agents/:id/detect-mcp", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		c.Locals("user_id", uuid.New())
		return handler.DetectAndMapMCPServers(c)
	})

	req := httptest.NewRequest("POST", "/agents/not-a-uuid/detect-mcp", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// AgentHandler.UpdateAgentKeys Tests
// ===========================

func TestAgentHandler_UpdateAgentKeys_NoOrgContext(t *testing.T) {
	handler := &AgentHandler{}
	app := newTestApp()
	app.Put("/agents/:id/keys", handler.UpdateAgentKeys)

	body := `{"publicKey":"test"}`
	req := httptest.NewRequest("PUT", "/agents/"+uuid.New().String()+"/keys", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAgentHandler_UpdateAgentKeys_NoUserContext(t *testing.T) {
	handler := &AgentHandler{}
	app := newTestApp()
	app.Put("/agents/:id/keys", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		// No user_id
		return handler.UpdateAgentKeys(c)
	})

	body := `{"publicKey":"test"}`
	req := httptest.NewRequest("PUT", "/agents/"+uuid.New().String()+"/keys", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAgentHandler_UpdateAgentKeys_InvalidAgentID(t *testing.T) {
	handler := &AgentHandler{}
	app := fiber.New()
	app.Put("/agents/:id/keys", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		c.Locals("user_id", uuid.New())
		return handler.UpdateAgentKeys(c)
	})

	body := `{"publicKey":"test"}`
	req := httptest.NewRequest("PUT", "/agents/not-a-uuid/keys", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestAgentHandler_UpdateAgentKeys_InvalidJSON(t *testing.T) {
	handler := &AgentHandler{}
	app := fiber.New()
	app.Put("/agents/:id/keys", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		c.Locals("user_id", uuid.New())
		return handler.UpdateAgentKeys(c)
	})

	req := httptest.NewRequest("PUT", "/agents/"+uuid.New().String()+"/keys", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestAgentHandler_UpdateAgentKeys_EmptyPublicKey(t *testing.T) {
	handler := &AgentHandler{}
	app := fiber.New()
	app.Put("/agents/:id/keys", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		c.Locals("user_id", uuid.New())
		return handler.UpdateAgentKeys(c)
	})

	body := `{"publicKey":""}`
	req := httptest.NewRequest("PUT", "/agents/"+uuid.New().String()+"/keys", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// getAIMBaseURL Tests
// ===========================

func TestGetAIMBaseURL_HTTP(t *testing.T) {
	app := fiber.New()
	var result string
	app.Get("/test", func(c fiber.Ctx) error {
		result = getAIMBaseURL(c)
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Host = "localhost:8080"

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// c.Hostname() returns hostname without port
	assert.Equal(t, "http://localhost", result)
}

func TestGetAIMBaseURL_HTTPS_Protocol(t *testing.T) {
	app := fiber.New()
	var result string
	app.Get("/test", func(c fiber.Ctx) error {
		result = getAIMBaseURL(c)
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Host = "api.example.com"
	req.Header.Set("X-Forwarded-Proto", "https")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, "https://api.example.com", result)
}

func TestGetAIMBaseURL_DifferentHosts(t *testing.T) {
	// Note: c.Hostname() returns hostname without port
	tests := []struct {
		name           string
		host           string
		forwardedProto string
		expected       string
	}{
		{"localhost", "localhost", "", "http://localhost"},
		{"localhost with port", "localhost:3000", "", "http://localhost"}, // port stripped by Hostname()
		// Public hosts are coerced to https by normalizeAIMURL even when the
		// request arrives over http (TLS-terminating ingress doesn't forward the
		// scheme), so the embedded URL never 301s and strips the refresh POST body.
		{"domain", "api.aim.example.com", "", "https://api.aim.example.com"},
		{"domain https", "api.aim.example.com", "https", "https://api.aim.example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := fiber.New()
			var result string
			app.Get("/test", func(c fiber.Ctx) error {
				result = getAIMBaseURL(c)
				return c.SendStatus(fiber.StatusOK)
			})

			req := httptest.NewRequest("GET", "/test", nil)
			req.Host = tt.host
			if tt.forwardedProto != "" {
				req.Header.Set("X-Forwarded-Proto", tt.forwardedProto)
			}

			resp, err := app.Test(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tt.expected, result)
		})
	}
}
