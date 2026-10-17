package siem

import (
	"github.com/google/uuid"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// AlertSeverity is the SIEM severity of an alert with the severity: info,
// medium, high and critical for the alert severities info, warning, high and
// critical. Any other value is medium, so a SIEM that keeps medium and above
// does not lose an alert whose severity it does not know.
func AlertSeverity(severity domain.AlertSeverity) string {
	switch severity {
	case domain.AlertSeverityInfo:
		return SeverityInfo
	case domain.AlertSeverityHigh:
		return SeverityHigh
	case domain.AlertSeverityCritical:
		return SeverityCritical
	default:
		return SeverityMedium
	}
}

// alertAgentID is the agent the alert names: its resource when the resource
// is an agent, otherwise the agent its metadata names under agentId, as a
// honeytoken alert does. uuid.Nil when it names none.
func alertAgentID(alert *domain.Alert) uuid.UUID {
	if alert.ResourceType == "agent" && alert.ResourceID != uuid.Nil {
		return alert.ResourceID
	}
	if raw, ok := alert.Metadata["agentId"].(string); ok {
		if id, err := uuid.Parse(raw); err == nil {
			return id
		}
	}
	return uuid.Nil
}

// EventFromAlert is the SIEM event of an alert that was inserted: the
// alert's id, organization and type, its resource type and id when it has
// them, and the agent when the alert names one. The title, description and
// metadata are not carried.
func EventFromAlert(alert *domain.Alert) *SIEMEvent {
	details := map[string]interface{}{
		"alertId":        alert.ID.String(),
		"organizationId": alert.OrganizationID.String(),
		"alertType":      string(alert.AlertType),
	}
	if alert.ResourceType != "" {
		details["resourceType"] = alert.ResourceType
	}
	if alert.ResourceID != uuid.Nil {
		details["resourceId"] = alert.ResourceID.String()
	}

	event := &SIEMEvent{
		EventType: EventAlertCreated,
		Timestamp: alert.CreatedAt.UTC(),
		Severity:  AlertSeverity(alert.Severity),
		Details:   details,
		Source:    "opena2a-aim",
	}
	if agentID := alertAgentID(alert); agentID != uuid.Nil {
		event.AgentID = agentID.String()
	}
	return event
}
