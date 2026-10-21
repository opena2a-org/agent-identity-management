package application

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// GetUserFeedbackStats reads the agent's feedback counts from the repository
// the user feedback factor reads them from, for the agent it is given.
func TestTrustCalculator_GetUserFeedbackStats(t *testing.T) {
	ctx := context.Background()
	agentID := uuid.New()

	t.Run("returns the repository's counts for the agent", func(t *testing.T) {
		want := &domain.UserFeedbackStats{Total: 3, PositiveCount: 1, NeutralCount: 1, NegativeCount: 1, AverageRating: 3}
		repo := new(MockUserFeedbackRepository)
		repo.On("GetStats", agentID).Return(want, nil)
		calc := &TrustCalculator{}
		calc.SetUserFeedbackRepo(repo)

		got, err := calc.GetUserFeedbackStats(ctx, agentID)
		require.NoError(t, err)
		assert.Equal(t, want, got)
		repo.AssertExpectations(t)
	})

	t.Run("wraps a failed read", func(t *testing.T) {
		cause := errors.New("connection refused")
		repo := new(MockUserFeedbackRepository)
		repo.On("GetStats", agentID).Return(nil, cause)
		calc := &TrustCalculator{}
		calc.SetUserFeedbackRepo(repo)

		got, err := calc.GetUserFeedbackStats(ctx, agentID)
		assert.Nil(t, got)
		assert.ErrorIs(t, err, cause)
	})

	t.Run("errors when the repository returns no stats", func(t *testing.T) {
		repo := new(MockUserFeedbackRepository)
		repo.On("GetStats", agentID).Return(nil, nil)
		calc := &TrustCalculator{}
		calc.SetUserFeedbackRepo(repo)

		got, err := calc.GetUserFeedbackStats(ctx, agentID)
		assert.Nil(t, got)
		assert.Error(t, err)
	})

	t.Run("errors when no repository is configured", func(t *testing.T) {
		calc := &TrustCalculator{}
		got, err := calc.GetUserFeedbackStats(ctx, agentID)
		assert.Nil(t, got)
		assert.Error(t, err)
	})
}
