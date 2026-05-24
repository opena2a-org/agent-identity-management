package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// fakeAuthorizer is a stand-in for FGAEngine that lets us drive the handler
// through every branch without booting a real DB / NanoMind daemon / ASC.
type fakeAuthorizer struct {
	gotRequest *application.FGARequest
	result     *application.FGAResult
	err        error
}

func (f *fakeAuthorizer) Authorize(ctx context.Context, req *application.FGARequest) (*application.FGAResult, error) {
	f.gotRequest = req
	return f.result, f.err
}

// permissiveAuthorizeAgentRepo returns an agent whose OrganizationID
// matches whatever the caller asks for via the orgID field. Used by
// the happy-path tests in this file to satisfy the LoadOwned gate
// added in the AuthorizeHandler tenant-scope fix; the cross-tenant
// regression test in authorize_handler_cross_tenant_test.go uses a
// separate stub that returns a foreign-org agent.
type permissiveAuthorizeAgentRepo struct {
	domain.AgentRepository
	orgID uuid.UUID
}

func (r *permissiveAuthorizeAgentRepo) GetByID(id uuid.UUID) (*domain.Agent, error) {
	return &domain.Agent{ID: id, OrganizationID: r.orgID}, nil
}

// newAuthorizeTestApp wires the handler under a Fiber app that sets
// Locals("organization_id") on every request. callerOrgID is the
// caller's tenant; the permissive agentRepo (constructed alongside
// the handler) returns agents in this same org so LoadOwned passes
// through to the FGAEngine branch the test wants to exercise.
func newAuthorizeTestApp(h *AuthorizeHandler, callerOrgID uuid.UUID) *fiber.App {
	app := fiber.New()
	app.Post("/api/v1/agents/:id/authorize", func(c fiber.Ctx) error {
		c.Locals("organization_id", callerOrgID)
		return h.Authorize(c)
	})
	return app
}

func TestAuthorize_AllowResult_Returns200WithBody(t *testing.T) {
	agentID := uuid.New()
	fake := &fakeAuthorizer{
		result: &application.FGAResult{
			Allowed:        true,
			Outcome:        "ALLOW",
			StepsTriggered: []string{"capability_check", "attribute_check", "context_check", "chain_check"},
			LatencyMs:      7,
		},
	}
	orgID := uuid.New()
	h := NewAuthorizeHandlerWithInterface(fake, &permissiveAuthorizeAgentRepo{orgID: orgID})
	app := newAuthorizeTestApp(h, orgID)

	body := strings.NewReader(`{"capability":"file:read","resource":"/tmp/x","action":"read"}`)
	req := httptest.NewRequest("POST", "/api/v1/agents/"+agentID.String()+"/authorize", body)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var got application.FGAResult
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
	assert.True(t, got.Allowed)
	assert.Equal(t, "ALLOW", got.Outcome)

	// Path AgentID is forwarded to the engine; body fields are forwarded as-is.
	require.NotNil(t, fake.gotRequest)
	assert.Equal(t, agentID, fake.gotRequest.AgentID)
	assert.Equal(t, "file:read", fake.gotRequest.Capability)
	assert.Equal(t, "/tmp/x", fake.gotRequest.Resource)
	assert.Equal(t, "read", fake.gotRequest.Action)
}

func TestAuthorize_DenyResult_Returns200WithDenyBody(t *testing.T) {
	agentID := uuid.New()
	fake := &fakeAuthorizer{
		result: &application.FGAResult{
			Allowed:        false,
			Outcome:        "DENY_INTENT",
			DeniedBy:       "intent_check",
			DeniedReason:   "Intent classified as prompt_injection (confidence 0.95)",
			StepsTriggered: []string{"capability_check", "attribute_check", "context_check", "chain_check", "intent_check_sync"},
			LatencyMs:      812,
		},
	}
	orgID := uuid.New()
	h := NewAuthorizeHandlerWithInterface(fake, &permissiveAuthorizeAgentRepo{orgID: orgID})
	app := newAuthorizeTestApp(h, orgID)

	body := strings.NewReader(`{"capability":"file:write"}`)
	req := httptest.NewRequest("POST", "/api/v1/agents/"+agentID.String()+"/authorize", body)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// DENY is HTTP 200 by design -- the request succeeded, the decision is in the body.
	// Returning 403 would conflate FGA decisions with auth failures and pollute trace status.
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var got application.FGAResult
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
	assert.False(t, got.Allowed)
	assert.Equal(t, "DENY_INTENT", got.Outcome)
	assert.Equal(t, "intent_check", got.DeniedBy)
}

func TestAuthorize_InvalidAgentID_Returns400(t *testing.T) {
	orgID := uuid.New()
	h := NewAuthorizeHandlerWithInterface(&fakeAuthorizer{}, &permissiveAuthorizeAgentRepo{orgID: orgID})
	app := newAuthorizeTestApp(h, orgID)

	body := strings.NewReader(`{"capability":"file:read"}`)
	req := httptest.NewRequest("POST", "/api/v1/agents/not-a-uuid/authorize", body)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestAuthorize_MissingCapability_Returns400(t *testing.T) {
	agentID := uuid.New()
	orgID := uuid.New()
	h := NewAuthorizeHandlerWithInterface(&fakeAuthorizer{}, &permissiveAuthorizeAgentRepo{orgID: orgID})
	app := newAuthorizeTestApp(h, orgID)

	body := strings.NewReader(`{"resource":"/tmp/x"}`)
	req := httptest.NewRequest("POST", "/api/v1/agents/"+agentID.String()+"/authorize", body)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestAuthorize_BodyAgentIDMismatch_Returns400(t *testing.T) {
	pathID := uuid.New()
	bodyID := uuid.New()
	orgID := uuid.New()
	h := NewAuthorizeHandlerWithInterface(&fakeAuthorizer{}, &permissiveAuthorizeAgentRepo{orgID: orgID})
	app := newAuthorizeTestApp(h, orgID)

	body := strings.NewReader(`{"agentId":"` + bodyID.String() + `","capability":"file:read"}`)
	req := httptest.NewRequest("POST", "/api/v1/agents/"+pathID.String()+"/authorize", body)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestAuthorize_InvalidJSON_Returns400(t *testing.T) {
	agentID := uuid.New()
	orgID := uuid.New()
	h := NewAuthorizeHandlerWithInterface(&fakeAuthorizer{}, &permissiveAuthorizeAgentRepo{orgID: orgID})
	app := newAuthorizeTestApp(h, orgID)

	body := strings.NewReader(`{not-json`)
	req := httptest.NewRequest("POST", "/api/v1/agents/"+agentID.String()+"/authorize", body)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestAuthorize_EngineError_Returns500WithErrorResultBody(t *testing.T) {
	agentID := uuid.New()
	// FGAEngine's deferred span finalizer synthesizes an ERROR result on infra
	// failure paths -- replicate that contract.
	fake := &fakeAuthorizer{
		result: &application.FGAResult{
			Outcome:      "ERROR",
			DeniedReason: "failed to load FGA policy: connection refused",
			LatencyMs:    3,
		},
		err: errors.New("failed to load FGA policy: connection refused"),
	}
	orgID := uuid.New()
	h := NewAuthorizeHandlerWithInterface(fake, &permissiveAuthorizeAgentRepo{orgID: orgID})
	app := newAuthorizeTestApp(h, orgID)

	body := strings.NewReader(`{"capability":"file:read"}`)
	req := httptest.NewRequest("POST", "/api/v1/agents/"+agentID.String()+"/authorize", body)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)

	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var got application.FGAResult
	require.NoError(t, json.Unmarshal(raw, &got))
	assert.Equal(t, "ERROR", got.Outcome)
	assert.Contains(t, got.DeniedReason, "connection refused")
}

func TestAuthorize_EngineErrorWithNilResult_Returns500WithErrorResponse(t *testing.T) {
	agentID := uuid.New()
	fake := &fakeAuthorizer{
		result: nil, // defensive: handler should still return a clean 500
		err:    errors.New("totally unexpected"),
	}
	orgID := uuid.New()
	h := NewAuthorizeHandlerWithInterface(fake, &permissiveAuthorizeAgentRepo{orgID: orgID})
	app := newAuthorizeTestApp(h, orgID)

	body := strings.NewReader(`{"capability":"file:read"}`)
	req := httptest.NewRequest("POST", "/api/v1/agents/"+agentID.String()+"/authorize", body)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}
