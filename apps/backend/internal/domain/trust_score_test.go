package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestTrustScoreFactorsStruct(t *testing.T) {
	factors := TrustScoreFactors{
		VerificationStatus: 1.0,  // 25% weight
		Uptime:             0.99, // 15% weight
		SuccessRate:        0.95, // 15% weight
		SecurityAlerts:     0.8,  // 15% weight
		Compliance:         1.0,  // 10% weight
		Age:                0.5,  // 10% weight
		DriftDetection:     0.9,  // 5% weight
		UserFeedback:       0.85, // 5% weight
	}

	assert.Equal(t, 1.0, factors.VerificationStatus)
	assert.Equal(t, 0.99, factors.Uptime)
	assert.Equal(t, 0.95, factors.SuccessRate)
	assert.Equal(t, 0.8, factors.SecurityAlerts)
	assert.Equal(t, 1.0, factors.Compliance)
	assert.Equal(t, 0.5, factors.Age)
	assert.Equal(t, 0.9, factors.DriftDetection)
	assert.Equal(t, 0.85, factors.UserFeedback)
}

func TestTrustScoreFactorsWeightedCalculation(t *testing.T) {
	// Test weighted average calculation manually
	factors := TrustScoreFactors{
		VerificationStatus: 1.0,
		Uptime:             1.0,
		SuccessRate:        1.0,
		SecurityAlerts:     1.0,
		Compliance:         1.0,
		Age:                1.0,
		DriftDetection:     1.0,
		UserFeedback:       1.0,
	}

	// With all factors at 1.0, weighted average should be 1.0
	// Weights: 25 + 15 + 15 + 15 + 10 + 10 + 5 + 5 = 100%
	expectedScore := factors.VerificationStatus*0.25 +
		factors.Uptime*0.15 +
		factors.SuccessRate*0.15 +
		factors.SecurityAlerts*0.15 +
		factors.Compliance*0.10 +
		factors.Age*0.10 +
		factors.DriftDetection*0.05 +
		factors.UserFeedback*0.05

	assert.Equal(t, 1.0, expectedScore)
}

func TestTrustScoreStruct(t *testing.T) {
	now := time.Now()
	scoreID := uuid.New()
	agentID := uuid.New()

	score := TrustScore{
		ID:      scoreID,
		AgentID: agentID,
		Score:   0.85,
		Factors: TrustScoreFactors{
			VerificationStatus: 1.0,
			Uptime:             0.95,
			SuccessRate:        0.9,
			SecurityAlerts:     0.7,
			Compliance:         1.0,
			Age:                0.6,
			DriftDetection:     0.85,
			UserFeedback:       0.8,
		},
		Confidence:     0.9,
		LastCalculated: now,
		CreatedAt:      now,
	}

	assert.Equal(t, scoreID, score.ID)
	assert.Equal(t, agentID, score.AgentID)
	assert.Equal(t, 0.85, score.Score)
	assert.Equal(t, 0.9, score.Confidence)
	assert.NotZero(t, score.Factors.VerificationStatus)
}

func TestTrustScoreHistoryEntryStruct(t *testing.T) {
	now := time.Now()
	entryID := uuid.New()
	agentID := uuid.New()
	orgID := uuid.New()
	changedBy := uuid.New()
	previousScore := 0.75

	entry := TrustScoreHistoryEntry{
		ID:             entryID,
		AgentID:        agentID,
		OrganizationID: orgID,
		TrustScore:     0.85,
		PreviousScore:  &previousScore,
		ChangeReason:   "Verification completed successfully",
		ChangedBy:      &changedBy,
		RecordedAt:     now,
		CreatedAt:      now,
	}

	assert.Equal(t, entryID, entry.ID)
	assert.Equal(t, agentID, entry.AgentID)
	assert.Equal(t, orgID, entry.OrganizationID)
	assert.Equal(t, 0.85, entry.TrustScore)
	assert.NotNil(t, entry.PreviousScore)
	assert.Equal(t, 0.75, *entry.PreviousScore)
	assert.Equal(t, "Verification completed successfully", entry.ChangeReason)
	assert.NotNil(t, entry.ChangedBy)
}

func TestTrustScoreHistoryEntryAutomated(t *testing.T) {
	// Automated changes have nil ChangedBy
	entry := TrustScoreHistoryEntry{
		ID:           uuid.New(),
		AgentID:      uuid.New(),
		TrustScore:   0.6,
		ChangeReason: "Automated recalculation due to security alert",
		ChangedBy:    nil, // NULL for automated changes
	}

	assert.Nil(t, entry.ChangedBy)
	assert.Contains(t, entry.ChangeReason, "Automated")
}

func TestTrustScoreRanges(t *testing.T) {
	tests := []struct {
		name     string
		score    float64
		category string
	}{
		{"critical low", 0.0, "critical"},
		{"very low", 0.2, "low"},
		{"low", 0.4, "low"},
		{"medium", 0.6, "medium"},
		{"high", 0.8, "high"},
		{"perfect", 1.0, "high"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := TrustScore{
				ID:    uuid.New(),
				Score: tt.score,
			}

			// Verify score is within valid range
			assert.GreaterOrEqual(t, score.Score, 0.0)
			assert.LessOrEqual(t, score.Score, 1.0)
		})
	}
}

func TestTrustScoreConfidenceRanges(t *testing.T) {
	tests := []struct {
		name       string
		confidence float64
		valid      bool
	}{
		{"zero confidence", 0.0, true},
		{"low confidence", 0.3, true},
		{"medium confidence", 0.6, true},
		{"high confidence", 0.9, true},
		{"full confidence", 1.0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := TrustScore{
				ID:         uuid.New(),
				Confidence: tt.confidence,
			}

			assert.GreaterOrEqual(t, score.Confidence, 0.0)
			assert.LessOrEqual(t, score.Confidence, 1.0)
		})
	}
}

func TestTrustScoreFactorsBoundary(t *testing.T) {
	// Test all factors at minimum
	minFactors := TrustScoreFactors{
		VerificationStatus: 0.0,
		Uptime:             0.0,
		SuccessRate:        0.0,
		SecurityAlerts:     0.0,
		Compliance:         0.0,
		Age:                0.0,
		DriftDetection:     0.0,
		UserFeedback:       0.0,
	}

	for _, factor := range []float64{
		minFactors.VerificationStatus,
		minFactors.Uptime,
		minFactors.SuccessRate,
		minFactors.SecurityAlerts,
		minFactors.Compliance,
		minFactors.Age,
		minFactors.DriftDetection,
		minFactors.UserFeedback,
	} {
		assert.GreaterOrEqual(t, factor, 0.0)
		assert.LessOrEqual(t, factor, 1.0)
	}

	// Test all factors at maximum
	maxFactors := TrustScoreFactors{
		VerificationStatus: 1.0,
		Uptime:             1.0,
		SuccessRate:        1.0,
		SecurityAlerts:     1.0,
		Compliance:         1.0,
		Age:                1.0,
		DriftDetection:     1.0,
		UserFeedback:       1.0,
	}

	for _, factor := range []float64{
		maxFactors.VerificationStatus,
		maxFactors.Uptime,
		maxFactors.SuccessRate,
		maxFactors.SecurityAlerts,
		maxFactors.Compliance,
		maxFactors.Age,
		maxFactors.DriftDetection,
		maxFactors.UserFeedback,
	} {
		assert.GreaterOrEqual(t, factor, 0.0)
		assert.LessOrEqual(t, factor, 1.0)
	}
}
