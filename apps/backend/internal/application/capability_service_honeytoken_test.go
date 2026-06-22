package application

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Issue #293: MarkHoneytoken guards the zero-false-positive contract at mark time and
// CapabilityService.VerifyAction fires the same tripwire as the AgentService paths.

func newHoneytokenCapService() (*CapabilityService, *MockCapabilityRepoForService, *MockAgentRepoForCapability, *SharedMockAuditLogRepository, *MockAlertRepository) {
	capRepo := new(MockCapabilityRepoForService)
	agentRepo := new(MockAgentRepoForCapability)
	auditRepo := new(SharedMockAuditLogRepository)
	alertRepo := new(MockAlertRepository)
	svc := &CapabilityService{
		capabilityRepo: capRepo,
		agentRepo:      agentRepo,
		auditRepo:      auditRepo,
		alertRepo:      alertRepo,
	}
	return svc, capRepo, agentRepo, auditRepo, alertRepo
}

func TestCapabilityService_MarkHoneytoken_HappyPath(t *testing.T) {
	svc, capRepo, agentRepo, auditRepo, _ := newHoneytokenCapService()

	capID := uuid.New()
	agentID := uuid.New()
	orgID := uuid.New()
	cap := &domain.AgentCapability{ID: capID, AgentID: agentID, CapabilityType: "admin:exfiltrate"}

	capRepo.On("GetCapabilityByID", capID).Return(cap, nil)
	agentRepo.On("GetByID", agentID).Return(&domain.Agent{ID: agentID, OrganizationID: orgID, DisplayName: "decoy"}, nil)
	// Only the capability itself is active — no collision.
	capRepo.On("GetActiveCapabilitiesByAgentID", agentID).Return([]*domain.AgentCapability{cap}, nil)
	capRepo.On("SetHoneytoken", capID, true).Return(nil)
	auditRepo.On("Create", mock.AnythingOfType("*domain.AuditLog")).Return(nil)

	updated, err := svc.MarkHoneytoken(context.Background(), capID, true, nil)
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.True(t, updated.Honeytoken)
	capRepo.AssertCalled(t, "SetHoneytoken", capID, true)
	auditRepo.AssertNumberOfCalls(t, "Create", 1)
}

func TestCapabilityService_MarkHoneytoken_RejectsWildcard(t *testing.T) {
	svc, capRepo, agentRepo, _, _ := newHoneytokenCapService()

	capID := uuid.New()
	agentID := uuid.New()
	cap := &domain.AgentCapability{ID: capID, AgentID: agentID, CapabilityType: "file:*"}

	capRepo.On("GetCapabilityByID", capID).Return(cap, nil)
	agentRepo.On("GetByID", agentID).Return(&domain.Agent{ID: agentID, DisplayName: "decoy"}, nil)

	_, err := svc.MarkHoneytoken(context.Background(), capID, true, nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrHoneytokenNotAllowed)
	assert.Contains(t, err.Error(), "wildcard")
	capRepo.AssertNotCalled(t, "SetHoneytoken", mock.Anything, mock.Anything)
}

func TestCapabilityService_MarkHoneytoken_RejectsCollisionWithLiveGrant(t *testing.T) {
	svc, capRepo, agentRepo, _, _ := newHoneytokenCapService()

	capID := uuid.New()
	agentID := uuid.New()
	cap := &domain.AgentCapability{ID: capID, AgentID: agentID, CapabilityType: "file:read"}
	// A separate ACTIVE, non-honeytoken grant of the same type → marking would
	// make legitimate file:read traffic trip the alert.
	sibling := &domain.AgentCapability{ID: uuid.New(), AgentID: agentID, CapabilityType: "file:read"}

	capRepo.On("GetCapabilityByID", capID).Return(cap, nil)
	agentRepo.On("GetByID", agentID).Return(&domain.Agent{ID: agentID, DisplayName: "decoy"}, nil)
	capRepo.On("GetActiveCapabilitiesByAgentID", agentID).Return([]*domain.AgentCapability{cap, sibling}, nil)

	_, err := svc.MarkHoneytoken(context.Background(), capID, true, nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrHoneytokenNotAllowed)
	assert.Contains(t, err.Error(), "another active grant")
	capRepo.AssertNotCalled(t, "SetHoneytoken", mock.Anything, mock.Anything)
}

// Unmarking (honeytoken=false) must skip the guards so an operator can always clear
// the flag, even on a wildcard or colliding type.
func TestCapabilityService_MarkHoneytoken_UnmarkSkipsGuards(t *testing.T) {
	svc, capRepo, agentRepo, auditRepo, _ := newHoneytokenCapService()

	capID := uuid.New()
	agentID := uuid.New()
	cap := &domain.AgentCapability{ID: capID, AgentID: agentID, CapabilityType: "file:*", Honeytoken: true}

	capRepo.On("GetCapabilityByID", capID).Return(cap, nil)
	agentRepo.On("GetByID", agentID).Return(&domain.Agent{ID: agentID, DisplayName: "decoy"}, nil)
	capRepo.On("SetHoneytoken", capID, false).Return(nil)
	auditRepo.On("Create", mock.AnythingOfType("*domain.AuditLog")).Return(nil)

	updated, err := svc.MarkHoneytoken(context.Background(), capID, false, nil)
	require.NoError(t, err)
	assert.False(t, updated.Honeytoken)
	capRepo.AssertCalled(t, "SetHoneytoken", capID, false)
}

// VerifyAction (a verification path on CapabilityService) must raise the high-severity
// alert + audit when the request matches a honeytoken capability, without changing the
// authorization result.
func TestCapabilityService_VerifyAction_Honeytoken_RaisesAlertAndAudit(t *testing.T) {
	svc, capRepo, agentRepo, auditRepo, alertRepo := newHoneytokenCapService()

	agentID := uuid.New()
	orgID := uuid.New()
	honeyCapID := uuid.New()
	// PublicKey nil → signature check skipped. Honeytoken capability granted (in scope).
	agentRepo.On("GetByID", agentID).Return(&domain.Agent{
		ID: agentID, OrganizationID: orgID, DisplayName: "decoy", TrustScore: 0.9,
	}, nil)
	caps := []*domain.AgentCapability{
		{ID: honeyCapID, AgentID: agentID, CapabilityType: "admin:exfiltrate", Honeytoken: true},
	}
	capRepo.On("GetActiveCapabilitiesByAgentID", agentID).Return(caps, nil)
	// No capability definition → trust-threshold check is skipped.
	capRepo.On("GetCapabilityDefinition", "admin", "exfiltrate", mock.Anything).Return(nil, errors.New("not found"))
	agentRepo.On("Update", mock.AnythingOfType("*domain.Agent")).Return(nil)

	var capturedAlert *domain.Alert
	alertRepo.On("Create", mock.AnythingOfType("*domain.Alert")).Run(func(args mock.Arguments) {
		capturedAlert = args.Get(0).(*domain.Alert)
	}).Return(nil)
	auditRepo.On("Create", mock.AnythingOfType("*domain.AuditLog")).Return(nil)

	ip := "203.0.113.9"
	result, err := svc.VerifyAction(context.Background(), agentID, "admin:exfiltrate", nil, nil, &ip, nil)
	require.NoError(t, err)
	require.NotNil(t, result)
	// Decision preserved: a granted honeytoken still authorizes.
	assert.True(t, result.IsAuthorized, "honeytoken must not change the authorization result")

	alertRepo.AssertNumberOfCalls(t, "Create", 1)
	auditRepo.AssertNumberOfCalls(t, "Create", 1)
	require.NotNil(t, capturedAlert)
	assert.Equal(t, domain.AlertHoneytokenTriggered, capturedAlert.AlertType)
	assert.Equal(t, domain.AlertSeverityHigh, capturedAlert.Severity)
	assert.Equal(t, honeyCapID, capturedAlert.ResourceID)
	assert.Equal(t, ip, capturedAlert.SourceIP)
}
