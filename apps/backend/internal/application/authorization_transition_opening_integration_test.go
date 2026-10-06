//go:build integration

package application

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record/store"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record/transition"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The first change of an agent stored before records were written is led by
// the agent's opening state, in the change's transaction and trace: the
// state the tables held, as both previous_state and new_state, written by
// the system at the agent's opening event id. Its later changes write their
// own records only, and replay starts from the opening state.
func TestTransitionOpeningStateLeadsTheFirstChangeOfAnAgentStoredWithoutARecord(t *testing.T) {
	f := newTransitionFixture(t)
	ctx := context.Background()

	before, err := transition.CurrentState(ctx, f.db, f.orgID, f.agentID)
	require.NoError(t, err)
	_, err = f.capSvc.GrantCapability(ctx, f.agentID, "files:read", nil, &f.userID, "")
	require.NoError(t, err)

	all := f.allTransitions(t)
	require.Len(t, all, 2, "the opening state and the grant")
	opening, grant := all[0], all[1]
	assert.Equal(t, int64(1), opening.Seq)
	assert.Equal(t, string(transition.TriggerOpeningState), opening.Trigger)
	assert.Equal(t, transition.OpeningEventID(f.agentID), opening.EventID)
	assert.Equal(t, "system", opening.Actor)
	assert.Equal(t, f.agentID.String(), opening.Agent)
	assert.Empty(t, opening.Parent)
	assert.Empty(t, opening.Outcome)
	assert.False(t, opening.PreviousNull || opening.NewNull)
	assert.Equal(t, opening.Previous, opening.New, "an opening state changes nothing")
	assert.Equal(t, "verified", opening.New.Opena2a.Status)
	assert.Empty(t, opening.New.Opena2a.GrantedScope)
	assert.Equal(t, before.Keys, opening.New.Opena2a.Keys)
	assert.Equal(t, grant.Trace, opening.Trace, "the opening state is written in its change's trace")

	assert.Equal(t, int64(2), grant.Seq)
	assert.Equal(t, "direct_grant", grant.Trigger)
	assert.Equal(t, opening.New, grant.Previous)
	assert.Equal(t, []string{"files:read"}, grant.New.Opena2a.GrantedScope)

	_, err = f.capSvc.GrantCapability(ctx, f.agentID, "files:write", nil, &f.userID, "")
	require.NoError(t, err)
	require.Len(t, f.allTransitions(t), 3, "a later change writes its own record only")

	// The recorder reports the opening state it wrote with a change.
	other := uuid.New()
	f.insertAgent(t, other)
	trace, err := transition.NewTraceID()
	require.NoError(t, err)
	res, err := f.rec.Record(ctx, transition.Change{
		OrganizationID: f.orgID,
		AgentID:        other,
		Trigger:        transition.TriggerAgentSuspended,
		Actor:          transition.User(f.userID),
		TraceID:        trace,
		Apply: func(ctx context.Context, tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, `UPDATE agents SET status = 'suspended' WHERE id = $1`, other)
			return err
		},
	})
	require.NoError(t, err)
	require.True(t, res.Recorded)
	assert.Equal(t, transition.OpeningEventID(other), res.Appended.LeadEventID)
	assert.Equal(t, int64(5), res.Appended.Seq)
	assert.Equal(t, "verified", res.Previous.Status)

	// A change that joins a parent's trace is led by an opening state with a
	// trace of its own, which the server minted. The parent is a record of
	// the chain in that trace: the suspension above.
	joined := uuid.New()
	f.insertAgent(t, joined)
	parentTrace, parent := trace, res.Appended.EventID
	_, err = f.rec.Record(ctx, transition.Change{
		OrganizationID: f.orgID,
		AgentID:        joined,
		Trigger:        transition.TriggerAgentSuspended,
		Actor:          transition.User(f.userID),
		TraceID:        parentTrace,
		ParentID:       parent,
		Apply: func(ctx context.Context, tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, `UPDATE agents SET status = 'suspended' WHERE id = $1`, joined)
			return err
		},
	})
	require.NoError(t, err)
	all = f.allTransitions(t)
	require.Len(t, all, 7)
	joinedOpening, joinedChange := all[5], all[6]
	assert.Equal(t, string(transition.TriggerOpeningState), joinedOpening.Trigger)
	assert.Empty(t, joinedOpening.Parent)
	assert.Len(t, joinedOpening.Trace, 32)
	assert.NotEqual(t, parentTrace, joinedOpening.Trace)
	assert.Equal(t, parentTrace, joinedChange.Trace)
	assert.Equal(t, parent, joinedChange.Parent)

	replayed, err := transition.CheckReplay(ctx, f.db, f.orgID, f.keys.publicKey())
	require.NoError(t, err)
	assert.Equal(t, []int64{1, 2, 3}, replayed.Seqs[f.agentID])
	assert.Equal(t, []int64{4, 5}, replayed.Seqs[other])
	assert.Equal(t, []int64{6, 7}, replayed.Seqs[joined])
}

// The opening state takes the class of the change it leads. With record
// writes failing, a refused expansion takes it with it, and a reduction that
// commits without its record writes none. The next recorded change opens the
// agent's history from the state the tables then hold.
func TestTransitionOpeningStateCommitsOrFailsWithItsChange(t *testing.T) {
	f := newTransitionFixture(t)
	ctx := context.Background()
	userCtx := transition.WithActor(ctx, transition.User(f.userID))

	f.keys.setFail(errors.New("signer unavailable"))
	_, err := f.capSvc.GrantCapability(ctx, f.agentID, "files:read", nil, &f.userID, "")
	require.ErrorIs(t, err, transition.ErrRecordUnavailable)
	assert.Empty(t, f.allTransitions(t), "a refused expansion wrote an opening state")
	require.NoError(t, f.agentSvc.SuspendAgent(userCtx, f.agentID))
	assert.Empty(t, f.allTransitions(t), "a reduction that committed without its record wrote an opening state")
	f.keys.setFail(nil)

	require.NoError(t, f.agentSvc.ReactivateAgent(userCtx, f.agentID))
	all := f.allTransitions(t)
	require.Len(t, all, 2)
	assert.Equal(t, string(transition.TriggerOpeningState), all[0].Trigger)
	assert.Equal(t, "suspended", all[0].New.Opena2a.Status, "the opening state holds the state the tables held")
	assert.Empty(t, all[0].New.Scope)
	assert.Equal(t, "agent_reactivated", all[1].Trigger)
	assert.Equal(t, all[0].New, all[1].Previous)
	_, err = transition.CheckReplay(ctx, f.db, f.orgID, f.keys.publicKey())
	require.NoError(t, err)
}

// The sweep writes one opening state for each agent of the organization
// whose history no record opens, in agent id order and in one trace, from
// the state the tables hold. An agent already opened, and an agent of
// another organization, get none. A second run writes nothing. With record
// writes failing, the sweep stops, counts the failure as an observation and
// changes nothing; a later run writes the rest. Afterwards replay holds
// every agent of the organization, and a later change writes its own record
// only.
func TestTransitionOpeningStateSweepOpensEveryAgentWithoutARecord(t *testing.T) {
	f := newTransitionFixture(t)
	ctx := context.Background()

	opened := uuid.New()
	f.insertAgent(t, opened)
	_, err := f.capSvc.GrantCapability(ctx, opened, "files:read", nil, &f.userID, "")
	require.NoError(t, err)
	second := uuid.New()
	f.insertAgent(t, second)
	_, err = f.db.Exec(`UPDATE agents SET status = 'suspended', talks_to = '["github","memory"]'::jsonb WHERE id = $1`, second)
	require.NoError(t, err)

	otherOrg, otherUser := seedOrgAndUser(t, f.db, ctx, "transition-sweep-other")
	foreign := uuid.New()
	t.Cleanup(func() { _, _ = f.db.Exec(`DELETE FROM agents WHERE id = $1`, foreign) })
	_, err = f.db.Exec(`
		INSERT INTO agents (id, organization_id, name, display_name, agent_type, status, trust_score,
		                    created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $3, 'ai_agent', 'verified', 0.5, $4, NOW(), NOW())`,
		foreign, otherOrg, "transition-sweep-foreign-"+foreign.String()[:8], otherUser)
	require.NoError(t, err)

	replayed, err := transition.CheckReplay(ctx, f.db, f.orgID, f.keys.publicKey())
	require.NoError(t, err)
	assert.NotContains(t, replayed.States, f.agentID, "an agent with no record is not in the replay")
	assert.NotContains(t, replayed.States, second)
	chainBefore := len(f.allTransitions(t))

	swept, err := f.rec.OpenStates(ctx, f.orgID)
	require.NoError(t, err)
	wantWritten := []uuid.UUID{f.agentID, second}
	sort.Slice(wantWritten, func(i, j int) bool { return wantWritten[i].String() < wantWritten[j].String() })
	assert.Equal(t, wantWritten, swept.Written, "the sweep runs in agent id order")
	assert.Equal(t, 1, swept.Existing)
	assert.Equal(t, 0, swept.Gone)
	assert.Len(t, swept.TraceID, 32)

	all := f.allTransitions(t)
	require.Len(t, all, chainBefore+2)
	for i, r := range all[chainBefore:] {
		id := uuid.MustParse(r.Agent)
		assert.Equal(t, wantWritten[i], id)
		assert.Equal(t, string(transition.TriggerOpeningState), r.Trigger)
		assert.Equal(t, transition.OpeningEventID(id), r.EventID)
		assert.Equal(t, "system", r.Actor)
		assert.Equal(t, swept.TraceID, r.Trace, "one run is one trace")
		assert.Empty(t, r.Parent)
		assert.Equal(t, r.Previous, r.New)
		tables, err := transition.CurrentState(ctx, f.db, f.orgID, id)
		require.NoError(t, err)
		if id == second {
			assert.Equal(t, "suspended", r.New.Opena2a.Status)
			assert.Equal(t, []string{"github", "memory"}, r.New.Opena2a.TalksTo)
			assert.Empty(t, r.New.Scope)
		}
		assert.Equal(t, tables.Status, r.New.Opena2a.Status)
		assert.Equal(t, tables.Keys, r.New.Opena2a.Keys)
	}

	again, err := f.rec.OpenStates(ctx, f.orgID)
	require.NoError(t, err)
	assert.Empty(t, again.Written)
	assert.Equal(t, 3, again.Existing)
	assert.NotEqual(t, swept.TraceID, again.TraceID, "each run mints its own trace")
	require.Len(t, f.allTransitions(t), chainBefore+2, "a second run wrote a record")

	late := uuid.New()
	f.insertAgent(t, late)
	f.keys.setFail(errors.New("signer unavailable"))
	failed, err := f.rec.OpenStates(ctx, f.orgID)
	require.Error(t, err)
	var we *store.WriteError
	require.True(t, errors.As(err, &we), "want a *store.WriteError, got %v", err)
	assert.Empty(t, failed.Written)
	assert.Equal(t, float64(1), f.failures(t, store.ClassObservation, store.ReasonSigner),
		"the sweep stops at its first failed write")
	require.Len(t, f.allTransitions(t), chainBefore+2)
	f.keys.setFail(nil)
	rest, err := f.rec.OpenStates(ctx, f.orgID)
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{late}, rest.Written)

	replayed, err = transition.CheckReplay(ctx, f.db, f.orgID, f.keys.publicKey())
	require.NoError(t, err)
	for _, id := range []uuid.UUID{f.agentID, opened, second, late} {
		assert.Contains(t, replayed.States, id)
	}
	assert.NotContains(t, replayed.States, foreign, "the sweep wrote a record for another organization's agent")
	var foreignChains int
	require.NoError(t, f.db.QueryRow(`SELECT count(*) FROM record_chains WHERE organization_id = $1`, otherOrg).Scan(&foreignChains))
	assert.Zero(t, foreignChains)

	_, err = f.capSvc.GrantCapability(ctx, f.agentID, "files:read", nil, &f.userID, "")
	require.NoError(t, err)
	mine := 0
	for _, r := range f.allTransitions(t) {
		if r.Agent == f.agentID.String() {
			mine++
		}
	}
	assert.Equal(t, 2, mine, "the swept agent's change wrote a second opening state")
	_, err = transition.CheckReplay(ctx, f.db, f.orgID, f.keys.publicKey())
	require.NoError(t, err)
}

// A writer in the pre-chain state has no chain to open a history in: a sweep
// of an organization whose chain has not started stops at its first agent
// with ErrChainNotStarted, reports nothing written and starts no chain.
func TestTransitionOpeningStateSweepWritesNothingBeforeTheChain(t *testing.T) {
	f := newTransitionFixture(t)
	ctx := context.Background()
	org, user := seedOrgAndUser(t, f.db, ctx, "transition-sweep-prechain")
	agent := uuid.New()
	t.Cleanup(func() { _, _ = f.db.Exec(`DELETE FROM agents WHERE id = $1`, agent) })
	_, err := f.db.Exec(`
		INSERT INTO agents (id, organization_id, name, display_name, agent_type, status, trust_score,
		                    created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $3, 'ai_agent', 'verified', 0.5, $4, NOW(), NOW())`,
		agent, org, "transition-sweep-prechain-"+agent.String()[:8], user)
	require.NoError(t, err)

	metrics, err := store.NewMetrics(prometheus.NewRegistry())
	require.NoError(t, err)
	w, err := store.NewWriter(store.Config{DB: f.db, Keys: f.keys, Key: f.keys.publicKey(), Metrics: metrics, PreChain: true})
	require.NoError(t, err)
	rec, err := transition.NewRecorder(transition.Config{Writer: w, DB: f.db, Issuer: "urn:uuid:" + uuid.NewString()})
	require.NoError(t, err)

	swept, err := rec.OpenStates(ctx, org)
	require.ErrorIs(t, err, transition.ErrChainNotStarted)
	assert.Empty(t, swept.Written)
	assert.Zero(t, swept.Existing)
	var chains int
	require.NoError(t, f.db.QueryRow(`SELECT count(*) FROM record_chains WHERE organization_id = $1`, org).Scan(&chains))
	assert.Zero(t, chains)
}

// Replay starts each agent's history at its registration or its opening
// state. It refuses an agent whose first record is another change, an
// opening state that changes the agent's state, and an opening state after
// the agent's first record. A record at an agent's opening event id that does
// not open its history refuses the agent's next change.
func TestTransitionReplayStartsEachAgentAtTheRecordThatOpensItsHistory(t *testing.T) {
	state := func(status string) map[string]any {
		return map[string]any{
			"scope": []any{},
			"opena2a": map[string]any{
				"granted_scope": []any{}, "status": status, "keys": []any{}, "talks_to": []any{},
			},
		}
	}

	t.Run("a first record that is another change", func(t *testing.T) {
		f := newTransitionFixture(t)
		appendRawTransition(t, f, uuid.NewString(), "agent_suspended", state("verified"), state("suspended"))
		_, err := transition.Replay(context.Background(), f.db, f.orgID, f.keys.publicKey())
		var unopened *transition.UnopenedError
		require.True(t, errors.As(err, &unopened), "want *UnopenedError, got %v", err)
		assert.Equal(t, f.agentID, unopened.AgentID)
		assert.Equal(t, int64(1), unopened.Seq)
		assert.Equal(t, transition.TriggerAgentSuspended, unopened.Trigger)
	})

	t.Run("an opening state that changes the state", func(t *testing.T) {
		f := newTransitionFixture(t)
		appendRawTransition(t, f, transition.OpeningEventID(f.agentID), "opening_state", state("verified"), state("suspended"))
		_, err := transition.Replay(context.Background(), f.db, f.orgID, f.keys.publicKey())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "the opening state at seq 1 changes the agent's state")
	})

	t.Run("an opening state after the first record", func(t *testing.T) {
		f := newTransitionFixture(t)
		ctx := context.Background()
		_, err := f.capSvc.GrantCapability(ctx, f.agentID, "files:read", nil, &f.userID, "")
		require.NoError(t, err)
		appendRawTransition(t, f, uuid.NewString(), "opening_state", state("verified"), state("verified"))
		_, err = transition.Replay(ctx, f.db, f.orgID, f.keys.publicKey())
		var broken *transition.ContinuityError
		require.True(t, errors.As(err, &broken), "want *ContinuityError, got %v", err)
		assert.Equal(t, int64(3), broken.Seq)
		assert.Equal(t, int64(2), broken.PreviousSeq)
	})

	t.Run("a foreign record at the opening event id", func(t *testing.T) {
		f := newTransitionFixture(t)
		ctx := context.Background()
		w := rawWriter(t, f)
		trace, err := transition.NewTraceID()
		require.NoError(t, err)
		_, err = w.Write(ctx, store.Write{Class: store.ClassObservation, OrganizationID: f.orgID.String(), Draft: record.Draft{
			EventID: transition.OpeningEventID(f.agentID),
			Type:    "opena2a.administrative",
			Retained: map[string]any{"opena2a": map[string]any{
				"admin_action": "tag_created", "resource_type": "tag", "trace_origin": "server",
			}},
			Tenant: map[string]any{
				"trace_id":  trace,
				"parent_id": nil,
				"opena2a":   map[string]any{"resource_id": uuid.NewString()},
			},
			Personal: map[string]any{"actor": "system"},
		}})
		require.NoError(t, err)
		_, err = f.capSvc.GrantCapability(ctx, f.agentID, "files:read", nil, &f.userID, "")
		require.ErrorIs(t, err, transition.ErrRecordUnavailable)
		assert.True(t, strings.Contains(err.Error(), "does not open its history"), err.Error())
		current, err := transition.CurrentState(ctx, f.db, f.orgID, f.agentID)
		require.NoError(t, err)
		assert.Empty(t, current.GrantedScope, "the refused grant changed the agent")
	})
}

// rawWriter is a record writer on the fixture's chain that writes whatever
// draft it is given, past the recorder's rules.
func rawWriter(t *testing.T, f *transitionFixture) *store.Writer {
	t.Helper()
	metrics, err := store.NewMetrics(prometheus.NewRegistry())
	require.NoError(t, err)
	w, err := store.NewWriter(store.Config{DB: f.db, Keys: f.keys, Key: f.keys.publicKey(), Metrics: metrics})
	require.NoError(t, err)
	return w
}

// appendRawTransition appends a transition of the fixture's agent with the
// given trigger and states, as no path of the service would.
func appendRawTransition(t *testing.T, f *transitionFixture, eventID, trigger string, previous, next map[string]any) {
	t.Helper()
	trace, err := transition.NewTraceID()
	require.NoError(t, err)
	_, err = rawWriter(t, f).Write(context.Background(), store.Write{
		Class:          store.ClassObservation,
		OrganizationID: f.orgID.String(),
		Draft: record.Draft{
			EventID: eventID,
			Type:    transition.RecordType,
			Retained: map[string]any{
				"trigger": map[string]any{"type": trigger},
				"opena2a": map[string]any{
					"issuer":         "urn:uuid:" + uuid.NewString(),
					"writer_version": transition.WriterVersion,
					"source":         transition.Source,
					"state_space":    transition.StateSpaceAgent,
					"trace_origin":   "server",
				},
			},
			Tenant: map[string]any{
				"trace_id":       trace,
				"parent_id":      nil,
				"previous_state": previous,
				"new_state":      next,
				"opena2a": map[string]any{
					"organization_id":  f.orgID.String(),
					"subject_agent_id": f.agentID.String(),
				},
			},
			Personal: map[string]any{"actor": "system"},
		},
	})
	require.NoError(t, err)
}
