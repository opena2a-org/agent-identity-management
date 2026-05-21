package handlers

import (
	"context"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockA2AReaderImpl implements the A2AReader interface for tests that
// need to exercise the tenant-ownership guard in A2AHandler.RevokeConsent
// and A2AHandler.UpdateTaskState without standing up an actual A2AService.
type MockA2AReaderImpl struct {
	GetConsentFunc func(ctx context.Context, consentID uuid.UUID) (*domain.A2AConsentRecord, error)
	GetA2ATaskFunc func(ctx context.Context, taskID uuid.UUID) (*domain.A2ATask, error)
}

func (m *MockA2AReaderImpl) GetConsent(ctx context.Context, consentID uuid.UUID) (*domain.A2AConsentRecord, error) {
	if m.GetConsentFunc != nil {
		return m.GetConsentFunc(ctx, consentID)
	}
	return nil, nil
}

func (m *MockA2AReaderImpl) GetA2ATask(ctx context.Context, taskID uuid.UUID) (*domain.A2ATask, error) {
	if m.GetA2ATaskFunc != nil {
		return m.GetA2ATaskFunc(ctx, taskID)
	}
	return nil, nil
}

// A3d-vii.b: a cross-tenant POST /a2a/consent/:id/revoke must return
// 404 with the existence-secrecy body, never disclose that the consent
// exists in another organization, and never invoke the mutation path.
//
// Captured-flag pattern (NOT t.Fatalf): fiber.App.Test runs the handler
// on a separate goroutine; t.Fatalf from the mock goroutine would call
// runtime.Goexit on THAT goroutine only and silently pass even if the
// LoadOwned guard were removed. Instead, the test leaves a2aService nil
// — the LoadOwned guard returns 404 before the handler can reach
// h.a2aService.RevokeConsent, so the nil pointer is never dereferenced.
// If a regression removes the guard, the test would either surface 200
// (mutation succeeded with no auth) or a 500 from the nil panic — never
// the 404 asserted here.
func TestA2AHandler_RevokeConsent_CrossTenantReturns404(t *testing.T) {
	consentID := uuid.New()
	callerOrgID := uuid.New()
	victimOrgID := uuid.New()

	loaderCalled := false
	mockReader := &MockA2AReaderImpl{
		GetConsentFunc: func(ctx context.Context, id uuid.UUID) (*domain.A2AConsentRecord, error) {
			loaderCalled = true
			return &domain.A2AConsentRecord{
				ID:             id,
				OrganizationID: &victimOrgID,
			}, nil
		},
	}

	handler := &A2AHandler{a2aReader: mockReader}

	app := fiber.New()
	app.Post("/a2a/consent/:id/revoke", func(c fiber.Ctx) error {
		c.Locals("organization_id", callerOrgID)
		c.Locals("user_id", uuid.New())
		return handler.RevokeConsent(c)
	})

	req := httptest.NewRequest("POST", "/a2a/consent/"+consentID.String()+"/revoke",
		strings.NewReader(`{"reason":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.JSONEq(t, `{"error":"not found"}`, string(body))
	assert.True(t, loaderCalled, "GetConsent must be called to load the consent record")
}

// A3d-vii.b: a consent record with no assigned OrganizationID
// (OrganizationID == nil) must NOT be revocable by any caller. The
// consentOrgID accessor returns uuid.Nil for an unassigned consent,
// which never matches a real caller's org → 404. This protects the
// unassigned-consent state from being weaponized as a wildcard mutation
// target.
func TestA2AHandler_RevokeConsent_UnassignedConsentReturns404(t *testing.T) {
	consentID := uuid.New()
	callerOrgID := uuid.New()

	mockReader := &MockA2AReaderImpl{
		GetConsentFunc: func(ctx context.Context, id uuid.UUID) (*domain.A2AConsentRecord, error) {
			return &domain.A2AConsentRecord{
				ID:             id,
				OrganizationID: nil, // unassigned
			}, nil
		},
	}

	handler := &A2AHandler{a2aReader: mockReader}

	app := fiber.New()
	app.Post("/a2a/consent/:id/revoke", func(c fiber.Ctx) error {
		c.Locals("organization_id", callerOrgID)
		c.Locals("user_id", uuid.New())
		return handler.RevokeConsent(c)
	})

	req := httptest.NewRequest("POST", "/a2a/consent/"+consentID.String()+"/revoke",
		strings.NewReader(`{"reason":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

// A3d-vii.b: a cross-tenant PUT /a2a/tasks/:id/state must return 404
// and never mutate the victim org's task. A2ATask has no direct
// OrganizationID; the LoadOwned guard runs on the task's ClientAgentID
// — if the client agent belongs to another org, the verdict is 404.
//
// RemoteAgentID is intentionally NOT scoped: A2A tasks span orgs by
// protocol design. The test reflects that by only mocking the
// ClientAgentID's owning org.
func TestA2AHandler_UpdateTaskState_CrossTenantReturns404(t *testing.T) {
	taskID := uuid.New()
	clientAgentID := uuid.New()
	callerOrgID := uuid.New()
	victimOrgID := uuid.New()

	taskLoaded := false
	mockReader := &MockA2AReaderImpl{
		GetA2ATaskFunc: func(ctx context.Context, id uuid.UUID) (*domain.A2ATask, error) {
			taskLoaded = true
			return &domain.A2ATask{
				ID:            id,
				ClientAgentID: clientAgentID,
				RemoteAgentID: uuid.New(),
			}, nil
		},
	}

	agentLoaded := false
	mockAgentService := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			agentLoaded = true
			return &domain.Agent{ID: id, OrganizationID: victimOrgID}, nil
		},
	}

	handler := &A2AHandler{a2aReader: mockReader, agentService: mockAgentService}

	app := fiber.New()
	app.Put("/a2a/tasks/:id/state", func(c fiber.Ctx) error {
		c.Locals("organization_id", callerOrgID)
		return handler.UpdateTaskState(c)
	})

	req := httptest.NewRequest("PUT", "/a2a/tasks/"+taskID.String()+"/state",
		strings.NewReader(`{"state":"FAILED","errorCode":"AUTH","errorMessage":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.JSONEq(t, `{"error":"not found"}`, string(body))
	assert.True(t, taskLoaded, "GetA2ATask must be called to load the task")
	assert.True(t, agentLoaded, "agentService.GetAgent must be called to verify ClientAgentID ownership")
}

// A3d-vii.b defense-in-depth: a caller whose Locals("organization_id")
// somehow surfaces uuid.Nil (auth-chain bug, corrupted JWT claim, etc.)
// must NOT be able to revoke an unassigned consent. Both sides being
// uuid.Nil would otherwise match in a naive equality check; LoadOwned
// rejects either side being uuid.Nil up front. Phase 4.5 finding
// MEDIUM-1.
func TestA2AHandler_RevokeConsent_NilCallerOrgIsRejected(t *testing.T) {
	consentID := uuid.New()

	mockReader := &MockA2AReaderImpl{
		GetConsentFunc: func(ctx context.Context, id uuid.UUID) (*domain.A2AConsentRecord, error) {
			return &domain.A2AConsentRecord{
				ID:             id,
				OrganizationID: nil, // unassigned consent
			}, nil
		},
	}

	handler := &A2AHandler{a2aReader: mockReader}

	app := fiber.New()
	app.Post("/a2a/consent/:id/revoke", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.Nil) // pathological caller org
		c.Locals("user_id", uuid.New())
		return handler.RevokeConsent(c)
	})

	req := httptest.NewRequest("POST", "/a2a/consent/"+consentID.String()+"/revoke",
		strings.NewReader(`{"reason":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

// A3d-vii.b: a PUT /a2a/tasks/:id/state for a taskID that does not
// exist must return 404. The handler must NOT continue past the
// load-or-404 fallback into the agent-ownership branch (which would
// hand a nil ClientAgentID to LoadOwned). The test confirms that
// not-found returns 404 cleanly.
func TestA2AHandler_UpdateTaskState_TaskNotFoundReturns404(t *testing.T) {
	taskID := uuid.New()
	callerOrgID := uuid.New()

	mockReader := &MockA2AReaderImpl{
		GetA2ATaskFunc: func(ctx context.Context, id uuid.UUID) (*domain.A2ATask, error) {
			return nil, nil
		},
	}

	handler := &A2AHandler{a2aReader: mockReader}

	app := fiber.New()
	app.Put("/a2a/tasks/:id/state", func(c fiber.Ctx) error {
		c.Locals("organization_id", callerOrgID)
		return handler.UpdateTaskState(c)
	})

	req := httptest.NewRequest("PUT", "/a2a/tasks/"+taskID.String()+"/state",
		strings.NewReader(`{"state":"FAILED"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.JSONEq(t, `{"error":"not found"}`, string(body))
}
