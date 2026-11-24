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
