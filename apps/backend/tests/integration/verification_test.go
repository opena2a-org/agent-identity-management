package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetVerificationUnauthorized verifies GET verification requires authentication
func TestGetVerificationUnauthorized(t *testing.T) {
	ensureAIMBackendRunning(t) // Skip if AIM backend not running
	baseURL := getBaseURL()

	// Use a sample verification UUID
	verificationID := "00000000-0000-0000-0000-000000000000"
	resp, err := http.Get(baseURL + "/api/v1/verifications/" + verificationID)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestGetVerificationInvalidUUID verifies validation of UUID format
func TestGetVerificationInvalidUUID(t *testing.T) {
	ensureAIMBackendRunning(t) // Skip if AIM backend not running
	baseURL := getBaseURL()

	// Use invalid UUID format
	invalidID := "not-a-valid-uuid"
	req, err := http.NewRequest("GET", baseURL+"/api/v1/verifications/"+invalidID, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer mock-invalid-token")

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should return 400 (invalid UUID) or 401 (invalid token)
	// Either is acceptable - validation might happen before or after auth
	assert.True(t, resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnauthorized)
}

// TestSubmitVerificationResultUnauthorized was named "verifies POST result
// requires authentication" and targeted /api/v1/verifications/:id/result — the
// JWT-group mount. The route that was actually serving unauthenticated writes
// was /api/v1/SDK-API/verifications/:id/result, three lines away in main.go and
// never covered here. Anyone searching "is this route authed?" found a green
// test with the right name exercising a different route.
//
// It now targets the sdk-api route, which is the one that mattered. The JWT
// mount no longer exists at all.
//
// DURABILITY CAVEAT: apps/backend/tests/ is rsync --delete mirrored from the
// public repo by scripts/sync-to-cloud.sh, so this correction is reverted by the
// next sync. The durable fix belongs in the public tree, which is frozen pending
// Abdel's disclosure decision. Recorded in the roadmap unit as not-done rather
// than left to look done.
func TestSubmitVerificationResultUnauthorized(t *testing.T) {
	ensureAIMBackendRunning(t) // Skip if AIM backend not running
	baseURL := getBaseURL()

	verificationID := "00000000-0000-0000-0000-000000000000"
	resultData := map[string]interface{}{
		"result": "success",
		"reason": "Verification completed successfully",
	}

	body, _ := json.Marshal(resultData)
	resp, err := http.Post(baseURL+"/api/v1/sdk-api/verifications/"+verificationID+"/result", "application/json", bytes.NewBuffer(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	// 401 from the sdkAPI group middleware. Explicitly NOT 404: a 404 here would
	// mean the handler ran and the UPDATE executed with no credentials, which is
	// exactly what this endpoint did before the fix.
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode,
		"unauthenticated POST to the sdk-api result route must be rejected by the group middleware; "+
			"404 would mean the handler ran and the write executed")
}

// TestSubmitVerificationResultNeverReportsSuccess sends NO credentials, and is
// named for what it actually asserts rather than for what would be nice to
// assert. The stronger property — that a correctly-signed request from the
// OWNING agent is also refused — needs a registered agent and a private key,
// which this suite has no fixture for; it is covered instead by
// TestSubmitVerificationResult_OwningAgentCannotApprovePendingEvent in the
// handlers package, red-proofed against pre-fix code.
//
// Naming a test for a stronger property than it tests is the specific failure
// this file already committed once: TestSubmitVerificationResultUnauthorized
// was named for this defect and pointed at a different route for months.
func TestSubmitVerificationResultNeverReportsSuccess(t *testing.T) {
	ensureAIMBackendRunning(t) // Skip if AIM backend not running
	baseURL := getBaseURL()

	verificationID := "00000000-0000-0000-0000-000000000000"
	body, _ := json.Marshal(map[string]interface{}{"result": "success"})
	resp, err := http.Post(baseURL+"/api/v1/sdk-api/verifications/"+verificationID+"/result", "application/json", bytes.NewBuffer(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.NotEqual(t, http.StatusOK, resp.StatusCode,
		"the result endpoint is withdrawn; a 200 on any path means the decision channel is writable again")
	assert.NotEqual(t, http.StatusNotFound, resp.StatusCode,
		"404 was the pre-fix signature of the handler running and the UPDATE affecting zero rows")
}

// TestSubmitVerificationResultInvalidData verifies validation on result submission
func TestSubmitVerificationResultInvalidData(t *testing.T) {
	ensureAIMBackendRunning(t) // Skip if AIM backend not running
	baseURL := getBaseURL()

	verificationID := "00000000-0000-0000-0000-000000000000"

	// Missing required 'result' field
	invalidData := map[string]interface{}{
		"reason": "Some reason",
	}

	body, _ := json.Marshal(invalidData)
	req, err := http.NewRequest("POST", baseURL+"/api/v1/verifications/"+verificationID+"/result", bytes.NewBuffer(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer mock-invalid-token")

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should return 400 (validation error) or 401 (invalid token)
	// Either is acceptable - validation might happen before or after auth
	assert.True(t, resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnauthorized)
}

// TestSubmitVerificationResultInvalidValue verifies result value validation
func TestSubmitVerificationResultInvalidValue(t *testing.T) {
	ensureAIMBackendRunning(t) // Skip if AIM backend not running
	baseURL := getBaseURL()

	verificationID := "00000000-0000-0000-0000-000000000000"

	// Invalid result value (not "success" or "failure")
	invalidData := map[string]interface{}{
		"result": "invalid-value",
		"reason": "Some reason",
	}

	body, _ := json.Marshal(invalidData)
	req, err := http.NewRequest("POST", baseURL+"/api/v1/verifications/"+verificationID+"/result", bytes.NewBuffer(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer mock-invalid-token")

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should return 400 (validation error) or 401 (invalid token)
	// Either is acceptable - validation might happen before or after auth
	assert.True(t, resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnauthorized)
}

// TestCreateVerificationUnauthorized verifies POST verification requires authentication
func TestCreateVerificationUnauthorized(t *testing.T) {
	ensureAIMBackendRunning(t) // Skip if AIM backend not running
	baseURL := getBaseURL()

	verificationData := map[string]interface{}{
		"agent_id": "00000000-0000-0000-0000-000000000000",
		"action":   "read_file",
		"resource": "/etc/passwd",
	}

	body, _ := json.Marshal(verificationData)
	resp, err := http.Post(baseURL+"/api/v1/verifications", "application/json", bytes.NewBuffer(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TODO: Add authenticated tests once we have a test JWT generation utility
// - TestGetVerificationAuthorized (with valid ID from database)
// - TestGetVerificationNotFound (with non-existent but valid UUID)
// - TestSubmitVerificationResultSuccess (with "success" result)
// - TestSubmitVerificationResultFailure (with "failure" result)
// - TestCreateVerificationAuthorized (with valid agent signature)
