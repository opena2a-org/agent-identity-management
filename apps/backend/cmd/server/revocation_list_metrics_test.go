package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/config"
)

// scrapeMetrics returns the exposition the metrics listener serves.
func scrapeMetrics(t *testing.T) string {
	t.Helper()
	resp, err := newMetricsApp("").Test(httptest.NewRequest(fiber.MethodGet, "/metrics", nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return string(body)
}

// unlabelledSample returns the value of the sample of the series name, which
// has no labels, and whether exposition carries one.
func unlabelledSample(exposition, name string) (float64, bool) {
	for _, line := range strings.Split(exposition, "\n") {
		if value, ok := strings.CutPrefix(line, name+" "); ok {
			v, err := strconv.ParseFloat(value, 64)
			return v, err == nil
		}
	}
	return 0, false
}

// With REGISTRY_URL set, /metrics carries the age and the entries of the list
// the refresher holds. Without REGISTRY_URL it carries no
// aim_atc_revocation_ series.
func TestStartRevocationList_PutsTheListHeldOnMetrics(t *testing.T) {
	a := newRevocationListAuthority(t, "did:web:agents.example:revoked-agent")
	captureLockedLog(t)

	refresher, err := startRevocationList(config.RegistryConfig{URL: a.srv.URL}, a.srv.Client())
	require.NoError(t, err)
	require.NotNil(t, refresher)
	t.Cleanup(refresher.Stop)
	deadline := time.Now().Add(10 * time.Second)
	for ok, _ := refresher.State(); !ok; ok, _ = refresher.State() {
		if time.Now().After(deadline) {
			t.Fatal("no list was held 10 s after the start")
		}
		time.Sleep(10 * time.Millisecond)
	}

	exposition := scrapeMetrics(t)
	for _, line := range []string{
		"# TYPE aim_atc_revocation_list_age_seconds gauge",
		"# TYPE aim_atc_revocation_list_entries gauge",
		"aim_atc_revocation_list_entries 1",
	} {
		assert.Contains(t, exposition, line+"\n")
	}
	age, ok := unlabelledSample(exposition, "aim_atc_revocation_list_age_seconds")
	require.True(t, ok, "/metrics carries no aim_atc_revocation_list_age_seconds sample")
	assert.GreaterOrEqual(t, age, 0.0)
	assert.Less(t, age, 10.0, "the list was read at most 10 s before the scrape")

	none, err := startRevocationList(config.RegistryConfig{}, nil)
	require.NoError(t, err)
	require.Nil(t, none)
	assert.NotContains(t, scrapeMetrics(t), "aim_atc_revocation_", "no revocation list series without REGISTRY_URL")
}
