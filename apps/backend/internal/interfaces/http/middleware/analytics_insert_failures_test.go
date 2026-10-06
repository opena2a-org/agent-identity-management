package middleware

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===========================
// Failed api_calls inserts are counted and reported
// ===========================

// failureLineShape is the whole vocabulary of a report line: the event name, the period
// it covers, and one integer per error class, where a class is a two-character SQLSTATE
// class or one of three fixed words. Anything else in the line fails this match.
var failureLineShape = regexp.MustCompile(
	`^api_calls_insert_failed period_start=\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z period_end=\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z( ([0-9A-Z]{2}|timeout|pool|other)=[1-9][0-9]*)+\n$`)

// fixedClock returns t0 on the first call and steps forward by step on every later call.
func fixedClock(t0 time.Time, step time.Duration) func() time.Time {
	var mu sync.Mutex
	next := t0
	return func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		now := next
		next = next.Add(step)
		return now
	}
}

// useTestInsertFailureReporter points the package's reporter at a buffer for one test.
func useTestInsertFailureReporter(t *testing.T, now func() time.Time) *bytes.Buffer {
	t.Helper()
	var out bytes.Buffer
	saved := apiCallInsertFailures
	apiCallInsertFailures = newInsertFailureReporter(&out, now)
	t.Cleanup(func() { apiCallInsertFailures = saved })
	return &out
}

// useInsertTimeout shortens the insert deadline for one test.
func useInsertTimeout(t *testing.T, d time.Duration) {
	t.Helper()
	saved := apiCallInsertTimeout
	apiCallInsertTimeout = d
	t.Cleanup(func() { apiCallInsertTimeout = saved })
}

func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db, mock
}

func TestLogAPICall_FailedInsertWritesOneCountLine(t *testing.T) {
	t0 := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	out := useTestInsertFailureReporter(t, fixedClock(t0, 64*time.Second))
	db, mock := newMockDB(t)
	mock.ExpectExec("INSERT INTO api_calls").WillReturnError(&pq.Error{Code: "23502", Message: "null value"})

	logAPICall(db, APICallLog{Method: "GET", Endpoint: "/api/v1/agents", StatusCode: 200})
	require.NoError(t, mock.ExpectationsWereMet())
	assert.Empty(t, out.String(), "nothing is written until the period closes")

	FlushAPICallInsertFailures()

	assert.Equal(t,
		"api_calls_insert_failed period_start=2026-10-06T12:00:00Z period_end=2026-10-06T12:01:04Z 23=1\n",
		out.String())
}

func TestLogAPICall_WrittenRowIsNotCounted(t *testing.T) {
	out := useTestInsertFailureReporter(t, time.Now)
	db, mock := newMockDB(t)
	mock.ExpectExec("INSERT INTO api_calls").WillReturnResult(sqlmock.NewResult(1, 1))

	logAPICall(db, APICallLog{Method: "GET", Endpoint: "/api/v1/agents", StatusCode: 200})
	require.NoError(t, mock.ExpectationsWereMet())
	FlushAPICallInsertFailures()

	assert.Empty(t, out.String())
}

func TestLogAPICall_BrokenConnectionIsReplacedOnce(t *testing.T) {
	out := useTestInsertFailureReporter(t, time.Now)
	db, mock := newMockDB(t)
	// Keep one connection open for the whole test so the mock driver can still open a
	// fresh one after the broken connection is closed.
	held, err := db.Conn(context.Background())
	require.NoError(t, err)
	defer held.Close()
	mock.ExpectExec("INSERT INTO api_calls").WillReturnError(driver.ErrBadConn)
	mock.ExpectExec("INSERT INTO api_calls").WillReturnResult(sqlmock.NewResult(1, 1))

	logAPICall(db, APICallLog{Method: "GET", Endpoint: "/api/v1/agents", StatusCode: 200})
	require.NoError(t, mock.ExpectationsWereMet())
	FlushAPICallInsertFailures()

	assert.Empty(t, out.String(), "a row written on the second connection is not a loss")
}

func TestLogAPICall_SlowInsertIsCountedAsTimeout(t *testing.T) {
	out := useTestInsertFailureReporter(t, time.Now)
	useInsertTimeout(t, 50*time.Millisecond)
	db, mock := newMockDB(t)
	mock.ExpectExec("INSERT INTO api_calls").WillDelayFor(2 * time.Second).WillReturnResult(sqlmock.NewResult(1, 1))

	logAPICall(db, APICallLog{Method: "GET", Endpoint: "/api/v1/agents", StatusCode: 200})
	FlushAPICallInsertFailures()

	assert.Regexp(t, failureLineShape, out.String())
	assert.Contains(t, out.String(), " timeout=1\n")
}

func TestLogAPICall_NoFreeConnectionIsCountedAsPool(t *testing.T) {
	out := useTestInsertFailureReporter(t, time.Now)
	useInsertTimeout(t, 50*time.Millisecond)
	db, _ := newMockDB(t)
	db.SetMaxOpenConns(1)
	held, err := db.Conn(context.Background())
	require.NoError(t, err)
	defer held.Close()

	logAPICall(db, APICallLog{Method: "GET", Endpoint: "/api/v1/agents", StatusCode: 200})
	FlushAPICallInsertFailures()

	assert.Regexp(t, failureLineShape, out.String())
	assert.Contains(t, out.String(), " pool=1\n")
}

func TestInsertFailureReporter_OneLinePerPeriodThatHadAFailure(t *testing.T) {
	t0 := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	var out bytes.Buffer
	r := newInsertFailureReporter(&out, fixedClock(t0, 64*time.Second))

	r.record("08")
	r.record(insertFailureTimeout)
	r.record("08")
	r.record(insertFailurePool)
	r.flush()
	r.flush() // a period with no failure writes nothing
	r.record(insertFailureOther)
	r.flush()

	assert.Equal(t,
		"api_calls_insert_failed period_start=2026-10-06T12:00:00Z period_end=2026-10-06T12:01:04Z 08=2 pool=1 timeout=1\n"+
			"api_calls_insert_failed period_start=2026-10-06T12:02:08Z period_end=2026-10-06T12:03:12Z other=1\n",
		out.String())
}

func TestInsertFailureReporter_RunFlushesOnEachTick(t *testing.T) {
	var out bytes.Buffer
	r := newInsertFailureReporter(&out, time.Now)
	ticks := make(chan time.Time)
	done := make(chan struct{})
	go func() {
		r.run(ticks)
		close(done)
	}()

	r.record("53")
	ticks <- time.Now()
	close(ticks)
	<-done

	assert.Regexp(t, failureLineShape, out.String())
	assert.Contains(t, out.String(), " 53=1\n")
}

func TestClassifyInsertFailure(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want insertFailureClass
	}{
		{"driver error with a code", &pq.Error{Code: "23505"}, "23"},
		{"wrapped driver error with a code", fmt.Errorf("insert: %w", &pq.Error{Code: "08006"}), "08"},
		{"driver error with no code", &pq.Error{Message: "something failed"}, insertFailureOther},
		{"driver error with a one-character code", &pq.Error{Code: "4"}, insertFailureOther},
		{"driver error with a code outside the SQLSTATE alphabet", &pq.Error{Code: "a'123"}, insertFailureOther},
		{"deadline", context.DeadlineExceeded, insertFailureTimeout},
		{"wrapped deadline", fmt.Errorf("exec: %w", context.DeadlineExceeded), insertFailureTimeout},
		{"cancellation", context.Canceled, insertFailureTimeout},
		{"network timeout", timeoutNetError{}, insertFailureTimeout},
		{"broken connection", driver.ErrBadConn, insertFailureOther},
		{"error with no code", errors.New("planted.person@example.com"), insertFailureOther},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, classifyInsertFailure(tc.err))
		})
	}
}

type timeoutNetError struct{}

func (timeoutNetError) Error() string   { return "i/o timeout" }
func (timeoutNetError) Timeout() bool   { return true }
func (timeoutNetError) Temporary() bool { return true }

// TestInsertFailureLine_CarriesNoRowContent plants the row's identifying values in the
// driver error itself, the worst case of a driver that repeats the value it rejected,
// and checks that none of them reaches the line on either the periodic path or the
// shutdown path.
func TestInsertFailureLine_CarriesNoRowContent(t *testing.T) {
	const (
		plantedAddress   = "203.0.113.77"
		plantedUserAgent = "PlantedAgent/9.9"
		plantedEndpoint  = "/api/v1/planted/endpoint"
		plantedEmail     = "planted.person@example.com"
	)
	plantedID := uuid.MustParse("6f1c9a52-3b7d-4e0a-9c11-2d5e8f4b7a63")
	planted := []string{plantedAddress, plantedUserAgent, plantedEndpoint, plantedID.String(), plantedEmail}

	found := func(text string) []string {
		var hits []string
		for _, p := range planted {
			if strings.Contains(text, p) {
				hits = append(hits, p)
			}
		}
		return hits
	}

	injected := &pq.Error{
		Code: "22001",
		Message: fmt.Sprintf("value too long for row (%s, %s, %s, %s, %s)",
			plantedAddress, plantedUserAgent, plantedEndpoint, plantedID, plantedEmail),
		Detail: "Failing row contains " + plantedEmail + ".",
		Where:  plantedEndpoint,
	}
	// An error with no code takes the fallback class, the branch where a class read from
	// text would leak.
	injectedNoCode := errors.New(injected.Message)
	// Positive control: the scan finds every planted value in each injected error's own text.
	require.ElementsMatch(t, planted, found(injected.Error()+injected.Detail+injected.Where))
	require.ElementsMatch(t, planted, found(injectedNoCode.Error()))

	row := APICallLog{
		OrganizationID: &plantedID,
		UserID:         &plantedID,
		Method:         "POST",
		Endpoint:       plantedEndpoint,
		StatusCode:     500,
		UserAgent:      plantedUserAgent,
		IPAddress:      plantedAddress,
	}
	failInsert := func(t *testing.T) {
		db, mock := newMockDB(t)
		mock.ExpectExec("INSERT INTO api_calls").WillReturnError(injected)
		mock.ExpectExec("INSERT INTO api_calls").WillReturnError(injectedNoCode)
		logAPICall(db, row)
		logAPICall(db, row)
		require.NoError(t, mock.ExpectationsWereMet())
	}

	t.Run("periodic line", func(t *testing.T) {
		out := useTestInsertFailureReporter(t, time.Now)
		failInsert(t)
		ticks := make(chan time.Time)
		done := make(chan struct{})
		go func() {
			apiCallInsertFailures.run(ticks)
			close(done)
		}()
		ticks <- time.Now()
		close(ticks)
		<-done

		assert.Regexp(t, failureLineShape, out.String())
		assert.Contains(t, out.String(), " 22=1 other=1\n")
		assert.Empty(t, found(out.String()))
	})

	t.Run("shutdown line", func(t *testing.T) {
		out := useTestInsertFailureReporter(t, time.Now)
		failInsert(t)
		FlushAPICallInsertFailures()

		assert.Regexp(t, failureLineShape, out.String())
		assert.Contains(t, out.String(), " 22=1 other=1\n")
		assert.Empty(t, found(out.String()))
	})
}
