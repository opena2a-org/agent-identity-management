package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===========================
// TrustScoreHandler Integration Tests with Mocks
// ===========================

// Helper function to create a test app with trust score context
func createTrustScoreTestApp(handler fiber.Handler, orgID, userID uuid.UUID) *fiber.App {
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		return c.Next()
	})
	return app
}

// ===========================
// CalculateTrustScore Tests
// ===========================

func TestTrustScoreHandler_CalculateTrustScore_Success(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()
	calculatedAt := time.Now()

	agentMock := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{
				ID:             agentID,
				OrganizationID: orgID,
				Name:           "test-agent",
			}, nil
		},
	}

	auditMock := &MockAuditServiceImpl{
		LogActionFunc: func(ctx context.Context, orgID uuid.UUID, userID uuid.UUID, action domain.AuditAction, resourceType string, resourceID uuid.UUID, ipAddress string, userAgent string, metadata map[string]interface{}) error {
			return nil
		},
	}

	trustMock := &MockTrustCalculatorServicerImpl{
		CalculateTrustScoreFunc: func(ctx context.Context, id uuid.UUID) (*domain.TrustScore, error) {
			return &domain.TrustScore{
				AgentID: id,
				Score:   0.92,
				Factors: domain.TrustScoreFactors{
					VerificationStatus: 1.0,
					Uptime:             0.95,
					SuccessRate:        0.90,
					SecurityAlerts:     0.85,
					Compliance:         0.80,
					Age:                0.70,
					DriftDetection:     1.0,
					UserFeedback:       0.75,
				},
				Confidence:     0.88,
				LastCalculated: calculatedAt,
			}, nil
		},
	}

	handler := NewTrustScoreHandlerWithInterfaces(trustMock, agentMock, auditMock)
	app := createTrustScoreTestApp(handler.CalculateTrustScore, orgID, userID)
	app.Post("/agents/:id/trust-score/calculate", handler.CalculateTrustScore)

	req := httptest.NewRequest("POST", "/agents/"+agentID.String()+"/trust-score/calculate", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, agentID.String(), result["agentId"])
	assert.Equal(t, 0.92, result["score"])
	assert.NotNil(t, result["factors"])
}

func TestTrustScoreHandler_CalculateTrustScore_AgentNotFound(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()

	agentMock := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return nil, errors.New("agent not found")
		},
	}

	handler := NewTrustScoreHandlerWithInterfaces(&MockTrustCalculatorServicerImpl{}, agentMock, &MockAuditServiceImpl{})
	app := createTrustScoreTestApp(handler.CalculateTrustScore, orgID, userID)
	app.Post("/agents/:id/trust-score/calculate", handler.CalculateTrustScore)

	req := httptest.NewRequest("POST", "/agents/"+agentID.String()+"/trust-score/calculate", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

func TestTrustScoreHandler_CalculateTrustScore_AccessDenied(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()
	differentOrgID := uuid.New()

	agentMock := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{
				ID:             agentID,
				OrganizationID: differentOrgID, // Different organization
				Name:           "test-agent",
			}, nil
		},
	}

	handler := NewTrustScoreHandlerWithInterfaces(&MockTrustCalculatorServicerImpl{}, agentMock, &MockAuditServiceImpl{})
	app := createTrustScoreTestApp(handler.CalculateTrustScore, orgID, userID)
	app.Post("/agents/:id/trust-score/calculate", handler.CalculateTrustScore)

	req := httptest.NewRequest("POST", "/agents/"+agentID.String()+"/trust-score/calculate", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusForbidden, resp.StatusCode)
}

func TestTrustScoreHandler_CalculateTrustScore_CalculationError(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()

	agentMock := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{
				ID:             agentID,
				OrganizationID: orgID,
				Name:           "test-agent",
			}, nil
		},
	}

	trustMock := &MockTrustCalculatorServicerImpl{
		CalculateTrustScoreFunc: func(ctx context.Context, id uuid.UUID) (*domain.TrustScore, error) {
			return nil, errors.New("calculation failed")
		},
	}

	handler := NewTrustScoreHandlerWithInterfaces(trustMock, agentMock, &MockAuditServiceImpl{})
	app := createTrustScoreTestApp(handler.CalculateTrustScore, orgID, userID)
	app.Post("/agents/:id/trust-score/calculate", handler.CalculateTrustScore)

	req := httptest.NewRequest("POST", "/agents/"+agentID.String()+"/trust-score/calculate", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

// ===========================
// GetTrustScore Tests
// ===========================

func TestTrustScoreHandler_GetTrustScore_Success(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()
	calculatedAt := time.Now()

	agentMock := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{
				ID:             agentID,
				OrganizationID: orgID,
				Name:           "test-agent",
			}, nil
		},
	}

	trustMock := &MockTrustCalculatorServicerImpl{
		GetLatestTrustScoreFunc: func(ctx context.Context, id uuid.UUID) (*domain.TrustScore, error) {
			return &domain.TrustScore{
				AgentID: id,
				Score:   0.88,
				Factors: domain.TrustScoreFactors{
					VerificationStatus: 0.9,
					Uptime:             0.85,
					SuccessRate:        0.90,
				},
				Confidence:     0.85,
				LastCalculated: calculatedAt,
			}, nil
		},
	}

	handler := NewTrustScoreHandlerWithInterfaces(trustMock, agentMock, &MockAuditServiceImpl{})
	app := createTrustScoreTestApp(handler.GetTrustScore, orgID, userID)
	app.Get("/agents/:id/trust-score", handler.GetTrustScore)

	req := httptest.NewRequest("GET", "/agents/"+agentID.String()+"/trust-score", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, agentID.String(), result["agentId"])
	assert.Equal(t, "test-agent", result["agentName"])
	assert.Equal(t, 0.88, result["score"])
}

func TestTrustScoreHandler_GetTrustScore_NoStoredScoreCalculatesOnTheFly(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()

	agentMock := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{
				ID:             agentID,
				OrganizationID: orgID,
				Name:           "test-agent",
			}, nil
		},
	}

	trustMock := &MockTrustCalculatorServicerImpl{
		GetLatestTrustScoreFunc: func(ctx context.Context, id uuid.UUID) (*domain.TrustScore, error) {
			return nil, nil // No stored score
		},
		CalculateTrustScoreFunc: func(ctx context.Context, id uuid.UUID) (*domain.TrustScore, error) {
			return &domain.TrustScore{
				AgentID: id,
				Score:   0.75,
				Factors: domain.TrustScoreFactors{
					VerificationStatus: 0.8,
				},
				Confidence:     0.7,
				LastCalculated: time.Now(),
			}, nil
		},
	}

	handler := NewTrustScoreHandlerWithInterfaces(trustMock, agentMock, &MockAuditServiceImpl{})
	app := createTrustScoreTestApp(handler.GetTrustScore, orgID, userID)
	app.Get("/agents/:id/trust-score", handler.GetTrustScore)

	req := httptest.NewRequest("GET", "/agents/"+agentID.String()+"/trust-score", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, 0.75, result["score"])
}

func TestTrustScoreHandler_GetTrustScore_AgentNotFound(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()

	agentMock := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return nil, errors.New("agent not found")
		},
	}

	handler := NewTrustScoreHandlerWithInterfaces(&MockTrustCalculatorServicerImpl{}, agentMock, &MockAuditServiceImpl{})
	app := createTrustScoreTestApp(handler.GetTrustScore, orgID, userID)
	app.Get("/agents/:id/trust-score", handler.GetTrustScore)

	req := httptest.NewRequest("GET", "/agents/"+agentID.String()+"/trust-score", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

func TestTrustScoreHandler_GetTrustScore_AccessDenied(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()
	differentOrgID := uuid.New()

	agentMock := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{
				ID:             agentID,
				OrganizationID: differentOrgID,
				Name:           "test-agent",
			}, nil
		},
	}

	handler := NewTrustScoreHandlerWithInterfaces(&MockTrustCalculatorServicerImpl{}, agentMock, &MockAuditServiceImpl{})
	app := createTrustScoreTestApp(handler.GetTrustScore, orgID, userID)
	app.Get("/agents/:id/trust-score", handler.GetTrustScore)

	req := httptest.NewRequest("GET", "/agents/"+agentID.String()+"/trust-score", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusForbidden, resp.StatusCode)
}

func TestTrustScoreHandler_GetTrustScore_NilScoreAfterCalculation(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()

	agentMock := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{
				ID:             agentID,
				OrganizationID: orgID,
				Name:           "test-agent",
			}, nil
		},
	}

	trustMock := &MockTrustCalculatorServicerImpl{
		GetLatestTrustScoreFunc: func(ctx context.Context, id uuid.UUID) (*domain.TrustScore, error) {
			return nil, nil // No stored score
		},
		CalculateTrustScoreFunc: func(ctx context.Context, id uuid.UUID) (*domain.TrustScore, error) {
			return nil, nil // Also returns nil (edge case)
		},
	}

	handler := NewTrustScoreHandlerWithInterfaces(trustMock, agentMock, &MockAuditServiceImpl{})
	app := createTrustScoreTestApp(handler.GetTrustScore, orgID, userID)
	app.Get("/agents/:id/trust-score", handler.GetTrustScore)

	req := httptest.NewRequest("GET", "/agents/"+agentID.String()+"/trust-score", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

// ===========================
// GetTrustScoreBreakdown Tests
// ===========================

func TestTrustScoreHandler_GetTrustScoreBreakdown_Success(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()

	agentMock := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{
				ID:             agentID,
				OrganizationID: orgID,
				Name:           "test-agent",
			}, nil
		},
	}

	trustMock := &MockTrustCalculatorServicerImpl{
		GetLatestTrustScoreFunc: func(ctx context.Context, id uuid.UUID) (*domain.TrustScore, error) {
			return &domain.TrustScore{
				AgentID: id,
				Score:   0.85,
				Factors: domain.TrustScoreFactors{
					VerificationStatus: 1.0,
					Uptime:             0.95,
					SuccessRate:        0.90,
					SecurityAlerts:     0.85,
					Compliance:         0.80,
					Age:                0.70,
					DriftDetection:     1.0,
					UserFeedback:       0.75,
				},
				Confidence:     0.88,
				LastCalculated: time.Now(),
			}, nil
		},
	}

	handler := NewTrustScoreHandlerWithInterfaces(trustMock, agentMock, &MockAuditServiceImpl{})
	app := createTrustScoreTestApp(handler.GetTrustScoreBreakdown, orgID, userID)
	app.Get("/agents/:id/trust-score/breakdown", handler.GetTrustScoreBreakdown)

	req := httptest.NewRequest("GET", "/agents/"+agentID.String()+"/trust-score/breakdown", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, agentID.String(), result["agentId"])
	assert.Equal(t, "test-agent", result["agentName"])
	assert.Equal(t, 0.85, result["overall"])
	assert.NotNil(t, result["factors"])
	assert.NotNil(t, result["weights"])
	assert.NotNil(t, result["contributions"])
	assert.Equal(t, 0.88, result["confidence"])
}

func TestTrustScoreHandler_GetTrustScoreBreakdown_AgentNotFound(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()

	agentMock := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return nil, errors.New("agent not found")
		},
	}

	handler := NewTrustScoreHandlerWithInterfaces(&MockTrustCalculatorServicerImpl{}, agentMock, &MockAuditServiceImpl{})
	app := createTrustScoreTestApp(handler.GetTrustScoreBreakdown, orgID, userID)
	app.Get("/agents/:id/trust-score/breakdown", handler.GetTrustScoreBreakdown)

	req := httptest.NewRequest("GET", "/agents/"+agentID.String()+"/trust-score/breakdown", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

func TestTrustScoreHandler_GetTrustScoreBreakdown_AccessDenied(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()
	differentOrgID := uuid.New()

	agentMock := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{
				ID:             agentID,
				OrganizationID: differentOrgID,
				Name:           "test-agent",
			}, nil
		},
	}

	handler := NewTrustScoreHandlerWithInterfaces(&MockTrustCalculatorServicerImpl{}, agentMock, &MockAuditServiceImpl{})
	app := createTrustScoreTestApp(handler.GetTrustScoreBreakdown, orgID, userID)
	app.Get("/agents/:id/trust-score/breakdown", handler.GetTrustScoreBreakdown)

	req := httptest.NewRequest("GET", "/agents/"+agentID.String()+"/trust-score/breakdown", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusForbidden, resp.StatusCode)
}

func TestTrustScoreHandler_GetTrustScoreBreakdown_NilScoreAfterCalculation(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()

	agentMock := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{
				ID:             agentID,
				OrganizationID: orgID,
				Name:           "test-agent",
			}, nil
		},
	}

	trustMock := &MockTrustCalculatorServicerImpl{
		GetLatestTrustScoreFunc: func(ctx context.Context, id uuid.UUID) (*domain.TrustScore, error) {
			return nil, nil // No stored score
		},
		CalculateTrustScoreFunc: func(ctx context.Context, id uuid.UUID) (*domain.TrustScore, error) {
			return nil, nil // Also returns nil (edge case)
		},
	}

	handler := NewTrustScoreHandlerWithInterfaces(trustMock, agentMock, &MockAuditServiceImpl{})
	app := createTrustScoreTestApp(handler.GetTrustScoreBreakdown, orgID, userID)
	app.Get("/agents/:id/trust-score/breakdown", handler.GetTrustScoreBreakdown)

	req := httptest.NewRequest("GET", "/agents/"+agentID.String()+"/trust-score/breakdown", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

// ===========================
// GetTrustScoreHistory Tests
// ===========================

func TestTrustScoreHandler_GetTrustScoreHistory_Success(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()

	agentMock := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{
				ID:             agentID,
				OrganizationID: orgID,
				Name:           "test-agent",
			}, nil
		},
	}

	previousScore1 := 0.80
	previousScore2 := 0.75
	trustMock := &MockTrustCalculatorServicerImpl{
		GetTrustScoreHistoryAuditTrailFunc: func(ctx context.Context, id uuid.UUID, limit int) ([]*domain.TrustScoreHistoryEntry, error) {
			return []*domain.TrustScoreHistoryEntry{
				{
					ID:            uuid.New(),
					AgentID:       id,
					PreviousScore: &previousScore1,
					TrustScore:    0.85,
					ChangeReason:  "Improved verification status",
					RecordedAt:    time.Now().Add(-time.Hour),
				},
				{
					ID:            uuid.New(),
					AgentID:       id,
					PreviousScore: &previousScore2,
					TrustScore:    0.80,
					ChangeReason:  "Better uptime",
					RecordedAt:    time.Now().Add(-2 * time.Hour),
				},
			}, nil
		},
	}

	handler := NewTrustScoreHandlerWithInterfaces(trustMock, agentMock, &MockAuditServiceImpl{})
	app := createTrustScoreTestApp(handler.GetTrustScoreHistory, orgID, userID)
	app.Get("/agents/:id/trust-score/history", handler.GetTrustScoreHistory)

	req := httptest.NewRequest("GET", "/agents/"+agentID.String()+"/trust-score/history", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, agentID.String(), result["agentId"])
	assert.Equal(t, "test-agent", result["agentName"])
	assert.Equal(t, float64(2), result["total"])
	assert.NotNil(t, result["history"])
}

func TestTrustScoreHandler_GetTrustScoreHistory_WithLimit(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()
	requestedLimit := 0

	agentMock := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{
				ID:             agentID,
				OrganizationID: orgID,
				Name:           "test-agent",
			}, nil
		},
	}

	trustMock := &MockTrustCalculatorServicerImpl{
		GetTrustScoreHistoryAuditTrailFunc: func(ctx context.Context, id uuid.UUID, limit int) ([]*domain.TrustScoreHistoryEntry, error) {
			requestedLimit = limit
			return []*domain.TrustScoreHistoryEntry{}, nil
		},
	}

	handler := NewTrustScoreHandlerWithInterfaces(trustMock, agentMock, &MockAuditServiceImpl{})
	app := createTrustScoreTestApp(handler.GetTrustScoreHistory, orgID, userID)
	app.Get("/agents/:id/trust-score/history", handler.GetTrustScoreHistory)

	req := httptest.NewRequest("GET", "/agents/"+agentID.String()+"/trust-score/history?limit=10", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Equal(t, 10, requestedLimit)
}

func TestTrustScoreHandler_GetTrustScoreHistory_DefaultLimit(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()
	requestedLimit := 0

	agentMock := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{
				ID:             agentID,
				OrganizationID: orgID,
				Name:           "test-agent",
			}, nil
		},
	}

	trustMock := &MockTrustCalculatorServicerImpl{
		GetTrustScoreHistoryAuditTrailFunc: func(ctx context.Context, id uuid.UUID, limit int) ([]*domain.TrustScoreHistoryEntry, error) {
			requestedLimit = limit
			return []*domain.TrustScoreHistoryEntry{}, nil
		},
	}

	handler := NewTrustScoreHandlerWithInterfaces(trustMock, agentMock, &MockAuditServiceImpl{})
	app := createTrustScoreTestApp(handler.GetTrustScoreHistory, orgID, userID)
	app.Get("/agents/:id/trust-score/history", handler.GetTrustScoreHistory)

	req := httptest.NewRequest("GET", "/agents/"+agentID.String()+"/trust-score/history", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Equal(t, 30, requestedLimit) // Default is 30
}

func TestTrustScoreHandler_GetTrustScoreHistory_AgentNotFound(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()

	agentMock := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return nil, errors.New("agent not found")
		},
	}

	handler := NewTrustScoreHandlerWithInterfaces(&MockTrustCalculatorServicerImpl{}, agentMock, &MockAuditServiceImpl{})
	app := createTrustScoreTestApp(handler.GetTrustScoreHistory, orgID, userID)
	app.Get("/agents/:id/trust-score/history", handler.GetTrustScoreHistory)

	req := httptest.NewRequest("GET", "/agents/"+agentID.String()+"/trust-score/history", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

func TestTrustScoreHandler_GetTrustScoreHistory_AccessDenied(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()
	differentOrgID := uuid.New()

	agentMock := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{
				ID:             agentID,
				OrganizationID: differentOrgID,
				Name:           "test-agent",
			}, nil
		},
	}

	handler := NewTrustScoreHandlerWithInterfaces(&MockTrustCalculatorServicerImpl{}, agentMock, &MockAuditServiceImpl{})
	app := createTrustScoreTestApp(handler.GetTrustScoreHistory, orgID, userID)
	app.Get("/agents/:id/trust-score/history", handler.GetTrustScoreHistory)

	req := httptest.NewRequest("GET", "/agents/"+agentID.String()+"/trust-score/history", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusForbidden, resp.StatusCode)
}

func TestTrustScoreHandler_GetTrustScoreHistory_ServiceError(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()

	agentMock := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{
				ID:             agentID,
				OrganizationID: orgID,
				Name:           "test-agent",
			}, nil
		},
	}

	trustMock := &MockTrustCalculatorServicerImpl{
		GetTrustScoreHistoryAuditTrailFunc: func(ctx context.Context, id uuid.UUID, limit int) ([]*domain.TrustScoreHistoryEntry, error) {
			return nil, errors.New("database error")
		},
	}

	handler := NewTrustScoreHandlerWithInterfaces(trustMock, agentMock, &MockAuditServiceImpl{})
	app := createTrustScoreTestApp(handler.GetTrustScoreHistory, orgID, userID)
	app.Get("/agents/:id/trust-score/history", handler.GetTrustScoreHistory)

	req := httptest.NewRequest("GET", "/agents/"+agentID.String()+"/trust-score/history", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

// ===========================
// NewTrustScoreHandlerWithInterfaces Tests
// ===========================

func TestNewTrustScoreHandlerWithInterfaces_Success(t *testing.T) {
	trustMock := &MockTrustCalculatorServicerImpl{}
	agentMock := &MockAgentServiceImpl{}
	auditMock := &MockAuditServiceImpl{}

	handler := NewTrustScoreHandlerWithInterfaces(trustMock, agentMock, auditMock)

	assert.NotNil(t, handler)
	assert.NotNil(t, handler.trustCalculatorServicer)
	assert.NotNil(t, handler.agentServicer)
	assert.NotNil(t, handler.auditServicer)
}

// ===========================
// Helper Method Tests
// ===========================

func TestTrustScoreHandler_getTrustCalculator_ReturnsInterface(t *testing.T) {
	trustMock := &MockTrustCalculatorServicerImpl{}
	handler := NewTrustScoreHandlerWithInterfaces(trustMock, &MockAgentServiceImpl{}, &MockAuditServiceImpl{})

	result := handler.getTrustCalculator()
	assert.Equal(t, trustMock, result)
}

func TestTrustScoreHandler_getAgentService_ReturnsInterface(t *testing.T) {
	agentMock := &MockAgentServiceImpl{}
	handler := NewTrustScoreHandlerWithInterfaces(&MockTrustCalculatorServicerImpl{}, agentMock, &MockAuditServiceImpl{})

	result := handler.getAgentService()
	assert.Equal(t, agentMock, result)
}

func TestTrustScoreHandler_getAuditService_ReturnsInterface(t *testing.T) {
	auditMock := &MockAuditServiceImpl{}
	handler := NewTrustScoreHandlerWithInterfaces(&MockTrustCalculatorServicerImpl{}, &MockAgentServiceImpl{}, auditMock)

	result := handler.getAuditService()
	assert.Equal(t, auditMock, result)
}

// ===========================
// SubmitUserFeedback Tests
// ===========================

func submitFeedbackRequest(agentID uuid.UUID, jsonBody string) *http.Request {
	req := httptest.NewRequest("POST", "/agents/"+agentID.String()+"/feedback", strings.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func TestTrustScoreHandler_SubmitUserFeedback_Success(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()

	agentMock := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{ID: agentID, OrganizationID: orgID, Name: "test-agent"}, nil
		},
	}

	var captured struct {
		agentID, orgID uuid.UUID
		userID         *uuid.UUID
		rating         int
		comment        string
	}
	trustMock := &MockTrustCalculatorServicerImpl{
		RecordUserFeedbackFunc: func(ctx context.Context, aID, oID uuid.UUID, uID *uuid.UUID, rating int, comment string) (*domain.UserFeedback, error) {
			captured.agentID, captured.orgID, captured.userID, captured.rating, captured.comment = aID, oID, uID, rating, comment
			return &domain.UserFeedback{ID: uuid.New(), AgentID: aID, OrganizationID: oID, UserID: uID, Rating: rating, Comment: comment, CreatedAt: time.Now()}, nil
		},
	}

	handler := NewTrustScoreHandlerWithInterfaces(trustMock, agentMock, &MockAuditServiceImpl{})
	app := createTrustScoreTestApp(handler.SubmitUserFeedback, orgID, userID)
	app.Post("/agents/:id/feedback", handler.SubmitUserFeedback)

	resp, err := app.Test(submitFeedbackRequest(agentID, `{"rating":5,"comment":"great agent"}`))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusCreated, resp.StatusCode)
	// The handler must scope persistence to the caller's org + agent, not trust the body.
	assert.Equal(t, agentID, captured.agentID)
	assert.Equal(t, orgID, captured.orgID)
	require.NotNil(t, captured.userID)
	assert.Equal(t, userID, *captured.userID)
	assert.Equal(t, 5, captured.rating)
	assert.Equal(t, "great agent", captured.comment)
}

func TestTrustScoreHandler_SubmitUserFeedback_InvalidRating(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()

	agentMock := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{ID: agentID, OrganizationID: orgID, Name: "test-agent"}, nil
		},
	}
	// RecordUserFeedback must NOT be called for an out-of-range rating.
	trustMock := &MockTrustCalculatorServicerImpl{
		RecordUserFeedbackFunc: func(ctx context.Context, aID, oID uuid.UUID, uID *uuid.UUID, rating int, comment string) (*domain.UserFeedback, error) {
			t.Fatalf("RecordUserFeedback should not be called for invalid rating %d", rating)
			return nil, nil
		},
	}

	handler := NewTrustScoreHandlerWithInterfaces(trustMock, agentMock, &MockAuditServiceImpl{})
	app := createTrustScoreTestApp(handler.SubmitUserFeedback, orgID, userID)
	app.Post("/agents/:id/feedback", handler.SubmitUserFeedback)

	for _, body := range []string{`{"rating":0}`, `{"rating":6}`, `{"rating":-1}`} {
		resp, err := app.Test(submitFeedbackRequest(agentID, body))
		require.NoError(t, err)
		assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode, "body %s should be rejected", body)
		resp.Body.Close()
	}
}

func TestTrustScoreHandler_SubmitUserFeedback_CrossOrgDenied(t *testing.T) {
	orgID := uuid.New()
	otherOrgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()

	// Agent belongs to a different org than the caller.
	agentMock := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{ID: agentID, OrganizationID: otherOrgID, Name: "other-org-agent"}, nil
		},
	}
	trustMock := &MockTrustCalculatorServicerImpl{
		RecordUserFeedbackFunc: func(ctx context.Context, aID, oID uuid.UUID, uID *uuid.UUID, rating int, comment string) (*domain.UserFeedback, error) {
			t.Fatal("RecordUserFeedback should not be called for a cross-org agent")
			return nil, nil
		},
	}

	handler := NewTrustScoreHandlerWithInterfaces(trustMock, agentMock, &MockAuditServiceImpl{})
	app := createTrustScoreTestApp(handler.SubmitUserFeedback, orgID, userID)
	app.Post("/agents/:id/feedback", handler.SubmitUserFeedback)

	resp, err := app.Test(submitFeedbackRequest(agentID, `{"rating":5}`))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, fiber.StatusForbidden, resp.StatusCode)
}
