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

// Helper function for webhook tests that need org/user context
func withWebhookContext(handler func(c fiber.Ctx) error) fiber.Handler {
	return func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		c.Locals("user_id", uuid.New())
		return handler(c)
	}
}

// ===========================
// NewWebhookHandler Tests
// ===========================

func TestNewWebhookHandler_NilDeps(t *testing.T) {
	handler := NewWebhookHandler(nil, nil)
	assert.NotNil(t, handler)
}

// ===========================
// WebhookHandler.CreateWebhook Tests
// ===========================

func TestWebhookHandler_CreateWebhook_InvalidJSON(t *testing.T) {
	handler := &WebhookHandler{}
	app := fiber.New()
	app.Post("/webhooks", withWebhookContext(handler.CreateWebhook))

	req := httptest.NewRequest("POST", "/webhooks", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// WebhookHandler.GetWebhook Tests
// ===========================

func TestWebhookHandler_GetWebhook_InvalidID(t *testing.T) {
	handler := &WebhookHandler{}
	app := fiber.New()
	app.Get("/webhooks/:id", withWebhookContext(handler.GetWebhook))

	req := httptest.NewRequest("GET", "/webhooks/not-a-uuid", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// WebhookHandler.DeleteWebhook Tests
// ===========================

func TestWebhookHandler_DeleteWebhook_InvalidID(t *testing.T) {
	handler := &WebhookHandler{}
	app := fiber.New()
	app.Delete("/webhooks/:id", withWebhookContext(handler.DeleteWebhook))

	req := httptest.NewRequest("DELETE", "/webhooks/not-a-uuid", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// WebhookHandler.UpdateWebhook Tests
// ===========================

func TestWebhookHandler_UpdateWebhook_InvalidID(t *testing.T) {
	handler := &WebhookHandler{}
	app := fiber.New()
	app.Put("/webhooks/:id", withWebhookContext(handler.UpdateWebhook))

	body := `{"name":"updated"}`
	req := httptest.NewRequest("PUT", "/webhooks/not-a-uuid", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// Note: UpdateWebhook calls GetWebhook before parsing JSON, so we can't test
// invalid JSON without hitting the service. This validation is tested implicitly.

// ===========================
// WebhookHandler.TestWebhook Tests
// ===========================

func TestWebhookHandler_TestWebhook_InvalidID(t *testing.T) {
	handler := &WebhookHandler{}
	app := fiber.New()
	app.Post("/webhooks/:id/test", withWebhookContext(handler.TestWebhook))

	req := httptest.NewRequest("POST", "/webhooks/not-a-uuid/test", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// WebhookHandler Service Getter Tests
// ===========================

func TestWebhookHandler_getWebhookService_NilInterfaceReturnsNil(t *testing.T) {
	handler := &WebhookHandler{}
	result := handler.getWebhookService()
	assert.Nil(t, result)
}

func TestWebhookHandler_getAuditService_NilInterfaceReturnsNil(t *testing.T) {
	handler := &WebhookHandler{}
	result := handler.getAuditService()
	assert.Nil(t, result)
}
