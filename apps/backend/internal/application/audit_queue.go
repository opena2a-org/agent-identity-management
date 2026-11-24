package application

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// AuditBatchCreator is an audit log store that writes several entries with
// one statement. A row whose id is already stored is skipped, so an entry
// may be given to it again after a write whose outcome was not known.
type AuditBatchCreator interface {
	CreateBatch(logs []*domain.AuditLog) error
}

// AuditQueueConfig sizes an AuditQueue. A zero field takes its default.
type AuditQueueConfig struct {
	// Capacity is how many entries wait for the writer before Log writes
	// on the caller's goroutine instead. Default 4096.
	Capacity int
	// MaxBatch is how many entries the writer gathers before it writes them
	// without waiting for FlushInterval. Default 100.
	MaxBatch int
	// FlushInterval is the longest an entry waits to be written while the
	// store accepts writes. Default 200ms.
	FlushInterval time.Duration
	// MaxRetryDelay caps the wait between the attempts at an entry whose
	// write failed; the wait starts at FlushInterval and doubles. Default 5s.
	MaxRetryDelay time.Duration
	// MaxAttempts is how many times the writer tries an entry before it
	// gives up on the store and writes the entry to the server log as JSON
	// instead. Default 20, about 80 seconds with the default delays.
	MaxAttempts int
}

func (c AuditQueueConfig) withDefaults() AuditQueueConfig {
	if c.Capacity <= 0 {
		c.Capacity = 4096
	}
	if c.MaxBatch <= 0 {
		c.MaxBatch = 100
	}
	if c.FlushInterval <= 0 {
		c.FlushInterval = 200 * time.Millisecond
	}
	if c.MaxRetryDelay <= 0 {
		c.MaxRetryDelay = 5 * time.Second
	}
	if c.MaxRetryDelay < c.FlushInterval {
		c.MaxRetryDelay = c.FlushInterval
	}
	if c.MaxAttempts <= 0 {
		c.MaxAttempts = 20
	}
	return c
}

// AuditQueue takes audit log entries off the request path: Log hands the
// entry to a writer goroutine, which writes the entries it has gathered as
// one batch once MaxBatch are waiting or FlushInterval has passed.
//
// No entry is dropped. When the queue is full, or closed, Log writes the
// entry itself, as AuditService.Log does. When a batch fails, each entry in
// it is written on its own, so one entry the store refuses does not take the
// others with it, and an entry that still fails is tried again with a
// growing delay. Close writes every entry still waiting. An entry the store
// refuses MaxAttempts times, or that is still unwritten when Close's context
// ends, is written to the server log as JSON, and counted by Lost.
type AuditQueue struct {
	repo  domain.AuditLogRepository
	batch AuditBatchCreator
	cfg   AuditQueueConfig

	entries chan *domain.AuditLog
	flushes chan chan struct{}

	// mu orders Log's send against Close: Log sends under the read lock and
	// only while the queue is open, so no entry reaches the channel after the
	// writer has drained it for the last time.
	mu       sync.RWMutex
	closed   bool
	stop     chan struct{}
	stopCtx  context.Context
	done     chan struct{}
	closeErr error

	written atomic.Int64
	lost    atomic.Int64
}

// auditQueueFailuresPerRound is how many writes in a row may fail in one
// round before the writer leaves the rest of the round's entries for the
// next: with the store down, a round costs that many failed writes, not one
// for each entry waiting.
const auditQueueFailuresPerRound = 3

// retryEntry is an entry whose write has failed, and when it is next tried.
type retryEntry struct {
	entry    *domain.AuditLog
	attempts int
	next     time.Time
}

// NewAuditQueue starts the writer of a queue over repo. It writes batches
// with repo's CreateBatch when repo has one, and entry by entry with Create
// otherwise. Close it at shutdown.
func NewAuditQueue(repo domain.AuditLogRepository, cfg AuditQueueConfig) *AuditQueue {
	cfg = cfg.withDefaults()
	q := &AuditQueue{
		repo:    repo,
		cfg:     cfg,
		entries: make(chan *domain.AuditLog, cfg.Capacity),
		flushes: make(chan chan struct{}),
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
	}
	if batch, ok := repo.(AuditBatchCreator); ok {
		q.batch = batch
	}
	go q.run()
	return q
}

// Log queues entry to be written. It gives the entry its id and time now
// when it has none, so the row records when the act happened and the id can
// be referred to before the row is written. The entry's strings are copied:
// a handler passes request header values, which are views of a buffer the
// server reuses once the request ends.
//
// Log returns nil once the entry is queued. When the queue is full or closed
// it writes the entry before returning, and returns that write's error.
func (q *AuditQueue) Log(_ context.Context, entry *domain.AuditLog) error {
	if entry == nil {
		return nil
	}
	if entry.ID == uuid.Nil {
		entry.ID = uuid.New()
	}
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}
	entry.Action = domain.AuditAction(strings.Clone(string(entry.Action)))
	entry.ResourceType = strings.Clone(entry.ResourceType)
	entry.IPAddress = strings.Clone(entry.IPAddress)
	entry.UserAgent = strings.Clone(entry.UserAgent)

	q.mu.RLock()
	if !q.closed {
		select {
		case q.entries <- entry:
			q.mu.RUnlock()
			return nil
		default:
		}
	}
	q.mu.RUnlock()

	// Full or closed: the caller writes the entry, so it is not lost. When
	// the store refuses it, the server log keeps it.
	if err := q.writeOne(entry); err != nil {
		q.recordLost(entry, fmt.Sprintf("written by its caller and refused: %v", err))
		return err
	}
	q.written.Add(1)
	return nil
}

// Flush writes every entry queued before it was called, and tries again
// every entry whose write has failed, and returns once the writer has done
// so or ctx ends.
func (q *AuditQueue) Flush(ctx context.Context) error {
	ack := make(chan struct{})
	select {
	case q.flushes <- ack:
	case <-q.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case <-ack:
		return nil
	case <-q.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Close stops taking entries into the queue, writes every entry still
// waiting, and returns once they are written or ctx ends. An entry Log is
// given after Close is written by Log itself. Close returns an error when an
// entry could not be written; that entry is in the server log. Calling Close
// again returns the first call's result once it is known.
func (q *AuditQueue) Close(ctx context.Context) error {
	if q == nil {
		return nil
	}
	q.mu.Lock()
	first := !q.closed
	if first {
		q.closed = true
		q.stopCtx = ctx
	}
	q.mu.Unlock()
	if first {
		close(q.stop)
	}

	select {
	case <-q.done:
		return q.closeErr
	case <-ctx.Done():
		// The writer is still writing under the first call's context, and
		// ends when that context does.
		return fmt.Errorf("audit queue: entries still being written when the shutdown wait ended: %w", ctx.Err())
	}
}

// Written is the number of entries this queue has written to the store.
func (q *AuditQueue) Written() int64 { return q.written.Load() }

// Lost is the number of entries the store never accepted, each of which was
// written to the server log instead.
func (q *AuditQueue) Lost() int64 { return q.lost.Load() }

// run is the writer goroutine.
func (q *AuditQueue) run() {
	defer close(q.done)

	ticker := time.NewTicker(q.cfg.FlushInterval)
	defer ticker.Stop()

	batch := make([]*domain.AuditLog, 0, q.cfg.MaxBatch)
	retries := make([]retryEntry, 0)

	for {
		select {
		case entry := <-q.entries:
			batch = append(batch, entry)
			if len(batch) >= q.cfg.MaxBatch {
				retries = append(retries, q.writeBatch(batch)...)
				batch = batch[:0]
			}
		case <-ticker.C:
			retries = append(retries, q.writeBatch(batch)...)
			batch = batch[:0]
			retries = q.retry(retries, time.Now(), false)
		case ack := <-q.flushes:
			batch = q.drain(batch)
			retries = append(retries, q.writeBatch(batch)...)
			batch = batch[:0]
			retries = q.retry(retries, time.Now(), true)
			close(ack)
		case <-q.stop:
			q.closeErr = q.finish(q.drain(batch), retries)
			return
		}
	}
}

// drain moves every entry waiting in the channel into batch.
func (q *AuditQueue) drain(batch []*domain.AuditLog) []*domain.AuditLog {
	for {
		select {
		case entry := <-q.entries:
			batch = append(batch, entry)
		default:
			return batch
		}
	}
}

// writeBatch writes entries, as one batch when the store can, and returns the
// entries it could not write, to be tried again.
func (q *AuditQueue) writeBatch(entries []*domain.AuditLog) []retryEntry {
	if len(entries) == 0 {
		return nil
	}
	if q.batch != nil && len(entries) > 1 {
		if err := q.batch.CreateBatch(entries); err == nil {
			q.written.Add(int64(len(entries)))
			return nil
		}
	}

	// The batch failed, or there is no batch write: each entry on its own,
	// so the entries the store accepts are written. After
	// auditQueueFailuresPerRound failures in a row the store is taken to be
	// down, and the entries not yet tried wait for the next round untried.
	failed := make([]retryEntry, 0)
	next := time.Now().Add(q.cfg.FlushInterval)
	inARow := 0
	for _, entry := range entries {
		if inARow >= auditQueueFailuresPerRound {
			failed = append(failed, retryEntry{entry: entry, next: next})
			continue
		}
		if err := q.writeOne(entry); err != nil {
			inARow++
			log.Printf("audit queue: audit entry %s not written, will retry: %v", entry.ID, err)
			failed = append(failed, retryEntry{entry: entry, attempts: 1, next: next})
			continue
		}
		inARow = 0
		q.written.Add(1)
	}
	return q.giveUpOnExhausted(failed)
}

// retry tries again each entry whose delay has passed, or every entry when
// now is set, and returns those still unwritten.
func (q *AuditQueue) retry(retries []retryEntry, at time.Time, now bool) []retryEntry {
	if len(retries) == 0 {
		return retries
	}
	kept := retries[:0]
	inARow := 0
	for _, r := range retries {
		if (!now && at.Before(r.next)) || inARow >= auditQueueFailuresPerRound {
			kept = append(kept, r)
			continue
		}
		if err := q.writeOne(r.entry); err != nil {
			inARow++
			r.attempts++
			r.next = at.Add(q.retryDelay(r.attempts))
			log.Printf("audit queue: audit entry %s not written after %d attempts: %v", r.entry.ID, r.attempts, err)
			kept = append(kept, r)
			continue
		}
		inARow = 0
		q.written.Add(1)
	}
	return q.giveUpOnExhausted(kept)
}

// retryDelay is the wait after the attempts-th failed attempt.
func (q *AuditQueue) retryDelay(attempts int) time.Duration {
	delay := q.cfg.FlushInterval
	for i := 1; i < attempts && delay < q.cfg.MaxRetryDelay; i++ {
		delay *= 2
	}
	if delay > q.cfg.MaxRetryDelay {
		delay = q.cfg.MaxRetryDelay
	}
	return delay
}

// giveUpOnExhausted records in the server log the entries tried MaxAttempts
// times, and returns the others.
func (q *AuditQueue) giveUpOnExhausted(retries []retryEntry) []retryEntry {
	kept := retries[:0]
	for _, r := range retries {
		if r.attempts >= q.cfg.MaxAttempts {
			q.recordLost(r.entry, fmt.Sprintf("refused %d times", r.attempts))
			continue
		}
		kept = append(kept, r)
	}
	return kept
}

// finish writes, at Close, the entries still waiting and the entries waiting
// to be tried again, trying the failed ones until they are written or the
// Close context ends.
func (q *AuditQueue) finish(batch []*domain.AuditLog, retries []retryEntry) error {
	retries = append(retries, q.writeBatch(batch)...)

	ctx := q.stopCtx
	for len(retries) > 0 {
		select {
		case <-ctx.Done():
			for _, r := range retries {
				q.recordLost(r.entry, "unwritten at shutdown")
			}
			return fmt.Errorf("audit queue: %d audit entries not written at shutdown, recorded in the server log instead", len(retries))
		default:
		}
		retries = q.retry(retries, time.Now(), true)
		if len(retries) == 0 {
			break
		}
		select {
		case <-ctx.Done():
		case <-time.After(q.shutdownRetryDelay()):
		}
	}
	if lost := q.lost.Load(); lost > 0 {
		return fmt.Errorf("audit queue: %d audit entries refused by the store, recorded in the server log instead", lost)
	}
	return nil
}

// shutdownRetryDelay is the wait between the attempts Close makes at the
// entries the store refused: short, since shutdown is bounded, and no longer
// than FlushInterval.
func (q *AuditQueue) shutdownRetryDelay() time.Duration {
	const delay = 50 * time.Millisecond
	if q.cfg.FlushInterval < delay {
		return q.cfg.FlushInterval
	}
	return delay
}

// writeOne writes one entry, with the batch write when the store has one, so
// an entry written by a batch whose outcome was not known is not refused as
// a duplicate.
func (q *AuditQueue) writeOne(entry *domain.AuditLog) error {
	if q.batch != nil {
		return q.batch.CreateBatch([]*domain.AuditLog{entry})
	}
	return q.repo.Create(entry)
}

// recordLost writes an entry the store did not accept to the server log, the
// last record of the decision it holds.
func (q *AuditQueue) recordLost(entry *domain.AuditLog, why string) {
	q.lost.Add(1)
	raw, err := json.Marshal(entry)
	if err != nil {
		raw = []byte(fmt.Sprintf("{\"id\":%q,\"organizationId\":%q,\"action\":%q}", entry.ID, entry.OrganizationID, entry.Action))
	}
	log.Printf("ERROR: audit queue: audit entry not stored (%s); its record: %s", why, raw)
}
