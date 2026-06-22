package application

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Issue #293: a verification request that matches a honeytoken capability must raise
// a high-severity alert and record an audit event, while leaving the authorization
// decision unchanged (indistinguishability). Normal traffic against non-honeytoken
// capabilities must produce zero alerts/audits (no false positives).

// HasCapability is the FGA-engine verification path. A honeytoken match raises the
// alert + audit and links them, and the boolean result is unchanged.
func TestAgentService_HasCapability_Honeytoken_RaisesHighSeverityAlertAndAudit(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)
	mockCapabilityRepo := new(MockCapabilityRepository)
	mockAlertRepo := new(MockAlertRepository)
	mockAuditRepo := new(SharedMockAuditLogRepository)

	service := &AgentService{
		agentRepo:      mockAgentRepo,
		capabilityRepo: mockCapabilityRepo,
		alertRepo:      mockAlertRepo,
		auditRepo:      mockAuditRepo,
	}

	agentID := uuid.New()
	orgID := uuid.New()
	honeytokenCapID := uuid.New()
	// The honeytoken is the second grant so the test also proves detection is not
	// short-circuited by an earlier non-honeytoken match.
	capabilities := []*domain.AgentCapability{
		{ID: uuid.New(), AgentID: agentID, CapabilityType: "file:read"},
		{ID: honeytokenCapID, AgentID: agentID, CapabilityType: "admin:exfiltrate", Honeytoken: true},
	}

	mockCapabilityRepo.On("GetActiveCapabilitiesByAgentID", agentID).Return(capabilities, nil)
	mockAgentRepo.On("GetByID", agentID).Return(
		&domain.Agent{ID: agentID, OrganizationID: orgID, DisplayName: "decoy-agent"}, nil)

	var capturedAlert *domain.Alert
	mockAlertRepo.On("Create", mock.AnythingOfType("*domain.Alert")).Run(func(args mock.Arguments) {
		capturedAlert = args.Get(0).(*domain.Alert)
	}).Return(nil)

	var capturedAudit *domain.AuditLog
	mockAuditRepo.On("Create", mock.AnythingOfType("*domain.AuditLog")).Run(func(args mock.Arguments) {
		capturedAudit = args.Get(0).(*domain.AuditLog)
	}).Return(nil)

	allowed, err := service.HasCapability(context.Background(), agentID, "admin:exfiltrate", "/secrets")
	require.NoError(t, err)

	// Decision preserved: a granted (honeytoken) capability still authorizes — the
	// tripwire must be indistinguishable from a real grant to a probing attacker.
	assert.True(t, allowed, "honeytoken must not change the allow/deny decision")

	// Exactly one high-severity alert of the honeytoken type.
	mockAlertRepo.AssertNumberOfCalls(t, "Create", 1)
	require.NotNil(t, capturedAlert)
	assert.Equal(t, domain.AlertHoneytokenTriggered, capturedAlert.AlertType)
	assert.Equal(t, domain.AlertSeverityHigh, capturedAlert.Severity)
	assert.Equal(t, orgID, capturedAlert.OrganizationID)
	assert.Equal(t, honeytokenCapID, capturedAlert.ResourceID)

	// Exactly one audit event, linked to the alert.
	mockAuditRepo.AssertNumberOfCalls(t, "Create", 1)
	require.NotNil(t, capturedAudit)
	assert.Equal(t, domain.AuditActionHoneytokenTriggered, capturedAudit.Action)
	assert.Equal(t, honeytokenCapID, capturedAudit.ResourceID)
	require.NotNil(t, capturedAlert.AuditID, "alert should link to the audit event")
	assert.Equal(t, capturedAudit.ID, *capturedAlert.AuditID)
}

// No false positives: a verification request that matches only non-honeytoken
// capabilities raises no alert, records no audit, and never even fetches the agent.
func TestAgentService_HasCapability_NonHoneytoken_NoAlertNoAudit(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)
	mockCapabilityRepo := new(MockCapabilityRepository)
	mockAlertRepo := new(MockAlertRepository)
	mockAuditRepo := new(SharedMockAuditLogRepository)

	service := &AgentService{
		agentRepo:      mockAgentRepo,
		capabilityRepo: mockCapabilityRepo,
		alertRepo:      mockAlertRepo,
		auditRepo:      mockAuditRepo,
	}

	agentID := uuid.New()
	capabilities := []*domain.AgentCapability{
		{ID: uuid.New(), AgentID: agentID, CapabilityType: "file:read"},
	}
	mockCapabilityRepo.On("GetActiveCapabilitiesByAgentID", agentID).Return(capabilities, nil)

	allowed, err := service.HasCapability(context.Background(), agentID, "file:read", "/test.txt")
	require.NoError(t, err)
	assert.True(t, allowed)

	mockAlertRepo.AssertNotCalled(t, "Create", mock.Anything)
	mockAuditRepo.AssertNotCalled(t, "Create", mock.Anything)
	mockAgentRepo.AssertNotCalled(t, "GetByID", mock.Anything)
}

// VerifyCapability is the primary /verify path. Detection runs before the
// status/enforcement branches, so the alert+audit fire even when the request is
// ultimately denied for an unrelated reason (here: the agent is compromised). This
// proves "any verification request against a honeytoken" is covered and that the
// honeytoken is not what changed the decision.
func TestAgentService_VerifyCapability_Honeytoken_FiresBeforeDecision(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)
	mockCapabilityRepo := new(MockCapabilityRepository)
	mockAlertRepo := new(MockAlertRepository)
	mockAuditRepo := new(SharedMockAuditLogRepository)

	service := &AgentService{
		agentRepo:      mockAgentRepo,
		capabilityRepo: mockCapabilityRepo,
		alertRepo:      mockAlertRepo,
		auditRepo:      mockAuditRepo,
	}

	agentID := uuid.New()
	orgID := uuid.New()
	honeytokenCapID := uuid.New()
	capabilities := []*domain.AgentCapability{
		{ID: honeytokenCapID, AgentID: agentID, CapabilityType: "admin:exfiltrate", Honeytoken: true},
	}

	// Verified (so the status branch is skipped) but compromised (so the request
	// returns at step 3 — after honeytoken detection — without reaching the policy
	// engine). Detection is independent of this outcome.
	mockAgentRepo.On("GetByID", agentID).Return(&domain.Agent{
		ID:             agentID,
		OrganizationID: orgID,
		DisplayName:    "decoy-agent",
		Status:         domain.AgentStatusVerified,
		IsCompromised:  true,
	}, nil)
	mockCapabilityRepo.On("GetActiveCapabilitiesByAgentID", agentID).Return(capabilities, nil)

	var capturedAlert *domain.Alert
	mockAlertRepo.On("Create", mock.AnythingOfType("*domain.Alert")).Run(func(args mock.Arguments) {
		capturedAlert = args.Get(0).(*domain.Alert)
	}).Return(nil)
	mockAuditRepo.On("Create", mock.AnythingOfType("*domain.AuditLog")).Return(nil)

	allowed, _, _, err := service.VerifyCapability(
		context.Background(), agentID, "admin:exfiltrate", "/secrets", nil, "203.0.113.7")
	require.NoError(t, err)
	assert.False(t, allowed, "decision is driven by the compromised flag, not the honeytoken")

	mockAlertRepo.AssertNumberOfCalls(t, "Create", 1)
	mockAuditRepo.AssertNumberOfCalls(t, "Create", 1)
	require.NotNil(t, capturedAlert)
	assert.Equal(t, domain.AlertHoneytokenTriggered, capturedAlert.AlertType)
	assert.Equal(t, domain.AlertSeverityHigh, capturedAlert.Severity)
	assert.Equal(t, "203.0.113.7", capturedAlert.SourceIP)
}

// HasCapabilityNoAlert is the non-detecting variant used for a secondary
// has-capability check inside a request that already ran detection (the SDK /verify
// handler calls VerifyCapability first). It must NOT fire the honeytoken tripwire,
// otherwise a single verification double-alerts.
func TestAgentService_HasCapabilityNoAlert_DoesNotFireHoneytoken(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)
	mockCapabilityRepo := new(MockCapabilityRepository)
	mockAlertRepo := new(MockAlertRepository)
	mockAuditRepo := new(SharedMockAuditLogRepository)

	service := &AgentService{
		agentRepo:      mockAgentRepo,
		capabilityRepo: mockCapabilityRepo,
		alertRepo:      mockAlertRepo,
		auditRepo:      mockAuditRepo,
	}

	agentID := uuid.New()
	capabilities := []*domain.AgentCapability{
		{ID: uuid.New(), AgentID: agentID, CapabilityType: "admin:exfiltrate", Honeytoken: true},
	}
	mockCapabilityRepo.On("GetActiveCapabilitiesByAgentID", agentID).Return(capabilities, nil)

	allowed, err := service.HasCapabilityNoAlert(context.Background(), agentID, "admin:exfiltrate", "/secrets")
	require.NoError(t, err)
	assert.True(t, allowed, "decision must match HasCapability")

	mockAlertRepo.AssertNotCalled(t, "Create", mock.Anything)
	mockAuditRepo.AssertNotCalled(t, "Create", mock.Anything)
	mockAgentRepo.AssertNotCalled(t, "GetByID", mock.Anything)
}
