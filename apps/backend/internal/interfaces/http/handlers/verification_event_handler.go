package handlers

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// VerificationEventHandler handles verification event HTTP requests.
//
// SECURITY (A3d-iv): the handler holds agent + MCP repository lookups
// alongside the verification-event service so cross-tenant access on
// the four path-id routes can be denied at handler layer before the
// service is invoked.
//
//   - GetVerificationEvent / DeleteVerificationEvent — load the event
//     and check event.OrganizationID == callerOrgID via LoadOwned +
//     verificationEventOrgID (the event row carries org directly).
//   - GetAgentVerificationEvents — verify agent.OrganizationID via
//     LoadOwned + agentOrgID before reading the agent's events.
//   - GetMCPVerificationEvents — verify mcpServer.OrganizationID via
//     LoadOwned + mcpServerOrgID before reading the MCP's events.
//
// Cross-tenant returns 404 with body {"error":"not found"} for
// existence secrecy. See tenant_scope.go:41-46.
type VerificationEventHandler struct {
	service       *application.VerificationEventService
	agentRepo     domain.AgentRepository
	mcpServerRepo domain.MCPServerRepository
}

// NewVerificationEventHandler creates a new verification event handler.
func NewVerificationEventHandler(
	service *application.VerificationEventService,
	agentRepo domain.AgentRepository,
	mcpServerRepo domain.MCPServerRepository,
) *VerificationEventHandler {
	return &VerificationEventHandler{
		service:       service,
		agentRepo:     agentRepo,
		mcpServerRepo: mcpServerRepo,
	}
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

	events := make([]*domain.VerificationEvent, 0)
	var total int

	// Filter by agent if specified
	if agentIDStr != "" {
		agentID, err := uuid.Parse(agentIDStr)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid agent ID format",
			})
		}

		// SECURITY (A3d-iv R1): verify the query-supplied agentID belongs
		// to the caller's org before reading the agent's verification
		// events. The lint cannot catch this — it reads c.Query("agent_id")
		// rather than c.Params, so the AST scan that fires for path-id
		// IDORs is structurally blind to this path. Returns 404 on
		// cross-tenant for existence secrecy (same as the path-id
		// GetAgentVerificationEvents fix in this PR).
		if LoadOwned(c, h.agentRepo.GetByID, agentID, orgID, agentOrgID) == nil {
			return nil
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
	orgID, err := getOrganizationID(c)
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

	// SECURITY (A3d-iv): verify the event belongs to the caller's org
	// before returning it. Returns 404 on cross-tenant or non-existent
	// IDs (existence secrecy).
	loader := func(id uuid.UUID) (*domain.VerificationEvent, error) {
		return h.service.GetVerificationEvent(c.Context(), id)
	}
	event := LoadOwned(c, loader, eventID, orgID, verificationEventOrgID)
	if event == nil {
		return nil
	}

	return c.JSON(event)
}

// CreateVerificationEvent refuses every caller.
//
// A verification event records the outcome of a verification (status, result,
// signature, public key, confidence), and trust scoring reads those rows. Until
// that outcome is derived by the server from a verification it performed, and
// the agent is resolved within the caller's organization, this endpoint accepts
// nothing from anyone.
//
// The call to the service is removed from this handler rather than guarded, so
// the refusal holds for every mount and every role without reasoning about the
// middleware in front of it. Nothing is read from the request and no agent is
// looked up, so the response is the same whether or not the agent exists.
// Verification events are still recorded by the server's own verification
// paths (VerificationHandler.CreateVerification and the agent service), which
// call VerificationEventService directly.
//
// 403 with a distinct machine-readable code, as SubmitVerificationResult does:
// a caller can tell the refusal apart from a missing route or a failed write.
//
// @Summary Create verification event (disabled)
// @Description Disabled. Returns 403 for every caller until event outcomes are derived by the server.
// @Tags verification-events
// @Produce json
// @Failure 403 {object} map[string]interface{} "Endpoint disabled"
// @Router /api/v1/verification-events [post]
func (h *VerificationEventHandler) CreateVerificationEvent(c fiber.Ctx) error {
	return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
		"error": "verification events are recorded by the server; this endpoint does not accept them",
		"code":  "verificationEventWriteNotAccepted",
	})
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
	// SECURITY: No error logging to prevent information leakage
	events, err := h.service.GetRecentEvents(c.Context(), orgID, minutes)
	if err != nil {
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
	// SECURITY: No error logging to prevent information leakage
	stats, err := h.service.GetStatistics(c.Context(), orgID, startTime, endTime)
	if err != nil {
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
	orgID, err := getOrganizationID(c)
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

	// SECURITY (A3d-iv): verify the path agentID belongs to the caller's
	// org before reading the agent's verification events. Cross-tenant
	// callers can no longer enumerate another org's agent activity
	// timeline. Returns 404 on cross-tenant (existence secrecy).
	if LoadOwned(c, h.agentRepo.GetByID, agentID, orgID, agentOrgID) == nil {
		return nil
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
	orgID, err := getOrganizationID(c)
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

	// SECURITY (A3d-iv): verify the path mcpServerID belongs to the
	// caller's org before reading the MCP server's verification events.
	// Returns 404 on cross-tenant (existence secrecy).
	if LoadOwned(c, h.mcpServerRepo.GetByID, mcpServerID, orgID, mcpServerOrgID) == nil {
		return nil
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
	orgID, err := getOrganizationID(c)
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

	// SECURITY (A3d-iv): verify the event belongs to the caller's org
	// before deletion. Cross-tenant deletion would let a caller in org A
	// delete an event row in org B; without this check the deletion
	// also leaks existence through 204 vs 500 (depending on which
	// errors the underlying repo returns). Returns 404 on cross-tenant.
	loader := func(id uuid.UUID) (*domain.VerificationEvent, error) {
		return h.service.GetVerificationEvent(c.Context(), id)
	}
	if LoadOwned(c, loader, eventID, orgID, verificationEventOrgID) == nil {
		return nil
	}

	// Delete event
	if err := h.service.DeleteVerificationEvent(c.Context(), eventID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to delete verification event",
		})
	}

	return c.SendStatus(fiber.StatusNoContent)
}
