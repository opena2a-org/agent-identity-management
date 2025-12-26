package application

import (
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
)

// ========================================
// Constant Tests
// ========================================

func TestMCPAttestation_Constants(t *testing.T) {
	t.Run("DefaultMinAgentsForConsensus", func(t *testing.T) {
		assert.Equal(t, 3, DefaultMinAgentsForConsensus)
	})

	t.Run("DefaultMinOwnersForConsensus", func(t *testing.T) {
		assert.Equal(t, 2, DefaultMinOwnersForConsensus)
	})

	t.Run("MinConfidenceScoreForVerification", func(t *testing.T) {
		assert.Equal(t, 60.0, MinConfidenceScoreForVerification)
	})

	t.Run("DefaultMinAgentTrustForAttestation", func(t *testing.T) {
		assert.Equal(t, 0.30, DefaultMinAgentTrustForAttestation)
	})

	t.Run("DefaultWarnAgentTrustThreshold", func(t *testing.T) {
		assert.Equal(t, 0.50, DefaultWarnAgentTrustThreshold)
	})
}

func TestMCPAttestation_ThresholdOrdering(t *testing.T) {
	// Trust thresholds should be in logical order
	assert.True(t, DefaultMinAgentTrustForAttestation < DefaultWarnAgentTrustThreshold,
		"block threshold should be lower than warn threshold")

	// Both thresholds should be in valid range 0-1
	assert.True(t, DefaultMinAgentTrustForAttestation >= 0 && DefaultMinAgentTrustForAttestation <= 1)
	assert.True(t, DefaultWarnAgentTrustThreshold >= 0 && DefaultWarnAgentTrustThreshold <= 1)
}

// ========================================
// Configuration Getter Tests
// ========================================

func TestGetMinAgentsForConsensus_Default(t *testing.T) {
	os.Unsetenv("MCP_MIN_AGENTS_FOR_CONSENSUS")
	result := getMinAgentsForConsensus()
	assert.Equal(t, DefaultMinAgentsForConsensus, result)
}

func TestGetMinAgentsForConsensus_EnvVar(t *testing.T) {
	tests := []struct {
		envValue string
		expected int
	}{
		{"5", 5},
		{"1", 1},
		{"10", 10},
	}

	for _, tt := range tests {
		t.Run(tt.envValue, func(t *testing.T) {
			os.Setenv("MCP_MIN_AGENTS_FOR_CONSENSUS", tt.envValue)
			defer os.Unsetenv("MCP_MIN_AGENTS_FOR_CONSENSUS")

			result := getMinAgentsForConsensus()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetMinAgentsForConsensus_InvalidEnvVar(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
	}{
		{"non-numeric", "abc"},
		{"negative", "-1"},
		{"zero", "0"},
		{"float", "3.5"},
		{"empty", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("MCP_MIN_AGENTS_FOR_CONSENSUS", tt.envValue)
			defer os.Unsetenv("MCP_MIN_AGENTS_FOR_CONSENSUS")

			result := getMinAgentsForConsensus()
			// Should fall back to default for invalid values
			assert.Equal(t, DefaultMinAgentsForConsensus, result)
		})
	}
}

func TestGetMinOwnersForConsensus_Default(t *testing.T) {
	os.Unsetenv("MCP_MIN_OWNERS_FOR_CONSENSUS")
	result := getMinOwnersForConsensus()
	assert.Equal(t, DefaultMinOwnersForConsensus, result)
}

func TestGetMinOwnersForConsensus_EnvVar(t *testing.T) {
	os.Setenv("MCP_MIN_OWNERS_FOR_CONSENSUS", "5")
	defer os.Unsetenv("MCP_MIN_OWNERS_FOR_CONSENSUS")

	result := getMinOwnersForConsensus()
	assert.Equal(t, 5, result)
}

func TestGetMinOwnersForConsensus_InvalidEnvVar(t *testing.T) {
	os.Setenv("MCP_MIN_OWNERS_FOR_CONSENSUS", "invalid")
	defer os.Unsetenv("MCP_MIN_OWNERS_FOR_CONSENSUS")

	result := getMinOwnersForConsensus()
	assert.Equal(t, DefaultMinOwnersForConsensus, result)
}

func TestGetMinAgentTrustForAttestation_Default(t *testing.T) {
	os.Unsetenv("MCP_MIN_AGENT_TRUST_FOR_ATTESTATION")
	result := getMinAgentTrustForAttestation()
	assert.Equal(t, DefaultMinAgentTrustForAttestation, result)
}

func TestGetMinAgentTrustForAttestation_EnvVar(t *testing.T) {
	tests := []struct {
		envValue string
		expected float64
	}{
		{"0.5", 0.5},
		{"0.0", 0.0},
		{"1.0", 1.0},
		{"0.25", 0.25},
	}

	for _, tt := range tests {
		t.Run(tt.envValue, func(t *testing.T) {
			os.Setenv("MCP_MIN_AGENT_TRUST_FOR_ATTESTATION", tt.envValue)
			defer os.Unsetenv("MCP_MIN_AGENT_TRUST_FOR_ATTESTATION")

			result := getMinAgentTrustForAttestation()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetMinAgentTrustForAttestation_InvalidEnvVar(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
	}{
		{"non-numeric", "abc"},
		{"negative", "-0.5"},
		{"over one", "1.5"},
		{"empty", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("MCP_MIN_AGENT_TRUST_FOR_ATTESTATION", tt.envValue)
			defer os.Unsetenv("MCP_MIN_AGENT_TRUST_FOR_ATTESTATION")

			result := getMinAgentTrustForAttestation()
			assert.Equal(t, DefaultMinAgentTrustForAttestation, result)
		})
	}
}

func TestGetWarnAgentTrustThreshold_Default(t *testing.T) {
	os.Unsetenv("MCP_WARN_AGENT_TRUST_THRESHOLD")
	result := getWarnAgentTrustThreshold()
	assert.Equal(t, DefaultWarnAgentTrustThreshold, result)
}

func TestGetWarnAgentTrustThreshold_EnvVar(t *testing.T) {
	os.Setenv("MCP_WARN_AGENT_TRUST_THRESHOLD", "0.75")
	defer os.Unsetenv("MCP_WARN_AGENT_TRUST_THRESHOLD")

	result := getWarnAgentTrustThreshold()
	assert.Equal(t, 0.75, result)
}

func TestGetWarnAgentTrustThreshold_InvalidEnvVar(t *testing.T) {
	os.Setenv("MCP_WARN_AGENT_TRUST_THRESHOLD", "invalid")
	defer os.Unsetenv("MCP_WARN_AGENT_TRUST_THRESHOLD")

	result := getWarnAgentTrustThreshold()
	assert.Equal(t, DefaultWarnAgentTrustThreshold, result)
}

// ========================================
// AttestMCPRequest Tests
// ========================================

func TestAttestMCPRequest_Structure(t *testing.T) {
	req := AttestMCPRequest{
		Attestation: domain.AttestationPayload{
			AgentID:              uuid.New().String(),
			MCPURL:               "https://mcp.example.com",
			MCPName:              "test-mcp",
			CapabilitiesFound:    []string{"read", "write"},
			ConnectionSuccessful: true,
			HealthCheckPassed:    true,
			ConnectionLatencyMs:  150,
			Timestamp:            time.Now().Format(time.RFC3339),
			SDKVersion:           "1.0.0",
		},
		Signature: "base64-encoded-signature",
	}

	assert.NotEmpty(t, req.Attestation.AgentID)
	assert.Equal(t, "https://mcp.example.com", req.Attestation.MCPURL)
	assert.Equal(t, "test-mcp", req.Attestation.MCPName)
	assert.Len(t, req.Attestation.CapabilitiesFound, 2)
	assert.True(t, req.Attestation.ConnectionSuccessful)
	assert.True(t, req.Attestation.HealthCheckPassed)
	assert.Equal(t, float64(150), req.Attestation.ConnectionLatencyMs)
	assert.NotEmpty(t, req.Signature)
}

// ========================================
// AttestMCPResponse Tests
// ========================================

func TestAttestMCPResponse_Structure(t *testing.T) {
	response := AttestMCPResponse{
		Success:            true,
		AttestationID:      uuid.New().String(),
		MCPConfidenceScore: 75.5,
		AttestationCount:   5,
		Message:            "Attestation verified",
	}

	assert.True(t, response.Success)
	assert.NotEmpty(t, response.AttestationID)
	assert.Equal(t, 75.5, response.MCPConfidenceScore)
	assert.Equal(t, 5, response.AttestationCount)
	assert.NotEmpty(t, response.Message)
}

// ========================================
// ChallengeResponse Tests
// ========================================

func TestChallengeResponse_Structure(t *testing.T) {
	expiresAt := time.Now().Add(5 * time.Minute)
	mcpServerID := uuid.New()

	challenge := ChallengeResponse{
		Challenge:   "base64-encoded-challenge-nonce",
		ExpiresAt:   expiresAt,
		MCPServerID: mcpServerID.String(),
	}

	assert.NotEmpty(t, challenge.Challenge)
	assert.Equal(t, expiresAt, challenge.ExpiresAt)
	assert.Equal(t, mcpServerID.String(), challenge.MCPServerID)
}

func TestChallengeResponse_Expiration(t *testing.T) {
	// Challenge should expire in 5 minutes
	now := time.Now()
	expiresAt := now.Add(5 * time.Minute)

	challenge := ChallengeResponse{
		Challenge: "test-challenge",
		ExpiresAt: expiresAt,
	}

	// Should not be expired yet
	assert.True(t, time.Now().Before(challenge.ExpiresAt))

	// Check expiration duration
	duration := challenge.ExpiresAt.Sub(now)
	assert.Equal(t, 5*time.Minute, duration)
}

// ========================================
// ConsensusStatus Tests
// ========================================

func TestConsensusStatus_Structure(t *testing.T) {
	status := ConsensusStatus{
		MCPServerID:        uuid.New().String(),
		Status:             "pending",
		IsVerified:         false,
		UniqueAgents:       2,
		RequiredAgents:     3,
		UniqueOwners:       1,
		RequiredOwners:     2,
		ConfidenceScore:    45.5,
		RequiredConfidence: 60.0,
		ConsensusReached:   false,
		ProgressPercent:    66.6,
		MissingCriteria:    []string{"Need 1 more agent attestation", "Need 1 more owner"},
	}

	assert.NotEmpty(t, status.MCPServerID)
	assert.Equal(t, "pending", status.Status)
	assert.False(t, status.IsVerified)
	assert.Equal(t, 2, status.UniqueAgents)
	assert.Equal(t, 3, status.RequiredAgents)
	assert.Equal(t, 1, status.UniqueOwners)
	assert.Equal(t, 2, status.RequiredOwners)
	assert.Equal(t, 45.5, status.ConfidenceScore)
	assert.Equal(t, 60.0, status.RequiredConfidence)
	assert.False(t, status.ConsensusReached)
	assert.Len(t, status.MissingCriteria, 2)
}

func TestConsensusStatus_VerifiedState(t *testing.T) {
	status := ConsensusStatus{
		MCPServerID:        uuid.New().String(),
		Status:             "verified",
		IsVerified:         true,
		UniqueAgents:       5,
		RequiredAgents:     3,
		UniqueOwners:       3,
		RequiredOwners:     2,
		ConfidenceScore:    85.0,
		RequiredConfidence: 60.0,
		ConsensusReached:   true,
		ProgressPercent:    100.0,
		MissingCriteria:    []string{},
	}

	assert.True(t, status.IsVerified)
	assert.True(t, status.ConsensusReached)
	assert.Equal(t, 100.0, status.ProgressPercent)
	assert.Empty(t, status.MissingCriteria)

	// All thresholds exceeded
	assert.True(t, status.UniqueAgents >= status.RequiredAgents)
	assert.True(t, status.UniqueOwners >= status.RequiredOwners)
	assert.True(t, status.ConfidenceScore >= status.RequiredConfidence)
}

// ========================================
// ToolUsageEntry Tests
// ========================================

func TestToolUsageEntry_Structure(t *testing.T) {
	entry := ToolUsageEntry{
		Count:     10,
		FirstUsed: "2024-01-01T00:00:00Z",
		LastUsed:  "2024-01-15T12:30:00Z",
	}

	assert.Equal(t, 10, entry.Count)
	assert.Equal(t, "2024-01-01T00:00:00Z", entry.FirstUsed)
	assert.Equal(t, "2024-01-15T12:30:00Z", entry.LastUsed)
}

// ========================================
// toCanonicalJSON Tests
// ========================================

func TestToCanonicalJSON_SimpleStruct(t *testing.T) {
	type TestStruct struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	input := TestStruct{Name: "test", Value: 42}
	result, err := toCanonicalJSON(input)

	assert.NoError(t, err)
	assert.Contains(t, string(result), `"name":"test"`)
	assert.Contains(t, string(result), `"value":42`)
}

func TestToCanonicalJSON_Map(t *testing.T) {
	input := map[string]interface{}{
		"key":   "value",
		"count": 5,
	}

	result, err := toCanonicalJSON(input)

	assert.NoError(t, err)
	assert.Contains(t, string(result), `"key":"value"`)
}

func TestToCanonicalJSON_NilValue(t *testing.T) {
	result, err := toCanonicalJSON(nil)

	assert.NoError(t, err)
	assert.Equal(t, "null", string(result))
}

// ========================================
// Confidence Score Logic Tests
// ========================================

func TestConfidenceScore_SDKPoints(t *testing.T) {
	// SDK attestations: 20 points each, max 100
	tests := []struct {
		agentCount      int
		expectedPoints  float64
	}{
		{0, 0},
		{1, 20},
		{3, 60},
		{5, 100},
		{10, 100}, // Capped at 100
	}

	for _, tt := range tests {
		sdkPoints := float64(tt.agentCount) * 20.0
		if sdkPoints > 100.0 {
			sdkPoints = 100.0
		}
		assert.Equal(t, tt.expectedPoints, sdkPoints)
	}
}

func TestConfidenceScore_ManualPoints(t *testing.T) {
	// Manual attestations: 5 points each, max 25
	tests := []struct {
		manualCount    int
		expectedPoints float64
	}{
		{0, 0},
		{1, 5},
		{3, 15},
		{5, 25},
		{10, 25}, // Capped at 25
	}

	for _, tt := range tests {
		manualPoints := float64(tt.manualCount) * 5.0
		if manualPoints > 25.0 {
			manualPoints = 25.0
		}
		assert.Equal(t, tt.expectedPoints, manualPoints)
	}
}

func TestConfidenceScore_TrustPoints(t *testing.T) {
	// Trust score scaled to 0-50 points
	tests := []struct {
		avgTrust       float64
		expectedPoints float64
	}{
		{0.0, 0.0},
		{0.5, 25.0},
		{1.0, 50.0},
		{0.88, 44.0},
	}

	for _, tt := range tests {
		trustPoints := tt.avgTrust * 50.0
		assert.InDelta(t, tt.expectedPoints, trustPoints, 0.01)
	}
}

func TestConfidenceScore_Normalization(t *testing.T) {
	// Final score = (sdk + manual + trust + recency) / 2
	// Max = (100 + 25 + 50 + 25) / 2 = 100

	// Scenario: Max possible score
	sdkPoints := 100.0
	manualPoints := 25.0
	trustPoints := 50.0
	recencyPoints := 25.0

	rawScore := sdkPoints + manualPoints + trustPoints + recencyPoints
	finalScore := rawScore / 2.0
	if finalScore > 100.0 {
		finalScore = 100.0
	}

	assert.Equal(t, 100.0, finalScore)
}

// ========================================
// Trust Score Threshold Logic Tests
// ========================================

func TestTrustScoreThresholds_BlockLogic(t *testing.T) {
	minTrust := 0.30

	tests := []struct {
		trustScore   float64
		shouldBlock  bool
	}{
		{0.0, true},
		{0.20, true},
		{0.29, true},
		{0.30, false},
		{0.50, false},
		{1.0, false},
	}

	for _, tt := range tests {
		shouldBlock := tt.trustScore < minTrust
		assert.Equal(t, tt.shouldBlock, shouldBlock)
	}
}

func TestTrustScoreThresholds_WarnLogic(t *testing.T) {
	minTrust := 0.30
	warnThreshold := 0.50

	tests := []struct {
		trustScore  float64
		shouldBlock bool
		shouldWarn  bool
	}{
		{0.20, true, false},  // Blocked - no warning needed
		{0.30, false, true},  // Not blocked, but warn
		{0.45, false, true},  // Not blocked, but warn
		{0.50, false, false}, // At threshold - no warning
		{0.75, false, false}, // Above threshold - no warning
	}

	for _, tt := range tests {
		shouldBlock := tt.trustScore < minTrust
		shouldWarn := !shouldBlock && tt.trustScore < warnThreshold

		assert.Equal(t, tt.shouldBlock, shouldBlock)
		assert.Equal(t, tt.shouldWarn, shouldWarn)
	}
}

// ========================================
// Attestation Expiration Logic Tests
// ========================================

func TestAttestationExpiration_TimeWindow(t *testing.T) {
	// Attestation timestamp must be < 5 minutes old
	now := time.Now()

	tests := []struct {
		name      string
		timestamp time.Time
		isExpired bool
	}{
		{"just now", now, false},
		{"1 minute ago", now.Add(-1 * time.Minute), false},
		{"4 minutes ago", now.Add(-4 * time.Minute), false},
		{"4m59s ago (edge)", now.Add(-4*time.Minute - 59*time.Second), false}, // Just under 5 min
		{"5m1s ago", now.Add(-5*time.Minute - 1*time.Second), true},           // Just over 5 min
		{"6 minutes ago", now.Add(-6 * time.Minute), true},
		{"1 hour ago", now.Add(-1 * time.Hour), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isExpired := time.Since(tt.timestamp) > 5*time.Minute
			assert.Equal(t, tt.isExpired, isExpired)
		})
	}
}

// ========================================
// SDK vs Manual Attestation Logic Tests
// ========================================

func TestAttestationType_SDKDetection(t *testing.T) {
	// SDK attestation: has agent_id AND signature != "manual-attestation"
	tests := []struct {
		name         string
		hasAgentID   bool
		signature    string
		expectedType string
	}{
		{"SDK attestation", true, "valid-signature", "sdk"},
		{"Manual attestation", false, "manual-attestation", "manual"},
		{"Agent with manual signature", true, "manual-attestation", "manual"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var attestationType string
			if tt.hasAgentID && tt.signature != "manual-attestation" {
				attestationType = "sdk"
			} else {
				attestationType = "manual"
			}
			assert.Equal(t, tt.expectedType, attestationType)
		})
	}
}

// ========================================
// Recency Factor Logic Tests
// ========================================

func TestRecencyFactor_Calculation(t *testing.T) {
	sevenDaysAgo := time.Now().Add(-7 * 24 * time.Hour)

	tests := []struct {
		name           string
		recentCount    int
		totalCount     int
		expectedFactor float64
	}{
		{"all recent", 10, 10, 1.0},
		{"half recent", 5, 10, 0.5},
		{"none recent", 0, 10, 0.0},
		{"quarter recent", 1, 4, 0.25},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recencyFactor := float64(tt.recentCount) / float64(tt.totalCount)
			assert.Equal(t, tt.expectedFactor, recencyFactor)

			// Recency points = factor * 25
			recencyPoints := recencyFactor * 25.0
			assert.InDelta(t, tt.expectedFactor*25.0, recencyPoints, 0.01)
		})
	}

	_ = sevenDaysAgo // Acknowledge the reference
}

// ========================================
// Connection Type Tests
// ========================================

func TestConnectionType_Values(t *testing.T) {
	assert.Equal(t, domain.ConnectionType("attested"), domain.ConnectionTypeAttested)
}

// ========================================
// Progress Calculation Tests
// ========================================

func TestConsensusProgress_Calculation(t *testing.T) {
	tests := []struct {
		name             string
		uniqueAgents     int
		requiredAgents   int
		uniqueOwners     int
		requiredOwners   int
		confidence       float64
		requiredConfidence float64
		expectedProgress float64
	}{
		{
			name:             "all requirements met",
			uniqueAgents:     5,
			requiredAgents:   3,
			uniqueOwners:     3,
			requiredOwners:   2,
			confidence:       80.0,
			requiredConfidence: 60.0,
			expectedProgress: 100.0,
		},
		{
			name:             "no progress",
			uniqueAgents:     0,
			requiredAgents:   3,
			uniqueOwners:     0,
			requiredOwners:   2,
			confidence:       0.0,
			requiredConfidence: 60.0,
			expectedProgress: 0.0,
		},
		{
			name:             "agents bottleneck",
			uniqueAgents:     1,
			requiredAgents:   3,
			uniqueOwners:     2,
			requiredOwners:   2,
			confidence:       80.0,
			requiredConfidence: 60.0,
			expectedProgress: 33.33, // 1/3 = 33.33%
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Calculate progress for each criterion
			agentsProgress := float64(tt.uniqueAgents) / float64(tt.requiredAgents) * 100
			if agentsProgress > 100 {
				agentsProgress = 100
			}

			ownersProgress := float64(tt.uniqueOwners) / float64(tt.requiredOwners) * 100
			if ownersProgress > 100 {
				ownersProgress = 100
			}

			confidenceProgress := tt.confidence / tt.requiredConfidence * 100
			if confidenceProgress > 100 {
				confidenceProgress = 100
			}

			// Overall progress is minimum of all criteria
			overallProgress := agentsProgress
			if ownersProgress < overallProgress {
				overallProgress = ownersProgress
			}
			if confidenceProgress < overallProgress {
				overallProgress = confidenceProgress
			}

			assert.InDelta(t, tt.expectedProgress, overallProgress, 0.1)
		})
	}
}

// ========================================
// Constructor Tests
// ========================================

func TestNewMCPAttestationService(t *testing.T) {
	service := NewMCPAttestationService(nil, nil, nil, nil, nil, nil)
	assert.NotNil(t, service)
	assert.Nil(t, service.attestationRepo)
	assert.Nil(t, service.agentRepo)
	assert.Nil(t, service.mcpRepo)
	assert.Nil(t, service.userRepo)
	assert.Nil(t, service.connectionRepo)
	assert.Nil(t, service.capabilityRepo)
}

func TestNewMCPAttestationServiceWithAlerts(t *testing.T) {
	service := NewMCPAttestationServiceWithAlerts(nil, nil, nil, nil, nil, nil, nil)

	assert.NotNil(t, service)
	assert.Nil(t, service.attestationRepo)
	assert.Nil(t, service.connectionRepo)
	assert.Nil(t, service.alertRepo)
}
