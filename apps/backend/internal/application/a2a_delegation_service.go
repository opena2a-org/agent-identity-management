package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// MaxA2ADelegationSkills is the most skill IDs one delegation may name.
const MaxA2ADelegationSkills = 100

// ErrA2ADelegationAgentNotFound is a delegator or delegate that does not exist
// or belongs to another organization. The two are one answer on purpose.
var ErrA2ADelegationAgentNotFound = errors.New("agent not found")

// delegationAgentReader reads agents by ID within the caller's organization.
// *repository.AgentRepository implements it.
type delegationAgentReader interface {
	GetByIDs(ctx context.Context, callerOrgID uuid.UUID, ids []uuid.UUID) ([]*domain.Agent, error)
}

// A2ADelegationService grants, reads and revokes delegations between agents of
// one organization, and verifies delegation chains, applying each
// delegation's trust attenuation factor and minimum delegated trust score.
type A2ADelegationService struct {
	delegations domain.A2ADelegationRepository
	agents      delegationAgentReader
	now         func() time.Time
}

// NewA2ADelegationService creates a new A2ADelegationService
func NewA2ADelegationService(delegations domain.A2ADelegationRepository, agents delegationAgentReader) *A2ADelegationService {
	return &A2ADelegationService{
		delegations: delegations,
		agents:      agents,
		now:         func() time.Time { return time.Now().UTC() },
	}
}

// CreateA2ADelegationRequest is the body of POST /api/v1/a2a/delegations. A
// field left out takes the a2a_delegation column default.
type CreateA2ADelegationRequest struct {
	DelegatorAgentID       uuid.UUID  `json:"delegatorAgentId"`
	DelegateAgentID        uuid.UUID  `json:"delegateAgentId"`
	SkillIDs               []string   `json:"skillIds"`
	MaxChainDepth          *int       `json:"maxChainDepth"`
	TrustAttenuationFactor *float64   `json:"trustAttenuationFactor"`
	MinDelegatedTrustScore *float64   `json:"minDelegatedTrustScore"`
	CanDelegateFurther     bool       `json:"canDelegateFurther"`
	ValidUntil             *time.Time `json:"validUntil"`
}

// CreateDelegation stores a delegation from one of the organization's agents
// to another. A request it refuses returns no delegation and, as refusal, the
// reason, written for the caller. It returns ErrA2ADelegationAgentNotFound
// when either agent is not the organization's, and
// domain.ErrA2ADelegationActive when the two agents already have an active
// delegation.
func (s *A2ADelegationService) CreateDelegation(ctx context.Context, orgID, createdBy uuid.UUID, req CreateA2ADelegationRequest) (d *domain.A2ADelegation, refusal string, err error) {
	d, refusal = s.newDelegation(orgID, createdBy, req)
	if refusal != "" {
		return nil, refusal, nil
	}

	agents, err := s.agents.GetByIDs(ctx, orgID, []uuid.UUID{d.DelegatorAgentID, d.DelegateAgentID})
	if err != nil {
		return nil, "", fmt.Errorf("failed to read the delegation's agents: %w", err)
	}
	if len(agents) != 2 {
		return nil, "", ErrA2ADelegationAgentNotFound
	}

	if err := s.delegations.Create(ctx, d); err != nil {
		return nil, "", err
	}
	return d, "", nil
}

// newDelegation builds the delegation a request asks for, with the column
// defaults for the fields it leaves out, or returns the reason it is refused.
func (s *A2ADelegationService) newDelegation(orgID, createdBy uuid.UUID, req CreateA2ADelegationRequest) (*domain.A2ADelegation, string) {
	now := s.now()
	d := &domain.A2ADelegation{
		ID:                     uuid.New(),
		OrganizationID:         orgID,
		DelegatorAgentID:       req.DelegatorAgentID,
		DelegateAgentID:        req.DelegateAgentID,
		SkillIDs:               req.SkillIDs,
		MaxChainDepth:          domain.DefaultA2ADelegationMaxChainDepth,
		TrustAttenuationFactor: domain.DefaultA2ADelegationTrustAttenuationFactor,
		MinDelegatedTrustScore: domain.DefaultA2ADelegationMinTrustScore,
		CanDelegateFurther:     req.CanDelegateFurther,
		IsActive:               true,
		ValidFrom:              now,
		ValidUntil:             req.ValidUntil,
		CreatedBy:              &createdBy,
	}
	if req.MaxChainDepth != nil {
		d.MaxChainDepth = *req.MaxChainDepth
	}
	if req.TrustAttenuationFactor != nil {
		d.TrustAttenuationFactor = *req.TrustAttenuationFactor
	}
	if req.MinDelegatedTrustScore != nil {
		d.MinDelegatedTrustScore = *req.MinDelegatedTrustScore
	}

	switch {
	case d.DelegatorAgentID == uuid.Nil || d.DelegateAgentID == uuid.Nil:
		return nil, "delegatorAgentId and delegateAgentId are required"
	case d.DelegatorAgentID == d.DelegateAgentID:
		return nil, "an agent cannot delegate to itself"
	case d.MaxChainDepth < 1 || d.MaxChainDepth > domain.MaxA2ADelegationChainLength:
		return nil, fmt.Sprintf("maxChainDepth must be from 1 to %d", domain.MaxA2ADelegationChainLength)
	case d.TrustAttenuationFactor < 0 || d.TrustAttenuationFactor > 1:
		return nil, "trustAttenuationFactor must be a number from 0 to 1"
	case d.MinDelegatedTrustScore < 0 || d.MinDelegatedTrustScore > 1:
		return nil, "minDelegatedTrustScore must be a number from 0 to 1"
	case len(d.SkillIDs) > MaxA2ADelegationSkills:
		return nil, fmt.Sprintf("skillIds may name at most %d skills", MaxA2ADelegationSkills)
	case d.ValidUntil != nil && !d.ValidUntil.After(now):
		return nil, "validUntil must be in the future"
	}
	for _, skill := range d.SkillIDs {
		if strings.TrimSpace(skill) == "" {
			return nil, "skillIds must not contain an empty skill ID"
		}
	}
	return d, ""
}

// GetDelegation returns the organization's delegation, or
// domain.ErrA2ADelegationNotFound.
func (s *A2ADelegationService) GetDelegation(ctx context.Context, orgID, id uuid.UUID) (*domain.A2ADelegation, error) {
	found, err := s.delegations.GetByIDs(ctx, orgID, []uuid.UUID{id})
	if err != nil {
		return nil, err
	}
	for _, d := range found {
		if d.ID == id {
			return d, nil
		}
	}
	return nil, domain.ErrA2ADelegationNotFound
}

// ListDelegations returns the organization's delegations, newest first.
func (s *A2ADelegationService) ListDelegations(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.A2ADelegation, error) {
	return s.delegations.ListByOrganization(ctx, orgID, limit, offset)
}

// RevokeDelegation marks the organization's delegation inactive, or returns
// domain.ErrA2ADelegationNotFound.
func (s *A2ADelegationService) RevokeDelegation(ctx context.Context, orgID, id uuid.UUID) (*domain.A2ADelegation, error) {
	return s.delegations.Revoke(ctx, orgID, id)
}

// A2ADelegationNotFoundError names the first requested delegation that does
// not exist or belongs to another organization.
type A2ADelegationNotFoundError struct {
	DelegationID uuid.UUID
}

func (e *A2ADelegationNotFoundError) Error() string {
	return fmt.Sprintf("delegation %s not found", e.DelegationID)
}

func (e *A2ADelegationNotFoundError) Unwrap() error { return domain.ErrA2ADelegationNotFound }

// VerifyDelegationChain reads the organization's delegations named by ids, in
// that order, and the agents they name, and returns
// domain.VerifyA2ADelegationChain's verdict on them. When ids is empty, too
// long or repeats a delegation it returns no verdict and, as refusal, the
// reason, written for the caller. It returns an *A2ADelegationNotFoundError
// when a delegation is not the organization's.
func (s *A2ADelegationService) VerifyDelegationChain(ctx context.Context, orgID uuid.UUID, ids []uuid.UUID, skillID string) (verdict *domain.A2ADelegationChainVerification, refusal string, err error) {
	if len(ids) == 0 || len(ids) > domain.MaxA2ADelegationChainLength {
		return nil, fmt.Sprintf("delegationIds must name from 1 to %d delegations", domain.MaxA2ADelegationChainLength), nil
	}
	seen := make(map[uuid.UUID]bool, len(ids))
	for _, id := range ids {
		if seen[id] {
			return nil, fmt.Sprintf("delegationIds names delegation %s more than once", id), nil
		}
		seen[id] = true
	}

	found, err := s.delegations.GetByIDs(ctx, orgID, ids)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read the chain's delegations: %w", err)
	}
	byID := make(map[uuid.UUID]*domain.A2ADelegation, len(found))
	for _, d := range found {
		byID[d.ID] = d
	}
	chain := make([]*domain.A2ADelegation, len(ids))
	agentIDs := make([]uuid.UUID, 0, len(ids)+1)
	agentSeen := map[uuid.UUID]bool{}
	for i, id := range ids {
		d, ok := byID[id]
		if !ok {
			return nil, "", &A2ADelegationNotFoundError{DelegationID: id}
		}
		chain[i] = d
		for _, agentID := range []uuid.UUID{d.DelegatorAgentID, d.DelegateAgentID} {
			if !agentSeen[agentID] {
				agentSeen[agentID] = true
				agentIDs = append(agentIDs, agentID)
			}
		}
	}

	agentList, err := s.agents.GetByIDs(ctx, orgID, agentIDs)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read the chain's agents: %w", err)
	}
	agents := make(map[uuid.UUID]*domain.Agent, len(agentList))
	for _, a := range agentList {
		agents[a.ID] = a
	}

	return domain.VerifyA2ADelegationChain(chain, agents, skillID, s.now()), "", nil
}
