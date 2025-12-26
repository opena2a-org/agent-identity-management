package handlers

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function for SDK token tests that need user context
func withSDKTokenContext(handler func(c fiber.Ctx) error) fiber.Handler {
	return func(c fiber.Ctx) error {
		c.Locals("user_id", uuid.New())
		return handler(c)
	}
}

// ===========================
// NewSDKTokenHandler Tests
// ===========================

func TestNewSDKTokenHandler_NilDeps(t *testing.T) {
	handler := NewSDKTokenHandler(nil)
	assert.NotNil(t, handler)
}

// ===========================
// SDKTokenHandler.ListUserTokens Tests
// ===========================

func TestSDKTokenHandler_ListUserTokens_NoUserContext(t *testing.T) {
	handler := &SDKTokenHandler{}
	app := fiber.New()
	app.Get("/users/me/sdk-tokens", handler.ListUserTokens)

	req := httptest.NewRequest("GET", "/users/me/sdk-tokens", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestSDKTokenHandler_ListUserTokens_InvalidUserIDType(t *testing.T) {
	handler := &SDKTokenHandler{}
	app := fiber.New()
	app.Get("/users/me/sdk-tokens", func(c fiber.Ctx) error {
		c.Locals("user_id", "not-a-uuid") // Wrong type
		return handler.ListUserTokens(c)
	})

	req := httptest.NewRequest("GET", "/users/me/sdk-tokens", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

// ===========================
// SDKTokenHandler.GetActiveTokenCount Tests
// ===========================

func TestSDKTokenHandler_GetActiveTokenCount_NoUserContext(t *testing.T) {
	handler := &SDKTokenHandler{}
	app := fiber.New()
	app.Get("/users/me/sdk-tokens/count", handler.GetActiveTokenCount)

	req := httptest.NewRequest("GET", "/users/me/sdk-tokens/count", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

// ===========================
// SDKTokenHandler.RevokeToken Tests
// ===========================

func TestSDKTokenHandler_RevokeToken_NoUserContext(t *testing.T) {
	handler := &SDKTokenHandler{}
	app := fiber.New()
	app.Post("/users/me/sdk-tokens/:id/revoke", handler.RevokeToken)

	req := httptest.NewRequest("POST", "/users/me/sdk-tokens/"+uuid.New().String()+"/revoke", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestSDKTokenHandler_RevokeToken_InvalidTokenID(t *testing.T) {
	handler := &SDKTokenHandler{}
	app := fiber.New()
	app.Post("/users/me/sdk-tokens/:id/revoke", withSDKTokenContext(handler.RevokeToken))

	req := httptest.NewRequest("POST", "/users/me/sdk-tokens/not-a-uuid/revoke", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// SDKTokenHandler.RevokeAllTokens Tests
// ===========================

func TestSDKTokenHandler_RevokeAllTokens_NoUserContext(t *testing.T) {
	handler := &SDKTokenHandler{}
	app := fiber.New()
	app.Post("/users/me/sdk-tokens/revoke-all", handler.RevokeAllTokens)

	req := httptest.NewRequest("POST", "/users/me/sdk-tokens/revoke-all", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

// ===========================
// Struct Tests
// ===========================

func TestRevokeTokenRequest_Struct(t *testing.T) {
	req := RevokeTokenRequest{
		Reason: "Security concern - token may be compromised",
	}

	assert.Equal(t, "Security concern - token may be compromised", req.Reason)
}

func TestRevokeTokenRequest_EmptyReason(t *testing.T) {
	req := RevokeTokenRequest{}
	assert.Empty(t, req.Reason)
}

func TestRevokeTokenRequest_LongReason(t *testing.T) {
	req := RevokeTokenRequest{
		Reason: "This is a very long reason for revoking the token that explains in detail why we need to revoke it for security purposes and compliance requirements",
	}

	assert.NotEmpty(t, req.Reason)
	assert.Contains(t, req.Reason, "security")
}
