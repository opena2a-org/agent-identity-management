package utils

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// admitLoopback is the production policy with loopback admitted, so a test can
// reach an httptest server. Every other class is still refused.
func admitLoopback(ip net.IP) (string, bool) {
	if ip.IsLoopback() {
		return "", false
	}
	return DeniedAddressClass(ip)
}

func countingServer(t *testing.T) (*httptest.Server, *int32) {
	t.Helper()
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

// localhostURL names a loopback test server by hostname, so the address is only
// known once the name is resolved at dial time.
func localhostURL(t *testing.T, srv *httptest.Server) string {
	t.Helper()
	_, port, err := net.SplitHostPort(srv.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	return "http://localhost:" + port + "/"
}

// containsIPLiteral reports whether s contains an IPv4 or IPv6 address, bare,
// bracketed or with a port.
func containsIPLiteral(s string) bool {
	tokens := strings.FieldsFunc(s, func(r rune) bool {
		return !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f' || r >= 'A' && r <= 'F' || r == ':' || r == '.' || r == '[' || r == ']')
	})
	for _, tok := range tokens {
		candidates := []string{tok, strings.Trim(tok, ":.[]")}
		if host, _, err := net.SplitHostPort(strings.TrimRight(tok, ":.")); err == nil {
			candidates = append(candidates, host)
		}
		for _, c := range candidates {
			if net.ParseIP(strings.Trim(c, "[]")) != nil {
				return true
			}
		}
	}
	return false
}

func TestEgressClient_DoesNotFollowRedirects(t *testing.T) {
	for _, code := range []int{http.StatusTemporaryRedirect, http.StatusFound} {
		target, targetHits := countingServer(t)
		redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, target.URL+"/latest/meta-data/", code)
		}))
		t.Cleanup(redirector.Close)

		resp, err := NewEgressClientWithPolicy(5*time.Second, admitLoopback).Get(redirector.URL)
		if err != nil {
			t.Fatalf("%d: egress client: %v", code, err)
		}
		resp.Body.Close()
		if resp.StatusCode != code {
			t.Errorf("%d: egress client returned status %d, want the redirect itself", code, resp.StatusCode)
		}
		if got := atomic.LoadInt32(targetHits); got != 0 {
			t.Errorf("%d: egress client requested the redirect target %d time(s), want 0", code, got)
		}

		// Control: a plain client against the same endpoint does reach the target.
		resp, err = (&http.Client{Transport: &http.Transport{}}).Get(redirector.URL)
		if err != nil {
			t.Fatalf("%d: plain client: %v", code, err)
		}
		resp.Body.Close()
		if got := atomic.LoadInt32(targetHits); got != 1 {
			t.Errorf("%d: control: plain client reached the target %d time(s), want 1", code, got)
		}
	}
}

func TestEgressClient_RefusesAddressResolvedAtDialTime(t *testing.T) {
	srv, hits := countingServer(t)
	url := localhostURL(t, srv)

	_, err := NewEgressClient(5 * time.Second).Get(url)
	var refused *EgressRefusedError
	if !errors.As(err, &refused) {
		t.Fatalf("want an EgressRefusedError for a name resolving to loopback, got %v", err)
	}
	if refused.Class != "loopback" {
		t.Errorf("refusal class = %q, want loopback", refused.Class)
	}
	if containsIPLiteral(err.Error()) {
		t.Errorf("refusal text carries an address: %q", err.Error())
	}
	if got := atomic.LoadInt32(hits); got != 0 {
		t.Errorf("loopback server was reached %d time(s), want 0", got)
	}

	// Control: the same request with loopback admitted reaches the server.
	resp, err := NewEgressClientWithPolicy(5*time.Second, admitLoopback).Get(url)
	if err != nil {
		t.Fatalf("control: %v", err)
	}
	resp.Body.Close()
	if got := atomic.LoadInt32(hits); got != 1 {
		t.Errorf("control: loopback server reached %d time(s), want 1", got)
	}
}

func TestDeniedAddressClass_ValidatorAndDialHookAgree(t *testing.T) {
	rows := []struct {
		addr   string
		denied bool
	}{
		// IPv4
		{"0.0.0.0", true},
		{"0.1.2.3", true},
		{"10.1.2.3", true},
		{"100.64.0.1", true},
		{"127.0.0.1", true},
		{"127.1.2.3", true},
		{"168.63.129.16", true},
		{"169.254.169.254", true},
		{"172.16.0.1", true},
		{"172.31.255.255", true},
		{"192.0.0.8", true},
		{"192.0.2.1", true},
		{"192.168.1.1", true},
		{"198.18.0.1", true},
		{"198.19.255.255", true},
		{"198.51.100.1", true},
		{"203.0.113.1", true},
		{"224.0.0.1", true},
		{"239.255.255.250", true},
		{"240.0.0.1", true},
		{"255.255.255.255", true},
		{"8.8.8.8", false},
		{"1.1.1.1", false},
		{"100.128.0.1", false},
		{"168.63.129.17", false},
		{"172.32.0.1", false},
		// IPv6
		{"::", true},
		{"::1", true},
		{"100::1", true},
		{"2001:0:4136:e378:8000:63bf:3fff:fdd2", true},
		{"2001:db8::1", true},
		{"fd00::1", true},
		{"fe80::1", true},
		{"fec0::1", true},
		{"ff02::1", true},
		{"2606:4700:4700::1111", false},
		{"2001:4860:4860::8888", false},
		// IPv6 forms carrying an IPv4 address are judged on that address.
		{"::ffff:127.0.0.1", true},
		{"::ffff:10.0.0.1", true},
		{"::ffff:169.254.169.254", true},
		{"::ffff:8.8.8.8", false},
		{"64:ff9b::7f00:1", true},
		{"64:ff9b::a9fe:a9fe", true},
		{"64:ff9b::808:808", false},
		{"64:ff9b:1::a00:1", true},
		{"64:ff9b:1:a00:0:100:808:808", true},
		{"64:ff9b:1:808:8:808:808:808", false},
		{"2002:7f00:1::", true},
		{"2002:a9fe:a9fe::1", true},
		{"2002:808:808::1", false},
	}

	hook := egressControl(DeniedAddressClass)
	for _, row := range rows {
		ip := net.ParseIP(row.addr)
		if ip == nil {
			t.Fatalf("bad test address %q", row.addr)
		}
		network := "tcp6"
		if ip.To4() != nil && !strings.Contains(row.addr, ":") {
			network = "tcp4"
		}
		hostPort := net.JoinHostPort(row.addr, "443")

		hookErr := hook(network, hostPort, nil)
		if (hookErr != nil) != row.denied {
			t.Errorf("dial hook %s: refused=%v, want %v (%v)", row.addr, hookErr != nil, row.denied, hookErr)
		}

		validateErr := ValidateExternalURL("https://" + hostPort + "/")
		if (validateErr != nil) != row.denied {
			t.Errorf("ValidateExternalURL %s: refused=%v, want %v (%v)", row.addr, validateErr != nil, row.denied, validateErr)
		}

		for _, err := range []error{hookErr, validateErr} {
			if err != nil && containsIPLiteral(err.Error()) {
				t.Errorf("%s: refusal text carries an address: %q", row.addr, err.Error())
			}
		}
	}
}

func TestEgressControl_RefusesNonTCPNetworks(t *testing.T) {
	if err := egressControl(DeniedAddressClass)("udp4", "8.8.8.8:53", nil); err == nil {
		t.Error("dial hook admitted a udp4 connection")
	}
}

// The standard library reads the proxy environment once per process, so the
// counting proxy outlives a single test run and keeps the same address.
var (
	proxyOnce    sync.Once
	proxyAddress string
	proxied      int32
)

func countingProxyURL() string {
	proxyOnce.Do(func() {
		proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&proxied, 1)
			w.WriteHeader(http.StatusOK)
		}))
		proxyAddress = proxy.URL
	})
	return proxyAddress
}

func TestEgressClient_IgnoresProxyEnvironment(t *testing.T) {
	proxyURL := countingProxyURL()
	atomic.StoreInt32(&proxied, 0)
	for _, name := range []string{"HTTP_PROXY", "HTTPS_PROXY", "http_proxy", "https_proxy"} {
		t.Setenv(name, proxyURL)
	}
	t.Setenv("NO_PROXY", "")
	t.Setenv("no_proxy", "")

	// A non-loopback destination: proxy selection skips loopback hosts. The
	// egress client refuses this documentation address itself, without network.
	const target = "http://192.0.2.10/"

	if resp, err := NewEgressClient(5 * time.Second).Get(target); err == nil {
		resp.Body.Close()
	}
	if got := atomic.LoadInt32(&proxied); got != 0 {
		t.Errorf("egress client sent %d request(s) through the environment proxy, want 0", got)
	}

	// Control: a transport that reads the proxy environment does use it. No
	// other test in this package uses http.DefaultTransport, so the environment
	// it reads is the one set above.
	resp, err := (&http.Client{Transport: http.DefaultTransport, Timeout: 5 * time.Second}).Get(target)
	if err != nil {
		t.Fatalf("control: %v", err)
	}
	resp.Body.Close()
	if got := atomic.LoadInt32(&proxied); got != 1 {
		t.Errorf("control: default transport sent %d request(s) through the proxy, want 1", got)
	}
}

func TestEgressErrors_NameNoAddress(t *testing.T) {
	planted := []error{
		&net.DNSError{Err: "no such host", Name: "internal.example", Server: "10.0.0.2:53", IsNotFound: true},
		&net.OpError{Op: "dial", Net: "tcp", Addr: &net.TCPAddr{IP: net.ParseIP("10.0.0.5"), Port: 443}, Err: errors.New("connect: connection refused")},
		&net.OpError{Op: "dial", Net: "tcp", Addr: &net.TCPAddr{IP: net.ParseIP("fd00::5"), Port: 443}, Err: &EgressRefusedError{Class: "private"}},
	}
	for _, err := range planted {
		// Control: the check sees the address in the error as the transport produced it.
		if !containsIPLiteral(err.Error()) {
			t.Fatalf("control: no address found in planted error %q", err.Error())
		}
		described := describeEgressError(err)
		if containsIPLiteral(described.Error()) {
			t.Errorf("described error carries an address: %q", described.Error())
		}
		if !errors.Is(described, err) {
			t.Errorf("described error lost its cause: %q", described.Error())
		}
	}

	for _, raw := range []string{"http://localhost/", "http://metadata.google.internal/", "http://0.0.0.0/", "http://169.254.169.254/"} {
		if err := ValidateExternalURL(raw); err == nil || containsIPLiteral(err.Error()) {
			t.Errorf("ValidateExternalURL(%q) = %v, want a refusal that names no address", raw, err)
		}
	}
}
