package handlers

import (
	"context"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// FGAAuthorizer is the slice of FGAEngine that AuthorizeHandler depends on.
// Defined as an interface so handler tests can substitute a fake without
// constructing the full FGAEngine dependency tree (DB, AgentService, NanoMind
// daemon, Registry ASC).
type FGAAuthorizer interface {
	Authorize(ctx context.Context, req *application.FGARequest) (*application.FGAResult, error)
}

// AuthorizeHandler exposes FGAEngine.Authorize over HTTP. It is the entry
// point that produces the parent `fga.authorize` span and its 5-step child
// tree against the real backend (vs the synthetic gen-signal harness).
//
// SECURITY: FGAEngine is org-blind — FGARequest carries no caller-org
// field, and FGAEngine.Authorize never reads any caller context.
// The handler-layer agentRepo + LoadOwned is the SOLE tenant-scope
// gate. Without it, any authenticated caller in org A could POST
// /agents/<victim-agent-in-org-B>/authorize and receive the full
// 5-step FGA decision against the victim's policies + trust score +
// scan verdict, plus pollute the fga.authorize OTel span tree
// against the victim org.
type AuthorizeHandler struct {
	fga       FGAAuthorizer
	agentRepo domain.AgentRepository
}

// NewAuthorizeHandler constructs an AuthorizeHandler against a concrete
// FGAEngine. Test code should use NewAuthorizeHandlerWithInterface.
func NewAuthorizeHandler(fga *application.FGAEngine, agentRepo domain.AgentRepository) *AuthorizeHandler {
	return &AuthorizeHandler{fga: fga, agentRepo: agentRepo}
}

// NewAuthorizeHandlerWithInterface constructs an AuthorizeHandler against any
// FGAAuthorizer implementation, for testability.
func NewAuthorizeHandlerWithInterface(fga FGAAuthorizer, agentRepo domain.AgentRepository) *AuthorizeHandler {
	return &AuthorizeHandler{fga: fga, agentRepo: agentRepo}
}

// authorizeRequestBody mirrors application.FGARequest minus AgentID. The
// agent ID is taken from the URL path so the route parameter is the single
// source of truth and cannot be spoofed by a request body that disagrees.
type authorizeRequestBody struct {
	AgentID    *uuid.UUID `json:"agentId,omitempty"`
	Capability string     `json:"capability"`
	Resource   string     `json:"resource,omitempty"`
	Attributes []string   `json:"attributes,omitempty"`
	Action     string     `json:"action,omitempty"`
	DataClass  string     `json:"dataClass,omitempty"`
}

// Authorize executes an FGA decision for the agent in the URL path.
//
// @Summary FGA authorization decision
// @Description Run the 5-step Fine-Grained Authorization flow for an agent and capability. Returns the decision plus the steps that were evaluated. DENY is returned with HTTP 200 (the request succeeded; the result is in the body); HTTP 5xx is reserved for infrastructure failures.
// @Tags agents
// @Accept json
// @Produce json
// @Param id path string true "Agent ID"
// @Param request body authorizeRequestBody true "FGA authorization request"
// @Success 200 {object} application.FGAResult
// @Failure 400 {object} ErrorResponse "Invalid agent ID or request body"
// @Failure 500 {object} ErrorResponse "Authorization engine failure"
// @Router /agents/{id}/authorize [post]
func (h *AuthorizeHandler) Authorize(c fiber.Ctx) error {
	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error: "Invalid agent ID",
		})
	}

	var body authorizeRequestBody
	if err := c.Bind().JSON(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error: "Invalid request body",
		})
	}

	// If the body sets agentId, it must agree with the path. The path wins.
	if body.AgentID != nil && *body.AgentID != agentID {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error: "agentId in body does not match path parameter",
		})
	}

	if body.Capability == "" {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error: "capability is required",
		})
	}

	// SECURITY: tenant-scope the agent before invoking FGAEngine.
	// FGAEngine is org-blind (no CallerOrgID field on FGARequest;
	// no caller-context reads in FGAEngine.Authorize), so the
	// handler-layer LoadOwned is the only gate. Cross-tenant and
	// not-found both collapse to a fixed 404, preventing
	// cross-tenant FGA decision leaks and victim-org OTel span
	// pollution.
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}
	if LoadOwned(c, h.agentRepo.GetByID, agentID, orgID, agentOrgID) == nil {
		return nil
	}

	req := &application.FGARequest{
		AgentID:    agentID,
		Capability: body.Capability,
		Resource:   body.Resource,
		Attributes: body.Attributes,
		Action:     body.Action,
		DataClass:  body.DataClass,
	}

	result, err := h.fga.Authorize(c.Context(), req)
	if err != nil {
		// FGAEngine's deferred span finalizer already emits the
		// outcome=ERROR metric/log, so we just translate to HTTP. The
		// engine returns its synthesized ERROR result via the named
		// return; surface it so the caller can read DeniedReason.
		status := fiber.StatusInternalServerError
		if result == nil {
			return c.Status(status).JSON(ErrorResponse{Error: "authorization engine failure"})
		}
		return c.Status(status).JSON(result)
	}

	return c.Status(fiber.StatusOK).JSON(result)
}
