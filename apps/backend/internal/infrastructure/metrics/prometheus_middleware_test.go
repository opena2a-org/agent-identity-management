package metrics

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

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
// way the auth groups do, and handlers either answer or return an error.
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
	return app
}

func doRequest(t *testing.T, app *fiber.App, method, path string) int {
	t.Helper()
	resp, err := app.Test(httptest.NewRequest(method, path, nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	return resp.StatusCode
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
			before := requestSeries(t)
			got := doRequest(t, app, http.MethodGet, tc.path)
			require.Equal(t, tc.wantStatus, got, "status the client received")

			after := requestSeries(t)
			assert.Equal(t, before[tc.wantKey]+1, after[tc.wantKey],
				"series %q should count the request; recorded series: %v", tc.wantKey, after)
			for key, v := range after {
				if key != tc.wantKey && v != before[key] {
					t.Errorf("request was also recorded under %q", key)
				}
			}
		})
	}
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
