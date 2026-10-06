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

// A denied capability creates its alert with the capability-violation dedupe
// key, so repeats of the same agent, capability and resource coalesce on one
// open alert instead of creating an alert per request.
func TestAgentHandler_VerifyCapability_ViolationAlertCarriesDedupeKey(t *testing.T) {
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
				return false, "capability_not_granted", uuid.New(), nil
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

	require.Len(t, created, 2)
	assert.Equal(t, application.CapabilityViolationDedupeKey(agentID, "file:write", "/etc/passwd"), created[0].DedupeKey)
	assert.Equal(t, application.CapabilityViolationDedupeKey(agentID, "file:write", "/etc/shadow"), created[1].DedupeKey)
}
