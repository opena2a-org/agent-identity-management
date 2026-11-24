package application

import (
	"bytes"
	"context"
	"errors"
	"log"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// The verification route hands the audit entry of each decision to an
// AuditQueue, which writes the entries in batches after the response. These
// tests hold the queue to its promise: every entry it is given is stored, or,
// when the store refuses it for good, written to the server log.

// queueStore is an audit store that keeps its rows in memory, as the
// repository does: a batch is all or nothing and a row whose id is stored is
// skipped.
type queueStore struct {
	domain.AuditLogRepository

	mu      sync.Mutex
	rows    map[uuid.UUID]*domain.AuditLog
	batches []int
	calls   int
	// failWrites fails that many write calls before any succeeds.
	failWrites int
	// refuse fails every write that holds one of these ids.
	refuse map[uuid.UUID]bool
	// gate, when set, holds every write until it is closed; entered is told
	// of each write that reached it.
	gate    chan struct{}
	entered chan struct{}
}

func newQueueStore() *queueStore {
	return &queueStore{rows: map[uuid.UUID]*domain.AuditLog{}, refuse: map[uuid.UUID]bool{}}
}

func (s *queueStore) CreateBatch(logs []*domain.AuditLog) error {
	if s.entered != nil {
		s.entered <- struct{}{}
	}
	if s.gate != nil {
		<-s.gate
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	if s.failWrites > 0 {
		s.failWrites--
		return errors.New("connection reset")
	}
	for _, entry := range logs {
		if s.refuse[entry.ID] {
			return errors.New("violates foreign key constraint")
		}
	}
	for _, entry := range logs {
		if _, ok := s.rows[entry.ID]; !ok {
			s.rows[entry.ID] = entry
		}
	}
	s.batches = append(s.batches, len(logs))
	return nil
}

func (s *queueStore) Create(entry *domain.AuditLog) error {
	return s.CreateBatch([]*domain.AuditLog{entry})
}

func (s *queueStore) stored() map[uuid.UUID]*domain.AuditLog {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[uuid.UUID]*domain.AuditLog, len(s.rows))
	for id, row := range s.rows {
		out[id] = row
	}
	return out
}

func (s *queueStore) batchSizes() []int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]int(nil), s.batches...)
}

// createOnlyStore is a store with no batch write.
type createOnlyStore struct {
	domain.AuditLogRepository
	store *queueStore
}

func (s createOnlyStore) Create(entry *domain.AuditLog) error { return s.store.Create(entry) }

func decisionEntry(n int) *domain.AuditLog {
	agentID := uuid.New()
	return &domain.AuditLog{
		OrganizationID: uuid.New(),
		AgentID:        &agentID,
		Action:         "db:write",
		ResourceType:   "agent_action",
		ResourceID:     agentID,
		Metadata:       map[string]interface{}{"n": n},
	}
}

func closeQueue(t *testing.T, q *AuditQueue) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return q.Close(ctx)
}

func captureAuditQueueLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf syncBuffer
	prev := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(prev) })
	return &buf.buf
}

// syncBuffer is a log destination the writer goroutine and the test can share.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func TestAuditQueue_LogReturnsBeforeTheStoreWritesAndCloseStoresEveryEntry(t *testing.T) {
	store := newQueueStore()
	store.gate = make(chan struct{})
	q := NewAuditQueue(store, AuditQueueConfig{MaxBatch: 50, FlushInterval: time.Hour})

	ids := make([]uuid.UUID, 0, 120)
	for i := 0; i < 120; i++ {
		entry := decisionEntry(i)
		require.NoError(t, q.Log(context.Background(), entry))
		require.NotEqual(t, uuid.Nil, entry.ID, "Log gives the entry its id, so an alert can name it before the row exists")
		require.False(t, entry.Timestamp.IsZero(), "Log stamps the entry with the time of the decision")
		ids = append(ids, entry.ID)
	}
	// Every Log returned while the store held every write.
	assert.Empty(t, store.stored())

	close(store.gate)
	require.NoError(t, closeQueue(t, q))

	rows := store.stored()
	require.Len(t, rows, len(ids))
	for _, id := range ids {
		assert.Contains(t, rows, id)
	}
	assert.EqualValues(t, len(ids), q.Written())
	assert.Zero(t, q.Lost())
	for _, size := range store.batchSizes() {
		assert.LessOrEqual(t, size, len(ids))
	}
	assert.Less(t, len(store.batchSizes()), len(ids), "the entries are written in batches, not one statement each")
}

func TestAuditQueue_WritesAWaitingEntryOnTheInterval(t *testing.T) {
	store := newQueueStore()
	q := NewAuditQueue(store, AuditQueueConfig{MaxBatch: 1000, FlushInterval: 10 * time.Millisecond})
	t.Cleanup(func() { _ = closeQueue(t, q) })

	for i := 0; i < 3; i++ {
		require.NoError(t, q.Log(context.Background(), decisionEntry(i)))
	}
	require.Eventually(t, func() bool { return len(store.stored()) == 3 }, 5*time.Second, 5*time.Millisecond,
		"entries fewer than MaxBatch are written once FlushInterval passes, without Flush or Close")
}

func TestAuditQueue_WritesAFullBatchWithoutWaitingForTheInterval(t *testing.T) {
	store := newQueueStore()
	q := NewAuditQueue(store, AuditQueueConfig{MaxBatch: 10, FlushInterval: time.Hour})
	t.Cleanup(func() { _ = closeQueue(t, q) })

	for i := 0; i < 10; i++ {
		require.NoError(t, q.Log(context.Background(), decisionEntry(i)))
	}
	require.Eventually(t, func() bool { return len(store.stored()) == 10 }, 5*time.Second, 5*time.Millisecond)
	assert.Equal(t, []int{10}, store.batchSizes())
}

func TestAuditQueue_FlushStoresEveryQueuedEntry(t *testing.T) {
	store := newQueueStore()
	q := NewAuditQueue(store, AuditQueueConfig{MaxBatch: 1000, FlushInterval: time.Hour})
	t.Cleanup(func() { _ = closeQueue(t, q) })

	for i := 0; i < 7; i++ {
		require.NoError(t, q.Log(context.Background(), decisionEntry(i)))
	}
	require.NoError(t, q.Flush(context.Background()))
	assert.Len(t, store.stored(), 7)
	assert.Equal(t, []int{7}, store.batchSizes(), "the queued entries are one batch")
}

func TestAuditQueue_AFailedBatchIsWrittenEntryByEntryAndRetried(t *testing.T) {
	store := newQueueStore()
	// The batch fails, then the first two entries on their own.
	store.failWrites = 3
	q := NewAuditQueue(store, AuditQueueConfig{MaxBatch: 1000, FlushInterval: time.Hour})
	t.Cleanup(func() { _ = closeQueue(t, q) })

	var ids []uuid.UUID
	for i := 0; i < 5; i++ {
		entry := decisionEntry(i)
		require.NoError(t, q.Log(context.Background(), entry))
		ids = append(ids, entry.ID)
	}
	captureAuditQueueLog(t)
	require.NoError(t, q.Flush(context.Background()))

	rows := store.stored()
	require.Len(t, rows, 5, "the entries the store refused once are written on the retry")
	for _, id := range ids {
		assert.Contains(t, rows, id)
	}
	assert.Zero(t, q.Lost())
}

func TestAuditQueue_OneRefusedEntryDoesNotTakeTheBatchWithIt(t *testing.T) {
	store := newQueueStore()
	q := NewAuditQueue(store, AuditQueueConfig{MaxBatch: 1000, FlushInterval: time.Hour, MaxAttempts: 3})

	refused := decisionEntry(0)
	refused.ID = uuid.New()
	store.refuse[refused.ID] = true
	require.NoError(t, q.Log(context.Background(), refused))
	var kept []uuid.UUID
	for i := 1; i < 6; i++ {
		entry := decisionEntry(i)
		require.NoError(t, q.Log(context.Background(), entry))
		kept = append(kept, entry.ID)
	}

	logged := captureAuditQueueLog(t)
	err := closeQueue(t, q)
	require.Error(t, err, "Close reports an entry the store never accepted")

	rows := store.stored()
	require.Len(t, rows, len(kept))
	for _, id := range kept {
		assert.Contains(t, rows, id)
	}
	assert.EqualValues(t, 1, q.Lost())
	assert.Contains(t, logged.String(), refused.ID.String(),
		"the refused entry is written to the server log, its last record")
	assert.Contains(t, logged.String(), `"action":"db:write"`)
}

func TestAuditQueue_AFullQueueWritesOnTheCallersGoroutine(t *testing.T) {
	store := newQueueStore()
	store.gate = make(chan struct{})
	store.entered = make(chan struct{}, 4)
	q := NewAuditQueue(store, AuditQueueConfig{Capacity: 1, MaxBatch: 1, FlushInterval: time.Hour})

	first, second, third := decisionEntry(1), decisionEntry(2), decisionEntry(3)
	require.NoError(t, q.Log(context.Background(), first))
	// The writer has taken the first entry and is held in its write.
	<-store.entered
	// The second waits in the queue, which is then full.
	require.NoError(t, q.Log(context.Background(), second))

	done := make(chan error, 1)
	go func() { done <- q.Log(context.Background(), third) }()
	// The third is written by its caller, which is held in the same store.
	<-store.entered
	select {
	case err := <-done:
		t.Fatalf("Log of the third entry returned (%v) before its write was let through", err)
	default:
	}

	close(store.gate)
	require.NoError(t, <-done)
	require.NoError(t, closeQueue(t, q))
	rows := store.stored()
	assert.Len(t, rows, 3)
	for _, entry := range []*domain.AuditLog{first, second, third} {
		assert.Contains(t, rows, entry.ID)
	}
}

func TestAuditQueue_LogAfterCloseWritesTheEntry(t *testing.T) {
	store := newQueueStore()
	q := NewAuditQueue(store, AuditQueueConfig{})
	require.NoError(t, closeQueue(t, q))

	entry := decisionEntry(0)
	require.NoError(t, q.Log(context.Background(), entry))
	assert.Contains(t, store.stored(), entry.ID)

	refused := decisionEntry(1)
	refused.ID = uuid.New()
	store.refuse[refused.ID] = true
	assert.Error(t, q.Log(context.Background(), refused), "after Close the caller is told its write failed")

	require.NoError(t, closeQueue(t, q), "Close again returns the first Close's result")
	require.NoError(t, q.Flush(context.Background()), "Flush on a closed queue has nothing to do")
}

func TestAuditQueue_CloseRetriesAnEntryUntilTheStoreAcceptsIt(t *testing.T) {
	store := newQueueStore()
	q := NewAuditQueue(store, AuditQueueConfig{MaxBatch: 1000, FlushInterval: 5 * time.Millisecond, MaxAttempts: 50})
	captureAuditQueueLog(t)
	store.mu.Lock()
	store.failWrites = 6
	store.mu.Unlock()
	entry := decisionEntry(0)
	require.NoError(t, q.Log(context.Background(), entry))
	require.NoError(t, closeQueue(t, q))
	assert.Contains(t, store.stored(), entry.ID)
	assert.Zero(t, q.Lost())
}

func TestAuditQueue_CloseRecordsInTheServerLogWhatItCouldNotStoreInTime(t *testing.T) {
	store := newQueueStore()
	store.failWrites = 1 << 30
	q := NewAuditQueue(store, AuditQueueConfig{MaxBatch: 1000, FlushInterval: 5 * time.Millisecond, MaxAttempts: 1 << 30})
	entry := decisionEntry(0)
	require.NoError(t, q.Log(context.Background(), entry))

	logged := captureAuditQueueLog(t)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	require.Error(t, q.Close(ctx))
	<-q.done
	assert.EqualValues(t, 1, q.Lost())
	assert.Contains(t, logged.String(), entry.ID.String())
}

func TestAuditQueue_WithoutABatchWriteEachEntryIsCreated(t *testing.T) {
	store := newQueueStore()
	q := NewAuditQueue(createOnlyStore{store: store}, AuditQueueConfig{MaxBatch: 1000, FlushInterval: time.Hour})
	for i := 0; i < 4; i++ {
		require.NoError(t, q.Log(context.Background(), decisionEntry(i)))
	}
	require.NoError(t, closeQueue(t, q))
	assert.Len(t, store.stored(), 4)
	assert.Equal(t, []int{1, 1, 1, 1}, store.batchSizes())
}

func TestAuditQueue_ConcurrentCallersAndCloseLoseNoEntry(t *testing.T) {
	store := newQueueStore()
	q := NewAuditQueue(store, AuditQueueConfig{Capacity: 16, MaxBatch: 7, FlushInterval: time.Millisecond})

	const callers, each = 20, 50
	ids := make(chan uuid.UUID, callers*each)
	var wg sync.WaitGroup
	for c := 0; c < callers; c++ {
		wg.Add(1)
		go func(c int) {
			defer wg.Done()
			for i := 0; i < each; i++ {
				entry := decisionEntry(c*each + i)
				if err := q.Log(context.Background(), entry); err != nil {
					t.Errorf("Log: %v", err)
					return
				}
				ids <- entry.ID
			}
		}(c)
		// Close while the later callers are still logging: what they log
		// after it is written by Log itself.
		if c == callers/2 {
			go func() { _ = q.Close(context.Background()) }()
		}
	}
	wg.Wait()
	close(ids)
	require.NoError(t, closeQueue(t, q))

	rows := store.stored()
	count := 0
	for id := range ids {
		count++
		assert.Contains(t, rows, id)
	}
	assert.Equal(t, callers*each, count)
	assert.Len(t, rows, callers*each)
}

func TestAuditQueue_WithTheStoreDownARoundCostsAFewWritesAndLosesNothing(t *testing.T) {
	store := newQueueStore()
	store.failWrites = 1 << 30
	q := NewAuditQueue(store, AuditQueueConfig{MaxBatch: 1000, FlushInterval: time.Hour})
	t.Cleanup(func() { _ = closeQueue(t, q) })
	captureAuditQueueLog(t)

	var ids []uuid.UUID
	for i := 0; i < 50; i++ {
		entry := decisionEntry(i)
		require.NoError(t, q.Log(context.Background(), entry))
		ids = append(ids, entry.ID)
	}
	require.NoError(t, q.Flush(context.Background()))
	store.mu.Lock()
	calls := store.calls
	// The store comes back.
	store.failWrites = 0
	store.mu.Unlock()
	// The batch, then three entries on their own, then three retries.
	assert.Equal(t, 1+2*auditQueueFailuresPerRound, calls, "a round stops writing after three failures in a row")
	assert.Empty(t, store.stored())

	require.NoError(t, q.Flush(context.Background()))
	rows := store.stored()
	require.Len(t, rows, len(ids))
	assert.Zero(t, q.Lost())
}
