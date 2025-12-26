package handlers

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===========================
// AdminHandler Integration Tests
// ===========================
// These tests use mock implementations via NewAdminHandlerWithInterfaces

// Helper to create a handler with mocks for integration tests
func createAdminHandlerWithMocks(
	authService AuthServicer,
	adminService AdminServicer,
	agentService AgentServicer,
	mcpService MCPServicer,
	auditService AuditServicer,
	alertService AlertServicerExtended,
	registrationService RegistrationServicer,
	securityService SecurityServicer,
) *AdminHandler {
	return NewAdminHandlerWithInterfaces(
		authService,
		adminService,
		agentService,
		mcpService,
		auditService,
		alertService,
		registrationService,
		securityService,
	)
}

// ===========================
// GetDashboardStats Tests
// ===========================

func TestAdminHandler_GetDashboardStats_Success(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()

	// Create mock services
	mockAgentService := &MockAgentServiceImpl{
		ListAgentsFunc: func(ctx context.Context, oID uuid.UUID) ([]*domain.Agent, error) {
			return []*domain.Agent{
				{ID: uuid.New(), Status: domain.AgentStatusVerified, TrustScore: 85.0},
				{ID: uuid.New(), Status: domain.AgentStatusPending, TrustScore: 70.0},
				{ID: uuid.New(), Status: domain.AgentStatusVerified, TrustScore: 90.0},
			}, nil
		},
	}

	mockAuthService := &MockAuthServiceImpl{
		GetUsersByOrganizationFunc: func(ctx context.Context, oID uuid.UUID) ([]*domain.User, error) {
			return []*domain.User{
				{ID: uuid.New(), Email: "user1@test.com"},
				{ID: uuid.New(), Email: "user2@test.com"},
			}, nil
		},
		CountActiveUsersFunc: func(ctx context.Context, oID uuid.UUID, withinMinutes int) (int, error) {
			return 1, nil
		},
	}

	mockAlertService := &MockAlertServiceExtendedImpl{
		GetAlertsFunc: func(ctx context.Context, oID uuid.UUID, severity, status string, limit, offset int) ([]*domain.Alert, int, error) {
			return []*domain.Alert{
				{ID: uuid.New(), Severity: domain.AlertSeverityCritical},
				{ID: uuid.New(), Severity: domain.AlertSeverityHigh},
			}, 2, nil
		},
	}

	mockMCPService := &MockMCPServiceImpl{
		ListMCPServersFunc: func(ctx context.Context, oID uuid.UUID) ([]*domain.MCPServer, error) {
			return []*domain.MCPServer{
				{ID: uuid.New(), Status: domain.MCPServerStatusVerified},
				{ID: uuid.New(), Status: domain.MCPServerStatusPending},
			}, nil
		},
	}

	mockAuditService := &MockAuditServiceImpl{}
	mockSecurityService := &MockSecurityServiceImpl{
		CountOpenIncidentsFunc: func(ctx context.Context, oID uuid.UUID) (int, error) {
			return 3, nil
		},
	}

	handler := createAdminHandlerWithMocks(
		mockAuthService,
		nil, // adminService not needed for this test
		mockAgentService,
		mockMCPService,
		mockAuditService,
		mockAlertService,
		nil, // registrationService not needed
		mockSecurityService,
	)

	app := fiber.New()
	app.Get("/admin/dashboard-stats", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		return handler.GetDashboardStats(c)
	})

	req := httptest.NewRequest("GET", "/admin/dashboard-stats", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	// Verify response contains expected fields
	assert.Equal(t, float64(3), result["totalAgents"])
	assert.Equal(t, float64(2), result["verifiedAgents"])
	assert.Equal(t, float64(1), result["pendingAgents"])
	assert.Equal(t, float64(2), result["totalMcpServers"])
	assert.Equal(t, float64(1), result["activeMcpServers"])
	assert.Equal(t, float64(2), result["totalUsers"])
	assert.Equal(t, float64(1), result["activeUsers"])
	assert.Equal(t, float64(2), result["activeAlerts"])
	assert.Equal(t, float64(1), result["criticalAlerts"])
	assert.Equal(t, float64(3), result["securityIncidents"])
}

func TestAdminHandler_GetDashboardStats_AgentServiceError(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()

	mockAgentService := &MockAgentServiceImpl{
		ListAgentsFunc: func(ctx context.Context, oID uuid.UUID) ([]*domain.Agent, error) {
			return nil, assert.AnError
		},
	}

	handler := createAdminHandlerWithMocks(
		&MockAuthServiceImpl{},
		nil,
		mockAgentService,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	app := fiber.New()
	app.Get("/admin/dashboard-stats", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		return handler.GetDashboardStats(c)
	})

	req := httptest.NewRequest("GET", "/admin/dashboard-stats", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

// ===========================
// GetPendingUsers Tests
// ===========================

func TestAdminHandler_GetPendingUsers_Success(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()

	mockAdminService := &MockAdminServiceImpl{
		GetPendingUsersFunc: func(ctx context.Context, adminOrgID uuid.UUID) ([]*domain.User, error) {
			return []*domain.User{
				{ID: uuid.New(), Email: "pending1@test.com", Status: "pending"},
				{ID: uuid.New(), Email: "pending2@test.com", Status: "pending"},
			}, nil
		},
	}

	mockAuditService := &MockAuditServiceImpl{}

	handler := createAdminHandlerWithMocks(
		nil,
		mockAdminService,
		nil,
		nil,
		mockAuditService,
		nil,
		nil,
		nil,
	)

	app := fiber.New()
	app.Get("/admin/pending-users", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		return handler.GetPendingUsers(c)
	})

	req := httptest.NewRequest("GET", "/admin/pending-users", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, float64(2), result["total"])
	users := result["users"].([]interface{})
	assert.Len(t, users, 2)
}

func TestAdminHandler_GetPendingUsers_ServiceError(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()

	mockAdminService := &MockAdminServiceImpl{
		GetPendingUsersFunc: func(ctx context.Context, adminOrgID uuid.UUID) ([]*domain.User, error) {
			return nil, assert.AnError
		},
	}

	handler := createAdminHandlerWithMocks(
		nil,
		mockAdminService,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	app := fiber.New()
	app.Get("/admin/pending-users", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		return handler.GetPendingUsers(c)
	})

	req := httptest.NewRequest("GET", "/admin/pending-users", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

// ===========================
// isSuperAdmin Tests
// ===========================

func TestAdminHandler_isSuperAdmin_IsOldestAdmin(t *testing.T) {
	orgID := uuid.New()
	oldAdminID := uuid.New()
	newAdminID := uuid.New()

	oldTime := time.Now().Add(-time.Hour * 24 * 30) // 30 days ago
	newTime := time.Now().Add(-time.Hour)           // 1 hour ago

	mockAuthService := &MockAuthServiceImpl{
		GetUserByIDFunc: func(ctx context.Context, id uuid.UUID) (*domain.User, error) {
			return &domain.User{
				ID:             id,
				Role:           "admin",
				Status:         "active",
				OrganizationID: orgID,
			}, nil
		},
		GetUsersByOrganizationFunc: func(ctx context.Context, oID uuid.UUID) ([]*domain.User, error) {
			return []*domain.User{
				{ID: oldAdminID, Role: "admin", Status: "active", CreatedAt: oldTime},
				{ID: newAdminID, Role: "admin", Status: "active", CreatedAt: newTime},
			}, nil
		},
	}

	handler := createAdminHandlerWithMocks(
		mockAuthService,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	// Test that oldAdminID is super admin
	isSuperAdmin, err := handler.isSuperAdmin(context.Background(), oldAdminID, orgID)
	require.NoError(t, err)
	assert.True(t, isSuperAdmin)

	// Test that newAdminID is NOT super admin
	isSuperAdmin, err = handler.isSuperAdmin(context.Background(), newAdminID, orgID)
	require.NoError(t, err)
	assert.False(t, isSuperAdmin)
}

func TestAdminHandler_isSuperAdmin_NotAdmin(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()

	mockAuthService := &MockAuthServiceImpl{
		GetUserByIDFunc: func(ctx context.Context, id uuid.UUID) (*domain.User, error) {
			return &domain.User{
				ID:             id,
				Role:           "member", // Not admin
				Status:         "active",
				OrganizationID: orgID,
			}, nil
		},
	}

	handler := createAdminHandlerWithMocks(
		mockAuthService,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	isSuperAdmin, err := handler.isSuperAdmin(context.Background(), userID, orgID)
	require.NoError(t, err)
	assert.False(t, isSuperAdmin)
}

func TestAdminHandler_isSuperAdmin_DifferentOrg(t *testing.T) {
	orgID := uuid.New()
	otherOrgID := uuid.New()
	userID := uuid.New()

	mockAuthService := &MockAuthServiceImpl{
		GetUserByIDFunc: func(ctx context.Context, id uuid.UUID) (*domain.User, error) {
			return &domain.User{
				ID:             id,
				Role:           "admin",
				Status:         "active",
				OrganizationID: otherOrgID, // Different org
			}, nil
		},
	}

	handler := createAdminHandlerWithMocks(
		mockAuthService,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	isSuperAdmin, err := handler.isSuperAdmin(context.Background(), userID, orgID)
	require.NoError(t, err)
	assert.False(t, isSuperAdmin)
}

func TestAdminHandler_isSuperAdmin_UserNotFound(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()

	mockAuthService := &MockAuthServiceImpl{
		GetUserByIDFunc: func(ctx context.Context, id uuid.UUID) (*domain.User, error) {
			return nil, assert.AnError
		},
	}

	handler := createAdminHandlerWithMocks(
		mockAuthService,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	isSuperAdmin, err := handler.isSuperAdmin(context.Background(), userID, orgID)
	assert.Error(t, err)
	assert.False(t, isSuperAdmin)
}

// ===========================
// NewAdminHandlerWithInterfaces Tests
// ===========================

func TestNewAdminHandlerWithInterfaces_NilDeps(t *testing.T) {
	handler := NewAdminHandlerWithInterfaces(nil, nil, nil, nil, nil, nil, nil, nil)
	assert.NotNil(t, handler)
}

func TestNewAdminHandlerWithInterfaces_WithMocks(t *testing.T) {
	mockAuth := &MockAuthServiceImpl{}
	mockAdmin := &MockAdminServiceImpl{}
	mockAgent := &MockAgentServiceImpl{}
	mockMCP := &MockMCPServiceImpl{}
	mockAudit := &MockAuditServiceImpl{}
	mockAlert := &MockAlertServiceExtendedImpl{}
	mockReg := &MockRegistrationServiceImpl{}
	mockSec := &MockSecurityServiceImpl{}

	handler := NewAdminHandlerWithInterfaces(
		mockAuth, mockAdmin, mockAgent, mockMCP,
		mockAudit, mockAlert, mockReg, mockSec,
	)

	assert.NotNil(t, handler)
	assert.NotNil(t, handler.authServicer)
	assert.NotNil(t, handler.adminServicer)
	assert.NotNil(t, handler.agentServicer)
	assert.NotNil(t, handler.mcpServicer)
	assert.NotNil(t, handler.auditServicer)
	assert.NotNil(t, handler.alertServicer)
	assert.NotNil(t, handler.registrationServicer)
	assert.NotNil(t, handler.securityServicer)
}
