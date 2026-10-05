package handlers

import (
	"fmt"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
)

// assertReasonCode requires a refusal body to carry want in reasonCode, the API's
// refusal-code member, and the same value in code, the member it first shipped with.
func assertReasonCode(t *testing.T, body map[string]interface{}, want string) {
	t.Helper()
	assert.Equal(t, want, body["reasonCode"], "reasonCode: %v", body)
	assert.Equal(t, body["reasonCode"], body["code"], "code must equal reasonCode: %v", body)
}

// Every refusal that carries a machine-readable reason sends it as reasonCode and as code,
// with equal values, so a client reading either member reads the same reason. The wire
// values are spelled out: renaming one is a breaking change for clients.
func TestRefusals_CarryReasonCodeEqualToCode(t *testing.T) {
	t.Run("registration with no administrator", func(t *testing.T) {
		body := registrationErrorBody(application.ErrNoAdministrators, NoAdministratorsMessage)
		assertReasonCode(t, body, "noAdministrators")
	})

	t.Run("verification event write", func(t *testing.T) {
		app := fiber.New()
		app.Post("/api/v1/verification-events", (&VerificationEventHandler{}).CreateVerificationEvent)
		_, status, body := postJSON(t, app, "/api/v1/verification-events", map[string]interface{}{})
		assert.Equal(t, fiber.StatusForbidden, status)
		assertReasonCode(t, body, "verificationEventWriteNotAccepted")
	})

	agentID, orgID := uuid.New(), uuid.New()
	event := pendingEvent(agentID, orgID)
	spy := &writeSpy{}
	orgMissing := &MockOrganizationRepositoryerImpl{
		GetByIDFunc: func(id uuid.UUID) (*domain.Organization, error) { return nil, nil },
	}
	app := sdkWriteApp(newHandlerFor(spy.service(event), orgMissing), agentPrincipal("owning agent", agentID, orgID))

	t.Run("verification result write", func(t *testing.T) {
		_, status, body := postJSON(t, app,
			fmt.Sprintf("/api/v1/sdk-api/verifications/%s/result", event.ID),
			map[string]interface{}{"result": "success"})
		assert.Equal(t, fiber.StatusForbidden, status)
		assertReasonCode(t, body, "executionOutcomeNotAccepted")
	})

	t.Run("execution status with unknown enforcement mode", func(t *testing.T) {
		_, status, body := postJSON(t, app,
			fmt.Sprintf("/api/v1/sdk-api/verifications/%s/execution-status", event.ID),
			map[string]interface{}{"executed": false})
		assert.Equal(t, fiber.StatusServiceUnavailable, status)
		assertReasonCode(t, body, "enforcementModeUnavailable")
	})

	t.Run("registration error without a reason", func(t *testing.T) {
		body := registrationErrorBody(fmt.Errorf("boom"), "Registration failed")
		assert.NotContains(t, body, "reasonCode")
		assert.NotContains(t, body, "code")
	})
}
