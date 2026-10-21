package application

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/lib/pq"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/util/sqlarray"
)

// PAMService implements Privileged Access Management for AI agent fleets: the
// capability tier registry, approval requests for SUPER_PRIVILEGED
// capabilities, break-glass emergency declarations and certification
// campaigns. Every approval, declaration, campaign and certification decision
// it reads or writes belongs to the organization its caller names; the tier
// registry is one table shared by all organizations.
//
// It keeps these records. The capability verification and authorization
// routes do not read them.
type PAMService struct {
	db     *sql.DB
	logger *slog.Logger
	now    func() time.Time
}

// Durations the service applies.
const (
	// ApprovalDecisionWindow is how long an approval request waits for a
	// decision.
	ApprovalDecisionWindow = 5 * time.Minute
	// EmergencyMaxDuration caps a break-glass declaration.
	EmergencyMaxDuration = 60 * time.Minute
	// EmergencyExtension is the one extension a declaration can receive.
	EmergencyExtension = 15 * time.Minute
	// EmergencyMaxTotal caps a declaration with its extension, counted from
	// the declaration.
	EmergencyMaxTotal = 75 * time.Minute
	// CampaignDefaultLength is a campaign's length when it names no close.
	CampaignDefaultLength = 30 * 24 * time.Hour
	// PAMListLimit is the most records a list returns, newest first.
	PAMListLimit = 100
	// pamMaxText bounds every free-text field of a PAM record, in characters.
	pamMaxText = 1000
	// pamMaxListEntries bounds the agents and capabilities of a declaration.
	pamMaxListEntries = 100
)

// Statuses the service reports. A record's status is read from its times and
// its decision when it is read, so a request or declaration past its expiry
// reads EXPIRED and a campaign past its close reads CLOSED.
const (
	ApprovalStatusPending  = "PENDING"
	ApprovalStatusApproved = "APPROVED"
	ApprovalStatusDenied   = "DENIED"
	ApprovalStatusExpired  = "EXPIRED"

	EmergencyStatusActive  = "ACTIVE"
	EmergencyStatusExpired = "EXPIRED"
	EmergencyStatusRevoked = "REVOKED"

	CampaignStatusPending = "PENDING"
	CampaignStatusActive  = "ACTIVE"
	CampaignStatusClosed  = "CLOSED"
)

// Errors the service returns for a record it cannot act on.
var (
	// ErrPAMNotFound: the approval, declaration, campaign or agent is not one
	// of the organization's. A record of another organization and a record
	// that does not exist get this same error.
	ErrPAMNotFound = errors.New("pam: not found in the organization")
	// ErrPAMSameUser: the user who requested an approval cannot decide it, and
	// the user who declared an emergency cannot extend it.
	ErrPAMSameUser = errors.New("pam: a second user must act on this record")
	// ErrPAMApprovalNotPending: the approval was decided or its decision
	// window closed.
	ErrPAMApprovalNotPending = errors.New("pam: the approval is not pending")
	// ErrPAMEmergencyNotActive: the declaration expired or was revoked.
	ErrPAMEmergencyNotActive = errors.New("pam: the emergency declaration is not active")
	// ErrPAMEmergencyExtended: the declaration already received its one
	// extension.
	ErrPAMEmergencyExtended = errors.New("pam: the emergency declaration was already extended")
	// ErrPAMCampaignNotActive: the campaign has not started or has closed.
	ErrPAMCampaignNotActive = errors.New("pam: the campaign is not active")
)

// PAMRequestError is a request the service refuses. Reason names the field at
// fault.
type PAMRequestError struct {
	Reason string
}

func (e *PAMRequestError) Error() string { return e.Reason }

func refusePAM(format string, args ...interface{}) error {
	return &PAMRequestError{Reason: fmt.Sprintf(format, args...)}
}

// CapabilityTier represents the privilege classification.
type CapabilityTier struct {
	Capability     string `json:"capability"`
	Tier           string `json:"tier"` // STANDARD, PRIVILEGED, SUPER_PRIVILEGED
	TierReason     string `json:"tierReason"`
	MaxTTLSeconds  int    `json:"maxTtlSeconds"`
	RequiresMFA    bool   `json:"requiresMfa"`
	RequiresTicket bool   `json:"requiresTicket"`
	RenewalAllowed bool   `json:"renewalAllowed"`
}

// ApprovalRequest represents a request for SUPER_PRIVILEGED access and its
// decision.
type ApprovalRequest struct {
	ID               uuid.UUID  `json:"id"`
	OrganizationID   uuid.UUID  `json:"organizationId"`
	AgentID          uuid.UUID  `json:"agentId"`
	Capability       string     `json:"capability"`
	ResourcePath     string     `json:"resourcePath"`
	Justification    string     `json:"justification"`
	IntentClass      string     `json:"intentClass,omitempty"`
	IntentConfidence float64    `json:"intentConfidence,omitempty"`
	RequestedBy      *uuid.UUID `json:"requestedBy,omitempty"`
	ApproverID       *uuid.UUID `json:"approverId,omitempty"`
	ApproverDecision string     `json:"approverDecision,omitempty"` // APPROVED, DENIED
	ApproverNote     string     `json:"approverNote,omitempty"`
	TicketID         string     `json:"ticketId,omitempty"`
	TicketSystem     string     `json:"ticketSystem,omitempty"`
	RequestedAt      time.Time  `json:"requestedAt"`
	ExpiresAt        time.Time  `json:"expiresAt"`
	DecidedAt        *time.Time `json:"decidedAt,omitempty"`
	Status           string     `json:"status"` // PENDING, APPROVED, DENIED, EXPIRED
}

// EmergencyDeclaration represents a break-glass declaration.
type EmergencyDeclaration struct {
	ID              uuid.UUID   `json:"id"`
	OrganizationID  uuid.UUID   `json:"organizationId"`
	IncidentID      string      `json:"incidentId"`
	DeclaredBy      string      `json:"declaredBy"`
	DeclaredAt      time.Time   `json:"declaredAt"`
	ExpiresAt       time.Time   `json:"expiresAt"`
	ExtendedUntil   *time.Time  `json:"extendedUntil,omitempty"`
	ExtendedBy      string      `json:"extendedBy,omitempty"`
	AffectedAgents  []uuid.UUID `json:"affectedAgents"`
	GrantedCaps     []string    `json:"grantedCaps"`
	Justification   string      `json:"justification"`
	Status          string      `json:"status"` // ACTIVE, EXPIRED, REVOKED
	ReviewCompleted bool        `json:"reviewCompleted"`
}

// CertificationCampaign represents a periodic access review campaign.
type CertificationCampaign struct {
	ID             uuid.UUID `json:"id"`
	OrganizationID uuid.UUID `json:"organizationId"`
	Name           string    `json:"name"`
	ScopeType      string    `json:"scopeType"` // ALL, TIER, AGENT
	ScopeValue     string    `json:"scopeValue,omitempty"`
	StartsAt       time.Time `json:"startsAt"`
	ClosesAt       time.Time `json:"closesAt"`
	Status         string    `json:"status"` // PENDING, ACTIVE, CLOSED
	CreatedBy      string    `json:"createdBy"`
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
	return &PAMService{db: db, logger: logger, now: time.Now}
}

// checkText refuses a value longer than pamMaxText characters.
func checkText(field, value string) error {
	if utf8.RuneCountInString(value) > pamMaxText {
		return refusePAM("%s is longer than %d characters", field, pamMaxText)
	}
	return nil
}

// nullIfEmpty stores an empty string as NULL.
func nullIfEmpty(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

// ============================================================================
// Tier Classification (P1)
// ============================================================================

const capabilityTierColumns = `capability, tier, tier_reason, max_ttl_seconds, requires_mfa, requires_ticket, renewal_allowed`

// GetCapabilityTier returns the privilege tier for a capability. A capability
// the registry does not hold is STANDARD.
func (s *PAMService) GetCapabilityTier(ctx context.Context, capability string) (*CapabilityTier, error) {
	var tier CapabilityTier
	err := s.db.QueryRowContext(ctx,
		`SELECT `+capabilityTierColumns+`
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

// ListCapabilityTiers returns every capability the registry holds, by name.
func (s *PAMService) ListCapabilityTiers(ctx context.Context) ([]*CapabilityTier, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+capabilityTierColumns+` FROM privileged_capability_registry ORDER BY capability`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tiers := make([]*CapabilityTier, 0)
	for rows.Next() {
		var tier CapabilityTier
		if err := rows.Scan(&tier.Capability, &tier.Tier, &tier.TierReason, &tier.MaxTTLSeconds,
			&tier.RequiresMFA, &tier.RequiresTicket, &tier.RenewalAllowed); err != nil {
			return nil, err
		}
		tiers = append(tiers, &tier)
	}
	return tiers, rows.Err()
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

const approvalColumns = `p.id, a.organization_id, p.agent_id, p.capability, p.resource_path, p.justification,
	COALESCE(p.intent_class, ''), COALESCE(p.intent_confidence, 0), p.requested_by,
	p.approver_id, COALESCE(p.approver_decision, ''), COALESCE(p.approver_note, ''),
	COALESCE(p.ticket_id, ''), COALESCE(p.ticket_system, ''), p.requested_at, p.expires_at, p.decided_at`

func (s *PAMService) scanApproval(row interface{ Scan(...any) error }) (*ApprovalRequest, error) {
	var a ApprovalRequest
	var requestedBy, approverID uuid.NullUUID
	var decidedAt sql.NullTime
	if err := row.Scan(&a.ID, &a.OrganizationID, &a.AgentID, &a.Capability, &a.ResourcePath, &a.Justification,
		&a.IntentClass, &a.IntentConfidence, &requestedBy,
		&approverID, &a.ApproverDecision, &a.ApproverNote,
		&a.TicketID, &a.TicketSystem, &a.RequestedAt, &a.ExpiresAt, &decidedAt); err != nil {
		return nil, err
	}
	if requestedBy.Valid {
		a.RequestedBy = &requestedBy.UUID
	}
	if approverID.Valid {
		a.ApproverID = &approverID.UUID
	}
	if decidedAt.Valid {
		a.DecidedAt = &decidedAt.Time
	}
	a.Status = approvalStatus(&a, s.now())
	return &a, nil
}

// approvalStatus reads an approval's status at now.
func approvalStatus(a *ApprovalRequest, now time.Time) string {
	switch {
	case a.ApproverDecision == ApprovalStatusApproved:
		return ApprovalStatusApproved
	case a.ApproverDecision == ApprovalStatusDenied:
		return ApprovalStatusDenied
	case a.ApproverDecision != "" || !now.Before(a.ExpiresAt):
		return ApprovalStatusExpired
	default:
		return ApprovalStatusPending
	}
}

// RequestApproval records a request for SUPER_PRIVILEGED access by one of the
// organization's agents. The capability must be SUPER_PRIVILEGED in the tier
// registry, and the request waits ApprovalDecisionWindow for a decision.
func (s *PAMService) RequestApproval(ctx context.Context, orgID uuid.UUID, req *ApprovalRequest) (*ApprovalRequest, error) {
	if strings.TrimSpace(req.Justification) == "" {
		return nil, refusePAM("justification is required for SUPER_PRIVILEGED access")
	}
	if strings.TrimSpace(req.Capability) == "" {
		return nil, refusePAM("capability is required")
	}
	for _, f := range []struct{ name, value string }{
		{"capability", req.Capability}, {"resourcePath", req.ResourcePath}, {"justification", req.Justification},
		{"ticketId", req.TicketID}, {"ticketSystem", req.TicketSystem},
	} {
		if err := checkText(f.name, f.value); err != nil {
			return nil, err
		}
	}

	tier, err := s.GetCapabilityTier(ctx, req.Capability)
	if err != nil {
		return nil, err
	}
	if tier.Tier != domain.CapabilityTierSuperPrivileged {
		return nil, refusePAM("capability %s is in tier %s; approvals are requested for SUPER_PRIVILEGED capabilities", req.Capability, tier.Tier)
	}
	if tier.RequiresTicket && strings.TrimSpace(req.TicketID) == "" {
		return nil, refusePAM("capability %s requires a ticket: set ticketId", req.Capability)
	}

	req.ID = uuid.New()
	req.OrganizationID = orgID
	req.RequestedAt = s.now().UTC()
	req.ExpiresAt = req.RequestedAt.Add(ApprovalDecisionWindow)
	req.ApproverID, req.ApproverDecision, req.ApproverNote, req.DecidedAt = nil, "", "", nil

	var requestedBy uuid.NullUUID
	if req.RequestedBy != nil {
		requestedBy = uuid.NullUUID{UUID: *req.RequestedBy, Valid: true}
	}
	var intentConfidence sql.NullFloat64
	if req.IntentClass != "" {
		intentConfidence = sql.NullFloat64{Float64: req.IntentConfidence, Valid: true}
	}
	// The row is written only for an agent of the organization.
	result, err := s.db.ExecContext(ctx,
		`INSERT INTO pending_approvals (id, agent_id, capability, resource_path, justification,
			intent_class, intent_confidence, ticket_id, ticket_system, requested_by, requested_at, expires_at)
		 SELECT $1, a.id, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
		 FROM agents a WHERE a.id = $2 AND a.organization_id = $13`,
		req.ID, req.AgentID, req.Capability, req.ResourcePath, req.Justification,
		nullIfEmpty(req.IntentClass), intentConfidence, nullIfEmpty(req.TicketID), nullIfEmpty(req.TicketSystem),
		requestedBy, req.RequestedAt, req.ExpiresAt, orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create approval request: %w", err)
	}
	if n, err := result.RowsAffected(); err != nil {
		return nil, err
	} else if n == 0 {
		return nil, ErrPAMNotFound
	}
	req.Status = ApprovalStatusPending

	s.logger.Info("approval requested",
		"id", req.ID, "organizationId", orgID, "agentId", req.AgentID, "capability", req.Capability,
		"expiresAt", req.ExpiresAt,
	)

	return req, nil
}

// GetApproval returns one of the organization's approvals.
func (s *PAMService) GetApproval(ctx context.Context, orgID, approvalID uuid.UUID) (*ApprovalRequest, error) {
	approval, err := s.scanApproval(s.db.QueryRowContext(ctx,
		`SELECT `+approvalColumns+`
		 FROM pending_approvals p JOIN agents a ON a.id = p.agent_id
		 WHERE p.id = $1 AND a.organization_id = $2`, approvalID, orgID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrPAMNotFound
	}
	return approval, err
}

// ListApprovals returns the organization's approvals, newest first, at most
// PAMListLimit.
func (s *PAMService) ListApprovals(ctx context.Context, orgID uuid.UUID) ([]*ApprovalRequest, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+approvalColumns+`
		 FROM pending_approvals p JOIN agents a ON a.id = p.agent_id
		 WHERE a.organization_id = $1
		 ORDER BY p.requested_at DESC, p.id
		 LIMIT $2`, orgID, PAMListLimit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	approvals := make([]*ApprovalRequest, 0)
	for rows.Next() {
		approval, err := s.scanApproval(rows)
		if err != nil {
			return nil, err
		}
		approvals = append(approvals, approval)
	}
	return approvals, rows.Err()
}

// DecideApproval records an approver's decision on a pending approval of the
// organization. The user who requested it cannot decide it.
func (s *PAMService) DecideApproval(ctx context.Context, orgID, approvalID, approverID uuid.UUID, decision, note string) (*ApprovalRequest, error) {
	if decision != ApprovalStatusApproved && decision != ApprovalStatusDenied {
		return nil, refusePAM("decision must be APPROVED or DENIED")
	}
	if err := checkText("note", note); err != nil {
		return nil, err
	}

	approval, err := s.GetApproval(ctx, orgID, approvalID)
	if err != nil {
		return nil, err
	}
	if approval.RequestedBy != nil && *approval.RequestedBy == approverID {
		return nil, ErrPAMSameUser
	}
	if approval.Status != ApprovalStatusPending {
		return nil, ErrPAMApprovalNotPending
	}

	now := s.now().UTC()
	result, err := s.db.ExecContext(ctx,
		`UPDATE pending_approvals p
		 SET approver_id = $1, approver_decision = $2, approver_note = $3, decided_at = $4
		 FROM agents a
		 WHERE p.id = $5 AND a.id = p.agent_id AND a.organization_id = $6
		   AND p.approver_decision IS NULL AND p.expires_at > $4
		   AND (p.requested_by IS NULL OR p.requested_by <> $1)`,
		approverID, decision, nullIfEmpty(note), now, approvalID, orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update approval: %w", err)
	}
	if n, err := result.RowsAffected(); err != nil {
		return nil, err
	} else if n == 0 {
		// Decided, or past its window, since it was read.
		return nil, ErrPAMApprovalNotPending
	}

	approval.ApproverID = &approverID
	approval.ApproverDecision = decision
	approval.ApproverNote = note
	approval.DecidedAt = &now
	approval.Status = decision

	s.logger.Info("approval decided",
		"id", approvalID, "organizationId", orgID, "decision", decision, "approver", approverID,
	)

	return approval, nil
}

// ============================================================================
// Break-Glass Emergency Access (P3)
// ============================================================================

const emergencyColumns = `id, organization_id, incident_id, declared_by, declared_at, expires_at,
	extended_until, COALESCE(extended_by, ''), affected_agents, granted_caps, justification, status, review_completed`

func (s *PAMService) scanEmergency(row interface{ Scan(...any) error }) (*EmergencyDeclaration, error) {
	var d EmergencyDeclaration
	var extendedUntil sql.NullTime
	agents := make([]string, 0)
	if err := row.Scan(&d.ID, &d.OrganizationID, &d.IncidentID, &d.DeclaredBy, &d.DeclaredAt, &d.ExpiresAt,
		&extendedUntil, &d.ExtendedBy, pq.Array(&agents), pq.Array(&d.GrantedCaps), &d.Justification,
		&d.Status, &d.ReviewCompleted); err != nil {
		return nil, err
	}
	if extendedUntil.Valid {
		d.ExtendedUntil = &extendedUntil.Time
	}
	d.AffectedAgents = make([]uuid.UUID, 0, len(agents))
	for _, agent := range agents {
		id, err := uuid.Parse(agent)
		if err != nil {
			return nil, fmt.Errorf("emergency declaration %s holds an affected agent that is not a UUID: %w", d.ID, err)
		}
		d.AffectedAgents = append(d.AffectedAgents, id)
	}
	if d.GrantedCaps == nil {
		d.GrantedCaps = make([]string, 0)
	}
	d.Status = emergencyStatus(&d, s.now())
	return &d, nil
}

// emergencyEnds returns when a declaration ends: its extension when it has
// one, otherwise its expiry.
func emergencyEnds(d *EmergencyDeclaration) time.Time {
	if d.ExtendedUntil != nil {
		return *d.ExtendedUntil
	}
	return d.ExpiresAt
}

// emergencyStatus reads a declaration's status at now.
func emergencyStatus(d *EmergencyDeclaration, now time.Time) string {
	if d.Status != EmergencyStatusActive {
		return d.Status
	}
	if !now.Before(emergencyEnds(d)) {
		return EmergencyStatusExpired
	}
	return EmergencyStatusActive
}

// DeclareEmergency records a break-glass declaration for agents of the
// organization. It lasts at most EmergencyMaxDuration.
func (s *PAMService) DeclareEmergency(ctx context.Context, orgID uuid.UUID, decl *EmergencyDeclaration) (*EmergencyDeclaration, error) {
	if strings.TrimSpace(decl.IncidentID) == "" {
		return nil, refusePAM("incident ID is required for break-glass access")
	}
	if strings.TrimSpace(decl.Justification) == "" {
		return nil, refusePAM("justification is required")
	}
	if err := checkText("incidentId", decl.IncidentID); err != nil {
		return nil, err
	}
	if err := checkText("justification", decl.Justification); err != nil {
		return nil, err
	}
	if len(decl.AffectedAgents) == 0 {
		return nil, refusePAM("affectedAgents must name at least one agent")
	}
	if len(decl.GrantedCaps) == 0 {
		return nil, refusePAM("grantedCaps must name at least one capability")
	}
	if len(decl.AffectedAgents) > pamMaxListEntries || len(decl.GrantedCaps) > pamMaxListEntries {
		return nil, refusePAM("affectedAgents and grantedCaps name at most %d entries each", pamMaxListEntries)
	}
	for _, capability := range decl.GrantedCaps {
		if strings.TrimSpace(capability) == "" {
			return nil, refusePAM("grantedCaps holds an empty capability")
		}
		if err := checkText("grantedCaps", capability); err != nil {
			return nil, err
		}
	}

	decl.ID = uuid.New()
	decl.OrganizationID = orgID
	decl.DeclaredAt = s.now().UTC()
	decl.Status = EmergencyStatusActive
	decl.ExtendedUntil, decl.ExtendedBy, decl.ReviewCompleted = nil, "", false

	// Hard cap: 60 minutes maximum
	maxExpiry := decl.DeclaredAt.Add(EmergencyMaxDuration)
	if decl.ExpiresAt.IsZero() || decl.ExpiresAt.After(maxExpiry) {
		decl.ExpiresAt = maxExpiry
	}
	if !decl.ExpiresAt.After(decl.DeclaredAt) {
		return nil, refusePAM("expiresAt must be after the declaration")
	}

	// Every affected agent must be one of the organization's. An agent named
	// twice is recorded once.
	decl.AffectedAgents = distinctUUIDs(decl.AffectedAgents)
	agentIDs := make([]string, 0, len(decl.AffectedAgents))
	for _, id := range decl.AffectedAgents {
		agentIDs = append(agentIDs, id.String())
	}
	var owned int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM agents WHERE organization_id = $1 AND id = ANY($2::uuid[])`,
		orgID, pq.Array(agentIDs),
	).Scan(&owned); err != nil {
		return nil, err
	}
	if owned != len(decl.AffectedAgents) {
		return nil, ErrPAMNotFound
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO emergency_declarations (id, organization_id, incident_id, declared_by, declared_at, expires_at,
			affected_agents, granted_caps, justification, status)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		decl.ID, orgID, decl.IncidentID, decl.DeclaredBy, decl.DeclaredAt, decl.ExpiresAt,
		pq_from_uuids(decl.AffectedAgents), pq.Array(sqlarray.NonNilStrings(decl.GrantedCaps)),
		decl.Justification, decl.Status,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to declare emergency: %w", err)
	}

	s.logger.Warn("EMERGENCY DECLARED",
		"id", decl.ID, "organizationId", orgID, "incident", decl.IncidentID,
		"agents", len(decl.AffectedAgents), "caps", len(decl.GrantedCaps),
		"expiresAt", decl.ExpiresAt,
	)

	return decl, nil
}

// distinctUUIDs returns ids without repeats, in their first order.
func distinctUUIDs(ids []uuid.UUID) []uuid.UUID {
	seen := make(map[uuid.UUID]struct{}, len(ids))
	out := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if _, dup := seen[id]; !dup {
			seen[id] = struct{}{}
			out = append(out, id)
		}
	}
	return out
}

// GetEmergency returns one of the organization's declarations.
func (s *PAMService) GetEmergency(ctx context.Context, orgID, emergencyID uuid.UUID) (*EmergencyDeclaration, error) {
	decl, err := s.scanEmergency(s.db.QueryRowContext(ctx,
		`SELECT `+emergencyColumns+` FROM emergency_declarations WHERE id = $1 AND organization_id = $2`,
		emergencyID, orgID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrPAMNotFound
	}
	return decl, err
}

// ListEmergencies returns the organization's declarations, newest first, at
// most PAMListLimit.
func (s *PAMService) ListEmergencies(ctx context.Context, orgID uuid.UUID) ([]*EmergencyDeclaration, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+emergencyColumns+` FROM emergency_declarations
		 WHERE organization_id = $1
		 ORDER BY declared_at DESC, id
		 LIMIT $2`, orgID, PAMListLimit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	decls := make([]*EmergencyDeclaration, 0)
	for rows.Next() {
		decl, err := s.scanEmergency(rows)
		if err != nil {
			return nil, err
		}
		decls = append(decls, decl)
	}
	return decls, rows.Err()
}

// ExtendEmergency grants an active declaration its one EmergencyExtension,
// from its expiry, never past EmergencyMaxTotal from the declaration. The
// user who declared it cannot extend it.
func (s *PAMService) ExtendEmergency(ctx context.Context, orgID, emergencyID uuid.UUID, extendedBy string) (*EmergencyDeclaration, error) {
	decl, err := s.GetEmergency(ctx, orgID, emergencyID)
	if err != nil {
		return nil, err
	}
	if decl.DeclaredBy == extendedBy {
		return nil, ErrPAMSameUser
	}
	if decl.Status != EmergencyStatusActive {
		return nil, ErrPAMEmergencyNotActive
	}
	if decl.ExtendedUntil != nil {
		return nil, ErrPAMEmergencyExtended
	}

	until := decl.ExpiresAt.Add(EmergencyExtension)
	if limit := decl.DeclaredAt.Add(EmergencyMaxTotal); until.After(limit) {
		until = limit
	}
	result, err := s.db.ExecContext(ctx,
		`UPDATE emergency_declarations
		 SET extended_until = $1, extended_by = $2
		 WHERE id = $3 AND organization_id = $4 AND status = 'ACTIVE' AND extended_until IS NULL
		   AND expires_at > $5 AND declared_by <> $2`,
		until, extendedBy, emergencyID, orgID, s.now().UTC(),
	)
	if err != nil {
		return nil, err
	}
	if n, err := result.RowsAffected(); err != nil {
		return nil, err
	} else if n == 0 {
		// Extended, revoked or expired since it was read.
		return nil, ErrPAMEmergencyNotActive
	}
	decl.ExtendedUntil = &until
	decl.ExtendedBy = extendedBy

	s.logger.Warn("emergency extended",
		"id", emergencyID, "organizationId", orgID, "extendedBy", extendedBy, "until", until,
	)
	return decl, nil
}

// RevokeEmergency ends an active declaration of the organization.
func (s *PAMService) RevokeEmergency(ctx context.Context, orgID, emergencyID uuid.UUID) (*EmergencyDeclaration, error) {
	decl, err := s.GetEmergency(ctx, orgID, emergencyID)
	if err != nil {
		return nil, err
	}
	if decl.Status != EmergencyStatusActive {
		return nil, ErrPAMEmergencyNotActive
	}

	result, err := s.db.ExecContext(ctx,
		`UPDATE emergency_declarations SET status = 'REVOKED'
		 WHERE id = $1 AND organization_id = $2 AND status = 'ACTIVE'
		   AND COALESCE(extended_until, expires_at) > $3`,
		emergencyID, orgID, s.now().UTC(),
	)
	if err != nil {
		return nil, err
	}
	if n, err := result.RowsAffected(); err != nil {
		return nil, err
	} else if n == 0 {
		return nil, ErrPAMEmergencyNotActive
	}
	decl.Status = EmergencyStatusRevoked

	s.logger.Warn("emergency revoked", "id", emergencyID, "organizationId", orgID)
	return decl, nil
}

// ============================================================================
// Certification Campaigns (P4)
// ============================================================================

const campaignColumns = `id, organization_id, name, scope_type, COALESCE(scope_value, ''), starts_at, closes_at, created_by`

func (s *PAMService) scanCampaign(row interface{ Scan(...any) error }) (*CertificationCampaign, error) {
	var c CertificationCampaign
	if err := row.Scan(&c.ID, &c.OrganizationID, &c.Name, &c.ScopeType, &c.ScopeValue,
		&c.StartsAt, &c.ClosesAt, &c.CreatedBy); err != nil {
		return nil, err
	}
	c.Status = campaignStatus(&c, s.now())
	return &c, nil
}

// campaignStatus reads a campaign's status at now.
func campaignStatus(c *CertificationCampaign, now time.Time) string {
	switch {
	case now.Before(c.StartsAt):
		return CampaignStatusPending
	case now.Before(c.ClosesAt):
		return CampaignStatusActive
	default:
		return CampaignStatusClosed
	}
}

// CreateCampaign records a certification campaign of the organization. A
// campaign with no start starts now, and one with no close closes
// CampaignDefaultLength after its start.
func (s *PAMService) CreateCampaign(ctx context.Context, orgID uuid.UUID, campaign *CertificationCampaign) (*CertificationCampaign, error) {
	campaign.Name = strings.TrimSpace(campaign.Name)
	if campaign.Name == "" {
		return nil, refusePAM("name is required")
	}
	if err := checkText("name", campaign.Name); err != nil {
		return nil, err
	}

	var scopeAgent uuid.UUID
	switch campaign.ScopeType {
	case "ALL":
		if campaign.ScopeValue != "" {
			return nil, refusePAM("scopeValue must be empty for scopeType ALL")
		}
	case "TIER":
		switch campaign.ScopeValue {
		case domain.CapabilityTierStandard, domain.CapabilityTierPrivileged, domain.CapabilityTierSuperPrivileged:
		default:
			return nil, refusePAM("scopeValue must be STANDARD, PRIVILEGED or SUPER_PRIVILEGED for scopeType TIER")
		}
	case "AGENT":
		id, err := uuid.Parse(campaign.ScopeValue)
		if err != nil {
			return nil, refusePAM("scopeValue must be an agent ID for scopeType AGENT")
		}
		scopeAgent = id
	default:
		return nil, refusePAM("scopeType must be ALL, TIER or AGENT")
	}

	now := s.now().UTC()
	if campaign.StartsAt.IsZero() {
		campaign.StartsAt = now
	}
	// Default 30-day campaign
	if campaign.ClosesAt.IsZero() {
		campaign.ClosesAt = campaign.StartsAt.Add(CampaignDefaultLength)
	}
	if !campaign.ClosesAt.After(campaign.StartsAt) {
		return nil, refusePAM("closesAt must be after startsAt")
	}
	if !campaign.ClosesAt.After(now) {
		return nil, refusePAM("closesAt must be in the future")
	}

	if scopeAgent != uuid.Nil {
		if err := s.requireAgent(ctx, orgID, scopeAgent); err != nil {
			return nil, err
		}
	}

	campaign.ID = uuid.New()
	campaign.OrganizationID = orgID
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO certification_campaigns (id, organization_id, name, scope_type, scope_value, starts_at, closes_at, status, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, 'PENDING', $8)`,
		campaign.ID, orgID, campaign.Name, campaign.ScopeType, nullIfEmpty(campaign.ScopeValue),
		campaign.StartsAt, campaign.ClosesAt, campaign.CreatedBy,
	)
	if err != nil {
		return nil, err
	}
	campaign.Status = campaignStatus(campaign, now)

	s.logger.Info("certification campaign created",
		"id", campaign.ID, "organizationId", orgID, "name", campaign.Name,
		"scope", campaign.ScopeType, "closes", campaign.ClosesAt,
	)

	return campaign, nil
}

// requireAgent returns ErrPAMNotFound unless agentID is one of the
// organization's agents.
func (s *PAMService) requireAgent(ctx context.Context, orgID, agentID uuid.UUID) error {
	var exists bool
	if err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM agents WHERE id = $1 AND organization_id = $2)`, agentID, orgID,
	).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return ErrPAMNotFound
	}
	return nil
}

// GetCampaign returns one of the organization's campaigns.
func (s *PAMService) GetCampaign(ctx context.Context, orgID, campaignID uuid.UUID) (*CertificationCampaign, error) {
	campaign, err := s.scanCampaign(s.db.QueryRowContext(ctx,
		`SELECT `+campaignColumns+` FROM certification_campaigns WHERE id = $1 AND organization_id = $2`,
		campaignID, orgID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrPAMNotFound
	}
	return campaign, err
}

// ListCampaigns returns the organization's campaigns, newest start first, at
// most PAMListLimit.
func (s *PAMService) ListCampaigns(ctx context.Context, orgID uuid.UUID) ([]*CertificationCampaign, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+campaignColumns+` FROM certification_campaigns
		 WHERE organization_id = $1
		 ORDER BY starts_at DESC, id
		 LIMIT $2`, orgID, PAMListLimit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	campaigns := make([]*CertificationCampaign, 0)
	for rows.Next() {
		campaign, err := s.scanCampaign(rows)
		if err != nil {
			return nil, err
		}
		campaigns = append(campaigns, campaign)
	}
	return campaigns, rows.Err()
}

// RecordCertificationDecision records a reviewer's decision for an agent
// capability in an active campaign of the organization. A later decision for
// the same agent and capability in the campaign replaces it.
func (s *PAMService) RecordCertificationDecision(ctx context.Context, orgID uuid.UUID, decision *CertificationDecision) error {
	switch decision.Decision {
	case "":
		return refusePAM("decision is required (CERTIFIED, MODIFIED, REVOKED, or SUSPENDED)")
	case "CERTIFIED", "MODIFIED", "REVOKED", "SUSPENDED":
	default:
		return refusePAM("decision must be CERTIFIED, MODIFIED, REVOKED or SUSPENDED")
	}
	if strings.TrimSpace(decision.Capability) == "" {
		return refusePAM("capability is required")
	}
	if err := checkText("capability", decision.Capability); err != nil {
		return err
	}
	if err := checkText("note", decision.ReviewerNote); err != nil {
		return err
	}

	campaign, err := s.GetCampaign(ctx, orgID, decision.CampaignID)
	if err != nil {
		return err
	}
	if campaign.Status != CampaignStatusActive {
		return ErrPAMCampaignNotActive
	}
	switch campaign.ScopeType {
	case "AGENT":
		if campaign.ScopeValue != decision.AgentID.String() {
			return refusePAM("the campaign covers agent %s only", campaign.ScopeValue)
		}
	case "TIER":
		tier, err := s.GetCapabilityTier(ctx, decision.Capability)
		if err != nil {
			return err
		}
		if tier.Tier != campaign.ScopeValue {
			return refusePAM("capability %s is in tier %s; the campaign covers tier %s", decision.Capability, tier.Tier, campaign.ScopeValue)
		}
	}

	now := s.now().UTC()
	decision.DecidedAt = &now
	// The row is written only for an agent of the organization; a decision
	// that replaces an earlier one keeps that row's id.
	err = s.db.QueryRowContext(ctx,
		`INSERT INTO certification_decisions (id, campaign_id, agent_id, capability, decision, reviewer_id, reviewer_note, decided_at)
		 SELECT $1, $2, a.id, $4, $5, $6, $7, $8
		 FROM agents a WHERE a.id = $3 AND a.organization_id = $9
		 ON CONFLICT (campaign_id, agent_id, capability) DO UPDATE SET
			decision = EXCLUDED.decision, reviewer_id = EXCLUDED.reviewer_id,
			reviewer_note = EXCLUDED.reviewer_note, decided_at = EXCLUDED.decided_at
		 RETURNING id`,
		uuid.New(), decision.CampaignID, decision.AgentID, decision.Capability,
		decision.Decision, decision.ReviewerID, nullIfEmpty(decision.ReviewerNote), decision.DecidedAt, orgID,
	).Scan(&decision.ID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrPAMNotFound
	}
	return err
}

// ListCertificationDecisions returns the decisions of one of the
// organization's campaigns, latest first.
func (s *PAMService) ListCertificationDecisions(ctx context.Context, orgID, campaignID uuid.UUID) ([]*CertificationDecision, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT d.id, d.campaign_id, d.agent_id, d.capability, COALESCE(d.decision, ''),
			COALESCE(d.reviewer_id, ''), COALESCE(d.reviewer_note, ''), d.decided_at
		 FROM certification_decisions d
		 JOIN certification_campaigns c ON c.id = d.campaign_id
		 WHERE d.campaign_id = $1 AND c.organization_id = $2
		 ORDER BY d.decided_at DESC NULLS LAST, d.id`, campaignID, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	decisions := make([]*CertificationDecision, 0)
	for rows.Next() {
		var d CertificationDecision
		var decidedAt sql.NullTime
		if err := rows.Scan(&d.ID, &d.CampaignID, &d.AgentID, &d.Capability, &d.Decision,
			&d.ReviewerID, &d.ReviewerNote, &decidedAt); err != nil {
			return nil, err
		}
		if decidedAt.Valid {
			d.DecidedAt = &decidedAt.Time
		}
		decisions = append(decisions, &d)
	}
	return decisions, rows.Err()
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
