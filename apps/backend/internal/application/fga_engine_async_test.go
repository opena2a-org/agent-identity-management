package application

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.uber.org/goleak"
)

// captureHandler is a slog.Handler that records every record's message so
// tests can assert on what was logged.
type captureHandler struct {
	mu      sync.Mutex
	records []string
}

func (h *captureHandler) Enabled(context.Context, slog.Level) bool { return true }
func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, r.Message)
	return nil
}
func (h *captureHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *captureHandler) WithGroup(string) slog.Handler      { return h }
func (h *captureHandler) count(prefix string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	n := 0
	for _, m := range h.records {
		if strings.HasPrefix(m, prefix) {
			n++
		}
	}
	return n
}

// newTestEngineForAsync builds an FGAEngine wired only with what the async
// dispatch + worker path needs: a logger, a tracer, and a sized worker
// pool. asyncDropped is left nil — the dispatch path nil-checks it. The
// returned engine has no DB and no agentSvc; tests must not exercise the
// full Authorize() flow.
func newTestEngineForAsync(t *testing.T, daemonURL string, tracer trace.Tracer, workers, queueSize int) (*FGAEngine, *captureHandler) {
	t.Helper()
	logs := &captureHandler{}
	e := &FGAEngine{
		daemonURL:  daemonURL,
		logger:     slog.New(logs),
		tracer:     tracer,
		asyncQueue: make(chan asyncIntentJob, queueSize),
		asyncDone:  make(chan struct{}),
	}
	e.startAsyncWorkers(workers)
	return e, logs
}

// dispatchAsync mirrors the MEDIUM-risk dispatch shape from Authorize so
// tests can drive the queue without standing up a Postgres + agent
// service. The priority-select shape is load-bearing for shutdown
// safety, so the test harness must mirror it exactly. Keep in sync with
// the real branch in Authorize().
func (e *FGAEngine) dispatchAsync(ctx context.Context, req *FGARequest) {
	parentSpan := trace.SpanFromContext(ctx)
	detached := trace.ContextWithSpan(context.WithoutCancel(ctx), parentSpan)
	select {
	case <-e.asyncDone:
		e.logger.WarnContext(ctx, "fga.async_intent dropped (shutdown)",
			slog.String("agent.id", req.AgentID.String()),
			slog.String("agent.capability", req.Capability),
			slog.String("fga.async_drop_reason", "shutdown"),
		)
	default:
		select {
		case e.asyncQueue <- asyncIntentJob{ctx: detached, req: req}:
		default:
			e.logger.WarnContext(ctx, "fga.async_intent dropped (queue full)",
				slog.String("agent.id", req.AgentID.String()),
				slog.String("agent.capability", req.Capability),
				slog.String("fga.async_drop_reason", "queue_full"),
			)
		}
	}
}

// TestAsyncIntent_BackpressureDrops fires more requests than the queue +
// worker pool can hold while the daemon hangs, and asserts that the
// excess dispatches log a backpressure WARN.
func TestAsyncIntent_BackpressureDrops(t *testing.T) {
	// Snapshot baseline goroutines BEFORE we touch httptest, http.Client
	// transports, or the worker pool — so the post-test goleak only flags
	// goroutines this test created.
	leakOpt := goleak.IgnoreCurrent()

	// Daemon that blocks until the test releases it. While blocked, all
	// pool workers are stuck on the HTTP call, so further sends fill the
	// queue and then hit the default branch (drop).
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
		_, _ = io.WriteString(w, `{"result":"ok","confidence":0,"attackClass":""}`)
	}))
	defer func() {
		closeIdleHTTPConnections()
		goleak.VerifyNone(t, leakOpt)
	}()
	defer srv.Close()

	const (
		workers   = 2
		queueSize = 4
		extra     = 10 // beyond pool + queue capacity
	)

	tp := sdktrace.NewTracerProvider()
	defer func() { _ = tp.Shutdown(context.Background()) }()

	engine, logs := newTestEngineForAsync(t, srv.URL, tp.Tracer("test"), workers, queueSize)

	totalSent := workers + queueSize + extra
	for i := 0; i < totalSent; i++ {
		engine.dispatchAsync(context.Background(), &FGARequest{
			AgentID:    uuid.New(),
			Capability: "test:capability",
		})
	}

	// Give the dispatch loop a moment to fill workers + queue + drop the
	// rest. Drops are observed synchronously on dispatch (it's a select
	// default), so this is a conservative wait, not a poll.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if logs.count("fga.async_intent dropped") >= extra {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	dropped := logs.count("fga.async_intent dropped")
	assert.GreaterOrEqualf(t, dropped, extra,
		"expected >= %d dropped dispatches, got %d (workers=%d queue=%d sent=%d)",
		extra, dropped, workers, queueSize, totalSent)

	// Release blocked daemon handlers so workers can drain on Shutdown.
	close(release)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, engine.Shutdown(shutdownCtx))
}

// TestAsyncIntent_NoGoroutineLeak asserts that after Shutdown returns
// cleanly, the engine has not left any worker goroutines running. Uses
// goleak.IgnoreCurrent() to ignore goroutines that existed before the
// engine was constructed (e.g. test runtime, default HTTP transport).
func TestAsyncIntent_NoGoroutineLeak(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"result":"ok","confidence":0,"attackClass":""}`)
	}))
	defer srv.Close()

	tp := sdktrace.NewTracerProvider()
	defer func() { _ = tp.Shutdown(context.Background()) }()

	// Snapshot baseline goroutines BEFORE engine init so goleak only
	// flags engine-owned goroutines that didn't shut down.
	leakOpt := goleak.IgnoreCurrent()

	const workers = 4
	engine, _ := newTestEngineForAsync(t, srv.URL, tp.Tracer("test"), workers, 8)

	// At least the worker pool should be live now.
	require.GreaterOrEqual(t, runtime.NumGoroutine(), workers,
		"expected at least %d goroutines (the worker pool) after engine init",
		workers)

	// Push a few jobs and let them complete.
	for i := 0; i < 3; i++ {
		engine.dispatchAsync(context.Background(), &FGARequest{
			AgentID:    uuid.New(),
			Capability: "test:capability",
		})
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, engine.Shutdown(shutdownCtx))

	// HTTP keep-alive goroutines linger by design; close them so the
	// leak check only reflects engine-owned goroutines.
	closeIdleHTTPConnections()

	goleak.VerifyNone(t, leakOpt)
}

// closeIdleHTTPConnections forces the default HTTP transport to release
// keep-alive workers. Without this, goleak observes idle conn pool
// goroutines created by the test's httptest.Server interactions and
// flags them as leaks.
func closeIdleHTTPConnections() {
	if t, ok := http.DefaultTransport.(*http.Transport); ok {
		t.CloseIdleConnections()
	}
}

// TestAsyncIntent_DispatchAfterShutdown_NoPanic is the regression test
// for the adversarial review's HIGH finding: a `select { case ch <- v;
// default }` on a CLOSED channel still picks the send case and panics,
// because closed-channel sends are "ready" for select. The fix is the
// three-way select with a separate asyncDone signal — asyncQueue is
// never closed, so the send case can't panic. This test fires
// dispatches AFTER Shutdown to prove the path is safe.
func TestAsyncIntent_DispatchAfterShutdown_NoPanic(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"result":"ok","confidence":0,"attackClass":""}`)
	}))
	defer srv.Close()

	tp := sdktrace.NewTracerProvider()
	defer func() { _ = tp.Shutdown(context.Background()) }()

	engine, logs := newTestEngineForAsync(t, srv.URL, tp.Tracer("test"), 2, 4)

	// Shut the engine down first — workers exit, asyncDone closed.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, engine.Shutdown(shutdownCtx))

	// Now fire dispatches. With the old single-case select, this would
	// panic with "send on closed channel". With the new three-way
	// select, asyncDone is closed so each dispatch picks the shutdown
	// branch and drops cleanly.
	require.NotPanics(t, func() {
		for i := 0; i < 20; i++ {
			engine.dispatchAsync(context.Background(), &FGARequest{
				AgentID:    uuid.New(),
				Capability: "test:capability",
			})
		}
	}, "dispatch after Shutdown must not panic")

	// Every dispatch should have dropped under the shutdown reason.
	require.GreaterOrEqualf(t, logs.count("fga.async_intent dropped (shutdown)"), 20,
		"expected 20 shutdown-reason drops, got %d", logs.count("fga.async_intent dropped (shutdown)"))

	// Idempotent Shutdown: second call returns nil and doesn't deadlock.
	require.NoError(t, engine.Shutdown(shutdownCtx))
}

// TestAsyncIntent_TraceParentChained asserts that the worker's span
// (`fga.intent_check_async.worker`) is rooted in the same trace as the
// caller's `fga.authorize` span. Without this, the async check shows up
// as a detached trace in Tempo, which was the original H2 finding.
func TestAsyncIntent_TraceParentChained(t *testing.T) {
	leakOpt := goleak.IgnoreCurrent()
	defer func() {
		closeIdleHTTPConnections()
		goleak.VerifyNone(t, leakOpt)
	}()

	// Track whether the daemon was actually hit so we know the worker
	// ran (and produced its span).
	var daemonHits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&daemonHits, 1)
		_, _ = io.WriteString(w, `{"result":"ok","confidence":0,"attackClass":""}`)
	}))
	defer srv.Close()

	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	defer func() { _ = tp.Shutdown(context.Background()) }()

	tracer := tp.Tracer("test")
	engine, _ := newTestEngineForAsync(t, srv.URL, tracer, 2, 4)

	// Simulate the fga.authorize parent.
	parentCtx, parentSpan := tracer.Start(context.Background(), "fga.authorize")
	expectedTraceID := parentSpan.SpanContext().TraceID()

	engine.dispatchAsync(parentCtx, &FGARequest{
		AgentID:    uuid.New(),
		Capability: "test:capability",
	})
	parentSpan.End()

	// Wait for the worker to process the job.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(&daemonHits) >= 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	require.EqualValues(t, 1, atomic.LoadInt32(&daemonHits),
		"daemon should have been hit exactly once (worker ran)")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, engine.Shutdown(shutdownCtx))

	// Find the worker span in the recorder and assert its trace ID.
	ended := rec.Ended()
	var workerSpan sdktrace.ReadOnlySpan
	for _, s := range ended {
		if s.Name() == "fga.intent_check_async.worker" {
			workerSpan = s
			break
		}
	}
	require.NotNil(t, workerSpan, "expected an fga.intent_check_async.worker span")
	assert.Equal(t, expectedTraceID, workerSpan.SpanContext().TraceID(),
		"worker span must chain into the fga.authorize trace, not start a detached one")
	assert.Equal(t, parentSpan.SpanContext().SpanID(), workerSpan.Parent().SpanID(),
		"worker span's parent should be the fga.authorize span")
}
