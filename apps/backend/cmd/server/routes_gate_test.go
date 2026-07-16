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
