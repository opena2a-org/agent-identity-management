package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pyFuncBody returns the text of one top-level Python function: from its def
// line to the next top-level "def " or the end of the file.
func pyFuncBody(t *testing.T, src, def string) string {
	t.Helper()
	start := strings.Index(src, "\n"+def)
	require.NotEqual(t, -1, start, "function %q not found", def)
	rest := src[start+1:]
	end := strings.Index(rest[len(def):], "\ndef ")
	if end == -1 {
		return rest
	}
	return rest[:len(def)+end]
}

// srcContains asserts on presence without printing the source on failure: a
// failed pin names what is missing, not the file.
func srcContains(t *testing.T, src, needle, what string) {
	t.Helper()
	assert.True(t, strings.Contains(src, needle), "%s: %q not found", what, needle)
}

func srcNotContains(t *testing.T, src, needle, what string) {
	t.Helper()
	assert.False(t, strings.Contains(src, needle), "%s: %q still present", what, needle)
}

// TestSDKAPIKeyRegistrationContract pins the pair that lets an API key register
// an agent: the route and middleware the backend mounts, and the path and header
// the SDKs send. Measured 2026-09-22 against a self-hosted stack: the Python
// SDK's api-key mode posted to /api/v1/public/agents/register with a header no
// backend file reads, and every registration failed with 400. Either side
// drifting from the other fails here, in the tree that carries both.
func TestSDKAPIKeyRegistrationContract(t *testing.T) {
	mainSrc := aim03ReadRepoFile(t, "apps/backend/cmd/server/main.go")
	srcContains(t, mainSrc, `agents.Use(middleware.OptionalAPIKeyMiddleware(db))`,
		"the /agents group must try API-key auth")
	assert.True(t, regexp.MustCompile(`agents\.Post\("/", middleware\.MemberOrAPIKeyMiddleware\(\), h\.Agent\.CreateAgent\)`).MatchString(mainSrc),
		"POST /api/v1/agents must admit auth_method=api_key")

	mw := aim03ReadRepoFile(t, "apps/backend/internal/interfaces/http/middleware/api_key.go")
	srcContains(t, mw, `c.Get("X-API-Key")`, "the middleware reads X-API-Key")

	py := aim03ReadRepoFile(t, "sdk/python/aim_sdk/client.py")
	srcContains(t, py, `API_KEY_HEADER = "X-API-Key"`, "one module constant names the header")
	fn := pyFuncBody(t, py, "def _register_via_api_key(")
	srcContains(t, fn, `url = f"{aim_url.rstrip('/')}/api/v1/agents"`,
		"python api-key mode must POST /api/v1/agents")
	srcNotContains(t, fn, `url = f"{aim_url.rstrip('/')}/api/v1/public/agents/register"`,
		"the public route reads no API key")
	srcContains(t, fn, `API_KEY_HEADER: api_key`, "python api-key mode sends the constant")
	srcContains(t, fn, `registration_data["publicKey"]`,
		"python api-key mode must send a client-side public key; the agents route returns no private key")

	ts := aim03ReadRepoFile(t, "sdk/typescript/src/client/AIMClient.ts")
	srcContains(t, ts, `'/api/v1/agents'`, "typescript registerAgent posts /api/v1/agents")
	srcContains(t, ts, `headers['X-API-Key']`, "typescript sends X-API-Key")

	// The header nothing reads must not come back anywhere in the Python package.
	pkg := filepath.Join(aim03RepoRoot(t), "sdk", "python", "aim_sdk")
	entries, err := os.ReadDir(pkg)
	require.NoError(t, err)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".py") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(pkg, e.Name())) //nolint:gosec // test reads repo sources
		require.NoError(t, err)
		srcNotContains(t, string(b), "X-AIM-API-Key", e.Name()+" sends a header no backend reads")
	}
}

// aim03RepoRoot resolves the repository root the same way aim03ReadRepoFile
// does: walk up from the working directory until apps/backend/go.mod is found.
func aim03RepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	require.NoError(t, err)
	dir := wd
	for i := 0; i < 12; i++ {
		if _, err := os.Stat(filepath.Join(dir, "apps", "backend", "go.mod")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	require.FailNow(t, "repository root (apps/backend/go.mod) not found above "+wd)
	return ""
}
