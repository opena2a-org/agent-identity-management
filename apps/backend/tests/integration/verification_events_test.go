package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestListVerificationEventsUnauthorized tests that listing verification events requires authentication
func TestListVerificationEventsUnauthorized(t *testing.T) {
	baseURL := getBaseURL()

	resp, err := http.Get(baseURL + "/api/v1/verification-events")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestGetRecentVerificationEventsUnauthorized tests that getting recent verification events requires authentication
func TestGetRecentVerificationEventsUnauthorized(t *testing.T) {
	baseURL := getBaseURL()

	resp, err := http.Get(baseURL + "/api/v1/verification-events/recent")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestGetVerificationEventsStatisticsUnauthorized tests that getting verification events statistics requires authentication
func TestGetVerificationEventsStatisticsUnauthorized(t *testing.T) {
	baseURL := getBaseURL()

	resp, err := http.Get(baseURL + "/api/v1/verification-events/statistics")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestGetVerificationEventUnauthorized tests that getting single verification event requires authentication
func TestGetVerificationEventUnauthorized(t *testing.T) {
	baseURL := getBaseURL()
	eventID := "123e4567-e89b-12d3-a456-426614174000"

	resp, err := http.Get(baseURL + "/api/v1/verification-events/" + eventID)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestCreateVerificationEventUnauthorized tests that creating verification event requires authentication
func TestCreateVerificationEventUnauthorized(t *testing.T) {
	baseURL := getBaseURL()

	payload := map[string]interface{}{
		"event_type":  "action.verified",
		"description": "Test verification event",
		"agent_id":    "123e4567-e89b-12d3-a456-426614174000",
	}
	jsonData, err := json.Marshal(payload)
	require.NoError(t, err)

	resp, err := http.Post(baseURL+"/api/v1/verification-events", "application/json", bytes.NewBuffer(jsonData))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestDeleteVerificationEventUnauthorized tests that deleting verification event requires authentication
func TestDeleteVerificationEventUnauthorized(t *testing.T) {
	baseURL := getBaseURL()
	eventID := "123e4567-e89b-12d3-a456-426614174000"

	req, err := http.NewRequest("DELETE", baseURL+"/api/v1/verification-events/"+eventID, nil)
	require.NoError(t, err)

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestListVerificationEventsWithParams tests listing verification events with query parameters
func TestListVerificationEventsWithParams(t *testing.T) {
	baseURL := getBaseURL()

	resp, err := http.Get(baseURL + "/api/v1/verification-events?limit=10&offset=0")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestGetRecentVerificationEventsWithLimit tests getting recent events with limit parameter
func TestGetRecentVerificationEventsWithLimit(t *testing.T) {
	baseURL := getBaseURL()

	resp, err := http.Get(baseURL + "/api/v1/verification-events/recent?limit=5")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestCreateVerificationEventWithInvalidData tests creating verification event with invalid data
func TestCreateVerificationEventWithInvalidData(t *testing.T) {
	baseURL := getBaseURL()

	payload := map[string]interface{}{
		"event_type": "",
	}
	jsonData, err := json.Marshal(payload)
	require.NoError(t, err)

	resp, err := http.Post(baseURL+"/api/v1/verification-events", "application/json", bytes.NewBuffer(jsonData))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestGetVerificationEventWithInvalidID tests getting verification event with invalid ID
func TestGetVerificationEventWithInvalidID(t *testing.T) {
	baseURL := getBaseURL()

	resp, err := http.Get(baseURL + "/api/v1/verification-events/invalid-id")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

