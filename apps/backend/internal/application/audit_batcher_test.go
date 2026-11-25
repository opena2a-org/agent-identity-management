package application

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// batchAuditRepo records the audit entries written to it, in batches through
// CreateBatch and one at a time through Create. gate, when set, holds every
// write until it is closed; failBatch makes CreateBatch fail.
type batchAuditRepo struct {
	domain.AuditLogRepository

	mu        sync.Mutex
	batches   [][]*domain.AuditLog
	singles   []*domain.AuditLog
	failBatch bool
	gate      chan struct{}
}

func (r *batchAuditRepo) wait() {
	if r.gate != nil {
		<-r.gate
	}
}

func (r *batchAuditRepo) CreateBatch(entries []*domain.AuditLog) error {
	r.wait()
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.failBatch {
		return errors.New("batch insert failed")
	}
	r.batches = append(r.batches, append([]*domain.AuditLog(nil), entries...))
	return nil
}

func (r *batchAuditRepo) Create(entry *domain.AuditLog) error {
	r.wait()
	r.mu.Lock()
	defer r.mu.Unlock()
	r.singles = append(r.singles, entry)
	return nil
}

func (r *batchAuditRepo) written() []*domain.AuditLog {
	r.mu.Lock()
	defer r.mu.Unlock()
	all := make([]*domain.AuditLog, 0)
	for _, b := range r.batches {
		all = append(all, b...)
	}
	return append(all, r.singles...)
}

func (r *batchAuditRepo) batchSizes() []int {
	r.mu.Lock()
	defer r.mu.Unlock()
	sizes := make([]int, 0, len(r.batches))
	for _, b := range r.batches {
		sizes = append(sizes, len(b))
	}
	return sizes
}

// singleAuditRepo writes one entry at a time only: it has no CreateBatch.
type singleAuditRepo struct {
	domain.AuditLogRepository
	mu      sync.Mutex
	written []*domain.AuditLog
}

func (r *singleAuditRepo) Create(entry *domain.AuditLog) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.written = append(r.written, entry)
	return nil
}

func verifyEntry() *domain.AuditLog {
	return &domain.AuditLog{
		OrganizationID: uuid.New(),
		Action:         domain.AuditActionVerify,
		ResourceType:   "agent_capability",
		ResourceID:     uuid.New(),
	}
}

func TestAuditBatcher_WritesFullBatchesInOneWrite(t *testing.T) {
	repo := &batchAuditRepo{}
	b := NewAuditBatcher(repo, AuditBatcherConfig{BatchSize: 5, FlushInterval: time.Hour})

	for i := 0; i < 15; i++ {
		require.NoError(t, b.Enqueue(verifyEntry()))
	}
	require.Eventually(t, func() bool { return len(repo.written()) == 15 }, 5*time.Second, time.Millisecond)

	assert.Equal(t, []int{5, 5, 5}, repo.batchSizes())
	assert.Empty(t, repo.singles, "no entry is written on its own")
	require.True(t, b.Close(5*time.Second))
}

func TestAuditBatcher_WritesAPartBatchAtTheFlushInterval(t *testing.T) {
	repo := &batchAuditRepo{}
	b := NewAuditBatcher(repo, AuditBatcherConfig{BatchSize: 100, FlushInterval: 10 * time.Millisecond})
	t.Cleanup(func() { b.Close(5 * time.Second) })

	for i := 0; i < 3; i++ {
		require.NoError(t, b.Enqueue(verifyEntry()))
	}
	require.Eventually(t, func() bool { return len(repo.written()) == 3 }, 5*time.Second, time.Millisecond)
	assert.Equal(t, []int{3}, repo.batchSizes())
}

func TestAuditBatcher_EnqueueDoesNotWaitOnTheInsert(t *testing.T) {
	repo := &batchAuditRepo{gate: make(chan struct{})}
	b := NewAuditBatcher(repo, AuditBatcherConfig{BatchSize: 1, FlushInterval: time.Hour, QueueSize: 10})

	// The worker takes the first entry and is held in its write; the others
	// wait on the queue. None of the calls waits for a write.
	returned := make(chan struct{})
	go func() {
		for i := 0; i < 5; i++ {
			_ = b.Enqueue(verifyEntry())
		}
		close(returned)
	}()
	select {
	case <-returned:
	case <-time.After(5 * time.Second):
		t.Fatal("Enqueue waited on the audit insert")
	}
	assert.Empty(t, repo.written(), "no write has finished")

	close(repo.gate)
	require.True(t, b.Close(5*time.Second))
	assert.Len(t, repo.written(), 5)
}

func TestAuditBatcher_CloseWritesEveryQueuedEntry(t *testing.T) {
	repo := &batchAuditRepo{}
	b := NewAuditBatcher(repo, AuditBatcherConfig{BatchSize: 50, FlushInterval: time.Hour})

	ids := map[uuid.UUID]bool{}
	for i := 0; i < 120; i++ {
		e := verifyEntry()
		require.NoError(t, b.Enqueue(e))
		ids[e.ID] = true
	}
	require.True(t, b.Close(5*time.Second))

	written := repo.written()
	require.Len(t, written, 120)
	for _, e := range written {
		assert.True(t, ids[e.ID])
	}
	assert.True(t, b.Close(time.Second), "Close may be called again")
}

func TestAuditBatcher_EnqueueAfterCloseWritesTheEntryItself(t *testing.T) {
	repo := &batchAuditRepo{}
	b := NewAuditBatcher(repo, AuditBatcherConfig{})
	require.True(t, b.Close(5*time.Second))

	e := verifyEntry()
	require.NoError(t, b.Enqueue(e))
	require.Len(t, repo.singles, 1)
	assert.Same(t, e, repo.singles[0])
}

func TestAuditBatcher_FullQueueWritesTheEntryRatherThanLoseIt(t *testing.T) {
	repo := &batchAuditRepo{gate: make(chan struct{})}
	b := NewAuditBatcher(repo, AuditBatcherConfig{BatchSize: 1, FlushInterval: time.Hour, QueueSize: 1})

	// The worker holds one entry in a write and the queue holds one more; a
	// third is written by Enqueue itself, which waits for the gate.
	require.NoError(t, b.Enqueue(verifyEntry()))
	require.Eventually(t, func() bool { return b.Pending() == 0 }, 5*time.Second, time.Millisecond)
	require.NoError(t, b.Enqueue(verifyEntry()))

	third := make(chan error, 1)
	go func() { third <- b.Enqueue(verifyEntry()) }()
	select {
	case <-third:
		t.Fatal("Enqueue returned before writing the entry the full queue had no room for")
	case <-time.After(50 * time.Millisecond):
	}
	close(repo.gate)
	require.NoError(t, <-third)

	require.True(t, b.Close(5*time.Second))
	assert.Len(t, repo.written(), 3)
	assert.Len(t, repo.singles, 1, "the third entry was written by Enqueue")
}

func TestAuditBatcher_FailedBatchIsWrittenOneEntryAtATime(t *testing.T) {
	repo := &batchAuditRepo{failBatch: true}
	b := NewAuditBatcher(repo, AuditBatcherConfig{BatchSize: 4, FlushInterval: time.Hour})
	b.logf = func(string, ...interface{}) {}

	for i := 0; i < 4; i++ {
		require.NoError(t, b.Enqueue(verifyEntry()))
	}
	require.True(t, b.Close(5*time.Second))

	assert.Empty(t, repo.batches)
	assert.Len(t, repo.singles, 4)
}

func TestAuditBatcher_RepositoryWithoutBatchWritesOneAtATime(t *testing.T) {
	repo := &singleAuditRepo{}
	b := NewAuditBatcher(repo, AuditBatcherConfig{BatchSize: 3, FlushInterval: time.Hour})

	for i := 0; i < 7; i++ {
		require.NoError(t, b.Enqueue(verifyEntry()))
	}
	require.True(t, b.Close(5*time.Second))
	assert.Len(t, repo.written, 7)
}

func TestAuditBatcher_EntryKeepsTheTimeOfTheDecision(t *testing.T) {
	repo := &batchAuditRepo{}
	b := NewAuditBatcher(repo, AuditBatcherConfig{FlushInterval: time.Hour})

	before := time.Now()
	e := verifyEntry()
	require.NoError(t, b.Enqueue(e))
	assert.NotEqual(t, uuid.Nil, e.ID)
	assert.False(t, e.Timestamp.Before(before))
	stamped := e.Timestamp

	require.True(t, b.Close(5*time.Second))
	assert.Equal(t, stamped, repo.written()[0].Timestamp)
}

func TestAuditBatcher_NoDecisionIsLostUnderConcurrentEnqueue(t *testing.T) {
	repo := &batchAuditRepo{}
	b := NewAuditBatcher(repo, AuditBatcherConfig{BatchSize: 7, FlushInterval: time.Millisecond, QueueSize: 16})

	const workers, each = 8, 200
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < each; i++ {
				assert.NoError(t, b.Enqueue(verifyEntry()))
			}
		}()
	}
	// Close while entries are still arriving: those after it are written
	// by Enqueue itself.
	go b.Close(5 * time.Second)
	wg.Wait()
	require.True(t, b.Close(5*time.Second))

	seen := map[uuid.UUID]bool{}
	for _, e := range repo.written() {
		assert.False(t, seen[e.ID], "entry %s written twice", e.ID)
		seen[e.ID] = true
	}
	assert.Len(t, seen, workers*each)
}

func TestAuditBatcher_BatchSizeIsCapped(t *testing.T) {
	cfg := AuditBatcherConfig{BatchSize: MaxAuditBatchSize * 2}.withDefaults()
	assert.Equal(t, MaxAuditBatchSize, cfg.BatchSize)
	assert.Equal(t, DefaultAuditFlushInterval, cfg.FlushInterval)
	assert.Equal(t, DefaultAuditQueueSize, cfg.QueueSize)
}

func TestAuditServiceLogVerification_QueuesOnTheBatcher(t *testing.T) {
	repo := &batchAuditRepo{gate: make(chan struct{})}
	svc := NewAuditService(repo)
	svc.SetBatcher(NewAuditBatcher(repo, AuditBatcherConfig{BatchSize: 10, FlushInterval: time.Hour}))

	require.NoError(t, svc.LogVerification(context.Background(), verifyEntry()))
	assert.Empty(t, repo.written(), "LogVerification returned before the insert")

	close(repo.gate)
	require.True(t, svc.CloseBatcher(5*time.Second))
	require.Len(t, repo.batches, 1)
	assert.Len(t, repo.batches[0], 1)
}

func TestAuditServiceLogVerification_WithoutBatcherWritesAtOnce(t *testing.T) {
	repo := &batchAuditRepo{}
	svc := NewAuditService(repo)

	require.NoError(t, svc.LogVerification(context.Background(), verifyEntry()))
	assert.Len(t, repo.singles, 1)
	assert.True(t, svc.CloseBatcher(time.Second))
}
