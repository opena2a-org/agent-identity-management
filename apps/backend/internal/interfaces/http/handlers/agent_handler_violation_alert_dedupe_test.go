package handlers

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A capability denied because the agent has no granted capabilities creates
// its alert with the no-granted-capabilities dedupe key, so repeats of the same
// agent, capability and resource coalesce on one open alert instead of
// creating an alert per request. The key is neither the one
// AgentService.VerifyCapability gives a violation of granted capabilities nor
// the one it gives a monitoring-mode action by an agent with no grants, so
// neither of those alerts is counted on this one. The stubbed reason is the
// one AgentService.VerifyCapability gives for that denial.
func TestAgentHandler_VerifyCapability_ViolationAlertCarriesDedupeKey(t *testing.T) {
	created := verifyCapabilityAlerts(t, application.NoGrantedCapabilitiesDenialReason(domain.EnforcementModeStrict))

	require.Len(t, created, 2)
	agentID := created[0].ResourceID
	assert.Equal(t, application.NoGrantedCapabilitiesDedupeKey(agentID, "file:write", "/etc/passwd"), created[0].DedupeKey)
	assert.Equal(t, application.NoGrantedCapabilitiesDedupeKey(agentID, "file:write", "/etc/shadow"), created[1].DedupeKey)
	for _, alert := range created {
		assert.True(t, application.IsNoGrantedCapabilitiesAlert(alert))
	}
	assert.NotEqual(t, application.CapabilityViolationDedupeKey(agentID, "file:write", "/etc/passwd"), created[0].DedupeKey)
	assert.NotEqual(t, application.UnregisteredCapabilityDedupeKey(agentID, "file:write", "/etc/passwd"), created[0].DedupeKey)
}

// A capability violation that a security policy blocks already has its alert,
// raised by AgentService.VerifyCapability; the handler raises no second one.
func TestAgentHandler_VerifyCapability_PolicyBlockedViolationRaisesNoSecondAlert(t *testing.T) {
	created := verifyCapabilityAlerts(t,
		"Capability violation blocked by security policy 'default_policy': Agent does not have permission for capability 'file:write' (allowed: [db:read])")

	assert.Empty(t, created)
}

// verifyCapabilityAlerts denies file:write on /etc/passwd and then on
// /etc/shadow with the given reason, and returns the alerts the handler
// created.
func verifyCapabilityAlerts(t *testing.T, reason string) []*domain.Alert {
	t.Helper()
	orgID, userID, agentID := uuid.New(), uuid.New(), uuid.New()

	var created []*domain.Alert
	handler := NewAgentHandlerWithInterfaces(
		&MockAgentServiceImpl{
			GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
				return &domain.Agent{
					ID:             agentID,
					OrganizationID: orgID,
					Name:           "test-agent",
					DisplayName:    "Test Agent",
					Status:         domain.AgentStatusVerified,
				}, nil
			},
			VerifyCapabilityFunc: func(ctx context.Context, agentID uuid.UUID, capability string, resource string, metadata map[string]interface{}, sourceIP string) (bool, string, uuid.UUID, error) {
				return false, reason, uuid.New(), nil
			},
		},
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{},
		&MockAPIKeyServiceImpl{},
		nil,
		&MockAlertServiceImpl{
			CreateAlertFunc: func(ctx context.Context, alert *domain.Alert) error {
				created = append(created, alert)
				return nil
			},
		},
		&MockVerificationEventServiceImpl{},
		&MockCapabilityServiceImpl{},
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		&MockMCPAttestationServiceImpl{},
	)

	app := createTestAppWithAuth(handler, orgID, userID)
	app.Post("/agents/:id/verify-capability", handler.VerifyCapability)

	deny := func(body string) {
		t.Helper()
		req := httptest.NewRequest("POST", "/agents/"+agentID.String()+"/verify-capability", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, fiber.StatusForbidden, resp.StatusCode)
	}

	deny(`{"capability":"file:write","resource":"/etc/passwd"}`)
	deny(`{"capability":"file:write","resource":"/etc/shadow"}`)

	return created
}
