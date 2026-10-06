package handlers

import (
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// callerSeesAuditRequestDetail reports whether the caller may read the network
// address, user agent and metadata stored in an audit record.
//
// PRIVACY: GET /agents/:id/audit-logs and GET /mcp-servers/:id/audit-logs are
// open to every principal in the organization, while /admin/audit-logs, which
// returns the same records, admits only admins. Those three members describe
// the colleague whose request wrote the record (where it came from, which
// client sent it, and whatever the action logged, emails included), so only a
// principal the admin routes admit gets them. A manager, member, viewer, API
// key or agent gets the record without them. Regression:
// audit_log_member_view_test.go.
func callerSeesAuditRequestDetail(c fiber.Ctx) bool {
	role, _ := c.Locals("role").(string)
	return role == string(domain.RoleAdmin)
}

// auditLogMemberView is domain.AuditLog without ipAddress, userAgent and
// metadata. The members are absent from the JSON, not empty.
type auditLogMemberView struct {
	ID             uuid.UUID          `json:"id"`
	OrganizationID uuid.UUID          `json:"organizationId"`
	UserID         *uuid.UUID         `json:"userId,omitempty"`
	AgentID        *uuid.UUID         `json:"agentId,omitempty"`
	Action         domain.AuditAction `json:"action"`
	ResourceType   string             `json:"resourceType"`
	ResourceID     uuid.UUID          `json:"resourceId"`
	Timestamp      time.Time          `json:"timestamp"`
	AgentName      string             `json:"agentName,omitempty"`
	UserName       string             `json:"userName,omitempty"`
}

// auditLogsForCaller returns logs unchanged for an admin and as
// auditLogMemberView records for every other principal.
func auditLogsForCaller(c fiber.Ctx, logs []*domain.AuditLog) interface{} {
	if callerSeesAuditRequestDetail(c) {
		return logs
	}
	views := make([]auditLogMemberView, 0, len(logs))
	for _, log := range logs {
		if log == nil {
			continue
		}
		views = append(views, auditLogMemberView{
			ID:             log.ID,
			OrganizationID: log.OrganizationID,
			UserID:         log.UserID,
			AgentID:        log.AgentID,
			Action:         log.Action,
			ResourceType:   log.ResourceType,
			ResourceID:     log.ResourceID,
			Timestamp:      log.Timestamp,
			AgentName:      log.AgentName,
			UserName:       log.UserName,
		})
	}
	return views
}
