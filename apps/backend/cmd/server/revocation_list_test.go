package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/config"
)

// The line the server logs for REGISTRY_URL and ATC_CRL_ENDPOINT, and the
// values that stop it. A line names the scheme and host of REGISTRY_URL, and
// the scheme, host and path of ATC_CRL_ENDPOINT: no user name and no query.
func TestRevocationListStartupLine(t *testing.T) {
	const (
		notRead       = "Revocation list: not read (REGISTRY_URL is not set)"
		overrideAlone = "Revocation list: not read (REGISTRY_URL is not set; ATC_CRL_ENDPOINT is read only with it)"
		badBase       = "REGISTRY_URL must be an https URL with a host"
		badOverride   = "ATC_CRL_ENDPOINT must be an https URL with a host"
	)
	for _, tc := range []struct {
		name, url, override string
		line, err           string
	}{
		{name: "neither is set", line: notRead},
		{name: "an override alone", override: "https://mirror.example.com/revocations", line: overrideAlone},
		{name: "an override over http alone", override: "http://mirror.example.com/revocations", line: overrideAlone},
		{name: "REGISTRY_URL", url: "https://registry.example.com",
			line: "Revocation list: read in the background from the authority at https://registry.example.com"},
		{name: "REGISTRY_URL with a port, a path and a query", url: "https://registry.example.com:8443/base?region=eu",
			line: "Revocation list: read in the background from the authority at https://registry.example.com:8443"},
		{name: "REGISTRY_URL with a user name", url: "https://reader@registry.example.com",
			line: "Revocation list: read in the background from the authority at https://registry.example.com"},
		{name: "REGISTRY_URL and an override", url: "https://registry.example.com", override: "https://reader@mirror.example.com/revocations?region=eu",
			line: "Revocation list: read in the background from https://mirror.example.com/revocations, with the keys of the authority at https://registry.example.com"},
		{name: "REGISTRY_URL over http", url: "http://registry.example.com", err: badBase},
		{name: "REGISTRY_URL with no host", url: "https:///.well-known/atp", err: badBase},
		{name: "REGISTRY_URL with no scheme", url: "registry.example.com", err: badBase},
		{name: "an override over http", url: "https://registry.example.com", override: "http://mirror.example.com/revocations", err: badOverride},
		{name: "an override with no host", url: "https://registry.example.com", override: "/api/v1/trust/revocations", err: badOverride},
	} {
		t.Run(tc.name, func(t *testing.T) {
			line, err := revocationListStartupLine(config.RegistryConfig{URL: tc.url, ATCCRLEndpoint: tc.override})
			if tc.err != "" {
				if err == nil || err.Error() != tc.err {
					t.Fatalf("revocationListStartupLine = %q, %v; want the error %q", line, err, tc.err)
				}
				return
			}
			if err != nil || line != tc.line {
				t.Fatalf("revocationListStartupLine = %q, %v; want %q", line, err, tc.line)
			}
		})
	}
}

// revocationListAuthority is a TLS stub of an authority listening on
// 127.0.0.1. Its discovery document at /.well-known/atp names
// /api/v1/trust/revocations and lists one Ed25519 key, and the list it serves
// there, signed by that key, revokes revoked.
type revocationListAuthority struct {
	srv *httptest.Server

	mu    sync.Mutex
	paths []string
}

func newRevocationListAuthority(t *testing.T, revoked string) *revocationListAuthority {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	const keyID = "did:web:registry.example#list-1"
	discovery, err := json.Marshal(map[string]any{
		"endpoints":  map[string]any{"revocations": "/api/v1/trust/revocations"},
		"publicKeys": []any{map[string]any{"keyId": keyID, "algorithm": "Ed25519", "publicKeyHex": hex.EncodeToString(public)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	// The list without its signatures, written in its RFC 8785 form: members
	// in code unit order, and strings with no character to escape.
	unsigned := fmt.Sprintf(`{"entryCount":1,"nextUpdate":%q,"revocations":[{"agentDid":%q,"revokedAt":%q}],"thisUpdate":%q}`,
		now.Add(time.Hour).UTC().Format(time.RFC3339), revoked,
		now.Add(-time.Hour).UTC().Format(time.RFC3339), now.Add(-time.Minute).UTC().Format(time.RFC3339))
	signature := base64.StdEncoding.EncodeToString(ed25519.Sign(private, []byte(unsigned)))
	list := strings.TrimSuffix(unsigned, "}") +
		fmt.Sprintf(`,"signatures":[{"algorithm":"Ed25519","keyId":%q,"value":%q}]}`, keyID, signature)

	a := &revocationListAuthority{}
	a.srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a.mu.Lock()
		a.paths = append(a.paths, r.URL.Path)
		a.mu.Unlock()
		switch r.URL.Path {
		case "/.well-known/atp":
			_, _ = w.Write(discovery)
		case "/api/v1/trust/revocations":
			_, _ = w.Write([]byte(list))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(a.srv.Close)
	return a
}

func (a *revocationListAuthority) requested() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]string(nil), a.paths...)
}

// lockedLog is a buffer the standard logger writes to while the test reads
// it, so a line logged from another goroutine, the refresher's or the stub
// server's, is no data race.
type lockedLog struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *lockedLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

// lines returns the lines logged so far that contain marker.
func (l *lockedLog) lines(marker string) []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	var lines []string
	for _, line := range strings.Split(l.buf.String(), "\n") {
		if strings.Contains(line, marker) {
			lines = append(lines, line)
		}
	}
	return lines
}

// captureLockedLog sends the standard logger's lines, with no flags, to a
// lockedLog until the test ends.
func captureLockedLog(t *testing.T) *lockedLog {
	t.Helper()
	logged := &lockedLog{}
	flags, out := log.Flags(), log.Writer()
	log.SetFlags(0)
	log.SetOutput(logged)
	t.Cleanup(func() {
		log.SetFlags(flags)
		log.SetOutput(out)
	})
	return logged
}

// With REGISTRY_URL set, the server logs the line for it, and holds the list
// the authority's discovery document names, read in the background.
func TestStartRevocationList_HoldsTheListOfTheAuthorityAtRegistryURL(t *testing.T) {
	const revoked = "did:web:agents.example:revoked-agent"
	a := newRevocationListAuthority(t, revoked)
	logged := captureLockedLog(t)

	refresher, err := startRevocationList(config.RegistryConfig{URL: a.srv.URL}, a.srv.Client())
	if err != nil || refresher == nil {
		t.Fatalf("startRevocationList = %v, %v; want a refresher", refresher, err)
	}
	defer refresher.Stop()

	deadline := time.Now().Add(10 * time.Second)
	for {
		isRevoked, err := refresher.Client().IsRevoked(revoked)
		if err == nil && isRevoked {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("10 s after the start the list answers %v, %v; want true, nil", isRevoked, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	refresher.Stop()

	if got := a.requested(); len(got) < 2 || got[0] != "/.well-known/atp" || got[1] != "/api/v1/trust/revocations" {
		t.Fatalf("the authority was asked for %v, want /.well-known/atp and then /api/v1/trust/revocations", got)
	}
	lines := logged.lines("Revocation list:")
	want := "Revocation list: read in the background from the authority at " + a.srv.URL
	if len(lines) != 2 || lines[0] != want || !strings.HasPrefix(lines[1], "Revocation list: holds a list, thisUpdate ") {
		t.Fatalf("the server logged %q, want %q and then the line of the list held", lines, want)
	}
}

// Without REGISTRY_URL the server starts no refresher, with or without
// ATC_CRL_ENDPOINT, and a value that is not an https URL stops it before a
// refresher starts.
func TestStartRevocationList_StartsNoRefresherWithoutAnHTTPSRegistryURL(t *testing.T) {
	for _, tc := range []struct {
		name     string
		registry config.RegistryConfig
		err      string
	}{
		{name: "neither is set"},
		{name: "an override alone", registry: config.RegistryConfig{ATCCRLEndpoint: "https://127.0.0.1:1/api/v1/trust/revocations"}},
		{name: "REGISTRY_URL over http", registry: config.RegistryConfig{URL: "http://127.0.0.1:1"}, err: "REGISTRY_URL must be an https URL with a host"},
		{name: "an override over http", registry: config.RegistryConfig{URL: "https://127.0.0.1:1", ATCCRLEndpoint: "http://127.0.0.1:1/revocations"},
			err: "ATC_CRL_ENDPOINT must be an https URL with a host"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			refresher, err := startRevocationList(tc.registry, nil)
			defer refresher.Stop()
			if refresher != nil {
				t.Fatal("startRevocationList started a refresher")
			}
			if got := fmt.Sprint(err); (tc.err == "" && err != nil) || (tc.err != "" && got != tc.err) {
				t.Fatalf("startRevocationList returned the error %v, want %q", err, tc.err)
			}
		})
	}
}
