package application

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
)

// ========================================
// Factor Weights Tests
// ========================================

func TestMCPFactorWeights_SumToOne(t *testing.T) {
	// Verify all factor weights sum to 1.0 (100%)
	var total float64
	for _, weight := range mcpFactorWeights {
		total += weight
	}
	assert.InDelta(t, 1.0, total, 0.001, "All factor weights should sum to 1.0")
}

func TestMCPFactorWeights_IndividualValues(t *testing.T) {
	tests := []struct {
		name     string
		factor   string
		expected float64
	}{
		{"attestation_consensus weight", "attestation_consensus", 0.25},
		{"connection_health weight", "connection_health", 0.15},
		{"capability_stability weight", "capability_stability", 0.15},
		{"security_posture weight", "security_posture", 0.15},
		{"organization_compliance weight", "organization_compliance", 0.10},
		{"age weight", "age", 0.10},
		{"usage_patterns weight", "usage_patterns", 0.05},
		{"user_feedback weight", "user_feedback", 0.05},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			weight, exists := mcpFactorWeights[tt.factor]
			assert.True(t, exists, "Factor %s should exist", tt.factor)
			assert.Equal(t, tt.expected, weight)
		})
	}
}

func TestMCPFactorWeights_AllFactorsExist(t *testing.T) {
	expectedFactors := []string{
		"attestation_consensus",
		"connection_health",
		"capability_stability",
		"security_posture",
		"organization_compliance",
		"age",
		"usage_patterns",
		"user_feedback",
	}

	assert.Len(t, mcpFactorWeights, 8, "Should have 8 factors")

	for _, factor := range expectedFactors {
		_, exists := mcpFactorWeights[factor]
		assert.True(t, exists, "Factor %s should exist", factor)
	}
}

func TestMCPFactorWeights_AllPositive(t *testing.T) {
	for factor, weight := range mcpFactorWeights {
		assert.Greater(t, weight, 0.0, "Factor %s should have positive weight", factor)
	}
}

// ========================================
// Constructor Tests
// ========================================

func TestNewMCPTrustCalculator(t *testing.T) {
	calculator := NewMCPTrustCalculator(nil, nil, nil, nil)

	assert.NotNil(t, calculator)
	assert.Nil(t, calculator.mcpServerRepo)
	assert.Nil(t, calculator.attestationRepo)
	assert.Nil(t, calculator.capabilityRepo)
	assert.Nil(t, calculator.alertRepo)
}

func TestNewMCPTrustCalculatorWithRepo(t *testing.T) {
	calculator := NewMCPTrustCalculatorWithRepo(nil, nil, nil, nil, nil)

	assert.NotNil(t, calculator)
	assert.Nil(t, calculator.mcpServerRepo)
	assert.Nil(t, calculator.attestationRepo)
	assert.Nil(t, calculator.capabilityRepo)
	assert.Nil(t, calculator.alertRepo)
	assert.Nil(t, calculator.mcpTrustScoreRepo)
}

// ========================================
// Calculate Factor Logic Tests
// ========================================

func TestMCPTrustCalculator_CalculateAge_NewServer(t *testing.T) {
	calculator := &MCPTrustCalculator{}
	now := time.Now()

	server := &domain.MCPServer{
		ID:        uuid.New(),
		CreatedAt: now.Add(-time.Hour), // 1 hour old
	}

	age := calculator.calculateAge(server)

	// New servers should have lower age score (less than 1.0)
	assert.True(t, age >= 0.0 && age <= 1.0, "Age should be between 0 and 1")
}

func TestMCPTrustCalculator_CalculateAge_OldServer(t *testing.T) {
	calculator := &MCPTrustCalculator{}
	now := time.Now()

	server := &domain.MCPServer{
		ID:        uuid.New(),
		CreatedAt: now.AddDate(-1, 0, 0), // 1 year old
	}

	age := calculator.calculateAge(server)

	// Old servers should have higher age score (closer to 1.0)
	assert.True(t, age >= 0.5, "Old servers should have higher age score")
	assert.True(t, age <= 1.0, "Age should not exceed 1.0")
}

func TestMCPTrustCalculator_CalculateSecurityPosture_VerifiedWithPublicKey(t *testing.T) {
	calculator := &MCPTrustCalculator{}

	server := &domain.MCPServer{
		ID:         uuid.New(),
		Status:     domain.MCPServerStatusVerified,
		PublicKey:  "test-public-key",
		IsVerified: true,
	}

	security := calculator.calculateSecurityPosture(server)

	// Verified server with public key should have high security score
	assert.True(t, security >= 0.5, "Verified server with key should have good security score")
}

func TestMCPTrustCalculator_CalculateSecurityPosture_NoPublicKey(t *testing.T) {
	calculator := &MCPTrustCalculator{}

	server := &domain.MCPServer{
		ID:         uuid.New(),
		Status:     domain.MCPServerStatusPending,
		PublicKey:  "",
		IsVerified: false,
	}

	security := calculator.calculateSecurityPosture(server)

	// Server without public key should have lower security score
	assert.True(t, security >= 0.0 && security <= 1.0, "Security score should be valid")
}

func TestMCPTrustCalculator_CalculateConnectionHealth_NoRepo(t *testing.T) {
	calculator := &MCPTrustCalculator{}

	// Verified server
	server := &domain.MCPServer{
		ID:         uuid.New(),
		IsVerified: true,
	}

	health := calculator.calculateConnectionHealth(server)
	assert.Equal(t, 0.95, health, "Verified server without repo should return 0.95")

	// Unverified server
	server.IsVerified = false
	health = calculator.calculateConnectionHealth(server)
	assert.Equal(t, 0.70, health, "Unverified server without repo should return 0.70")
}

func TestMCPTrustCalculator_CalculateAttestationConsensus_StatusModifiers(t *testing.T) {
	calculator := &MCPTrustCalculator{}

	tests := []struct {
		name           string
		status         domain.MCPServerStatus
		attestations   int
		confidence     float64
		expectedRange  [2]float64 // min, max
	}{
		{
			name:          "verified with attestations",
			status:        domain.MCPServerStatusVerified,
			attestations:  5,
			confidence:    75.0,
			expectedRange: [2]float64{0.5, 1.0},
		},
		{
			name:          "pending reduces score",
			status:        domain.MCPServerStatusPending,
			attestations:  3,
			confidence:    60.0,
			expectedRange: [2]float64{0.0, 0.6}, // Score reduced by 0.5
		},
		{
			name:          "suspended heavily penalized",
			status:        domain.MCPServerStatusSuspended,
			attestations:  5,
			confidence:    80.0,
			expectedRange: [2]float64{0.0, 0.3}, // Score reduced by 0.2
		},
		{
			name:          "revoked gets zero",
			status:        domain.MCPServerStatusRevoked,
			attestations:  10,
			confidence:    100.0,
			expectedRange: [2]float64{0.0, 0.0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := &domain.MCPServer{
				ID:               uuid.New(),
				Status:           tt.status,
				AttestationCount: tt.attestations,
				ConfidenceScore:  tt.confidence,
			}

			score := calculator.calculateAttestationConsensus(server)
			assert.GreaterOrEqual(t, score, tt.expectedRange[0])
			assert.LessOrEqual(t, score, tt.expectedRange[1])
		})
	}
}

func TestMCPTrustCalculator_CalculateUsagePatterns_NoRepo(t *testing.T) {
	calculator := &MCPTrustCalculator{}

	server := &domain.MCPServer{
		ID:         uuid.New(),
		IsVerified: true,
	}

	patterns := calculator.calculateUsagePatterns(server)

	// Without attestation repo, should return baseline
	assert.True(t, patterns >= 0.0 && patterns <= 1.0, "Usage patterns should be valid")
}

func TestMCPTrustCalculator_CalculateUserFeedback_Default(t *testing.T) {
	calculator := &MCPTrustCalculator{}

	server := &domain.MCPServer{
		ID: uuid.New(),
	}

	feedback := calculator.calculateUserFeedback(server)

	// Without explicit feedback, should return neutral/baseline
	assert.True(t, feedback >= 0.0 && feedback <= 1.0, "User feedback should be valid")
}

func TestMCPTrustCalculator_CalculateOrganizationCompliance_NoAlerts(t *testing.T) {
	calculator := &MCPTrustCalculator{}

	// Server with all compliance indicators
	server := &domain.MCPServer{
		ID:          uuid.New(),
		Status:      domain.MCPServerStatusVerified,
		CreatedBy:   uuid.New(),
		Description: "Test server with proper documentation",
		IsVerified:  true,
	}

	compliance := calculator.calculateOrganizationCompliance(server)

	// Server with all compliance factors should have high score
	assert.True(t, compliance >= 0.9, "Fully compliant server should have high compliance score")
	assert.True(t, compliance <= 1.0, "Compliance should not exceed 1.0")
}

func TestMCPTrustCalculator_CalculateOrganizationCompliance_MinimalCompliance(t *testing.T) {
	calculator := &MCPTrustCalculator{}

	// Server with minimal compliance (no creator, no description)
	server := &domain.MCPServer{
		ID:     uuid.New(),
		Status: domain.MCPServerStatusPending,
	}

	compliance := calculator.calculateOrganizationCompliance(server)

	// Server with just valid status should have some compliance
	assert.True(t, compliance > 0.0, "Server with valid status should have some compliance")
	assert.True(t, compliance < 1.0, "Server without all factors shouldn't have full compliance")
}

func TestMCPTrustCalculator_CalculateCapabilityStability_NoRepo(t *testing.T) {
	calculator := &MCPTrustCalculator{}

	server := &domain.MCPServer{
		ID:         uuid.New(),
		IsVerified: true,
	}

	stability := calculator.calculateCapabilityStability(server)

	// Without capability repo, should return baseline
	assert.True(t, stability >= 0.0 && stability <= 1.0, "Stability should be valid")
}

// ========================================
// CalculateFactors Tests
// ========================================

func TestMCPTrustCalculator_CalculateFactors_ReturnsAllFactors(t *testing.T) {
	calculator := &MCPTrustCalculator{}

	server := &domain.MCPServer{
		ID:         uuid.New(),
		Status:     domain.MCPServerStatusVerified,
		IsVerified: true,
		CreatedAt:  time.Now().AddDate(0, -6, 0), // 6 months old
	}

	factors, err := calculator.CalculateFactors(server)

	assert.NoError(t, err)
	assert.NotNil(t, factors)

	// All factors should be in valid range
	assert.True(t, factors.AttestationConsensus >= 0 && factors.AttestationConsensus <= 1)
	assert.True(t, factors.ConnectionHealth >= 0 && factors.ConnectionHealth <= 1)
	assert.True(t, factors.CapabilityStability >= 0 && factors.CapabilityStability <= 1)
	assert.True(t, factors.SecurityPosture >= 0 && factors.SecurityPosture <= 1)
	assert.True(t, factors.OrganizationCompliance >= 0 && factors.OrganizationCompliance <= 1)
	assert.True(t, factors.Age >= 0 && factors.Age <= 1)
	assert.True(t, factors.UsagePatterns >= 0 && factors.UsagePatterns <= 1)
	assert.True(t, factors.UserFeedback >= 0 && factors.UserFeedback <= 1)
}

// ========================================
// Score Calculation Logic Tests
// ========================================

func TestMCPTrustScore_WeightedAverageCalculation(t *testing.T) {
	// Test that weighted average is calculated correctly
	factors := domain.MCPTrustScoreFactors{
		AttestationConsensus:   1.0, // 25%
		ConnectionHealth:       1.0, // 15%
		CapabilityStability:    1.0, // 15%
		SecurityPosture:        1.0, // 15%
		OrganizationCompliance: 1.0, // 10%
		Age:                    1.0, // 10%
		UsagePatterns:          1.0, // 5%
		UserFeedback:           1.0, // 5%
	}

	// All 1.0 factors should result in 1.0 score
	score := factors.AttestationConsensus*mcpFactorWeights["attestation_consensus"] +
		factors.ConnectionHealth*mcpFactorWeights["connection_health"] +
		factors.CapabilityStability*mcpFactorWeights["capability_stability"] +
		factors.SecurityPosture*mcpFactorWeights["security_posture"] +
		factors.OrganizationCompliance*mcpFactorWeights["organization_compliance"] +
		factors.Age*mcpFactorWeights["age"] +
		factors.UsagePatterns*mcpFactorWeights["usage_patterns"] +
		factors.UserFeedback*mcpFactorWeights["user_feedback"]

	assert.InDelta(t, 1.0, score, 0.001)
}

func TestMCPTrustScore_WeightedAverageCalculation_AllZero(t *testing.T) {
	// Test that all zero factors result in zero score
	factors := domain.MCPTrustScoreFactors{
		AttestationConsensus:   0.0,
		ConnectionHealth:       0.0,
		CapabilityStability:    0.0,
		SecurityPosture:        0.0,
		OrganizationCompliance: 0.0,
		Age:                    0.0,
		UsagePatterns:          0.0,
		UserFeedback:           0.0,
	}

	score := factors.AttestationConsensus*mcpFactorWeights["attestation_consensus"] +
		factors.ConnectionHealth*mcpFactorWeights["connection_health"] +
		factors.CapabilityStability*mcpFactorWeights["capability_stability"] +
		factors.SecurityPosture*mcpFactorWeights["security_posture"] +
		factors.OrganizationCompliance*mcpFactorWeights["organization_compliance"] +
		factors.Age*mcpFactorWeights["age"] +
		factors.UsagePatterns*mcpFactorWeights["usage_patterns"] +
		factors.UserFeedback*mcpFactorWeights["user_feedback"]

	assert.Equal(t, 0.0, score)
}

func TestMCPTrustScore_WeightedAverageCalculation_MixedFactors(t *testing.T) {
	// Test with mixed factor values
	factors := domain.MCPTrustScoreFactors{
		AttestationConsensus:   0.8,  // 0.8 * 0.25 = 0.20
		ConnectionHealth:       0.9,  // 0.9 * 0.15 = 0.135
		CapabilityStability:    1.0,  // 1.0 * 0.15 = 0.15
		SecurityPosture:        0.7,  // 0.7 * 0.15 = 0.105
		OrganizationCompliance: 0.85, // 0.85 * 0.10 = 0.085
		Age:                    0.5,  // 0.5 * 0.10 = 0.05
		UsagePatterns:          0.6,  // 0.6 * 0.05 = 0.03
		UserFeedback:           0.75, // 0.75 * 0.05 = 0.0375
	}

	expected := 0.20 + 0.135 + 0.15 + 0.105 + 0.085 + 0.05 + 0.03 + 0.0375

	score := factors.AttestationConsensus*mcpFactorWeights["attestation_consensus"] +
		factors.ConnectionHealth*mcpFactorWeights["connection_health"] +
		factors.CapabilityStability*mcpFactorWeights["capability_stability"] +
		factors.SecurityPosture*mcpFactorWeights["security_posture"] +
		factors.OrganizationCompliance*mcpFactorWeights["organization_compliance"] +
		factors.Age*mcpFactorWeights["age"] +
		factors.UsagePatterns*mcpFactorWeights["usage_patterns"] +
		factors.UserFeedback*mcpFactorWeights["user_feedback"]

	assert.InDelta(t, expected, score, 0.001)
}

// ========================================
// Calculate Confidence Tests
// ========================================

func TestMCPTrustCalculator_CalculateConfidence_FullData(t *testing.T) {
	calculator := &MCPTrustCalculator{}

	server := &domain.MCPServer{
		ID:                   uuid.New(),
		Status:               domain.MCPServerStatusVerified,
		PublicKey:            "test-key",
		URL:                  "https://example.com/mcp",
		AttestationCount:     5,
		Capabilities:         []string{"tools"},
		CreatedAt:            time.Now().AddDate(0, -1, 0), // 1 month old
		ConnectedAgentsCount: 3,
	}

	factors := &domain.MCPTrustScoreFactors{}
	confidence := calculator.calculateConfidence(server, factors)

	// Server with lots of data should have high confidence
	assert.True(t, confidence >= 0.8, "Well-documented server should have high confidence")
	assert.True(t, confidence <= 1.0, "Confidence should not exceed 1.0")
}

func TestMCPTrustCalculator_CalculateConfidence_MinimalData(t *testing.T) {
	calculator := &MCPTrustCalculator{}

	server := &domain.MCPServer{
		ID:        uuid.New(),
		CreatedAt: time.Now(), // Just created
	}

	factors := &domain.MCPTrustScoreFactors{}
	confidence := calculator.calculateConfidence(server, factors)

	// Server with minimal data should have low confidence
	assert.True(t, confidence >= 0.0, "Confidence should be positive")
	assert.True(t, confidence <= 0.5, "New server with minimal data should have low confidence")
}

func TestMCPTrustCalculator_CalculateConfidence_ManyAttestations(t *testing.T) {
	calculator := &MCPTrustCalculator{}

	server := &domain.MCPServer{
		ID:               uuid.New(),
		Status:           domain.MCPServerStatusVerified,
		AttestationCount: 10, // Many attestations
		CreatedAt:        time.Now().AddDate(0, -3, 0),
	}

	factors := &domain.MCPTrustScoreFactors{}
	confidence := calculator.calculateConfidence(server, factors)

	// Many attestations should boost confidence
	assert.True(t, confidence > 0.3, "Many attestations should provide reasonable confidence")
}

// ========================================
// Calculate (End-to-End) Tests
// ========================================

func TestMCPTrustCalculator_Calculate_ReturnsValidScore(t *testing.T) {
	calculator := &MCPTrustCalculator{}

	server := &domain.MCPServer{
		ID:               uuid.New(),
		Status:           domain.MCPServerStatusVerified,
		PublicKey:        "test-public-key",
		URL:              "https://example.com/mcp",
		AttestationCount: 5,
		ConfidenceScore:  75.0,
		CreatedAt:        time.Now().AddDate(0, -6, 0), // 6 months old
		IsVerified:       true,
		CreatedBy:        uuid.New(),
		Description:      "Test MCP server",
	}

	score, err := calculator.Calculate(server)

	assert.NoError(t, err)
	assert.NotNil(t, score)
	assert.Equal(t, server.ID, score.MCPServerID)
	assert.True(t, score.Score >= 0.0 && score.Score <= 1.0, "Score should be in valid range")
	assert.True(t, score.Confidence >= 0.0 && score.Confidence <= 1.0, "Confidence should be in valid range")
	assert.NotZero(t, score.ID)
	assert.NotZero(t, score.LastCalculated)
}

func TestMCPTrustCalculator_Calculate_RevokedServerLowScore(t *testing.T) {
	calculator := &MCPTrustCalculator{}

	server := &domain.MCPServer{
		ID:         uuid.New(),
		Status:     domain.MCPServerStatusRevoked,
		CreatedAt:  time.Now().AddDate(-1, 0, 0),
		IsVerified: false,
	}

	score, err := calculator.Calculate(server)

	assert.NoError(t, err)
	assert.NotNil(t, score)
	// Revoked servers should have low scores
	assert.True(t, score.Score < 0.5, "Revoked server should have low trust score")
}

func TestMCPTrustCalculator_Calculate_NewPendingServer(t *testing.T) {
	calculator := &MCPTrustCalculator{}

	server := &domain.MCPServer{
		ID:        uuid.New(),
		Status:    domain.MCPServerStatusPending,
		CreatedAt: time.Now().Add(-time.Hour),
	}

	score, err := calculator.Calculate(server)

	assert.NoError(t, err)
	assert.NotNil(t, score)
	// New pending servers have lower but valid scores
	assert.True(t, score.Score >= 0.0, "Score should be non-negative")
	assert.True(t, score.Score <= 1.0, "Score should not exceed 1.0")
}

func TestMCPTrustCalculator_Calculate_HighTrustServer(t *testing.T) {
	calculator := &MCPTrustCalculator{}

	server := &domain.MCPServer{
		ID:                   uuid.New(),
		Status:               domain.MCPServerStatusVerified,
		PublicKey:            "ed25519-public-key-here",
		URL:                  "https://trusted.company.com/mcp",
		AttestationCount:     20,
		ConfidenceScore:      95.0,
		CreatedAt:            time.Now().AddDate(-2, 0, 0), // 2 years old
		IsVerified:           true,
		CreatedBy:            uuid.New(),
		Description:          "Fully documented production MCP server",
		Capabilities:         []string{"tools", "prompts", "resources"},
		ConnectedAgentsCount: 10,
	}

	score, err := calculator.Calculate(server)

	assert.NoError(t, err)
	assert.NotNil(t, score)
	// Well-established verified server should have high trust
	assert.True(t, score.Score >= 0.7, "High-trust server should have score >= 0.7")
	assert.True(t, score.Confidence >= 0.7, "Well-documented server should have high confidence")
}

// ========================================
// CalculateUserFeedback Tests
// ========================================

func TestMCPTrustCalculator_CalculateUserFeedback_ReturnsBaseline(t *testing.T) {
	calculator := &MCPTrustCalculator{}

	server := &domain.MCPServer{ID: uuid.New()}
	feedback := calculator.calculateUserFeedback(server)

	// MVP returns baseline of 0.75
	assert.Equal(t, 0.75, feedback, "User feedback should return baseline value")
}
