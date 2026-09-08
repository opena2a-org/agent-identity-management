package main

import (
	"net/http"
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

// TestSensitiveAgentRoutesAreMemberGated is a source-wiring guard.
//
// GET /agents/:id/credentials and GET /agents/:id/sdk return agent private key
// material (the decrypted Ed25519 private key / an SDK zip with the key embedded).
// The /agents group applies OptionalAPIKeyMiddleware, so without a per-route role
// gate a bare org-scoped API key is admitted straight to the private-key retrieval
// path. These routes — like rotate-credentials and key replacement — MUST carry
// middleware.MemberMiddleware() (JWT role-gated; API keys carry no role → 401).
//
// setupRoutes() cannot be exercised in isolation (it needs a live DB + full service
// wiring), so this guard reads main.go directly. It FAILS if any listed route loses
// its MemberMiddleware() gate — e.g. reverted to no gate, or widened to
// MemberOrAPIKeyMiddleware() (which "MemberMiddleware()" is deliberately not a
// substring of).
func TestSensitiveAgentRoutesAreMemberGated(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed; cannot locate main.go")
	}
	mainPath := filepath.Join(filepath.Dir(thisFile), "main.go")
	src, err := os.ReadFile(mainPath) //nolint:gosec // G304: path derived from runtime.Caller, not user input
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	lines := strings.Split(string(src), "\n")

	// route-registration substring -> must appear on the SAME line as the gate.
	sensitive := []string{
		`agents.Get("/:id/credentials"`,
		`agents.Get("/:id/sdk"`,
		`agents.Post("/:id/rotate-credentials"`,
		`agents.Put("/:id/keys"`,
	}

	for _, route := range sensitive {
		var found bool
		for _, line := range lines {
			if strings.Contains(line, route) {
				found = true
				if !strings.Contains(line, "middleware.MemberMiddleware()") {
					t.Errorf("route %s must be gated by middleware.MemberMiddleware() "+
						"(returns/derives private key material); got:\n  %s",
						route, strings.TrimSpace(line))
				}
				break
			}
		}
		if !found {
			t.Errorf("route registration %s not found in main.go — did it move or get renamed? "+
				"Update this guard so private-key routes stay covered.", route)
		}
	}
}

// TestSDKVerificationRoutesRequireJustifiedBareMount is a guard for the class,
// not for two routes.
//
// The `/api/v1/sdk-api/verifications*` routes are registered on the bare `app`,
// ABOVE the sdkAPI group. Fiber matches in registration order, so a route
// registered there is never reached by the group's Ed25519/JWT middleware — its
// only middleware is a rate limiter, which authenticates nothing. That is how
// POST .../result and POST .../execution-status served unauthenticated writes to
// production for months.
//
// Nothing in either tree previously treated "registered on app, above the group"
// as a condition requiring justification, which is why PR #236 fixed the GET on
// one line and left the two POSTs on the next two. This guard makes the bare
// mount an explicit, enumerated decision: every route the table marks Bare must
// appear below with the in-handler control that makes it safe. A new one fails
// until someone writes down why it does not need the group.
//
// AIM-03 moved the registration out of main() into sdkAPIRouteTable, so this is
// no longer a source scan: it mounts the real table and asks the running app
// which routes the group's middleware actually reaches. A reordering that put a
// route above the group's Use chain — the failure the old string-order check
// existed to catch — now fails as a request that arrives unauthenticated.
func TestSDKVerificationRoutesRequireJustifiedBareMount(t *testing.T) {
	// "METHOD full-path" -> the in-handler authentication that makes serving it
	// outside the sdkAPI group acceptable.
	justifiedBareMounts := map[string]string{
		"POST /api/v1/sdk-api/verifications": "CreateVerification verifies an Ed25519 signature over the request " +
			"(verifySignature) and gates on agent.Status before writing.",
		"GET /api/v1/sdk-api/verifications/:id": "read path; the SDK-signature handler GetVerificationSDK verifies three " +
			"X-AIM-* headers, an Ed25519 signature and event ownership (404, not 403, on mismatch). NOTE: cloud currently " +
			"mounts GetVerification here, not GetVerificationSDK; that divergence is tracked separately and must not be " +
			"shipped ahead of this change.",
	}

	// Mount the real table. The handlers are stand-ins — what a test may
	// substitute is the handler, never the path — and each middleware records
	// that it ran, so "is this route authenticated" is answered by a request
	// rather than by reading the file.
	const sentinelStatus = http.StatusTeapot
	var groupMiddlewareRan, bareMiddlewareRan bool
	app := fiber.New()
	table := registerSDKAPIRoutes(app, sdkAPIDeps{
		BareMiddleware: func(c fiber.Ctx) error {
			bareMiddlewareRan = true
			return c.Next()
		},
		GroupMiddleware: []fiber.Handler{func(c fiber.Ctx) error {
			groupMiddlewareRan = true
			return c.Next()
		}},
		Handlers: sdkAPITestHandlers(func(c fiber.Ctx) error { return c.SendStatus(sentinelStatus) }),
	})
	require.NotEmpty(t, table, "registerSDKAPIRoutes returned no routes")

	for _, route := range table {
		groupMiddlewareRan, bareMiddlewareRan = false, false

		resp, err := app.Test(httptest.NewRequest(route.Method, concreteSDKAPIPath(route.FullPath()), nil))
		require.NoError(t, err)
		status := resp.StatusCode
		require.NoError(t, resp.Body.Close())

		require.Equal(t, sentinelStatus, status,
			"%s %s is in the table but the mounted app does not serve it", route.Method, route.FullPath())

		key := route.Method + " " + route.FullPath()
		if route.Bare {
			if _, ok := justifiedBareMounts[key]; !ok {
				t.Errorf("route %s is registered on the bare `app` above the sdkAPI group with no recorded justification.\n\n"+
					"Routes registered there are NOT reached by the group's Ed25519/JWT middleware — their only middleware is a "+
					"rate limiter, which authenticates nothing. Either drop Bare so it is served from the group, or add it to "+
					"justifiedBareMounts in this test naming the in-handler authentication that makes the bare mount safe.", key)
			}
			// The security claim behind the justification, asserted rather than
			// assumed: a bare route really is outside the group's chain.
			assert.False(t, groupMiddlewareRan,
				"%s is marked Bare but the group middleware reached it; the Bare/justification split is describing the wrong thing", key)
			assert.True(t, bareMiddlewareRan, "%s did not run the bare middleware chain", key)
			continue
		}

		assert.True(t, groupMiddlewareRan,
			"%s is served WITHOUT the group's middleware. Fiber applies a group's Use chain only to routes registered "+
				"after it, so this route is unauthenticated: every sdkAPIDeps.GroupMiddleware must be registered before "+
				"any grouped route in registerSDKAPIRoutes.", key)
		if _, ok := justifiedBareMounts[key]; ok {
			t.Errorf("%s is listed in justifiedBareMounts but is no longer a bare mount — delete the stale justification", key)
		}
	}

	// The two verification write routes must be on the group, and the JWT mount
	// that authenticated a user without checking ownership must not come back.
	for _, required := range []string{
		"POST /api/v1/sdk-api/verifications/:id/result",
		"POST /api/v1/sdk-api/verifications/:id/execution-status",
	} {
		var found bool
		for _, route := range table {
			if !route.Bare && route.Method+" "+route.FullPath() == required {
				found = true
				break
			}
		}
		assert.True(t, found, "%s is missing: the write routes must be served from the authenticated sdkAPI group", required)
	}

	_, thisFile, ok := callerFile()
	if !ok {
		t.Fatal("runtime.Caller failed; cannot locate main.go")
	}
	mainPath := filepath.Join(filepath.Dir(thisFile), "main.go")
	src, err := os.ReadFile(mainPath) //nolint:gosec // G304: path derived from runtime.Caller, not user input
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	stripped := stripComments(string(src))

	if strings.Contains(stripped, `verifications.Post("/:id/result"`) {
		t.Error(`registration verifications.Post("/:id/result") must not exist: it authenticates a user without checking ownership`)
	}

	// One table, one place. A path registered anywhere but sdkAPIRouteTable is
	// invisible to every guard above — which is precisely how the backend came
	// to serve /agents/:id/isolation while all three SDKs posted
	// /agents/:id/isolation-attestation.
	if strings.Contains(stripped, sdkAPIBasePath) {
		t.Errorf("main.go names %q directly. SDK-API paths belong in sdkAPIRouteTable (sdk_api_routes.go), which main.go "+
			"and the tests both mount through registerSDKAPIRoutes; a path written anywhere else is one no parity test can see.",
			sdkAPIBasePath)
	}
}

// concreteSDKAPIPath substitutes a real UUID for every :param in a registered
// path, so a test can call the route it just mounted without restating it.
func concreteSDKAPIPath(path string) string {
	const id = "6f1a8f4e-6b6f-4a3a-9a9a-2f2b1c0d4e5f"
	segments := strings.Split(path, "/")
	for i, segment := range segments {
		if strings.HasPrefix(segment, ":") {
			segments[i] = id
		}
	}
	return strings.Join(segments, "/")
}

func callerFile() (uintptr, string, bool) {
	pc, file, _, ok := runtime.Caller(1)
	return pc, file, ok
}

// stripComments drops whole-line comments so an absence assertion is not
// satisfied or defeated by prose. main.go documents the removed JWT mount by
// name; that comment is the explanation, not a route.
func stripComments(src string) string {
	var code []string
	for _, line := range strings.Split(src, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "//") {
			code = append(code, line)
		}
	}
	return strings.Join(code, "\n")
}
