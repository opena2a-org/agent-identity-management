package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testMetricsToken64 = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	testMetricsToken32 = "0123456789abcdef0123456789abcdef"
	testMetricsToken31 = "0123456789abcdef0123456789abcde"
)

// captureLog sends the standard logger to a buffer for the rest of the test.
// wireForTest wires metrics on a fresh API app with the dedicated listener on
// metricsAddr, and stops the listener when the test ends.
func wireForTest(t *testing.T, metricsAddr, rawToken string) (*fiber.App, *metricsServer) {
	t.Helper()
	api := fiber.New()
	m, err := wireMetrics(api, ":8080", metricsAddr, rawToken)
	require.NoError(t, err)
	require.NotNil(t, m)
	t.Cleanup(func() { _ = m.Stop() })
	return api, m
}

// apiGet sends GET path to the API app in process.
func apiGet(t *testing.T, api *fiber.App, path, authHeader string) int {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	resp, err := api.Test(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
	return resp.StatusCode
}

// listenerGet sends GET path to a real listener and returns status and body.
func listenerGet(t *testing.T, addr net.Addr, path, authHeader string) (int, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, "http://"+addr.String()+path, nil)
	require.NoError(t, err)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, string(body)
}

// freeLoopbackPort returns a port that was free on 127.0.0.1 a moment ago.
func freeLoopbackPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := ln.Addr().(*net.TCPAddr).Port
	require.NoError(t, ln.Close())
	return port
}

func TestAPIListenerHasNoMetricsRouteWithoutToken(t *testing.T) {
	captureLog(t)
	for _, raw := range []string{"", testMetricsToken31} {
		api, m := wireForTest(t, "127.0.0.1:0", raw)

		assert.Equal(t, fiber.StatusNotFound, apiGet(t, api, "/metrics", ""), "token of %d characters", len(raw))
		assert.Equal(t, fiber.StatusNotFound, apiGet(t, api, "/metrics", "Bearer "+raw), "token of %d characters", len(raw))
		for _, r := range api.GetRoutes() {
			assert.NotEqual(t, metricsPath, r.Path, "the API app registers %s %s", r.Method, r.Path)
		}

		// Control: the dedicated listener does serve the exposition.
		status, body := listenerGet(t, m.Addr(), "/metrics", "")
		assert.Equal(t, http.StatusOK, status)
		assert.Contains(t, body, "aim_active_agents")
	}
}

func TestMetricsListenAddrDefaultsToLoopback(t *testing.T) {
	t.Setenv(metricsListenAddrEnv, "")
	assert.Equal(t, "127.0.0.1:9464", metricsListenAddr())
	assert.True(t, isLoopbackAddr(metricsListenAddr()))

	t.Setenv(metricsListenAddrEnv, "0.0.0.0:9464")
	assert.Equal(t, "0.0.0.0:9464", metricsListenAddr())
}

func TestIsLoopbackAddrAcceptsOnlyLoopbackIPLiterals(t *testing.T) {
	for addr, want := range map[string]bool{
		"127.0.0.1:9464":   true,
		"127.0.0.2:9464":   true,
		"[::1]:9464":       true,
		"0.0.0.0:9464":     false,
		":9464":            false,
		"[::]:9464":        false,
		"localhost:9464":   false,
		"metrics.internal": false,
		"10.0.0.5:9464":    false,
		"127.0.0.1":        false,
	} {
		assert.Equal(t, want, isLoopbackAddr(addr), addr)
	}
}

func TestDedicatedListenerServesOnlyMetrics(t *testing.T) {
	// The routes are read from apps that are not serving: a serving app adds
	// its HEAD routes on its own goroutine.
	for _, tok := range []string{"", testMetricsToken64} {
		routes := newMetricsApp(tok).GetRoutes()
		require.NotEmpty(t, routes)
		for _, r := range routes {
			assert.Equal(t, metricsPath, r.Path, "the metrics app registers %s %s", r.Method, r.Path)
			assert.Contains(t, []string{http.MethodGet, http.MethodHead}, r.Method, "the metrics app registers %s %s", r.Method, r.Path)
		}
	}

	captureLog(t)
	_, m := wireForTest(t, "127.0.0.1:0", "")
	status, _ := listenerGet(t, m.Addr(), "/health", "")
	assert.Equal(t, http.StatusNotFound, status)
	status, _ = listenerGet(t, m.Addr(), "/api/v1/agents", "")
	assert.Equal(t, http.StatusNotFound, status)
}

func TestNonLoopbackMetricsAddrWithoutTokenRefusesToStart(t *testing.T) {
	captureLog(t)
	port := freeLoopbackPort(t)
	for _, raw := range []string{"", testMetricsToken31} {
		for _, addr := range []string{
			fmt.Sprintf("0.0.0.0:%d", port),
			fmt.Sprintf(":%d", port),
			fmt.Sprintf("[::]:%d", port),
			fmt.Sprintf("localhost:%d", port),
		} {
			api := fiber.New()
			m, err := wireMetrics(api, ":8080", addr, raw)
			require.Error(t, err, "%s with a token of %d characters", addr, len(raw))
			assert.Nil(t, m)
			assert.Contains(t, err.Error(), metricsListenAddrEnv)
			assert.Contains(t, err.Error(), "METRICS_AUTH_TOKEN")
			assert.Empty(t, api.GetRoutes(), "%s: a route was mounted before the refusal", addr)
		}
	}

	// Nothing was bound: the port is still free on the loopback address.
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	require.NoError(t, err, "the refused wiring left a socket open")
	_ = ln.Close()
}

func TestMetricsTokenGatesBothSurfaces(t *testing.T) {
	captureLog(t)
	for _, tok := range []string{testMetricsToken64, testMetricsToken32} {
		api, m := wireForTest(t, "127.0.0.1:0", tok)

		assert.Equal(t, fiber.StatusUnauthorized, apiGet(t, api, "/metrics", ""))
		assert.Equal(t, fiber.StatusUnauthorized, apiGet(t, api, "/metrics", "Bearer wrong"))
		assert.Equal(t, fiber.StatusOK, apiGet(t, api, "/metrics", "Bearer "+tok))

		status, _ := listenerGet(t, m.Addr(), "/metrics", "")
		assert.Equal(t, http.StatusUnauthorized, status)
		status, _ = listenerGet(t, m.Addr(), "/metrics", "Bearer wrong")
		assert.Equal(t, http.StatusUnauthorized, status)
		status, body := listenerGet(t, m.Addr(), "/metrics", "Bearer "+tok)
		assert.Equal(t, http.StatusOK, status)
		assert.Contains(t, body, "aim_active_agents")
	}
}

func TestShortMetricsTokenIsTreatedAsUnset(t *testing.T) {
	logs := captureLog(t)
	api, m := wireForTest(t, "127.0.0.1:0", testMetricsToken31)

	assert.Equal(t, fiber.StatusNotFound, apiGet(t, api, "/metrics", "Bearer "+testMetricsToken31))
	status, _ := listenerGet(t, m.Addr(), "/metrics", "")
	assert.Equal(t, http.StatusOK, status, "loopback listener without an effective token is open")
	assert.Contains(t, logs.String(), "ERROR: METRICS_AUTH_TOKEN is shorter than 32 characters and is treated as unset")
	assert.NotContains(t, logs.String(), testMetricsToken31[:8])
}

func TestMetricsBindFailureKeepsAPIServingWithoutFallback(t *testing.T) {
	held, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer held.Close()

	for _, raw := range []string{"", testMetricsToken64} {
		logs := captureLog(t)
		api := fiber.New()
		api.Get("/health", func(c fiber.Ctx) error { return c.SendString("ok") })

		m, err := wireMetrics(api, ":8080", held.Addr().String(), raw)
		require.NoError(t, err, "a bind failure must not stop the server")
		require.NotNil(t, m)
		assert.Nil(t, m.Addr(), "the listener reports serving after a failed bind")
		assert.NoError(t, m.Stop())

		assert.Equal(t, fiber.StatusOK, apiGet(t, api, "/health", ""))
		if raw == "" {
			assert.Equal(t, fiber.StatusNotFound, apiGet(t, api, "/metrics", ""), "metrics fell back to the API listener")
		} else {
			assert.Equal(t, fiber.StatusUnauthorized, apiGet(t, api, "/metrics", ""), "the API route lost its bearer check")
		}

		out := logs.String()
		assert.Equal(t, 1, strings.Count(out, "ERROR: metrics listener "), out)
		assert.Contains(t, out, "METRICS_LISTEN_ADDR="+held.Addr().String())
		assert.Contains(t, out, "the API keeps serving")
		assert.NotContains(t, out, "serves /metrics without a token")
	}
}

func TestMetricsStartupLinesNameEachSurfaceAndNeverTheToken(t *testing.T) {
	// Two tokens of different lengths must produce the same lines once the
	// bound address is masked: nothing derived from the token is logged.
	lines := func(tok string) string {
		logs := captureLog(t)
		_, m := wireForTest(t, "127.0.0.1:0", tok)
		return strings.ReplaceAll(logs.String(), m.Addr().String(), "ADDR")
	}
	longer := testMetricsToken64 + "ffff0000"
	a, b := lines(testMetricsToken64), lines(longer)
	assert.Equal(t, a, b)

	assert.Contains(t, a, "metrics: API listener :8080 serves /metrics and requires METRICS_AUTH_TOKEN")
	assert.Contains(t, a, "metrics: listener ADDR serves /metrics and requires METRICS_AUTH_TOKEN")
	assert.Contains(t, a, "no longer served on the API port without METRICS_AUTH_TOKEN")
	assert.Contains(t, a, "openssl rand -hex 32")
	assert.Contains(t, a, "credentials_file")
	assert.Contains(t, a, "http://ADDR/metrics")
	for _, tok := range []string{testMetricsToken64, longer} {
		for i := 0; i+8 <= len(tok); i++ {
			require.NotContains(t, a+b, tok[i:i+8])
		}
		assert.NotContains(t, a+b, strconv.Itoa(len(tok)))
	}

	open := func() string {
		logs := captureLog(t)
		_, m := wireForTest(t, "127.0.0.1:0", "")
		return strings.ReplaceAll(logs.String(), m.Addr().String(), "ADDR")
	}()
	assert.Contains(t, open, "metrics: API listener :8080 does not serve /metrics")
	assert.Contains(t, open, "metrics: listener ADDR serves /metrics without a token (loopback only)")
	assert.Contains(t, open, "no longer served on the API port without METRICS_AUTH_TOKEN")
}

func TestAPIMetricsRequestsLeaveAPeerAddressedRecord(t *testing.T) {
	logs := captureLog(t)
	api, _ := wireForTest(t, "127.0.0.1:0", testMetricsToken64)
	api.Get("/other", func(c fiber.Ctx) error { return c.SendString("ok") })

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go func() { _ = api.Listener(ln, fiber.ListenConfig{DisableStartupMessage: true}) }()
	t.Cleanup(func() { _ = api.ShutdownWithTimeout(time.Second) })

	send := func(path, auth string) int {
		req, err := http.NewRequest(http.MethodGet, "http://"+ln.Addr().String()+path, nil)
		require.NoError(t, err)
		req.Header.Set("X-Forwarded-For", "203.0.113.9")
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		var resp *http.Response
		require.Eventually(t, func() bool {
			resp, err = (&http.Client{Timeout: 2 * time.Second}).Do(req)
			return err == nil
		}, 5*time.Second, 20*time.Millisecond)
		_ = resp.Body.Close()
		return resp.StatusCode
	}

	assert.Equal(t, http.StatusUnauthorized, send("/metrics", ""))
	assert.Equal(t, http.StatusUnauthorized, send("/metrics", "Bearer wrong-token-value"))
	assert.Equal(t, http.StatusOK, send("/metrics", "Bearer "+testMetricsToken64))
	assert.Equal(t, http.StatusOK, send("/other", ""))

	var records []string
	for _, line := range strings.Split(logs.String(), "\n") {
		if strings.HasPrefix(line, "metrics access: ") {
			records = append(records, line)
		}
	}
	require.Len(t, records, 3, logs.String())
	for i, want := range []string{" 401 GET /metrics ", " 401 GET /metrics ", " 200 GET /metrics "} {
		assert.Contains(t, records[i], want)
		assert.Contains(t, records[i], "peer=127.0.0.1")
	}
	out := logs.String()
	assert.NotContains(t, out, "203.0.113.9", "a header-derived address was logged")
	assert.NotContains(t, out, "wrong-token-value")
	assert.NotContains(t, out, testMetricsToken64[:8])
}

func TestMetricsServerStopIsBoundedAndRepeatable(t *testing.T) {
	captureLog(t)
	_, m := wireForTest(t, "127.0.0.1:0", "")
	addr := m.Addr()
	require.NotNil(t, addr)

	start := time.Now()
	require.NoError(t, m.Stop())
	assert.Less(t, time.Since(start), metricsShutdownTimeout+time.Second)
	assert.Nil(t, m.Addr())
	assert.NoError(t, m.Stop(), "a second Stop")

	_, err := (&http.Client{Timeout: time.Second}).Get("http://" + addr.String() + "/metrics")
	assert.Error(t, err, "the listener still accepts connections after Stop")
}

// TestMainMountsMetricsOnlyThroughTheWiring guards main.go, whose main() cannot
// run in a test: the API app registers no /metrics route of its own, and the
// wiring is called once.
func TestMainMountsMetricsOnlyThroughTheWiring(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	mainPath := filepath.Join(filepath.Dir(thisFile), "main.go")
	f, err := parser.ParseFile(token.NewFileSet(), mainPath, nil, 0)
	require.NoError(t, err)

	wired := 0
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch fn := call.Fun.(type) {
		case *ast.Ident:
			if fn.Name == "wireMetrics" {
				wired++
			}
		case *ast.SelectorExpr:
			recv, ok := fn.X.(*ast.Ident)
			if !ok || recv.Name != "app" || len(call.Args) == 0 {
				return true
			}
			if lit, ok := call.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
				path, _ := strconv.Unquote(lit.Value)
				assert.NotEqual(t, metricsPath, path, "main.go registers app.%s(%q) on the API listener", fn.Sel.Name, path)
			}
		}
		return true
	})
	assert.Equal(t, 1, wired, "main.go calls wireMetrics once")
}
