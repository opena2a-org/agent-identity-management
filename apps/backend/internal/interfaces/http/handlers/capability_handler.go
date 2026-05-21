package handlers

import (
	"context"
	"encoding/base64"
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// CapabilityHandler handles capability-related HTTP requests
// agentByIDLookup is the single-method subset of any agent repository this
// handler needs. Both domain.AgentRepository (the production type) and the
// test-side MockAgentRepositoryerImpl satisfy it via Go structural typing,
// which lets the handler accept either without having to define a shared
// interface upstream that both sides agree on.
type agentByIDLookup interface {
	GetByID(id uuid.UUID) (*domain.Agent, error)
}

type CapabilityHandler struct {
	// Concrete service pointers (used by existing code)
	capabilityService        *application.CapabilityService
	capabilityRequestService *application.CapabilityRequestService
	agentRepo                agentByIDLookup
	orgRepo                  domain.OrganizationRepository

	// Interface fields for testability (used when set)
	capabilityServicer CapabilityServicer
}

// NewCapabilityHandler creates a new capability handler
func NewCapabilityHandler(
	capabilityService *application.CapabilityService,
	capabilityRequestService *application.CapabilityRequestService,
	agentRepo domain.AgentRepository,
	orgRepo domain.OrganizationRepository,
) *CapabilityHandler {
	return &CapabilityHandler{
		capabilityService:        capabilityService,
		capabilityRequestService: capabilityRequestService,
		agentRepo:                agentRepo,
		orgRepo:                  orgRepo,
	}
}

// NewCapabilityHandlerWithInterfaces creates a CapabilityHandler using
// interfaces for testability. Tests that exercise paths past either of
// the tenant-scoping checks (defect #25 in GrantCapability, defect #44
// in RevokeCapability) must overwrite handler.agentRepo with a
// scenario-specific mock that returns an agent whose OrganizationID
// matches the caller's org in the test's c.Locals. Without that, the
// nil agentRepo will panic on first GetByID dispatch.
func NewCapabilityHandlerWithInterfaces(
	capabilityService CapabilityServicer,
) *CapabilityHandler {
	return &CapabilityHandler{
		capabilityServicer: capabilityService,
	}
}

// Helper method to use interface when available, otherwise use concrete type
func (h *CapabilityHandler) getCapabilityService() CapabilityServicer {
	if h.capabilityServicer != nil {
		return h.capabilityServicer
	}
	return h.capabilityService
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
// GrantCapability grants a capability to an agent
// SECURITY: Debug logging removed to prevent information leakage
func (h *CapabilityHandler) GrantCapability(c fiber.Ctx) error {
	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error: "Invalid agent ID",
		})
	}

	var req GrantCapabilityRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error: "Invalid request body",
		})
	}

	// Get user ID from JWT claims
	userID, err := h.getUserIDFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(ErrorResponse{
			Error: "Unauthorized",
		})
	}

	// Get organization ID from context for capability validation
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

	// SECURITY (defect #25): verify the agent referenced by URL parameter
	// belongs to the caller's organization. Without this check, a valid SDK
	// token for org A could grant capabilities to agent B in org C.
	if LoadOwned(c, h.agentRepo.GetByID, agentID, orgID, agentOrgID) == nil {
		return nil
	}

	capSvc := h.getCapabilityService()

	// Validate capability format and auto-register custom capabilities
	if err := capSvc.ValidateAndRegisterCapability(context.Background(), req.CapabilityType, orgID); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error: err.Error(),
		})
	}

	// For SDK/API key authentication, userID will be uuid.Nil
	// Pass nil pointer instead of pointer to uuid.Nil to allow NULL in database
	var userIDPtr *uuid.UUID
	if userID != uuid.Nil {
		userIDPtr = &userID
	}

	capability, err := capSvc.GrantCapability(
		context.Background(),
		agentID,
		req.CapabilityType,
		req.Scope,
		userIDPtr,
		req.ExecutionMode,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
			Error: err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(capability)
}

// RegisterCapabilityRequest is the request body for registering a capability
type RegisterCapabilityRequest struct {
	CapabilityType string `json:"capabilityType" validate:"required"`
	Description    string `json:"description,omitempty"`
	RiskLevel      string `json:"riskLevel,omitempty"`
}

// RegisterCapabilityResponse is the response for capability registration
type RegisterCapabilityResponse struct {
	Success        bool   `json:"success"`
	CapabilityType string `json:"capabilityType"`
	Status         string `json:"status"` // "granted", "pending", "already_exists"
	Message        string `json:"message"`
	RequestID      string `json:"requestId,omitempty"` // Only set if status is "pending"
}

// RegisterCapability godoc
// @Summary Register a capability for an agent (SDK endpoint)
// @Description Register a capability that an agent intends to use. In MONITORING mode, capabilities are auto-granted. In STRICT mode, a pending request is created requiring admin approval.
// @Tags sdk-api
// @Accept json
// @Produce json
// @Param id path string true "Agent ID"
// @Param request body RegisterCapabilityRequest true "Capability registration request"
// @Success 200 {object} RegisterCapabilityResponse
// @Success 201 {object} RegisterCapabilityResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} RegisterCapabilityResponse "Capability already exists"
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/sdk-api/agents/{id}/capabilities/register [post]
func (h *CapabilityHandler) RegisterCapability(c fiber.Ctx) error {
	// Parse agent ID from path
	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error: "Invalid agent ID",
		})
	}

	// Parse request body
	var req RegisterCapabilityRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error: "Invalid request body",
		})
	}

	if req.CapabilityType == "" {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error: "capabilityType is required",
		})
	}

	// Get organization ID from context. The route is mounted only on
	// the sdkAPI group (cmd/server/main.go:322) which sits behind
	// PQCAgentMiddleware — the middleware sets organization_id from the
	// signed agent's database record, not from any caller-controllable
	// input.
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

	// SECURITY (defect #48): verify the agent referenced by the URL
	// parameter belongs to the caller's organization. Without this check,
	// a valid SDK-authenticated agent in org A could register capabilities
	// on agent B in org B by substituting agent B's UUID in the path; in
	// MONITORING enforcement mode that registration auto-grants the
	// capability. Same threat class as defect #25, fixed for the sibling
	// GrantCapability handler in PR #136 via the same LoadOwned pattern.
	agent := LoadOwned(c, h.agentRepo.GetByID, agentID, orgID, agentOrgID)
	if agent == nil {
		return nil // helper already wrote the 404 response
	}

	// Resolve the capability service via the helper (sibling handlers in
	// this file use the same pattern; lets tests inject a mock through
	// NewCapabilityHandlerWithInterfaces).
	capSvc := h.getCapabilityService()

	// Check if agent already has this capability
	existingCaps, err := capSvc.GetAgentCapabilities(context.Background(), agentID, true)
	if err == nil {
		for _, cap := range existingCaps {
			if cap.CapabilityType == req.CapabilityType {
				return c.Status(fiber.StatusConflict).JSON(RegisterCapabilityResponse{
					Success:        true,
					CapabilityType: req.CapabilityType,
					Status:         "already_exists",
					Message:        "Capability already granted to this agent",
				})
			}
		}
	}

	// Get organization to check enforcement mode
	org, err := h.orgRepo.GetByID(agent.OrganizationID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
			Error: "Failed to get organization settings",
		})
	}

	// MONITORING mode: Auto-grant the capability
	if org.EnforcementMode == domain.EnforcementModeMonitoring {
		// Validate and auto-register the capability type if custom
		if err := capSvc.ValidateAndRegisterCapability(context.Background(), req.CapabilityType, agent.OrganizationID); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
				Error: err.Error(),
			})
		}

		// Auto-grant the capability
		capability, err := capSvc.GrantCapability(
			context.Background(),
			agentID,
			req.CapabilityType,
			nil, // No scope
			nil, // System auto-grant
			"",  // Default execution mode based on risk level
		)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
				Error: err.Error(),
			})
		}

		return c.Status(fiber.StatusCreated).JSON(RegisterCapabilityResponse{
			Success:        true,
			CapabilityType: capability.CapabilityType,
			Status:         "granted",
			Message:        "Capability auto-granted (monitoring mode)",
		})
	}

	// STRICT mode: Create a pending capability request
	// Create the capability request for admin approval
	input := &domain.CreateCapabilityRequestInput{
		AgentID:        agentID,
		CapabilityType: req.CapabilityType,
		Reason:         req.Description,
		RequestedBy:    agent.CreatedBy, // Use agent owner as requester
	}

	if input.Reason == "" {
		input.Reason = "Capability registered via SDK"
	}

	request, err := h.capabilityRequestService.CreateRequest(c.Context(), input)
	if err != nil {
		errMsg := err.Error()
		// Check if pending request already exists
		if len(errMsg) > 7 && errMsg[:7] == "pending" {
			return c.Status(fiber.StatusConflict).JSON(RegisterCapabilityResponse{
				Success:        true,
				CapabilityType: req.CapabilityType,
				Status:         "pending",
				Message:        "A pending request for this capability already exists",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
			Error: err.Error(),
		})
	}

	return c.Status(fiber.StatusAccepted).JSON(RegisterCapabilityResponse{
		Success:        true,
		CapabilityType: req.CapabilityType,
		Status:         "pending",
		Message:        "Capability request created - awaiting admin approval (strict mode)",
		RequestID:      request.ID.String(),
	})
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

	capSvc := h.getCapabilityService()
	capabilities, err := capSvc.GetAgentCapabilities(
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
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}

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

	// SECURITY (A3c #44): tenant-scope the capability before revocation.
	// CapabilityService.RevokeCapability does NOT enforce org scoping
	// internally — without this check, a caller in org A could revoke a
	// capability in org B by passing the foreign capability UUID. Load
	// the capability, look up its owning agent, verify the agent's org
	// matches the caller's org. Returns 404 (not 403) for both "not
	// found" and "exists in another org" to avoid the cross-tenant
	// existence side channel; see tenant_scope.go:41-46.
	capSvc := h.getCapabilityService()
	capability, err := capSvc.GetCapabilityByID(c.Context(), capabilityID)
	if err != nil || capability == nil {
		respondResourceNotFound(c)
		return nil
	}
	agent, err := h.agentRepo.GetByID(capability.AgentID)
	if err != nil || agent == nil {
		respondResourceNotFound(c)
		return nil
	}
	if agent.OrganizationID != orgID {
		respondResourceNotFound(c)
		return nil
	}

	if err := capSvc.RevokeCapability(
		c.Context(),
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

	capSvc := h.getCapabilityService()
	result, err := capSvc.VerifyAction(
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

	capSvc := h.getCapabilityService()
	violations, total, err := capSvc.GetViolationsByAgent(
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

// ListCapabilities godoc
// @Summary List all available capabilities
// @Description Get all capability types available in the system with metadata
// @Tags capabilities
// @Produce json
// @Success 200 {object} application.ListCapabilitiesResponse
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

	// Call capability service to list all capabilities with metadata
	capSvc := h.getCapabilityService()
	response, err := capSvc.ListCapabilitiesWithMetadata(context.Background(), orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
			Error: err.Error(),
		})
	}

	return c.JSON(response)
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
	ExecutionMode  string                 `json:"executionMode,omitempty"` // auto, notify, review
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
