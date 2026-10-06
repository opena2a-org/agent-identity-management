//go:build integration

package application

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record/store"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record/trace"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record/transition"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// registrar is an agent service that reads the organization's enforcement
// mode and generates a key for an agent that brings none.
func (f *transitionFixture) registrar(t *testing.T) *AgentService {
	t.Helper()
	masterKey := make([]byte, 32)
	_, err := rand.Read(masterKey)
	require.NoError(t, err)
	vault, err := crypto.NewKeyVault(base64.StdEncoding.EncodeToString(masterKey))
	require.NoError(t, err)
	svc := NewAgentService(repository.NewAgentRepository(f.db), transitionTrust{},
		repository.NewTrustScoreRepository(f.db), vault, nil, nil,
		repository.NewCapabilityRepository(sqlx.NewDb(f.db, "postgres")), nil, nil, nil,
		repository.NewOrganizationRepository(f.db), nil)
	svc.SetTransitionRecorder(f.rec)
	return svc
}

func registration(name, publicKey string, capabilities ...string) *CreateAgentRequest {
	return &CreateAgentRequest{
		Name: name, DisplayName: name, Description: "Writes the weekly report",
		AgentType: domain.AgentTypeLangChain, PublicKey: publicKey, Capabilities: capabilities,
	}
}

func (f *transitionFixture) count(t *testing.T, query string, args ...any) int {
	t.Helper()
	var n int
	require.NoError(t, f.db.QueryRow(query, args...).Scan(&n))
	return n
}

// A registration writes one registration_baseline record whose previous_state
// is null and whose new state is the state the registration committed: the
// status decided from the organization's enforcement mode, the baseline
// capabilities and the agent's key with its custody. A later change of the
// agent continues from it, and replay rebuilds each new agent's state.
func TestTransitionTriggerOnRegistrationOpensTheAgentsHistory(t *testing.T) {
	f := newTransitionFixture(t)
	ctx := context.Background()
	svc := f.registrar(t)
	onboarding := &recordingFirstAgentSink{}
	svc.SetOnboardingEvents(onboarding)
	suffix := uuid.NewString()[:8]

	f.setEnforcement(t, domain.EnforcementModeMonitoring)
	verified, err := svc.CreateAgent(ctx, registration("reg-server-"+suffix, "", "files:read", "db:read"),
		f.orgID, f.userID, nil, nil, "")
	require.NoError(t, err)
	assert.Equal(t, domain.AgentStatusVerified, verified.Status)
	external, err := svc.CreateAgent(ctx, registration("reg-external-"+suffix, randomKey(t, ed25519.PublicKeySize), "api:call"),
		f.orgID, f.userID, nil, nil, "")
	require.NoError(t, err)
	assert.Equal(t, domain.AgentStatusPending, external.Status, "an agent holding its own key is not verified automatically")
	f.setEnforcement(t, domain.EnforcementModeStrict)
	strict, err := svc.CreateAgent(ctx, registration("reg-strict-"+suffix, ""), f.orgID, f.userID, nil, nil, "")
	require.NoError(t, err)
	assert.Equal(t, domain.AgentStatusPending, strict.Status, "strict mode verifies by hand")
	assert.Equal(t, 1, f.count(t, `SELECT COUNT(*) FROM trust_scores WHERE agent_id = $1`, verified.ID),
		"one trust score history row for the state the registration committed")
	assert.Equal(t, []uuid.UUID{f.orgID, f.orgID, f.orgID}, onboarding.orgs,
		"a recorded registration tells the onboarding sink, as an unrecorded one does")

	require.NoError(t, svc.SuspendAgent(ctx, verified.ID))

	got := f.transitions(t)
	require.Len(t, got, 4, "one record per registration and one for the suspension")
	want := []struct {
		agent   *domain.Agent
		status  string
		custody string
		granted []string
	}{
		{verified, "verified", transition.KeyCustodyServer, []string{"db:read", "files:read"}},
		{external, "pending", transition.KeyCustodyExternal, []string{"api:call"}},
		{strict, "pending", transition.KeyCustodyServer, []string{}},
	}
	for i, w := range want {
		r := got[i]
		assert.Equal(t, "registration_baseline", r.Trigger, "seq %d", r.Seq)
		assert.True(t, r.PreviousNull, "seq %d: a registration has no previous state", r.Seq)
		assert.Equal(t, w.agent.ID.String(), r.Agent, "seq %d", r.Seq)
		assert.Equal(t, transition.OpeningEventID(w.agent.ID), r.EventID, "seq %d: a registration opens the agent's history", r.Seq)
		assert.Equal(t, "user:"+f.userID.String(), r.Actor, "seq %d", r.Seq)
		assert.Empty(t, r.Parent, "seq %d", r.Seq)
		assert.Empty(t, r.Outcome, "seq %d", r.Seq)
		assert.Equal(t, w.status, r.New.Opena2a.Status, "seq %d", r.Seq)
		assert.Equal(t, w.granted, r.New.Opena2a.GrantedScope, "seq %d", r.Seq)
		assert.Equal(t, w.granted, r.New.Scope, "seq %d", r.Seq)
		require.Len(t, r.New.Opena2a.Keys, 1, "seq %d", r.Seq)
		assert.Equal(t, "Ed25519", r.New.Opena2a.Keys[0].Alg)
		assert.Equal(t, transition.KeyRoleCurrent, r.New.Opena2a.Keys[0].Role)
		assert.Equal(t, w.custody, r.New.Opena2a.Keys[0].Custody, "seq %d", r.Seq)
		if i > 0 {
			assert.NotEqual(t, got[i-1].Trace, r.Trace, "seq %d", r.Seq)
		}
	}
	suspension := got[3]
	assert.Equal(t, "agent_suspended", suspension.Trigger)
	assert.False(t, suspension.PreviousNull)
	assert.Equal(t, got[0].New, suspension.Previous, "the suspension does not continue from the registration")

	assert.Equal(t, 1, f.count(t, `SELECT COUNT(*) FROM agents WHERE id = $1 AND verified_at IS NOT NULL`, verified.ID),
		"the verified registration did not store when it was verified")
	assert.Equal(t, 0, f.count(t, `SELECT COUNT(*) FROM agents WHERE id = $1 AND verified_at IS NOT NULL`, strict.ID))

	replayed, err := transition.CheckReplay(ctx, f.db, f.orgID, f.keys.publicKey())
	require.NoError(t, err)
	assert.Len(t, replayed.Seqs[verified.ID], 2)
	assert.Len(t, replayed.Seqs[external.ID], 1)
	assert.Len(t, replayed.Seqs[strict.ID], 1)
	assert.NotContains(t, replayed.Seqs, f.agentID, "an agent with no change has no record")
}

// With record writes failing, a registration and a pending capability
// request are refused and leave nothing behind: no agent, no baseline
// capability, no request and no record. A registration for an agent that
// already has a row is refused before its statement runs.
func TestTransitionTriggerOnRegistrationAndAPendingRequestWhenTheRecordPathFails(t *testing.T) {
	f := newTransitionFixture(t)
	ctx := context.Background()
	svc := f.registrar(t)
	name := "reg-refused-" + uuid.NewString()[:8]
	f.setEnforcement(t, domain.EnforcementModeStrict)
	f.keys.setFail(errors.New("record key unavailable"))

	_, err := svc.CreateAgent(ctx, registration(name, "", "files:read"), f.orgID, f.userID, nil, nil, "")
	require.ErrorIs(t, err, transition.ErrRecordUnavailable)
	assert.Zero(t, f.count(t, `SELECT COUNT(*) FROM agents WHERE organization_id = $1 AND name = $2`, f.orgID, name))
	assert.Zero(t, f.count(t, `SELECT COUNT(*) FROM agent_capabilities c JOIN agents a ON a.id = c.agent_id
		WHERE a.organization_id = $1`, f.orgID))

	_, err = f.reqSvc.CreateRequest(transition.WithActor(ctx, transition.Agent(f.agentID)), &domain.CreateCapabilityRequestInput{
		AgentID: f.agentID, CapabilityType: "db:write", Reason: "needed by the reporting job", RequestedBy: f.userID,
	})
	require.ErrorIs(t, err, transition.ErrRecordUnavailable)
	assert.Zero(t, f.count(t, `SELECT COUNT(*) FROM capability_requests WHERE agent_id = $1`, f.agentID),
		"a refused request left its row behind")

	assert.Equal(t, float64(2), f.failures(t, store.ClassExpansion, store.ReasonSigner))
	assert.Empty(t, f.transitions(t))

	f.keys.setFail(nil)
	trace, err := transition.NewTraceID()
	require.NoError(t, err)
	ran := false
	_, err = f.rec.Record(ctx, transition.Change{
		OrganizationID: f.orgID,
		AgentID:        f.agentID,
		Trigger:        transition.TriggerRegistrationBaseline,
		Actor:          transition.User(f.userID),
		TraceID:        trace,
		Apply:          func(context.Context, *sql.Tx) error { ran = true; return nil },
	})
	require.ErrorIs(t, err, transition.ErrInvalidChange)
	assert.False(t, ran, "a registration of an existing agent ran its statement")
	assert.Empty(t, f.transitions(t))
}

// A pending capability request writes a capability_requested record that
// leaves the agent's state as it was, at the event id derived from the
// request's id. Its approval and its rejection each name that record as
// parent_id and join its trace, so the chain walks from a request to its
// decision. A request filed while no recorder was set has no record, and its
// decision has no parent and a trace of its own.
func TestTransitionTriggerOnACapabilityRequestWalksToItsDecision(t *testing.T) {
	f := newTransitionFixture(t)
	ctx := context.Background()
	agentCtx := transition.WithActor(ctx, transition.Agent(f.agentID))
	f.setEnforcement(t, domain.EnforcementModeStrict)

	approved := f.request(t, agentCtx, "db:write")
	rejected := f.request(t, agentCtx, "net:egress")
	unrecorded := &domain.CapabilityRequest{
		AgentID: f.agentID, CapabilityType: "api:call", Reason: "needed by the reporting job", RequestedBy: f.userID,
	}
	require.NoError(t, repository.NewCapabilityRequestRepository(sqlx.NewDb(f.db, "postgres")).Create(unrecorded))
	require.NoError(t, f.reqSvc.ApproveRequest(ctx, approved.ID, f.userID))
	require.NoError(t, f.reqSvc.RejectRequest(ctx, rejected.ID, f.userID))
	require.NoError(t, f.reqSvc.ApproveRequest(ctx, unrecorded.ID, f.userID))

	got := f.transitions(t)
	require.Len(t, got, 5)
	byEvent := map[string]transitionRecord{}
	for _, r := range got {
		byEvent[r.EventID] = r
	}
	walk := func(requestID uuid.UUID) (filed, decided transitionRecord) {
		t.Helper()
		filed, ok := byEvent[transition.RequestEventID(requestID)]
		require.True(t, ok, "request %s has no record", requestID)
		var decisions []transitionRecord
		for _, r := range got {
			if r.Parent == filed.EventID {
				decisions = append(decisions, r)
			}
		}
		require.Len(t, decisions, 1, "request %s", requestID)
		return filed, decisions[0]
	}
	user, agentActor := "user:"+f.userID.String(), "agent:"+f.agentID.String()

	filed, decided := walk(approved.ID)
	assert.Equal(t, "capability_requested", filed.Trigger)
	assert.Equal(t, agentActor, filed.Actor)
	assert.Equal(t, filed.Previous, filed.New, "a pending request changed the agent's state")
	assert.Empty(t, filed.Outcome)
	assert.Empty(t, filed.Parent)
	assert.Equal(t, "request_approved", decided.Trigger)
	assert.Equal(t, user, decided.Actor)
	assert.Equal(t, filed.Trace, decided.Trace, "the approval does not join its request's trace")
	assert.Equal(t, []string{"db:write"}, decided.New.Opena2a.GrantedScope)

	filed, decided = walk(rejected.ID)
	assert.Equal(t, "capability_requested", filed.Trigger)
	assert.Equal(t, "request_rejected", decided.Trigger)
	assert.Equal(t, transition.OutcomeRejected, decided.Outcome)
	assert.Equal(t, filed.Trace, decided.Trace, "the rejection does not join its request's trace")
	assert.Equal(t, decided.Previous, decided.New)

	assert.NotContains(t, byEvent, transition.RequestEventID(unrecorded.ID))
	last := got[4]
	assert.Equal(t, "request_approved", last.Trigger)
	assert.Empty(t, last.Parent, "a request with no record gave its decision a parent")
	for _, r := range got[:4] {
		assert.NotEqual(t, r.Trace, last.Trace, "seq %d", r.Seq)
	}
	assert.Equal(t, []string{"api:call", "db:write"}, last.New.Opena2a.GrantedScope)

	parent, found, err := f.rec.RequestParent(ctx, f.orgID, f.agentID, approved.ID)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, transition.Parent{EventID: transition.RequestEventID(approved.ID), TraceID: got[0].Trace}, parent)
	_, _, err = f.rec.RequestParent(ctx, f.orgID, uuid.New(), approved.ID)
	assert.Error(t, err, "the request's record named another agent")
	_, found, err = f.rec.RequestParent(ctx, uuid.New(), f.agentID, approved.ID)
	require.NoError(t, err)
	assert.False(t, found, "another organization's chain holds the request's record")

	_, err = transition.CheckReplay(ctx, f.db, f.orgID, f.keys.publicKey())
	require.NoError(t, err)
}

// When the record at a request's event id is not its capability_requested
// record, the decision cannot name its request. An approval is refused and
// the request stays pending; a rejection narrows nothing, so it still
// commits and is recorded without the link.
func TestTransitionTriggerOfADecisionWhoseRequestRecordIsNotItsOwn(t *testing.T) {
	f := newTransitionFixture(t)
	ctx := context.Background()
	request := &domain.CapabilityRequest{
		AgentID: f.agentID, CapabilityType: "db:write", Reason: "needed by the reporting job", RequestedBy: f.userID,
	}
	require.NoError(t, repository.NewCapabilityRequestRepository(sqlx.NewDb(f.db, "postgres")).Create(request))
	trace, err := transition.NewTraceID()
	require.NoError(t, err)
	_, err = f.rec.Record(ctx, transition.Change{
		OrganizationID: f.orgID,
		AgentID:        f.agentID,
		Trigger:        transition.TriggerDirectGrant,
		Actor:          transition.User(f.userID),
		TraceID:        trace,
		EventID:        transition.RequestEventID(request.ID),
		Apply: func(ctx context.Context, tx *sql.Tx) error {
			return repository.CreateCapabilityTx(ctx, tx, &domain.AgentCapability{
				AgentID: f.agentID, CapabilityType: "files:read", GrantedBy: &f.userID, GrantedAt: time.Now(),
			})
		},
	})
	require.NoError(t, err)

	err = f.reqSvc.ApproveRequest(ctx, request.ID, f.userID)
	require.ErrorIs(t, err, transition.ErrRecordUnavailable)
	assert.Equal(t, "pending", f.requestStatus(t, request.ID))
	require.NoError(t, f.reqSvc.RejectRequest(ctx, request.ID, f.userID))
	assert.Equal(t, "rejected", f.requestStatus(t, request.ID))

	got := f.transitions(t)
	require.Len(t, got, 2)
	assert.Equal(t, "request_rejected", got[1].Trigger)
	assert.Empty(t, got[1].Parent)
	assert.NotEqual(t, got[0].Trace, got[1].Trace)
	assert.Equal(t, []string{"files:read"}, got[1].New.Opena2a.GrantedScope, "the refused approval granted its capability")
}

// newestRecord verifies the fixture's chain and reads the trigger, parent_id
// and trace_id of its newest record. Unlike transitions, it reads a chain
// that holds a record of another type or a record whose tenant part was
// erased; Parent and Trace are empty for an erased tenant part.
func (f *transitionFixture) newestRecord(t *testing.T) transitionRecord {
	t.Helper()
	records, err := store.ReadChain(context.Background(), f.db, f.chainID)
	require.NoError(t, err)
	res, err := record.Verify(records, f.keys.publicKey())
	require.NoError(t, err)
	require.True(t, res.OK(), "the chain does not verify: %v", res.Failure)

	var payload, tenant []byte
	require.NoError(t, f.db.QueryRow(`
		SELECT payload, tenant_part FROM audit_records
		 WHERE chain_id = $1 ORDER BY seq DESC LIMIT 1`, f.chainID).Scan(&payload, &tenant))
	var retained struct {
		EventID string `json:"event_id"`
		Trigger struct {
			Type string `json:"type"`
		} `json:"trigger"`
	}
	require.NoError(t, json.Unmarshal(payload, &retained))
	out := transitionRecord{EventID: retained.EventID, Trigger: retained.Trigger.Type}
	if tenant != nil {
		var part struct {
			TraceID  string  `json:"trace_id"`
			ParentID *string `json:"parent_id"`
		}
		require.NoError(t, json.Unmarshal(tenant, &part))
		out.Trace = part.TraceID
		if part.ParentID != nil {
			out.Parent = *part.ParentID
		}
	}
	return out
}

// A record at a request's event id whose record type is not
// authorization_transition is not the request's record, even when its
// trigger names capability_requested and its tenant part names the agent and
// a trace. An approval is refused and the request stays pending; a rejection
// still commits and is recorded without the link.
func TestTransitionTriggerOfADecisionWhoseRequestRecordIsOfAnotherType(t *testing.T) {
	f := newTransitionFixture(t)
	ctx := context.Background()
	request := &domain.CapabilityRequest{
		AgentID: f.agentID, CapabilityType: "db:write", Reason: "needed by the reporting job", RequestedBy: f.userID,
	}
	require.NoError(t, repository.NewCapabilityRequestRepository(sqlx.NewDb(f.db, "postgres")).Create(request))
	traced, err := trace.Begin(ctx)
	require.NoError(t, err)
	other := record.Draft{
		EventID: transition.RequestEventID(request.ID),
		Type:    "opena2a.administrative",
		Retained: map[string]any{
			"trigger": map[string]any{"type": string(transition.TriggerCapabilityRequested)},
			"opena2a": map[string]any{"source": "system"},
		},
		Tenant: map[string]any{"opena2a": map[string]any{
			"organization_id":  f.orgID.String(),
			"subject_agent_id": f.agentID.String(),
		}},
	}
	require.NoError(t, trace.Stamp(traced, &other, nil))
	_, err = f.w.Write(traced, store.Write{Class: store.ClassObservation, OrganizationID: f.orgID.String(), Draft: other})
	require.NoError(t, err)
	written := f.newestRecord(t)
	require.Equal(t, transition.RequestEventID(request.ID), written.EventID)
	require.Equal(t, string(transition.TriggerCapabilityRequested), written.Trigger)

	_, _, err = f.rec.RequestParent(ctx, f.orgID, f.agentID, request.ID)
	assert.Error(t, err, "a record of another type was read as the request's record")
	err = f.reqSvc.ApproveRequest(ctx, request.ID, f.userID)
	require.ErrorIs(t, err, transition.ErrRecordUnavailable)
	assert.Equal(t, "pending", f.requestStatus(t, request.ID))
	require.NoError(t, f.reqSvc.RejectRequest(ctx, request.ID, f.userID))
	assert.Equal(t, "rejected", f.requestStatus(t, request.ID))

	decided := f.newestRecord(t)
	assert.Equal(t, "request_rejected", decided.Trigger)
	assert.Empty(t, decided.Parent)
	assert.NotEqual(t, written.Trace, decided.Trace)
}

// A request whose capability_requested record has had its tenant part erased
// names no trace for its decision to join. Its approval commits and is
// recorded without a link to the request's record, in a trace of its own.
func TestTransitionTriggerOfADecisionWhoseRequestRecordWasErased(t *testing.T) {
	f := newTransitionFixture(t)
	ctx := context.Background()
	agentCtx := transition.WithActor(ctx, transition.Agent(f.agentID))
	f.setEnforcement(t, domain.EnforcementModeStrict)

	request := f.request(t, agentCtx, "db:write")
	filed := f.newestRecord(t)
	require.Equal(t, transition.RequestEventID(request.ID), filed.EventID)
	require.Equal(t, "capability_requested", filed.Trigger)
	_, err := f.db.Exec(`UPDATE audit_records SET tenant_part = NULL, tenant_salt = NULL WHERE event_id = $1`,
		filed.EventID)
	require.NoError(t, err)

	_, found, err := f.rec.RequestParent(ctx, f.orgID, f.agentID, request.ID)
	require.NoError(t, err)
	assert.False(t, found, "a request record with an erased tenant part was given as a parent")

	require.NoError(t, f.reqSvc.ApproveRequest(ctx, request.ID, f.userID))
	assert.Equal(t, "approved", f.requestStatus(t, request.ID))
	decided := f.newestRecord(t)
	assert.Equal(t, "request_approved", decided.Trigger)
	assert.Empty(t, decided.Parent)
	assert.NotEqual(t, filed.Trace, decided.Trace)
}
