package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestListAPIKeysUnauthorized verifies API keys endpoint requires authentication
func TestListAPIKeysUnauthorized(t *testing.T) {
	baseURL := getBaseURL()

	resp, err := http.Get(baseURL + "/api/v1/api-keys")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestGenerateAPIKeyUnauthorized verifies API key generation requires authentication
func TestGenerateAPIKeyUnauthorized(t *testing.T) {
	baseURL := getBaseURL()

	keyData := map[string]interface{}{
		"agent_id":        "00000000-0000-0000-0000-000000000000",
		"name":            "Test Key",
		"expires_in_days": 90,
	}

	body, _ := json.Marshal(keyData)
	resp, err := http.Post(baseURL+"/api/v1/api-keys", "application/json", bytes.NewBuffer(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestRevokeAPIKeyUnauthorized verifies API key revocation requires authentication
func TestRevokeAPIKeyUnauthorized(t *testing.T) {
	baseURL := getBaseURL()

	keyID := "00000000-0000-0000-0000-000000000000"
	req, err := http.NewRequest("DELETE", baseURL+"/api/v1/api-keys/"+keyID, nil)
	require.NoError(t, err)

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestVerifyAPIKeyUnauthorized verifies API key verification endpoint
func TestVerifyAPIKeyUnauthorized(t *testing.T) {
	baseURL := getBaseURL()

	keyID := "00000000-0000-0000-0000-000000000000"
	resp, err := http.Get(baseURL + "/api/v1/api-keys/" + keyID + "/verify")
	require.NoError(t, err)
	defer resp.Body.Close()

	// Verification might work without auth but fail due to invalid key
	// Acceptable responses: 401 (no auth), 404 (key not found), 400 (invalid)
	assert.True(t, resp.StatusCode == http.StatusUnauthorized ||
		resp.StatusCode == http.StatusNotFound ||
		resp.StatusCode == http.StatusBadRequest)
}

// TODO: Add authorized tests
// - TestGenerateAPIKeyAuthorized
// - TestListAPIKeysAuthorized
// - TestRevokeAPIKeyAuthorized
// - TestVerifyAPIKeyAuthorized
// - TestAPIKeyExpiration
