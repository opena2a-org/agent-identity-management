package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

type CapabilityRequestHandlers struct {
	service   *application.CapabilityRequestService
	agentRepo domain.AgentRepository
}

func NewCapabilityRequestHandlers(service *application.CapabilityRequestService, agentRepo domain.AgentRepository) *CapabilityRequestHandlers {
	return &CapabilityRequestHandlers{
		service:   service,
		agentRepo: agentRepo,
	}
}

// CreateCapabilityRequest godoc
// @Summary Create a new capability request
// @Description Agents can request additional capabilities after registration. The requester is automatically derived from the agent's owner.
// @Tags capability-requests
// @Accept json
// @Produce json
// @Param id path string true "Agent ID"
// @Param request body domain.CreateCapabilityRequestInput true "Capability request details"
// @Success 201 {object} domain.CapabilityRequest
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Router /api/v1/sdk-api/agents/{id}/capability-requests [post]
func (h *CapabilityRequestHandlers) CreateCapabilityRequest(c fiber.Ctx) error {
	// Get agent ID from path
	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid agent ID",
		})
	}

	// SECURITY (A3d-ix): the PQC SDK auth middleware authenticates the
	// caller via the X-Agent-ID header, but this handler trusts the
	// URL path :id for the target agent — these are independent
	// values. Without LoadOwned, an SDK client with X-Agent-ID =
	// agent-in-org-A could send the body to a URL containing
	// agent-in-org-B and plant a phantom capability request on the
	// victim's agent (the handler used `agent.CreatedBy` from the
	// victim's record as RequestedBy). LoadOwned collapses any
	// cross-tenant agentID to a fixed 404 BEFORE the service call.
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}
	agent := LoadOwned(c, h.agentRepo.GetByID, agentID, orgID, agentOrgID)
	if agent == nil {
		return nil
	}

	// Parse request body
	type RequestBody struct {
		CapabilityType string `json:"capabilityType" validate:"required"`
		Reason         string `json:"reason" validate:"required,min=10"`
	}

	var req RequestBody
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	// Validate required fields
	if req.CapabilityType == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "capability_type is required",
		})
	}

	if len(req.Reason) < 10 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "reason must be at least 10 characters",
		})
	}

	// Create capability request input
	// Use the agent's CreatedBy field (owner) as the RequestedBy
	input := &domain.CreateCapabilityRequestInput{
		AgentID:        agentID,
		CapabilityType: req.CapabilityType,
		Reason:         req.Reason,
		RequestedBy:    agent.CreatedBy, // Using the agent's owner as the requester
	}

	// Create the request
	request, err := h.service.CreateRequest(c.Context(), input)
	if err != nil {
		// Check for specific error types
		errMsg := err.Error()
		if errMsg == "agent not found" {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "agent not found",
			})
		}
		// Check if capability already granted or pending request exists
		if len(errMsg) > 10 {
			if errMsg[:10] == "capability" || errMsg[:7] == "pending" {
				return c.Status(fiber.StatusConflict).JSON(fiber.Map{
					"error": errMsg,
				})
			}
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "failed to create capability request",
			"details": errMsg,
		})
	}

	return c.Status(fiber.StatusCreated).JSON(request)
}

// ListCapabilityRequests godoc
// @Summary List capability requests (Admin only)
// @Description Get all capability requests with optional filtering
// @Tags capability-requests
// @Accept json
// @Produce json
// @Security Bearer
// @Param status query string false "Filter by status (pending, approved, rejected)"
// @Param agent_id query string false "Filter by agent ID"
// @Param limit query int false "Limit results"
// @Param offset query int false "Offset for pagination"
// @Success 200 {array} domain.CapabilityRequestWithDetails
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Router /api/v1/admin/capability-requests [get]
func (h *CapabilityRequestHandlers) ListCapabilityRequests(c fiber.Ctx) error {
	// Get organization ID from context (for multi-tenancy)
	orgID, ok := c.Locals("organization_id").(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "unauthorized - organization context not found",
		})
	}

	// Build filter from query params
	filter := domain.CapabilityRequestFilter{
		OrganizationID: &orgID, // Filter by organization for data isolation
	}

	if statusStr := c.Query("status"); statusStr != "" {
		status := domain.CapabilityRequestStatus(statusStr)
		filter.Status = &status
	}

	if agentIDStr := c.Query("agent_id"); agentIDStr != "" {
		agentID, err := uuid.Parse(agentIDStr)
		if err == nil {
			filter.AgentID = &agentID
		}
	}

	// Parse limit and offset
	filter.Limit = 100 // default
	filter.Offset = 0  // default
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil {
			filter.Limit = parsedLimit
		}
	}
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if parsedOffset, err := strconv.Atoi(offsetStr); err == nil {
			filter.Offset = parsedOffset
		}
	}

	requests, err := h.service.ListRequests(c.Context(), filter)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to list capability requests",
		})
	}

	// Return empty array instead of null for better API consistency
	if requests == nil {
		requests = []*domain.CapabilityRequestWithDetails{}
	}

	return c.Status(fiber.StatusOK).JSON(requests)
}

// GetCapabilityRequest godoc
// @Summary Get a capability request by ID
// @Description Get detailed information about a specific capability request
// @Tags capability-requests
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path string true "Capability Request ID"
// @Success 200 {object} domain.CapabilityRequestWithDetails
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/admin/capability-requests/{id} [get]
func (h *CapabilityRequestHandlers) GetCapabilityRequest(c fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid capability request ID",
		})
	}

	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}

	// SECURITY (defect #22): CapabilityRequest has no OrganizationID field;
	// ownership is determined by the request's AgentID -> Agent.OrganizationID.
	// Without this check, an admin in org A could read pending capability
	// requests for agents in org B by guessing request UUIDs.
	loader := func(reqID uuid.UUID) (*domain.CapabilityRequestWithDetails, error) {
		return h.service.GetRequest(c.Context(), reqID)
	}
	request := LoadOwnedViaAgent(c, loader, id, orgID,
		func(r *domain.CapabilityRequestWithDetails) uuid.UUID { return r.AgentID },
		h.agentRepo)
	if request == nil {
		return nil
	}

	return c.Status(fiber.StatusOK).JSON(request)
}

// ApproveCapabilityRequest godoc
// @Summary Approve a capability request (Admin only)
// @Description Approve a pending capability request and grant the capability
// @Tags capability-requests
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path string true "Capability Request ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/admin/capability-requests/{id}/approve [post]
func (h *CapabilityRequestHandlers) ApproveCapabilityRequest(c fiber.Ctx) error {
	// Get user ID from context (admin user)
	userID, ok := c.Locals("user_id").(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "unauthorized",
		})
	}

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid capability request ID",
		})
	}

	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}

	// SECURITY (defect #22, the worst finding in the audit): cross-tenant
	// privilege escalation. Before this check, an admin in org A could
	// approve a pending capability request for an agent in org B and thereby
	// grant arbitrary capabilities to a stranger's agent. Verify ownership
	// BEFORE invoking ApproveRequest so the grant cannot happen.
	loader := func(reqID uuid.UUID) (*domain.CapabilityRequestWithDetails, error) {
		return h.service.GetRequest(c.Context(), reqID)
	}
	if LoadOwnedViaAgent(c, loader, id, orgID,
		func(r *domain.CapabilityRequestWithDetails) uuid.UUID { return r.AgentID },
		h.agentRepo) == nil {
		return nil
	}

	if err := h.service.ApproveRequest(c.Context(), id, userID); err != nil {
		if err.Error() == "capability request not found" {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "capability request not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "capability request approved and capability granted",
	})
}

// RejectCapabilityRequest godoc
// @Summary Reject a capability request (Admin only)
// @Description Reject a pending capability request
// @Tags capability-requests
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path string true "Capability Request ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/admin/capability-requests/{id}/reject [post]
func (h *CapabilityRequestHandlers) RejectCapabilityRequest(c fiber.Ctx) error {
	// Get user ID from context (admin user)
	userID, ok := c.Locals("user_id").(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "unauthorized",
		})
	}

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid capability request ID",
		})
	}

	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}

	// SECURITY (A3d-ix, mirror of defect #22 fix on ApproveCapabilityRequest
	// at line 270): admin in org A could otherwise reject a pending
	// capability request belonging to an agent in org B by guessing the
	// request UUID. CapabilityRequest has no OrganizationID field; ownership
	// is determined by the request's AgentID -> Agent.OrganizationID.
	loader := func(reqID uuid.UUID) (*domain.CapabilityRequestWithDetails, error) {
		return h.service.GetRequest(c.Context(), reqID)
	}
	if LoadOwnedViaAgent(c, loader, id, orgID,
		func(r *domain.CapabilityRequestWithDetails) uuid.UUID { return r.AgentID },
		h.agentRepo) == nil {
		return nil
	}

	if err := h.service.RejectRequest(c.Context(), id, userID); err != nil {
		if err.Error() == "capability request not found" {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "capability request not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "capability request rejected",
	})
}

// ListAgentCapabilityRequests godoc
// @Summary List capability requests for a specific agent
// @Description SDK endpoint to list all capability requests for an agent
// @Tags sdk-api
// @Produce json
// @Param id path string true "Agent ID"
// @Success 200 {array} domain.CapabilityRequestWithDetails
// @Router /api/v1/sdk-api/agents/{id}/capability-requests [get]
func (h *CapabilityRequestHandlers) ListAgentCapabilityRequests(c fiber.Ctx) error {
	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid agent ID",
		})
	}

	// SECURITY (A3d-ix): same shape as CreateCapabilityRequest — the
	// PQC SDK auth middleware authenticates via X-Agent-ID header,
	// but the handler trusts the URL path :id. Without LoadOwned, any
	// SDK client could read the pending capability-request queue for
	// any agent in any tenant by supplying the victim's UUID.
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}
	if LoadOwned(c, h.agentRepo.GetByID, agentID, orgID, agentOrgID) == nil {
		return nil
	}

	filter := domain.CapabilityRequestFilter{
		AgentID: &agentID,
		Limit:   100,
		Offset:  0,
	}

	requests, err := h.service.ListRequests(c.Context(), filter)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to list capability requests",
		})
	}

	if requests == nil {
		requests = []*domain.CapabilityRequestWithDetails{}
	}

	return c.JSON(requests)
}
