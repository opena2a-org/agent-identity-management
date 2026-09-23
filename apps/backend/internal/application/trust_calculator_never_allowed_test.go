package application

import (
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// The numbers a fresh agent reads, pinned from the 2026-09-22 measurement on a
// self-hosted stack. None of these is a defect in the composition rule: the
// verified fallbacks exceed the pending ones, and the low numbers come from
// refusal rows being counted as failed verifications (unit B2's read). They
// are pinned so the not-evaluable branch in the policy service is measured
// against the same regime it exists for.

func neverAllowedCalculator(t *testing.T, stats *domain.AgentVerificationStatistics) (*TrustCalculator, *domain.Agent) {
	t.Helper()
	mockCapabilityRepo := new(MockCapabilityRepository)
	mockAlertRepo := new(TrustCalcMockAlertRepository)
	var verRepo domain.VerificationEventRepository
	if stats != nil {
		m := new(SharedMockVerificationEventRepository)
		m.On("GetAgentStatistics", mock.Anything, mock.Anything, mock.Anything).Return(stats, nil).Maybe()
		verRepo = m
	}
	calculator := NewTrustCalculatorWithVerification(
		new(AgentServiceMockTrustScoreRepository), new(MockAPIKeyRepository),
		new(AgentServiceMockAuditLogRepository), mockCapabilityRepo,
		new(TrustCalcMockAgentRepository), mockAlertRepo, verRepo)
	agent := &domain.Agent{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		Status:         domain.AgentStatusVerified,
		UpdatedAt:      time.Now(),
		CreatedAt:      time.Now().Add(-24 * time.Hour), // under seven days: age 0.3
	}
	mockCapabilityRepo.On("GetViolationsByAgentID", mock.Anything, 500, 0).Return([]*domain.CapabilityViolation{}, 0, nil).Maybe()
	mockAlertRepo.On("GetUnacknowledgedByResourceID", mock.Anything).Return([]*domain.Alert{}, nil).Maybe()
	mockAlertRepo.On("GetByResourceID", mock.Anything, 100, 0).Return([]*domain.Alert{}, nil).Maybe()
	// Wired but empty: compliance and feedback excluded, isolation at its 0.3 baseline.
	calculator.SetSnapshotRepo(&stubSnapshotRepo{snapshot: nil})
	feedbackRepo := new(MockUserFeedbackRepository)
	feedbackRepo.On("GetStats", mock.Anything).Return(&domain.UserFeedbackStats{Total: 0}, nil).Maybe()
	calculator.SetUserFeedbackRepo(feedbackRepo)
	isolationRepo := new(MockIsolationAttestationRepository)
	isolationRepo.On("GetLatest", mock.Anything).Return(nil, nil).Maybe()
	calculator.SetIsolationRepo(isolationRepo)
	return calculator, agent
}

// C4a: two refusal rows and no success read 0 / 0.1 / 0 on factors 1-3 and
// compose to 0.2727 with securityAlerts at 1.0 (the measured 0.1661 is the
// same regime with securityAlerts at 0.375).
func TestTrustCalculator_Calculate_NeverAllowed_TwoRefusalRows(t *testing.T) {
	calculator, agent := neverAllowedCalculator(t, &domain.AgentVerificationStatistics{
		TotalVerifications: 2, SuccessCount: 0, FailedCount: 2, LastVerification: time.Now().Add(-time.Hour),
	})
	score, err := calculator.Calculate(agent)
	assert.NoError(t, err)
	f := score.Factors
	assert.Equal(t, 0.0, f.VerificationStatus)
	assert.InDelta(t, 0.1, f.Uptime, 1e-9)
	assert.Equal(t, 0.0, f.SuccessRate)
	assert.Equal(t, 1.0, f.SecurityAlerts)
	assert.InDelta(t, 0.3, f.Age, 1e-9)
	assert.InDelta(t, 0.2727, score.Score, 5e-4)
}

// C4b: the same agent with zero rows reads the verified fallbacks, 0.8245,
// which is above the pending-zero-rows 0.5925: verification itself never
// lowers a no-data agent.
func TestTrustCalculator_Calculate_ZeroRows_VerifiedAbovePending(t *testing.T) {
	calcV, verified := neverAllowedCalculator(t, nil)
	verifiedScore, err := calcV.Calculate(verified)
	assert.NoError(t, err)
	calcP, pending := neverAllowedCalculator(t, nil)
	pending.Status = domain.AgentStatusPending
	pendingScore, err := calcP.Calculate(pending)
	assert.NoError(t, err)
	assert.InDelta(t, 0.8245, verifiedScore.Score, 5e-4)
	assert.InDelta(t, 0.5925, pendingScore.Score, 5e-4)
	assert.True(t, verifiedScore.Score > pendingScore.Score)
	assert.False(t, math.IsNaN(verifiedScore.Score))
}
