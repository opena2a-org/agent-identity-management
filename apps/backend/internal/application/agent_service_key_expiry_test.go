package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// EnforceKeyExpiry (#359). It used to return ErrKeyExpiryEnforcementUnavailable because
// the loop behind it could only reach its body by destroying key material. It now
// delegates to the repository's narrow UPDATE; the predicate itself is pinned against a
// real database in repository/agent_key_expiry_integration_test.go.

func TestEnforceKeyExpiryReturnsHowManyAgentsTheRepositorySuspended(t *testing.T) {
	repo := new(SharedMockAgentRepository)
	repo.On("SuspendAgentsWithExpiredKeys", mock.AnythingOfType("time.Time")).
		Return([]uuid.UUID{uuid.New(), uuid.New()}, nil)
	svc := &AgentService{agentRepo: repo}

	before := time.Now()
	suspended, err := svc.EnforceKeyExpiry(context.Background())
	after := time.Now()

	require.NoError(t, err)
	assert.Equal(t, 2, suspended)
	repo.AssertExpectations(t)

	// The cutoff is the current time, not a zero value that would expire nothing.
	cutoff := repo.Calls[0].Arguments.Get(0).(time.Time)
	assert.False(t, cutoff.Before(before), "cutoff %v is before the call started", cutoff)
	assert.False(t, cutoff.After(after), "cutoff %v is after the call returned", cutoff)

	// The destructive path is not taken: no partial read, no full-row write.
	repo.AssertNotCalled(t, "List", mock.Anything, mock.Anything)
	repo.AssertNotCalled(t, "Update", mock.Anything)
}

func TestEnforceKeyExpiryReportsARepositoryFailureInsteadOfZero(t *testing.T) {
	repo := new(SharedMockAgentRepository)
	repo.On("SuspendAgentsWithExpiredKeys", mock.AnythingOfType("time.Time")).
		Return(nil, errors.New("connection refused"))
	svc := &AgentService{agentRepo: repo}

	suspended, err := svc.EnforceKeyExpiry(context.Background())

	require.Error(t, err)
	assert.ErrorContains(t, err, "connection refused")
	assert.Equal(t, 0, suspended)
}
