package handlers

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The AIP spec's Quick Start documents two endpoints against the reference deployment:
//
//	GET /.well-known/aip
//	GET /api/v1/did/{did}
//
// Both returned 404 in production for months because this repository carried
// aip_handler.go but never mounted it: no Handlers field, no constructor call, no route.
// The published spec promised endpoints the reference deployment refused.
//
// These tests pin the mounting topology from cmd/server/main.go: that the routes are
// REACHABLE and UNAUTHENTICATED, which is exactly what was broken, and — since
// unauthenticated is now a ruled condition rather than an accident — that the route
// carries the limiter those conditions require.
//
// The ResolveDID response body is asserted in aip_handler_test.go, which resolves
// through a fake repository. This file deliberately stops at the wiring.

// mountAIPRoutes mirrors the registration order in cmd/server/main.go: the well-known
// route on the root app, then the /api/v1 group, then DID resolution on that group.
func mountAIPRoutes(app *fiber.App, h *AIPHandler, resolve fiber.Handler) {
	app.Get("/.well-known/aip", h.WellKnownAIP)

	v1 := app.Group("/api/v1")
	v1.Get("/did/*", resolve)
}

// notFoundSentinel is a handler that must never run. Reaching it proves a route matched.
func reached(hit *bool) fiber.Handler {
	return func(c fiber.Ctx) error {
		*hit = true
		return c.SendString("reached")
	}
}

func TestWellKnownAIPIsMountedAndPublic(t *testing.T) {
	app := fiber.New()
	mountAIPRoutes(app, NewAIPHandler(nil), func(c fiber.Ctx) error { return nil })

	resp, err := app.Test(httptest.NewRequest("GET", "/.well-known/aip", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode,
		"/.well-known/aip must be reachable without authentication; the AIP spec Quick Start documents it")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var doc map[string]any
	require.NoError(t, json.Unmarshal(body, &doc))

	// The discovery document is what an implementer reads first, so pin its contract.
	assert.Equal(t, "did:aip:provider_opena2a", doc["providerDid"])
	assert.Equal(t, "1.0", doc["version"])

	endpoints, ok := doc["endpoints"].(map[string]any)
	require.True(t, ok, "discovery document must carry an endpoints directory")
	assert.Equal(t, "/api/v1/did/{did}", endpoints["didResolve"],
		"the advertised didResolve path must match the route actually mounted below")
}

// The discovery document advertises a resolver path. If that path is not mounted, the
// document sends implementers to a 404 — which is precisely the shipped defect.
func TestAdvertisedDIDResolvePathIsActuallyMounted(t *testing.T) {
	var hit bool
	app := fiber.New()
	mountAIPRoutes(app, NewAIPHandler(nil), reached(&hit))

	// Take the path straight from the discovery document rather than hard-coding it, so
	// this test cannot drift away from what the handler advertises.
	resp, err := app.Test(httptest.NewRequest("GET", "/.well-known/aip", nil))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(body, &doc))
	advertised := doc["endpoints"].(map[string]any)["didResolve"].(string)

	// Substitute a concrete DID for the {did} template segment.
	const sampleDID = "did:aip:aim_3f2504e0-4f89-11d3-9a0c-0305e82c3301"
	path := advertised[:len(advertised)-len("{did}")] + sampleDID

	r, err := app.Test(httptest.NewRequest("GET", path, nil))
	require.NoError(t, err)
	assert.NotEqual(t, fiber.StatusNotFound, r.StatusCode,
		"the didResolve path advertised by /.well-known/aip must be mounted")
	assert.True(t, hit, "request must reach the DID resolution handler, unauthenticated")
}

// Pins the placement decision recorded in cmd/server/main.go.
//
// An earlier version of that comment claimed an app-level /api/v1/* route is shadowed by
// the app.Group("/api/v1") registered after it, inherited from a note in
// agent-identity-management. Under the Fiber version in go.mod that is false, and these
// subtests are what established it. The real difference is middleware: a route on app
// bypasses the group's chain, and a route on v1 runs on it. Recording that here so the
// comment rests on a check rather than on inherited folklore.
func TestDIDRoutePlacementIsAMiddlewareChoiceNotARoutingOne(t *testing.T) {
	// An app-level /api/v1/* route registered before the group still matches, and the
	// group's middleware never runs for it.
	t.Run("app-level route matches and bypasses group middleware", func(t *testing.T) {
		var rootHit, mwRan bool
		app := fiber.New()
		app.Get("/api/v1/did/*", reached(&rootHit))
		v1 := app.Group("/api/v1")
		v1.Use(func(c fiber.Ctx) error { mwRan = true; return c.Next() })

		_, err := app.Test(httptest.NewRequest("GET", "/api/v1/did/did:aip:aim_x", nil))
		require.NoError(t, err)

		assert.True(t, rootHit, "app-level /api/v1/* is reachable in this Fiber version")
		assert.False(t, mwRan, "and it bypasses the group middleware chain")
	})

	// The placement main.go actually uses: on v1, so the route runs on the group chain.
	t.Run("v1-mounted route runs on the group middleware chain", func(t *testing.T) {
		var routeHit, mwRan bool
		app := fiber.New()
		v1 := app.Group("/api/v1")
		v1.Use(func(c fiber.Ctx) error { mwRan = true; return c.Next() })
		v1.Get("/did/*", reached(&routeHit))

		_, err := app.Test(httptest.NewRequest("GET", "/api/v1/did/did:aip:aim_x", nil))
		require.NoError(t, err)

		assert.True(t, routeHit, "the v1-mounted DID route must be reachable")
		assert.True(t, mwRan, "and it must run on the group middleware chain")
	})
}

// backendFile resolves a path relative to apps/backend from this test's own location,
// so the source-reading assertions below do not depend on the working directory.
func backendFile(t *testing.T, rel string) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok, "runtime.Caller failed; cannot locate the backend tree")
	// internal/interfaces/http/handlers -> apps/backend
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..", rel)
}

func readBackendFile(t *testing.T, rel string) string {
	t.Helper()
	src, err := os.ReadFile(backendFile(t, rel)) //nolint:gosec // G304: path derived from runtime.Caller, not user input
	require.NoError(t, err, "read %s", rel)
	return string(src)
}

// TestDIDResolutionRouteCarriesTheSharedRateLimiter is a source-wiring guard.
//
// /api/v1/did/* is served unauthenticated to the whole internet, and it is the only
// route on the v1 group that is BOTH unauthenticated and backed by a database read
// (agentRepo.GetByID). Public mounting was permitted on conditions, and the first of
// them is that the route is rate-limited (COUNCIL_LEDGER 2026-09-02, CHIEF-CISO).
//
// setupRoutes() cannot be exercised in isolation — it needs a live DB and the full
// service wiring — and this package's own tests mount the handler on fiber apps they
// build themselves, so they would certify a limiter that is not the deployed one.
// The only way to assert the DEPLOYED chain is to read main.go, which is what this
// does: the middleware must sit on the same LINE as the registration, because a
// group-level Use() applies only to routes registered after it and would let this
// guard stay green while the route ran bare.
func TestDIDResolutionRouteCarriesTheSharedRateLimiter(t *testing.T) {
	mainGo := readBackendFile(t, "cmd/server/main.go")

	const registration = `v1.Get("/did/*"`

	var line string
	for _, l := range strings.Split(mainGo, "\n") {
		trimmed := strings.TrimSpace(l)
		// Comments describe routes; they do not register them.
		if strings.HasPrefix(trimmed, "//") || !strings.Contains(trimmed, registration) {
			continue
		}
		require.Empty(t, line, "two registrations of %s in main.go; only one can be the deployed one", registration)
		line = trimmed
	}
	require.NotEmpty(t, line,
		"route registration %s not found in main.go — did it move or get renamed? "+
			"The AIP spec Quick Start documents this path, so it must stay mounted, and "+
			"this guard must follow it.", registration)

	t.Run("AIMC-06.AC1 the DID route registration carries the rate limiter", func(t *testing.T) {
		assert.Contains(t, line, "middleware.RateLimitMiddleware()",
			"the public DID resolver must be rate limited on its own registration line; "+
				"the /api/v1 group applies no limiter, so without this one caller can drive "+
				"unbounded agent lookups against the database. Got:\n  %s", line)
		assert.Contains(t, line, "h.AIP.ResolveDID",
			"the limiter must be on the line that registers ResolveDID, not on some other route")
	})

	t.Run("AIMC-06.AC1 it is the same limiter every authenticated group applies", func(t *testing.T) {
		// Not a bespoke limiter for this route: the ruling names the agents group's
		// limiter, so a change to that shared budget moves this route with it.
		agentsLine := findLine(mainGo, `agents.Use(middleware.RateLimitMiddleware())`)
		require.NotEmpty(t, agentsLine,
			"the agents group no longer applies middleware.RateLimitMiddleware(); this guard "+
				"names it as the reference limiter and must be re-pointed if that moved")

		body := rateLimitMiddlewareBody(t)
		assert.Contains(t, body, "Max:        rateLimitMax(100)",
			"RateLimitMiddleware must stay the 100 req/min budget the ruling names")
		assert.Contains(t, body, "Expiration: 1 * time.Minute",
			"RateLimitMiddleware must stay a one-minute window")
	})

	t.Run("AIMC-06.AC1 the limiter keys anonymous callers on the client address", func(t *testing.T) {
		// The route is unauthenticated by design, so c.Locals("user_id") is never set
		// and the key generator always falls through to the address branch. If that
		// branch went away, this route would key every caller in the world into one
		// bucket — turning the limiter into a denial of service on a published
		// spec endpoint.
		assert.Contains(t, rateLimitMiddlewareBody(t), `return "ip:" + getClientIP(c)`,
			"RateLimitMiddleware must fall back to the client address for unauthenticated callers")

		// And that address must not be one the caller picks. getClientIP walks
		// X-Forwarded-For from the RIGHT; taking ips[0] would hand a single caller an
		// unbounded number of buckets. The behaviour is tested in the middleware
		// package (rate_limit_test.go); this asserts the reverted form has not come
		// back, because that revert is silent and otherwise green. Comments are
		// stripped first: rate_limit.go names the reverted form in prose to explain
		// why it is gone, and that explanation is not a code path.
		code := dropLineComments(readBackendFile(t, "internal/interfaces/http/middleware/rate_limit.go"))
		assert.NotContains(t, code, "ips[0]",
			"getClientIP must not key on the leftmost X-Forwarded-For entry: every hop "+
				"APPENDS to that header, so ips[0] is written by the client itself")
	})
}

// dropLineComments removes whole-line comments so an absence assertion is neither
// satisfied nor defeated by prose about the thing that must be absent.
func dropLineComments(src string) string {
	var code []string
	for _, l := range strings.Split(src, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(l), "//") {
			code = append(code, l)
		}
	}
	return strings.Join(code, "\n")
}

// findLine returns the first non-comment line containing want, or "".
func findLine(src, want string) string {
	for _, l := range strings.Split(src, "\n") {
		trimmed := strings.TrimSpace(l)
		if strings.HasPrefix(trimmed, "//") || !strings.Contains(trimmed, want) {
			continue
		}
		return trimmed
	}
	return ""
}

// rateLimitMiddlewareBody returns the source of RateLimitMiddleware alone.
// StrictRateLimitMiddleware below it carries the same field names with a tenth of the
// budget, so asserting over the whole file would pass on either one.
func rateLimitMiddlewareBody(t *testing.T) string {
	t.Helper()
	src := readBackendFile(t, "internal/interfaces/http/middleware/rate_limit.go")
	i := strings.Index(src, "func RateLimitMiddleware()")
	require.GreaterOrEqual(t, i, 0, "func RateLimitMiddleware() not found in rate_limit.go")
	rest := src[i:]
	if j := strings.Index(rest[1:], "\nfunc "); j >= 0 {
		return rest[:j+1]
	}
	return rest
}
