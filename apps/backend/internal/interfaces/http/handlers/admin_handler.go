package handlers

import (
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/application"
	"github.com/opena2a/identity/backend/internal/domain"
)

type AdminHandler struct {
	authService         *application.AuthService
	adminService        *application.AdminService
	agentService        *application.AgentService
	mcpService          *application.MCPService
	auditService        *application.AuditService
	alertService        *application.AlertService
	registrationService *application.RegistrationService
}

func NewAdminHandler(
	authService *application.AuthService,
	adminService *application.AdminService,
	agentService *application.AgentService,
	mcpService *application.MCPService,
	auditService *application.AuditService,
	alertService *application.AlertService,
	registrationService *application.RegistrationService,
) *AdminHandler {
	return &AdminHandler{
		authService:         authService,
		adminService:        adminService,
		agentService:        agentService,
		mcpService:          mcpService,
		auditService:        auditService,
		alertService:        alertService,
		registrationService: registrationService,
	}
}

// ListUsers returns all users in the organization including pending registration requests
func (h *AdminHandler) ListUsers(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)

	// Get approved users
	users, err := h.authService.GetUsersByOrganization(c.Context(), orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch users",
		})
	}

	// Get pending registration requests
	pendingRequests, _, err := h.registrationService.ListPendingRegistrationRequests(c.Context(), orgID, 100, 0)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch pending registration requests",
		})
	}

	// Convert pending requests to a user-like format for the frontend
	type UserWithStatus struct {
		ID                    uuid.UUID  `json:"id"`
		Email                 string     `json:"email"`
		Name                  string     `json:"name"`
		Role                  string     `json:"role"`
		Status                string     `json:"status"`
		CreatedAt             time.Time  `json:"created_at"`
		Provider              string     `json:"provider,omitempty"`
		LastLoginAt           *time.Time `json:"last_login_at,omitempty"`
		RequestedAt           *time.Time `json:"requested_at,omitempty"`
		PictureURL            *string    `json:"picture_url,omitempty"`
		IsRegistrationRequest bool       `json:"is_registration_request"`
	}

	var allUsers []UserWithStatus

	// Add approved users
	for _, user := range users {
		allUsers = append(allUsers, UserWithStatus{
			ID:                    user.ID,
			Email:                 user.Email,
			Name:                  user.Name,
			Role:                  string(user.Role),
			Status:                string(user.Status),
			CreatedAt:             user.CreatedAt,
			LastLoginAt:           user.LastLoginAt,
			IsRegistrationRequest: false,
		})
	}

	// Add pending registration requests
	for _, req := range pendingRequests {
		fullName := req.FirstName
		if req.LastName != "" {
			if fullName != "" {
				fullName += " "
			}
			fullName += req.LastName
		}
		if fullName == "" {
			fullName = req.Email
		}

		allUsers = append(allUsers, UserWithStatus{
			ID:                    req.ID,
			Email:                 req.Email,
			Name:                  fullName,
			Role:                  "pending",
			Status:                "pending_approval",
			CreatedAt:             req.CreatedAt,
			Provider:              func() string {
			if req.OAuthProvider != nil {
				return string(*req.OAuthProvider)
			}
			return "manual"
		}(),
			RequestedAt:           &req.RequestedAt,
			PictureURL:            req.ProfilePictureURL,
			IsRegistrationRequest: true,
		})
	}

	// Log audit
	h.auditService.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionView,
		"users",
		orgID, // Use orgID for collection operations
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"total_users":           len(users),
			"pending_registrations": len(pendingRequests),
			"total_combined":        len(allUsers),
		},
	)

	return c.JSON(fiber.Map{
		"users":                 allUsers,
		"total":                 len(allUsers),
		"approved_users":        len(users),
		"pending_registrations": len(pendingRequests),
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
		domain.AuditActionUpdate,
		"user",
		targetUserID,
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"action": "deactivate",
			"type":   "soft_delete",
		},
	)

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "User deactivated successfully",
	})
}

// ActivateUser reactivates a deactivated user account
func (h *AdminHandler) ActivateUser(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	adminID := c.Locals("user_id").(uuid.UUID)
	targetUserID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid user ID",
		})
	}

	// Verify user belongs to the same organization
	user, err := h.authService.GetUserByID(c.Context(), targetUserID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "User not found",
		})
	}

	if user.OrganizationID != orgID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "User not found in organization",
		})
	}

	// Activate user using admin service
	if err := h.adminService.ActivateUser(c.Context(), targetUserID, adminID); err != nil {
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
		"user",
		targetUserID,
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"action": "activate",
		},
	)

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "User activated successfully",
	})
}

// PermanentlyDeleteUser permanently deletes a user from the database (hard delete)
func (h *AdminHandler) PermanentlyDeleteUser(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	adminID := c.Locals("user_id").(uuid.UUID)
	targetUserID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid user ID",
		})
	}

	// Verify user belongs to the same organization
	user, err := h.authService.GetUserByID(c.Context(), targetUserID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "User not found",
		})
	}

	if user.OrganizationID != orgID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "User not found in organization",
		})
	}

	// Cannot delete yourself
	if targetUserID == adminID {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Cannot delete your own account",
		})
	}

	// Store user info for audit log before deletion
	userEmail := user.Email
	userName := user.Name

	// Permanently delete user using admin service
	if err := h.adminService.PermanentlyDeleteUser(c.Context(), targetUserID, adminID); err != nil {
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
		map[string]interface{}{
			"action":     "permanent_delete",
			"user_email": userEmail,
			"user_name":  userName,
			"warning":    "irreversible_hard_delete",
		},
	)

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "User permanently deleted",
	})
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

	// Log this audit log query with enhanced metadata
	metadata := map[string]interface{}{
		"results_returned": len(logs),
		"total_available":  total,
		"page_number":      (filters.Offset / filters.Limit) + 1,
		"page_size":        filters.Limit,
	}

	// Only include non-empty filters
	if filters.Action != "" {
		metadata["filter_action"] = filters.Action
	}
	if filters.EntityType != "" {
		metadata["filter_resource_type"] = filters.EntityType
	}
	if filters.EntityID != "" {
		metadata["filter_resource_id"] = filters.EntityID
	}
	if filters.UserID != "" {
		metadata["filter_user_id"] = filters.UserID
	}
	if filters.StartDate != "" {
		metadata["filter_start_date"] = filters.StartDate
	}
	if filters.EndDate != "" {
		metadata["filter_end_date"] = filters.EndDate
	}

	h.auditService.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionView,
		"audit_logs",
		orgID, // Use orgID for collection operations
		c.IP(),
		c.Get("User-Agent"),
		metadata,
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

	// Log audit with enhanced metadata
	metadata := map[string]interface{}{
		"results_returned": len(alerts),
		"total_available":  total,
		"page_number":      (offset / limit) + 1,
		"page_size":        limit,
	}

	// Only include non-empty filters
	if severity != "" {
		metadata["filter_severity"] = severity
	}
	if status != "" {
		metadata["filter_status"] = status
	}

	h.auditService.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionView,
		"alerts",
		orgID, // Use orgID for collection operations
		c.IP(),
		c.Get("User-Agent"),
		metadata,
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

	// Log audit with dashboard metrics
	h.auditService.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionView,
		"dashboard_stats",
		orgID, // Use orgID for collection operations
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"total_agents":      len(agents),
			"verified_agents":   verifiedAgents,
			"total_mcp_servers": len(mcpServersList),
			"total_users":       len(users),
			"active_alerts":     total,
			"critical_alerts":   criticalAlerts,
		},
	)

	return c.JSON(fiber.Map{
		// Agent metrics
		"total_agents":      len(agents),
		"verified_agents":   verifiedAgents,
		"pending_agents":    pendingAgents,
		"verification_rate": verificationRate,
		"avg_trust_score":   avgTrustScore,

		// MCP Server metrics
		"total_mcp_servers":  len(mcpServersList),
		"active_mcp_servers": activeMCPServers,

		// User metrics
		"total_users":  len(users),
		"active_users": len(users), // TODO: track last_active_at

		// Security metrics
		"active_alerts":      total,
		"critical_alerts":    criticalAlerts,
		"security_incidents": 0, // TODO: add incidents tracking

		// Organization
		"organization_id": orgID,
	})
}

// GetPendingUsers returns users awaiting approval
func (h *AdminHandler) GetPendingUsers(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)

	users, err := h.adminService.GetPendingUsers(c.Context(), orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch pending users",
		})
	}

	// Log audit
	h.auditService.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionView,
		"pending_users",
		orgID, // Use orgID for collection operations
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"total_pending": len(users),
		},
	)

	return c.JSON(fiber.Map{
		"users": users,
		"total": len(users),
	})
}

// ApproveUser approves a pending user
func (h *AdminHandler) ApproveUser(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	adminID := c.Locals("user_id").(uuid.UUID)
	targetUserID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid user ID",
		})
	}

	if err := h.adminService.ApproveUser(c.Context(), targetUserID, adminID); err != nil {
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
		"user_approval",
		targetUserID,
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"action": "approved",
		},
	)

	return c.JSON(fiber.Map{
		"message": "User approved successfully",
	})
}

// RejectUser rejects a pending user
func (h *AdminHandler) RejectUser(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	adminID := c.Locals("user_id").(uuid.UUID)
	targetUserID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid user ID",
		})
	}

	var req struct {
		Reason string `json:"reason"`
	}

	if err := c.Bind().JSON(&req); err != nil {
		// Reason is optional
		req.Reason = ""
	}

	if err := h.adminService.RejectUser(c.Context(), targetUserID, adminID, req.Reason); err != nil {
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
		"user_rejection",
		targetUserID,
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"action": "rejected",
			"reason": req.Reason,
		},
	)

	return c.JSON(fiber.Map{
		"message": "User rejected successfully",
	})
}

// ApproveRegistrationRequest approves a pending registration request from the users page
func (h *AdminHandler) ApproveRegistrationRequest(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	adminID := c.Locals("user_id").(uuid.UUID)
	requestID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request ID",
		})
	}

	// Approve registration request
	newUser, err := h.registrationService.ApproveRegistrationRequest(c.Context(), requestID, adminID, orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to approve registration: %v", err),
		})
	}

	// Log audit
	h.auditService.LogAction(
		c.Context(),
		orgID,
		adminID,
		domain.AuditActionUpdate,
		"registration_approval",
		requestID,
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"action":     "approved",
			"user_email": newUser.Email,
			"user_name":  newUser.Name,
		},
	)

	return c.JSON(fiber.Map{
		"message": "Registration request approved successfully",
		"user":    newUser,
	})
}

// RejectRegistrationRequest rejects a pending registration request from the users page
func (h *AdminHandler) RejectRegistrationRequest(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	adminID := c.Locals("user_id").(uuid.UUID)
	requestID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request ID",
		})
	}

	var req struct {
		Reason string `json:"reason"`
	}

	if err := c.Bind().JSON(&req); err != nil {
		// Reason is optional
		req.Reason = "Rejected by admin"
	}

	// Reject registration request
	if err := h.registrationService.RejectRegistrationRequest(c.Context(), requestID, adminID, req.Reason); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to reject registration: %v", err),
		})
	}

	// Log audit
	h.auditService.LogAction(
		c.Context(),
		orgID,
		adminID,
		domain.AuditActionDelete,
		"registration_rejection",
		requestID,
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"action": "rejected",
			"reason": req.Reason,
		},
	)

	return c.JSON(fiber.Map{
		"message": "Registration request rejected successfully",
	})
}

// GetOrganizationSettings retrieves organization settings
func (h *AdminHandler) GetOrganizationSettings(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)

	org, err := h.adminService.GetOrganizationSettings(c.Context(), orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch organization settings",
		})
	}

	// Log audit with settings viewed
	h.auditService.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionView,
		"organization_settings",
		orgID,
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"organization_name": org.Name,
			"plan_type":         org.PlanType,
			"is_active":         org.IsActive,
		},
	)

	return c.JSON(fiber.Map{
		"id":               org.ID,
		"name":             org.Name,
		"domain":           org.Domain,
		"plan_type":        org.PlanType,
		"max_agents":       org.MaxAgents,
		"max_users":        org.MaxUsers,
		"auto_approve_sso": org.AutoApproveSSO,
		"is_active":        org.IsActive,
	})
}

// UpdateOrganizationSettings updates organization settings
func (h *AdminHandler) UpdateOrganizationSettings(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	adminID := c.Locals("user_id").(uuid.UUID)

	var req struct {
		AutoApproveSSO bool `json:"auto_approve_sso"`
	}

	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if err := h.adminService.UpdateOrganizationSettings(c.Context(), orgID, req.AutoApproveSSO); err != nil {
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
		"organization_settings",
		orgID,
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"auto_approve_sso": req.AutoApproveSSO,
		},
	)

	return c.JSON(fiber.Map{
		"message":          "Organization settings updated successfully",
		"auto_approve_sso": req.AutoApproveSSO,
	})
}

// ApproveDrift approves configuration drift by updating agent's registered configuration
func (h *AdminHandler) ApproveDrift(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)
	alertID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid alert ID",
		})
	}

	var req struct {
		ApprovedMCPServers []string `json:"approvedMcpServers"`
	}

	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Call alert service to approve drift
	approveDriftReq := &application.ApproveDriftRequest{
		AlertID:            alertID,
		OrganizationID:     orgID,
		UserID:             userID,
		ApprovedMCPServers: req.ApprovedMCPServers,
	}

	if err := h.alertService.ApproveDrift(c.Context(), approveDriftReq); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Log audit
	h.auditService.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionUpdate,
		"drift_approval",
		alertID,
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"approved_mcp_servers": req.ApprovedMCPServers,
		},
	)

	return c.JSON(fiber.Map{
		"message": "Configuration drift approved successfully",
	})
}
