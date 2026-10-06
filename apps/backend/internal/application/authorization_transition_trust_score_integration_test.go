//go:build integration

package application

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record/transition"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// interleavingTrust stands for another request that changes the agent while
// a service recalculates its trust score: its first Calculate after set runs
// during, between the service's read of the agent and its write of the score.
// Every call returns score.
type interleavingTrust struct {
	score  float64
	mu     sync.Mutex
	during func()
}

func (c *interleavingTrust) set(during func()) {
	c.mu.Lock()
	c.during = during
	c.mu.Unlock()
}

func (c *interleavingTrust) Calculate(a *domain.Agent) (*domain.TrustScore, error) {
	c.mu.Lock()
	during := c.during
	c.during = nil
	c.mu.Unlock()
	if during != nil {
		during()
	}
	return &domain.TrustScore{ID: uuid.New(), AgentID: a.ID, Score: c.score}, nil
}

func (c *interleavingTrust) CalculateFactors(*domain.Agent) (*domain.TrustScoreFactors, error) {
	return &domain.TrustScoreFactors{}, nil
}

// interleavedServices returns a capability service and an agent service that
// share the fixture's recorder and recalculate trust scores with calc.
func (f *transitionFixture) interleavedServices(calc *interleavingTrust) (*CapabilityService, *AgentService) {
	agentRepo := repository.NewAgentRepository(f.db)
	capRepo := repository.NewCapabilityRepository(sqlx.NewDb(f.db, "postgres"))
	trustRepo := repository.NewTrustScoreRepository(f.db)
	capSvc := NewCapabilityService(capRepo, agentRepo, repository.NewAuditLogRepository(f.db), nil, calc, trustRepo)
	capSvc.SetTransitionRecorder(f.rec)
	agentSvc := NewAgentService(agentRepo, calc, trustRepo, nil, nil, f.policySvc, capRepo, nil, nil, nil, nil, nil)
	agentSvc.SetTransitionRecorder(f.rec)
	return capSvc, agentSvc
}

func (f *transitionFixture) trustScore(t *testing.T) float64 {
	t.Helper()
	var score float64
	require.NoError(t, f.db.QueryRow(`SELECT trust_score FROM agents WHERE id = $1`, f.agentID).Scan(&score))
	return score
}

func (f *transitionFixture) publicKey(t *testing.T) string {
	t.Helper()
	var key string
	require.NoError(t, f.db.QueryRow(`SELECT public_key FROM agents WHERE id = $1`, f.agentID).Scan(&key))
	return key
}

// The trust score a service stores after a recorded change, or on request,
// changes no other column of the agent. The service read the agent before the
// change, so a write of its whole row would put back a status or a key that
// another request changed meanwhile, with no record: a suspension undone, or
// a key rotated out made current again. Each case lets another request
// suspend the agent or rotate its key while the score is recalculated; that
// change stays, the score is stored, and the tables still replay from the
// records.
func TestTransitionTriggerFollowOnTrustScoreKeepsAConcurrentChange(t *testing.T) {
	type op func(t *testing.T, f *transitionFixture, ctx context.Context, capSvc *CapabilityService, agentSvc *AgentService)
	const (
		suspension = "suspension"
		rotation   = "rotation"
	)
	for name, tc := range map[string]struct {
		setup      func(t *testing.T, f *transitionFixture, ctx context.Context)
		run        op
		concurrent string
		status     string
	}{
		"a grant": {
			run: func(t *testing.T, f *transitionFixture, ctx context.Context, capSvc *CapabilityService, _ *AgentService) {
				_, err := capSvc.GrantCapability(ctx, f.agentID, "files:read", nil, &f.userID, "")
				require.NoError(t, err)
			},
			concurrent: suspension, status: "suspended",
		},
		"a capability revocation": {
			run: func(t *testing.T, f *transitionFixture, ctx context.Context, capSvc *CapabilityService, _ *AgentService) {
				granted, err := f.capSvc.GrantCapability(ctx, f.agentID, "files:read", nil, &f.userID, "")
				require.NoError(t, err)
				require.NoError(t, capSvc.RevokeCapability(ctx, granted.ID, &f.userID))
			},
			concurrent: suspension, status: "suspended",
		},
		"an agent update": {
			run: func(t *testing.T, f *transitionFixture, ctx context.Context, _ *CapabilityService, agentSvc *AgentService) {
				_, err := agentSvc.UpdateAgent(ctx, f.agentID, &CreateAgentRequest{Description: "updated"}, f.userID)
				require.NoError(t, err)
			},
			concurrent: suspension, status: "suspended",
		},
		"a talks_to addition": {
			run: func(t *testing.T, f *transitionFixture, ctx context.Context, _ *CapabilityService, agentSvc *AgentService) {
				_, added, err := agentSvc.AddMCPServers(ctx, f.agentID, []string{"github"})
				require.NoError(t, err)
				require.Equal(t, []string{"github"}, added)
			},
			concurrent: suspension, status: "suspended",
		},
		"a talks_to removal": {
			setup: func(t *testing.T, f *transitionFixture, ctx context.Context) {
				_, _, err := f.agentSvc.AddMCPServers(ctx, f.agentID, []string{"github", "filesystem"})
				require.NoError(t, err)
			},
			run: func(t *testing.T, f *transitionFixture, ctx context.Context, _ *CapabilityService, agentSvc *AgentService) {
				_, removed, err := agentSvc.RemoveMCPServers(ctx, f.agentID, []string{"github"})
				require.NoError(t, err)
				require.Equal(t, []string{"github"}, removed)
			},
			concurrent: suspension, status: "suspended",
		},
		"a recalculation": {
			run: func(t *testing.T, f *transitionFixture, ctx context.Context, _ *CapabilityService, agentSvc *AgentService) {
				_, err := agentSvc.RecalculateTrustScore(ctx, f.agentID)
				require.NoError(t, err)
			},
			concurrent: suspension, status: "suspended",
		},
		"a verification": {
			setup: func(t *testing.T, f *transitionFixture, ctx context.Context) {
				_, err := f.db.Exec(`UPDATE agents SET status = 'pending', verified_at = NULL WHERE id = $1`, f.agentID)
				require.NoError(t, err)
			},
			run: func(t *testing.T, f *transitionFixture, ctx context.Context, _ *CapabilityService, agentSvc *AgentService) {
				require.NoError(t, agentSvc.VerifyAgent(ctx, f.agentID))
			},
			concurrent: rotation, status: "verified",
		},
		"a suspension": {
			run: func(t *testing.T, f *transitionFixture, ctx context.Context, _ *CapabilityService, agentSvc *AgentService) {
				require.NoError(t, agentSvc.SuspendAgent(ctx, f.agentID))
			},
			concurrent: rotation, status: "suspended",
		},
		"a reactivation": {
			setup: func(t *testing.T, f *transitionFixture, ctx context.Context) {
				require.NoError(t, f.agentSvc.SuspendAgent(ctx, f.agentID))
			},
			run: func(t *testing.T, f *transitionFixture, ctx context.Context, _ *CapabilityService, agentSvc *AgentService) {
				require.NoError(t, agentSvc.ReactivateAgent(ctx, f.agentID))
			},
			concurrent: rotation, status: "verified",
		},
		"an agent revocation": {
			run: func(t *testing.T, f *transitionFixture, ctx context.Context, _ *CapabilityService, agentSvc *AgentService) {
				require.NoError(t, agentSvc.RevokeAgent(ctx, f.agentID))
			},
			concurrent: rotation, status: "revoked",
		},
	} {
		t.Run(name, func(t *testing.T) {
			f := newTransitionFixture(t)
			ctx := transition.WithActor(context.Background(), transition.User(f.userID))
			if tc.setup != nil {
				tc.setup(t, f, ctx)
			}
			calc := &interleavingTrust{score: 0.75}
			capSvc, agentSvc := f.interleavedServices(calc)

			var rotatedKey string
			calc.set(func() {
				switch tc.concurrent {
				case suspension:
					require.NoError(t, f.agentSvc.SuspendAgent(ctx, f.agentID))
				case rotation:
					key, _, err := f.agentSvc.RotateCredentials(ctx, f.agentID)
					require.NoError(t, err)
					rotatedKey = key
				}
			})
			tc.run(t, f, ctx, capSvc, agentSvc)

			calc.mu.Lock()
			ran := calc.during == nil
			calc.mu.Unlock()
			require.True(t, ran, "the concurrent change did not run")
			assert.Equal(t, tc.status, f.status(t, f.agentID), "the agent's status")
			if tc.concurrent == rotation {
				assert.Equal(t, rotatedKey, f.publicKey(t), "the key rotated in meanwhile is no longer current")
			}
			assert.InDelta(t, 0.75, f.trustScore(t), 1e-9, "the recalculated score is stored")

			replayed, err := transition.CheckReplay(ctx, f.db, f.orgID, f.keys.publicKey())
			require.NoError(t, err, "the tables differ from the state the records rebuild")
			tables, err := transition.CurrentState(ctx, f.db, f.orgID, f.agentID)
			require.NoError(t, err)
			assert.True(t, transition.Equal(tables, replayed.States[f.agentID]))
		})
	}
}
