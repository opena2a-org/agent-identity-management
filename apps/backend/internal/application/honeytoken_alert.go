package application

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// raiseHoneytokenAlert records the audit event and the high-severity alert for a
// honeytoken verification hit (issue #293). Package-level so both the AgentService
// verification paths and CapabilityService.VerifyAction raise an identical tripwire.
// Mirrors the best-effort, nil-safe alert wiring used for MCP manifest drift (#275):
// write failures are logged and swallowed so capability verification is never blocked
// by alerting. The audit event is written first (when an audit repo is wired) so the
// alert can link to it through Alert.AuditID.
//
// This helper lives in its own file (not agent_service.go) on purpose: agent_service.go
// is protected from the AIM Cloud sync (the cloud keeps a divergent copy), so a symbol
// that synced files such as capability_service.go depend on must live in a synced file,
// or the cloud build fails with "undefined: raiseHoneytokenAlert".
func raiseHoneytokenAlert(
	alertRepo domain.AlertRepository,
	auditRepo domain.AuditLogRepository,
	agent *domain.Agent,
	cap *domain.AgentCapability,
	requestedCapability string,
	resource string,
	sourceIP string,
) {
	if agent == nil || cap == nil {
		return
	}

	// Audit event first so the alert can reference it.
	var auditID *uuid.UUID
	if auditRepo != nil {
		entry := &domain.AuditLog{
			ID:             uuid.New(),
			OrganizationID: agent.OrganizationID,
			AgentID:        &agent.ID,
			Action:         domain.AuditActionHoneytokenTriggered,
			ResourceType:   "capability",
			ResourceID:     cap.ID,
			IPAddress:      sourceIP,
			Metadata: map[string]interface{}{
				"honeytoken":           true,
				"capabilityId":         cap.ID.String(),
				"honeytokenCapability": cap.CapabilityType,
				"requestedCapability":  requestedCapability,
				"resource":             resource,
				"agentName":            agent.DisplayName,
			},
			Timestamp: time.Now().UTC(),
		}
		if err := auditRepo.Create(entry); err != nil {
			fmt.Printf("⚠️  Failed to record honeytoken audit event for capability %s: %v\n", cap.ID, err)
		} else {
			id := entry.ID
			auditID = &id
		}
	}

	if alertRepo == nil {
		return
	}
	alert := &domain.Alert{
		ID:             uuid.New(),
		OrganizationID: agent.OrganizationID,
		AlertType:      domain.AlertHoneytokenTriggered,
		Severity:       domain.AlertSeverityHigh,
		Title:          fmt.Sprintf("Honeytoken capability triggered: %s", agent.DisplayName),
		Description: fmt.Sprintf(
			"Agent '%s' issued a verification request against honeytoken capability '%s' "+
				"(requested: '%s', resource: '%s'). No legitimate workflow exercises a honeytoken, "+
				"so this is a high-confidence compromise indicator. The authorization decision was "+
				"left unchanged; this alert is a tripwire only.",
			agent.DisplayName, cap.CapabilityType, requestedCapability, resource),
		ResourceType: "capability",
		ResourceID:   cap.ID,
		AuditID:      auditID,
		AgentName:    agent.DisplayName,
		SourceIP:     sourceIP,
		Metadata: map[string]interface{}{
			"honeytoken":           true,
			"agentId":              agent.ID.String(),
			"capabilityId":         cap.ID.String(),
			"honeytokenCapability": cap.CapabilityType,
			"requestedCapability":  requestedCapability,
			"resource":             resource,
		},
		IsAcknowledged: false,
		CreatedAt:      time.Now().UTC(),
	}
	if err := alertRepo.Create(alert); err != nil {
		fmt.Printf("⚠️  Failed to create honeytoken alert for capability %s: %v\n", cap.ID, err)
		return
	}
	fmt.Printf("🚨 Honeytoken triggered: agent %s matched honeytoken capability %s (severity=high)\n",
		agent.DisplayName, cap.CapabilityType)
}
