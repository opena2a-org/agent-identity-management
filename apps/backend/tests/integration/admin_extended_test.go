package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetPendingUsersUnauthorized tests that getting pending users requires authentication
func TestGetPendingUsersUnauthorized(t *testing.T) {
	baseURL := getBaseURL()

	resp, err := http.Get(baseURL + "/api/v1/admin/users/pending")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestApproveUserUnauthorized tests that approving user requires authentication
func TestApproveUserUnauthorized(t *testing.T) {
	baseURL := getBaseURL()
	userID := "123e4567-e89b-12d3-a456-426614174000"

	payload := map[string]interface{}{
		"role": "member",
	}
	jsonData, err := json.Marshal(payload)
	require.NoError(t, err)

	resp, err := http.Post(baseURL+"/api/v1/admin/users/"+userID+"/approve", "application/json", bytes.NewBuffer(jsonData))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestRejectUserUnauthorized tests that rejecting user requires authentication
func TestRejectUserUnauthorized(t *testing.T) {
	baseURL := getBaseURL()
	userID := "123e4567-e89b-12d3-a456-426614174000"

	payload := map[string]interface{}{
		"reason": "Does not meet organization requirements",
	}
	jsonData, err := json.Marshal(payload)
	require.NoError(t, err)

	resp, err := http.Post(baseURL+"/api/v1/admin/users/"+userID+"/reject", "application/json", bytes.NewBuffer(jsonData))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestUpdateUserRoleUnauthorized tests that updating user role requires authentication
func TestUpdateUserRoleUnauthorized(t *testing.T) {
	baseURL := getBaseURL()
	userID := "123e4567-e89b-12d3-a456-426614174000"

	payload := map[string]interface{}{
		"role": "manager",
	}
	jsonData, err := json.Marshal(payload)
	require.NoError(t, err)

	req, err := http.NewRequest("PUT", baseURL+"/api/v1/admin/users/"+userID+"/role", bytes.NewBuffer(jsonData))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestDeactivateUserUnauthorized tests that deactivating user requires authentication
func TestDeactivateUserUnauthorized(t *testing.T) {
	baseURL := getBaseURL()
	userID := "123e4567-e89b-12d3-a456-426614174000"

	resp, err := http.Post(baseURL+"/api/v1/admin/users/"+userID+"/deactivate", "application/json", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestActivateUserUnauthorized tests that activating user requires authentication
func TestActivateUserUnauthorized(t *testing.T) {
	baseURL := getBaseURL()
	userID := "123e4567-e89b-12d3-a456-426614174000"

	resp, err := http.Post(baseURL+"/api/v1/admin/users/"+userID+"/activate", "application/json", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestPermanentlyDeleteUserUnauthorized tests that permanently deleting user requires authentication
func TestPermanentlyDeleteUserUnauthorized(t *testing.T) {
	baseURL := getBaseURL()
	userID := "123e4567-e89b-12d3-a456-426614174000"

	req, err := http.NewRequest("DELETE", baseURL+"/api/v1/admin/users/"+userID, nil)
	require.NoError(t, err)

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestApproveRegistrationRequestUnauthorized tests that approving registration request requires authentication
func TestApproveRegistrationRequestUnauthorized(t *testing.T) {
	baseURL := getBaseURL()
	requestID := "123e4567-e89b-12d3-a456-426614174000"

	payload := map[string]interface{}{
		"role": "member",
	}
	jsonData, err := json.Marshal(payload)
	require.NoError(t, err)

	resp, err := http.Post(baseURL+"/api/v1/admin/registration-requests/"+requestID+"/approve", "application/json", bytes.NewBuffer(jsonData))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestRejectRegistrationRequestUnauthorized tests that rejecting registration request requires authentication
func TestRejectRegistrationRequestUnauthorized(t *testing.T) {
	baseURL := getBaseURL()
	requestID := "123e4567-e89b-12d3-a456-426614174000"

	payload := map[string]interface{}{
		"reason": "Email domain not allowed",
	}
	jsonData, err := json.Marshal(payload)
	require.NoError(t, err)

	resp, err := http.Post(baseURL+"/api/v1/admin/registration-requests/"+requestID+"/reject", "application/json", bytes.NewBuffer(jsonData))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestGetOrganizationSettingsUnauthorized tests that getting organization settings requires authentication
func TestGetOrganizationSettingsUnauthorized(t *testing.T) {
	baseURL := getBaseURL()

	resp, err := http.Get(baseURL + "/api/v1/admin/organization/settings")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestUpdateOrganizationSettingsUnauthorized tests that updating organization settings requires authentication
func TestUpdateOrganizationSettingsUnauthorized(t *testing.T) {
	baseURL := getBaseURL()

	payload := map[string]interface{}{
		"name":       "Updated Organization",
		"max_agents": 200,
	}
	jsonData, err := json.Marshal(payload)
	require.NoError(t, err)

	req, err := http.NewRequest("PUT", baseURL+"/api/v1/admin/organization/settings", bytes.NewBuffer(jsonData))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestGetDashboardStatsUnauthorized tests that getting dashboard stats requires authentication
func TestGetDashboardStatsUnauthorized(t *testing.T) {
	baseURL := getBaseURL()

	resp, err := http.Get(baseURL + "/api/v1/admin/dashboard/stats")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestUpdateUserRoleWithInvalidRole tests updating user role with invalid role
func TestUpdateUserRoleWithInvalidRole(t *testing.T) {
	baseURL := getBaseURL()
	userID := "123e4567-e89b-12d3-a456-426614174000"

	payload := map[string]interface{}{
		"role": "invalid-role",
	}
	jsonData, err := json.Marshal(payload)
	require.NoError(t, err)

	req, err := http.NewRequest("PUT", baseURL+"/api/v1/admin/users/"+userID+"/role", bytes.NewBuffer(jsonData))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestApproveUserWithEmptyBody tests approving user with empty body
func TestApproveUserWithEmptyBody(t *testing.T) {
	baseURL := getBaseURL()
	userID := "123e4567-e89b-12d3-a456-426614174000"

	resp, err := http.Post(baseURL+"/api/v1/admin/users/"+userID+"/approve", "application/json", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestDeactivateUserWithInvalidID tests deactivating user with invalid ID
func TestDeactivateUserWithInvalidID(t *testing.T) {
	baseURL := getBaseURL()

	resp, err := http.Post(baseURL+"/api/v1/admin/users/invalid-id/deactivate", "application/json", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestGetPendingUsersWithParams tests getting pending users with query parameters
func TestGetPendingUsersWithParams(t *testing.T) {
	baseURL := getBaseURL()

	resp, err := http.Get(baseURL + "/api/v1/admin/users/pending?limit=10&offset=0")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestUpdateOrganizationSettingsWithInvalidData tests updating organization settings with invalid data
func TestUpdateOrganizationSettingsWithInvalidData(t *testing.T) {
	baseURL := getBaseURL()

	payload := map[string]interface{}{
		"max_agents": -1,
	}
	jsonData, err := json.Marshal(payload)
	require.NoError(t, err)

	req, err := http.NewRequest("PUT", baseURL+"/api/v1/admin/organization/settings", bytes.NewBuffer(jsonData))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestGetDashboardStatsWithParams tests getting dashboard stats with query parameters
func TestGetDashboardStatsWithParams(t *testing.T) {
	baseURL := getBaseURL()

	resp, err := http.Get(baseURL + "/api/v1/admin/dashboard/stats?period=30d")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

