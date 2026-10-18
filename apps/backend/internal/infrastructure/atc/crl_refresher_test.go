package atc

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// steppedRefresher is a refresher of c whose waits are recorded instead of
// slept: the i-th wait runs steps[i] and the loop goes on, and a wait with no
// step ends the loop. Its hold-down draws return draw, and the bound of each
// draw is recorded. The tests' clocks read whole seconds, as the times of a
// list are written, so that the waits they expect are exact.
type steppedRefresher struct {
	*CRLRefresher
	waits []time.Duration
	bound []int64
	lines []string
}

func newSteppedRefresher(c *CachedCRLClient, draw int64, steps ...func()) *steppedRefresher {
	s := &steppedRefresher{CRLRefresher: NewCRLRefresher(c)}
	s.sleep = func(_ context.Context, d time.Duration) bool {
		s.waits = append(s.waits, d)
		if i := len(s.waits) - 1; i < len(steps) {
			steps[i]()
			return true
		}
		return false
	}
	s.jitter = func(n int64) int64 {
		s.bound = append(s.bound, n)
		return draw
	}
	s.logf = func(format string, args ...any) {
		s.lines = append(s.lines, fmt.Sprintf(format, args...))
	}
	return s
}

func requireWaits(t *testing.T, s *steppedRefresher, want ...time.Duration) {
	t.Helper()
	if fmt.Sprint(s.waits) != fmt.Sprint(want) {
		t.Fatalf("the refresher waited %v, want %v", s.waits, want)
	}
}

func requireLines(t *testing.T, s *steppedRefresher, want ...string) {
	t.Helper()
	if len(s.lines) != len(want) {
		t.Fatalf("the refresher logged %d lines, want %d:\n%s", len(s.lines), len(want), strings.Join(s.lines, "\n"))
	}
	for i, prefix := range want {
		if !strings.HasPrefix(s.lines[i], prefix) {
			t.Errorf("line %d is %q, want it to start with %q", i, s.lines[i], prefix)
		}
	}
}

// The first refresh starts before any wait, and a list whose nextUpdate is
// an hour away is read again 4 minutes later.
func TestCRLRefresher_RefreshesAtStartThenEveryFourMinutes(t *testing.T) {
	signer := newEd25519Signer(t, testListKeyID)
	a := newCRLAuthority(t, discoveryFor(signer))
	now := time.Now().Truncate(time.Second)
	doc := testList(now, revokedDID)
	doc["nextUpdate"] = now.Add(time.Hour).UTC().Format(time.RFC3339)
	a.setList(signList(t, doc, signer))
	c := newTestCRLClient(t, a, "", &now)
	s := newSteppedRefresher(c, 0, func() { now = now.Add(4 * time.Minute) })

	requireUnknown(t, c, revokedDID)
	s.run(context.Background())

	requireWaits(t, s, 4*time.Minute, 4*time.Minute)
	requireAnswer(t, c, revokedDID, true)
	if paths, _ := a.requests(); len(paths) != 2 {
		t.Fatalf("the list was read %d times, want 2: at start and 4 minutes later", len(paths))
	}
	nextUpdate := now.Add(-4 * time.Minute).Add(time.Hour).UTC().Format(time.RFC3339)
	requireLines(t, s, "Revocation list: holds a list, thisUpdate "+now.Add(-5*time.Minute).UTC().Format(time.RFC3339)+
		", nextUpdate "+nextUpdate+"; next refresh in 4m0s")
}

// A list whose nextUpdate comes sooner than 4 minutes and 60 s is read again
// 60 s before its nextUpdate; once that time has passed, it is read again
// after the hold-down.
func TestCRLRefresher_RefreshesSixtySecondsBeforeNextUpdate(t *testing.T) {
	signer := newEd25519Signer(t, testListKeyID)
	a := newCRLAuthority(t, discoveryFor(signer))
	now := time.Now().Truncate(time.Second)
	a.setList(signList(t, testList(now), signer)) // nextUpdate 4 minutes after now
	c := newTestCRLClient(t, a, "", &now)
	s := newSteppedRefresher(c, 4*int64(time.Second), func() { now = now.Add(3*time.Minute + 30*time.Second) })

	s.run(context.Background())

	requireWaits(t, s, 3*time.Minute, 5*time.Second)
	requireAnswer(t, c, otherDID, false)
	if len(s.bound) != 1 || s.bound[0] != int64(29*time.Second)+1 {
		t.Fatalf("the hold-down was drawn below %v, want one draw below %d", s.bound, int64(29*time.Second)+1)
	}
}

// After a refresh that holds no list the refresher waits 1 s plus a draw
// below 29 s and 1 ns, so from 1 s to 30 s, and it is ready to answer once a
// later refresh holds a list.
func TestCRLRefresher_HoldsDownOneToThirtySecondsAfterAFailedRefresh(t *testing.T) {
	for _, tc := range []struct {
		name string
		draw int64
		wait time.Duration
	}{
		{"the lowest draw", 0, time.Second},
		{"the highest draw", int64(29 * time.Second), 30 * time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			signer := newEd25519Signer(t, testListKeyID)
			a := newCRLAuthority(t, discoveryFor(signer))
			now := time.Now().Truncate(time.Second)
			a.setList(signList(t, testList(now, revokedDID), signer))
			a.set(func(a *crlAuthority) { a.listStatus = http.StatusServiceUnavailable })
			c := newTestCRLClient(t, a, "", &now)
			s := newSteppedRefresher(c, tc.draw, func() {
				a.set(func(a *crlAuthority) { a.listStatus = http.StatusOK })
			})

			s.run(context.Background())

			requireWaits(t, s, tc.wait, 3*time.Minute)
			if len(s.bound) != 1 || s.bound[0] != int64(29*time.Second)+1 {
				t.Fatalf("the hold-down was drawn below %v, want one draw below %d", s.bound, int64(29*time.Second)+1)
			}
			requireAnswer(t, c, revokedDID, true)
			requireLines(t, s,
				"Revocation list: refresh failed (fetch): the list at "+a.srv.URL+testListPath+" answered 503; no list fit to answer from is held; next refresh in "+tc.wait.String(),
				"Revocation list: holds a list")
		})
	}
}

// A line is logged after the first refresh and when a refresh ends in
// another state than the one before it: a list held, or a refusal with its
// outcome and whether the list held is still fit to answer from.
func TestCRLRefresher_LogsWhenTheStateChanges(t *testing.T) {
	signer := newEd25519Signer(t, testListKeyID)
	a := newCRLAuthority(t, discoveryFor(signer))
	now := time.Now().Truncate(time.Second)
	a.setList(signList(t, testList(now, revokedDID), signer))
	c := newTestCRLClient(t, a, "", &now)
	fail := func() { a.set(func(a *crlAuthority) { a.listStatus = http.StatusBadGateway }) }
	s := newSteppedRefresher(c, 0,
		func() {}, // a second refresh holds the list again
		fail,      // the list answers 502
		func() {}, // and again, within 5 minutes of the last read
		func() { now = now.Add(5*time.Minute + time.Second) }, // and again, more than 5 minutes after it
		func() { // a new list is served
			a.setList(signList(t, testList(now, revokedDID), signer))
			a.set(func(a *crlAuthority) { a.listStatus = http.StatusOK })
		},
	)

	s.run(context.Background())

	if len(s.waits) != 6 {
		t.Fatalf("the refresher refreshed %d times, want 6", len(s.waits))
	}
	requireLines(t, s,
		"Revocation list: holds a list",
		"Revocation list: refresh failed (fetch): the list at "+a.srv.URL+testListPath+" answered 502; the list held is still answered from;",
		"Revocation list: refresh failed (fetch): the list at "+a.srv.URL+testListPath+" answered 502; no list fit to answer from is held;",
		"Revocation list: holds a list")
	requireAnswer(t, c, revokedDID, true)
}

// A list refused by a check is logged with the outcome of that check.
func TestCRLRefresher_LogsTheOutcomeOfARefusedList(t *testing.T) {
	signer := newEd25519Signer(t, testListKeyID)
	other := newEd25519Signer(t, testListKeyID)
	a := newCRLAuthority(t, discoveryFor(signer))
	now := time.Now().Truncate(time.Second)
	a.setList(signList(t, testList(now, revokedDID), other))
	c := newTestCRLClient(t, a, "", &now)
	s := newSteppedRefresher(c, 0)

	s.run(context.Background())

	requireLines(t, s, "Revocation list: refresh failed (signature): ")
	requireUnknown(t, c, revokedDID)
}

// Start refreshes at once, and Stop ends the refresher while it waits for
// its next refresh and while a refresh is in flight.
func TestCRLRefresher_StopEndsTheRefresher(t *testing.T) {
	t.Run("while it waits", func(t *testing.T) {
		signer := newEd25519Signer(t, testListKeyID)
		a := newCRLAuthority(t, discoveryFor(signer))
		a.setList(signList(t, testList(time.Now(), revokedDID), signer))
		c, err := NewCachedCRLClient(a.srv.URL, "", a.srv.Client())
		if err != nil {
			t.Fatal(err)
		}
		r := NewCRLRefresher(c)
		r.logf = func(string, ...any) {}
		r.Start()
		deadline := time.Now().Add(10 * time.Second)
		for {
			if revoked, err := c.IsRevoked(revokedDID); err == nil && revoked {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("no list was held 10 s after Start")
			}
			time.Sleep(10 * time.Millisecond)
		}
		requireStops(t, r)
	})

	t.Run("while a refresh is in flight", func(t *testing.T) {
		arrived := make(chan struct{}, 1)
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {
			case arrived <- struct{}{}:
			default:
			}
			<-r.Context().Done()
		}))
		t.Cleanup(srv.Close)
		c, err := NewCachedCRLClient(srv.URL, "", srv.Client())
		if err != nil {
			t.Fatal(err)
		}
		r := NewCRLRefresher(c)
		r.logf = func(format string, args ...any) { t.Errorf("a stopped refresh logged: "+format, args...) }
		r.Start()
		select {
		case <-arrived:
		case <-time.After(10 * time.Second):
			t.Fatal("the refresher sent no request 10 s after Start")
		}
		requireStops(t, r)
	})

	t.Run("a refresher that is nil or was not started", func(t *testing.T) {
		var nilRefresher *CRLRefresher
		requireStops(t, nilRefresher)
		c, err := NewCachedCRLClient("https://registry.example", "", nil)
		if err != nil {
			t.Fatal(err)
		}
		requireStops(t, NewCRLRefresher(c))
	})
}

func requireStops(t *testing.T, r *CRLRefresher) {
	t.Helper()
	stopped := make(chan struct{})
	go func() {
		r.Stop()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("Stop had not returned 5 s after it was called")
	}
}
