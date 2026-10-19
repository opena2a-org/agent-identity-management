package metrics

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// requestSeries returns every aim_http_requests_total series in the registry,
// keyed "method path status", with its counter value.
func requestSeries(t *testing.T) map[string]float64 {
	t.Helper()
	families, err := registry.Gather()
	require.NoError(t, err)
	series := map[string]float64{}
	for _, mf := range families {
		if mf.GetName() != "aim_http_requests_total" {
			continue
		}
		for _, m := range mf.GetMetric() {
			labels := map[string]string{}
			for _, lp := range m.GetLabel() {
				labels[lp.GetName()] = lp.GetValue()
			}
			key := labels["method"] + " " + labels["path"] + " " + labels["status"]
			series[key] = m.GetCounter().GetValue()
		}
	}
	return series
}

// newMetricsTestApp builds an app shaped like the server: the metrics
// middleware runs globally, a group refuses requests in a Use middleware the
// way the auth groups do, handlers either answer or return an error, and
// routes take path parameters and a wildcard, one of them behind a
// middleware on its own registration line, as the DID resolver's rate
// limiter is.
func newMetricsTestApp() *fiber.App {
	app := fiber.New()
	app.Use(PrometheusMiddleware())
	app.Get("/mwtest/ok", func(c fiber.Ctx) error {
		return c.SendString("ok")
	})
	app.Get("/mwtest/forbidden", func(c fiber.Ctx) error {
		return fiber.ErrForbidden
	})
	app.Get("/mwtest/broken", func(c fiber.Ctx) error {
		return errors.New("plain error")
	})
	protected := app.Group("/mwtest/protected")
	protected.Use(func(c fiber.Ctx) error {
		return fiber.ErrUnauthorized
	})
	protected.Get("/:id", func(c fiber.Ctx) error {
		return c.SendString("never reached without credentials")
	})
	app.Get("/mwtest/things/:id", func(c fiber.Ctx) error {
		return c.SendString("ok")
	})
	app.Get("/mwtest/files/*", func(c fiber.Ctx) error {
		return c.SendString("ok")
	})
	api := app.Group("/mwtest/api")
	api.Get("/agents/:id/logs/:logId", func(c fiber.Ctx) error {
		return fiber.ErrNotFound
	})
	app.Get("/mwtest/limited/:id", func(c fiber.Ctx) error {
		return fiber.ErrTooManyRequests
	}, func(c fiber.Ctx) error {
		return c.SendString("never reached past the limiter")
	})
	return app
}

func doRequest(t *testing.T, app *fiber.App, method, path string) int {
	t.Helper()
	resp, err := app.Test(httptest.NewRequest(method, path, nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	return resp.StatusCode
}

// requireRecordedOnlyUnder sends one request and fails unless the client
// received wantStatus and the request was counted under the series wantKey
// ("method path status") and under no other.
func requireRecordedOnlyUnder(t *testing.T, app *fiber.App, method, path string, wantStatus int, wantKey string) {
	t.Helper()
	before := requestSeries(t)
	got := doRequest(t, app, method, path)
	require.Equal(t, wantStatus, got, "status the client received")

	after := requestSeries(t)
	assert.Equal(t, before[wantKey]+1, after[wantKey],
		"series %q should count the request; recorded series: %v", wantKey, after)
	for key, v := range after {
		if key != wantKey && v != before[key] {
			t.Errorf("request was also recorded under %q", key)
		}
	}
}

func TestPrometheusMiddlewareStatusLabelIsTheStatusTheClientReceived(t *testing.T) {
	app := newMetricsTestApp()

	cases := []struct {
		name       string
		path       string
		wantStatus int
		wantKey    string
	}{
		{"matched route answering 200", "/mwtest/ok", http.StatusOK, "GET /mwtest/ok 200"},
		{"handler returning a fiber error", "/mwtest/forbidden", http.StatusForbidden, "GET /mwtest/forbidden 403"},
		{"handler returning a plain error", "/mwtest/broken", http.StatusInternalServerError, "GET /mwtest/broken 500"},
		{"unmatched path", "/probe-chosen-label-zz9", http.StatusNotFound, "GET unmatched 404"},
		{"refused by a group middleware", "/mwtest/protected/abc", http.StatusUnauthorized, "GET unmatched 401"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			requireRecordedOnlyUnder(t, app, http.MethodGet, tc.path, tc.wantStatus, tc.wantKey)
		})
	}
}

// A request a route handler served is counted under that route's path as it
// was registered, group prefix included, not under the path the caller sent.
// A request no route handler served stays unmatched, even when its path fits
// a route registered for another method.
func TestPrometheusMiddlewarePathLabelIsTheRouteThatServedTheRequest(t *testing.T) {
	app := newMetricsTestApp()

	cases := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		wantKey    string
	}{
		{"path parameter", http.MethodGet, "/mwtest/things/abc123xyz", http.StatusOK,
			"GET /mwtest/things/:id 200"},
		{"wildcard", http.MethodGet, "/mwtest/files/did:example:caller-chosen", http.StatusOK,
			"GET /mwtest/files/* 200"},
		{"group route with two parameters, answering 404", http.MethodGet,
			"/mwtest/api/agents/550e8400-e29b-41d4-a716-446655440000/logs/77", http.StatusNotFound,
			"GET /mwtest/api/agents/:id/logs/:logId 404"},
		{"refused by a middleware on the route's own line", http.MethodGet, "/mwtest/limited/caller-chosen",
			http.StatusTooManyRequests, "GET /mwtest/limited/:id 429"},
		{"a method the route does not accept", http.MethodPost, "/mwtest/things/abc123xyz",
			http.StatusMethodNotAllowed, "POST unmatched 405"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			requireRecordedOnlyUnder(t, app, tc.method, tc.path, tc.wantStatus, tc.wantKey)
		})
	}
}

// servedExposition returns the body PrometheusHandler serves for the
// package registry.
func servedExposition(t *testing.T) string {
	t.Helper()
	app := fiber.New()
	app.Get("/metrics", PrometheusHandler())
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/metrics", nil))
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, resp.Body.Close())
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	return string(body)
}

// The values in a path are the caller's to choose. Requests that put 25
// distinct values on each of four routes share each route's series, and none
// of the values reaches the exposition /metrics serves.
func TestPrometheusMiddlewareCallerChosenPathValuesAddNoSeries(t *testing.T) {
	app := newMetricsTestApp()

	routes := []struct {
		prefix, suffix string
		wantStatus     int
	}{
		{"/mwtest/things/", "", http.StatusOK},
		{"/mwtest/files/", "", http.StatusOK},
		{"/mwtest/api/agents/", "/logs/77", http.StatusNotFound},
		{"/mwtest/limited/", "", http.StatusTooManyRequests},
	}

	// The first request on each route may create the route's series.
	for _, r := range routes {
		require.Equal(t, r.wantStatus, doRequest(t, app, http.MethodGet, r.prefix+"first-value"+r.suffix))
	}
	baseline := requestSeries(t)

	var chosen []string
	for i := 0; i < 25; i++ {
		for j, r := range routes {
			value := fmt.Sprintf("caller-chosen-%d-%d", j, i)
			chosen = append(chosen, value)
			require.Equal(t, r.wantStatus, doRequest(t, app, http.MethodGet, r.prefix+value+r.suffix))
		}
	}

	after := requestSeries(t)
	assert.Equal(t, len(baseline), len(after),
		"distinct values in a served route's path must not add series")
	for _, key := range []string{
		"GET /mwtest/things/:id 200",
		"GET /mwtest/files/* 200",
		"GET /mwtest/api/agents/:id/logs/:logId 404",
		"GET /mwtest/limited/:id 429",
	} {
		assert.Equal(t, baseline[key]+25, after[key], "series %q should count the route's 25 requests", key)
	}

	exposition := servedExposition(t)
	require.Contains(t, exposition, `path="/mwtest/things/:id"`, "positive control: the route's series is not served")
	for _, value := range chosen {
		assert.NotContains(t, exposition, value, "a value a caller put in a path is served on /metrics")
	}
}

// Fiber reuses a request's context for later requests. Mixed-method traffic
// through one app, sequential and concurrent, must leave every recorded
// method label a valid method and the registry gatherable.
func TestPrometheusMiddlewareLabelsSurviveContextReuse(t *testing.T) {
	app := fiber.New()
	app.Use(PrometheusMiddleware())
	methods := []string{
		http.MethodGet, http.MethodPost, http.MethodDelete,
		http.MethodPatch, http.MethodPut, http.MethodOptions,
	}
	for _, m := range methods {
		app.Add([]string{m}, "/reuse/:id", func(c fiber.Ctx) error { return c.SendString("ok") })
	}

	// reuseCounts sums the requests counted under the route and fails on a
	// method label that is not a valid method. requestSeries fails the test
	// when the registry does not gather.
	reuseCounts := func() float64 {
		total := 0.0
		for key, v := range requestSeries(t) {
			fields := strings.Fields(key)
			if len(fields) != 3 || fields[1] != "/reuse/:id" {
				continue
			}
			total += v
			assert.True(t, validHTTPMethods[fields[0]], "corrupted method label in series %q", key)
		}
		return total
	}
	before := reuseCounts()

	send := func(method string, i int) {
		resp, err := app.Test(httptest.NewRequest(method, fmt.Sprintf("/reuse/%d", i), nil),
			fiber.TestConfig{Timeout: 10 * time.Second, FailOnTimeout: true})
		if err != nil {
			t.Errorf("app.Test(%s): %v", method, err)
			return
		}
		if err := resp.Body.Close(); err != nil {
			t.Errorf("close the %s response body: %v", method, err)
		}
	}
	for i := 0; i < 200; i++ {
		send(methods[i%len(methods)], i)
	}
	var wg sync.WaitGroup
	for i := 0; i < 400; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			send(methods[(i*7)%len(methods)], i)
		}(i)
	}
	wg.Wait()

	assert.Equal(t, before+600, reuseCounts(), "the 600 requests counted under /reuse/:id")
}

func TestPrometheusMiddlewareStatusLabelFollowsTheConfiguredErrorHandler(t *testing.T) {
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c fiber.Ctx, err error) error {
			return c.Status(http.StatusServiceUnavailable).SendString("unavailable")
		},
	})
	app.Use(PrometheusMiddleware())
	app.Get("/mwtest/custom-handler", func(c fiber.Ctx) error {
		return fiber.ErrForbidden
	})

	before := requestSeries(t)
	require.Equal(t, http.StatusServiceUnavailable, doRequest(t, app, http.MethodGet, "/mwtest/custom-handler"))
	after := requestSeries(t)

	assert.Equal(t, before["GET /mwtest/custom-handler 503"]+1, after["GET /mwtest/custom-handler 503"])
	assert.Equal(t, before["GET /mwtest/custom-handler 403"], after["GET /mwtest/custom-handler 403"])
}

func TestPrometheusMiddlewareUnmatchedPathsAddNoSeries(t *testing.T) {
	app := newMetricsTestApp()

	// The first unmatched request may create the shared series.
	require.Equal(t, http.StatusNotFound, doRequest(t, app, http.MethodGet, "/unmatched-first"))
	require.Equal(t, http.StatusUnauthorized, doRequest(t, app, http.MethodGet, "/mwtest/protected/first"))
	baseline := len(requestSeries(t))

	for i := 0; i < 25; i++ {
		require.Equal(t, http.StatusNotFound,
			doRequest(t, app, http.MethodGet, fmt.Sprintf("/caller-chosen-%d/segment-%d", i, i*7)))
		require.Equal(t, http.StatusUnauthorized,
			doRequest(t, app, http.MethodGet, fmt.Sprintf("/mwtest/protected/caller-chosen-%d", i)))
	}

	assert.Equal(t, baseline, len(requestSeries(t)),
		"distinct unmatched or refused paths must not add series")
}
