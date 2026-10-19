package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type fakeAccountDeletionPurger struct {
	deleted int
	err     error
	calls   chan struct{}
}

func (f *fakeAccountDeletionPurger) Purge(context.Context) (int, error) {
	select {
	case f.calls <- struct{}{}:
	default:
	}
	return f.deleted, f.err
}

// The job logs its interval and the delay when it starts, runs the purge at
// once and then on every tick.
func TestAccountDeletionPurgeJob_RunsAtStartAndEachInterval(t *testing.T) {
	logs := captureLog(t)
	purger := &fakeAccountDeletionPurger{calls: make(chan struct{}, 8)}

	stop := startAccountDeletionPurgeJob(purger, time.Hour)
	select {
	case <-purger.calls:
	case <-time.After(2 * time.Second):
		t.Fatal("the purge did not run at start")
	}
	stop()
	if !strings.Contains(logs.String(), "Account deletion purge job started (runs every 1h0m0s; deletes an account 29 days after its deletion request)") {
		t.Errorf("start line does not carry the interval and the delay:\n%s", logs.String())
	}

	purger = &fakeAccountDeletionPurger{calls: make(chan struct{}, 8)}
	stop = startAccountDeletionPurgeJob(purger, 10*time.Millisecond)
	defer stop()
	for i := 0; i < 3; i++ {
		select {
		case <-purger.calls:
		case <-time.After(2 * time.Second):
			t.Fatalf("the purge ran %d time(s) in 2s at a 10ms interval", i)
		}
	}
}

// Stopping the job returns only after its goroutine has exited: the stop line
// is already written, and no run starts afterwards.
func TestAccountDeletionPurgeJob_StopWaitsForTheJobToExit(t *testing.T) {
	logs := captureLog(t)
	purger := &fakeAccountDeletionPurger{calls: make(chan struct{}, 64)}

	stop := startAccountDeletionPurgeJob(purger, time.Millisecond)
	select {
	case <-purger.calls:
	case <-time.After(2 * time.Second):
		t.Fatal("the purge did not run in 2s")
	}
	stop()

	if !strings.Contains(logs.String(), "Account deletion purge job stopped") {
		t.Errorf("stop returned before the job logged its stop line:\n%s", logs.String())
	}
	for len(purger.calls) > 0 {
		<-purger.calls
	}
	select {
	case <-purger.calls:
		t.Fatal("the purge ran after stop returned")
	case <-time.After(20 * time.Millisecond):
	}
	stop()
}

// A run that deletes accounts logs the count.
func TestAccountDeletionPurge_LogsTheDeletedCount(t *testing.T) {
	logs := captureLog(t)

	runAccountDeletionPurge(context.Background(), &fakeAccountDeletionPurger{deleted: 2, calls: make(chan struct{}, 1)})

	if !strings.Contains(logs.String(), "Account deletion purge deleted 2 account(s)") {
		t.Errorf("log does not report the deleted count:\n%s", logs.String())
	}
}

// A run with nothing to delete stays quiet.
func TestAccountDeletionPurge_NothingToDeleteLogsNothing(t *testing.T) {
	logs := captureLog(t)

	runAccountDeletionPurge(context.Background(), &fakeAccountDeletionPurger{calls: make(chan struct{}, 1)})

	if logs.Len() != 0 {
		t.Errorf("a run with nothing to delete wrote a line:\n%s", logs.String())
	}
}

// A failed run is logged with what it deleted before the failure, and is not
// fatal.
func TestAccountDeletionPurge_LogsAnError(t *testing.T) {
	logs := captureLog(t)

	runAccountDeletionPurge(context.Background(), &fakeAccountDeletionPurger{deleted: 1, err: errors.New("users table locked"), calls: make(chan struct{}, 1)})

	if !strings.Contains(logs.String(), "Account deletion purge error, 1 account(s) deleted before it: users table locked") {
		t.Errorf("log does not carry the error:\n%s", logs.String())
	}
}

// A run the stopping job cancelled writes no line.
func TestAccountDeletionPurge_ACancelledRunLogsNothing(t *testing.T) {
	logs := captureLog(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	runAccountDeletionPurge(ctx, &fakeAccountDeletionPurger{err: context.Canceled, calls: make(chan struct{}, 1)})

	if logs.Len() != 0 {
		t.Errorf("a cancelled run wrote a line:\n%s", logs.String())
	}
}
