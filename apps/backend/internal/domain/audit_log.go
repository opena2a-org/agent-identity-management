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
	ID             uuid.UUID              `json:"id"`
	OrganizationID uuid.UUID              `json:"organizationId"`
	UserID         *uuid.UUID             `json:"userId,omitempty"`  // Nullable for agent-initiated actions
	AgentID        *uuid.UUID             `json:"agentId,omitempty"` // For agent-initiated actions (attestations, etc.)
	Action         AuditAction            `json:"action"`
	ResourceType   string                 `json:"resourceType"` // agent, api_key, user, etc.
	ResourceID     uuid.UUID              `json:"resourceId"`
	IPAddress      string                 `json:"ipAddress"`
	UserAgent      string                 `json:"userAgent"`
	Metadata       map[string]interface{} `json:"metadata"`
	Timestamp      time.Time              `json:"timestamp"`

	// Populated via JOIN queries (not stored in DB)
	AgentName string `json:"agentName,omitempty"` // Name of agent that performed the action
	UserName  string `json:"userName,omitempty"`  // Name of user that performed the action
}

// AuditLogRepository defines the interface for audit log persistence.
//
// SECURITY: every list-style query that filters by a non-org-rooted
// key (user, resource, agent) takes the caller's orgID as the first
// parameter and filters by it in the SQL WHERE clause. Without this,
// any authenticated admin could probe other tenants by supplying the
// victim's UUID — the SQL primitives ran `WHERE user_id = $1`,
// `WHERE resource_type = $1 AND resource_id = $2`, and
// `WHERE agent_id = $1` with no org filter. See
// todo/2026-05-21-a3d-viii-followup-audit-log-query-idor.md.
type AuditLogRepository interface {
	Create(log *AuditLog) error
	GetByID(id uuid.UUID) (*AuditLog, error)
	GetByOrganization(orgID uuid.UUID, limit, offset int) ([]*AuditLog, error)
	GetByUser(orgID, userID uuid.UUID, limit, offset int) ([]*AuditLog, error)
	GetByResource(orgID uuid.UUID, resourceType string, resourceID uuid.UUID) ([]*AuditLog, error)
	Search(query string, limit, offset int) ([]*AuditLog, error)

	// Agent activity methods - logs actions performed BY an agent
	// within the caller's organization.
	GetByAgent(orgID, agentID uuid.UUID, limit, offset int) ([]*AuditLog, error)

	// Security policy query methods
	CountActionsByAgentInTimeWindow(agentID uuid.UUID, action AuditAction, windowMinutes int) (int, error)
	GetRecentActionsByAgent(agentID uuid.UUID, limit int) ([]*AuditLog, error)
	GetAgentActionsByIPAddress(agentID uuid.UUID, ipAddress string, limit int) ([]*AuditLog, error)
}
