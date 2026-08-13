package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
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

// TestSDKVerificationRoutesRequireJustifiedBareMount is a source-wiring guard for
// the class, not for two routes.
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
// mount an explicit, enumerated decision: every app-level registration under
// that prefix must appear below with the in-handler control that makes it safe.
// A new one fails until someone writes down why it does not need the group.
func TestSDKVerificationRoutesRequireJustifiedBareMount(t *testing.T) {
	// route registration substring -> the in-handler authentication that makes
	// serving it outside the sdkAPI group acceptable.
	justifiedBareMounts := map[string]string{
		`app.Post("/api/v1/sdk-api/verifications"`: "CreateVerification verifies an Ed25519 signature over the request " +
			"(verifySignature) and gates on agent.Status before writing.",
		`app.Get("/api/v1/sdk-api/verifications/:id"`: "read path; the SDK-signature handler GetVerificationSDK verifies three " +
			"X-AIM-* headers, an Ed25519 signature and event ownership. NOTE: cloud currently mounts GetVerification here, " +
			"not GetVerificationSDK; that divergence is tracked separately and must not be shipped ahead of this change.",
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

	const prefix = `"/api/v1/sdk-api/verifications`
	for _, line := range strings.Split(string(src), "\n") {
		trimmed := strings.TrimSpace(line)
		// Comments describe routes; they do not register them.
		if strings.HasPrefix(trimmed, "//") || !strings.Contains(trimmed, prefix) {
			continue
		}
		// Group-relative registrations (sdkAPI.Post("/verifications/...")) do not
		// carry the full path, so anything matching here is app-level.
		if !strings.HasPrefix(trimmed, "app.") {
			continue
		}
		var justified bool
		for mount := range justifiedBareMounts {
			if strings.Contains(trimmed, mount) {
				justified = true
				break
			}
		}
		if !justified {
			t.Errorf("route registered on bare `app` above the sdkAPI group with no recorded justification:\n  %s\n\n"+
				"Routes registered here are NOT reached by the group's Ed25519/JWT middleware — their only middleware is a "+
				"rate limiter, which authenticates nothing. Either move it onto the sdkAPI group, or add it to "+
				"justifiedBareMounts in this test naming the in-handler authentication that makes the bare mount safe.",
				trimmed)
		}
	}

	// ORDER, not just presence. Fiber applies a group's Use() chain only to routes
	// registered AFTER it. Asserting that `sdkAPI.Post("/verifications/:id/result"`
	// merely EXISTS is satisfied by a file where the three sdkAPI.Use(...) lines sit
	// BELOW it — which serves the route with no middleware at all and re-opens the
	// unauthenticated write with this guard still green. The pre-existing guard in
	// this file avoids the problem by requiring the gate on the same LINE as the
	// registration; a group chain cannot be expressed that way, so the ordering is
	// asserted explicitly instead.
	lines := strings.Split(string(src), "\n")
	lastUse, firstGuardedRoute := -1, -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") {
			continue
		}
		if strings.HasPrefix(trimmed, "sdkAPI.Use(") {
			lastUse = i
		}
		if strings.Contains(trimmed, `sdkAPI.Post("/verifications/:id/`) && firstGuardedRoute == -1 {
			firstGuardedRoute = i
		}
	}
	switch {
	case lastUse == -1:
		t.Error("no sdkAPI.Use(...) found: the group carries no middleware, so its routes are unauthenticated")
	case firstGuardedRoute == -1:
		t.Error("no sdkAPI.Post(\"/verifications/:id/...\") found: the write routes are not on the group")
	case firstGuardedRoute < lastUse:
		t.Errorf("verification write route registered at line %d, BEFORE the last sdkAPI.Use(...) at line %d. "+
			"Fiber applies a group's middleware only to routes registered after it, so this route is served "+
			"with no authentication. Move the registration below every sdkAPI.Use(...) line.",
			firstGuardedRoute+1, lastUse+1)
	}

	// The two write routes must be on the group, and must not have come back.
	stripped := stripComments(string(src))
	for _, forbidden := range []string{
		`app.Post("/api/v1/sdk-api/verifications/:id/result"`,
		`app.Post("/api/v1/sdk-api/verifications/:id/execution-status"`,
		`verifications.Post("/:id/result"`,
	} {
		if strings.Contains(stripped, forbidden) {
			t.Errorf("registration %s must not exist: it bypasses authentication (bare app mount) or "+
				"authenticates a user without checking ownership (JWT mount)", forbidden)
		}
	}
	for _, required := range []string{
		`sdkAPI.Post("/verifications/:id/result"`,
		`sdkAPI.Post("/verifications/:id/execution-status"`,
	} {
		if !strings.Contains(stripped, required) {
			t.Errorf("registration %s is missing: the write routes must be served from the authenticated sdkAPI group", required)
		}
	}
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
