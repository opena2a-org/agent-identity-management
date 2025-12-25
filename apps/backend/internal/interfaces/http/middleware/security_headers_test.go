package middleware

import (
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecurityHeadersMiddleware_SetsAllHeaders(t *testing.T) {
	app := fiber.New()
	app.Use(SecurityHeadersMiddleware())
	app.Get("/test", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	// Check all security headers are set
	assert.Equal(t, "nosniff", resp.Header.Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", resp.Header.Get("X-Frame-Options"))
	assert.Equal(t, "1; mode=block", resp.Header.Get("X-XSS-Protection"))
	assert.Equal(t, "strict-origin-when-cross-origin", resp.Header.Get("Referrer-Policy"))
	assert.Contains(t, resp.Header.Get("Permissions-Policy"), "accelerometer=()")
	assert.Contains(t, resp.Header.Get("Permissions-Policy"), "camera=()")
	assert.Contains(t, resp.Header.Get("Permissions-Policy"), "microphone=()")
}

func TestSecurityHeadersMiddleware_SetsCacheControl(t *testing.T) {
	app := fiber.New()
	app.Use(SecurityHeadersMiddleware())
	app.Get("/test", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	cacheControl := resp.Header.Get("Cache-Control")
	assert.Contains(t, cacheControl, "no-store")
	assert.Contains(t, cacheControl, "no-cache")
	assert.Contains(t, cacheControl, "must-revalidate")

	assert.Equal(t, "no-cache", resp.Header.Get("Pragma"))
	assert.Equal(t, "0", resp.Header.Get("Expires"))
}

func TestSecurityHeadersMiddleware_SetsCSP(t *testing.T) {
	app := fiber.New()
	app.Use(SecurityHeadersMiddleware())
	app.Get("/test", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	csp := resp.Header.Get("Content-Security-Policy")
	assert.Contains(t, csp, "default-src 'self'")
	assert.Contains(t, csp, "frame-ancestors 'none'")
	assert.Contains(t, csp, "form-action 'self'")
}

func TestSecurityHeadersMiddleware_NoHSTSInDevelopment(t *testing.T) {
	// Ensure we're in development mode
	originalEnv := os.Getenv("ENVIRONMENT")
	os.Setenv("ENVIRONMENT", "development")
	defer os.Setenv("ENVIRONMENT", originalEnv)

	app := fiber.New()
	app.Use(SecurityHeadersMiddleware())
	app.Get("/test", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// HSTS should NOT be set in development
	assert.Empty(t, resp.Header.Get("Strict-Transport-Security"))
}

func TestSecurityHeadersMiddleware_HSTSInProduction(t *testing.T) {
	// Set production environment
	originalEnv := os.Getenv("ENVIRONMENT")
	os.Setenv("ENVIRONMENT", "production")
	defer os.Setenv("ENVIRONMENT", originalEnv)

	app := fiber.New()
	app.Use(SecurityHeadersMiddleware())
	app.Get("/test", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	hsts := resp.Header.Get("Strict-Transport-Security")
	assert.NotEmpty(t, hsts)
	assert.Contains(t, hsts, "max-age=31536000")
	assert.Contains(t, hsts, "includeSubDomains")
	assert.Contains(t, hsts, "preload")
}

func TestAPISecurityHeadersMiddleware_SetsHeaders(t *testing.T) {
	app := fiber.New()
	app.Use(APISecurityHeadersMiddleware())
	app.Get("/api/test", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/api/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	// Check security headers
	assert.Equal(t, "nosniff", resp.Header.Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", resp.Header.Get("X-Frame-Options"))
	assert.Equal(t, "1; mode=block", resp.Header.Get("X-XSS-Protection"))
	assert.Equal(t, "strict-origin-when-cross-origin", resp.Header.Get("Referrer-Policy"))
}

func TestAPISecurityHeadersMiddleware_SetsCacheControl(t *testing.T) {
	app := fiber.New()
	app.Use(APISecurityHeadersMiddleware())
	app.Get("/api/test", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/api/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	cacheControl := resp.Header.Get("Cache-Control")
	assert.Contains(t, cacheControl, "no-store")
	assert.Contains(t, cacheControl, "no-cache")

	assert.Equal(t, "no-cache", resp.Header.Get("Pragma"))
}

func TestAPISecurityHeadersMiddleware_NoHSTSInDevelopment(t *testing.T) {
	originalEnv := os.Getenv("ENVIRONMENT")
	os.Setenv("ENVIRONMENT", "development")
	defer os.Setenv("ENVIRONMENT", originalEnv)

	app := fiber.New()
	app.Use(APISecurityHeadersMiddleware())
	app.Get("/api/test", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/api/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Empty(t, resp.Header.Get("Strict-Transport-Security"))
}

func TestAPISecurityHeadersMiddleware_HSTSInProduction(t *testing.T) {
	originalEnv := os.Getenv("ENVIRONMENT")
	os.Setenv("ENVIRONMENT", "production")
	defer os.Setenv("ENVIRONMENT", originalEnv)

	app := fiber.New()
	app.Use(APISecurityHeadersMiddleware())
	app.Get("/api/test", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/api/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	hsts := resp.Header.Get("Strict-Transport-Security")
	assert.NotEmpty(t, hsts)
	assert.Contains(t, hsts, "max-age=31536000")
	assert.Contains(t, hsts, "includeSubDomains")
}
