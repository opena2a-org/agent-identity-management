package application

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/registry"
)

// defaultATCVersion is used when an agent has no declared version. The credential
// carries a non-empty version so downstream verifiers never see an empty field.
const defaultATCVersion = "agent-v1"

// atcPublisherDIDPrefix names an AIM organization as a credential publisher.
// ATX core section 2 names publishers with the did:opena2a method and its
// registered "publisher" type prefix; the identifier is "aim_" plus the
// organization UUID, the same namespace convention as the agent's
// did:aip:aim_<uuid>, so it never collides with a Registry-named publisher and
// always matches the schema's DID pattern whatever the organization is called.
const atcPublisherDIDPrefix = "did:opena2a:publisher:aim_"

// atcMaxSingleAuthorityTrustLevel is the highest trustLevel a credential signed
// by one authority may carry. ATX core section 12 (Conformance) forbids a
// conforming issuer to assert trustLevel 3 or higher on a credential that does
// not carry signatures from at least two distinct authorities, counted as
// section 1.3 step 7 counts them, and to assert trustLevel 4 without the root
// cosignature of section 7 rule 4. AIM asks one issuer to sign each credential
// and obtains no cosignature, so it never asks for a level above this one.
const atcMaxSingleAuthorityTrustLevel = 2

// errATCPublicOriginNotConfigured is returned when the service has no public
// origin to build the buildAttestation reference from. The field is mandatory in
// an ATX credential, so issuance fails closed rather than send an empty value.
var errATCPublicOriginNotConfigured = errors.New("atc issuance: public origin (FRONTEND_URL) is not configured")

// ErrATCNotConforming is returned when the credential issuer answers with a
// credential AIM does not hand on (see checkIssuedATC).
var ErrATCNotConforming = errors.New("atc issuance: the issued credential does not conform to ATX")

// ErrATCAgentRevoked is returned when the agent a credential is asked for is
// revoked, or is revoked while the Registry is issuing the credential. AIM asks
// the Registry to revoke the credentials it issued for a revoked agent
// (AgentService.RevokeAgent), so it does not ask it for a new one.
var ErrATCAgentRevoked = errors.New("atc issuance: the agent is revoked")

// The signature suites ATX registers (core section 14), as an ATX credential
// names them in a signature's algorithm member.
const (
	atcSignatureEd25519 = "Ed25519"
	atcSignatureMLDSA65 = "ML-DSA-65"
)

// atcAgentReader loads the agent being credentialed.
type atcAgentReader interface {
	GetByID(id uuid.UUID) (*domain.Agent, error)
}

// atcOrgReader resolves the publisher name for the credential. Optional: when
// nil or on lookup failure the service falls back to the organization UUID.
type atcOrgReader interface {
	GetByID(id uuid.UUID) (*domain.Organization, error)
}

// atcTrustScorer computes the agent's 9-factor behavioral trust score. This is
// the score the credential carries — NOT the Registry's supply-chain package
// score.
type atcTrustScorer interface {
	CalculateTrustScore(ctx context.Context, agentID uuid.UUID) (*domain.TrustScore, error)
}

// atcRegistryClient delegates signing + transparency-log recording to the
// Registry's ATC issuance endpoint.
type atcRegistryClient interface {
	IssueATC(ctx context.Context, req registry.ATCIssuanceRequest) (*registry.AgentTrustCredential, error)
}

// ATCIssuanceResult bundles the signed credential with the unsigned provenance
// AIM is honest about but cannot yet carry in the signed credential (ATX v1.2).
type ATCIssuanceResult struct {
	Credential *registry.AgentTrustCredential `json:"credential"`

	// Confidence is the score's self-reported confidence (0-1) from the 9-factor
	// calculator. It is informational context for the badge consumer, not a
	// signed field.
	Confidence float64 `json:"confidence"`

	// IsolationSelfReported is true while factor 9 (execution isolation) is
	// self-reported. AIM has no verified isolation writer until an independent
	// verification source exists; surfacing this keeps the badge honest about
	// the one factor whose provenance is not yet attested.
	IsolationSelfReported bool `json:"isolationSelfReported"`
}

// ATCIssuanceService computes an AIM agent's behavioral trust score and delegates
// Agent Trust Credential issuance to the Registry. AIM does not sign or store
// ATCs itself; the Registry is the Certificate Authority.
type ATCIssuanceService struct {
	agents atcAgentReader
	// grants is the agent_capabilities table. The credential's capabilities are
	// the agent's active grants at issuance (ATX core: capabilityJustification
	// keys are a subset of the granted set), never the list the SDK reported at
	// registration, which is stored on the agent row and is a declaration.
	grants activeCapabilityReader
	orgs   atcOrgReader
	scorer atcTrustScorer
	client atcRegistryClient

	// publicOrigin is the deployment's public origin (FRONTEND_URL), which serves
	// the public DID resolver at /api/v1/did/. The credential's buildAttestation
	// references the agent's DID document there.
	publicOrigin string

	// revoker is asked to revoke the agent's credentials again when the agent is
	// revoked while the Registry is issuing one; set by SetRegistryATCRevoker.
	revoker RegistryATCRevoker
	// revocations, when set, stores that request before it is sent, in place
	// of sending it to revoker from memory; set by SetRegistryATCRevocations.
	revocations *RegistryATCRevocations
}

// NewATCIssuanceService wires the issuance trigger. grants is the capability
// repository the credential's capabilities are read from. publicOrigin is the
// deployment's public origin (FRONTEND_URL).
func NewATCIssuanceService(
	agents atcAgentReader,
	grants activeCapabilityReader,
	orgs atcOrgReader,
	scorer atcTrustScorer,
	client atcRegistryClient,
	publicOrigin string,
) *ATCIssuanceService {
	return &ATCIssuanceService{
		agents:       agents,
		grants:       grants,
		orgs:         orgs,
		scorer:       scorer,
		client:       client,
		publicOrigin: strings.TrimRight(strings.TrimSpace(publicOrigin), "/"),
	}
}

// SetRegistryATCRevoker wires the Registry client IssueForAgent asks to revoke
// the agent's credentials again when the agent is revoked while the Registry is
// issuing one. When unset, IssueForAgent still hands no such credential on.
func (s *ATCIssuanceService) SetRegistryATCRevoker(revoker RegistryATCRevoker) {
	s.revoker = revoker
}

// SetRegistryATCRevocations wires the server's store of revocation requests.
// When set, IssueForAgent stores its request to revoke the agent's credentials
// again there, so a server stop before the Registry has answered does not end
// it, and the revoker set by SetRegistryATCRevoker is not used.
func (s *ATCIssuanceService) SetRegistryATCRevocations(q *RegistryATCRevocations) {
	s.revocations = q
}

// IssueForAgent computes the agent's behavioral score, builds the issuance
// request, and calls the Registry. It returns the signed credential plus the
// unsigned provenance context, ErrATCAgentRevoked when the agent is revoked
// when it is first read or when it is read again after the Registry has
// answered, or ErrATCNotConforming when the Registry's credential fails
// checkIssuedATC.
//
// Side effect: CalculateTrustScore recomputes and persists the agent's current
// trust score, so an issuance attempt updates stored trust state even if the
// subsequent Registry call fails. This is intentional — the credential carries
// the freshly-computed score — but it means /atc is not a side-effect-free read.
// A revoked agent is refused before its score is computed.
func (s *ATCIssuanceService) IssueForAgent(ctx context.Context, agentID uuid.UUID) (*ATCIssuanceResult, error) {
	if s.publicOrigin == "" {
		return nil, errATCPublicOriginNotConfigured
	}

	agent, err := s.loadAgent(agentID)
	if err != nil {
		return nil, fmt.Errorf("atc issuance: load agent: %w", err)
	}
	if agent.Status == domain.AgentStatusRevoked {
		return nil, ErrATCAgentRevoked
	}

	// The credential asserts the agent's active grants as of this call. A grant
	// revoked before now is absent; one granted since the last credential is
	// present. A failed read stops the issuance: the registration-time list on the
	// agent row is never signed in its place.
	capabilities, dropped, err := activeCapabilityTokens(s.grants, agentID)
	if err != nil {
		return nil, fmt.Errorf("atc issuance: load capability grants: %w", err)
	}
	if dropped > 0 {
		log.Printf("atc issuance: agent %s: %d active capability grant(s) are outside the credential token grammar and were left out of the credential", agentID, dropped)
	}

	score, err := s.scorer.CalculateTrustScore(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("atc issuance: calculate trust score: %w", err)
	}

	req := buildATCIssuanceRequest(agent, capabilities, score, s.resolvePublisher(agent), s.publicOrigin, time.Now().UTC())

	cred, issueErr := s.client.IssueATC(ctx, req)

	// The agent may be revoked while the Registry is issuing. RevokeAgent's
	// revocation request may then reach the Registry before this issuance does,
	// so the agent is read again once the Registry has answered. For an agent
	// revoked by then, the credential is not handed on, and the Registry is
	// asked again to revoke the agent's credentials, so a revocation request
	// reaches it after this issuance. It is asked whatever the issuance
	// answered: a request whose answer did not arrive may still have been
	// signed.
	current, err := s.loadAgent(agentID)
	switch {
	case err == nil && current.Status == domain.AgentStatusRevoked:
		if s.revocations != nil {
			s.revocations.Request(ctx, agentID)
		} else {
			startRegistryATCRevocation(s.revoker, agentID, nil)
		}
		return nil, ErrATCAgentRevoked
	case issueErr != nil:
		return nil, fmt.Errorf("atc issuance: registry issue: %w", issueErr)
	case err != nil:
		// Whether the agent was revoked is not known, so no credential is handed on.
		return nil, fmt.Errorf("atc issuance: reload agent after issuance: %w", err)
	}
	if err := checkIssuedATC(req, cred); err != nil {
		return nil, err
	}

	return &ATCIssuanceResult{
		Credential:            cred,
		Confidence:            score.Confidence,
		IsolationSelfReported: true,
	}, nil
}

// loadAgent reads the agent being credentialed. A read that returns no agent
// fails as a failed read does.
func (s *ATCIssuanceService) loadAgent(agentID uuid.UUID) (*domain.Agent, error) {
	agent, err := s.agents.GetByID(agentID)
	if err != nil {
		return nil, err
	}
	if agent == nil {
		return nil, fmt.Errorf("agent %s not found", agentID)
	}
	return agent, nil
}

// checkIssuedATC refuses an issued credential that AIM must not hand on. AIM
// returns the issuer's signed bytes verbatim, so this is the last point at which
// a credential that breaks ATX issuance can be stopped. It checks what can be
// checked without the issuer's keys; whether each signature verifies is the
// relying party's verifier's work (ATX core section 1.3).
//
//   - Signature suites. Core section 1.1: "The signature block carries at
//     minimum one Ed25519 signature and one ML-DSA-65 signature." The ATX
//     credential schema (schemas/atx-credential-v1.1.schema.json in atx-spec)
//     admits only Ed25519 and ML-DSA-65 as signatures[].algorithm, and they
//     are the only suites section 14 registers, so a signature in any other
//     suite is refused as well.
//   - Agent and trustScore. The credential names the agent AIM asked about and
//     carries the trustScore AIM sent, compared in the form the v1.1 signature
//     covers (printf %.6f, core section 1.3a.2). An issuer that read the score
//     on another scale or replaced it would sign a score AIM did not compute.
//   - trustLevel. The credential asserts no higher level than AIM asked for,
//     which is at most the level one authority may assert (core section 12). A
//     lower level is a narrower claim and is accepted.
func checkIssuedATC(req registry.ATCIssuanceRequest, cred *registry.AgentTrustCredential) error {
	if cred == nil {
		return fmt.Errorf("%w: the issuer returned no credential", ErrATCNotConforming)
	}

	var ed25519Sigs, mldsa65Sigs int
	for _, sig := range cred.Signatures {
		switch sig.Algorithm {
		case atcSignatureEd25519:
			ed25519Sigs++
		case atcSignatureMLDSA65:
			mldsa65Sigs++
		default:
			return fmt.Errorf("%w: signature %q uses algorithm %q, not Ed25519 or ML-DSA-65",
				ErrATCNotConforming, sig.KeyID, sig.Algorithm)
		}
	}
	if ed25519Sigs == 0 {
		return fmt.Errorf("%w: no Ed25519 signature", ErrATCNotConforming)
	}
	if mldsa65Sigs == 0 {
		return fmt.Errorf("%w: no ML-DSA-65 signature", ErrATCNotConforming)
	}

	if cred.AgentDID != req.AgentDID {
		return fmt.Errorf("%w: credential names agent %q, requested %q", ErrATCNotConforming, cred.AgentDID, req.AgentDID)
	}
	if req.TrustScore != nil {
		got, want := fmt.Sprintf("%.6f", cred.TrustScore), fmt.Sprintf("%.6f", *req.TrustScore)
		if got != want {
			return fmt.Errorf("%w: trustScore %s, requested %s", ErrATCNotConforming, got, want)
		}
	}
	if req.TrustLevel != nil && cred.TrustLevel > *req.TrustLevel {
		return fmt.Errorf("%w: trustLevel %d, requested at most %d", ErrATCNotConforming, cred.TrustLevel, *req.TrustLevel)
	}
	return nil
}

// resolvePublisher returns the organization name as the credential publisher,
// falling back to the organization UUID when the name is unavailable.
func (s *ATCIssuanceService) resolvePublisher(agent *domain.Agent) string {
	if s.orgs != nil {
		if org, err := s.orgs.GetByID(agent.OrganizationID); err == nil && org != nil && org.Name != "" {
			return org.Name
		}
	}
	return agent.OrganizationID.String()
}

// buildATCIssuanceRequest assembles the Registry issuance request from an agent,
// its active capability grants as grantedCapabilityTokens lists them, and its
// behavioral score. Pure (no I/O) so it is directly unit-testable. now is
// injected for the same reason. publicOrigin is the deployment's public origin
// with no trailing slash.
//
// capabilities is sent as given; a nil list is sent as the empty array because
// the credential schema requires the field.
func buildATCIssuanceRequest(agent *domain.Agent, capabilities []string, score *domain.TrustScore, publisher, publicOrigin string, now time.Time) registry.ATCIssuanceRequest {
	version := agent.Version
	if version == "" {
		version = defaultATCVersion
	}
	if capabilities == nil {
		capabilities = []string{}
	}

	level := atcCredentialTrustLevel(score.Score)
	scoreVal := atcWireTrustScore(score.Score)
	agentDID := domain.BuildAgentDID(agent.ID)

	generatedAt := score.LastCalculated
	if generatedAt.IsZero() {
		generatedAt = now
	}

	return registry.ATCIssuanceRequest{
		AgentID:          agent.ID.String(),
		AgentDID:         agentDID,
		Publisher:        publisher,
		PublisherDID:     atcPublisherDID(agent.OrganizationID),
		Version:          version,
		ContentHash:      atcContentHash(agent),
		BuildAttestation: atcBuildAttestation(publicOrigin, agentDID),
		Capabilities:     capabilities,
		TrustScore:       &scoreVal,
		TrustLevel:       &level,
		BehavioralProfile: &registry.ATCBehavioralProfile{
			Checksum:        atcBehavioralChecksum(score.Factors),
			GeneratedAt:     generatedAt.UTC(),
			ObservationDays: atcObservationDays(agent.CreatedAt, now),
		},
	}
}

// atcWireTrustScore converts the 9-factor calculator's 0-1 score to the 0-100
// scale ATX puts on the wire: "trustScore rides the wire as a 0-100 JSON number"
// (ATX core section 1.1). It is rounded to six fractional digits, the precision
// the v1.1 signed form encodes (section 1.3a.2 rule 3, printf %.6f), so 0.29 is
// sent as 29 rather than 28.999999999999996.
func atcWireTrustScore(score float64) float64 {
	return math.Round(score*100*1e6) / 1e6
}

// atcPublisherDID is the credential's publisherDid for an agent's organization.
func atcPublisherDID(orgID uuid.UUID) string {
	return atcPublisherDIDPrefix + orgID.String()
}

// atcBuildAttestation is the credential's buildAttestation for an AIM agent. An
// AIM agent is registered, not built, so there is no build provenance to cite.
// The reference is the agent's DID document on this deployment's public DID
// resolver. For an agent with a public key, that document publishes the key
// whose SHA-256 is contentHash, so a verifier can fetch it and recompute the hash.
func atcBuildAttestation(publicOrigin, agentDID string) string {
	return publicOrigin + "/api/v1/did/" + agentDID
}

// atcTrustLevel maps a 0-1 behavioral score to the 0-4 trust level. The cut-offs
// are subject to calibration and align with the five trust levels:
// >=0.90 -> 4, >=0.75 -> 3, >=0.50 -> 2, >=0.25 -> 1, otherwise 0.
func atcTrustLevel(score float64) int {
	switch {
	case score >= 0.90:
		return 4
	case score >= 0.75:
		return 3
	case score >= 0.50:
		return 2
	case score >= 0.25:
		return 1
	default:
		return 0
	}
}

// atcCredentialTrustLevel is the trustLevel the issuance request asks the issuer
// to sign: the behavioral level from atcTrustLevel, capped at the level one
// signing authority may assert. A score that maps to 3 or 4 is requested as 2;
// the full behavioral score still travels in trustScore.
func atcCredentialTrustLevel(score float64) int {
	level := atcTrustLevel(score)
	if level > atcMaxSingleAuthorityTrustLevel {
		return atcMaxSingleAuthorityTrustLevel
	}
	return level
}

// atcContentHash binds the credential to the agent's public key. The key is the
// stable identity material for an AIM agent (there is no package artifact). When
// no public key is set yet, we hash the agent ID so the field is never empty.
// The value is lowercase hex with no algorithm prefix: the Registry signs it
// verbatim, and the ATX v1.1 credential schema requires ^[0-9a-f]{64}$.
func atcContentHash(agent *domain.Agent) string {
	var data []byte
	switch {
	case agent.PublicKey != nil && *agent.PublicKey != "":
		if decoded, err := base64.StdEncoding.DecodeString(*agent.PublicKey); err == nil {
			data = decoded
		} else {
			data = []byte(*agent.PublicKey)
		}
	default:
		idBytes := agent.ID
		data = idBytes[:]
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// atcBehavioralChecksum is the SHA-256 of the canonical 9-factor breakdown. It
// binds the behavioral profile to the exact factor values that produced the
// score without putting the raw factors on the wire. encoding/json marshals
// struct fields in declaration order, so this is deterministic.
func atcBehavioralChecksum(factors domain.TrustScoreFactors) string {
	b, _ := json.Marshal(factors)
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// atcObservationDays is the whole-day age of the agent at issuance time, used as
// the behavioral profile's observation window. Clamped to >= 0.
func atcObservationDays(createdAt, now time.Time) int {
	if createdAt.IsZero() {
		return 0
	}
	days := int(now.Sub(createdAt).Hours() / 24)
	if days < 0 {
		return 0
	}
	return days
}
