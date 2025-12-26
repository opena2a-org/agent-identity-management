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
// NewPasswordResetHandler Tests
// ===========================

func TestNewPasswordResetHandler_NilDeps(t *testing.T) {
	handler := NewPasswordResetHandler(nil, nil)
	assert.NotNil(t, handler)
}

// ===========================
// PasswordResetHandler.RequestPasswordReset Tests
// ===========================

func TestPasswordResetHandler_RequestPasswordReset_InvalidJSON(t *testing.T) {
	handler := &PasswordResetHandler{}
	app := fiber.New()
	app.Post("/auth/forgot-password", handler.RequestPasswordReset)

	req := httptest.NewRequest("POST", "/auth/forgot-password", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// PasswordResetHandler.VerifyResetToken Tests
// ===========================

func TestPasswordResetHandler_VerifyResetToken_MissingEmail(t *testing.T) {
	handler := &PasswordResetHandler{}
	app := fiber.New()
	app.Get("/auth/verify-reset-token", handler.VerifyResetToken)

	req := httptest.NewRequest("GET", "/auth/verify-reset-token?token=test", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestPasswordResetHandler_VerifyResetToken_MissingToken(t *testing.T) {
	handler := &PasswordResetHandler{}
	app := fiber.New()
	app.Get("/auth/verify-reset-token", handler.VerifyResetToken)

	req := httptest.NewRequest("GET", "/auth/verify-reset-token?email=test@example.com", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestPasswordResetHandler_VerifyResetToken_BothMissing(t *testing.T) {
	handler := &PasswordResetHandler{}
	app := fiber.New()
	app.Get("/auth/verify-reset-token", handler.VerifyResetToken)

	req := httptest.NewRequest("GET", "/auth/verify-reset-token", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// PasswordResetHandler.ResetPassword Tests
// ===========================

func TestPasswordResetHandler_ResetPassword_InvalidJSON(t *testing.T) {
	handler := &PasswordResetHandler{}
	app := fiber.New()
	app.Post("/auth/reset-password", handler.ResetPassword)

	req := httptest.NewRequest("POST", "/auth/reset-password", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestPasswordResetHandler_ResetPassword_ShortPassword(t *testing.T) {
	handler := &PasswordResetHandler{}
	app := fiber.New()
	app.Post("/auth/reset-password", handler.ResetPassword)

	// Password less than 8 characters
	body := `{"email":"test@example.com","token":"abc123","newPassword":"short"}`
	req := httptest.NewRequest("POST", "/auth/reset-password", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}
