package handlers

import (
	"context"
	"fmt"
	"log"
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
	securityService     *application.SecurityService
}

func NewAdminHandler(
	authService *application.AuthService,
	adminService *application.AdminService,
	agentService *application.AgentService,
	mcpService *application.MCPService,
	auditService *application.AuditService,
	alertService *application.AlertService,
	registrationService *application.RegistrationService,
	securityService *application.SecurityService,
) *AdminHandler {
	return &AdminHandler{
		authService:         authService,
		adminService:        adminService,
		agentService:        agentService,
		mcpService:          mcpService,
		auditService:        auditService,
		alertService:        alertService,
		registrationService: registrationService,
		securityService:     securityService,
	}
}

// ListUsers returns all users in the organization including pending registration requests
func (h *AdminHandler) ListUsers(c fiber.Ctx) error {
	// 🔍 Safe type assertion with error checking
	orgIDValue := c.Locals("organization_id")
	if orgIDValue == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Organization ID not found in context",
		})
	}

	orgID, ok := orgIDValue.(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Invalid organization ID type in context",
		})
	}

	userIDValue := c.Locals("user_id")
	if userIDValue == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "User ID not found in context",
		})
	}

	userID, ok := userIDValue.(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Invalid user ID type in context",
		})
	}

	// Get approved users
	users, err := h.authService.GetUsersByOrganization(c.Context(), orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch users",
		})
	}

	// Get pending registration requests (optional - table may not exist in all deployments)
	pendingRequests, _, err := h.registrationService.ListPendingRegistrationRequests(c.Context(), orgID, 100, 0)
	if err != nil {
		// ℹ️ If table doesn't exist or query fails, just show approved users
		log.Printf("⚠️ Warning: Failed to fetch pending registration requests (table may not exist): %v", err)
		pendingRequests = []*domain.UserRegistrationRequest{} // Empty slice, no pending requests
	}

	// Convert pending requests to a user-like format for the frontend
	type UserWithStatus struct {
		ID                    uuid.UUID  `json:"id"`
		Email                 string     `json:"email"`
		Name                  string     `json:"name"`
		Role                  string     `json:"role"`
		Status                string     `json:"status"`
		CreatedAt             time.Time  `json:"createdAt"`
		Provider              string     `json:"provider,omitempty"`
		LastLoginAt           *time.Time `json:"lastLoginAt,omitempty"`
		RequestedAt           *time.Time `json:"requestedAt,omitempty"`
		PictureURL            *string    `json:"pictureUrl,omitempty"`
		IsRegistrationRequest bool       `json:"isRegistrationRequest"`
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
			ID:        req.ID,
			Email:     req.Email,
			Name:      fullName,
			Role:      "pending",
			Status:    "pending_approval",
			CreatedAt: req.CreatedAt,
			Provider: func() string {
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
			"totalUsers":            len(users),
			"pendingRegistrations":  len(pendingRequests),
			"totalCombined":         len(allUsers),
		},
	)

	return c.JSON(fiber.Map{
		"users":                allUsers,
		"total":                len(allUsers),
		"approvedUsers":        len(users),
		"pendingRegistrations": len(pendingRequests),
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

	// Check if target user is the super admin (first admin user in the organization)
	// Super admin is identified as the oldest admin user by created_at timestamp
	isSuperAdmin, err := h.isSuperAdmin(c.Context(), targetUserID, orgID)
	if err != nil {
		log.Printf("⚠️ Error checking super admin status: %v", err)
	}

	if isSuperAdmin {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Cannot deactivate the super administrator account. This account is protected to ensure system access.",
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

	// Check if target user is the super admin (first admin user in the organization)
	// Super admin is identified as the oldest admin user by created_at timestamp
	isSuperAdmin, err := h.isSuperAdmin(c.Context(), targetUserID, orgID)
	if err != nil {
		log.Printf("⚠️ Error checking super admin status: %v", err)
	}

	if isSuperAdmin {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Cannot delete the super administrator account. This account is protected to ensure system access.",
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
	// 🔍 Safe type assertion with error checking
	orgIDValue := c.Locals("organization_id")
	if orgIDValue == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Organization ID not found in context",
		})
	}

	orgID, ok := orgIDValue.(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Invalid organization ID type in context",
		})
	}

	userIDValue := c.Locals("user_id")
	if userIDValue == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "User ID not found in context",
		})
	}

	userID, ok := userIDValue.(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Invalid user ID type in context",
		})
	}

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
	// 🔍 Safe type assertion with error checking
	orgIDValue := c.Locals("organization_id")
	if orgIDValue == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Organization ID not found in context",
		})
	}

	orgID, ok := orgIDValue.(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Invalid organization ID type in context",
		})
	}

	userIDValue := c.Locals("user_id")
	if userIDValue == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "User ID not found in context",
		})
	}

	userID, ok := userIDValue.(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Invalid user ID type in context",
		})
	}

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

	// Get alert counts (all, acknowledged, unacknowledged)
	allCount, acknowledgedCount, unacknowledgedCount, err := h.alertService.CountUnacknowledged(c.Context(), orgID)
	if err != nil {
		// If count fails, set defaults but don't fail the request
		allCount = total
		acknowledgedCount = 0
		unacknowledgedCount = 0
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
		"alerts":               alerts,
		"total":                total,
		"allCount":            allCount,
		"acknowledgedCount":   acknowledgedCount,
		"unacknowledgedCount": unacknowledgedCount,
		"limit":                limit,
		"offset":               offset,
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

// BulkAcknowledgeAlerts acknowledges multiple alerts at once
func (h *AdminHandler) BulkAcknowledgeAlerts(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)

	var req struct {
		UserID string `json:"user_id"`
	}

	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// If user_id supplied, ensure it matches authenticated user
	if req.UserID != "" {
		bodyUserID, err := uuid.Parse(req.UserID)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": fmt.Sprintf("Invalid user ID: %s", req.UserID),
			})
		}
		if bodyUserID != userID {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "User ID mismatch",
			})
		}
	}

	ackCount, err := h.alertService.BulkAcknowledgeAlerts(c.Context(), orgID, userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Log audit with metadata
	h.auditService.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionAcknowledge,
		"alert",
		uuid.Nil,
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"bulk_acknowledge_scope": "all_alerts",
		},
	)

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message":           fmt.Sprintf("Acknowledged %d alerts", ackCount),
		"acknowledgedCount": ackCount,
		"bulkAcknowledged":  ackCount > 0,
	})
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

	// Get security incidents count
	securityIncidents := 0
	if h.securityService != nil {
		incidentCount, err := h.securityService.CountOpenIncidents(c.Context(), orgID)
		if err == nil {
			securityIncidents = incidentCount
		}
	}

	// Get active users count (users who logged in within the last 60 minutes)
	activeUsers := len(users) // Default to total users if count fails
	activeUserCount, err := h.authService.CountActiveUsers(c.Context(), orgID, 60)
	if err == nil {
		activeUsers = activeUserCount
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
			"totalAgents":      len(agents),
			"verifiedAgents":   verifiedAgents,
			"totalMcpServers": len(mcpServersList),
			"totalUsers":       len(users),
			"activeAlerts":     total,
			"criticalAlerts":   criticalAlerts,
		},
	)

	return c.JSON(fiber.Map{
		// Agent metrics
		"totalAgents":      len(agents),
		"verifiedAgents":   verifiedAgents,
		"pendingAgents":    pendingAgents,
		"verificationRate": verificationRate,
		"avgTrustScore":   avgTrustScore,

		// MCP Server metrics
		"totalMcpServers":  len(mcpServersList),
		"activeMcpServers": activeMCPServers,

		// User metrics
		"totalUsers":  len(users),
		"activeUsers": activeUsers,

		// Security metrics
		"activeAlerts":      total,
		"criticalAlerts":    criticalAlerts,
		"securityIncidents": securityIncidents,

		// Organization
		"organizationId": orgID,
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
			"totalPending": len(users),
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
	// 🔍 Safe type assertion with error checking
	orgIDValue := c.Locals("organization_id")
	if orgIDValue == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Organization ID not found in context",
		})
	}

	orgID, ok := orgIDValue.(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Invalid organization ID type in context",
		})
	}

	userIDValue := c.Locals("user_id")
	if userIDValue == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "User ID not found in context",
		})
	}

	userID, ok := userIDValue.(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Invalid user ID type in context",
		})
	}

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
			"organizationName": org.Name,
			"isActive":         org.IsActive,
		},
	)

	return c.JSON(fiber.Map{
		"id":        org.ID,
		"name":      org.Name,
		"domain":    org.Domain,
		"maxAgents": org.MaxAgents,
		"maxUsers":  org.MaxUsers,
		"isActive":  org.IsActive,
	})
}

// GetUnacknowledgedAlertCount returns the count of unacknowledged alerts for an organization
func (h *AdminHandler) GetUnacknowledgedAlertCount(c fiber.Ctx) error {
	// Get organization ID from user context
	orgIDValue := c.Locals("organization_id")
	if orgIDValue == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Organization ID not found in context",
		})
	}

	orgID, ok := orgIDValue.(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Invalid organization ID type in context",
		})
	}

	// Call alert service to count alerts
	allCount, acknowledgedCount, unacknowledgedCount, err := h.alertService.CountUnacknowledged(c.Context(), orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"allCount":            allCount,
		"acknowledgedCount":   acknowledgedCount,
		"unacknowledgedCount": unacknowledgedCount,
	})
}

// isSuperAdmin checks if the given user is the super admin (first admin user created in the organization)
// Super admin is protected from deactivation and deletion to ensure system access
func (h *AdminHandler) isSuperAdmin(ctx context.Context, userID, orgID uuid.UUID) (bool, error) {
	// Get the user to check their role
	user, err := h.authService.GetUserByID(ctx, userID)
	if err != nil {
		return false, err
	}

	// Only admin users can be super admins
	if user.Role != "admin" {
		return false, nil
	}

	// Verify user belongs to the organization
	if user.OrganizationID != orgID {
		return false, nil
	}

	// Get all users in the organization
	users, err := h.authService.GetUsersByOrganization(ctx, orgID)
	if err != nil {
		return false, err
	}

	// Find all admin users and sort by created_at (oldest first)
	var admins []*domain.User
	for _, u := range users {
		if u.Role == "admin" && u.Status == "active" {
			admins = append(admins, u)
		}
	}

	// If no admins found or only one admin (must be super admin), return true for that admin
	if len(admins) == 0 {
		return false, nil
	}

	// Find the oldest admin (super admin)
	oldestAdmin := admins[0]
	for _, admin := range admins {
		if admin.CreatedAt.Before(oldestAdmin.CreatedAt) {
			oldestAdmin = admin
		}
	}

	// User is super admin if they are the oldest admin created
	return oldestAdmin.ID == userID, nil
}
