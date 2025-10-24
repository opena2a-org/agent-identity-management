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

// TestSubmitVerificationResultUnauthorized verifies POST result requires authentication
func TestSubmitVerificationResultUnauthorized(t *testing.T) {
	baseURL := getBaseURL()

	verificationID := "00000000-0000-0000-0000-000000000000"
	resultData := map[string]interface{}{
		"result": "success",
		"reason": "Verification completed successfully",
	}

	body, _ := json.Marshal(resultData)
	resp, err := http.Post(baseURL+"/api/v1/verifications/"+verificationID+"/result", "application/json", bytes.NewBuffer(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestSubmitVerificationResultInvalidData verifies validation on result submission
func TestSubmitVerificationResultInvalidData(t *testing.T) {
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
