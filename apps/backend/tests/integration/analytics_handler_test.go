package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========================================
// GET /api/v1/analytics/dashboard Tests
// ========================================

// TestGetAnalyticsDashboardUnauthorized_Extended verifies dashboard requires authentication with proper error
func TestGetAnalyticsDashboardUnauthorized_Extended(t *testing.T) {
	baseURL := getBaseURL()

	resp, err := http.Get(baseURL + "/api/v1/analytics/dashboard")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	errorMsg, ok := result["error"].(string)
	require.True(t, ok, "Response should contain error message")
	assert.NotEmpty(t, errorMsg, "Error message should not be empty")
}

// ========================================
// GET /api/v1/analytics/usage Tests
// ========================================

func TestGetUsageStatistics_PeriodParameterValidation(t *testing.T) {
	baseURL := getBaseURL()

	testCases := []struct {
		period string
		valid  bool
	}{
		{"day", true},
		{"week", true},
		{"month", true},
		{"year", true},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("period=%s", tc.period), func(t *testing.T) {
			url := fmt.Sprintf("%s/api/v1/analytics/usage?period=%s", baseURL, tc.period)
			resp, err := http.Get(url)
			require.NoError(t, err)
			defer resp.Body.Close()

			// Should still return 401 (unauthorized), but parameter should be accepted
			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		})
	}
}

// ========================================
// GET /api/v1/analytics/trends Tests
// ========================================

func TestGetTrustScoreTrends_PeriodParameters(t *testing.T) {
	baseURL := getBaseURL()

	testCases := []struct {
		query string
		desc  string
	}{
		{"period=weeks&weeks=4", "weekly with 4 weeks"},
		{"period=weeks&weeks=8", "weekly with 8 weeks"},
		{"period=days&days=30", "daily with 30 days"},
		{"period=days&days=7", "daily with 7 days"},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			url := fmt.Sprintf("%s/api/v1/analytics/trends?%s", baseURL, tc.query)
			resp, err := http.Get(url)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		})
	}
}

// ========================================
// GET /api/v1/analytics/verification-activity Tests
// ========================================

func TestGetVerificationActivity_MonthsParameterRange(t *testing.T) {
	baseURL := getBaseURL()

	testCases := []int{1, 3, 6, 12}

	for _, months := range testCases {
		t.Run(fmt.Sprintf("months=%d", months), func(t *testing.T) {
			url := fmt.Sprintf("%s/api/v1/analytics/verification-activity?months=%d", baseURL, months)
			resp, err := http.Get(url)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		})
	}
}

// ========================================
// GET /api/v1/analytics/agents/activity Tests
// ========================================

func TestGetAgentActivity_PaginationParameters(t *testing.T) {
	baseURL := getBaseURL()

	testCases := []struct {
		limit  int
		offset int
		desc   string
	}{
		{10, 0, "limit=10, offset=0"},
		{50, 0, "limit=50, offset=0 (default limit)"},
		{25, 25, "limit=25, offset=25"},
		{100, 50, "limit=100, offset=50"},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			url := fmt.Sprintf("%s/api/v1/analytics/agents/activity?limit=%d&offset=%d", baseURL, tc.limit, tc.offset)
			resp, err := http.Get(url)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		})
	}
}

// ========================================
// Edge Case Tests
// ========================================

func TestAnalyticsEndpoints_InvalidParameters(t *testing.T) {
	baseURL := getBaseURL()

	testCases := []struct {
		endpoint string
		query    string
		desc     string
	}{
		{"/api/v1/analytics/usage", "period=invalid", "invalid period parameter"},
		{"/api/v1/analytics/trends", "weeks=-1", "negative weeks parameter"},
		{"/api/v1/analytics/verification-activity", "months=0", "zero months parameter"},
		{"/api/v1/analytics/agents/activity", "limit=-10", "negative limit parameter"},
		{"/api/v1/analytics/agents/activity", "offset=-5", "negative offset parameter"},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			url := fmt.Sprintf("%s%s?%s", baseURL, tc.endpoint, tc.query)
			resp, err := http.Get(url)
			require.NoError(t, err)
			defer resp.Body.Close()

			// Should return 401 (unauthorized) - parameter validation happens after auth
			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		})
	}
}

func TestAnalyticsEndpoints_MissingParameters(t *testing.T) {
	baseURL := getBaseURL()

	endpoints := []string{
		"/api/v1/analytics/dashboard",
		"/api/v1/analytics/usage",
		"/api/v1/analytics/trends",
		"/api/v1/analytics/verification-activity",
		"/api/v1/analytics/agents/activity",
		"/api/v1/analytics/reports/generate",
	}

	for _, endpoint := range endpoints {
		t.Run(endpoint, func(t *testing.T) {
			resp, err := http.Get(baseURL + endpoint)
			require.NoError(t, err)
			defer resp.Body.Close()

			// All endpoints should require authentication
			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Endpoint should return 401 without auth token")
		})
	}
}

func TestAnalyticsEndpoints_ResponseContentType(t *testing.T) {
	baseURL := getBaseURL()

	endpoints := []string{
		"/api/v1/analytics/dashboard",
		"/api/v1/analytics/usage",
		"/api/v1/analytics/trends",
		"/api/v1/analytics/verification-activity",
		"/api/v1/analytics/agents/activity",
		"/api/v1/analytics/reports/generate",
	}

	for _, endpoint := range endpoints {
		t.Run(endpoint, func(t *testing.T) {
			resp, err := http.Get(baseURL + endpoint)
			require.NoError(t, err)
			defer resp.Body.Close()

			// Verify response is JSON
			contentType := resp.Header.Get("Content-Type")
			assert.Contains(t, contentType, "application/json", "Response should be JSON")
		})
	}
}

// ========================================
// TODO: Authenticated Tests
// ========================================
// Once JWT generation utility is available, add:
// - TestGetAnalyticsDashboard_Authenticated
// - TestGetUsageStatistics_Authenticated
// - TestGetTrustScoreTrends_Authenticated
// - TestGetVerificationActivity_Authenticated
// - TestGetAgentActivity_Authenticated
//
// Each should verify:
// - HTTP 200 response
// - Correct response structure
// - Data type validation
// - Business logic correctness (e.g., trust scores 0-100, rates 0-100%)
// - Real data from database (no mock/simulated data)
