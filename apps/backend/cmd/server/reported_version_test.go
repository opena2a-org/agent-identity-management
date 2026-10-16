package main

// GET /health/ready and GET /api/v1/status report one version string: the
// bare semver the image tag carries. The publish workflow stamps
// main.buildVersion with the git ref name, so a release built from the tag
// platform-v1.1.0 arrives here as "platform-v1.1.0" while the image is tagged
// 1.1.0; the server reduces it to 1.1.0. A value that is not a release tag (a
// commit sha, "dev") is reported as stamped, and an unstamped build reports
// null on both routes.

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/config"
)

const reportedVersionTestSha = "89abcdef0123456789abcdef0123456789abcdef"

func TestReportedVersionIsTheBareSemverTheImageTagCarries(t *testing.T) {
	cases := []struct {
		name    string
		stamped string
		want    string
	}{
		{"platform release tag", "platform-v1.1.0", "1.1.0"},
		{"bare semver", "1.1.0", "1.1.0"},
		{"commit sha passes through", reportedVersionTestSha, reportedVersionTestSha},
		{"legacy v tag", "v1.1.0", "1.1.0"},
		{"pre-release tag keeps its suffix", "platform-v1.2.0-rc.1", "1.2.0-rc.1"},
		{"build metadata is kept", "platform-v1.2.0+build.7", "1.2.0+build.7"},
		{"Dockerfile default passes through", "dev", "dev"},
		{"a non-platform tag is not reduced", "sdk-ts-v2.0.0", "sdk-ts-v2.0.0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := reportedVersion(tc.stamped)
			require.NotNil(t, got, "a stamped build must report a version")
			assert.Equal(t, tc.want, *got)
		})
	}

	t.Run("unstamped build reports null", func(t *testing.T) {
		assert.Nil(t, reportedVersion(""))
	})
}

func reportedVersionGet(t *testing.T, app *fiber.App, path string) map[string]any {
	t.Helper()
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, path, nil),
		fiber.TestConfig{Timeout: 10 * time.Second, FailOnTimeout: true})
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, "%s: %s", path, body)
	var m map[string]any
	require.NoError(t, json.Unmarshal(body, &m))
	return m
}

func reportedVersionApp() *fiber.App {
	app := fiber.New()
	ok := func(context.Context) error { return nil }
	app.Get("/health/ready", newHealthReadyHandler(ok, nil))
	app.Get("/api/v1/status", newStatusHandler(func() error { return nil }, nil, false, config.LoadServer().Environment, time.Now()))
	return app
}

func TestHealthReadyAndStatusReportTheSameVersion(t *testing.T) {
	prev := buildVersion
	t.Cleanup(func() { buildVersion = prev })

	t.Run("a build stamped with the platform tag reports the bare semver on both routes", func(t *testing.T) {
		buildVersion = "platform-v1.1.0"
		app := reportedVersionApp()
		ready := reportedVersionGet(t, app, "/health/ready")
		status := reportedVersionGet(t, app, "/api/v1/status")
		assert.Equal(t, "1.1.0", ready["version"])
		assert.Equal(t, "1.1.0", status["version"])
	})

	t.Run("a build stamped with a commit sha reports it on both routes", func(t *testing.T) {
		buildVersion = reportedVersionTestSha
		app := reportedVersionApp()
		assert.Equal(t, reportedVersionTestSha, reportedVersionGet(t, app, "/health/ready")["version"])
		assert.Equal(t, reportedVersionTestSha, reportedVersionGet(t, app, "/api/v1/status")["version"])
	})

	t.Run("an unstamped build reports null on both routes", func(t *testing.T) {
		buildVersion = ""
		app := reportedVersionApp()
		ready := reportedVersionGet(t, app, "/health/ready")
		status := reportedVersionGet(t, app, "/api/v1/status")
		v, present := ready["version"]
		require.True(t, present)
		assert.Nil(t, v)
		v, present = status["version"]
		require.True(t, present)
		assert.Nil(t, v)
	})

	t.Run("the status body keeps its other fields", func(t *testing.T) {
		buildVersion = "platform-v1.1.0"
		status := reportedVersionGet(t, reportedVersionApp(), "/api/v1/status")
		assert.Equal(t, "operational", status["status"])
		assert.Equal(t, "development", status["environment"])
		services, ok := status["services"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "healthy", services["database"])
		assert.Equal(t, "not configured", services["redis"])
		assert.Equal(t, "unavailable", services["email"])
		_, ok = status["uptime"].(float64)
		assert.True(t, ok, "uptime is a number of seconds")
	})
}

// The status route must take its version from the build stamp, never from a
// literal written into the server source.
func TestStatusRouteCarriesNoVersionLiteral(t *testing.T) {
	src, err := os.ReadFile(aim07Path(t, "main.go"))
	require.NoError(t, err)
	literal := regexp.MustCompile(`"version":\s+"[0-9]+\.[0-9]+\.[0-9]+"`)
	assert.Empty(t, literal.FindAllString(string(src), -1),
		"main.go answers a fixed version string; the routes must report the stamped build version")
}
