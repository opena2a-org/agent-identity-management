package application

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// Default identity for an agent registered through a bootstrap token when the
// caller names none.
const (
	BootstrapDefaultAgentName        = "my-first-agent"
	BootstrapDefaultAgentDescription = "Registered from the onboarding screen"
)

var (
	// ErrBootstrapTokenScope is returned for a token whose scope does not
	// permit agent registration.
	ErrBootstrapTokenScope = errors.New("bootstrap token scope does not permit agent registration")

	// ErrBootstrapInvalidPublicKey is returned when the caller supplies a
	// public key that is not a base64 Ed25519 public key.
	ErrBootstrapInvalidPublicKey = errors.New("publicKey must be a base64-encoded 32-byte Ed25519 public key")
)

// BootstrapAgentRegistrar is the part of AgentService a bootstrap exchange
// uses.
type BootstrapAgentRegistrar interface {
	CreateAgent(ctx context.Context, req *CreateAgentRequest, orgID, userID uuid.UUID, sdkTokenID *uuid.UUID, apiKeyID *uuid.UUID, userEmail string) (*domain.Agent, error)
	GetAgentCredentials(ctx context.Context, agentID uuid.UUID) (string, string, error)
}

// BootstrapAuditLogger is the part of AuditService a bootstrap token uses.
type BootstrapAuditLogger interface {
	LogAction(ctx context.Context, orgID, userID uuid.UUID, action domain.AuditAction, resourceType string, resourceID uuid.UUID, ipAddress, userAgent string, metadata map[string]interface{}) error
}

// BootstrapEventSink records the onboarding events a bootstrap token causes.
// Its methods must not block.
type BootstrapEventSink interface {
	TokenMinted(orgID uuid.UUID)
	TokenExchanged(orgID uuid.UUID)
}

// BootstrapRequestMeta is the request context recorded in the audit trail.
type BootstrapRequestMeta struct {
	IPAddress string
	UserAgent string
}

// MintedBootstrapToken is a newly minted token. Plaintext is never stored and
// is returned to the minting user once.
type MintedBootstrapToken struct {
	Token     *domain.BootstrapToken
	Plaintext string
}

// BootstrapExchangeRequest describes the agent to register. Every field is
// optional; the organization and owner always come from the token.
type BootstrapExchangeRequest struct {
	Name        string
	DisplayName string
	Description string
	AgentType   domain.AgentType
	Version     string
	PublicKey   string
}

// BootstrapExchangeResult is the registered agent and its key material.
// PrivateKey is set only when the caller supplied no public key and the
// server generated the pair.
type BootstrapExchangeResult struct {
	Agent      *domain.Agent
	PublicKey  string
	PrivateKey string
}

// BootstrapTokenService mints, revokes and exchanges bootstrap tokens.
type BootstrapTokenService struct {
	repo   domain.BootstrapTokenRepository
	agents BootstrapAgentRegistrar
	audit  BootstrapAuditLogger
	events BootstrapEventSink
	now    func() time.Time
}

// NewBootstrapTokenService creates a BootstrapTokenService. audit may be nil.
func NewBootstrapTokenService(repo domain.BootstrapTokenRepository, agents BootstrapAgentRegistrar, audit BootstrapAuditLogger) *BootstrapTokenService {
	return &BootstrapTokenService{repo: repo, agents: agents, audit: audit, now: time.Now}
}

// SetOnboardingEvents wires the onboarding telemetry that records mints and
// exchanges. When unset, nothing is recorded.
func (s *BootstrapTokenService) SetOnboardingEvents(events BootstrapEventSink) {
	s.events = events
}

// SetClock replaces the service clock. Tests only.
func (s *BootstrapTokenService) SetClock(now func() time.Time) {
	s.now = now
}

// clock returns the current time at the precision PostgreSQL stores, so a
// timestamp written and later compared in a WHERE clause round-trips exactly.
func (s *BootstrapTokenService) clock() time.Time {
	return s.now().UTC().Truncate(time.Microsecond)
}

// Mint issues a token for userID in orgID, revoking that user's previous
// unused token in the same organization.
func (s *BootstrapTokenService) Mint(ctx context.Context, orgID, userID uuid.UUID, meta BootstrapRequestMeta) (*MintedBootstrapToken, error) {
	if orgID == uuid.Nil || userID == uuid.Nil {
		return nil, ErrInvalidOrgOrUser
	}

	plaintext, hash, displayPrefix, err := domain.GenerateBootstrapToken()
	if err != nil {
		return nil, err
	}
	now := s.clock()
	token := &domain.BootstrapToken{
		ID:             uuid.New(),
		OrganizationID: orgID,
		CreatedBy:      userID,
		TokenHash:      hash,
		DisplayPrefix:  displayPrefix,
		Scope:          domain.BootstrapTokenScopeAgentsRegister,
		CreatedAt:      now,
		ExpiresAt:      now.Add(domain.BootstrapTokenTTL),
	}

	// A concurrent mint by the same user can win the one-open-token index
	// between this transaction's revoke and insert; one retry revokes the
	// winner and inserts.
	if err := s.repo.CreateReplacingOpen(ctx, token, now); err != nil {
		if err := s.repo.CreateReplacingOpen(ctx, token, now); err != nil {
			return nil, fmt.Errorf("mint bootstrap token: %w", err)
		}
	}

	s.logAudit(ctx, orgID, userID, domain.AuditActionGenerate, "bootstrap_token", token.ID, meta, map[string]interface{}{
		"displayPrefix": displayPrefix,
		"scope":         token.Scope,
		"expiresAt":     token.ExpiresAt,
	})
	if s.events != nil {
		s.events.TokenMinted(orgID)
	}

	return &MintedBootstrapToken{Token: token, Plaintext: plaintext}, nil
}

// Revoke revokes every unused token userID minted in orgID and returns how
// many it revoked.
func (s *BootstrapTokenService) Revoke(ctx context.Context, orgID, userID uuid.UUID, meta BootstrapRequestMeta) (int64, error) {
	if orgID == uuid.Nil || userID == uuid.Nil {
		return 0, ErrInvalidOrgOrUser
	}
	n, err := s.repo.RevokeOpenForUser(ctx, orgID, userID, s.clock())
	if err != nil {
		return 0, err
	}
	if n > 0 {
		s.logAudit(ctx, orgID, userID, domain.AuditActionRevoke, "bootstrap_token", uuid.Nil, meta, map[string]interface{}{
			"revoked": n,
		})
	}
	return n, nil
}

// RevokeExposed revokes a token that reached the server somewhere it may have
// been recorded, such as a URL. A string that is not a well-formed token is
// ignored.
func (s *BootstrapTokenService) RevokeExposed(ctx context.Context, plaintext string) error {
	if domain.ValidateBootstrapTokenFormat(plaintext) != nil {
		return nil
	}
	return s.repo.RevokeByHash(ctx, domain.HashBootstrapToken(plaintext), s.clock())
}

// Exchange registers one agent in the token's organization, owned by the user
// who minted it, and consumes the token. Nothing in req can choose the
// organization or the owner.
func (s *BootstrapTokenService) Exchange(ctx context.Context, plaintext string, req BootstrapExchangeRequest, meta BootstrapRequestMeta) (*BootstrapExchangeResult, error) {
	if err := domain.ValidateBootstrapTokenFormat(plaintext); err != nil {
		return nil, err
	}
	if req.PublicKey != "" {
		decoded, err := base64.StdEncoding.DecodeString(req.PublicKey)
		if err != nil || len(decoded) != 32 {
			return nil, ErrBootstrapInvalidPublicKey
		}
	}

	token, err := s.repo.GetByHash(ctx, domain.HashBootstrapToken(plaintext))
	if err != nil {
		return nil, err
	}
	now := s.clock()
	if err := token.CheckUsable(now); err != nil {
		return nil, err
	}
	if token.Scope != domain.BootstrapTokenScopeAgentsRegister {
		return nil, ErrBootstrapTokenScope
	}

	claimed, err := s.repo.Claim(ctx, token.ID, now)
	if err != nil {
		return nil, err
	}
	if !claimed {
		// Another exchange consumed it, or it was revoked, between the read
		// and the claim. Report the current reason.
		if current, getErr := s.repo.GetByHash(ctx, token.TokenHash); getErr == nil {
			if reason := current.CheckUsable(now); reason != nil {
				return nil, reason
			}
		}
		return nil, domain.ErrBootstrapTokenUsed
	}

	name := req.Name
	if name == "" {
		name = BootstrapDefaultAgentName
	}
	displayName := req.DisplayName
	if displayName == "" {
		displayName = name
	}
	description := req.Description
	if description == "" {
		description = BootstrapDefaultAgentDescription
	}
	agentType := req.AgentType
	if agentType == "" {
		agentType = domain.AgentTypeCustom
	}

	agent, err := s.agents.CreateAgent(ctx, &CreateAgentRequest{
		Name:        name,
		DisplayName: displayName,
		Description: description,
		AgentType:   agentType,
		Version:     req.Version,
		PublicKey:   req.PublicKey,
	}, token.OrganizationID, token.CreatedBy, nil, nil, "")
	if err != nil {
		// No agent exists, so the caller may retry with the same token, for
		// example under another name.
		if relErr := s.repo.ReleaseClaim(ctx, token.ID, now); relErr != nil {
			log.Printf("bootstrap token %s: release claim after failed registration: %v", token.ID, relErr)
		}
		return nil, err
	}

	if err := s.repo.SetAgent(ctx, token.ID, agent.ID); err != nil {
		log.Printf("bootstrap token %s: record agent %s: %v", token.ID, agent.ID, err)
	}

	result := &BootstrapExchangeResult{Agent: agent, PublicKey: req.PublicKey}
	if req.PublicKey == "" {
		publicKey, privateKey, err := s.agents.GetAgentCredentials(ctx, agent.ID)
		if err != nil {
			return nil, fmt.Errorf("retrieve agent credentials: %w", err)
		}
		result.PublicKey = publicKey
		result.PrivateKey = privateKey
	}

	s.logAudit(ctx, token.OrganizationID, token.CreatedBy, domain.AuditActionCreate, "agent", agent.ID, meta, map[string]interface{}{
		"agentName":        agent.Name,
		"agentType":        agent.AgentType,
		"registration":     "bootstrap_token",
		"bootstrapTokenId": token.ID.String(),
	})
	if s.events != nil {
		s.events.TokenExchanged(token.OrganizationID)
	}

	return result, nil
}

func (s *BootstrapTokenService) logAudit(ctx context.Context, orgID, userID uuid.UUID, action domain.AuditAction, resourceType string, resourceID uuid.UUID, meta BootstrapRequestMeta, metadata map[string]interface{}) {
	if s.audit == nil {
		return
	}
	if err := s.audit.LogAction(ctx, orgID, userID, action, resourceType, resourceID, meta.IPAddress, meta.UserAgent, metadata); err != nil {
		log.Printf("bootstrap token audit %s %s: %v", action, resourceType, err)
	}
}
