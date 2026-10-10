package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record/trace"
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

// unmintableRun is a trace run that cannot mint the trace of one organization.
type unmintableRun struct {
	run *trace.Run
	org uuid.UUID
	err error
}

func (r unmintableRun) For(ctx context.Context, organizationID string) (context.Context, error) {
	if organizationID == r.org.String() {
		return nil, r.err
	}
	return r.run.For(ctx, organizationID)
}

// When the trace of an organization cannot be minted, the recorded key expiry sweep
// still reports the agents that failed before it, and still suspends the agents after it.
func TestKeyExpirySweepKeepsEarlierErrorsAndGoesOnWhenATraceCannotBeMinted(t *testing.T) {
	orgA, orgB := uuid.New(), uuid.New()
	failed := repository.AgentRef{ID: uuid.New(), OrganizationID: orgA}
	unminted := repository.AgentRef{ID: uuid.New(), OrganizationID: orgB}
	after := repository.AgentRef{ID: uuid.New(), OrganizationID: orgA}
	errWrite := errors.New("the record could not be written")
	errMint := errors.New("trace: mint: the random source failed")

	var suspendedIDs []uuid.UUID
	suspended, err := sweepExpiredKeys(context.Background(), []repository.AgentRef{failed, unminted, after},
		unmintableRun{run: trace.NewRun(), org: orgB, err: errMint},
		func(_ context.Context, ref repository.AgentRef, traceID string) error {
			assert.True(t, trace.ValidID(traceID), "agent %s has no trace", ref.ID)
			if ref.ID == failed.ID {
				return errWrite
			}
			suspendedIDs = append(suspendedIDs, ref.ID)
			return nil
		})

	require.Error(t, err)
	assert.ErrorIs(t, err, errWrite, "the error of an agent before the mint failure was dropped")
	assert.ErrorIs(t, err, errMint)
	assert.ErrorContains(t, err, "agent "+failed.ID.String())
	assert.ErrorContains(t, err, "agent "+unminted.ID.String())
	assert.Equal(t, 1, suspended)
	assert.Equal(t, []uuid.UUID{after.ID}, suspendedIDs, "the agents after the mint failure were skipped")
}
