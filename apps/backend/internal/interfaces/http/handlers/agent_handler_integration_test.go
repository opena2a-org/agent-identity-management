package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
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

// Helper to create a test app with authentication context
func createTestAppWithAuth(handler *AgentHandler, orgID, userID uuid.UUID) *fiber.App {
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		return c.Next()
	})
	return app
}

// ===========================
// AgentHandler.ListAgents Tests with Mocks
// ===========================

func TestAgentHandler_ListAgents_Success(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()

	mockAgentService := &MockAgentServiceImpl{
		ListAgentsFunc: func(ctx context.Context, id uuid.UUID) ([]*domain.Agent, error) {
			return []*domain.Agent{
				{ID: uuid.New(), OrganizationID: orgID, Name: "agent1", Status: domain.AgentStatusVerified},
				{ID: uuid.New(), OrganizationID: orgID, Name: "agent2", Status: domain.AgentStatusPending},
			}, nil
		},
	}

	mockCapabilityService := &MockCapabilityServiceImpl{
		GetAgentCapabilitiesFunc: func(ctx context.Context, agentID uuid.UUID, activeOnly bool) ([]*domain.AgentCapability, error) {
			return []*domain.AgentCapability{}, nil
		},
	}

	mockTagService := &MockTagServiceImpl{
		GetAgentTagsFunc: func(ctx context.Context, agentID uuid.UUID) ([]*domain.Tag, error) {
			return []*domain.Tag{}, nil
		},
	}

	mockAttestationService := &MockMCPAttestationServiceImpl{
		GetMCPServersForAgentFunc: func(ctx context.Context, agentID uuid.UUID) ([]*domain.MCPServer, error) {
			return []*domain.MCPServer{}, nil
		},
	}

	handler := NewAgentHandlerWithInterfaces(
		mockAgentService,
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{},
		&MockAPIKeyServiceImpl{},
		nil, // trustScoreHandler
		&MockAlertServiceImpl{},
		&MockVerificationEventServiceImpl{},
		mockCapabilityService,
		mockTagService,
		&MockOrganizationRepository{},
		mockAttestationService,
	)

	app := createTestAppWithAuth(handler, orgID, userID)
	app.Get("/agents", handler.ListAgents)

	req := httptest.NewRequest("GET", "/agents", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	agents, ok := result["agents"].([]interface{})
	assert.True(t, ok)
	assert.Len(t, agents, 2)
}

func TestAgentHandler_ListAgents_ServiceError(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()

	mockAgentService := &MockAgentServiceImpl{
		ListAgentsFunc: func(ctx context.Context, id uuid.UUID) ([]*domain.Agent, error) {
			return nil, errors.New("database connection failed")
		},
	}

	handler := NewAgentHandlerWithInterfaces(
		mockAgentService,
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{},
		&MockAPIKeyServiceImpl{},
		nil,
		&MockAlertServiceImpl{},
		&MockVerificationEventServiceImpl{},
		&MockCapabilityServiceImpl{},
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		&MockMCPAttestationServiceImpl{},
	)

	app := createTestAppWithAuth(handler, orgID, userID)
	app.Get("/agents", handler.ListAgents)

	req := httptest.NewRequest("GET", "/agents", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

// ===========================
// AgentHandler.GetAgent Tests with Mocks
// ===========================

func TestAgentHandler_GetAgent_Success(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()

	mockAgentService := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{
				ID:             agentID,
				OrganizationID: orgID,
				Name:           "test-agent",
				Status:         domain.AgentStatusVerified,
				TrustScore:     85.5,
			}, nil
		},
	}

	mockCapabilityService := &MockCapabilityServiceImpl{
		GetAgentCapabilitiesFunc: func(ctx context.Context, agentID uuid.UUID, activeOnly bool) ([]*domain.AgentCapability, error) {
			return []*domain.AgentCapability{
				{ID: uuid.New(), AgentID: agentID, CapabilityType: "file:read"},
			}, nil
		},
	}

	mockTagService := &MockTagServiceImpl{
		GetAgentTagsFunc: func(ctx context.Context, agentID uuid.UUID) ([]*domain.Tag, error) {
			return []*domain.Tag{
				{ID: uuid.New(), Key: "env", Value: "production", Color: "#00FF00"},
			}, nil
		},
	}

	mockAttestationService := &MockMCPAttestationServiceImpl{
		GetMCPServersForAgentFunc: func(ctx context.Context, agentID uuid.UUID) ([]*domain.MCPServer, error) {
			return []*domain.MCPServer{}, nil
		},
	}

	handler := NewAgentHandlerWithInterfaces(
		mockAgentService,
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{},
		&MockAPIKeyServiceImpl{},
		nil,
		&MockAlertServiceImpl{},
		&MockVerificationEventServiceImpl{},
		mockCapabilityService,
		mockTagService,
		&MockOrganizationRepository{},
		mockAttestationService,
	)

	app := createTestAppWithAuth(handler, orgID, userID)
	app.Get("/agents/:id", handler.GetAgent)

	req := httptest.NewRequest("GET", "/agents/"+agentID.String(), nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	assert.Equal(t, agentID.String(), result["id"])
	assert.Equal(t, "test-agent", result["name"])
}

func TestAgentHandler_GetAgent_NotFound(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()

	mockAgentService := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return nil, errors.New("agent not found")
		},
	}

	handler := NewAgentHandlerWithInterfaces(
		mockAgentService,
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{},
		&MockAPIKeyServiceImpl{},
		nil,
		&MockAlertServiceImpl{},
		&MockVerificationEventServiceImpl{},
		&MockCapabilityServiceImpl{},
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		&MockMCPAttestationServiceImpl{},
	)

	app := createTestAppWithAuth(handler, orgID, userID)
	app.Get("/agents/:id", handler.GetAgent)

	req := httptest.NewRequest("GET", "/agents/"+agentID.String(), nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

func TestAgentHandler_GetAgent_WrongOrganization(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()
	otherOrgID := uuid.New()

	mockAgentService := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{
				ID:             agentID,
				OrganizationID: otherOrgID, // Different org
				Name:           "test-agent",
			}, nil
		},
	}

	handler := NewAgentHandlerWithInterfaces(
		mockAgentService,
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{},
		&MockAPIKeyServiceImpl{},
		nil,
		&MockAlertServiceImpl{},
		&MockVerificationEventServiceImpl{},
		&MockCapabilityServiceImpl{},
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		&MockMCPAttestationServiceImpl{},
	)

	app := createTestAppWithAuth(handler, orgID, userID)
	app.Get("/agents/:id", handler.GetAgent)

	req := httptest.NewRequest("GET", "/agents/"+agentID.String(), nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// A3b: cross-tenant access returns 404 (not 403) to avoid leaking
	// resource existence across orgs. See tenant_scope.go:41-46.
	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.JSONEq(t, `{"error":"not found"}`, string(body))
}

// ===========================
// AgentHandler.CreateAgent Tests with Mocks
// ===========================

func TestAgentHandler_CreateAgent_Success(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	newAgentID := uuid.New()
	publicKey := "test-public-key"

	mockAgentService := &MockAgentServiceImpl{
		CreateAgentFunc: func(ctx context.Context, req *application.CreateAgentRequest, orgID, userID uuid.UUID, sdkTokenID *uuid.UUID, apiKeyID *uuid.UUID, userEmail string) (*domain.Agent, error) {
			return &domain.Agent{
				ID:             newAgentID,
				OrganizationID: orgID,
				Name:           req.Name,
				DisplayName:    req.DisplayName,
				Status:         domain.AgentStatusPending,
				TrustScore:     50.0,
				PublicKey:      &publicKey,
			}, nil
		},
		GetAgentCredentialsFunc: func(ctx context.Context, agentID uuid.UUID) (publicKey, privateKey string, err error) {
			return "test-public-key", "test-private-key", nil
		},
	}

	mockAuditService := &MockAuditServiceImpl{}

	handler := NewAgentHandlerWithInterfaces(
		mockAgentService,
		&MockMCPServiceImpl{},
		mockAuditService,
		&MockAPIKeyServiceImpl{},
		nil,
		&MockAlertServiceImpl{},
		&MockVerificationEventServiceImpl{},
		&MockCapabilityServiceImpl{},
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		&MockMCPAttestationServiceImpl{},
	)

	app := createTestAppWithAuth(handler, orgID, userID)
	app.Post("/agents", handler.CreateAgent)

	body := `{"name":"new-agent","displayName":"New Agent","description":"Test agent","agentType":"claude"}`
	req := httptest.NewRequest("POST", "/agents", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusCreated, resp.StatusCode)

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(respBody, &result)

	assert.Equal(t, newAgentID.String(), result["id"])
	assert.Equal(t, "new-agent", result["name"])
}

func TestAgentHandler_CreateAgent_InvalidJSON_WithMocks(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()

	handler := NewAgentHandlerWithInterfaces(
		&MockAgentServiceImpl{},
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{},
		&MockAPIKeyServiceImpl{},
		nil,
		&MockAlertServiceImpl{},
		&MockVerificationEventServiceImpl{},
		&MockCapabilityServiceImpl{},
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		&MockMCPAttestationServiceImpl{},
	)

	app := createTestAppWithAuth(handler, orgID, userID)
	app.Post("/agents", handler.CreateAgent)

	req := httptest.NewRequest("POST", "/agents", strings.NewReader("not valid json"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestAgentHandler_CreateAgent_MissingName(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()

	// Mock service returns error when name is missing (validation in service layer)
	mockAgentService := &MockAgentServiceImpl{
		CreateAgentFunc: func(ctx context.Context, req *application.CreateAgentRequest, orgID, userID uuid.UUID, sdkTokenID *uuid.UUID, apiKeyID *uuid.UUID, userEmail string) (*domain.Agent, error) {
			if req.Name == "" {
				return nil, errors.New("name is required")
			}
			return nil, errors.New("unexpected call")
		},
	}

	handler := NewAgentHandlerWithInterfaces(
		mockAgentService,
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{},
		&MockAPIKeyServiceImpl{},
		nil,
		&MockAlertServiceImpl{},
		&MockVerificationEventServiceImpl{},
		&MockCapabilityServiceImpl{},
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		&MockMCPAttestationServiceImpl{},
	)

	app := createTestAppWithAuth(handler, orgID, userID)
	app.Post("/agents", handler.CreateAgent)

	body := `{"displayName":"Test","description":"A test","agentType":"claude"}`
	req := httptest.NewRequest("POST", "/agents", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Service validation returns error, handler returns 500 (internal server error)
	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

// ===========================
// AgentHandler.DeleteAgent Tests with Mocks
// ===========================

func TestAgentHandler_DeleteAgent_Success(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()

	mockAgentService := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{
				ID:             agentID,
				OrganizationID: orgID,
				Name:           "agent-to-delete",
			}, nil
		},
		DeleteAgentFunc: func(ctx context.Context, id uuid.UUID) error {
			return nil
		},
	}

	handler := NewAgentHandlerWithInterfaces(
		mockAgentService,
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{},
		&MockAPIKeyServiceImpl{},
		nil,
		&MockAlertServiceImpl{},
		&MockVerificationEventServiceImpl{},
		&MockCapabilityServiceImpl{},
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		&MockMCPAttestationServiceImpl{},
	)

	app := createTestAppWithAuth(handler, orgID, userID)
	app.Delete("/agents/:id", handler.DeleteAgent)

	req := httptest.NewRequest("DELETE", "/agents/"+agentID.String(), nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusNoContent, resp.StatusCode)
}

func TestAgentHandler_DeleteAgent_NotFound(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()

	mockAgentService := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return nil, errors.New("not found")
		},
	}

	handler := NewAgentHandlerWithInterfaces(
		mockAgentService,
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{},
		&MockAPIKeyServiceImpl{},
		nil,
		&MockAlertServiceImpl{},
		&MockVerificationEventServiceImpl{},
		&MockCapabilityServiceImpl{},
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		&MockMCPAttestationServiceImpl{},
	)

	app := createTestAppWithAuth(handler, orgID, userID)
	app.Delete("/agents/:id", handler.DeleteAgent)

	req := httptest.NewRequest("DELETE", "/agents/"+agentID.String(), nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

// ===========================
// AgentHandler.VerifyAgent Tests with Mocks
// ===========================

func TestAgentHandler_VerifyAgent_Success(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()

	mockAgentService := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{
				ID:             agentID,
				OrganizationID: orgID,
				Name:           "agent-to-verify",
				Status:         domain.AgentStatusPending,
			}, nil
		},
		VerifyAgentFunc: func(ctx context.Context, id uuid.UUID) error {
			return nil
		},
	}

	handler := NewAgentHandlerWithInterfaces(
		mockAgentService,
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{},
		&MockAPIKeyServiceImpl{},
		nil,
		&MockAlertServiceImpl{},
		&MockVerificationEventServiceImpl{},
		&MockCapabilityServiceImpl{},
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		&MockMCPAttestationServiceImpl{},
	)

	app := createTestAppWithAuth(handler, orgID, userID)
	app.Post("/agents/:id/verify", handler.VerifyAgent)

	req := httptest.NewRequest("POST", "/agents/"+agentID.String()+"/verify", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

// ===========================
// AgentHandler.SuspendAgent Tests with Mocks
// ===========================

func TestAgentHandler_SuspendAgent_Success(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()

	mockAgentService := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{
				ID:             agentID,
				OrganizationID: orgID,
				Name:           "agent-to-suspend",
				Status:         domain.AgentStatusVerified,
			}, nil
		},
		SuspendAgentFunc: func(ctx context.Context, id uuid.UUID) error {
			return nil
		},
	}

	handler := NewAgentHandlerWithInterfaces(
		mockAgentService,
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{},
		&MockAPIKeyServiceImpl{},
		nil,
		&MockAlertServiceImpl{},
		&MockVerificationEventServiceImpl{},
		&MockCapabilityServiceImpl{},
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		&MockMCPAttestationServiceImpl{},
	)

	app := createTestAppWithAuth(handler, orgID, userID)
	app.Post("/agents/:id/suspend", handler.SuspendAgent)

	req := httptest.NewRequest("POST", "/agents/"+agentID.String()+"/suspend", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

// ===========================
// AgentHandler.ReactivateAgent Tests with Mocks
// ===========================

func TestAgentHandler_ReactivateAgent_Success(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()

	mockAgentService := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{
				ID:             agentID,
				OrganizationID: orgID,
				Name:           "agent-to-reactivate",
				Status:         domain.AgentStatusSuspended,
			}, nil
		},
		ReactivateAgentFunc: func(ctx context.Context, id uuid.UUID) error {
			return nil
		},
	}

	handler := NewAgentHandlerWithInterfaces(
		mockAgentService,
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{},
		&MockAPIKeyServiceImpl{},
		nil,
		&MockAlertServiceImpl{},
		&MockVerificationEventServiceImpl{},
		&MockCapabilityServiceImpl{},
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		&MockMCPAttestationServiceImpl{},
	)

	app := createTestAppWithAuth(handler, orgID, userID)
	app.Post("/agents/:id/reactivate", handler.ReactivateAgent)

	req := httptest.NewRequest("POST", "/agents/"+agentID.String()+"/reactivate", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

// ===========================
// AgentHandler.RotateCredentials Tests with Mocks
// ===========================

func TestAgentHandler_RotateCredentials_Success(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()

	mockAgentService := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{
				ID:             agentID,
				OrganizationID: orgID,
				Name:           "agent-to-rotate",
			}, nil
		},
		RotateCredentialsFunc: func(ctx context.Context, id uuid.UUID) (string, string, error) {
			return "new-public-key", "new-private-key", nil
		},
	}

	handler := NewAgentHandlerWithInterfaces(
		mockAgentService,
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{},
		&MockAPIKeyServiceImpl{},
		nil,
		&MockAlertServiceImpl{},
		&MockVerificationEventServiceImpl{},
		&MockCapabilityServiceImpl{},
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		&MockMCPAttestationServiceImpl{},
	)

	app := createTestAppWithAuth(handler, orgID, userID)
	app.Post("/agents/:id/rotate-credentials", handler.RotateCredentials)

	req := httptest.NewRequest("POST", "/agents/"+agentID.String()+"/rotate-credentials", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	assert.Equal(t, "new-public-key", result["publicKey"])
	assert.Equal(t, "new-private-key", result["privateKey"])
}

// ===========================
// AgentHandler.VerifyCapability Tests with Mocks
// ===========================

func TestAgentHandler_VerifyCapability_Allowed(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()
	auditID := uuid.New()

	mockAgentService := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{
				ID:             agentID,
				OrganizationID: orgID,
				Name:           "test-agent",
				Status:         domain.AgentStatusVerified,
			}, nil
		},
		VerifyCapabilityFunc: func(ctx context.Context, agentID uuid.UUID, capability string, resource string, metadata map[string]interface{}, sourceIP string) (bool, string, uuid.UUID, error) {
			return true, "capability verified", auditID, nil
		},
	}

	handler := NewAgentHandlerWithInterfaces(
		mockAgentService,
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{},
		&MockAPIKeyServiceImpl{},
		nil,
		&MockAlertServiceImpl{},
		&MockVerificationEventServiceImpl{},
		&MockCapabilityServiceImpl{},
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		&MockMCPAttestationServiceImpl{},
	)

	app := createTestAppWithAuth(handler, orgID, userID)
	app.Post("/agents/:id/verify-capability", handler.VerifyCapability)

	body := `{"capability":"file:read","resource":"/tmp/test.txt"}`
	req := httptest.NewRequest("POST", "/agents/"+agentID.String()+"/verify-capability", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(respBody, &result)

	assert.Equal(t, true, result["allowed"])
}

func TestAgentHandler_VerifyCapability_Denied(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()
	auditID := uuid.New()

	mockAgentService := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{
				ID:             agentID,
				OrganizationID: orgID,
				Name:           "test-agent",
				Status:         domain.AgentStatusVerified,
			}, nil
		},
		VerifyCapabilityFunc: func(ctx context.Context, agentID uuid.UUID, capability string, resource string, metadata map[string]interface{}, sourceIP string) (bool, string, uuid.UUID, error) {
			return false, "capability not granted", auditID, nil
		},
	}

	handler := NewAgentHandlerWithInterfaces(
		mockAgentService,
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{},
		&MockAPIKeyServiceImpl{},
		nil,
		&MockAlertServiceImpl{},
		&MockVerificationEventServiceImpl{},
		&MockCapabilityServiceImpl{},
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		&MockMCPAttestationServiceImpl{},
	)

	app := createTestAppWithAuth(handler, orgID, userID)
	app.Post("/agents/:id/verify-capability", handler.VerifyCapability)

	body := `{"capability":"file:write","resource":"/etc/passwd"}`
	req := httptest.NewRequest("POST", "/agents/"+agentID.String()+"/verify-capability", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Handler returns 403 Forbidden when capability is denied
	assert.Equal(t, fiber.StatusForbidden, resp.StatusCode)

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(respBody, &result)

	assert.Equal(t, false, result["allowed"])
}

// ===========================
// enrichAgentResponse Tests with Mocks
// ===========================

func TestEnrichAgentResponse_WithCapabilities(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()

	mockCapabilityService := &MockCapabilityServiceImpl{
		GetAgentCapabilitiesFunc: func(ctx context.Context, id uuid.UUID, activeOnly bool) ([]*domain.AgentCapability, error) {
			return []*domain.AgentCapability{
				{ID: uuid.New(), AgentID: agentID, CapabilityType: "file:read"},
				{ID: uuid.New(), AgentID: agentID, CapabilityType: "file:write"},
			}, nil
		},
	}

	mockTagService := &MockTagServiceImpl{
		GetAgentTagsFunc: func(ctx context.Context, id uuid.UUID) ([]*domain.Tag, error) {
			return []*domain.Tag{
				{ID: uuid.New(), Key: "env", Value: "production", Color: "#00FF00"},
			}, nil
		},
	}

	handler := NewAgentHandlerWithInterfaces(
		&MockAgentServiceImpl{},
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{},
		&MockAPIKeyServiceImpl{},
		nil,
		&MockAlertServiceImpl{},
		&MockVerificationEventServiceImpl{},
		mockCapabilityService,
		mockTagService,
		&MockOrganizationRepository{},
		&MockMCPAttestationServiceImpl{},
	)

	app := createTestAppWithAuth(handler, orgID, userID)
	app.Get("/test", func(c fiber.Ctx) error {
		agent := &domain.Agent{
			ID:             agentID,
			OrganizationID: orgID,
			Name:           "test-agent",
			DisplayName:    "Test Agent",
			Status:         domain.AgentStatusVerified,
			TrustScore:     85.0,
			CreatedAt:      time.Now(),
		}
		result := handler.enrichAgentResponse(c, agent)
		return c.JSON(result)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	// Check capabilities are enriched
	capabilities, ok := result["capabilities"].([]interface{})
	assert.True(t, ok)
	assert.Len(t, capabilities, 2)

	// Check tags are enriched
	tags, ok := result["tags"].([]interface{})
	assert.True(t, ok)
	assert.Len(t, tags, 1)

	// Check basic agent fields are present
	assert.Equal(t, "test-agent", result["name"])
	assert.Equal(t, "Test Agent", result["displayName"])
}

// ===========================
// AgentHandler.AddMCPServersToAgent Tests with Mocks
// ===========================

func TestAgentHandler_AddMCPServersToAgent_Success(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()

	mockAgentService := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{
				ID:             agentID,
				OrganizationID: orgID,
				Name:           "test-agent",
			}, nil
		},
		AddMCPServersFunc: func(ctx context.Context, agentID uuid.UUID, mcpServerIdentifiers []string) (*domain.Agent, []string, error) {
			return &domain.Agent{
				ID:      agentID,
				TalksTo: mcpServerIdentifiers,
			}, mcpServerIdentifiers, nil
		},
	}

	handler := NewAgentHandlerWithInterfaces(
		mockAgentService,
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{},
		&MockAPIKeyServiceImpl{},
		nil,
		&MockAlertServiceImpl{},
		&MockVerificationEventServiceImpl{},
		&MockCapabilityServiceImpl{},
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		&MockMCPAttestationServiceImpl{},
	)

	app := createTestAppWithAuth(handler, orgID, userID)
	app.Put("/agents/:id/mcp-servers", handler.AddMCPServersToAgent)

	// Use correct field name: mcpServerIds (camelCase from application.AddMCPServersRequest)
	body := `{"mcpServerIds":["mcp-server-1","mcp-server-2"]}`
	req := httptest.NewRequest("PUT", "/agents/"+agentID.String()+"/mcp-servers", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

// ===========================
// AgentHandler.RemoveMCPServerFromAgent Tests with Mocks
// ===========================

func TestAgentHandler_RemoveMCPServerFromAgent_Success(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()
	mcpServerID := uuid.New()

	mockAgentService := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{
				ID:             agentID,
				OrganizationID: orgID,
				Name:           "test-agent",
			}, nil
		},
		RemoveMCPServerFunc: func(ctx context.Context, agentID uuid.UUID, mcpServerIdentifier string) (*domain.Agent, error) {
			return &domain.Agent{
				ID:      agentID,
				TalksTo: []string{},
			}, nil
		},
	}

	handler := NewAgentHandlerWithInterfaces(
		mockAgentService,
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{},
		&MockAPIKeyServiceImpl{},
		nil,
		&MockAlertServiceImpl{},
		&MockVerificationEventServiceImpl{},
		&MockCapabilityServiceImpl{},
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		&MockMCPAttestationServiceImpl{},
	)

	app := createTestAppWithAuth(handler, orgID, userID)
	// Use correct param name: mcp_id (from handler.RemoveMCPServerFromAgent)
	app.Delete("/agents/:id/mcp-servers/:mcp_id", handler.RemoveMCPServerFromAgent)

	req := httptest.NewRequest("DELETE", "/agents/"+agentID.String()+"/mcp-servers/"+mcpServerID.String(), nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

// ===========================
// NewAgentHandlerWithInterfaces Tests
// ===========================

func TestNewAgentHandlerWithInterfaces(t *testing.T) {
	handler := NewAgentHandlerWithInterfaces(
		&MockAgentServiceImpl{},
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{},
		&MockAPIKeyServiceImpl{},
		nil,
		&MockAlertServiceImpl{},
		&MockVerificationEventServiceImpl{},
		&MockCapabilityServiceImpl{},
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		&MockMCPAttestationServiceImpl{},
	)

	assert.NotNil(t, handler)
	assert.Nil(t, handler.mcpServiceConcrete) // Should be nil when using interfaces
}

// ===========================
// AgentHandler.UpdateAgent Tests with Mocks
// ===========================

func TestAgentHandler_UpdateAgent_Success(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()

	mockAgentService := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{
				ID:             agentID,
				OrganizationID: orgID,
				Name:           "original-agent",
			}, nil
		},
		UpdateAgentFunc: func(ctx context.Context, id uuid.UUID, req *application.CreateAgentRequest, requestedBy uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{
				ID:             agentID,
				OrganizationID: orgID,
				Name:           req.Name,
				DisplayName:    req.DisplayName,
			}, nil
		},
	}

	handler := NewAgentHandlerWithInterfaces(
		mockAgentService,
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{},
		&MockAPIKeyServiceImpl{},
		nil,
		&MockAlertServiceImpl{},
		&MockVerificationEventServiceImpl{},
		&MockCapabilityServiceImpl{},
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		&MockMCPAttestationServiceImpl{},
	)

	app := createTestAppWithAuth(handler, orgID, userID)
	app.Put("/agents/:id", handler.UpdateAgent)

	body := `{"name":"updated-agent","displayName":"Updated Agent"}`
	req := httptest.NewRequest("PUT", "/agents/"+agentID.String(), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(respBody, &result)

	assert.Equal(t, "updated-agent", result["name"])
}

func TestAgentHandler_UpdateAgent_NotFound(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()

	mockAgentService := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return nil, errors.New("not found")
		},
	}

	handler := NewAgentHandlerWithInterfaces(
		mockAgentService,
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{},
		&MockAPIKeyServiceImpl{},
		nil,
		&MockAlertServiceImpl{},
		&MockVerificationEventServiceImpl{},
		&MockCapabilityServiceImpl{},
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		&MockMCPAttestationServiceImpl{},
	)

	app := createTestAppWithAuth(handler, orgID, userID)
	app.Put("/agents/:id", handler.UpdateAgent)

	body := `{"name":"updated-agent"}`
	req := httptest.NewRequest("PUT", "/agents/"+agentID.String(), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

// ===========================
// AgentHandler.GetAgentActivity Tests with Mocks
// ===========================

func TestAgentHandler_GetAgentActivity_Success(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()

	mockAgentService := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{
				ID:             agentID,
				OrganizationID: orgID,
				Name:           "test-agent",
			}, nil
		},
	}

	mockAuditService := &MockAuditServiceImpl{
		GetAgentActivityFunc: func(ctx context.Context, orgID, agentID uuid.UUID, limit, offset int) ([]*domain.AuditLog, error) {
			return []*domain.AuditLog{
				{ID: uuid.New(), Action: domain.AuditActionCreate, ResourceType: "agent"},
				{ID: uuid.New(), Action: domain.AuditActionUpdate, ResourceType: "agent"},
			}, nil
		},
	}

	handler := NewAgentHandlerWithInterfaces(
		mockAgentService,
		&MockMCPServiceImpl{},
		mockAuditService,
		&MockAPIKeyServiceImpl{},
		nil,
		&MockAlertServiceImpl{},
		&MockVerificationEventServiceImpl{},
		&MockCapabilityServiceImpl{},
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		&MockMCPAttestationServiceImpl{},
	)

	app := createTestAppWithAuth(handler, orgID, userID)
	app.Get("/agents/:id/activity", handler.GetAgentActivity)

	req := httptest.NewRequest("GET", "/agents/"+agentID.String()+"/activity", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	activities, ok := result["activities"].([]interface{})
	assert.True(t, ok)
	assert.Len(t, activities, 2)
}

// ===========================
// AgentHandler.GetAgentAlerts Tests with Mocks
// ===========================

func TestAgentHandler_GetAgentAlerts_Success(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()

	mockAgentService := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{
				ID:             agentID,
				OrganizationID: orgID,
				Name:           "test-agent",
			}, nil
		},
	}

	mockAlertService := &MockAlertServiceImpl{
		GetAlertsByAgentFunc: func(ctx context.Context, agentID uuid.UUID, limit, offset int) ([]*domain.Alert, error) {
			return []*domain.Alert{
				{ID: uuid.New(), Title: "Alert 1", Severity: domain.AlertSeverityWarning},
				{ID: uuid.New(), Title: "Alert 2", Severity: domain.AlertSeverityHigh},
			}, nil
		},
	}

	handler := NewAgentHandlerWithInterfaces(
		mockAgentService,
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{},
		&MockAPIKeyServiceImpl{},
		nil,
		mockAlertService,
		&MockVerificationEventServiceImpl{},
		&MockCapabilityServiceImpl{},
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		&MockMCPAttestationServiceImpl{},
	)

	app := createTestAppWithAuth(handler, orgID, userID)
	app.Get("/agents/:id/alerts", handler.GetAgentAlerts)

	req := httptest.NewRequest("GET", "/agents/"+agentID.String()+"/alerts", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	alerts, ok := result["alerts"].([]interface{})
	assert.True(t, ok)
	assert.Len(t, alerts, 2)
}

// ===========================
// AgentHandler.UpdateAgentTrustScore Tests with Mocks
// ===========================

func TestAgentHandler_UpdateAgentTrustScore_Success(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()

	mockAgentService := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{
				ID:             agentID,
				OrganizationID: orgID,
				Name:           "test-agent",
				TrustScore:     85.0,
			}, nil
		},
		UpdateTrustScoreFunc: func(ctx context.Context, agentID uuid.UUID, newScore float64) error {
			return nil
		},
	}

	handler := NewAgentHandlerWithInterfaces(
		mockAgentService,
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{},
		&MockAPIKeyServiceImpl{},
		nil,
		&MockAlertServiceImpl{},
		&MockVerificationEventServiceImpl{},
		&MockCapabilityServiceImpl{},
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		&MockMCPAttestationServiceImpl{},
	)

	app := createTestAppWithAuth(handler, orgID, userID)
	app.Put("/agents/:id/trust-score", handler.UpdateAgentTrustScore)

	body := `{"score":9.5,"reason":"Manual override for testing"}`
	req := httptest.NewRequest("PUT", "/agents/"+agentID.String()+"/trust-score", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestAgentHandler_UpdateAgentTrustScore_InvalidScore(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()

	mockAgentService := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{
				ID:             agentID,
				OrganizationID: orgID,
				Name:           "test-agent",
			}, nil
		},
	}

	handler := NewAgentHandlerWithInterfaces(
		mockAgentService,
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{},
		&MockAPIKeyServiceImpl{},
		nil,
		&MockAlertServiceImpl{},
		&MockVerificationEventServiceImpl{},
		&MockCapabilityServiceImpl{},
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		&MockMCPAttestationServiceImpl{},
	)

	app := createTestAppWithAuth(handler, orgID, userID)
	app.Put("/agents/:id/trust-score", handler.UpdateAgentTrustScore)

	// Score out of range (> 100.0)
	body := `{"score":150.0,"reason":"Invalid score"}`
	req := httptest.NewRequest("PUT", "/agents/"+agentID.String()+"/trust-score", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// AgentHandler.UpdateAgentKeys Tests with Mocks
// ===========================

func TestAgentHandler_UpdateAgentKeys_Success(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()
	publicKey := "new-public-key"

	mockAgentService := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{
				ID:             agentID,
				OrganizationID: orgID,
				Name:           "test-agent",
				PublicKey:      &publicKey,
			}, nil
		},
		UpdateAgentPublicKeyFunc: func(ctx context.Context, agentID uuid.UUID, publicKey string) error {
			return nil
		},
	}

	handler := NewAgentHandlerWithInterfaces(
		mockAgentService,
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{},
		&MockAPIKeyServiceImpl{},
		nil,
		&MockAlertServiceImpl{},
		&MockVerificationEventServiceImpl{},
		&MockCapabilityServiceImpl{},
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		&MockMCPAttestationServiceImpl{},
	)

	app := createTestAppWithAuth(handler, orgID, userID)
	app.Put("/agents/:id/keys", handler.UpdateAgentKeys)

	body := `{"publicKey":"base64-encoded-public-key"}`
	req := httptest.NewRequest("PUT", "/agents/"+agentID.String()+"/keys", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestAgentHandler_UpdateAgentKeys_MissingPublicKey(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()

	mockAgentService := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{
				ID:             agentID,
				OrganizationID: orgID,
				Name:           "test-agent",
			}, nil
		},
	}

	handler := NewAgentHandlerWithInterfaces(
		mockAgentService,
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{},
		&MockAPIKeyServiceImpl{},
		nil,
		&MockAlertServiceImpl{},
		&MockVerificationEventServiceImpl{},
		&MockCapabilityServiceImpl{},
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		&MockMCPAttestationServiceImpl{},
	)

	app := createTestAppWithAuth(handler, orgID, userID)
	app.Put("/agents/:id/keys", handler.UpdateAgentKeys)

	body := `{}`
	req := httptest.NewRequest("PUT", "/agents/"+agentID.String()+"/keys", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// AgentHandler.GetAgentByIdentifier Tests with Mocks
// ===========================

func TestAgentHandler_GetAgentByIdentifier_ByUUID(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()

	mockAgentService := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{
				ID:             agentID,
				OrganizationID: orgID,
				Name:           "test-agent",
				DisplayName:    "Test Agent",
			}, nil
		},
	}

	mockCapabilityService := &MockCapabilityServiceImpl{
		GetAgentCapabilitiesFunc: func(ctx context.Context, agentID uuid.UUID, activeOnly bool) ([]*domain.AgentCapability, error) {
			return []*domain.AgentCapability{}, nil
		},
	}

	handler := NewAgentHandlerWithInterfaces(
		mockAgentService,
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{},
		&MockAPIKeyServiceImpl{},
		nil,
		&MockAlertServiceImpl{},
		&MockVerificationEventServiceImpl{},
		mockCapabilityService,
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		&MockMCPAttestationServiceImpl{},
	)

	app := createTestAppWithAuth(handler, orgID, userID)
	app.Get("/sdk-api/agents/:identifier", handler.GetAgentByIdentifier)

	req := httptest.NewRequest("GET", "/sdk-api/agents/"+agentID.String(), nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	assert.Equal(t, true, result["success"])
}

func TestAgentHandler_GetAgentByIdentifier_ByName(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()

	mockAgentService := &MockAgentServiceImpl{
		GetAgentByNameFunc: func(ctx context.Context, orgID uuid.UUID, name string) (*domain.Agent, error) {
			return &domain.Agent{
				ID:             agentID,
				OrganizationID: orgID,
				Name:           name,
				DisplayName:    "Test Agent",
			}, nil
		},
	}

	mockCapabilityService := &MockCapabilityServiceImpl{
		GetAgentCapabilitiesFunc: func(ctx context.Context, agentID uuid.UUID, activeOnly bool) ([]*domain.AgentCapability, error) {
			return []*domain.AgentCapability{}, nil
		},
	}

	handler := NewAgentHandlerWithInterfaces(
		mockAgentService,
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{},
		&MockAPIKeyServiceImpl{},
		nil,
		&MockAlertServiceImpl{},
		&MockVerificationEventServiceImpl{},
		mockCapabilityService,
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		&MockMCPAttestationServiceImpl{},
	)

	app := createTestAppWithAuth(handler, orgID, userID)
	app.Get("/sdk-api/agents/:identifier", handler.GetAgentByIdentifier)

	req := httptest.NewRequest("GET", "/sdk-api/agents/my-agent-name", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

// ===========================
// AgentHandler.GetCredentials Tests with Mocks
// ===========================

func TestAgentHandler_GetCredentials_Success(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()

	mockAgentService := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{
				ID:             agentID,
				OrganizationID: orgID,
				Name:           "test-agent",
			}, nil
		},
		GetAgentCredentialsFunc: func(ctx context.Context, agentID uuid.UUID) (publicKey, privateKey string, err error) {
			return "public-key-base64", "private-key-base64", nil
		},
	}

	handler := NewAgentHandlerWithInterfaces(
		mockAgentService,
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{},
		&MockAPIKeyServiceImpl{},
		nil,
		&MockAlertServiceImpl{},
		&MockVerificationEventServiceImpl{},
		&MockCapabilityServiceImpl{},
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		&MockMCPAttestationServiceImpl{},
	)

	app := createTestAppWithAuth(handler, orgID, userID)
	app.Get("/agents/:id/credentials", handler.GetCredentials)

	req := httptest.NewRequest("GET", "/agents/"+agentID.String()+"/credentials", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	assert.Equal(t, "public-key-base64", result["publicKey"])
	assert.Equal(t, "private-key-base64", result["privateKey"])
}

// ===========================
// AgentHandler.LogCapabilityResult Tests with Mocks
// ===========================

func TestAgentHandler_LogCapabilityResult_Success(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()
	auditID := uuid.New()

	mockAgentService := &MockAgentServiceImpl{
		// GetAgent stub satisfies the LoadOwned tenant-scope gate
		// added in this PR. The returned agent's OrganizationID
		// matches the caller's orgID so the gate passes through to
		// the success path.
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{ID: id, OrganizationID: orgID}, nil
		},
		LogCapabilityResultFunc: func(ctx context.Context, agentID uuid.UUID, auditID uuid.UUID, success bool, errorMsg string, result map[string]interface{}) error {
			return nil
		},
	}

	handler := NewAgentHandlerWithInterfaces(
		mockAgentService,
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{},
		&MockAPIKeyServiceImpl{},
		nil,
		&MockAlertServiceImpl{},
		&MockVerificationEventServiceImpl{},
		&MockCapabilityServiceImpl{},
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		&MockMCPAttestationServiceImpl{},
	)

	app := createTestAppWithAuth(handler, orgID, userID)
	app.Post("/agents/:id/log-capability/:audit_id", handler.LogCapabilityResult)

	body := `{"success":true,"result":{"data":"test"}}`
	req := httptest.NewRequest("POST", "/agents/"+agentID.String()+"/log-capability/"+auditID.String(), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestAgentHandler_LogCapabilityResult_InvalidAgentID_WithMocks(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	auditID := uuid.New()

	handler := NewAgentHandlerWithInterfaces(
		&MockAgentServiceImpl{},
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{},
		&MockAPIKeyServiceImpl{},
		nil,
		&MockAlertServiceImpl{},
		&MockVerificationEventServiceImpl{},
		&MockCapabilityServiceImpl{},
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		&MockMCPAttestationServiceImpl{},
	)

	app := createTestAppWithAuth(handler, orgID, userID)
	app.Post("/agents/:id/log-capability/:audit_id", handler.LogCapabilityResult)

	body := `{"success":true}`
	req := httptest.NewRequest("POST", "/agents/invalid-uuid/log-capability/"+auditID.String(), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// AgentHandler.GetAgentMCPServers Tests with Mocks
// ===========================

func TestAgentHandler_GetAgentMCPServers_Success(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()
	mcpServerID := uuid.New()

	mockAgentService := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{
				ID:             agentID,
				OrganizationID: orgID,
				Name:           "test-agent",
				TalksTo:        []string{mcpServerID.String()},
			}, nil
		},
	}

	mockMCPService := &MockMCPServiceImpl{
		GetMCPServerFunc: func(ctx context.Context, id uuid.UUID) (*domain.MCPServer, error) {
			return &domain.MCPServer{
				ID:             mcpServerID,
				OrganizationID: orgID,
				Name:           "test-mcp-server",
				URL:            "https://mcp.example.com",
				Status:         domain.MCPServerStatusVerified,
				Capabilities:   []string{"read", "write"},
			}, nil
		},
	}

	mockAttestationService := &MockMCPAttestationServiceImpl{
		GetMCPServersForAgentFunc: func(ctx context.Context, agentID uuid.UUID) ([]*domain.MCPServer, error) {
			return []*domain.MCPServer{}, nil
		},
	}

	handler := NewAgentHandlerWithInterfaces(
		mockAgentService,
		mockMCPService,
		&MockAuditServiceImpl{},
		&MockAPIKeyServiceImpl{},
		nil,
		&MockAlertServiceImpl{},
		&MockVerificationEventServiceImpl{},
		&MockCapabilityServiceImpl{},
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		mockAttestationService,
	)

	app := createTestAppWithAuth(handler, orgID, userID)
	app.Get("/agents/:id/mcp-servers", handler.GetAgentMCPServers)

	req := httptest.NewRequest("GET", "/agents/"+agentID.String()+"/mcp-servers", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	mcpServers, ok := result["mcpServers"].([]interface{})
	assert.True(t, ok)
	assert.Len(t, mcpServers, 1)
}

// ===========================
// Additional Edge Case Tests
// ===========================

func TestAgentHandler_InvalidAgentID_Various(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()

	handler := NewAgentHandlerWithInterfaces(
		&MockAgentServiceImpl{},
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{},
		&MockAPIKeyServiceImpl{},
		nil,
		&MockAlertServiceImpl{},
		&MockVerificationEventServiceImpl{},
		&MockCapabilityServiceImpl{},
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		&MockMCPAttestationServiceImpl{},
	)

	app := createTestAppWithAuth(handler, orgID, userID)
	app.Get("/agents/:id", handler.GetAgent)
	app.Put("/agents/:id", handler.UpdateAgent)
	app.Delete("/agents/:id", handler.DeleteAgent)
	app.Post("/agents/:id/verify", handler.VerifyAgent)
	app.Post("/agents/:id/suspend", handler.SuspendAgent)
	app.Post("/agents/:id/reactivate", handler.ReactivateAgent)
	app.Post("/agents/:id/rotate-credentials", handler.RotateCredentials)

	endpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/agents/invalid-uuid"},
		{"PUT", "/agents/invalid-uuid"},
		{"DELETE", "/agents/invalid-uuid"},
		{"POST", "/agents/invalid-uuid/verify"},
		{"POST", "/agents/invalid-uuid/suspend"},
		{"POST", "/agents/invalid-uuid/reactivate"},
		{"POST", "/agents/invalid-uuid/rotate-credentials"},
	}

	for _, ep := range endpoints {
		t.Run(ep.method+" "+ep.path, func(t *testing.T) {
			var req *http.Request
			if ep.method == "PUT" {
				req = httptest.NewRequest(ep.method, ep.path, strings.NewReader(`{}`))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequest(ep.method, ep.path, nil)
			}

			resp, err := app.Test(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
		})
	}
}

// ===========================
// GetAgentAlerts Tests with Mocks
// ===========================

func TestAgentHandler_GetAgentAlerts_InvalidAgentID(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()

	handler := NewAgentHandlerWithInterfaces(
		&MockAgentServiceImpl{},
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{},
		&MockAPIKeyServiceImpl{},
		nil,
		&MockAlertServiceImpl{},
		&MockVerificationEventServiceImpl{},
		&MockCapabilityServiceImpl{},
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		&MockMCPAttestationServiceImpl{},
	)

	app := createTestAppWithAuth(handler, orgID, userID)
	app.Get("/agents/:id/alerts", handler.GetAgentAlerts)

	req := httptest.NewRequest("GET", "/agents/invalid-uuid/alerts", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestAgentHandler_GetAgentAlerts_AgentNotFound(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()

	mockAgentService := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return nil, errors.New("agent not found")
		},
	}

	handler := NewAgentHandlerWithInterfaces(
		mockAgentService,
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{},
		&MockAPIKeyServiceImpl{},
		nil,
		&MockAlertServiceImpl{},
		&MockVerificationEventServiceImpl{},
		&MockCapabilityServiceImpl{},
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		&MockMCPAttestationServiceImpl{},
	)

	app := createTestAppWithAuth(handler, orgID, userID)
	app.Get("/agents/:id/alerts", handler.GetAgentAlerts)

	req := httptest.NewRequest("GET", "/agents/"+agentID.String()+"/alerts", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

func TestAgentHandler_GetAgentAlerts_AccessDenied(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()
	differentOrgID := uuid.New()

	mockAgentService := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{
				ID:             agentID,
				OrganizationID: differentOrgID, // Different org
				Name:           "test-agent",
			}, nil
		},
	}

	handler := NewAgentHandlerWithInterfaces(
		mockAgentService,
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{},
		&MockAPIKeyServiceImpl{},
		nil,
		&MockAlertServiceImpl{},
		&MockVerificationEventServiceImpl{},
		&MockCapabilityServiceImpl{},
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		&MockMCPAttestationServiceImpl{},
	)

	app := createTestAppWithAuth(handler, orgID, userID)
	app.Get("/agents/:id/alerts", handler.GetAgentAlerts)

	req := httptest.NewRequest("GET", "/agents/"+agentID.String()+"/alerts", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// A3b: cross-tenant access returns 404 (not 403) to avoid leaking
	// resource existence across orgs. See tenant_scope.go:41-46.
	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.JSONEq(t, `{"error":"not found"}`, string(body))
}

func TestAgentHandler_GetAgentAlerts_SuccessWithPagination(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()

	mockAgentService := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{
				ID:             agentID,
				OrganizationID: orgID,
				Name:           "test-agent",
			}, nil
		},
	}

	mockAlertService := &MockAlertServiceImpl{
		GetAlertsByAgentFunc: func(ctx context.Context, agentID uuid.UUID, limit, offset int) ([]*domain.Alert, error) {
			return []*domain.Alert{
				{ID: uuid.New(), OrganizationID: orgID, AlertType: domain.AlertSecurityBreach},
			}, nil
		},
	}

	handler := NewAgentHandlerWithInterfaces(
		mockAgentService,
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{},
		&MockAPIKeyServiceImpl{},
		nil,
		mockAlertService,
		&MockVerificationEventServiceImpl{},
		&MockCapabilityServiceImpl{},
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		&MockMCPAttestationServiceImpl{},
	)

	app := createTestAppWithAuth(handler, orgID, userID)
	app.Get("/agents/:id/alerts", handler.GetAgentAlerts)

	req := httptest.NewRequest("GET", "/agents/"+agentID.String()+"/alerts?limit=10&offset=5", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	alerts, ok := result["alerts"].([]interface{})
	assert.True(t, ok)
	assert.Len(t, alerts, 1)
}

func TestAgentHandler_GetAgentAlerts_ServiceError(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()

	mockAgentService := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{
				ID:             agentID,
				OrganizationID: orgID,
				Name:           "test-agent",
			}, nil
		},
	}

	mockAlertService := &MockAlertServiceImpl{
		GetAlertsByAgentFunc: func(ctx context.Context, agentID uuid.UUID, limit, offset int) ([]*domain.Alert, error) {
			return nil, errors.New("database error")
		},
	}

	handler := NewAgentHandlerWithInterfaces(
		mockAgentService,
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{},
		&MockAPIKeyServiceImpl{},
		nil,
		mockAlertService,
		&MockVerificationEventServiceImpl{},
		&MockCapabilityServiceImpl{},
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		&MockMCPAttestationServiceImpl{},
	)

	app := createTestAppWithAuth(handler, orgID, userID)
	app.Get("/agents/:id/alerts", handler.GetAgentAlerts)

	req := httptest.NewRequest("GET", "/agents/"+agentID.String()+"/alerts", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

// ===========================
// DetectAndMapMCPServers Tests with Mocks
// ===========================

func TestAgentHandler_DetectAndMapMCPServers_InvalidAgentID_WithMocks(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()

	handler := NewAgentHandlerWithInterfaces(
		&MockAgentServiceImpl{},
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{},
		&MockAPIKeyServiceImpl{},
		nil,
		&MockAlertServiceImpl{},
		&MockVerificationEventServiceImpl{},
		&MockCapabilityServiceImpl{},
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		&MockMCPAttestationServiceImpl{},
	)

	app := createTestAppWithAuth(handler, orgID, userID)
	app.Post("/agents/:id/detect-mcp-servers", handler.DetectAndMapMCPServers)

	req := httptest.NewRequest("POST", "/agents/invalid-uuid/detect-mcp-servers", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestAgentHandler_DetectAndMapMCPServers_InvalidJSON(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()

	handler := NewAgentHandlerWithInterfaces(
		&MockAgentServiceImpl{},
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{},
		&MockAPIKeyServiceImpl{},
		nil,
		&MockAlertServiceImpl{},
		&MockVerificationEventServiceImpl{},
		&MockCapabilityServiceImpl{},
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		&MockMCPAttestationServiceImpl{},
	)

	app := createTestAppWithAuth(handler, orgID, userID)
	app.Post("/agents/:id/detect-mcp-servers", handler.DetectAndMapMCPServers)

	req := httptest.NewRequest("POST", "/agents/"+agentID.String()+"/detect-mcp-servers", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestAgentHandler_DetectAndMapMCPServers_AgentNotFound(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()

	mockAgentService := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return nil, errors.New("agent not found")
		},
	}

	handler := NewAgentHandlerWithInterfaces(
		mockAgentService,
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{},
		&MockAPIKeyServiceImpl{},
		nil,
		&MockAlertServiceImpl{},
		&MockVerificationEventServiceImpl{},
		&MockCapabilityServiceImpl{},
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		&MockMCPAttestationServiceImpl{},
	)

	app := createTestAppWithAuth(handler, orgID, userID)
	app.Post("/agents/:id/detect-mcp-servers", handler.DetectAndMapMCPServers)

	// configPath (camelCase) is required for DetectAndMapMCPServers
	body := `{"configPath":"/path/to/config.json"}`
	req := httptest.NewRequest("POST", "/agents/"+agentID.String()+"/detect-mcp-servers", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

func TestAgentHandler_DetectAndMapMCPServers_AccessDenied(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()
	differentOrgID := uuid.New()

	mockAgentService := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{
				ID:             agentID,
				OrganizationID: differentOrgID, // Different org
				Name:           "test-agent",
			}, nil
		},
	}

	handler := NewAgentHandlerWithInterfaces(
		mockAgentService,
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{},
		&MockAPIKeyServiceImpl{},
		nil,
		&MockAlertServiceImpl{},
		&MockVerificationEventServiceImpl{},
		&MockCapabilityServiceImpl{},
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		&MockMCPAttestationServiceImpl{},
	)

	app := createTestAppWithAuth(handler, orgID, userID)
	app.Post("/agents/:id/detect-mcp-servers", handler.DetectAndMapMCPServers)

	// configPath (camelCase) is required for DetectAndMapMCPServers
	body := `{"configPath":"/path/to/config.json"}`
	req := httptest.NewRequest("POST", "/agents/"+agentID.String()+"/detect-mcp-servers", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// A3b-ii: cross-tenant access returns 404 (not 403) to avoid leaking
	// resource existence across orgs. See tenant_scope.go:41-46.
	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
	body2, _ := io.ReadAll(resp.Body)
	assert.JSONEq(t, `{"error":"not found"}`, string(body2))
}

// ===========================
// AgentHandler.DownloadSDK Tests
// ===========================

func TestAgentHandler_DownloadSDK_Success(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()

	mockAgentService := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{
				ID:             agentID,
				OrganizationID: orgID,
				Name:           "test-agent",
				Status:         domain.AgentStatusVerified,
			}, nil
		},
		GetAgentCredentialsFunc: func(ctx context.Context, id uuid.UUID) (string, string, error) {
			return "test-public-key", "test-private-key", nil
		},
	}

	handler := NewAgentHandlerWithInterfaces(
		mockAgentService,
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{},
		&MockAPIKeyServiceImpl{},
		nil,
		&MockAlertServiceImpl{},
		&MockVerificationEventServiceImpl{},
		&MockCapabilityServiceImpl{},
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		&MockMCPAttestationServiceImpl{},
	)

	app := createTestAppWithAuth(handler, orgID, userID)
	app.Get("/agents/:id/sdk", handler.DownloadSDK)

	req := httptest.NewRequest("GET", "/agents/"+agentID.String()+"/sdk?lang=python", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/zip", resp.Header.Get("Content-Type"))
}

func TestAgentHandler_DownloadSDK_InvalidAgentID(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()

	handler := NewAgentHandlerWithInterfaces(
		&MockAgentServiceImpl{},
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{},
		&MockAPIKeyServiceImpl{},
		nil,
		&MockAlertServiceImpl{},
		&MockVerificationEventServiceImpl{},
		&MockCapabilityServiceImpl{},
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		&MockMCPAttestationServiceImpl{},
	)

	app := createTestAppWithAuth(handler, orgID, userID)
	app.Get("/agents/:id/sdk", handler.DownloadSDK)

	req := httptest.NewRequest("GET", "/agents/not-a-uuid/sdk", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestAgentHandler_DownloadSDK_InvalidLanguage(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()

	mockAgentService := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{
				ID:             agentID,
				OrganizationID: orgID,
				Name:           "test-agent",
			}, nil
		},
	}

	handler := NewAgentHandlerWithInterfaces(
		mockAgentService,
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{},
		&MockAPIKeyServiceImpl{},
		nil,
		&MockAlertServiceImpl{},
		&MockVerificationEventServiceImpl{},
		&MockCapabilityServiceImpl{},
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		&MockMCPAttestationServiceImpl{},
	)

	app := createTestAppWithAuth(handler, orgID, userID)
	app.Get("/agents/:id/sdk", handler.DownloadSDK)

	req := httptest.NewRequest("GET", "/agents/"+agentID.String()+"/sdk?lang=ruby", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestAgentHandler_DownloadSDK_AgentNotFound(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()

	mockAgentService := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return nil, errors.New("agent not found")
		},
	}

	handler := NewAgentHandlerWithInterfaces(
		mockAgentService,
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{},
		&MockAPIKeyServiceImpl{},
		nil,
		&MockAlertServiceImpl{},
		&MockVerificationEventServiceImpl{},
		&MockCapabilityServiceImpl{},
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		&MockMCPAttestationServiceImpl{},
	)

	app := createTestAppWithAuth(handler, orgID, userID)
	app.Get("/agents/:id/sdk", handler.DownloadSDK)

	req := httptest.NewRequest("GET", "/agents/"+agentID.String()+"/sdk", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

func TestAgentHandler_DownloadSDK_WrongOrganization(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()
	differentOrgID := uuid.New()

	mockAgentService := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{
				ID:             agentID,
				OrganizationID: differentOrgID, // Different org
				Name:           "test-agent",
			}, nil
		},
	}

	handler := NewAgentHandlerWithInterfaces(
		mockAgentService,
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{},
		&MockAPIKeyServiceImpl{},
		nil,
		&MockAlertServiceImpl{},
		&MockVerificationEventServiceImpl{},
		&MockCapabilityServiceImpl{},
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		&MockMCPAttestationServiceImpl{},
	)

	app := createTestAppWithAuth(handler, orgID, userID)
	app.Get("/agents/:id/sdk", handler.DownloadSDK)

	req := httptest.NewRequest("GET", "/agents/"+agentID.String()+"/sdk", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// A3b: cross-tenant access returns 404 (not 403) to avoid leaking
	// resource existence across orgs. See tenant_scope.go:41-46.
	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.JSONEq(t, `{"error":"not found"}`, string(body))
}

func TestAgentHandler_DownloadSDK_CredentialsError(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()

	mockAgentService := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{
				ID:             agentID,
				OrganizationID: orgID,
				Name:           "test-agent",
			}, nil
		},
		GetAgentCredentialsFunc: func(ctx context.Context, id uuid.UUID) (string, string, error) {
			return "", "", errors.New("failed to retrieve credentials")
		},
	}

	handler := NewAgentHandlerWithInterfaces(
		mockAgentService,
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{},
		&MockAPIKeyServiceImpl{},
		nil,
		&MockAlertServiceImpl{},
		&MockVerificationEventServiceImpl{},
		&MockCapabilityServiceImpl{},
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		&MockMCPAttestationServiceImpl{},
	)

	app := createTestAppWithAuth(handler, orgID, userID)
	app.Get("/agents/:id/sdk", handler.DownloadSDK)

	req := httptest.NewRequest("GET", "/agents/"+agentID.String()+"/sdk", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

func TestAgentHandler_DownloadSDK_NodejsNotImplemented(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()

	mockAgentService := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{
				ID:             agentID,
				OrganizationID: orgID,
				Name:           "test-agent",
			}, nil
		},
		GetAgentCredentialsFunc: func(ctx context.Context, id uuid.UUID) (string, string, error) {
			return "pub", "priv", nil
		},
	}

	handler := NewAgentHandlerWithInterfaces(
		mockAgentService,
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{},
		&MockAPIKeyServiceImpl{},
		nil,
		&MockAlertServiceImpl{},
		&MockVerificationEventServiceImpl{},
		&MockCapabilityServiceImpl{},
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		&MockMCPAttestationServiceImpl{},
	)

	app := createTestAppWithAuth(handler, orgID, userID)
	app.Get("/agents/:id/sdk", handler.DownloadSDK)

	req := httptest.NewRequest("GET", "/agents/"+agentID.String()+"/sdk?lang=nodejs", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusNotImplemented, resp.StatusCode)
}

func TestAgentHandler_DownloadSDK_GoNotImplemented(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()

	mockAgentService := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{
				ID:             agentID,
				OrganizationID: orgID,
				Name:           "test-agent",
			}, nil
		},
		GetAgentCredentialsFunc: func(ctx context.Context, id uuid.UUID) (string, string, error) {
			return "pub", "priv", nil
		},
	}

	handler := NewAgentHandlerWithInterfaces(
		mockAgentService,
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{},
		&MockAPIKeyServiceImpl{},
		nil,
		&MockAlertServiceImpl{},
		&MockVerificationEventServiceImpl{},
		&MockCapabilityServiceImpl{},
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		&MockMCPAttestationServiceImpl{},
	)

	app := createTestAppWithAuth(handler, orgID, userID)
	app.Get("/agents/:id/sdk", handler.DownloadSDK)

	req := httptest.NewRequest("GET", "/agents/"+agentID.String()+"/sdk?lang=go", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusNotImplemented, resp.StatusCode)
}

// ===========================
// AgentHandler.RecalculateAgentTrustScore Tests
// ===========================

func TestAgentHandler_RecalculateAgentTrustScore_InvalidAgentID(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()

	handler := NewAgentHandlerWithInterfaces(
		&MockAgentServiceImpl{},
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{},
		&MockAPIKeyServiceImpl{},
		nil, // trustScoreHandler
		&MockAlertServiceImpl{},
		&MockVerificationEventServiceImpl{},
		&MockCapabilityServiceImpl{},
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		&MockMCPAttestationServiceImpl{},
	)

	app := createTestAppWithAuth(handler, orgID, userID)
	app.Post("/agents/:id/trust-score/recalculate", handler.RecalculateAgentTrustScore)

	req := httptest.NewRequest("POST", "/agents/not-a-uuid/trust-score/recalculate", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// The handler delegates to trustScoreHandler which is nil, so expect 500
	assert.True(t, resp.StatusCode >= 400, "Expected error status")
}

// ===========================
// AgentHandler.GetAgentTrustScore Tests
// ===========================

func TestAgentHandler_GetAgentTrustScore_InvalidAgentID(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()

	handler := NewAgentHandlerWithInterfaces(
		&MockAgentServiceImpl{},
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{},
		&MockAPIKeyServiceImpl{},
		nil, // trustScoreHandler
		&MockAlertServiceImpl{},
		&MockVerificationEventServiceImpl{},
		&MockCapabilityServiceImpl{},
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		&MockMCPAttestationServiceImpl{},
	)

	app := createTestAppWithAuth(handler, orgID, userID)
	app.Get("/agents/:id/trust-score", handler.GetAgentTrustScore)

	req := httptest.NewRequest("GET", "/agents/not-a-uuid/trust-score", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// The handler delegates to trustScoreHandler which is nil, so expect 500
	assert.True(t, resp.StatusCode >= 400, "Expected error status")
}

// ===========================
// AgentHandler.GetAgentTrustScoreHistory Tests
// ===========================

func TestAgentHandler_GetAgentTrustScoreHistory_InvalidAgentID(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()

	handler := NewAgentHandlerWithInterfaces(
		&MockAgentServiceImpl{},
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{},
		&MockAPIKeyServiceImpl{},
		nil, // trustScoreHandler
		&MockAlertServiceImpl{},
		&MockVerificationEventServiceImpl{},
		&MockCapabilityServiceImpl{},
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		&MockMCPAttestationServiceImpl{},
	)

	app := createTestAppWithAuth(handler, orgID, userID)
	app.Get("/agents/:id/trust-score/history", handler.GetAgentTrustScoreHistory)

	req := httptest.NewRequest("GET", "/agents/not-a-uuid/trust-score/history", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// The handler delegates to trustScoreHandler which is nil, so expect 500
	assert.True(t, resp.StatusCode >= 400, "Expected error status")
}

// ===========================
// A3b: cross-tenant access on read-only handlers returns 404 (not 403).
// New tests for handlers previously without cross-org coverage. Pairs with
// the flips of *_WrongOrganization / *_AccessDenied tests above for the
// same handler family.
// ===========================

func TestAgentHandler_GetCredentials_CrossOrgReturns404(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()
	differentOrgID := uuid.New()

	mockAgentService := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{
				ID:             agentID,
				OrganizationID: differentOrgID,
				Name:           "test-agent",
			}, nil
		},
	}

	handler := NewAgentHandlerWithInterfaces(
		mockAgentService,
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{},
		&MockAPIKeyServiceImpl{},
		nil,
		&MockAlertServiceImpl{},
		&MockVerificationEventServiceImpl{},
		&MockCapabilityServiceImpl{},
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		&MockMCPAttestationServiceImpl{},
	)

	app := createTestAppWithAuth(handler, orgID, userID)
	app.Get("/agents/:id/credentials", handler.GetCredentials)

	req := httptest.NewRequest("GET", "/agents/"+agentID.String()+"/credentials", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.JSONEq(t, `{"error":"not found"}`, string(body))
}

func TestAgentHandler_GetAgentMCPServers_CrossOrgReturns404(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()
	differentOrgID := uuid.New()

	mockAgentService := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{
				ID:             agentID,
				OrganizationID: differentOrgID,
				Name:           "test-agent",
			}, nil
		},
	}

	handler := NewAgentHandlerWithInterfaces(
		mockAgentService,
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{},
		&MockAPIKeyServiceImpl{},
		nil,
		&MockAlertServiceImpl{},
		&MockVerificationEventServiceImpl{},
		&MockCapabilityServiceImpl{},
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		&MockMCPAttestationServiceImpl{},
	)

	app := createTestAppWithAuth(handler, orgID, userID)
	app.Get("/agents/:id/mcp-servers", handler.GetAgentMCPServers)

	req := httptest.NewRequest("GET", "/agents/"+agentID.String()+"/mcp-servers", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.JSONEq(t, `{"error":"not found"}`, string(body))
}

func TestAgentHandler_GetAgentByIdentifier_UUID_CrossOrgReturns404(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()
	differentOrgID := uuid.New()

	mockAgentService := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{
				ID:             agentID,
				OrganizationID: differentOrgID,
				Name:           "test-agent",
			}, nil
		},
	}

	handler := NewAgentHandlerWithInterfaces(
		mockAgentService,
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{},
		&MockAPIKeyServiceImpl{},
		nil,
		&MockAlertServiceImpl{},
		&MockVerificationEventServiceImpl{},
		&MockCapabilityServiceImpl{},
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		&MockMCPAttestationServiceImpl{},
	)

	app := createTestAppWithAuth(handler, orgID, userID)
	app.Get("/agents/by-identifier/:identifier", handler.GetAgentByIdentifier)

	req := httptest.NewRequest("GET", "/agents/by-identifier/"+agentID.String(), nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.JSONEq(t, `{"error":"not found"}`, string(body))
}

func TestAgentHandler_GetAgentActivity_CrossOrgReturns404(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()
	differentOrgID := uuid.New()

	mockAgentService := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{
				ID:             agentID,
				OrganizationID: differentOrgID,
				Name:           "test-agent",
			}, nil
		},
	}

	handler := NewAgentHandlerWithInterfaces(
		mockAgentService,
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{},
		&MockAPIKeyServiceImpl{},
		nil,
		&MockAlertServiceImpl{},
		&MockVerificationEventServiceImpl{},
		&MockCapabilityServiceImpl{},
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		&MockMCPAttestationServiceImpl{},
	)

	app := createTestAppWithAuth(handler, orgID, userID)
	app.Get("/agents/:id/activity", handler.GetAgentActivity)

	req := httptest.NewRequest("GET", "/agents/"+agentID.String()+"/activity", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.JSONEq(t, `{"error":"not found"}`, string(body))
}

// ===========================
// A3b-ii: cross-tenant access on mutation/lifecycle handlers returns
// 404 (not 403). Table-driven sweep — each row exercises one of the
// 12 LoadOwned conversions in this PR. The mock GetAgent returns an
// agent in a different org; every handler must short-circuit at
// LoadOwned and never reach the mutation service call. Body content
// is minimal — handlers that parse JSON before LoadOwned just need a
// parseable payload to reach the LoadOwned check.
// ===========================

func TestAgentHandler_MutationHandlers_CrossOrgReturns404(t *testing.T) {
	callerOrgID := uuid.New()
	callerUserID := uuid.New()
	agentID := uuid.New()
	differentOrgID := uuid.New()

	mockAgentService := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{
				ID:             agentID,
				OrganizationID: differentOrgID,
				Name:           "victim-agent",
			}, nil
		},
	}

	tests := []struct {
		name        string
		method      string
		requestPath string
		body        string
		setup       func(*fiber.App, *AgentHandler)
	}{
		{
			name:        "UpdateAgent",
			method:      "PUT",
			requestPath: "/agents/" + agentID.String(),
			body:        `{"name":"x"}`,
			setup:       func(app *fiber.App, h *AgentHandler) { app.Put("/agents/:id", h.UpdateAgent) },
		},
		{
			name:        "DeleteAgent",
			method:      "DELETE",
			requestPath: "/agents/" + agentID.String(),
			body:        "",
			setup:       func(app *fiber.App, h *AgentHandler) { app.Delete("/agents/:id", h.DeleteAgent) },
		},
		{
			name:        "VerifyAgent",
			method:      "POST",
			requestPath: "/agents/" + agentID.String() + "/verify",
			body:        "",
			setup: func(app *fiber.App, h *AgentHandler) {
				app.Post("/agents/:id/verify", h.VerifyAgent)
			},
		},
		{
			name:        "AddMCPServersToAgent",
			method:      "PUT",
			requestPath: "/agents/" + agentID.String() + "/mcp-servers",
			// Non-empty list — handler validates that mcpServerIds is
			// non-empty BEFORE LoadOwned. The empty-list path returns
			// 400 from body validation, which is independent of the
			// cross-tenant check we're verifying here.
			body: `{"mcpServerIds":["some-server"]}`,
			setup: func(app *fiber.App, h *AgentHandler) {
				app.Put("/agents/:id/mcp-servers", h.AddMCPServersToAgent)
			},
		},
		{
			name:        "RemoveMCPServerFromAgent",
			method:      "DELETE",
			requestPath: "/agents/" + agentID.String() + "/mcp-servers/foo",
			body:        "",
			setup: func(app *fiber.App, h *AgentHandler) {
				app.Delete("/agents/:id/mcp-servers/:mcp_id", h.RemoveMCPServerFromAgent)
			},
		},
		{
			name:        "UpdateAgentTrustScore",
			method:      "PUT",
			requestPath: "/agents/" + agentID.String() + "/trust-score",
			body:        `{"score":50.0}`,
			setup: func(app *fiber.App, h *AgentHandler) {
				app.Put("/agents/:id/trust-score", h.UpdateAgentTrustScore)
			},
		},
		{
			name:        "SuspendAgent",
			method:      "POST",
			requestPath: "/agents/" + agentID.String() + "/suspend",
			body:        "",
			setup: func(app *fiber.App, h *AgentHandler) {
				app.Post("/agents/:id/suspend", h.SuspendAgent)
			},
		},
		{
			name:        "ReactivateAgent",
			method:      "POST",
			requestPath: "/agents/" + agentID.String() + "/reactivate",
			body:        "",
			setup: func(app *fiber.App, h *AgentHandler) {
				app.Post("/agents/:id/reactivate", h.ReactivateAgent)
			},
		},
		{
			name:        "RevokeAgent",
			method:      "POST",
			requestPath: "/agents/" + agentID.String() + "/revoke",
			body:        "",
			setup: func(app *fiber.App, h *AgentHandler) {
				app.Post("/agents/:id/revoke", h.RevokeAgent)
			},
		},
		{
			name:        "RotateCredentials",
			method:      "POST",
			requestPath: "/agents/" + agentID.String() + "/rotate-credentials",
			body:        "",
			setup: func(app *fiber.App, h *AgentHandler) {
				app.Post("/agents/:id/rotate-credentials", h.RotateCredentials)
			},
		},
		{
			name:        "UpdateAgentKeys",
			method:      "PUT",
			requestPath: "/agents/" + agentID.String() + "/keys",
			body:        `{"publicKey":"AAAA"}`,
			setup: func(app *fiber.App, h *AgentHandler) {
				app.Put("/agents/:id/keys", h.UpdateAgentKeys)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewAgentHandlerWithInterfaces(
				mockAgentService,
				&MockMCPServiceImpl{},
				&MockAuditServiceImpl{},
				&MockAPIKeyServiceImpl{},
				nil,
				&MockAlertServiceImpl{},
				&MockVerificationEventServiceImpl{},
				&MockCapabilityServiceImpl{},
				&MockTagServiceImpl{},
				&MockOrganizationRepository{},
				&MockMCPAttestationServiceImpl{},
			)

			app := createTestAppWithAuth(handler, callerOrgID, callerUserID)
			tt.setup(app, handler)

			var req *http.Request
			if tt.body == "" {
				req = httptest.NewRequest(tt.method, tt.requestPath, nil)
			} else {
				req = httptest.NewRequest(tt.method, tt.requestPath, strings.NewReader(tt.body))
				req.Header.Set("Content-Type", "application/json")
			}

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
