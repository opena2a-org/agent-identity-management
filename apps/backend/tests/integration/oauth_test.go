package integration

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// oauthTestAgent holds an agent's ID and Ed25519 keypair for OAuth tests.
type oauthTestAgent struct {
	AgentID    string
	PublicKey  ed25519.PublicKey
	PrivateKey ed25519.PrivateKey
}

// createOAuthTestAgent creates an agent with a known Ed25519 public key
// and returns the agent ID and keypair for signing JWT assertions.
func createOAuthTestAgent(t *testing.T, tc *TestContext) *oauthTestAgent {
	t.Helper()

	// Generate Ed25519 keypair
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	pubKeyB64 := base64.StdEncoding.EncodeToString(pub)

	// Create agent with the public key
	agentName := fmt.Sprintf("oauth-test-agent-%d", time.Now().UnixNano())
	agentBody := map[string]interface{}{
		"name":        agentName,
		"displayName": agentName,
		"agentType":   "claude",
		"publicKey":   pubKeyB64,
	}

	respBody := tc.AssertStatusCode("POST", "/api/v1/agents", agentBody, tc.AdminToken, 201)

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(respBody, &result))

	agentID, ok := result["id"].(string)
	require.True(t, ok, "agent creation should return an id")

	return &oauthTestAgent{
		AgentID:    agentID,
		PublicKey:  pub,
		PrivateKey: priv,
	}
}

// buildSignedJWTAssertion creates a properly Ed25519-signed JWT for OAuth testing.
// Format: base64url(header).base64url(payload).base64url(signature)
func buildSignedJWTAssertion(sub string, privateKey ed25519.PrivateKey) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"EdDSA","typ":"JWT"}`))
	payload, _ := json.Marshal(map[string]interface{}{"sub": sub, "iss": sub, "aud": "aim"})
	payloadB64 := base64.RawURLEncoding.EncodeToString(payload)

	signedContent := header + "." + payloadB64
	signature := ed25519.Sign(privateKey, []byte(signedContent))
	sigB64 := base64.RawURLEncoding.EncodeToString(signature)

	return signedContent + "." + sigB64
}

// buildFakeJWTAssertion creates a JWT-like string with a fake signature for error path testing.
func buildFakeJWTAssertion(sub string) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"EdDSA","typ":"JWT"}`))
	payload, _ := json.Marshal(map[string]interface{}{"sub": sub, "iss": sub, "aud": "aim"})
	payloadB64 := base64.RawURLEncoding.EncodeToString(payload)
	sig := base64.RawURLEncoding.EncodeToString([]byte("fake-signature-for-testing"))
	return header + "." + payloadB64 + "." + sig
}

func TestOAuthTokenEndpointExists(t *testing.T) {
	ensureAIMBackendRunning(t)
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tc := NewTestContext(t)
	require.NoError(t, tc.WaitForBackend())
	require.NoError(t, tc.LoginAsAdmin())

	agent := createOAuthTestAgent(t, tc)

	body := map[string]interface{}{
		"grant_type":       "urn:ietf:params:oauth:grant-type:jwt-bearer",
		"client_id":        agent.AgentID,
		"client_assertion": buildSignedJWTAssertion(agent.AgentID, agent.PrivateKey),
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
		"client_assertion": buildFakeJWTAssertion("some-client"),
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
		"client_assertion": buildFakeJWTAssertion("different-client-id"),
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
	require.NoError(t, tc.LoginAsAdmin())

	agent := createOAuthTestAgent(t, tc)

	body := map[string]interface{}{
		"grant_type":            "urn:ietf:params:oauth:grant-type:jwt-bearer",
		"client_id":             agent.AgentID,
		"client_assertion":      buildSignedJWTAssertion(agent.AgentID, agent.PrivateKey),
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
	require.NoError(t, tc.LoginAsAdmin())

	agent := createOAuthTestAgent(t, tc)

	body := map[string]interface{}{
		"grant_type":       "urn:ietf:params:oauth:grant-type:jwt-bearer",
		"client_id":        agent.AgentID,
		"client_assertion": buildSignedJWTAssertion(agent.AgentID, agent.PrivateKey),
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
