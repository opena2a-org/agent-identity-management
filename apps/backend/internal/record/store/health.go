package store

import (
	"fmt"
	"sync"
	"time"
)

// RecordPathWindow is how long a failed expansion, destruction or reduction
// write keeps the record path unavailable, and how recent a successful write
// must be for the path to read ok.
const RecordPathWindow = 300 * time.Second

// PathState is the record path's state as a readiness check reports it.
type PathState string

const (
	// PathOK: a record write succeeded within RecordPathWindow and none
	// failed.
	PathOK PathState = "ok"
	// PathUnavailable: an expansion, destruction or reduction write failed
	// within RecordPathWindow.
	PathUnavailable PathState = "unavailable"
	// PathUnknown: no record write succeeded or failed within
	// RecordPathWindow.
	PathUnknown PathState = "unknown"
)

// PathStatus is the record path's state and, when it is unavailable, the
// class and reason of the newest failure. It names no organization.
type PathStatus struct {
	State  PathState
	Reason string
}

// pathHealth keeps the times of this process's newest failed and newest
// successful record writes. An observation's failure does not make the path
// unavailable.
type pathHealth struct {
	mu           sync.Mutex
	failedAt     time.Time
	failedClass  Class
	failedReason Reason
	succeededAt  time.Time
}

func (h *pathHealth) failed(at time.Time, class Class, reason Reason) {
	if class == ClassObservation {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if at.Before(h.failedAt) {
		return
	}
	h.failedAt, h.failedClass, h.failedReason = at, class, reason
}

func (h *pathHealth) succeeded(at time.Time) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if at.After(h.succeededAt) {
		h.succeededAt = at
	}
}

func (h *pathHealth) status(now time.Time) PathStatus {
	h.mu.Lock()
	defer h.mu.Unlock()
	within := func(t time.Time) bool { return !t.IsZero() && now.Sub(t) < RecordPathWindow }
	switch {
	case within(h.failedAt):
		return PathStatus{State: PathUnavailable,
			Reason: fmt.Sprintf("record write failed (class %s, reason %s)", h.failedClass, h.failedReason)}
	case within(h.succeededAt):
		return PathStatus{State: PathOK}
	default:
		return PathStatus{State: PathUnknown}
	}
}

// RecordPath reports the state of this process's record path at now, from
// the record writes its writer and debt settler made. It is what a
// readiness check reports as its recordPath dependency.
func (w *Writer) RecordPath(now time.Time) PathStatus {
	return w.health.status(now)
}
