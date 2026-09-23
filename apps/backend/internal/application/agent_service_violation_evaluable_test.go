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

// On the capability-violation path the agent row was loaded before the
// violation is inserted, so its violation count is one behind the database
// (migration 091's trigger bumps the row). The evaluation that follows the
// record must still count the violation just recorded: a never-allowed agent
// at a stored 0.26 that takes a blocked violation in a strict-mode
// organization is suspended, not exempted.
func TestAgentService_VerifyCapability_NeverAllowedAgent_BlockedViolation_Suspends(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)
	mockTrustScoreRepo := new(AgentServiceMockTrustScoreRepository)
	mockAlertRepo := new(MockAlertRepository)
	mockCapabilityRepo := new(MockCapabilityRepository)
	mockPolicyRepo := new(AgentServiceMockSecurityPolicyRepository)
	mockOrgRepo := new(MockOrganizationRepository)

	agent := &domain.Agent{
		ID:                       uuid.New(),
		OrganizationID:           uuid.New(),
		Name:                     "my-first-agent",
		Status:                   domain.AgentStatusVerified,
		TrustScore:               0.26,
		CapabilityViolationCount: 0,
		CreatedAt:                time.Now().Add(-time.Hour),
		UpdatedAt:                time.Now(),
	}

	policyService := NewSecurityPolicyService(mockPolicyRepo, mockAlertRepo, nil)
	policyService.SetAgentRepository(mockAgentRepo)
	policyService.SetVerificationEventRepo(statsMock(agent.ID, 2, 0))

	service := &AgentService{
		agentRepo:      mockAgentRepo,
		trustScoreRepo: mockTrustScoreRepo,
		alertRepo:      mockAlertRepo,
		policyService:  policyService,
		capabilityRepo: mockCapabilityRepo,
		orgRepo:        mockOrgRepo,
	}

	mockAgentRepo.On("GetByID", agent.ID).Return(agent, nil)
	mockCapabilityRepo.On("GetActiveCapabilitiesByAgentID", agent.ID).Return([]*domain.AgentCapability{
		{ID: uuid.New(), AgentID: agent.ID, CapabilityType: "db:read"},
	}, nil)
	mockOrgRepo.On("GetByID", agent.OrganizationID).Return(&domain.Organization{
		ID:              agent.OrganizationID,
		EnforcementMode: domain.EnforcementModeStrict,
	}, nil)
	// No capability-violation policy seeded: the evaluator's default is block + alert.
	mockPolicyRepo.On("GetByType", agent.OrganizationID, domain.PolicyTypeCapabilityViolation).
		Return([]*domain.SecurityPolicy{}, nil)
	mockAlertRepo.On("Create", mock.AnythingOfType("*domain.Alert")).Return(nil)
	mockAlertRepo.On("GetByOrganization", agent.OrganizationID, 50, 0).Return([]*domain.Alert{}, nil)
	mockCapabilityRepo.On("CreateViolation", mock.AnythingOfType("*domain.CapabilityViolation")).Return(nil)
	mockAgentRepo.On("UpdateTrustScore", agent.ID, mock.AnythingOfType("float64")).Return(nil)
	mockTrustScoreRepo.On("UpdateScore", agent.ID, mock.AnythingOfType("float64")).Return(nil)
	mockAgentRepo.On("Update", mock.AnythingOfType("*domain.Agent")).Return(nil)

	allowed, reason, _, err := service.VerifyCapability(context.Background(), agent.ID, "db:write", "", nil, "")
	assert.NoError(t, err)
	assert.False(t, allowed)
	assert.Contains(t, reason, "Capability violation blocked")
	assert.Equal(t, domain.AgentStatusSuspended, agent.Status,
		"the violation just recorded counts as evidence; the evaluation that follows it suspends the agent")
	mockAgentRepo.AssertCalled(t, "Update", mock.AnythingOfType("*domain.Agent"))
}
