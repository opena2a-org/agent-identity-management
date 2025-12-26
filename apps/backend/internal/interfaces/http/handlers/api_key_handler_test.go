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
// NewAPIKeyHandler Tests
// ===========================

func TestNewAPIKeyHandler_NilDeps(t *testing.T) {
	handler := NewAPIKeyHandler(nil, nil)
	assert.NotNil(t, handler)
}

// ===========================
// APIKeyHandler.ListAPIKeys Tests
// ===========================

func TestAPIKeyHandler_ListAPIKeys_NoOrgContext(t *testing.T) {
	handler := &APIKeyHandler{}
	app := fiber.New()
	app.Get("/api-keys", handler.ListAPIKeys)

	req := httptest.NewRequest("GET", "/api-keys", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAPIKeyHandler_ListAPIKeys_InvalidOrgIDType(t *testing.T) {
	handler := &APIKeyHandler{}
	app := fiber.New()
	app.Get("/api-keys", func(c fiber.Ctx) error {
		c.Locals("organization_id", "not-a-uuid") // Wrong type
		return handler.ListAPIKeys(c)
	})

	req := httptest.NewRequest("GET", "/api-keys", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

func TestAPIKeyHandler_ListAPIKeys_InvalidAgentIDFilter(t *testing.T) {
	handler := &APIKeyHandler{}
	app := fiber.New()
	app.Get("/api-keys", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		return handler.ListAPIKeys(c)
	})

	req := httptest.NewRequest("GET", "/api-keys?agent_id=not-a-uuid", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// APIKeyHandler.CreateAPIKey Tests
// ===========================

func TestAPIKeyHandler_CreateAPIKey_NoOrgContext(t *testing.T) {
	handler := &APIKeyHandler{}
	app := fiber.New()
	app.Post("/api-keys", handler.CreateAPIKey)

	body := `{"agentId":"` + uuid.New().String() + `","name":"test-key"}`
	req := httptest.NewRequest("POST", "/api-keys", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAPIKeyHandler_CreateAPIKey_InvalidOrgIDType(t *testing.T) {
	handler := &APIKeyHandler{}
	app := fiber.New()
	app.Post("/api-keys", func(c fiber.Ctx) error {
		c.Locals("organization_id", "not-a-uuid") // Wrong type
		return handler.CreateAPIKey(c)
	})

	body := `{"agentId":"` + uuid.New().String() + `","name":"test-key"}`
	req := httptest.NewRequest("POST", "/api-keys", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

func TestAPIKeyHandler_CreateAPIKey_NoUserContext(t *testing.T) {
	handler := &APIKeyHandler{}
	app := fiber.New()
	app.Post("/api-keys", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		// No user_id set
		return handler.CreateAPIKey(c)
	})

	body := `{"agentId":"` + uuid.New().String() + `","name":"test-key"}`
	req := httptest.NewRequest("POST", "/api-keys", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAPIKeyHandler_CreateAPIKey_InvalidUserIDType(t *testing.T) {
	handler := &APIKeyHandler{}
	app := fiber.New()
	app.Post("/api-keys", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		c.Locals("user_id", "not-a-uuid") // Wrong type
		return handler.CreateAPIKey(c)
	})

	body := `{"agentId":"` + uuid.New().String() + `","name":"test-key"}`
	req := httptest.NewRequest("POST", "/api-keys", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

func TestAPIKeyHandler_CreateAPIKey_InvalidJSON(t *testing.T) {
	handler := &APIKeyHandler{}
	app := fiber.New()
	app.Post("/api-keys", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		c.Locals("user_id", uuid.New())
		return handler.CreateAPIKey(c)
	})

	req := httptest.NewRequest("POST", "/api-keys", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestAPIKeyHandler_CreateAPIKey_MissingRequiredFields(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"missing agentId", `{"name":"test-key"}`},
		{"missing name", `{"agentId":"` + uuid.New().String() + `"}`},
		{"empty agentId", `{"agentId":"","name":"test-key"}`},
		{"empty name", `{"agentId":"` + uuid.New().String() + `","name":""}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := &APIKeyHandler{}
			app := fiber.New()
			app.Post("/api-keys", func(c fiber.Ctx) error {
				c.Locals("organization_id", uuid.New())
				c.Locals("user_id", uuid.New())
				return handler.CreateAPIKey(c)
			})

			req := httptest.NewRequest("POST", "/api-keys", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")

			resp, err := app.Test(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
		})
	}
}

func TestAPIKeyHandler_CreateAPIKey_InvalidAgentID(t *testing.T) {
	handler := &APIKeyHandler{}
	app := fiber.New()
	app.Post("/api-keys", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		c.Locals("user_id", uuid.New())
		return handler.CreateAPIKey(c)
	})

	body := `{"agentId":"not-a-uuid","name":"test-key"}`
	req := httptest.NewRequest("POST", "/api-keys", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// APIKeyHandler.DisableAPIKey Tests
// ===========================

func TestAPIKeyHandler_DisableAPIKey_NoOrgContext(t *testing.T) {
	handler := &APIKeyHandler{}
	app := fiber.New()
	app.Patch("/api-keys/:id/disable", handler.DisableAPIKey)

	req := httptest.NewRequest("PATCH", "/api-keys/"+uuid.New().String()+"/disable", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAPIKeyHandler_DisableAPIKey_InvalidOrgIDType(t *testing.T) {
	handler := &APIKeyHandler{}
	app := fiber.New()
	app.Patch("/api-keys/:id/disable", func(c fiber.Ctx) error {
		c.Locals("organization_id", "not-a-uuid") // Wrong type
		return handler.DisableAPIKey(c)
	})

	req := httptest.NewRequest("PATCH", "/api-keys/"+uuid.New().String()+"/disable", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

func TestAPIKeyHandler_DisableAPIKey_NoUserContext(t *testing.T) {
	handler := &APIKeyHandler{}
	app := fiber.New()
	app.Patch("/api-keys/:id/disable", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		// No user_id set
		return handler.DisableAPIKey(c)
	})

	req := httptest.NewRequest("PATCH", "/api-keys/"+uuid.New().String()+"/disable", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAPIKeyHandler_DisableAPIKey_InvalidUserIDType(t *testing.T) {
	handler := &APIKeyHandler{}
	app := fiber.New()
	app.Patch("/api-keys/:id/disable", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		c.Locals("user_id", "not-a-uuid") // Wrong type
		return handler.DisableAPIKey(c)
	})

	req := httptest.NewRequest("PATCH", "/api-keys/"+uuid.New().String()+"/disable", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

func TestAPIKeyHandler_DisableAPIKey_InvalidKeyID(t *testing.T) {
	handler := &APIKeyHandler{}
	app := fiber.New()
	app.Patch("/api-keys/:id/disable", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		c.Locals("user_id", uuid.New())
		return handler.DisableAPIKey(c)
	})

	req := httptest.NewRequest("PATCH", "/api-keys/not-a-uuid/disable", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// APIKeyHandler.DeleteAPIKey Tests
// ===========================

func TestAPIKeyHandler_DeleteAPIKey_NoOrgContext(t *testing.T) {
	handler := &APIKeyHandler{}
	app := fiber.New()
	app.Delete("/api-keys/:id", handler.DeleteAPIKey)

	req := httptest.NewRequest("DELETE", "/api-keys/"+uuid.New().String(), nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAPIKeyHandler_DeleteAPIKey_InvalidOrgIDType(t *testing.T) {
	handler := &APIKeyHandler{}
	app := fiber.New()
	app.Delete("/api-keys/:id", func(c fiber.Ctx) error {
		c.Locals("organization_id", "not-a-uuid") // Wrong type
		return handler.DeleteAPIKey(c)
	})

	req := httptest.NewRequest("DELETE", "/api-keys/"+uuid.New().String(), nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

func TestAPIKeyHandler_DeleteAPIKey_NoUserContext(t *testing.T) {
	handler := &APIKeyHandler{}
	app := fiber.New()
	app.Delete("/api-keys/:id", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		// No user_id set
		return handler.DeleteAPIKey(c)
	})

	req := httptest.NewRequest("DELETE", "/api-keys/"+uuid.New().String(), nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAPIKeyHandler_DeleteAPIKey_InvalidUserIDType(t *testing.T) {
	handler := &APIKeyHandler{}
	app := fiber.New()
	app.Delete("/api-keys/:id", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		c.Locals("user_id", "not-a-uuid") // Wrong type
		return handler.DeleteAPIKey(c)
	})

	req := httptest.NewRequest("DELETE", "/api-keys/"+uuid.New().String(), nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

func TestAPIKeyHandler_DeleteAPIKey_InvalidKeyID(t *testing.T) {
	handler := &APIKeyHandler{}
	app := fiber.New()
	app.Delete("/api-keys/:id", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		c.Locals("user_id", uuid.New())
		return handler.DeleteAPIKey(c)
	})

	req := httptest.NewRequest("DELETE", "/api-keys/not-a-uuid", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}
