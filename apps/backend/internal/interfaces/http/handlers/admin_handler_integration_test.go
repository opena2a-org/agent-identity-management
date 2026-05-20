package handlers

import (
	"context"
	"encoding/json"
	"errors"
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

// ===========================
// ListUsers Tests
// ===========================

func TestAdminHandler_ListUsers_Success(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()

	mockAuthService := &MockAuthServiceImpl{
		GetUsersByOrganizationFunc: func(ctx context.Context, oID uuid.UUID) ([]*domain.User, error) {
			return []*domain.User{
				{ID: uuid.New(), Email: "user1@test.com", Name: "User 1", Role: "admin", Status: "active"},
				{ID: uuid.New(), Email: "user2@test.com", Name: "User 2", Role: "member", Status: "active"},
			}, nil
		},
	}

	mockRegService := &MockRegistrationServiceImpl{
		ListPendingRegistrationRequestsFunc: func(ctx context.Context, oID uuid.UUID, limit, offset int) ([]*domain.UserRegistrationRequest, int, error) {
			return []*domain.UserRegistrationRequest{
				{ID: uuid.New(), Email: "pending@test.com", FirstName: "Pending", LastName: "User"},
			}, 1, nil
		},
	}

	mockAuditService := &MockAuditServiceImpl{}

	handler := createAdminHandlerWithMocks(
		mockAuthService,
		nil,
		nil,
		nil,
		mockAuditService,
		nil,
		mockRegService,
		nil,
	)

	app := fiber.New()
	app.Get("/admin/users", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		return handler.ListUsers(c)
	})

	req := httptest.NewRequest("GET", "/admin/users", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, float64(3), result["total"])
	assert.Equal(t, float64(2), result["approvedUsers"])
	assert.Equal(t, float64(1), result["pendingRegistrations"])
}

func TestAdminHandler_ListUsers_NoOrgID(t *testing.T) {
	handler := createAdminHandlerWithMocks(nil, nil, nil, nil, nil, nil, nil, nil)

	app := fiber.New()
	app.Get("/admin/users", handler.ListUsers) // No middleware to set context

	req := httptest.NewRequest("GET", "/admin/users", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAdminHandler_ListUsers_AuthServiceError(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()

	mockAuthService := &MockAuthServiceImpl{
		GetUsersByOrganizationFunc: func(ctx context.Context, oID uuid.UUID) ([]*domain.User, error) {
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

	app := fiber.New()
	app.Get("/admin/users", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		return handler.ListUsers(c)
	})

	req := httptest.NewRequest("GET", "/admin/users", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

// ===========================
// UpdateUserRole Tests
// ===========================

func TestAdminHandler_UpdateUserRole_Success(t *testing.T) {
	orgID := uuid.New()
	adminID := uuid.New()
	targetUserID := uuid.New()

	mockAuthService := &MockAuthServiceImpl{
		UpdateUserRoleFunc: func(ctx context.Context, uID, oID uuid.UUID, role domain.UserRole, aID uuid.UUID) (*domain.User, error) {
			return &domain.User{
				ID:    uID,
				Email: "user@test.com",
				Role:  role,
			}, nil
		},
	}

	mockAuditService := &MockAuditServiceImpl{}

	handler := createAdminHandlerWithMocks(
		mockAuthService,
		nil,
		nil,
		nil,
		mockAuditService,
		nil,
		nil,
		nil,
	)

	app := fiber.New()
	app.Put("/admin/users/:id/role", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", adminID)
		return handler.UpdateUserRole(c)
	})

	body := `{"role": "admin"}`
	req := httptest.NewRequest("PUT", "/admin/users/"+targetUserID.String()+"/role", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestAdminHandler_UpdateUserRole_InvalidID(t *testing.T) {
	orgID := uuid.New()
	adminID := uuid.New()

	handler := createAdminHandlerWithMocks(nil, nil, nil, nil, nil, nil, nil, nil)

	app := fiber.New()
	app.Put("/admin/users/:id/role", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", adminID)
		return handler.UpdateUserRole(c)
	})

	req := httptest.NewRequest("PUT", "/admin/users/invalid-uuid/role", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestAdminHandler_UpdateUserRole_InvalidRoleValue(t *testing.T) {
	orgID := uuid.New()
	adminID := uuid.New()
	targetUserID := uuid.New()

	handler := createAdminHandlerWithMocks(nil, nil, nil, nil, nil, nil, nil, nil)

	app := fiber.New()
	app.Put("/admin/users/:id/role", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", adminID)
		return handler.UpdateUserRole(c)
	})

	body := `{"role": "superadmin"}`
	req := httptest.NewRequest("PUT", "/admin/users/"+targetUserID.String()+"/role", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// DeactivateUser Tests
// ===========================

func TestAdminHandler_DeactivateUser_Success(t *testing.T) {
	orgID := uuid.New()
	adminID := uuid.New()
	targetUserID := uuid.New()

	mockAuthService := &MockAuthServiceImpl{
		GetUserByIDFunc: func(ctx context.Context, id uuid.UUID) (*domain.User, error) {
			return &domain.User{
				ID:             id,
				Role:           "member",
				OrganizationID: orgID,
				CreatedAt:      time.Now(),
			}, nil
		},
		GetUsersByOrganizationFunc: func(ctx context.Context, oID uuid.UUID) ([]*domain.User, error) {
			return []*domain.User{
				{ID: adminID, Role: "admin", Status: "active", CreatedAt: time.Now().Add(-time.Hour)},
			}, nil
		},
		DeactivateUserFunc: func(ctx context.Context, uID, oID, aID uuid.UUID) error {
			return nil
		},
	}

	mockAuditService := &MockAuditServiceImpl{}

	handler := createAdminHandlerWithMocks(
		mockAuthService,
		nil,
		nil,
		nil,
		mockAuditService,
		nil,
		nil,
		nil,
	)

	app := fiber.New()
	app.Delete("/admin/users/:id/deactivate", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", adminID)
		return handler.DeactivateUser(c)
	})

	req := httptest.NewRequest("DELETE", "/admin/users/"+targetUserID.String()+"/deactivate", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestAdminHandler_DeactivateUser_CannotDeactivateSelf(t *testing.T) {
	orgID := uuid.New()
	adminID := uuid.New()

	handler := createAdminHandlerWithMocks(nil, nil, nil, nil, nil, nil, nil, nil)

	app := fiber.New()
	app.Delete("/admin/users/:id/deactivate", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", adminID)
		return handler.DeactivateUser(c)
	})

	req := httptest.NewRequest("DELETE", "/admin/users/"+adminID.String()+"/deactivate", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// ActivateUser Tests
// ===========================

func TestAdminHandler_ActivateUser_Success(t *testing.T) {
	orgID := uuid.New()
	adminID := uuid.New()
	targetUserID := uuid.New()

	mockAuthService := &MockAuthServiceImpl{
		GetUserByIDFunc: func(ctx context.Context, id uuid.UUID) (*domain.User, error) {
			return &domain.User{
				ID:             id,
				Email:          "user@test.com",
				OrganizationID: orgID,
				Status:         "inactive",
			}, nil
		},
	}

	mockAdminService := &MockAdminServiceImpl{
		ActivateUserFunc: func(ctx context.Context, uID, aID uuid.UUID) error {
			return nil
		},
	}

	mockAuditService := &MockAuditServiceImpl{}

	handler := createAdminHandlerWithMocks(
		mockAuthService,
		mockAdminService,
		nil,
		nil,
		mockAuditService,
		nil,
		nil,
		nil,
	)

	app := fiber.New()
	app.Post("/admin/users/:id/activate", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", adminID)
		return handler.ActivateUser(c)
	})

	req := httptest.NewRequest("POST", "/admin/users/"+targetUserID.String()+"/activate", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestAdminHandler_ActivateUser_NotFound(t *testing.T) {
	orgID := uuid.New()
	adminID := uuid.New()
	targetUserID := uuid.New()

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

	app := fiber.New()
	app.Post("/admin/users/:id/activate", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", adminID)
		return handler.ActivateUser(c)
	})

	req := httptest.NewRequest("POST", "/admin/users/"+targetUserID.String()+"/activate", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

func TestAdminHandler_ActivateUser_WrongOrg(t *testing.T) {
	orgID := uuid.New()
	otherOrgID := uuid.New()
	adminID := uuid.New()
	targetUserID := uuid.New()

	mockAuthService := &MockAuthServiceImpl{
		GetUserByIDFunc: func(ctx context.Context, id uuid.UUID) (*domain.User, error) {
			return &domain.User{
				ID:             id,
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

	app := fiber.New()
	app.Post("/admin/users/:id/activate", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", adminID)
		return handler.ActivateUser(c)
	})

	req := httptest.NewRequest("POST", "/admin/users/"+targetUserID.String()+"/activate", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusForbidden, resp.StatusCode)
}

// ===========================
// PermanentlyDeleteUser Tests
// ===========================

func TestAdminHandler_PermanentlyDeleteUser_Success(t *testing.T) {
	orgID := uuid.New()
	adminID := uuid.New()
	targetUserID := uuid.New()

	mockAuthService := &MockAuthServiceImpl{
		GetUserByIDFunc: func(ctx context.Context, id uuid.UUID) (*domain.User, error) {
			return &domain.User{
				ID:             id,
				Email:          "user@test.com",
				Name:           "Test User",
				OrganizationID: orgID,
				Role:           "member",
				CreatedAt:      time.Now(),
			}, nil
		},
		GetUsersByOrganizationFunc: func(ctx context.Context, oID uuid.UUID) ([]*domain.User, error) {
			return []*domain.User{
				{ID: adminID, Role: "admin", Status: "active", CreatedAt: time.Now().Add(-time.Hour)},
			}, nil
		},
	}

	mockAdminService := &MockAdminServiceImpl{
		PermanentlyDeleteUserFunc: func(ctx context.Context, uID, aID uuid.UUID) error {
			return nil
		},
	}

	mockAuditService := &MockAuditServiceImpl{}

	handler := createAdminHandlerWithMocks(
		mockAuthService,
		mockAdminService,
		nil,
		nil,
		mockAuditService,
		nil,
		nil,
		nil,
	)

	app := fiber.New()
	app.Delete("/admin/users/:id/permanent", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", adminID)
		return handler.PermanentlyDeleteUser(c)
	})

	req := httptest.NewRequest("DELETE", "/admin/users/"+targetUserID.String()+"/permanent", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestAdminHandler_PermanentlyDeleteUser_CannotDeleteSelf(t *testing.T) {
	orgID := uuid.New()
	adminID := uuid.New()

	mockAuthService := &MockAuthServiceImpl{
		GetUserByIDFunc: func(ctx context.Context, id uuid.UUID) (*domain.User, error) {
			return &domain.User{
				ID:             id,
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

	app := fiber.New()
	app.Delete("/admin/users/:id/permanent", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", adminID)
		return handler.PermanentlyDeleteUser(c)
	})

	req := httptest.NewRequest("DELETE", "/admin/users/"+adminID.String()+"/permanent", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestAdminHandler_GetAuditLogs_Success(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	logID := uuid.New()

	mockAuditService := &MockAuditServiceImpl{
		GetAuditLogsFunc: func(ctx context.Context, oID uuid.UUID, action string, entityType string, entityID *uuid.UUID, uID *uuid.UUID, startDate *time.Time, endDate *time.Time, limit int, offset int) ([]*domain.AuditLog, int, error) {
			return []*domain.AuditLog{
				{
					ID:             logID,
					OrganizationID: oID,
					UserID:         &userID,
					Action:         domain.AuditActionCreate,
					ResourceType:   "agent",
					ResourceID:     uuid.New(),
				},
			}, 1, nil
		},
		LogActionFunc: func(ctx context.Context, orgID uuid.UUID, userID uuid.UUID, action domain.AuditAction, resourceType string, resourceID uuid.UUID, ipAddress string, userAgent string, metadata map[string]interface{}) error {
			return nil
		},
	}

	handler := createAdminHandlerWithMocks(
		nil,
		nil,
		nil,
		nil,
		mockAuditService,
		nil,
		nil,
		nil,
	)

	app := fiber.New()
	app.Get("/admin/audit-logs", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		return handler.GetAuditLogs(c)
	})

	req := httptest.NewRequest("GET", "/admin/audit-logs", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestAdminHandler_GetAuditLogs_ServiceError(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()

	mockAuditService := &MockAuditServiceImpl{
		GetAuditLogsFunc: func(ctx context.Context, oID uuid.UUID, action string, entityType string, entityID *uuid.UUID, uID *uuid.UUID, startDate *time.Time, endDate *time.Time, limit int, offset int) ([]*domain.AuditLog, int, error) {
			return nil, 0, errors.New("database error")
		},
	}

	handler := createAdminHandlerWithMocks(
		nil,
		nil,
		nil,
		nil,
		mockAuditService,
		nil,
		nil,
		nil,
	)

	app := fiber.New()
	app.Get("/admin/audit-logs", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		return handler.GetAuditLogs(c)
	})

	req := httptest.NewRequest("GET", "/admin/audit-logs", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

func TestAdminHandler_GetAuditLogs_WithFilters(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()

	mockAuditService := &MockAuditServiceImpl{
		GetAuditLogsFunc: func(ctx context.Context, oID uuid.UUID, action string, entityType string, entityID *uuid.UUID, uID *uuid.UUID, startDate *time.Time, endDate *time.Time, limit int, offset int) ([]*domain.AuditLog, int, error) {
			return []*domain.AuditLog{}, 0, nil
		},
		LogActionFunc: func(ctx context.Context, orgID uuid.UUID, userID uuid.UUID, action domain.AuditAction, resourceType string, resourceID uuid.UUID, ipAddress string, userAgent string, metadata map[string]interface{}) error {
			return nil
		},
	}

	handler := createAdminHandlerWithMocks(
		nil,
		nil,
		nil,
		nil,
		mockAuditService,
		nil,
		nil,
		nil,
	)

	app := fiber.New()
	app.Get("/admin/audit-logs", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		return handler.GetAuditLogs(c)
	})

	req := httptest.NewRequest("GET", "/admin/audit-logs?action=create&entity_type=agent&limit=50&offset=10", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestAdminHandler_GetAlerts_Success(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	alertID := uuid.New()

	mockAlertService := &MockAlertServiceExtendedImpl{
		GetAlertsFunc: func(ctx context.Context, oID uuid.UUID, severity, status string, limit, offset int) ([]*domain.Alert, int, error) {
			return []*domain.Alert{
				{
					ID:             alertID,
					OrganizationID: oID,
					Severity:       domain.AlertSeverityCritical,
					Title:          "Test Alert",
				},
			}, 1, nil
		},
		CountUnacknowledgedFunc: func(ctx context.Context, oID uuid.UUID) (allCount, acknowledgedCount, unacknowledgedCount int, err error) {
			return 10, 5, 5, nil
		},
		CountBySeverityFunc: func(ctx context.Context, oID uuid.UUID, status string) (critical, high, warning, info int, err error) {
			return 1, 2, 3, 4, nil
		},
	}

	mockAuditService := &MockAuditServiceImpl{
		LogActionFunc: func(ctx context.Context, orgID uuid.UUID, userID uuid.UUID, action domain.AuditAction, resourceType string, resourceID uuid.UUID, ipAddress string, userAgent string, metadata map[string]interface{}) error {
			return nil
		},
	}

	handler := createAdminHandlerWithMocks(
		nil,
		nil,
		nil,
		nil,
		mockAuditService,
		mockAlertService,
		nil,
		nil,
	)

	app := fiber.New()
	app.Get("/admin/alerts", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		return handler.GetAlerts(c)
	})

	req := httptest.NewRequest("GET", "/admin/alerts", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestAdminHandler_GetAlerts_ServiceError(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()

	mockAlertService := &MockAlertServiceExtendedImpl{
		GetAlertsFunc: func(ctx context.Context, oID uuid.UUID, severity, status string, limit, offset int) ([]*domain.Alert, int, error) {
			return nil, 0, errors.New("database error")
		},
	}

	handler := createAdminHandlerWithMocks(
		nil,
		nil,
		nil,
		nil,
		nil,
		mockAlertService,
		nil,
		nil,
	)

	app := fiber.New()
	app.Get("/admin/alerts", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		return handler.GetAlerts(c)
	})

	req := httptest.NewRequest("GET", "/admin/alerts", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

func TestAdminHandler_GetAlerts_WithFilters(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()

	mockAlertService := &MockAlertServiceExtendedImpl{
		GetAlertsFunc: func(ctx context.Context, oID uuid.UUID, severity, status string, limit, offset int) ([]*domain.Alert, int, error) {
			return []*domain.Alert{}, 0, nil
		},
		CountUnacknowledgedFunc: func(ctx context.Context, oID uuid.UUID) (allCount, acknowledgedCount, unacknowledgedCount int, err error) {
			return 0, 0, 0, nil
		},
		CountBySeverityFunc: func(ctx context.Context, oID uuid.UUID, status string) (critical, high, warning, info int, err error) {
			return 0, 0, 0, 0, nil
		},
	}

	mockAuditService := &MockAuditServiceImpl{
		LogActionFunc: func(ctx context.Context, orgID uuid.UUID, userID uuid.UUID, action domain.AuditAction, resourceType string, resourceID uuid.UUID, ipAddress string, userAgent string, metadata map[string]interface{}) error {
			return nil
		},
	}

	handler := createAdminHandlerWithMocks(
		nil,
		nil,
		nil,
		nil,
		mockAuditService,
		mockAlertService,
		nil,
		nil,
	)

	app := fiber.New()
	app.Get("/admin/alerts", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		return handler.GetAlerts(c)
	})

	req := httptest.NewRequest("GET", "/admin/alerts?severity=critical&status=active&limit=50&offset=10", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestAdminHandler_GetAuditLogByID_Success(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	logID := uuid.New()

	mockAuditService := &MockAuditServiceImpl{
		GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*domain.AuditLog, error) {
			return &domain.AuditLog{
				ID:             id,
				OrganizationID: orgID,
				UserID:         &userID,
				Action:         domain.AuditActionCreate,
				ResourceType:   "agent",
			}, nil
		},
	}

	handler := createAdminHandlerWithMocks(nil, nil, nil, nil, mockAuditService, nil, nil, nil)

	app := fiber.New()
	app.Get("/admin/audit-logs/:id", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		return handler.GetAuditLogByID(c)
	})

	req := httptest.NewRequest("GET", "/admin/audit-logs/"+logID.String(), nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestAdminHandler_GetAuditLogByID_NotFound(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	logID := uuid.New()

	mockAuditService := &MockAuditServiceImpl{
		GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*domain.AuditLog, error) {
			return nil, nil
		},
	}

	handler := createAdminHandlerWithMocks(nil, nil, nil, nil, mockAuditService, nil, nil, nil)

	app := fiber.New()
	app.Get("/admin/audit-logs/:id", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		return handler.GetAuditLogByID(c)
	})

	req := httptest.NewRequest("GET", "/admin/audit-logs/"+logID.String(), nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

func TestAdminHandler_GetAuditLogByID_ServiceError(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	logID := uuid.New()

	mockAuditService := &MockAuditServiceImpl{
		GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*domain.AuditLog, error) {
			return nil, errors.New("database error")
		},
	}

	handler := createAdminHandlerWithMocks(nil, nil, nil, nil, mockAuditService, nil, nil, nil)

	app := fiber.New()
	app.Get("/admin/audit-logs/:id", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		return handler.GetAuditLogByID(c)
	})

	req := httptest.NewRequest("GET", "/admin/audit-logs/"+logID.String(), nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Behavior change (PR for defect #21): loader errors now collapse into
	// 404 instead of 500 so the response shape cannot be used to
	// distinguish "row exists in another org" from "DB error" — both look
	// like "not found" to the caller. See handlers.LoadOwned for the
	// SECURITY rationale. A future PR may introduce domain.ErrNotFound to
	// re-establish the 404-vs-500 distinction for genuine DB failures.
	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

func TestAdminHandler_ExportAuditLogs_Success(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()

	mockAuditService := &MockAuditServiceImpl{
		GetAuditLogsFunc: func(ctx context.Context, oID uuid.UUID, action string, entityType string, entityID *uuid.UUID, uID *uuid.UUID, startDate *time.Time, endDate *time.Time, limit int, offset int) ([]*domain.AuditLog, int, error) {
			return []*domain.AuditLog{{ID: uuid.New(), OrganizationID: oID, Action: domain.AuditActionCreate}}, 1, nil
		},
		LogActionFunc: func(ctx context.Context, orgID, userID uuid.UUID, action domain.AuditAction, resourceType string, resourceID uuid.UUID, ipAddress, userAgent string, metadata map[string]interface{}) error {
			return nil // No-op for testing
		},
	}

	handler := createAdminHandlerWithMocks(nil, nil, nil, nil, mockAuditService, nil, nil, nil)

	app := fiber.New()
	app.Get("/admin/audit-logs/export", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		return handler.ExportAuditLogs(c)
	})

	req := httptest.NewRequest("GET", "/admin/audit-logs/export?format=json", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestAdminHandler_ExportAuditLogs_ServiceError(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()

	mockAuditService := &MockAuditServiceImpl{
		GetAuditLogsFunc: func(ctx context.Context, oID uuid.UUID, action string, entityType string, entityID *uuid.UUID, uID *uuid.UUID, startDate *time.Time, endDate *time.Time, limit int, offset int) ([]*domain.AuditLog, int, error) {
			return nil, 0, errors.New("database error")
		},
	}

	handler := createAdminHandlerWithMocks(nil, nil, nil, nil, mockAuditService, nil, nil, nil)

	app := fiber.New()
	app.Get("/admin/audit-logs/export", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		return handler.ExportAuditLogs(c)
	})

	req := httptest.NewRequest("GET", "/admin/audit-logs/export", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

func TestAdminHandler_ResolveAlert_Success(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	alertID := uuid.New()

	mockAlertService := &MockAlertServiceExtendedImpl{
		ResolveAlertFunc: func(ctx context.Context, aID, oID, uID uuid.UUID, resolution string) error {
			return nil
		},
	}
	mockAuditService := &MockAuditServiceImpl{
		LogActionFunc: func(ctx context.Context, orgID uuid.UUID, userID uuid.UUID, action domain.AuditAction, resourceType string, resourceID uuid.UUID, ipAddress string, userAgent string, metadata map[string]interface{}) error {
			return nil
		},
	}

	handler := createAdminHandlerWithMocks(nil, nil, nil, nil, mockAuditService, mockAlertService, nil, nil)

	app := fiber.New()
	app.Post("/admin/alerts/:id/resolve", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		return handler.ResolveAlert(c)
	})

	body := strings.NewReader(`{"resolution": "Issue fixed"}`)
	req := httptest.NewRequest("POST", "/admin/alerts/"+alertID.String()+"/resolve", body)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusNoContent, resp.StatusCode)
}

func TestAdminHandler_ResolveAlert_ServiceError(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	alertID := uuid.New()

	mockAlertService := &MockAlertServiceExtendedImpl{
		ResolveAlertFunc: func(ctx context.Context, aID, oID, uID uuid.UUID, resolution string) error {
			return errors.New("failed to resolve")
		},
	}

	handler := createAdminHandlerWithMocks(nil, nil, nil, nil, nil, mockAlertService, nil, nil)

	app := fiber.New()
	app.Post("/admin/alerts/:id/resolve", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		return handler.ResolveAlert(c)
	})

	body := strings.NewReader(`{"resolution": "Issue fixed"}`)
	req := httptest.NewRequest("POST", "/admin/alerts/"+alertID.String()+"/resolve", body)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

func TestAdminHandler_RejectUser_Success(t *testing.T) {
	orgID := uuid.New()
	adminID := uuid.New()
	targetUserID := uuid.New()

	mockAdminService := &MockAdminServiceImpl{
		RejectUserFunc: func(ctx context.Context, userID, adminID uuid.UUID, reason string) error {
			return nil
		},
	}
	mockAuditService := &MockAuditServiceImpl{
		LogActionFunc: func(ctx context.Context, orgID uuid.UUID, userID uuid.UUID, action domain.AuditAction, resourceType string, resourceID uuid.UUID, ipAddress string, userAgent string, metadata map[string]interface{}) error {
			return nil
		},
	}

	handler := createAdminHandlerWithMocks(nil, mockAdminService, nil, nil, mockAuditService, nil, nil, nil)

	app := fiber.New()
	app.Post("/admin/users/:id/reject", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", adminID)
		return handler.RejectUser(c)
	})

	body := strings.NewReader(`{"reason": "Did not meet requirements"}`)
	req := httptest.NewRequest("POST", "/admin/users/"+targetUserID.String()+"/reject", body)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestAdminHandler_RejectUser_ServiceError(t *testing.T) {
	orgID := uuid.New()
	adminID := uuid.New()
	targetUserID := uuid.New()

	mockAdminService := &MockAdminServiceImpl{
		RejectUserFunc: func(ctx context.Context, userID, adminID uuid.UUID, reason string) error {
			return errors.New("user not found")
		},
	}

	handler := createAdminHandlerWithMocks(nil, mockAdminService, nil, nil, nil, nil, nil, nil)

	app := fiber.New()
	app.Post("/admin/users/:id/reject", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", adminID)
		return handler.RejectUser(c)
	})

	body := strings.NewReader(`{"reason": "Did not meet requirements"}`)
	req := httptest.NewRequest("POST", "/admin/users/"+targetUserID.String()+"/reject", body)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

func TestAdminHandler_RejectRegistrationRequest_Success(t *testing.T) {
	orgID := uuid.New()
	adminID := uuid.New()
	requestID := uuid.New()

	mockRegistrationService := &MockRegistrationServiceImpl{
		RejectRegistrationRequestFunc: func(ctx context.Context, reqID, adminID uuid.UUID, reason string) error {
			return nil
		},
	}
	mockAuditService := &MockAuditServiceImpl{
		LogActionFunc: func(ctx context.Context, orgID uuid.UUID, userID uuid.UUID, action domain.AuditAction, resourceType string, resourceID uuid.UUID, ipAddress string, userAgent string, metadata map[string]interface{}) error {
			return nil
		},
	}

	handler := createAdminHandlerWithMocks(nil, nil, nil, nil, mockAuditService, nil, mockRegistrationService, nil)

	app := fiber.New()
	app.Post("/admin/registration-requests/:id/reject", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", adminID)
		return handler.RejectRegistrationRequest(c)
	})

	body := strings.NewReader(`{"reason": "Invalid email domain"}`)
	req := httptest.NewRequest("POST", "/admin/registration-requests/"+requestID.String()+"/reject", body)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestAdminHandler_RejectRegistrationRequest_ServiceError(t *testing.T) {
	orgID := uuid.New()
	adminID := uuid.New()
	requestID := uuid.New()

	mockRegistrationService := &MockRegistrationServiceImpl{
		RejectRegistrationRequestFunc: func(ctx context.Context, reqID, adminID uuid.UUID, reason string) error {
			return errors.New("request not found")
		},
	}

	handler := createAdminHandlerWithMocks(nil, nil, nil, nil, nil, nil, mockRegistrationService, nil)

	app := fiber.New()
	app.Post("/admin/registration-requests/:id/reject", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", adminID)
		return handler.RejectRegistrationRequest(c)
	})

	body := strings.NewReader(`{"reason": "Invalid email domain"}`)
	req := httptest.NewRequest("POST", "/admin/registration-requests/"+requestID.String()+"/reject", body)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

func TestAdminHandler_ApproveRegistrationRequest_Success(t *testing.T) {
	orgID := uuid.New()
	adminID := uuid.New()
	requestID := uuid.New()

	mockRegistrationService := &MockRegistrationServiceImpl{
		ApproveRegistrationRequestFunc: func(ctx context.Context, reqID, adminID, oID uuid.UUID) (*domain.User, error) {
			return &domain.User{ID: uuid.New(), OrganizationID: oID, Email: "newuser@example.com", Name: "New User"}, nil
		},
	}
	mockAuditService := &MockAuditServiceImpl{
		LogActionFunc: func(ctx context.Context, orgID uuid.UUID, userID uuid.UUID, action domain.AuditAction, resourceType string, resourceID uuid.UUID, ipAddress string, userAgent string, metadata map[string]interface{}) error {
			return nil
		},
	}

	handler := createAdminHandlerWithMocks(nil, nil, nil, nil, mockAuditService, nil, mockRegistrationService, nil)

	app := fiber.New()
	app.Post("/admin/registration-requests/:id/approve", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", adminID)
		return handler.ApproveRegistrationRequest(c)
	})

	req := httptest.NewRequest("POST", "/admin/registration-requests/"+requestID.String()+"/approve", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestAdminHandler_ApproveRegistrationRequest_ServiceError(t *testing.T) {
	orgID := uuid.New()
	adminID := uuid.New()
	requestID := uuid.New()

	mockRegistrationService := &MockRegistrationServiceImpl{
		ApproveRegistrationRequestFunc: func(ctx context.Context, reqID, adminID, oID uuid.UUID) (*domain.User, error) {
			return nil, errors.New("request not found")
		},
	}

	handler := createAdminHandlerWithMocks(nil, nil, nil, nil, nil, nil, mockRegistrationService, nil)

	app := fiber.New()
	app.Post("/admin/registration-requests/:id/approve", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", adminID)
		return handler.ApproveRegistrationRequest(c)
	})

	req := httptest.NewRequest("POST", "/admin/registration-requests/"+requestID.String()+"/approve", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

func TestAdminHandler_ApproveUser_Success(t *testing.T) {
	orgID := uuid.New()
	adminID := uuid.New()
	targetUserID := uuid.New()

	mockAdminService := &MockAdminServiceImpl{
		ApproveUserFunc: func(ctx context.Context, userID, adminID uuid.UUID) error {
			return nil
		},
	}
	mockAuditService := &MockAuditServiceImpl{
		LogActionFunc: func(ctx context.Context, orgID uuid.UUID, userID uuid.UUID, action domain.AuditAction, resourceType string, resourceID uuid.UUID, ipAddress string, userAgent string, metadata map[string]interface{}) error {
			return nil
		},
	}

	handler := createAdminHandlerWithMocks(nil, mockAdminService, nil, nil, mockAuditService, nil, nil, nil)

	app := fiber.New()
	app.Post("/admin/users/:id/approve", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", adminID)
		return handler.ApproveUser(c)
	})

	req := httptest.NewRequest("POST", "/admin/users/"+targetUserID.String()+"/approve", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestAdminHandler_ApproveUser_ServiceError(t *testing.T) {
	orgID := uuid.New()
	adminID := uuid.New()
	targetUserID := uuid.New()

	mockAdminService := &MockAdminServiceImpl{
		ApproveUserFunc: func(ctx context.Context, userID, adminID uuid.UUID) error {
			return errors.New("user not found")
		},
	}

	handler := createAdminHandlerWithMocks(nil, mockAdminService, nil, nil, nil, nil, nil, nil)

	app := fiber.New()
	app.Post("/admin/users/:id/approve", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", adminID)
		return handler.ApproveUser(c)
	})

	req := httptest.NewRequest("POST", "/admin/users/"+targetUserID.String()+"/approve", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

func TestAdminHandler_GetUnacknowledgedAlertCount_Success(t *testing.T) {
	orgID := uuid.New()

	mockAlertService := &MockAlertServiceExtendedImpl{
		CountUnacknowledgedFunc: func(ctx context.Context, oID uuid.UUID) (allCount, acknowledgedCount, unacknowledgedCount int, err error) {
			return 10, 5, 5, nil
		},
	}

	handler := createAdminHandlerWithMocks(nil, nil, nil, nil, nil, mockAlertService, nil, nil)

	app := fiber.New()
	app.Get("/admin/alerts/unacknowledged-count", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		return handler.GetUnacknowledgedAlertCount(c)
	})

	req := httptest.NewRequest("GET", "/admin/alerts/unacknowledged-count", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.Equal(t, float64(10), result["allCount"])
	assert.Equal(t, float64(5), result["acknowledgedCount"])
	assert.Equal(t, float64(5), result["unacknowledgedCount"])
}

func TestAdminHandler_GetUnacknowledgedAlertCount_ServiceError(t *testing.T) {
	orgID := uuid.New()

	mockAlertService := &MockAlertServiceExtendedImpl{
		CountUnacknowledgedFunc: func(ctx context.Context, oID uuid.UUID) (allCount, acknowledgedCount, unacknowledgedCount int, err error) {
			return 0, 0, 0, errors.New("database error")
		},
	}

	handler := createAdminHandlerWithMocks(nil, nil, nil, nil, nil, mockAlertService, nil, nil)

	app := fiber.New()
	app.Get("/admin/alerts/unacknowledged-count", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		return handler.GetUnacknowledgedAlertCount(c)
	})

	req := httptest.NewRequest("GET", "/admin/alerts/unacknowledged-count", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

func TestAdminHandler_GetEnforcementSettings_Success(t *testing.T) {
	orgID := uuid.New()

	mockAdminService := &MockAdminServiceImpl{
		GetEnforcementSettingsFunc: func(ctx context.Context, oID uuid.UUID) (*application.EnforcementSettings, error) {
			return &application.EnforcementSettings{
				EnforcementMode: "strict",
			}, nil
		},
	}

	handler := createAdminHandlerWithMocks(nil, mockAdminService, nil, nil, nil, nil, nil, nil)

	app := fiber.New()
	app.Get("/admin/enforcement-settings", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		return handler.GetEnforcementSettings(c)
	})

	req := httptest.NewRequest("GET", "/admin/enforcement-settings", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestAdminHandler_GetEnforcementSettings_ServiceError(t *testing.T) {
	orgID := uuid.New()

	mockAdminService := &MockAdminServiceImpl{
		GetEnforcementSettingsFunc: func(ctx context.Context, oID uuid.UUID) (*application.EnforcementSettings, error) {
			return nil, errors.New("database error")
		},
	}

	handler := createAdminHandlerWithMocks(nil, mockAdminService, nil, nil, nil, nil, nil, nil)

	app := fiber.New()
	app.Get("/admin/enforcement-settings", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		return handler.GetEnforcementSettings(c)
	})

	req := httptest.NewRequest("GET", "/admin/enforcement-settings", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

func TestAdminHandler_UpdateEnforcementSettings_Success(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()

	mockAdminService := &MockAdminServiceImpl{
		UpdateEnforcementModeFunc: func(ctx context.Context, oID uuid.UUID, mode domain.EnforcementMode) error {
			return nil
		},
		GetEnforcementSettingsFunc: func(ctx context.Context, oID uuid.UUID) (*application.EnforcementSettings, error) {
			return &application.EnforcementSettings{
				EnforcementMode: "strict",
			}, nil
		},
	}
	mockAuditService := &MockAuditServiceImpl{
		LogActionFunc: func(ctx context.Context, orgID uuid.UUID, userID uuid.UUID, action domain.AuditAction, resourceType string, resourceID uuid.UUID, ipAddress string, userAgent string, metadata map[string]interface{}) error {
			return nil
		},
	}

	handler := createAdminHandlerWithMocks(nil, mockAdminService, nil, nil, mockAuditService, nil, nil, nil)

	app := fiber.New()
	app.Put("/admin/enforcement-settings", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		return handler.UpdateEnforcementSettings(c)
	})

	body := strings.NewReader(`{"enforcementMode": "strict"}`)
	req := httptest.NewRequest("PUT", "/admin/enforcement-settings", body)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestAdminHandler_UpdateEnforcementSettings_ServiceError(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()

	mockAdminService := &MockAdminServiceImpl{
		UpdateEnforcementModeFunc: func(ctx context.Context, oID uuid.UUID, mode domain.EnforcementMode) error {
			return errors.New("database error")
		},
	}

	handler := createAdminHandlerWithMocks(nil, mockAdminService, nil, nil, nil, nil, nil, nil)

	app := fiber.New()
	app.Put("/admin/enforcement-settings", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		return handler.UpdateEnforcementSettings(c)
	})

	body := strings.NewReader(`{"enforcementMode": "strict"}`)
	req := httptest.NewRequest("PUT", "/admin/enforcement-settings", body)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

func TestAdminHandler_BulkAcknowledgeAlerts_Success(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()

	mockAlertService := &MockAlertServiceExtendedImpl{
		BulkAcknowledgeAlertsFunc: func(ctx context.Context, oID, uID uuid.UUID) (int, error) {
			return 5, nil
		},
	}
	mockAuditService := &MockAuditServiceImpl{
		LogActionFunc: func(ctx context.Context, orgID uuid.UUID, userID uuid.UUID, action domain.AuditAction, resourceType string, resourceID uuid.UUID, ipAddress string, userAgent string, metadata map[string]interface{}) error {
			return nil
		},
	}

	handler := createAdminHandlerWithMocks(nil, nil, nil, nil, mockAuditService, mockAlertService, nil, nil)

	app := fiber.New()
	app.Post("/admin/alerts/bulk-acknowledge", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		return handler.BulkAcknowledgeAlerts(c)
	})

	body := strings.NewReader(`{}`)
	req := httptest.NewRequest("POST", "/admin/alerts/bulk-acknowledge", body)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.Equal(t, float64(5), result["acknowledgedCount"])
}

func TestAdminHandler_BulkAcknowledgeAlerts_ServiceError(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()

	mockAlertService := &MockAlertServiceExtendedImpl{
		BulkAcknowledgeAlertsFunc: func(ctx context.Context, oID, uID uuid.UUID) (int, error) {
			return 0, errors.New("database error")
		},
	}

	handler := createAdminHandlerWithMocks(nil, nil, nil, nil, nil, mockAlertService, nil, nil)

	app := fiber.New()
	app.Post("/admin/alerts/bulk-acknowledge", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		return handler.BulkAcknowledgeAlerts(c)
	})

	body := strings.NewReader(`{}`)
	req := httptest.NewRequest("POST", "/admin/alerts/bulk-acknowledge", body)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

func TestAdminHandler_AcknowledgeAlert_Success(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	alertID := uuid.New()

	mockAlertService := &MockAlertServiceExtendedImpl{
		AcknowledgeAlertFunc: func(ctx context.Context, aID, oID, uID uuid.UUID) error {
			return nil
		},
	}
	mockAuditService := &MockAuditServiceImpl{
		LogActionFunc: func(ctx context.Context, orgID uuid.UUID, userID uuid.UUID, action domain.AuditAction, resourceType string, resourceID uuid.UUID, ipAddress string, userAgent string, metadata map[string]interface{}) error {
			return nil
		},
	}

	handler := createAdminHandlerWithMocks(nil, nil, nil, nil, mockAuditService, mockAlertService, nil, nil)

	app := fiber.New()
	app.Post("/admin/alerts/:id/acknowledge", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		return handler.AcknowledgeAlert(c)
	})

	req := httptest.NewRequest("POST", "/admin/alerts/"+alertID.String()+"/acknowledge", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusNoContent, resp.StatusCode)
}

func TestAdminHandler_AcknowledgeAlert_ServiceError(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	alertID := uuid.New()

	mockAlertService := &MockAlertServiceExtendedImpl{
		AcknowledgeAlertFunc: func(ctx context.Context, aID, oID, uID uuid.UUID) error {
			return errors.New("alert not found")
		},
	}

	handler := createAdminHandlerWithMocks(nil, nil, nil, nil, nil, mockAlertService, nil, nil)

	app := fiber.New()
	app.Post("/admin/alerts/:id/acknowledge", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		return handler.AcknowledgeAlert(c)
	})

	req := httptest.NewRequest("POST", "/admin/alerts/"+alertID.String()+"/acknowledge", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

func TestAdminHandler_GetOrganizationSettings_Success(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()

	mockAdminService := &MockAdminServiceImpl{
		GetOrganizationSettingsFunc: func(ctx context.Context, oID uuid.UUID) (*domain.Organization, error) {
			return &domain.Organization{
				ID:        oID,
				Name:      "Test Org",
				Domain:    "test.example.com",
				MaxAgents: 100,
				MaxUsers:  50,
				IsActive:  true,
			}, nil
		},
	}
	mockAuditService := &MockAuditServiceImpl{
		LogActionFunc: func(ctx context.Context, orgID uuid.UUID, userID uuid.UUID, action domain.AuditAction, resourceType string, resourceID uuid.UUID, ipAddress string, userAgent string, metadata map[string]interface{}) error {
			return nil
		},
	}

	handler := createAdminHandlerWithMocks(nil, mockAdminService, nil, nil, mockAuditService, nil, nil, nil)

	app := fiber.New()
	app.Get("/admin/organization-settings", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		return handler.GetOrganizationSettings(c)
	})

	req := httptest.NewRequest("GET", "/admin/organization-settings", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.Equal(t, "Test Org", result["name"])
	assert.Equal(t, "test.example.com", result["domain"])
}

func TestAdminHandler_GetOrganizationSettings_ServiceError(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()

	mockAdminService := &MockAdminServiceImpl{
		GetOrganizationSettingsFunc: func(ctx context.Context, oID uuid.UUID) (*domain.Organization, error) {
			return nil, errors.New("database error")
		},
	}

	handler := createAdminHandlerWithMocks(nil, mockAdminService, nil, nil, nil, nil, nil, nil)

	app := fiber.New()
	app.Get("/admin/organization-settings", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		return handler.GetOrganizationSettings(c)
	})

	req := httptest.NewRequest("GET", "/admin/organization-settings", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}
