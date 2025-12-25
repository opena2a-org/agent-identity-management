package middleware

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetClientIP_NoTrustedProxies(t *testing.T) {
	// Without trusted proxies configured, should use direct IP
	originalEnv := os.Getenv("TRUSTED_PROXIES")
	os.Unsetenv("TRUSTED_PROXIES")
	defer func() {
		if originalEnv != "" {
			os.Setenv("TRUSTED_PROXIES", originalEnv)
		}
	}()

	app := fiber.New()
	var capturedIP string

	app.Get("/test", func(c fiber.Ctx) error {
		capturedIP = getClientIP(c)
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Forwarded-For", "1.2.3.4, 5.6.7.8")
	req.Header.Set("X-Real-IP", "9.10.11.12")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should ignore X-Forwarded-For and X-Real-IP since no trusted proxies
	assert.NotEqual(t, "1.2.3.4", capturedIP)
	assert.NotEqual(t, "9.10.11.12", capturedIP)
}

func TestGetClientIP_WithTrustedProxy(t *testing.T) {
	// With trusted proxies, should respect X-Real-IP
	originalEnv := os.Getenv("TRUSTED_PROXIES")
	// Note: In test environment, the direct IP is typically "0.0.0.0"
	os.Setenv("TRUSTED_PROXIES", "*")
	defer func() {
		if originalEnv != "" {
			os.Setenv("TRUSTED_PROXIES", originalEnv)
		} else {
			os.Unsetenv("TRUSTED_PROXIES")
		}
	}()

	app := fiber.New()
	var capturedIP string

	app.Get("/test", func(c fiber.Ctx) error {
		capturedIP = getClientIP(c)
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Real-IP", "203.0.113.50")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should use X-Real-IP when trusted proxy is configured with wildcard
	assert.Equal(t, "203.0.113.50", capturedIP)
}

func TestGetClientIP_XForwardedFor(t *testing.T) {
	originalEnv := os.Getenv("TRUSTED_PROXIES")
	os.Setenv("TRUSTED_PROXIES", "*")
	defer func() {
		if originalEnv != "" {
			os.Setenv("TRUSTED_PROXIES", originalEnv)
		} else {
			os.Unsetenv("TRUSTED_PROXIES")
		}
	}()

	app := fiber.New()
	var capturedIP string

	app.Get("/test", func(c fiber.Ctx) error {
		capturedIP = getClientIP(c)
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Forwarded-For", "198.51.100.178, 192.0.2.1, 10.0.0.1")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should use first IP from X-Forwarded-For when X-Real-IP is not set
	assert.Equal(t, "198.51.100.178", capturedIP)
}

func TestRateLimitMiddleware_WithAuthenticatedUser(t *testing.T) {
	app := fiber.New()

	userID := uuid.New()

	// Add authentication simulation before rate limiter
	app.Use(func(c fiber.Ctx) error {
		c.Locals("user_id", userID)
		return c.Next()
	})
	app.Use(RateLimitMiddleware())
	app.Get("/test", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "success"})
	})

	// First request should succeed
	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestRateLimitMiddleware_WithoutAuthentication(t *testing.T) {
	app := fiber.New()
	app.Use(RateLimitMiddleware())
	app.Get("/test", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "success"})
	})

	// Request without authentication
	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestStrictRateLimitMiddleware_LimitReached(t *testing.T) {
	app := fiber.New()
	app.Use(StrictRateLimitMiddleware())
	app.Get("/test", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "success"})
	})

	// Make 11 requests (strict limit is 10 per minute)
	hitLimit := false
	for i := 0; i < 11; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)

		if resp.StatusCode == fiber.StatusTooManyRequests {
			hitLimit = true
			body, _ := io.ReadAll(resp.Body)
			var result map[string]interface{}
			json.Unmarshal(body, &result)
			assert.Contains(t, result["error"], "Rate limit exceeded")
			resp.Body.Close()
			break
		}
		resp.Body.Close()
	}

	assert.True(t, hitLimit, "Should hit rate limit after 10 requests")
}

func TestRateLimitMiddleware_RateLimitExceeded(t *testing.T) {
	app := fiber.New()
	// Create a new rate limiter with very low limit for testing
	app.Use(RateLimitMiddleware())
	app.Get("/test", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "success"})
	})

	// Make 101 requests (limit is 100 per minute)
	hitLimit := false
	for i := 0; i < 101; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)

		if resp.StatusCode == fiber.StatusTooManyRequests {
			hitLimit = true
			body, _ := io.ReadAll(resp.Body)
			var result map[string]interface{}
			json.Unmarshal(body, &result)
			assert.Contains(t, result["error"], "Rate limit exceeded")
			resp.Body.Close()
			break
		}
		resp.Body.Close()
	}

	assert.True(t, hitLimit, "Should hit rate limit after 100 requests")
}

func TestGetClientIP_SpecificTrustedProxy(t *testing.T) {
	originalEnv := os.Getenv("TRUSTED_PROXIES")
	os.Setenv("TRUSTED_PROXIES", "127.0.0.1, 10.0.0.1")
	defer func() {
		if originalEnv != "" {
			os.Setenv("TRUSTED_PROXIES", originalEnv)
		} else {
			os.Unsetenv("TRUSTED_PROXIES")
		}
	}()

	app := fiber.New()
	var capturedIP string

	app.Get("/test", func(c fiber.Ctx) error {
		capturedIP = getClientIP(c)
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	// Set some forwarded headers that should be ignored since direct IP isn't trusted
	req.Header.Set("X-Real-IP", "192.168.1.100")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Since the test connection doesn't come from 127.0.0.1 or 10.0.0.1,
	// headers should be ignored and direct IP should be used
	assert.NotEqual(t, "192.168.1.100", capturedIP)
}

func TestGetClientIP_XRealIPPreferredOverXForwardedFor(t *testing.T) {
	originalEnv := os.Getenv("TRUSTED_PROXIES")
	os.Setenv("TRUSTED_PROXIES", "*")
	defer func() {
		if originalEnv != "" {
			os.Setenv("TRUSTED_PROXIES", originalEnv)
		} else {
			os.Unsetenv("TRUSTED_PROXIES")
		}
	}()

	app := fiber.New()
	var capturedIP string

	app.Get("/test", func(c fiber.Ctx) error {
		capturedIP = getClientIP(c)
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Real-IP", "203.0.113.100")
	req.Header.Set("X-Forwarded-For", "198.51.100.1, 192.0.2.1")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// X-Real-IP should be preferred over X-Forwarded-For
	assert.Equal(t, "203.0.113.100", capturedIP)
}
