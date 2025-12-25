package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===========================
// RecoveryMiddleware Tests
// ===========================

func TestRecoveryMiddleware_NoPanic(t *testing.T) {
	app := fiber.New()
	app.Use(RecoveryMiddleware())
	app.Get("/test", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)

	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestRecoveryMiddleware_RecoversPanic(t *testing.T) {
	app := fiber.New()
	app.Use(RecoveryMiddleware())
	app.Get("/panic", func(c fiber.Ctx) error {
		panic("test panic")
	})

	req := httptest.NewRequest("GET", "/panic", nil)
	resp, err := app.Test(req)

	require.NoError(t, err)
	// Recovery middleware should catch the panic and return 500
	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

func TestRecoveryMiddleware_RecoversNilPanic(t *testing.T) {
	app := fiber.New()
	app.Use(RecoveryMiddleware())
	app.Get("/nil-panic", func(c fiber.Ctx) error {
		panic(nil)
	})

	req := httptest.NewRequest("GET", "/nil-panic", nil)
	resp, err := app.Test(req)

	require.NoError(t, err)
	// Recovery should still work with nil panic
	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

func TestRecoveryMiddleware_RecoversErrorPanic(t *testing.T) {
	app := fiber.New()
	app.Use(RecoveryMiddleware())
	app.Get("/error-panic", func(c fiber.Ctx) error {
		panic(fiber.NewError(fiber.StatusBadRequest, "bad request"))
	})

	req := httptest.NewRequest("GET", "/error-panic", nil)
	resp, err := app.Test(req)

	require.NoError(t, err)
	// Fiber recover middleware returns the error code from fiber.Error panics
	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestRecoveryMiddleware_MultiplePaths(t *testing.T) {
	app := fiber.New()
	app.Use(RecoveryMiddleware())

	app.Get("/ok", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	app.Get("/panic", func(c fiber.Ctx) error {
		panic("boom")
	})

	// First request - OK
	req1 := httptest.NewRequest("GET", "/ok", nil)
	resp1, err := app.Test(req1)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp1.StatusCode)

	// Second request - Panic (should be recovered)
	req2 := httptest.NewRequest("GET", "/panic", nil)
	resp2, err := app.Test(req2)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusInternalServerError, resp2.StatusCode)

	// Third request - OK (app should still work)
	req3 := httptest.NewRequest("GET", "/ok", nil)
	resp3, err := app.Test(req3)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp3.StatusCode)
}
