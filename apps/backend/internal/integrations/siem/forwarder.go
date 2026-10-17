package siem

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// Event severities, lowest first.
const (
	SeverityInfo     = "info"
	SeverityLow      = "low"
	SeverityMedium   = "medium"
	SeverityHigh     = "high"
	SeverityCritical = "critical"
)

var severityRank = map[string]int{
	SeverityInfo:     0,
	SeverityLow:      1,
	SeverityMedium:   2,
	SeverityHigh:     3,
	SeverityCritical: 4,
}

// ValidSeverity reports whether s is one of the five severities.
func ValidSeverity(s string) bool {
	_, ok := severityRank[s]
	return ok
}

// ForwarderConfig bounds the Forwarder's buffer, batches and retries. A zero
// field takes the default named beside it.
type ForwarderConfig struct {
	MinSeverity   string        // events below it are not sent (default: info, so every event is)
	QueueSize     int           // events held while waiting for delivery (default: 1000)
	BatchSize     int           // events per request (default: 100)
	FlushInterval time.Duration // longest an event waits for its batch to fill (default: 5s)
	MaxAttempts   int           // requests per batch per adapter, the first included (default: 3)
	RetryBackoff  time.Duration // wait before the second request, doubled before each later one (default: 1s)
}

func (c ForwarderConfig) withDefaults() ForwarderConfig {
	if c.MinSeverity == "" {
		c.MinSeverity = SeverityInfo
	}
	if c.QueueSize <= 0 {
		c.QueueSize = 1000
	}
	if c.BatchSize <= 0 {
		c.BatchSize = 100
	}
	if c.FlushInterval <= 0 {
		c.FlushInterval = 5 * time.Second
	}
	if c.MaxAttempts <= 0 {
		c.MaxAttempts = 3
	}
	if c.RetryBackoff <= 0 {
		c.RetryBackoff = time.Second
	}
	return c
}

// ForwarderStats counts what became of the events given to Enqueue. Sent and
// Failed count once per adapter: an event two adapters accept is two Sent.
type ForwarderStats struct {
	Sent     int64 // accepted by an adapter
	Failed   int64 // not accepted by an adapter after every attempt
	Dropped  int64 // not queued: the queue was full or the Forwarder was closing
	Filtered int64 // below MinSeverity
}

// AdapterStats counts one adapter's deliveries, in events.
type AdapterStats struct {
	Adapter string // the adapter's Name
	Sent    int64  // accepted by the adapter
	Failed  int64  // not accepted by the adapter after every attempt
}

// Health is the Forwarder's record of its own deliveries. It is read from
// the deliveries the Forwarder made; reading it sends no request.
type Health struct {
	// Failing is true when the most recent delivery to an adapter was not
	// accepted after every attempt, or when an event was dropped after the
	// last delivery an adapter accepted. It is false before the first
	// delivery and drop.
	Failing bool
	// LastDelivery is when the most recent delivery to an adapter ended,
	// accepted or not; zero before the first.
	LastDelivery time.Time
	// LastDeliveryFailed is whether that delivery was not accepted.
	LastDeliveryFailed bool
	// LastDrop is when the most recent event was dropped; zero when none was.
	LastDrop time.Time
}

// Forwarder delivers events to SIEM adapters in the background. Enqueue
// never blocks its caller: it queues the event, or counts it as dropped when
// the queue is full. One worker sends the queue in batches, and retries a
// batch an adapter refused with a retryable error. Each adapter is retried on
// its own, so one adapter's failure never sends another a batch twice.
type Forwarder struct {
	adapters []BatchAdapter
	cfg      ForwarderConfig
	minRank  int
	logger   *slog.Logger

	queue     chan *SIEMEvent
	flushes   chan chan struct{}
	closing   chan struct{}
	done      chan struct{}
	closeOnce sync.Once
	runCtx    context.Context
	cancelRun context.CancelFunc

	sent, failed, dropped, filtered atomic.Int64
	// adapterSent and adapterFailed count per adapter, in the order of adapters.
	adapterSent, adapterFailed []atomic.Int64

	healthMu sync.Mutex
	// adapterFailing holds, per adapter, whether its most recent delivery
	// was not accepted.
	adapterFailing     []bool
	lastDelivery       time.Time
	lastDeliveryFailed bool
	lastDrop           time.Time
	// droppedSinceAccepted is set by a drop and cleared by a delivery an
	// adapter accepts.
	droppedSinceAccepted bool
}

// NewForwarder starts a Forwarder that delivers to the adapters. Close stops it.
func NewForwarder(adapters []BatchAdapter, cfg ForwarderConfig, logger *slog.Logger) *Forwarder {
	cfg = cfg.withDefaults()
	if logger == nil {
		logger = slog.Default()
	}
	runCtx, cancelRun := context.WithCancel(context.Background())
	f := &Forwarder{
		adapters:       adapters,
		cfg:            cfg,
		minRank:        severityRank[cfg.MinSeverity],
		logger:         logger,
		queue:          make(chan *SIEMEvent, cfg.QueueSize),
		flushes:        make(chan chan struct{}),
		closing:        make(chan struct{}),
		done:           make(chan struct{}),
		runCtx:         runCtx,
		cancelRun:      cancelRun,
		adapterSent:    make([]atomic.Int64, len(adapters)),
		adapterFailed:  make([]atomic.Int64, len(adapters)),
		adapterFailing: make([]bool, len(adapters)),
	}
	go f.run()
	return f
}

// Enqueue queues the event for delivery and reports whether it was queued.
// An event below MinSeverity is not queued. A severity that is not one of
// the five counts as info.
func (f *Forwarder) Enqueue(event *SIEMEvent) bool {
	if severityRank[event.Severity] < f.minRank {
		f.filtered.Add(1)
		return false
	}
	select {
	case <-f.closing:
		f.drop("closing")
		return false
	default:
	}
	select {
	case f.queue <- event:
		return true
	default:
		f.drop("queue full")
		return false
	}
}

func (f *Forwarder) drop(reason string) {
	n := f.dropped.Add(1)
	f.healthMu.Lock()
	f.lastDrop = time.Now()
	f.droppedSinceAccepted = true
	f.healthMu.Unlock()
	// The first drop and every thousandth after it, so a stalled endpoint is
	// visible without a line per event.
	if n == 1 || n%1000 == 0 {
		f.logger.Warn("SIEM event not queued", "reason", reason, "droppedTotal", n)
	}
}

// Stats returns the counts so far.
func (f *Forwarder) Stats() ForwarderStats {
	return ForwarderStats{
		Sent:     f.sent.Load(),
		Failed:   f.failed.Load(),
		Dropped:  f.dropped.Load(),
		Filtered: f.filtered.Load(),
	}
}

// AdapterStats returns the counts so far of each adapter, in the order the
// adapters were given to NewForwarder.
func (f *Forwarder) AdapterStats() []AdapterStats {
	stats := make([]AdapterStats, len(f.adapters))
	for i, adapter := range f.adapters {
		stats[i] = AdapterStats{
			Adapter: adapter.Name(),
			Sent:    f.adapterSent[i].Load(),
			Failed:  f.adapterFailed[i].Load(),
		}
	}
	return stats
}

// Health returns the Forwarder's record of its deliveries and drops so far.
func (f *Forwarder) Health() Health {
	f.healthMu.Lock()
	defer f.healthMu.Unlock()
	failing := f.droppedSinceAccepted
	for _, adapterFailing := range f.adapterFailing {
		failing = failing || adapterFailing
	}
	return Health{
		Failing:            failing,
		LastDelivery:       f.lastDelivery,
		LastDeliveryFailed: f.lastDeliveryFailed,
		LastDrop:           f.lastDrop,
	}
}

// recordDelivery records the end of a delivery of a batch to adapter i.
func (f *Forwarder) recordDelivery(i int, accepted bool) {
	f.healthMu.Lock()
	defer f.healthMu.Unlock()
	f.adapterFailing[i] = !accepted
	f.lastDelivery = time.Now()
	f.lastDeliveryFailed = !accepted
	if accepted {
		f.droppedSinceAccepted = false
	}
}

// Flush delivers the events queued when it is called and returns once their
// delivery has ended, accepted or not. When ctx ends first it returns ctx's
// error and the delivery goes on. After Close it returns at once.
func (f *Forwarder) Flush(ctx context.Context) error {
	delivered := make(chan struct{})
	select {
	case f.flushes <- delivered:
	case <-f.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case <-delivered:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Close stops taking events and delivers the queued ones. When ctx ends
// first, the request in flight is cancelled, the rest of the queue is not
// sent, and ctx's error is returned. Calling Close again returns at once.
func (f *Forwarder) Close(ctx context.Context) error {
	f.closeOnce.Do(func() { close(f.closing) })
	select {
	case <-f.done:
		f.cancelRun()
		return nil
	case <-ctx.Done():
		f.cancelRun()
		<-f.done
		return ctx.Err()
	}
}

func (f *Forwarder) run() {
	defer close(f.done)
	ticker := time.NewTicker(f.cfg.FlushInterval)
	defer ticker.Stop()

	batch := make([]*SIEMEvent, 0, f.cfg.BatchSize)
	flush := func() {
		if len(batch) > 0 {
			f.deliver(batch)
			batch = batch[:0]
		}
	}

	for {
		select {
		case event := <-f.queue:
			batch = append(batch, event)
			if len(batch) >= f.cfg.BatchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		case delivered := <-f.flushes:
			// The events queued now, and not those queued while they are sent.
			for n := len(f.queue); n > 0; n-- {
				batch = append(batch, <-f.queue)
				if len(batch) >= f.cfg.BatchSize {
					flush()
				}
			}
			flush()
			close(delivered)
		case <-f.closing:
			for {
				if f.runCtx.Err() != nil {
					return
				}
				select {
				case event := <-f.queue:
					batch = append(batch, event)
					if len(batch) >= f.cfg.BatchSize {
						flush()
					}
				default:
					flush()
					return
				}
			}
		}
	}
}

func (f *Forwarder) deliver(batch []*SIEMEvent) {
	for i, adapter := range f.adapters {
		err := f.sendWithRetry(adapter, batch)
		if err != nil {
			f.failed.Add(int64(len(batch)))
			f.adapterFailed[i].Add(int64(len(batch)))
			f.recordDelivery(i, false)
			f.logger.Error("SIEM delivery failed",
				"adapter", adapter.Name(), "events", len(batch), "error", err)
			continue
		}
		f.sent.Add(int64(len(batch)))
		f.adapterSent[i].Add(int64(len(batch)))
		f.recordDelivery(i, true)
	}
}

func (f *Forwarder) sendWithRetry(adapter BatchAdapter, batch []*SIEMEvent) error {
	backoff := f.cfg.RetryBackoff
	var err error
	for attempt := 1; attempt <= f.cfg.MaxAttempts; attempt++ {
		if err = adapter.SendBatch(f.runCtx, batch); err == nil {
			return nil
		}
		var status *StatusError
		if errors.As(err, &status) && !status.Retryable() {
			return err
		}
		if attempt == f.cfg.MaxAttempts {
			break
		}
		select {
		case <-f.runCtx.Done():
			return err
		case <-time.After(backoff):
		}
		backoff *= 2
	}
	return err
}
