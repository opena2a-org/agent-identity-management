package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// authTestErrorHandler is a custom error handler that preserves already-sent responses
func authTestErrorHandler(c fiber.Ctx, err error) error {
	if errors.Is(err, ErrUnauthorized) {
		return nil
	}
	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
		"error": err.Error(),
	})
}

// newAuthTestApp creates a Fiber app with the auth test error handler
func newAuthTestApp() *fiber.App {
	return fiber.New(fiber.Config{
		ErrorHandler: authTestErrorHandler,
	})
}

// ===========================
// NewAuthHandler Tests
// ===========================

func TestNewAuthHandler_NilDeps(t *testing.T) {
	handler := NewAuthHandler(nil, nil, nil, nil)
	assert.NotNil(t, handler)
}

// ===========================
// AuthHandler.Me Tests - Calling actual handler
// ===========================

func TestAuthHandler_Me_NoUserContext(t *testing.T) {
	handler := &AuthHandler{}
	app := newAuthTestApp()
	app.Get("/me", handler.Me)

	req := httptest.NewRequest("GET", "/me", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)
	assert.Contains(t, result["error"], "Unauthorized")
}

func TestAuthHandler_Me_InvalidUserIDType(t *testing.T) {
	handler := &AuthHandler{}
	app := newAuthTestApp()
	app.Get("/me", func(c fiber.Ctx) error {
		c.Locals("user_id", "not-a-uuid") // Invalid type
		return handler.Me(c)
	})

	req := httptest.NewRequest("GET", "/me", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

// ===========================
// AuthHandler.LocalLogin Tests - Calling actual handler
// ===========================

func TestAuthHandler_LocalLogin_InvalidJSON(t *testing.T) {
	handler := &AuthHandler{}
	app := fiber.New()
	app.Post("/login", handler.LocalLogin)

	req := httptest.NewRequest("POST", "/login", strings.NewReader("not valid json"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestAuthHandler_LocalLogin_EmptyBody(t *testing.T) {
	handler := &AuthHandler{}
	app := fiber.New()
	app.Post("/login", handler.LocalLogin)

	req := httptest.NewRequest("POST", "/login", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestAuthHandler_LocalLogin_MissingPassword(t *testing.T) {
	handler := &AuthHandler{}
	app := fiber.New()
	app.Post("/login", handler.LocalLogin)

	req := httptest.NewRequest("POST", "/login", strings.NewReader(`{"email":"test@example.com"}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestAuthHandler_LocalLogin_MissingEmail(t *testing.T) {
	handler := &AuthHandler{}
	app := fiber.New()
	app.Post("/login", handler.LocalLogin)

	req := httptest.NewRequest("POST", "/login", strings.NewReader(`{"password":"password123"}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestAuthHandler_LocalLogin_EmptyEmail(t *testing.T) {
	handler := &AuthHandler{}
	app := fiber.New()
	app.Post("/login", handler.LocalLogin)

	req := httptest.NewRequest("POST", "/login", strings.NewReader(`{"email":"","password":"password123"}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestAuthHandler_LocalLogin_EmptyPassword(t *testing.T) {
	handler := &AuthHandler{}
	app := fiber.New()
	app.Post("/login", handler.LocalLogin)

	req := httptest.NewRequest("POST", "/login", strings.NewReader(`{"email":"test@example.com","password":""}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// AuthHandler.ChangePassword Tests - Calling actual handler
// ===========================

func TestAuthHandler_ChangePassword_InvalidJSON(t *testing.T) {
	handler := &AuthHandler{}
	app := fiber.New()
	app.Post("/change-password", handler.ChangePassword)

	req := httptest.NewRequest("POST", "/change-password", strings.NewReader("not valid json"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestAuthHandler_ChangePassword_EmptyCurrentPassword(t *testing.T) {
	handler := &AuthHandler{}
	app := fiber.New()
	app.Post("/change-password", handler.ChangePassword)

	req := httptest.NewRequest("POST", "/change-password", strings.NewReader(`{"currentPassword":"","newPassword":"newpass"}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestAuthHandler_ChangePassword_EmptyNewPassword(t *testing.T) {
	handler := &AuthHandler{}
	app := fiber.New()
	app.Post("/change-password", handler.ChangePassword)

	req := httptest.NewRequest("POST", "/change-password", strings.NewReader(`{"currentPassword":"oldpass","newPassword":""}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestAuthHandler_ChangePassword_NoUserContext(t *testing.T) {
	handler := &AuthHandler{}
	app := newAuthTestApp()
	app.Post("/change-password", handler.ChangePassword)

	req := httptest.NewRequest("POST", "/change-password", strings.NewReader(`{"currentPassword":"old","newPassword":"new"}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAuthHandler_ChangePassword_InvalidUserContext(t *testing.T) {
	handler := &AuthHandler{}
	app := newAuthTestApp()
	app.Post("/change-password", func(c fiber.Ctx) error {
		c.Locals("user_id", "not-a-uuid")
		return handler.ChangePassword(c)
	})

	req := httptest.NewRequest("POST", "/change-password", strings.NewReader(`{"currentPassword":"old","newPassword":"new"}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

// ===========================
// AuthHandler.GetCurrentOrganization Tests - Calling actual handler
// ===========================

func TestAuthHandler_GetCurrentOrganization_NoOrgContext(t *testing.T) {
	handler := &AuthHandler{}
	app := newAuthTestApp()
	app.Get("/organization", handler.GetCurrentOrganization)

	req := httptest.NewRequest("GET", "/organization", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAuthHandler_GetCurrentOrganization_InvalidOrgContext(t *testing.T) {
	handler := &AuthHandler{}
	app := newAuthTestApp()
	app.Get("/organization", func(c fiber.Ctx) error {
		c.Locals("organization_id", "not-a-uuid")
		return handler.GetCurrentOrganization(c)
	})

	req := httptest.NewRequest("GET", "/organization", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

// ===========================
// AuthHandler.Logout Tests - Calling actual handler
// ===========================

func TestAuthHandler_Logout_Success(t *testing.T) {
	handler := &AuthHandler{}
	app := fiber.New()
	app.Post("/logout", handler.Logout)

	req := httptest.NewRequest("POST", "/logout", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)
	assert.Equal(t, "Logged out successfully", result["message"])
}

func TestAuthHandler_Logout_WithNilUserID(t *testing.T) {
	// Test logout when user_id is nil UUID (not logged in)
	handler := &AuthHandler{}
	app := fiber.New()
	app.Post("/logout", func(c fiber.Ctx) error {
		c.Locals("user_id", uuid.Nil)
		c.Locals("organization_id", uuid.Nil)
		return handler.Logout(c)
	})

	req := httptest.NewRequest("POST", "/logout", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

