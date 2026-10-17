package siem

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeAdapter records each batch it is given and answers with the next error
// of errs (nil once they run out). When block is set, each call waits on it.
type fakeAdapter struct {
	name    string
	mu      sync.Mutex
	errs    []error
	always  error
	batches [][]string
	entered chan struct{}
	block   chan struct{}
}

func (a *fakeAdapter) Name() string { return a.name }

func (a *fakeAdapter) Send(ctx context.Context, event *SIEMEvent) error {
	return a.SendBatch(ctx, []*SIEMEvent{event})
}

func (a *fakeAdapter) SendBatch(ctx context.Context, events []*SIEMEvent) error {
	types := make([]string, len(events))
	for i, e := range events {
		types[i] = string(e.EventType)
	}
	a.mu.Lock()
	a.batches = append(a.batches, types)
	var err error
	switch {
	case len(a.errs) > 0:
		err, a.errs = a.errs[0], a.errs[1:]
	case a.always != nil:
		err = a.always
	}
	a.mu.Unlock()

	if a.entered != nil {
		a.entered <- struct{}{}
	}
	if a.block != nil {
		select {
		case <-a.block:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return err
}

func (a *fakeAdapter) received() [][]string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([][]string(nil), a.batches...)
}

func event(eventType, severity string) *SIEMEvent {
	return &SIEMEvent{EventType: EventType(eventType), Severity: severity, Timestamp: time.Now()}
}

// testConfig never flushes on the interval, so a test decides when a batch is sent.
func testConfig() ForwarderConfig {
	return ForwarderConfig{BatchSize: 100, FlushInterval: time.Hour, RetryBackoff: time.Millisecond}
}

func closeForwarder(t *testing.T, f *Forwarder) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, f.Close(ctx))
}

func TestForwarder_SendsAFullBatchInOneRequest(t *testing.T) {
	adapter := &fakeAdapter{name: "fake"}
	cfg := testConfig()
	cfg.BatchSize = 3
	f := NewForwarder([]BatchAdapter{adapter}, cfg, nil)

	for _, et := range []string{"a", "b", "c"} {
		require.True(t, f.Enqueue(event(et, SeverityInfo)))
	}
	require.Eventually(t, func() bool { return len(adapter.received()) == 1 }, 5*time.Second, 5*time.Millisecond)
	closeForwarder(t, f)

	assert.Equal(t, [][]string{{"a", "b", "c"}}, adapter.received())
	assert.Equal(t, ForwarderStats{Sent: 3}, f.Stats())
}

func TestForwarder_SendsAPartBatchOnTheFlushInterval(t *testing.T) {
	adapter := &fakeAdapter{name: "fake"}
	cfg := testConfig()
	cfg.FlushInterval = 10 * time.Millisecond
	f := NewForwarder([]BatchAdapter{adapter}, cfg, nil)
	t.Cleanup(func() { closeForwarder(t, f) })

	require.True(t, f.Enqueue(event("a", SeverityInfo)))
	require.Eventually(t, func() bool { return len(adapter.received()) == 1 }, 5*time.Second, 5*time.Millisecond)
	assert.Equal(t, [][]string{{"a"}}, adapter.received())
}

func TestForwarder_CloseSendsWhatIsQueued(t *testing.T) {
	adapter := &fakeAdapter{name: "fake"}
	f := NewForwarder([]BatchAdapter{adapter}, testConfig(), nil)

	require.True(t, f.Enqueue(event("a", SeverityInfo)))
	require.True(t, f.Enqueue(event("b", SeverityInfo)))
	closeForwarder(t, f)

	assert.Equal(t, [][]string{{"a", "b"}}, adapter.received())
	assert.False(t, f.Enqueue(event("c", SeverityInfo)), "a closed forwarder queues nothing")
	assert.Equal(t, ForwarderStats{Sent: 2, Dropped: 1}, f.Stats())
	closeForwarder(t, f) // a second Close returns at once
}

func TestForwarder_RetriesARetryableRefusal(t *testing.T) {
	adapter := &fakeAdapter{name: "fake", errs: []error{
		&StatusError{Service: "fake", StatusCode: 503},
		errors.New("connection reset"),
	}}
	f := NewForwarder([]BatchAdapter{adapter}, testConfig(), nil)

	require.True(t, f.Enqueue(event("a", SeverityInfo)))
	closeForwarder(t, f)

	assert.Equal(t, [][]string{{"a"}, {"a"}, {"a"}}, adapter.received(), "two failures, then the third attempt is accepted")
	assert.Equal(t, ForwarderStats{Sent: 1}, f.Stats())
}

func TestForwarder_GivesUpAfterMaxAttempts(t *testing.T) {
	adapter := &fakeAdapter{name: "fake", always: &StatusError{Service: "fake", StatusCode: 500}}
	cfg := testConfig()
	cfg.MaxAttempts = 2
	f := NewForwarder([]BatchAdapter{adapter}, cfg, nil)

	require.True(t, f.Enqueue(event("a", SeverityInfo)))
	closeForwarder(t, f)

	assert.Len(t, adapter.received(), 2)
	assert.Equal(t, ForwarderStats{Failed: 1}, f.Stats())
}

func TestForwarder_DoesNotRetryARefusalThatRepeats(t *testing.T) {
	adapter := &fakeAdapter{name: "fake", always: &StatusError{Service: "fake", StatusCode: 403}}
	f := NewForwarder([]BatchAdapter{adapter}, testConfig(), nil)

	require.True(t, f.Enqueue(event("a", SeverityInfo)))
	closeForwarder(t, f)

	assert.Len(t, adapter.received(), 1, "a 403 is not retried")
	assert.Equal(t, ForwarderStats{Failed: 1}, f.Stats())
}

func TestForwarder_RetriesEachAdapterOnItsOwn(t *testing.T) {
	failing := &fakeAdapter{name: "failing", always: &StatusError{Service: "failing", StatusCode: 500}}
	healthy := &fakeAdapter{name: "healthy"}
	f := NewForwarder([]BatchAdapter{failing, healthy}, testConfig(), nil)

	require.True(t, f.Enqueue(event("a", SeverityInfo)))
	closeForwarder(t, f)

	assert.Len(t, failing.received(), 3)
	assert.Equal(t, [][]string{{"a"}}, healthy.received(), "the healthy adapter is sent the batch once")
	assert.Equal(t, ForwarderStats{Sent: 1, Failed: 1}, f.Stats())
}

func TestForwarder_LeavesOutEventsBelowMinSeverity(t *testing.T) {
	adapter := &fakeAdapter{name: "fake"}
	cfg := testConfig()
	cfg.MinSeverity = SeverityMedium
	f := NewForwarder([]BatchAdapter{adapter}, cfg, nil)

	assert.False(t, f.Enqueue(event("info", SeverityInfo)))
	assert.False(t, f.Enqueue(event("low", SeverityLow)))
	assert.False(t, f.Enqueue(event("unknown", "verbose")), "an unknown severity counts as info")
	assert.True(t, f.Enqueue(event("medium", SeverityMedium)))
	assert.True(t, f.Enqueue(event("critical", SeverityCritical)))
	closeForwarder(t, f)

	assert.Equal(t, [][]string{{"medium", "critical"}}, adapter.received())
	assert.Equal(t, ForwarderStats{Sent: 2, Filtered: 3}, f.Stats())
}

func TestForwarder_DropsWhenTheQueueIsFull(t *testing.T) {
	adapter := &fakeAdapter{name: "fake", entered: make(chan struct{}, 4), block: make(chan struct{})}
	cfg := testConfig()
	cfg.QueueSize = 1
	cfg.BatchSize = 1
	f := NewForwarder([]BatchAdapter{adapter}, cfg, nil)

	require.True(t, f.Enqueue(event("a", SeverityInfo)))
	<-adapter.entered // the worker holds "a" and waits on the adapter
	require.True(t, f.Enqueue(event("b", SeverityInfo)), "the queue holds one event")
	assert.False(t, f.Enqueue(event("c", SeverityInfo)), "a full queue never blocks the caller")

	close(adapter.block)
	closeForwarder(t, f)

	assert.Equal(t, [][]string{{"a"}, {"b"}}, adapter.received())
	assert.Equal(t, ForwarderStats{Sent: 2, Dropped: 1}, f.Stats())
}

func TestForwarder_CloseStopsAtItsDeadline(t *testing.T) {
	adapter := &fakeAdapter{name: "fake", entered: make(chan struct{}, 4), block: make(chan struct{})}
	cfg := testConfig()
	cfg.MaxAttempts = 1
	f := NewForwarder([]BatchAdapter{adapter}, cfg, nil)

	require.True(t, f.Enqueue(event("a", SeverityInfo)))
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	err := f.Close(ctx)

	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Equal(t, ForwarderStats{Failed: 1}, f.Stats(), "the request in flight is cancelled")
}

func flushForwarder(t *testing.T, f *Forwarder) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, f.Flush(ctx))
}

func TestForwarder_FlushSendsWhatIsQueued(t *testing.T) {
	adapter := &fakeAdapter{name: "fake"}
	f := NewForwarder([]BatchAdapter{adapter}, testConfig(), nil)

	require.True(t, f.Enqueue(event("a", SeverityInfo)))
	require.True(t, f.Enqueue(event("b", SeverityInfo)))
	flushForwarder(t, f)
	assert.Equal(t, [][]string{{"a", "b"}}, adapter.received(), "sent before the hour-long flush interval")

	flushForwarder(t, f)
	assert.Len(t, adapter.received(), 1, "a flush with nothing queued sends nothing")

	closeForwarder(t, f)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	assert.NoError(t, f.Flush(ctx), "a flush after Close returns at once")
}

func TestForwarder_HealthAcrossAcceptRefuseAndRecover(t *testing.T) {
	adapter := &fakeAdapter{name: "fake", errs: []error{
		nil,
		&StatusError{Service: "fake", StatusCode: 403},
	}}
	f := NewForwarder([]BatchAdapter{adapter}, testConfig(), nil)
	t.Cleanup(func() { closeForwarder(t, f) })

	assert.Equal(t, Health{}, f.Health(), "before the first delivery")

	require.True(t, f.Enqueue(event("accepted", SeverityInfo)))
	flushForwarder(t, f)
	health := f.Health()
	assert.False(t, health.Failing)
	assert.False(t, health.LastDeliveryFailed)
	assert.False(t, health.LastDelivery.IsZero())
	accepted := health.LastDelivery

	require.True(t, f.Enqueue(event("refused", SeverityInfo)))
	flushForwarder(t, f)
	health = f.Health()
	assert.True(t, health.Failing, "the most recent delivery was refused")
	assert.True(t, health.LastDeliveryFailed)
	assert.False(t, health.LastDelivery.Before(accepted))

	require.True(t, f.Enqueue(event("recovered", SeverityInfo)))
	flushForwarder(t, f)
	health = f.Health()
	assert.False(t, health.Failing, "a later accepted delivery ends the failing state")
	assert.False(t, health.LastDeliveryFailed)
	assert.True(t, health.LastDrop.IsZero())

	assert.Equal(t, ForwarderStats{Sent: 2, Failed: 1}, f.Stats())
	assert.Equal(t, []AdapterStats{{Adapter: "fake", Sent: 2, Failed: 1}}, f.AdapterStats())
}

func TestForwarder_HealthFailsWhileAnyAdapterFails(t *testing.T) {
	failing := &fakeAdapter{name: "failing", errs: []error{&StatusError{Service: "failing", StatusCode: 403}}}
	healthy := &fakeAdapter{name: "healthy"}
	f := NewForwarder([]BatchAdapter{failing, healthy}, testConfig(), nil)
	t.Cleanup(func() { closeForwarder(t, f) })

	require.True(t, f.Enqueue(event("a", SeverityInfo)))
	flushForwarder(t, f)
	health := f.Health()
	assert.True(t, health.Failing, "the failing adapter's most recent delivery was refused")
	assert.False(t, health.LastDeliveryFailed, "the healthy adapter's delivery ended last")

	require.True(t, f.Enqueue(event("b", SeverityInfo)))
	flushForwarder(t, f)
	assert.False(t, f.Health().Failing, "both adapters accepted their most recent delivery")
	assert.Equal(t, []AdapterStats{
		{Adapter: "failing", Sent: 1, Failed: 1},
		{Adapter: "healthy", Sent: 2},
	}, f.AdapterStats())
}

func TestForwarder_HealthFailsAfterADropUntilAnAcceptedDelivery(t *testing.T) {
	adapter := &fakeAdapter{name: "fake", entered: make(chan struct{}, 4), block: make(chan struct{})}
	cfg := testConfig()
	cfg.QueueSize = 1
	cfg.BatchSize = 1
	f := NewForwarder([]BatchAdapter{adapter}, cfg, nil)
	t.Cleanup(func() { closeForwarder(t, f) })

	require.True(t, f.Enqueue(event("a", SeverityInfo)))
	<-adapter.entered // the worker holds "a" and waits on the adapter
	require.True(t, f.Enqueue(event("b", SeverityInfo)), "the queue holds one event")
	before := time.Now()
	require.False(t, f.Enqueue(event("c", SeverityInfo)), "the queue is full")

	health := f.Health()
	assert.True(t, health.Failing, "an event was dropped and no delivery was accepted since")
	assert.True(t, health.LastDelivery.IsZero(), "no delivery has ended yet")
	assert.False(t, health.LastDrop.Before(before))

	close(adapter.block)
	flushForwarder(t, f)
	health = f.Health()
	assert.False(t, health.Failing, "a delivery accepted after the drop ends the failing state")
	assert.Equal(t, ForwarderStats{Sent: 2, Dropped: 1}, f.Stats())
}
