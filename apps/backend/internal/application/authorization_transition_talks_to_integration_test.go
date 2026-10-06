//go:build integration

package application

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record/store"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record/transition"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// talksToServices are the MCP and detection services of the fixture, each
// with the fixture's recorder set.
func (f *transitionFixture) talksToServices() (*MCPService, *DetectionService) {
	mcp := &MCPService{agentRepo: repository.NewAgentRepository(f.db)}
	mcp.SetTransitionRecorder(f.rec)
	detect := NewDetectionService(f.db, nil, nil)
	detect.SetTransitionRecorder(f.rec)
	return mcp, detect
}

func (f *transitionFixture) storedTalksTo(t *testing.T) []string {
	t.Helper()
	state, err := transition.CurrentState(context.Background(), f.db, f.orgID, f.agentID)
	require.NoError(t, err)
	return state.TalksTo
}

func detectionReport(servers ...string) *domain.DetectionReportRequest {
	req := &domain.DetectionReportRequest{}
	for _, server := range servers {
		req.Detections = append(req.Detections, domain.DetectionEvent{
			MCPServer: server, DetectionMethod: domain.DetectionMethodSDKRuntime, Confidence: 90,
		})
	}
	return req
}

// Every writer of an agent's talks_to list writes exactly one transition
// record per change, with the trigger of its act, the actor who acted and the
// whole list before and after. A call that leaves the list as it was writes
// none. Replaying the chain rebuilds the list in the table.
func TestTransitionTriggersOfEveryTalksToWriterReplayToTheTables(t *testing.T) {
	f := newTransitionFixture(t)
	ctx := context.Background()
	userCtx := transition.WithActor(ctx, transition.User(f.userID))
	mcp, detect := f.talksToServices()

	_, added, err := f.agentSvc.AddMCPServers(userCtx, f.agentID, []string{"memory", "github"})
	require.NoError(t, err)
	assert.Equal(t, []string{"memory", "github"}, added)
	_, added, err = f.agentSvc.AddMCPServers(userCtx, f.agentID, []string{"memory"})
	require.NoError(t, err)
	assert.Empty(t, added, "an entry the list holds is not added again")

	updated, err := f.agentSvc.UpdateAgent(userCtx, f.agentID,
		&CreateAgentRequest{DisplayName: "renamed agent", TalksTo: []string{"filesystem", "memory"}}, f.userID)
	require.NoError(t, err)
	assert.Equal(t, []string{"filesystem", "memory"}, updated.TalksTo)
	_, err = f.agentSvc.UpdateAgent(userCtx, f.agentID,
		&CreateAgentRequest{TalksTo: []string{"memory", "filesystem", "memory"}}, f.userID)
	require.NoError(t, err, "the same set in another order")

	_, err = f.agentSvc.RemoveMCPServer(userCtx, f.agentID, "filesystem")
	require.NoError(t, err)
	_, err = f.agentSvc.RemoveMCPServer(userCtx, f.agentID, "not-listed")
	require.NoError(t, err)

	mcp.updateAgentTalksTo(ctx, f.agentID, "slack", transition.User(f.userID))
	mcp.updateAgentTalksTo(ctx, f.agentID, "slack", transition.User(f.userID))

	resp, err := detect.ReportDetections(ctx, f.agentID, f.orgID, detectionReport("memory", "postgres", "redis"))
	require.NoError(t, err)
	assert.Equal(t, []string{"postgres", "redis"}, resp.NewMCPs)
	assert.Equal(t, []string{"memory"}, resp.ExistingMCPs)

	got := f.transitions(t)
	require.Len(t, got, 5, "one record per change of the list, none for a call that changed nothing")
	user, agent := "user:"+f.userID.String(), "agent:"+f.agentID.String()
	want := []struct {
		trigger, actor string
		talksTo        []string
	}{
		{"talks_to_added", user, []string{"github", "memory"}},
		{"talks_to_replaced", user, []string{"filesystem", "memory"}},
		{"talks_to_removed", user, []string{"memory"}},
		{"talks_to_added", user, []string{"memory", "slack"}},
		{"detection_reported", agent, []string{"memory", "postgres", "redis", "slack"}},
	}
	for i, w := range want {
		r := got[i]
		assert.Equal(t, w.trigger, r.Trigger, "seq %d", r.Seq)
		assert.Equal(t, w.actor, r.Actor, "seq %d", r.Seq)
		assert.Equal(t, w.talksTo, r.New.Opena2a.TalksTo, "seq %d", r.Seq)
		assert.Equal(t, "verified", r.New.Opena2a.Status, "seq %d: a talks_to change changes nothing else", r.Seq)
		if i == 0 {
			assert.Equal(t, []string{}, r.Previous.Opena2a.TalksTo)
		} else {
			assert.Equal(t, got[i-1].New, r.Previous, "seq %d: previous_state is not the new_state before it", r.Seq)
		}
	}

	replayed, err := transition.CheckReplay(ctx, f.db, f.orgID, f.keys.publicKey())
	require.NoError(t, err)
	assert.Len(t, replayed.Seqs[f.agentID], 6, "the opening state and the five changes")
	assert.Equal(t, []string{"memory", "postgres", "redis", "slack"}, replayed.States[f.agentID].TalksTo)
	var displayName string
	require.NoError(t, f.db.QueryRow(`SELECT display_name FROM agents WHERE id = $1`, f.agentID).Scan(&displayName))
	assert.Equal(t, "renamed agent", displayName, "the descriptive columns are stored with their own statement")
}

// An update answers an empty talks_to list as an empty list, never as null:
// both when the list was empty and stays as it is, and when the update
// empties a list that held entries.
func TestTransitionTriggerOfAnUpdateAnswersAnEmptyTalksToListAsAnEmptyList(t *testing.T) {
	f := newTransitionFixture(t)
	userCtx := transition.WithActor(context.Background(), transition.User(f.userID))
	answered := func(agent *domain.Agent) string {
		t.Helper()
		body, err := json.Marshal(agent)
		require.NoError(t, err)
		return string(body)
	}

	unchanged, err := f.agentSvc.UpdateAgent(userCtx, f.agentID, &CreateAgentRequest{TalksTo: []string{}}, f.userID)
	require.NoError(t, err)
	assert.Equal(t, []string{}, unchanged.TalksTo)
	assert.Contains(t, answered(unchanged), `"talksTo":[]`, "an empty list left as it is")
	assert.Empty(t, f.transitions(t), "a list left as it is has no record")

	_, err = f.agentSvc.UpdateAgent(userCtx, f.agentID, &CreateAgentRequest{TalksTo: []string{"memory"}}, f.userID)
	require.NoError(t, err)
	emptied, err := f.agentSvc.UpdateAgent(userCtx, f.agentID, &CreateAgentRequest{TalksTo: []string{}}, f.userID)
	require.NoError(t, err)
	assert.Equal(t, []string{}, emptied.TalksTo)
	assert.Contains(t, answered(emptied), `"talksTo":[]`, "a list the update emptied")
	assert.Equal(t, []string{}, f.storedTalksTo(t))
	assert.Len(t, f.transitions(t), 2, "one record per change of the list")
}

// A registration's record holds the talks_to list it stored, and a list
// stored as objects is read as its readers read it: an object stands for its
// id, else its name.
func TestTransitionTriggerReadsTheTalksToListAsItsReadersDo(t *testing.T) {
	f := newTransitionFixture(t)
	ctx := context.Background()
	userCtx := transition.WithActor(ctx, transition.User(f.userID))

	req := registration("reg-talks-"+f.agentID.String()[:8], "")
	req.TalksTo = []string{"memory", "github"}
	registered, err := f.registrar(t).CreateAgent(ctx, req, f.orgID, f.userID, nil, nil, "")
	require.NoError(t, err)

	_, err = f.db.Exec(`UPDATE agents SET talks_to = '[{"id": "srv-1", "name": "ignored"}, {"name": "memory"}]'::jsonb WHERE id = $1`, f.agentID)
	require.NoError(t, err)
	_, _, err = f.agentSvc.AddMCPServers(userCtx, f.agentID, []string{"github"})
	require.NoError(t, err)

	got := f.transitions(t)
	require.Len(t, got, 2)
	assert.Equal(t, "registration_baseline", got[0].Trigger)
	assert.Equal(t, registered.ID.String(), got[0].Agent)
	assert.Equal(t, []string{"github", "memory"}, got[0].New.Opena2a.TalksTo)
	assert.Equal(t, "talks_to_added", got[1].Trigger)
	assert.Equal(t, []string{"memory", "srv-1"}, got[1].Previous.Opena2a.TalksTo)
	assert.Equal(t, []string{"github", "memory", "srv-1"}, got[1].New.Opena2a.TalksTo)
	_, err = transition.CheckReplay(ctx, f.db, f.orgID, f.keys.publicKey())
	require.NoError(t, err)
}

// With record writes failing, a removal that leaves the list non-empty
// commits without its record, is counted and writes a SECURITY line, while an
// addition, a replacement, a detection report and a removal that empties the
// list are each refused and leave the list as it was.
func TestTransitionTriggerOfATalksToRemovalDegradesWhileAnEmptyingFailsClosed(t *testing.T) {
	f := newTransitionFixture(t)
	ctx := context.Background()
	userCtx := transition.WithActor(ctx, transition.User(f.userID))
	_, detect := f.talksToServices()

	_, _, err := f.agentSvc.AddMCPServers(userCtx, f.agentID, []string{"a", "b"})
	require.NoError(t, err)
	f.keys.setFail(errors.New("record key unavailable"))

	_, _, err = f.agentSvc.AddMCPServers(userCtx, f.agentID, []string{"c"})
	require.ErrorIs(t, err, transition.ErrRecordUnavailable)
	_, err = f.agentSvc.UpdateAgent(userCtx, f.agentID,
		&CreateAgentRequest{DisplayName: "not stored", TalksTo: []string{"a", "x"}}, f.userID)
	require.ErrorIs(t, err, transition.ErrRecordUnavailable, "a change that removes one entry and adds another")
	_, err = detect.ReportDetections(ctx, f.agentID, f.orgID, detectionReport("z"))
	require.ErrorIs(t, err, transition.ErrRecordUnavailable)
	assert.Equal(t, []string{"a", "b"}, f.storedTalksTo(t))
	var displayName string
	require.NoError(t, f.db.QueryRow(`SELECT display_name FROM agents WHERE id = $1`, f.agentID).Scan(&displayName))
	assert.NotEqual(t, "not stored", displayName, "a refused update stored its descriptive columns")

	_, err = f.agentSvc.RemoveMCPServer(userCtx, f.agentID, "a")
	require.NoError(t, err)
	assert.Equal(t, []string{"b"}, f.storedTalksTo(t))
	_, err = f.agentSvc.RemoveMCPServer(userCtx, f.agentID, "b")
	require.ErrorIs(t, err, transition.ErrRecordUnavailable, "a list emptied is read as unrestricted")
	assert.Equal(t, []string{"b"}, f.storedTalksTo(t))

	assert.Equal(t, float64(4), f.failures(t, store.ClassExpansion, store.ReasonSigner))
	assert.Equal(t, float64(1), f.failures(t, store.ClassReduction, store.ReasonSigner))
	require.Len(t, f.transitions(t), 1, "only the addition made before the fault has a record")
	var lines []string
	for _, line := range strings.Split(f.logs.String(), "\n") {
		if strings.HasPrefix(line, "SECURITY "+transition.EventTransitionUnrecorded) {
			lines = append(lines, line)
		}
	}
	require.Len(t, lines, 1)
	assert.Contains(t, lines[0], "trigger=talks_to_removed")
	assert.Contains(t, lines[0], "subject="+f.agentID.String())
	assert.NotContains(t, lines[0], f.userID.String(), "the line names the user who acted")
}

// The recorder holds each change to its trigger's rule under the agent's row
// lock: a talks_to change whose locked list gives another class, or which
// also changes the status, is refused, and so is a suspension whose
// statement also changes the list. Nothing they ran is kept.
func TestTransitionTriggerRefusesATalksToChangeOutsideItsRule(t *testing.T) {
	f := newTransitionFixture(t)
	ctx := context.Background()
	userCtx := transition.WithActor(ctx, transition.User(f.userID))
	_, _, err := f.agentSvc.AddMCPServers(userCtx, f.agentID, []string{"a", "b"})
	require.NoError(t, err)
	before, err := transition.CurrentState(ctx, f.db, f.orgID, f.agentID)
	require.NoError(t, err)

	set := func(entries string) func(context.Context, *sql.Tx) error {
		return func(ctx context.Context, tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, `UPDATE agents SET talks_to = $2::jsonb WHERE id = $1`, f.agentID, entries)
			return err
		}
	}
	trace, err := transition.NewTraceID()
	require.NoError(t, err)
	for name, c := range map[string]transition.Change{
		"a removal that adds": {Trigger: transition.TriggerTalksToRemoved, Class: store.ClassReduction,
			Apply: set(`["a", "b", "c"]`)},
		"a removal that empties the list": {Trigger: transition.TriggerTalksToRemoved, Class: store.ClassReduction,
			Apply: set(`[]`)},
		"a removal that also suspends": {Trigger: transition.TriggerTalksToRemoved, Class: store.ClassReduction,
			Apply: func(ctx context.Context, tx *sql.Tx) error {
				if err := set(`["a"]`)(ctx, tx); err != nil {
					return err
				}
				_, err := tx.ExecContext(ctx, `UPDATE agents SET status = 'suspended' WHERE id = $1`, f.agentID)
				return err
			}},
		"a suspension that adds an entry": {Trigger: transition.TriggerAgentSuspended,
			Apply: func(ctx context.Context, tx *sql.Tx) error {
				if err := set(`["a", "b", "c"]`)(ctx, tx); err != nil {
					return err
				}
				_, err := tx.ExecContext(ctx, `UPDATE agents SET status = 'suspended' WHERE id = $1`, f.agentID)
				return err
			}},
	} {
		c.OrganizationID, c.AgentID, c.Actor, c.TraceID = f.orgID, f.agentID, transition.User(f.userID), trace
		_, err := f.rec.Record(ctx, c)
		require.ErrorIs(t, err, transition.ErrInvalidChange, name)
	}
	_, err = f.rec.Record(ctx, transition.Change{
		OrganizationID: f.orgID, AgentID: f.agentID, Actor: transition.User(f.userID), TraceID: trace,
		Trigger: transition.TriggerTalksToAdded, Class: store.ClassExpansion, Apply: set(`["b", "a", "a"]`),
	})
	require.ErrorIs(t, err, transition.ErrNoChange)

	after, err := transition.CurrentState(ctx, f.db, f.orgID, f.agentID)
	require.NoError(t, err)
	assert.True(t, transition.Equal(before, after), "a refused change was kept")
	require.Len(t, f.transitions(t), 1)
}
