package application

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// newCapabilityThresholdService builds an AgentService for a verified agent
// with the given trust score holding the given grants, in an organization with
// the given enforcement mode and no security policies. definitions maps
// "namespace:action" to the definition GetCapabilityDefinition returns for the
// agent's organization; any other capability has none (sql.ErrNoRows).
func newCapabilityThresholdService(
	t *testing.T,
	mode domain.EnforcementMode,
	trustScore float64,
	grants []string,
	definitions map[string]*domain.CapabilityDefinition,
) (*AgentService, *domain.Agent, *MockCapabilityRepository) {
	t.Helper()
	agentRepo := new(MockAgentRepository)
	capabilityRepo := new(MockCapabilityRepository)
	policyRepo := new(AgentServiceMockSecurityPolicyRepository)
	orgRepo := new(MockOrganizationRepository)
	alertRepo := new(MockAlertRepository)

	agent := &domain.Agent{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		Name:           "threshold-agent",
		DisplayName:    "Threshold Agent",
		Status:         domain.AgentStatusVerified,
		TrustScore:     trustScore,
		CreatedAt:      time.Now().Add(-time.Hour),
		UpdatedAt:      time.Now(),
	}

	policyService := NewSecurityPolicyService(policyRepo, alertRepo, nil)
	policyService.SetAgentRepository(agentRepo)
	policyService.SetVerificationEventRepo(statsMock(agent.ID, 2, 0))

	agentRepo.On("GetByID", agent.ID).Return(agent, nil)
	capabilityRepo.On("GetActiveCapabilitiesByAgentID", agent.ID).Return(activeGrants(agent.ID, grants), nil)
	orgRepo.On("GetByID", agent.OrganizationID).Return(&domain.Organization{
		ID:              agent.OrganizationID,
		EnforcementMode: mode,
	}, nil)
	policyRepo.On("GetByType", agent.OrganizationID, mock.Anything).Return([]*domain.SecurityPolicy{}, nil)
	for capability, def := range definitions {
		namespace, action, err := domain.ParseCapability(capability)
		require.NoError(t, err)
		capabilityRepo.On("GetCapabilityDefinition", namespace, action, &agent.OrganizationID).Return(def, nil)
	}
	capabilityRepo.On("GetCapabilityDefinition", mock.Anything, mock.Anything, &agent.OrganizationID).Return(nil, sql.ErrNoRows)

	return &AgentService{
		agentRepo:      agentRepo,
		alertRepo:      alertRepo,
		policyService:  policyService,
		capabilityRepo: capabilityRepo,
		orgRepo:        orgRepo,
	}, agent, capabilityRepo
}

func definitionWithMinimum(capability, riskLevel string) *domain.CapabilityDefinition {
	namespace, action, _ := domain.ParseCapability(capability)
	return &domain.CapabilityDefinition{
		ID:             uuid.New(),
		Namespace:      namespace,
		Action:         action,
		CapabilityType: "core",
		RiskLevel:      riskLevel,
		MinTrustScore:  domain.DefaultMinTrustScoreForRiskLevel(riskLevel),
	}
}

// In a strict organization, an agent whose trust score is below the minimum
// of a granted capability's definition is denied that action, with the reason
// naming both scores.
func TestAgentService_VerifyCapability_StrictBelowCapabilityMinimum_Denied(t *testing.T) {
	service, agent, capabilityRepo := newCapabilityThresholdService(t, domain.EnforcementModeStrict, 0.40,
		[]string{"file:delete"},
		map[string]*domain.CapabilityDefinition{"file:delete": definitionWithMinimum("file:delete", domain.RiskLevelHigh)})

	allowed, reason, auditID, err := service.VerifyCapability(context.Background(), agent.ID, "file:delete", "/tmp/report.csv", nil, "")
	require.NoError(t, err)
	assert.False(t, allowed)
	assert.Equal(t, "Action denied: agent trust score 40% is below the 50% required for capability 'file:delete'", reason)
	assert.Equal(t, TrustBelowCapabilityThresholdReason(0.40, 0.50, "file:delete"), reason)
	assert.NotEqual(t, uuid.Nil, auditID)
	capabilityRepo.AssertCalled(t, "GetCapabilityDefinition", "file", "delete", &agent.OrganizationID)
}

// A wildcard grant is checked against the definition of the action requested,
// not the grant.
func TestAgentService_VerifyCapability_StrictWildcardGrantBelowMinimum_Denied(t *testing.T) {
	service, agent, _ := newCapabilityThresholdService(t, domain.EnforcementModeStrict, 0.60,
		[]string{"system:*"},
		map[string]*domain.CapabilityDefinition{"system:admin": definitionWithMinimum("system:admin", domain.RiskLevelCritical)})

	allowed, reason, _, err := service.VerifyCapability(context.Background(), agent.ID, "system:admin", "", nil, "")
	require.NoError(t, err)
	assert.False(t, allowed)
	assert.Equal(t, TrustBelowCapabilityThresholdReason(0.60, 0.70, "system:admin"), reason)
}

// At or above the minimum, and for a capability with a zero minimum or with no
// definition, a granted action is allowed by a strict organization.
func TestAgentService_VerifyCapability_StrictAtOrAboveMinimum_Allowed(t *testing.T) {
	definitions := map[string]*domain.CapabilityDefinition{
		"file:delete": definitionWithMinimum("file:delete", domain.RiskLevelHigh),
		"file:read":   definitionWithMinimum("file:read", domain.RiskLevelLow),
	}
	cases := []struct {
		name       string
		trustScore float64
		capability string
	}{
		{"at the minimum", 0.50, "file:delete"},
		{"above the minimum", 0.51, "file:delete"},
		{"zero minimum", 0.05, "file:read"},
		{"no definition", 0.05, "billing:refund"},
		{"not namespace:action", 0.05, "read_database"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			service, agent, _ := newCapabilityThresholdService(t, domain.EnforcementModeStrict, tc.trustScore,
				[]string{tc.capability}, definitions)

			allowed, reason, _, err := service.VerifyCapability(context.Background(), agent.ID, tc.capability, "", nil, "")
			require.NoError(t, err)
			assert.True(t, allowed, reason)
			assert.Equal(t, "Action matches registered capabilities and passes all security policies", reason)
		})
	}
}

// A monitoring organization allows the action, as it does every other check on
// this path, and its reason says the agent is below the minimum.
func TestAgentService_VerifyCapability_MonitoringBelowCapabilityMinimum_AllowedWithReason(t *testing.T) {
	service, agent, _ := newCapabilityThresholdService(t, domain.EnforcementModeMonitoring, 0.20,
		[]string{"db:write"},
		map[string]*domain.CapabilityDefinition{"db:write": definitionWithMinimum("db:write", domain.RiskLevelMedium)})

	allowed, reason, _, err := service.VerifyCapability(context.Background(), agent.ID, "db:write", "orders", nil, "")
	require.NoError(t, err)
	assert.True(t, allowed)
	assert.Equal(t, "Action allowed by MONITORING mode: agent trust score 20% is below the 30% required for capability 'db:write'", reason)
}

// A failure to read the capability's definition denies the action and returns
// the error; the reason does not carry the error's text.
func TestAgentService_VerifyCapability_DefinitionLookupFails_Denied(t *testing.T) {
	agentRepo := new(MockAgentRepository)
	capabilityRepo := new(MockCapabilityRepository)
	orgRepo := new(MockOrganizationRepository)

	agent := &domain.Agent{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		Name:           "threshold-agent",
		Status:         domain.AgentStatusVerified,
		TrustScore:     0.9,
	}
	agentRepo.On("GetByID", agent.ID).Return(agent, nil)
	capabilityRepo.On("GetActiveCapabilitiesByAgentID", agent.ID).Return(activeGrants(agent.ID, []string{"file:delete"}), nil)
	orgRepo.On("GetByID", agent.OrganizationID).Return(&domain.Organization{
		ID:              agent.OrganizationID,
		EnforcementMode: domain.EnforcementModeStrict,
	}, nil)
	lookupErr := errors.New("connection reset by peer")
	capabilityRepo.On("GetCapabilityDefinition", "file", "delete", &agent.OrganizationID).Return(nil, lookupErr)

	service := &AgentService{agentRepo: agentRepo, capabilityRepo: capabilityRepo, orgRepo: orgRepo}

	allowed, reason, _, err := service.VerifyCapability(context.Background(), agent.ID, "file:delete", "", nil, "")
	assert.ErrorIs(t, err, lookupErr)
	assert.False(t, allowed)
	assert.Equal(t, "Failed to fetch the definition of capability 'file:delete'", reason)
}
