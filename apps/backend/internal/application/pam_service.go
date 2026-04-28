package application

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// PAMService implements Privileged Access Management for AI agent fleets.
// Tiers: STANDARD (4h TTL) / PRIVILEGED (30min TTL) / SUPER_PRIVILEGED (15min, human approval).
type PAMService struct {
	db     *sql.DB
	logger *slog.Logger
}

// CapabilityTier represents the privilege classification.
type CapabilityTier struct {
	Capability     string `json:"capability"`
	Tier           string `json:"tier"`           // STANDARD, PRIVILEGED, SUPER_PRIVILEGED
	TierReason     string `json:"tierReason"`
	MaxTTLSeconds  int    `json:"maxTtlSeconds"`
	RequiresMFA    bool   `json:"requiresMfa"`
	RequiresTicket bool   `json:"requiresTicket"`
	RenewalAllowed bool   `json:"renewalAllowed"`
}

// ApprovalRequest represents a pending SUPER_PRIVILEGED approval.
type ApprovalRequest struct {
	ID                uuid.UUID       `json:"id"`
	AgentID           uuid.UUID       `json:"agentId"`
	Capability        string          `json:"capability"`
	ResourcePath      string          `json:"resourcePath"`
	Justification     string          `json:"justification"`
	IntentClass       string          `json:"intentClass,omitempty"`
	IntentConfidence  float64         `json:"intentConfidence,omitempty"`
	ApproverID        *uuid.UUID      `json:"approverId,omitempty"`
	ApproverDecision  string          `json:"approverDecision,omitempty"` // APPROVED, DENIED, TIMEOUT
	ApproverNote      string          `json:"approverNote,omitempty"`
	TicketID          string          `json:"ticketId,omitempty"`
	TicketSystem      string          `json:"ticketSystem,omitempty"`
	RequestedAt       time.Time       `json:"requestedAt"`
	ExpiresAt         time.Time       `json:"expiresAt"`
	DecidedAt         *time.Time      `json:"decidedAt,omitempty"`
}

// EmergencyDeclaration represents an active break-glass session.
type EmergencyDeclaration struct {
	ID              uuid.UUID   `json:"id"`
	IncidentID      string      `json:"incidentId"`
	DeclaredBy      string      `json:"declaredBy"`
	DeclaredAt      time.Time   `json:"declaredAt"`
	ExpiresAt       time.Time   `json:"expiresAt"`
	ExtendedUntil   *time.Time  `json:"extendedUntil,omitempty"`
	AffectedAgents  []uuid.UUID `json:"affectedAgents"`
	GrantedCaps     []string    `json:"grantedCaps"`
	Justification   string      `json:"justification"`
	Status          string      `json:"status"` // ACTIVE, EXPIRED, REVOKED
	ReviewCompleted bool        `json:"reviewCompleted"`
}

// CertificationCampaign represents a periodic access review campaign.
type CertificationCampaign struct {
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	ScopeType  string    `json:"scopeType"` // ALL, TEAM, TIER, AGENT
	ScopeValue string    `json:"scopeValue,omitempty"`
	StartsAt   time.Time `json:"startsAt"`
	ClosesAt   time.Time `json:"closesAt"`
	Status     string    `json:"status"` // PENDING, ACTIVE, GRACE_PERIOD, CLOSED
	CreatedBy  string    `json:"createdBy"`
}

// CertificationDecision represents a reviewer's decision on a capability.
type CertificationDecision struct {
	ID           uuid.UUID  `json:"id"`
	CampaignID   uuid.UUID  `json:"campaignId"`
	AgentID      uuid.UUID  `json:"agentId"`
	Capability   string     `json:"capability"`
	Decision     string     `json:"decision,omitempty"` // CERTIFIED, MODIFIED, REVOKED, SUSPENDED
	ReviewerID   string     `json:"reviewerId,omitempty"`
	ReviewerNote string     `json:"reviewerNote,omitempty"`
	DecidedAt    *time.Time `json:"decidedAt,omitempty"`
}

func NewPAMService(db *sql.DB, logger *slog.Logger) *PAMService {
	if logger == nil {
		logger = slog.Default()
	}
	return &PAMService{db: db, logger: logger}
}

// ============================================================================
// Tier Classification (P1)
// ============================================================================

// GetCapabilityTier returns the privilege tier for a capability.
func (s *PAMService) GetCapabilityTier(ctx context.Context, capability string) (*CapabilityTier, error) {
	if s.db == nil {
		return defaultTier(capability), nil
	}

	var tier CapabilityTier
	err := s.db.QueryRowContext(ctx,
		`SELECT capability, tier, tier_reason, max_ttl_seconds, requires_mfa, requires_ticket, renewal_allowed
		 FROM privileged_capability_registry WHERE capability = $1`, capability,
	).Scan(&tier.Capability, &tier.Tier, &tier.TierReason, &tier.MaxTTLSeconds,
		&tier.RequiresMFA, &tier.RequiresTicket, &tier.RenewalAllowed)

	if err == sql.ErrNoRows {
		return defaultTier(capability), nil
	}
	if err != nil {
		return nil, err
	}
	return &tier, nil
}

func defaultTier(capability string) *CapabilityTier {
	return &CapabilityTier{
		Capability:     capability,
		Tier:           "STANDARD",
		TierReason:     "Default tier",
		MaxTTLSeconds:  14400,
		RequiresMFA:    false,
		RenewalAllowed: true,
	}
}

// ============================================================================
// Approval Gates (P2)
// ============================================================================

// RequestApproval creates a pending approval for SUPER_PRIVILEGED access.
func (s *PAMService) RequestApproval(ctx context.Context, req *ApprovalRequest) (*ApprovalRequest, error) {
	if req.Justification == "" {
		return nil, fmt.Errorf("justification is required for SUPER_PRIVILEGED access")
	}

	req.ID = uuid.New()
	req.RequestedAt = time.Now().UTC()
	if req.ExpiresAt.IsZero() {
		req.ExpiresAt = req.RequestedAt.Add(5 * time.Minute) // 5min approval timeout
	}

	if s.db != nil {
		_, err := s.db.ExecContext(ctx,
			`INSERT INTO pending_approvals (id, agent_id, capability, resource_path, justification,
				intent_class, intent_confidence, ticket_id, ticket_system, requested_at, expires_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
			req.ID, req.AgentID, req.Capability, req.ResourcePath, req.Justification,
			req.IntentClass, req.IntentConfidence, req.TicketID, req.TicketSystem,
			req.RequestedAt, req.ExpiresAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create approval request: %w", err)
		}
	}

	s.logger.Info("approval requested",
		"id", req.ID, "agentId", req.AgentID, "capability", req.Capability,
		"expiresAt", req.ExpiresAt,
	)

	return req, nil
}

// DecideApproval records an approver's decision.
func (s *PAMService) DecideApproval(ctx context.Context, approvalID uuid.UUID, approverID uuid.UUID, decision string, note string) error {
	if decision != "APPROVED" && decision != "DENIED" {
		return fmt.Errorf("decision must be APPROVED or DENIED")
	}

	now := time.Now().UTC()

	if s.db != nil {
		result, err := s.db.ExecContext(ctx,
			`UPDATE pending_approvals
			 SET approver_id = $1, approver_decision = $2, approver_note = $3, decided_at = $4
			 WHERE id = $5 AND approver_decision IS NULL AND expires_at > NOW()`,
			approverID, decision, note, now, approvalID,
		)
		if err != nil {
			return fmt.Errorf("failed to update approval: %w", err)
		}
		rows, _ := result.RowsAffected()
		if rows == 0 {
			return fmt.Errorf("approval not found, already decided, or expired")
		}
	}

	s.logger.Info("approval decided",
		"id", approvalID, "decision", decision, "approver", approverID,
	)

	return nil
}

// ============================================================================
// Break-Glass Emergency Access (P3)
// ============================================================================

// DeclareEmergency creates a break-glass emergency access session.
// Duration hard cap: 60 minutes. NanoMind intent check still runs.
func (s *PAMService) DeclareEmergency(ctx context.Context, decl *EmergencyDeclaration) (*EmergencyDeclaration, error) {
	if decl.IncidentID == "" {
		return nil, fmt.Errorf("incident ID is required for break-glass access")
	}
	if decl.Justification == "" {
		return nil, fmt.Errorf("justification is required")
	}

	decl.ID = uuid.New()
	decl.DeclaredAt = time.Now().UTC()
	decl.Status = "ACTIVE"

	// Hard cap: 60 minutes maximum
	maxExpiry := decl.DeclaredAt.Add(60 * time.Minute)
	if decl.ExpiresAt.IsZero() || decl.ExpiresAt.After(maxExpiry) {
		decl.ExpiresAt = maxExpiry
	}

	if s.db != nil {
		_, err := s.db.ExecContext(ctx,
			`INSERT INTO emergency_declarations (id, incident_id, declared_by, declared_at, expires_at,
				affected_agents, granted_caps, justification, status)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			decl.ID, decl.IncidentID, decl.DeclaredBy, decl.DeclaredAt, decl.ExpiresAt,
			pq_from_uuids(decl.AffectedAgents), pq.Array(nonNilStrings(decl.GrantedCaps)),
			decl.Justification, decl.Status,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to declare emergency: %w", err)
		}
	}

	s.logger.Warn("EMERGENCY DECLARED",
		"id", decl.ID, "incident", decl.IncidentID,
		"agents", len(decl.AffectedAgents), "caps", len(decl.GrantedCaps),
		"expiresAt", decl.ExpiresAt,
	)

	return decl, nil
}

// ExtendEmergency grants a single 15-minute extension (requires second approver).
// Total duration (declaration + extension) cannot exceed 75 minutes.
func (s *PAMService) ExtendEmergency(ctx context.Context, emergencyID uuid.UUID, extendedBy string) error {
	now := time.Now().UTC()
	newExpiry := now.Add(15 * time.Minute)

	if s.db != nil {
		// Enforce 75-minute total cap: declared_at + 75 minutes
		result, err := s.db.ExecContext(ctx,
			`UPDATE emergency_declarations
			 SET extended_until = LEAST($1, declared_at + INTERVAL '75 minutes'), extended_by = $2
			 WHERE id = $3 AND status = 'ACTIVE' AND extended_until IS NULL`,
			newExpiry, extendedBy, emergencyID,
		)
		if err != nil {
			return err
		}
		rows, _ := result.RowsAffected()
		if rows == 0 {
			return fmt.Errorf("emergency not found, not active, or already extended")
		}
	}

	s.logger.Warn("emergency extended",
		"id", emergencyID, "extendedBy", extendedBy, "until", newExpiry,
	)
	return nil
}

// ============================================================================
// Certification Campaigns (P4)
// ============================================================================

// CreateCampaign creates a new certification campaign.
func (s *PAMService) CreateCampaign(ctx context.Context, campaign *CertificationCampaign) (*CertificationCampaign, error) {
	campaign.ID = uuid.New()
	campaign.Status = "PENDING"

	// Default 30-day campaign
	if campaign.ClosesAt.IsZero() {
		campaign.ClosesAt = campaign.StartsAt.Add(30 * 24 * time.Hour)
	}

	if s.db != nil {
		_, err := s.db.ExecContext(ctx,
			`INSERT INTO certification_campaigns (id, name, scope_type, scope_value, starts_at, closes_at, status, created_by)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			campaign.ID, campaign.Name, campaign.ScopeType, campaign.ScopeValue,
			campaign.StartsAt, campaign.ClosesAt, campaign.Status, campaign.CreatedBy,
		)
		if err != nil {
			return nil, err
		}
	}

	s.logger.Info("certification campaign created",
		"id", campaign.ID, "name", campaign.Name,
		"scope", campaign.ScopeType, "closes", campaign.ClosesAt,
	)

	return campaign, nil
}

// RecordCertificationDecision records a reviewer's decision for an agent capability.
func (s *PAMService) RecordCertificationDecision(ctx context.Context, decision *CertificationDecision) error {
	if decision.Decision == "" {
		return fmt.Errorf("decision is required (CERTIFIED, MODIFIED, REVOKED, or SUSPENDED)")
	}

	now := time.Now().UTC()
	decision.DecidedAt = &now
	decision.ID = uuid.New()

	if s.db != nil {
		_, err := s.db.ExecContext(ctx,
			`INSERT INTO certification_decisions (id, campaign_id, agent_id, capability, decision, reviewer_id, reviewer_note, decided_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			 ON CONFLICT (campaign_id, agent_id, capability) DO UPDATE SET
				decision = EXCLUDED.decision, reviewer_id = EXCLUDED.reviewer_id,
				reviewer_note = EXCLUDED.reviewer_note, decided_at = EXCLUDED.decided_at`,
			decision.ID, decision.CampaignID, decision.AgentID, decision.Capability,
			decision.Decision, decision.ReviewerID, decision.ReviewerNote, decision.DecidedAt,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

// ============================================================================
// Helpers
// ============================================================================

func pq_from_uuids(uuids []uuid.UUID) string {
	if len(uuids) == 0 {
		return "{}"
	}
	parts := make([]string, len(uuids))
	for i, u := range uuids {
		parts[i] = u.String()
	}
	return "{" + joinStrings(parts, ",") + "}"
}

func joinStrings(parts []string, sep string) string {
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += sep
		}
		result += p
	}
	return result
}
