package utils

import (
	"errors"
	"net"
	"net/http"
	"syscall"
	"time"
)

// EgressRefusedError is returned when a request would connect to an address
// DeniedAddressClass refuses. Its text names the class only, never the address.
type EgressRefusedError struct {
	Class string
}

func (e *EgressRefusedError) Error() string {
	return "destination address is not allowed (" + e.Class + ")"
}

// defaultEgressTransport is shared by every client NewEgressClient returns, so
// deliveries reuse connections instead of each holding its own idle pool.
var defaultEgressTransport = newEgressTransport(DeniedAddressClass)

// NewEgressClient returns the HTTP client for every request the server sends
// to a URL a tenant or agent supplied (webhooks, MCP servers, agent cards).
//
//   - It never follows a redirect: a 3xx is returned as the response, and its
//     Location is never requested.
//   - It checks the address of every connection it opens, retries included,
//     against DeniedAddressClass after the name is resolved and before the
//     connection is made, so a name that resolves to an internal address after
//     ValidateExternalURL accepted it is still refused.
//   - It ignores HTTP_PROXY and HTTPS_PROXY, so the address checked is the
//     destination's own.
//   - Its errors name no resolved address and no resolver detail.
func NewEgressClient(timeout time.Duration) *http.Client {
	return egressClient(timeout, defaultEgressTransport)
}

// NewEgressClientWithPolicy is NewEgressClient with the address policy supplied
// by the caller. It exists so tests can admit a loopback test server; production
// code uses NewEgressClient.
func NewEgressClientWithPolicy(timeout time.Duration, denied func(net.IP) (string, bool)) *http.Client {
	return egressClient(timeout, newEgressTransport(denied))
}

func egressClient(timeout time.Duration, transport http.RoundTripper) *http.Client {
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		// A 3xx is the response. Following it would send the request to an
		// address the endpoint chose after the URL was accepted.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

func newEgressTransport(denied func(net.IP) (string, bool)) *egressTransport {
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
		Control:   egressControl(denied),
	}
	return &egressTransport{base: &http.Transport{
		Proxy:                 nil,
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}}
}

// egressControl runs after the dialer has resolved the name and created the
// socket, and before it connects, so the address it checks is the address the
// connection is made to.
func egressControl(denied func(net.IP) (string, bool)) func(network, address string, _ syscall.RawConn) error {
	return func(network, address string, _ syscall.RawConn) error {
		if network != "tcp4" && network != "tcp6" {
			return &EgressRefusedError{Class: "unsupported network"}
		}
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return &EgressRefusedError{Class: "unparseable address"}
		}
		ip := net.ParseIP(host)
		if ip == nil {
			return &EgressRefusedError{Class: "unparseable address"}
		}
		if class, refused := denied(ip); refused {
			return &EgressRefusedError{Class: class}
		}
		return nil
	}
}

// egressTransport replaces transport errors that carry a resolved address or
// resolver detail with text that names neither.
type egressTransport struct {
	base *http.Transport
}

func (t *egressTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if err != nil {
		return nil, describeEgressError(err)
	}
	return resp, nil
}

func (t *egressTransport) CloseIdleConnections() {
	t.base.CloseIdleConnections()
}

// egressError keeps the original error for errors.Is and errors.As while
// reporting text without addresses.
type egressError struct {
	msg   string
	cause error
}

func (e *egressError) Error() string { return e.msg }
func (e *egressError) Unwrap() error { return e.cause }

func (e *egressError) Timeout() bool {
	var ne net.Error
	return errors.As(e.cause, &ne) && ne.Timeout()
}

func describeEgressError(err error) error {
	var refused *EgressRefusedError
	if errors.As(err, &refused) {
		return &egressError{msg: refused.Error(), cause: err}
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return &egressError{msg: "destination host could not be resolved", cause: err}
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		if opErr.Timeout() {
			return &egressError{msg: "connection to destination timed out", cause: err}
		}
		return &egressError{msg: "connection to destination failed", cause: err}
	}
	return err
}
