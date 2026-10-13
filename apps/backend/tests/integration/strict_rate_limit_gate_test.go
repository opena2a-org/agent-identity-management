package integration

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests need no backend: they run the gate against a local server that
// answers like the backend's strict limiter.

// fixedWindowServer admits max requests per window on a strict-limited path,
// writes X-RateLimit-Limit, X-RateLimit-Remaining and X-RateLimit-Reset on
// each admitted response, and answers 429 with Retry-After once the window is
// spent, as the backend's limiter does. Other paths are not counted.
type fixedWindowServer struct {
	mu        sync.Mutex
	max       int
	window    time.Duration
	windowEnd time.Time
	hits      int

	served   int // requests on strict-limited paths
	limited  int // of those, answered 429
	lastBody int // bytes in the body of the last admitted request
}

func (s *fixedWindowServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strictLimitGroup(r.URL.Path) == "" {
		w.WriteHeader(http.StatusOK)
		return
	}
	body, _ := io.ReadAll(r.Body)

	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if now.After(s.windowEnd) {
		s.windowEnd = now.Add(s.window)
		s.hits = 0
	}
	s.served++
	s.hits++
	remaining := s.max - s.hits
	// Whole seconds, truncated, as the limiter writes it.
	reset := strconv.Itoa(int(time.Until(s.windowEnd).Seconds()))
	if remaining < 0 {
		s.limited++
		w.Header().Set("Retry-After", reset)
		w.WriteHeader(http.StatusTooManyRequests)
		return
	}
	s.lastBody = len(body)
	w.Header().Set("X-RateLimit-Limit", strconv.Itoa(s.max))
	w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
	w.Header().Set("X-RateLimit-Reset", reset)
	w.WriteHeader(http.StatusOK)
}

func (s *fixedWindowServer) counts() (served, limited, lastBody int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.served, s.limited, s.lastBody
}

func gatedClient(srv *httptest.Server) *http.Client {
	return &http.Client{Transport: newStrictLimitGate(srv.Client().Transport)}
}

func postLogin(t *testing.T, client *http.Client, url string) int {
	t.Helper()
	resp, err := client.Post(url+"/api/v1/public/login", "application/json", bytes.NewReader([]byte(`{"email":"a@example.com"}`)))
	require.NoError(t, err)
	resp.Body.Close()
	return resp.StatusCode
}

// Without the gate the fixture refuses requests once the window is spent,
// which is what the other tests here prove the gate prevents.
func TestStrictLimitGate_FixtureRefusesWithoutTheGate(t *testing.T) {
	limiter := &fixedWindowServer{max: 3, window: 5 * time.Second}
	srv := httptest.NewServer(limiter)
	defer srv.Close()

	var statuses []int
	for i := 0; i < 5; i++ {
		statuses = append(statuses, postLogin(t, srv.Client(), srv.URL))
	}
	assert.Equal(t, []int{200, 200, 200, 429, 429}, statuses)
}

// Eight requests at three a window are answered 200 every time: the gate holds
// the fourth and the seventh until the window resets.
func TestStrictLimitGate_HoldsRequestsSoNoneIsRefused(t *testing.T) {
	limiter := &fixedWindowServer{max: 3, window: time.Second}
	srv := httptest.NewServer(limiter)
	defer srv.Close()
	client := gatedClient(srv)

	start := time.Now()
	for i := 0; i < 8; i++ {
		assert.Equal(t, http.StatusOK, postLogin(t, client, srv.URL), "request %d", i+1)
	}
	elapsed := time.Since(start)

	served, limited, _ := limiter.counts()
	assert.Equal(t, 8, served)
	assert.Equal(t, 0, limited)
	assert.GreaterOrEqual(t, elapsed, 2*time.Second, "eight requests at three a window span three windows")
}

// A window spent before this run began is not visible in any header the gate
// has read. The first request is refused; the gate waits the Retry-After the
// server names and sends the same request, body included, once more.
func TestStrictLimitGate_RetriesOnceAfterARefusalFromAWindowItDidNotSee(t *testing.T) {
	limiter := &fixedWindowServer{max: 2, window: time.Second}
	limiter.windowEnd = time.Now().Add(time.Second)
	limiter.hits = 2
	srv := httptest.NewServer(limiter)
	defer srv.Close()
	client := gatedClient(srv)

	assert.Equal(t, http.StatusOK, postLogin(t, client, srv.URL))

	served, limited, lastBody := limiter.counts()
	assert.Equal(t, 2, served, "the refused request and its retry")
	assert.Equal(t, 1, limited)
	assert.Equal(t, len(`{"email":"a@example.com"}`), lastBody, "the retry carries the body")
}

// A hold is not time the caller's http.Client.Timeout counts: a client that
// allows one second gets its answer after a two-second hold.
func TestStrictLimitGate_AHoldOutlastsTheClientsTimeout(t *testing.T) {
	limiter := &fixedWindowServer{max: 1, window: 2 * time.Second}
	srv := httptest.NewServer(limiter)
	defer srv.Close()
	client := &http.Client{Transport: newStrictLimitGate(srv.Client().Transport), Timeout: time.Second}

	assert.Equal(t, http.StatusOK, postLogin(t, client, srv.URL))
	start := time.Now()
	assert.Equal(t, http.StatusOK, postLogin(t, client, srv.URL))
	assert.GreaterOrEqual(t, time.Since(start), time.Second, "the second request waited for the next window")

	served, limited, _ := limiter.counts()
	assert.Equal(t, 2, served)
	assert.Equal(t, 0, limited)
}

// Each strict-limited group has its own counter on the server, so a spent
// public window does not hold a request to the auth group.
func TestStrictLimitGate_TracksEachGroupOnItsOwn(t *testing.T) {
	limiter := &fixedWindowServer{max: 1, window: 5 * time.Second}
	srv := httptest.NewServer(limiter)
	defer srv.Close()
	gate := newStrictLimitGate(srv.Client().Transport)
	client := &http.Client{Transport: gate}

	assert.Equal(t, http.StatusOK, postLogin(t, client, srv.URL))
	// The fixture counts all strict paths together, so the auth request below
	// would be the second hit of the same window on the server; a new window is
	// started for it to keep the fixture's answer at 200.
	limiter.mu.Lock()
	limiter.hits = 0
	limiter.mu.Unlock()

	start := time.Now()
	resp, err := client.Post(srv.URL+"/api/v1/auth/logout", "application/json", nil)
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Less(t, time.Since(start), time.Second, "the auth group's window is not the public group's")
	assert.Equal(t, time.Duration(0), gate.Held())
}

// Routes outside the strict-limited groups are passed through unchanged.
func TestStrictLimitGate_LeavesOtherRoutesAlone(t *testing.T) {
	limiter := &fixedWindowServer{max: 1, window: 5 * time.Second}
	srv := httptest.NewServer(limiter)
	defer srv.Close()
	gate := newStrictLimitGate(srv.Client().Transport)
	client := &http.Client{Transport: gate}

	start := time.Now()
	for i := 0; i < 5; i++ {
		resp, err := client.Get(srv.URL + "/health")
		require.NoError(t, err)
		resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	}
	assert.Less(t, time.Since(start), time.Second)
	assert.Equal(t, time.Duration(0), gate.Held())
	served, _, _ := limiter.counts()
	assert.Equal(t, 0, served)
}

// The package's own default transport is the gate, so a test that calls
// http.Post is paced like one that goes through TestContext.
func TestStrictLimitGate_IsThePackageDefaultTransport(t *testing.T) {
	gate, ok := http.DefaultTransport.(*strictLimitGate)
	require.True(t, ok, "http.DefaultTransport is the strict limit gate")
	assert.Same(t, strictGate, gate)
}
