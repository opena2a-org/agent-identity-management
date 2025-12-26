package handlers

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===========================
// SecurityHandler Integration Tests
// ===========================
// These tests use mock implementations via NewSecurityHandlerWithInterfaces

// Helper to create a handler with mocks for integration tests
func createSecurityHandlerWithMocks(
	securityService SecurityServicerExtended,
	auditService AuditServicer,
	alertService AlertServicerExtended,
	agentService AgentServicer,
	capabilityService CapabilityServicer,
) *SecurityHandler {
	return NewSecurityHandlerWithInterfaces(
		securityService,
		auditService,
		alertService,
		agentService,
		capabilityService,
	)
}

// ===========================
// GetThreats Tests
// ===========================

func TestSecurityHandler_GetThreats_Success(t *testing.T) {
	orgID := uuid.New()

	mockSecurityService := &MockSecurityServiceExtendedImpl{
		GetThreatsFunc: func(ctx context.Context, oID uuid.UUID, limit, offset int) ([]*domain.Threat, error) {
			return []*domain.Threat{
				{ID: uuid.New(), ThreatType: domain.ThreatTypeBruteForce, Severity: domain.AlertSeverityHigh},
				{ID: uuid.New(), ThreatType: domain.ThreatTypeSuspiciousActivity, Severity: domain.AlertSeverityWarning},
			}, nil
		},
	}

	handler := createSecurityHandlerWithMocks(
		mockSecurityService,
		nil,
		nil,
		nil,
		nil,
	)

	app := fiber.New()
	app.Get("/security/threats", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		return handler.GetThreats(c)
	})

	req := httptest.NewRequest("GET", "/security/threats", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, float64(2), result["total"])
	threats := result["threats"].([]interface{})
	assert.Len(t, threats, 2)
}

func TestSecurityHandler_GetThreats_WithPagination(t *testing.T) {
	orgID := uuid.New()

	var capturedLimit, capturedOffset int
	mockSecurityService := &MockSecurityServiceExtendedImpl{
		GetThreatsFunc: func(ctx context.Context, oID uuid.UUID, limit, offset int) ([]*domain.Threat, error) {
			capturedLimit = limit
			capturedOffset = offset
			return []*domain.Threat{}, nil
		},
	}

	handler := createSecurityHandlerWithMocks(mockSecurityService, nil, nil, nil, nil)

	app := fiber.New()
	app.Get("/security/threats", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		return handler.GetThreats(c)
	})

	req := httptest.NewRequest("GET", "/security/threats?limit=25&offset=10", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Equal(t, 25, capturedLimit)
	assert.Equal(t, 10, capturedOffset)
}

func TestSecurityHandler_GetThreats_ServiceError(t *testing.T) {
	orgID := uuid.New()

	mockSecurityService := &MockSecurityServiceExtendedImpl{
		GetThreatsFunc: func(ctx context.Context, oID uuid.UUID, limit, offset int) ([]*domain.Threat, error) {
			return nil, assert.AnError
		},
	}

	handler := createSecurityHandlerWithMocks(mockSecurityService, nil, nil, nil, nil)

	app := fiber.New()
	app.Get("/security/threats", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		return handler.GetThreats(c)
	})

	req := httptest.NewRequest("GET", "/security/threats", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

// ===========================
// GetAnomalies Tests
// ===========================

func TestSecurityHandler_GetAnomalies_Success(t *testing.T) {
	orgID := uuid.New()

	mockSecurityService := &MockSecurityServiceExtendedImpl{
		GetAnomaliesFunc: func(ctx context.Context, oID uuid.UUID, limit, offset int) ([]*domain.Anomaly, error) {
			return []*domain.Anomaly{
				{ID: uuid.New(), AnomalyType: domain.AnomalyTypeUnusualAccessPattern, Severity: domain.AlertSeverityWarning},
			}, nil
		},
	}

	handler := createSecurityHandlerWithMocks(mockSecurityService, nil, nil, nil, nil)

	app := fiber.New()
	app.Get("/security/anomalies", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		return handler.GetAnomalies(c)
	})

	req := httptest.NewRequest("GET", "/security/anomalies", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, float64(1), result["total"])
}

func TestSecurityHandler_GetAnomalies_ServiceError(t *testing.T) {
	orgID := uuid.New()

	mockSecurityService := &MockSecurityServiceExtendedImpl{
		GetAnomaliesFunc: func(ctx context.Context, oID uuid.UUID, limit, offset int) ([]*domain.Anomaly, error) {
			return nil, assert.AnError
		},
	}

	handler := createSecurityHandlerWithMocks(mockSecurityService, nil, nil, nil, nil)

	app := fiber.New()
	app.Get("/security/anomalies", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		return handler.GetAnomalies(c)
	})

	req := httptest.NewRequest("GET", "/security/anomalies", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

// ===========================
// GetSecurityMetrics Tests
// ===========================

func TestSecurityHandler_GetSecurityMetrics_Success(t *testing.T) {
	orgID := uuid.New()

	mockSecurityService := &MockSecurityServiceExtendedImpl{
		GetSecurityMetricsFunc: func(ctx context.Context, oID uuid.UUID) (*domain.SecurityMetrics, error) {
			return &domain.SecurityMetrics{
				SecurityScore:   85,
				SecurityGrade:   "B",
				ActionsBlocked:  10,
			}, nil
		},
	}

	handler := createSecurityHandlerWithMocks(mockSecurityService, nil, nil, nil, nil)

	app := fiber.New()
	app.Get("/security/metrics", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		return handler.GetSecurityMetrics(c)
	})

	req := httptest.NewRequest("GET", "/security/metrics", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestSecurityHandler_GetSecurityMetrics_ServiceError(t *testing.T) {
	orgID := uuid.New()

	mockSecurityService := &MockSecurityServiceExtendedImpl{
		GetSecurityMetricsFunc: func(ctx context.Context, oID uuid.UUID) (*domain.SecurityMetrics, error) {
			return nil, assert.AnError
		},
	}

	handler := createSecurityHandlerWithMocks(mockSecurityService, nil, nil, nil, nil)

	app := fiber.New()
	app.Get("/security/metrics", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		return handler.GetSecurityMetrics(c)
	})

	req := httptest.NewRequest("GET", "/security/metrics", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

// ===========================
// GetSecurityDashboard Tests
// ===========================

func TestSecurityHandler_GetSecurityDashboard_Success(t *testing.T) {
	orgID := uuid.New()

	mockSecurityService := &MockSecurityServiceExtendedImpl{
		GetSecurityMetricsFunc: func(ctx context.Context, oID uuid.UUID) (*domain.SecurityMetrics, error) {
			return &domain.SecurityMetrics{TotalThreats: 5}, nil
		},
		GetThreatsFunc: func(ctx context.Context, oID uuid.UUID, limit, offset int) ([]*domain.Threat, error) {
			return []*domain.Threat{{ID: uuid.New()}}, nil
		},
		GetAnomaliesFunc: func(ctx context.Context, oID uuid.UUID, limit, offset int) ([]*domain.Anomaly, error) {
			return []*domain.Anomaly{{ID: uuid.New()}}, nil
		},
	}

	mockAlertService := &MockAlertServiceExtendedImpl{
		CountUnacknowledgedFunc: func(ctx context.Context, oID uuid.UUID) (allCount, acknowledgedCount, unacknowledgedCount int, err error) {
			return 10, 5, 5, nil
		},
		GetAlertsFunc: func(ctx context.Context, oID uuid.UUID, severity, status string, limit, offset int) ([]*domain.Alert, int, error) {
			return []*domain.Alert{{ID: uuid.New()}}, 1, nil
		},
	}

	mockAgentService := &MockAgentServiceImpl{
		ListAgentsFunc: func(ctx context.Context, oID uuid.UUID) ([]*domain.Agent, error) {
			return []*domain.Agent{
				{ID: uuid.New(), Status: domain.AgentStatusVerified, TrustScore: 0.85},
				{ID: uuid.New(), Status: domain.AgentStatusPending, TrustScore: 0.40},
				{ID: uuid.New(), Status: domain.AgentStatusSuspended, TrustScore: 0.60},
			}, nil
		},
	}

	handler := createSecurityHandlerWithMocks(
		mockSecurityService,
		nil,
		mockAlertService,
		mockAgentService,
		nil,
	)

	app := fiber.New()
	app.Get("/security/dashboard", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		return handler.GetSecurityDashboard(c)
	})

	req := httptest.NewRequest("GET", "/security/dashboard", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	// Verify response structure
	assert.Contains(t, result, "metrics")
	assert.Contains(t, result, "threats")
	assert.Contains(t, result, "anomalies")
	assert.Contains(t, result, "alerts")
	assert.Contains(t, result, "agents")

	agents := result["agents"].(map[string]interface{})
	assert.Equal(t, float64(3), agents["total"])
	assert.Equal(t, float64(1), agents["verified"])
	assert.Equal(t, float64(1), agents["suspended"])
	assert.Equal(t, float64(1), agents["pending"])
	assert.Equal(t, float64(1), agents["lowTrust"]) // Agent with 0.40 trust score
}

func TestSecurityHandler_GetSecurityDashboard_MetricsError(t *testing.T) {
	orgID := uuid.New()

	mockSecurityService := &MockSecurityServiceExtendedImpl{
		GetSecurityMetricsFunc: func(ctx context.Context, oID uuid.UUID) (*domain.SecurityMetrics, error) {
			return nil, assert.AnError
		},
	}

	handler := createSecurityHandlerWithMocks(mockSecurityService, nil, nil, nil, nil)

	app := fiber.New()
	app.Get("/security/dashboard", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		return handler.GetSecurityDashboard(c)
	})

	req := httptest.NewRequest("GET", "/security/dashboard", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

func TestSecurityHandler_GetSecurityDashboard_AgentsError(t *testing.T) {
	orgID := uuid.New()

	mockSecurityService := &MockSecurityServiceExtendedImpl{
		GetSecurityMetricsFunc: func(ctx context.Context, oID uuid.UUID) (*domain.SecurityMetrics, error) {
			return &domain.SecurityMetrics{}, nil
		},
		GetThreatsFunc: func(ctx context.Context, oID uuid.UUID, limit, offset int) ([]*domain.Threat, error) {
			return []*domain.Threat{}, nil
		},
		GetAnomaliesFunc: func(ctx context.Context, oID uuid.UUID, limit, offset int) ([]*domain.Anomaly, error) {
			return []*domain.Anomaly{}, nil
		},
	}

	mockAlertService := &MockAlertServiceExtendedImpl{
		CountUnacknowledgedFunc: func(ctx context.Context, oID uuid.UUID) (allCount, acknowledgedCount, unacknowledgedCount int, err error) {
			return 0, 0, 0, nil
		},
		GetAlertsFunc: func(ctx context.Context, oID uuid.UUID, severity, status string, limit, offset int) ([]*domain.Alert, int, error) {
			return []*domain.Alert{}, 0, nil
		},
	}

	mockAgentService := &MockAgentServiceImpl{
		ListAgentsFunc: func(ctx context.Context, oID uuid.UUID) ([]*domain.Agent, error) {
			return nil, assert.AnError
		},
	}

	handler := createSecurityHandlerWithMocks(
		mockSecurityService,
		nil,
		mockAlertService,
		mockAgentService,
		nil,
	)

	app := fiber.New()
	app.Get("/security/dashboard", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		return handler.GetSecurityDashboard(c)
	})

	req := httptest.NewRequest("GET", "/security/dashboard", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

// ===========================
// ListSecurityAlerts Tests
// ===========================

func TestSecurityHandler_ListSecurityAlerts_Success(t *testing.T) {
	orgID := uuid.New()

	mockAlertService := &MockAlertServiceExtendedImpl{
		GetAlertsFunc: func(ctx context.Context, oID uuid.UUID, severity, status string, limit, offset int) ([]*domain.Alert, int, error) {
			return []*domain.Alert{
				{ID: uuid.New(), Severity: domain.AlertSeverityCritical},
				{ID: uuid.New(), Severity: domain.AlertSeverityHigh},
			}, 2, nil
		},
		CountUnacknowledgedFunc: func(ctx context.Context, oID uuid.UUID) (allCount, acknowledgedCount, unacknowledgedCount int, err error) {
			return 10, 5, 5, nil
		},
	}

	handler := createSecurityHandlerWithMocks(nil, nil, mockAlertService, nil, nil)

	app := fiber.New()
	app.Get("/security/alerts", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		return handler.ListSecurityAlerts(c)
	})

	req := httptest.NewRequest("GET", "/security/alerts", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, float64(2), result["total"])
	assert.Equal(t, float64(10), result["allCount"])
	assert.Equal(t, float64(5), result["acknowledgedCount"])
	assert.Equal(t, float64(5), result["unacknowledgedCount"])
}

func TestSecurityHandler_ListSecurityAlerts_ServiceError(t *testing.T) {
	orgID := uuid.New()

	mockAlertService := &MockAlertServiceExtendedImpl{
		GetAlertsFunc: func(ctx context.Context, oID uuid.UUID, severity, status string, limit, offset int) ([]*domain.Alert, int, error) {
			return nil, 0, assert.AnError
		},
	}

	handler := createSecurityHandlerWithMocks(nil, nil, mockAlertService, nil, nil)

	app := fiber.New()
	app.Get("/security/alerts", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		return handler.ListSecurityAlerts(c)
	})

	req := httptest.NewRequest("GET", "/security/alerts", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

func TestSecurityHandler_ListSecurityAlerts_CountError(t *testing.T) {
	orgID := uuid.New()

	mockAlertService := &MockAlertServiceExtendedImpl{
		GetAlertsFunc: func(ctx context.Context, oID uuid.UUID, severity, status string, limit, offset int) ([]*domain.Alert, int, error) {
			return []*domain.Alert{{ID: uuid.New()}}, 5, nil
		},
		CountUnacknowledgedFunc: func(ctx context.Context, oID uuid.UUID) (allCount, acknowledgedCount, unacknowledgedCount int, err error) {
			return 0, 0, 0, assert.AnError
		},
	}

	handler := createSecurityHandlerWithMocks(nil, nil, mockAlertService, nil, nil)

	app := fiber.New()
	app.Get("/security/alerts", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		return handler.ListSecurityAlerts(c)
	})

	req := httptest.NewRequest("GET", "/security/alerts", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should still succeed with defaults
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	// When count fails, allCount should be set to total
	assert.Equal(t, float64(5), result["allCount"])
	assert.Equal(t, float64(0), result["acknowledgedCount"])
	assert.Equal(t, float64(0), result["unacknowledgedCount"])
}

// ===========================
// GetViolations Tests
// ===========================

func TestSecurityHandler_GetViolations_Success(t *testing.T) {
	orgID := uuid.New()

	mockCapabilityService := &MockCapabilityServiceImpl{
		GetViolationsByOrganizationFunc: func(ctx context.Context, oID uuid.UUID, limit, offset int) ([]*domain.CapabilityViolation, int, error) {
			return []*domain.CapabilityViolation{
				{ID: uuid.New(), AttemptedCapability: "file:read"},
				{ID: uuid.New(), AttemptedCapability: "network:connect"},
			}, 10, nil
		},
	}

	handler := createSecurityHandlerWithMocks(nil, nil, nil, nil, mockCapabilityService)

	app := fiber.New()
	app.Get("/security/violations", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		return handler.GetViolations(c)
	})

	req := httptest.NewRequest("GET", "/security/violations", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, float64(10), result["total"])
	violations := result["violations"].([]interface{})
	assert.Len(t, violations, 2)
}

func TestSecurityHandler_GetViolations_ServiceError(t *testing.T) {
	orgID := uuid.New()

	mockCapabilityService := &MockCapabilityServiceImpl{
		GetViolationsByOrganizationFunc: func(ctx context.Context, oID uuid.UUID, limit, offset int) ([]*domain.CapabilityViolation, int, error) {
			return nil, 0, assert.AnError
		},
	}

	handler := createSecurityHandlerWithMocks(nil, nil, nil, nil, mockCapabilityService)

	app := fiber.New()
	app.Get("/security/violations", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		return handler.GetViolations(c)
	})

	req := httptest.NewRequest("GET", "/security/violations", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

// ===========================
// NewSecurityHandlerWithInterfaces Tests
// ===========================

func TestNewSecurityHandlerWithInterfaces_NilDeps(t *testing.T) {
	handler := NewSecurityHandlerWithInterfaces(nil, nil, nil, nil, nil)
	assert.NotNil(t, handler)
}

func TestNewSecurityHandlerWithInterfaces_WithMocks(t *testing.T) {
	mockSec := &MockSecurityServiceExtendedImpl{}
	mockAudit := &MockAuditServiceImpl{}
	mockAlert := &MockAlertServiceExtendedImpl{}
	mockAgent := &MockAgentServiceImpl{}
	mockCap := &MockCapabilityServiceImpl{}

	handler := NewSecurityHandlerWithInterfaces(
		mockSec, mockAudit, mockAlert, mockAgent, mockCap,
	)

	assert.NotNil(t, handler)
	assert.NotNil(t, handler.securityServicer)
	assert.NotNil(t, handler.auditServicer)
	assert.NotNil(t, handler.alertServicer)
	assert.NotNil(t, handler.agentServicer)
	assert.NotNil(t, handler.capabilityServicer)
}
