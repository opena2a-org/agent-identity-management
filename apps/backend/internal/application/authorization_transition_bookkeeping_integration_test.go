//go:build integration

package application

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record/transition"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// interleavingAgents stands for another request that changes the agent while
// a service makes a bookkeeping write: its first GetByID after set reads the
// agent and then runs during, between the service's read of the agent and its
// write.
type interleavingAgents struct {
	domain.AgentRepository
	mu     sync.Mutex
	during func()
}

func (a *interleavingAgents) set(during func()) {
	a.mu.Lock()
	a.during = during
	a.mu.Unlock()
}

func (a *interleavingAgents) ran() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.during == nil
}

func (a *interleavingAgents) GetByID(id uuid.UUID) (*domain.Agent, error) {
	agent, err := a.AgentRepository.GetByID(id)
	a.mu.Lock()
	during := a.during
	a.during = nil
	a.mu.Unlock()
	if during != nil {
		during()
	}
	return agent, err
}

// checkingService returns a capability service that shares the fixture's
// recorder and reads agents through agents.
func (f *transitionFixture) checkingService(agents domain.AgentRepository) *CapabilityService {
	capRepo := repository.NewCapabilityRepository(sqlx.NewDb(f.db, "postgres"))
	trustRepo := repository.NewTrustScoreRepository(f.db)
	capSvc := NewCapabilityService(capRepo, agents, repository.NewAuditLogRepository(f.db), nil, transitionTrust{}, trustRepo)
	capSvc.SetTransitionRecorder(f.rec)
	return capSvc
}

// lastCapabilityCheck returns the agent's last_capability_check_at, or nil
// while it is NULL.
func (f *transitionFixture) lastCapabilityCheck(t *testing.T) *time.Time {
	t.Helper()
	var at *time.Time
	require.NoError(t, f.db.QueryRow(`SELECT last_capability_check_at FROM agents WHERE id = $1`, f.agentID).Scan(&at))
	return at
}

// checkAuthorized grants the fixture's agent files:read and returns a check of
// it that must be authorized, with the time just before the check.
func checkAuthorized(t *testing.T, f *transitionFixture, ctx context.Context) func(*CapabilityService) time.Time {
	t.Helper()
	_, err := f.capSvc.GrantCapability(ctx, f.agentID, "files:read", nil, &f.userID, "")
	require.NoError(t, err)
	return func(capSvc *CapabilityService) time.Time {
		before := time.Now()
		result, err := capSvc.VerifyAction(ctx, f.agentID, "files:read", nil, nil, nil, nil)
		require.NoError(t, err)
		require.True(t, result.IsAuthorized, result.Message)
		return before
	}
}

// An authorized capability check stores its own timestamp and changes no
// other column of the agent. The service read the agent before the write, so
// a write of its whole row would put back a status or a key that another
// request changed meanwhile, with no record: a suspension undone, or a key
// rotated out made current again. Each case lets another request suspend the
// agent or rotate its key between the read and the write; that change stays,
// the timestamp is stored, and the tables still replay from the records.
func TestTransitionTriggerCapabilityCheckKeepsAConcurrentChange(t *testing.T) {
	for _, concurrent := range []string{"suspension", "rotation"} {
		t.Run("beside a "+concurrent, func(t *testing.T) {
			f := newTransitionFixture(t)
			ctx := transition.WithActor(context.Background(), transition.User(f.userID))
			check := checkAuthorized(t, f, ctx)
			agents := &interleavingAgents{AgentRepository: repository.NewAgentRepository(f.db)}
			capSvc := f.checkingService(agents)

			status := "verified"
			var rotatedKey string
			agents.set(func() {
				switch concurrent {
				case "suspension":
					require.NoError(t, f.agentSvc.SuspendAgent(ctx, f.agentID))
					status = "suspended"
				case "rotation":
					key, _, err := f.agentSvc.RotateCredentials(ctx, f.agentID)
					require.NoError(t, err)
					rotatedKey = key
				}
			})
			at := check(capSvc)

			require.True(t, agents.ran(), "the concurrent change did not run")
			assert.Equal(t, status, f.status(t, f.agentID), "the agent's status")
			if rotatedKey != "" {
				assert.Equal(t, rotatedKey, f.publicKey(t), "the key rotated in meanwhile is no longer current")
			}
			stored := f.lastCapabilityCheck(t)
			require.NotNil(t, stored, "last_capability_check_at is not stored")
			assert.WithinDuration(t, at, *stored, time.Second)

			replayed, err := transition.CheckReplay(ctx, f.db, f.orgID, f.keys.publicKey())
			require.NoError(t, err, "the tables differ from the state the records rebuild")
			tables, err := transition.CurrentState(ctx, f.db, f.orgID, f.agentID)
			require.NoError(t, err)
			assert.True(t, transition.Equal(tables, replayed.States[f.agentID]))
		})
	}
}

// An authorized capability check is an observation: it writes no record, so
// it does not wait on the record path. With every record write failing, it
// succeeds, stores its timestamp and writes no record.
func TestTransitionTriggerCapabilityCheckSucceedsWhileTheRecordPathFails(t *testing.T) {
	f := newTransitionFixture(t)
	ctx := transition.WithActor(context.Background(), transition.User(f.userID))
	check := checkAuthorized(t, f, ctx)
	capSvc := f.checkingService(repository.NewAgentRepository(f.db))
	records := len(f.allTransitions(t))

	f.keys.setFail(errors.New("the record key is unavailable"))
	at := check(capSvc)

	stored := f.lastCapabilityCheck(t)
	require.NotNil(t, stored, "last_capability_check_at is not stored")
	assert.WithinDuration(t, at, *stored, time.Second)
	assert.Equal(t, "verified", f.status(t, f.agentID))
	assert.Len(t, f.allTransitions(t), records, "a capability check wrote a record")
}

// UpdateLastCapabilityCheck stores the database's time, returns the time it
// stored, and returns sql.ErrNoRows for an id no agent has.
func TestTransitionTriggerCapabilityCheckStoresTheDatabaseTime(t *testing.T) {
	f := newTransitionFixture(t)
	ctx := context.Background()
	agents := repository.NewAgentRepository(f.db)

	checkedAt, err := agents.UpdateLastCapabilityCheck(ctx, f.agentID)
	require.NoError(t, err)
	stored := f.lastCapabilityCheck(t)
	require.NotNil(t, stored)
	assert.True(t, checkedAt.Equal(*stored), "returned %s, stored %s", checkedAt, *stored)
	var updatedAt time.Time
	require.NoError(t, f.db.QueryRow(`SELECT updated_at FROM agents WHERE id = $1`, f.agentID).Scan(&updatedAt))
	assert.True(t, updatedAt.Equal(*stored), "updated_at is not the time of the check")

	_, err = agents.UpdateLastCapabilityCheck(ctx, uuid.New())
	assert.ErrorIs(t, err, sql.ErrNoRows)
}
