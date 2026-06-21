package registry

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func float64Ptr(v float64) *float64 { return &v }
func intPtr(v int) *int             { return &v }

func sampleRequest() ATCIssuanceRequest {
	return ATCIssuanceRequest{
		AgentID:      "11111111-1111-1111-1111-111111111111",
		AgentDID:     "did:aip:aim_11111111-1111-1111-1111-111111111111",
		Publisher:    "Acme Org",
		Version:      "agent-v1",
		ContentHash:  "sha256:abc",
		Capabilities: []string{"file:read"},
		TrustScore:   float64Ptr(0.82),
		TrustLevel:   intPtr(3),
		BehavioralProfile: &ATCBehavioralProfile{
			Checksum:        "sha256:deadbeef",
			GeneratedAt:     time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC),
			ObservationDays: 42,
		},
	}
}

func TestClient_IssueATC_Success(t *testing.T) {
	var gotAuth, gotCT, gotPath string
	var gotReq ATCIssuanceRequest

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotCT = r.Header.Get("Content-Type")
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		// Include signed TBS fields the AIM mirror struct does NOT model
		// (scanSummary, issuerChain, publisherDid) so the Raw-fidelity assertion
		// below proves the verbatim bytes — the ones a consumer verifies against —
		// survive the client even though the struct drops these fields.
		_, _ = w.Write([]byte(`{
			"id":"22222222-2222-2222-2222-222222222222",
			"atcVersion":"1.1",
			"agentId":"11111111-1111-1111-1111-111111111111",
			"agentDid":"did:aip:aim_11111111-1111-1111-1111-111111111111",
			"publisherDid":"did:web:registry.oa2a.org",
			"transparencyLogIndex":7,
			"scanSummary":{"hma":"pass","criticalFindings":0,"highFindings":0},
			"issuerChain":["did:web:registry.oa2a.org"],
			"trustScore":0.82,
			"trustLevel":3,
			"expiresAt":"2026-06-27T00:00:00Z",
			"signatures":[{"keyId":"k1","algorithm":"Ed25519","value":"sig"}]
		}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "secret-token", nil)
	cred, err := c.IssueATC(context.Background(), sampleRequest())
	if err != nil {
		t.Fatalf("IssueATC returned error: %v", err)
	}

	if gotPath != issueATCPath {
		t.Errorf("path = %q, want %q", gotPath, issueATCPath)
	}
	if gotAuth != "Bearer secret-token" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer secret-token")
	}
	if gotCT != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", gotCT)
	}
	if gotReq.AgentDID != sampleRequest().AgentDID {
		t.Errorf("agentDid round-trip = %q", gotReq.AgentDID)
	}
	if gotReq.TrustScore == nil || *gotReq.TrustScore != 0.82 {
		t.Errorf("trustScore not carried in request body: %+v", gotReq.TrustScore)
	}
	if cred.TransparencyLogIndex != 7 {
		t.Errorf("transparencyLogIndex = %d, want 7", cred.TransparencyLogIndex)
	}
	if cred.TrustLevel != 3 {
		t.Errorf("trustLevel = %d, want 3", cred.TrustLevel)
	}
	if len(cred.Raw) == 0 {
		t.Error("expected Raw to be populated with verbatim response bytes")
	}
	// Raw must preserve signed TBS fields the mirror struct does not model. The
	// handler returns cred.Raw (not the parsed struct) precisely so a consumer can
	// re-project the full TBS and verify the signature; if Raw dropped these, the
	// returned credential would be unverifiable.
	var rawObj map[string]json.RawMessage
	if err := json.Unmarshal(cred.Raw, &rawObj); err != nil {
		t.Fatalf("Raw is not valid JSON: %v", err)
	}
	for _, field := range []string{"scanSummary", "issuerChain", "publisherDid"} {
		if _, ok := rawObj[field]; !ok {
			t.Errorf("Raw is missing signed field %q that the mirror struct does not model; "+
				"returning the parsed struct instead of Raw would yield an unverifiable credential", field)
		}
	}
}

func TestClient_IssueATC_MissingToken(t *testing.T) {
	c := NewClient("https://example.invalid", "", nil)
	_, err := c.IssueATC(context.Background(), sampleRequest())
	if !errors.Is(err, ErrTokenNotConfigured) {
		t.Fatalf("expected ErrTokenNotConfigured, got %v", err)
	}
}

func TestClient_IssueATC_Non2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"insufficient_scope"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", nil)
	_, err := c.IssueATC(context.Background(), sampleRequest())
	if err == nil {
		t.Fatal("expected error on 403, got nil")
	}
}
