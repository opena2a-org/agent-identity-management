package handlers

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===========================
// AgentHandler.GetAgentKeyVault Tests
// ===========================

func TestAgentHandler_GetAgentKeyVault_NoOrgContext(t *testing.T) {
	handler := &AgentHandler{}
	app := fiber.New()
	app.Get("/agents/:id/key-vault", handler.GetAgentKeyVault)

	req := httptest.NewRequest("GET", "/agents/"+uuid.New().String()+"/key-vault", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAgentHandler_GetAgentKeyVault_NoUserContext(t *testing.T) {
	handler := &AgentHandler{}
	app := fiber.New()
	app.Get("/agents/:id/key-vault", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		// No user_id set
		return handler.GetAgentKeyVault(c)
	})

	req := httptest.NewRequest("GET", "/agents/"+uuid.New().String()+"/key-vault", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAgentHandler_GetAgentKeyVault_InvalidAgentID(t *testing.T) {
	handler := &AgentHandler{}
	app := fiber.New()
	app.Get("/agents/:id/key-vault", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		c.Locals("user_id", uuid.New())
		return handler.GetAgentKeyVault(c)
	})

	req := httptest.NewRequest("GET", "/agents/not-a-uuid/key-vault", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// AgentHandler.GetAgentAuditLogs Tests
// ===========================

func TestAgentHandler_GetAgentAuditLogs_NoOrgContext(t *testing.T) {
	handler := &AgentHandler{}
	app := fiber.New()
	app.Get("/agents/:id/audit-logs", handler.GetAgentAuditLogs)

	req := httptest.NewRequest("GET", "/agents/"+uuid.New().String()+"/audit-logs", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAgentHandler_GetAgentAuditLogs_NoUserContext(t *testing.T) {
	handler := &AgentHandler{}
	app := fiber.New()
	app.Get("/agents/:id/audit-logs", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		// No user_id set
		return handler.GetAgentAuditLogs(c)
	})

	req := httptest.NewRequest("GET", "/agents/"+uuid.New().String()+"/audit-logs", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAgentHandler_GetAgentAuditLogs_InvalidAgentID(t *testing.T) {
	handler := &AgentHandler{}
	app := fiber.New()
	app.Get("/agents/:id/audit-logs", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		c.Locals("user_id", uuid.New())
		return handler.GetAgentAuditLogs(c)
	})

	req := httptest.NewRequest("GET", "/agents/not-a-uuid/audit-logs", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}
