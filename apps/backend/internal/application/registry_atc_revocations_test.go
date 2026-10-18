package application

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// memoryRegistryATCRevocationStore keeps requests as registry_atc_revocations
// does, in a map, on a clock that only the test moves: Claim takes the due
// requests and holds them, and Delete and Retry act only under the request ID
// a request was claimed with. Two senders given one store are two servers on
// one database.
type memoryRegistryATCRevocationStore struct {
	mu       sync.Mutex
	now      time.Time
	rows     map[uuid.UUID]storedRegistryATCRevocation
	storeErr error
	claimErr error
}

type storedRegistryATCRevocation struct {
	requestID   uuid.UUID
	requestedAt time.Time
	attempts    int
	due         time.Time
}

func newMemoryRegistryATCRevocationStore() *memoryRegistryATCRevocationStore {
	return &memoryRegistryATCRevocationStore{
		now:  time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC),
		rows: make(map[uuid.UUID]storedRegistryATCRevocation),
	}
}

func (m *memoryRegistryATCRevocationStore) Store(_ context.Context, agentID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.storeErr != nil {
		return m.storeErr
	}
	m.rows[agentID] = storedRegistryATCRevocation{requestID: uuid.New(), requestedAt: m.now, due: m.now}
	return nil
}

func (m *memoryRegistryATCRevocationStore) Claim(_ context.Context, limit int, hold time.Duration) ([]domain.RegistryATCRevocation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.claimErr != nil {
		return nil, m.claimErr
	}
	claimed := make([]domain.RegistryATCRevocation, 0)
	for agentID, row := range m.rows {
		if len(claimed) == limit {
			break
		}
		if row.due.After(m.now) {
			continue
		}
		row.attempts++
		row.due = m.now.Add(hold)
		m.rows[agentID] = row
		claimed = append(claimed, domain.RegistryATCRevocation{
			AgentID: agentID, RequestID: row.requestID, RequestedAt: row.requestedAt, Attempts: row.attempts,
		})
	}
	return claimed, nil
}

func (m *memoryRegistryATCRevocationStore) Delete(_ context.Context, r domain.RegistryATCRevocation) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if row, ok := m.rows[r.AgentID]; ok && row.requestID == r.RequestID {
		delete(m.rows, r.AgentID)
	}
	return nil
}

func (m *memoryRegistryATCRevocationStore) Retry(_ context.Context, r domain.RegistryATCRevocation, after time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if row, ok := m.rows[r.AgentID]; ok && row.requestID == r.RequestID {
		row.due = m.now.Add(after)
		m.rows[r.AgentID] = row
	}
	return nil
}

// advance moves the store's clock, as time passes while no request is sent.
func (m *memoryRegistryATCRevocationStore) advance(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.now = m.now.Add(d)
}

// pending returns the request stored for agentID and the store's clock.
func (m *memoryRegistryATCRevocationStore) pending(agentID uuid.UUID) (storedRegistryATCRevocation, time.Time, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	row, ok := m.rows[agentID]
	return row, m.now, ok
}

func (m *memoryRegistryATCRevocationStore) stored(agentID uuid.UUID) bool {
	_, _, ok := m.pending(agentID)
	return ok
}

// testRegistryATCRevocations is a sender for store that looks for due
// requests every few milliseconds, so a test waits on the store's clock and
// not on the poll interval.
func testRegistryATCRevocations(store RegistryATCRevocationStore, revoker RegistryATCRevoker) *RegistryATCRevocations {
	q := NewRegistryATCRevocations(store, revoker)
	q.poll = 5 * time.Millisecond
	return q
}

func waitForRegistryRevocation(t *testing.T, revoker *fakeRegistryATCRevoker) registryATCRevocationCall {
	t.Helper()
	select {
	case call := <-revoker.calls:
		return call
	case <-time.After(registryRevocationWait):
		t.Fatalf("the Registry was not asked to revoke the agent's credentials within %s", registryRevocationWait)
		return registryATCRevocationCall{}
	}
}

// waitForStoredRetry waits until the request for agentID is due after the
// store's clock plus delay, with attempts requests started. A request in
// flight is due after the hold, so delay must differ from the hold.
func waitForStoredRetry(t *testing.T, store *memoryRegistryATCRevocationStore, agentID uuid.UUID, attempts int, delay time.Duration) {
	t.Helper()
	require.NotEqual(t, registryATCRevocationHold, delay, "a retry after the hold cannot be told from a request in flight")
	require.Eventually(t, func() bool {
		row, now, ok := store.pending(agentID)
		return ok && row.attempts == attempts && row.due.Equal(now.Add(delay))
	}, registryRevocationWait, time.Millisecond, "the request was not stored with %d attempt(s), due %s later", attempts, delay)
}

func waitUntilNotStored(t *testing.T, store *memoryRegistryATCRevocationStore, agentID uuid.UUID) {
	t.Helper()
	require.Eventually(t, func() bool { return !store.stored(agentID) }, registryRevocationWait, time.Millisecond,
		"the request for agent %s is still stored", agentID)
}

func TestRegistryATCRevocationsSendsAStoredRequestAndDeletesIt(t *testing.T) {
	store := newMemoryRegistryATCRevocationStore()
	revoker := newFakeRegistryATCRevoker()
	q := testRegistryATCRevocations(store, revoker)
	stop := q.Start()
	defer stop()
	agentID := uuid.New()

	q.Request(context.Background(), agentID)

	call := waitForRegistryRevocation(t, revoker)
	assert.Equal(t, agentID.String(), call.agentID)
	assert.Equal(t, "revoked", call.reason)
	waitUntilNotStored(t, store, agentID)
}

// The restart this store exists for: the first server's request failed and
// its retry was waiting when the server stopped. The next server, started on
// the same store once the retry has fallen due, sends it.
func TestRegistryATCRevocationsRequestWaitingForARetryWhenTheServerStopsIsSentByTheNextServer(t *testing.T) {
	store := newMemoryRegistryATCRevocationStore()
	firstRegistry := newFakeRegistryATCRevoker(&registry.ATCStatusError{StatusCode: http.StatusServiceUnavailable})
	first := testRegistryATCRevocations(store, firstRegistry)
	stopFirst := first.Start()
	agentID := uuid.New()

	first.Request(context.Background(), agentID)
	waitForRegistryRevocation(t, firstRegistry)
	waitForStoredRetry(t, store, agentID, 1, 5*time.Second)
	stopFirst()
	assert.Empty(t, firstRegistry.calls, "the first server sent the retry before it stopped")

	store.advance(5 * time.Second)
	secondRegistry := newFakeRegistryATCRevoker()
	second := testRegistryATCRevocations(store, secondRegistry)
	stopSecond := second.Start()
	defer stopSecond()

	call := waitForRegistryRevocation(t, secondRegistry)
	assert.Equal(t, agentID.String(), call.agentID)
	assert.Equal(t, "revoked", call.reason)
	waitUntilNotStored(t, store, agentID)
}

// A server that stops while a request is in flight ends it, and the request
// stays stored with its hold: the next server sends it once the hold has
// passed.
func TestRegistryATCRevocationsStopEndsARequestInFlightAndTheNextServerSendsIt(t *testing.T) {
	store := newMemoryRegistryATCRevocationStore()
	inFlight := &contextBoundRegistryATCRevoker{calls: make(chan string, 4)}
	first := testRegistryATCRevocations(store, inFlight)
	stopFirst := first.Start()
	agentID := uuid.New()

	first.Request(context.Background(), agentID)
	select {
	case <-inFlight.calls:
	case <-time.After(registryRevocationWait):
		t.Fatalf("the Registry was not asked to revoke the agent's credentials within %s", registryRevocationWait)
	}
	stopped := make(chan struct{})
	go func() {
		stopFirst()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(registryRevocationWait):
		t.Fatalf("stop had not returned after %s with a request in flight", registryRevocationWait)
	}
	row, now, ok := store.pending(agentID)
	require.True(t, ok, "the request the stop ended is no longer stored")
	assert.Equal(t, 1, row.attempts)
	assert.Equal(t, now.Add(registryATCRevocationHold), row.due, "the ended request is due again once its hold has passed")

	store.advance(registryATCRevocationHold)
	secondRegistry := newFakeRegistryATCRevoker()
	second := testRegistryATCRevocations(store, secondRegistry)
	stopSecond := second.Start()
	defer stopSecond()

	assert.Equal(t, agentID.String(), waitForRegistryRevocation(t, secondRegistry).agentID)
	waitUntilNotStored(t, store, agentID)
}

// contextBoundRegistryATCRevoker answers each request only when its context
// ends, as a Registry that never answers would.
type contextBoundRegistryATCRevoker struct {
	calls chan string
}

func (c *contextBoundRegistryATCRevoker) RevokeATC(ctx context.Context, agentID, _ string) error {
	c.calls <- agentID
	<-ctx.Done()
	return ctx.Err()
}

// The retries follow defaultRegistryATCRevocationRetryDelays on the store's
// clock: each failed request makes the stored one due after the next delay.
func TestRegistryATCRevocationsRetriesOnTheStoredSchedule(t *testing.T) {
	store := newMemoryRegistryATCRevocationStore()
	revoker := newFakeRegistryATCRevoker(
		&registry.ATCStatusError{StatusCode: http.StatusServiceUnavailable},
		errors.New("registry: revocation request failed: connection refused"),
	)
	q := testRegistryATCRevocations(store, revoker)
	stop := q.Start()
	defer stop()
	agentID := uuid.New()

	q.Request(context.Background(), agentID)
	waitForRegistryRevocation(t, revoker)
	waitForStoredRetry(t, store, agentID, 1, 5*time.Second)

	store.advance(5 * time.Second)
	waitForRegistryRevocation(t, revoker)
	waitForStoredRetry(t, store, agentID, 2, 30*time.Second)

	store.advance(30 * time.Second)
	waitForRegistryRevocation(t, revoker)
	waitUntilNotStored(t, store, agentID)
	assert.Empty(t, revoker.calls)
}

// The attempts end where those of a request sent from memory end, and the
// request is then deleted.
func TestRegistryATCRevocationsDeletesARequestWhoseAttemptsEnd(t *testing.T) {
	for name, refusal := range map[string]error{
		"no token":        registry.ErrTokenNotConfigured,
		"forbidden":       &registry.ATCStatusError{StatusCode: http.StatusForbidden},
		"route not found": &registry.ATCStatusError{StatusCode: http.StatusNotFound},
	} {
		t.Run(name, func(t *testing.T) {
			store := newMemoryRegistryATCRevocationStore()
			revoker := newFakeRegistryATCRevoker(refusal)
			q := testRegistryATCRevocations(store, revoker)
			stop := q.Start()
			defer stop()
			agentID := uuid.New()

			q.Request(context.Background(), agentID)

			waitForRegistryRevocation(t, revoker)
			waitUntilNotStored(t, store, agentID)
			assert.Empty(t, revoker.calls, "a refusal that repeats was retried")
		})
	}

	t.Run("after the last retry", func(t *testing.T) {
		store := newMemoryRegistryATCRevocationStore()
		unavailable := &registry.ATCStatusError{StatusCode: http.StatusServiceUnavailable}
		revoker := newFakeRegistryATCRevoker(unavailable, unavailable, unavailable)
		q := testRegistryATCRevocations(store, revoker)
		q.delays = []time.Duration{3 * time.Minute}
		stop := q.Start()
		defer stop()
		agentID := uuid.New()

		q.Request(context.Background(), agentID)
		waitForRegistryRevocation(t, revoker)
		waitForStoredRetry(t, store, agentID, 1, 3*time.Minute)

		store.advance(3 * time.Minute)
		waitForRegistryRevocation(t, revoker)
		waitUntilNotStored(t, store, agentID)
		assert.Empty(t, revoker.calls, "a request was sent after the one that followed the last delay")
	})
}

// Issuance stores the request again when the agent was revoked while the
// Registry was issuing. When that happens while the earlier request is in
// flight, the earlier request's answer does not delete the later one, which
// is sent after it.
func TestRegistryATCRevocationsRequestStoredAgainWhileOneIsInFlightIsSentAfterIt(t *testing.T) {
	store := newMemoryRegistryATCRevocationStore()
	revoker := newFakeRegistryATCRevoker()
	revoker.release = make(chan struct{})
	q := testRegistryATCRevocations(store, revoker)
	stop := q.Start()
	defer stop()
	agentID := uuid.New()

	q.Request(context.Background(), agentID)
	waitForRegistryRevocation(t, revoker)
	earlier, _, ok := store.pending(agentID)
	require.True(t, ok)

	q.Request(context.Background(), agentID)
	later, _, ok := store.pending(agentID)
	require.True(t, ok)
	require.NotEqual(t, earlier.requestID, later.requestID)
	close(revoker.release)

	assert.Equal(t, agentID.String(), waitForRegistryRevocation(t, revoker).agentID,
		"the request stored while another was in flight was not sent")
	waitUntilNotStored(t, store, agentID)
	assert.Empty(t, revoker.calls)
}

// A request the store refuses is still sent, from memory.
func TestRegistryATCRevocationsSendsFromMemoryARequestTheStoreRefuses(t *testing.T) {
	store := newMemoryRegistryATCRevocationStore()
	store.storeErr = errors.New("database unavailable")
	revoker := newFakeRegistryATCRevoker()
	q := testRegistryATCRevocations(store, revoker)
	agentID := uuid.New()

	q.Request(context.Background(), agentID)

	assert.Equal(t, agentID.String(), waitForRegistryRevocation(t, revoker).agentID)
	assert.False(t, store.stored(agentID))
}

// The caller's cancellation does not end the statement that stores the
// request: a request whose context is already cancelled is stored.
func TestRegistryATCRevocationsStoresARequestWhoseContextIsCancelled(t *testing.T) {
	store := &contextCheckingRegistryATCRevocationStore{memoryRegistryATCRevocationStore: newMemoryRegistryATCRevocationStore()}
	revoker := newFakeRegistryATCRevoker()
	q := testRegistryATCRevocations(store, revoker)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	agentID := uuid.New()

	q.Request(ctx, agentID)

	assert.True(t, store.stored(agentID), "the request was not stored")
	assertNoRegistryRevocation(t, revoker)
}

// contextCheckingRegistryATCRevocationStore refuses to store on a context
// that has ended, as a database statement does.
type contextCheckingRegistryATCRevocationStore struct {
	*memoryRegistryATCRevocationStore
}

func (c *contextCheckingRegistryATCRevocationStore) Store(ctx context.Context, agentID uuid.UUID) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return c.memoryRegistryATCRevocationStore.Store(ctx, agentID)
}

// A store that cannot be read sends nothing, and the requests are sent once
// it can be read again.
func TestRegistryATCRevocationsSendsOnceTheStoreCanBeReadAgain(t *testing.T) {
	store := newMemoryRegistryATCRevocationStore()
	store.claimErr = errors.New("database unavailable")
	revoker := newFakeRegistryATCRevoker()
	q := testRegistryATCRevocations(store, revoker)
	stop := q.Start()
	defer stop()
	agentID := uuid.New()

	q.Request(context.Background(), agentID)
	assertNoRegistryRevocation(t, revoker)

	store.mu.Lock()
	store.claimErr = nil
	store.mu.Unlock()

	assert.Equal(t, agentID.String(), waitForRegistryRevocation(t, revoker).agentID)
	waitUntilNotStored(t, store, agentID)
}

// RevokeAgent stores its request once the revocation is stored, through the
// store the server wires, and sends nothing from memory.
func TestRevokeAgentStoresTheRegistryRevocationRequest(t *testing.T) {
	s, _, agent := revocableAgentService(t)
	memory := newFakeRegistryATCRevoker()
	s.SetRegistryATCRevoker(memory)
	store := newMemoryRegistryATCRevocationStore()
	s.SetRegistryATCRevocations(NewRegistryATCRevocations(store, newFakeRegistryATCRevoker()))

	require.NoError(t, s.RevokeAgent(context.Background(), agent.ID))

	assert.Equal(t, domain.AgentStatusRevoked, agent.Status)
	assert.True(t, store.stored(agent.ID), "RevokeAgent stored no request for the Registry")
	assertNoRegistryRevocation(t, memory)
}

// Nothing is stored for a revocation AIM refused or did not store.
func TestRevokeAgentStoresNoRegistryRevocationRequestWhenTheRevocationIsNotStored(t *testing.T) {
	t.Run("already revoked", func(t *testing.T) {
		s, _, agent := revocableAgentService(t)
		agent.Status = domain.AgentStatusRevoked
		store := newMemoryRegistryATCRevocationStore()
		s.SetRegistryATCRevocations(NewRegistryATCRevocations(store, newFakeRegistryATCRevoker()))

		require.Error(t, s.RevokeAgent(context.Background(), agent.ID))
		assert.False(t, store.stored(agent.ID))
	})

	t.Run("not stored", func(t *testing.T) {
		repo := new(MockAgentRepository)
		agent := createTestAgentForService()
		repo.On("GetByID", agent.ID).Return(agent, nil)
		repo.On("Update", mock.AnythingOfType("*domain.Agent")).Return(errors.New("database unavailable"))
		s := &AgentService{agentRepo: repo}
		store := newMemoryRegistryATCRevocationStore()
		s.SetRegistryATCRevocations(NewRegistryATCRevocations(store, newFakeRegistryATCRevoker()))

		require.Error(t, s.RevokeAgent(context.Background(), agent.ID))
		assert.False(t, store.stored(agent.ID))
	})
}

// Issuance stores its second request for an agent revoked while the Registry
// was issuing, through the store the server wires.
func TestIssueForAgentStoresTheRevocationRequestForAnAgentRevokedDuringIssuance(t *testing.T) {
	agent := verifiedAgentForIssuance()
	reader := &revocableAgentReader{agent: agent}
	client := &fakeRegistryClient{respond: func(req registry.ATCIssuanceRequest) *registry.AgentTrustCredential {
		reader.revoke()
		return conformingATC(req)
	}}
	svc := NewATCIssuanceService(reader, &fakeCapabilityReader{}, nil, &countingScorer{score: &domain.TrustScore{Score: 0.8}}, client, testATCPublicOrigin)
	memory := newFakeRegistryATCRevoker()
	svc.SetRegistryATCRevoker(memory)
	store := newMemoryRegistryATCRevocationStore()
	svc.SetRegistryATCRevocations(NewRegistryATCRevocations(store, newFakeRegistryATCRevoker()))

	res, err := svc.IssueForAgent(context.Background(), agent.ID)

	require.ErrorIs(t, err, ErrATCAgentRevoked)
	assert.Nil(t, res)
	assert.True(t, store.stored(agent.ID), "issuance stored no request for the Registry")
	assertNoRegistryRevocation(t, memory)
}
