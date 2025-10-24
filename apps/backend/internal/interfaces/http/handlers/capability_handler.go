package handlers

import (
	"context"
	"encoding/base64"
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/application"
	"github.com/opena2a/identity/backend/internal/domain"
)

// CapabilityHandler handles capability-related HTTP requests
type CapabilityHandler struct {
	capabilityService *application.CapabilityService
}

// NewCapabilityHandler creates a new capability handler
func NewCapabilityHandler(capabilityService *application.CapabilityService) *CapabilityHandler {
	return &CapabilityHandler{
		capabilityService: capabilityService,
	}
}

// GrantCapability godoc
// @Summary Grant a capability to an agent
// @Description Add a new capability to an agent's registered capabilities
// @Tags capabilities
// @Accept json
// @Produce json
// @Param id path string true "Agent ID"
// @Param capability body GrantCapabilityRequest true "Capability to grant"
// @Success 201 {object} domain.AgentCapability
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /agents/{id}/capabilities [post]
func (h *CapabilityHandler) GrantCapability(c fiber.Ctx) error {
	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		println("ERROR: Invalid agent ID:", err.Error())
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error: "Invalid agent ID",
		})
	}

	var req GrantCapabilityRequest
	if err := c.Bind().JSON(&req); err != nil {
		println("ERROR: Failed to bind request body:", err.Error())
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error: "Invalid request body",
		})
	}

	println("DEBUG: GrantCapability - AgentID:", agentID.String(), "CapabilityType:", req.CapabilityType)

	// Get user ID from JWT claims
	userID, err := h.getUserIDFromContext(c)
	if err != nil {
		println("ERROR: Failed to get user ID:", err.Error())
		return c.Status(fiber.StatusUnauthorized).JSON(ErrorResponse{
			Error: "Unauthorized",
		})
	}

	println("DEBUG: UserID:", userID.String())

	// For SDK/API key authentication, userID will be uuid.Nil
	// Pass nil pointer instead of pointer to uuid.Nil to allow NULL in database
	var userIDPtr *uuid.UUID
	if userID != uuid.Nil {
		userIDPtr = &userID
	}

	capability, err := h.capabilityService.GrantCapability(
		context.Background(),
		agentID,
		req.CapabilityType,
		req.Scope,
		userIDPtr,
	)
	if err != nil {
		println("ERROR: GrantCapability service failed:", err.Error())
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
			Error: err.Error(),
		})
	}

	println("DEBUG: Capability granted successfully:", capability.ID.String())
	return c.Status(fiber.StatusCreated).JSON(capability)
}

// GetAgentCapabilities godoc
// @Summary Get agent capabilities
// @Description Retrieve all capabilities for an agent
// @Tags capabilities
// @Produce json
// @Param id path string true "Agent ID"
// @Param activeOnly query boolean false "Only return active (non-revoked) capabilities"
// @Success 200 {array} domain.AgentCapability
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /agents/{id}/capabilities [get]
func (h *CapabilityHandler) GetAgentCapabilities(c fiber.Ctx) error {
	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error: "Invalid agent ID",
		})
	}

	activeOnly := c.Query("activeOnly", "true") == "true"

	capabilities, err := h.capabilityService.GetAgentCapabilities(
		context.Background(),
		agentID,
		activeOnly,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
			Error: err.Error(),
		})
	}

	return c.JSON(capabilities)
}

// RevokeCapability godoc
// @Summary Revoke a capability
// @Description Revoke a capability from an agent
// @Tags capabilities
// @Produce json
// @Param agentId path string true "Agent ID"
// @Param capabilityId path string true "Capability ID"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /agents/{agentId}/capabilities/{capabilityId} [delete]
func (h *CapabilityHandler) RevokeCapability(c fiber.Ctx) error {
	capabilityID, err := uuid.Parse(c.Params("capabilityId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error: "Invalid capability ID",
		})
	}

	// Get user ID from JWT claims
	userID, err := h.getUserIDFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(ErrorResponse{
			Error: "Unauthorized",
		})
	}

	if err := h.capabilityService.RevokeCapability(
		context.Background(),
		capabilityID,
		&userID,
	); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
			Error: err.Error(),
		})
	}

	return c.JSON(SuccessResponse{
		Message: "Capability revoked successfully",
	})
}

// VerifyAction godoc
// @Summary Verify an action
// @Description Verify if an agent is authorized to perform a specific action
// @Tags capabilities
// @Accept json
// @Produce json
// @Param request body VerifyActionRequest true "Action verification request"
// @Success 200 {object} application.VerificationResult
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /internal/verify-action [post]
func (h *CapabilityHandler) VerifyAction(c fiber.Ctx) error {
	var req VerifyActionRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error: "Invalid request body",
		})
	}

	agentID, err := uuid.Parse(req.AgentID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error: "Invalid agent ID",
		})
	}

	// Decode signature and payload from base64
	signature, err := base64.StdEncoding.DecodeString(req.Signature)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error: "Invalid signature encoding",
		})
	}

	payload, err := base64.StdEncoding.DecodeString(req.RequestPayload)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error: "Invalid payload encoding",
		})
	}

	// Get source IP
	sourceIP := c.IP()

	result, err := h.capabilityService.VerifyAction(
		context.Background(),
		agentID,
		req.RequestedCapability,
		signature,
		payload,
		&sourceIP,
		req.Metadata,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
			Error: err.Error(),
		})
	}

	return c.JSON(result)
}

// GetViolationsByAgent godoc
// @Summary Get violations for an agent
// @Description Retrieve capability violations for a specific agent
// @Tags capabilities
// @Produce json
// @Param id path string true "Agent ID"
// @Param limit query int false "Limit" default(20)
// @Param offset query int false "Offset" default(0)
// @Success 200 {object} ViolationsResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /agents/{id}/violations [get]
func (h *CapabilityHandler) GetViolationsByAgent(c fiber.Ctx) error {
	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error: "Invalid agent ID",
		})
	}

	limit := 20
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil {
			limit = parsedLimit
		}
	}

	offset := 0
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if parsedOffset, err := strconv.Atoi(offsetStr); err == nil {
			offset = parsedOffset
		}
	}

	violations, total, err := h.capabilityService.GetViolationsByAgent(
		context.Background(),
		agentID,
		limit,
		offset,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
			Error: err.Error(),
		})
	}

	return c.JSON(ViolationsResponse{
		Violations: violations,
		Total:      total,
		Limit:      limit,
		Offset:     offset,
	})
}

// GetViolationsByOrganization godoc
// @Summary Get violations for an organization
// @Description Retrieve all capability violations for an organization
// @Tags capabilities
// @Produce json
// @Param orgId path string true "Organization ID"
// @Param limit query int false "Limit" default(20)
// @Param offset query int false "Offset" default(0)
// @Success 200 {object} ViolationsResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /organizations/{orgId}/violations [get]
func (h *CapabilityHandler) GetViolationsByOrganization(c fiber.Ctx) error {
	orgID, err := uuid.Parse(c.Params("orgId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error: "Invalid organization ID",
		})
	}

	limit := 20
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil {
			limit = parsedLimit
		}
	}

	offset := 0
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if parsedOffset, err := strconv.Atoi(offsetStr); err == nil {
			offset = parsedOffset
		}
	}

	violations, total, err := h.capabilityService.GetViolationsByOrganization(
		context.Background(),
		orgID,
		limit,
		offset,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
			Error: err.Error(),
		})
	}

	return c.JSON(ViolationsResponse{
		Violations: violations,
		Total:      total,
		Limit:      limit,
		Offset:     offset,
	})
}

// ListCapabilities godoc
// @Summary List all available capabilities
// @Description Get all capability types available in the system
// @Tags capabilities
// @Produce json
// @Success 200 {array} application.CapabilityDefinition
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /capabilities [get]
func (h *CapabilityHandler) ListCapabilities(c fiber.Ctx) error {
	// Get organization ID from context
	orgIDValue := c.Locals("organization_id")
	if orgIDValue == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(ErrorResponse{
			Error: "Organization ID not found in context",
		})
	}

	orgID, ok := orgIDValue.(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
			Error: "Invalid organization ID type in context",
		})
	}

	// Call capability service to list all capabilities
	capabilities, err := h.capabilityService.ListCapabilities(context.Background(), orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
			Error: err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"capabilities": capabilities,
	})
}

// GetRecentViolations godoc
// @Summary Get recent violations
// @Description Retrieve violations from the last N minutes for an organization
// @Tags capabilities
// @Produce json
// @Param orgId path string true "Organization ID"
// @Param minutes query int false "Minutes to look back" default(60)
// @Success 200 {array} domain.CapabilityViolation
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /organizations/{orgId}/violations/recent [get]
func (h *CapabilityHandler) GetRecentViolations(c fiber.Ctx) error {
	orgID, err := uuid.Parse(c.Params("orgId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error: "Invalid organization ID",
		})
	}

	minutes := 60
	if minutesStr := c.Query("minutes"); minutesStr != "" {
		if parsedMinutes, err := strconv.Atoi(minutesStr); err == nil {
			minutes = parsedMinutes
		}
	}

	violations, err := h.capabilityService.GetRecentViolations(
		context.Background(),
		orgID,
		minutes,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
			Error: err.Error(),
		})
	}

	return c.JSON(violations)
}

// Helper function to extract user ID from JWT claims or use system user for API key auth
func (h *CapabilityHandler) getUserIDFromContext(c fiber.Ctx) (uuid.UUID, error) {
	// Check authentication method
	authMethod := c.Locals("auth_method")

	// If API key authentication, use the agent's agent_id as user_id
	// (API keys are associated with agents, not users directly)
	if authMethod != nil && authMethod.(string) == "api_key" {
		// For SDK API key auth, we can use a system user ID or the agent's user
		// For now, return a nil UUID to indicate system/SDK access
		return uuid.Nil, nil
	}

	// Extract user ID from JWT claims stored in locals
	userIDValue := c.Locals("user_id")
	if userIDValue == nil {
		return uuid.Nil, fiber.ErrUnauthorized
	}

	userID, ok := userIDValue.(uuid.UUID)
	if !ok {
		return uuid.Nil, fiber.ErrUnauthorized
	}

	return userID, nil
}

// Request/Response types
type GrantCapabilityRequest struct {
	CapabilityType string                 `json:"capabilityType" validate:"required"`
	Scope          map[string]interface{} `json:"scope,omitempty"`
}

type VerifyActionRequest struct {
	AgentID             string                 `json:"agentId" validate:"required"`
	Signature           string                 `json:"signature" validate:"required"`
	RequestPayload      string                 `json:"requestPayload" validate:"required"`
	RequestedCapability string                 `json:"requestedCapability" validate:"required"`
	Metadata            map[string]interface{} `json:"metadata,omitempty"`
}

type ViolationsResponse struct {
	Violations []*domain.CapabilityViolation `json:"violations"`
	Total      int                           `json:"total"`
	Limit      int                           `json:"limit"`
	Offset     int                           `json:"offset"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type SuccessResponse struct {
	Message string `json:"message"`
}
