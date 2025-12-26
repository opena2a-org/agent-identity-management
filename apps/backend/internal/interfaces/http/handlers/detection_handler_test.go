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

// Helper function for detection tests that need org context
func withDetectionContext(handler func(c fiber.Ctx) error) fiber.Handler {
	return func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		c.Locals("user_id", uuid.New())
		return handler(c)
	}
}

// Helper function for detection tests with API key auth
func withDetectionAPIKeyContext(handler func(c fiber.Ctx) error) fiber.Handler {
	return func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		c.Locals("auth_method", "api_key")
		return handler(c)
	}
}

// ===========================
// NewDetectionHandler Tests
// ===========================

func TestNewDetectionHandler_NilDeps(t *testing.T) {
	handler := NewDetectionHandler(nil, nil)
	assert.NotNil(t, handler)
}

// ===========================
// DetectionHandler.ReportDetection Tests
// ===========================

func TestDetectionHandler_ReportDetection_InvalidAgentID(t *testing.T) {
	handler := &DetectionHandler{}
	app := fiber.New()
	app.Post("/agents/:id/detection/report", withDetectionContext(handler.ReportDetection))

	body := `{"detections":[{"name":"test"}]}`
	req := httptest.NewRequest("POST", "/agents/not-a-uuid/detection/report", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestDetectionHandler_ReportDetection_NoOrgContext(t *testing.T) {
	handler := &DetectionHandler{}
	app := fiber.New()
	app.Post("/agents/:id/detection/report", handler.ReportDetection)

	body := `{"detections":[{"name":"test"}]}`
	req := httptest.NewRequest("POST", "/agents/"+uuid.New().String()+"/detection/report", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestDetectionHandler_ReportDetection_NoUserContextWithJWT(t *testing.T) {
	handler := &DetectionHandler{}
	app := fiber.New()
	// Set org but no user (simulating JWT auth without user)
	app.Post("/agents/:id/detection/report", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		// No user_id set and no auth_method set (defaults to JWT path)
		return handler.ReportDetection(c)
	})

	body := `{"detections":[{"name":"test"}]}`
	req := httptest.NewRequest("POST", "/agents/"+uuid.New().String()+"/detection/report", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestDetectionHandler_ReportDetection_InvalidJSON(t *testing.T) {
	handler := &DetectionHandler{}
	app := fiber.New()
	app.Post("/agents/:id/detection/report", withDetectionContext(handler.ReportDetection))

	req := httptest.NewRequest("POST", "/agents/"+uuid.New().String()+"/detection/report", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestDetectionHandler_ReportDetection_EmptyDetections(t *testing.T) {
	handler := &DetectionHandler{}
	app := fiber.New()
	app.Post("/agents/:id/detection/report", withDetectionContext(handler.ReportDetection))

	body := `{"detections":[]}`
	req := httptest.NewRequest("POST", "/agents/"+uuid.New().String()+"/detection/report", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestDetectionHandler_ReportDetection_APIKeyAuthWorks(t *testing.T) {
	handler := &DetectionHandler{}
	app := fiber.New()
	app.Post("/agents/:id/detection/report", withDetectionAPIKeyContext(handler.ReportDetection))

	// Empty detections should fail validation, not auth
	body := `{"detections":[]}`
	req := httptest.NewRequest("POST", "/agents/"+uuid.New().String()+"/detection/report", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should fail with 400 (validation) not 401 (auth), proving API key auth works
	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// DetectionHandler.GetDetectionStatus Tests
// ===========================

func TestDetectionHandler_GetDetectionStatus_InvalidAgentID(t *testing.T) {
	handler := &DetectionHandler{}
	app := fiber.New()
	app.Get("/agents/:id/detection/status", withDetectionContext(handler.GetDetectionStatus))

	req := httptest.NewRequest("GET", "/agents/not-a-uuid/detection/status", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestDetectionHandler_GetDetectionStatus_NoOrgContext(t *testing.T) {
	handler := &DetectionHandler{}
	app := fiber.New()
	app.Get("/agents/:id/detection/status", handler.GetDetectionStatus)

	req := httptest.NewRequest("GET", "/agents/"+uuid.New().String()+"/detection/status", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

// ===========================
// DetectionHandler.ReportCapabilities Tests
// ===========================

func TestDetectionHandler_ReportCapabilities_InvalidAgentID(t *testing.T) {
	handler := &DetectionHandler{}
	app := fiber.New()
	app.Post("/agents/:id/capabilities/report", withDetectionContext(handler.ReportCapabilities))

	body := `{"detectedAt":"2024-01-01T00:00:00Z"}`
	req := httptest.NewRequest("POST", "/agents/not-a-uuid/capabilities/report", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestDetectionHandler_ReportCapabilities_NoOrgContext(t *testing.T) {
	handler := &DetectionHandler{}
	app := fiber.New()
	app.Post("/agents/:id/capabilities/report", handler.ReportCapabilities)

	body := `{"detectedAt":"2024-01-01T00:00:00Z"}`
	req := httptest.NewRequest("POST", "/agents/"+uuid.New().String()+"/capabilities/report", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestDetectionHandler_ReportCapabilities_InvalidJSON(t *testing.T) {
	handler := &DetectionHandler{}
	app := fiber.New()
	app.Post("/agents/:id/capabilities/report", withDetectionContext(handler.ReportCapabilities))

	req := httptest.NewRequest("POST", "/agents/"+uuid.New().String()+"/capabilities/report", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestDetectionHandler_ReportCapabilities_MissingDetectedAt(t *testing.T) {
	handler := &DetectionHandler{}
	app := fiber.New()
	app.Post("/agents/:id/capabilities/report", withDetectionContext(handler.ReportCapabilities))

	body := `{}`
	req := httptest.NewRequest("POST", "/agents/"+uuid.New().String()+"/capabilities/report", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// DetectionHandler.GetLatestCapabilityReport Tests
// ===========================

func TestDetectionHandler_GetLatestCapabilityReport_InvalidAgentID(t *testing.T) {
	handler := &DetectionHandler{}
	app := fiber.New()
	app.Get("/detection/agents/:id/capabilities/latest", withDetectionContext(handler.GetLatestCapabilityReport))

	req := httptest.NewRequest("GET", "/detection/agents/not-a-uuid/capabilities/latest", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestDetectionHandler_GetLatestCapabilityReport_NoOrgContext(t *testing.T) {
	handler := &DetectionHandler{}
	app := fiber.New()
	app.Get("/detection/agents/:id/capabilities/latest", handler.GetLatestCapabilityReport)

	req := httptest.NewRequest("GET", "/detection/agents/"+uuid.New().String()+"/capabilities/latest", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}
