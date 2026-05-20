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
// MCPHandler Integration Tests
// ===========================
// These tests use mock implementations via NewMCPHandlerWithInterfaces

// Helper to create an MCPHandler with mocks for integration tests
func createMCPHandlerWithMocks(
	mcpService MCPServicerExtended,
	mcpCapabilityService MCPCapabilityServicer,
	auditService AuditServicer,
	agentRepository AgentRepositoryer,
	verificationEventRepository VerificationEventRepositoryer,
	tagService TagServicer,
	attestationService MCPAttestationServicerExtended,
) *MCPHandler {
	return NewMCPHandlerWithInterfaces(
		mcpService,
		mcpCapabilityService,
		auditService,
		agentRepository,
		verificationEventRepository,
		tagService,
		attestationService,
	)
}

// ===========================
// ListMCPServers Tests
// ===========================

func TestMCPHandler_ListMCPServers_Success(t *testing.T) {
	orgID := uuid.New()
	serverID1 := uuid.New()
	serverID2 := uuid.New()

	mockMCPService := &MockMCPServiceExtendedImpl{
		ListMCPServersFunc: func(ctx context.Context, oID uuid.UUID) ([]*domain.MCPServer, error) {
			return []*domain.MCPServer{
				{ID: serverID1, OrganizationID: oID, Name: "Server1", Status: domain.MCPServerStatusVerified},
				{ID: serverID2, OrganizationID: oID, Name: "Server2", Status: domain.MCPServerStatusPending},
			}, nil
		},
	}

	mockCapabilityService := &MockMCPCapabilityServiceImpl{
		GetCapabilitiesFunc: func(ctx context.Context, mcpServerID uuid.UUID) ([]*domain.MCPServerCapability, error) {
			return []*domain.MCPServerCapability{
				{ID: uuid.New(), CapabilityType: "tool", Name: "test-tool"},
			}, nil
		},
	}

	mockTagService := &MockTagServiceImpl{
		GetMCPServerTagsFunc: func(ctx context.Context, mcpServerID uuid.UUID) ([]*domain.Tag, error) {
			return []*domain.Tag{}, nil
		},
	}

	handler := createMCPHandlerWithMocks(
		mockMCPService,
		mockCapabilityService,
		nil,
		nil,
		nil,
		mockTagService,
		nil,
	)

	app := fiber.New()
	app.Get("/mcp-servers", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		return handler.ListMCPServers(c)
	})

	req := httptest.NewRequest("GET", "/mcp-servers", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, float64(2), result["total"])
	servers := result["mcpServers"].([]interface{})
	assert.Len(t, servers, 2)
}

func TestMCPHandler_ListMCPServers_ServiceError(t *testing.T) {
	orgID := uuid.New()

	mockMCPService := &MockMCPServiceExtendedImpl{
		ListMCPServersFunc: func(ctx context.Context, oID uuid.UUID) ([]*domain.MCPServer, error) {
			return nil, assert.AnError
		},
	}

	handler := createMCPHandlerWithMocks(
		mockMCPService,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	app := fiber.New()
	app.Get("/mcp-servers", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		return handler.ListMCPServers(c)
	})

	req := httptest.NewRequest("GET", "/mcp-servers", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

// ===========================
// GetMCPServer Tests
// ===========================

func TestMCPHandler_GetMCPServer_Success(t *testing.T) {
	orgID := uuid.New()
	serverID := uuid.New()

	mockMCPService := &MockMCPServiceExtendedImpl{
		GetMCPServerFunc: func(ctx context.Context, id uuid.UUID) (*domain.MCPServer, error) {
			return &domain.MCPServer{
				ID:             serverID,
				OrganizationID: orgID,
				Name:           "TestServer",
				Status:         domain.MCPServerStatusVerified,
			}, nil
		},
	}

	mockTagService := &MockTagServiceImpl{
		GetMCPServerTagsFunc: func(ctx context.Context, mcpServerID uuid.UUID) ([]*domain.Tag, error) {
			return []*domain.Tag{
				{ID: uuid.New(), Key: "env", Value: "production"},
			}, nil
		},
	}

	handler := createMCPHandlerWithMocks(
		mockMCPService,
		nil,
		nil,
		nil,
		nil,
		mockTagService,
		nil,
	)

	app := fiber.New()
	app.Get("/mcp-servers/:id", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		return handler.GetMCPServer(c)
	})

	req := httptest.NewRequest("GET", "/mcp-servers/"+serverID.String(), nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, serverID.String(), result["id"])
	assert.Equal(t, "TestServer", result["name"])
}

func TestMCPHandler_GetMCPServer_InvalidID_Integration(t *testing.T) {
	orgID := uuid.New()

	handler := createMCPHandlerWithMocks(nil, nil, nil, nil, nil, nil, nil)

	app := fiber.New()
	app.Get("/mcp-servers/:id", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		return handler.GetMCPServer(c)
	})

	req := httptest.NewRequest("GET", "/mcp-servers/invalid-uuid", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestMCPHandler_GetMCPServer_NotFound(t *testing.T) {
	orgID := uuid.New()
	serverID := uuid.New()

	mockMCPService := &MockMCPServiceExtendedImpl{
		GetMCPServerFunc: func(ctx context.Context, id uuid.UUID) (*domain.MCPServer, error) {
			return nil, assert.AnError
		},
	}

	handler := createMCPHandlerWithMocks(mockMCPService, nil, nil, nil, nil, nil, nil)

	app := fiber.New()
	app.Get("/mcp-servers/:id", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		return handler.GetMCPServer(c)
	})

	req := httptest.NewRequest("GET", "/mcp-servers/"+serverID.String(), nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

func TestMCPHandler_GetMCPServer_AccessDenied(t *testing.T) {
	orgID := uuid.New()
	otherOrgID := uuid.New()
	serverID := uuid.New()

	mockMCPService := &MockMCPServiceExtendedImpl{
		GetMCPServerFunc: func(ctx context.Context, id uuid.UUID) (*domain.MCPServer, error) {
			return &domain.MCPServer{
				ID:             serverID,
				OrganizationID: otherOrgID, // Different org
				Name:           "TestServer",
			}, nil
		},
	}

	handler := createMCPHandlerWithMocks(mockMCPService, nil, nil, nil, nil, nil, nil)

	app := fiber.New()
	app.Get("/mcp-servers/:id", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		return handler.GetMCPServer(c)
	})

	req := httptest.NewRequest("GET", "/mcp-servers/"+serverID.String(), nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusForbidden, resp.StatusCode)
}

// ===========================
// GetMCPServerByName Tests
// ===========================

func TestMCPHandler_GetMCPServerByName_Success(t *testing.T) {
	orgID := uuid.New()
	serverID := uuid.New()

	mockMCPService := &MockMCPServiceExtendedImpl{
		GetMCPServerByNameFunc: func(ctx context.Context, oID uuid.UUID, name string) (*domain.MCPServer, error) {
			return &domain.MCPServer{
				ID:           serverID,
				OrganizationID: oID,
				Name:         name,
				Capabilities: []string{"tool1", "tool2"},
			}, nil
		},
	}

	handler := createMCPHandlerWithMocks(mockMCPService, nil, nil, nil, nil, nil, nil)

	app := fiber.New()
	app.Get("/mcp-servers/by-name", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		return handler.GetMCPServerByName(c)
	})

	req := httptest.NewRequest("GET", "/mcp-servers/by-name?name=TestServer", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, "TestServer", result["name"])
	assert.Equal(t, true, result["hasCachedCapabilities"])
	assert.Equal(t, float64(2), result["toolCount"])
}

func TestMCPHandler_GetMCPServerByName_MissingName_Integration(t *testing.T) {
	orgID := uuid.New()

	handler := createMCPHandlerWithMocks(nil, nil, nil, nil, nil, nil, nil)

	app := fiber.New()
	app.Get("/mcp-servers/by-name", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		return handler.GetMCPServerByName(c)
	})

	req := httptest.NewRequest("GET", "/mcp-servers/by-name", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestMCPHandler_GetMCPServerByName_NotFound(t *testing.T) {
	orgID := uuid.New()

	mockMCPService := &MockMCPServiceExtendedImpl{
		GetMCPServerByNameFunc: func(ctx context.Context, oID uuid.UUID, name string) (*domain.MCPServer, error) {
			return nil, assert.AnError
		},
	}

	handler := createMCPHandlerWithMocks(mockMCPService, nil, nil, nil, nil, nil, nil)

	app := fiber.New()
	app.Get("/mcp-servers/by-name", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		return handler.GetMCPServerByName(c)
	})

	req := httptest.NewRequest("GET", "/mcp-servers/by-name?name=NonExistent", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

// ===========================
// GetMCPServerCapabilities Tests
// ===========================

func TestMCPHandler_GetMCPServerCapabilities_Success(t *testing.T) {
	orgID := uuid.New()
	serverID := uuid.New()

	mockMCPService := &MockMCPServiceExtendedImpl{
		GetMCPServerFunc: func(ctx context.Context, id uuid.UUID) (*domain.MCPServer, error) {
			return &domain.MCPServer{
				ID:             serverID,
				OrganizationID: orgID,
				Name:           "TestServer",
			}, nil
		},
	}

	mockCapabilityService := &MockMCPCapabilityServiceImpl{
		GetCapabilitiesFunc: func(ctx context.Context, mcpServerID uuid.UUID) ([]*domain.MCPServerCapability, error) {
			return []*domain.MCPServerCapability{
				{ID: uuid.New(), CapabilityType: "tool", Name: "read_file"},
				{ID: uuid.New(), CapabilityType: "tool", Name: "write_file"},
				{ID: uuid.New(), CapabilityType: "prompt", Name: "summarize"},
			}, nil
		},
	}

	handler := createMCPHandlerWithMocks(mockMCPService, mockCapabilityService, nil, nil, nil, nil, nil)

	app := fiber.New()
	app.Get("/mcp-servers/:id/capabilities", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		return handler.GetMCPServerCapabilities(c)
	})

	req := httptest.NewRequest("GET", "/mcp-servers/"+serverID.String()+"/capabilities", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, float64(3), result["total"])
}

func TestMCPHandler_GetMCPServerCapabilities_InvalidID_Integration(t *testing.T) {
	orgID := uuid.New()

	handler := createMCPHandlerWithMocks(nil, nil, nil, nil, nil, nil, nil)

	app := fiber.New()
	app.Get("/mcp-servers/:id/capabilities", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		return handler.GetMCPServerCapabilities(c)
	})

	req := httptest.NewRequest("GET", "/mcp-servers/invalid-uuid/capabilities", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// GetMCPServerAgents Tests
// ===========================

func TestMCPHandler_GetMCPServerAgents_Success(t *testing.T) {
	orgID := uuid.New()
	serverID := uuid.New()
	agentID := uuid.New()

	mockMCPService := &MockMCPServiceExtendedImpl{
		GetMCPServerFunc: func(ctx context.Context, id uuid.UUID) (*domain.MCPServer, error) {
			return &domain.MCPServer{
				ID:             serverID,
				OrganizationID: orgID,
				Name:           "TestServer",
			}, nil
		},
	}

	mockAgentRepo := &MockAgentRepositoryerImpl{
		GetByMCPServerFunc: func(mcpServerID, oID uuid.UUID) ([]*domain.Agent, error) {
			return []*domain.Agent{
				{ID: agentID, Name: "TestAgent", Status: domain.AgentStatusVerified},
			}, nil
		},
		GetByMCPServerNameFunc: func(mcpServerName string, oID uuid.UUID) ([]*domain.Agent, error) {
			return []*domain.Agent{}, nil
		},
	}

	handler := createMCPHandlerWithMocks(mockMCPService, nil, nil, mockAgentRepo, nil, nil, nil)

	app := fiber.New()
	app.Get("/mcp-servers/:id/agents", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		return handler.GetMCPServerAgents(c)
	})

	req := httptest.NewRequest("GET", "/mcp-servers/"+serverID.String()+"/agents", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, float64(1), result["total"])
}

func TestMCPHandler_GetMCPServerAgents_InvalidID_Integration(t *testing.T) {
	orgID := uuid.New()

	handler := createMCPHandlerWithMocks(nil, nil, nil, nil, nil, nil, nil)

	app := fiber.New()
	app.Get("/mcp-servers/:id/agents", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		return handler.GetMCPServerAgents(c)
	})

	req := httptest.NewRequest("GET", "/mcp-servers/invalid-uuid/agents", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// GetConnectedAgents Tests
// ===========================

func TestMCPHandler_GetConnectedAgents_Success(t *testing.T) {
	serverID := uuid.New()

	mockMCPService := &MockMCPServiceExtendedImpl{
		GetConnectedAgentsFunc: func(ctx context.Context, mcpServerID uuid.UUID) ([]application.ConnectedAgent, error) {
			return []application.ConnectedAgent{
				{ID: uuid.New(), Name: "Agent1"},
				{ID: uuid.New(), Name: "Agent2"},
			}, nil
		},
	}

	handler := createMCPHandlerWithMocks(mockMCPService, nil, nil, nil, nil, nil, nil)

	app := fiber.New()
	app.Get("/mcp-servers/:id/connected-agents", func(c fiber.Ctx) error {
		return handler.GetConnectedAgents(c)
	})

	req := httptest.NewRequest("GET", "/mcp-servers/"+serverID.String()+"/connected-agents", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, float64(2), result["count"])
}

func TestMCPHandler_GetConnectedAgents_InvalidID_Integration(t *testing.T) {
	handler := createMCPHandlerWithMocks(nil, nil, nil, nil, nil, nil, nil)

	app := fiber.New()
	app.Get("/mcp-servers/:id/connected-agents", func(c fiber.Ctx) error {
		return handler.GetConnectedAgents(c)
	})

	req := httptest.NewRequest("GET", "/mcp-servers/invalid-uuid/connected-agents", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// MCPHandler.CreateMCPServer — defect #18 read-oracle closure
// ===========================

// SECURITY (defect #18): CreateMCPServer with a body-supplied
// `registeredByAgent` UUID that belongs to a different organization must
// drop the linkage AND scrub the response — the body-supplied UUID and the
// cross-org organization UUID must not appear anywhere in the response.
// PR #136 closed the write-side; this guards the read oracle.
func TestMCPHandler_CreateMCPServer_CrossOrgRegisteredByAgent_NoEcho(t *testing.T) {
	callerOrgID := uuid.New()
	crossOrgID := uuid.New()
	userID := uuid.New()
	crossOrgAgentID := uuid.New()

	mockMCPService := &MockMCPServiceExtendedImpl{
		CreateMCPServerFunc: func(ctx context.Context, req *application.CreateMCPServerRequest, orgID, uID uuid.UUID, agentID *uuid.UUID, sdkTokenID *uuid.UUID, apiKeyID *uuid.UUID) (*domain.MCPServer, error) {
			// Service is called with agentID == nil because the handler's
			// cross-org check dropped the body-supplied linkage. Echo that
			// reality in the returned server: RegisteredByAgent must be nil.
			return &domain.MCPServer{
				ID:                uuid.New(),
				OrganizationID:    orgID,
				Name:              req.Name,
				URL:               req.URL,
				Status:            domain.MCPServerStatusPending,
				IsVerified:        false,
				TrustScore:        0,
				RegisteredByAgent: agentID, // nil when linkage dropped — verified below
			}, nil
		},
	}

	mockAgentRepo := &MockAgentRepositoryerImpl{
		GetByIDFunc: func(id uuid.UUID) (*domain.Agent, error) {
			// Simulate a real cross-org agent — exists in the DB but in a
			// different org. The handler must reject the linkage based on
			// the OrganizationID mismatch.
			if id == crossOrgAgentID {
				return &domain.Agent{ID: id, OrganizationID: crossOrgID}, nil
			}
			return nil, nil
		},
	}

	mockAudit := &MockAuditServiceImpl{}

	handler := NewMCPHandlerWithInterfaces(
		mockMCPService, nil, mockAudit, mockAgentRepo, nil, nil, nil,
	)

	app := fiber.New()
	app.Post("/mcp-servers", func(c fiber.Ctx) error {
		c.Locals("organization_id", callerOrgID)
		c.Locals("user_id", userID)
		return handler.CreateMCPServer(c)
	})

	body := `{"name":"x","url":"http://x.example.com","registeredByAgent":"` + crossOrgAgentID.String() + `"}`
	req := httptest.NewRequest("POST", "/mcp-servers", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Registration itself is otherwise valid; the cross-org body field is
	// just silently dropped, so the 201 contract is preserved.
	assert.Equal(t, fiber.StatusCreated, resp.StatusCode, "MCP registration with cross-org body field should still succeed (silent linkage drop)")

	bodyBytes, _ := io.ReadAll(resp.Body)
	bodyStr := string(bodyBytes)
	assert.NotContains(t, bodyStr, crossOrgAgentID.String(), "response body must not echo the cross-org agent UUID supplied in the request body")
	assert.NotContains(t, bodyStr, crossOrgID.String(), "response body must not echo the cross-org organization UUID")
}

// SECURITY (defect #18): same-org `registeredByAgent` must be HONORED —
// the linkage flows through and `registeredByAgent` IS present in the
// response. This is the legitimate SDK use case; the redaction guard
// must not over-apply.
func TestMCPHandler_CreateMCPServer_SameOrgRegisteredByAgent_LinkageHonored(t *testing.T) {
	callerOrgID := uuid.New()
	userID := uuid.New()
	sameOrgAgentID := uuid.New()
	var receivedAgentID *uuid.UUID

	mockMCPService := &MockMCPServiceExtendedImpl{
		CreateMCPServerFunc: func(ctx context.Context, req *application.CreateMCPServerRequest, orgID, uID uuid.UUID, agentID *uuid.UUID, sdkTokenID *uuid.UUID, apiKeyID *uuid.UUID) (*domain.MCPServer, error) {
			receivedAgentID = agentID
			return &domain.MCPServer{
				ID:                uuid.New(),
				OrganizationID:    orgID,
				Name:              req.Name,
				URL:               req.URL,
				Status:            domain.MCPServerStatusVerified,
				IsVerified:        true,
				TrustScore:        75,
				RegisteredByAgent: agentID,
			}, nil
		},
	}

	mockAgentRepo := &MockAgentRepositoryerImpl{
		GetByIDFunc: func(id uuid.UUID) (*domain.Agent, error) {
			if id == sameOrgAgentID {
				return &domain.Agent{ID: id, OrganizationID: callerOrgID}, nil
			}
			return nil, nil
		},
	}

	mockAudit := &MockAuditServiceImpl{}

	handler := NewMCPHandlerWithInterfaces(
		mockMCPService, nil, mockAudit, mockAgentRepo, nil, nil, nil,
	)

	app := fiber.New()
	app.Post("/mcp-servers", func(c fiber.Ctx) error {
		c.Locals("organization_id", callerOrgID)
		c.Locals("user_id", userID)
		return handler.CreateMCPServer(c)
	})

	body := `{"name":"x","url":"http://x.example.com","registeredByAgent":"` + sameOrgAgentID.String() + `"}`
	req := httptest.NewRequest("POST", "/mcp-servers", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusCreated, resp.StatusCode)
	require.NotNil(t, receivedAgentID, "service must be called with the same-org agentID (linkage honored)")
	assert.Equal(t, sameOrgAgentID, *receivedAgentID, "service must receive the parsed same-org agentID")

	bodyBytes, _ := io.ReadAll(resp.Body)
	bodyStr := string(bodyBytes)
	assert.Contains(t, bodyStr, sameOrgAgentID.String(), "response body must echo the same-org agent UUID when linkage is honored")
}

// ===========================
// NewMCPHandlerWithInterfaces Tests
// ===========================

func TestNewMCPHandlerWithInterfaces_NilDeps(t *testing.T) {
	handler := NewMCPHandlerWithInterfaces(nil, nil, nil, nil, nil, nil, nil)
	assert.NotNil(t, handler)
}

func TestNewMCPHandlerWithInterfaces_WithMocks(t *testing.T) {
	mockMCP := &MockMCPServiceExtendedImpl{}
	mockCap := &MockMCPCapabilityServiceImpl{}
	mockAudit := &MockAuditServiceImpl{}
	mockAgentRepo := &MockAgentRepositoryerImpl{}
	mockVerifRepo := &MockVerificationEventRepositoryerImpl{}
	mockTag := &MockTagServiceImpl{}
	mockAttest := &MockMCPAttestationServiceExtendedImpl{}

	handler := NewMCPHandlerWithInterfaces(
		mockMCP, mockCap, mockAudit, mockAgentRepo,
		mockVerifRepo, mockTag, mockAttest,
	)

	assert.NotNil(t, handler)
	assert.NotNil(t, handler.mcpServicer)
	assert.NotNil(t, handler.mcpCapabilityServicer)
	assert.NotNil(t, handler.auditServicer)
	assert.NotNil(t, handler.agentRepositoryer)
	assert.NotNil(t, handler.verificationEventRepositoryer)
	assert.NotNil(t, handler.tagServicer)
	assert.NotNil(t, handler.attestationServicer)
}
