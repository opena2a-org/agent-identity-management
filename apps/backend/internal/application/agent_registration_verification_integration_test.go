//go:build integration

package application

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
)

// The possession proof against a real database: the challenge is used up by the
// UPDATE of its row, so two registrations that present it at once cannot both
// win it, and the agent each of them stores carries the status it was given.

func (f *transitionFixture) challengeService(t *testing.T, svc *AgentService) *RegistrationChallengeService {
	t.Helper()
	challenges := NewRegistrationChallengeService(repository.NewRegistrationChallengeRepository(f.db))
	svc.SetRegistrationChallenges(challenges)
	return challenges
}

func (f *transitionFixture) registerWithProof(ctx context.Context, svc *AgentService, name, publicKey string, proof *PossessionProof) (*domain.Agent, error) {
	req := registration(name, publicKey, "api:call")
	req.PossessionProof = proof
	return svc.CreateAgent(ctx, req, f.orgID, f.userID, nil, nil, "")
}

func (f *transitionFixture) challengeUsed(t *testing.T, id uuid.UUID) bool {
	t.Helper()
	var used sql.NullTime
	require.NoError(t, f.db.QueryRow(`SELECT used_at FROM agent_registration_challenges WHERE id = $1`, id).Scan(&used))
	return used.Valid
}

func TestAGoodProofVerifiesASelfKeyedAgentAndEachOtherProofLeavesItPendingInTheDatabase(t *testing.T) {
	f := newTransitionFixture(t)
	ctx := context.Background()
	svc := f.registrar(t)
	challenges := f.challengeService(t, svc)
	f.setEnforcement(t, domain.EnforcementModeMonitoring)
	suffix := uuid.NewString()[:8]
	priv, pub := newKey(t)

	issued, err := challenges.Issue(ctx, f.orgID, f.userID)
	require.NoError(t, err)
	good, err := f.registerWithProof(ctx, svc, "proof-good-"+suffix, pub, signed(t, priv, f.orgID, issued, pub))
	require.NoError(t, err)
	assert.Equal(t, domain.AgentStatusVerified, good.Status)
	assert.Equal(t, 1, f.count(t, `SELECT COUNT(*) FROM agents WHERE id = $1 AND status = 'verified' AND verified_at IS NOT NULL`, good.ID))
	assert.True(t, f.challengeUsed(t, issued.ID))

	reused, err := f.registerWithProof(ctx, svc, "proof-reused-"+suffix, pub, signed(t, priv, f.orgID, issued, pub))
	require.NoError(t, err)
	assert.Equal(t, domain.AgentStatusPending, reused.Status)
	assert.Equal(t, PendingReasonProofUsed, reused.RegistrationPendingReason)

	expired, err := challenges.Issue(ctx, f.orgID, f.userID)
	require.NoError(t, err)
	_, err = f.db.Exec(`UPDATE agent_registration_challenges SET expires_at = now() - interval '1 second' WHERE id = $1`, expired.ID)
	require.NoError(t, err)
	late, err := f.registerWithProof(ctx, svc, "proof-expired-"+suffix, pub, signed(t, priv, f.orgID, expired, pub))
	require.NoError(t, err)
	assert.Equal(t, domain.AgentStatusPending, late.Status)
	assert.Equal(t, PendingReasonProofExpired, late.RegistrationPendingReason)
	assert.True(t, f.challengeUsed(t, expired.ID))

	otherOrg, otherUser := seedOrgAndUser(t, f.db, ctx, "proof-other")
	for name, asked := range map[string][2]uuid.UUID{
		"another organization": {otherOrg, f.userID},
		"another user":         {f.orgID, otherUser},
	} {
		foreign, err := challenges.Issue(ctx, asked[0], asked[1])
		require.NoError(t, err)
		wrong, err := f.registerWithProof(ctx, svc, "proof-foreign-"+suffix+"-"+name[8:11], pub, signed(t, priv, f.orgID, foreign, pub))
		require.NoError(t, err)
		assert.Equal(t, domain.AgentStatusPending, wrong.Status, name)
		assert.Equal(t, PendingReasonProofNotAccepted, wrong.RegistrationPendingReason, name)
		assert.True(t, f.challengeUsed(t, foreign.ID), name)
	}

	f.setEnforcement(t, domain.EnforcementModeStrict)
	strictIssued, err := challenges.Issue(ctx, f.orgID, f.userID)
	require.NoError(t, err)
	strict, err := f.registerWithProof(ctx, svc, "proof-strict-"+suffix, pub, signed(t, priv, f.orgID, strictIssued, pub))
	require.NoError(t, err)
	assert.Equal(t, domain.AgentStatusPending, strict.Status)
	assert.Equal(t, PendingReasonStrictMode, strict.RegistrationPendingReason)
	assert.True(t, f.challengeUsed(t, strictIssued.ID))
}

func TestTheChallengeIsSingleUseUnderTwoConcurrentRegistrationsInTheDatabase(t *testing.T) {
	f := newTransitionFixture(t)
	ctx := context.Background()
	svc := f.registrar(t)
	challenges := f.challengeService(t, svc)
	f.setEnforcement(t, domain.EnforcementModeMonitoring)
	suffix := uuid.NewString()[:8]

	for round := 0; round < 10; round++ {
		priv, pub := newKey(t)
		issued, err := challenges.Issue(ctx, f.orgID, f.userID)
		require.NoError(t, err)
		proof := signed(t, priv, f.orgID, issued, pub)

		start := make(chan struct{})
		var wg sync.WaitGroup
		agents := make([]*domain.Agent, 2)
		errs := make([]error, 2)
		for i := range agents {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-start
				agents[i], errs[i] = f.registerWithProof(ctx, svc, fmt.Sprintf("proof-race-%s-%d-%d", suffix, round, i), pub, proof)
			}(i)
		}
		close(start)
		wg.Wait()

		verified := 0
		for i := range agents {
			require.NoError(t, errs[i])
			if agents[i].Status == domain.AgentStatusVerified {
				verified++
			}
		}
		assert.Equal(t, 1, verified, "round %d: exactly one registration is verified", round)
	}
}

func TestExpiredChallengesAreDeletedAndOthersKept(t *testing.T) {
	f := newTransitionFixture(t)
	ctx := context.Background()
	repo := repository.NewRegistrationChallengeRepository(f.db)
	challenges := NewRegistrationChallengeService(repo)

	live, err := challenges.Issue(ctx, f.orgID, f.userID)
	require.NoError(t, err)
	gone, err := challenges.Issue(ctx, f.orgID, f.userID)
	require.NoError(t, err)
	_, err = f.db.Exec(`UPDATE agent_registration_challenges SET expires_at = now() - interval '1 minute' WHERE id = $1`, gone.ID)
	require.NoError(t, err)

	_, err = repo.DeleteExpired(ctx, time.Now())
	require.NoError(t, err)

	assert.Equal(t, 1, f.count(t, `SELECT COUNT(*) FROM agent_registration_challenges WHERE id = $1`, live.ID))
	assert.Equal(t, 0, f.count(t, `SELECT COUNT(*) FROM agent_registration_challenges WHERE id = $1`, gone.ID))
}
