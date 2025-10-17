package integration

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetThreatsUnauthorized tests that getting threats requires authentication
func TestGetThreatsUnauthorized(t *testing.T) {
	baseURL := getBaseURL()

	resp, err := http.Get(baseURL + "/api/v1/security/threats")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestGetAnomaliesUnauthorized tests that getting anomalies requires authentication
func TestGetAnomaliesUnauthorized(t *testing.T) {
	baseURL := getBaseURL()

	resp, err := http.Get(baseURL + "/api/v1/security/anomalies")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestGetSecurityMetricsUnauthorized tests that getting security metrics requires authentication
func TestGetSecurityMetricsUnauthorized(t *testing.T) {
	baseURL := getBaseURL()

	resp, err := http.Get(baseURL + "/api/v1/security/metrics")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestRunSecurityScanUnauthorized tests that running security scan requires authentication
func TestRunSecurityScanUnauthorized(t *testing.T) {
	baseURL := getBaseURL()
	scanID := "123e4567-e89b-12d3-a456-426614174000"

	resp, err := http.Get(fmt.Sprintf("%s/api/v1/security/scan/%s", baseURL, scanID))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestGetIncidentsUnauthorized tests that getting incidents requires authentication
func TestGetIncidentsUnauthorized(t *testing.T) {
	baseURL := getBaseURL()

	resp, err := http.Get(baseURL + "/api/v1/security/incidents")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestResolveIncidentUnauthorized tests that resolving incident requires authentication
func TestResolveIncidentUnauthorized(t *testing.T) {
	baseURL := getBaseURL()
	incidentID := "123e4567-e89b-12d3-a456-426614174000"

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/security/incidents/%s/resolve", baseURL, incidentID), nil)
	require.NoError(t, err)

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}
