package integration

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// The server mounts StrictRateLimitMiddleware on the public, auth and OAuth
// route groups: ten requests a minute per client address with the production
// limits, one hundred under ENVIRONMENT=test. Each group has its own counter.
// The /api/v1/mcp-servers group runs RateLimitMiddleware before the JWT
// middleware sets the user, so that limiter keys every request by client
// address, signed in or not: one hundred a minute with the production limits,
// one thousand under ENVIRONMENT=test, and its counter is the one the response
// headers report. One run of this package sends more requests to the public
// group, and to the MCP server group, than one window admits, so every
// request from this package to one of those groups passes through
// strictLimitGate. The gate reads the X-RateLimit-Remaining
// and X-RateLimit-Reset headers the limiter writes on each response and, once
// the window is spent, holds the next request to that group until the window
// resets, instead of letting the server answer 429. A 429 can still arrive
// when the window was spent before this run began (a dashboard login from the
// same address, for example); the gate then waits the Retry-After the server
// names and sends that request once more.
//
// init puts the gate in front of http.DefaultTransport, so a test that calls
// http.Post or builds its own http.Client with a nil Transport is paced too.

// strictLimitGroups lists the path prefixes the gate paces, each a separate
// per-address counter on the server: the three groups the server limits with
// StrictRateLimitMiddleware, and the MCP server group.
var strictLimitGroups = []string{"/api/v1/public/", "/api/v1/auth/", "/api/v1/oauth/", "/api/v1/mcp-servers/"}

// strictLimitGroup returns the paced group a path belongs to, or "" when the
// path is not under one. The group's own path without the trailing slash
// (GET /api/v1/mcp-servers, the list) belongs to it too.
func strictLimitGroup(path string) string {
	for _, prefix := range strictLimitGroups {
		if strings.HasPrefix(path, prefix) || path == strings.TrimSuffix(prefix, "/") {
			return prefix
		}
	}
	return ""
}

type strictWindow struct {
	known     bool      // a response from this group has been read
	remaining int       // requests the current window still admits, after those already sent
	resetAt   time.Time // when the current window ends
}

type strictLimitGate struct {
	base   http.RoundTripper
	mu     sync.Mutex
	groups map[string]*strictWindow
	// Held is the total time the gate has held requests, for a test log line.
	held time.Duration
}

func newStrictLimitGate(base http.RoundTripper) *strictLimitGate {
	return &strictLimitGate{base: base, groups: map[string]*strictWindow{}}
}

// strictGate paces every request this package sends to a strict-limited group.
var strictGate = newStrictLimitGate(http.DefaultTransport)

func init() {
	http.DefaultTransport = strictGate
}

// resetMargin covers the limiter truncating X-RateLimit-Reset to whole seconds.
const resetMargin = time.Second

func (g *strictLimitGate) RoundTrip(req *http.Request) (*http.Response, error) {
	group := strictLimitGroup(req.URL.Path)
	if group == "" {
		return g.base.RoundTrip(req)
	}

	g.waitForWindow(group)
	resp, err := g.send(req)
	if err != nil {
		return nil, err
	}
	g.observe(group, resp)
	if resp.StatusCode != http.StatusTooManyRequests {
		return resp, nil
	}

	// The window was spent by requests this gate did not see. Wait for the
	// reset the server names, then send the same request once more.
	if req.Body != nil && req.GetBody == nil {
		return resp, nil
	}
	wait := retryAfter(resp)
	resp.Body.Close()
	g.hold(wait)
	if req.GetBody != nil {
		body, err := req.GetBody()
		if err != nil {
			return nil, err
		}
		req.Body = body
	}
	g.mu.Lock()
	delete(g.groups, group)
	g.mu.Unlock()
	g.waitForWindow(group)
	resp, err = g.send(req)
	if err != nil {
		return nil, err
	}
	g.observe(group, resp)
	return resp, nil
}

// attemptTimeout bounds one request on the wire, as TestContext's client does.
const attemptTimeout = 30 * time.Second

// send puts the request on the wire. The time the gate held it is not time
// the caller's http.Client.Timeout should count: a window can be a minute
// away and clients in this package allow as little as five seconds. By the
// time the hold ends the client's deadline has often passed, so the request
// goes out on a context of its own, bounded by attemptTimeout.
func (g *strictLimitGate) send(req *http.Request) (*http.Response, error) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(req.Context()), attemptTimeout)
	detached := req.Clone(ctx)
	detached.Cancel = nil //nolint:staticcheck // the client's timeout closes this channel; it must not end the detached request
	resp, err := g.base.RoundTrip(detached)
	if err != nil {
		cancel()
		return nil, err
	}
	resp.Body = &bodyReleasingContext{ReadCloser: resp.Body, cancel: cancel}
	return resp, nil
}

// bodyReleasingContext ends the detached request's context when the caller
// closes the body.
type bodyReleasingContext struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (b *bodyReleasingContext) Close() error {
	b.cancel()
	return b.ReadCloser.Close()
}

// waitForWindow returns once the group's current window admits one more
// request, and reserves that request.
func (g *strictLimitGate) waitForWindow(group string) {
	for {
		g.mu.Lock()
		w := g.groups[group]
		if w == nil {
			w = &strictWindow{}
			g.groups[group] = w
		}
		if !w.known || time.Now().After(w.resetAt) {
			// Nothing read yet, or the window has ended: send and learn from
			// the response.
			w.known = false
			g.mu.Unlock()
			return
		}
		if w.remaining > 0 {
			w.remaining--
			g.mu.Unlock()
			return
		}
		wait := time.Until(w.resetAt)
		g.mu.Unlock()
		g.hold(wait)
	}
}

// observe records what the limiter said about the group's window.
func (g *strictLimitGate) observe(group string, resp *http.Response) {
	remaining, okRemaining := headerInt(resp, "X-RateLimit-Remaining")
	reset, okReset := headerInt(resp, "X-RateLimit-Reset")
	if !okRemaining || !okReset {
		return
	}
	resetAt := time.Now().Add(time.Duration(reset)*time.Second + resetMargin)

	g.mu.Lock()
	defer g.mu.Unlock()
	w := g.groups[group]
	if w == nil {
		w = &strictWindow{}
		g.groups[group] = w
	}
	sameWindow := w.known && resetAt.Before(w.resetAt.Add(2*time.Second))
	if sameWindow && w.remaining < remaining {
		// Requests this gate reserved and sent, whose responses the server
		// has not written yet, are not in the header's count.
		remaining = w.remaining
	}
	w.known = true
	w.remaining = remaining
	w.resetAt = resetAt
}

func (g *strictLimitGate) hold(d time.Duration) {
	if d <= 0 {
		return
	}
	if d > 61*time.Second {
		d = 61 * time.Second
	}
	g.mu.Lock()
	g.held += d
	g.mu.Unlock()
	time.Sleep(d)
}

// Held reports how long the gate has held requests so far.
func (g *strictLimitGate) Held() time.Duration {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.held
}

// retryAfter is how long a 429 asks the client to wait: Retry-After, else
// X-RateLimit-Reset, else a full window.
func retryAfter(resp *http.Response) time.Duration {
	if seconds, ok := headerInt(resp, "Retry-After"); ok {
		return time.Duration(seconds)*time.Second + resetMargin
	}
	if seconds, ok := headerInt(resp, "X-RateLimit-Reset"); ok {
		return time.Duration(seconds)*time.Second + resetMargin
	}
	return time.Minute
}

func headerInt(resp *http.Response, name string) (int, bool) {
	value := resp.Header.Get(name)
	if value == "" {
		return 0, false
	}
	n, err := strconv.Atoi(value)
	if err != nil || n < 0 {
		return 0, false
	}
	return n, true
}
