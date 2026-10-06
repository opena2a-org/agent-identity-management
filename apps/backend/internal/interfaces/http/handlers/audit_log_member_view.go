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
// PRIVACY: GET /agents/:id/audit-logs, GET /agents/:id/activity and
// GET /mcp-servers/:id/audit-logs are open to every principal in the
// organization, while /admin/audit-logs, which returns the same records,
// admits only admins. Those three members describe
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
// metadata. The members are absent from the JSON, not empty. actorType is
// derived from userId and agentId, which the view already carries.
type auditLogMemberView struct {
	ID             uuid.UUID             `json:"id"`
	OrganizationID uuid.UUID             `json:"organizationId"`
	UserID         *uuid.UUID            `json:"userId,omitempty"`
	AgentID        *uuid.UUID            `json:"agentId,omitempty"`
	Action         domain.AuditAction    `json:"action"`
	ResourceType   string                `json:"resourceType"`
	ResourceID     uuid.UUID             `json:"resourceId"`
	Timestamp      time.Time             `json:"timestamp"`
	AgentName      string                `json:"agentName,omitempty"`
	UserName       string                `json:"userName,omitempty"`
	ActorType      domain.AuditActorType `json:"actorType"`
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
			ActorType:      log.ActorType(),
		})
	}
	return views
}

// agentActivityDecisionMembers are the metadata members of an agent's own call
// that record AIM's decision on it: what the agent asked for, on what, at what
// risk and trust, and whether and why AIM refused it. The agent page builds its
// activity timeline and its refused-call findings from them.
var agentActivityDecisionMembers = []string{
	"actionType",
	"resource",
	"riskLevel",
	"trustScore",
	"autoApproved",
	"denialReason",
}

// agentActivityDecision returns the decision members of an activity record's
// metadata for a caller that does not see the record's request detail, or nil
// when the metadata holds none. Every other member (the context the agent sent
// with its call, anything the action logged about a person) is left out.
func agentActivityDecision(metadata map[string]interface{}) map[string]interface{} {
	var decision map[string]interface{}
	for _, member := range agentActivityDecisionMembers {
		value, ok := metadata[member]
		if !ok {
			continue
		}
		if decision == nil {
			decision = make(map[string]interface{}, len(agentActivityDecisionMembers))
		}
		decision[member] = value
	}
	return decision
}
