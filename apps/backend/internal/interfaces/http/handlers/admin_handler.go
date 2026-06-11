package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

type AdminHandler struct {
	// Concrete service pointers (used by existing code)
	authService         *application.AuthService
	adminService        *application.AdminService
	agentService        *application.AgentService
	mcpService          *application.MCPService
	auditService        *application.AuditService
	alertService        *application.AlertService
	registrationService *application.RegistrationService
	securityService     *application.SecurityService

	// Repository handle for handler-layer tenant scoping.
	// SECURITY (A3d-v): used by ApproveUser / RejectUser to load the
	// target user and verify it belongs to the caller's org BEFORE
	// invoking the service. The service-layer check exists but returns
	// a distinct error string for cross-tenant vs not-found (existence
	// side channel) — handler-layer LoadOwned collapses both to 404.
	userRepo domain.UserRepository

	// Interface fields for testability (used when set)
	authServicer         AuthServicer
	adminServicer        AdminServicer
	agentServicer        AgentServicer
	mcpServicer          MCPServicer
	auditServicer        AuditServicer
	alertServicer        AlertServicerExtended
	registrationServicer RegistrationServicer
	securityServicer     SecurityServicer
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
	userRepo domain.UserRepository,
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
		userRepo:            userRepo,
	}
}

// NewAdminHandlerWithInterfaces creates an AdminHandler using interface-based dependencies
// This constructor is primarily used for testing with mock implementations
func NewAdminHandlerWithInterfaces(
	authService AuthServicer,
	adminService AdminServicer,
	agentService AgentServicer,
	mcpService MCPServicer,
	auditService AuditServicer,
	alertService AlertServicerExtended,
	registrationService RegistrationServicer,
	securityService SecurityServicer,
	userRepo domain.UserRepository,
) *AdminHandler {
	return &AdminHandler{
		authServicer:         authService,
		adminServicer:        adminService,
		agentServicer:        agentService,
		mcpServicer:          mcpService,
		auditServicer:        auditService,
		alertServicer:        alertService,
		registrationServicer: registrationService,
		securityServicer:     securityService,
		userRepo:             userRepo,
	}
}

// Helper methods to get the appropriate service (interface or concrete)

func (h *AdminHandler) getAuthService() AuthServicer {
	if h.authServicer != nil {
		return h.authServicer
	}
	return h.authService
}

func (h *AdminHandler) getAdminService() AdminServicer {
	if h.adminServicer != nil {
		return h.adminServicer
	}
	return h.adminService
}

func (h *AdminHandler) getAgentService() AgentServicer {
	if h.agentServicer != nil {
		return h.agentServicer
	}
	return h.agentService
}

func (h *AdminHandler) getMCPService() MCPServicer {
	if h.mcpServicer != nil {
		return h.mcpServicer
	}
	return h.mcpService
}

func (h *AdminHandler) getAuditService() AuditServicer {
	if h.auditServicer != nil {
		return h.auditServicer
	}
	return h.auditService
}

func (h *AdminHandler) getAlertService() AlertServicerExtended {
	if h.alertServicer != nil {
		return h.alertServicer
	}
	return h.alertService
}

func (h *AdminHandler) getRegistrationService() RegistrationServicer {
	if h.registrationServicer != nil {
		return h.registrationServicer
	}
	return h.registrationService
}

func (h *AdminHandler) getSecurityService() SecurityServicer {
	if h.securityServicer != nil {
		return h.securityServicer
	}
	return h.securityService
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
	users, err := h.getAuthService().GetUsersByOrganization(c.Context(), orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch users",
		})
	}

	// Get pending registration requests (optional - table may not exist in all deployments)
	pendingRequests, _, err := h.getRegistrationService().ListPendingRegistrationRequests(c.Context(), orgID, 100, 0)
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

	allUsers := make([]UserWithStatus, 0)

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
	h.getAuditService().LogAction(
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

	// SECURITY (A3d-viii): verify the target user belongs to the caller's
	// org before invoking the update flow. The service-layer check returns
	// a distinct error string for cross-tenant vs not-found, which the
	// handler then serialises as 500 with the error in the body — an
	// existence side channel that lets an attacker enumerate user UUIDs
	// across tenants. Handler-layer LoadOwned collapses both to 404.
	if LoadOwned(c, h.userRepo.GetByID, targetUserID, orgID, userOrgID) == nil {
		return nil
	}

	// Update user role
	user, err := h.getAuthService().UpdateUserRole(c.Context(), targetUserID, orgID, role, adminID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Log audit
	h.getAuditService().LogAction(
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

	// SECURITY (A3d-viii): verify the target user belongs to the caller's
	// org BEFORE the super-admin lookup and the service call. The service
	// echoes a distinct error string for cross-tenant vs not-found, which
	// the handler then surfaces as 500 with the body — an existence side
	// channel. LoadOwned collapses both to 404.
	if LoadOwned(c, h.userRepo.GetByID, targetUserID, orgID, userOrgID) == nil {
		return nil
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

	if err := h.getAuthService().DeactivateUser(c.Context(), targetUserID, orgID, adminID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Log audit
	h.getAuditService().LogAction(
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

	// SECURITY (A3d-viii): replace the pre-fix existence oracle (404
	// "User not found" vs 403 "User not found in organization") with the
	// LoadOwned helper. Both branches now collapse to a fixed 404 body,
	// closing the side channel that let an attacker distinguish "no such
	// user system-wide" from "user exists in another org".
	if LoadOwned(c, h.userRepo.GetByID, targetUserID, orgID, userOrgID) == nil {
		return nil
	}

	// Activate user using admin service
	if err := h.getAdminService().ActivateUser(c.Context(), targetUserID, adminID, orgID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Log audit
	h.getAuditService().LogAction(
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

	// Cannot delete yourself
	if targetUserID == adminID {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Cannot delete your own account",
		})
	}

	// SECURITY (A3d-viii): replace the pre-fix existence oracle (404
	// "User not found" vs 403 "User not found in organization") with the
	// LoadOwned helper. Both branches now collapse to a fixed 404 body,
	// closing the side channel. The returned user is reused below for
	// the audit-log email/name capture.
	user := LoadOwned(c, h.userRepo.GetByID, targetUserID, orgID, userOrgID)
	if user == nil {
		return nil
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
	if err := h.getAdminService().PermanentlyDeleteUser(c.Context(), targetUserID, adminID, orgID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Log audit
	h.getAuditService().LogAction(
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
	logs, total, err := h.getAuditService().GetAuditLogs(
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

	h.getAuditService().LogAction(
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

// GetAuditLogByID returns a single audit log entry by ID
func (h *AdminHandler) GetAuditLogByID(c fiber.Ctx) error {
	// Parse audit log ID from URL
	idParam := c.Params("id")
	if idParam == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Audit log ID is required",
		})
	}

	auditLogID, err := uuid.Parse(idParam)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid audit log ID format",
		})
	}

	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}

	// SECURITY (defect #21): tenant-scoping. An admin in org A used to be
	// able to read any audit log by guessing its UUID. LoadOwned wraps the
	// service GetByID, verifies auditLog.OrganizationID == orgID, and
	// returns the same 404 for both "not found" and "exists in another
	// org" to prevent a cross-tenant existence side channel.
	loader := func(id uuid.UUID) (*domain.AuditLog, error) {
		return h.getAuditService().GetByID(c.Context(), id)
	}
	auditLog := LoadOwned(c, loader, auditLogID, orgID, auditLogOrgID)
	if auditLog == nil {
		return nil
	}

	return c.JSON(auditLog)
}

// ExportAuditLogs exports audit logs as CSV or JSON
func (h *AdminHandler) ExportAuditLogs(c fiber.Ctx) error {
	// Safe type assertion with error checking
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

	// Parse format (csv or json, default to csv)
	format := strings.ToLower(c.Query("format", "csv"))
	if format != "csv" && format != "json" {
		format = "csv"
	}

	// Parse date filters
	var startDate, endDate *time.Time
	if startStr := c.Query("start_date"); startStr != "" {
		if parsed, err := time.Parse(time.RFC3339, startStr); err == nil {
			startDate = &parsed
		}
	}
	if endStr := c.Query("end_date"); endStr != "" {
		if parsed, err := time.Parse(time.RFC3339, endStr); err == nil {
			endDate = &parsed
		}
	}

	// Get all audit logs (no pagination for export, up to 10000)
	logs, total, err := h.getAuditService().GetAuditLogs(
		c.Context(),
		orgID,
		"", // action filter
		"", // entity type filter
		nil, // entity ID
		nil, // user ID filter
		startDate,
		endDate,
		10000, // max export limit
		0,     // offset
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch audit logs for export",
		})
	}

	// Log the export action
	h.getAuditService().LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionView,
		"audit_logs_export",
		orgID,
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"format":        format,
			"total_records": total,
			"exported":      len(logs),
		},
	)

	if format == "json" {
		// Return as JSON array
		c.Set("Content-Type", "application/json")
		c.Set("Content-Disposition", "attachment; filename=audit-logs.json")
		return c.JSON(logs)
	}

	// Return as CSV
	var csvBuilder strings.Builder
	csvBuilder.WriteString("ID,Timestamp,Action,ResourceType,ResourceID,UserID,IPAddress,UserAgent,Metadata\n")

	for _, log := range logs {
		// Serialize metadata to JSON string for CSV
		metadataJSON := ""
		if log.Metadata != nil {
			if jsonBytes, err := json.Marshal(log.Metadata); err == nil {
				metadataJSON = strings.ReplaceAll(string(jsonBytes), "\"", "\"\"") // Escape quotes for CSV
			}
		}

		// Format: escape quotes in fields and wrap in quotes
		csvBuilder.WriteString(fmt.Sprintf("\"%s\",\"%s\",\"%s\",\"%s\",\"%s\",\"%s\",\"%s\",\"%s\",\"%s\"\n",
			log.ID.String(),
			log.Timestamp.Format(time.RFC3339),
			log.Action,
			log.ResourceType,
			log.ResourceID.String(),
			log.UserID.String(),
			log.IPAddress,
			strings.ReplaceAll(log.UserAgent, "\"", "\"\""),
			metadataJSON,
		))
	}

	c.Set("Content-Type", "text/csv")
	c.Set("Content-Disposition", "attachment; filename=audit-logs.csv")
	return c.SendString(csvBuilder.String())
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
	alerts, total, err := h.getAlertService().GetAlerts(
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
	allCount, acknowledgedCount, unacknowledgedCount, err := h.getAlertService().CountUnacknowledged(c.Context(), orgID)
	if err != nil {
		// If count fails, set defaults but don't fail the request
		allCount = total
		acknowledgedCount = 0
		unacknowledgedCount = 0
	}

	// Get severity counts based on current status filter
	criticalCount, highCount, warningCount, infoCount, err := h.getAlertService().CountBySeverity(c.Context(), orgID, status)
	if err != nil {
		// If severity count fails, set defaults but don't fail the request
		criticalCount, highCount, warningCount, infoCount = 0, 0, 0, 0
	}

	// Map severity buckets for frontend (warning = medium, info = lowAndInfo)
	mediumCount := warningCount
	lowAndInfoCount := infoCount

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

	h.getAuditService().LogAction(
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
		"alerts":              alerts,
		"total":               total,
		"allCount":            allCount,
		"acknowledgedCount":   acknowledgedCount,
		"unacknowledgedCount": unacknowledgedCount,
		"criticalCount":       criticalCount,
		"highCount":           highCount,
		"mediumCount":         mediumCount,
		"lowAndInfoCount":     lowAndInfoCount,
		"limit":               limit,
		"offset":              offset,
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

	if err := h.getAlertService().AcknowledgeAlert(c.Context(), alertID, orgID, userID); err != nil {
		// SECURITY (A3d-v R7 follow-up): cross-tenant and not-found
		// collapse to ErrAlertNotFound. The 404 is intentional —
		// existence-secrecy across tenants. err.Error() is NOT echoed
		// so the SQL error / wrapped detail cannot leak through.
		if errors.Is(err, application.ErrAlertNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to acknowledge alert",
		})
	}

	// Log audit
	h.getAuditService().LogAction(
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
		UserID string `json:"userId"`
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

	ackCount, err := h.getAlertService().BulkAcknowledgeAlerts(c.Context(), orgID, userID)
	if err != nil {
		// Parity with AcknowledgeAlert / ResolveAlert above: do not
		// echo err.Error() (could carry SQL detail / wrapped state).
		// Bulk path is tenant-scoped at the repo (WHERE organization_id
		// = $3) so there is no IDOR, but the no-echo discipline keeps
		// the file consistent.
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to bulk acknowledge alerts",
		})
	}

	// Log audit with metadata
	h.getAuditService().LogAction(
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

	if err := h.getAlertService().ResolveAlert(c.Context(), alertID, orgID, userID, req.Resolution); err != nil {
		// SECURITY (A3d-v R7 follow-up): see AcknowledgeAlert above.
		if errors.Is(err, application.ErrAlertNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to resolve alert",
		})
	}

	// Log audit
	h.getAuditService().LogAction(
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
	agents, err := h.getAgentService().ListAgents(c.Context(), orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch agents",
		})
	}

	// Get total users
	users, err := h.getAuthService().GetUsersByOrganization(c.Context(), orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch users",
		})
	}

	// Get active alerts count
	alerts, total, err := h.getAlertService().GetAlerts(c.Context(), orgID, "", "open", 1000, 0)
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
	mcpServersList, err := h.getMCPService().ListMCPServers(c.Context(), orgID)
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
	securitySvc := h.getSecurityService()
	if securitySvc != nil {
		incidentCount, err := securitySvc.CountOpenIncidents(c.Context(), orgID)
		if err == nil {
			securityIncidents = incidentCount
		}
	}

	// Get active users count (users who logged in within the last 60 minutes)
	activeUsers := len(users) // Default to total users if count fails
	activeUserCount, err := h.getAuthService().CountActiveUsers(c.Context(), orgID, 60)
	if err == nil {
		activeUsers = activeUserCount
	}

	// Log audit with dashboard metrics
	h.getAuditService().LogAction(
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

	users, err := h.getAdminService().GetPendingUsers(c.Context(), orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch pending users",
		})
	}

	// Log audit
	h.getAuditService().LogAction(
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

	// SECURITY (A3d-v): verify the target user belongs to the caller's
	// org before invoking the approve flow. The service-layer check
	// exists but returns a distinct error string for cross-tenant vs
	// not-found, which the handler then serialises as 500 with the
	// error in the body — an existence side channel. Handler-layer
	// LoadOwned collapses both to 404.
	if LoadOwned(c, h.userRepo.GetByID, targetUserID, orgID, userOrgID) == nil {
		return nil
	}

	if err := h.getAdminService().ApproveUser(c.Context(), targetUserID, adminID, orgID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Log audit
	h.getAuditService().LogAction(
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

	// SECURITY (A3d-v): see ApproveUser comment. Same existence-secrecy
	// fix applied to the reject path.
	if LoadOwned(c, h.userRepo.GetByID, targetUserID, orgID, userOrgID) == nil {
		return nil
	}

	if err := h.getAdminService().RejectUser(c.Context(), targetUserID, adminID, orgID, req.Reason); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Log audit
	h.getAuditService().LogAction(
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

// ListRegistrationRequests returns pending registration requests for the
// admin registrations page (includes signup profile answers via metadata)
func (h *AdminHandler) ListRegistrationRequests(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)

	limit := 50
	if parsed, err := strconv.Atoi(c.Query("limit", "50")); err == nil && parsed > 0 && parsed <= 100 {
		limit = parsed
	}
	offset := 0
	if parsed, err := strconv.Atoi(c.Query("offset", "0")); err == nil && parsed >= 0 {
		offset = parsed
	}

	requests, total, err := h.getRegistrationService().ListPendingRegistrationRequests(c.Context(), orgID, limit, offset)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch registration requests",
		})
	}

	return c.JSON(fiber.Map{
		"requests": requests,
		"total":    total,
		"limit":    limit,
		"offset":   offset,
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

	// SECURITY (A3d-v): verify the registration request either belongs
	// to the caller's org or is unassigned (OrganizationID == nil). The
	// service ignores request.OrganizationID and stamps the new user
	// with the admin's org — fine for unassigned-claim flows, but a
	// cross-tenant write IDOR if the request was already tied to a
	// different org. Returns 404 with body {"error":"not found"} on
	// cross-tenant or non-existent ID.
	regReq, err := h.getRegistrationService().GetRegistrationRequest(c.Context(), requestID)
	if err != nil || regReq == nil {
		respondResourceNotFound(c)
		return nil
	}
	if regReq.OrganizationID != nil && *regReq.OrganizationID != orgID {
		respondResourceNotFound(c)
		return nil
	}

	// Approve registration request
	newUser, err := h.getRegistrationService().ApproveRegistrationRequest(c.Context(), requestID, adminID, orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to approve registration: %v", err),
		})
	}

	// Log audit
	h.getAuditService().LogAction(
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

	// SECURITY (A3d-v): see ApproveRegistrationRequest comment. The
	// reject path is even worse pre-fix — the service signature does
	// not take orgID at all, so any admin can reject any pending
	// request system-wide. Handler-layer guard closes it.
	regReq, err := h.getRegistrationService().GetRegistrationRequest(c.Context(), requestID)
	if err != nil || regReq == nil {
		respondResourceNotFound(c)
		return nil
	}
	if regReq.OrganizationID != nil && *regReq.OrganizationID != orgID {
		respondResourceNotFound(c)
		return nil
	}

	// Reject registration request
	if err := h.getRegistrationService().RejectRegistrationRequest(c.Context(), requestID, adminID, req.Reason); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to reject registration: %v", err),
		})
	}

	// Log audit
	h.getAuditService().LogAction(
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

	org, err := h.getAdminService().GetOrganizationSettings(c.Context(), orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch organization settings",
		})
	}

	// Log audit with settings viewed
	h.getAuditService().LogAction(
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
	allCount, acknowledgedCount, unacknowledgedCount, err := h.getAlertService().CountUnacknowledged(c.Context(), orgID)
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

// GetEnforcementSettings returns the enforcement settings for the organization
func (h *AdminHandler) GetEnforcementSettings(c fiber.Ctx) error {
	// Safe type assertion with error checking
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

	settings, err := h.getAdminService().GetEnforcementSettings(c.Context(), orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch enforcement settings",
		})
	}

	return c.JSON(settings)
}

// UpdateEnforcementSettingsRequest represents the request body for updating enforcement settings
type UpdateEnforcementSettingsRequest struct {
	EnforcementMode string `json:"enforcementMode" validate:"required"`
}

// UpdateEnforcementSettings updates the enforcement mode for the organization
func (h *AdminHandler) UpdateEnforcementSettings(c fiber.Ctx) error {
	// Safe type assertion with error checking
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

	var req UpdateEnforcementSettingsRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Convert string to EnforcementMode
	var mode domain.EnforcementMode
	switch req.EnforcementMode {
	case "strict":
		mode = domain.EnforcementModeStrict
	case "monitoring":
		mode = domain.EnforcementModeMonitoring
	default:
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid enforcement mode. Must be 'strict' or 'monitoring'",
		})
	}

	if err := h.getAdminService().UpdateEnforcementMode(c.Context(), orgID, mode); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Log audit
	h.getAuditService().LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionUpdate,
		"enforcement_settings",
		orgID,
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"newEnforcementMode": req.EnforcementMode,
		},
	)

	// Return updated settings
	settings, err := h.getAdminService().GetEnforcementSettings(c.Context(), orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch updated settings",
		})
	}

	return c.JSON(settings)
}

// isSuperAdmin checks if the given user is the super admin (first admin user created in the organization)
// Super admin is protected from deactivation and deletion to ensure system access
func (h *AdminHandler) isSuperAdmin(ctx context.Context, userID, orgID uuid.UUID) (bool, error) {
	// Get the user to check their role
	user, err := h.getAuthService().GetUserByID(ctx, userID)
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
	users, err := h.getAuthService().GetUsersByOrganization(ctx, orgID)
	if err != nil {
		return false, err
	}

	// Find all admin users and sort by created_at (oldest first)
	admins := make([]*domain.User, 0)
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
