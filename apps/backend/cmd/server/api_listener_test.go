package main

import (
	"io"
	"net"
	"net/http"
	"net/netip"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/config"
)

// unsetHost removes HOST from the environment for the length of the test.
func unsetHost(t *testing.T) {
	t.Helper()
	t.Setenv("HOST", "")
	require.NoError(t, os.Unsetenv("HOST"))
}

// requireLoopbackAddr fails the test unless addr's host is a loopback IP
// literal. It runs before any listener starts, so a non-loopback address is
// never bound.
func requireLoopbackAddr(t *testing.T, addr string) {
	t.Helper()
	host, _, err := net.SplitHostPort(addr)
	require.NoError(t, err, "the API listener address %q", addr)
	ip, err := netip.ParseAddr(host)
	require.NoError(t, err, "the API listener address %q has no IP literal host", addr)
	require.True(t, ip.IsLoopback(), "the API listener address %q is not loopback", addr)
}

// startAPIListener starts app as main starts the API: app.Listen on
// apiListenAddr(server) with apiListenConfig(server.Host). It returns the
// address the listener bound and shuts the app down when the test ends.
func startAPIListener(t *testing.T, app *fiber.App, server config.ServerConfig) net.Addr {
	t.Helper()
	addr := apiListenAddr(server)
	requireLoopbackAddr(t, addr)

	cfg := apiListenConfig(server.Host)
	cfg.DisableStartupMessage = true
	bound := make(chan net.Addr, 1)
	cfg.ListenerAddrFunc = func(a net.Addr) { bound <- a }
	errs := make(chan error, 1)
	go func() { errs <- app.Listen(addr, cfg) }()

	select {
	case a := <-bound:
		t.Cleanup(func() { _ = app.Shutdown() })
		return a
	case err := <-errs:
		t.Fatalf("app.Listen(%q): %v", addr, err)
	case <-time.After(5 * time.Second):
		t.Fatalf("app.Listen(%q) did not bind within 5s", addr)
	}
	return nil
}

func healthApp() *fiber.App {
	app := fiber.New()
	app.Get("/health", func(c fiber.Ctx) error { return c.SendString("ok") })
	return app
}

// requireServes fails the test unless GET /health on bound answers 200 "ok".
func requireServes(t *testing.T, bound net.Addr) {
	t.Helper()
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("http://" + bound.String() + "/health")
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "ok", string(body))
}

// TestAPIListener_BindsLoopbackWhenHostIsUnsetOutsideAContainer starts the API
// listener with HOST unset and no container detected, and checks the address
// it bound is loopback and answers there.
func TestAPIListener_BindsLoopbackWhenHostIsUnsetOutsideAContainer(t *testing.T) {
	unsetHost(t)
	server := config.ServerConfig{Host: config.ListenHost(func() bool { return false }), Port: "0"}
	assert.Equal(t, "127.0.0.1", server.Host)

	bound := startAPIListener(t, healthApp(), server)
	tcp, ok := bound.(*net.TCPAddr)
	require.True(t, ok, "bound address %v is not TCP", bound)
	assert.True(t, tcp.IP.Equal(net.IPv4(127, 0, 0, 1)), "the listener bound %v, want 127.0.0.1", bound)
	requireServes(t, bound)
}

// TestAPIListener_BindsAnIPv6LoopbackHost starts the API listener with
// HOST=::1, which listens on IPv6.
func TestAPIListener_BindsAnIPv6LoopbackHost(t *testing.T) {
	probe, err := net.Listen("tcp6", "[::1]:0")
	if err != nil {
		t.Skipf("this host has no IPv6 loopback: %v", err)
	}
	require.NoError(t, probe.Close())

	t.Setenv("HOST", "::1")
	server := config.ServerConfig{Host: config.ListenHost(func() bool { return false }), Port: "0"}
	assert.Equal(t, fiber.NetworkTCP6, apiListenConfig(server.Host).ListenerNetwork)

	bound := startAPIListener(t, healthApp(), server)
	tcp, ok := bound.(*net.TCPAddr)
	require.True(t, ok, "bound address %v is not TCP", bound)
	assert.True(t, tcp.IP.Equal(net.IPv6loopback), "the listener bound %v, want ::1", bound)
	requireServes(t, bound)
}

// TestAPIListenAddr checks the address and network the API listener is given,
// without listening on any of them.
func TestAPIListenAddr(t *testing.T) {
	cases := []struct {
		name        string
		host        string
		inContainer bool
		wantAddr    string
		wantNetwork string
	}{
		{name: "HOST unset outside a container", wantAddr: "127.0.0.1:8080", wantNetwork: fiber.NetworkTCP4},
		{name: "HOST unset in a container", inContainer: true, wantAddr: "0.0.0.0:8080", wantNetwork: fiber.NetworkTCP4},
		{name: "HOST set in a container", host: "127.0.0.1", inContainer: true, wantAddr: "127.0.0.1:8080", wantNetwork: fiber.NetworkTCP4},
		{name: "HOST set to an IPv6 literal", host: "::1", wantAddr: "[::1]:8080", wantNetwork: fiber.NetworkTCP6},
		{name: "HOST set to a host name", host: "localhost", wantAddr: "localhost:8080", wantNetwork: fiber.NetworkTCP4},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.host == "" {
				unsetHost(t)
			} else {
				t.Setenv("HOST", tc.host)
			}
			inContainer := tc.inContainer
			server := config.ServerConfig{Host: config.ListenHost(func() bool { return inContainer }), Port: "8080"}
			assert.Equal(t, tc.wantAddr, apiListenAddr(server))
			assert.Equal(t, tc.wantNetwork, apiListenConfig(server.Host).ListenerNetwork)
		})
	}
}

// TestStartupSummary_EndsWithTheAPIListenerAddress checks that the startup
// summary keeps its port line first and ends with the address the API
// listener binds, which names HOST.
func TestStartupSummary_EndsWithTheAPIListenerAddress(t *testing.T) {
	const hostNote = " (HOST sets the address; unset, it is 0.0.0.0 in a container and 127.0.0.1 outside one)"
	cfg := &config.Config{Server: config.ServerConfig{Host: "127.0.0.1", Port: "8080"}}
	lines := startupSummary(cfg, nil)
	require.Len(t, lines, 4)
	assert.Equal(t, "Agent Identity Management API starting on port 8080", lines[0])
	assert.Equal(t, "API listener: 127.0.0.1:8080"+hostNote, lines[3])

	cfg.Server.Host = "::1"
	assert.Equal(t, "API listener: [::1]:8080"+hostNote, startupSummary(cfg, nil)[3])
}

// TestMainStartsTheAPIListenerOnItsConfiguredAddress pins main to the address
// and network the tests above check: one apiListenAddr(cfg.Server) value,
// passed to the metrics wiring and to app.Listen.
func TestMainStartsTheAPIListenerOnItsConfiguredAddress(t *testing.T) {
	mainSrc := aim03ReadRepoFile(t, "apps/backend/cmd/server/main.go")
	assert.Equal(t, 1, strings.Count(mainSrc, "apiAddr := apiListenAddr(cfg.Server)"))
	assert.Equal(t, 1, strings.Count(mainSrc, "app.Listen("), "main starts one listener with app.Listen")
	assert.Contains(t, mainSrc, "app.Listen(apiAddr, apiListenConfig(cfg.Server.Host))")
	assert.Contains(t, mainSrc, "wireMetrics(app, apiAddr, ")
}
