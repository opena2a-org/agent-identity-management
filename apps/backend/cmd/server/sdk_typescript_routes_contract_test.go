package main

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The TypeScript SDK's requests live in sdk/typescript/tests/fixtures/sdk_requests,
// and AIMClient.sdk-routes.test.ts holds the client to each fixture's method and
// path. This test holds the server to them: every path is a route it mounts.
// verifyAction posted to /api/v1/verify, which no route serves (#662).

type tsRequestFixturePath struct {
	Method string `json:"method"`
	Path   string `json:"path"`
}

func readTSRequestFixturePath(t *testing.T, name string) tsRequestFixturePath {
	t.Helper()
	var fx tsRequestFixturePath
	src := aim03ReadRepoFile(t, "sdk/typescript/tests/fixtures/sdk_requests/"+name+".json")
	require.NoError(t, json.Unmarshal([]byte(src), &fx), "parse the %s fixture", name)
	return fx
}

func TestTypeScriptSDKRequestPathsAreMounted(t *testing.T) {
	table := sdkAPIRouteTable(sdkAPITestHandlers(func(c fiber.Ctx) error { return nil }))
	inTable := map[string]sdkAPIRoute{}
	for _, route := range table {
		inTable[route.Method+" "+route.FullPath()] = route
	}

	t.Run("the SDK-API paths are in the route table", func(t *testing.T) {
		for _, name := range []string{"verify_action", "verify_action_minimal", "register_capability"} {
			fx := readTSRequestFixturePath(t, name)
			pattern := strings.ReplaceAll(fx.Path, "{id}", ":id")
			_, ok := inTable[fx.Method+" "+pattern]
			assert.True(t, ok, "%s: %s %s is not in the SDK-API route table:\n%s", name, fx.Method, pattern, aim03FormatTable(table))
		}

		verify := readTSRequestFixturePath(t, "verify_action")
		route := inTable[verify.Method+" "+verify.Path]
		assert.True(t, route.Bare,
			"the verification route authenticates the request from its body, and the TypeScript SDK sends it no Authorization header")
	})

	t.Run("the verification request reaches its handler through every route main mounts", func(t *testing.T) {
		app, _ := strictCensusApp(t)
		fx := readTSRequestFixturePath(t, "verify_action")

		req := httptest.NewRequest(fx.Method, fx.Path, strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Real-IP", "10.66.2.1")
		resp, err := app.Test(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		raw, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		// An empty body is refused by the handler itself, which is what shows the
		// request reached it.
		assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode, "body: %s", raw)
		assert.Contains(t, string(raw), "agentId, capability, signature, and publicKey are required")
	})

	t.Run("the agent update path is mounted on UpdateAgent", func(t *testing.T) {
		fx := readTSRequestFixturePath(t, "update_agent")
		require.Equal(t, fiber.MethodPut, fx.Method)
		require.Equal(t, "/api/v1/agents/{id}", fx.Path)
		mainSrc := aim03ReadRepoFile(t, "apps/backend/cmd/server/main.go")
		assert.Regexp(t, regexp.MustCompile(`agentsHeld\.Put\("/:id", [^\n]*h\.Agent\.UpdateAgent\)`), mainSrc,
			"PUT /api/v1/agents/:id must be served by UpdateAgent")
	})
}
