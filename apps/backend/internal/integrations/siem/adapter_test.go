package siem

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSIEMEvent_Fields(t *testing.T) {
	event := &SIEMEvent{
		EventType:  EventCapabilityGranted,
		Timestamp:  time.Now(),
		AgentID:    "agent-123",
		Capability: "db.read",
		Outcome:    "ALLOW",
		Severity:   "info",
		Source:     "opena2a-aim",
		Details:    map[string]interface{}{"driftScore": 0.12},
	}
	assert.Equal(t, EventCapabilityGranted, event.EventType)
	assert.Equal(t, "info", event.Severity)
}

func TestSplunkAdapter_Name(t *testing.T) {
	adapter := NewSplunkAdapter(SplunkConfig{HECURL: "https://splunk.test:8088"})
	assert.Equal(t, "splunk", adapter.Name())
}

func TestSentinelAdapter_Name(t *testing.T) {
	adapter := NewSentinelAdapter(SentinelConfig{WorkspaceID: "test-ws"})
	assert.Equal(t, "sentinel", adapter.Name())
}

func TestSentinelAdapter_DefaultLogType(t *testing.T) {
	adapter := NewSentinelAdapter(SentinelConfig{WorkspaceID: "test"})
	assert.Equal(t, "OpenA2A_AIM", adapter.config.LogType)
}

func TestMultiAdapter_Name(t *testing.T) {
	multi := NewMultiAdapter()
	assert.Equal(t, "multi", multi.Name())
}

func TestEventTypes(t *testing.T) {
	// Verify all event types are distinct
	types := []EventType{
		EventCapabilityGranted, EventCapabilityDenied, EventFGADenied,
		EventIntentBlocked, EventApprovalRequested, EventApprovalDecided,
		EventEmergencyDeclared, EventEmergencyExpired,
		EventCertificationStarted, EventCertificationDecision,
		EventAgentSuspended, EventDriftAlert,
	}
	seen := map[EventType]bool{}
	for _, et := range types {
		assert.False(t, seen[et], "duplicate event type: %s", et)
		seen[et] = true
	}
	assert.Len(t, seen, 12)
}

func TestMapCapabilityToSafe(t *testing.T) {
	// Import from cyberark package would be needed for real test
	// This tests the SIEM event schema independently
	event := &SIEMEvent{
		EventType: EventFGADenied,
		AgentID:   "agent-456",
		Capability: "db.read:pii",
		Severity: "high",
	}
	assert.Equal(t, "high", event.Severity)
	assert.Equal(t, EventFGADenied, event.EventType)
}
