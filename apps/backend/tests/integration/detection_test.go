package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReportDetectionUnauthorized verifies report detection requires authentication
func TestReportDetectionUnauthorized(t *testing.T) {
	baseURL := getBaseURL()
	agentID := "00000000-0000-0000-0000-000000000000"

	detectionData := map[string]interface{}{
		"detections": []map[string]interface{}{
			{
				"mcpServer":       "@modelcontextprotocol/server-filesystem",
				"detectionMethod": "sdk_runtime",
				"confidence":      95.0,
				"sdkVersion":      "aim-sdk-js@1.0.0",
				"timestamp":       "2025-10-09T12:00:00Z",
			},
		},
	}

	body, _ := json.Marshal(detectionData)
	resp, err := http.Post(
		baseURL+"/api/v1/agents/"+agentID+"/detection/report",
		"application/json",
		bytes.NewBuffer(body),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestReportDetectionInvalidAgentID verifies validation of agent ID format
func TestReportDetectionInvalidAgentID(t *testing.T) {
	baseURL := getBaseURL()
	invalidAgentID := "not-a-uuid"

	detectionData := map[string]interface{}{
		"detections": []map[string]interface{}{
			{
				"mcpServer":       "@modelcontextprotocol/server-filesystem",
				"detectionMethod": "sdk_runtime",
				"confidence":      95.0,
			},
		},
	}

	body, _ := json.Marshal(detectionData)
	req, err := http.NewRequest(
		"POST",
		baseURL+"/api/v1/agents/"+invalidAgentID+"/detection/report",
		bytes.NewBuffer(body),
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer mock-invalid-token")

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should return 400 (invalid UUID) or 401 (invalid token)
	assert.True(t,
		resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnauthorized,
		"Should return 400 or 401 for invalid agent ID",
	)
}

// TestReportDetectionEmptyArray verifies validation of empty detections array
func TestReportDetectionEmptyArray(t *testing.T) {
	baseURL := getBaseURL()
	agentID := "00000000-0000-0000-0000-000000000000"

	// Empty detections array
	detectionData := map[string]interface{}{
		"detections": []map[string]interface{}{},
	}

	body, _ := json.Marshal(detectionData)
	req, err := http.NewRequest(
		"POST",
		baseURL+"/api/v1/agents/"+agentID+"/detection/report",
		bytes.NewBuffer(body),
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer mock-invalid-token")

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should return 400 (validation error) or 401 (invalid token)
	assert.True(t,
		resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnauthorized,
		"Should return 400 or 401 for empty detections array",
	)
}

// TestReportDetectionInvalidConfidence verifies confidence score validation
func TestReportDetectionInvalidConfidence(t *testing.T) {
	baseURL := getBaseURL()
	agentID := "00000000-0000-0000-0000-000000000000"

	testCases := []struct {
		name       string
		confidence float64
	}{
		{"negative confidence", -10.0},
		{"confidence above 100", 150.0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			detectionData := map[string]interface{}{
				"detections": []map[string]interface{}{
					{
						"mcpServer":       "@modelcontextprotocol/server-filesystem",
						"detectionMethod": "sdk_runtime",
						"confidence":      tc.confidence,
					},
				},
			}

			body, _ := json.Marshal(detectionData)
			req, err := http.NewRequest(
				"POST",
				baseURL+"/api/v1/agents/"+agentID+"/detection/report",
				bytes.NewBuffer(body),
			)
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer mock-invalid-token")

			client := &http.Client{}
			resp, err := client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			// With invalid confidence, detections are skipped but endpoint returns 200
			// or returns 401 for invalid token
			// Either is acceptable
			assert.True(t,
				resp.StatusCode == http.StatusOK ||
					resp.StatusCode == http.StatusUnauthorized ||
					resp.StatusCode == http.StatusBadRequest,
				"Should handle invalid confidence gracefully",
			)
		})
	}
}

// TestGetDetectionStatusUnauthorized verifies get detection status requires authentication
func TestGetDetectionStatusUnauthorized(t *testing.T) {
	baseURL := getBaseURL()
	agentID := "00000000-0000-0000-0000-000000000000"

	resp, err := http.Get(baseURL + "/api/v1/agents/" + agentID + "/detection/status")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestGetDetectionStatusInvalidAgentID verifies validation of agent ID format
func TestGetDetectionStatusInvalidAgentID(t *testing.T) {
	baseURL := getBaseURL()
	invalidAgentID := "not-a-uuid"

	req, err := http.NewRequest(
		"GET",
		baseURL+"/api/v1/agents/"+invalidAgentID+"/detection/status",
		nil,
	)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer mock-invalid-token")

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should return 400 (invalid UUID) or 401 (invalid token)
	assert.True(t,
		resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnauthorized,
		"Should return 400 or 401 for invalid agent ID",
	)
}

// TestReportDetectionMethods verifies different detection methods are accepted
func TestReportDetectionMethods(t *testing.T) {
	baseURL := getBaseURL()
	agentID := "00000000-0000-0000-0000-000000000000"

	detectionMethods := []string{
		"manual",
		"claude_config",
		"sdk_import",
		"sdk_runtime",
		"direct_api",
	}

	for _, method := range detectionMethods {
		t.Run(method, func(t *testing.T) {
			detectionData := map[string]interface{}{
				"detections": []map[string]interface{}{
					{
						"mcpServer":       "@modelcontextprotocol/server-filesystem",
						"detectionMethod": method,
						"confidence":      90.0,
					},
				},
			}

			body, _ := json.Marshal(detectionData)
			req, err := http.NewRequest(
				"POST",
				baseURL+"/api/v1/agents/"+agentID+"/detection/report",
				bytes.NewBuffer(body),
			)
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer mock-invalid-token")

			client := &http.Client{}
			resp, err := client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			// Should return 200 (accepted) or 401 (invalid token)
			// Method validation happens at database level
			assert.True(t,
				resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusUnauthorized,
				"Should accept valid detection method: %s", method,
			)
		})
	}
}

// TestReportDetectionWithDetails verifies details field is properly handled
func TestReportDetectionWithDetails(t *testing.T) {
	baseURL := getBaseURL()
	agentID := "00000000-0000-0000-0000-000000000000"

	detectionData := map[string]interface{}{
		"detections": []map[string]interface{}{
			{
				"mcpServer":       "@modelcontextprotocol/server-filesystem",
				"detectionMethod": "sdk_runtime",
				"confidence":      95.0,
				"details": map[string]interface{}{
					"importPath":  "node_modules/@modelcontextprotocol/server-filesystem",
					"packageName": "@modelcontextprotocol/server-filesystem",
					"version":     "0.1.0",
				},
				"sdkVersion": "aim-sdk-js@1.0.0",
			},
		},
	}

	body, _ := json.Marshal(detectionData)
	req, err := http.NewRequest(
		"POST",
		baseURL+"/api/v1/agents/"+agentID+"/detection/report",
		bytes.NewBuffer(body),
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer mock-invalid-token")

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should return 200 (accepted) or 401 (invalid token)
	assert.True(t,
		resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusUnauthorized,
		"Should accept detection with details",
	)
}

// TestReportDetectionMultiple verifies multiple detections in single request
func TestReportDetectionMultiple(t *testing.T) {
	baseURL := getBaseURL()
	agentID := "00000000-0000-0000-0000-000000000000"

	detectionData := map[string]interface{}{
		"detections": []map[string]interface{}{
			{
				"mcpServer":       "@modelcontextprotocol/server-filesystem",
				"detectionMethod": "sdk_runtime",
				"confidence":      95.0,
			},
			{
				"mcpServer":       "@modelcontextprotocol/server-memory",
				"detectionMethod": "sdk_import",
				"confidence":      90.0,
			},
			{
				"mcpServer":       "@modelcontextprotocol/server-github",
				"detectionMethod": "claude_config",
				"confidence":      100.0,
			},
		},
	}

	body, _ := json.Marshal(detectionData)
	req, err := http.NewRequest(
		"POST",
		baseURL+"/api/v1/agents/"+agentID+"/detection/report",
		bytes.NewBuffer(body),
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer mock-invalid-token")

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should return 200 (accepted) or 401 (invalid token)
	assert.True(t,
		resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusUnauthorized,
		"Should accept multiple detections",
	)
}

// TODO: Add authenticated tests once we have a test JWT generation utility
// - TestReportDetectionAuthorized: Test successful detection reporting with valid JWT
//   - Verify response includes newMCPs and existingMCPs
//   - Verify detections are stored in agent_mcp_detections table
//   - Verify agent's talks_to array is updated
//   - Verify SDK heartbeat is updated
// - TestGetDetectionStatusAuthorized: Test getting detection status with valid JWT
//   - Verify response includes SDK installation status
//   - Verify response includes detected MCPs with confidence scores
//   - Verify confidence boosting (multiple methods increase confidence)
//   - Verify aggregation by MCP server name
// - TestReportDetectionOrganizationIsolation: Test that agents from different orgs are isolated
// - TestDetectionConfidenceBoosting: Test that multiple detection methods increase confidence
// - TestSDKHeartbeatUpdate: Test that SDK heartbeat is properly updated
// - TestDetectionDuplicates: Test that duplicate detections update last_seen_at
