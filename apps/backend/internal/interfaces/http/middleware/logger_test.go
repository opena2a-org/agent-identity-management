package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
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

// ===========================
// Bootstrap token redaction
// ===========================

func TestRedactBootstrapTokens(t *testing.T) {
	token, _, _, err := domain.GenerateBootstrapToken()
	require.NoError(t, err)

	assert.Equal(t, "/api/v1/agents", RedactBootstrapTokens("/api/v1/agents"))
	redacted := RedactBootstrapTokens("/x/" + token + "/y/" + token)
	assert.Equal(t, "/x/aim_ob_[REDACTED]/y/aim_ob_[REDACTED]", redacted)
	assert.NotContains(t, redacted, token[len(domain.BootstrapTokenPrefix):])
}

// The request log line never carries a bootstrap token: not from the header,
// not from the JSON body, not from a query string, and not from the path.
func TestLoggerMiddleware_NeverLogsABootstrapToken(t *testing.T) {
	token, _, _, err := domain.GenerateBootstrapToken()
	require.NoError(t, err)
	secret := token[len(domain.BootstrapTokenPrefix):]

	var logged bytes.Buffer
	app := fiber.New()
	app.Use(newLoggerMiddleware(&logged))
	app.Post("/api/v1/onboarding/bootstrap-tokens/exchange", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusCreated)
	})
	app.Get("/api/v1/onboarding/*", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusNotFound)
	})

	requests := []*http.Request{}

	inHeader := httptest.NewRequest(http.MethodPost, "/api/v1/onboarding/bootstrap-tokens/exchange", nil)
	inHeader.Header.Set("X-AIM-Bootstrap-Token", token)
	requests = append(requests, inHeader)

	inBody := httptest.NewRequest(http.MethodPost, "/api/v1/onboarding/bootstrap-tokens/exchange",
		strings.NewReader(`{"bootstrapToken":"`+token+`"}`))
	inBody.Header.Set("Content-Type", "application/json")
	requests = append(requests, inBody)

	requests = append(requests,
		httptest.NewRequest(http.MethodPost, "/api/v1/onboarding/bootstrap-tokens/exchange?token="+token, nil),
		httptest.NewRequest(http.MethodGet, "/api/v1/onboarding/bootstrap-tokens/"+token, nil),
	)

	for _, req := range requests {
		resp, err := app.Test(req)
		require.NoError(t, err)
		_ = resp.Body.Close()
	}

	out := logged.String()
	require.Equal(t, len(requests), strings.Count(out, "\n"), "one line per request:\n%s", out)
	assert.Contains(t, out, "/api/v1/onboarding/bootstrap-tokens/exchange")
	assert.Contains(t, out, "/api/v1/onboarding/bootstrap-tokens/aim_ob_[REDACTED]")
	assert.NotContains(t, out, secret)
	assert.NotContains(t, out, secret[:12])
}
