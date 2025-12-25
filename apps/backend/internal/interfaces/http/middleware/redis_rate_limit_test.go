package middleware

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===========================
// RateLimitConfig Tests
// ===========================

func TestRateLimitConfig_DefaultValues(t *testing.T) {
	config := RateLimitConfig{}

	assert.Equal(t, 0, config.DefaultMaxRequests)
	assert.Equal(t, time.Duration(0), config.DefaultWindow)
	assert.Equal(t, 0, config.StrictMaxRequests)
	assert.Equal(t, time.Duration(0), config.StrictWindow)
	assert.Nil(t, config.Cache)
}

func TestRateLimitConfig_WithValues(t *testing.T) {
	config := RateLimitConfig{
		DefaultMaxRequests: 100,
		DefaultWindow:      time.Minute,
		StrictMaxRequests:  10,
		StrictWindow:       time.Minute,
	}

	assert.Equal(t, 100, config.DefaultMaxRequests)
	assert.Equal(t, time.Minute, config.DefaultWindow)
	assert.Equal(t, 10, config.StrictMaxRequests)
	assert.Equal(t, time.Minute, config.StrictWindow)
}

// ===========================
// RateLimitInfo Tests
// ===========================

func TestRateLimitInfo_DefaultValues(t *testing.T) {
	info := RateLimitInfo{}

	assert.Equal(t, int64(0), info.Limit)
	assert.Equal(t, int64(0), info.Remaining)
	assert.True(t, info.Reset.IsZero())
}

func TestRateLimitInfo_WithValues(t *testing.T) {
	resetTime := time.Now().Add(time.Minute)
	info := RateLimitInfo{
		Limit:     100,
		Remaining: 50,
		Reset:     resetTime,
	}

	assert.Equal(t, int64(100), info.Limit)
	assert.Equal(t, int64(50), info.Remaining)
	assert.Equal(t, resetTime, info.Reset)
}

// ===========================
// DefaultRateLimitConfig Tests
// ===========================

func TestDefaultRateLimitConfig_DefaultEnvValues(t *testing.T) {
	// Test with default environment (no env vars set)
	config := DefaultRateLimitConfig()

	assert.Equal(t, 100, config.DefaultMaxRequests)
	assert.Equal(t, 60*time.Second, config.DefaultWindow)
	assert.Equal(t, 10, config.StrictMaxRequests)
	assert.Equal(t, 60*time.Second, config.StrictWindow)
}

func TestDefaultRateLimitConfig_CustomEnvValues(t *testing.T) {
	// Set custom environment variables
	t.Setenv("RATE_LIMIT_DEFAULT_MAX", "200")
	t.Setenv("RATE_LIMIT_DEFAULT_WINDOW_SEC", "120")
	t.Setenv("RATE_LIMIT_STRICT_MAX", "20")
	t.Setenv("RATE_LIMIT_STRICT_WINDOW_SEC", "30")

	config := DefaultRateLimitConfig()

	assert.Equal(t, 200, config.DefaultMaxRequests)
	assert.Equal(t, 120*time.Second, config.DefaultWindow)
	assert.Equal(t, 20, config.StrictMaxRequests)
	assert.Equal(t, 30*time.Second, config.StrictWindow)
}

func TestDefaultRateLimitConfig_InvalidEnvValues(t *testing.T) {
	// Set invalid environment variables (non-numeric)
	t.Setenv("RATE_LIMIT_DEFAULT_MAX", "invalid")
	t.Setenv("RATE_LIMIT_DEFAULT_WINDOW_SEC", "invalid")

	config := DefaultRateLimitConfig()

	// Should fall back to 0 when parsing fails
	assert.Equal(t, 0, config.DefaultMaxRequests)
	assert.Equal(t, time.Duration(0), config.DefaultWindow)
}

// ===========================
// getEnvDefault Tests
// ===========================

func TestGetEnvDefault_EnvVarSet(t *testing.T) {
	t.Setenv("TEST_VAR", "custom_value")

	result := getEnvDefault("TEST_VAR", "default_value")

	assert.Equal(t, "custom_value", result)
}

func TestGetEnvDefault_EnvVarNotSet(t *testing.T) {
	// Use a unique key that definitely isn't set
	result := getEnvDefault("DEFINITELY_NOT_SET_XYZ123", "default_value")

	assert.Equal(t, "default_value", result)
}

func TestGetEnvDefault_EmptyEnvVar(t *testing.T) {
	t.Setenv("EMPTY_VAR", "")

	result := getEnvDefault("EMPTY_VAR", "default_value")

	// Empty string should use default
	assert.Equal(t, "default_value", result)
}

// ===========================
// NewRedisRateLimiter Tests
// ===========================

func TestNewRedisRateLimiter_WithConfig(t *testing.T) {
	config := &RateLimitConfig{
		DefaultMaxRequests: 100,
		DefaultWindow:      time.Minute,
	}

	limiter := NewRedisRateLimiter(config)

	assert.NotNil(t, limiter)
	assert.Equal(t, config, limiter.config)
}

func TestNewRedisRateLimiter_NilCache(t *testing.T) {
	config := &RateLimitConfig{
		DefaultMaxRequests: 100,
		DefaultWindow:      time.Minute,
		Cache:              nil,
	}

	limiter := NewRedisRateLimiter(config)

	assert.NotNil(t, limiter)
	assert.Nil(t, limiter.config.Cache)
}

// ===========================
// Middleware with nil cache Tests
// ===========================

func TestRedisRateLimiter_Middleware_NilCache(t *testing.T) {
	config := &RateLimitConfig{
		DefaultMaxRequests: 100,
		DefaultWindow:      time.Minute,
		Cache:              nil, // No Redis
	}

	limiter := NewRedisRateLimiter(config)
	var handlerCalled bool

	app := fiber.New()
	app.Use(limiter.Middleware())
	app.Get("/test", func(c fiber.Ctx) error {
		handlerCalled = true
		return c.JSON(fiber.Map{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should pass through when cache is nil
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.True(t, handlerCalled)
}

func TestRedisRateLimiter_StrictMiddleware_NilCache(t *testing.T) {
	config := &RateLimitConfig{
		StrictMaxRequests: 10,
		StrictWindow:      time.Minute,
		Cache:             nil, // No Redis
	}

	limiter := NewRedisRateLimiter(config)
	var handlerCalled bool

	app := fiber.New()
	app.Use(limiter.StrictMiddleware())
	app.Get("/test", func(c fiber.Ctx) error {
		handlerCalled = true
		return c.JSON(fiber.Map{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should pass through when cache is nil
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.True(t, handlerCalled)
}

// ===========================
// getKey Tests
// ===========================

func TestRedisRateLimiter_GetKey_WithUserID(t *testing.T) {
	config := &RateLimitConfig{}
	limiter := NewRedisRateLimiter(config)

	userID := uuid.New()

	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Locals("user_id", userID)
		return c.Next()
	})
	app.Use(limiter.Middleware())
	app.Get("/test", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestRedisRateLimiter_GetKey_WithOrgID(t *testing.T) {
	config := &RateLimitConfig{}
	limiter := NewRedisRateLimiter(config)

	orgID := uuid.New()

	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		return c.Next()
	})
	app.Use(limiter.Middleware())
	app.Get("/test", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestRedisRateLimiter_GetKey_FallsBackToIP(t *testing.T) {
	config := &RateLimitConfig{}
	limiter := NewRedisRateLimiter(config)

	app := fiber.New()
	app.Use(limiter.Middleware())
	app.Get("/test", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	// No user_id or organization_id set
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

// ===========================
// OrganizationLimitMiddleware Tests
// ===========================

func TestRedisRateLimiter_OrganizationLimitMiddleware_NilCache(t *testing.T) {
	config := &RateLimitConfig{
		DefaultMaxRequests: 100,
		DefaultWindow:      time.Minute,
		Cache:              nil,
	}

	limiter := NewRedisRateLimiter(config)

	getOrgLimit := func(orgID uuid.UUID) (int, time.Duration) {
		return 50, 30 * time.Second
	}

	var handlerCalled bool

	app := fiber.New()
	app.Use(limiter.OrganizationLimitMiddleware(getOrgLimit))
	app.Get("/test", func(c fiber.Ctx) error {
		handlerCalled = true
		return c.JSON(fiber.Map{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.True(t, handlerCalled)
}

func TestRedisRateLimiter_OrganizationLimitMiddleware_WithOrgID(t *testing.T) {
	config := &RateLimitConfig{
		DefaultMaxRequests: 100,
		DefaultWindow:      time.Minute,
		Cache:              nil,
	}

	limiter := NewRedisRateLimiter(config)
	orgID := uuid.New()

	getOrgLimit := func(id uuid.UUID) (int, time.Duration) {
		if id == orgID {
			return 200, 2 * time.Minute
		}
		return 0, 0
	}

	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		return c.Next()
	})
	app.Use(limiter.OrganizationLimitMiddleware(getOrgLimit))
	app.Get("/test", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestRedisRateLimiter_OrganizationLimitMiddleware_NoOrgLimit(t *testing.T) {
	config := &RateLimitConfig{
		DefaultMaxRequests: 100,
		DefaultWindow:      time.Minute,
		Cache:              nil,
	}

	limiter := NewRedisRateLimiter(config)
	orgID := uuid.New()

	getOrgLimit := func(id uuid.UUID) (int, time.Duration) {
		// Return 0 to use defaults
		return 0, 0
	}

	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		return c.Next()
	})
	app.Use(limiter.OrganizationLimitMiddleware(getOrgLimit))
	app.Get("/test", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}
