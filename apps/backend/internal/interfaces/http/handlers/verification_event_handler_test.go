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

// Helper function for verification event tests that need org context
func withVerificationEventContext(handler func(c fiber.Ctx) error) fiber.Handler {
	return func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		c.Locals("user_id", uuid.New())
		return handler(c)
	}
}

// ===========================
// NewVerificationEventHandler Tests
// ===========================

func TestNewVerificationEventHandler_NilDeps(t *testing.T) {
	handler := NewVerificationEventHandler(nil)
	assert.NotNil(t, handler)
}

// ===========================
// VerificationEventHandler.ListVerificationEvents Tests
// ===========================

func TestVerificationEventHandler_ListVerificationEvents_NoOrgContext(t *testing.T) {
	handler := &VerificationEventHandler{}
	app := fiber.New()
	app.Get("/verification-events", handler.ListVerificationEvents)

	req := httptest.NewRequest("GET", "/verification-events", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestVerificationEventHandler_ListVerificationEvents_InvalidAgentID(t *testing.T) {
	handler := &VerificationEventHandler{}
	app := fiber.New()
	app.Get("/verification-events", withVerificationEventContext(handler.ListVerificationEvents))

	req := httptest.NewRequest("GET", "/verification-events?agent_id=not-a-uuid", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// VerificationEventHandler.GetVerificationEvent Tests
// ===========================

func TestVerificationEventHandler_GetVerificationEvent_NoOrgContext(t *testing.T) {
	handler := &VerificationEventHandler{}
	app := fiber.New()
	app.Get("/verification-events/:id", handler.GetVerificationEvent)

	req := httptest.NewRequest("GET", "/verification-events/"+uuid.New().String(), nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestVerificationEventHandler_GetVerificationEvent_InvalidID(t *testing.T) {
	handler := &VerificationEventHandler{}
	app := fiber.New()
	app.Get("/verification-events/:id", withVerificationEventContext(handler.GetVerificationEvent))

	req := httptest.NewRequest("GET", "/verification-events/not-a-uuid", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// VerificationEventHandler.CreateVerificationEvent Tests
// ===========================

func TestVerificationEventHandler_CreateVerificationEvent_NoOrgContext(t *testing.T) {
	handler := &VerificationEventHandler{}
	app := fiber.New()
	app.Post("/verification-events", handler.CreateVerificationEvent)

	body := `{"agentId":"test"}`
	req := httptest.NewRequest("POST", "/verification-events", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestVerificationEventHandler_CreateVerificationEvent_InvalidJSON(t *testing.T) {
	handler := &VerificationEventHandler{}
	app := fiber.New()
	app.Post("/verification-events", withVerificationEventContext(handler.CreateVerificationEvent))

	req := httptest.NewRequest("POST", "/verification-events", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestVerificationEventHandler_CreateVerificationEvent_InvalidAgentID(t *testing.T) {
	handler := &VerificationEventHandler{}
	app := fiber.New()
	app.Post("/verification-events", withVerificationEventContext(handler.CreateVerificationEvent))

	body := `{"agentId":"not-a-uuid"}`
	req := httptest.NewRequest("POST", "/verification-events", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// VerificationEventHandler.GetRecentEvents Tests
// ===========================

func TestVerificationEventHandler_GetRecentEvents_NoOrgContext(t *testing.T) {
	handler := &VerificationEventHandler{}
	app := fiber.New()
	app.Get("/verification-events/recent", handler.GetRecentEvents)

	req := httptest.NewRequest("GET", "/verification-events/recent", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

// ===========================
// VerificationEventHandler.GetStatistics Tests
// ===========================

func TestVerificationEventHandler_GetStatistics_NoOrgContext(t *testing.T) {
	handler := &VerificationEventHandler{}
	app := fiber.New()
	app.Get("/verification-events/statistics", handler.GetStatistics)

	req := httptest.NewRequest("GET", "/verification-events/statistics", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

// ===========================
// VerificationEventHandler.GetAgentVerificationEvents Tests
// ===========================

func TestVerificationEventHandler_GetAgentVerificationEvents_NoOrgContext(t *testing.T) {
	handler := &VerificationEventHandler{}
	app := fiber.New()
	app.Get("/verification-events/agent/:agent_id", handler.GetAgentVerificationEvents)

	req := httptest.NewRequest("GET", "/verification-events/agent/"+uuid.New().String(), nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestVerificationEventHandler_GetAgentVerificationEvents_InvalidAgentID(t *testing.T) {
	handler := &VerificationEventHandler{}
	app := fiber.New()
	app.Get("/verification-events/agent/:agent_id", withVerificationEventContext(handler.GetAgentVerificationEvents))

	req := httptest.NewRequest("GET", "/verification-events/agent/not-a-uuid", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// VerificationEventHandler.GetMCPVerificationEvents Tests
// ===========================

func TestVerificationEventHandler_GetMCPVerificationEvents_NoOrgContext(t *testing.T) {
	handler := &VerificationEventHandler{}
	app := fiber.New()
	app.Get("/verification-events/mcp/:mcp_id", handler.GetMCPVerificationEvents)

	req := httptest.NewRequest("GET", "/verification-events/mcp/"+uuid.New().String(), nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestVerificationEventHandler_GetMCPVerificationEvents_InvalidMCPID(t *testing.T) {
	handler := &VerificationEventHandler{}
	app := fiber.New()
	app.Get("/verification-events/mcp/:mcp_id", withVerificationEventContext(handler.GetMCPVerificationEvents))

	req := httptest.NewRequest("GET", "/verification-events/mcp/not-a-uuid", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// VerificationEventHandler.GetVerificationStats Tests
// ===========================

func TestVerificationEventHandler_GetVerificationStats_NoOrgContext(t *testing.T) {
	handler := &VerificationEventHandler{}
	app := fiber.New()
	app.Get("/verification-events/stats", handler.GetVerificationStats)

	req := httptest.NewRequest("GET", "/verification-events/stats", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

// ===========================
// VerificationEventHandler.DeleteVerificationEvent Tests
// ===========================

func TestVerificationEventHandler_DeleteVerificationEvent_NoOrgContext(t *testing.T) {
	handler := &VerificationEventHandler{}
	app := fiber.New()
	app.Delete("/verification-events/:id", handler.DeleteVerificationEvent)

	req := httptest.NewRequest("DELETE", "/verification-events/"+uuid.New().String(), nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestVerificationEventHandler_DeleteVerificationEvent_InvalidID(t *testing.T) {
	handler := &VerificationEventHandler{}
	app := fiber.New()
	app.Delete("/verification-events/:id", withVerificationEventContext(handler.DeleteVerificationEvent))

	req := httptest.NewRequest("DELETE", "/verification-events/not-a-uuid", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}
