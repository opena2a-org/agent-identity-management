package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sdkPackageDir is the Python SDK package, relative to the repository root.
const sdkPackageDir = "sdk/python/aim_sdk"

// sdkServerPathStart matches the start of a Python string literal that holds a
// path on the server. Either the literal is an f-string that opens with the
// server URL (aim_url, self.aim_url, client.aim_url, each with or without
// .rstrip('/')), or it opens with /api/, the form the SDK passes as an endpoint
// and joins to the server URL when it sends the request.
var sdkServerPathStart = regexp.MustCompile(`(["'])(?:\{(?:\w+\.)*aim_url(?:\.rstrip\('/'\))?\}(/)|(/api/))`)

// sdkServerPath is one path a Python SDK module builds on the server URL.
type sdkServerPath struct {
	file string
	line int
	path string
}

// sdkPathAt reads the path of the string literal that src[start:] is in, up to
// the closing quote, a query string or white space. An interpolated expression
// is written as {}, whatever it holds, so a quote or a bracket inside it ends
// nothing.
func sdkPathAt(src string, start int, quote byte) string {
	var path strings.Builder
	depth := 0
	for i := start; i < len(src); i++ {
		c := src[i]
		switch {
		case c == '{':
			if depth == 0 {
				path.WriteString("{}")
			}
			depth++
		case c == '}':
			if depth > 0 {
				depth--
			}
		case depth > 0:
		case c == quote || c == '?' || c == ' ' || c == '\n':
			return path.String()
		default:
			path.WriteByte(c)
		}
	}
	return path.String()
}

// sdkServerPaths returns every path src builds on the server URL, with the
// line it is on.
func sdkServerPaths(file, src string) []sdkServerPath {
	var paths []sdkServerPath
	for _, m := range sdkServerPathStart.FindAllStringSubmatchIndex(src, -1) {
		quote := src[m[2]]
		// The path starts at the slash after the server URL, or at /api/.
		start := m[4]
		if start < 0 {
			start = m[6]
		}
		paths = append(paths, sdkServerPath{
			file: file,
			line: 1 + strings.Count(src[:m[0]], "\n"),
			path: sdkPathAt(src, start, quote),
		})
	}
	return paths
}

// sdkModules returns every Python file of the SDK package, relative to the
// repository root.
func sdkModules(t *testing.T) []string {
	t.Helper()
	root := aim03RepoRoot(t)
	var modules []string
	err := filepath.WalkDir(filepath.Join(root, filepath.FromSlash(sdkPackageDir)), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".py" {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		modules = append(modules, filepath.ToSlash(rel))
		return nil
	})
	require.NoError(t, err)
	sort.Strings(modules)
	return modules
}

// sdkPathsOffTheRouteTable lists every path the census finds in the SDK that
// the server does not register, with what it is. None of them is a route a
// request to this server reaches. The census holds the list exact: a path that
// is not here fails it, and so does an entry the SDK no longer builds or the
// server has come to register.
var sdkPathsOffTheRouteTable = []struct{ module, path, what string }{
	{"a2a.py", "/api/v1/a2a/tasks/{}",
		"get_task_history names the agent in the path; the server registers GET /api/v1/a2a/tasks and PUT /api/v1/a2a/tasks/:id/state"},
	{"a2a.py", "/api/v1/a2a/security/settings/{}",
		"get_security_settings and update_security_settings name the organization in the path; the server registers /api/v1/a2a/security/settings"},
	{"a2a.py", "/api/v1/a2a/security/violations/{}",
		"get_security_violations names the organization in the path; the server registers /api/v1/a2a/security/violations"},
	{"auto_hooks.py", "/api/v1/agents/{}/policies",
		"PolicyCache fetches a per-agent policy route; the module's own comment says no released server registers it"},
	{"client.py", "/dashboard/admin/verifications",
		"a dashboard link built on the server URL; the server registers no /dashboard route"},
	{"telemetry/relay.py", "/api/v1/telemetry/runtime",
		"a path on the Registry, joined to the Registry URL and not to the server URL"},
}

// The path reader on the forms the SDK writes: a path after the server URL in
// an f-string, with or without .rstrip('/') and on self or client, and an
// endpoint literal that opens with /api/. A URL on another address is not a
// path on the server.
func TestSDKServerPaths_ReadsEveryFormTheSDKWrites(t *testing.T) {
	src := strings.Join([]string{
		`url = f"{aim_url}/api/v1/oauth/device/code"`,
		`url = f"{aim_url.rstrip('/')}/api/v1/agents/{existing_agent['id']}/keys"`,
		`url = f"{self.aim_url}/api/v1/sdk-api/verifications/{verification_id}/result"`,
		`url = f"{client.aim_url}/api/v1/mcp-servers/{mcp_server_id}/challenge?agent_id={self.agent_id}"`,
		`self._make_request(method="GET", endpoint=f"/api/v1/agents?{query_string}")`,
		`self._make_request("POST", '/api/v1/a2a/tasks', data=data)`,
		`ROUTE = "/api/v1/agents/{agent_id}/policies"`,
		`link = f"{self.aim_url}/dashboard/admin/verifications"`,
		`card_url = f"{agent_url}/.well-known/agent.json"`,
		`page = f"{ctx.dashboard_url}/dashboard/mcp"`,
		`url = f"{self.aim_url}{endpoint}"`,
	}, "\n")

	var got []string
	for _, p := range sdkServerPaths("example.py", src) {
		got = append(got, fmt.Sprintf("%d %s", p.line, p.path))
	}
	assert.Equal(t, []string{
		"1 /api/v1/oauth/device/code",
		"2 /api/v1/agents/{}/keys",
		"3 /api/v1/sdk-api/verifications/{}/result",
		"4 /api/v1/mcp-servers/{}/challenge",
		"5 /api/v1/agents",
		"6 /api/v1/a2a/tasks",
		"7 /api/v1/agents/{}/policies",
		"8 /dashboard/admin/verifications",
	}, got)
}

// TestSDKLoginRoutesContract pins the pair that lets `aim-sdk login --url`
// complete against a stack built from this repository: every URL the CLI
// builds on the server URL must be a route main.go registers, and the login
// must be the RFC 8628 device grant the backend serves. Measured 2026-09-22:
// the CLI posted its authorization code to /api/v1/auth/token and opened
// /auth/login with PKCE parameters; no backend in this repository registers
// the first, and the dashboard ignores the second, so no self-hosted stack
// completed the login.
//
// The routes are read from the table the server mounts, not from the text of
// main.go, so a route registered through a helper (registerSDKAPIRoutes,
// registerBootstrapTokenRoutes, registerOnboardingTelemetryRoutes) counts like
// any other. The paths are read from every module of the SDK package.
func TestSDKLoginRoutesContract(t *testing.T) {
	table := serverRouteTable(t)
	registered := func(route string) bool {
		method, path, _ := strings.Cut(route, " ")
		return answers(table, method, path)
	}
	require.True(t, registered("POST /api/v1/auth/login/local"), "census sanity: the local login route must be found")
	require.True(t, registered("POST /api/v1/auth/refresh"), "census sanity: the refresh route must be found")
	require.False(t, registered("POST /api/v1/auth/token"), "census sanity: a route no backend registers must not be found")

	t.Run("backend serves the device grant", func(t *testing.T) {
		for _, r := range []string{
			"POST /api/v1/oauth/device/code",
			"POST /api/v1/oauth/device/token",
			"POST /api/v1/oauth/device/approve",
			"GET /api/v1/oauth/device/verify",
		} {
			assert.True(t, registered(r), "%s must be registered", r)
		}
	})

	t.Run("the census reads the routes main.go mounts through a helper", func(t *testing.T) {
		mainSrc := aim03ReadRepoFile(t, "apps/backend/cmd/server/main.go")
		for _, helper := range []string{"registerSDKAPIRoutes(", "registerBootstrapTokenRoutes(", "registerOnboardingTelemetryRoutes("} {
			require.True(t, strings.Contains(mainSrc, "\t"+helper), "main.go must call %s)", helper)
		}

		helperRoutes := []string{
			"POST /api/v1" + bootstrapTokenMintPath,
			"POST /api/v1" + bootstrapTokenRevokePath,
			"POST /api/v1" + bootstrapTokenExchangePath,
			"POST /api/v1" + onboardingEventsPath,
			"GET /api/v1" + onboardingMetricsPath,
		}
		sdkAPI := sdkAPIRouteTable(sdkAPITestHandlers(func(fiber.Ctx) error { return nil }))
		require.NotEmpty(t, sdkAPI, "the SDK-API route table must not be empty")
		for _, route := range sdkAPI {
			helperRoutes = append(helperRoutes, route.Method+" "+route.FullPath())
		}
		for _, r := range helperRoutes {
			assert.True(t, registered(r), "%s is mounted through a helper and must be in the census", r)
		}
	})

	cli := aim03ReadRepoFile(t, sdkPackageDir+"/cli.py")
	oauth := aim03ReadRepoFile(t, sdkPackageDir+"/oauth.py")
	t.Run("every path the SDK builds on the server URL is a registered route", func(t *testing.T) {
		known := map[string]bool{}
		for _, off := range sdkPathsOffTheRouteTable {
			known[off.module+" "+off.path] = true
		}

		found := map[string]int{}
		unregistered := map[string]bool{}
		total := 0
		for _, file := range sdkModules(t) {
			module := strings.TrimPrefix(file, sdkPackageDir+"/")
			for _, p := range sdkServerPaths(file, aim03ReadRepoFile(t, file)) {
				found[module]++
				total++
				if answers(table, "", p.path) {
					continue
				}
				unregistered[module+" "+p.path] = true
				if !known[module+" "+p.path] {
					t.Errorf("%s:%d builds %s on the server URL, which the server does not register", p.file, p.line, p.path)
				}
			}
		}
		for _, off := range sdkPathsOffTheRouteTable {
			assert.True(t, unregistered[off.module+" "+off.path],
				"%s no longer builds %s, or the server registers it now: remove it from sdkPathsOffTheRouteTable (%s)",
				off.module, off.path, off.what)
		}
		for _, module := range []string{"cli.py", "oauth.py", "client.py", "a2a.py", "demo.py"} {
			assert.NotZero(t, found[module], "%s must build at least one path on the server URL", module)
		}
		require.Greater(t, total, 50, "the SDK builds far more paths than the scan found; the scan is broken, not the SDK")
		t.Logf("checked %d paths in %d modules", total, len(found))
	})
	t.Run("the login is the device grant by name", func(t *testing.T) {
		assert.True(t, strings.Contains(cli, "/api/v1/oauth/device/code"), "login must request a device code")
		assert.True(t, strings.Contains(cli, "/api/v1/oauth/device/token"), "login must poll the device token route")
		assert.True(t, strings.Contains(cli, "urn:ietf:params:oauth:grant-type:device_code"), "RFC 8628 grant type")
		assert.False(t, strings.Contains(cli, "/api/v1/auth/token"), "the exchange route no backend registers must be gone")
		assert.False(t, strings.Contains(cli, "code_challenge"), "PKCE has no server side in this repository")
		assert.True(t, strings.Contains(oauth, "/api/v1/auth/logout"), "logout must post the logout route the backend serves")
		assert.False(t, strings.Contains(oauth, "/api/v1/auth/revoke"), "no backend registers /api/v1/auth/revoke")
	})
}

// TestDeviceVerificationPageAndURI pins the two dashboard-side facts the device
// grant needs on a self-hosted stack: the verification URI the backend hands the
// CLI is rooted on the dashboard origin, and the dashboard serves the page
// before login so the user code on the URL survives.
func TestDeviceVerificationPageAndURI(t *testing.T) {
	t.Run("verification URI is rooted on the dashboard origin", func(t *testing.T) {
		mainSrc := aim03ReadRepoFile(t, "apps/backend/cmd/server/main.go")
		ctor := regexp.MustCompile(`(?s)application\.NewDeviceAuthService\((.*?)\)\n`).FindStringSubmatch(mainSrc)
		require.Len(t, ctor, 2, "the NewDeviceAuthService call must be found")
		assert.True(t, strings.Contains(ctor[1], "cfg.Server.FrontendURL"),
			"the device verification URI must be built from FRONTEND_URL (the dashboard), not the API origin")
	})
	t.Run("/device renders outside the gated shell", func(t *testing.T) {
		// The dashboard has no edge file: a page is gated only by the dashboard shell's
		// layout, so a page outside that shell renders before login. /device stays
		// outside it so the user code on the URL survives the redirect to sign-in.
		root := aim03RepoRoot(t)
		for _, edge := range []string{"apps/web/middleware.ts", "apps/web/proxy.ts"} {
			_, err := os.Stat(filepath.Join(root, edge))
			assert.True(t, os.IsNotExist(err), "%s must not exist: routing decisions belong to the dashboard shell", edge)
		}
		layouts, err := filepath.Glob(filepath.Join(root, "apps", "web", "app", "device", "layout.*"))
		require.NoError(t, err)
		assert.Empty(t, layouts, "/device must have no layout of its own that could gate it before login")
	})
	t.Run("the dashboard has a device approval page that approves through the api client", func(t *testing.T) {
		page := aim03ReadRepoFile(t, "apps/web/app/device/page.tsx")
		assert.True(t, strings.Contains(page, "api.approveDevice("), "the page must approve through the api client, not a raw fetch")
		assert.False(t, strings.Contains(page, "document.cookie"), "the page reads the session from the api client, never from document.cookie")
		client := aim03ReadRepoFile(t, "apps/web/lib/api.ts")
		assert.True(t, strings.Contains(client, `"/api/v1/oauth/device/approve"`), "the api client must post the approve route")
		assert.True(t, strings.Contains(client, "/api/v1/oauth/device/verify?user_code="), "the api client must read the pending request")
	})
}
