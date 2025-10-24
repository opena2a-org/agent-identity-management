package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/application"
	"github.com/opena2a/identity/backend/internal/domain"
)

type CapabilityRequestHandlers struct {
	service *application.CapabilityRequestService
}

func NewCapabilityRequestHandlers(service *application.CapabilityRequestService) *CapabilityRequestHandlers {
	return &CapabilityRequestHandlers{
		service: service,
	}
}

// CreateCapabilityRequest godoc
// @Summary Create a new capability request (SDK)
// @Description Agents can request additional capabilities after registration via SDK
// @Tags capability-requests
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param id path string true "Agent ID"
// @Param request body domain.CreateCapabilityRequestInput true "Capability request details"
// @Success 201 {object} domain.CapabilityRequest
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
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

	// Get user ID from API key context (the user who owns the API key)
	userID, ok := c.Locals("user_id").(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "unauthorized - user ID not found",
		})
	}

	// Parse request body
	type RequestBody struct {
		CapabilityType string `json:"capability_type" validate:"required"`
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
	input := &domain.CreateCapabilityRequestInput{
		AgentID:        agentID,
		CapabilityType: req.CapabilityType,
		Reason:         req.Reason,
		RequestedBy:    userID,
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
			"error": "failed to create capability request",
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

	request, err := h.service.GetRequest(c.Context(), id)
	if err != nil {
		if err.Error() == "capability request not found" {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "capability request not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to get capability request",
		})
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
