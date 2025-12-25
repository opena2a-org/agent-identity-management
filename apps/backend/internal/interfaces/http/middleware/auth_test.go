package middleware

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/infrastructure/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthMiddleware_NoToken(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret-key-for-testing-purposes-32chars")
	jwtService := auth.NewJWTService()

	app := fiber.New()
	app.Use(AuthMiddleware(jwtService))
	app.Get("/protected", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)
	assert.Equal(t, "No authentication token provided", result["error"])
}

func TestAuthMiddleware_InvalidHeaderFormat(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret-key-for-testing-purposes-32chars")
	jwtService := auth.NewJWTService()

	app := fiber.New()
	app.Use(AuthMiddleware(jwtService))
	app.Get("/protected", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "InvalidFormat token123")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)
	assert.Equal(t, "Invalid authorization header format", result["error"])
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret-key-for-testing-purposes-32chars")
	jwtService := auth.NewJWTService()

	app := fiber.New()
	app.Use(AuthMiddleware(jwtService))
	app.Get("/protected", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)
	assert.Equal(t, "Invalid or expired token", result["error"])
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret-key-for-testing-purposes-32chars")
	jwtService := auth.NewJWTService()

	userID := uuid.New()
	orgID := uuid.New()

	// Generate valid token
	accessToken, _, err := jwtService.GenerateTokenPair(
		userID.String(),
		orgID.String(),
		"test@example.com",
		"admin",
	)
	require.NoError(t, err)

	var capturedUserID uuid.UUID
	var capturedOrgID uuid.UUID
	var capturedRole string

	app := fiber.New()
	app.Use(AuthMiddleware(jwtService))
	app.Get("/protected", func(c fiber.Ctx) error {
		capturedUserID = c.Locals("user_id").(uuid.UUID)
		capturedOrgID = c.Locals("organization_id").(uuid.UUID)
		capturedRole = c.Locals("role").(string)
		return c.JSON(fiber.Map{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Equal(t, userID, capturedUserID)
	assert.Equal(t, orgID, capturedOrgID)
	assert.Equal(t, "admin", capturedRole)
}

func TestAuthMiddleware_TokenFromCookie(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret-key-for-testing-purposes-32chars")
	jwtService := auth.NewJWTService()

	userID := uuid.New()
	orgID := uuid.New()

	accessToken, _, err := jwtService.GenerateTokenPair(
		userID.String(),
		orgID.String(),
		"test@example.com",
		"member",
	)
	require.NoError(t, err)

	var capturedUserID uuid.UUID

	app := fiber.New()
	app.Use(AuthMiddleware(jwtService))
	app.Get("/protected", func(c fiber.Ctx) error {
		capturedUserID = c.Locals("user_id").(uuid.UUID)
		return c.JSON(fiber.Map{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: accessToken})

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Equal(t, userID, capturedUserID)
}

func TestAuthMiddleware_SkipsIfEd25519Authenticated(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret-key-for-testing-purposes-32chars")
	jwtService := auth.NewJWTService()

	app := fiber.New()
	// Simulate Ed25519 authentication before AuthMiddleware
	app.Use(func(c fiber.Ctx) error {
		c.Locals("authenticated_via", "ed25519")
		c.Locals("user_id", uuid.New())
		c.Locals("organization_id", uuid.New())
		return c.Next()
	})
	app.Use(AuthMiddleware(jwtService))
	app.Get("/protected", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "success"})
	})

	// No token provided, but should pass because already authenticated
	req := httptest.NewRequest("GET", "/protected", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestAuthMiddleware_SkipsIfAPIKeyAuthenticated(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret-key-for-testing-purposes-32chars")
	jwtService := auth.NewJWTService()

	app := fiber.New()
	// Simulate API key authentication before AuthMiddleware
	app.Use(func(c fiber.Ctx) error {
		c.Locals("auth_method", "api_key")
		c.Locals("user_id", uuid.New())
		c.Locals("organization_id", uuid.New())
		return c.Next()
	})
	app.Use(AuthMiddleware(jwtService))
	app.Get("/protected", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestOptionalAuthMiddleware_NoToken(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret-key-for-testing-purposes-32chars")
	jwtService := auth.NewJWTService()

	var hasUserID bool

	app := fiber.New()
	app.Use(OptionalAuthMiddleware(jwtService))
	app.Get("/public", func(c fiber.Ctx) error {
		hasUserID = c.Locals("user_id") != nil
		return c.JSON(fiber.Map{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/public", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.False(t, hasUserID, "user_id should not be set without token")
}

func TestOptionalAuthMiddleware_InvalidToken(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret-key-for-testing-purposes-32chars")
	jwtService := auth.NewJWTService()

	var hasUserID bool

	app := fiber.New()
	app.Use(OptionalAuthMiddleware(jwtService))
	app.Get("/public", func(c fiber.Ctx) error {
		hasUserID = c.Locals("user_id") != nil
		return c.JSON(fiber.Map{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/public", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should succeed but without user context
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.False(t, hasUserID, "user_id should not be set with invalid token")
}

func TestOptionalAuthMiddleware_ValidToken(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret-key-for-testing-purposes-32chars")
	jwtService := auth.NewJWTService()

	userID := uuid.New()
	orgID := uuid.New()

	accessToken, _, err := jwtService.GenerateTokenPair(
		userID.String(),
		orgID.String(),
		"test@example.com",
		"admin",
	)
	require.NoError(t, err)

	var capturedUserID uuid.UUID
	var hasUserID bool

	app := fiber.New()
	app.Use(OptionalAuthMiddleware(jwtService))
	app.Get("/public", func(c fiber.Ctx) error {
		if uid := c.Locals("user_id"); uid != nil {
			capturedUserID = uid.(uuid.UUID)
			hasUserID = true
		}
		return c.JSON(fiber.Map{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/public", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.True(t, hasUserID, "user_id should be set with valid token")
	assert.Equal(t, userID, capturedUserID)
}
