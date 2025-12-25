package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===========================
// LoggerMiddleware Tests
// ===========================

func TestLoggerMiddleware_Success(t *testing.T) {
	app := fiber.New()
	app.Use(LoggerMiddleware())
	app.Get("/test", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)

	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestLoggerMiddleware_NotFound(t *testing.T) {
	app := fiber.New()
	app.Use(LoggerMiddleware())
	app.Get("/exists", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/not-found", nil)
	resp, err := app.Test(req)

	require.NoError(t, err)
	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

func TestLoggerMiddleware_DifferentMethods(t *testing.T) {
	app := fiber.New()
	app.Use(LoggerMiddleware())

	app.Get("/resource", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"method": "GET"})
	})
	app.Post("/resource", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"method": "POST"})
	})
	app.Put("/resource", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"method": "PUT"})
	})
	app.Delete("/resource", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"method": "DELETE"})
	})

	methods := []string{"GET", "POST", "PUT", "DELETE"}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/resource", nil)
			resp, err := app.Test(req)
			require.NoError(t, err)
			assert.Equal(t, fiber.StatusOK, resp.StatusCode)
		})
	}
}

func TestLoggerMiddleware_Error(t *testing.T) {
	app := fiber.New()
	app.Use(LoggerMiddleware())
	app.Get("/error", func(c fiber.Ctx) error {
		return fiber.NewError(fiber.StatusBadRequest, "bad request")
	})

	req := httptest.NewRequest("GET", "/error", nil)
	resp, err := app.Test(req)

	require.NoError(t, err)
	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestLoggerMiddleware_WithQueryParams(t *testing.T) {
	app := fiber.New()
	app.Use(LoggerMiddleware())
	app.Get("/search", func(c fiber.Ctx) error {
		query := c.Query("q")
		return c.JSON(fiber.Map{"query": query})
	})

	req := httptest.NewRequest("GET", "/search?q=test&page=1", nil)
	resp, err := app.Test(req)

	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestLoggerMiddleware_WithHeaders(t *testing.T) {
	app := fiber.New()
	app.Use(LoggerMiddleware())
	app.Get("/test", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Request-ID", "test-123")
	req.Header.Set("User-Agent", "test-agent")

	resp, err := app.Test(req)

	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestLoggerMiddleware_LongPath(t *testing.T) {
	app := fiber.New()
	app.Use(LoggerMiddleware())
	app.Get("/api/v1/organizations/:orgId/agents/:agentId/trust-scores", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/api/v1/organizations/123/agents/456/trust-scores", nil)
	resp, err := app.Test(req)

	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}
