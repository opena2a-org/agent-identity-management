package application

import (
	"context"
	"errors"
	"testing"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// The lifecycle methods read domain.AgentStatusTransition before they write. A refused or
// already-applied act must not reach the repository: the agent row is left exactly as it
// was, including a revoked agent's status.
func TestAgentServiceLifecycleActsFollowTheTransitionTable(t *testing.T) {
	type act struct {
		name string
		run  func(s *AgentService, ctx context.Context, a *domain.Agent) error
	}
	verify := act{"verify", func(s *AgentService, ctx context.Context, a *domain.Agent) error { return s.VerifyAgent(ctx, a.ID) }}
	suspend := act{"suspend", func(s *AgentService, ctx context.Context, a *domain.Agent) error { return s.SuspendAgent(ctx, a.ID) }}
	reactivate := act{"reactivate", func(s *AgentService, ctx context.Context, a *domain.Agent) error {
		return s.ReactivateAgent(ctx, a.ID)
	}}

	const refused, unchanged = "refused", "unchanged"
	for _, tc := range []struct {
		act  act
		from domain.AgentStatus
		want string
	}{
		{reactivate, domain.AgentStatusRevoked, refused},
		{reactivate, domain.AgentStatusPending, refused},
		{reactivate, domain.AgentStatusVerified, unchanged},
		{reactivate, domain.AgentStatusSuspended, string(domain.AgentStatusVerified)},
		{verify, domain.AgentStatusRevoked, refused},
		{verify, domain.AgentStatusSuspended, refused},
		{verify, domain.AgentStatusVerified, unchanged},
		{verify, domain.AgentStatusPending, string(domain.AgentStatusVerified)},
		{suspend, domain.AgentStatusRevoked, refused},
		{suspend, domain.AgentStatusSuspended, unchanged},
		{suspend, domain.AgentStatusVerified, string(domain.AgentStatusSuspended)},
		{suspend, domain.AgentStatusPending, string(domain.AgentStatusSuspended)},
	} {
		t.Run(tc.act.name+"/"+string(tc.from), func(t *testing.T) {
			repo := new(MockAgentRepository)
			calc := new(AgentServiceMockTrustScoreCalculator)
			scores := new(AgentServiceMockTrustScoreRepository)
			s := &AgentService{agentRepo: repo, trustCalc: calc, trustScoreRepo: scores}

			agent := createTestAgentForService()
			agent.Status = tc.from
			verifiedAt := agent.VerifiedAt
			repo.On("GetByID", agent.ID).Return(agent, nil)
			repo.On("Update", mock.AnythingOfType("*domain.Agent")).Return(nil).Maybe()
			calc.On("Calculate", mock.AnythingOfType("*domain.Agent")).Return(nil, errors.New("not scored in this test")).Maybe()

			err := tc.act.run(s, context.Background(), agent)

			switch tc.want {
			case refused:
				var refusal *domain.AgentStatusTransitionError
				assert.True(t, errors.As(err, &refusal), "want a transition refusal, got %v", err)
				assert.Equal(t, tc.from, agent.Status, "a refused act leaves the status as it was")
				repo.AssertNotCalled(t, "Update", mock.Anything)
			case unchanged:
				assert.NoError(t, err)
				assert.Equal(t, tc.from, agent.Status)
				assert.Same(t, verifiedAt, agent.VerifiedAt, "an act already applied writes nothing")
				repo.AssertNotCalled(t, "Update", mock.Anything)
			default:
				assert.NoError(t, err)
				assert.Equal(t, domain.AgentStatus(tc.want), agent.Status)
				repo.AssertCalled(t, "Update", agent)
			}
		})
	}
}
