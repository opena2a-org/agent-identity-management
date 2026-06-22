package application

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/registry"
)

// defaultATCVersion is used when an agent has no declared version. The credential
// carries a non-empty version so downstream verifiers never see an empty field.
const defaultATCVersion = "agent-v1"

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
// score (see todo/roadmap/aim-atx-issuer.md).
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
	// self-reported. AIM has no verified isolation writer until the
	// aim-isolation-verification track ships; surfacing this keeps the badge
	// honest about the one factor whose provenance is not yet attested.
	IsolationSelfReported bool `json:"isolationSelfReported"`
}

// ATCIssuanceService computes an AIM agent's behavioral trust score and delegates
// Agent Trust Credential issuance to the Registry. AIM does not sign or store
// ATCs itself; the Registry is the Certificate Authority.
type ATCIssuanceService struct {
	agents atcAgentReader
	orgs   atcOrgReader
	scorer atcTrustScorer
	client atcRegistryClient
}

// NewATCIssuanceService wires the issuance trigger.
func NewATCIssuanceService(
	agents atcAgentReader,
	orgs atcOrgReader,
	scorer atcTrustScorer,
	client atcRegistryClient,
) *ATCIssuanceService {
	return &ATCIssuanceService{
		agents: agents,
		orgs:   orgs,
		scorer: scorer,
		client: client,
	}
}

// IssueForAgent computes the agent's behavioral score, builds the issuance
// request, and calls the Registry. It returns the signed credential plus the
// unsigned provenance context.
//
// Side effect: CalculateTrustScore recomputes and persists the agent's current
// trust score, so an issuance attempt updates stored trust state even if the
// subsequent Registry call fails. This is intentional — the credential carries
// the freshly-computed score — but it means /atc is not a side-effect-free read.
func (s *ATCIssuanceService) IssueForAgent(ctx context.Context, agentID uuid.UUID) (*ATCIssuanceResult, error) {
	agent, err := s.agents.GetByID(agentID)
	if err != nil {
		return nil, fmt.Errorf("atc issuance: load agent: %w", err)
	}

	score, err := s.scorer.CalculateTrustScore(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("atc issuance: calculate trust score: %w", err)
	}

	req := buildATCIssuanceRequest(agent, score, s.resolvePublisher(agent), time.Now().UTC())

	cred, err := s.client.IssueATC(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("atc issuance: registry issue: %w", err)
	}

	return &ATCIssuanceResult{
		Credential:            cred,
		Confidence:            score.Confidence,
		IsolationSelfReported: true,
	}, nil
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

// buildATCIssuanceRequest assembles the Registry issuance request from an agent
// and its behavioral score. Pure (no I/O) so it is directly unit-testable. now
// is injected for the same reason.
func buildATCIssuanceRequest(agent *domain.Agent, score *domain.TrustScore, publisher string, now time.Time) registry.ATCIssuanceRequest {
	version := agent.Version
	if version == "" {
		version = defaultATCVersion
	}

	level := atcTrustLevel(score.Score)
	scoreVal := score.Score

	generatedAt := score.LastCalculated
	if generatedAt.IsZero() {
		generatedAt = now
	}

	return registry.ATCIssuanceRequest{
		AgentID:      agent.ID.String(),
		AgentDID:     domain.BuildAgentDID(agent.ID),
		Publisher:    publisher,
		Version:      version,
		ContentHash:  atcContentHash(agent),
		Capabilities: agent.Capabilities,
		TrustScore:   &scoreVal,
		TrustLevel:   &level,
		BehavioralProfile: &registry.ATCBehavioralProfile{
			Checksum:        atcBehavioralChecksum(score.Factors),
			GeneratedAt:     generatedAt.UTC(),
			ObservationDays: atcObservationDays(agent.CreatedAt, now),
		},
	}
}

// atcTrustLevel maps a 0-1 behavioral score to the 0-4 trust level. CDS-documented
// mapping (calibration-subject), aligned with the five trust levels:
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

// atcContentHash binds the credential to the agent's public key. The key is the
// stable identity material for an AIM agent (there is no package artifact). When
// no public key is set yet, we hash the agent ID so the field is never empty.
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
	return "sha256:" + hex.EncodeToString(sum[:])
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
