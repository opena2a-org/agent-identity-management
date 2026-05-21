package handlers

import (
	"context"
	"encoding/json"
	"io"
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

// ===========================
// CapabilityHandler Integration Tests
// ===========================
// These tests use mock implementations via NewCapabilityHandlerWithInterfaces

// ===========================
// GetAgentCapabilities Tests
// ===========================

func TestCapabilityHandler_GetAgentCapabilities_Success(t *testing.T) {
	agentID := uuid.New()
	orgID := uuid.New()

	mockCapabilityService := &MockCapabilityServiceImpl{
		GetAgentCapabilitiesFunc: func(ctx context.Context, aID uuid.UUID, activeOnly bool) ([]*domain.AgentCapability, error) {
			return []*domain.AgentCapability{
				{ID: uuid.New(), AgentID: aID, CapabilityType: "file:read"},
				{ID: uuid.New(), AgentID: aID, CapabilityType: "network:connect"},
			}, nil
		},
	}

	handler := NewCapabilityHandlerWithInterfaces(mockCapabilityService)
	handler.agentRepo = &MockAgentRepositoryerImpl{
		GetByIDFunc: func(id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{ID: agentID, OrganizationID: orgID}, nil
		},
	}

	app := fiber.New()
	app.Get("/agents/:id/capabilities", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		return handler.GetAgentCapabilities(c)
	})

	req := httptest.NewRequest("GET", "/agents/"+agentID.String()+"/capabilities", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var result []map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Len(t, result, 2)
}

func TestCapabilityHandler_GetAgentCapabilities_InvalidID(t *testing.T) {
	orgID := uuid.New()
	handler := NewCapabilityHandlerWithInterfaces(&MockCapabilityServiceImpl{})

	app := fiber.New()
	app.Get("/agents/:id/capabilities", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		return handler.GetAgentCapabilities(c)
	})

	req := httptest.NewRequest("GET", "/agents/invalid-uuid/capabilities", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestCapabilityHandler_GetAgentCapabilities_ServiceError(t *testing.T) {
	agentID := uuid.New()
	orgID := uuid.New()

	mockCapabilityService := &MockCapabilityServiceImpl{
		GetAgentCapabilitiesFunc: func(ctx context.Context, aID uuid.UUID, activeOnly bool) ([]*domain.AgentCapability, error) {
			return nil, assert.AnError
		},
	}

	handler := NewCapabilityHandlerWithInterfaces(mockCapabilityService)
	handler.agentRepo = &MockAgentRepositoryerImpl{
		GetByIDFunc: func(id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{ID: agentID, OrganizationID: orgID}, nil
		},
	}

	app := fiber.New()
	app.Get("/agents/:id/capabilities", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		return handler.GetAgentCapabilities(c)
	})

	req := httptest.NewRequest("GET", "/agents/"+agentID.String()+"/capabilities", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

// ===========================
// RevokeCapability Tests
// ===========================

func TestCapabilityHandler_RevokeCapability_Success(t *testing.T) {
	agentID := uuid.New()
	capabilityID := uuid.New()
	userID := uuid.New()
	orgID := uuid.New()

	mockCapabilityService := &MockCapabilityServiceImpl{
		GetCapabilityByIDFunc: func(ctx context.Context, capID uuid.UUID) (*domain.AgentCapability, error) {
			return &domain.AgentCapability{ID: capabilityID, AgentID: agentID}, nil
		},
		RevokeCapabilityFunc: func(ctx context.Context, capID uuid.UUID, revokedBy *uuid.UUID) error {
			return nil
		},
	}

	handler := NewCapabilityHandlerWithInterfaces(mockCapabilityService)
	handler.agentRepo = &MockAgentRepositoryerImpl{
		GetByIDFunc: func(id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{ID: agentID, OrganizationID: orgID}, nil
		},
	}

	app := fiber.New()
	app.Delete("/agents/:agentId/capabilities/:capabilityId", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		return handler.RevokeCapability(c)
	})

	req := httptest.NewRequest("DELETE", "/agents/"+agentID.String()+"/capabilities/"+capabilityID.String(), nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestCapabilityHandler_RevokeCapability_InvalidCapabilityID_Integration(t *testing.T) {
	agentID := uuid.New()
	orgID := uuid.New()
	userID := uuid.New()

	handler := NewCapabilityHandlerWithInterfaces(&MockCapabilityServiceImpl{})

	app := fiber.New()
	app.Delete("/agents/:agentId/capabilities/:capabilityId", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		return handler.RevokeCapability(c)
	})

	req := httptest.NewRequest("DELETE", "/agents/"+agentID.String()+"/capabilities/invalid-uuid", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestCapabilityHandler_RevokeCapability_Unauthorized(t *testing.T) {
	agentID := uuid.New()
	capabilityID := uuid.New()
	orgID := uuid.New()

	handler := NewCapabilityHandlerWithInterfaces(&MockCapabilityServiceImpl{})

	app := fiber.New()
	// Set org_id but NOT user_id — verifies the user-context guard still
	// returns 401 after the A3c #44 org-scoping check passes.
	app.Delete("/agents/:agentId/capabilities/:capabilityId", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		return handler.RevokeCapability(c)
	})

	req := httptest.NewRequest("DELETE", "/agents/"+agentID.String()+"/capabilities/"+capabilityID.String(), nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

// A3c #44: cross-tenant capability revocation must return 404.
func TestCapabilityHandler_RevokeCapability_CrossOrgReturns404(t *testing.T) {
	agentID := uuid.New()
	capabilityID := uuid.New()
	callerOrgID := uuid.New()
	differentOrgID := uuid.New()
	userID := uuid.New()

	mockCapabilityService := &MockCapabilityServiceImpl{
		GetCapabilityByIDFunc: func(ctx context.Context, capID uuid.UUID) (*domain.AgentCapability, error) {
			return &domain.AgentCapability{ID: capabilityID, AgentID: agentID}, nil
		},
		RevokeCapabilityFunc: func(ctx context.Context, capID uuid.UUID, revokedBy *uuid.UUID) error {
			// Should NOT be reached.
			t.Fatalf("RevokeCapability called for cross-tenant request — security regression")
			return nil
		},
	}

	handler := NewCapabilityHandlerWithInterfaces(mockCapabilityService)
	handler.agentRepo = &MockAgentRepositoryerImpl{
		GetByIDFunc: func(id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{ID: agentID, OrganizationID: differentOrgID}, nil
		},
	}

	app := fiber.New()
	app.Delete("/agents/:agentId/capabilities/:capabilityId", func(c fiber.Ctx) error {
		c.Locals("organization_id", callerOrgID)
		c.Locals("user_id", userID)
		return handler.RevokeCapability(c)
	})

	req := httptest.NewRequest("DELETE", "/agents/"+agentID.String()+"/capabilities/"+capabilityID.String(), nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.JSONEq(t, `{"error":"not found"}`, string(body))
}

func TestCapabilityHandler_RevokeCapability_ServiceError(t *testing.T) {
	agentID := uuid.New()
	capabilityID := uuid.New()
	userID := uuid.New()
	orgID := uuid.New()

	mockCapabilityService := &MockCapabilityServiceImpl{
		GetCapabilityByIDFunc: func(ctx context.Context, capID uuid.UUID) (*domain.AgentCapability, error) {
			return &domain.AgentCapability{ID: capabilityID, AgentID: agentID}, nil
		},
		RevokeCapabilityFunc: func(ctx context.Context, capID uuid.UUID, revokedBy *uuid.UUID) error {
			return assert.AnError
		},
	}

	handler := NewCapabilityHandlerWithInterfaces(mockCapabilityService)
	handler.agentRepo = &MockAgentRepositoryerImpl{
		GetByIDFunc: func(id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{ID: agentID, OrganizationID: orgID}, nil
		},
	}

	app := fiber.New()
	app.Delete("/agents/:agentId/capabilities/:capabilityId", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		return handler.RevokeCapability(c)
	})

	req := httptest.NewRequest("DELETE", "/agents/"+agentID.String()+"/capabilities/"+capabilityID.String(), nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

// ===========================
// GetViolationsByAgent Tests
// ===========================

func TestCapabilityHandler_GetViolationsByAgent_Success(t *testing.T) {
	agentID := uuid.New()
	orgID := uuid.New()

	mockCapabilityService := &MockCapabilityServiceImpl{
		GetViolationsByAgentFunc: func(ctx context.Context, aID uuid.UUID, limit, offset int) ([]*domain.CapabilityViolation, int, error) {
			return []*domain.CapabilityViolation{
				{ID: uuid.New(), AgentID: aID, AttemptedCapability: "file:write"},
			}, 5, nil
		},
	}

	handler := NewCapabilityHandlerWithInterfaces(mockCapabilityService)
	handler.agentRepo = &MockAgentRepositoryerImpl{
		GetByIDFunc: func(id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{ID: agentID, OrganizationID: orgID}, nil
		},
	}

	app := fiber.New()
	app.Get("/agents/:id/violations", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		return handler.GetViolationsByAgent(c)
	})

	req := httptest.NewRequest("GET", "/agents/"+agentID.String()+"/violations", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, float64(5), result["total"])
}

func TestCapabilityHandler_GetViolationsByAgent_InvalidID(t *testing.T) {
	orgID := uuid.New()
	handler := NewCapabilityHandlerWithInterfaces(&MockCapabilityServiceImpl{})

	app := fiber.New()
	app.Get("/agents/:id/violations", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		return handler.GetViolationsByAgent(c)
	})

	req := httptest.NewRequest("GET", "/agents/invalid-uuid/violations", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestCapabilityHandler_GetViolationsByAgent_ServiceError(t *testing.T) {
	agentID := uuid.New()
	orgID := uuid.New()

	mockCapabilityService := &MockCapabilityServiceImpl{
		GetViolationsByAgentFunc: func(ctx context.Context, aID uuid.UUID, limit, offset int) ([]*domain.CapabilityViolation, int, error) {
			return nil, 0, assert.AnError
		},
	}

	handler := NewCapabilityHandlerWithInterfaces(mockCapabilityService)
	handler.agentRepo = &MockAgentRepositoryerImpl{
		GetByIDFunc: func(id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{ID: agentID, OrganizationID: orgID}, nil
		},
	}

	app := fiber.New()
	app.Get("/agents/:id/violations", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		return handler.GetViolationsByAgent(c)
	})

	req := httptest.NewRequest("GET", "/agents/"+agentID.String()+"/violations", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

// Audit-doc #49: cross-tenant violation read must return 404 with
// existence-secrecy body, never disclose that the agent exists in
// another org. Uses a captured-flag pattern (NOT t.Fatalf) because
// fiber's app.Test runs the handler on a separate goroutine; t.Fatalf
// from a non-test-goroutine calls runtime.Goexit on that goroutine
// only and would NOT fail this test. Mirrors the panic-proof pattern
// from PR #146.
func TestCapabilityHandler_GetViolationsByAgent_CrossOrgReturns404(t *testing.T) {
	agentID := uuid.New()
	callerOrgID := uuid.New()
	differentOrgID := uuid.New()

	serviceCalled := false
	mockCapabilityService := &MockCapabilityServiceImpl{
		GetViolationsByAgentFunc: func(ctx context.Context, aID uuid.UUID, limit, offset int) ([]*domain.CapabilityViolation, int, error) {
			serviceCalled = true
			return nil, 0, nil
		},
	}

	handler := NewCapabilityHandlerWithInterfaces(mockCapabilityService)
	handler.agentRepo = &MockAgentRepositoryerImpl{
		GetByIDFunc: func(id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{ID: agentID, OrganizationID: differentOrgID}, nil
		},
	}

	app := fiber.New()
	app.Get("/agents/:id/violations", func(c fiber.Ctx) error {
		c.Locals("organization_id", callerOrgID)
		return handler.GetViolationsByAgent(c)
	})

	req := httptest.NewRequest("GET", "/agents/"+agentID.String()+"/violations", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.JSONEq(t, `{"error":"not found"}`, string(body))
	assert.False(t, serviceCalled, "service must NOT be called for a cross-tenant request — security regression")
}

// Audit-doc #51: GetAgentCapabilities cross-tenant must return 404,
// not 200 with the foreign agent's capability list. Same panic-proof
// pattern as the #49 test.
func TestCapabilityHandler_GetAgentCapabilities_CrossOrgReturns404(t *testing.T) {
	agentID := uuid.New()
	callerOrgID := uuid.New()
	differentOrgID := uuid.New()

	serviceCalled := false
	mockCapabilityService := &MockCapabilityServiceImpl{
		GetAgentCapabilitiesFunc: func(ctx context.Context, aID uuid.UUID, activeOnly bool) ([]*domain.AgentCapability, error) {
			serviceCalled = true
			return nil, nil
		},
	}

	handler := NewCapabilityHandlerWithInterfaces(mockCapabilityService)
	handler.agentRepo = &MockAgentRepositoryerImpl{
		GetByIDFunc: func(id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{ID: agentID, OrganizationID: differentOrgID}, nil
		},
	}

	app := fiber.New()
	app.Get("/agents/:id/capabilities", func(c fiber.Ctx) error {
		c.Locals("organization_id", callerOrgID)
		return handler.GetAgentCapabilities(c)
	})

	req := httptest.NewRequest("GET", "/agents/"+agentID.String()+"/capabilities", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.JSONEq(t, `{"error":"not found"}`, string(body))
	assert.False(t, serviceCalled, "service must NOT be called for a cross-tenant request — security regression")
}

// ===========================
// ListCapabilities Tests
// ===========================

func TestCapabilityHandler_ListCapabilities_Success(t *testing.T) {
	orgID := uuid.New()

	mockCapabilityService := &MockCapabilityServiceImpl{
		ListCapabilitiesWithMetadataFunc: func(ctx context.Context, oID uuid.UUID) (*application.ListCapabilitiesResponse, error) {
			return &application.ListCapabilitiesResponse{
				Capabilities: []application.CapabilityDefinition{
					{Type: "file:read", Description: "Read files"},
					{Type: "file:write", Description: "Write files"},
				},
			}, nil
		},
	}

	handler := NewCapabilityHandlerWithInterfaces(mockCapabilityService)

	app := fiber.New()
	app.Get("/capabilities", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		return handler.ListCapabilities(c)
	})

	req := httptest.NewRequest("GET", "/capabilities", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestCapabilityHandler_ListCapabilities_NoOrgID(t *testing.T) {
	handler := NewCapabilityHandlerWithInterfaces(&MockCapabilityServiceImpl{})

	app := fiber.New()
	app.Get("/capabilities", handler.ListCapabilities)

	req := httptest.NewRequest("GET", "/capabilities", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestCapabilityHandler_ListCapabilities_InvalidOrgIDType_Integration(t *testing.T) {
	handler := NewCapabilityHandlerWithInterfaces(&MockCapabilityServiceImpl{})

	app := fiber.New()
	app.Get("/capabilities", func(c fiber.Ctx) error {
		c.Locals("organization_id", "not-a-uuid") // Wrong type
		return handler.ListCapabilities(c)
	})

	req := httptest.NewRequest("GET", "/capabilities", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

func TestCapabilityHandler_ListCapabilities_ServiceError(t *testing.T) {
	orgID := uuid.New()

	mockCapabilityService := &MockCapabilityServiceImpl{
		ListCapabilitiesWithMetadataFunc: func(ctx context.Context, oID uuid.UUID) (*application.ListCapabilitiesResponse, error) {
			return nil, assert.AnError
		},
	}

	handler := NewCapabilityHandlerWithInterfaces(mockCapabilityService)

	app := fiber.New()
	app.Get("/capabilities", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		return handler.ListCapabilities(c)
	})

	req := httptest.NewRequest("GET", "/capabilities", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

// ===========================
// GrantCapability Tests
// ===========================

func TestCapabilityHandler_GrantCapability_Success(t *testing.T) {
	agentID := uuid.New()
	orgID := uuid.New()
	userID := uuid.New()

	mockCapabilityService := &MockCapabilityServiceImpl{
		ValidateAndRegisterCapabilityFunc: func(ctx context.Context, capability string, oID uuid.UUID) error {
			return nil
		},
		GrantCapabilityFunc: func(ctx context.Context, aID uuid.UUID, capType string, scope map[string]interface{}, grantedBy *uuid.UUID, executionMode string) (*domain.AgentCapability, error) {
			return &domain.AgentCapability{
				ID:             uuid.New(),
				AgentID:        aID,
				CapabilityType: capType,
			}, nil
		},
	}

	// Tenant-scoping (defect #25 fix) requires the agent referenced by URL
	// to belong to the caller's org. Mock returns an agent owned by orgID
	// so LoadOwned succeeds and execution proceeds to the capability grant.
	handler := NewCapabilityHandlerWithInterfaces(mockCapabilityService)
	handler.agentRepo = &MockAgentRepositoryerImpl{
		GetByIDFunc: func(id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{ID: id, OrganizationID: orgID}, nil
		},
	}

	app := fiber.New()
	app.Post("/agents/:id/capabilities", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		return handler.GrantCapability(c)
	})

	body := `{"capabilityType": "file:read", "scope": {}}`
	req := httptest.NewRequest("POST", "/agents/"+agentID.String()+"/capabilities", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusCreated, resp.StatusCode)
}

func TestCapabilityHandler_GrantCapability_InvalidAgentID_Integration(t *testing.T) {
	handler := NewCapabilityHandlerWithInterfaces(&MockCapabilityServiceImpl{})

	app := fiber.New()
	app.Post("/agents/:id/capabilities", handler.GrantCapability)

	req := httptest.NewRequest("POST", "/agents/invalid-uuid/capabilities", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestCapabilityHandler_GrantCapability_InvalidBody(t *testing.T) {
	agentID := uuid.New()

	handler := NewCapabilityHandlerWithInterfaces(&MockCapabilityServiceImpl{})

	app := fiber.New()
	app.Post("/agents/:id/capabilities", handler.GrantCapability)

	req := httptest.NewRequest("POST", "/agents/"+agentID.String()+"/capabilities", strings.NewReader("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestCapabilityHandler_GrantCapability_NoUserID(t *testing.T) {
	agentID := uuid.New()
	orgID := uuid.New()

	handler := NewCapabilityHandlerWithInterfaces(&MockCapabilityServiceImpl{})

	app := fiber.New()
	app.Post("/agents/:id/capabilities", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		// No user_id set
		return handler.GrantCapability(c)
	})

	body := `{"capabilityType": "file:read", "scope": {}}`
	req := httptest.NewRequest("POST", "/agents/"+agentID.String()+"/capabilities", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestCapabilityHandler_GrantCapability_NoOrgID(t *testing.T) {
	agentID := uuid.New()
	userID := uuid.New()

	handler := NewCapabilityHandlerWithInterfaces(&MockCapabilityServiceImpl{})

	app := fiber.New()
	app.Post("/agents/:id/capabilities", func(c fiber.Ctx) error {
		c.Locals("user_id", userID)
		// No organization_id set
		return handler.GrantCapability(c)
	})

	body := `{"capabilityType": "file:read", "scope": {}}`
	req := httptest.NewRequest("POST", "/agents/"+agentID.String()+"/capabilities", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestCapabilityHandler_GrantCapability_ValidationError(t *testing.T) {
	agentID := uuid.New()
	orgID := uuid.New()
	userID := uuid.New()

	mockCapabilityService := &MockCapabilityServiceImpl{
		ValidateAndRegisterCapabilityFunc: func(ctx context.Context, capability string, oID uuid.UUID) error {
			return assert.AnError
		},
	}

	handler := NewCapabilityHandlerWithInterfaces(mockCapabilityService)
	// Tenant-scoping (defect #25 fix) precondition.
	handler.agentRepo = &MockAgentRepositoryerImpl{
		GetByIDFunc: func(id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{ID: id, OrganizationID: orgID}, nil
		},
	}

	app := fiber.New()
	app.Post("/agents/:id/capabilities", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		return handler.GrantCapability(c)
	})

	body := `{"capabilityType": "invalid:capability", "scope": {}}`
	req := httptest.NewRequest("POST", "/agents/"+agentID.String()+"/capabilities", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// NewCapabilityHandlerWithInterfaces Tests
// ===========================

func TestNewCapabilityHandlerWithInterfaces_NilDeps(t *testing.T) {
	handler := NewCapabilityHandlerWithInterfaces(nil)
	assert.NotNil(t, handler)
}

func TestNewCapabilityHandlerWithInterfaces_WithMock(t *testing.T) {
	mockCap := &MockCapabilityServiceImpl{}

	handler := NewCapabilityHandlerWithInterfaces(mockCap)

	assert.NotNil(t, handler)
	assert.NotNil(t, handler.capabilityServicer)
}

// ===========================
// RegisterCapability Tests (defect #48 — cross-tenant scoping)
// ===========================
//
// SECURITY context: PR #136 fixed defect #25 for the sibling
// GrantCapability handler by wrapping the URL-derived agentID lookup in
// LoadOwned so a cross-tenant request returns 404. RegisterCapability had
// the same IDOR shape but was silently lint-exempt via the
// `bodyMentionsOrganizationID` lenient heuristic (the handler referenced
// `agent.OrganizationID` only for victim-side lookups, never compared to
// the caller). These tests assert the LoadOwned wrap closes the gap.

// TestCapabilityHandler_RegisterCapability_CrossOrg_Returns404 asserts
// that a caller authenticated to org A cannot register capabilities on an
// agent in org B by substituting the foreign agent's UUID in the URL.
// LoadOwned returns 404 (not 403) to preserve existence-secrecy across
// organizations.
//
// Enforcement mode is irrelevant for this test: LoadOwned short-circuits
// before the handler reaches `orgRepo.GetByID(agent.OrganizationID)`.
// Without this fix the request would, in MONITORING mode, auto-grant the
// requested capability on agent B.
//
// REGRESSION GUARANTEE: The mock `orgRepo` records whether it was
// invoked. A future change that removes the `LoadOwned` wrap (or
// otherwise routes cross-org execution past the scoping check) reaches
// `orgRepo.GetByID(agent.OrganizationID)` at the post-LoadOwned org
// lookup, sets the flag, and the post-call assertion fails with a clear
// message. This makes the test a clean structural guard rather than
// relying on a downstream nil-pointer panic if `orgRepo` were left unset.
func TestCapabilityHandler_RegisterCapability_CrossOrg_Returns404(t *testing.T) {
	agentID := uuid.New()
	callerOrgID := uuid.New()
	otherOrgID := uuid.New()
	userID := uuid.New()

	handler := NewCapabilityHandlerWithInterfaces(&MockCapabilityServiceImpl{})
	handler.agentRepo = &MockAgentRepositoryerImpl{
		GetByIDFunc: func(id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{ID: id, OrganizationID: otherOrgID}, nil
		},
	}
	orgRepoCalled := false
	handler.orgRepo = &MockOrganizationRepository{
		GetByIDFunc: func(id uuid.UUID) (*domain.Organization, error) {
			orgRepoCalled = true
			return &domain.Organization{ID: id, EnforcementMode: domain.EnforcementModeMonitoring}, nil
		},
	}

	app := fiber.New()
	app.Post("/sdk-api/agents/:id/capabilities/register", func(c fiber.Ctx) error {
		c.Locals("organization_id", callerOrgID)
		c.Locals("user_id", userID)
		return handler.RegisterCapability(c)
	})

	body := `{"capabilityType": "file:read"}`
	req := httptest.NewRequest("POST", "/sdk-api/agents/"+agentID.String()+"/capabilities/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)

	// Response body must NOT echo the cross-org agent's UUID or any other
	// existence signal — same response shape as "agent does not exist."
	var body404 map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body404))
	assert.Equal(t, "not found", body404["error"])
	assert.NotContains(t, body404, "agentId")
	assert.NotContains(t, body404, "organizationId")

	// Regression assertion: LoadOwned must short-circuit before the
	// org-settings lookup. If `orgRepoCalled` is true, the handler
	// reached the post-LoadOwned org lookup on a cross-org request —
	// the scoping check was bypassed and the IDOR has regressed.
	assert.False(t, orgRepoCalled,
		"orgRepo.GetByID was reached on a cross-org request; LoadOwned scoping was bypassed")
}

// TestCapabilityHandler_RegisterCapability_NoOrgID_Integration mirrors
// GrantCapability's defensive coverage for the case where the
// organization_id local is missing entirely — middleware misconfiguration
// or a stripped JWT. The handler must 401 before any data access.
func TestCapabilityHandler_RegisterCapability_NoOrgID_Integration(t *testing.T) {
	agentID := uuid.New()
	userID := uuid.New()

	handler := NewCapabilityHandlerWithInterfaces(&MockCapabilityServiceImpl{})

	app := fiber.New()
	app.Post("/sdk-api/agents/:id/capabilities/register", func(c fiber.Ctx) error {
		c.Locals("user_id", userID)
		// organization_id intentionally absent
		return handler.RegisterCapability(c)
	})

	body := `{"capabilityType": "file:read"}`
	req := httptest.NewRequest("POST", "/sdk-api/agents/"+agentID.String()+"/capabilities/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

// TestCapabilityHandler_RegisterCapability_SameOrg_Monitoring_AutoGrants
// confirms the LoadOwned wrap does not break the legitimate happy path
// for a same-org caller registering on their own agent in MONITORING
// mode. Without this test, a regression that broke MONITORING auto-grant
// would not surface in CI.
func TestCapabilityHandler_RegisterCapability_SameOrg_Monitoring_AutoGrants(t *testing.T) {
	agentID := uuid.New()
	orgID := uuid.New()
	userID := uuid.New()

	mockCapabilityService := &MockCapabilityServiceImpl{
		GetAgentCapabilitiesFunc: func(ctx context.Context, aID uuid.UUID, activeOnly bool) ([]*domain.AgentCapability, error) {
			return nil, nil
		},
		ValidateAndRegisterCapabilityFunc: func(ctx context.Context, capability string, oID uuid.UUID) error {
			return nil
		},
		GrantCapabilityFunc: func(ctx context.Context, aID uuid.UUID, capType string, scope map[string]interface{}, grantedBy *uuid.UUID, executionMode string) (*domain.AgentCapability, error) {
			return &domain.AgentCapability{
				ID:             uuid.New(),
				AgentID:        aID,
				CapabilityType: capType,
			}, nil
		},
	}

	handler := NewCapabilityHandlerWithInterfaces(mockCapabilityService)
	handler.agentRepo = &MockAgentRepositoryerImpl{
		GetByIDFunc: func(id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{ID: id, OrganizationID: orgID}, nil
		},
	}
	handler.orgRepo = &MockOrganizationRepository{
		GetByIDFunc: func(id uuid.UUID) (*domain.Organization, error) {
			return &domain.Organization{
				ID:              id,
				EnforcementMode: domain.EnforcementModeMonitoring,
			}, nil
		},
	}

	app := fiber.New()
	app.Post("/sdk-api/agents/:id/capabilities/register", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		return handler.RegisterCapability(c)
	})

	body := `{"capabilityType": "file:read"}`
	req := httptest.NewRequest("POST", "/sdk-api/agents/"+agentID.String()+"/capabilities/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusCreated, resp.StatusCode)

	var registered RegisterCapabilityResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&registered))
	assert.True(t, registered.Success)
	assert.Equal(t, "granted", registered.Status)
	assert.Equal(t, "file:read", registered.CapabilityType)
}
