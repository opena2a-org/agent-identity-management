package registry

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// conformanceFixture is the subset of an atx-conformance fixture the issuance
// byte-match test needs: the pinned credential bytes and the expected outcome.
type conformanceFixture struct {
	Name     string `json:"name"`
	Expected struct {
		VerifyResult string `json:"verifyResult"`
	} `json:"expected"`
	ATX json.RawMessage `json:"atx"`
}

// TestATCIssuance_ConformanceByteMatch replays every pinned atx-conformance
// credential as a Registry issuance response and asserts AIM's issuance path
// is byte-preserving: the credential surfaced to callers via Raw must be the
// Registry's exact signed bytes, or downstream local verifiers
// (@opena2a/atx-verify, the Java LocalAtxVerifier, pkg/atcverify) would see a
// signature over bytes the verifier can no longer reproduce.
//
// The suite directory is injected via ATX_CONFORMANCE_DIR (CI clones
// opena2a-standards/atx-conformance at a pinned ref and verifies
// MANIFEST.sha256 first); the test skips when unset so local `go test ./...`
// does not require the suite checkout.
func TestATCIssuance_ConformanceByteMatch(t *testing.T) {
	dir := os.Getenv("ATX_CONFORMANCE_DIR")
	if dir == "" {
		t.Skip("ATX_CONFORMANCE_DIR not set; conformance byte-match runs in CI")
	}

	paths, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		t.Fatalf("glob fixtures: %v", err)
	}
	if len(paths) == 0 {
		t.Fatalf("no fixtures found in %s", dir)
	}

	for _, path := range paths {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			var fixture conformanceFixture
			if err := json.Unmarshal(raw, &fixture); err != nil {
				t.Fatalf("decode fixture: %v", err)
			}
			if len(fixture.ATX) == 0 {
				t.Fatalf("fixture %s has no atx payload", fixture.Name)
			}

			// Serve the fixture credential as the Registry's issuance response.
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != issueATCPath {
					t.Errorf("unexpected path %s", r.URL.Path)
					http.NotFound(w, r)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(fixture.ATX)
			}))
			defer srv.Close()

			client := NewATCClient(srv.URL, "test-token", srv.Client())
			cred, err := client.IssueATC(context.Background(), ATCIssuanceRequest{
				AgentID: "conformance", AgentDID: "did:aip:aim_conformance",
			})

			// The client must decode every pinned credential shape — including
			// the REJECT fixtures, whose defects (expiry, tampered signature,
			// untrusted issuer) are verification-time findings, not JSON
			// malformations. malformed-schema.json is still valid JSON.
			if err != nil {
				t.Fatalf("IssueATC failed on pinned credential %s: %v", fixture.Name, err)
			}

			// Byte-match: Raw is the verbatim signed credential.
			if !bytes.Equal(cred.Raw, fixture.ATX) {
				t.Fatalf("issuance did not preserve the Registry's signed bytes for %s", fixture.Name)
			}

			// Decode fidelity: the convenience fields AIM surfaces must agree
			// with the pinned credential, so handlers never contradict Raw.
			var want struct {
				ID           string   `json:"id"`
				ATCVersion   string   `json:"atcVersion"`
				AgentDID     string   `json:"agentDid"`
				TrustScore   float64  `json:"trustScore"`
				TrustLevel   int      `json:"trustLevel"`
				Capabilities []string `json:"capabilities"`
			}
			if err := json.Unmarshal(fixture.ATX, &want); err != nil {
				t.Fatalf("decode pinned credential: %v", err)
			}
			if cred.ID != want.ID || cred.ATCVersion != want.ATCVersion || cred.AgentDID != want.AgentDID {
				t.Fatalf("decoded identity fields diverge from pinned credential %s: got (%s,%s,%s)",
					fixture.Name, cred.ID, cred.ATCVersion, cred.AgentDID)
			}
			if cred.TrustScore != want.TrustScore || cred.TrustLevel != want.TrustLevel {
				t.Fatalf("decoded trust fields diverge from pinned credential %s: got (%v,%d) want (%v,%d)",
					fixture.Name, cred.TrustScore, cred.TrustLevel, want.TrustScore, want.TrustLevel)
			}
			if len(cred.Capabilities) != len(want.Capabilities) {
				t.Fatalf("decoded capabilities diverge from pinned credential %s: got %d want %d",
					fixture.Name, len(cred.Capabilities), len(want.Capabilities))
			}
		})
	}
}
