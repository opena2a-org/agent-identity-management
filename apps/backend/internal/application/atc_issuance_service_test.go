package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/registry"
)

// testATCPublicOrigin stands in for FRONTEND_URL in these tests.
const testATCPublicOrigin = "https://aim.example.com"

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
	wantFallback := hex.EncodeToString(idSum[:])
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

// TestBuildATCIssuanceRequest_HashFieldsMatchATXSchema pins the wire form of the
// two hash fields AIM supplies to the published ATX v1.1 credential schema
// (schemas/atx-credential-v1.1.schema.json in atx-spec). The Registry signs
// contentHash verbatim, so a prefixed value here yields a credential that fails
// schema validation in every conformance verifier. behavioralProfile.checksum
// is the opposite case: the schema requires the "sha256:" prefix there.
func TestBuildATCIssuanceRequest_HashFieldsMatchATXSchema(t *testing.T) {
	contentHashPattern := regexp.MustCompile(`^[0-9a-f]{64}$`)
	checksumPattern := regexp.MustCompile(`^sha256:.+`)

	id := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	now := time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)
	pk := "dGhpcyBpcyBhIGZha2Uga2V5" // base64
	score := &domain.TrustScore{Score: 0.6, Factors: domain.TrustScoreFactors{Uptime: 0.9}}

	for name, agent := range map[string]*domain.Agent{
		"public key":  {ID: id, PublicKey: &pk},
		"id fallback": {ID: id},
	} {
		req := buildATCIssuanceRequest(agent, score, "Acme Org", testATCPublicOrigin, now)
		if !contentHashPattern.MatchString(req.ContentHash) {
			t.Errorf("%s: contentHash %q does not match the ATX schema pattern %s",
				name, req.ContentHash, contentHashPattern)
		}
		if !checksumPattern.MatchString(req.BehavioralProfile.Checksum) {
			t.Errorf("%s: behavioralProfile.checksum %q does not match the ATX schema pattern %s",
				name, req.BehavioralProfile.Checksum, checksumPattern)
		}
	}
}

// TestBuildATCIssuanceRequest_FieldsMatchATXSchema pins every field AIM puts in
// the issuance request to its rule in the ATX v1.1 credential schema
// (schemas/atx-credential-v1.1.schema.json in atx-spec). The Registry copies
// these fields into the credential it signs, so a value that breaks a rule here
// yields a credential that fails schema validation in a conformance verifier.
// publisherDid and buildAttestation are required by the schema, and trustScore
// is a 0-100 number on the wire (ATX core section 1.1).
func TestBuildATCIssuanceRequest_FieldsMatchATXSchema(t *testing.T) {
	didPattern := regexp.MustCompile(`^did:[a-z0-9]+:[A-Za-z0-9._:@/-]+(#[A-Za-z0-9._-]+)?$`)
	capabilityPattern := regexp.MustCompile(`^[a-z0-9_-]+:[A-Za-z0-9_.*-]+$`)

	id := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	orgID := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	now := time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)
	agent := &domain.Agent{
		ID:             id,
		OrganizationID: orgID,
		Capabilities:   []string{"file:read", "api:call"},
		CreatedAt:      now.Add(-48 * time.Hour),
	}

	for _, c := range []struct {
		score float64
		want  float64
	}{
		{0, 0},
		{0.29, 29},
		{0.62, 62},
		{0.875, 87.5},
		{1, 100},
	} {
		req := buildATCIssuanceRequest(agent, &domain.TrustScore{Score: c.score}, "Acme Org", testATCPublicOrigin, now)
		if req.TrustScore == nil || *req.TrustScore != c.want {
			t.Errorf("score %v: trustScore = %+v, want %v on the 0-100 wire scale", c.score, req.TrustScore, c.want)
		}
	}

	req := buildATCIssuanceRequest(agent, &domain.TrustScore{Score: 0.62}, "Acme Org", testATCPublicOrigin, now)

	for field, did := range map[string]string{"agentDid": req.AgentDID, "publisherDid": req.PublisherDID} {
		if !didPattern.MatchString(did) {
			t.Errorf("%s %q does not match the ATX schema DID pattern %s", field, did, didPattern)
		}
	}
	if want := "did:opena2a:publisher:aim_" + orgID.String(); req.PublisherDID != want {
		t.Errorf("publisherDid = %q, want %q", req.PublisherDID, want)
	}
	if want := testATCPublicOrigin + "/api/v1/did/did:aip:aim_" + id.String(); req.BuildAttestation != want {
		t.Errorf("buildAttestation = %q, want %q", req.BuildAttestation, want)
	}
	for field, v := range map[string]string{"agentId": req.AgentID, "publisher": req.Publisher, "version": req.Version} {
		if v == "" {
			t.Errorf("%s is empty; the ATX schema requires minLength 1", field)
		}
	}
	for _, capability := range req.Capabilities {
		if !capabilityPattern.MatchString(capability) {
			t.Errorf("capability %q does not match the ATX schema pattern %s", capability, capabilityPattern)
		}
	}
	if req.TrustLevel == nil || *req.TrustLevel < 0 || *req.TrustLevel > 4 {
		t.Errorf("trustLevel = %+v, want an integer in [0, 4]", req.TrustLevel)
	}
	if req.BehavioralProfile == nil || req.BehavioralProfile.ObservationDays < 0 {
		t.Errorf("behavioralProfile = %+v, want observationDays >= 0", req.BehavioralProfile)
	}

	// The mandatory credential fields are on the wire under their ATX names.
	raw, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	var wire map[string]json.RawMessage
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}
	for _, key := range []string{"agentId", "agentDid", "publisher", "publisherDid", "version", "contentHash", "buildAttestation", "capabilities", "trustScore", "trustLevel", "behavioralProfile"} {
		if _, ok := wire[key]; !ok {
			t.Errorf("issuance request JSON has no %q member", key)
		}
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

// TestBuildATCIssuanceRequest_TrustLevelFitsOneSigningAuthority pins ATX core
// section 12: a conforming issuer MUST NOT assert trustLevel 3 or higher on a
// credential that does not carry signatures from at least two distinct
// authorities, nor trustLevel 4 without the root cosignature of section 7. One
// issuer signs every credential AIM requests, so the level AIM asks for stays at
// 2 or below while the 0-100 trustScore still carries the full behavioral score.
func TestBuildATCIssuanceRequest_TrustLevelFitsOneSigningAuthority(t *testing.T) {
	now := time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)
	agent := &domain.Agent{ID: uuid.New(), OrganizationID: uuid.New(), CreatedAt: now}

	cases := []struct {
		score     float64
		wantLevel int
		wantScore float64
	}{
		{1.0, 2, 100},
		{0.95, 2, 95},
		{0.90, 2, 90},
		{0.82, 2, 82},
		{0.75, 2, 75},
		{0.74, 2, 74},
		{0.50, 2, 50},
		{0.49, 1, 49},
		{0.25, 1, 25},
		{0.24, 0, 24},
		{0.0, 0, 0},
	}
	for _, c := range cases {
		req := buildATCIssuanceRequest(agent, &domain.TrustScore{Score: c.score}, "Acme Org", testATCPublicOrigin, now)
		if req.TrustLevel == nil {
			t.Fatalf("score %v: request carries no trustLevel", c.score)
		}
		if *req.TrustLevel != c.wantLevel {
			t.Errorf("score %v: trustLevel = %d, want %d (one signing authority allows at most 2)", c.score, *req.TrustLevel, c.wantLevel)
		}
		if req.TrustScore == nil || *req.TrustScore != c.wantScore {
			t.Errorf("score %v: trustScore = %v, want %v", c.score, req.TrustScore, c.wantScore)
		}
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

	req := buildATCIssuanceRequest(agent, score, "Acme Org", testATCPublicOrigin, now)

	if req.AgentDID != "did:aip:aim_"+id.String() {
		t.Errorf("agentDid = %q", req.AgentDID)
	}
	if req.Version != defaultATCVersion {
		t.Errorf("version = %q, want default %q", req.Version, defaultATCVersion)
	}
	if req.Publisher != "Acme Org" {
		t.Errorf("publisher = %q", req.Publisher)
	}
	if req.TrustScore == nil || *req.TrustScore != 82 {
		t.Errorf("trustScore = %+v, want 82 (0.82 on the 0-100 wire scale)", req.TrustScore)
	}
	// 0.82 maps to behavioral level 3; one signing authority caps the request at 2.
	if req.TrustLevel == nil || *req.TrustLevel != 2 {
		t.Errorf("trustLevel = %+v, want 2", req.TrustLevel)
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
	client := &fakeRegistryClient{cred: &registry.AgentTrustCredential{TransparencyLogIndex: 5, TrustLevel: 2}}

	svc := NewATCIssuanceService(
		&fakeAgentReader{agent: agent},
		&fakeOrgReader{org: &domain.Organization{Name: "Acme"}},
		&fakeScorer{score: score},
		client,
		testATCPublicOrigin+"/",
	)

	res, err := svc.IssueForAgent(context.Background(), id)
	if err != nil {
		t.Fatalf("IssueForAgent: %v", err)
	}
	if client.gotReq.Publisher != "Acme" {
		t.Errorf("publisher resolved = %q, want Acme", client.gotReq.Publisher)
	}
	// 0.91 maps to behavioral level 4; one signing authority caps the request at 2.
	if client.gotReq.TrustLevel == nil || *client.gotReq.TrustLevel != 2 {
		t.Errorf("level in request = %+v, want 2", client.gotReq.TrustLevel)
	}
	if client.gotReq.TrustScore == nil || *client.gotReq.TrustScore != 91 {
		t.Errorf("score in request = %+v, want 91", client.gotReq.TrustScore)
	}
	if want := "did:opena2a:publisher:aim_" + agent.OrganizationID.String(); client.gotReq.PublisherDID != want {
		t.Errorf("publisherDid in request = %q, want %q", client.gotReq.PublisherDID, want)
	}
	// The configured origin's trailing slash is not doubled.
	if want := testATCPublicOrigin + "/api/v1/did/did:aip:aim_" + id.String(); client.gotReq.BuildAttestation != want {
		t.Errorf("buildAttestation in request = %q, want %q", client.gotReq.BuildAttestation, want)
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
	svc := NewATCIssuanceService(&fakeAgentReader{agent: agent}, nil, &fakeScorer{score: &domain.TrustScore{Score: 0.3}}, client, testATCPublicOrigin)
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
		testATCPublicOrigin,
	)
	if _, err := svc.IssueForAgent(context.Background(), id); err == nil {
		t.Fatal("expected score error to propagate")
	}
}

// TestIssueForAgent_NoPublicOriginFailsClosed: buildAttestation is a mandatory
// credential field built from the public origin, so with none configured the
// service refuses before computing a score or calling the Registry.
func TestIssueForAgent_NoPublicOriginFailsClosed(t *testing.T) {
	id := uuid.New()
	client := &fakeRegistryClient{cred: &registry.AgentTrustCredential{}}
	svc := NewATCIssuanceService(
		&fakeAgentReader{agent: &domain.Agent{ID: id, OrganizationID: uuid.New()}},
		nil,
		&fakeScorer{score: &domain.TrustScore{Score: 0.8}},
		client,
		"  ",
	)
	_, err := svc.IssueForAgent(context.Background(), id)
	if !errors.Is(err, errATCPublicOriginNotConfigured) {
		t.Fatalf("err = %v, want errATCPublicOriginNotConfigured", err)
	}
	if client.gotReq.AgentID != "" {
		t.Errorf("the Registry was called with %+v; want no call", client.gotReq)
	}
}

func (f *fakeAgentReader) ListRevokedIDs(limit, offset int) ([]uuid.UUID, error) {
	return nil, nil
}

func (f *fakeAgentReader) SuspendAgentsWithExpiredKeys(now time.Time) ([]uuid.UUID, error) {
	return nil, nil
}
