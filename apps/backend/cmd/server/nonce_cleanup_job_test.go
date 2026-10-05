package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type fakeNonceCleaner struct {
	deleted int
	err     error
	calls   chan struct{}
}

func (f *fakeNonceCleaner) CleanupExpiredNonces(context.Context) (int, error) {
	select {
	case f.calls <- struct{}{}:
	default:
	}
	return f.deleted, f.err
}

// The job logs its interval when it starts and calls the cleanup on every
// tick, with no admin call involved.
func TestNonceCleanupJob_RunsTheCleanupEachInterval(t *testing.T) {
	logs := captureLog(t)
	cleaner := &fakeNonceCleaner{calls: make(chan struct{}, 8)}

	stop := startNonceCleanupJob(cleaner, 10*time.Millisecond)
	defer close(stop)

	if !strings.Contains(logs.String(), "A2A nonce cleanup job started (runs every 10ms)") {
		t.Errorf("start line does not carry the interval:\n%s", logs.String())
	}
	for i := 0; i < 2; i++ {
		select {
		case <-cleaner.calls:
		case <-time.After(2 * time.Second):
			t.Fatalf("cleanup ran %d time(s) in 2s at a 10ms interval", i)
		}
	}
}

// A tick that deletes rows logs the count.
func TestNonceCleanup_LogsTheDeletedCount(t *testing.T) {
	logs := captureLog(t)
	cleaner := &fakeNonceCleaner{deleted: 3, calls: make(chan struct{}, 1)}

	runNonceCleanup(cleaner)

	if !strings.Contains(logs.String(), "A2A nonce cleanup deleted 3 expired nonce(s)") {
		t.Errorf("log does not report the deleted count:\n%s", logs.String())
	}
}

// A tick with nothing to delete stays quiet.
func TestNonceCleanup_NothingToDeleteLogsNothing(t *testing.T) {
	logs := captureLog(t)
	cleaner := &fakeNonceCleaner{calls: make(chan struct{}, 1)}

	runNonceCleanup(cleaner)

	if logs.Len() != 0 {
		t.Errorf("a tick with nothing to delete wrote a line:\n%s", logs.String())
	}
}

// A failed tick is logged, not fatal; the next tick runs again.
func TestNonceCleanup_LogsAnError(t *testing.T) {
	logs := captureLog(t)
	cleaner := &fakeNonceCleaner{err: errors.New("nonce table locked"), calls: make(chan struct{}, 1)}

	runNonceCleanup(cleaner)

	if !strings.Contains(logs.String(), "A2A nonce cleanup error: nonce table locked") {
		t.Errorf("log does not carry the error:\n%s", logs.String())
	}
}
