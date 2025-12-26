package handlers

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function for trust score tests that need org/user context
func withTrustScoreContext(handler func(c fiber.Ctx) error) fiber.Handler {
	return func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		c.Locals("user_id", uuid.New())
		return handler(c)
	}
}

// ===========================
// NewTrustScoreHandler Tests
// ===========================

func TestNewTrustScoreHandler_NilDeps(t *testing.T) {
	handler := NewTrustScoreHandler(nil, nil, nil)
	assert.NotNil(t, handler)
}

// ===========================
// TrustScoreHandler.CalculateTrustScore Tests
// ===========================

func TestTrustScoreHandler_CalculateTrustScore_InvalidAgentID(t *testing.T) {
	handler := &TrustScoreHandler{}
	app := fiber.New()
	app.Post("/agents/:id/trust-score/calculate", withTrustScoreContext(handler.CalculateTrustScore))

	req := httptest.NewRequest("POST", "/agents/not-a-uuid/trust-score/calculate", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// TrustScoreHandler.GetTrustScore Tests
// ===========================

func TestTrustScoreHandler_GetTrustScore_InvalidAgentID(t *testing.T) {
	handler := &TrustScoreHandler{}
	app := fiber.New()
	app.Get("/agents/:id/trust-score", withTrustScoreContext(handler.GetTrustScore))

	req := httptest.NewRequest("GET", "/agents/not-a-uuid/trust-score", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// TrustScoreHandler.GetTrustScoreBreakdown Tests
// ===========================

func TestTrustScoreHandler_GetTrustScoreBreakdown_InvalidAgentID(t *testing.T) {
	handler := &TrustScoreHandler{}
	app := fiber.New()
	app.Get("/agents/:id/trust-score/breakdown", withTrustScoreContext(handler.GetTrustScoreBreakdown))

	req := httptest.NewRequest("GET", "/agents/not-a-uuid/trust-score/breakdown", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// TrustScoreHandler.GetTrustScoreHistory Tests
// ===========================

func TestTrustScoreHandler_GetTrustScoreHistory_InvalidAgentID(t *testing.T) {
	handler := &TrustScoreHandler{}
	app := fiber.New()
	app.Get("/agents/:id/trust-score/history", withTrustScoreContext(handler.GetTrustScoreHistory))

	req := httptest.NewRequest("GET", "/agents/not-a-uuid/trust-score/history", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// TrustScoreHandler Service Getter Tests
// ===========================

func TestTrustScoreHandler_getTrustCalculator_NilInterfaceReturnsNil(t *testing.T) {
	handler := &TrustScoreHandler{}
	result := handler.getTrustCalculator()
	assert.Nil(t, result)
}

func TestTrustScoreHandler_getAgentService_NilInterfaceReturnsNil(t *testing.T) {
	handler := &TrustScoreHandler{}
	result := handler.getAgentService()
	assert.Nil(t, result)
}

func TestTrustScoreHandler_getAuditService_NilInterfaceReturnsNil(t *testing.T) {
	handler := &TrustScoreHandler{}
	result := handler.getAuditService()
	assert.Nil(t, result)
}
