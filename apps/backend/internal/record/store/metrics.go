package store

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Console line event names. A failed write's line is also its SECURITY line.
const (
	EventRecordWriteFailed = "record_write_failed"
	EventAppendLockWaits   = "record_append_lock_waits"
)

// lockWaitBuckets are the upper bounds, in seconds, of the append-lock wait
// histogram and of the periodic console line. They are fixed so that every
// period's line has the same fields.
var lockWaitBuckets = []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.25, 0.5, 1, 2}

// Metrics are the writer's series. No series carries an organization label.
type Metrics struct {
	failures *prometheus.CounterVec
	lockWait prometheus.Histogram
}

// NewMetrics registers the writer's series with reg, every failure series at
// zero.
func NewMetrics(reg prometheus.Registerer) (*Metrics, error) {
	m := &Metrics{
		failures: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "aim_record_write_failures_total",
			Help: "Record writes that failed, by the class of the write and the reason it failed",
		}, []string{"class", "reason"}),
		lockWait: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "aim_record_append_lock_wait_seconds",
			Help:    "Time a record write waited for its chain's append lock, in process and in the database",
			Buckets: lockWaitBuckets,
		}),
	}
	for _, c := range []prometheus.Collector{m.failures, m.lockWait} {
		if err := reg.Register(c); err != nil {
			return nil, fmt.Errorf("store: register metrics: %w", err)
		}
	}
	for _, class := range classes {
		for _, reason := range reasons {
			m.failures.WithLabelValues(string(class), string(reason))
		}
	}
	return m, nil
}

// lockWaitPeriod counts append-lock waits per bucket since the last console
// line.
type lockWaitPeriod struct {
	mu     sync.Mutex
	since  time.Time
	counts []uint64
}

func (p *lockWaitPeriod) observe(seconds float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.counts == nil {
		p.counts = make([]uint64, len(lockWaitBuckets)+1)
	}
	i := 0
	for i < len(lockWaitBuckets) && seconds > lockWaitBuckets[i] {
		i++
	}
	p.counts[i]++
}

// line returns the period's console line and starts a new period. Each
// bucket's count is cumulative, as in the histogram: le_inf is every wait of
// the period.
func (p *lockWaitPeriod) line(now time.Time) string {
	p.mu.Lock()
	counts := p.counts
	since := p.since
	p.counts = make([]uint64, len(lockWaitBuckets)+1)
	p.since = now
	p.mu.Unlock()
	if counts == nil {
		counts = make([]uint64, len(lockWaitBuckets)+1)
	}

	var b strings.Builder
	b.WriteString(EventAppendLockWaits)
	period := 0.0
	if !since.IsZero() {
		period = now.Sub(since).Seconds()
	}
	fmt.Fprintf(&b, " period_seconds=%s", strconv.FormatFloat(period, 'f', 3, 64))
	var total uint64
	for i, upper := range lockWaitBuckets {
		total += counts[i]
		fmt.Fprintf(&b, " le_%s=%d", strconv.FormatFloat(upper, 'f', -1, 64), total)
	}
	total += counts[len(lockWaitBuckets)]
	fmt.Fprintf(&b, " le_inf=%d", total)
	return b.String()
}

// EmitLockWaits writes the append-lock wait console line of the period that
// ends now, and starts the next period. The line carries no organization.
func (w *Writer) EmitLockWaits() {
	w.log.Print(w.waits.line(w.now()))
}

// ReportLockWaits writes one append-lock wait console line every period
// until ctx ends.
func (w *Writer) ReportLockWaits(ctx context.Context, every time.Duration) {
	w.waits.mu.Lock()
	w.waits.since = w.now()
	w.waits.mu.Unlock()
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			w.EmitLockWaits()
		}
	}
}
