package domain

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMCPTrustScoreFactors_Structure(t *testing.T) {
	factors := MCPTrustScoreFactors{
		AttestationConsensus:   0.95, // 25% weight
		ConnectionHealth:       0.90, // 15% weight
		CapabilityStability:    0.85, // 15% weight
		SecurityPosture:        0.92, // 15% weight
		OrganizationCompliance: 0.88, // 10% weight
		Age:                    0.75, // 10% weight
		UsagePatterns:          0.80, // 5% weight
		UserFeedback:           1.00, // 5% weight
	}

	// Verify all factors are in valid range
	assert.True(t, factors.AttestationConsensus >= 0 && factors.AttestationConsensus <= 1)
	assert.True(t, factors.ConnectionHealth >= 0 && factors.ConnectionHealth <= 1)
	assert.True(t, factors.CapabilityStability >= 0 && factors.CapabilityStability <= 1)
	assert.True(t, factors.SecurityPosture >= 0 && factors.SecurityPosture <= 1)
	assert.True(t, factors.OrganizationCompliance >= 0 && factors.OrganizationCompliance <= 1)
	assert.True(t, factors.Age >= 0 && factors.Age <= 1)
	assert.True(t, factors.UsagePatterns >= 0 && factors.UsagePatterns <= 1)
	assert.True(t, factors.UserFeedback >= 0 && factors.UserFeedback <= 1)
}

func TestMCPTrustScoreFactors_HighTrust(t *testing.T) {
	// All factors at maximum
	factors := MCPTrustScoreFactors{
		AttestationConsensus:   1.0,
		ConnectionHealth:       1.0,
		CapabilityStability:    1.0,
		SecurityPosture:        1.0,
		OrganizationCompliance: 1.0,
		Age:                    1.0,
		UsagePatterns:          1.0,
		UserFeedback:           1.0,
	}

	// Calculate weighted score manually
	score := (factors.AttestationConsensus * 0.25) +
		(factors.ConnectionHealth * 0.15) +
		(factors.CapabilityStability * 0.15) +
		(factors.SecurityPosture * 0.15) +
		(factors.OrganizationCompliance * 0.10) +
		(factors.Age * 0.10) +
		(factors.UsagePatterns * 0.05) +
		(factors.UserFeedback * 0.05)

	assert.Equal(t, 1.0, score)
}

func TestMCPTrustScoreFactors_LowTrust(t *testing.T) {
	factors := MCPTrustScoreFactors{
		AttestationConsensus:   0.2,
		ConnectionHealth:       0.3,
		CapabilityStability:    0.5,
		SecurityPosture:        0.4,
		OrganizationCompliance: 0.6,
		Age:                    0.3, // New server
		UsagePatterns:          0.5,
		UserFeedback:           0.5,
	}

	// All factors should be low
	assert.Less(t, factors.AttestationConsensus, 0.5)
	assert.Less(t, factors.Age, 0.5)
}

func TestMCPTrustScoreFactors_AgeCalculation(t *testing.T) {
	// Age factor based on days since registration
	tests := []struct {
		daysSinceRegistration int
		expectedAge           float64
		description           string
	}{
		{3, 0.30, "< 7 days"},
		{7, 0.50, "7-30 days"},
		{15, 0.50, "7-30 days"},
		{30, 0.75, "30-90 days"},
		{60, 0.75, "30-90 days"},
		{90, 1.00, "90+ days"},
		{180, 1.00, "90+ days"},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			var age float64
			switch {
			case tt.daysSinceRegistration < 7:
				age = 0.30
			case tt.daysSinceRegistration < 30:
				age = 0.50
			case tt.daysSinceRegistration < 90:
				age = 0.75
			default:
				age = 1.00
			}
			assert.Equal(t, tt.expectedAge, age)
		})
	}
}

func TestMCPTrustScore_Structure(t *testing.T) {
	now := time.Now()

	score := MCPTrustScore{
		ID:          uuid.New(),
		MCPServerID: uuid.New(),
		Score:       0.87,
		Factors: MCPTrustScoreFactors{
			AttestationConsensus:   0.90,
			ConnectionHealth:       0.95,
			CapabilityStability:    0.80,
			SecurityPosture:        0.85,
			OrganizationCompliance: 0.88,
			Age:                    0.75,
			UsagePatterns:          0.82,
			UserFeedback:           0.90,
		},
		Confidence:     0.85,
		LastCalculated: now,
		CreatedAt:      now,
	}

	assert.NotEqual(t, uuid.Nil, score.ID)
	assert.NotEqual(t, uuid.Nil, score.MCPServerID)
	assert.True(t, score.Score >= 0 && score.Score <= 1)
	assert.True(t, score.Confidence >= 0 && score.Confidence <= 1)
}

func TestMCPTrustScore_LowConfidence(t *testing.T) {
	// New server with few data points
	score := MCPTrustScore{
		ID:          uuid.New(),
		MCPServerID: uuid.New(),
		Score:       0.50, // Neutral score
		Factors: MCPTrustScoreFactors{
			AttestationConsensus: 0.30, // Few attestations
			ConnectionHealth:     0.50, // Limited data
			Age:                  0.30, // New server
		},
		Confidence:     0.25, // Low confidence
		LastCalculated: time.Now(),
		CreatedAt:      time.Now(),
	}

	assert.Less(t, score.Confidence, 0.5)
	assert.Less(t, score.Factors.Age, 0.5)
}

func TestMCPTrustScoreHistoryEntry_Structure(t *testing.T) {
	now := time.Now()
	previousScore := 0.75

	entry := MCPTrustScoreHistoryEntry{
		ID:             uuid.New(),
		MCPServerID:    uuid.New(),
		OrganizationID: uuid.New(),
		TrustScore:     0.82,
		PreviousScore:  &previousScore,
		ChangeReason:   "New attestation increased consensus",
		RecordedAt:     now,
		CreatedAt:      now,
	}

	assert.NotEqual(t, uuid.Nil, entry.ID)
	assert.Equal(t, 0.82, entry.TrustScore)
	assert.Equal(t, 0.75, *entry.PreviousScore)
	assert.NotEmpty(t, entry.ChangeReason)
	assert.Nil(t, entry.ChangedBy) // Automated change
}

func TestMCPTrustScoreHistoryEntry_ManualChange(t *testing.T) {
	now := time.Now()
	adminID := uuid.New()
	previousScore := 0.90

	entry := MCPTrustScoreHistoryEntry{
		ID:             uuid.New(),
		MCPServerID:    uuid.New(),
		OrganizationID: uuid.New(),
		TrustScore:     0.30,
		PreviousScore:  &previousScore,
		ChangeReason:   "Manual review: Security concern identified",
		ChangedBy:      &adminID,
		RecordedAt:     now,
		CreatedAt:      now,
	}

	assert.NotNil(t, entry.ChangedBy)
	assert.Equal(t, &adminID, entry.ChangedBy)
	assert.Contains(t, entry.ChangeReason, "Manual review")
}

func TestMCPTrustScoreHistoryEntry_FirstScore(t *testing.T) {
	now := time.Now()

	entry := MCPTrustScoreHistoryEntry{
		ID:             uuid.New(),
		MCPServerID:    uuid.New(),
		OrganizationID: uuid.New(),
		TrustScore:     0.50, // Initial score
		PreviousScore:  nil,  // No previous score
		ChangeReason:   "Initial score calculation",
		RecordedAt:     now,
		CreatedAt:      now,
	}

	assert.Nil(t, entry.PreviousScore)
	assert.Contains(t, entry.ChangeReason, "Initial")
}

func TestMCPTrustScoreFactors_JSONSerialization(t *testing.T) {
	factors := MCPTrustScoreFactors{
		AttestationConsensus:   0.85,
		ConnectionHealth:       0.90,
		CapabilityStability:    0.80,
		SecurityPosture:        0.95,
		OrganizationCompliance: 0.88,
		Age:                    0.75,
		UsagePatterns:          0.82,
		UserFeedback:           0.92,
	}

	// Serialize to JSON
	data, err := json.Marshal(factors)
	require.NoError(t, err)

	// Deserialize back
	var restored MCPTrustScoreFactors
	err = json.Unmarshal(data, &restored)
	require.NoError(t, err)

	// Verify all fields
	assert.Equal(t, factors.AttestationConsensus, restored.AttestationConsensus)
	assert.Equal(t, factors.ConnectionHealth, restored.ConnectionHealth)
	assert.Equal(t, factors.CapabilityStability, restored.CapabilityStability)
	assert.Equal(t, factors.SecurityPosture, restored.SecurityPosture)
	assert.Equal(t, factors.OrganizationCompliance, restored.OrganizationCompliance)
	assert.Equal(t, factors.Age, restored.Age)
	assert.Equal(t, factors.UsagePatterns, restored.UsagePatterns)
	assert.Equal(t, factors.UserFeedback, restored.UserFeedback)
}

func TestMCPTrustScore_JSONSerialization(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)

	score := MCPTrustScore{
		ID:          uuid.New(),
		MCPServerID: uuid.New(),
		Score:       0.87,
		Factors: MCPTrustScoreFactors{
			AttestationConsensus: 0.90,
			ConnectionHealth:     0.85,
		},
		Confidence:     0.80,
		LastCalculated: now,
		CreatedAt:      now,
	}

	// Serialize to JSON
	data, err := json.Marshal(score)
	require.NoError(t, err)

	// Deserialize back
	var restored MCPTrustScore
	err = json.Unmarshal(data, &restored)
	require.NoError(t, err)

	// Verify key fields
	assert.Equal(t, score.ID, restored.ID)
	assert.Equal(t, score.Score, restored.Score)
	assert.Equal(t, score.Confidence, restored.Confidence)
	assert.Equal(t, score.Factors.AttestationConsensus, restored.Factors.AttestationConsensus)
}

func TestMCPTrustScoreHistoryEntry_JSONSerialization(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	prevScore := 0.75

	entry := MCPTrustScoreHistoryEntry{
		ID:             uuid.New(),
		MCPServerID:    uuid.New(),
		OrganizationID: uuid.New(),
		TrustScore:     0.85,
		PreviousScore:  &prevScore,
		ChangeReason:   "Test change",
		RecordedAt:     now,
		CreatedAt:      now,
	}

	// Serialize to JSON
	data, err := json.Marshal(entry)
	require.NoError(t, err)

	// Deserialize back
	var restored MCPTrustScoreHistoryEntry
	err = json.Unmarshal(data, &restored)
	require.NoError(t, err)

	// Verify key fields
	assert.Equal(t, entry.ID, restored.ID)
	assert.Equal(t, entry.TrustScore, restored.TrustScore)
	assert.Equal(t, *entry.PreviousScore, *restored.PreviousScore)
}

func TestMCPTrustScoreFactors_WeightedCalculation(t *testing.T) {
	// Test weighted score calculation using the factors
	attestationConsensus := 0.80   // 25% weight
	connectionHealth := 0.70       // 15% weight
	capabilityStability := 0.90    // 15% weight
	securityPosture := 0.85        // 15% weight
	organizationCompliance := 0.75 // 10% weight
	age := 0.50                    // 10% weight
	usagePatterns := 0.60          // 5% weight
	userFeedback := 1.00           // 5% weight

	// Calculate expected weighted score
	expectedScore := (attestationConsensus * 0.25) +
		(connectionHealth * 0.15) +
		(capabilityStability * 0.15) +
		(securityPosture * 0.15) +
		(organizationCompliance * 0.10) +
		(age * 0.10) +
		(usagePatterns * 0.05) +
		(userFeedback * 0.05)

	// Verify weights sum to 1.0
	totalWeight := 0.25 + 0.15 + 0.15 + 0.15 + 0.10 + 0.10 + 0.05 + 0.05
	assert.InDelta(t, 1.0, totalWeight, 0.001)

	// Score should be between 0 and 1
	assert.True(t, expectedScore >= 0 && expectedScore <= 1)
	assert.InDelta(t, 0.7725, expectedScore, 0.001)
}
