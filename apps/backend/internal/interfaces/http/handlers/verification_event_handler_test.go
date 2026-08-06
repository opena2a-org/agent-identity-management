package handlers

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// verificationEventTestAgentRepo is a domain.AgentRepository stub that
// only implements GetByID. Mirrors secretsTestAgentRepo from A3d-iii.
type verificationEventTestAgentRepo struct {
	domain.AgentRepository
	getByID func(id uuid.UUID) (*domain.Agent, error)
}

func (r *verificationEventTestAgentRepo) GetByID(id uuid.UUID) (*domain.Agent, error) {
	return r.getByID(id)
}

// verificationEventTestMCPRepo is a domain.MCPServerRepository stub
// that only implements GetByID.
type verificationEventTestMCPRepo struct {
	domain.MCPServerRepository
	getByID func(id uuid.UUID) (*domain.MCPServer, error)
}

func (r *verificationEventTestMCPRepo) GetByID(id uuid.UUID) (*domain.MCPServer, error) {
	return r.getByID(id)
}

// verificationEventTestEventRepo is a domain.VerificationEventRepository
// stub that only implements GetByID + Delete.
type verificationEventTestEventRepo struct {
	domain.VerificationEventRepository
	getByID func(id uuid.UUID) (*domain.VerificationEvent, error)
}

func (r *verificationEventTestEventRepo) GetByID(id uuid.UUID) (*domain.VerificationEvent, error) {
	return r.getByID(id)
}

func (r *verificationEventTestEventRepo) Delete(id uuid.UUID) error {
	return nil
}

// Helper function for verification event tests that need org context
func withVerificationEventContext(handler func(c fiber.Ctx) error) fiber.Handler {
	return func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		c.Locals("user_id", uuid.New())
		return handler(c)
	}
}

// ===========================
// NewVerificationEventHandler Tests
// ===========================

func TestNewVerificationEventHandler_NilDeps(t *testing.T) {
	handler := NewVerificationEventHandler(nil, nil, nil)
	assert.NotNil(t, handler)
}

// ===========================
// VerificationEventHandler.ListVerificationEvents Tests
// ===========================

func TestVerificationEventHandler_ListVerificationEvents_NoOrgContext(t *testing.T) {
	handler := &VerificationEventHandler{}
	app := fiber.New()
	app.Get("/verification-events", handler.ListVerificationEvents)

	req := httptest.NewRequest("GET", "/verification-events", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestVerificationEventHandler_ListVerificationEvents_InvalidAgentID(t *testing.T) {
	handler := &VerificationEventHandler{}
	app := fiber.New()
	app.Get("/verification-events", withVerificationEventContext(handler.ListVerificationEvents))

	req := httptest.NewRequest("GET", "/verification-events?agent_id=not-a-uuid", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// VerificationEventHandler.GetVerificationEvent Tests
// ===========================

func TestVerificationEventHandler_GetVerificationEvent_NoOrgContext(t *testing.T) {
	handler := &VerificationEventHandler{}
	app := fiber.New()
	app.Get("/verification-events/:id", handler.GetVerificationEvent)

	req := httptest.NewRequest("GET", "/verification-events/"+uuid.New().String(), nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestVerificationEventHandler_GetVerificationEvent_InvalidID(t *testing.T) {
	handler := &VerificationEventHandler{}
	app := fiber.New()
	app.Get("/verification-events/:id", withVerificationEventContext(handler.GetVerificationEvent))

	req := httptest.NewRequest("GET", "/verification-events/not-a-uuid", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// VerificationEventHandler.CreateVerificationEvent Tests
// ===========================

func TestVerificationEventHandler_CreateVerificationEvent_NoOrgContext(t *testing.T) {
	handler := &VerificationEventHandler{}
	app := fiber.New()
	app.Post("/verification-events", handler.CreateVerificationEvent)

	body := `{"agentId":"test"}`
	req := httptest.NewRequest("POST", "/verification-events", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestVerificationEventHandler_CreateVerificationEvent_InvalidJSON(t *testing.T) {
	handler := &VerificationEventHandler{}
	app := fiber.New()
	app.Post("/verification-events", withVerificationEventContext(handler.CreateVerificationEvent))

	req := httptest.NewRequest("POST", "/verification-events", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestVerificationEventHandler_CreateVerificationEvent_InvalidAgentID(t *testing.T) {
	handler := &VerificationEventHandler{}
	app := fiber.New()
	app.Post("/verification-events", withVerificationEventContext(handler.CreateVerificationEvent))

	body := `{"agentId":"not-a-uuid"}`
	req := httptest.NewRequest("POST", "/verification-events", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// VerificationEventHandler.GetRecentEvents Tests
// ===========================

func TestVerificationEventHandler_GetRecentEvents_NoOrgContext(t *testing.T) {
	handler := &VerificationEventHandler{}
	app := fiber.New()
	app.Get("/verification-events/recent", handler.GetRecentEvents)

	req := httptest.NewRequest("GET", "/verification-events/recent", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

// ===========================
// VerificationEventHandler.GetStatistics Tests
// ===========================

func TestVerificationEventHandler_GetStatistics_NoOrgContext(t *testing.T) {
	handler := &VerificationEventHandler{}
	app := fiber.New()
	app.Get("/verification-events/statistics", handler.GetStatistics)

	req := httptest.NewRequest("GET", "/verification-events/statistics", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

// ===========================
// VerificationEventHandler.GetAgentVerificationEvents Tests
// ===========================

func TestVerificationEventHandler_GetAgentVerificationEvents_NoOrgContext(t *testing.T) {
	handler := &VerificationEventHandler{}
	app := fiber.New()
	app.Get("/verification-events/agent/:agent_id", handler.GetAgentVerificationEvents)

	req := httptest.NewRequest("GET", "/verification-events/agent/"+uuid.New().String(), nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestVerificationEventHandler_GetAgentVerificationEvents_InvalidAgentID(t *testing.T) {
	handler := &VerificationEventHandler{}
	app := fiber.New()
	app.Get("/verification-events/agent/:agent_id", withVerificationEventContext(handler.GetAgentVerificationEvents))

	req := httptest.NewRequest("GET", "/verification-events/agent/not-a-uuid", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// VerificationEventHandler.GetMCPVerificationEvents Tests
// ===========================

func TestVerificationEventHandler_GetMCPVerificationEvents_NoOrgContext(t *testing.T) {
	handler := &VerificationEventHandler{}
	app := fiber.New()
	app.Get("/verification-events/mcp/:mcp_id", handler.GetMCPVerificationEvents)

	req := httptest.NewRequest("GET", "/verification-events/mcp/"+uuid.New().String(), nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestVerificationEventHandler_GetMCPVerificationEvents_InvalidMCPID(t *testing.T) {
	handler := &VerificationEventHandler{}
	app := fiber.New()
	app.Get("/verification-events/mcp/:mcp_id", withVerificationEventContext(handler.GetMCPVerificationEvents))

	req := httptest.NewRequest("GET", "/verification-events/mcp/not-a-uuid", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// VerificationEventHandler.GetVerificationStats Tests
// ===========================

func TestVerificationEventHandler_GetVerificationStats_NoOrgContext(t *testing.T) {
	handler := &VerificationEventHandler{}
	app := fiber.New()
	app.Get("/verification-events/stats", handler.GetVerificationStats)

	req := httptest.NewRequest("GET", "/verification-events/stats", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

// ===========================
// VerificationEventHandler.DeleteVerificationEvent Tests
// ===========================

func TestVerificationEventHandler_DeleteVerificationEvent_NoOrgContext(t *testing.T) {
	handler := &VerificationEventHandler{}
	app := fiber.New()
	app.Delete("/verification-events/:id", handler.DeleteVerificationEvent)

	req := httptest.NewRequest("DELETE", "/verification-events/"+uuid.New().String(), nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestVerificationEventHandler_DeleteVerificationEvent_InvalidID(t *testing.T) {
	handler := &VerificationEventHandler{}
	app := fiber.New()
	app.Delete("/verification-events/:id", withVerificationEventContext(handler.DeleteVerificationEvent))

	req := httptest.NewRequest("DELETE", "/verification-events/not-a-uuid", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// getOrganizationID Helper Tests
// ===========================

func TestGetOrganizationID_Success(t *testing.T) {
	app := fiber.New()
	expectedOrgID := uuid.New()

	app.Get("/test", func(c fiber.Ctx) error {
		c.Locals("organization_id", expectedOrgID)
		orgID, err := getOrganizationID(c)
		assert.NoError(t, err)
		assert.Equal(t, expectedOrgID, orgID)
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestGetOrganizationID_MissingContext(t *testing.T) {
	app := fiber.New()

	app.Get("/test", func(c fiber.Ctx) error {
		_, err := getOrganizationID(c)
		assert.Error(t, err)
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestGetOrganizationID_WrongType(t *testing.T) {
	app := fiber.New()

	app.Get("/test", func(c fiber.Ctx) error {
		c.Locals("organization_id", "not-a-uuid") // Wrong type
		_, err := getOrganizationID(c)
		assert.Error(t, err)
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

// ===========================
// A3d-iv: cross-tenant access on path-id VerificationEventHandler
// routes must return 404 with existence-secrecy body. Each row
// exercises the LoadOwned guard wired in this PR. The mock repos
// return resources in a different org; the LoadOwned gate fires
// before any further service dispatch. Same nil-deref-as-proof
// pattern as PR #150 / #152 / #180 / #181.
// ===========================

func TestVerificationEventHandler_CrossOrgReturns404(t *testing.T) {
	callerOrgID := uuid.New()
	differentOrgID := uuid.New()
	eventID := uuid.New()
	agentID := uuid.New()
	mcpServerID := uuid.New()

	// A real service backed by a stub eventRepo that returns events in
	// the FOREIGN org. The handler's LoadOwned must short-circuit and
	// never reach the service's other repos (which are nil).
	eventRepo := &verificationEventTestEventRepo{
		getByID: func(id uuid.UUID) (*domain.VerificationEvent, error) {
			return &domain.VerificationEvent{
				ID:             eventID,
				OrganizationID: differentOrgID,
				StartedAt:      time.Now(),
			}, nil
		},
	}
	svc := application.NewVerificationEventService(eventRepo, nil, nil)

	agentRepo := &verificationEventTestAgentRepo{
		getByID: func(id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{ID: agentID, OrganizationID: differentOrgID, Name: "victim-agent"}, nil
		},
	}
	mcpRepo := &verificationEventTestMCPRepo{
		getByID: func(id uuid.UUID) (*domain.MCPServer, error) {
			return &domain.MCPServer{ID: mcpServerID, OrganizationID: differentOrgID, Name: "victim-mcp"}, nil
		},
	}
	handler := NewVerificationEventHandler(svc, agentRepo, mcpRepo)

	tests := []struct {
		name        string
		method      string
		requestPath string
		setup       func(*fiber.App, *VerificationEventHandler)
	}{
		{
			name:        "GetVerificationEvent",
			method:      "GET",
			requestPath: "/verification-events/" + eventID.String(),
			setup: func(app *fiber.App, h *VerificationEventHandler) {
				app.Get("/verification-events/:id", func(c fiber.Ctx) error {
					c.Locals("organization_id", callerOrgID)
					return h.GetVerificationEvent(c)
				})
			},
		},
		{
			name:        "DeleteVerificationEvent",
			method:      "DELETE",
			requestPath: "/verification-events/" + eventID.String(),
			setup: func(app *fiber.App, h *VerificationEventHandler) {
				app.Delete("/verification-events/:id", func(c fiber.Ctx) error {
					c.Locals("organization_id", callerOrgID)
					return h.DeleteVerificationEvent(c)
				})
			},
		},
		{
			name:        "GetAgentVerificationEvents",
			method:      "GET",
			requestPath: "/verification-events/agent/" + agentID.String(),
			setup: func(app *fiber.App, h *VerificationEventHandler) {
				app.Get("/verification-events/agent/:id", func(c fiber.Ctx) error {
					c.Locals("organization_id", callerOrgID)
					return h.GetAgentVerificationEvents(c)
				})
			},
		},
		{
			name:        "GetMCPVerificationEvents",
			method:      "GET",
			requestPath: "/verification-events/mcp/" + mcpServerID.String(),
			setup: func(app *fiber.App, h *VerificationEventHandler) {
				app.Get("/verification-events/mcp/:id", func(c fiber.Ctx) error {
					c.Locals("organization_id", callerOrgID)
					return h.GetMCPVerificationEvents(c)
				})
			},
		},
		{
			// R1 from A3d-iv Phase 4.5: ?agent_id= query-param IDOR on
			// ListVerificationEvents. Same bug class as
			// GetAgentVerificationEvents but read via c.Query() not
			// c.Params(), so the AST lint cannot flag it. Fixed in the
			// same PR to avoid leaving a sibling P0 unpatched.
			name:        "ListVerificationEvents_AgentFilter",
			method:      "GET",
			requestPath: "/verification-events?agent_id=" + agentID.String(),
			setup: func(app *fiber.App, h *VerificationEventHandler) {
				app.Get("/verification-events", func(c fiber.Ctx) error {
					c.Locals("organization_id", callerOrgID)
					return h.ListVerificationEvents(c)
				})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := fiber.New()
			tt.setup(app, handler)

			req := httptest.NewRequest(tt.method, tt.requestPath, nil)
			resp, err := app.Test(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, fiber.StatusNotFound, resp.StatusCode,
				"%s: cross-tenant request must return 404, got %d", tt.name, resp.StatusCode)
			body, _ := io.ReadAll(resp.Body)
			assert.JSONEq(t, `{"error":"not found"}`, string(body),
				"%s: cross-tenant 404 body must be existence-secrecy shape, got %s", tt.name, string(body))
		})
	}
}

func (r *verificationEventTestAgentRepo) ListRevokedIDs(limit, offset int) ([]uuid.UUID, error) {
	return nil, nil
}
