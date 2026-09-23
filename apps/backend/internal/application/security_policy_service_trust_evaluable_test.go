package application

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// A trust score computed from no allowed action cannot discriminate: two
// status refusals and ten capability violations both drive the verification,
// uptime and success-rate factors to zero. A threshold compared against that
// number is a comparison against noise, so the low-trust policy and the
// update-time suspension do not evaluate an agent that has no allowed action
// in the calculator's window. Measured 2026-09-22 on a self-hosted stack: a
// fresh agent with a db:read grant was refused while pending (two rows,
// status failed), read 0.26 after verification, was blocked by 'Critical
// Trust Score Block' on its granted call, and was suspended at 0.1661 after
// a recalculation, having never run anything.

func criticalTrustScoreBlockPolicy() *domain.SecurityPolicy {
	return &domain.SecurityPolicy{
		ID:                uuid.New(),
		Name:              "Critical Trust Score Block",
		PolicyType:        domain.PolicyTypeTrustScoreLow,
		EnforcementAction: domain.EnforcementBlockAndAlert,
		AppliesTo:         "trust_score_below:0.3",
		IsEnabled:         true,
		// The seeded rules carry "threshold": 50; the evaluator reads
		// "trust_threshold" and falls back to 0.3 (filed separately).
		Rules: map[string]interface{}{"threshold": 50.0, "auto_disable": true},
	}
}

func neverAllowedAgent() *domain.Agent {
	return &domain.Agent{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		Name:           "my-first-agent",
		Status:         domain.AgentStatusVerified,
		TrustScore:     0.26,
		CreatedAt:      time.Now().Add(-time.Hour),
		UpdatedAt:      time.Now(),
	}
}

func statsMock(agentID uuid.UUID, total, success int) *SharedMockVerificationEventRepository {
	m := new(SharedMockVerificationEventRepository)
	m.On("GetAgentStatistics", agentID, mock.Anything, mock.Anything).Return(&domain.AgentVerificationStatistics{
		AgentID:            agentID,
		TotalVerifications: total,
		SuccessCount:       success,
		FailedCount:        total - success,
		LastVerification:   time.Now().Add(-time.Minute),
	}, nil).Maybe()
	return m
}

// C1: a verified agent at 0.26 whose only rows are refusals is not blocked by
// the low-trust policy.
func TestSecurityPolicyService_EvaluateTrustScoreLow_NoAllowedAction_NotEvaluable(t *testing.T) {
	mockPolicyRepo := new(MockSecurityPolicyRepository)
	service := NewSecurityPolicyService(mockPolicyRepo, nil, nil)
	agent := neverAllowedAgent()
	service.SetVerificationEventRepo(statsMock(agent.ID, 2, 0))
	mockPolicyRepo.On("GetByType", agent.OrganizationID, domain.PolicyTypeTrustScoreLow).
		Return([]*domain.SecurityPolicy{criticalTrustScoreBlockPolicy()}, nil)

	blocked, alert, name, err := service.EvaluateTrustScoreLow(context.Background(), agent, "db:read", "", uuid.New())
	assert.NoError(t, err)
	assert.False(t, blocked, "a score computed from no allowed action does not block")
	assert.False(t, alert, "nor does it alert")
	assert.Equal(t, "", name)
}

// C2: a recalculation that lands below the critical constant does not suspend
// an agent that has never run an allowed action.
func TestSecurityPolicyService_EvaluateTrustScoreOnUpdate_NoAllowedAction_NoSuspend(t *testing.T) {
	mockAlertRepo := new(MockAlertRepoForPolicies)
	service := NewSecurityPolicyService(nil, mockAlertRepo, nil)
	agent := neverAllowedAgent()
	service.SetVerificationEventRepo(statsMock(agent.ID, 2, 0))

	result, err := service.EvaluateTrustScoreOnUpdate(context.Background(), agent, 0.26, 0.1661)
	assert.NoError(t, err)
	assert.False(t, result.ShouldSuspend, "no suspension from a number computed from no allowed action")
	assert.False(t, result.ShouldAlert)
	assert.Equal(t, domain.AgentStatusVerified, agent.Status)
	mockAlertRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

// C3: the same agent with allowed actions in the window IS evaluable, so the
// branch cannot over-exempt: 10 rows, 3 successes, 0.26 blocks.
func TestSecurityPolicyService_EvaluateTrustScoreLow_WithAllowedActions_StillBlocks(t *testing.T) {
	mockPolicyRepo := new(MockSecurityPolicyRepository)
	service := NewSecurityPolicyService(mockPolicyRepo, nil, nil)
	agent := neverAllowedAgent()
	service.SetVerificationEventRepo(statsMock(agent.ID, 10, 3))
	mockPolicyRepo.On("GetByType", agent.OrganizationID, domain.PolicyTypeTrustScoreLow).
		Return([]*domain.SecurityPolicy{criticalTrustScoreBlockPolicy()}, nil)

	blocked, alert, name, err := service.EvaluateTrustScoreLow(context.Background(), agent, "db:read", "", uuid.New())
	assert.NoError(t, err)
	assert.True(t, blocked)
	assert.True(t, alert)
	assert.Equal(t, "Critical Trust Score Block", name)
}

// Fail-closed: with no verification repository wired, the service keeps
// today's behaviour and evaluates every agent.
func TestSecurityPolicyService_EvaluateTrustScoreLow_NoRepoWired_Evaluable(t *testing.T) {
	mockPolicyRepo := new(MockSecurityPolicyRepository)
	service := NewSecurityPolicyService(mockPolicyRepo, nil, nil)
	agent := neverAllowedAgent()
	mockPolicyRepo.On("GetByType", agent.OrganizationID, domain.PolicyTypeTrustScoreLow).
		Return([]*domain.SecurityPolicy{criticalTrustScoreBlockPolicy()}, nil)

	blocked, _, name, err := service.EvaluateTrustScoreLow(context.Background(), agent, "db:read", "", uuid.New())
	assert.NoError(t, err)
	assert.True(t, blocked)
	assert.Equal(t, "Critical Trust Score Block", name)
}

// A capability violation is evidence the server produced against the agent's
// own attempt; it is not the lifecycle noise the branch exists to ignore. An
// agent with a violation on record is evaluated like any other, whatever its
// verification rows say.

// C1(a): the update-time evaluator suspends a never-allowed agent that has
// one capability violation on record.
func TestSecurityPolicyService_EvaluateTrustScoreOnUpdate_NoAllowedAction_WithViolation_Suspends(t *testing.T) {
	mockAlertRepo := new(MockAlertRepoForPolicies)
	mockAgentRepo := new(MockAgentRepository)
	service := NewSecurityPolicyService(nil, mockAlertRepo, nil)
	service.SetAgentRepository(mockAgentRepo)
	agent := neverAllowedAgent()
	agent.CapabilityViolationCount = 1
	service.SetVerificationEventRepo(statsMock(agent.ID, 2, 0))
	mockAlertRepo.On("GetByOrganization", agent.OrganizationID, 50, 0).Return([]*domain.Alert{}, nil)
	mockAlertRepo.On("Create", mock.AnythingOfType("*domain.Alert")).Return(nil)
	mockAgentRepo.On("Update", mock.AnythingOfType("*domain.Agent")).Return(nil)

	result, err := service.EvaluateTrustScoreOnUpdate(context.Background(), agent, 0.26, 0.16)
	assert.NoError(t, err)
	assert.True(t, result.ShouldSuspend, "a violation on record makes the agent evaluable")
	assert.True(t, result.ShouldAlert)
	assert.Equal(t, domain.SeverityCritical, result.AlertSeverity)
	assert.Equal(t, domain.AgentStatusSuspended, agent.Status)
	mockAlertRepo.AssertNumberOfCalls(t, "Create", 1)
	mockAgentRepo.AssertCalled(t, "Update", mock.AnythingOfType("*domain.Agent"))
}

// C1(b): the low-trust policy blocks the same agent on its next call.
func TestSecurityPolicyService_EvaluateTrustScoreLow_NoAllowedAction_WithViolation_Blocks(t *testing.T) {
	mockPolicyRepo := new(MockSecurityPolicyRepository)
	service := NewSecurityPolicyService(mockPolicyRepo, nil, nil)
	agent := neverAllowedAgent()
	agent.CapabilityViolationCount = 1
	service.SetVerificationEventRepo(statsMock(agent.ID, 2, 0))
	mockPolicyRepo.On("GetByType", agent.OrganizationID, domain.PolicyTypeTrustScoreLow).
		Return([]*domain.SecurityPolicy{criticalTrustScoreBlockPolicy()}, nil)

	blocked, alert, name, err := service.EvaluateTrustScoreLow(context.Background(), agent, "db:read", "", uuid.New())
	assert.NoError(t, err)
	assert.True(t, blocked, "a violation on record makes the agent evaluable")
	assert.True(t, alert)
	assert.Equal(t, "Critical Trust Score Block", name)
}
