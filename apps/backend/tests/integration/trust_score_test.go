package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCalculateTrustScoreUnauthorized tests that calculating trust score requires authentication
func TestCalculateTrustScoreUnauthorized(t *testing.T) {
	baseURL := getBaseURL()
	agentID := "123e4567-e89b-12d3-a456-426614174000"

	resp, err := http.Post(baseURL+"/api/v1/trust-score/calculate/"+agentID, "application/json", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestCalculateTrustScoreInvalidAgentID tests calculating trust score with invalid agent ID
func TestCalculateTrustScoreInvalidAgentID(t *testing.T) {
	baseURL := getBaseURL()

	resp, err := http.Post(baseURL+"/api/v1/trust-score/calculate/invalid-id", "application/json", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestGetTrustScoreUnauthorized tests that getting trust score requires authentication
func TestGetTrustScoreUnauthorized(t *testing.T) {
	baseURL := getBaseURL()
	agentID := "123e4567-e89b-12d3-a456-426614174000"

	resp, err := http.Get(baseURL + "/api/v1/trust-score/agents/" + agentID)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestGetTrustScoreHistoryUnauthorized tests that getting trust score history requires authentication
func TestGetTrustScoreHistoryUnauthorized(t *testing.T) {
	baseURL := getBaseURL()
	agentID := "123e4567-e89b-12d3-a456-426614174000"

	resp, err := http.Get(baseURL + "/api/v1/trust-score/agents/" + agentID + "/history")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestGetTrustScoreTrendsUnauthorized tests that getting trust score trends requires authentication
func TestGetTrustScoreTrendsUnauthorized(t *testing.T) {
	baseURL := getBaseURL()

	resp, err := http.Get(baseURL + "/api/v1/trust-score/trends")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestGetTrustScoreTrendsWithParams tests trust score trends with query parameters
func TestGetTrustScoreTrendsWithParams(t *testing.T) {
	baseURL := getBaseURL()

	resp, err := http.Get(baseURL + "/api/v1/trust-score/trends?days=30")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestCalculateTrustScoreEmptyBody tests calculating trust score with empty body
func TestCalculateTrustScoreEmptyBody(t *testing.T) {
	baseURL := getBaseURL()
	agentID := "123e4567-e89b-12d3-a456-426614174000"

	payload := map[string]interface{}{}
	jsonData, err := json.Marshal(payload)
	require.NoError(t, err)

	resp, err := http.Post(baseURL+"/api/v1/trust-score/calculate/"+agentID, "application/json", bytes.NewBuffer(jsonData))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

