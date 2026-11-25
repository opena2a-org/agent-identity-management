package application

import (
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// The audit batcher takes the audit write off the verification hot path: a
// verification decision puts its audit entry on a queue and returns, and a
// background worker writes the queued entries in batches. The decision itself
// never reads the audit log, so a revocation still takes effect on the next
// verification whatever the queue holds.
//
// No entry is dropped. An entry that finds the queue full, or the batcher
// closed, is written before Enqueue returns. A batch whose write fails is
// written again one entry at a time, and Close writes every entry still
// queued before it returns.

const (
	// DefaultAuditBatchSize is the most entries one batch write holds.
	DefaultAuditBatchSize = 100
	// DefaultAuditFlushInterval is the longest a queued entry waits for a
	// batch to fill before it is written.
	DefaultAuditFlushInterval = 250 * time.Millisecond
	// DefaultAuditQueueSize is how many entries can wait to be written
	// before Enqueue writes an entry itself.
	DefaultAuditQueueSize = 10000
	// MaxAuditBatchSize is the largest batch size the batcher uses; a
	// larger one asked for is lowered to it, which keeps a batch's insert
	// within PostgreSQL's limit on statement parameters.
	MaxAuditBatchSize = 1000
)

// AuditBatchWriter writes a batch of audit entries in one write: either all
// of them are written or none is.
type AuditBatchWriter interface {
	CreateBatch(entries []*domain.AuditLog) error
}

// AuditBatcherConfig sets the batcher's sizes; a zero or negative value takes
// its default.
type AuditBatcherConfig struct {
	BatchSize     int
	FlushInterval time.Duration
	QueueSize     int
}

func (c AuditBatcherConfig) withDefaults() AuditBatcherConfig {
	if c.BatchSize <= 0 {
		c.BatchSize = DefaultAuditBatchSize
	}
	if c.BatchSize > MaxAuditBatchSize {
		c.BatchSize = MaxAuditBatchSize
	}
	if c.FlushInterval <= 0 {
		c.FlushInterval = DefaultAuditFlushInterval
	}
	if c.QueueSize <= 0 {
		c.QueueSize = DefaultAuditQueueSize
	}
	return c
}

// AuditBatcher queues audit entries and writes them in batches.
type AuditBatcher struct {
	repo   domain.AuditLogRepository
	writer AuditBatchWriter // nil when repo cannot write a batch: entries are written one at a time
	cfg    AuditBatcherConfig
	queue  chan *domain.AuditLog
	done   chan struct{}
	logf   func(format string, args ...interface{})

	// mu orders Enqueue's sends against Close closing the queue: a send
	// holds it for reading, Close for writing.
	mu     sync.RWMutex
	closed bool
}

// NewAuditBatcher returns a batcher writing to repo, which writes batches
// with CreateBatch when repo has it. Its worker runs from the start; stop it
// with Close.
func NewAuditBatcher(repo domain.AuditLogRepository, cfg AuditBatcherConfig) *AuditBatcher {
	cfg = cfg.withDefaults()
	b := &AuditBatcher{
		repo:  repo,
		cfg:   cfg,
		queue: make(chan *domain.AuditLog, cfg.QueueSize),
		done:  make(chan struct{}),
		logf:  log.Printf,
	}
	if w, ok := repo.(AuditBatchWriter); ok {
		b.writer = w
	}
	go b.run()
	return b
}

// Enqueue queues entry to be written and returns without waiting for the
// write. The entry is given its id and time now when it has none, so it
// records when the decision was made, not when it was written. When the queue
// is full or the batcher is closed, entry is written before Enqueue returns.
// The caller must not change entry afterwards.
func (b *AuditBatcher) Enqueue(entry *domain.AuditLog) error {
	if entry.ID == uuid.Nil {
		entry.ID = uuid.New()
	}
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	b.mu.RLock()
	if !b.closed {
		select {
		case b.queue <- entry:
			b.mu.RUnlock()
			return nil
		default:
		}
	}
	b.mu.RUnlock()

	// Full or closed: write it here rather than lose it.
	return b.repo.Create(entry)
}

// Pending is the number of entries queued and not yet taken by the worker.
func (b *AuditBatcher) Pending() int {
	return len(b.queue)
}

// Close stops taking entries onto the queue, writes every entry still queued,
// and returns true once they are written. It returns false when they are not
// all written within timeout; the worker goes on writing them. An entry
// enqueued after Close is written by Enqueue itself. Close may be called more
// than once.
func (b *AuditBatcher) Close(timeout time.Duration) bool {
	b.mu.Lock()
	if !b.closed {
		b.closed = true
		close(b.queue)
	}
	b.mu.Unlock()

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-b.done:
		return true
	case <-timer.C:
		return false
	}
}

func (b *AuditBatcher) run() {
	defer close(b.done)
	ticker := time.NewTicker(b.cfg.FlushInterval)
	defer ticker.Stop()

	batch := make([]*domain.AuditLog, 0, b.cfg.BatchSize)
	for {
		select {
		case entry, ok := <-b.queue:
			if !ok {
				b.flush(batch)
				return
			}
			batch = append(batch, entry)
			if len(batch) >= b.cfg.BatchSize {
				b.flush(batch)
				batch = make([]*domain.AuditLog, 0, b.cfg.BatchSize)
			}
		case <-ticker.C:
			if len(batch) > 0 {
				b.flush(batch)
				batch = make([]*domain.AuditLog, 0, b.cfg.BatchSize)
			}
		}
	}
}

// flush writes batch in one write, or one entry at a time when the repository
// cannot write a batch or the batch write fails, so one bad entry does not
// lose the others.
func (b *AuditBatcher) flush(batch []*domain.AuditLog) {
	if len(batch) == 0 {
		return
	}
	if b.writer != nil {
		err := b.writer.CreateBatch(batch)
		if err == nil {
			return
		}
		b.logf("audit batch: writing %d entries in one batch failed, writing them one at a time: %v", len(batch), err)
	}
	for _, entry := range batch {
		if err := b.repo.Create(entry); err != nil {
			b.logf("audit batch: writing audit entry %s (action %s) failed: %v", entry.ID, entry.Action, err)
		}
	}
}
