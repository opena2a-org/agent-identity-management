package application

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
)

// TestCheckIntentSync_RespectsCtxCancel asserts that ctx cancellation
// aborts the in-flight HTTP call to the NanoMind daemon, returning well
// before the 800ms client timeout would fire.
//
// Without ctx wiring, this test would block until the client.Timeout
// expires (≈ 800ms). With ctx wiring, client.Do returns immediately on
// cancel and checkIntentSync returns a fail-open IntentCheckResult.
func TestCheckIntentSync_RespectsCtxCancel(t *testing.T) {
	// Daemon hangs until either the client cancels (we want this) or
	// the test finishes (release channel as a safety net).
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	defer func() {
		close(release)
		srv.Close()
		closeIdleHTTPConnections()
	}()

	logs := &captureHandler{}
	engine := &FGAEngine{
		daemonURL: srv.URL,
		logger:    slog.New(logs),
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel before the 800ms client timeout would fire. If ctx isn't
	// honored, checkIntentSync blocks for the full 800ms and this test
	// fails the deadline assertion below.
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	result := engine.checkIntentSync(ctx, &FGARequest{
		AgentID:    uuid.New(),
		Capability: "test:capability",
		Resource:   "test/resource",
	})
	elapsed := time.Since(start)

	if elapsed >= 500*time.Millisecond {
		t.Fatalf("checkIntentSync took %s; expected < 500ms (ctx cancel ignored, hit 800ms client timeout?)", elapsed)
	}
	if result == nil {
		t.Fatal("checkIntentSync returned nil result on ctx cancel; expected fail-open IntentCheckResult")
	}
	if result.Blocked {
		t.Fatal("checkIntentSync blocked on ctx cancel; expected fail-open (Blocked=false)")
	}
}

// TestCheckIntentSync_RespectsCtxDeadline asserts that a context with a
// short deadline aborts the call, again well before the 800ms client
// timeout. Same shape as the cancel test, but exercises the deadline
// branch since http transports treat them slightly differently.
func TestCheckIntentSync_RespectsCtxDeadline(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	defer func() {
		close(release)
		srv.Close()
		closeIdleHTTPConnections()
	}()

	logs := &captureHandler{}
	engine := &FGAEngine{
		daemonURL: srv.URL,
		logger:    slog.New(logs),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	result := engine.checkIntentSync(ctx, &FGARequest{
		AgentID:    uuid.New(),
		Capability: "test:capability",
		Resource:   "test/resource",
	})
	elapsed := time.Since(start)

	if elapsed >= 500*time.Millisecond {
		t.Fatalf("checkIntentSync took %s; expected < 500ms (ctx deadline ignored, hit 800ms client timeout?)", elapsed)
	}
	if result == nil {
		t.Fatal("checkIntentSync returned nil result on ctx deadline; expected fail-open IntentCheckResult")
	}
	if result.Blocked {
		t.Fatal("checkIntentSync blocked on ctx deadline; expected fail-open (Blocked=false)")
	}
}
