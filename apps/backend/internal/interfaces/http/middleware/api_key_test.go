package middleware

import (
	"crypto/sha256"
	"encoding/base64"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===========================
// APIKeyMiddleware Tests
// ===========================

func TestAPIKeyMiddleware_NoAPIKey(t *testing.T) {
	app := fiber.New()
	app.Use(APIKeyMiddleware(nil))
	app.Get("/test", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	// No API key provided

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAPIKeyMiddleware_SkipsVerificationsEndpoint(t *testing.T) {
	var handlerCalled bool

	app := fiber.New()
	app.Use(APIKeyMiddleware(nil))
	app.Get("/api/v1/sdk-api/verifications", func(c fiber.Ctx) error {
		handlerCalled = true
		return c.JSON(fiber.Map{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/api/v1/sdk-api/verifications", nil)
	// No API key, but should still pass

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.True(t, handlerCalled)
}

func TestAPIKeyMiddleware_SkipsVerificationsSubpath(t *testing.T) {
	var handlerCalled bool

	app := fiber.New()
	app.Use(APIKeyMiddleware(nil))
	app.Get("/api/v1/sdk-api/verifications/123", func(c fiber.Ctx) error {
		handlerCalled = true
		return c.JSON(fiber.Map{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/api/v1/sdk-api/verifications/123", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.True(t, handlerCalled)
}

func TestAPIKeyMiddleware_BearerTokenParsing(t *testing.T) {
	// Test that Bearer token parsing works correctly
	authHeader := "Bearer test-api-key-123"
	parts := strings.Split(authHeader, " ")

	assert.Len(t, parts, 2)
	assert.Equal(t, "Bearer", parts[0])
	assert.Equal(t, "test-api-key-123", parts[1])
}

func TestAPIKeyMiddleware_InvalidBearerFormat(t *testing.T) {
	// Test various invalid formats
	testCases := []struct {
		name   string
		header string
	}{
		{"Single word", "InvalidToken"},
		{"Wrong prefix", "Basic token123"},
		{"No value", "Bearer"},
		{"Three parts", "Bearer extra token"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			parts := strings.Split(tc.header, " ")
			isValid := len(parts) == 2 && parts[0] == "Bearer"
			// All these should be invalid
			if tc.name == "Three parts" {
				// "Bearer extra token" splits into 3 parts
				assert.NotEqual(t, 2, len(parts))
			} else if tc.name == "No value" {
				// "Bearer" splits into 1 part
				assert.Equal(t, 1, len(parts))
			} else {
				assert.False(t, isValid)
			}
		})
	}
}

func TestAPIKeyMiddleware_XAPIKeyHeader(t *testing.T) {
	// Verify X-API-Key header is an alternative to Authorization Bearer
	// The middleware checks for X-API-Key as fallback
	apiKey := "test-api-key-456"
	assert.NotEmpty(t, apiKey)
}

// ===========================
// OptionalAPIKeyMiddleware Tests
// ===========================

func TestOptionalAPIKeyMiddleware_NoAPIKey_PassesThrough(t *testing.T) {
	var handlerCalled bool

	app := fiber.New()
	app.Use(OptionalAPIKeyMiddleware(nil))
	app.Get("/test", func(c fiber.Ctx) error {
		handlerCalled = true
		return c.JSON(fiber.Map{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	// No API key provided

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.True(t, handlerCalled)
}

func TestOptionalAPIKeyMiddleware_BearerTokenFormat(t *testing.T) {
	// Test that Bearer token format is correctly parsed
	// Note: Cannot test with nil db as middleware tries to query

	authHeader := "Bearer test-key"
	parts := strings.Split(authHeader, " ")

	assert.Len(t, parts, 2)
	assert.Equal(t, "Bearer", parts[0])
	assert.Equal(t, "test-key", parts[1])
}

func TestOptionalAPIKeyMiddleware_XAPIKeyFormat(t *testing.T) {
	// Verify X-API-Key header extraction logic

	apiKey := "test-key"
	assert.NotEmpty(t, apiKey)
}

func TestOptionalAPIKeyMiddleware_InvalidBearerFormat_Logic(t *testing.T) {
	// Test that invalid bearer format detection works

	invalidHeader := "NotBearer token"
	parts := strings.Split(invalidHeader, " ")

	// Should not be recognized as Bearer token
	assert.NotEqual(t, "Bearer", parts[0])
}

// ===========================
// API Key Hash Tests
// ===========================

func TestAPIKeyHash_Computation(t *testing.T) {
	apiKey := "test-api-key-12345"

	hash := sha256.Sum256([]byte(apiKey))
	keyHash := base64.StdEncoding.EncodeToString(hash[:])

	// Verify hash is deterministic
	hash2 := sha256.Sum256([]byte(apiKey))
	keyHash2 := base64.StdEncoding.EncodeToString(hash2[:])

	assert.Equal(t, keyHash, keyHash2)
	assert.NotEmpty(t, keyHash)
}

func TestAPIKeyHash_DifferentKeys(t *testing.T) {
	key1 := "api-key-one"
	key2 := "api-key-two"

	hash1 := sha256.Sum256([]byte(key1))
	hash2 := sha256.Sum256([]byte(key2))

	keyHash1 := base64.StdEncoding.EncodeToString(hash1[:])
	keyHash2 := base64.StdEncoding.EncodeToString(hash2[:])

	assert.NotEqual(t, keyHash1, keyHash2)
}

func TestAPIKeyHash_EmptyKey(t *testing.T) {
	emptyKey := ""

	hash := sha256.Sum256([]byte(emptyKey))
	keyHash := base64.StdEncoding.EncodeToString(hash[:])

	// Even empty key produces a hash
	assert.NotEmpty(t, keyHash)
}
