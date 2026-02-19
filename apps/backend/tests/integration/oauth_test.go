package integration

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildJWTAssertion creates a minimal JWT-like string for testing.
// Format: base64url(header).base64url(payload).base64url(signature)
func buildJWTAssertion(sub string) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	payload, _ := json.Marshal(map[string]interface{}{"sub": sub, "iss": sub, "aud": "aim"})
	payloadB64 := base64.RawURLEncoding.EncodeToString(payload)
	sig := base64.RawURLEncoding.EncodeToString([]byte("test-signature"))
	return header + "." + payloadB64 + "." + sig
}

func TestOAuthTokenEndpointExists(t *testing.T) {
	ensureAIMBackendRunning(t)
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tc := NewTestContext(t)
	require.NoError(t, tc.WaitForBackend())

	clientID := "test-agent-id"
	body := map[string]interface{}{
		"grant_type":       "urn:ietf:params:oauth:grant-type:jwt-bearer",
		"client_id":        clientID,
		"client_assertion": buildJWTAssertion(clientID),
	}

	jsonBody, err := json.Marshal(body)
	require.NoError(t, err)

	req, err := http.NewRequest("POST", tc.Config.BaseURL+"/api/v1/oauth/token", bytes.NewReader(jsonBody))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := tc.Client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(respBody, &result))

	assert.Equal(t, 200, resp.StatusCode, "OAuth token endpoint should exist and return 200: %s", string(respBody))
	assert.Contains(t, result, "access_token")
	assert.Equal(t, "Bearer", result["token_type"])
}

func TestOAuthTokenWithInvalidGrantType(t *testing.T) {
	ensureAIMBackendRunning(t)
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tc := NewTestContext(t)
	require.NoError(t, tc.WaitForBackend())

	body := map[string]interface{}{
		"grant_type": "invalid_grant",
		"client_id":  "test-agent",
	}

	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", tc.Config.BaseURL+"/api/v1/oauth/token", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := tc.Client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, 400, resp.StatusCode)

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(respBody, &result))
	assert.Equal(t, "unsupported_grant_type", result["error"])
}

func TestOAuthTokenWithMissingClientId(t *testing.T) {
	ensureAIMBackendRunning(t)
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tc := NewTestContext(t)
	require.NoError(t, tc.WaitForBackend())

	body := map[string]interface{}{
		"grant_type":       "urn:ietf:params:oauth:grant-type:jwt-bearer",
		"client_assertion": buildJWTAssertion("some-client"),
	}

	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", tc.Config.BaseURL+"/api/v1/oauth/token", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := tc.Client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, 400, resp.StatusCode)

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(respBody, &result))
	assert.Equal(t, "invalid_request", result["error"])
}

func TestOAuthTokenWithInvalidClientAssertion(t *testing.T) {
	ensureAIMBackendRunning(t)
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tc := NewTestContext(t)
	require.NoError(t, tc.WaitForBackend())

	// Assertion sub doesn't match client_id
	body := map[string]interface{}{
		"grant_type":       "urn:ietf:params:oauth:grant-type:jwt-bearer",
		"client_id":        "actual-client-id",
		"client_assertion": buildJWTAssertion("different-client-id"),
	}

	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", tc.Config.BaseURL+"/api/v1/oauth/token", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := tc.Client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, 401, resp.StatusCode)

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(respBody, &result))
	assert.Equal(t, "invalid_client", result["error"])
}

func TestOAuthTokenWithMalformedAssertion(t *testing.T) {
	ensureAIMBackendRunning(t)
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tc := NewTestContext(t)
	require.NoError(t, tc.WaitForBackend())

	body := map[string]interface{}{
		"grant_type":       "urn:ietf:params:oauth:grant-type:jwt-bearer",
		"client_id":        "test-client",
		"client_assertion": "not-a-valid-jwt",
	}

	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", tc.Config.BaseURL+"/api/v1/oauth/token", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := tc.Client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, 400, resp.StatusCode)

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(respBody, &result))
	assert.Equal(t, "invalid_request", result["error"])
}

func TestOAuthTokenWithUnsupportedAssertionType(t *testing.T) {
	ensureAIMBackendRunning(t)
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tc := NewTestContext(t)
	require.NoError(t, tc.WaitForBackend())

	clientID := "test-client"
	body := map[string]interface{}{
		"grant_type":            "urn:ietf:params:oauth:grant-type:jwt-bearer",
		"client_id":             clientID,
		"client_assertion":      buildJWTAssertion(clientID),
		"client_assertion_type": "urn:unsupported:assertion-type",
	}

	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", tc.Config.BaseURL+"/api/v1/oauth/token", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := tc.Client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, 400, resp.StatusCode)

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(respBody, &result))
	assert.Equal(t, "invalid_request", result["error"])
}

func TestOAuthTokenContentType(t *testing.T) {
	ensureAIMBackendRunning(t)
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tc := NewTestContext(t)
	require.NoError(t, tc.WaitForBackend())

	// Send request without Content-Type header
	req, _ := http.NewRequest("POST", tc.Config.BaseURL+"/api/v1/oauth/token", bytes.NewReader([]byte("{}")))
	// Deliberately NOT setting Content-Type

	resp, err := tc.Client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, 400, resp.StatusCode)

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(respBody, &result))
	assert.Equal(t, "invalid_request", result["error"])
}

func TestOAuthTokenEmptyBody(t *testing.T) {
	ensureAIMBackendRunning(t)
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tc := NewTestContext(t)
	require.NoError(t, tc.WaitForBackend())

	// Send empty body
	req, _ := http.NewRequest("POST", tc.Config.BaseURL+"/api/v1/oauth/token", bytes.NewReader([]byte("")))
	req.Header.Set("Content-Type", "application/json")

	resp, err := tc.Client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, 400, resp.StatusCode)

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(respBody, &result))
	assert.Contains(t, result, "error")
}

func TestOAuthTokenResponseFormat(t *testing.T) {
	ensureAIMBackendRunning(t)
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tc := NewTestContext(t)
	require.NoError(t, tc.WaitForBackend())

	clientID := "response-format-test-agent"
	body := map[string]interface{}{
		"grant_type":       "urn:ietf:params:oauth:grant-type:jwt-bearer",
		"client_id":        clientID,
		"client_assertion": buildJWTAssertion(clientID),
	}

	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", tc.Config.BaseURL+"/api/v1/oauth/token", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := tc.Client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode)

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(respBody, &result))

	// Verify RFC 6749 token response format
	assert.Contains(t, result, "access_token", "Response must contain access_token")
	assert.Contains(t, result, "token_type", "Response must contain token_type")
	assert.Contains(t, result, "expires_in", "Response must contain expires_in")
	assert.Equal(t, "Bearer", result["token_type"])
	assert.Greater(t, result["expires_in"].(float64), float64(0))
}
