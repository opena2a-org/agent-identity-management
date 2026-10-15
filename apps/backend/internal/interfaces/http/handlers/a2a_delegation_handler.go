package handlers

import (
	"errors"
	"log"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// A2ADelegationHandler serves /api/v1/a2a/delegations: a manager grants and
// revokes delegations between the organization's agents, and any caller in
// the organization reads them and verifies a delegation chain.
type A2ADelegationHandler struct {
	service      *application.A2ADelegationService
	auditService *application.AuditService
}

// NewA2ADelegationHandler creates a new A2ADelegationHandler
func NewA2ADelegationHandler(service *application.A2ADelegationService, auditService *application.AuditService) *A2ADelegationHandler {
	return &A2ADelegationHandler{service: service, auditService: auditService}
}

// delegationOrgID extracts OrganizationID from a *domain.A2ADelegation.
func delegationOrgID(d *domain.A2ADelegation) uuid.UUID { return d.OrganizationID }

// delegationLoader reads the caller's delegation for LoadOwned, which answers
// 404 for one that does not exist and for one of another organization alike.
// The service's read is scoped to the organization in SQL as well.
func (h *A2ADelegationHandler) delegationLoader(c fiber.Ctx, orgID uuid.UUID) func(uuid.UUID) (*domain.A2ADelegation, error) {
	return func(id uuid.UUID) (*domain.A2ADelegation, error) {
		return h.service.GetDelegation(c.Context(), orgID, id)
	}
}

// CreateDelegation grants a delegation from one of the organization's agents
// to another.
// POST /api/v1/a2a/delegations
func (h *A2ADelegationHandler) CreateDelegation(c fiber.Ctx) error {
	orgID, userID, err := RequireOrgAndUserID(c)
	if err != nil {
		return err
	}

	var req application.CreateA2ADelegationRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	d, refusal, err := h.service.CreateDelegation(c.Context(), orgID, userID, req)
	switch {
	case errors.Is(err, application.ErrA2ADelegationAgentNotFound):
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Agent not found"})
	case errors.Is(err, domain.ErrA2ADelegationActive):
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": domain.A2ADelegationActiveMessage})
	case err != nil:
		return respondServerError(c, fiber.StatusInternalServerError, err)
	case refusal != "":
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": refusal})
	}

	h.logAudit(c, orgID, userID, domain.AuditActionCreate, d.ID, map[string]interface{}{
		"delegatorAgentId":       d.DelegatorAgentID,
		"delegateAgentId":        d.DelegateAgentID,
		"skillIds":               d.SkillIDs,
		"maxChainDepth":          d.MaxChainDepth,
		"trustAttenuationFactor": d.TrustAttenuationFactor,
		"minDelegatedTrustScore": d.MinDelegatedTrustScore,
		"canDelegateFurther":     d.CanDelegateFurther,
	})
	return c.Status(fiber.StatusCreated).JSON(d)
}

// ListDelegations lists the organization's delegations, newest first.
// GET /api/v1/a2a/delegations
func (h *A2ADelegationHandler) ListDelegations(c fiber.Ctx) error {
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}
	p := ParsePaginationWithDefaults(c, 50, 100)

	delegations, err := h.service.ListDelegations(c.Context(), orgID, p.Limit, p.Offset)
	if err != nil {
		return respondServerError(c, fiber.StatusInternalServerError, err)
	}
	return c.JSON(fiber.Map{
		"delegations": delegations,
		"limit":       p.Limit,
		"offset":      p.Offset,
	})
}

// GetDelegation returns one of the organization's delegations.
// GET /api/v1/a2a/delegations/:id
func (h *A2ADelegationHandler) GetDelegation(c fiber.Ctx) error {
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid delegation ID"})
	}

	d := LoadOwned(c, h.delegationLoader(c, orgID), id, orgID, delegationOrgID)
	if d == nil {
		return nil // LoadOwned wrote the 404
	}
	return c.JSON(d)
}

// RevokeDelegation marks one of the organization's delegations inactive. A
// chain that holds it no longer verifies.
// POST /api/v1/a2a/delegations/:id/revoke
func (h *A2ADelegationHandler) RevokeDelegation(c fiber.Ctx) error {
	orgID, userID, err := RequireOrgAndUserID(c)
	if err != nil {
		return err
	}
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid delegation ID"})
	}

	// The ownership check precedes the write and the audit record, so another
	// organization's ID changes nothing and records nothing.
	if LoadOwned(c, h.delegationLoader(c, orgID), id, orgID, delegationOrgID) == nil {
		return nil // LoadOwned wrote the 404
	}
	d, err := h.service.RevokeDelegation(c.Context(), orgID, id)
	switch {
	case errors.Is(err, domain.ErrA2ADelegationNotFound):
		respondResourceNotFound(c)
		return nil
	case err != nil:
		return respondServerError(c, fiber.StatusInternalServerError, err)
	}

	h.logAudit(c, orgID, userID, domain.AuditActionRevoke, d.ID, map[string]interface{}{
		"delegatorAgentId": d.DelegatorAgentID,
		"delegateAgentId":  d.DelegateAgentID,
	})
	return c.JSON(d)
}

// verifyA2ADelegationChainRequest is the body of
// POST /api/v1/a2a/delegations/verify: the chain's delegation IDs, root
// delegation first, and optionally the skill the chain is used for.
type verifyA2ADelegationChainRequest struct {
	DelegationIDs []uuid.UUID `json:"delegationIds"`
	SkillID       string      `json:"skillId"`
}

// VerifyDelegationChain verifies a chain of the organization's delegations
// and answers the verdict with each delegation's effective trust. A chain
// that fails a check is answered 200 with valid false; a request naming a
// delegation the organization does not hold is answered 404.
// POST /api/v1/a2a/delegations/verify
func (h *A2ADelegationHandler) VerifyDelegationChain(c fiber.Ctx) error {
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}

	var req verifyA2ADelegationChainRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	verdict, refusal, err := h.service.VerifyDelegationChain(c.Context(), orgID, req.DelegationIDs, req.SkillID)
	var notFound *application.A2ADelegationNotFoundError
	switch {
	case errors.As(err, &notFound):
		// The ID is one the caller sent; the line is stated here.
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error":        "Delegation not found",
			"delegationId": notFound.DelegationID,
		})
	case err != nil:
		return respondServerError(c, fiber.StatusInternalServerError, err)
	case refusal != "":
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": refusal})
	}
	return c.JSON(verdict)
}

// logAudit records a delegation write. The write has already been made, so a
// record that cannot be written is logged here and does not change the answer.
func (h *A2ADelegationHandler) logAudit(c fiber.Ctx, orgID, userID uuid.UUID, action domain.AuditAction, id uuid.UUID, metadata map[string]interface{}) {
	if h.auditService == nil {
		return
	}
	if err := h.auditService.LogAction(c.Context(), orgID, userID, action, "a2a_delegation", id,
		c.IP(), c.Get("User-Agent"), metadata); err != nil {
		log.Printf("a2a delegation %s: audit record (%s) not written: %v", id, action, err)
	}
}
