package domain

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConnectionType_Constants(t *testing.T) {
	tests := []struct {
		constant ConnectionType
		expected string
	}{
		{ConnectionTypeAutoDetected, "auto_detected"},
		{ConnectionTypeUserRegistered, "user_registered"},
		{ConnectionTypeAttested, "attested"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, ConnectionType(tt.expected), tt.constant)
		})
	}
}

func TestVerificationMethod_Constants(t *testing.T) {
	tests := []struct {
		constant VerificationMethod
		expected string
	}{
		{VerificationMethodAgentAttestation, "agent_attestation"},
		{VerificationMethodAPIKey, "api_key"},
		{VerificationMethodManual, "manual"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, VerificationMethod(tt.expected), tt.constant)
		})
	}
}

func TestAgentMCPConnection_Structure(t *testing.T) {
	now := time.Now()
	detectionID := uuid.New()
	lastAttestedAt := now.Add(-time.Hour)

	connection := AgentMCPConnection{
		ID:               uuid.New(),
		AgentID:          uuid.New(),
		MCPServerID:      uuid.New(),
		DetectionID:      &detectionID,
		ConnectionType:   ConnectionTypeAttested,
		FirstConnectedAt: now.Add(-24 * time.Hour),
		LastAttestedAt:   &lastAttestedAt,
		AttestationCount: 5,
		IsActive:         true,
		CreatedAt:        now.Add(-24 * time.Hour),
		UpdatedAt:        now,
	}

	assert.NotEqual(t, uuid.Nil, connection.ID)
	assert.Equal(t, ConnectionTypeAttested, connection.ConnectionType)
	assert.Equal(t, &detectionID, connection.DetectionID)
	assert.Equal(t, 5, connection.AttestationCount)
	assert.True(t, connection.IsActive)
}

func TestAgentMCPConnection_AutoDetected(t *testing.T) {
	connection := AgentMCPConnection{
		ID:               uuid.New(),
		AgentID:          uuid.New(),
		MCPServerID:      uuid.New(),
		ConnectionType:   ConnectionTypeAutoDetected,
		FirstConnectedAt: time.Now(),
		AttestationCount: 0,
		IsActive:         true,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	assert.Equal(t, ConnectionTypeAutoDetected, connection.ConnectionType)
	assert.Nil(t, connection.DetectionID)
	assert.Nil(t, connection.LastAttestedAt)
	assert.Equal(t, 0, connection.AttestationCount)
}

func TestAttestationPayload_Structure(t *testing.T) {
	payload := AttestationPayload{
		AgentID:              uuid.New().String(),
		CapabilitiesFound:    []string{"tools", "prompts", "resources"},
		Challenge:            "base64-challenge-nonce",
		ConnectionLatencyMs:  35.5,
		ConnectionSuccessful: true,
		HealthCheckPassed:    true,
		MCPName:              "github-mcp",
		MCPURL:               "https://mcp.github.com/api",
		SDKVersion:           "1.0.0",
		Timestamp:            time.Now().UTC().Format(time.RFC3339),
	}

	assert.NotEmpty(t, payload.AgentID)
	assert.Len(t, payload.CapabilitiesFound, 3)
	assert.Equal(t, 35.5, payload.ConnectionLatencyMs)
	assert.True(t, payload.ConnectionSuccessful)
	assert.True(t, payload.HealthCheckPassed)
	assert.Equal(t, "github-mcp", payload.MCPName)
	assert.Equal(t, "https://mcp.github.com/api", payload.MCPURL)
	assert.Equal(t, "1.0.0", payload.SDKVersion)
}

func TestAttestationPayload_ToCanonicalJSON(t *testing.T) {
	payload := AttestationPayload{
		AgentID:              "test-agent-id",
		CapabilitiesFound:    []string{"tools", "resources"},
		Challenge:            "test-challenge",
		ConnectionLatencyMs:  35.0,
		ConnectionSuccessful: true,
		HealthCheckPassed:    true,
		MCPName:              "test-mcp",
		MCPURL:               "https://test.mcp.com",
		SDKVersion:           "1.0.0",
		Timestamp:            "2024-01-15T10:30:00Z",
	}

	canonical, err := payload.ToCanonicalJSON()
	require.NoError(t, err)

	// Verify it's valid JSON
	var parsed map[string]interface{}
	err = json.Unmarshal(canonical, &parsed)
	require.NoError(t, err)

	// Verify key fields are present
	assert.Equal(t, "test-agent-id", parsed["agentId"])
	assert.Equal(t, "test-mcp", parsed["mcpName"])
	assert.Equal(t, true, parsed["connectionSuccessful"])
	assert.Equal(t, true, parsed["healthCheckPassed"])
}

func TestAttestationPayload_ToCanonicalJSON_WithoutChallenge(t *testing.T) {
	payload := AttestationPayload{
		AgentID:              "test-agent",
		CapabilitiesFound:    []string{},
		Challenge:            "", // Empty challenge
		ConnectionLatencyMs:  50.0,
		ConnectionSuccessful: true,
		HealthCheckPassed:    false,
		MCPName:              "test-mcp",
		MCPURL:               "https://test.com",
		SDKVersion:           "2.0.0",
		Timestamp:            "2024-01-15T10:30:00Z",
	}

	canonical, err := payload.ToCanonicalJSON()
	require.NoError(t, err)

	// Challenge should not be present when empty
	canonicalStr := string(canonical)
	assert.NotContains(t, canonicalStr, "challenge")
}

func TestAttestationPayload_ToCanonicalJSON_FloatFormatting(t *testing.T) {
	// Test that integer float values get .0 suffix (Python compatibility)
	payload := AttestationPayload{
		AgentID:              "test",
		CapabilitiesFound:    []string{},
		ConnectionLatencyMs:  35.0, // Integer value
		ConnectionSuccessful: true,
		HealthCheckPassed:    true,
		MCPName:              "test",
		MCPURL:               "https://test.com",
		SDKVersion:           "1.0",
		Timestamp:            "2024-01-01T00:00:00Z",
	}

	canonical, err := payload.ToCanonicalJSON()
	require.NoError(t, err)

	// Should contain "35.0" not "35"
	assert.Contains(t, string(canonical), "35.0")
}

func TestAttestationChallenge_Structure(t *testing.T) {
	now := time.Now()

	challenge := AttestationChallenge{
		ID:          uuid.New(),
		Challenge:   "random-base64-nonce-32-bytes",
		AgentID:     uuid.New(),
		MCPServerID: uuid.New(),
		ExpiresAt:   now.Add(5 * time.Minute),
		UsedAt:      nil,
		CreatedAt:   now,
	}

	assert.NotEqual(t, uuid.Nil, challenge.ID)
	assert.NotEmpty(t, challenge.Challenge)
	assert.True(t, challenge.ExpiresAt.After(now))
	assert.Nil(t, challenge.UsedAt)
}

func TestAttestationChallenge_Used(t *testing.T) {
	now := time.Now()
	usedAt := now.Add(1 * time.Minute)

	challenge := AttestationChallenge{
		ID:          uuid.New(),
		Challenge:   "test-challenge",
		AgentID:     uuid.New(),
		MCPServerID: uuid.New(),
		ExpiresAt:   now.Add(5 * time.Minute),
		UsedAt:      &usedAt,
		CreatedAt:   now,
	}

	assert.NotNil(t, challenge.UsedAt)
	assert.True(t, (*challenge.UsedAt).Before(challenge.ExpiresAt))
}

func TestMCPAttestation_Structure(t *testing.T) {
	now := time.Now()
	agentID := uuid.New()
	verifiedAt := now.Add(-10 * time.Minute)

	attestation := MCPAttestation{
		ID:          uuid.New(),
		MCPServerID: uuid.New(),
		AgentID:     &agentID,
		AttestationData: AttestationPayload{
			AgentID:              agentID.String(),
			CapabilitiesFound:    []string{"tools"},
			ConnectionLatencyMs:  25.0,
			ConnectionSuccessful: true,
			HealthCheckPassed:    true,
			MCPName:              "test-mcp",
			MCPURL:               "https://test.com",
			SDKVersion:           "1.0.0",
			Timestamp:            now.Format(time.RFC3339),
		},
		Signature:         "base64-signature",
		SignatureVerified: true,
		VerifiedAt:        &verifiedAt,
		ExpiresAt:         now.Add(24 * time.Hour),
		IsValid:           true,
		CreatedAt:         now,
		AgentName:         "test-agent",
		AgentTrustScore:   0.85,
	}

	assert.NotEqual(t, uuid.Nil, attestation.ID)
	assert.Equal(t, &agentID, attestation.AgentID)
	assert.True(t, attestation.SignatureVerified)
	assert.True(t, attestation.IsValid)
	assert.Equal(t, "test-agent", attestation.AgentName)
	assert.Equal(t, 0.85, attestation.AgentTrustScore)
}

func TestMCPAttestation_ManualWithoutAgent(t *testing.T) {
	now := time.Now()

	attestation := MCPAttestation{
		ID:          uuid.New(),
		MCPServerID: uuid.New(),
		AgentID:     nil, // Manual attestation
		AttestationData: AttestationPayload{
			MCPName: "manual-mcp",
			MCPURL:  "https://manual.com",
		},
		SignatureVerified: false,
		ExpiresAt:         now.Add(24 * time.Hour),
		IsValid:           true,
		CreatedAt:         now,
	}

	assert.Nil(t, attestation.AgentID)
	assert.False(t, attestation.SignatureVerified)
}

func TestAttestationWithAgentDetails_Structure(t *testing.T) {
	details := AttestationWithAgentDetails{
		ID:                    uuid.New(),
		AgentID:               uuid.New(),
		AgentName:             "verified-agent",
		AgentTrustScore:       92.5,
		VerifiedAt:            time.Now().Format(time.RFC3339),
		ExpiresAt:             time.Now().Add(24 * time.Hour).Format(time.RFC3339),
		CapabilitiesConfirmed: []string{"tools", "resources", "prompts"},
		ConnectionLatencyMs:   15.5,
		HealthCheckPassed:     true,
		IsValid:               true,
		AttestationType:       "sdk",
		AttestedBy:            "verified-agent",
		AttesterType:          "agent",
		SignatureVerified:     true,
		SDKVersion:            "1.5.0",
		ConnectionSuccessful:  true,
		AgentOwnerName:        "John Doe",
		AgentOwnerID:          uuid.New(),
	}

	assert.Equal(t, "verified-agent", details.AgentName)
	assert.Equal(t, 92.5, details.AgentTrustScore)
	assert.Len(t, details.CapabilitiesConfirmed, 3)
	assert.True(t, details.SignatureVerified)
	assert.Equal(t, "sdk", details.AttestationType)
	assert.Equal(t, "agent", details.AttesterType)
}

func TestAttestationWithAgentDetails_ManualAttestation(t *testing.T) {
	details := AttestationWithAgentDetails{
		ID:                    uuid.New(),
		AgentID:               uuid.New(),
		AgentName:             "",
		VerifiedAt:            time.Now().Format(time.RFC3339),
		ExpiresAt:             time.Now().Add(24 * time.Hour).Format(time.RFC3339),
		CapabilitiesConfirmed: []string{"tools"},
		IsValid:               true,
		AttestationType:       "manual",
		AttestedBy:            "Admin User",
		AttesterType:          "user",
		SignatureVerified:     false,
	}

	assert.Equal(t, "manual", details.AttestationType)
	assert.Equal(t, "user", details.AttesterType)
	assert.False(t, details.SignatureVerified)
}

func TestEscapeJSONString(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"with spaces", "with spaces"},
		{`with "quotes"`, `with \"quotes\"`},
		{`back\\slash`, `back\\\\slash`},
		{"new\nline", `new\nline`},
		{"tab\there", `tab\there`},
		{"carriage\rreturn", `carriage\rreturn`},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := escapeJSONString(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatJSONField(t *testing.T) {
	result := formatJSONField("key", "value")
	assert.Equal(t, `"key":"value"`, result)

	result = formatJSONField("name", "test \"quoted\"")
	assert.Equal(t, `"name":"test \"quoted\""`, result)
}

func TestFormatJSONFloat(t *testing.T) {
	// Integer value should have .0
	result := formatJSONFloat("latency", 35.0)
	assert.Equal(t, `"latency":35.0`, result)

	// Non-integer value
	result = formatJSONFloat("score", 35.5)
	assert.Equal(t, `"score":35.5`, result)
}

func TestFormatJSONBool(t *testing.T) {
	result := formatJSONBool("active", true)
	assert.Equal(t, `"active":true`, result)

	result = formatJSONBool("disabled", false)
	assert.Equal(t, `"disabled":false`, result)
}

func TestFormatJSONArray(t *testing.T) {
	result := formatJSONArray("items", []string{})
	assert.Equal(t, `"items":[]`, result)

	result = formatJSONArray("items", []string{"a", "b", "c"})
	assert.Equal(t, `"items":["a","b","c"]`, result)
}

func TestJoinStrings(t *testing.T) {
	result := joinStrings([]string{}, ",")
	assert.Equal(t, "", result)

	result = joinStrings([]string{"a"}, ",")
	assert.Equal(t, "a", result)

	result = joinStrings([]string{"a", "b", "c"}, ",")
	assert.Equal(t, "a,b,c", result)
}

func TestMCPAttestation_JSONSerialization(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	agentID := uuid.New()

	attestation := MCPAttestation{
		ID:          uuid.New(),
		MCPServerID: uuid.New(),
		AgentID:     &agentID,
		AttestationData: AttestationPayload{
			AgentID:              agentID.String(),
			CapabilitiesFound:    []string{"tools"},
			ConnectionLatencyMs:  20.0,
			ConnectionSuccessful: true,
			HealthCheckPassed:    true,
			MCPName:              "test",
			MCPURL:               "https://test.com",
			SDKVersion:           "1.0",
			Timestamp:            now.Format(time.RFC3339),
		},
		Signature:         "sig",
		SignatureVerified: true,
		ExpiresAt:         now.Add(24 * time.Hour),
		IsValid:           true,
		CreatedAt:         now,
	}

	// Serialize to JSON
	data, err := json.Marshal(attestation)
	require.NoError(t, err)

	// Deserialize back
	var restored MCPAttestation
	err = json.Unmarshal(data, &restored)
	require.NoError(t, err)

	// Verify key fields
	assert.Equal(t, attestation.ID, restored.ID)
	assert.Equal(t, attestation.SignatureVerified, restored.SignatureVerified)
	assert.Equal(t, attestation.IsValid, restored.IsValid)
}
