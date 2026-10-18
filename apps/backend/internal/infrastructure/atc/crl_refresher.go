package atc

import (
	"context"
	"errors"
	"log"
	"math/rand/v2"
	"sync"
	"time"
)

// The schedule of a CRLRefresher.
const (
	// crlRefreshInterval is the longest wait after a refresh that holds a
	// list or reads the list held again.
	crlRefreshInterval = 4 * time.Minute

	// crlRefreshAhead is how long before the nextUpdate of the list held the
	// next refresh starts, when that is sooner than crlRefreshInterval.
	crlRefreshAhead = 60 * time.Second

	// crlHoldDownMin and crlHoldDownMax bound the hold-down: the wait after a
	// refresh that fails or whose list is refused, drawn for each wait.
	crlHoldDownMin = 1 * time.Second
	crlHoldDownMax = 30 * time.Second

	// crlRefreshTimeout bounds one refresh.
	crlRefreshTimeout = 30 * time.Second

	// crlStallAfter is how long after the end of the last refresh, or after
	// Start while no refresh has ended, State reads the refresher as stalled:
	// two refresh intervals. A refresher that runs ends a refresh at most
	// crlRefreshInterval plus crlRefreshTimeout after the one before.
	crlStallAfter = 2 * crlRefreshInterval
)

// Reason words of State besides the outcomes of a refresh.
const (
	// CRLReasonNoList: no list is held, and no refresh has ended, or the last
	// one ended with no answer before its deadline (CRLOutcomeFetch).
	CRLReasonNoList = "noList"
	// CRLReasonRefresherStalled: no refresh has ended for crlStallAfter, or
	// the refresher was not started.
	CRLReasonRefresherStalled = "refresherStalled"
)

// CRLRefresher refreshes the list of a CachedCRLClient from one goroutine,
// whose first refresh starts when Start is called. After a refresh that holds
// a list or reads the list held again, the next refresh starts
// crlRefreshInterval later, or crlRefreshAhead before the nextUpdate of the
// list held when that is sooner. After a refresh that fails or whose list is
// refused, and when the next refresh would start in less than crlHoldDownMin,
// it starts after a hold-down drawn from crlHoldDownMin to crlHoldDownMax. No
// request waits for a refresh: IsRevoked reads only the list held, and State
// reads the list held and the end and outcome of the last refresh.
//
// A line is logged after the first refresh and after each refresh that ends
// in another state than the one before it. The state is the outcome of a
// refresh that fails or whose list is refused, or none, and whether the list
// held is fit to answer from.
type CRLRefresher struct {
	client *CachedCRLClient

	logf   func(format string, args ...any)
	jitter func(n int64) int64 // a draw from 0 to n-1
	sleep  func(ctx context.Context, d time.Duration) bool

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}

	// started is when Start was called, and ended when the last refresh
	// ended, both by the client's clock; outcome is the outcome of the last
	// refresh, empty after one that held a list or read it again. Guarded by
	// mu.
	started time.Time
	ended   time.Time
	outcome string
}

// NewCRLRefresher returns a refresher of client. It refreshes nothing until
// Start is called.
func NewCRLRefresher(client *CachedCRLClient) *CRLRefresher {
	return &CRLRefresher{client: client, logf: log.Printf, jitter: rand.Int64N, sleep: sleepContext}
}

// Client returns the client the refresher refreshes.
func (r *CRLRefresher) Client() *CachedCRLClient { return r.client }

// Start starts the refresher's goroutine. A second call does nothing.
func (r *CRLRefresher) Start() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.done != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	r.cancel, r.done = cancel, done
	r.started = r.client.now()
	go func() {
		defer close(done)
		r.run(ctx)
	}()
}

// Stop cancels a refresh in flight and the wait for the next one, and returns
// once the refresher's goroutine has ended. It does nothing on a nil
// refresher or on one that was not started.
func (r *CRLRefresher) Stop() {
	if r == nil {
		return
	}
	r.mu.Lock()
	cancel, done := r.cancel, r.done
	r.mu.Unlock()
	if done == nil {
		return
	}
	cancel()
	<-done
}

// crlRefreshState is the state a refresh ended in. outcome is empty after a
// refresh that held a list or read the list held again, and is the outcome of
// the refusal otherwise; answerable tells whether the list held is fit to
// answer from.
type crlRefreshState struct {
	outcome    string
	answerable bool
}

// run refreshes until ctx ends.
func (r *CRLRefresher) run(ctx context.Context) {
	var last *crlRefreshState
	for {
		err := r.refresh(ctx)
		if ctx.Err() != nil {
			return
		}
		held, now := r.client.held.Load(), r.client.now()
		state := crlRefreshState{answerable: answerable(held, now) == nil}
		var wait time.Duration
		if err != nil {
			state.outcome = refreshOutcome(err)
			wait = r.holdDown()
		} else {
			wait = r.afterHeld(held, now)
		}
		r.mu.Lock()
		r.ended, r.outcome = now, state.outcome
		r.mu.Unlock()
		if last == nil || state != *last {
			r.logState(state, held, err, wait)
		}
		last = &state
		if !r.sleep(ctx, wait) {
			return
		}
	}
}

// State reports whether the list held is fit to answer from, as IsRevoked
// reads it, and when it is not, one word for why: refresherStalled when no
// refresh has ended for crlStallAfter, counted from Start until the first one
// ends, or when the refresher was not started; otherwise the outcome of the
// last refresh when it held no new list and that outcome is not fetch, one of
// status, timeout, parse, shape, signature, rollback and expired; expired
// when a list is held; and noList when none is. It reads no address.
func (r *CRLRefresher) State() (bool, string) {
	now := r.client.now()
	held := r.client.held.Load()
	if answerable(held, now) == nil {
		return true, ""
	}
	r.mu.Lock()
	since, outcome := r.ended, r.outcome
	if since.IsZero() {
		since = r.started
	}
	r.mu.Unlock()
	switch {
	case since.IsZero() || now.Sub(since) > crlStallAfter:
		return false, CRLReasonRefresherStalled
	case outcome != "" && outcome != CRLOutcomeFetch:
		return false, outcome
	case held != nil:
		return false, CRLOutcomeExpired
	default:
		return false, CRLReasonNoList
	}
}

// refresh runs one refresh, bounded by crlRefreshTimeout.
func (r *CRLRefresher) refresh(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, crlRefreshTimeout)
	defer cancel()
	return r.client.Refresh(ctx)
}

// afterHeld returns the wait after a refresh, ended at now, that held list or
// read it again.
func (r *CRLRefresher) afterHeld(list *crlList, now time.Time) time.Duration {
	if list == nil {
		return r.holdDown()
	}
	wait := crlRefreshInterval
	if ahead := list.nextUpdate.Add(-crlRefreshAhead).Sub(now); ahead < wait {
		wait = ahead
	}
	if wait < crlHoldDownMin {
		return r.holdDown()
	}
	return wait
}

// holdDown draws a wait from crlHoldDownMin to crlHoldDownMax.
func (r *CRLRefresher) holdDown() time.Duration {
	return crlHoldDownMin + time.Duration(r.jitter(int64(crlHoldDownMax-crlHoldDownMin)+1))
}

// refreshOutcome returns the outcome a refresh error names. Refresh returns
// only a *CRLRefreshError; any other error is counted as a fetch.
func refreshOutcome(err error) string {
	var refreshErr *CRLRefreshError
	if errors.As(err, &refreshErr) {
		return refreshErr.Outcome
	}
	return CRLOutcomeFetch
}

// logState logs the state a refresh ended in, and the wait before the next.
func (r *CRLRefresher) logState(state crlRefreshState, held *crlList, err error, wait time.Duration) {
	switch {
	case err == nil && state.answerable:
		r.logf("Revocation list: holds a list, thisUpdate %s, nextUpdate %s; next refresh in %s",
			held.thisUpdate.UTC().Format(time.RFC3339), held.nextUpdate.UTC().Format(time.RFC3339), wait)
	case err == nil:
		r.logf("Revocation list: read the list held again; no list fit to answer from is held; next refresh in %s", wait)
	default:
		cause := err
		var refreshErr *CRLRefreshError
		if errors.As(err, &refreshErr) {
			cause = refreshErr.Err
		}
		holding := "no list fit to answer from is held"
		if state.answerable {
			holding = "the list held is still answered from"
		}
		r.logf("Revocation list: refresh failed (%s): %v; %s; next refresh in %s", state.outcome, cause, holding, wait)
	}
}

// sleepContext waits d, and returns false when ctx ends first.
func sleepContext(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
