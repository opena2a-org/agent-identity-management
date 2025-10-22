package handlers

import (
	"fmt"
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
// @Summary Create a new capability request
// @Description Agents can request additional capabilities after registration
// @Tags capability-requests
// @Accept json
// @Produce json
// @Security Bearer
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
	// Build filter from query params
	filter := domain.CapabilityRequestFilter{}

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
