package application

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

type capabilityCheckFixture struct {
	service      *AgentService
	agent        *domain.Agent
	alerts       *coalescingAlertRepo
	capabilities *MockCapabilityRepository
	grants       *mock.Call
}

// grant replaces the agent's granted capabilities with the given ones.
func (f *capabilityCheckFixture) grant(grants ...string) {
	f.grants.Unset()
	f.grants = f.capabilities.On("GetActiveCapabilitiesByAgentID", f.agent.ID).Return(activeGrants(f.agent.ID, grants), nil)
}

func activeGrants(agentID uuid.UUID, grants []string) []*domain.AgentCapability {
	var active []*domain.AgentCapability
	for _, grant := range grants {
		active = append(active, &domain.AgentCapability{ID: uuid.New(), AgentID: agentID, CapabilityType: grant})
	}
	return active
}

// withKey returns the stored alerts that carry key.
func (f *capabilityCheckFixture) withKey(key string) []*domain.Alert {
	var matched []*domain.Alert
	for _, a := range f.alerts.alerts {
		if a.DedupeKey == key {
			matched = append(matched, a)
		}
	}
	return matched
}

// newCapabilityCheckFixture builds an AgentService for a verified agent holding
// the given grants, in an organization with the given enforcement mode and no
// capability-violation policy, whose evaluator default is block + alert.
// Alerts are stored in a coalescingAlertRepo.
func newCapabilityCheckFixture(t *testing.T, mode domain.EnforcementMode, grants ...string) *capabilityCheckFixture {
	t.Helper()
	mockAgentRepo := new(MockAgentRepository)
	mockTrustScoreRepo := new(AgentServiceMockTrustScoreRepository)
	mockAlertRepo := new(MockAlertRepository)
	mockCapabilityRepo := new(MockCapabilityRepository)
	mockPolicyRepo := new(AgentServiceMockSecurityPolicyRepository)
	mockOrgRepo := new(MockOrganizationRepository)

	alerts := newCoalescingAlertRepo()
	alerts.AlertRepository = mockAlertRepo

	agent := &domain.Agent{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		Name:           "repeat-agent",
		DisplayName:    "Repeat Agent",
		Status:         domain.AgentStatusVerified,
		TrustScore:     0.8,
		CreatedAt:      time.Now().Add(-time.Hour),
		UpdatedAt:      time.Now(),
	}

	policyService := NewSecurityPolicyService(mockPolicyRepo, alerts, nil)
	policyService.SetAgentRepository(mockAgentRepo)
	policyService.SetVerificationEventRepo(statsMock(agent.ID, 2, 0))

	mockAgentRepo.On("GetByID", agent.ID).Return(agent, nil)
	grantsCall := mockCapabilityRepo.On("GetActiveCapabilitiesByAgentID", agent.ID).Return(activeGrants(agent.ID, grants), nil)
	mockOrgRepo.On("GetByID", agent.OrganizationID).Return(&domain.Organization{
		ID:              agent.OrganizationID,
		EnforcementMode: mode,
	}, nil)
	mockPolicyRepo.On("GetByType", agent.OrganizationID, domain.PolicyTypeCapabilityViolation).
		Return([]*domain.SecurityPolicy{}, nil)
	mockCapabilityRepo.On("CreateViolation", mock.AnythingOfType("*domain.CapabilityViolation")).Return(nil)
	mockAlertRepo.On("GetByOrganization", agent.OrganizationID, 50, 0).Return([]*domain.Alert{}, nil)
	mockAgentRepo.On("UpdateTrustScore", agent.ID, mock.AnythingOfType("float64")).Return(nil)
	mockTrustScoreRepo.On("UpdateScore", agent.ID, mock.AnythingOfType("float64")).Return(nil)
	mockAgentRepo.On("Update", mock.AnythingOfType("*domain.Agent")).Return(nil)

	return &capabilityCheckFixture{
		service: &AgentService{
			agentRepo:      mockAgentRepo,
			trustScoreRepo: mockTrustScoreRepo,
			alertRepo:      alerts,
			policyService:  policyService,
			capabilityRepo: mockCapabilityRepo,
			orgRepo:        mockOrgRepo,
		},
		agent:        agent,
		alerts:       alerts,
		capabilities: mockCapabilityRepo,
		grants:       grantsCall,
	}
}

// In monitoring mode an agent with no granted capabilities is allowed every
// action, and each one raises an "Unregistered Capability Used" alert. Three
// requests for the same capability and resource are one alert counted three
// times, not three alerts.
func TestAgentService_VerifyCapability_MonitoringNoGrants_RepeatsShareOneAlert(t *testing.T) {
	f := newCapabilityCheckFixture(t, domain.EnforcementModeMonitoring)

	for i := 0; i < 3; i++ {
		allowed, _, _, err := f.service.VerifyCapability(context.Background(), f.agent.ID, "file:write", "/etc/passwd", nil, "")
		require.NoError(t, err)
		require.True(t, allowed)
	}
	allowed, _, _, err := f.service.VerifyCapability(context.Background(), f.agent.ID, "file:write", "/etc/shadow", nil, "")
	require.NoError(t, err)
	require.True(t, allowed)

	require.Equal(t, 2, f.alerts.count(), "one alert per capability and resource")
	repeated := f.withKey(UnregisteredCapabilityDedupeKey(f.agent.ID, "file:write", "/etc/passwd"))
	require.Len(t, repeated, 1)
	assert.Contains(t, repeated[0].Title, "Unregistered Capability Used (Monitoring Mode)")
	assert.Equal(t, 3, f.alerts.occurrences[repeated[0].ID])
	require.Len(t, f.withKey(UnregisteredCapabilityDedupeKey(f.agent.ID, "file:write", "/etc/shadow")), 1)
}

// An agent with grants attempting a capability outside them raises a
// "Capability Violation Detected" alert. Repeats of the same capability and
// resource are counted on one alert.
func TestAgentService_VerifyCapability_ViolationRepeatsShareOneAlert(t *testing.T) {
	f := newCapabilityCheckFixture(t, domain.EnforcementModeMonitoring, "db:read")

	for i := 0; i < 3; i++ {
		_, _, _, err := f.service.VerifyCapability(context.Background(), f.agent.ID, "db:write", "orders", nil, "")
		require.NoError(t, err)
	}

	violations := f.withKey(CapabilityViolationDedupeKey(f.agent.ID, "db:write", "orders"))
	require.Len(t, violations, 1)
	assert.Contains(t, violations[0].Title, "Capability Violation Detected")
	assert.Equal(t, 3, f.alerts.occurrences[violations[0].ID])
	for _, a := range f.alerts.alerts {
		if a.DedupeKey == "" {
			assert.NotContains(t, a.Title, "Capability Violation", "a violation alert was stored without a dedupe key")
		}
	}
}

// In strict mode an agent with no granted capabilities is denied. The service
// raises no alert for that denial; the handler recognizes the reason with
// IsNoGrantedCapabilitiesDenial and raises it, so the reason the service gives
// must be one that predicate accepts.
func TestAgentService_VerifyCapability_StrictNoGrants_ReasonIsTheOneTheHandlerAlertsOn(t *testing.T) {
	f := newCapabilityCheckFixture(t, domain.EnforcementModeStrict)

	allowed, reason, _, err := f.service.VerifyCapability(context.Background(), f.agent.ID, "file:write", "/etc/passwd", nil, "")
	require.NoError(t, err)
	assert.False(t, allowed)
	assert.Equal(t, NoGrantedCapabilitiesDenialReason(domain.EnforcementModeStrict), reason)
	assert.True(t, IsNoGrantedCapabilitiesDenial(reason))
	assert.Zero(t, f.alerts.count(), "the handler raises this alert, not the service")

	assert.False(t, IsNoGrantedCapabilitiesDenial(
		"Action allowed by MONITORING mode (no capabilities registered yet) - logged for review. Policy: default_policy"))
	assert.False(t, IsNoGrantedCapabilitiesDenial(
		"Capability violation blocked by security policy 'default_policy': Agent does not have permission for capability 'file:write' (allowed: [db:read])"))
}

func TestCapabilityAlertDedupeKeys_DifferByAlert(t *testing.T) {
	agentID := uuid.New()
	keys := map[string]func(uuid.UUID, string, string) string{
		"capability violation":    CapabilityViolationDedupeKey,
		"unregistered capability": UnregisteredCapabilityDedupeKey,
		"no granted capabilities": NoGrantedCapabilitiesDedupeKey,
	}
	for nameA, keyA := range keys {
		for nameB, keyB := range keys {
			if nameA == nameB {
				continue
			}
			assert.NotEqual(t, keyA(agentID, "file:write", "/etc/passwd"), keyB(agentID, "file:write", "/etc/passwd"),
				"a %s alert is not counted on a %s alert", nameA, nameB)
		}
		assert.Equal(t, keyA(agentID, "file:write", "/etc/passwd"), keyA(agentID, "file:write", "/etc/passwd"), nameA)
		assert.NotEqual(t, keyA(agentID, "a:b", "c"), keyA(agentID, "a", "b:c"), nameA)
	}

	assert.True(t, IsNoGrantedCapabilitiesAlert(&domain.Alert{DedupeKey: NoGrantedCapabilitiesDedupeKey(agentID, "file:write", "/etc/passwd")}))
	assert.False(t, IsNoGrantedCapabilitiesAlert(&domain.Alert{DedupeKey: CapabilityViolationDedupeKey(agentID, "file:write", "/etc/passwd")}))
	assert.False(t, IsNoGrantedCapabilitiesAlert(&domain.Alert{DedupeKey: UnregisteredCapabilityDedupeKey(agentID, "file:write", "/etc/passwd")}))
	assert.False(t, IsNoGrantedCapabilitiesAlert(&domain.Alert{}))
}

// The review's scenario. A strict-mode organization denies an agent with no
// granted capabilities, and the handler raises its alert for that denial
// (here at Info, for a request that says its risk is low). Within the
// coalescing window the agent is granted one capability and attempts the same
// capability outside it. That violation raises its own High "Capability
// Violation Detected" alert; it is not counted on the no-grants alert.
func TestAgentService_VerifyCapability_StrictViolationAfterNoGrantsDenial_RaisesHighAlert(t *testing.T) {
	f := newCapabilityCheckFixture(t, domain.EnforcementModeStrict)
	alertService := NewAlertService(f.alerts, nil, nil)

	allowed, reason, _, err := f.service.VerifyCapability(context.Background(), f.agent.ID, "file:write", "/etc/passwd", nil, "")
	require.NoError(t, err)
	require.False(t, allowed)
	require.True(t, IsNoGrantedCapabilitiesDenial(reason))
	// The alert the capability-check handler raises for this denial;
	// TestAgentHandler_VerifyCapability_ViolationAlertCarriesDedupeKey pins its key.
	require.NoError(t, alertService.CreateAlert(context.Background(), &domain.Alert{
		OrganizationID: f.agent.OrganizationID,
		AlertType:      domain.AlertSecurityBreach,
		Severity:       domain.AlertSeverityInfo,
		Title:          "Capability Violation: " + f.agent.DisplayName + " attempted file:write",
		ResourceType:   "agent",
		ResourceID:     f.agent.ID,
		AgentName:      f.agent.DisplayName,
		DedupeKey:      NoGrantedCapabilitiesDedupeKey(f.agent.ID, "file:write", "/etc/passwd"),
	}))

	f.grant("db:read")
	allowed, _, _, err = f.service.VerifyCapability(context.Background(), f.agent.ID, "file:write", "/etc/passwd", nil, "")
	require.NoError(t, err)
	require.False(t, allowed)

	noGrants := f.withKey(NoGrantedCapabilitiesDedupeKey(f.agent.ID, "file:write", "/etc/passwd"))
	require.Len(t, noGrants, 1)
	assert.Equal(t, 1, f.alerts.occurrences[noGrants[0].ID], "the violation is not counted on the no-grants alert")

	violations := f.withKey(CapabilityViolationDedupeKey(f.agent.ID, "file:write", "/etc/passwd"))
	require.Len(t, violations, 1)
	assert.Equal(t, domain.AlertSeverityHigh, violations[0].Severity)
	assert.Contains(t, violations[0].Title, "Capability Violation Detected")
	assert.Equal(t, 1, f.alerts.occurrences[violations[0].ID])
}
