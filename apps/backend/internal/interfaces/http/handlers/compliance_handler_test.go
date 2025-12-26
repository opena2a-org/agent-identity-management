package handlers

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function for compliance tests that need org/user context
func withComplianceContext(handler func(c fiber.Ctx) error) fiber.Handler {
	return func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		c.Locals("user_id", uuid.New())
		return handler(c)
	}
}

// ===========================
// NewComplianceHandler Tests
// ===========================

func TestNewComplianceHandler_NilDeps(t *testing.T) {
	handler := NewComplianceHandler(nil, nil)
	assert.NotNil(t, handler)
}

// ===========================
// ComplianceHandler.RunComplianceCheck Tests
// ===========================

func TestComplianceHandler_RunComplianceCheck_InvalidJSON(t *testing.T) {
	handler := &ComplianceHandler{}
	app := fiber.New()
	app.Post("/compliance/check", withComplianceContext(handler.RunComplianceCheck))

	req := httptest.NewRequest("POST", "/compliance/check", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// ComplianceHandler.ExportComplianceReport Tests
// ===========================

func TestComplianceHandler_ExportComplianceReport_InvalidFormat(t *testing.T) {
	handler := &ComplianceHandler{}
	app := fiber.New()
	app.Get("/compliance/export", withComplianceContext(handler.ExportComplianceReport))

	req := httptest.NewRequest("GET", "/compliance/export?format=xml", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestComplianceHandler_ExportComplianceReport_InvalidFramework(t *testing.T) {
	handler := &ComplianceHandler{}
	app := fiber.New()
	app.Get("/compliance/export", withComplianceContext(handler.ExportComplianceReport))

	req := httptest.NewRequest("GET", "/compliance/export?framework=soc2", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// ComplianceHandler.CollectEvidence Tests
// ===========================

func TestComplianceHandler_CollectEvidence_InvalidJSON(t *testing.T) {
	handler := &ComplianceHandler{}
	app := fiber.New()
	app.Post("/compliance/evidence", withComplianceContext(handler.CollectEvidence))

	req := httptest.NewRequest("POST", "/compliance/evidence", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestComplianceHandler_CollectEvidence_MissingFields(t *testing.T) {
	handler := &ComplianceHandler{}
	app := fiber.New()
	app.Post("/compliance/evidence", withComplianceContext(handler.CollectEvidence))

	// Missing both framework and checkName
	req := httptest.NewRequest("POST", "/compliance/evidence", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestComplianceHandler_CollectEvidence_MissingFramework(t *testing.T) {
	handler := &ComplianceHandler{}
	app := fiber.New()
	app.Post("/compliance/evidence", withComplianceContext(handler.CollectEvidence))

	req := httptest.NewRequest("POST", "/compliance/evidence", strings.NewReader(`{"checkName":"test-check"}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestComplianceHandler_CollectEvidence_MissingCheckName(t *testing.T) {
	handler := &ComplianceHandler{}
	app := fiber.New()
	app.Post("/compliance/evidence", withComplianceContext(handler.CollectEvidence))

	req := httptest.NewRequest("POST", "/compliance/evidence", strings.NewReader(`{"framework":"aim"}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestComplianceHandler_CollectEvidence_InvalidFramework(t *testing.T) {
	handler := &ComplianceHandler{}
	app := fiber.New()
	app.Post("/compliance/evidence", withComplianceContext(handler.CollectEvidence))

	req := httptest.NewRequest("POST", "/compliance/evidence", strings.NewReader(`{"framework":"soc2","checkName":"test-check"}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// ComplianceHandler.GetComplianceTrending Tests
// ===========================

func TestComplianceHandler_GetComplianceTrending_InvalidFramework(t *testing.T) {
	handler := &ComplianceHandler{}
	app := fiber.New()
	app.Get("/compliance/trending", withComplianceContext(handler.GetComplianceTrending))

	req := httptest.NewRequest("GET", "/compliance/trending?framework=hipaa", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// ComplianceHandler.RecordComplianceSnapshot Tests
// ===========================

func TestComplianceHandler_RecordComplianceSnapshot_InvalidJSON(t *testing.T) {
	handler := &ComplianceHandler{}
	app := fiber.New()
	app.Post("/compliance/snapshot", withComplianceContext(handler.RecordComplianceSnapshot))

	req := httptest.NewRequest("POST", "/compliance/snapshot", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestComplianceHandler_RecordComplianceSnapshot_InvalidFramework(t *testing.T) {
	handler := &ComplianceHandler{}
	app := fiber.New()
	app.Post("/compliance/snapshot", withComplianceContext(handler.RecordComplianceSnapshot))

	req := httptest.NewRequest("POST", "/compliance/snapshot", strings.NewReader(`{"framework":"gdpr"}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// ComplianceHandler.GetEvidenceForCheck Tests
// ===========================

func TestComplianceHandler_GetEvidenceForCheck_MissingCheckName(t *testing.T) {
	handler := &ComplianceHandler{}
	app := fiber.New()
	// Route without checkName param should still work - empty string check
	app.Get("/compliance/evidence/check/:checkName", withComplianceContext(handler.GetEvidenceForCheck))

	// This tests the empty check name case in handler logic
	// Because Fiber's ":checkName" will match anything, we need a different approach
	// Let's test with an empty param by using optional param
	app2 := fiber.New()
	app2.Get("/compliance/evidence/check", withComplianceContext(func(c fiber.Ctx) error {
		// Manually set empty param
		return handler.GetEvidenceForCheck(c)
	}))

	req := httptest.NewRequest("GET", "/compliance/evidence/check", nil)

	resp, err := app2.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// The handler checks for empty checkName and returns BadRequest
	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}
