package application

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// Demo agents (`aim-sdk demo`) carry synthetic activity: the shared cross-org
// trust distribution must never include them. A demo score of 0.95 leaking in
// would show up as a Bucket80to100 entry and an inflated TotalAgents.
func TestBuildTrustDistributionExcludesDemoAgents(t *testing.T) {
	orgID := uuid.New()
	realAgent := &domain.Agent{ID: uuid.New(), AgentType: domain.AgentTypeClaude, TrustScore: 0.55}
	demoAgent := &domain.Agent{ID: uuid.New(), AgentType: domain.AgentTypeDemo, TrustScore: 0.95}

	agentRepo := new(MockAgentRepository)
	agentRepo.On("GetByOrganization", orgID).Return([]*domain.Agent{realAgent, demoAgent}, nil)

	trustRepo := new(AgentServiceMockTrustScoreRepository)
	// No stored score: the service falls back to each agent's own TrustScore.
	trustRepo.On("GetLatest", realAgent.ID).Return(nil, assert.AnError)

	svc := &CommunityIntelligenceService{
		agentRepo: agentRepo,
		trustRepo: trustRepo,
	}

	dist, err := svc.buildTrustDistribution(context.Background(), orgID)
	assert.NoError(t, err)
	assert.Equal(t, 1, dist.TotalAgents, "the demo agent must not be counted")
	assert.Equal(t, 0, dist.ScoreDistribution.Bucket80to100, "the demo agent's 0.95 must not enter the buckets")
	assert.Equal(t, 1, dist.ScoreDistribution.Bucket40to60, "the real agent's 0.55 must remain")
	// GetLatest was consulted for the real agent only.
	trustRepo.AssertNumberOfCalls(t, "GetLatest", 1)
}

// An org whose only agent is the demo agent contributes an empty distribution,
// exactly like an org with no agents at all.
func TestBuildTrustDistributionDemoOnlyOrgIsEmpty(t *testing.T) {
	orgID := uuid.New()
	demoAgent := &domain.Agent{ID: uuid.New(), AgentType: domain.AgentTypeDemo, TrustScore: 0.95}

	agentRepo := new(MockAgentRepository)
	agentRepo.On("GetByOrganization", orgID).Return([]*domain.Agent{demoAgent}, nil)

	svc := &CommunityIntelligenceService{
		agentRepo: agentRepo,
		trustRepo: new(AgentServiceMockTrustScoreRepository),
	}

	dist, err := svc.buildTrustDistribution(context.Background(), orgID)
	assert.NoError(t, err)
	assert.Equal(t, 0, dist.TotalAgents)
}
