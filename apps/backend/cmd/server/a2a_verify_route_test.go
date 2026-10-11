package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/interfaces/http/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// POST /api/v1/a2a/verify reaches the verify handler without credentials, and
// any other method on the path is answered 405 with Allow: POST, not 401 by the
// authentication of the /a2a group mounted after it, as setupRoutes mounts it.
func TestA2AVerifyAnswersOtherMethods405(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret-key-for-testing-purposes-32chars")
	app := fiber.New(fiber.Config{ErrorHandler: customErrorHandler})
	v1 := app.Group("/api/v1")
	verified := 0
	mountA2AVerifyRoute(v1, func(c fiber.Ctx) error {
		verified++
		return c.SendStatus(fiber.StatusOK)
	})
	a2a := v1.Group("/a2a")
	a2a.Use(middleware.AuthMiddleware(auth.NewJWTService()))
	a2a.Post("/sign", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	resp, err := app.Test(httptest.NewRequest(http.MethodPost, "/api/v1/a2a/verify", strings.NewReader(`{}`)))
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Equal(t, 1, verified, "POST did not reach the verify handler")

	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		resp, err := app.Test(httptest.NewRequest(method, "/api/v1/a2a/verify", nil))
		require.NoError(t, err, method)
		assert.Equal(t, fiber.StatusMethodNotAllowed, resp.StatusCode, method)
		assert.Equal(t, http.MethodPost, resp.Header.Get(fiber.HeaderAllow), method)
	}
	assert.Equal(t, 1, verified, "a method other than POST reached the verify handler")

	// The rest of the group still asks for credentials.
	resp, err = app.Test(httptest.NewRequest(http.MethodPost, "/api/v1/a2a/sign", nil))
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

// setupRoutes mounts /a2a/verify once, through mountA2AVerifyRoute. A second
// mount that the router serves on the same path is never reached: the first
// answers every request to the path that reaches the routes. The routes are
// read from the router mountAPIRoutes builds, not from the source, so a mount
// with a trailing slash, through Add, or through a group helper is counted
// as the router would match it.
func TestA2AVerifyIsMountedOnce(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok, "runtime.Caller failed; cannot locate main.go")
	src, err := os.ReadFile(filepath.Join(filepath.Dir(thisFile), "main.go")) //nolint:gosec // G304: path derived from runtime.Caller, not user input
	require.NoError(t, err)
	code := stripComments(string(src))

	assert.Equal(t, 1, strings.Count(code, "h.A2A.VerifyRequest"),
		"the verify handler is mounted more than once, or not at all")
	assert.Equal(t, 1, strings.Count(code, "mountA2AVerifyRoute(v1, h.A2A.VerifyRequest)"),
		"setupRoutes does not mount the verify handler through mountA2AVerifyRoute")

	const path = "/api/v1/a2a/verify"
	server, _ := strictCensusApp(t)
	alone := fiber.New()
	mountA2AVerifyRoute(alone.Group("/api/v1"), func(c fiber.Ctx) error { return nil })
	assert.Equal(t, routesServing(alone, path), routesServing(server, path),
		"a route other than mountA2AVerifyRoute's is mounted on %s, where mountA2AVerifyRoute answers first", path)
}

// routesServing lists, as "METHOD /registered/path", every route of app that
// the router matches for path. Middleware registered with Use is left out.
func routesServing(app *fiber.App, path string) []string {
	var served []string
	for _, route := range app.GetRoutes(true) {
		if fiber.RoutePatternMatch(path, route.Path, app.Config()) {
			served = append(served, route.Method+" "+route.Path)
		}
	}
	sort.Strings(served)
	return served
}

// POST /api/v1/a2a/verify takes no credentials, so it carries the shared rate
// limiter on its own registration line, as GET /api/v1/did/* and GET
// /api/v1/revocations do: one client is answered 429 once it has sent the
// general limit in a minute.
func TestA2AVerifyIsRateLimited(t *testing.T) {
	server, _ := strictCensusApp(t)
	general, _ := middleware.RateLimitsInForce()

	admitted := 0
	for ; admitted <= general; admitted++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/a2a/verify", strings.NewReader(`{}`))
		req.Header.Set("X-Real-IP", "198.51.100.7")
		resp, err := server.Test(req)
		require.NoError(t, err)
		_ = resp.Body.Close()
		if resp.StatusCode == fiber.StatusTooManyRequests {
			break
		}
	}
	assert.Equal(t, general, admitted, "POST /api/v1/a2a/verify did not refuse one client past the general limit")
}
