package integration

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/cloudflare/circl/sign/mldsa/mldsa44"
	"github.com/cloudflare/circl/sign/mldsa/mldsa65"
	"github.com/cloudflare/circl/sign/mldsa/mldsa87"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// PQC E2E TEST SUITE
// Tests for Post-Quantum Cryptography (NIST FIPS 204 ML-DSA)
// =============================================================================

// TestGetSupportedAlgorithms verifies the crypto algorithms endpoint
func TestGetSupportedAlgorithms(t *testing.T) {
	ensureAIMBackendRunning(t)
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}
	baseURL := getBaseURL()

	resp, err := http.Get(baseURL + "/api/v1/crypto/algorithms")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Should return 200 OK")

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err, "Response should be valid JSON")

	// The endpoint returns three categories: classical, postQuantum, hybrid
	assert.Contains(t, result, "classical", "Response should contain classical algorithms")
	assert.Contains(t, result, "postQuantum", "Response should contain postQuantum algorithms")
	assert.Contains(t, result, "hybrid", "Response should contain hybrid algorithms")
}

// TestPQCKeyRegistrationFlow tests the complete PQC key registration flow
func TestPQCKeyRegistrationFlow(t *testing.T) {
	ensureAIMBackendRunning(t)
	tc := NewTestContext(t)

	// Login as admin to create test agent
	require.NoError(t, tc.WaitForBackend())
	require.NoError(t, tc.LoginAsAdmin())

	// Generate Ed25519 keypair for the agent
	edPubKey, edPrivKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	// Create test agent with Ed25519
	agentResp, err := tc.Post("/api/v1/agents", map[string]interface{}{
		"name":        fmt.Sprintf("pqc-test-agent-%d", time.Now().UnixNano()),
		"displayName": "PQC Test Agent",
		"description": "Test agent for PQC integration tests",
		"agentType":   "ai_agent",
		"publicKey":   base64.StdEncoding.EncodeToString(edPubKey),
	}, tc.AdminToken)
	require.NoError(t, err, "Should create agent")

	var agent map[string]interface{}
	require.NoError(t, json.Unmarshal(agentResp, &agent))
	agentID := agent["id"].(string)

	// Verify agent has Ed25519 but no PQC key yet
	keyInfoResp, err := tc.Get(fmt.Sprintf("/api/v1/agents/%s/pqc-key", agentID), tc.AdminToken)
	require.NoError(t, err)

	var keyInfo map[string]interface{}
	require.NoError(t, json.Unmarshal(keyInfoResp, &keyInfo))

	assert.Equal(t, "not_configured", keyInfo["pqcStatus"], "PQC should not be configured initially")
	assert.False(t, keyInfo["hybridModeEnabled"].(bool), "Hybrid mode should be disabled")

	// Generate ML-DSA-65 keypair
	mldsaPubKey, mldsaPrivKey, err := mldsa65.GenerateKey(rand.Reader)
	require.NoError(t, err)
	_ = mldsaPrivKey // Will be used for signing

	// Register PQC key
	registerResp, err := tc.Post(fmt.Sprintf("/api/v1/agents/%s/pqc-key", agentID), map[string]interface{}{
		"publicKey":    base64.StdEncoding.EncodeToString(mldsaPubKey.Bytes()),
		"algorithm":    "ML-DSA-65",
		"enableHybrid": true,
	}, tc.AdminToken)
	require.NoError(t, err, "Should register PQC key")

	var registerResult map[string]interface{}
	require.NoError(t, json.Unmarshal(registerResp, &registerResult))
	assert.Equal(t, "PQC key registered successfully", registerResult["message"], "PQC key registration should succeed")

	// Verify PQC key is now registered
	keyInfoResp2, err := tc.Get(fmt.Sprintf("/api/v1/agents/%s/pqc-key", agentID), tc.AdminToken)
	require.NoError(t, err)

	var keyInfo2 map[string]interface{}
	require.NoError(t, json.Unmarshal(keyInfoResp2, &keyInfo2))

	assert.Equal(t, "hybrid_mode", keyInfo2["pqcStatus"], "Should be in hybrid mode")
	assert.True(t, keyInfo2["hybridModeEnabled"].(bool), "Hybrid mode should be enabled")
	assert.Equal(t, "ML-DSA-65", keyInfo2["pqcKeyAlgorithm"], "Should have ML-DSA-65 key")

	// Store keys for signature tests
	_ = edPrivKey // Keep for signature test

	t.Logf("Agent %s successfully configured with hybrid Ed25519+ML-DSA-65 authentication", agentID)
}

// TestHybridModeToggle tests enabling and disabling hybrid mode
func TestHybridModeToggle(t *testing.T) {
	ensureAIMBackendRunning(t)
	tc := NewTestContext(t)

	require.NoError(t, tc.WaitForBackend())
	require.NoError(t, tc.LoginAsAdmin())

	// Generate keys
	edPubKey, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	mldsaPubKey, _, err := mldsa65.GenerateKey(rand.Reader)
	require.NoError(t, err)

	// Create agent with PQC key
	agentResp, err := tc.Post("/api/v1/agents", map[string]interface{}{
		"name":        fmt.Sprintf("hybrid-toggle-agent-%d", time.Now().UnixNano()),
		"displayName": "Hybrid Toggle Agent",
		"description": "Test agent for hybrid mode toggle",
		"agentType":   "ai_agent",
		"publicKey":   base64.StdEncoding.EncodeToString(edPubKey),
	}, tc.AdminToken)
	require.NoError(t, err)

	var agent map[string]interface{}
	require.NoError(t, json.Unmarshal(agentResp, &agent))
	agentID := agent["id"].(string)

	// Register PQC key with hybrid mode enabled
	_, err = tc.Post(fmt.Sprintf("/api/v1/agents/%s/pqc-key", agentID), map[string]interface{}{
		"publicKey":    base64.StdEncoding.EncodeToString(mldsaPubKey.Bytes()),
		"algorithm":    "ML-DSA-65",
		"enableHybrid": true,
	}, tc.AdminToken)
	require.NoError(t, err)

	// Disable hybrid mode
	disableResp, err := tc.Post(fmt.Sprintf("/api/v1/agents/%s/hybrid-mode", agentID), map[string]interface{}{
		"enable": false,
	}, tc.AdminToken)
	require.NoError(t, err)

	var disableResult map[string]interface{}
	require.NoError(t, json.Unmarshal(disableResp, &disableResult))
	assert.False(t, disableResult["hybridModeEnabled"].(bool), "Hybrid mode should be disabled")

	// Verify PQC status changed
	keyInfoResp, err := tc.Get(fmt.Sprintf("/api/v1/agents/%s/pqc-key", agentID), tc.AdminToken)
	require.NoError(t, err)

	var keyInfo map[string]interface{}
	require.NoError(t, json.Unmarshal(keyInfoResp, &keyInfo))
	assert.Equal(t, "pqc_only", keyInfo["pqcStatus"], "Should be pqc_only mode")

	// Re-enable hybrid mode
	enableResp, err := tc.Post(fmt.Sprintf("/api/v1/agents/%s/hybrid-mode", agentID), map[string]interface{}{
		"enable": true,
	}, tc.AdminToken)
	require.NoError(t, err)

	var enableResult map[string]interface{}
	require.NoError(t, json.Unmarshal(enableResp, &enableResult))
	assert.True(t, enableResult["hybridModeEnabled"].(bool), "Hybrid mode should be enabled")
}

// TestPQCKeyRotation tests rotating PQC keys
func TestPQCKeyRotation(t *testing.T) {
	ensureAIMBackendRunning(t)
	tc := NewTestContext(t)

	require.NoError(t, tc.WaitForBackend())
	require.NoError(t, tc.LoginAsAdmin())

	// Generate initial keys
	edPubKey, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	mldsaPubKey1, _, err := mldsa65.GenerateKey(rand.Reader)
	require.NoError(t, err)

	mldsaPubKey2, _, err := mldsa65.GenerateKey(rand.Reader)
	require.NoError(t, err)

	// Create agent and register first PQC key
	agentResp, err := tc.Post("/api/v1/agents", map[string]interface{}{
		"name":        fmt.Sprintf("key-rotation-agent-%d", time.Now().UnixNano()),
		"displayName": "Key Rotation Agent",
		"description": "Test agent for key rotation",
		"agentType":   "ai_agent",
		"publicKey":   base64.StdEncoding.EncodeToString(edPubKey),
	}, tc.AdminToken)
	require.NoError(t, err)

	var agent map[string]interface{}
	require.NoError(t, json.Unmarshal(agentResp, &agent))
	agentID := agent["id"].(string)

	// Register first PQC key
	_, err = tc.Post(fmt.Sprintf("/api/v1/agents/%s/pqc-key", agentID), map[string]interface{}{
		"publicKey":    base64.StdEncoding.EncodeToString(mldsaPubKey1.Bytes()),
		"algorithm":    "ML-DSA-65",
		"enableHybrid": true,
	}, tc.AdminToken)
	require.NoError(t, err)

	// Get initial key info
	keyInfo1Resp, err := tc.Get(fmt.Sprintf("/api/v1/agents/%s/pqc-key", agentID), tc.AdminToken)
	require.NoError(t, err)

	var keyInfo1 map[string]interface{}
	require.NoError(t, json.Unmarshal(keyInfo1Resp, &keyInfo1))
	initialPQCKey := keyInfo1["pqcPublicKey"].(string)

	// Rotate to second PQC key
	rotateResp, err := tc.Put(fmt.Sprintf("/api/v1/agents/%s/pqc-key", agentID), map[string]interface{}{
		"newPublicKey": base64.StdEncoding.EncodeToString(mldsaPubKey2.Bytes()),
		"algorithm":    "ML-DSA-65",
	}, tc.AdminToken)
	require.NoError(t, err)

	var rotateResult map[string]interface{}
	require.NoError(t, json.Unmarshal(rotateResp, &rotateResult))
	assert.Equal(t, "PQC key rotated successfully", rotateResult["message"], "Key rotation should succeed")

	// Verify key was rotated
	keyInfo2Resp, err := tc.Get(fmt.Sprintf("/api/v1/agents/%s/pqc-key", agentID), tc.AdminToken)
	require.NoError(t, err)

	var keyInfo2 map[string]interface{}
	require.NoError(t, json.Unmarshal(keyInfo2Resp, &keyInfo2))
	newPQCKey := keyInfo2["pqcPublicKey"].(string)

	assert.NotEqual(t, initialPQCKey, newPQCKey, "PQC key should have changed after rotation")
}

// TestAllMLDSAVariants tests all ML-DSA security levels
func TestAllMLDSAVariants(t *testing.T) {
	ensureAIMBackendRunning(t)
	tc := NewTestContext(t)

	require.NoError(t, tc.WaitForBackend())
	require.NoError(t, tc.LoginAsAdmin())

	variants := []string{"ML-DSA-44", "ML-DSA-65", "ML-DSA-87"}

	for _, variant := range variants {
		t.Run(variant, func(t *testing.T) {
			// Generate Ed25519 key
			edPubKey, _, err := ed25519.GenerateKey(rand.Reader)
			require.NoError(t, err)

			// Generate ML-DSA key with variant-specific generator
			var mldsaPubKeyBytes []byte
			switch variant {
			case "ML-DSA-44":
				pubKey, _, err := mldsa44.GenerateKey(rand.Reader)
				require.NoError(t, err)
				mldsaPubKeyBytes = pubKey.Bytes()
			case "ML-DSA-65":
				pubKey, _, err := mldsa65.GenerateKey(rand.Reader)
				require.NoError(t, err)
				mldsaPubKeyBytes = pubKey.Bytes()
			case "ML-DSA-87":
				pubKey, _, err := mldsa87.GenerateKey(rand.Reader)
				require.NoError(t, err)
				mldsaPubKeyBytes = pubKey.Bytes()
			}

			// Create agent
			agentResp, err := tc.Post("/api/v1/agents", map[string]interface{}{
				"name":        fmt.Sprintf("variant-test-%s-%d", variant, time.Now().UnixNano()),
				"displayName": fmt.Sprintf("Variant Test %s", variant),
				"description": fmt.Sprintf("Test agent for %s", variant),
				"agentType":   "ai_agent",
				"publicKey":   base64.StdEncoding.EncodeToString(edPubKey),
			}, tc.AdminToken)
			require.NoError(t, err)

			var agent map[string]interface{}
			require.NoError(t, json.Unmarshal(agentResp, &agent))
			agentID := agent["id"].(string)

			// Register PQC key with specific variant
			registerResp, err := tc.Post(fmt.Sprintf("/api/v1/agents/%s/pqc-key", agentID), map[string]interface{}{
				"publicKey":    base64.StdEncoding.EncodeToString(mldsaPubKeyBytes),
				"algorithm":    variant,
				"enableHybrid": true,
			}, tc.AdminToken)
			require.NoError(t, err, "Should register %s key", variant)

			var result map[string]interface{}
			require.NoError(t, json.Unmarshal(registerResp, &result))
			assert.Equal(t, "PQC key registered successfully", result["message"], "%s key registration should succeed", variant)

			t.Logf("Successfully registered %s key for agent %s", variant, agentID)
		})
	}
}

// TestBackwardCompatibility verifies Ed25519-only agents still work
func TestBackwardCompatibility(t *testing.T) {
	ensureAIMBackendRunning(t)
	tc := NewTestContext(t)

	require.NoError(t, tc.WaitForBackend())
	require.NoError(t, tc.LoginAsAdmin())

	// Generate Ed25519 keypair
	edPubKey, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	// Create agent with Ed25519 only (no PQC)
	agentResp, err := tc.Post("/api/v1/agents", map[string]interface{}{
		"name":        fmt.Sprintf("ed25519-only-agent-%d", time.Now().UnixNano()),
		"displayName": "Ed25519 Only Agent",
		"description": "Test agent for backward compatibility",
		"agentType":   "ai_agent",
		"publicKey":   base64.StdEncoding.EncodeToString(edPubKey),
	}, tc.AdminToken)
	require.NoError(t, err, "Should create Ed25519-only agent")

	var agent map[string]interface{}
	require.NoError(t, json.Unmarshal(agentResp, &agent))
	agentID := agent["id"].(string)

	// Verify agent works without PQC
	keyInfoResp, err := tc.Get(fmt.Sprintf("/api/v1/agents/%s/pqc-key", agentID), tc.AdminToken)
	require.NoError(t, err)

	var keyInfo map[string]interface{}
	require.NoError(t, json.Unmarshal(keyInfoResp, &keyInfo))

	assert.Equal(t, "not_configured", keyInfo["pqcStatus"], "PQC should not be configured")
	assert.False(t, keyInfo["hybridModeEnabled"].(bool), "Hybrid mode should be disabled")
	assert.Nil(t, keyInfo["pqcPublicKey"], "Should have no PQC key")

	t.Logf("Backward compatibility verified: Ed25519-only agent %s works correctly", agentID)
}

// TestInvalidPQCKeyRejection tests that invalid PQC keys are rejected
func TestInvalidPQCKeyRejection(t *testing.T) {
	ensureAIMBackendRunning(t)
	tc := NewTestContext(t)

	require.NoError(t, tc.WaitForBackend())
	require.NoError(t, tc.LoginAsAdmin())

	// Generate Ed25519 keypair
	edPubKey, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	// Create agent
	agentResp, err := tc.Post("/api/v1/agents", map[string]interface{}{
		"name":        fmt.Sprintf("invalid-key-test-%d", time.Now().UnixNano()),
		"displayName": "Invalid Key Test Agent",
		"description": "Test agent for invalid key rejection",
		"agentType":   "ai_agent",
		"publicKey":   base64.StdEncoding.EncodeToString(edPubKey),
	}, tc.AdminToken)
	require.NoError(t, err)

	var agent map[string]interface{}
	require.NoError(t, json.Unmarshal(agentResp, &agent))
	agentID := agent["id"].(string)

	testCases := []struct {
		name      string
		key       string
		algorithm string
		wantError bool
	}{
		{
			name:      "Empty key",
			key:       "",
			algorithm: "ML-DSA-65",
			wantError: true,
		},
		{
			name:      "Invalid base64",
			key:       "not-valid-base64!!!",
			algorithm: "ML-DSA-65",
			wantError: true,
		},
		{
			name:      "Too short key",
			key:       base64.StdEncoding.EncodeToString([]byte("short")),
			algorithm: "ML-DSA-65",
			wantError: true,
		},
		{
			name:      "Unsupported algorithm",
			key:       base64.StdEncoding.EncodeToString(make([]byte, 1952)), // Correct size for ML-DSA-65
			algorithm: "UNSUPPORTED-ALGO",
			wantError: true,
		},
	}

	for _, tc2 := range testCases {
		t.Run(tc2.name, func(t *testing.T) {
			_, err := tc.Post(fmt.Sprintf("/api/v1/agents/%s/pqc-key", agentID), map[string]interface{}{
				"publicKey":    tc2.key,
				"algorithm":    tc2.algorithm,
				"enableHybrid": true,
			}, tc.AdminToken)

			if tc2.wantError {
				// The request should fail with error status code
				require.Error(t, err, "Should reject invalid key: %s", tc2.name)
			}
		})
	}
}

// =============================================================================
// INTEROPERABILITY MATRIX TESTS
// =============================================================================

// TestInteroperabilityMatrix documents the expected interoperability
func TestInteroperabilityMatrix(t *testing.T) {
	t.Log("=== PQC Interoperability Matrix ===")
	t.Log("Signer -> Verifier combinations that must pass:")
	t.Log("")
	t.Log("| Signer      | Verifier    | Status       |")
	t.Log("|-------------|-------------|--------------|")
	t.Log("| Go          | Go          | ✅ Unit Tests |")
	t.Log("| Go          | Python      | ⏳ Pending    |")
	t.Log("| Go          | TypeScript  | ⏳ Pending    |")
	t.Log("| Python      | Go          | ⏳ Pending    |")
	t.Log("| Python      | Python      | ✅ Unit Tests |")
	t.Log("| Python      | TypeScript  | ⏳ Pending    |")
	t.Log("| TypeScript  | Go          | ⏳ Pending    |")
	t.Log("| TypeScript  | Python      | ⏳ Pending    |")
	t.Log("| TypeScript  | TypeScript  | ⏳ Pending    |")
	t.Log("")
	t.Log("Note: Cross-SDK tests require SDK clients with PQC support running")
	t.Log("against a live backend. These will be implemented as the SDKs mature.")
}
