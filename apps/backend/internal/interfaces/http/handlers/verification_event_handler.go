package handlers

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/application"
	"github.com/opena2a/identity/backend/internal/domain"
)

// VerificationEventHandler handles verification event HTTP requests
type VerificationEventHandler struct {
	service *application.VerificationEventService
}

// NewVerificationEventHandler creates a new verification event handler
func NewVerificationEventHandler(service *application.VerificationEventService) *VerificationEventHandler {
	return &VerificationEventHandler{service: service}
}

// getOrganizationID extracts organization ID from fiber context
func getOrganizationID(c fiber.Ctx) (uuid.UUID, error) {
	orgID, ok := c.Locals("organization_id").(uuid.UUID)
	if !ok {
		return uuid.Nil, fiber.NewError(fiber.StatusUnauthorized, "organization ID not found in context")
	}
	return orgID, nil
}

// RegisterRoutes registers verification event routes
func (h *VerificationEventHandler) RegisterRoutes(app *fiber.App, authMiddleware fiber.Handler) {
	api := app.Group("/api/v1/verification-events")
	api.Use(authMiddleware)

	api.Get("/", h.ListVerificationEvents)
	api.Get("/recent", h.GetRecentEvents)
	api.Get("/statistics", h.GetStatistics)
	api.Get("/:id", h.GetVerificationEvent)
	api.Post("/", h.CreateVerificationEvent)
	api.Delete("/:id", h.DeleteVerificationEvent)
}

// ListVerificationEvents retrieves verification events for the authenticated user's organization
// @Summary List verification events
// @Description Get paginated list of verification events for the organization
// @Tags verification-events
// @Accept json
// @Produce json
// @Param limit query int false "Number of events to return" default(50)
// @Param offset query int false "Number of events to skip" default(0)
// @Param agent_id query string false "Filter by agent ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/verification-events [get]
func (h *VerificationEventHandler) ListVerificationEvents(c fiber.Ctx) error {
	// Get organization ID from auth context
	orgID, err := getOrganizationID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized",
		})
	}

	// Parse query parameters
	limit, err := strconv.Atoi(c.Query("limit", "50"))
	if err != nil || limit < 1 || limit > 100 {
		limit = 50
	}

	offset, err := strconv.Atoi(c.Query("offset", "0"))
	if err != nil || offset < 0 {
		offset = 0
	}

	agentIDStr := c.Query("agent_id")

	var events []*domain.VerificationEvent
	var total int

	// Filter by agent if specified
	if agentIDStr != "" {
		agentID, err := uuid.Parse(agentIDStr)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid agent ID format",
			})
		}
		events, total, err = h.service.ListAgentVerificationEvents(c.Context(), agentID, limit, offset)
	} else {
		events, total, err = h.service.ListVerificationEvents(c.Context(), orgID, limit, offset)
	}

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve verification events",
		})
	}

	return c.JSON(fiber.Map{
		"events": events,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// GetVerificationEvent retrieves a specific verification event by ID
// @Summary Get verification event
// @Description Get details of a specific verification event
// @Tags verification-events
// @Accept json
// @Produce json
// @Param id path string true "Verification Event ID"
// @Success 200 {object} domain.VerificationEvent
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/verification-events/{id} [get]
func (h *VerificationEventHandler) GetVerificationEvent(c fiber.Ctx) error {
	// Get organization ID from auth context
	_, err := getOrganizationID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized",
		})
	}

	// Parse event ID
	eventID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid event ID format",
		})
	}

	// Get event
	event, err := h.service.GetVerificationEvent(c.Context(), eventID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Verification event not found",
		})
	}

	return c.JSON(event)
}

// CreateVerificationEventRequest represents the request body for creating a verification event
type CreateVerificationEventRequest struct {
	AgentID          string                           `json:"agentId" validate:"required"`
	Protocol         domain.VerificationProtocol      `json:"protocol" validate:"required"`
	VerificationType domain.VerificationType          `json:"verificationType" validate:"required"`
	Status           domain.VerificationEventStatus   `json:"status" validate:"required"`
	Result           *domain.VerificationResult       `json:"result,omitempty"`
	Signature        *string                          `json:"signature,omitempty"`
	MessageHash      *string                          `json:"messageHash,omitempty"`
	Nonce            *string                          `json:"nonce,omitempty"`
	PublicKey        *string                          `json:"publicKey,omitempty"`
	Confidence       float64                          `json:"confidence"`
	DurationMs       int                              `json:"durationMs"`
	ErrorCode        *string                          `json:"errorCode,omitempty"`
	ErrorReason      *string                          `json:"errorReason,omitempty"`
	InitiatorType    domain.InitiatorType             `json:"initiatorType" validate:"required"`
	InitiatorID      *string                          `json:"initiatorId,omitempty"`
	InitiatorName    *string                          `json:"initiatorName,omitempty"`
	InitiatorIP      *string                          `json:"initiatorIp,omitempty"`
	Action           *string                          `json:"action,omitempty"`
	ResourceType     *string                          `json:"resourceType,omitempty"`
	ResourceID       *string                          `json:"resourceId,omitempty"`
	Location         *string                          `json:"location,omitempty"`
	StartedAt        time.Time                        `json:"startedAt"`
	CompletedAt      *time.Time                       `json:"completedAt,omitempty"`
	Details          *string                          `json:"details,omitempty"`
	Metadata         map[string]interface{}           `json:"metadata,omitempty"`

	// Configuration Drift Detection (WHO and WHAT)
	CurrentMCPServers   []string `json:"currentMcpServers,omitempty"`   // Runtime: MCP servers being communicated with
	CurrentCapabilities []string `json:"currentCapabilities,omitempty"` // Runtime: Capabilities being used
}

// CreateVerificationEvent creates a new verification event
// @Summary Create verification event
// @Description Create a new verification event (manual logging)
// @Tags verification-events
// @Accept json
// @Produce json
// @Param event body CreateVerificationEventRequest true "Verification Event"
// @Success 201 {object} domain.VerificationEvent
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/verification-events [post]
func (h *VerificationEventHandler) CreateVerificationEvent(c fiber.Ctx) error {
	// Get organization ID from auth context
	orgID, err := getOrganizationID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized",
		})
	}

	// Parse request body
	var req CreateVerificationEventRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Parse agent ID
	agentID, err := uuid.Parse(req.AgentID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID format",
		})
	}

	// Parse optional initiator ID
	var initiatorID *uuid.UUID
	if req.InitiatorID != nil {
		id, err := uuid.Parse(*req.InitiatorID)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid initiator ID format",
			})
		}
		initiatorID = &id
	}

	// Create service request
	serviceReq := &application.CreateVerificationEventRequest{
		OrganizationID:   orgID,
		AgentID:          agentID,
		Protocol:         req.Protocol,
		VerificationType: req.VerificationType,
		Status:           req.Status,
		Result:           req.Result,
		Signature:        req.Signature,
		MessageHash:      req.MessageHash,
		Nonce:            req.Nonce,
		PublicKey:        req.PublicKey,
		DurationMs:       req.DurationMs,
		ErrorCode:        req.ErrorCode,
		ErrorReason:      req.ErrorReason,
		InitiatorType:    req.InitiatorType,
		InitiatorID:      initiatorID,
		InitiatorName:    req.InitiatorName,
		InitiatorIP:      req.InitiatorIP,
		Action:           req.Action,
		ResourceType:     req.ResourceType,
		ResourceID:       req.ResourceID,
		Location:         req.Location,
		StartedAt:        req.StartedAt,
		CompletedAt:      req.CompletedAt,
		Details:          req.Details,
		Metadata:         req.Metadata,

		// Configuration Drift Detection
		CurrentMCPServers:   req.CurrentMCPServers,
		CurrentCapabilities: req.CurrentCapabilities,
	}

	// Create event
	event, err := h.service.CreateVerificationEvent(c.Context(), serviceReq)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create verification event",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(event)
}

// GetRecentEvents retrieves recent verification events for real-time monitoring
// @Summary Get recent verification events
// @Description Get verification events from the last N minutes for real-time monitoring
// @Tags verification-events
// @Accept json
// @Produce json
// @Param minutes query int false "Number of minutes to look back" default(15)
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/verification-events/recent [get]
func (h *VerificationEventHandler) GetRecentEvents(c fiber.Ctx) error {
	// Get organization ID from auth context
	orgID, err := getOrganizationID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized",
		})
	}

	// Parse minutes parameter (allow up to 7 days = 10080 minutes)
	minutes, err := strconv.Atoi(c.Query("minutes", "15"))
	if err != nil || minutes < 1 || minutes > 10080 {
		minutes = 15 // Default to 15 minutes
	}

	// Get recent events
	events, err := h.service.GetRecentEvents(c.Context(), orgID, minutes)
	if err != nil {
		// Log the actual error for debugging
		println("ERROR in GetRecentEvents:", err.Error())
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve recent events: " + err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"events":  events,
		"minutes": minutes,
		"count":   len(events),
	})
}

// GetStatistics retrieves aggregated verification statistics
// @Summary Get verification statistics
// @Description Get aggregated statistics for verification events in a time range
// @Tags verification-events
// @Accept json
// @Produce json
// @Param period query string false "Time period (24h, 7d, 30d, custom)" default(24h)
// @Param start_time query string false "Start time for custom period (RFC3339)"
// @Param end_time query string false "End time for custom period (RFC3339)"
// @Success 200 {object} domain.VerificationStatistics
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/verification-events/statistics [get]
func (h *VerificationEventHandler) GetStatistics(c fiber.Ctx) error {
	// Get organization ID from auth context
	orgID, err := getOrganizationID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized",
		})
	}

	// Parse time range
	period := c.Query("period", "24h")
	var startTime, endTime time.Time

	switch period {
	case "24h":
		endTime = time.Now()
		startTime = endTime.Add(-24 * time.Hour)
	case "7d":
		endTime = time.Now()
		startTime = endTime.Add(-7 * 24 * time.Hour)
	case "30d":
		endTime = time.Now()
		startTime = endTime.Add(-30 * 24 * time.Hour)
	case "custom":
		startTimeStr := c.Query("start_time")
		endTimeStr := c.Query("end_time")

		if startTimeStr == "" || endTimeStr == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "start_time and end_time required for custom period",
			})
		}

		startTime, err = time.Parse(time.RFC3339, startTimeStr)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid start_time format (use RFC3339)",
			})
		}

		endTime, err = time.Parse(time.RFC3339, endTimeStr)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid end_time format (use RFC3339)",
			})
		}
	default:
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid period (use 24h, 7d, 30d, or custom)",
		})
	}

	// Get statistics
	stats, err := h.service.GetStatistics(c.Context(), orgID, startTime, endTime)
	if err != nil {
		// Log the actual error for debugging
		println("ERROR in GetStatistics:", err.Error())
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve statistics: " + err.Error(),
		})
	}

	return c.JSON(stats)
}

// GetAgentVerificationEvents retrieves verification events for a specific agent
// @Summary Get agent verification events
// @Description Get all verification events for a specific agent with pagination
// @Tags verification-events
// @Accept json
// @Produce json
// @Param id path string true "Agent ID"
// @Param limit query int false "Number of events to return" default(50)
// @Param offset query int false "Number of events to skip" default(0)
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/verification-events/agent/{id} [get]
func (h *VerificationEventHandler) GetAgentVerificationEvents(c fiber.Ctx) error {
	// Get organization ID from auth context
	_, err := getOrganizationID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized",
		})
	}

	// Parse agent ID
	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID format",
		})
	}

	// Parse pagination parameters
	limit, err := strconv.Atoi(c.Query("limit", "50"))
	if err != nil || limit < 1 || limit > 100 {
		limit = 50
	}

	offset, err := strconv.Atoi(c.Query("offset", "0"))
	if err != nil || offset < 0 {
		offset = 0
	}

	// Get verification events for this agent
	events, total, err := h.service.ListAgentVerificationEvents(c.Context(), agentID, limit, offset)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve verification events",
		})
	}

	return c.JSON(fiber.Map{
		"events": events,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// GetMCPVerificationEvents retrieves verification events for a specific MCP server
// @Summary Get MCP server verification events
// @Description Get all verification events for a specific MCP server with pagination
// @Tags verification-events
// @Accept json
// @Produce json
// @Param id path string true "MCP Server ID"
// @Param limit query int false "Number of events to return" default(50)
// @Param offset query int false "Number of events to skip" default(0)
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/verification-events/mcp/{id} [get]
func (h *VerificationEventHandler) GetMCPVerificationEvents(c fiber.Ctx) error {
	// Get organization ID from auth context
	_, err := getOrganizationID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized",
		})
	}

	// Parse MCP server ID
	mcpServerID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid MCP server ID format",
		})
	}

	// Parse pagination parameters
	limit, err := strconv.Atoi(c.Query("limit", "50"))
	if err != nil || limit < 1 || limit > 100 {
		limit = 50
	}

	offset, err := strconv.Atoi(c.Query("offset", "0"))
	if err != nil || offset < 0 {
		offset = 0
	}

	// Get verification events for this MCP server
	// We need to add this method to the service layer
	events, total, err := h.service.ListMCPVerificationEvents(c.Context(), mcpServerID, limit, offset)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve verification events",
		})
	}

	return c.JSON(fiber.Map{
		"events": events,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// GetVerificationStats retrieves aggregated verification statistics
// @Summary Get verification statistics
// @Description Get overall verification statistics including success rates and type distribution
// @Tags verification-events
// @Accept json
// @Produce json
// @Param period query string false "Time period (24h, 7d, 30d)" default(24h)
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/verification-events/stats [get]
func (h *VerificationEventHandler) GetVerificationStats(c fiber.Ctx) error {
	// Get organization ID from auth context
	orgID, err := getOrganizationID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized",
		})
	}

	// Parse time period
	period := c.Query("period", "24h")
	var startTime, endTime time.Time
	endTime = time.Now()

	switch period {
	case "24h":
		startTime = endTime.Add(-24 * time.Hour)
	case "7d":
		startTime = endTime.Add(-7 * 24 * time.Hour)
	case "30d":
		startTime = endTime.Add(-30 * 24 * time.Hour)
	default:
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid period (use 24h, 7d, or 30d)",
		})
	}

	// Get statistics from service
	stats, err := h.service.GetStatistics(c.Context(), orgID, startTime, endTime)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve verification statistics",
		})
	}

	return c.JSON(stats)
}

// DeleteVerificationEvent deletes a verification event
// @Summary Delete verification event
// @Description Delete a verification event (admin only)
// @Tags verification-events
// @Accept json
// @Produce json
// @Param id path string true "Verification Event ID"
// @Success 204
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/verification-events/{id} [delete]
func (h *VerificationEventHandler) DeleteVerificationEvent(c fiber.Ctx) error {
	// Get organization ID from auth context
	_, err := getOrganizationID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized",
		})
	}

	// Parse event ID
	eventID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid event ID format",
		})
	}

	// Delete event
	if err := h.service.DeleteVerificationEvent(c.Context(), eventID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to delete verification event",
		})
	}

	return c.SendStatus(fiber.StatusNoContent)
}
