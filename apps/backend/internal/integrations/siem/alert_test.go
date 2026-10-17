package siem

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

func TestEventFromAlert_ForAnAlertOnAnAgent(t *testing.T) {
	alertID, orgID, agentID, auditID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	at := time.Date(2026, 10, 16, 18, 4, 5, 6_000_000, time.FixedZone("MDT", -6*3600))

	event := EventFromAlert(&domain.Alert{
		ID:             alertID,
		OrganizationID: orgID,
		AlertType:      domain.AlertSecurityBreach,
		Severity:       domain.AlertSeverityHigh,
		Title:          "Capability Violation Detected: agent",
		Description:    "Agent 'agent' attempted unauthorized capability 'db:write'",
		ResourceType:   "agent",
		ResourceID:     agentID,
		AuditID:        &auditID,
		AgentName:      "agent",
		SourceIP:       "192.0.2.10",
		Metadata:       map[string]interface{}{"capability": "db:write"},
		CreatedAt:      at,
		DedupeKey:      "capability_violation:key",
	})

	assert.Equal(t, EventAlertCreated, event.EventType)
	assert.Equal(t, EventType("aim.alert.created"), event.EventType)
	assert.Equal(t, at.UTC(), event.Timestamp)
	assert.Equal(t, time.UTC, event.Timestamp.Location())
	assert.Equal(t, agentID.String(), event.AgentID)
	assert.Empty(t, event.Capability)
	assert.Empty(t, event.Outcome)
	assert.Equal(t, SeverityHigh, event.Severity)
	assert.Equal(t, "opena2a-aim", event.Source)
	assert.Equal(t, map[string]interface{}{
		"alertId":        alertID.String(),
		"organizationId": orgID.String(),
		"alertType":      "security_breach",
		"resourceType":   "agent",
		"resourceId":     agentID.String(),
	}, event.Details)
}

// A honeytoken alert's resource is the capability; the agent is named in its
// metadata.
func TestEventFromAlert_NamesTheAgentInTheMetadata(t *testing.T) {
	agentID, capabilityID := uuid.New(), uuid.New()

	event := EventFromAlert(&domain.Alert{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		AlertType:      domain.AlertHoneytokenTriggered,
		Severity:       domain.AlertSeverityHigh,
		ResourceType:   "capability",
		ResourceID:     capabilityID,
		Metadata:       map[string]interface{}{"agentId": agentID.String(), "capabilityId": capabilityID.String()},
		CreatedAt:      time.Now(),
	})

	assert.Equal(t, agentID.String(), event.AgentID)
	assert.Equal(t, "capability", event.Details["resourceType"])
	assert.Equal(t, capabilityID.String(), event.Details["resourceId"])
}

func TestEventFromAlert_WithoutAgentOrResource(t *testing.T) {
	alertID, orgID := uuid.New(), uuid.New()

	event := EventFromAlert(&domain.Alert{
		ID:             alertID,
		OrganizationID: orgID,
		AlertType:      domain.AlertUnusualActivity,
		Severity:       domain.AlertSeverityInfo,
		Metadata:       map[string]interface{}{"agentId": "not-a-uuid"},
		CreatedAt:      time.Now(),
	})

	assert.Empty(t, event.AgentID)
	assert.Equal(t, map[string]interface{}{
		"alertId":        alertID.String(),
		"organizationId": orgID.String(),
		"alertType":      "unusual_activity",
	}, event.Details)
}

// A resource that is not an agent is not taken for one.
func TestEventFromAlert_AnMCPServerResourceIsNotAnAgent(t *testing.T) {
	serverID := uuid.New()

	event := EventFromAlert(&domain.Alert{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		AlertType:      domain.AlertType("mcp_capability_drift"),
		Severity:       domain.AlertSeverityWarning,
		ResourceType:   "mcp_server",
		ResourceID:     serverID,
		CreatedAt:      time.Now(),
	})

	assert.Empty(t, event.AgentID)
	assert.Equal(t, serverID.String(), event.Details["resourceId"])
}

func TestAlertSeverity(t *testing.T) {
	for alert, want := range map[domain.AlertSeverity]string{
		domain.AlertSeverityInfo:     SeverityInfo,
		domain.AlertSeverityWarning:  SeverityMedium,
		domain.AlertSeverityHigh:     SeverityHigh,
		domain.AlertSeverityCritical: SeverityCritical,
		"":                           SeverityMedium,
		"severe":                     SeverityMedium,
	} {
		assert.Equal(t, want, AlertSeverity(alert), "alert severity %q", alert)
	}
}

// The event carries the alert's ids and type, never its title, description,
// source address or metadata.
func TestEventFromAlert_CarriesNoAlertText(t *testing.T) {
	event := EventFromAlert(&domain.Alert{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		AlertType:      domain.AlertSecurityBreach,
		Severity:       domain.AlertSeverityCritical,
		Title:          "title-text",
		Description:    "description-text",
		ResourceType:   "agent",
		ResourceID:     uuid.New(),
		AgentName:      "agent-name-text",
		SourceIP:       "198.51.100.7",
		Metadata:       map[string]interface{}{"resource": "metadata-text"},
		CreatedAt:      time.Now(),
	})

	raw, err := json.Marshal(event)
	require.NoError(t, err)
	for _, text := range []string{"title-text", "description-text", "agent-name-text", "198.51.100.7", "metadata-text"} {
		assert.NotContains(t, string(raw), text)
	}
}
