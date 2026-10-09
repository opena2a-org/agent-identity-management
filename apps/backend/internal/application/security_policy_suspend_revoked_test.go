package application

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/testutil/mocks"
)

// The low-trust suspension reads the status transition table itself, so a
// revoked agent is refused whatever its caller checked, and is never written as
// suspended, from where reactivate would accept it.
func TestSuspendAgentForLowTrustScore_FollowsTheStatusTransitionTable(t *testing.T) {
	t.Run("a revoked agent is refused and not written", func(t *testing.T) {
		repo := &mocks.MockAgentRepository{}
		s := &SecurityPolicyService{agentRepo: repo}
		agent := &domain.Agent{ID: uuid.New(), Status: domain.AgentStatusRevoked}

		err := s.suspendAgentForLowTrustScore(context.Background(), agent)
		var refusal *domain.AgentStatusTransitionError
		require.ErrorAs(t, err, &refusal)
		assert.Equal(t, domain.AgentStatusRevoked, agent.Status)
		repo.AssertNotCalled(t, "Update", mock.Anything)
	})

	t.Run("an agent already suspended is not written again", func(t *testing.T) {
		repo := &mocks.MockAgentRepository{}
		s := &SecurityPolicyService{agentRepo: repo}
		agent := &domain.Agent{ID: uuid.New(), Status: domain.AgentStatusSuspended}

		require.NoError(t, s.suspendAgentForLowTrustScore(context.Background(), agent))
		repo.AssertNotCalled(t, "Update", mock.Anything)
	})

	for _, from := range []domain.AgentStatus{domain.AgentStatusVerified, domain.AgentStatusPending} {
		t.Run("a "+string(from)+" agent is suspended", func(t *testing.T) {
			repo := &mocks.MockAgentRepository{}
			repo.On("Update", mock.MatchedBy(func(a *domain.Agent) bool {
				return a.Status == domain.AgentStatusSuspended
			})).Return(nil).Once()
			s := &SecurityPolicyService{agentRepo: repo}
			agent := &domain.Agent{ID: uuid.New(), Status: from}

			require.NoError(t, s.suspendAgentForLowTrustScore(context.Background(), agent))
			assert.Equal(t, domain.AgentStatusSuspended, agent.Status)
			repo.AssertExpectations(t)
		})
	}
}
