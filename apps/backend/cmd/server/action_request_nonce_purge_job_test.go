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
	defer close(stop)

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
	defer close(stop)

	for i := 0; i < 3; i++ {
		select {
		case <-purger.calls:
		case <-time.After(2 * time.Second):
			t.Fatalf("purge ran %d time(s) in 2s at a 10ms interval", i)
		}
	}
}

// A failed run is logged without its error text, which could carry a value
// from the table.
func TestActionRequestNoncePurge_FailureLogsCountsOnly(t *testing.T) {
	logs := captureLog(t)
	purger := &fakeActionRequestNoncePurger{calls: make(chan struct{}, 1), err: errors.New("key (nonce)=(\\xdeadbeef)"),
		result: application.ActionRequestNoncePurgeResult{Deleted: 1, Organizations: 1}}

	runActionRequestNoncePurge(purger)

	out := logs.String()
	if !strings.Contains(out, "s6_nonce_purge success=false deleted=1 organizations=1") {
		t.Errorf("failure line missing:\n%s", out)
	}
	if strings.Contains(out, "deadbeef") {
		t.Errorf("the purge line carries a table value:\n%s", out)
	}
}
