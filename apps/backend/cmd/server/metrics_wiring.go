package main

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/gofiber/fiber/v3"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/metrics"
)

// Metrics wiring: which listener serves GET /metrics, on which address, behind
// which check, and how it stops.
//
//   - A dedicated listener (METRICS_LISTEN_ADDR, default 127.0.0.1:9464)
//     serves only GET /metrics. Without a token it is open, so it may bind a
//     loopback address only; any other address without a token refuses to
//     start. With a token it requires the bearer on every address.
//   - The API listener serves /metrics only when a token is configured, behind
//     the bearer check, and logs every request to it. Without a token the API
//     has no /metrics route and answers 404.
//   - If the dedicated listener cannot bind, the server logs an error and keeps
//     serving the API. Nothing mounts /metrics on the API listener instead.
//
// A METRICS_AUTH_TOKEN shorter than metricsTokenMinLength characters is
// treated as unset.

const (
	metricsListenAddrEnv = "METRICS_LISTEN_ADDR"

	// defaultMetricsListenAddr is loopback on the port the OpenTelemetry
	// Prometheus exporter also defaults to; 9090 is Prometheus itself.
	defaultMetricsListenAddr = "127.0.0.1:9464"

	// metricsTokenMinLength is the shortest METRICS_AUTH_TOKEN accepted.
	// `openssl rand -hex 32` yields 64 characters.
	metricsTokenMinLength = 32

	// metricsShutdownTimeout bounds how long Stop waits for in-flight scrapes.
	metricsShutdownTimeout = 2 * time.Second

	metricsPath = "/metrics"
)

// metricsListenAddr returns METRICS_LISTEN_ADDR, or defaultMetricsListenAddr
// when it is unset.
func metricsListenAddr() string {
	if v := os.Getenv(metricsListenAddrEnv); v != "" {
		return v
	}
	return defaultMetricsListenAddr
}

// effectiveMetricsToken returns the configured token when it is at least
// metricsTokenMinLength characters long, and "" (unset) otherwise. Every
// bearer check and bind decision takes its result, never the raw value.
func effectiveMetricsToken(raw string) string {
	if utf8.RuneCountInString(raw) >= metricsTokenMinLength {
		return raw
	}
	return ""
}

// isLoopbackAddr reports whether the host of a host:port address is a
// loopback IP literal. An empty host, an unspecified address (0.0.0.0, ::)
// and any hostname, localhost included, are not loopback: what they bind is
// decided by the resolver and the kernel, not by this check.
func isLoopbackAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// metricsServer is the dedicated metrics listener. Addr is nil whenever it is
// not serving: after a failed bind, after Stop, or after its serve loop ended.
type metricsServer struct {
	app  *fiber.App
	done chan struct{} // closed when the serve loop has returned

	mu       sync.Mutex
	addr     net.Addr
	ln       net.Listener
	stopping bool
}

// Addr returns the bound address, or nil when the listener is not serving.
func (m *metricsServer) Addr() net.Addr {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.addr
}

// Stop shuts the listener down, waiting at most metricsShutdownTimeout for
// in-flight requests. It returns nil when the listener was not serving and the
// drain error when the bound was exceeded. It never exits the process.
func (m *metricsServer) Stop() error {
	m.mu.Lock()
	if m.addr == nil || m.stopping {
		m.mu.Unlock()
		return nil
	}
	m.stopping = true
	ln := m.ln
	m.mu.Unlock()

	err := m.app.ShutdownWithTimeout(metricsShutdownTimeout)
	// Shutdown closes the listener once the serve loop holds it; closing it
	// here too covers a Stop that runs before the loop has started.
	_ = ln.Close()
	<-m.done

	m.mu.Lock()
	m.addr = nil
	m.mu.Unlock()
	return err
}

// serve runs the app on ln. A return that Stop did not cause logs one error
// line and marks the listener not serving; the API is unaffected.
func (m *metricsServer) serve(ln net.Listener) {
	defer close(m.done)
	err := m.app.Listener(ln, fiber.ListenConfig{DisableStartupMessage: true})

	m.mu.Lock()
	stopping := m.stopping
	m.addr = nil
	m.mu.Unlock()

	if !stopping {
		log.Printf("ERROR: metrics listener %s stopped: %v; metrics are not served, the API keeps serving", ln.Addr(), err)
	}
}

// newMetricsApp builds the dedicated listener's app: GET /metrics (and the
// HEAD Fiber derives from it) and nothing else, behind the bearer check when
// token is set.
func newMetricsApp(token string) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:      "AIM metrics",
		ServerHeader: "AIM/1.0",
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	})
	if token != "" {
		app.Get(metricsPath, metrics.MetricsAuthMiddleware(token), metrics.PrometheusHandler())
	} else {
		app.Get(metricsPath, metrics.PrometheusHandler())
	}
	return app
}

// metricsAccessLog records each /metrics request on the API listener, served
// or refused: time, status, method, path and the TCP peer address. The API app
// trusts no proxy header, so c.IP() is the peer. No header value is written.
func metricsAccessLog() fiber.Handler {
	return func(c fiber.Ctx) error {
		err := c.Next()
		status := c.Response().StatusCode()
		if err != nil {
			status = fiber.StatusInternalServerError
			var fe *fiber.Error
			if errors.As(err, &fe) {
				status = fe.Code
			}
		}
		log.Printf("metrics access: %s %d %s %s peer=%s", time.Now().UTC().Format(time.RFC3339), status, c.Method(), c.Path(), c.IP())
		return err
	}
}

// wireMetrics mounts the metrics surfaces. api is the API listener's app and
// apiAddr the address it listens on (used in log lines only); metricsAddr is
// the dedicated listener's host:port; rawToken is METRICS_AUTH_TOKEN as
// configured. It must run before the API's global middleware is added, so the
// API route is not itself recorded by PrometheusMiddleware.
//
// It makes one bind attempt. The only error is the refusal of a non-loopback
// metricsAddr without a token, returned before any socket is opened. A bind
// failure is logged and returns a nil error with a server whose Addr is nil.
func wireMetrics(api *fiber.App, apiAddr, metricsAddr, rawToken string) (*metricsServer, error) {
	token := effectiveMetricsToken(rawToken)
	if rawToken != "" && token == "" {
		log.Printf("ERROR: METRICS_AUTH_TOKEN is shorter than %d characters and is treated as unset; generate one with: openssl rand -hex 32", metricsTokenMinLength)
	}

	loopback := isLoopbackAddr(metricsAddr)
	if token == "" && !loopback {
		return nil, fmt.Errorf("%s=%s is not a loopback address and METRICS_AUTH_TOKEN is not set: the metrics listener serves off loopback only with a token (openssl rand -hex 32)", metricsListenAddrEnv, metricsAddr)
	}

	if token != "" {
		api.Get(metricsPath, metricsAccessLog(), metrics.MetricsAuthMiddleware(token), metrics.PrometheusHandler())
		log.Printf("metrics: API listener %s serves /metrics and requires METRICS_AUTH_TOKEN as a bearer token", apiAddr)
	} else {
		log.Printf("metrics: API listener %s does not serve /metrics (METRICS_AUTH_TOKEN is not set)", apiAddr)
	}

	m := &metricsServer{app: newMetricsApp(token), done: make(chan struct{})}
	ln, err := net.Listen("tcp", metricsAddr)
	if err != nil {
		close(m.done)
		log.Printf("ERROR: metrics listener %s=%s could not bind: %v; metrics are not served, the API keeps serving", metricsListenAddrEnv, metricsAddr, err)
		logMetricsTransition(metricsAddr)
		return m, nil
	}
	bound := ln.Addr()
	m.addr = bound
	m.ln = ln
	go m.serve(ln)

	switch {
	case token == "":
		log.Printf("metrics: listener %s serves /metrics without a token (loopback only)", bound)
	case loopback:
		log.Printf("metrics: listener %s serves /metrics and requires METRICS_AUTH_TOKEN as a bearer token", bound)
	default:
		log.Printf("WARN: metrics: listener %s is not on loopback; it requires METRICS_AUTH_TOKEN as a bearer token, and should also be reachable only from the scraper's network", bound)
	}
	logMetricsTransition(bound.String())
	return m, nil
}

// logMetricsTransition names the behaviour change and both ways to keep
// scraping.
func logMetricsTransition(metricsAddr string) {
	log.Printf("metrics: /metrics is no longer served on the API port without METRICS_AUTH_TOKEN. To scrape from another host, set METRICS_AUTH_TOKEN (openssl rand -hex 32) and give Prometheus the token with authorization.credentials_file. To scrape from this host, target http://%s/metrics", metricsAddr)
}
