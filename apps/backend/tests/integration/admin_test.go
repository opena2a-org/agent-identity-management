package integration

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestListUsersUnauthorized verifies admin users endpoint requires authentication
func TestListUsersUnauthorized(t *testing.T) {
	baseURL := getBaseURL()

	resp, err := http.Get(baseURL + "/api/v1/admin/users")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestGetAuditLogsUnauthorized verifies audit logs endpoint requires authentication
func TestGetAuditLogsUnauthorized(t *testing.T) {
	baseURL := getBaseURL()

	resp, err := http.Get(baseURL + "/api/v1/admin/audit-logs")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestGetAlertsUnauthorized verifies alerts endpoint requires authentication
func TestGetAlertsUnauthorized(t *testing.T) {
	baseURL := getBaseURL()

	resp, err := http.Get(baseURL + "/api/v1/admin/alerts")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestAcknowledgeAlertUnauthorized verifies alert acknowledgment requires authentication
func TestAcknowledgeAlertUnauthorized(t *testing.T) {
	baseURL := getBaseURL()

	alertID := "00000000-0000-0000-0000-000000000000"
	req, err := http.NewRequest("POST", baseURL+"/api/v1/admin/alerts/"+alertID+"/acknowledge", nil)
	require.NoError(t, err)

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TODO: Add authorized admin tests
// - TestListUsersAuthorized (admin role)
// - TestUpdateUserRoleAuthorized (admin role)
// - TestDeactivateUserAuthorized (admin role)
// - TestGetAuditLogsAuthorized (admin role)
// - TestFilterAuditLogsAuthorized (admin role)
// - TestGetAlertsAuthorized (admin role)
// - TestAcknowledgeAlertAuthorized (admin role)
// - TestResolveAlertAuthorized (admin role)
// - TestAdminEndpointsRequireAdminRole (with non-admin token)
