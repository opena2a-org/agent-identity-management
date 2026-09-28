package handlers

// Cross-tenant tests for the handlers #358 moved off the "needs review"
// allowlist and onto LoadOwned. Each foreign-org case must answer the fixed
// 404 body LoadOwned writes (the same as for a missing row) and must not
// reach the service call behind the gate.

import (
	"context"
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

func do358(t *testing.T, app *fiber.App, method, path, body string) (int, string) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, string(b)
}

func TestMCPHandler_VerifyMCPCapability_CrossOrgReturns404AndWritesNothing(t *testing.T) {
	callerOrg, victimOrg := uuid.New(), uuid.New()
	victimServer := uuid.New()
	ownServer := uuid.New()
	verifyCalls := 0

	svc := &MockMCPServiceExtendedImpl{
		GetMCPServerFunc: func(ctx context.Context, id uuid.UUID) (*domain.MCPServer, error) {
			switch id {
			case victimServer:
				return &domain.MCPServer{ID: id, OrganizationID: victimOrg, Name: "victim-mcp"}, nil
			case ownServer:
				return &domain.MCPServer{ID: id, OrganizationID: callerOrg, Name: "own-mcp"}, nil
			}
			return nil, errors.New("mcp server not found")
		},
		VerifyMCPCapabilityFunc: func(ctx context.Context, mcpID uuid.UUID, capability, resource, targetService string, metadata map[string]interface{}) (bool, string, uuid.UUID, error) {
			verifyCalls++
			return true, "ok", uuid.New(), nil
		},
	}
	handler := createMCPHandlerWithMocks(svc, nil, nil, nil, nil, nil, nil)
	app := fiber.New()
	app.Post("/mcp-servers/:id/verify-capability", func(c fiber.Ctx) error {
		c.Locals("organization_id", callerOrg)
		return handler.VerifyMCPCapability(c)
	})
	body := `{"capability":"db:query","resource":"SELECT 1","targetService":"postgresql://x"}`

	status, resp := do358(t, app, "POST", "/mcp-servers/"+victimServer.String()+"/verify-capability", body)
	assert.Equal(t, fiber.StatusNotFound, status)
	assert.JSONEq(t, `{"error":"not found"}`, resp)
	assert.NotContains(t, resp, "victim-mcp")
	assert.Equal(t, 0, verifyCalls, "a foreign server must not reach VerifyMCPCapability (it writes a verification event)")

	status, missing := do358(t, app, "POST", "/mcp-servers/"+uuid.New().String()+"/verify-capability", body)
	assert.Equal(t, fiber.StatusNotFound, status)
	assert.Equal(t, resp, missing, "foreign and missing must be indistinguishable")

	status, _ = do358(t, app, "POST", "/mcp-servers/"+ownServer.String()+"/verify-capability", body)
	assert.Equal(t, fiber.StatusOK, status)
	assert.Equal(t, 1, verifyCalls)
}

func TestMCPHandler_GetConnectedAgents_CrossOrgReturns404(t *testing.T) {
	callerOrg := uuid.New()
	victimServer := uuid.New()
	listed := false

	svc := &MockMCPServiceExtendedImpl{
		GetMCPServerFunc: func(ctx context.Context, id uuid.UUID) (*domain.MCPServer, error) {
			return &domain.MCPServer{ID: id, OrganizationID: uuid.New(), Name: "victim-mcp"}, nil
		},
		GetConnectedAgentsFunc: func(ctx context.Context, mcpServerID uuid.UUID) ([]application.ConnectedAgent, error) {
			listed = true
			return []application.ConnectedAgent{{ID: uuid.New(), Name: "victim-agent"}}, nil
		},
	}
	handler := createMCPHandlerWithMocks(svc, nil, nil, nil, nil, nil, nil)
	app := fiber.New()
	app.Get("/mcp-servers/:id/connected-agents", func(c fiber.Ctx) error {
		c.Locals("organization_id", callerOrg)
		return handler.GetConnectedAgents(c)
	})

	status, resp := do358(t, app, "GET", "/mcp-servers/"+victimServer.String()+"/connected-agents", "")
	assert.Equal(t, fiber.StatusNotFound, status)
	assert.NotContains(t, resp, "victim-agent")
	assert.False(t, listed)
}

func TestMCPGraphHandler_GetMCPServerConnections_CrossOrgReturns404(t *testing.T) {
	callerOrg := uuid.New()
	victimServer := uuid.New()

	// agentRepository is nil: the gate must answer before the agent read.
	handler := &MCPGraphHandler{mcpServerByID: &MockMCPServerRepositoryerImpl{
		GetByIDFunc: func(id uuid.UUID) (*domain.MCPServer, error) {
			return &domain.MCPServer{ID: id, OrganizationID: uuid.New(), Name: "victim-mcp", Status: "verified"}, nil
		},
	}}
	app := fiber.New()
	app.Get("/mcp-servers/:id/connections", func(c fiber.Ctx) error {
		c.Locals("organization_id", callerOrg)
		return handler.GetMCPServerConnections(c)
	})

	status, resp := do358(t, app, "GET", "/mcp-servers/"+victimServer.String()+"/connections", "")
	assert.Equal(t, fiber.StatusNotFound, status)
	assert.JSONEq(t, `{"error":"not found"}`, resp)
	assert.NotContains(t, resp, "victim-mcp")
}

type fakeTagLookup struct {
	tags map[uuid.UUID]*domain.Tag
}

func (f fakeTagLookup) GetTagByID(ctx context.Context, id uuid.UUID) (*domain.Tag, error) {
	if tag, ok := f.tags[id]; ok {
		return tag, nil
	}
	return nil, errors.New("tag not found")
}

func TestTagHandler_UpdateTag_CrossOrgAndMissingAnswerTheSame404(t *testing.T) {
	callerOrg := uuid.New()
	victimTag := uuid.New()

	// tagService is nil: reaching the service would panic, so both cases
	// prove the gate answers first.
	handler := &TagHandler{tagByID: fakeTagLookup{tags: map[uuid.UUID]*domain.Tag{
		victimTag: {ID: victimTag, OrganizationID: uuid.New(), Key: "victim-key"},
	}}}
	app := fiber.New()
	app.Put("/tags/:id", func(c fiber.Ctx) error {
		c.Locals("organization_id", callerOrg)
		c.Locals("user_id", uuid.New())
		return handler.UpdateTag(c)
	})
	body := `{"value":"renamed"}`

	status, foreign := do358(t, app, "PUT", "/tags/"+victimTag.String(), body)
	assert.Equal(t, fiber.StatusNotFound, status)
	assert.JSONEq(t, `{"error":"not found"}`, foreign)

	status, missing := do358(t, app, "PUT", "/tags/"+uuid.New().String(), body)
	assert.Equal(t, fiber.StatusNotFound, status)
	assert.Equal(t, foreign, missing, "a foreign tag must not be distinguishable from a missing one")
}
