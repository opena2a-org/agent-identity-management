//go:build integration

package application

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record/store"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record/transition"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// orgExpiredKeys lists only the fixture organization's agents to the key
// expiry sweep, so a run leaves other tests' agents alone.
type orgExpiredKeys struct {
	*repository.AgentRepository
	orgID uuid.UUID
}

func (r orgExpiredKeys) AgentsWithExpiredKeys(ctx context.Context, now time.Time) ([]repository.AgentRef, error) {
	refs, err := r.AgentRepository.AgentsWithExpiredKeys(ctx, now)
	out := []repository.AgentRef{}
	for _, ref := range refs {
		if ref.OrganizationID == r.orgID {
			out = append(out, ref)
		}
	}
	return out, err
}

// sweeper is an agent service whose key expiry sweep sees only the fixture's
// organization.
func (f *transitionFixture) sweeper() *AgentService {
	svc := NewAgentService(orgExpiredKeys{repository.NewAgentRepository(f.db), f.orgID}, transitionTrust{},
		repository.NewTrustScoreRepository(f.db), nil, nil, nil, nil, nil, nil, nil, nil, nil)
	svc.SetTransitionRecorder(f.rec)
	return svc
}

func (f *transitionFixture) setEnforcement(t *testing.T, mode domain.EnforcementMode) {
	t.Helper()
	_, err := f.db.Exec(`UPDATE organizations SET enforcement_mode = $2 WHERE id = $1`, f.orgID, string(mode))
	require.NoError(t, err)
}

func (f *transitionFixture) requestStatus(t *testing.T, id uuid.UUID) string {
	t.Helper()
	var status string
	require.NoError(t, f.db.QueryRow(`SELECT status FROM capability_requests WHERE id = $1`, id).Scan(&status))
	return status
}

func (f *transitionFixture) request(t *testing.T, ctx context.Context, capability string) *domain.CapabilityRequest {
	t.Helper()
	req, err := f.reqSvc.CreateRequest(ctx, &domain.CreateCapabilityRequestInput{
		AgentID: f.agentID, CapabilityType: capability, Reason: "needed by the reporting job", RequestedBy: f.userID,
	})
	require.NoError(t, err)
	return req
}

// randomKey is n random bytes in the standard base64 a stored key uses.
func randomKey(t *testing.T, n int) string {
	t.Helper()
	b := make([]byte, n)
	_, err := rand.Read(b)
	require.NoError(t, err)
	return base64.StdEncoding.EncodeToString(b)
}

// multibaseID is the identifier of a stored key other than Ed25519: "u"
// followed by the key bytes in unpadded base64url.
func multibaseID(t *testing.T, stored string) string {
	t.Helper()
	raw, err := base64.StdEncoding.DecodeString(stored)
	require.NoError(t, err)
	return "u" + base64.RawURLEncoding.EncodeToString(raw)
}

// A capability request's automatic approval, approval and rejection, a
// re-registration that drops a capability, a low trust score suspension, a
// reactivation, an agent's own key update, a PQC key update and rotation and
// the key expiry sweep each write exactly one transition record with the
// trigger of its path and the agent's state before and after. A rejection
// leaves the state as it was and says so. Replaying the chain rebuilds both
// agents' states in the tables.
func TestTransitionTriggersOnRequestKeyVerificationAndSuspensionPathsReplayToTheTables(t *testing.T) {
	f := newTransitionFixture(t)
	ctx := context.Background()
	userCtx := transition.WithActor(ctx, transition.User(f.userID))
	agentCtx := transition.WithActor(ctx, transition.Agent(f.agentID))
	other := uuid.New()
	f.insertAgent(t, other)
	original, err := transition.CurrentState(ctx, f.db, f.orgID, f.agentID)
	require.NoError(t, err)
	require.Len(t, original.Keys, 1)

	f.setEnforcement(t, domain.EnforcementModeMonitoring)
	auto := f.request(t, agentCtx, "db:read")
	assert.Equal(t, domain.CapabilityRequestStatusAutoApproved, auto.Status)
	assert.Equal(t, string(domain.CapabilityRequestStatusAutoApproved), f.requestStatus(t, auto.ID))

	f.setEnforcement(t, domain.EnforcementModeStrict)
	approved := f.request(t, agentCtx, "db:write")
	require.Equal(t, domain.CapabilityRequestStatusPending, approved.Status)
	require.Len(t, f.transitions(t), 1, "a pending request changes no authorization")
	require.NoError(t, f.reqSvc.ApproveRequest(ctx, approved.ID, f.userID))
	assert.Equal(t, "approved", f.requestStatus(t, approved.ID))
	rejected := f.request(t, agentCtx, "net:egress")
	require.NoError(t, f.reqSvc.RejectRequest(ctx, rejected.ID, f.userID))
	assert.Equal(t, "rejected", f.requestStatus(t, rejected.ID))

	_, err = f.agentSvc.UpdateAgent(ctx, f.agentID, &CreateAgentRequest{Capabilities: []string{"db:write"}}, f.userID)
	require.NoError(t, err)

	agent, err := f.agentSvc.GetAgent(ctx, f.agentID)
	require.NoError(t, err)
	require.NoError(t, f.policySvc.suspendAgentForLowTrustScore(userCtx, agent))
	// A suspended agent is reactivated; verification moves only a pending
	// agent (TestTransitionTriggerOfAVerificationMovesAPendingAgent).
	require.NoError(t, f.agentSvc.ReactivateAgent(userCtx, f.agentID))

	sdkKey := randomKey(t, ed25519.PublicKeySize)
	require.NoError(t, f.agentSvc.UpdateAgentPublicKey(userCtx, f.agentID, sdkKey, "jwt"))
	firstPQC, secondPQC := randomKey(t, 1952), randomKey(t, 1952)
	require.NoError(t, f.agentSvc.UpdateAgentPQCKey(userCtx, f.agentID, firstPQC, "ML-DSA-65", true))
	require.NoError(t, f.agentSvc.RotateAgentPQCKey(userCtx, f.agentID, secondPQC, "ML-DSA-65"))

	_, err = f.db.Exec(`UPDATE agents SET key_expires_at = NOW() - INTERVAL '1 day' WHERE organization_id = $1`, f.orgID)
	require.NoError(t, err)
	suspended, err := f.sweeper().EnforceKeyExpiry(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, suspended)
	suspended, err = f.sweeper().EnforceKeyExpiry(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, suspended, "a second run finds nothing to suspend")

	got := f.transitions(t)
	require.Len(t, got, 11, "one record per change")
	user, agentActor := "user:"+f.userID.String(), "agent:"+f.agentID.String()
	want := []struct {
		trigger, actor, status string
		granted, scope         []string
	}{
		{"request_auto_approved", agentActor, "verified", []string{"db:read"}, []string{"db:read"}},
		{"request_approved", user, "verified", []string{"db:read", "db:write"}, []string{"db:read", "db:write"}},
		{"request_rejected", user, "verified", []string{"db:read", "db:write"}, []string{"db:read", "db:write"}},
		{"revocation_on_reregistration", user, "verified", []string{"db:write"}, []string{"db:write"}},
		{"agent_suspended", "system", "suspended", []string{"db:write"}, []string{}},
		{"agent_reactivated", user, "verified", []string{"db:write"}, []string{"db:write"}},
		{"key_updated", user, "verified", []string{"db:write"}, []string{"db:write"}},
		{"key_updated", user, "verified", []string{"db:write"}, []string{"db:write"}},
		{"key_rotated", user, "verified", []string{"db:write"}, []string{"db:write"}},
	}
	for i, w := range want {
		r := got[i]
		assert.Equal(t, int64(i+1), r.Seq)
		assert.Equal(t, f.agentID.String(), r.Agent, "seq %d", r.Seq)
		assert.Equal(t, w.trigger, r.Trigger, "seq %d", r.Seq)
		assert.Equal(t, w.actor, r.Actor, "seq %d", r.Seq)
		assert.Equal(t, w.status, r.New.Opena2a.Status, "seq %d", r.Seq)
		assert.Equal(t, w.granted, r.New.Opena2a.GrantedScope, "seq %d", r.Seq)
		assert.Equal(t, w.scope, r.New.Scope, "seq %d", r.Seq)
		if i > 0 {
			assert.Equal(t, got[i-1].New, r.Previous, "seq %d: previous_state is not the new_state before it", r.Seq)
		}
		if w.trigger == "request_rejected" {
			assert.Equal(t, transition.OutcomeRejected, r.Outcome)
			assert.Equal(t, r.Previous, r.New, "a rejection changed the agent's state")
		} else {
			assert.Empty(t, r.Outcome, "seq %d", r.Seq)
		}
	}

	// The agent's own key replaces the original one, which stays as the
	// previous key; the PQC key is added and then rotated.
	ed := original.Keys[0]
	assert.Equal(t, original.Keys, got[5].New.Opena2a.Keys)
	edKeys := got[6].New.Opena2a.Keys
	require.Len(t, edKeys, 2)
	assert.True(t, strings.HasPrefix(edKeys[0].ID, "did:key:z6Mk"), edKeys[0].ID)
	assert.NotEqual(t, ed.ID, edKeys[0].ID)
	assert.Equal(t, transition.Key{Alg: "Ed25519", ID: edKeys[0].ID, Role: transition.KeyRoleCurrent,
		Custody: transition.KeyCustodyExternal}, edKeys[0])
	assert.Equal(t, transition.Key{Alg: "Ed25519", ID: ed.ID, Role: transition.KeyRolePrevious}, edKeys[1])
	assert.Equal(t, append(append([]transition.Key{}, edKeys...), transition.Key{
		Alg: "ML-DSA-65", ID: multibaseID(t, firstPQC), Role: transition.KeyRoleCurrent, Custody: transition.KeyCustodyExternal,
	}), got[7].New.Opena2a.Keys)
	assert.Equal(t, append(append([]transition.Key{}, edKeys...),
		transition.Key{Alg: "ML-DSA-65", ID: multibaseID(t, secondPQC), Role: transition.KeyRoleCurrent, Custody: transition.KeyCustodyExternal},
		transition.Key{Alg: "ML-DSA-65", ID: multibaseID(t, firstPQC), Role: transition.KeyRolePrevious},
	), got[8].New.Opena2a.Keys)

	// The sweep suspends each agent under its own record, and one run shares
	// its trace.
	swept := got[9:]
	agents := []string{swept[0].Agent, swept[1].Agent}
	wantAgents := []string{f.agentID.String(), other.String()}
	sort.Strings(wantAgents)
	assert.Equal(t, wantAgents, agents, "the sweep runs in agent id order")
	assert.Equal(t, swept[0].Trace, swept[1].Trace, "one run is one trace")
	for _, r := range swept {
		assert.Equal(t, "key_expired_suspension", r.Trigger)
		assert.Equal(t, "system", r.Actor)
		assert.Equal(t, "suspended", r.New.Opena2a.Status)
		assert.Equal(t, "verified", r.Previous.Opena2a.Status)
		if r.Agent == f.agentID.String() {
			assert.Equal(t, got[8].New, r.Previous)
		}
	}
	for i := 1; i < len(got); i++ {
		if got[i].Trace == got[i-1].Trace {
			assert.True(t, i == 10, "seq %d shares a trace with the record before it", got[i].Seq)
		}
	}

	replayed, err := transition.CheckReplay(ctx, f.db, f.orgID, f.keys.publicKey())
	require.NoError(t, err)
	assert.Len(t, replayed.Seqs[f.agentID], 10)
	assert.Len(t, replayed.Seqs[other], 1)
	for _, id := range []uuid.UUID{f.agentID, other} {
		tables, err := transition.CurrentState(ctx, f.db, f.orgID, id)
		require.NoError(t, err)
		assert.True(t, transition.Equal(tables, replayed.States[id]), "agent %s", id)
		assert.Equal(t, "suspended", tables.Status)
	}
}

// A rejection is a null transition: one whose statement changes the agent's
// state is refused, and neither the change nor a record is kept.
func TestTransitionTriggerOfARejectionThatChangesTheStateIsRefused(t *testing.T) {
	f := newTransitionFixture(t)
	ctx := context.Background()
	trace, err := transition.NewTraceID()
	require.NoError(t, err)

	_, err = f.rec.Record(ctx, transition.Change{
		OrganizationID: f.orgID,
		AgentID:        f.agentID,
		Trigger:        transition.TriggerRequestRejected,
		Actor:          transition.User(f.userID),
		TraceID:        trace,
		Apply: func(ctx context.Context, tx *sql.Tx) error {
			return repository.CreateCapabilityTx(ctx, tx, &domain.AgentCapability{
				AgentID: f.agentID, CapabilityType: "files:read", GrantedBy: &f.userID, GrantedAt: time.Now(),
			})
		},
	})
	require.ErrorIs(t, err, transition.ErrInvalidChange)
	state, err := transition.CurrentState(ctx, f.db, f.orgID, f.agentID)
	require.NoError(t, err)
	assert.Empty(t, state.GrantedScope)
	assert.Empty(t, f.transitions(t))
}

// With record writes failing, an approval, an automatic approval, a
// reactivation and the agent's key updates are refused and change nothing,
// and a refused approval leaves its request pending without a compensating
// write. A rejection, a re-registration revocation, a low trust score
// suspension and the key expiry sweep commit without their records, are
// counted, and each writes a SECURITY line that names the agent and not the
// user who acted.
func TestTransitionTriggerOnRequestAndKeyPathsWhenTheRecordPathFails(t *testing.T) {
	f := newTransitionFixture(t)
	ctx := context.Background()
	userCtx := transition.WithActor(ctx, transition.User(f.userID))
	agentCtx := transition.WithActor(ctx, transition.Agent(f.agentID))
	other := uuid.New()
	f.insertAgent(t, other)

	_, err := f.capSvc.GrantCapability(ctx, f.agentID, "files:read", nil, &f.userID, "")
	require.NoError(t, err)
	_, err = f.capSvc.GrantCapability(ctx, f.agentID, "files:write", nil, &f.userID, "")
	require.NoError(t, err)
	f.setEnforcement(t, domain.EnforcementModeStrict)
	pendingApproval := f.request(t, agentCtx, "db:write")
	pendingRejection := f.request(t, agentCtx, "net:egress")
	_, err = f.db.Exec(`UPDATE agents SET key_expires_at = NOW() - INTERVAL '1 day' WHERE id = $1`, other)
	require.NoError(t, err)

	f.keys.setFail(errors.New("record key unavailable"))

	require.NoError(t, f.reqSvc.RejectRequest(ctx, pendingRejection.ID, f.userID))
	assert.Equal(t, "rejected", f.requestStatus(t, pendingRejection.ID))
	_, err = f.agentSvc.UpdateAgent(ctx, f.agentID, &CreateAgentRequest{Capabilities: []string{"files:write"}}, f.userID)
	require.NoError(t, err)
	agent, err := f.agentSvc.GetAgent(ctx, f.agentID)
	require.NoError(t, err)
	require.NoError(t, f.policySvc.suspendAgentForLowTrustScore(userCtx, agent))
	suspended, err := f.sweeper().EnforceKeyExpiry(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, suspended)

	before, err := transition.CurrentState(ctx, f.db, f.orgID, f.agentID)
	require.NoError(t, err)
	assert.Equal(t, "suspended", before.Status)
	assert.Equal(t, []string{"files:write"}, before.GrantedScope)
	otherState, err := transition.CurrentState(ctx, f.db, f.orgID, other)
	require.NoError(t, err)
	assert.Equal(t, "suspended", otherState.Status)

	err = f.reqSvc.ApproveRequest(ctx, pendingApproval.ID, f.userID)
	require.ErrorIs(t, err, transition.ErrRecordUnavailable)
	assert.Equal(t, "pending", f.requestStatus(t, pendingApproval.ID), "a refused approval left its request decided")
	f.setEnforcement(t, domain.EnforcementModeMonitoring)
	_, err = f.reqSvc.CreateRequest(agentCtx, &domain.CreateCapabilityRequestInput{
		AgentID: f.agentID, CapabilityType: "api:call", Reason: "needed by the reporting job", RequestedBy: f.userID,
	})
	require.ErrorIs(t, err, transition.ErrRecordUnavailable)
	var requests int
	require.NoError(t, f.db.QueryRow(`SELECT COUNT(*) FROM capability_requests WHERE agent_id = $1 AND capability_type = 'api:call'`,
		f.agentID).Scan(&requests))
	assert.Zero(t, requests, "a refused automatic approval left its request behind")
	require.ErrorIs(t, f.agentSvc.ReactivateAgent(userCtx, f.agentID), transition.ErrRecordUnavailable)
	require.ErrorIs(t, f.agentSvc.UpdateAgentPublicKey(userCtx, f.agentID, randomKey(t, ed25519.PublicKeySize), "jwt"),
		transition.ErrRecordUnavailable)
	require.ErrorIs(t, f.agentSvc.UpdateAgentPQCKey(userCtx, f.agentID, randomKey(t, 1952), "ML-DSA-65", true),
		transition.ErrRecordUnavailable)

	after, err := transition.CurrentState(ctx, f.db, f.orgID, f.agentID)
	require.NoError(t, err)
	assert.True(t, transition.Equal(before, after), "a refused widening changed the agent")

	assert.Equal(t, float64(5), f.failures(t, store.ClassExpansion, store.ReasonSigner))
	assert.Equal(t, float64(4), f.failures(t, store.ClassReduction, store.ReasonSigner))
	require.Len(t, f.transitions(t), 2, "only the grants made before the fault have records")

	var lines []string
	for _, line := range strings.Split(f.logs.String(), "\n") {
		if strings.HasPrefix(line, "SECURITY "+transition.EventTransitionUnrecorded) {
			lines = append(lines, line)
		}
	}
	require.Len(t, lines, 4)
	for i, trigger := range []string{"request_rejected", "revocation_on_reregistration", "agent_suspended", "key_expired_suspension"} {
		assert.Contains(t, lines[i], "trigger="+trigger+" ")
		assert.Contains(t, lines[i], "organization="+f.orgID.String())
		assert.NotContains(t, lines[i], f.userID.String(), "the line names the user who acted")
	}
	assert.Contains(t, lines[3], "subject="+other.String())
}

// A verification moves a pending agent to verified under one agent_verified
// record. With record writes failing it is refused and the agent stays
// pending.
func TestTransitionTriggerOfAVerificationMovesAPendingAgent(t *testing.T) {
	f := newTransitionFixture(t)
	ctx := context.Background()
	userCtx := transition.WithActor(ctx, transition.User(f.userID))
	pending := uuid.New()
	f.insertAgent(t, pending)
	_, err := f.db.Exec(`UPDATE agents SET status = 'pending' WHERE id = $1`, pending)
	require.NoError(t, err)

	f.keys.setFail(errors.New("record key unavailable"))
	require.ErrorIs(t, f.agentSvc.VerifyAgent(userCtx, pending), transition.ErrRecordUnavailable)
	state, err := transition.CurrentState(ctx, f.db, f.orgID, pending)
	require.NoError(t, err)
	assert.Equal(t, "pending", state.Status)
	assert.Empty(t, f.transitions(t))

	f.keys.setFail(nil)
	require.NoError(t, f.agentSvc.VerifyAgent(userCtx, pending))
	got := f.transitions(t)
	require.Len(t, got, 1)
	assert.Equal(t, pending.String(), got[0].Agent)
	assert.Equal(t, "agent_verified", got[0].Trigger)
	assert.Equal(t, "user:"+f.userID.String(), got[0].Actor)
	assert.Equal(t, "pending", got[0].Previous.Opena2a.Status)
	assert.Equal(t, "verified", got[0].New.Opena2a.Status)
}
