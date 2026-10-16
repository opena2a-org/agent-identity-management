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

// VerifyCapability reads the organization's enforcement mode to decide whether a
// denial is enforced (STRICT) or only logged (MONITORING). When that read fails,
// or finds no organization, the verification is denied with a reason that says
// the mode could not be read, and the read's error is returned: the mode is not
// taken to be MONITORING, which would allow what STRICT mode denies. Nothing is
// recorded for the denied request: no violation, no alert, no trust score change.

// newModeUnreadableService builds an AgentService for an agent in status with
// a trust score of 0.40 holding grants, whose organization read answers org and
// orgErr. file:delete has a definition whose minimum trust score (0.50) the
// agent is below. The mocks fail the test on any call they were not told to
// expect.
func newModeUnreadableService(t *testing.T, status domain.AgentStatus, grants []string, org *domain.Organization, orgErr error) (*AgentService, *domain.Agent, *MockCapabilityRepository, *MockAlertRepository) {
	t.Helper()
	agentRepo := new(MockAgentRepository)
	agentRepo.Test(t)
	capabilityRepo := new(MockCapabilityRepository)
	capabilityRepo.Test(t)
	alertRepo := new(MockAlertRepository)
	alertRepo.Test(t)
	trustScoreRepo := new(AgentServiceMockTrustScoreRepository)
	trustScoreRepo.Test(t)
	orgRepo := new(MockOrganizationRepository)
	policyRepo := new(AgentServiceMockSecurityPolicyRepository)

	agent := &domain.Agent{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		Name:           "mode-unreadable-agent",
		DisplayName:    "Mode Unreadable Agent",
		Status:         status,
		TrustScore:     0.40,
		CreatedAt:      time.Now().Add(-time.Hour),
		UpdatedAt:      time.Now(),
	}

	policyService := NewSecurityPolicyService(policyRepo, alertRepo, nil)
	policyService.SetAgentRepository(agentRepo)
	policyService.SetVerificationEventRepo(statsMock(agent.ID, 2, 0))

	agentRepo.On("GetByID", agent.ID).Return(agent, nil)
	capabilityRepo.On("GetActiveCapabilitiesByAgentID", agent.ID).Return(activeGrants(agent.ID, grants), nil)
	if org != nil {
		orgRepo.On("GetByID", agent.OrganizationID).Return(org, orgErr)
	} else {
		orgRepo.On("GetByID", agent.OrganizationID).Return(nil, orgErr)
	}
	policyRepo.On("GetByType", agent.OrganizationID, mock.Anything).Return([]*domain.SecurityPolicy{}, nil)
	capabilityRepo.On("GetCapabilityDefinition", "file", "delete", &agent.OrganizationID).
		Return(definitionWithMinimum("file:delete", domain.RiskLevelHigh), nil)
	capabilityRepo.On("GetCapabilityDefinition", mock.Anything, mock.Anything, &agent.OrganizationID).Return(nil, sql.ErrNoRows)

	return &AgentService{
		agentRepo:      agentRepo,
		trustScoreRepo: trustScoreRepo,
		alertRepo:      alertRepo,
		policyService:  policyService,
		capabilityRepo: capabilityRepo,
		orgRepo:        orgRepo,
	}, agent, capabilityRepo, alertRepo
}

func TestAgentService_VerifyCapability_DeniedWhenTheEnforcementModeCannotBeRead(t *testing.T) {
	readFailure := errors.New("pq: connection reset by peer")
	reads := map[string]struct {
		org *domain.Organization
		err error
	}{
		"the read fails":           {nil, readFailure},
		"no organization is found": {nil, nil},
	}
	checks := map[string]struct {
		status domain.AgentStatus
		grants []string
	}{
		"granted, below the capability's minimum trust score": {domain.AgentStatusVerified, []string{"file:delete"}},
		"the agent is not verified":                           {domain.AgentStatusPending, []string{"file:delete"}},
		"no capability is granted":                            {domain.AgentStatusVerified, nil},
		"another capability is granted":                       {domain.AgentStatusVerified, []string{"file:read"}},
	}
	for readName, read := range reads {
		for checkName, check := range checks {
			t.Run(readName+", "+checkName, func(t *testing.T) {
				service, agent, capabilityRepo, alertRepo := newModeUnreadableService(t, check.status, check.grants, read.org, read.err)

				allowed, reason, auditID, err := service.VerifyCapability(context.Background(), agent.ID, "file:delete", "/tmp/report.csv", nil, "")

				assert.False(t, allowed)
				assert.Equal(t, "Action denied: the enforcement mode of the agent's organization could not be read", reason)
				require.Error(t, err)
				if read.err != nil {
					assert.ErrorIs(t, err, read.err)
				}
				assert.NotEqual(t, uuid.Nil, auditID)
				capabilityRepo.AssertNotCalled(t, "CreateViolation", mock.Anything)
				alertRepo.AssertNotCalled(t, "Create", mock.Anything)
			})
		}
	}
}

// The control: the same agent in an organization whose STRICT mode is read is
// denied by the capability's minimum, and with its MONITORING mode read is
// allowed.
func TestAgentService_VerifyCapability_ModeReadDecidesTheBelowMinimumRequest(t *testing.T) {
	strict, agent, _, _ := newModeUnreadableService(t, domain.AgentStatusVerified, []string{"file:delete"},
		&domain.Organization{EnforcementMode: domain.EnforcementModeStrict}, nil)
	allowed, reason, _, err := strict.VerifyCapability(context.Background(), agent.ID, "file:delete", "/tmp/report.csv", nil, "")
	require.NoError(t, err)
	assert.False(t, allowed)
	assert.Equal(t, TrustBelowCapabilityThresholdReason(0.40, 0.50, "file:delete"), reason)

	monitoring, agent, _, _ := newModeUnreadableService(t, domain.AgentStatusVerified, []string{"file:delete"},
		&domain.Organization{EnforcementMode: domain.EnforcementModeMonitoring}, nil)
	allowed, reason, _, err = monitoring.VerifyCapability(context.Background(), agent.ID, "file:delete", "/tmp/report.csv", nil, "")
	require.NoError(t, err)
	assert.True(t, allowed, reason)
}
