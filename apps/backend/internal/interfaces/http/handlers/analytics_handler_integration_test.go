package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createAnalyticsTestApp creates a test Fiber app for AnalyticsHandler with mocked services
func createAnalyticsTestApp(t *testing.T, handler *AnalyticsHandler, orgID uuid.UUID) *fiber.App {
	app := fiber.New()

	// Middleware to set test context values
	app.Use(func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", uuid.New())
		return c.Next()
	})

	// Register routes
	app.Get("/usage", handler.GetUsageStatistics)
	app.Get("/trends", handler.GetTrustScoreTrends)
	app.Get("/verification-activity", handler.GetVerificationActivity)
	app.Get("/agents/activity", handler.GetAgentActivity)
	app.Get("/dashboard", handler.GetDashboardStats)
	app.Get("/activity", handler.GetActivitySummary)

	return app
}

// TestGetUsageStatistics tests the GetUsageStatistics handler method
func TestGetUsageStatistics(t *testing.T) {
	orgID := uuid.New()

	t.Run("success with agents", func(t *testing.T) {
		agentService := &MockAgentServiceImpl{
			ListAgentsFunc: func(_ context.Context, org uuid.UUID) ([]*domain.Agent, error) {
				return []*domain.Agent{
					{ID: uuid.New(), OrganizationID: org, Name: "Agent1", Status: "verified", TrustScore: 0.85},
					{ID: uuid.New(), OrganizationID: org, Name: "Agent2", Status: "pending", TrustScore: 0.70},
					{ID: uuid.New(), OrganizationID: org, Name: "Agent3", Status: "verified", TrustScore: 0.95},
				}, nil
			},
		}

		handler := NewAnalyticsHandlerWithInterfaces(
			agentService,
			&MockMCPServiceImpl{},
			&MockVerificationEventServiceExtendedImpl{},
			&MockAuthServiceImpl{},
			&MockAlertServiceExtendedImpl{},
			&MockSecurityServiceForAnalyticsImpl{},
			nil, // no DB for this test
		)

		app := createAnalyticsTestApp(t, handler, orgID)
		req := httptest.NewRequest(http.MethodGet, "/usage", nil)
		resp, err := app.Test(req)

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, _ := io.ReadAll(resp.Body)
		var result map[string]interface{}
		err = json.Unmarshal(body, &result)
		require.NoError(t, err)

		assert.Equal(t, float64(3), result["totalAgents"])
		assert.Equal(t, float64(2), result["activeAgents"])
	})

	t.Run("success with period parameter", func(t *testing.T) {
		agentService := &MockAgentServiceImpl{
			ListAgentsFunc: func(_ context.Context, org uuid.UUID) ([]*domain.Agent, error) {
				return []*domain.Agent{
					{ID: uuid.New(), Status: "verified"},
				}, nil
			},
		}

		handler := NewAnalyticsHandlerWithInterfaces(
			agentService,
			&MockMCPServiceImpl{},
			&MockVerificationEventServiceExtendedImpl{},
			&MockAuthServiceImpl{},
			&MockAlertServiceExtendedImpl{},
			&MockSecurityServiceForAnalyticsImpl{},
			nil,
		)

		app := createAnalyticsTestApp(t, handler, orgID)
		req := httptest.NewRequest(http.MethodGet, "/usage?period=week", nil)
		resp, err := app.Test(req)

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, _ := io.ReadAll(resp.Body)
		var result map[string]interface{}
		json.Unmarshal(body, &result)
		assert.Equal(t, "Last 7 days", result["period"])
	})

	t.Run("success with days parameter", func(t *testing.T) {
		agentService := &MockAgentServiceImpl{
			ListAgentsFunc: func(_ context.Context, org uuid.UUID) ([]*domain.Agent, error) {
				return []*domain.Agent{}, nil
			},
		}

		handler := NewAnalyticsHandlerWithInterfaces(
			agentService,
			&MockMCPServiceImpl{},
			&MockVerificationEventServiceExtendedImpl{},
			&MockAuthServiceImpl{},
			&MockAlertServiceExtendedImpl{},
			&MockSecurityServiceForAnalyticsImpl{},
			nil,
		)

		app := createAnalyticsTestApp(t, handler, orgID)
		req := httptest.NewRequest(http.MethodGet, "/usage?days=14", nil)
		resp, err := app.Test(req)

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, _ := io.ReadAll(resp.Body)
		var result map[string]interface{}
		json.Unmarshal(body, &result)
		assert.Equal(t, "Last 14 days", result["period"])
	})

	t.Run("unauthorized when no org ID", func(t *testing.T) {
		handler := NewAnalyticsHandlerWithInterfaces(
			&MockAgentServiceImpl{},
			&MockMCPServiceImpl{},
			&MockVerificationEventServiceExtendedImpl{},
			&MockAuthServiceImpl{},
			&MockAlertServiceExtendedImpl{},
			&MockSecurityServiceForAnalyticsImpl{},
			nil,
		)

		app := fiber.New()
		app.Get("/usage", handler.GetUsageStatistics)

		req := httptest.NewRequest(http.MethodGet, "/usage", nil)
		resp, err := app.Test(req)

		require.NoError(t, err)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("error when agent service fails", func(t *testing.T) {
		agentService := &MockAgentServiceImpl{
			ListAgentsFunc: func(_ context.Context, org uuid.UUID) ([]*domain.Agent, error) {
				return nil, assert.AnError
			},
		}

		handler := NewAnalyticsHandlerWithInterfaces(
			agentService,
			&MockMCPServiceImpl{},
			&MockVerificationEventServiceExtendedImpl{},
			&MockAuthServiceImpl{},
			&MockAlertServiceExtendedImpl{},
			&MockSecurityServiceForAnalyticsImpl{},
			nil,
		)

		app := createAnalyticsTestApp(t, handler, orgID)
		req := httptest.NewRequest(http.MethodGet, "/usage", nil)
		resp, err := app.Test(req)

		require.NoError(t, err)
		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	})
}

// TestGetTrustScoreTrends tests the GetTrustScoreTrends handler method
func TestGetTrustScoreTrends(t *testing.T) {
	orgID := uuid.New()

	t.Run("success with weekly trends", func(t *testing.T) {
		now := time.Now()
		agentService := &MockAgentServiceImpl{
			ListAgentsFunc: func(_ context.Context, org uuid.UUID) ([]*domain.Agent, error) {
				return []*domain.Agent{
					{ID: uuid.New(), Name: "Agent1", TrustScore: 0.85, CreatedAt: now.AddDate(0, 0, -7)},
					{ID: uuid.New(), Name: "Agent2", TrustScore: 0.90, CreatedAt: now.AddDate(0, 0, -3)},
				}, nil
			},
		}

		handler := NewAnalyticsHandlerWithInterfaces(
			agentService,
			&MockMCPServiceImpl{},
			&MockVerificationEventServiceExtendedImpl{},
			&MockAuthServiceImpl{},
			&MockAlertServiceExtendedImpl{},
			&MockSecurityServiceForAnalyticsImpl{},
			nil, // No DB - will use fallback
		)

		app := createAnalyticsTestApp(t, handler, orgID)
		req := httptest.NewRequest(http.MethodGet, "/trends?period=weeks&weeks=4", nil)
		resp, err := app.Test(req)

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, _ := io.ReadAll(resp.Body)
		var result map[string]interface{}
		json.Unmarshal(body, &result)

		assert.Contains(t, result, "trends")
		assert.Contains(t, result, "period")
	})

	t.Run("success with daily trends", func(t *testing.T) {
		now := time.Now()
		agentService := &MockAgentServiceImpl{
			ListAgentsFunc: func(_ context.Context, org uuid.UUID) ([]*domain.Agent, error) {
				return []*domain.Agent{
					{ID: uuid.New(), Name: "Agent1", TrustScore: 0.80, CreatedAt: now.AddDate(0, 0, -2)},
					{ID: uuid.New(), Name: "Agent2", TrustScore: 0.92, CreatedAt: now.AddDate(0, 0, -1)},
				}, nil
			},
		}

		handler := NewAnalyticsHandlerWithInterfaces(
			agentService,
			&MockMCPServiceImpl{},
			&MockVerificationEventServiceExtendedImpl{},
			&MockAuthServiceImpl{},
			&MockAlertServiceExtendedImpl{},
			&MockSecurityServiceForAnalyticsImpl{},
			nil,
		)

		app := createAnalyticsTestApp(t, handler, orgID)
		req := httptest.NewRequest(http.MethodGet, "/trends?period=days&days=7", nil)
		resp, err := app.Test(req)

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, _ := io.ReadAll(resp.Body)
		var result map[string]interface{}
		json.Unmarshal(body, &result)

		assert.Equal(t, "daily", result["dataType"])
	})

	t.Run("unauthorized when no org ID", func(t *testing.T) {
		handler := NewAnalyticsHandlerWithInterfaces(
			&MockAgentServiceImpl{},
			&MockMCPServiceImpl{},
			&MockVerificationEventServiceExtendedImpl{},
			&MockAuthServiceImpl{},
			&MockAlertServiceExtendedImpl{},
			&MockSecurityServiceForAnalyticsImpl{},
			nil,
		)

		app := fiber.New()
		app.Get("/trends", handler.GetTrustScoreTrends)

		req := httptest.NewRequest(http.MethodGet, "/trends", nil)
		resp, err := app.Test(req)

		require.NoError(t, err)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})
}

// TestGetVerificationActivity tests the GetVerificationActivity handler method
func TestGetVerificationActivity(t *testing.T) {
	orgID := uuid.New()

	t.Run("success with agents and MCP servers", func(t *testing.T) {
		now := time.Now()
		agentService := &MockAgentServiceImpl{
			ListAgentsFunc: func(_ context.Context, org uuid.UUID) ([]*domain.Agent, error) {
				return []*domain.Agent{
					{ID: uuid.New(), Status: "verified", CreatedAt: now.AddDate(0, -1, 0)},
					{ID: uuid.New(), Status: "pending", CreatedAt: now.AddDate(0, 0, -5)},
					{ID: uuid.New(), Status: "verified", CreatedAt: now},
				}, nil
			},
		}

		mcpService := &MockMCPServiceImpl{
			ListMCPServersFunc: func(_ context.Context, org uuid.UUID) ([]*domain.MCPServer, error) {
				return []*domain.MCPServer{
					{ID: uuid.New(), Status: "verified", CreatedAt: now.AddDate(0, -1, 0)},
					{ID: uuid.New(), Status: "pending", CreatedAt: now},
				}, nil
			},
		}

		handler := NewAnalyticsHandlerWithInterfaces(
			agentService,
			mcpService,
			&MockVerificationEventServiceExtendedImpl{},
			&MockAuthServiceImpl{},
			&MockAlertServiceExtendedImpl{},
			&MockSecurityServiceForAnalyticsImpl{},
			nil,
		)

		app := createAnalyticsTestApp(t, handler, orgID)
		req := httptest.NewRequest(http.MethodGet, "/verification-activity?months=6", nil)
		resp, err := app.Test(req)

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, _ := io.ReadAll(resp.Body)
		var result map[string]interface{}
		json.Unmarshal(body, &result)

		assert.Contains(t, result, "activity")
		assert.Contains(t, result, "currentStats")
		currentStats := result["currentStats"].(map[string]interface{})
		assert.Equal(t, float64(3), currentStats["totalAgents"])
		assert.Equal(t, float64(2), currentStats["totalMCP"])
	})

	t.Run("agent service error returns 500", func(t *testing.T) {
		agentService := &MockAgentServiceImpl{
			ListAgentsFunc: func(_ context.Context, org uuid.UUID) ([]*domain.Agent, error) {
				return nil, assert.AnError
			},
		}

		handler := NewAnalyticsHandlerWithInterfaces(
			agentService,
			&MockMCPServiceImpl{},
			&MockVerificationEventServiceExtendedImpl{},
			&MockAuthServiceImpl{},
			&MockAlertServiceExtendedImpl{},
			&MockSecurityServiceForAnalyticsImpl{},
			nil,
		)

		app := createAnalyticsTestApp(t, handler, orgID)
		req := httptest.NewRequest(http.MethodGet, "/verification-activity", nil)
		resp, err := app.Test(req)

		require.NoError(t, err)
		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	})

	t.Run("continues when MCP service fails", func(t *testing.T) {
		agentService := &MockAgentServiceImpl{
			ListAgentsFunc: func(_ context.Context, org uuid.UUID) ([]*domain.Agent, error) {
				return []*domain.Agent{{ID: uuid.New(), Status: "verified"}}, nil
			},
		}

		mcpService := &MockMCPServiceImpl{
			ListMCPServersFunc: func(_ context.Context, org uuid.UUID) ([]*domain.MCPServer, error) {
				return nil, assert.AnError
			},
		}

		handler := NewAnalyticsHandlerWithInterfaces(
			agentService,
			mcpService,
			&MockVerificationEventServiceExtendedImpl{},
			&MockAuthServiceImpl{},
			&MockAlertServiceExtendedImpl{},
			&MockSecurityServiceForAnalyticsImpl{},
			nil,
		)

		app := createAnalyticsTestApp(t, handler, orgID)
		req := httptest.NewRequest(http.MethodGet, "/verification-activity", nil)
		resp, err := app.Test(req)

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

// TestGetAgentActivity tests the GetAgentActivity handler method
func TestGetAgentActivity(t *testing.T) {
	orgID := uuid.New()

	t.Run("returns empty activities when no DB", func(t *testing.T) {
		handler := NewAnalyticsHandlerWithInterfaces(
			&MockAgentServiceImpl{},
			&MockMCPServiceImpl{},
			&MockVerificationEventServiceExtendedImpl{},
			&MockAuthServiceImpl{},
			&MockAlertServiceExtendedImpl{},
			&MockSecurityServiceForAnalyticsImpl{},
			nil, // No DB connection
		)

		app := createAnalyticsTestApp(t, handler, orgID)
		req := httptest.NewRequest(http.MethodGet, "/agents/activity?limit=10&offset=0", nil)
		resp, err := app.Test(req)

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, _ := io.ReadAll(resp.Body)
		var result map[string]interface{}
		json.Unmarshal(body, &result)

		activities := result["activities"].([]interface{})
		assert.Equal(t, 0, len(activities))
		assert.Contains(t, result, "summary")
	})

	t.Run("unauthorized when no org ID", func(t *testing.T) {
		handler := NewAnalyticsHandlerWithInterfaces(
			&MockAgentServiceImpl{},
			&MockMCPServiceImpl{},
			&MockVerificationEventServiceExtendedImpl{},
			&MockAuthServiceImpl{},
			&MockAlertServiceExtendedImpl{},
			&MockSecurityServiceForAnalyticsImpl{},
			nil,
		)

		app := fiber.New()
		app.Get("/agents/activity", handler.GetAgentActivity)

		req := httptest.NewRequest(http.MethodGet, "/agents/activity", nil)
		resp, err := app.Test(req)

		require.NoError(t, err)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})
}

// TestGetDashboardStats tests the GetDashboardStats handler method
func TestGetDashboardStats(t *testing.T) {
	orgID := uuid.New()

	t.Run("success with all metrics", func(t *testing.T) {
		agentService := &MockAgentServiceImpl{
			ListAgentsFunc: func(_ context.Context, org uuid.UUID) ([]*domain.Agent, error) {
				return []*domain.Agent{
					{ID: uuid.New(), Status: "verified", TrustScore: 0.85},
					{ID: uuid.New(), Status: "verified", TrustScore: 0.90},
					{ID: uuid.New(), Status: "pending", TrustScore: 0.75},
				}, nil
			},
		}

		mcpService := &MockMCPServiceImpl{
			ListMCPServersFunc: func(_ context.Context, org uuid.UUID) ([]*domain.MCPServer, error) {
				return []*domain.MCPServer{
					{ID: uuid.New(), Status: "verified"},
					{ID: uuid.New(), Status: "pending"},
				}, nil
			},
		}

		verificationService := &MockVerificationEventServiceExtendedImpl{
			GetLast24HoursStatisticsFunc: func(_ context.Context, org uuid.UUID) (*domain.VerificationStatistics, error) {
				return &domain.VerificationStatistics{
					TotalVerifications: 100,
					SuccessCount:       95,
					FailedCount:        5,
					AvgDurationMs:      120,
				}, nil
			},
		}

		authService := &MockAuthServiceImpl{
			GetUsersByOrganizationFunc: func(_ context.Context, org uuid.UUID) ([]*domain.User, error) {
				return []*domain.User{
					{ID: uuid.New(), Status: domain.UserStatusActive},
					{ID: uuid.New(), Status: domain.UserStatusActive},
					{ID: uuid.New(), Status: domain.UserStatusDeactivated},
				}, nil
			},
		}

		alertService := &MockAlertServiceExtendedImpl{
			GetAlertsFunc: func(_ context.Context, org uuid.UUID, severity, status string, limit, offset int) ([]*domain.Alert, int, error) {
				return []*domain.Alert{
					{ID: uuid.New(), Severity: domain.AlertSeverityCritical},
					{ID: uuid.New(), Severity: domain.AlertSeverityHigh},
				}, 2, nil
			},
		}

		securityService := &MockSecurityServiceForAnalyticsImpl{
			GetIncidentsFunc: func(_ context.Context, org uuid.UUID, status domain.IncidentStatus, limit, offset int) ([]*domain.SecurityIncident, error) {
				return []*domain.SecurityIncident{
					{ID: uuid.New()},
				}, nil
			},
		}

		handler := NewAnalyticsHandlerWithInterfaces(
			agentService,
			mcpService,
			verificationService,
			authService,
			alertService,
			securityService,
			nil,
		)

		app := createAnalyticsTestApp(t, handler, orgID)
		req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
		resp, err := app.Test(req)

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, _ := io.ReadAll(resp.Body)
		var result map[string]interface{}
		json.Unmarshal(body, &result)

		assert.Equal(t, float64(3), result["totalAgents"])
		assert.Equal(t, float64(2), result["verifiedAgents"])
		assert.Equal(t, float64(1), result["pendingAgents"])
		assert.Equal(t, float64(2), result["totalMcpServers"])
		assert.Equal(t, float64(1), result["activeMcpServers"])
		assert.Equal(t, float64(3), result["totalUsers"])
		assert.Equal(t, float64(2), result["activeUsers"])
		assert.Equal(t, float64(2), result["activeAlerts"])
		assert.Equal(t, float64(1), result["criticalAlerts"])
		assert.Equal(t, float64(1), result["securityIncidents"])
		assert.Equal(t, float64(100), result["totalVerifications"])
		assert.Equal(t, float64(95), result["successfulVerifications"])
	})

	t.Run("handles service failures gracefully", func(t *testing.T) {
		agentService := &MockAgentServiceImpl{
			ListAgentsFunc: func(_ context.Context, org uuid.UUID) ([]*domain.Agent, error) {
				return []*domain.Agent{{ID: uuid.New(), Status: "verified", TrustScore: 0.9}}, nil
			},
		}

		mcpService := &MockMCPServiceImpl{
			ListMCPServersFunc: func(_ context.Context, org uuid.UUID) ([]*domain.MCPServer, error) {
				return []*domain.MCPServer{{ID: uuid.New(), Status: "verified"}}, nil
			},
		}

		verificationService := &MockVerificationEventServiceExtendedImpl{
			GetLast24HoursStatisticsFunc: func(_ context.Context, org uuid.UUID) (*domain.VerificationStatistics, error) {
				return nil, assert.AnError // Verification stats fail
			},
		}

		authService := &MockAuthServiceImpl{
			GetUsersByOrganizationFunc: func(_ context.Context, org uuid.UUID) ([]*domain.User, error) {
				return nil, assert.AnError // Auth fails
			},
		}

		alertService := &MockAlertServiceExtendedImpl{
			GetAlertsFunc: func(_ context.Context, org uuid.UUID, severity, status string, limit, offset int) ([]*domain.Alert, int, error) {
				return nil, 0, assert.AnError // Alerts fail
			},
		}

		securityService := &MockSecurityServiceForAnalyticsImpl{
			GetIncidentsFunc: func(_ context.Context, org uuid.UUID, status domain.IncidentStatus, limit, offset int) ([]*domain.SecurityIncident, error) {
				return nil, assert.AnError // Security fails
			},
		}

		handler := NewAnalyticsHandlerWithInterfaces(
			agentService,
			mcpService,
			verificationService,
			authService,
			alertService,
			securityService,
			nil,
		)

		app := createAnalyticsTestApp(t, handler, orgID)
		req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
		resp, err := app.Test(req)

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, _ := io.ReadAll(resp.Body)
		var result map[string]interface{}
		json.Unmarshal(body, &result)

		// Should still have agent and MCP metrics
		assert.Equal(t, float64(1), result["totalAgents"])
		assert.Equal(t, float64(1), result["totalMcpServers"])
		// Should default to 0 for failed services
		assert.Equal(t, float64(0), result["totalUsers"])
		assert.Equal(t, float64(0), result["totalVerifications"])
	})

	t.Run("agent service error returns 500", func(t *testing.T) {
		agentService := &MockAgentServiceImpl{
			ListAgentsFunc: func(_ context.Context, org uuid.UUID) ([]*domain.Agent, error) {
				return nil, assert.AnError
			},
		}

		handler := NewAnalyticsHandlerWithInterfaces(
			agentService,
			&MockMCPServiceImpl{},
			&MockVerificationEventServiceExtendedImpl{},
			&MockAuthServiceImpl{},
			&MockAlertServiceExtendedImpl{},
			&MockSecurityServiceForAnalyticsImpl{},
			nil,
		)

		app := createAnalyticsTestApp(t, handler, orgID)
		req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
		resp, err := app.Test(req)

		require.NoError(t, err)
		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	})

	t.Run("MCP service error returns 500", func(t *testing.T) {
		agentService := &MockAgentServiceImpl{
			ListAgentsFunc: func(_ context.Context, org uuid.UUID) ([]*domain.Agent, error) {
				return []*domain.Agent{}, nil
			},
		}

		mcpService := &MockMCPServiceImpl{
			ListMCPServersFunc: func(_ context.Context, org uuid.UUID) ([]*domain.MCPServer, error) {
				return nil, assert.AnError
			},
		}

		handler := NewAnalyticsHandlerWithInterfaces(
			agentService,
			mcpService,
			&MockVerificationEventServiceExtendedImpl{},
			&MockAuthServiceImpl{},
			&MockAlertServiceExtendedImpl{},
			&MockSecurityServiceForAnalyticsImpl{},
			nil,
		)

		app := createAnalyticsTestApp(t, handler, orgID)
		req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
		resp, err := app.Test(req)

		require.NoError(t, err)
		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	})
}

// TestGetActivitySummary tests the GetActivitySummary handler method
func TestGetActivitySummary(t *testing.T) {
	orgID := uuid.New()

	t.Run("success with default days", func(t *testing.T) {
		agentService := &MockAgentServiceImpl{
			ListAgentsFunc: func(_ context.Context, org uuid.UUID) ([]*domain.Agent, error) {
				return []*domain.Agent{
					{ID: uuid.New(), Name: "Agent1"},
					{ID: uuid.New(), Name: "Agent2"},
				}, nil
			},
		}

		mcpService := &MockMCPServiceImpl{
			ListMCPServersFunc: func(_ context.Context, org uuid.UUID) ([]*domain.MCPServer, error) {
				return []*domain.MCPServer{
					{ID: uuid.New(), Name: "MCP1"},
				}, nil
			},
		}

		handler := NewAnalyticsHandlerWithInterfaces(
			agentService,
			mcpService,
			&MockVerificationEventServiceExtendedImpl{},
			&MockAuthServiceImpl{},
			&MockAlertServiceExtendedImpl{},
			&MockSecurityServiceForAnalyticsImpl{},
			nil, // No DB - will use defaults
		)

		app := createAnalyticsTestApp(t, handler, orgID)
		req := httptest.NewRequest(http.MethodGet, "/activity", nil)
		resp, err := app.Test(req)

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, _ := io.ReadAll(resp.Body)
		var result map[string]interface{}
		json.Unmarshal(body, &result)

		assert.Contains(t, result, "period")
		assert.Contains(t, result, "summary")

		summary := result["summary"].(map[string]interface{})
		assert.Equal(t, float64(2), summary["totalAgents"])
		assert.Equal(t, float64(1), summary["totalMcpServers"])
	})

	t.Run("success with custom days", func(t *testing.T) {
		handler := NewAnalyticsHandlerWithInterfaces(
			&MockAgentServiceImpl{},
			&MockMCPServiceImpl{},
			&MockVerificationEventServiceExtendedImpl{},
			&MockAuthServiceImpl{},
			&MockAlertServiceExtendedImpl{},
			&MockSecurityServiceForAnalyticsImpl{},
			nil,
		)

		app := createAnalyticsTestApp(t, handler, orgID)
		req := httptest.NewRequest(http.MethodGet, "/activity?days=14", nil)
		resp, err := app.Test(req)

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, _ := io.ReadAll(resp.Body)
		var result map[string]interface{}
		json.Unmarshal(body, &result)

		period := result["period"].(map[string]interface{})
		assert.Equal(t, float64(14), period["days"])
	})

	t.Run("unauthorized when no org ID", func(t *testing.T) {
		handler := NewAnalyticsHandlerWithInterfaces(
			&MockAgentServiceImpl{},
			&MockMCPServiceImpl{},
			&MockVerificationEventServiceExtendedImpl{},
			&MockAuthServiceImpl{},
			&MockAlertServiceExtendedImpl{},
			&MockSecurityServiceForAnalyticsImpl{},
			nil,
		)

		app := fiber.New()
		app.Get("/activity", handler.GetActivitySummary)

		req := httptest.NewRequest(http.MethodGet, "/activity", nil)
		resp, err := app.Test(req)

		require.NoError(t, err)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})
}

// TestAnalyticsHandlerHelperMethods tests the helper methods for service selection
func TestAnalyticsHandlerHelperMethods(t *testing.T) {
	t.Run("getAgentService returns interface when set", func(t *testing.T) {
		mockService := &MockAgentServiceImpl{}
		handler := NewAnalyticsHandlerWithInterfaces(
			mockService,
			&MockMCPServiceImpl{},
			&MockVerificationEventServiceExtendedImpl{},
			&MockAuthServiceImpl{},
			&MockAlertServiceExtendedImpl{},
			&MockSecurityServiceForAnalyticsImpl{},
			nil,
		)
		assert.NotNil(t, handler.getAgentService())
	})

	t.Run("getMCPService returns interface when set", func(t *testing.T) {
		mockService := &MockMCPServiceImpl{}
		handler := NewAnalyticsHandlerWithInterfaces(
			&MockAgentServiceImpl{},
			mockService,
			&MockVerificationEventServiceExtendedImpl{},
			&MockAuthServiceImpl{},
			&MockAlertServiceExtendedImpl{},
			&MockSecurityServiceForAnalyticsImpl{},
			nil,
		)
		assert.NotNil(t, handler.getMCPService())
	})

	t.Run("getVerificationEventService returns interface when set", func(t *testing.T) {
		mockService := &MockVerificationEventServiceExtendedImpl{}
		handler := NewAnalyticsHandlerWithInterfaces(
			&MockAgentServiceImpl{},
			&MockMCPServiceImpl{},
			mockService,
			&MockAuthServiceImpl{},
			&MockAlertServiceExtendedImpl{},
			&MockSecurityServiceForAnalyticsImpl{},
			nil,
		)
		assert.NotNil(t, handler.getVerificationEventService())
	})

	t.Run("getAuthService returns interface when set", func(t *testing.T) {
		mockService := &MockAuthServiceImpl{}
		handler := NewAnalyticsHandlerWithInterfaces(
			&MockAgentServiceImpl{},
			&MockMCPServiceImpl{},
			&MockVerificationEventServiceExtendedImpl{},
			mockService,
			&MockAlertServiceExtendedImpl{},
			&MockSecurityServiceForAnalyticsImpl{},
			nil,
		)
		assert.NotNil(t, handler.getAuthService())
	})

	t.Run("getAlertService returns interface when set", func(t *testing.T) {
		mockService := &MockAlertServiceExtendedImpl{}
		handler := NewAnalyticsHandlerWithInterfaces(
			&MockAgentServiceImpl{},
			&MockMCPServiceImpl{},
			&MockVerificationEventServiceExtendedImpl{},
			&MockAuthServiceImpl{},
			mockService,
			&MockSecurityServiceForAnalyticsImpl{},
			nil,
		)
		assert.NotNil(t, handler.getAlertService())
	})

	t.Run("getSecurityService returns interface when set", func(t *testing.T) {
		mockService := &MockSecurityServiceForAnalyticsImpl{}
		handler := NewAnalyticsHandlerWithInterfaces(
			&MockAgentServiceImpl{},
			&MockMCPServiceImpl{},
			&MockVerificationEventServiceExtendedImpl{},
			&MockAuthServiceImpl{},
			&MockAlertServiceExtendedImpl{},
			mockService,
			nil,
		)
		assert.NotNil(t, handler.getSecurityService())
	})
}
