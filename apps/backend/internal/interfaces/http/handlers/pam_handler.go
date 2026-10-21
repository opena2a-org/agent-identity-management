package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// PAMServicer is what PAMHandler needs of the PAM service. Every method but
// ListCapabilityTiers names the caller's organization.
type PAMServicer interface {
	ListCapabilityTiers(ctx context.Context) ([]*application.CapabilityTier, error)
	RequestApproval(ctx context.Context, orgID uuid.UUID, req *application.ApprovalRequest) (*application.ApprovalRequest, error)
	GetApproval(ctx context.Context, orgID, approvalID uuid.UUID) (*application.ApprovalRequest, error)
	ListApprovals(ctx context.Context, orgID uuid.UUID) ([]*application.ApprovalRequest, error)
	DecideApproval(ctx context.Context, orgID, approvalID, approverID uuid.UUID, decision, note string) (*application.ApprovalRequest, error)
	DeclareEmergency(ctx context.Context, orgID uuid.UUID, decl *application.EmergencyDeclaration) (*application.EmergencyDeclaration, error)
	GetEmergency(ctx context.Context, orgID, emergencyID uuid.UUID) (*application.EmergencyDeclaration, error)
	ListEmergencies(ctx context.Context, orgID uuid.UUID) ([]*application.EmergencyDeclaration, error)
	ExtendEmergency(ctx context.Context, orgID, emergencyID uuid.UUID, extendedBy string) (*application.EmergencyDeclaration, error)
	RevokeEmergency(ctx context.Context, orgID, emergencyID uuid.UUID) (*application.EmergencyDeclaration, error)
	CreateCampaign(ctx context.Context, orgID uuid.UUID, campaign *application.CertificationCampaign) (*application.CertificationCampaign, error)
	GetCampaign(ctx context.Context, orgID, campaignID uuid.UUID) (*application.CertificationCampaign, error)
	ListCampaigns(ctx context.Context, orgID uuid.UUID) ([]*application.CertificationCampaign, error)
	RecordCertificationDecision(ctx context.Context, orgID uuid.UUID, decision *application.CertificationDecision) error
	ListCertificationDecisions(ctx context.Context, orgID, campaignID uuid.UUID) ([]*application.CertificationDecision, error)
}

// PAMHandler serves /api/v1/pam: the capability tier registry, approval
// requests for SUPER_PRIVILEGED capabilities, break-glass emergency
// declarations and certification campaigns of the caller's organization.
// Each write records an audit row; the route table sets who may write.
type PAMHandler struct {
	service PAMServicer
	audit   AuditServicer
}

func NewPAMHandler(service PAMServicer, audit AuditServicer) *PAMHandler {
	return &PAMHandler{service: service, audit: audit}
}

// Audit resource types of the PAM records.
const (
	pamApprovalResource              = "pam_approval"
	pamEmergencyResource             = "pam_emergency"
	pamCampaignResource              = "pam_certification_campaign"
	pamCertificationDecisionResource = "pam_certification_decision"
)

// Request bodies. A field a body may not carry is refused by name.
type pamApprovalBody struct {
	AgentID       string `json:"agentId"`
	Capability    string `json:"capability"`
	ResourcePath  string `json:"resourcePath"`
	Justification string `json:"justification"`
	TicketID      string `json:"ticketId"`
	TicketSystem  string `json:"ticketSystem"`
}

type pamApprovalDecisionBody struct {
	Decision string `json:"decision"`
	Note     string `json:"note"`
}

type pamEmergencyBody struct {
	IncidentID     string     `json:"incidentId"`
	Justification  string     `json:"justification"`
	AffectedAgents []string   `json:"affectedAgents"`
	GrantedCaps    []string   `json:"grantedCaps"`
	ExpiresAt      *time.Time `json:"expiresAt"`
}

type pamCampaignBody struct {
	Name       string     `json:"name"`
	ScopeType  string     `json:"scopeType"`
	ScopeValue string     `json:"scopeValue"`
	StartsAt   *time.Time `json:"startsAt"`
	ClosesAt   *time.Time `json:"closesAt"`
}

type pamCertificationDecisionBody struct {
	AgentID    string `json:"agentId"`
	Capability string `json:"capability"`
	Decision   string `json:"decision"`
	Note       string `json:"note"`
}

// decodePAMBody decodes a JSON object body into v. It returns the reason for
// a 400, or "" when v holds the body.
func decodePAMBody(body []byte, v interface{}) string {
	if trimmed := bytes.TrimSpace(body); len(trimmed) == 0 || trimmed[0] != '{' {
		return "Invalid request body"
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		if name, ok := application.UnknownJSONField(err); ok {
			return "unknown field: " + name
		}
		return "Invalid request body"
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return "Invalid request body"
	}
	return ""
}

// respondPAMError answers a service error: a refused request is 400 with its
// reason, a second-user rule is 403 with sameUser, a record or agent that is
// not the caller's organization's is 404, a record in a state the action does
// not apply to is 409, and anything else is respondServerError.
func respondPAMError(c fiber.Ctx, err error, sameUser string) error {
	var invalid *application.PAMRequestError
	switch {
	case errors.As(err, &invalid):
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": invalid.Reason})
	case errors.Is(err, application.ErrPAMNotFound):
		respondResourceNotFound(c)
		return nil
	case errors.Is(err, application.ErrPAMSameUser):
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": sameUser})
	case errors.Is(err, application.ErrPAMApprovalNotPending):
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "the approval is not pending: it was decided or its decision window closed"})
	case errors.Is(err, application.ErrPAMEmergencyNotActive):
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "the emergency declaration is not active: it expired or was revoked"})
	case errors.Is(err, application.ErrPAMEmergencyExtended):
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "the emergency declaration was already extended"})
	case errors.Is(err, application.ErrPAMCampaignNotActive):
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "the campaign is not active: it has not started or has closed"})
	}
	return respondServerError(c, fiber.StatusInternalServerError, err)
}

// loadApproval returns the caller's organization's approval the :id parameter
// names. Otherwise it answers, 400 for an id that is not a UUID and 404 for an
// approval the organization does not have, and returns nil.
func (h *PAMHandler) loadApproval(c fiber.Ctx, orgID uuid.UUID) *application.ApprovalRequest {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		_ = c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid approval id"})
		return nil
	}
	return LoadOwned(c, func(id uuid.UUID) (*application.ApprovalRequest, error) {
		return h.service.GetApproval(c.Context(), orgID, id)
	}, id, orgID, func(a *application.ApprovalRequest) uuid.UUID { return a.OrganizationID })
}

// loadEmergency returns the caller's organization's declaration the :id
// parameter names, answering 400 or 404 as loadApproval does.
func (h *PAMHandler) loadEmergency(c fiber.Ctx, orgID uuid.UUID) *application.EmergencyDeclaration {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		_ = c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid emergency declaration id"})
		return nil
	}
	return LoadOwned(c, func(id uuid.UUID) (*application.EmergencyDeclaration, error) {
		return h.service.GetEmergency(c.Context(), orgID, id)
	}, id, orgID, func(d *application.EmergencyDeclaration) uuid.UUID { return d.OrganizationID })
}

// loadCampaign returns the caller's organization's campaign the :id parameter
// names, answering 400 or 404 as loadApproval does.
func (h *PAMHandler) loadCampaign(c fiber.Ctx, orgID uuid.UUID) *application.CertificationCampaign {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		_ = c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid campaign id"})
		return nil
	}
	return LoadOwned(c, func(id uuid.UUID) (*application.CertificationCampaign, error) {
		return h.service.GetCampaign(c.Context(), orgID, id)
	}, id, orgID, func(campaign *application.CertificationCampaign) uuid.UUID { return campaign.OrganizationID })
}

// logAudit writes a PAM record's audit row. A failure is logged and does not
// change the answer.
func (h *PAMHandler) logAudit(c fiber.Ctx, orgID, userID uuid.UUID, action domain.AuditAction, resourceType string, resourceID uuid.UUID, metadata map[string]interface{}) {
	if h.audit == nil {
		return
	}
	if err := h.audit.LogAction(c.Context(), orgID, userID, action, resourceType, resourceID, c.IP(), c.Get("User-Agent"), metadata); err != nil {
		log.Printf("pam %s %s: the %s audit row was not written: %v", resourceType, resourceID, action, err)
	}
}

// ListTiers lists the capability tier registry.
// @Summary List capability tiers
// @Tags pam
// @Produce json
// @Router /api/v1/pam/tiers [get]
func (h *PAMHandler) ListTiers(c fiber.Ctx) error {
	_, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}
	tiers, err := h.service.ListCapabilityTiers(c.Context())
	if err != nil {
		return respondServerError(c, fiber.StatusInternalServerError, err)
	}
	return c.JSON(fiber.Map{"tiers": tiers, "total": len(tiers)})
}

// RequestApproval records a request for SUPER_PRIVILEGED access by one of the
// organization's agents.
// @Summary Request approval for a SUPER_PRIVILEGED capability
// @Tags pam
// @Accept json
// @Produce json
// @Router /api/v1/pam/approvals [post]
func (h *PAMHandler) RequestApproval(c fiber.Ctx) error {
	orgID, userID, err := RequireOrgAndUserID(c)
	if err != nil {
		return err
	}
	var body pamApprovalBody
	if reason := decodePAMBody(c.Body(), &body); reason != "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": reason})
	}
	agentID, err := uuid.Parse(body.AgentID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "agentId must be an agent ID"})
	}

	approval, err := h.service.RequestApproval(c.Context(), orgID, &application.ApprovalRequest{
		AgentID:       agentID,
		Capability:    body.Capability,
		ResourcePath:  body.ResourcePath,
		Justification: body.Justification,
		TicketID:      body.TicketID,
		TicketSystem:  body.TicketSystem,
		RequestedBy:   &userID,
	})
	if err != nil {
		return respondPAMError(c, err, "")
	}
	h.logAudit(c, orgID, userID, domain.AuditActionCreate, pamApprovalResource, approval.ID, map[string]interface{}{
		"agentId":    approval.AgentID.String(),
		"capability": approval.Capability,
		"expiresAt":  approval.ExpiresAt,
	})
	return c.Status(fiber.StatusCreated).JSON(approval)
}

// ListApprovals lists the organization's approvals, newest first.
// @Summary List approvals
// @Tags pam
// @Produce json
// @Router /api/v1/pam/approvals [get]
func (h *PAMHandler) ListApprovals(c fiber.Ctx) error {
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}
	approvals, err := h.service.ListApprovals(c.Context(), orgID)
	if err != nil {
		return respondServerError(c, fiber.StatusInternalServerError, err)
	}
	return c.JSON(fiber.Map{"approvals": approvals, "total": len(approvals)})
}

// GetApproval returns one of the organization's approvals with its status.
// @Summary Get an approval
// @Tags pam
// @Produce json
// @Router /api/v1/pam/approvals/{id} [get]
func (h *PAMHandler) GetApproval(c fiber.Ctx) error {
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}
	approval := h.loadApproval(c, orgID)
	if approval == nil {
		return nil
	}
	return c.JSON(approval)
}

// DecideApproval approves or denies a pending approval. The user who
// requested it cannot decide it.
// @Summary Decide an approval
// @Tags pam
// @Accept json
// @Produce json
// @Router /api/v1/pam/approvals/{id}/decision [post]
func (h *PAMHandler) DecideApproval(c fiber.Ctx) error {
	orgID, userID, err := RequireOrgAndUserID(c)
	if err != nil {
		return err
	}
	approval := h.loadApproval(c, orgID)
	if approval == nil {
		return nil
	}
	var body pamApprovalDecisionBody
	if reason := decodePAMBody(c.Body(), &body); reason != "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": reason})
	}

	decided, err := h.service.DecideApproval(c.Context(), orgID, approval.ID, userID, body.Decision, body.Note)
	if err != nil {
		return respondPAMError(c, err, "the user who requested an approval cannot decide it")
	}
	h.logAudit(c, orgID, userID, domain.AuditActionUpdate, pamApprovalResource, decided.ID, map[string]interface{}{
		"agentId":    decided.AgentID.String(),
		"capability": decided.Capability,
		"decision":   decided.ApproverDecision,
	})
	return c.JSON(decided)
}

// DeclareEmergency records a break-glass declaration for agents of the
// organization.
// @Summary Declare a break-glass emergency
// @Tags pam
// @Accept json
// @Produce json
// @Router /api/v1/pam/emergencies [post]
func (h *PAMHandler) DeclareEmergency(c fiber.Ctx) error {
	orgID, userID, err := RequireOrgAndUserID(c)
	if err != nil {
		return err
	}
	var body pamEmergencyBody
	if reason := decodePAMBody(c.Body(), &body); reason != "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": reason})
	}
	agents := make([]uuid.UUID, 0, len(body.AffectedAgents))
	for _, raw := range body.AffectedAgents {
		id, err := uuid.Parse(raw)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "affectedAgents must hold agent IDs"})
		}
		agents = append(agents, id)
	}
	decl := &application.EmergencyDeclaration{
		IncidentID:     body.IncidentID,
		Justification:  body.Justification,
		AffectedAgents: agents,
		GrantedCaps:    body.GrantedCaps,
		DeclaredBy:     userID.String(),
	}
	if body.ExpiresAt != nil {
		decl.ExpiresAt = *body.ExpiresAt
	}

	declared, err := h.service.DeclareEmergency(c.Context(), orgID, decl)
	if err != nil {
		return respondPAMError(c, err, "")
	}
	h.logAudit(c, orgID, userID, domain.AuditActionCreate, pamEmergencyResource, declared.ID, map[string]interface{}{
		"incidentId":     declared.IncidentID,
		"affectedAgents": len(declared.AffectedAgents),
		"grantedCaps":    declared.GrantedCaps,
		"expiresAt":      declared.ExpiresAt,
	})
	return c.Status(fiber.StatusCreated).JSON(declared)
}

// ListEmergencies lists the organization's break-glass declarations, newest
// first.
// @Summary List break-glass declarations
// @Tags pam
// @Produce json
// @Router /api/v1/pam/emergencies [get]
func (h *PAMHandler) ListEmergencies(c fiber.Ctx) error {
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}
	decls, err := h.service.ListEmergencies(c.Context(), orgID)
	if err != nil {
		return respondServerError(c, fiber.StatusInternalServerError, err)
	}
	return c.JSON(fiber.Map{"emergencies": decls, "total": len(decls)})
}

// ExtendEmergency grants an active declaration its one extension. The user who
// declared it cannot extend it.
// @Summary Extend a break-glass declaration
// @Tags pam
// @Produce json
// @Router /api/v1/pam/emergencies/{id}/extend [post]
func (h *PAMHandler) ExtendEmergency(c fiber.Ctx) error {
	orgID, userID, err := RequireOrgAndUserID(c)
	if err != nil {
		return err
	}
	decl := h.loadEmergency(c, orgID)
	if decl == nil {
		return nil
	}
	extended, err := h.service.ExtendEmergency(c.Context(), orgID, decl.ID, userID.String())
	if err != nil {
		return respondPAMError(c, err, "the user who declared an emergency cannot extend it")
	}
	h.logAudit(c, orgID, userID, domain.AuditActionUpdate, pamEmergencyResource, extended.ID, map[string]interface{}{
		"incidentId":    extended.IncidentID,
		"extendedUntil": extended.ExtendedUntil,
	})
	return c.JSON(extended)
}

// RevokeEmergency ends an active declaration.
// @Summary Revoke a break-glass declaration
// @Tags pam
// @Produce json
// @Router /api/v1/pam/emergencies/{id}/revoke [post]
func (h *PAMHandler) RevokeEmergency(c fiber.Ctx) error {
	orgID, userID, err := RequireOrgAndUserID(c)
	if err != nil {
		return err
	}
	decl := h.loadEmergency(c, orgID)
	if decl == nil {
		return nil
	}
	revoked, err := h.service.RevokeEmergency(c.Context(), orgID, decl.ID)
	if err != nil {
		return respondPAMError(c, err, "")
	}
	h.logAudit(c, orgID, userID, domain.AuditActionRevoke, pamEmergencyResource, revoked.ID, map[string]interface{}{
		"incidentId": revoked.IncidentID,
	})
	return c.JSON(revoked)
}

// CreateCampaign records a certification campaign of the organization.
// @Summary Create a certification campaign
// @Tags pam
// @Accept json
// @Produce json
// @Router /api/v1/pam/campaigns [post]
func (h *PAMHandler) CreateCampaign(c fiber.Ctx) error {
	orgID, userID, err := RequireOrgAndUserID(c)
	if err != nil {
		return err
	}
	var body pamCampaignBody
	if reason := decodePAMBody(c.Body(), &body); reason != "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": reason})
	}
	campaign := &application.CertificationCampaign{
		Name:       body.Name,
		ScopeType:  body.ScopeType,
		ScopeValue: body.ScopeValue,
		CreatedBy:  userID.String(),
	}
	if body.StartsAt != nil {
		campaign.StartsAt = *body.StartsAt
	}
	if body.ClosesAt != nil {
		campaign.ClosesAt = *body.ClosesAt
	}

	created, err := h.service.CreateCampaign(c.Context(), orgID, campaign)
	if err != nil {
		return respondPAMError(c, err, "")
	}
	h.logAudit(c, orgID, userID, domain.AuditActionCreate, pamCampaignResource, created.ID, map[string]interface{}{
		"scopeType": created.ScopeType,
		"startsAt":  created.StartsAt,
		"closesAt":  created.ClosesAt,
	})
	return c.Status(fiber.StatusCreated).JSON(created)
}

// ListCampaigns lists the organization's certification campaigns, newest
// start first.
// @Summary List certification campaigns
// @Tags pam
// @Produce json
// @Router /api/v1/pam/campaigns [get]
func (h *PAMHandler) ListCampaigns(c fiber.Ctx) error {
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}
	campaigns, err := h.service.ListCampaigns(c.Context(), orgID)
	if err != nil {
		return respondServerError(c, fiber.StatusInternalServerError, err)
	}
	return c.JSON(fiber.Map{"campaigns": campaigns, "total": len(campaigns)})
}

// RecordCertificationDecision records a reviewer's decision on an agent's
// capability in an active campaign.
// @Summary Record a certification decision
// @Tags pam
// @Accept json
// @Produce json
// @Router /api/v1/pam/campaigns/{id}/decisions [post]
func (h *PAMHandler) RecordCertificationDecision(c fiber.Ctx) error {
	orgID, userID, err := RequireOrgAndUserID(c)
	if err != nil {
		return err
	}
	campaign := h.loadCampaign(c, orgID)
	if campaign == nil {
		return nil
	}
	var body pamCertificationDecisionBody
	if reason := decodePAMBody(c.Body(), &body); reason != "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": reason})
	}
	agentID, err := uuid.Parse(body.AgentID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "agentId must be an agent ID"})
	}

	decision := &application.CertificationDecision{
		CampaignID:   campaign.ID,
		AgentID:      agentID,
		Capability:   body.Capability,
		Decision:     body.Decision,
		ReviewerID:   userID.String(),
		ReviewerNote: body.Note,
	}
	if err := h.service.RecordCertificationDecision(c.Context(), orgID, decision); err != nil {
		return respondPAMError(c, err, "")
	}
	h.logAudit(c, orgID, userID, domain.AuditActionCreate, pamCertificationDecisionResource, decision.ID, map[string]interface{}{
		"campaignId": campaign.ID.String(),
		"agentId":    decision.AgentID.String(),
		"capability": decision.Capability,
		"decision":   decision.Decision,
	})
	return c.JSON(decision)
}

// ListCertificationDecisions lists the decisions of one of the organization's
// campaigns.
// @Summary List a campaign's certification decisions
// @Tags pam
// @Produce json
// @Router /api/v1/pam/campaigns/{id}/decisions [get]
func (h *PAMHandler) ListCertificationDecisions(c fiber.Ctx) error {
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}
	campaign := h.loadCampaign(c, orgID)
	if campaign == nil {
		return nil
	}
	decisions, err := h.service.ListCertificationDecisions(c.Context(), orgID, campaign.ID)
	if err != nil {
		return respondServerError(c, fiber.StatusInternalServerError, err)
	}
	return c.JSON(fiber.Map{"decisions": decisions, "total": len(decisions)})
}
