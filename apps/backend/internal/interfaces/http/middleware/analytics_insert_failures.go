package middleware

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lib/pq"
)

// A failed api_calls insert is counted here and reported as one count line per period,
// so that a row lost to a failed insert leaves a trace instead of reading as a lower
// count in api_calls.
//
// The line carries the period and one integer per error class, and nothing else. The
// class is read from the driver's error code, never from the error's text: a driver
// error can repeat the value it rejected, and the row holds the caller's address, user
// agent, identifiers and endpoint. Do not print the error here, and do not add a field
// to the line or a key to the counter (organization, route, status).

// insertFailureReportPeriod is how often a pending line is written. A period with no
// failure writes nothing, so a process writes at most one line per period.
const insertFailureReportPeriod = 64 * time.Second

// apiCallInsertTimeout bounds one insert, the wait for a connection included, so that
// an insert that cannot finish is counted rather than left waiting with no trace.
var apiCallInsertTimeout = 30 * time.Second

// insertFailureClass is a two-character SQLSTATE class or one of the fixed words below.
type insertFailureClass string

const (
	// insertFailureTimeout: the statement did not finish before the deadline, or the
	// driver reported a deadline or cancellation.
	insertFailureTimeout insertFailureClass = "timeout"
	// insertFailurePool: no connection could be had from the pool before the deadline.
	insertFailurePool insertFailureClass = "pool"
	// insertFailureOther: an error that carries no SQLSTATE code.
	insertFailureOther insertFailureClass = "other"
)

// classifyInsertFailure names an insert error's class from its code alone.
func classifyInsertFailure(err error) insertFailureClass {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		if len(pqErr.Code) >= 2 && isSQLStateChar(pqErr.Code[0]) && isSQLStateChar(pqErr.Code[1]) {
			return insertFailureClass(pqErr.Code.Class())
		}
		return insertFailureOther
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return insertFailureTimeout
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return insertFailureTimeout
	}
	return insertFailureOther
}

// isSQLStateChar reports whether b is in the SQLSTATE alphabet (digits and A-Z).
func isSQLStateChar(b byte) bool {
	return ('0' <= b && b <= '9') || ('A' <= b && b <= 'Z')
}

// insertFailureReporter counts failed inserts by class and writes the counts as one line.
type insertFailureReporter struct {
	mu          sync.Mutex
	out         io.Writer
	now         func() time.Time
	periodStart time.Time
	counts      map[insertFailureClass]uint64
}

func newInsertFailureReporter(out io.Writer, now func() time.Time) *insertFailureReporter {
	return &insertFailureReporter{
		out:         out,
		now:         now,
		periodStart: now().UTC(),
		counts:      map[insertFailureClass]uint64{},
	}
}

func (r *insertFailureReporter) record(class insertFailureClass) {
	r.mu.Lock()
	r.counts[class]++
	r.mu.Unlock()
}

// flush closes the current period and writes its line if the period had a failure.
func (r *insertFailureReporter) flush() {
	r.mu.Lock()
	defer r.mu.Unlock()

	start := r.periodStart
	end := r.now().UTC()
	counts := r.counts
	r.periodStart = end
	r.counts = map[insertFailureClass]uint64{}
	if len(counts) == 0 {
		return
	}

	classes := make([]string, 0, len(counts))
	for class := range counts {
		classes = append(classes, string(class))
	}
	sort.Strings(classes)

	var line strings.Builder
	line.WriteString("api_calls_insert_failed period_start=")
	line.WriteString(start.Format(time.RFC3339))
	line.WriteString(" period_end=")
	line.WriteString(end.Format(time.RFC3339))
	for _, class := range classes {
		line.WriteString(" ")
		line.WriteString(class)
		line.WriteString("=")
		line.WriteString(strconv.FormatUint(counts[insertFailureClass(class)], 10))
	}
	line.WriteString("\n")
	_, _ = io.WriteString(r.out, line.String())
}

// run flushes once per tick until ticks is closed.
func (r *insertFailureReporter) run(ticks <-chan time.Time) {
	for range ticks {
		r.flush()
	}
}

// apiCallInsertFailures writes without the standard logger's prefix, so the line holds
// only what flush puts in it.
var (
	apiCallInsertFailures       = newInsertFailureReporter(os.Stderr, time.Now)
	startInsertFailureReporting sync.Once
)

// FlushAPICallInsertFailures writes the pending insert-failure line, if any. The server
// calls it on shutdown so that failures counted since the last period are not lost.
func FlushAPICallInsertFailures() {
	apiCallInsertFailures.flush()
}
