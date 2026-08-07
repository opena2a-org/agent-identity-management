package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/registry"
)

func TestATCTrustLevel(t *testing.T) {
	cases := []struct {
		score float64
		want  int
	}{
		{0.95, 4},
		{0.90, 4},
		{0.89, 3},
		{0.75, 3},
		{0.74, 2},
		{0.50, 2},
		{0.49, 1},
		{0.25, 1},
		{0.24, 0},
		{0.0, 0},
	}
	for _, c := range cases {
		if got := atcTrustLevel(c.score); got != c.want {
			t.Errorf("atcTrustLevel(%v) = %d, want %d", c.score, got, c.want)
		}
	}
}

func TestATCObservationDays(t *testing.T) {
	now := time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)
	if got := atcObservationDays(now.Add(-72*time.Hour), now); got != 3 {
		t.Errorf("observationDays = %d, want 3", got)
	}
	if got := atcObservationDays(time.Time{}, now); got != 0 {
		t.Errorf("zero createdAt should give 0, got %d", got)
	}
	if got := atcObservationDays(now.Add(24*time.Hour), now); got != 0 {
		t.Errorf("future createdAt should clamp to 0, got %d", got)
	}
}

func TestATCContentHash_PublicKeyVsFallback(t *testing.T) {
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	// No public key -> hash of the agent ID bytes.
	noKey := &domain.Agent{ID: id}
	idSum := sha256.Sum256(id[:])
	wantFallback := "sha256:" + hex.EncodeToString(idSum[:])
	if got := atcContentHash(noKey); got != wantFallback {
		t.Errorf("fallback contentHash = %q, want %q", got, wantFallback)
	}

	// With a public key, the hash must differ from the fallback.
	pk := "dGhpcyBpcyBhIGZha2Uga2V5" // base64
	withKey := &domain.Agent{ID: id, PublicKey: &pk}
	if got := atcContentHash(withKey); got == wantFallback {
		t.Error("contentHash with public key should differ from ID fallback")
	}
}

func TestATCBehavioralChecksum_Deterministic(t *testing.T) {
	f := domain.TrustScoreFactors{VerificationStatus: 1, Uptime: 0.9, Compliance: 0.5}
	a := atcBehavioralChecksum(f)
	b := atcBehavioralChecksum(f)
	if a != b {
		t.Errorf("checksum not deterministic: %q vs %q", a, b)
	}
	raw, _ := json.Marshal(f)
	sum := sha256.Sum256(raw)
	if want := "sha256:" + hex.EncodeToString(sum[:]); a != want {
		t.Errorf("checksum = %q, want %q", a, want)
	}
	// A different breakdown must produce a different checksum.
	if atcBehavioralChecksum(domain.TrustScoreFactors{VerificationStatus: 0.1}) == a {
		t.Error("different factors should produce different checksum")
	}
}

func TestBuildATCIssuanceRequest(t *testing.T) {
	id := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	now := time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)
	agent := &domain.Agent{
		ID:           id,
		Version:      "", // exercise default
		Capabilities: []string{"file:read", "api:call"},
		CreatedAt:    now.Add(-240 * time.Hour), // 10 days
	}
	score := &domain.TrustScore{
		Score:          0.82,
		Confidence:     0.7,
		LastCalculated: now.Add(-time.Hour),
		Factors:        domain.TrustScoreFactors{VerificationStatus: 1},
	}

	req := buildATCIssuanceRequest(agent, score, "Acme Org", now)

	if req.AgentDID != "did:aip:aim_"+id.String() {
		t.Errorf("agentDid = %q", req.AgentDID)
	}
	if req.Version != defaultATCVersion {
		t.Errorf("version = %q, want default %q", req.Version, defaultATCVersion)
	}
	if req.Publisher != "Acme Org" {
		t.Errorf("publisher = %q", req.Publisher)
	}
	if req.TrustScore == nil || *req.TrustScore != 0.82 {
		t.Errorf("trustScore = %+v", req.TrustScore)
	}
	if req.TrustLevel == nil || *req.TrustLevel != 3 {
		t.Errorf("trustLevel = %+v, want 3", req.TrustLevel)
	}
	if req.BehavioralProfile == nil || req.BehavioralProfile.ObservationDays != 10 {
		t.Errorf("observationDays = %+v, want 10", req.BehavioralProfile)
	}
	if len(req.Capabilities) != 2 {
		t.Errorf("capabilities = %v", req.Capabilities)
	}
}

// --- fakes for IssueForAgent end-to-end ---

type fakeAgentReader struct {
	agent *domain.Agent
	err   error
}

func (f *fakeAgentReader) GetByID(id uuid.UUID) (*domain.Agent, error) { return f.agent, f.err }

type fakeOrgReader struct{ org *domain.Organization }

func (f *fakeOrgReader) GetByID(id uuid.UUID) (*domain.Organization, error) { return f.org, nil }

type fakeScorer struct {
	score *domain.TrustScore
	err   error
}

func (f *fakeScorer) CalculateTrustScore(ctx context.Context, id uuid.UUID) (*domain.TrustScore, error) {
	return f.score, f.err
}

type fakeRegistryClient struct {
	gotReq registry.ATCIssuanceRequest
	cred   *registry.AgentTrustCredential
	err    error
}

func (f *fakeRegistryClient) IssueATC(ctx context.Context, req registry.ATCIssuanceRequest) (*registry.AgentTrustCredential, error) {
	f.gotReq = req
	return f.cred, f.err
}

func TestIssueForAgent_HappyPath(t *testing.T) {
	id := uuid.New()
	agent := &domain.Agent{ID: id, OrganizationID: uuid.New(), Capabilities: []string{"x"}}
	score := &domain.TrustScore{Score: 0.91, Confidence: 0.6}
	client := &fakeRegistryClient{cred: &registry.AgentTrustCredential{TransparencyLogIndex: 5, TrustLevel: 4}}

	svc := NewATCIssuanceService(
		&fakeAgentReader{agent: agent},
		&fakeOrgReader{org: &domain.Organization{Name: "Acme"}},
		&fakeScorer{score: score},
		client,
	)

	res, err := svc.IssueForAgent(context.Background(), id)
	if err != nil {
		t.Fatalf("IssueForAgent: %v", err)
	}
	if client.gotReq.Publisher != "Acme" {
		t.Errorf("publisher resolved = %q, want Acme", client.gotReq.Publisher)
	}
	if client.gotReq.TrustLevel == nil || *client.gotReq.TrustLevel != 4 {
		t.Errorf("level in request = %+v, want 4", client.gotReq.TrustLevel)
	}
	if res.Credential.TransparencyLogIndex != 5 {
		t.Errorf("credential index = %d, want 5", res.Credential.TransparencyLogIndex)
	}
	if res.Confidence != 0.6 {
		t.Errorf("confidence = %v, want 0.6", res.Confidence)
	}
	if !res.IsolationSelfReported {
		t.Error("IsolationSelfReported should be true until aim-isolation-verification")
	}
}

func TestIssueForAgent_PublisherFallbackToOrgID(t *testing.T) {
	id := uuid.New()
	orgID := uuid.New()
	agent := &domain.Agent{ID: id, OrganizationID: orgID}
	client := &fakeRegistryClient{cred: &registry.AgentTrustCredential{}}

	// nil org reader -> fall back to org UUID string.
	svc := NewATCIssuanceService(&fakeAgentReader{agent: agent}, nil, &fakeScorer{score: &domain.TrustScore{Score: 0.3}}, client)
	if _, err := svc.IssueForAgent(context.Background(), id); err != nil {
		t.Fatalf("IssueForAgent: %v", err)
	}
	if client.gotReq.Publisher != orgID.String() {
		t.Errorf("publisher = %q, want org UUID %q", client.gotReq.Publisher, orgID.String())
	}
}

func TestIssueForAgent_ScoreErrorPropagates(t *testing.T) {
	id := uuid.New()
	svc := NewATCIssuanceService(
		&fakeAgentReader{agent: &domain.Agent{ID: id}},
		nil,
		&fakeScorer{err: errors.New("boom")},
		&fakeRegistryClient{cred: &registry.AgentTrustCredential{}},
	)
	if _, err := svc.IssueForAgent(context.Background(), id); err == nil {
		t.Fatal("expected score error to propagate")
	}
}

func (f *fakeAgentReader) ListRevokedIDs(limit, offset int) ([]uuid.UUID, error) {
	return nil, nil
}
