package domain

import (
	"time"

	"github.com/google/uuid"
)

// AuditAction represents the type of action logged
type AuditAction string

const (
	// Auth actions
	AuditActionLogin  AuditAction = "login"
	AuditActionLogout AuditAction = "logout"

	// Agent actions
	AuditActionCreate AuditAction = "create"
	AuditActionUpdate AuditAction = "update"
	AuditActionDelete AuditAction = "delete"
	AuditActionVerify AuditAction = "verify"
	AuditActionAttest AuditAction = "attest" // ✅ For agent attestation of MCPs
	AuditActionView   AuditAction = "view"

	// API Key actions
	AuditActionRevoke AuditAction = "revoke"

	// Trust score actions
	AuditActionCalculate AuditAction = "calculate"

	// Alert actions
	AuditActionAcknowledge AuditAction = "acknowledge"
	AuditActionResolve     AuditAction = "resolve"

	// Compliance actions
	AuditActionGenerate AuditAction = "generate"
	AuditActionExport   AuditAction = "export"
	AuditActionCheck    AuditAction = "check"

	// Webhook actions
	AuditActionTest AuditAction = "test"

	// Legacy constants for backward compatibility
	ActionLogin          AuditAction = "login"
	ActionLogout         AuditAction = "logout"
	ActionCreateAgent    AuditAction = "create_agent"
	ActionUpdateAgent    AuditAction = "update_agent"
	ActionDeleteAgent    AuditAction = "delete_agent"
	ActionVerifyAgent    AuditAction = "verify_agent"
	ActionCreateAPIKey   AuditAction = "create_api_key"
	ActionRevokeAPIKey   AuditAction = "revoke_api_key"
	ActionUpdateUserRole AuditAction = "update_user_role"
)

// AuditLog represents a logged action
type AuditLog struct {
	ID             uuid.UUID   `json:"id"`
	OrganizationID uuid.UUID   `json:"organization_id"`
	UserID         uuid.UUID   `json:"user_id"`
	Action         AuditAction `json:"action"`
	ResourceType   string      `json:"resource_type"` // agent, api_key, user, etc.
	ResourceID     uuid.UUID   `json:"resource_id"`
	IPAddress      string      `json:"ip_address"`
	UserAgent      string      `json:"user_agent"`
	Metadata       map[string]interface{} `json:"metadata"`
	Timestamp      time.Time   `json:"timestamp"`
}

// AuditLogRepository defines the interface for audit log persistence
type AuditLogRepository interface {
	Create(log *AuditLog) error
	GetByOrganization(orgID uuid.UUID, limit, offset int) ([]*AuditLog, error)
	GetByUser(userID uuid.UUID, limit, offset int) ([]*AuditLog, error)
	GetByResource(resourceType string, resourceID uuid.UUID) ([]*AuditLog, error)
	Search(query string, limit, offset int) ([]*AuditLog, error)

	// Security policy query methods
	CountActionsByAgentInTimeWindow(agentID uuid.UUID, action AuditAction, windowMinutes int) (int, error)
	GetRecentActionsByAgent(agentID uuid.UUID, limit int) ([]*AuditLog, error)
	GetAgentActionsByIPAddress(agentID uuid.UUID, ipAddress string, limit int) ([]*AuditLog, error)
}
