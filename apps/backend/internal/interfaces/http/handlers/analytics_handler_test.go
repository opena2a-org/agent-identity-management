package handlers

import (
	"net/http/httptest"
	"testing"

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
