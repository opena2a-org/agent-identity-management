//go:build integration

package application

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record/store"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record/transition"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// a2aService returns an A2A service that signs for the fixture's agents and
// stores the key it generates for one through the fixture's recorder.
func (f *transitionFixture) a2aService(t *testing.T) *A2AService {
	t.Helper()
	masterKey := make([]byte, 32)
	_, err := rand.Read(masterKey)
	require.NoError(t, err)
	vault, err := crypto.NewKeyVault(base64.StdEncoding.EncodeToString(masterKey))
	require.NoError(t, err)
	svc := NewA2AService(nil, nil, nil, nil, nil, nil, repository.NewA2ARequestNonceRepository(f.db),
		nil, nil, nil, nil, nil, repository.NewAgentRepository(f.db), nil, vault, nil)
	svc.SetTransitionRecorder(f.rec)
	return svc
}

// violate asks for a capability the agent was never granted, with the trust
// score set so that this one violation crosses the compromise threshold.
func (f *transitionFixture) violate(t *testing.T, agentID uuid.UUID) {
	t.Helper()
	_, err := f.db.Exec(`UPDATE agents SET trust_score = 0.35 WHERE id = $1`, agentID)
	require.NoError(t, err)
	res, err := f.capSvc.VerifyAction(context.Background(), agentID, "admin:exfiltrate", nil, nil, nil, nil)
	require.NoError(t, err)
	require.False(t, res.IsAuthorized)
}

func (f *transitionFixture) status(t *testing.T, agentID uuid.UUID) string {
	t.Helper()
	var status string
	require.NoError(t, f.db.QueryRow(`SELECT status FROM agents WHERE id = $1`, agentID).Scan(&status))
	return status
}

// The key the service generates for an agent that signs through it, the
// suspension of an agent whose violations cross the compromise threshold, and
// an agent's deletion each write exactly one record with the trigger of its
// path and the agent's state before and after. A deletion's new_state is null,
// and replay ends the agent's history there.
func TestTransitionTriggerOfAServerKeyACompromiseAndADeletion(t *testing.T) {
	f := newTransitionFixture(t)
	ctx := context.Background()
	userCtx := transition.WithActor(ctx, transition.User(f.userID))
	signer := f.a2aService(t)

	before, err := transition.CurrentState(ctx, f.db, f.orgID, f.agentID)
	require.NoError(t, err)
	require.Len(t, before.Keys, 1)
	agentHeld := before.Keys[0]
	require.Equal(t, transition.KeyCustodyExternal, agentHeld.Custody)

	_, err = f.capSvc.GrantCapability(ctx, f.agentID, "files:read", nil, &f.userID, "")
	require.NoError(t, err)

	// The first signature generates the agent's server key; the second signs
	// with it and changes nothing.
	first, err := signer.SignA2ARequest(userCtx, f.agentID, "POST", "/a2a/tasks", []byte(`{}`))
	require.NoError(t, err)
	assert.NotEmpty(t, first.Signature)
	var publicKey string
	require.NoError(t, f.db.QueryRow(`SELECT public_key FROM agents WHERE id = $1`, f.agentID).Scan(&publicKey))
	_, err = signer.SignA2ARequest(userCtx, f.agentID, "POST", "/a2a/tasks", []byte(`{}`))
	require.NoError(t, err)
	var stillPublic string
	require.NoError(t, f.db.QueryRow(`SELECT public_key FROM agents WHERE id = $1`, f.agentID).Scan(&stillPublic))
	assert.Equal(t, publicKey, stillPublic, "the second signature replaced the server key")

	// A violation that crosses the threshold suspends the agent; a later one
	// finds it suspended and leaves it as it is.
	f.violate(t, f.agentID)
	assert.Equal(t, "suspended", f.status(t, f.agentID))
	f.violate(t, f.agentID)

	// A revoked agent that crosses the threshold stays revoked.
	revoked := uuid.New()
	f.insertAgent(t, revoked)
	require.NoError(t, f.agentSvc.RevokeAgent(userCtx, revoked))
	f.violate(t, revoked)
	assert.Equal(t, "revoked", f.status(t, revoked))

	require.NoError(t, f.agentSvc.DeleteAgent(userCtx, f.agentID))
	var rows int
	require.NoError(t, f.db.QueryRow(`SELECT COUNT(*) FROM agents WHERE id = $1`, f.agentID).Scan(&rows))
	assert.Zero(t, rows)
	require.NoError(t, f.db.QueryRow(`SELECT COUNT(*) FROM agent_capabilities WHERE agent_id = $1`, f.agentID).Scan(&rows))
	assert.Zero(t, rows)

	got := f.transitions(t)
	require.Len(t, got, 5, "one record per change, none for a change that changed nothing")
	user, system := "user:"+f.userID.String(), "system"
	var mine []transitionRecord
	for _, r := range got {
		if r.Agent == f.agentID.String() {
			mine = append(mine, r)
		} else {
			assert.Equal(t, revoked.String(), r.Agent)
			assert.Equal(t, "agent_revoked", r.Trigger, "the revoked agent's only record is its revocation")
		}
	}
	require.Len(t, mine, 4)
	want := []struct{ trigger, actor string }{
		{"direct_grant", user},
		{"key_updated", user},
		{"compromise_suspension", system},
		{"agent_deleted", user},
	}
	for i, w := range want {
		r := mine[i]
		assert.Equal(t, w.trigger, r.Trigger, "seq %d", r.Seq)
		assert.Equal(t, w.actor, r.Actor, "seq %d", r.Seq)
		assert.False(t, r.PreviousNull, "seq %d", r.Seq)
		if i > 0 {
			assert.Equal(t, mine[i-1].New, r.Previous, "seq %d: previous_state is not the new_state before it", r.Seq)
		}
	}

	// The server key replaces the key the agent held, and no previous key is
	// kept. Nothing else changes.
	key := mine[1]
	assert.Equal(t, []transition.Key{agentHeld}, key.Previous.Opena2a.Keys)
	require.Len(t, key.New.Opena2a.Keys, 1)
	server := key.New.Opena2a.Keys[0]
	assert.Equal(t, transition.KeyRoleCurrent, server.Role)
	assert.Equal(t, transition.KeyCustodyServer, server.Custody)
	assert.NotEqual(t, agentHeld.ID, server.ID)
	assert.True(t, strings.HasPrefix(server.ID, "did:key:z6Mk"), server.ID)
	assert.Equal(t, key.Previous.Scope, key.New.Scope)
	assert.Equal(t, "verified", key.New.Opena2a.Status)

	suspension := mine[2]
	assert.Equal(t, "verified", suspension.Previous.Opena2a.Status)
	assert.Equal(t, "suspended", suspension.New.Opena2a.Status)
	assert.Equal(t, []string{"files:read"}, suspension.New.Opena2a.GrantedScope)
	assert.Empty(t, suspension.New.Scope)
	assert.Equal(t, key.New.Opena2a.Keys, suspension.New.Opena2a.Keys)

	deletion := mine[3]
	assert.True(t, deletion.NewNull, "a deletion's new_state is null")
	assert.Equal(t, "suspended", deletion.Previous.Opena2a.Status)

	replayed, err := transition.CheckReplay(ctx, f.db, f.orgID, f.keys.publicKey())
	require.NoError(t, err)
	assert.True(t, replayed.Deleted[f.agentID])
	_, held := replayed.States[f.agentID]
	assert.False(t, held, "a deleted agent has no state")
	require.Len(t, replayed.Seqs[f.agentID], 4)
	assert.Equal(t, "revoked", replayed.States[revoked].Status)
}

// With record writes failing, the generation of a server key and a deletion
// are refused and change nothing, while a compromise suspension commits
// without its record, is counted and writes a SECURITY line that names the
// agent.
func TestTransitionTriggerOfAServerKeyOrADeletionIsRefusedWhenTheRecordPathFails(t *testing.T) {
	f := newTransitionFixture(t)
	ctx := context.Background()
	userCtx := transition.WithActor(ctx, transition.User(f.userID))
	signer := f.a2aService(t)

	_, err := f.capSvc.GrantCapability(ctx, f.agentID, "files:read", nil, &f.userID, "")
	require.NoError(t, err)
	before, err := transition.CurrentState(ctx, f.db, f.orgID, f.agentID)
	require.NoError(t, err)

	f.keys.setFail(errors.New("record key unavailable"))

	_, err = signer.SignA2ARequest(userCtx, f.agentID, "POST", "/a2a/tasks", []byte(`{}`))
	require.ErrorIs(t, err, transition.ErrRecordUnavailable)
	var encrypted sql.NullString
	require.NoError(t, f.db.QueryRow(`SELECT encrypted_private_key FROM agents WHERE id = $1`, f.agentID).Scan(&encrypted))
	assert.False(t, encrypted.Valid && encrypted.String != "", "a refused server key was stored")

	err = f.agentSvc.DeleteAgent(userCtx, f.agentID)
	require.ErrorIs(t, err, transition.ErrRecordUnavailable)
	unchanged, err := transition.CurrentState(ctx, f.db, f.orgID, f.agentID)
	require.NoError(t, err, "a refused deletion deleted the agent")
	assert.True(t, transition.Equal(before, unchanged), "a refused change changed the agent")

	f.violate(t, f.agentID)
	assert.Equal(t, "suspended", f.status(t, f.agentID))

	assert.Equal(t, float64(1), f.failures(t, store.ClassExpansion, store.ReasonSigner))
	assert.Equal(t, float64(1), f.failures(t, store.ClassDestruction, store.ReasonSigner))
	assert.Equal(t, float64(1), f.failures(t, store.ClassReduction, store.ReasonSigner))
	require.Len(t, f.transitions(t), 1, "only the grant made before the fault has a record")

	var lines []string
	for _, line := range strings.Split(f.logs.String(), "\n") {
		if strings.HasPrefix(line, "SECURITY "+transition.EventTransitionUnrecorded) {
			lines = append(lines, line)
		}
	}
	require.Len(t, lines, 1)
	assert.Contains(t, lines[0], "trigger=compromise_suspension")
	assert.Contains(t, lines[0], "organization="+f.orgID.String())
	assert.Contains(t, lines[0], "subject="+f.agentID.String())
	assert.NotContains(t, lines[0], f.userID.String())
}

// A deletion whose statement leaves the agent in place is refused. Once an
// agent's records end in its deletion, a row with its id in the tables is a
// mismatch, and a record after the deletion breaks continuity.
func TestTransitionTriggerReplayEndsAnAgentAtItsDeletion(t *testing.T) {
	f := newTransitionFixture(t)
	ctx := context.Background()
	userCtx := transition.WithActor(ctx, transition.User(f.userID))

	trace, err := transition.NewTraceID()
	require.NoError(t, err)
	_, err = f.rec.Record(ctx, transition.Change{
		OrganizationID: f.orgID,
		AgentID:        f.agentID,
		Trigger:        transition.TriggerAgentDeleted,
		Actor:          transition.User(f.userID),
		TraceID:        trace,
		Apply:          func(context.Context, *sql.Tx) error { return nil },
	})
	require.ErrorIs(t, err, transition.ErrInvalidChange)
	assert.Empty(t, f.transitions(t))

	require.NoError(t, f.agentSvc.DeleteAgent(userCtx, f.agentID))
	_, err = transition.CheckReplay(ctx, f.db, f.orgID, f.keys.publicKey())
	require.NoError(t, err)

	// A path that writes no record puts a row with the deleted agent's id back.
	f.insertAgent(t, f.agentID)
	_, err = transition.CheckReplay(ctx, f.db, f.orgID, f.keys.publicKey())
	var mismatch *transition.MismatchError
	require.True(t, errors.As(err, &mismatch), "want *MismatchError, got %v", err)
	assert.Equal(t, f.agentID, mismatch.AgentID)
	assert.True(t, mismatch.Deleted)

	require.NoError(t, f.agentSvc.SuspendAgent(userCtx, f.agentID))
	_, err = transition.Replay(ctx, f.db, f.orgID, f.keys.publicKey())
	var broken *transition.ContinuityError
	require.True(t, errors.As(err, &broken), "want *ContinuityError, got %v", err)
	assert.Equal(t, f.agentID, broken.AgentID)
	assert.Equal(t, int64(2), broken.Seq)
	assert.Equal(t, int64(1), broken.PreviousSeq)
}
