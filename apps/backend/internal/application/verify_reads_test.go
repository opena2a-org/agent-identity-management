package application

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// replicaRouting builds a service whose primary shows the agent as revoked and
// whose replica, lagging, still shows it as verified with a granted capability.
func replicaRouting(t *testing.T, fence func(context.Context) bool) (*AgentService, *MockAgentRepository, *MockAgentRepository, *MockCapabilityRepository, *MockCapabilityRepository, uuid.UUID) {
	t.Helper()
	agentID, orgID := uuid.New(), uuid.New()
	base := domain.Agent{ID: agentID, OrganizationID: orgID, Name: "a", DisplayName: "A", TrustScore: 0.9,
		CreatedAt: time.Now().Add(-time.Hour), UpdatedAt: time.Now()}
	revoked, stale := base, base
	revoked.Status = domain.AgentStatusSuspended
	stale.Status = domain.AgentStatusVerified

	primaryAgents, replicaAgents := new(MockAgentRepository), new(MockAgentRepository)
	primaryCaps, replicaCaps := new(MockCapabilityRepository), new(MockCapabilityRepository)
	primaryAgents.On("GetByID", agentID).Return(&revoked, nil).Maybe()
	replicaAgents.On("GetByID", agentID).Return(&stale, nil).Maybe()
	grant := &domain.AgentCapability{AgentID: agentID, CapabilityType: "file:read"}
	primaryCaps.On("GetActiveCapabilitiesByAgentID", agentID).Return([]*domain.AgentCapability{}, nil).Maybe()
	replicaCaps.On("GetActiveCapabilitiesByAgentID", agentID).Return([]*domain.AgentCapability{grant}, nil).Maybe()

	primaryCaps.On("GetCapabilityDefinition", "file", "read", &orgID).Return(nil, sql.ErrNoRows).Maybe()
	primaryCaps.On("GetCapabilityDefinition", mock.Anything, mock.Anything, &orgID).Return(nil, sql.ErrNoRows).Maybe()
	replicaCaps.On("GetCapabilityDefinition", mock.Anything, mock.Anything, &orgID).Return(nil, sql.ErrNoRows).Maybe()
	orgs := new(MockOrganizationRepository)
	orgs.On("GetByID", orgID).Return(&domain.Organization{ID: orgID, EnforcementMode: domain.EnforcementModeStrict}, nil).Maybe()

	policyRepo := new(AgentServiceMockSecurityPolicyRepository)
	policyRepo.On("GetByType", orgID, mock.Anything).Return([]*domain.SecurityPolicy{}, nil).Maybe()
	alerts := new(MockAlertRepository)
	service := &AgentService{agentRepo: primaryAgents, capabilityRepo: primaryCaps, orgRepo: orgs, alertRepo: alerts,
		policyService: NewSecurityPolicyService(policyRepo, alerts, nil)}
	service.SetVerifyReads(&ReplicaVerifyReads{
		Primary: VerifyReads{Agent: primaryAgents, Capability: primaryCaps, Organization: orgs},
		Replica: VerifyReads{Agent: replicaAgents, Capability: replicaCaps, Organization: orgs},
		Fence:   fence,
	})
	return service, primaryAgents, replicaAgents, primaryCaps, replicaCaps, agentID
}

// While the replica has not caught up with the primary, a revocation the
// primary has committed denies the verification: the reads go to the primary.
func TestVerifyCapability_RevocationHoldsWhileTheReplicaLags(t *testing.T) {
	service, _, replicaAgents, _, replicaCaps, agentID := replicaRouting(t, func(context.Context) bool { return false })

	allowed, reason, _, err := service.VerifyCapability(context.Background(), agentID, "file:read", "/tmp/x", nil, "")

	assert.NoError(t, err)
	assert.False(t, allowed, reason)
	replicaAgents.AssertNotCalled(t, "GetByID", agentID)
	replicaCaps.AssertNotCalled(t, "GetActiveCapabilitiesByAgentID", agentID)
}

// A replica the fence clears answers the verification's reads.
func TestVerifyCapability_ReadsTheReplicaWhenTheFenceClearsIt(t *testing.T) {
	service, primaryAgents, replicaAgents, _, _, agentID := replicaRouting(t, func(context.Context) bool { return true })

	allowed, reason, _, err := service.VerifyCapability(context.Background(), agentID, "file:read", "/tmp/x", nil, "")

	assert.NoError(t, err)
	assert.True(t, allowed, reason)
	replicaAgents.AssertCalled(t, "GetByID", agentID)
	primaryAgents.AssertNotCalled(t, "GetByID", agentID)
}

// With no replica set, the verification reads the repositories the service was
// built with.
func TestVerifyCapability_ReadsThePrimaryWithoutAReplica(t *testing.T) {
	service := &AgentService{}
	reads := service.verifyReads(context.Background())
	assert.Nil(t, reads.Agent)

	primary := new(MockAgentRepository)
	service = &AgentService{agentRepo: primary}
	assert.Same(t, primary, service.verifyReads(context.Background()).Agent)
}

// A fence that is missing reads the primary.
func TestReplicaVerifyReads_NoFenceReadsThePrimary(t *testing.T) {
	primary, replica := new(MockAgentRepository), new(MockAgentRepository)
	routes := &ReplicaVerifyReads{Primary: VerifyReads{Agent: primary}, Replica: VerifyReads{Agent: replica}}
	assert.Same(t, primary, routes.VerifyReads(context.Background()).Agent)
}

// policyRouting builds a service whose agent is verified with file:read
// granted on both the primary and the replica, with separate capability and
// security policy repositories on each, so a test can see where the
// capability definition and the security policies were read.
func policyRouting(t *testing.T, fence func(context.Context) bool) (service *AgentService, primaryCaps, replicaCaps *MockCapabilityRepository, primaryPolicies, replicaPolicies *AgentServiceMockSecurityPolicyRepository, agentID uuid.UUID) {
	t.Helper()
	agentID, orgID := uuid.New(), uuid.New()
	agent := &domain.Agent{ID: agentID, OrganizationID: orgID, Name: "a", DisplayName: "A", TrustScore: 0.9,
		Status: domain.AgentStatusVerified, CreatedAt: time.Now().Add(-time.Hour), UpdatedAt: time.Now()}
	agents := new(MockAgentRepository)
	agents.On("GetByID", agentID).Return(agent, nil).Maybe()
	grant := &domain.AgentCapability{AgentID: agentID, CapabilityType: "file:read"}
	orgs := new(MockOrganizationRepository)
	orgs.On("GetByID", orgID).Return(&domain.Organization{ID: orgID, EnforcementMode: domain.EnforcementModeStrict}, nil).Maybe()

	primaryCaps, replicaCaps = new(MockCapabilityRepository), new(MockCapabilityRepository)
	primaryPolicies, replicaPolicies = new(AgentServiceMockSecurityPolicyRepository), new(AgentServiceMockSecurityPolicyRepository)
	for _, caps := range []*MockCapabilityRepository{primaryCaps, replicaCaps} {
		caps.On("GetActiveCapabilitiesByAgentID", agentID).Return([]*domain.AgentCapability{grant}, nil).Maybe()
	}
	primaryCaps.On("GetCapabilityDefinition", "file", "read", &orgID).Return(nil, sql.ErrNoRows).Maybe()
	replicaCaps.On("GetCapabilityDefinition", "file", "read", &orgID).
		Return(&domain.CapabilityDefinition{Namespace: "file", Action: "read", MinTrustScore: 0.8}, nil).Maybe()
	for _, policies := range []*AgentServiceMockSecurityPolicyRepository{primaryPolicies, replicaPolicies} {
		policies.On("GetByType", orgID, mock.Anything).Return([]*domain.SecurityPolicy{}, nil).Maybe()
	}

	alerts := new(MockAlertRepository)
	service = &AgentService{agentRepo: agents, capabilityRepo: primaryCaps, orgRepo: orgs, alertRepo: alerts,
		policyService: NewSecurityPolicyService(primaryPolicies, alerts, nil)}
	service.SetVerifyReads(&ReplicaVerifyReads{
		Primary: VerifyReads{Agent: agents, Capability: primaryCaps, Organization: orgs, Policy: primaryPolicies},
		Replica: VerifyReads{Agent: agents, Capability: replicaCaps, Organization: orgs, Policy: replicaPolicies},
		Fence:   fence,
	})
	return service, primaryCaps, replicaCaps, primaryPolicies, replicaPolicies, agentID
}

// A replica the fence clears also answers the capability definition and the
// security policies the verification evaluates.
func TestVerifyCapability_ReadsDefinitionsAndPoliciesFromTheReplica(t *testing.T) {
	service, primaryCaps, replicaCaps, primaryPolicies, replicaPolicies, agentID := policyRouting(t, func(context.Context) bool { return true })

	allowed, reason, _, err := service.VerifyCapability(context.Background(), agentID, "file:read", "/tmp/x", nil, "")

	assert.NoError(t, err)
	assert.True(t, allowed, reason)
	replicaCaps.AssertCalled(t, "GetCapabilityDefinition", "file", "read", mock.Anything)
	primaryCaps.AssertNotCalled(t, "GetCapabilityDefinition", mock.Anything, mock.Anything, mock.Anything)
	replicaPolicies.AssertCalled(t, "GetByType", mock.Anything, domain.PolicyTypeTrustScoreLow)
	primaryPolicies.AssertNotCalled(t, "GetByType", mock.Anything, mock.Anything)
}

// While the replica lags, the capability definition and the security policies
// are read from the primary, so a definition or policy changed there applies
// at once.
func TestVerifyCapability_ReadsDefinitionsAndPoliciesFromThePrimaryWhileTheReplicaLags(t *testing.T) {
	service, primaryCaps, replicaCaps, primaryPolicies, replicaPolicies, agentID := policyRouting(t, func(context.Context) bool { return false })

	allowed, reason, _, err := service.VerifyCapability(context.Background(), agentID, "file:read", "/tmp/x", nil, "")

	assert.NoError(t, err)
	assert.True(t, allowed, reason)
	primaryCaps.AssertCalled(t, "GetCapabilityDefinition", "file", "read", mock.Anything)
	replicaCaps.AssertNotCalled(t, "GetCapabilityDefinition", mock.Anything, mock.Anything, mock.Anything)
	primaryPolicies.AssertCalled(t, "GetByType", mock.Anything, domain.PolicyTypeTrustScoreLow)
	replicaPolicies.AssertNotCalled(t, "GetByType", mock.Anything, mock.Anything)
}

// The verification's policy service is the service's own when no policy
// repository is given, and otherwise a copy that reads the given ones and
// leaves the service's own unchanged.
func TestVerifyPolicies_SwapsOnlyTheReads(t *testing.T) {
	primary, replica := new(AgentServiceMockSecurityPolicyRepository), new(AgentServiceMockSecurityPolicyRepository)
	primaryCaps, replicaCaps := new(MockCapabilityRepository), new(MockCapabilityRepository)
	alerts := new(MockAlertRepository)
	policyService := NewSecurityPolicyService(primary, alerts, nil)
	policyService.SetCapabilityRepository(primaryCaps)
	service := &AgentService{policyService: policyService}

	assert.Same(t, policyService, service.verifyPolicies(VerifyReads{}))

	swapped := service.verifyPolicies(VerifyReads{Capability: replicaCaps, Policy: replica})
	assert.Same(t, replica, swapped.policyRepo)
	assert.Same(t, replicaCaps, swapped.capabilityReader)
	assert.Same(t, alerts, swapped.alertRepo)
	assert.Same(t, primary, policyService.policyRepo)
	assert.Same(t, primaryCaps, policyService.capabilityReader)
}
