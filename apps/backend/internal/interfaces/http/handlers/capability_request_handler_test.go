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

// Helper function for capability request tests that need org/user context
func withCapabilityRequestContext(handler func(c fiber.Ctx) error) fiber.Handler {
	return func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		c.Locals("user_id", uuid.New())
		return handler(c)
	}
}

// ===========================
// NewCapabilityRequestHandlers Tests
// ===========================

func TestNewCapabilityRequestHandlers_NilDeps(t *testing.T) {
	handler := NewCapabilityRequestHandlers(nil, nil)
	assert.NotNil(t, handler)
}

// ===========================
// CapabilityRequestHandlers.CreateCapabilityRequest Tests
// ===========================

func TestCapabilityRequestHandlers_CreateCapabilityRequest_InvalidAgentID(t *testing.T) {
	handler := &CapabilityRequestHandlers{}
	app := fiber.New()
	app.Post("/agents/:id/capability-requests", handler.CreateCapabilityRequest)

	body := `{"capabilityType":"file_write","reason":"this is a long enough reason"}`
	req := httptest.NewRequest("POST", "/agents/not-a-uuid/capability-requests", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// Note: CreateCapabilityRequest calls agentRepo.GetByID before JSON parsing,
// so InvalidJSON test would hit nil repo. Validation is tested implicitly.

// ===========================
// CapabilityRequestHandlers.ListCapabilityRequests Tests
// ===========================

func TestCapabilityRequestHandlers_ListCapabilityRequests_NoOrgContext(t *testing.T) {
	handler := &CapabilityRequestHandlers{}
	app := fiber.New()
	app.Get("/capability-requests", handler.ListCapabilityRequests)

	req := httptest.NewRequest("GET", "/capability-requests", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

// ===========================
// CapabilityRequestHandlers.GetCapabilityRequest Tests
// ===========================

func TestCapabilityRequestHandlers_GetCapabilityRequest_InvalidID(t *testing.T) {
	handler := &CapabilityRequestHandlers{}
	app := fiber.New()
	app.Get("/capability-requests/:id", handler.GetCapabilityRequest)

	req := httptest.NewRequest("GET", "/capability-requests/not-a-uuid", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// CapabilityRequestHandlers.ApproveCapabilityRequest Tests
// ===========================

func TestCapabilityRequestHandlers_ApproveCapabilityRequest_NoUserContext(t *testing.T) {
	handler := &CapabilityRequestHandlers{}
	app := fiber.New()
	app.Post("/capability-requests/:id/approve", handler.ApproveCapabilityRequest)

	req := httptest.NewRequest("POST", "/capability-requests/"+uuid.New().String()+"/approve", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestCapabilityRequestHandlers_ApproveCapabilityRequest_InvalidID(t *testing.T) {
	handler := &CapabilityRequestHandlers{}
	app := fiber.New()
	app.Post("/capability-requests/:id/approve", withCapabilityRequestContext(handler.ApproveCapabilityRequest))

	req := httptest.NewRequest("POST", "/capability-requests/not-a-uuid/approve", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// CapabilityRequestHandlers.RejectCapabilityRequest Tests
// ===========================

func TestCapabilityRequestHandlers_RejectCapabilityRequest_NoUserContext(t *testing.T) {
	handler := &CapabilityRequestHandlers{}
	app := fiber.New()
	app.Post("/capability-requests/:id/reject", handler.RejectCapabilityRequest)

	req := httptest.NewRequest("POST", "/capability-requests/"+uuid.New().String()+"/reject", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestCapabilityRequestHandlers_RejectCapabilityRequest_InvalidID(t *testing.T) {
	handler := &CapabilityRequestHandlers{}
	app := fiber.New()
	app.Post("/capability-requests/:id/reject", withCapabilityRequestContext(handler.RejectCapabilityRequest))

	req := httptest.NewRequest("POST", "/capability-requests/not-a-uuid/reject", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// CapabilityRequestHandlers.ListAgentCapabilityRequests Tests
// ===========================

func TestCapabilityRequestHandlers_ListAgentCapabilityRequests_InvalidAgentID(t *testing.T) {
	handler := &CapabilityRequestHandlers{}
	app := fiber.New()
	app.Get("/agents/:id/capability-requests", handler.ListAgentCapabilityRequests)

	req := httptest.NewRequest("GET", "/agents/not-a-uuid/capability-requests", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}
