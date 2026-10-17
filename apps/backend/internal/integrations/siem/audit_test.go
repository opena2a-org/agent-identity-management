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

func TestEventFromAuditLog_AgentEntry(t *testing.T) {
	agentID, userID := uuid.New(), uuid.New()
	entry := &domain.AuditLog{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		UserID:         &userID,
		AgentID:        &agentID,
		Action:         domain.AuditActionHoneytokenTriggered,
		ResourceType:   "agent",
		ResourceID:     agentID,
		IPAddress:      "203.0.113.7",
		UserAgent:      "aim-sdk/1.0",
		Metadata:       map[string]interface{}{"capability": "db:write", "nested": map[string]interface{}{"k": "v"}},
		Timestamp:      time.Date(2026, 10, 11, 12, 0, 0, 0, time.FixedZone("MDT", -6*3600)),
	}

	event := EventFromAuditLog(entry)
	// A later change to the entry's metadata does not reach the event.
	entry.Metadata["capability"] = "changed"
	entry.Metadata["nested"].(map[string]interface{})["k"] = "changed"

	assert.Equal(t, EventType("aim.audit.honeytoken_triggered"), event.EventType)
	assert.Equal(t, SeverityHigh, event.Severity)
	assert.Equal(t, agentID.String(), event.AgentID)
	assert.Equal(t, "opena2a-aim", event.Source)
	assert.Equal(t, time.UTC, event.Timestamp.Location())
	assert.True(t, event.Timestamp.Equal(entry.Timestamp))

	raw, err := json.Marshal(event)
	require.NoError(t, err)
	var got map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &got))
	details := got["details"].(map[string]interface{})
	assert.Equal(t, entry.ID.String(), details["auditLogId"])
	assert.Equal(t, entry.OrganizationID.String(), details["organizationId"])
	assert.Equal(t, "honeytoken_triggered", details["action"])
	assert.Equal(t, "agent", details["actorType"])
	assert.Equal(t, "agent", details["resourceType"])
	assert.Equal(t, agentID.String(), details["resourceId"])
	assert.Equal(t, userID.String(), details["userId"])
	assert.Equal(t, "203.0.113.7", details["ipAddress"])
	assert.Equal(t, "aim-sdk/1.0", details["userAgent"])
	assert.Equal(t, map[string]interface{}{"capability": "db:write", "nested": map[string]interface{}{"k": "v"}}, details["metadata"])
}

func TestEventFromAuditLog_SystemEntryOmitsAbsentFields(t *testing.T) {
	nilID := uuid.Nil
	entry := &domain.AuditLog{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		AgentID:        &nilID,
		Action:         domain.AuditActionCalculate,
		ResourceType:   "trust_score",
		ResourceID:     uuid.New(),
		Timestamp:      time.Now(),
	}

	event := EventFromAuditLog(entry)
	assert.Equal(t, "", event.AgentID, "a nil agent id is absent")
	assert.Equal(t, SeverityInfo, event.Severity)
	assert.Equal(t, "system", event.Details["actorType"])
	for _, key := range []string{"userId", "ipAddress", "userAgent", "metadata"} {
		_, ok := event.Details[key]
		assert.False(t, ok, "%s is absent", key)
	}
}

func TestAuditSeverity(t *testing.T) {
	cases := map[domain.AuditAction]string{
		domain.AuditActionHoneytokenTriggered:   SeverityHigh,
		domain.AuditActionRefreshTokenReuse:     SeverityHigh,
		domain.AuditActionVerificationRefused:   SeverityMedium,
		domain.AuditActionCredentialMintRefused: SeverityMedium,
		domain.AuditActionRefreshSessionRevoked: SeverityMedium,
		domain.AuditActionLogin:                 SeverityInfo,
		domain.AuditActionDelete:                SeverityInfo,
		domain.AuditActionHoneytokenMarked:      SeverityInfo,
	}
	for action, want := range cases {
		assert.Equal(t, want, AuditSeverity(action), "action %s", action)
	}
}

// An audit entry a handler writes carries the address and user agent as the
// handler received them, which may be views of the request buffer (see
// requestView). The forwarder serializes the event after the request has
// ended, when the server has read the next request into that buffer, so the
// event copies both out of the entry when it is built, inside the request.
func TestEventFromAuditLog_CopiesTheAddressAndUserAgentOutOfTheRequestBuffer(t *testing.T) {
	view := newRequestView(t, "203.0.113.70", "aim-sdk/1.0 (first request)")
	entry := &domain.AuditLog{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		Action:         domain.AuditActionLogin,
		ResourceType:   "user",
		ResourceID:     uuid.New(),
		IPAddress:      view.address,
		UserAgent:      view.userAgent,
		Timestamp:      time.Now(),
	}
	event := EventFromAuditLog(entry)

	view.overwrite()

	assert.Equal(t, "203.0.113.70", event.Details["ipAddress"])
	assert.Equal(t, "aim-sdk/1.0 (first request)", event.Details["userAgent"])
}
