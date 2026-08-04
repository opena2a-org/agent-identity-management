package metrics

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mountMetrics builds an app that gates a 200-returning /metrics stand-in with
// MetricsAuthMiddleware(token). A downstream 200 means the guard let the request
// through; 401 means it blocked. Using a stand-in keeps the assertions about the
// guard, not the Prometheus encoder.
func mountMetrics(token string) *fiber.App {
	app := fiber.New()
	app.Get("/metrics", MetricsAuthMiddleware(token), func(c fiber.Ctx) error {
		return c.SendString("metrics-body")
	})
	return app
}

func doGet(t *testing.T, app *fiber.App, authHeader string) (int, string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	resp, err := app.Test(req)
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, string(body)
}

func TestMetricsAuth_TokenUnset_StaysOpen(t *testing.T) {
	// Backward compatibility: with no token configured the endpoint is open, so
	// existing Prometheus scrapers keep working.
	app := mountMetrics("")

	status, body := doGet(t, app, "")

	assert.Equal(t, fiber.StatusOK, status)
	assert.Equal(t, "metrics-body", body)
}

func TestMetricsAuth_TokenSet_RequiresValidBearer(t *testing.T) {
	const token = "s3cr3t-scrape-token"
	app := mountMetrics(token)

	t.Run("correct bearer passes", func(t *testing.T) {
		status, body := doGet(t, app, "Bearer "+token)
		assert.Equal(t, fiber.StatusOK, status)
		assert.Equal(t, "metrics-body", body)
	})

	t.Run("missing header is rejected", func(t *testing.T) {
		status, _ := doGet(t, app, "")
		assert.Equal(t, fiber.StatusUnauthorized, status)
	})

	t.Run("wrong token is rejected", func(t *testing.T) {
		status, _ := doGet(t, app, "Bearer not-the-token")
		assert.Equal(t, fiber.StatusUnauthorized, status)
	})

	t.Run("non-bearer scheme is rejected", func(t *testing.T) {
		status, _ := doGet(t, app, "Basic "+token)
		assert.Equal(t, fiber.StatusUnauthorized, status)
	})

	t.Run("prefix of the token is rejected (exact match, not prefix)", func(t *testing.T) {
		// Guards against a truncated-compare bug: a value that is a prefix of the
		// real token must not pass.
		status, _ := doGet(t, app, "Bearer "+token[:len(token)-1])
		assert.Equal(t, fiber.StatusUnauthorized, status)
	})

	t.Run("401 advertises the bearer scheme", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
		assert.Contains(t, resp.Header.Get("WWW-Authenticate"), "Bearer")
	})
}
