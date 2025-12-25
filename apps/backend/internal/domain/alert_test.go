package domain

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAlertType_Constants(t *testing.T) {
	// Verify all alert type constants have expected values
	tests := []struct {
		constant AlertType
		expected string
	}{
		{AlertCertificateExpiring, "certificate_expiring"},
		{AlertAPIKeyExpiring, "api_key_expiring"},
		{AlertTrustScoreLow, "trust_score_low"},
		{AlertTrustScoreDrop, "trust_score_drop"},
		{AlertAgentOffline, "agent_offline"},
		{AlertSecurityBreach, "security_breach"},
		{AlertUnusualActivity, "unusual_activity"},
		{AlertTypeConfigurationDrift, "configuration_drift"},
		{AlertLowTrustAttestation, "low_trust_attestation"},
		{AlertLowTrustAttestationBlock, "low_trust_attestation_block"},
		{AlertMCPCapabilityDrift, "mcp_capability_drift"},
		{AlertAuthFailurePattern, "auth_failure_pattern"},
		{AlertAccountLocked, "account_locked"},
		{AlertBruteForceAttempt, "brute_force_attempt"},
		{AlertDataExfiltration, "data_exfiltration"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, AlertType(tt.expected), tt.constant)
		})
	}
}

func TestAlertSeverity_Constants(t *testing.T) {
	// Verify severity constants
	assert.Equal(t, AlertSeverity("info"), AlertSeverityInfo)
	assert.Equal(t, AlertSeverity("warning"), AlertSeverityWarning)
	assert.Equal(t, AlertSeverity("high"), AlertSeverityHigh)
	assert.Equal(t, AlertSeverity("critical"), AlertSeverityCritical)

	// Verify legacy constants for backward compatibility
	assert.Equal(t, AlertSeverityInfo, SeverityInfo)
	assert.Equal(t, AlertSeverityWarning, SeverityWarning)
	assert.Equal(t, AlertSeverityCritical, SeverityCritical)
}

func TestAlert_Structure(t *testing.T) {
	now := time.Now()
	orgID := uuid.New()
	resourceID := uuid.New()
	auditID := uuid.New()

	alert := Alert{
		ID:             uuid.New(),
		OrganizationID: orgID,
		AlertType:      AlertSecurityBreach,
		Severity:       AlertSeverityCritical,
		Title:          "Security Breach Detected",
		Description:    "Unauthorized access attempt detected from suspicious IP",
		ResourceType:   "agent",
		ResourceID:     resourceID,
		AuditID:        &auditID,
		AgentName:      "compromised-agent",
		SourceIP:       "192.168.1.100",
		Metadata: map[string]interface{}{
			"attempts":    5,
			"blocked":     true,
			"attack_type": "brute_force",
		},
		IsAcknowledged: false,
		CreatedAt:      now,
	}

	assert.NotEqual(t, uuid.Nil, alert.ID)
	assert.Equal(t, orgID, alert.OrganizationID)
	assert.Equal(t, AlertSecurityBreach, alert.AlertType)
	assert.Equal(t, AlertSeverityCritical, alert.Severity)
	assert.Equal(t, "Security Breach Detected", alert.Title)
	assert.Contains(t, alert.Description, "Unauthorized access")
	assert.Equal(t, "agent", alert.ResourceType)
	assert.Equal(t, resourceID, alert.ResourceID)
	assert.Equal(t, &auditID, alert.AuditID)
	assert.Equal(t, "compromised-agent", alert.AgentName)
	assert.Equal(t, "192.168.1.100", alert.SourceIP)
	assert.False(t, alert.IsAcknowledged)
	assert.Nil(t, alert.AcknowledgedBy)
	assert.Nil(t, alert.AcknowledgedAt)
}

func TestAlert_Acknowledge(t *testing.T) {
	now := time.Now()
	userID := uuid.New()
	ackTime := now.Add(10 * time.Minute)

	alert := Alert{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		AlertType:      AlertTrustScoreLow,
		Severity:       AlertSeverityWarning,
		Title:          "Low Trust Score Alert",
		Description:    "Agent trust score dropped below threshold",
		ResourceType:   "agent",
		ResourceID:     uuid.New(),
		IsAcknowledged: false,
		CreatedAt:      now,
	}

	// Initially not acknowledged
	assert.False(t, alert.IsAcknowledged)
	assert.Nil(t, alert.AcknowledgedBy)
	assert.Nil(t, alert.AcknowledgedAt)

	// Simulate acknowledgment
	alert.IsAcknowledged = true
	alert.AcknowledgedBy = &userID
	alert.AcknowledgedAt = &ackTime

	assert.True(t, alert.IsAcknowledged)
	assert.Equal(t, &userID, alert.AcknowledgedBy)
	assert.Equal(t, &ackTime, alert.AcknowledgedAt)
}

func TestAlert_CertificateExpiring(t *testing.T) {
	expiryDate := time.Now().Add(7 * 24 * time.Hour)

	alert := Alert{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		AlertType:      AlertCertificateExpiring,
		Severity:       AlertSeverityWarning,
		Title:          "Certificate Expiring Soon",
		Description:    "Agent certificate will expire in 7 days",
		ResourceType:   "agent",
		ResourceID:     uuid.New(),
		Metadata: map[string]interface{}{
			"expires_at":    expiryDate.Format(time.RFC3339),
			"days_until":    7,
			"cert_type":     "client",
			"serial_number": "1234567890",
		},
		IsAcknowledged: false,
		CreatedAt:      time.Now(),
	}

	assert.Equal(t, AlertCertificateExpiring, alert.AlertType)
	assert.Equal(t, AlertSeverityWarning, alert.Severity)
	assert.Equal(t, 7, alert.Metadata["days_until"])
}

func TestAlert_APIKeyExpiring(t *testing.T) {
	alert := Alert{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		AlertType:      AlertAPIKeyExpiring,
		Severity:       AlertSeverityInfo,
		Title:          "API Key Expiring",
		Description:    "API key will expire in 30 days",
		ResourceType:   "api_key",
		ResourceID:     uuid.New(),
		Metadata: map[string]interface{}{
			"key_name":   "production-key",
			"days_until": 30,
		},
		IsAcknowledged: false,
		CreatedAt:      time.Now(),
	}

	assert.Equal(t, AlertAPIKeyExpiring, alert.AlertType)
	assert.Equal(t, AlertSeverityInfo, alert.Severity)
	assert.Equal(t, "api_key", alert.ResourceType)
}

func TestAlert_TrustScoreDrop(t *testing.T) {
	alert := Alert{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		AlertType:      AlertTrustScoreDrop,
		Severity:       AlertSeverityHigh,
		Title:          "Significant Trust Score Drop",
		Description:    "Agent trust score dropped from 0.85 to 0.45",
		ResourceType:   "agent",
		ResourceID:     uuid.New(),
		AgentName:      "suspicious-agent",
		Metadata: map[string]interface{}{
			"previous_score": 0.85,
			"current_score":  0.45,
			"drop_percent":   47.1,
			"factors":        []string{"auth_failures", "unusual_behavior"},
		},
		IsAcknowledged: false,
		CreatedAt:      time.Now(),
	}

	assert.Equal(t, AlertTrustScoreDrop, alert.AlertType)
	assert.Equal(t, AlertSeverityHigh, alert.Severity)
	assert.Equal(t, 0.85, alert.Metadata["previous_score"])
	assert.Equal(t, 0.45, alert.Metadata["current_score"])
}

func TestAlert_AuthFailurePattern(t *testing.T) {
	alert := Alert{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		AlertType:      AlertAuthFailurePattern,
		Severity:       AlertSeverityHigh,
		Title:          "Authentication Failure Pattern Detected",
		Description:    "Multiple failed authentication attempts from same IP",
		ResourceType:   "user",
		ResourceID:     uuid.New(),
		SourceIP:       "10.0.0.50",
		Metadata: map[string]interface{}{
			"failure_count": 10,
			"time_window":   "5m",
			"user_agent":    "suspicious-bot/1.0",
		},
		IsAcknowledged: false,
		CreatedAt:      time.Now(),
	}

	assert.Equal(t, AlertAuthFailurePattern, alert.AlertType)
	assert.Equal(t, "10.0.0.50", alert.SourceIP)
	assert.Equal(t, 10, alert.Metadata["failure_count"])
}

func TestAlert_BruteForceAttempt(t *testing.T) {
	alert := Alert{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		AlertType:      AlertBruteForceAttempt,
		Severity:       AlertSeverityCritical,
		Title:          "Brute Force Attack Detected",
		Description:    "Potential brute force attack blocked",
		ResourceType:   "system",
		ResourceID:     uuid.New(),
		SourceIP:       "203.0.113.50",
		Metadata: map[string]interface{}{
			"blocked_at":     time.Now().Format(time.RFC3339),
			"total_attempts": 100,
			"unique_targets": 5,
		},
		IsAcknowledged: false,
		CreatedAt:      time.Now(),
	}

	assert.Equal(t, AlertBruteForceAttempt, alert.AlertType)
	assert.Equal(t, AlertSeverityCritical, alert.Severity)
	assert.Equal(t, "203.0.113.50", alert.SourceIP)
}

func TestAlert_DataExfiltration(t *testing.T) {
	alert := Alert{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		AlertType:      AlertDataExfiltration,
		Severity:       AlertSeverityCritical,
		Title:          "Potential Data Exfiltration Detected",
		Description:    "Large data transfer detected from agent",
		ResourceType:   "agent",
		ResourceID:     uuid.New(),
		AgentName:      "data-agent",
		SourceIP:       "10.0.0.100",
		Metadata: map[string]interface{}{
			"bytes_transferred": 1073741824, // 1GB
			"destination":       "external",
			"duration_seconds":  300,
		},
		IsAcknowledged: false,
		CreatedAt:      time.Now(),
	}

	assert.Equal(t, AlertDataExfiltration, alert.AlertType)
	assert.Equal(t, AlertSeverityCritical, alert.Severity)
	assert.Equal(t, 1073741824, alert.Metadata["bytes_transferred"])
}

func TestAlert_MCPCapabilityDrift(t *testing.T) {
	alert := Alert{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		AlertType:      AlertMCPCapabilityDrift,
		Severity:       AlertSeverityWarning,
		Title:          "MCP Capability Drift Detected",
		Description:    "MCP server capabilities have changed unexpectedly",
		ResourceType:   "mcp_server",
		ResourceID:     uuid.New(),
		Metadata: map[string]interface{}{
			"added_tools":     []string{"new_tool"},
			"removed_tools":   []string{"old_tool"},
			"server_endpoint": "https://mcp.example.com",
		},
		IsAcknowledged: false,
		CreatedAt:      time.Now(),
	}

	assert.Equal(t, AlertMCPCapabilityDrift, alert.AlertType)
	assert.Contains(t, alert.Description, "capabilities have changed")
}

func TestAlert_JSONSerialization(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	auditID := uuid.New()

	alert := Alert{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		AlertType:      AlertUnusualActivity,
		Severity:       AlertSeverityHigh,
		Title:          "Unusual Activity Detected",
		Description:    "Agent behavior deviates from baseline",
		ResourceType:   "agent",
		ResourceID:     uuid.New(),
		AuditID:        &auditID,
		AgentName:      "test-agent",
		SourceIP:       "192.168.1.1",
		Metadata: map[string]interface{}{
			"deviation_score": 0.75,
		},
		IsAcknowledged: false,
		CreatedAt:      now,
	}

	// Serialize to JSON
	data, err := json.Marshal(alert)
	require.NoError(t, err)

	// Deserialize back
	var restored Alert
	err = json.Unmarshal(data, &restored)
	require.NoError(t, err)

	// Verify key fields
	assert.Equal(t, alert.ID, restored.ID)
	assert.Equal(t, alert.AlertType, restored.AlertType)
	assert.Equal(t, alert.Severity, restored.Severity)
	assert.Equal(t, alert.Title, restored.Title)
	assert.Equal(t, alert.AgentName, restored.AgentName)
	assert.Equal(t, alert.IsAcknowledged, restored.IsAcknowledged)
}

func TestAlert_WithoutOptionalFields(t *testing.T) {
	alert := Alert{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		AlertType:      AlertAgentOffline,
		Severity:       AlertSeverityInfo,
		Title:          "Agent Offline",
		Description:    "Agent has not reported in",
		ResourceType:   "agent",
		ResourceID:     uuid.New(),
		IsAcknowledged: false,
		CreatedAt:      time.Now(),
	}

	// Optional fields should be nil/empty
	assert.Nil(t, alert.AuditID)
	assert.Empty(t, alert.AgentName)
	assert.Empty(t, alert.SourceIP)
	assert.Nil(t, alert.Metadata)
	assert.Nil(t, alert.AcknowledgedBy)
	assert.Nil(t, alert.AcknowledgedAt)
}

func TestAlert_LowTrustAttestation(t *testing.T) {
	alert := Alert{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		AlertType:      AlertLowTrustAttestation,
		Severity:       AlertSeverityWarning,
		Title:          "Low Trust Attestation Attempt",
		Description:    "Low-trust agent attempted MCP attestation",
		ResourceType:   "agent",
		ResourceID:     uuid.New(),
		AgentName:      "untrusted-agent",
		Metadata: map[string]interface{}{
			"trust_score": 0.3,
			"threshold":   0.5,
			"action":      "logged",
		},
		IsAcknowledged: false,
		CreatedAt:      time.Now(),
	}

	assert.Equal(t, AlertLowTrustAttestation, alert.AlertType)
	assert.Equal(t, float64(0.3), alert.Metadata["trust_score"])
}

func TestAlert_LowTrustAttestationBlock(t *testing.T) {
	alert := Alert{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		AlertType:      AlertLowTrustAttestationBlock,
		Severity:       AlertSeverityHigh,
		Title:          "Low Trust Attestation Blocked",
		Description:    "Low-trust agent blocked from MCP attestation",
		ResourceType:   "agent",
		ResourceID:     uuid.New(),
		AgentName:      "blocked-agent",
		Metadata: map[string]interface{}{
			"trust_score": 0.2,
			"threshold":   0.5,
			"action":      "blocked",
		},
		IsAcknowledged: false,
		CreatedAt:      time.Now(),
	}

	assert.Equal(t, AlertLowTrustAttestationBlock, alert.AlertType)
	assert.Equal(t, "blocked", alert.Metadata["action"])
}

func TestAlert_AccountLocked(t *testing.T) {
	alert := Alert{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		AlertType:      AlertAccountLocked,
		Severity:       AlertSeverityHigh,
		Title:          "Account Locked",
		Description:    "User account locked due to failed authentication attempts",
		ResourceType:   "user",
		ResourceID:     uuid.New(),
		SourceIP:       "192.168.1.50",
		Metadata: map[string]interface{}{
			"failed_attempts": 5,
			"lockout_duration": "30m",
			"unlock_at":        time.Now().Add(30 * time.Minute).Format(time.RFC3339),
		},
		IsAcknowledged: false,
		CreatedAt:      time.Now(),
	}

	assert.Equal(t, AlertAccountLocked, alert.AlertType)
	assert.Equal(t, AlertSeverityHigh, alert.Severity)
	assert.Equal(t, "user", alert.ResourceType)
}
