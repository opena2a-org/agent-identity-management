package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
)

type fakeActionRequestNoncePurger struct {
	result application.ActionRequestNoncePurgeResult
	err    error
	calls  chan struct{}
}

func (f *fakeActionRequestNoncePurger) Purge(context.Context) (application.ActionRequestNoncePurgeResult, error) {
	select {
	case f.calls <- struct{}{}:
	default:
	}
	return f.result, f.err
}

// The job logs its constants, and its first purge has completed when it
// returns, so the server never listens before a purge ran.
func TestActionRequestNoncePurgeJob_PurgesBeforeReturning(t *testing.T) {
	logs := captureLog(t)
	purger := &fakeActionRequestNoncePurger{calls: make(chan struct{}, 1),
		result: application.ActionRequestNoncePurgeResult{Deleted: 2, Organizations: 3, Duration: 4 * time.Millisecond}}

	stop := startActionRequestNoncePurgeJob(purger, time.Hour)
	defer stop()

	select {
	case <-purger.calls:
	default:
		t.Fatal("no purge ran before the job returned")
	}
	out := logs.String()
	for _, want := range []string{
		"window 30s", "nonces kept 31s past expiry", "purge every 1h0m0s", "admission timeout 5s",
		"signedBytes at most 65536 bytes (87382 base64url characters)", "body at most 88406 bytes (allowance 1024)",
		"payload depth at most 32",
		"s6_nonce_purge success=true deleted=2 organizations=3 duration_ms=4",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("log lacks %q:\n%s", want, out)
		}
	}
}

func TestActionRequestNoncePurgeJob_PurgesEveryInterval(t *testing.T) {
	captureLog(t)
	purger := &fakeActionRequestNoncePurger{calls: make(chan struct{}, 8)}
	stop := startActionRequestNoncePurgeJob(purger, 10*time.Millisecond)
	defer stop()

	for i := 0; i < 3; i++ {
		select {
		case <-purger.calls:
		case <-time.After(2 * time.Second):
			t.Fatalf("purge ran %d time(s) in 2s at a 10ms interval", i)
		}
	}
}

// Stopping the job returns only after its goroutine has exited: the stop line
// is already written, so it cannot reach a later test's captured log, and no
// purge runs afterwards.
func TestActionRequestNoncePurgeJob_StopWaitsForTheJobToExit(t *testing.T) {
	logs := captureLog(t)
	purger := &fakeActionRequestNoncePurger{calls: make(chan struct{}, 64)}

	stop := startActionRequestNoncePurgeJob(purger, time.Millisecond)
	<-purger.calls // the purge the job ran before returning
	select {
	case <-purger.calls:
	case <-time.After(2 * time.Second):
		t.Fatal("no purge ran in 2s at a 1ms interval")
	}
	stop()

	if !strings.Contains(logs.String(), "Action-request nonce purge job stopped") {
		t.Errorf("stop returned before the job logged its stop line:\n%s", logs.String())
	}
	for len(purger.calls) > 0 {
		<-purger.calls
	}
	select {
	case <-purger.calls:
		t.Fatal("purge ran after stop returned")
	case <-time.After(20 * time.Millisecond):
	}
	stop()
}

// A purge the job cancelled while stopping writes no line, so a stop writes
// nothing after its stop line.
func TestActionRequestNoncePurge_CancelledRunLogsNothing(t *testing.T) {
	logs := captureLog(t)
	purger := &fakeActionRequestNoncePurger{calls: make(chan struct{}, 1), err: context.Canceled}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	runActionRequestNoncePurge(ctx, purger)

	if logs.Len() != 0 {
		t.Errorf("a cancelled run wrote a line:\n%s", logs.String())
	}
}

// A failed run is logged without its error text, which could carry a value
// from the table.
func TestActionRequestNoncePurge_FailureLogsCountsOnly(t *testing.T) {
	logs := captureLog(t)
	purger := &fakeActionRequestNoncePurger{calls: make(chan struct{}, 1), err: errors.New("key (nonce)=(\\xdeadbeef)"),
		result: application.ActionRequestNoncePurgeResult{Deleted: 1, Organizations: 1}}

	runActionRequestNoncePurge(context.Background(), purger)

	out := logs.String()
	if !strings.Contains(out, "s6_nonce_purge success=false deleted=1 organizations=1") {
		t.Errorf("failure line missing:\n%s", out)
	}
	if strings.Contains(out, "deadbeef") {
		t.Errorf("the purge line carries a table value:\n%s", out)
	}
}
