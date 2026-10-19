package main

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/config"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// reevaluationFixture is a trustReevaluation over in-memory stand-ins. Agents
// are listed in the order they were added.
type reevaluationFixture struct {
	mu sync.Mutex

	order      []uuid.UUID
	agents     map[uuid.UUID]*domain.Agent
	latest     map[uuid.UUID]*domain.TrustScore
	calculated map[uuid.UUID]float64
	calcErr    map[uuid.UUID]error
	listErr    error
	updateErr  error
	// listing, when set, answers the listing in place of order and listErr.
	listing     func(ctx context.Context) ([]uuid.UUID, error)
	onCalculate func(id uuid.UUID)

	passes       int
	calculations []uuid.UUID
	created      []*domain.TrustScore
	updated      map[uuid.UUID]float64
	notified     []recordedScoreChange
}

// recordedScoreChange is one trust_score.changed the job asked for.
type recordedScoreChange struct {
	agentID           uuid.UUID
	previous, current float64
}

func newReevaluationFixture() *reevaluationFixture {
	return &reevaluationFixture{
		agents:     map[uuid.UUID]*domain.Agent{},
		latest:     map[uuid.UUID]*domain.TrustScore{},
		calculated: map[uuid.UUID]float64{},
		calcErr:    map[uuid.UUID]error{},
		updated:    map[uuid.UUID]float64{},
	}
}

// agent adds a verified agent whose last recorded score is recorded (none when
// negative) and whose score now calculates to calculated.
func (f *reevaluationFixture) agent(recorded, calculated float64) uuid.UUID {
	id := uuid.New()
	f.order = append(f.order, id)
	f.agents[id] = &domain.Agent{ID: id, Status: domain.AgentStatusVerified, TrustScore: recorded}
	if recorded >= 0 {
		f.latest[id] = &domain.TrustScore{AgentID: id, Score: recorded}
	}
	f.calculated[id] = calculated
	return id
}

func (f *reevaluationFixture) job() *trustReevaluation {
	return &trustReevaluation{
		listAgentIDs: func(ctx context.Context) ([]uuid.UUID, error) {
			f.mu.Lock()
			f.passes++
			listing := f.listing
			f.mu.Unlock()
			if listing != nil {
				return listing(ctx)
			}
			return f.order, f.listErr
		},
		agents:     f,
		scores:     f,
		calculator: f,
		webhooks:   f,
	}
}

func (f *reevaluationFixture) passCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.passes
}

func (f *reevaluationFixture) GetByID(id uuid.UUID) (*domain.Agent, error) {
	agent, ok := f.agents[id]
	if !ok {
		return nil, errors.New("agent not found")
	}
	return agent, nil
}

func (f *reevaluationFixture) UpdateTrustScore(id uuid.UUID, newScore float64) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	f.updated[id] = newScore
	return nil
}

func (f *reevaluationFixture) NotifyTrustScoreChanged(agent *domain.Agent, previousScore, newScore float64) {
	f.notified = append(f.notified, recordedScoreChange{agentID: agent.ID, previous: previousScore, current: newScore})
}

func (f *reevaluationFixture) GetLatest(agentID uuid.UUID) (*domain.TrustScore, error) {
	return f.latest[agentID], nil
}

func (f *reevaluationFixture) Create(score *domain.TrustScore) error {
	f.created = append(f.created, score)
	return nil
}

func (f *reevaluationFixture) Calculate(agent *domain.Agent) (*domain.TrustScore, error) {
	f.calculations = append(f.calculations, agent.ID)
	if f.onCalculate != nil {
		f.onCalculate(agent.ID)
	}
	if err := f.calcErr[agent.ID]; err != nil {
		return nil, err
	}
	return &domain.TrustScore{AgentID: agent.ID, Score: f.calculated[agent.ID]}, nil
}

// awaitSignal waits for ch, failing with otherwise after 10 seconds.
func awaitSignal(t *testing.T, ch <-chan struct{}, otherwise string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(10 * time.Second):
		t.Fatal(otherwise)
	}
}

// A score that moved is recorded in both places the recalculate route records
// it, with no request.
func TestTrustReevaluation_RecordsAScoreThatMoved(t *testing.T) {
	f := newReevaluationFixture()
	id := f.agent(0.90, 0.72)

	pass, err := f.job().run(context.Background())
	require.NoError(t, err)

	assert.Equal(t, trustReevaluationPass{Evaluated: 1, Changed: 1}, pass)
	require.Len(t, f.created, 1, "the new score is added to the trust score history")
	assert.Equal(t, id, f.created[0].AgentID)
	assert.Equal(t, 0.72, f.created[0].Score)
	assert.Equal(t, map[uuid.UUID]float64{id: 0.72}, f.updated, "the agent carries the new score")
}

// A recorded score is announced once, from the score the agent carried (one
// set by hand, when an administrator set it) to the score recorded.
func TestTrustReevaluation_AScoreThatMovedSendsTrustScoreChanged(t *testing.T) {
	for _, carried := range []float64{0.90, 0.50} {
		f := newReevaluationFixture()
		id := f.agent(0.90, 0.72)
		f.agents[id].TrustScore = carried

		_, err := f.job().run(context.Background())
		require.NoError(t, err)

		assert.Equal(t, []recordedScoreChange{{agentID: id, previous: carried, current: 0.72}}, f.notified)
	}
}

// A score the agent row could not be given is not announced.
func TestTrustReevaluation_AScoreThatWasNotStoredSendsNothing(t *testing.T) {
	f := newReevaluationFixture()
	f.agent(0.90, 0.72)
	f.updateErr = errors.New("connection reset")

	pass, err := f.job().run(context.Background())
	require.NoError(t, err)

	assert.Equal(t, 1, pass.Failed)
	assert.Empty(t, f.notified)
}

// A pass over agents whose scores have not moved writes nothing: no history
// row, no write to the agent that would make it look recently updated, and no
// trust_score.changed.
func TestTrustReevaluation_LeavesAScoreThatHasNotMovedAlone(t *testing.T) {
	cases := []struct {
		name       string
		recorded   float64
		calculated float64
	}{
		{name: "the same score", recorded: 0.90, calculated: 0.90},
		{name: "up by less than the stored precision", recorded: 0.90, calculated: 0.9004},
		{name: "down by less than the stored precision", recorded: 0.90, calculated: 0.8996},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newReevaluationFixture()
			f.agent(tc.recorded, tc.calculated)

			pass, err := f.job().run(context.Background())
			require.NoError(t, err)

			assert.Equal(t, trustReevaluationPass{Evaluated: 1}, pass)
			assert.Empty(t, f.created)
			assert.Empty(t, f.updated)
			assert.Empty(t, f.notified)
		})
	}
}

func TestTrustReevaluation_RecordsAMoveOfTheStoredPrecisionOrMore(t *testing.T) {
	for _, calculated := range []float64{0.9015, 0.8985} {
		f := newReevaluationFixture()
		id := f.agent(0.90, calculated)

		pass, err := f.job().run(context.Background())
		require.NoError(t, err)

		assert.Equal(t, 1, pass.Changed)
		assert.Equal(t, map[uuid.UUID]float64{id: calculated}, f.updated)
	}
}

func TestTrustReevaluation_RecordsAFirstScore(t *testing.T) {
	f := newReevaluationFixture()
	id := f.agent(-1, 0.80)

	pass, err := f.job().run(context.Background())
	require.NoError(t, err)

	assert.Equal(t, trustReevaluationPass{Evaluated: 1, Changed: 1}, pass)
	require.Len(t, f.created, 1)
	assert.Equal(t, map[uuid.UUID]float64{id: 0.80}, f.updated)
}

// The comparison is with the last calculated score, so a score set by hand on
// the agent is not put back while the calculation stands where it stood.
func TestTrustReevaluation_LeavesAHandSetScoreUntilTheCalculationMoves(t *testing.T) {
	f := newReevaluationFixture()
	id := f.agent(0.90, 0.90)
	f.agents[id].TrustScore = 0.50

	pass, err := f.job().run(context.Background())
	require.NoError(t, err)

	assert.Equal(t, trustReevaluationPass{Evaluated: 1}, pass)
	assert.Empty(t, f.created)
	assert.Empty(t, f.updated)
	assert.Empty(t, f.notified)
}

func TestTrustReevaluation_AnAgentThatFailsDoesNotStopThePass(t *testing.T) {
	f := newReevaluationFixture()
	broken := f.agent(0.90, 0.50)
	f.calcErr[broken] = errors.New("statistics unavailable")
	gone := uuid.New()
	f.order = append(f.order, gone)
	moved := f.agent(0.90, 0.72)

	pass, err := f.job().run(context.Background())
	require.NoError(t, err)

	assert.Equal(t, 1, pass.Evaluated)
	assert.Equal(t, 1, pass.Changed)
	assert.Equal(t, 2, pass.Failed)
	require.Error(t, pass.FirstErr)
	assert.Contains(t, pass.FirstErr.Error(), broken.String(), "the first failure names its agent")
	assert.Contains(t, pass.FirstErr.Error(), "statistics unavailable")
	assert.Equal(t, map[uuid.UUID]float64{moved: 0.72}, f.updated, "a failed agent's score is left as it was")
	assert.Contains(t, pass.String(), "2 failed (first: agent "+broken.String())
}

func TestTrustReevaluation_SkipsAnAgentRevokedSinceTheListing(t *testing.T) {
	f := newReevaluationFixture()
	id := f.agent(0.90, 0.10)
	f.agents[id].Status = domain.AgentStatusRevoked

	pass, err := f.job().run(context.Background())
	require.NoError(t, err)

	assert.Equal(t, trustReevaluationPass{}, pass)
	assert.Empty(t, f.calculations)
	assert.Empty(t, f.updated)
}

func TestTrustReevaluation_AFailedListingEndsThePass(t *testing.T) {
	f := newReevaluationFixture()
	f.agent(0.90, 0.72)
	f.listErr = errors.New("connection refused")

	_, err := f.job().run(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "list agents: connection refused")
	assert.Empty(t, f.calculations)
}

func TestTrustReevaluation_PassSummary(t *testing.T) {
	assert.Equal(t, "re-evaluated 12 agent(s), 3 score(s) changed",
		trustReevaluationPass{Evaluated: 12, Changed: 3}.String())
}

// The listing is every agent in every organization that is not revoked, in one
// query with a total order.
func TestTrustReevaluation_ListsEveryAgentThatIsNotRevoked(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	first, second := uuid.New(), uuid.New()
	mock.ExpectQuery(`SELECT id FROM agents WHERE status <> $1 ORDER BY created_at, id`).
		WithArgs("revoked").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(first.String()).AddRow(second.String()))

	ids, err := reevaluableAgentIDs(db)(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{first, second}, ids)
	require.NoError(t, mock.ExpectationsWereMet())
}

// One pass per tick, and none once the job's context is cancelled.
func TestTrustReevaluation_RunsAPassOnEachTickUntilCancelled(t *testing.T) {
	f := newReevaluationFixture()
	id := f.agent(0.90, 0.72)
	reached := make(chan struct{}, 2)
	f.onCalculate = func(uuid.UUID) { reached <- struct{}{} }
	ticks := make(chan time.Time)
	ctx, cancel := context.WithCancel(context.Background())
	returned := make(chan struct{})
	job := f.job()
	go func() {
		job.loop(ctx, ticks)
		close(returned)
	}()

	// The channel is unbuffered, so a tick is taken only once the pass before it is over.
	ticks <- time.Now()
	ticks <- time.Now()
	awaitSignal(t, reached, "the first pass did not reach the agent")
	awaitSignal(t, reached, "the second pass did not reach the agent")
	cancel()
	awaitSignal(t, returned, "the job did not stop when its context was cancelled")

	assert.Equal(t, 2, f.passCount())
	assert.Equal(t, []uuid.UUID{id, id}, f.calculations)
}

// Cancelling the job ends a pass that is under way before its next agent; the
// agent it was on is recorded in full.
func TestTrustReevaluation_CancellingEndsAPassBetweenAgents(t *testing.T) {
	f := newReevaluationFixture()
	first := f.agent(0.90, 0.72)
	f.agent(0.90, 0.72)
	ticks := make(chan time.Time)
	ctx, cancel := context.WithCancel(context.Background())
	returned := make(chan struct{})
	f.onCalculate = func(uuid.UUID) { cancel() }
	job := f.job()
	go func() {
		job.loop(ctx, ticks)
		close(returned)
	}()

	ticks <- time.Now()
	awaitSignal(t, returned, "the job did not stop when its context was cancelled")

	assert.Equal(t, []uuid.UUID{first}, f.calculations, "the second agent is not reached")
	assert.Equal(t, map[uuid.UUID]float64{first: 0.72}, f.updated, "the agent the pass was on keeps its new score")
}

// A listing the stop cancelled is not logged as a failed pass, whatever error
// the database returns for it.
func TestTrustReevaluation_ACancelledListingIsNotLoggedAsAFailure(t *testing.T) {
	logs := captureLog(t)
	f := newReevaluationFixture()
	ctx, cancel := context.WithCancel(context.Background())
	listed := make(chan struct{})
	f.listing = func(ctx context.Context) ([]uuid.UUID, error) {
		close(listed)
		<-ctx.Done()
		return nil, errors.New("pq: canceling statement due to user request")
	}
	ticks := make(chan time.Time)
	returned := make(chan struct{})
	job := f.job()
	go func() {
		job.loop(ctx, ticks)
		close(returned)
	}()

	ticks <- time.Now()
	awaitSignal(t, listed, "the pass did not list the agents")
	cancel()
	awaitSignal(t, returned, "the job did not stop when its context was cancelled")

	assert.NotContains(t, logs.String(), "Trust re-evaluation failed")
}

// The started job runs passes on its own, at the interval it was given.
// Stopping it returns once its goroutine has exited: the stop line is written
// and no pass starts afterwards. Calling stop again does nothing.
func TestTrustReevaluation_TheStartedJobRunsOnItsIntervalUntilStopped(t *testing.T) {
	logs := captureLog(t)
	f := newReevaluationFixture()
	f.agent(0.90, 0.90)

	stop := startTrustReevaluationJob(f.job(), 5*time.Millisecond)
	require.Eventually(t, func() bool { return f.passCount() >= 2 }, 10*time.Second, 5*time.Millisecond,
		"the job re-evaluates on its interval with no request")
	stop()

	passes := f.passCount()
	time.Sleep(30 * time.Millisecond)
	assert.Equal(t, passes, f.passCount(), "no pass starts after stop returns")
	assert.Contains(t, logs.String(), "Trust re-evaluation job started (runs every 5ms)")
	assert.Contains(t, logs.String(), "Trust re-evaluation: re-evaluated 1 agent(s), 0 score(s) changed")
	assert.Contains(t, logs.String(), "Trust re-evaluation job stopped")
	stop()
}

func TestTrustReevaluationInterval_ReadsTheVariable(t *testing.T) {
	cases := []struct {
		name  string
		value string
		set   bool
		want  time.Duration
		warns bool
	}{
		{name: "unset uses the one-hour default", want: time.Hour},
		{name: "empty uses the one-hour default", value: "", set: true, want: time.Hour},
		{name: "only spaces uses the one-hour default", value: "   ", set: true, want: time.Hour},
		{name: "minutes", value: "30m", set: true, want: 30 * time.Minute},
		{name: "hours", value: "2h", set: true, want: 2 * time.Hour},
		{name: "surrounding spaces are ignored", value: " 15m ", set: true, want: 15 * time.Minute},
		{name: "the floor itself is used as given", value: "5m", set: true, want: 5 * time.Minute},
		{name: "under the floor runs at the floor", value: "1m", set: true, want: 5 * time.Minute, warns: true},
		{name: "zero uses the default", value: "0", set: true, want: time.Hour, warns: true},
		{name: "negative uses the default", value: "-1h", set: true, want: time.Hour, warns: true},
		{name: "a number without a unit uses the default", value: "60", set: true, want: time.Hour, warns: true},
		{name: "text uses the default", value: "hourly", set: true, want: time.Hour, warns: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			vars := map[string]string{}
			if tc.set {
				vars["TRUST_REEVALUATION_INTERVAL"] = tc.value
			}
			got, warning := trustReevaluationInterval(intervalVariables(vars))
			assert.Equal(t, tc.want, got)
			if tc.warns {
				assert.Contains(t, warning, "TRUST_REEVALUATION_INTERVAL", "the warning names the variable to fix")
				assert.Contains(t, warning, got.String(), "the warning states the interval in effect")
			} else {
				assert.Empty(t, warning)
			}
		})
	}
}

// The reader asks for TRUST_REEVALUATION_INTERVAL only, and
// config.JobsConfig holds the job's interval under that name.
func TestTrustReevaluationInterval_ReadsTheNameConfigHolds(t *testing.T) {
	var asked []string
	trustReevaluationInterval(func(name string) string {
		asked = append(asked, name)
		return ""
	})
	assert.Equal(t, []string{"TRUST_REEVALUATION_INTERVAL"}, asked)

	got, warning := trustReevaluationInterval(config.JobsConfig{TrustReevaluationInterval: "30m"}.Value)
	assert.Equal(t, 30*time.Minute, got)
	assert.Empty(t, warning)
}

// main starts the trust re-evaluation job at the interval
// TRUST_REEVALUATION_INTERVAL sets, over the request pool, the agent and trust
// score repositories and the trust calculator, with the agent service sending
// trust_score.changed, and stops it when it returns.
func TestMain_StartsTheTrustReevaluationJob(t *testing.T) {
	start := jobStartCall(t, "trustReevaluationInterval", "startTrustReevaluationJob")
	assert.Equal(t, map[string]string{
		"listAgentIDs": "reevaluableAgentIDs(db)",
		"agents":       "repos.Agent",
		"scores":       "repos.TrustScore",
		"calculator":   "services.Trust",
		"webhooks":     "services.Agent",
	}, jobLiteralFields(t, start.Args[0], "trustReevaluation"))
}
