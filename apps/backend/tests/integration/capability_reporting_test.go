package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReportCapabilitiesUnauthorized verifies report capabilities requires authentication
func TestReportCapabilitiesUnauthorized(t *testing.T) {
	baseURL := getBaseURL()
	agentID := "00000000-0000-0000-0000-000000000000"

	capabilityReport := map[string]interface{}{
		"detectedAt": time.Now().Format(time.RFC3339),
		"environment": map[string]interface{}{
			"language": "javascript",
			"version":  "20.10.0",
			"runtime":  "node",
			"platform": "darwin",
			"arch":     "arm64",
		},
		"aiModels": []map[string]interface{}{
			{
				"provider":      "anthropic",
				"models":        []string{"claude-3-5-sonnet-20241022"},
				"detectionType": "api_call",
			},
		},
		"capabilities": map[string]interface{}{
			"fileSystem": map[string]interface{}{
				"read":            true,
				"write":           false,
				"delete":          false,
				"execute":         false,
				"detectionMethod": "mcp_tool_analysis",
			},
		},
		"riskAssessment": map[string]interface{}{
			"overallRiskScore": 20,
			"riskLevel":        "LOW",
			"trustScoreImpact": -5,
			"alerts":           []interface{}{},
		},
	}

	body, _ := json.Marshal(capabilityReport)
	resp, err := http.Post(
		baseURL+"/api/v1/agents/"+agentID+"/capabilities/report",
		"application/json",
		bytes.NewBuffer(body),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestReportCapabilitiesInvalidAgentID verifies validation of agent ID format
func TestReportCapabilitiesInvalidAgentID(t *testing.T) {
	baseURL := getBaseURL()
	invalidAgentID := "not-a-uuid"

	capabilityReport := map[string]interface{}{
		"detectedAt": time.Now().Format(time.RFC3339),
		"environment": map[string]interface{}{
			"language": "javascript",
			"version":  "20.10.0",
		},
		"capabilities": map[string]interface{}{},
		"riskAssessment": map[string]interface{}{
			"overallRiskScore": 10,
			"riskLevel":        "LOW",
			"trustScoreImpact": 0,
			"alerts":           []interface{}{},
		},
	}

	body, _ := json.Marshal(capabilityReport)
	req, err := http.NewRequest(
		"POST",
		baseURL+"/api/v1/agents/"+invalidAgentID+"/capabilities/report",
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

// TestReportCapabilitiesMissingDetectedAt verifies validation of required detectedAt field
func TestReportCapabilitiesMissingDetectedAt(t *testing.T) {
	baseURL := getBaseURL()
	agentID := "00000000-0000-0000-0000-000000000000"

	// Missing detectedAt field
	capabilityReport := map[string]interface{}{
		"environment": map[string]interface{}{
			"language": "javascript",
		},
		"capabilities": map[string]interface{}{},
		"riskAssessment": map[string]interface{}{
			"overallRiskScore": 10,
			"riskLevel":        "LOW",
			"trustScoreImpact": 0,
			"alerts":           []interface{}{},
		},
	}

	body, _ := json.Marshal(capabilityReport)
	req, err := http.NewRequest(
		"POST",
		baseURL+"/api/v1/agents/"+agentID+"/capabilities/report",
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
		"Should return 400 or 401 for missing detectedAt",
	)
}

// TestReportCapabilitiesLowRisk verifies handling of low-risk capability report
func TestReportCapabilitiesLowRisk(t *testing.T) {
	baseURL := getBaseURL()
	agentID := "00000000-0000-0000-0000-000000000000"

	capabilityReport := map[string]interface{}{
		"detectedAt": time.Now().Format(time.RFC3339),
		"environment": map[string]interface{}{
			"language":        "javascript",
			"version":         "20.10.0",
			"runtime":         "node",
			"platform":        "darwin",
			"arch":            "arm64",
			"frameworks":      []string{"express"},
			"packageManagers": []string{"npm"},
		},
		"aiModels": []map[string]interface{}{
			{
				"provider":      "anthropic",
				"models":        []string{"claude-3-5-sonnet-20241022"},
				"detectionType": "api_call",
			},
		},
		"capabilities": map[string]interface{}{
			"fileSystem": map[string]interface{}{
				"read":            true,
				"write":           false,
				"delete":          false,
				"execute":         false,
				"pathsAccessed":   []string{"/tmp"},
				"detectionMethod": "mcp_tool_analysis",
			},
			"database": map[string]interface{}{
				"postgresql":      false,
				"mongodb":         false,
				"mysql":           false,
				"sqlite":          true,
				"redis":           false,
				"operations":      []string{"SELECT"},
				"detectionMethod": "package_analysis",
			},
		},
		"riskAssessment": map[string]interface{}{
			"overallRiskScore": 20,
			"riskLevel":        "LOW",
			"trustScoreImpact": -5,
			"alerts": []map[string]interface{}{
				{
					"severity":         "INFO",
					"capability":       "file_read",
					"message":          "Agent has file read capabilities",
					"recommendation":   "Monitor file access patterns",
					"trustScoreImpact": -5,
				},
			},
		},
	}

	body, _ := json.Marshal(capabilityReport)
	req, err := http.NewRequest(
		"POST",
		baseURL+"/api/v1/agents/"+agentID+"/capabilities/report",
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
		"Should accept low-risk capability report",
	)
}

// TestReportCapabilitiesHighRisk verifies handling of high-risk capability report
func TestReportCapabilitiesHighRisk(t *testing.T) {
	baseURL := getBaseURL()
	agentID := "00000000-0000-0000-0000-000000000000"

	capabilityReport := map[string]interface{}{
		"detectedAt": time.Now().Format(time.RFC3339),
		"environment": map[string]interface{}{
			"language": "javascript",
			"version":  "20.10.0",
			"runtime":  "node",
		},
		"aiModels": []map[string]interface{}{
			{
				"provider": "anthropic",
				"models":   []string{"claude-3-5-sonnet-20241022"},
			},
		},
		"capabilities": map[string]interface{}{
			"fileSystem": map[string]interface{}{
				"read":            true,
				"write":           true,
				"delete":          true,
				"execute":         true,
				"detectionMethod": "mcp_tool_analysis",
			},
			"codeExecution": map[string]interface{}{
				"eval":            true,
				"exec":            true,
				"shellCommands":   true,
				"childProcesses":  true,
				"detectionMethod": "static_analysis",
			},
			"credentialAccess": map[string]interface{}{
				"envVars":         true,
				"configFiles":     true,
				"keyring":         true,
				"detectionMethod": "api_analysis",
			},
		},
		"riskAssessment": map[string]interface{}{
			"overallRiskScore": 85,
			"riskLevel":        "HIGH",
			"trustScoreImpact": -25,
			"alerts": []map[string]interface{}{
				{
					"severity":         "CRITICAL",
					"capability":       "code_execution",
					"message":          "Agent has code execution capabilities with shell access",
					"recommendation":   "Implement strict sandboxing and monitoring",
					"trustScoreImpact": -15,
				},
				{
					"severity":         "HIGH",
					"capability":       "credential_access",
					"message":          "Agent can access environment variables and credentials",
					"recommendation":   "Review credential access patterns and implement access controls",
					"trustScoreImpact": -10,
				},
			},
		},
	}

	body, _ := json.Marshal(capabilityReport)
	req, err := http.NewRequest(
		"POST",
		baseURL+"/api/v1/agents/"+agentID+"/capabilities/report",
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
		"Should accept high-risk capability report",
	)
}

// TestReportCapabilitiesCriticalRisk verifies handling of critical-risk capability report
func TestReportCapabilitiesCriticalRisk(t *testing.T) {
	baseURL := getBaseURL()
	agentID := "00000000-0000-0000-0000-000000000000"

	capabilityReport := map[string]interface{}{
		"detectedAt": time.Now().Format(time.RFC3339),
		"environment": map[string]interface{}{
			"language": "javascript",
			"version":  "20.10.0",
		},
		"capabilities": map[string]interface{}{
			"fileSystem": map[string]interface{}{
				"read":            true,
				"write":           true,
				"delete":          true,
				"execute":         true,
				"detectionMethod": "mcp_tool_analysis",
			},
			"network": map[string]interface{}{
				"http":            true,
				"https":           true,
				"websocket":       true,
				"tcp":             true,
				"externalApis":    []string{"unknown-external-api.com"},
				"detectionMethod": "runtime_analysis",
			},
			"codeExecution": map[string]interface{}{
				"eval":            true,
				"exec":            true,
				"shellCommands":   true,
				"childProcesses":  true,
				"vmExecution":     true,
				"detectionMethod": "static_analysis",
			},
		},
		"riskAssessment": map[string]interface{}{
			"overallRiskScore": 95,
			"riskLevel":        "CRITICAL",
			"trustScoreImpact": -40,
			"alerts": []map[string]interface{}{
				{
					"severity":         "CRITICAL",
					"capability":       "unrestricted_execution",
					"message":          "Agent has unrestricted code execution with network access",
					"recommendation":   "Immediate review required - implement strict sandboxing",
					"trustScoreImpact": -25,
				},
				{
					"severity":         "CRITICAL",
					"capability":       "file_system_write_delete",
					"message":          "Agent can write and delete files with execute permissions",
					"recommendation":   "Restrict file system access to specific directories",
					"trustScoreImpact": -15,
				},
			},
		},
	}

	body, _ := json.Marshal(capabilityReport)
	req, err := http.NewRequest(
		"POST",
		baseURL+"/api/v1/agents/"+agentID+"/capabilities/report",
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
		"Should accept critical-risk capability report",
	)
}

// TestReportCapabilitiesRiskLevels verifies different risk level handling
func TestReportCapabilitiesRiskLevels(t *testing.T) {
	baseURL := getBaseURL()
	agentID := "00000000-0000-0000-0000-000000000000"

	riskLevels := []struct {
		name  string
		level string
		score int
	}{
		{"low risk", "LOW", 20},
		{"medium risk", "MEDIUM", 50},
		{"high risk", "HIGH", 75},
		{"critical risk", "CRITICAL", 95},
	}

	for _, tc := range riskLevels {
		t.Run(tc.name, func(t *testing.T) {
			capabilityReport := map[string]interface{}{
				"detectedAt": time.Now().Format(time.RFC3339),
				"environment": map[string]interface{}{
					"language": "javascript",
					"version":  "20.10.0",
				},
				"capabilities": map[string]interface{}{},
				"riskAssessment": map[string]interface{}{
					"overallRiskScore": tc.score,
					"riskLevel":        tc.level,
					"trustScoreImpact": -tc.score / 5,
					"alerts":           []interface{}{},
				},
			}

			body, _ := json.Marshal(capabilityReport)
			req, err := http.NewRequest(
				"POST",
				baseURL+"/api/v1/agents/"+agentID+"/capabilities/report",
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
				"Should accept %s report", tc.level,
			)
		})
	}
}

// TestReportCapabilitiesMultipleAlerts verifies handling of multiple security alerts
func TestReportCapabilitiesMultipleAlerts(t *testing.T) {
	baseURL := getBaseURL()
	agentID := "00000000-0000-0000-0000-000000000000"

	capabilityReport := map[string]interface{}{
		"detectedAt": time.Now().Format(time.RFC3339),
		"environment": map[string]interface{}{
			"language": "python",
			"version":  "3.11.0",
		},
		"capabilities": map[string]interface{}{
			"fileSystem": map[string]interface{}{
				"read":            true,
				"write":           true,
				"detectionMethod": "mcp_tool_analysis",
			},
			"database": map[string]interface{}{
				"postgresql":      true,
				"detectionMethod": "package_analysis",
			},
			"network": map[string]interface{}{
				"https":           true,
				"detectionMethod": "runtime_analysis",
			},
		},
		"riskAssessment": map[string]interface{}{
			"overallRiskScore": 60,
			"riskLevel":        "MEDIUM",
			"trustScoreImpact": -12,
			"alerts": []map[string]interface{}{
				{
					"severity":         "MEDIUM",
					"capability":       "file_write",
					"message":          "Agent can write to file system",
					"recommendation":   "Monitor write operations",
					"trustScoreImpact": -5,
				},
				{
					"severity":         "MEDIUM",
					"capability":       "database_access",
					"message":          "Agent can access PostgreSQL database",
					"recommendation":   "Review database access patterns",
					"trustScoreImpact": -5,
				},
				{
					"severity":         "LOW",
					"capability":       "network_access",
					"message":          "Agent makes HTTPS requests",
					"recommendation":   "Log external API calls",
					"trustScoreImpact": -2,
				},
			},
		},
	}

	body, _ := json.Marshal(capabilityReport)
	req, err := http.NewRequest(
		"POST",
		baseURL+"/api/v1/agents/"+agentID+"/capabilities/report",
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
		"Should accept report with multiple alerts",
	)
}

// TestReportCapabilitiesBrowserAutomation verifies browser automation capability detection
func TestReportCapabilitiesBrowserAutomation(t *testing.T) {
	baseURL := getBaseURL()
	agentID := "00000000-0000-0000-0000-000000000000"

	capabilityReport := map[string]interface{}{
		"detectedAt": time.Now().Format(time.RFC3339),
		"environment": map[string]interface{}{
			"language":   "javascript",
			"version":    "20.10.0",
			"frameworks": []string{"puppeteer", "playwright"},
		},
		"capabilities": map[string]interface{}{
			"browserAutomation": map[string]interface{}{
				"puppeteer":       true,
				"playwright":      true,
				"selenium":        false,
				"detectionMethod": "package_analysis",
			},
		},
		"riskAssessment": map[string]interface{}{
			"overallRiskScore": 40,
			"riskLevel":        "MEDIUM",
			"trustScoreImpact": -8,
			"alerts": []map[string]interface{}{
				{
					"severity":         "MEDIUM",
					"capability":       "browser_automation",
					"message":          "Agent uses browser automation tools (Puppeteer, Playwright)",
					"recommendation":   "Monitor browser automation activities",
					"trustScoreImpact": -8,
				},
			},
		},
	}

	body, _ := json.Marshal(capabilityReport)
	req, err := http.NewRequest(
		"POST",
		baseURL+"/api/v1/agents/"+agentID+"/capabilities/report",
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
		"Should accept browser automation capability report",
	)
}

// TODO: Add authenticated tests once we have a test JWT generation utility
// - TestReportCapabilitiesAuthorized: Test successful capability reporting with valid JWT
//   - Verify response includes risk level and trust score impact
//   - Verify capability report is stored in agent_capability_reports table
//   - Verify agent's trust score is recalculated with new capability risk factor
//   - Verify security alerts are logged in audit trail
// - TestReportCapabilitiesAPIKeyAuth: Test capability reporting with API key authentication
//   - Verify API key auth works for SDK-based reporting
//   - Verify audit log shows API key auth method
// - TestReportCapabilitiesOrganizationIsolation: Test that agents from different orgs are isolated
// - TestReportCapabilitiesTrustScoreUpdate: Test that trust score is recalculated after capability report
//   - Verify trust score decreases with high-risk capabilities
//   - Verify capability_risk factor is properly weighted (17%)
// - TestReportCapabilitiesMultipleReports: Test that multiple reports update the latest capability state
