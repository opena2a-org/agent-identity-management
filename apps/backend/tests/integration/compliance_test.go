package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetComplianceStatusUnauthorized tests that getting compliance status requires authentication
func TestGetComplianceStatusUnauthorized(t *testing.T) {
	baseURL := getBaseURL()

	resp, err := http.Get(baseURL + "/api/v1/compliance/status")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestGetComplianceMetricsUnauthorized tests that getting compliance metrics requires authentication
func TestGetComplianceMetricsUnauthorized(t *testing.T) {
	baseURL := getBaseURL()

	resp, err := http.Get(baseURL + "/api/v1/compliance/metrics")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestExportAuditLogUnauthorized tests that exporting audit log requires authentication
func TestExportAuditLogUnauthorized(t *testing.T) {
	baseURL := getBaseURL()

	resp, err := http.Get(baseURL + "/api/v1/compliance/audit-log/export")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestGetAccessReviewUnauthorized tests that getting access review requires authentication
func TestGetAccessReviewUnauthorized(t *testing.T) {
	baseURL := getBaseURL()

	resp, err := http.Get(baseURL + "/api/v1/compliance/access-review")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestGetDataRetentionUnauthorized tests that getting data retention requires authentication
func TestGetDataRetentionUnauthorized(t *testing.T) {
	baseURL := getBaseURL()

	resp, err := http.Get(baseURL + "/api/v1/compliance/audit-log/data-retention")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestRunComplianceCheckUnauthorized tests that running compliance check requires authentication
func TestRunComplianceCheckUnauthorized(t *testing.T) {
	baseURL := getBaseURL()

	resp, err := http.Post(baseURL+"/api/v1/compliance/check", "application/json", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestGenerateComplianceReportUnauthorized tests that generating compliance report requires authentication
func TestGenerateComplianceReportUnauthorized(t *testing.T) {
	baseURL := getBaseURL()

	resp, err := http.Post(baseURL+"/api/v1/compliance/reports/generate", "application/json", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestRunComplianceCheckWithValidPayload tests compliance check with valid payload
func TestRunComplianceCheckWithValidPayload(t *testing.T) {
	baseURL := getBaseURL()

	payload := map[string]interface{}{
		"check_type": "SOC2",
		"scope":      "all",
	}
	jsonData, err := json.Marshal(payload)
	require.NoError(t, err)

	resp, err := http.Post(baseURL+"/api/v1/compliance/check", "application/json", bytes.NewBuffer(jsonData))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestGenerateComplianceReportWithParams tests generating compliance report with parameters
func TestGenerateComplianceReportWithParams(t *testing.T) {
	baseURL := getBaseURL()

	payload := map[string]interface{}{
		"report_type": "SOC2",
		"start_date":  "2024-01-01",
		"end_date":    "2024-12-31",
	}
	jsonData, err := json.Marshal(payload)
	require.NoError(t, err)

	resp, err := http.Post(baseURL+"/api/v1/compliance/reports/generate", "application/json", bytes.NewBuffer(jsonData))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestExportAuditLogWithParams tests audit log export with query parameters
func TestExportAuditLogWithParams(t *testing.T) {
	baseURL := getBaseURL()

	resp, err := http.Get(baseURL + "/api/v1/compliance/audit-log/export?format=csv&start=2024-01-01")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

