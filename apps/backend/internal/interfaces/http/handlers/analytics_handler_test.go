package handlers

import (
	"context"
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
// NewAnalyticsHandler Tests
// ===========================

func TestNewAnalyticsHandler_NilDeps(t *testing.T) {
	handler := NewAnalyticsHandler(nil, nil, nil, nil, nil, nil, nil, nil)
	assert.NotNil(t, handler)
}

// ===========================
// AnalyticsHandler.GetUsageStatistics Tests
// ===========================

func TestAnalyticsHandler_GetUsageStatistics_NoOrgContext(t *testing.T) {
	handler := &AnalyticsHandler{}
	app := fiber.New()
	app.Get("/analytics/usage", handler.GetUsageStatistics)

	req := httptest.NewRequest("GET", "/analytics/usage", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

// ===========================
// AnalyticsHandler.GetTrustScoreTrends Tests
// ===========================

func TestAnalyticsHandler_GetTrustScoreTrends_NoOrgContext(t *testing.T) {
	handler := &AnalyticsHandler{}
	app := fiber.New()
	app.Get("/analytics/trust-trends", handler.GetTrustScoreTrends)

	req := httptest.NewRequest("GET", "/analytics/trust-trends", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

// ===========================
// AnalyticsHandler.GetVerificationActivity Tests
// ===========================

func TestAnalyticsHandler_GetVerificationActivity_NoOrgContext(t *testing.T) {
	handler := &AnalyticsHandler{}
	app := fiber.New()
	app.Get("/analytics/verifications", handler.GetVerificationActivity)

	req := httptest.NewRequest("GET", "/analytics/verifications", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

// ===========================
// AnalyticsHandler.GetAgentActivity Tests
// ===========================

func TestAnalyticsHandler_GetAgentActivity_NoOrgContext(t *testing.T) {
	handler := &AnalyticsHandler{}
	app := fiber.New()
	app.Get("/analytics/agents", handler.GetAgentActivity)

	req := httptest.NewRequest("GET", "/analytics/agents", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

// ===========================
// AnalyticsHandler.GetDashboardStats Tests
// ===========================

func TestAnalyticsHandler_GetDashboardStats_NoOrgContext(t *testing.T) {
	handler := &AnalyticsHandler{}
	app := fiber.New()
	app.Get("/analytics/dashboard", handler.GetDashboardStats)

	req := httptest.NewRequest("GET", "/analytics/dashboard", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

// ===========================
// AnalyticsHandler.GetActivitySummary Tests
// ===========================

func TestAnalyticsHandler_GetActivitySummary_NoOrgContext(t *testing.T) {
	handler := &AnalyticsHandler{}
	app := fiber.New()
	app.Get("/analytics/activity-summary", handler.GetActivitySummary)

	req := httptest.NewRequest("GET", "/analytics/activity-summary", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

// ===========================
// Helper Function Tests
// ===========================

func TestAnalytics_countByStatus(t *testing.T) {
	agents := []*domain.Agent{
		{ID: uuid.New(), Status: domain.AgentStatusVerified},
		{ID: uuid.New(), Status: domain.AgentStatusVerified},
		{ID: uuid.New(), Status: domain.AgentStatusPending},
		{ID: uuid.New(), Status: domain.AgentStatusSuspended},
	}

	tests := []struct {
		name     string
		status   string
		expected int
	}{
		{"Verified agents", string(domain.AgentStatusVerified), 2},
		{"Pending agents", string(domain.AgentStatusPending), 1},
		{"Suspended agents", string(domain.AgentStatusSuspended), 1},
		{"Revoked agents (none)", string(domain.AgentStatusRevoked), 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := countByStatus(agents, tt.status)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAnalytics_countByStatus_EmptyAgents(t *testing.T) {
	result := countByStatus([]*domain.Agent{}, string(domain.AgentStatusVerified))
	assert.Equal(t, 0, result)
}

func TestAnalytics_countByStatus_NilAgents(t *testing.T) {
	result := countByStatus(nil, string(domain.AgentStatusVerified))
	assert.Equal(t, 0, result)
}

func TestAnalytics_calculateAverageTrustScore(t *testing.T) {
	tests := []struct {
		name     string
		agents   []*domain.Agent
		expected float64
	}{
		{
			name:     "Empty agents",
			agents:   []*domain.Agent{},
			expected: 0.0,
		},
		{
			name:     "Nil agents",
			agents:   nil,
			expected: 0.0,
		},
		{
			name: "Single agent",
			agents: []*domain.Agent{
				{ID: uuid.New(), TrustScore: 0.8},
			},
			expected: 0.8,
		},
		{
			name: "Multiple agents",
			agents: []*domain.Agent{
				{ID: uuid.New(), TrustScore: 0.8},
				{ID: uuid.New(), TrustScore: 0.6},
				{ID: uuid.New(), TrustScore: 1.0},
			},
			expected: 0.8, // (0.8 + 0.6 + 1.0) / 3 = 0.8
		},
		{
			name: "All zeros",
			agents: []*domain.Agent{
				{ID: uuid.New(), TrustScore: 0.0},
				{ID: uuid.New(), TrustScore: 0.0},
			},
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateAverageTrustScore(tt.agents)
			// Use InDelta for floating point comparison
			assert.InDelta(t, tt.expected, result, 0.0001)
		})
	}
}

// ===========================
// AnalyticsHandler.GetTrustScoreTrends Additional Tests
// ===========================

func TestAnalyticsHandler_GetTrustScoreTrends_InvalidOrgIDType(t *testing.T) {
	handler := &AnalyticsHandler{}
	app := fiber.New()
	app.Get("/analytics/trust-trends", func(c fiber.Ctx) error {
		c.Locals("organization_id", "not-a-uuid")
		return handler.GetTrustScoreTrends(c)
	})

	req := httptest.NewRequest("GET", "/analytics/trust-trends", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

// ===========================
// AnalyticsHandler.GetVerificationActivity Additional Tests
// ===========================

func TestAnalyticsHandler_GetVerificationActivity_InvalidOrgIDType(t *testing.T) {
	handler := &AnalyticsHandler{}
	app := fiber.New()
	app.Get("/analytics/verifications", func(c fiber.Ctx) error {
		c.Locals("organization_id", "not-a-uuid")
		return handler.GetVerificationActivity(c)
	})

	req := httptest.NewRequest("GET", "/analytics/verifications", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

// ===========================
// AnalyticsHandler.GetActivitySummary Additional Tests
// ===========================

func TestAnalyticsHandler_GetActivitySummary_InvalidOrgIDType(t *testing.T) {
	handler := &AnalyticsHandler{}
	app := fiber.New()
	app.Get("/analytics/activity-summary", func(c fiber.Ctx) error {
		c.Locals("organization_id", "not-a-uuid")
		return handler.GetActivitySummary(c)
	})

	req := httptest.NewRequest("GET", "/analytics/activity-summary", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// GetActivitySummary returns Unauthorized for invalid org ID type
	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

// ===========================
// AnalyticsHandler.GetVerificationActivity Success Tests
// ===========================

func TestAnalyticsHandler_GetVerificationActivity_Success(t *testing.T) {
	orgID := uuid.New()
	agentID := uuid.New()
	mcpID := uuid.New()

	mockAgentService := &MockAgentServiceImpl{
		ListAgentsFunc: func(ctx context.Context, org uuid.UUID) ([]*domain.Agent, error) {
			return []*domain.Agent{
				{ID: agentID, OrganizationID: orgID, Name: "Test Agent", Status: domain.AgentStatusVerified, TrustScore: 0.85},
				{ID: uuid.New(), OrganizationID: orgID, Name: "Pending Agent", Status: domain.AgentStatusPending, TrustScore: 0.5},
			}, nil
		},
	}

	mockMCPService := &MockMCPServiceImpl{
		ListMCPServersFunc: func(ctx context.Context, org uuid.UUID) ([]*domain.MCPServer, error) {
			return []*domain.MCPServer{
				{ID: mcpID, OrganizationID: orgID, Name: "Test MCP Server", Status: domain.MCPServerStatusVerified},
			}, nil
		},
	}

	handler := NewAnalyticsHandlerWithInterfaces(
		mockAgentService,
		mockMCPService,
		nil, // verification event service
		nil, // auth service
		nil, // alert service
		nil, // security service
		nil, // db
	)

	app := fiber.New()
	app.Get("/analytics/verification-activity", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		return handler.GetVerificationActivity(c)
	})

	req := httptest.NewRequest("GET", "/analytics/verification-activity?months=6", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestAnalyticsHandler_GetVerificationActivity_AgentServiceError(t *testing.T) {
	orgID := uuid.New()

	mockAgentService := &MockAgentServiceImpl{
		ListAgentsFunc: func(ctx context.Context, org uuid.UUID) ([]*domain.Agent, error) {
			return nil, assert.AnError
		},
	}

	handler := NewAnalyticsHandlerWithInterfaces(
		mockAgentService,
		nil, // mcp service
		nil, // verification event service
		nil, // auth service
		nil, // alert service
		nil, // security service
		nil, // db
	)

	app := fiber.New()
	app.Get("/analytics/verification-activity", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		return handler.GetVerificationActivity(c)
	})

	req := httptest.NewRequest("GET", "/analytics/verification-activity", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

func TestAnalyticsHandler_GetVerificationActivity_MCPServiceError(t *testing.T) {
	orgID := uuid.New()

	mockAgentService := &MockAgentServiceImpl{
		ListAgentsFunc: func(ctx context.Context, org uuid.UUID) ([]*domain.Agent, error) {
			return []*domain.Agent{}, nil
		},
	}

	mockMCPService := &MockMCPServiceImpl{
		ListMCPServersFunc: func(ctx context.Context, org uuid.UUID) ([]*domain.MCPServer, error) {
			return nil, assert.AnError // Error returns empty list, not failure
		},
	}

	handler := NewAnalyticsHandlerWithInterfaces(
		mockAgentService,
		mockMCPService,
		nil, nil, nil, nil, nil,
	)

	app := fiber.New()
	app.Get("/analytics/verification-activity", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		return handler.GetVerificationActivity(c)
	})

	req := httptest.NewRequest("GET", "/analytics/verification-activity", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// MCP service error is handled gracefully - returns empty list
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

// ===========================
// AnalyticsHandler.GetTrustScoreTrends Success Tests
// ===========================

func TestAnalyticsHandler_GetTrustScoreTrends_Success_WeeklyPeriod(t *testing.T) {
	orgID := uuid.New()

	mockAgentService := &MockAgentServiceImpl{
		ListAgentsFunc: func(ctx context.Context, org uuid.UUID) ([]*domain.Agent, error) {
			return []*domain.Agent{
				{ID: uuid.New(), OrganizationID: orgID, Name: "Agent 1", TrustScore: 0.9, CreatedAt: time.Now().AddDate(0, 0, -7)},
				{ID: uuid.New(), OrganizationID: orgID, Name: "Agent 2", TrustScore: 0.8, CreatedAt: time.Now().AddDate(0, 0, -14)},
			}, nil
		},
	}

	handler := NewAnalyticsHandlerWithInterfaces(
		mockAgentService,
		nil, nil, nil, nil, nil, nil,
	)

	app := fiber.New()
	app.Get("/analytics/trust-trends", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		return handler.GetTrustScoreTrends(c)
	})

	req := httptest.NewRequest("GET", "/analytics/trust-trends?period=weeks&weeks=4", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestAnalyticsHandler_GetTrustScoreTrends_Success_DailyPeriod(t *testing.T) {
	orgID := uuid.New()

	mockAgentService := &MockAgentServiceImpl{
		ListAgentsFunc: func(ctx context.Context, org uuid.UUID) ([]*domain.Agent, error) {
			return []*domain.Agent{
				{ID: uuid.New(), OrganizationID: orgID, Name: "Agent 1", TrustScore: 0.85, CreatedAt: time.Now().AddDate(0, 0, -1)},
				{ID: uuid.New(), OrganizationID: orgID, Name: "Agent 2", TrustScore: 0.75, CreatedAt: time.Now().AddDate(0, 0, -3)},
			}, nil
		},
	}

	handler := NewAnalyticsHandlerWithInterfaces(
		mockAgentService,
		nil, nil, nil, nil, nil, nil,
	)

	app := fiber.New()
	app.Get("/analytics/trust-trends", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		return handler.GetTrustScoreTrends(c)
	})

	req := httptest.NewRequest("GET", "/analytics/trust-trends?period=days&days=30", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestAnalyticsHandler_GetTrustScoreTrends_EmptyAgents(t *testing.T) {
	orgID := uuid.New()

	mockAgentService := &MockAgentServiceImpl{
		ListAgentsFunc: func(ctx context.Context, org uuid.UUID) ([]*domain.Agent, error) {
			return []*domain.Agent{}, nil
		},
	}

	handler := NewAnalyticsHandlerWithInterfaces(
		mockAgentService,
		nil, nil, nil, nil, nil, nil,
	)

	app := fiber.New()
	app.Get("/analytics/trust-trends", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		return handler.GetTrustScoreTrends(c)
	})

	req := httptest.NewRequest("GET", "/analytics/trust-trends", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

// ===========================
// AnalyticsHandler.GetDashboardStats Success Tests
// ===========================

func TestAnalyticsHandler_GetDashboardStats_Success(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()

	mockAgentService := &MockAgentServiceImpl{
		ListAgentsFunc: func(ctx context.Context, org uuid.UUID) ([]*domain.Agent, error) {
			return []*domain.Agent{
				{ID: uuid.New(), OrganizationID: orgID, Name: "Agent 1", Status: domain.AgentStatusVerified, TrustScore: 0.9},
				{ID: uuid.New(), OrganizationID: orgID, Name: "Agent 2", Status: domain.AgentStatusPending, TrustScore: 0.5},
			}, nil
		},
	}

	mockMCPService := &MockMCPServiceImpl{
		ListMCPServersFunc: func(ctx context.Context, org uuid.UUID) ([]*domain.MCPServer, error) {
			return []*domain.MCPServer{
				{ID: uuid.New(), OrganizationID: orgID, Name: "MCP 1", Status: domain.MCPServerStatusVerified},
			}, nil
		},
	}

	mockVerificationService := &MockVerificationEventServiceExtendedImpl{
		GetLast24HoursStatisticsFunc: func(ctx context.Context, org uuid.UUID) (*domain.VerificationStatistics, error) {
			return &domain.VerificationStatistics{
				TotalVerifications: 100,
				SuccessCount:       95,
				FailedCount:        5,
				PendingCount:       0,
				AvgDurationMs:      150,
			}, nil
		},
	}

	mockAuthService := &MockAuthServiceImpl{
		GetUsersByOrganizationFunc: func(ctx context.Context, org uuid.UUID) ([]*domain.User, error) {
			return []*domain.User{
				{ID: userID, OrganizationID: orgID, Email: "user@example.com", Status: domain.UserStatusActive},
			}, nil
		},
	}

	mockAlertService := &MockAlertServiceExtendedImpl{
		GetAlertsFunc: func(ctx context.Context, org uuid.UUID, severity, status string, limit, offset int) ([]*domain.Alert, int, error) {
			return []*domain.Alert{
				{ID: uuid.New(), Severity: domain.AlertSeverityCritical},
			}, 1, nil
		},
	}

	mockSecurityService := &MockSecurityServiceForAnalyticsImpl{
		GetIncidentsFunc: func(ctx context.Context, org uuid.UUID, status domain.IncidentStatus, limit, offset int) ([]*domain.SecurityIncident, error) {
			return []*domain.SecurityIncident{}, nil
		},
	}

	handler := NewAnalyticsHandlerWithInterfaces(
		mockAgentService,
		mockMCPService,
		mockVerificationService,
		mockAuthService,
		mockAlertService,
		mockSecurityService,
		nil, // db
	)

	app := fiber.New()
	app.Get("/analytics/dashboard", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		return handler.GetDashboardStats(c)
	})

	req := httptest.NewRequest("GET", "/analytics/dashboard", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestAnalyticsHandler_GetDashboardStats_InvalidOrgIDType(t *testing.T) {
	handler := &AnalyticsHandler{}
	app := fiber.New()
	app.Get("/analytics/dashboard", func(c fiber.Ctx) error {
		c.Locals("organization_id", "not-a-uuid")
		return handler.GetDashboardStats(c)
	})

	req := httptest.NewRequest("GET", "/analytics/dashboard", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

func TestAnalyticsHandler_GetDashboardStats_AgentServiceError(t *testing.T) {
	orgID := uuid.New()

	mockAgentService := &MockAgentServiceImpl{
		ListAgentsFunc: func(ctx context.Context, org uuid.UUID) ([]*domain.Agent, error) {
			return nil, assert.AnError
		},
	}

	handler := NewAnalyticsHandlerWithInterfaces(
		mockAgentService,
		nil, nil, nil, nil, nil, nil,
	)

	app := fiber.New()
	app.Get("/analytics/dashboard", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		return handler.GetDashboardStats(c)
	})

	req := httptest.NewRequest("GET", "/analytics/dashboard", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

func TestAnalyticsHandler_GetDashboardStats_MCPServiceError(t *testing.T) {
	orgID := uuid.New()

	mockAgentService := &MockAgentServiceImpl{
		ListAgentsFunc: func(ctx context.Context, org uuid.UUID) ([]*domain.Agent, error) {
			return []*domain.Agent{}, nil
		},
	}

	mockMCPService := &MockMCPServiceImpl{
		ListMCPServersFunc: func(ctx context.Context, org uuid.UUID) ([]*domain.MCPServer, error) {
			return nil, assert.AnError
		},
	}

	handler := NewAnalyticsHandlerWithInterfaces(
		mockAgentService,
		mockMCPService,
		nil, nil, nil, nil, nil,
	)

	app := fiber.New()
	app.Get("/analytics/dashboard", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		return handler.GetDashboardStats(c)
	})

	req := httptest.NewRequest("GET", "/analytics/dashboard", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

// ===========================
// AnalyticsHandler.GetUsageStatistics Success Tests
// ===========================

func TestAnalyticsHandler_GetUsageStatistics_Success(t *testing.T) {
	orgID := uuid.New()

	mockAgentService := &MockAgentServiceImpl{
		ListAgentsFunc: func(ctx context.Context, org uuid.UUID) ([]*domain.Agent, error) {
			return []*domain.Agent{
				{ID: uuid.New(), OrganizationID: orgID, Name: "Agent 1", Status: domain.AgentStatusVerified},
				{ID: uuid.New(), OrganizationID: orgID, Name: "Agent 2", Status: domain.AgentStatusPending},
			}, nil
		},
	}

	handler := NewAnalyticsHandlerWithInterfaces(
		mockAgentService,
		nil, nil, nil, nil, nil, nil,
	)

	app := fiber.New()
	app.Get("/analytics/usage", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		return handler.GetUsageStatistics(c)
	})

	req := httptest.NewRequest("GET", "/analytics/usage?days=30", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestAnalyticsHandler_GetUsageStatistics_WithPeriodParam(t *testing.T) {
	orgID := uuid.New()

	mockAgentService := &MockAgentServiceImpl{
		ListAgentsFunc: func(ctx context.Context, org uuid.UUID) ([]*domain.Agent, error) {
			return []*domain.Agent{}, nil
		},
	}

	handler := NewAnalyticsHandlerWithInterfaces(
		mockAgentService,
		nil, nil, nil, nil, nil, nil,
	)

	tests := []struct {
		name   string
		period string
	}{
		{"day period", "day"},
		{"week period", "week"},
		{"month period", "month"},
		{"year period", "year"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := fiber.New()
			app.Get("/analytics/usage", func(c fiber.Ctx) error {
				c.Locals("organization_id", orgID)
				return handler.GetUsageStatistics(c)
			})

			req := httptest.NewRequest("GET", "/analytics/usage?period="+tt.period, nil)

			resp, err := app.Test(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, fiber.StatusOK, resp.StatusCode)
		})
	}
}

func TestAnalyticsHandler_GetUsageStatistics_AgentServiceError(t *testing.T) {
	orgID := uuid.New()

	mockAgentService := &MockAgentServiceImpl{
		ListAgentsFunc: func(ctx context.Context, org uuid.UUID) ([]*domain.Agent, error) {
			return nil, assert.AnError
		},
	}

	handler := NewAnalyticsHandlerWithInterfaces(
		mockAgentService,
		nil, nil, nil, nil, nil, nil,
	)

	app := fiber.New()
	app.Get("/analytics/usage", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		return handler.GetUsageStatistics(c)
	})

	req := httptest.NewRequest("GET", "/analytics/usage", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

// ===========================
// AnalyticsHandler.GetAgentActivity Success Tests
// ===========================

func TestAnalyticsHandler_GetAgentActivity_NoDatabase(t *testing.T) {
	orgID := uuid.New()

	// Handler with no database returns empty activities gracefully
	handler := NewAnalyticsHandlerWithInterfaces(
		nil, nil, nil, nil, nil, nil, nil,
	)

	app := fiber.New()
	app.Get("/analytics/agents/activity", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		return handler.GetAgentActivity(c)
	})

	req := httptest.NewRequest("GET", "/analytics/agents/activity?limit=50&offset=0", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

// ===========================
// AnalyticsHandler.GetActivitySummary Success Tests
// ===========================

func TestAnalyticsHandler_GetActivitySummary_Success(t *testing.T) {
	orgID := uuid.New()

	mockAgentService := &MockAgentServiceImpl{
		ListAgentsFunc: func(ctx context.Context, org uuid.UUID) ([]*domain.Agent, error) {
			return []*domain.Agent{
				{ID: uuid.New(), OrganizationID: orgID, Name: "Agent 1"},
			}, nil
		},
	}

	mockMCPService := &MockMCPServiceImpl{
		ListMCPServersFunc: func(ctx context.Context, org uuid.UUID) ([]*domain.MCPServer, error) {
			return []*domain.MCPServer{}, nil
		},
	}

	handler := NewAnalyticsHandlerWithInterfaces(
		mockAgentService,
		mockMCPService,
		nil, nil, nil, nil, nil,
	)

	app := fiber.New()
	app.Get("/analytics/activity-summary", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		return handler.GetActivitySummary(c)
	})

	req := httptest.NewRequest("GET", "/analytics/activity-summary?days=7", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestAnalyticsHandler_GetActivitySummary_InvalidDaysParam(t *testing.T) {
	orgID := uuid.New()

	mockAgentService := &MockAgentServiceImpl{
		ListAgentsFunc: func(ctx context.Context, org uuid.UUID) ([]*domain.Agent, error) {
			return []*domain.Agent{}, nil
		},
	}

	mockMCPService := &MockMCPServiceImpl{
		ListMCPServersFunc: func(ctx context.Context, org uuid.UUID) ([]*domain.MCPServer, error) {
			return []*domain.MCPServer{}, nil
		},
	}

	handler := NewAnalyticsHandlerWithInterfaces(
		mockAgentService,
		mockMCPService,
		nil, nil, nil, nil, nil,
	)

	app := fiber.New()
	app.Get("/analytics/activity-summary", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		return handler.GetActivitySummary(c)
	})

	// Invalid days parameter should default to 7
	req := httptest.NewRequest("GET", "/analytics/activity-summary?days=invalid", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

// ===========================
// AnalyticsHandler Service Getter Tests
// ===========================

func TestAnalyticsHandler_getAgentService_NilInterfaceReturnsNil(t *testing.T) {
	handler := &AnalyticsHandler{}
	result := handler.getAgentService()
	assert.Nil(t, result)
}

func TestAnalyticsHandler_getMCPService_NilInterfaceReturnsNil(t *testing.T) {
	handler := &AnalyticsHandler{}
	result := handler.getMCPService()
	assert.Nil(t, result)
}

func TestAnalyticsHandler_getVerificationEventService_NilInterfaceReturnsNil(t *testing.T) {
	handler := &AnalyticsHandler{}
	result := handler.getVerificationEventService()
	assert.Nil(t, result)
}

func TestAnalyticsHandler_getAuthService_NilInterfaceReturnsNil(t *testing.T) {
	handler := &AnalyticsHandler{}
	result := handler.getAuthService()
	assert.Nil(t, result)
}

func TestAnalyticsHandler_getAlertService_NilInterfaceReturnsNil(t *testing.T) {
	handler := &AnalyticsHandler{}
	result := handler.getAlertService()
	assert.Nil(t, result)
}

func TestAnalyticsHandler_getSecurityService_NilInterfaceReturnsNil(t *testing.T) {
	handler := &AnalyticsHandler{}
	result := handler.getSecurityService()
	assert.Nil(t, result)
}
