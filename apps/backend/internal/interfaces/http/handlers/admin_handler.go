package handlers

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/application"
	"github.com/opena2a/identity/backend/internal/domain"
)

type AdminHandler struct {
	authService  *application.AuthService
	agentService *application.AgentService
	mcpService   *application.MCPService
	auditService *application.AuditService
	alertService *application.AlertService
}

func NewAdminHandler(
	authService *application.AuthService,
	agentService *application.AgentService,
	mcpService *application.MCPService,
	auditService *application.AuditService,
	alertService *application.AlertService,
) *AdminHandler {
	return &AdminHandler{
		authService:  authService,
		agentService: agentService,
		mcpService:   mcpService,
		auditService: auditService,
		alertService: alertService,
	}
}

// ListUsers returns all users in the organization
func (h *AdminHandler) ListUsers(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)

	users, err := h.authService.GetUsersByOrganization(c.Context(), orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch users",
		})
	}

	// Log audit
	h.auditService.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionView,
		"users",
		uuid.Nil,
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"total_users": len(users),
		},
	)

	return c.JSON(fiber.Map{
		"users": users,
		"total": len(users),
	})
}

// UpdateUserRole updates a user's role (admin only)
func (h *AdminHandler) UpdateUserRole(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	adminID := c.Locals("user_id").(uuid.UUID)
	targetUserID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid user ID",
		})
	}

	var req struct {
		Role string `json:"role"`
	}

	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validate role
	var role domain.UserRole
	switch req.Role {
	case "admin":
		role = domain.RoleAdmin
	case "manager":
		role = domain.RoleManager
	case "member":
		role = domain.RoleMember
	case "viewer":
		role = domain.RoleViewer
	default:
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid role. Must be: admin, manager, member, or viewer",
		})
	}

	// Update user role
	user, err := h.authService.UpdateUserRole(c.Context(), targetUserID, orgID, role, adminID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Log audit
	h.auditService.LogAction(
		c.Context(),
		orgID,
		adminID,
		domain.AuditActionUpdate,
		"user_role",
		targetUserID,
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"user_email": user.Email,
			"new_role":   req.Role,
		},
	)

	return c.JSON(fiber.Map{
		"id":    user.ID,
		"email": user.Email,
		"role":  user.Role,
	})
}

// DeactivateUser deactivates a user account
func (h *AdminHandler) DeactivateUser(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	adminID := c.Locals("user_id").(uuid.UUID)
	targetUserID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid user ID",
		})
	}

	// Cannot deactivate yourself
	if targetUserID == adminID {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Cannot deactivate your own account",
		})
	}

	if err := h.authService.DeactivateUser(c.Context(), targetUserID, orgID, adminID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Log audit
	h.auditService.LogAction(
		c.Context(),
		orgID,
		adminID,
		domain.AuditActionDelete,
		"user",
		targetUserID,
		c.IP(),
		c.Get("User-Agent"),
		nil,
	)

	return c.SendStatus(fiber.StatusNoContent)
}

// GetAuditLogs returns audit logs with filtering
func (h *AdminHandler) GetAuditLogs(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)

	// Parse filters
	var filters struct {
		Action     string `query:"action"`
		EntityType string `query:"entity_type"`
		EntityID   string `query:"entity_id"`
		UserID     string `query:"user_id"`
		StartDate  string `query:"start_date"`
		EndDate    string `query:"end_date"`
		Limit      int    `query:"limit"`
		Offset     int    `query:"offset"`
	}

	if err := c.Bind().Query(&filters); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid query parameters",
		})
	}

	// Set defaults
	if filters.Limit == 0 {
		filters.Limit = 100
	}

	// Parse dates if provided
	var startDate, endDate *time.Time
	if filters.StartDate != "" {
		parsed, err := time.Parse(time.RFC3339, filters.StartDate)
		if err == nil {
			startDate = &parsed
		}
	}
	if filters.EndDate != "" {
		parsed, err := time.Parse(time.RFC3339, filters.EndDate)
		if err == nil {
			endDate = &parsed
		}
	}

	// Parse entity ID if provided
	var entityID *uuid.UUID
	if filters.EntityID != "" {
		parsed, err := uuid.Parse(filters.EntityID)
		if err == nil {
			entityID = &parsed
		}
	}

	// Parse user ID if provided
	var filterUserID *uuid.UUID
	if filters.UserID != "" {
		parsed, err := uuid.Parse(filters.UserID)
		if err == nil {
			filterUserID = &parsed
		}
	}

	// Get audit logs
	logs, total, err := h.auditService.GetAuditLogs(
		c.Context(),
		orgID,
		filters.Action,
		filters.EntityType,
		entityID,
		filterUserID,
		startDate,
		endDate,
		filters.Limit,
		filters.Offset,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch audit logs",
		})
	}

	// Log this audit log query
	h.auditService.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionView,
		"audit_logs",
		uuid.Nil,
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"filters": filters,
			"total":   total,
		},
	)

	return c.JSON(fiber.Map{
		"logs":   logs,
		"total":  total,
		"limit":  filters.Limit,
		"offset": filters.Offset,
	})
}

// GetAlerts returns all alerts with optional filtering
func (h *AdminHandler) GetAlerts(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)

	// Parse filters
	severity := c.Query("severity")
	status := c.Query("status")

	// Parse limit and offset with defaults (Fiber v3 compatibility)
	limit := 100
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

	// Get alerts
	alerts, total, err := h.alertService.GetAlerts(
		c.Context(),
		orgID,
		severity,
		status,
		limit,
		offset,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch alerts",
		})
	}

	// Log audit
	h.auditService.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionView,
		"alerts",
		uuid.Nil,
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"severity": severity,
			"status":   status,
			"total":    total,
		},
	)

	return c.JSON(fiber.Map{
		"alerts": alerts,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// AcknowledgeAlert marks an alert as acknowledged
func (h *AdminHandler) AcknowledgeAlert(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)
	alertID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid alert ID",
		})
	}

	if err := h.alertService.AcknowledgeAlert(c.Context(), alertID, orgID, userID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Log audit
	h.auditService.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionAcknowledge,
		"alert",
		alertID,
		c.IP(),
		c.Get("User-Agent"),
		nil,
	)

	return c.SendStatus(fiber.StatusNoContent)
}

// ResolveAlert marks an alert as resolved
func (h *AdminHandler) ResolveAlert(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)
	alertID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid alert ID",
		})
	}

	var req struct {
		Resolution string `json:"resolution"`
	}

	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if err := h.alertService.ResolveAlert(c.Context(), alertID, orgID, userID, req.Resolution); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Log audit
	h.auditService.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionResolve,
		"alert",
		alertID,
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"resolution": req.Resolution,
		},
	)

	return c.SendStatus(fiber.StatusNoContent)
}

// GetDashboardStats returns high-level statistics for admin dashboard
func (h *AdminHandler) GetDashboardStats(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)

	// Get total agents
	agents, err := h.agentService.ListAgents(c.Context(), orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch agents",
		})
	}

	// Get total users
	users, err := h.authService.GetUsersByOrganization(c.Context(), orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch users",
		})
	}

	// Get active alerts count
	alerts, total, err := h.alertService.GetAlerts(c.Context(), orgID, "", "open", 1000, 0)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch alerts",
		})
	}

	// Count critical alerts
	criticalAlerts := 0
	for _, alert := range alerts {
		if alert.Severity == domain.AlertSeverityCritical {
			criticalAlerts++
		}
	}

	// Get MCP servers from dedicated MCP service
	mcpServersList, err := h.mcpService.ListMCPServers(c.Context(), orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch MCP servers",
		})
	}

	// Count active MCP servers
	activeMCPServers := 0
	for _, mcp := range mcpServersList {
		if mcp.Status == domain.MCPServerStatusVerified {
			activeMCPServers++
		}
	}

	// Count verified agents and calculate metrics
	verifiedAgents := 0
	pendingAgents := 0
	totalTrustScore := 0.0

	for _, agent := range agents {
		if agent.Status == domain.AgentStatusVerified {
			verifiedAgents++
		}
		if agent.Status == domain.AgentStatusPending {
			pendingAgents++
		}
		totalTrustScore += agent.TrustScore
	}

	// Calculate average trust score
	avgTrustScore := 0.0
	if len(agents) > 0 {
		avgTrustScore = totalTrustScore / float64(len(agents))
	}

	// Calculate verification rate
	verificationRate := 0.0
	if len(agents) > 0 {
		verificationRate = float64(verifiedAgents) / float64(len(agents)) * 100
	}

	// Log audit
	h.auditService.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionView,
		"dashboard_stats",
		uuid.Nil,
		c.IP(),
		c.Get("User-Agent"),
		nil,
	)

	return c.JSON(fiber.Map{
		// Agent metrics
		"total_agents":       len(agents),
		"verified_agents":    verifiedAgents,
		"pending_agents":     pendingAgents,
		"verification_rate":  verificationRate,
		"avg_trust_score":    avgTrustScore,

		// MCP Server metrics
		"total_mcp_servers":  len(mcpServersList),
		"active_mcp_servers": activeMCPServers,

		// User metrics
		"total_users":        len(users),
		"active_users":       len(users), // TODO: track last_active_at

		// Security metrics
		"active_alerts":      total,
		"critical_alerts":    criticalAlerts,
		"security_incidents": 0, // TODO: add incidents tracking

		// Organization
		"organization_id":    orgID,
	})
}
