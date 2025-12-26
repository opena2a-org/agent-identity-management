package handlers

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===========================
// NewAuthRefreshHandler Tests
// ===========================

func TestNewAuthRefreshHandler_NilDeps(t *testing.T) {
	handler := NewAuthRefreshHandler(nil, nil)
	assert.NotNil(t, handler)
}

// ===========================
// AuthRefreshHandler.RefreshToken Tests
// ===========================

func TestAuthRefreshHandler_RefreshToken_InvalidJSON(t *testing.T) {
	handler := &AuthRefreshHandler{}
	app := fiber.New()
	app.Post("/auth/refresh", handler.RefreshToken)

	req := httptest.NewRequest("POST", "/auth/refresh", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestAuthRefreshHandler_RefreshToken_MissingToken(t *testing.T) {
	handler := &AuthRefreshHandler{}
	app := fiber.New()
	app.Post("/auth/refresh", handler.RefreshToken)

	body := `{}`
	req := httptest.NewRequest("POST", "/auth/refresh", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestAuthRefreshHandler_RefreshToken_EmptyToken(t *testing.T) {
	handler := &AuthRefreshHandler{}
	app := fiber.New()
	app.Post("/auth/refresh", handler.RefreshToken)

	body := `{"refreshToken":""}`
	req := httptest.NewRequest("POST", "/auth/refresh", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// Struct Tests
// ===========================

func TestRefreshTokenRequest_Struct(t *testing.T) {
	req := RefreshTokenRequest{
		RefreshToken: "test-refresh-token-123",
	}

	assert.Equal(t, "test-refresh-token-123", req.RefreshToken)
}

func TestRefreshTokenRequest_EmptyStruct(t *testing.T) {
	req := RefreshTokenRequest{}
	assert.Empty(t, req.RefreshToken)
}

func TestRefreshTokenResponse_Struct(t *testing.T) {
	resp := RefreshTokenResponse{
		AccessToken:  "access-token-abc",
		RefreshToken: "refresh-token-xyz",
		TokenType:    "Bearer",
		ExpiresIn:    900,
	}

	assert.Equal(t, "access-token-abc", resp.AccessToken)
	assert.Equal(t, "refresh-token-xyz", resp.RefreshToken)
	assert.Equal(t, "Bearer", resp.TokenType)
	assert.Equal(t, 900, resp.ExpiresIn)
}

func TestRefreshTokenResponse_DefaultValues(t *testing.T) {
	resp := RefreshTokenResponse{}
	assert.Empty(t, resp.AccessToken)
	assert.Empty(t, resp.RefreshToken)
	assert.Empty(t, resp.TokenType)
	assert.Equal(t, 0, resp.ExpiresIn)
}
