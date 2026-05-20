package application

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Regression coverage for defect #1 in the LF demo audit: SDK-driven agent
// re-registration must NOT silently mutate the agent's granted capability set
// in strict mode. The fix routes every new capability declared during
// UpdateAgent through CapabilityRequestService.CreateRequest, which encodes
// the mode-aware policy:
//
//   - monitoring mode: auto-approve, grant the capability, leave an audit row
//   - strict mode:     create a pending request, do NOT grant
//
// First-registration (CreateAgent) preserves baseline declaration semantics:
// declared caps are granted directly in BOTH modes, because first
// registration IS the baseline. The trust boundary protects against
// re-registration mutation, not against creation.
//
// These tests use the in-memory mocks already defined in
// capability_request_service_test.go (MockCapabilityRequestRepository,
// MockCapabilityRepoForCapReq, MockAgentRepositoryForCapReq,
// MockOrganizationRepositoryForCapReq) so they exercise the real wiring of
// CapabilityRequestService through AgentService.

func buildCapLockTestRig(
	t *testing.T,
	enforcement domain.EnforcementMode,
) (
	*AgentService,
	*MockAgentRepositoryForCapReq,
	*MockCapabilityRepoForCapReq,
	*MockCapabilityRequestRepository,
	*MockOrganizationRepositoryForCapReq,
	uuid.UUID, // orgID
) {
	t.Helper()

	agentRepo := NewMockAgentRepositoryForCapReq()
	capabilityRepo := NewMockCapabilityRepoForCapReq()
	requestRepo := NewMockCapabilityRequestRepository()
	orgRepo := NewMockOrganizationRepositoryForCapReq()

	orgID := uuid.New()
	orgRepo.AddOrg(&domain.Organization{
		ID:              orgID,
		Name:            "test-org",
		EnforcementMode: enforcement,
	})

	capRequestService := NewCapabilityRequestService(requestRepo, capabilityRepo, agentRepo, orgRepo)

	service := &AgentService{
		agentRepo:                agentRepo,
		capabilityRepo:           capabilityRepo,
		orgRepo:                  orgRepo,
		capabilityRequestService: capRequestService,
	}

	return service, agentRepo, capabilityRepo, requestRepo, orgRepo, orgID
}

// buildFirstRegisterTestRig wires AgentService with the real testify-mock
// AgentRepository + KeyVault + capabilityRepo so we can drive CreateAgent
// end-to-end and observe which caps actually land in storage. The OLD code
// had a `!isStrictMode` guard that silently dropped declared caps in strict
// mode; if that guard regresses, tests 1 + 2 FAIL because the capabilityRepo
// gets no Create calls in strict mode.
func buildFirstRegisterTestRig(
	t *testing.T,
	enforcement domain.EnforcementMode,
) (
	*AgentService,
	*MockAgentRepository,
	*MockCapabilityRepository,
	uuid.UUID,
) {
	t.Helper()

	mockAgentRepo := new(MockAgentRepository)
	mockTrustCalc := new(AgentServiceMockTrustScoreCalculator)
	mockTrustScoreRepo := new(AgentServiceMockTrustScoreRepository)
	mockCapabilityRepo := new(MockCapabilityRepository)
	orgRepo := NewMockOrganizationRepositoryForCapReq()

	masterKey := base64.StdEncoding.EncodeToString([]byte("test-master-key-32-bytes-long!!!"))
	keyVault, err := crypto.NewKeyVault(masterKey)
	require.NoError(t, err)

	orgID := uuid.New()
	orgRepo.AddOrg(&domain.Organization{
		ID:              orgID,
		Name:            "test-org",
		EnforcementMode: enforcement,
	})

	mockAgentRepo.On("Create", mock.AnythingOfType("*domain.Agent")).Return(nil)
	mockAgentRepo.On("Update", mock.AnythingOfType("*domain.Agent")).Return(nil)
	mockTrustCalc.On("Calculate", mock.AnythingOfType("*domain.Agent")).Return(&domain.TrustScore{ID: uuid.New(), Score: 0.5}, nil)
	mockTrustScoreRepo.On("Create", mock.AnythingOfType("*domain.TrustScore")).Return(nil)

	service := &AgentService{
		agentRepo:      mockAgentRepo,
		trustCalc:      mockTrustCalc,
		trustScoreRepo: mockTrustScoreRepo,
		keyVault:       keyVault,
		capabilityRepo: mockCapabilityRepo,
		orgRepo:        orgRepo,
	}

	return service, mockAgentRepo, mockCapabilityRepo, orgID
}

// Test 1: first-register STRICT mode → baseline caps granted directly via
// capabilityRepo.CreateCapability.
//
// Pre-fix behavior: CreateAgent's capability block was guarded by
// `&& !isStrictMode`, so strict-mode declarations were silently dropped and
// capabilityRepo.CreateCapability was NEVER called. This test asserts the
// CreateCapability mock receives one call per declared capability — proving
// the strict-mode guard is gone.
func TestCapabilityLock_FirstRegisterStrict_GrantsBaselineDirectly(t *testing.T) {
	service, mockAgentRepo, mockCapabilityRepo, orgID := buildFirstRegisterTestRig(t, domain.EnforcementModeStrict)

	mockCapabilityRepo.On("CreateCapability", mock.AnythingOfType("*domain.AgentCapability")).Return(nil)

	userID := uuid.New()
	req := &CreateAgentRequest{
		Name:         "test-cap-lock",
		DisplayName:  "Test Cap Lock",
		Description:  "Strict-mode first-registration baseline test",
		AgentType:    domain.AgentTypeAI,
		Version:      "1.0.0",
		PublicKey:    "Y2FwLWxvY2stdGVzdC1maXJzdC1zdHJpY3QtYmFzZWxpbmU=", // skip key generation
		Capabilities: []string{"flight:search", "flight:book"},
	}

	agent, err := service.CreateAgent(context.Background(), req, orgID, userID, nil, nil, "")
	require.NoError(t, err)
	require.NotNil(t, agent)

	mockCapabilityRepo.AssertNumberOfCalls(t, "CreateCapability", 2)
	mockCapabilityRepo.AssertCalled(t, "CreateCapability", mock.MatchedBy(func(c *domain.AgentCapability) bool {
		return c.CapabilityType == "flight:search"
	}))
	mockCapabilityRepo.AssertCalled(t, "CreateCapability", mock.MatchedBy(func(c *domain.AgentCapability) bool {
		return c.CapabilityType == "flight:book"
	}))
	mockAgentRepo.AssertCalled(t, "Create", mock.AnythingOfType("*domain.Agent"))
}

// Test 2: first-register MONITORING mode → baseline caps granted directly.
// Symmetric to test 1; preserves the existing monitoring behavior.
func TestCapabilityLock_FirstRegisterMonitoring_GrantsBaselineDirectly(t *testing.T) {
	service, _, mockCapabilityRepo, orgID := buildFirstRegisterTestRig(t, domain.EnforcementModeMonitoring)

	mockCapabilityRepo.On("CreateCapability", mock.AnythingOfType("*domain.AgentCapability")).Return(nil)

	userID := uuid.New()
	req := &CreateAgentRequest{
		Name:         "test-cap-lock",
		DisplayName:  "Test Cap Lock",
		Description:  "Monitoring-mode first-registration baseline test",
		AgentType:    domain.AgentTypeAI,
		Version:      "1.0.0",
		PublicKey:    "Y2FwLWxvY2stdGVzdC1maXJzdC1tb24tYmFzZWxpbmU=",
		Capabilities: []string{"flight:search", "flight:book"},
	}

	agent, err := service.CreateAgent(context.Background(), req, orgID, userID, nil, nil, "")
	require.NoError(t, err)
	require.NotNil(t, agent)

	mockCapabilityRepo.AssertNumberOfCalls(t, "CreateCapability", 2)
}

// Test 3: re-register monitoring mode + new cap → auto-approved request + cap
// granted. The CapabilityRequestService auto-approves in monitoring mode and
// grants the capability, leaving a capability_requests row (status=
// auto_approved) for audit trail.
func TestCapabilityLock_ReRegisterMonitoring_NewCap_AutoApprovesAndGrants(t *testing.T) {
	service, agentRepo, capabilityRepo, requestRepo, _, orgID := buildCapLockTestRig(t, domain.EnforcementModeMonitoring)

	agentID := uuid.New()
	requestedBy := uuid.New()
	agentRepo.AddAgent(&domain.Agent{
		ID:             agentID,
		OrganizationID: orgID,
		Name:           "test-cap-lock",
	})

	// Seed baseline caps.
	for _, capType := range []string{"flight:search", "flight:book"} {
		require.NoError(t, capabilityRepo.CreateCapability(&domain.AgentCapability{
			AgentID:        agentID,
			CapabilityType: capType,
		}))
	}

	req := &CreateAgentRequest{
		Capabilities: []string{"flight:search", "flight:book", "email:send"},
	}

	_, err := service.UpdateAgent(context.Background(), agentID, req, requestedBy)
	require.NoError(t, err)

	// New cap was granted.
	caps, err := capabilityRepo.GetActiveCapabilitiesByAgentID(agentID)
	require.NoError(t, err)
	assert.Len(t, caps, 3, "monitoring mode auto-grants the new capability")

	// Request row exists with status=auto_approved.
	requests, err := requestRepo.List(domain.CapabilityRequestFilter{AgentID: &agentID})
	require.NoError(t, err)
	require.Len(t, requests, 1, "monitoring mode still creates a capability_requests row for audit trail")
	assert.Equal(t, domain.CapabilityRequestStatusAutoApproved, requests[0].Status)
	assert.Equal(t, "email:send", requests[0].CapabilityType)
}

// Test 4: re-register strict mode + new cap → pending request, cap NOT
// granted. The before-fix behavior was a single log line and no recourse.
// After-fix the request is durable in capability_requests with status=
// pending, surfaced to admins, and the capability is denied until approval.
func TestCapabilityLock_ReRegisterStrict_NewCap_CreatesPendingRequestNoGrant(t *testing.T) {
	service, agentRepo, capabilityRepo, requestRepo, _, orgID := buildCapLockTestRig(t, domain.EnforcementModeStrict)

	agentID := uuid.New()
	requestedBy := uuid.New()
	agentRepo.AddAgent(&domain.Agent{
		ID:             agentID,
		OrganizationID: orgID,
		Name:           "test-cap-lock",
	})

	for _, capType := range []string{"flight:search", "flight:book"} {
		require.NoError(t, capabilityRepo.CreateCapability(&domain.AgentCapability{
			AgentID:        agentID,
			CapabilityType: capType,
		}))
	}

	req := &CreateAgentRequest{
		Capabilities: []string{"flight:search", "flight:book", "email:send"},
	}

	_, err := service.UpdateAgent(context.Background(), agentID, req, requestedBy)
	require.NoError(t, err)

	// New cap is NOT granted.
	caps, err := capabilityRepo.GetActiveCapabilitiesByAgentID(agentID)
	require.NoError(t, err)
	assert.Len(t, caps, 2, "strict mode does NOT grant the new capability; it requires admin approval")
	for _, c := range caps {
		assert.NotEqual(t, "email:send", c.CapabilityType, "email:send must remain ungranted in strict mode")
	}

	// Pending request row exists with the new cap.
	requests, err := requestRepo.List(domain.CapabilityRequestFilter{AgentID: &agentID})
	require.NoError(t, err)
	require.Len(t, requests, 1, "strict mode creates a pending capability_requests row")
	assert.Equal(t, domain.CapabilityRequestStatusPending, requests[0].Status)
	assert.Equal(t, "email:send", requests[0].CapabilityType)
	assert.Equal(t, requestedBy, requests[0].RequestedBy, "RequestedBy is the authenticated user driving the re-registration")
}

// Test 5: re-register strict mode + cap removal → cap revoked (preserved
// behavior). Removing a capability is the safe direction (reducing
// privilege); the audit doc deferred "should removal also require approval"
// to a follow-up.
func TestCapabilityLock_ReRegisterStrict_CapRemoval_RevokesPreservedBehavior(t *testing.T) {
	service, agentRepo, capabilityRepo, requestRepo, _, orgID := buildCapLockTestRig(t, domain.EnforcementModeStrict)

	agentID := uuid.New()
	requestedBy := uuid.New()
	agentRepo.AddAgent(&domain.Agent{
		ID:             agentID,
		OrganizationID: orgID,
		Name:           "test-cap-lock",
	})

	for _, capType := range []string{"flight:search", "flight:book"} {
		require.NoError(t, capabilityRepo.CreateCapability(&domain.AgentCapability{
			AgentID:        agentID,
			CapabilityType: capType,
		}))
	}

	req := &CreateAgentRequest{
		Capabilities: []string{"flight:search"}, // dropping flight:book
	}

	_, err := service.UpdateAgent(context.Background(), agentID, req, requestedBy)
	require.NoError(t, err)

	caps, err := capabilityRepo.GetActiveCapabilitiesByAgentID(agentID)
	require.NoError(t, err)
	require.Len(t, caps, 1, "removed cap should be revoked even in strict mode (safe direction)")
	assert.Equal(t, "flight:search", caps[0].CapabilityType)

	// No new request rows (no NEW caps; only removal).
	requests, err := requestRepo.List(domain.CapabilityRequestFilter{AgentID: &agentID})
	require.NoError(t, err)
	assert.Empty(t, requests, "cap removal does not create a capability_requests row in this PR (deferred)")
}

// Test 6: re-register strict mode + redeclare existing caps without
// additions → no-op for caps, no request row, no spurious grants.
func TestCapabilityLock_ReRegisterStrict_RedeclareUnchanged_NoOp(t *testing.T) {
	service, agentRepo, capabilityRepo, requestRepo, _, orgID := buildCapLockTestRig(t, domain.EnforcementModeStrict)

	agentID := uuid.New()
	requestedBy := uuid.New()
	agentRepo.AddAgent(&domain.Agent{
		ID:             agentID,
		OrganizationID: orgID,
		Name:           "test-cap-lock",
	})

	for _, capType := range []string{"flight:search", "flight:book"} {
		require.NoError(t, capabilityRepo.CreateCapability(&domain.AgentCapability{
			AgentID:        agentID,
			CapabilityType: capType,
		}))
	}

	req := &CreateAgentRequest{
		Capabilities: []string{"flight:search", "flight:book"},
	}

	_, err := service.UpdateAgent(context.Background(), agentID, req, requestedBy)
	require.NoError(t, err)

	caps, err := capabilityRepo.GetActiveCapabilitiesByAgentID(agentID)
	require.NoError(t, err)
	assert.Len(t, caps, 2)

	requests, err := requestRepo.List(domain.CapabilityRequestFilter{AgentID: &agentID})
	require.NoError(t, err)
	assert.Empty(t, requests)
}

// Test 7: nil capabilityRequestService produces an explicit error rather
// than silently bypassing the gate. The previous PR draft had a fallback
// path that defaulted to direct-grant when the service was nil; an
// adversarial review surfaced that as a strict-mode bypass surface (a future
// refactor or feature-flag could leave the service unset and silently
// re-enable the original defect). Hard fail is the correct behavior.
func TestCapabilityLock_NilService_FailsLoudly(t *testing.T) {
	agentRepo := NewMockAgentRepositoryForCapReq()
	capabilityRepo := NewMockCapabilityRepoForCapReq()
	orgRepo := NewMockOrganizationRepositoryForCapReq()

	orgID := uuid.New()
	orgRepo.AddOrg(&domain.Organization{
		ID:              orgID,
		Name:            "test-org",
		EnforcementMode: domain.EnforcementModeStrict,
	})

	agentID := uuid.New()
	agentRepo.AddAgent(&domain.Agent{
		ID:             agentID,
		OrganizationID: orgID,
		Name:           "test-no-service",
	})

	require.NoError(t, capabilityRepo.CreateCapability(&domain.AgentCapability{
		AgentID:        agentID,
		CapabilityType: "flight:search",
	}))

	service := &AgentService{
		agentRepo:                agentRepo,
		capabilityRepo:           capabilityRepo,
		orgRepo:                  orgRepo,
		capabilityRequestService: nil, // explicit nil — must NOT silently bypass
	}

	req := &CreateAgentRequest{
		Capabilities: []string{"flight:search", "email:send"},
	}

	_, err := service.UpdateAgent(context.Background(), agentID, req, uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "capabilityRequestService is required")

	// Existing capability remains intact; new cap NOT granted.
	caps, err := capabilityRepo.GetActiveCapabilitiesByAgentID(agentID)
	require.NoError(t, err)
	assert.Len(t, caps, 1)
	assert.Equal(t, "flight:search", caps[0].CapabilityType)
}
