package registry

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func float64Ptr(v float64) *float64 { return &v }
func intPtr(v int) *int             { return &v }

func sampleRequest() ATCIssuanceRequest {
	return ATCIssuanceRequest{
		AgentID:          "11111111-1111-1111-1111-111111111111",
		AgentDID:         "did:aip:aim_11111111-1111-1111-1111-111111111111",
		Publisher:        "Acme Org",
		PublisherDID:     "did:opena2a:publisher:aim_33333333-3333-3333-3333-333333333333",
		Version:          "agent-v1",
		ContentHash:      "sha256:abc",
		BuildAttestation: "https://aim.example.com/api/v1/did/did:aip:aim_11111111-1111-1111-1111-111111111111",
		Capabilities:     []string{"file:read"},
		TrustScore:       float64Ptr(82),
		TrustLevel:       intPtr(3),
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
			"trustScore":82,
			"trustLevel":3,
			"expiresAt":"2026-06-27T00:00:00Z",
			"signatures":[{"keyId":"k1","algorithm":"Ed25519","value":"sig"}]
		}`))
	}))
	defer srv.Close()

	c := NewATCClient(srv.URL, "secret-token", nil)
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
	if gotReq.TrustScore == nil || *gotReq.TrustScore != 82 {
		t.Errorf("trustScore not carried in request body: %+v", gotReq.TrustScore)
	}
	if gotReq.PublisherDID != sampleRequest().PublisherDID {
		t.Errorf("publisherDid not carried in request body: %q", gotReq.PublisherDID)
	}
	if gotReq.BuildAttestation != sampleRequest().BuildAttestation {
		t.Errorf("buildAttestation not carried in request body: %q", gotReq.BuildAttestation)
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
	c := NewATCClient("https://example.invalid", "", nil)
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

	c := NewATCClient(srv.URL, "tok", nil)
	_, err := c.IssueATC(context.Background(), sampleRequest())
	if err == nil {
		t.Fatal("expected error on 403, got nil")
	}
}

func TestClient_RevokeATC_PostsTheReasonToTheAgentsRevokeRoute(t *testing.T) {
	const agentID = "11111111-1111-1111-1111-111111111111"
	var gotMethod, gotPath, gotAuth, gotCT string
	var gotBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.EscapedPath()
		gotAuth = r.Header.Get("Authorization")
		gotCT = r.Header.Get("Content-Type")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode request body: %v", err)
		}
		// The Registry's answer to a stored revocation.
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":"ATC revoked","agentId":"` + agentID + `","reason":"revoked"}`))
	}))
	defer srv.Close()

	c := NewATCClient(srv.URL, "not-a-real-token", nil)
	if err := c.RevokeATC(context.Background(), agentID, "revoked"); err != nil {
		t.Fatalf("RevokeATC returned error: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if want := "/internal/atc/" + agentID + "/revoke"; gotPath != want {
		t.Errorf("path = %q, want %q", gotPath, want)
	}
	if gotAuth != "Bearer not-a-real-token" {
		t.Errorf("Authorization = %q, want the service-account bearer", gotAuth)
	}
	if gotCT != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", gotCT)
	}
	if len(gotBody) != 1 || gotBody["reason"] != "revoked" {
		t.Errorf("body = %v, want only the reason", gotBody)
	}
}

// The agent ID is one path segment: an ID holding a slash cannot name another
// Registry route.
func TestClient_RevokeATC_EscapesTheAgentID(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if err := NewATCClient(srv.URL, "not-a-real-token", nil).RevokeATC(context.Background(), "../issue", "revoked"); err != nil {
		t.Fatalf("RevokeATC returned error: %v", err)
	}
	if want := "/internal/atc/..%2Fissue/revoke"; gotPath != want {
		t.Errorf("path = %q, want %q", gotPath, want)
	}
}

func TestClient_RevokeATC_SendsNothingWithoutATokenOrAnAgentID(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	err := NewATCClient(srv.URL, "", nil).RevokeATC(context.Background(), "11111111-1111-1111-1111-111111111111", "revoked")
	if !errors.Is(err, ErrTokenNotConfigured) {
		t.Errorf("without a token: got %v, want ErrTokenNotConfigured", err)
	}
	if err := NewATCClient(srv.URL, "not-a-real-token", nil).RevokeATC(context.Background(), "", "revoked"); err == nil {
		t.Error("without an agent ID: got nil, want an error")
	}
	if n := requests.Load(); n != 0 {
		t.Errorf("the Registry received %d requests, want 0", n)
	}
}

// Configured reports the token RevokeATC and IssueATC need, so the server
// stores no revocation request a client without one would not send.
func TestClient_ConfiguredReportsTheToken(t *testing.T) {
	for _, tc := range []struct {
		name  string
		token string
		want  bool
	}{
		{"a token", "not-a-real-token", true},
		{"no token", "", false},
		{"only spaces", "   ", false},
	} {
		if got := NewATCClient("https://registry.example", tc.token, nil).Configured(); got != tc.want {
			t.Errorf("%s: Configured() = %t, want %t", tc.name, got, tc.want)
		}
	}

	t.Setenv("REGISTRY_ATC_TOKEN", "")
	if NewATCClientFromEnv().Configured() {
		t.Error("REGISTRY_ATC_TOKEN empty: Configured() = true, want false")
	}
	t.Setenv("REGISTRY_ATC_TOKEN", "not-a-real-token")
	if !NewATCClientFromEnv().Configured() {
		t.Error("REGISTRY_ATC_TOKEN set: Configured() = false, want true")
	}

	var unset *ATCClient
	if unset.Configured() {
		t.Error("a nil client: Configured() = true, want false")
	}
}

func TestClient_RevokeATC_Non2xxIsAStatusError(t *testing.T) {
	for _, tc := range []struct {
		status    int
		retryable bool
	}{
		{http.StatusBadRequest, false},
		{http.StatusUnauthorized, false},
		{http.StatusForbidden, false},
		{http.StatusNotFound, false},
		{http.StatusRequestTimeout, true},
		{http.StatusTooManyRequests, true},
		{http.StatusInternalServerError, true},
		{http.StatusBadGateway, true},
		{http.StatusServiceUnavailable, true},
	} {
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(`{"error":"Failed to revoke ATC"}`))
			}))
			defer srv.Close()

			err := NewATCClient(srv.URL, "not-a-real-token", nil).RevokeATC(context.Background(), "11111111-1111-1111-1111-111111111111", "revoked")
			var statusErr *ATCStatusError
			if !errors.As(err, &statusErr) {
				t.Fatalf("got %v, want an *ATCStatusError", err)
			}
			if statusErr.StatusCode != tc.status {
				t.Errorf("StatusCode = %d, want %d", statusErr.StatusCode, tc.status)
			}
			if statusErr.Retryable() != tc.retryable {
				t.Errorf("Retryable() = %v, want %v", statusErr.Retryable(), tc.retryable)
			}
		})
	}
}
