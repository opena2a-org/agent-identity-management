package siem

import (
	"encoding/json"
	"strings"

	"github.com/google/uuid"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// AuditEventPrefix begins the event type of every audit log entry: the
// entry's action follows it, as in aim.audit.login.
const AuditEventPrefix = "aim.audit."

// AuditEventType is the event type of an audit log entry with the action.
func AuditEventType(action domain.AuditAction) EventType {
	return EventType(AuditEventPrefix + string(action))
}

// AuditSeverity is the severity of an audit log entry with the action.
// A honeytoken match and a replayed refresh token are high, as the alert a
// honeytoken match raises is. A refusal of a revoked session, of a credential
// request from one, or of an agent whose status does not let it act is
// medium. Every other action is info.
func AuditSeverity(action domain.AuditAction) string {
	switch action {
	case domain.AuditActionHoneytokenTriggered, domain.AuditActionRefreshTokenReuse:
		return SeverityHigh
	case domain.AuditActionVerificationRefused,
		domain.AuditActionCredentialMintRefused,
		domain.AuditActionRefreshSessionRevoked:
		return SeverityMedium
	default:
		return SeverityInfo
	}
}

// EventFromAuditLog is the SIEM event of an audit log entry. The entry's
// metadata is encoded when the event is made, so a later change to the
// entry's map does not change the event.
func EventFromAuditLog(entry *domain.AuditLog) *SIEMEvent {
	details := map[string]interface{}{
		"auditLogId":     entry.ID.String(),
		"organizationId": entry.OrganizationID.String(),
		"action":         string(entry.Action),
		"actorType":      string(entry.ActorType()),
		"resourceType":   entry.ResourceType,
		"resourceId":     entry.ResourceID.String(),
	}
	if entry.UserID != nil && *entry.UserID != uuid.Nil {
		details["userId"] = entry.UserID.String()
	}
	// The address and user agent are copied: a handler receives them as views
	// of the request buffer (c.Get, and c.IP behind a trusted proxy), which
	// the server rewrites when it reads the next request, and the forwarder
	// serializes the event after the request has ended. OnCreate runs inside
	// the request, so here the views are still the entry's own.
	if entry.IPAddress != "" {
		details["ipAddress"] = strings.Clone(entry.IPAddress)
	}
	if entry.UserAgent != "" {
		details["userAgent"] = strings.Clone(entry.UserAgent)
	}
	if len(entry.Metadata) > 0 {
		if raw, err := json.Marshal(entry.Metadata); err == nil {
			details["metadata"] = json.RawMessage(raw)
		}
	}

	event := &SIEMEvent{
		EventType: AuditEventType(entry.Action),
		Timestamp: entry.Timestamp.UTC(),
		Severity:  AuditSeverity(entry.Action),
		Details:   details,
		Source:    "opena2a-aim",
	}
	if entry.AgentID != nil && *entry.AgentID != uuid.Nil {
		event.AgentID = entry.AgentID.String()
	}
	return event
}
