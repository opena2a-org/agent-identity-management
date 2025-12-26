package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// ===========================
// WebhookHandler Integration Tests
// ===========================

// Test IDs for webhook tests
var (
	webhookTestOrgID  = uuid.MustParse("11111111-1111-1111-1111-111111111111")
	webhookTestUserID = uuid.MustParse("22222222-2222-2222-2222-222222222222")
)

func setupWebhookTestApp(handler *WebhookHandler) *fiber.App {
	app := fiber.New()

	// Middleware to set organization and user context
	app.Use(func(c fiber.Ctx) error {
		c.Locals("organization_id", webhookTestOrgID)
		c.Locals("user_id", webhookTestUserID)
		return c.Next()
	})

	// Routes
	app.Post("/webhooks", handler.CreateWebhook)
	app.Get("/webhooks", handler.ListWebhooks)
	app.Get("/webhooks/:id", handler.GetWebhook)
	app.Put("/webhooks/:id", handler.UpdateWebhook)
	app.Delete("/webhooks/:id", handler.DeleteWebhook)
	app.Post("/webhooks/:id/test", handler.TestWebhook)

	return app
}

// ===========================
// CreateWebhook Tests
// ===========================

func TestWebhookHandler_CreateWebhook_Success(t *testing.T) {
	webhookID := uuid.New()

	mockWebhookService := &MockWebhookServicerImpl{
		CreateWebhookFunc: func(ctx context.Context, req *application.CreateWebhookRequest, orgID, userID uuid.UUID) (*domain.Webhook, error) {
			return &domain.Webhook{
				ID:             webhookID,
				OrganizationID: orgID,
				Name:           req.Name,
				URL:            req.URL,
				IsActive:       true,
			}, nil
		},
	}
	mockAuditService := &MockAuditServiceImpl{}

	handler := NewWebhookHandlerWithInterfaces(mockWebhookService, mockAuditService)
	app := setupWebhookTestApp(handler)

	reqBody := map[string]interface{}{
		"name":   "Test Webhook",
		"url":    "https://example.com/webhook",
		"events": []string{"agent.created", "agent.updated"},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/webhooks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		t.Errorf("Expected status 201, got %d. Body: %s", resp.StatusCode, string(respBody))
	}

	var result domain.Webhook
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if result.ID != webhookID {
		t.Errorf("Expected webhook ID %s, got %s", webhookID, result.ID)
	}
	if result.Name != "Test Webhook" {
		t.Errorf("Expected name 'Test Webhook', got '%s'", result.Name)
	}
}

func TestWebhookHandler_CreateWebhook_InvalidBody(t *testing.T) {
	mockWebhookService := &MockWebhookServicerImpl{}
	mockAuditService := &MockAuditServiceImpl{}

	handler := NewWebhookHandlerWithInterfaces(mockWebhookService, mockAuditService)
	app := setupWebhookTestApp(handler)

	req := httptest.NewRequest(http.MethodPost, "/webhooks", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}
}

func TestWebhookHandler_CreateWebhook_ServiceError(t *testing.T) {
	mockWebhookService := &MockWebhookServicerImpl{
		CreateWebhookFunc: func(ctx context.Context, req *application.CreateWebhookRequest, orgID, userID uuid.UUID) (*domain.Webhook, error) {
			return nil, errors.New("service error")
		},
	}
	mockAuditService := &MockAuditServiceImpl{}

	handler := NewWebhookHandlerWithInterfaces(mockWebhookService, mockAuditService)
	app := setupWebhookTestApp(handler)

	reqBody := map[string]interface{}{
		"name": "Test Webhook",
		"url":  "https://example.com/webhook",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/webhooks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", resp.StatusCode)
	}
}

// ===========================
// ListWebhooks Tests
// ===========================

func TestWebhookHandler_ListWebhooks_Success(t *testing.T) {
	webhookID := uuid.New()

	mockWebhookService := &MockWebhookServicerImpl{
		ListWebhooksFunc: func(ctx context.Context, orgID uuid.UUID) ([]*domain.Webhook, error) {
			return []*domain.Webhook{
				{ID: webhookID, OrganizationID: orgID, Name: "Webhook 1", URL: "https://example.com/1"},
				{ID: uuid.New(), OrganizationID: orgID, Name: "Webhook 2", URL: "https://example.com/2"},
			}, nil
		},
	}
	mockAuditService := &MockAuditServiceImpl{}

	handler := NewWebhookHandlerWithInterfaces(mockWebhookService, mockAuditService)
	app := setupWebhookTestApp(handler)

	req := httptest.NewRequest(http.MethodGet, "/webhooks", nil)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	total := int(result["total"].(float64))
	if total != 2 {
		t.Errorf("Expected total 2, got %d", total)
	}
}

func TestWebhookHandler_ListWebhooks_Empty(t *testing.T) {
	mockWebhookService := &MockWebhookServicerImpl{
		ListWebhooksFunc: func(ctx context.Context, orgID uuid.UUID) ([]*domain.Webhook, error) {
			return nil, nil
		},
	}
	mockAuditService := &MockAuditServiceImpl{}

	handler := NewWebhookHandlerWithInterfaces(mockWebhookService, mockAuditService)
	app := setupWebhookTestApp(handler)

	req := httptest.NewRequest(http.MethodGet, "/webhooks", nil)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	total := int(result["total"].(float64))
	if total != 0 {
		t.Errorf("Expected total 0, got %d", total)
	}
}

func TestWebhookHandler_ListWebhooks_ServiceError(t *testing.T) {
	mockWebhookService := &MockWebhookServicerImpl{
		ListWebhooksFunc: func(ctx context.Context, orgID uuid.UUID) ([]*domain.Webhook, error) {
			return nil, errors.New("database error")
		},
	}
	mockAuditService := &MockAuditServiceImpl{}

	handler := NewWebhookHandlerWithInterfaces(mockWebhookService, mockAuditService)
	app := setupWebhookTestApp(handler)

	req := httptest.NewRequest(http.MethodGet, "/webhooks", nil)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", resp.StatusCode)
	}
}

// ===========================
// GetWebhook Tests
// ===========================

func TestWebhookHandler_GetWebhook_Success(t *testing.T) {
	webhookID := uuid.New()

	mockWebhookService := &MockWebhookServicerImpl{
		GetWebhookFunc: func(ctx context.Context, id uuid.UUID) (*domain.Webhook, error) {
			return &domain.Webhook{
				ID:             webhookID,
				OrganizationID: webhookTestOrgID,
				Name:           "Test Webhook",
				URL:            "https://example.com/webhook",
			}, nil
		},
	}
	mockAuditService := &MockAuditServiceImpl{}

	handler := NewWebhookHandlerWithInterfaces(mockWebhookService, mockAuditService)
	app := setupWebhookTestApp(handler)

	req := httptest.NewRequest(http.MethodGet, "/webhooks/"+webhookID.String(), nil)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Errorf("Expected status 200, got %d. Body: %s", resp.StatusCode, string(respBody))
	}
}

func TestWebhookHandler_GetWebhook_InvalidIDIntegration(t *testing.T) {
	mockWebhookService := &MockWebhookServicerImpl{}
	mockAuditService := &MockAuditServiceImpl{}

	handler := NewWebhookHandlerWithInterfaces(mockWebhookService, mockAuditService)
	app := setupWebhookTestApp(handler)

	req := httptest.NewRequest(http.MethodGet, "/webhooks/invalid-uuid", nil)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}
}

func TestWebhookHandler_GetWebhook_NotFound(t *testing.T) {
	webhookID := uuid.New()

	mockWebhookService := &MockWebhookServicerImpl{
		GetWebhookFunc: func(ctx context.Context, id uuid.UUID) (*domain.Webhook, error) {
			return nil, errors.New("not found")
		},
	}
	mockAuditService := &MockAuditServiceImpl{}

	handler := NewWebhookHandlerWithInterfaces(mockWebhookService, mockAuditService)
	app := setupWebhookTestApp(handler)

	req := httptest.NewRequest(http.MethodGet, "/webhooks/"+webhookID.String(), nil)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", resp.StatusCode)
	}
}

func TestWebhookHandler_GetWebhook_AccessDenied(t *testing.T) {
	webhookID := uuid.New()
	otherOrgID := uuid.New()

	mockWebhookService := &MockWebhookServicerImpl{
		GetWebhookFunc: func(ctx context.Context, id uuid.UUID) (*domain.Webhook, error) {
			return &domain.Webhook{
				ID:             webhookID,
				OrganizationID: otherOrgID, // Different org
				Name:           "Other Org Webhook",
			}, nil
		},
	}
	mockAuditService := &MockAuditServiceImpl{}

	handler := NewWebhookHandlerWithInterfaces(mockWebhookService, mockAuditService)
	app := setupWebhookTestApp(handler)

	req := httptest.NewRequest(http.MethodGet, "/webhooks/"+webhookID.String(), nil)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", resp.StatusCode)
	}
}

// ===========================
// UpdateWebhook Tests
// ===========================

func TestWebhookHandler_UpdateWebhook_Success(t *testing.T) {
	webhookID := uuid.New()

	mockWebhookService := &MockWebhookServicerImpl{
		GetWebhookFunc: func(ctx context.Context, id uuid.UUID) (*domain.Webhook, error) {
			return &domain.Webhook{
				ID:             webhookID,
				OrganizationID: webhookTestOrgID,
				Name:           "Original Webhook",
			}, nil
		},
		UpdateWebhookFunc: func(ctx context.Context, id uuid.UUID, req *application.CreateWebhookRequest) (*domain.Webhook, error) {
			return &domain.Webhook{
				ID:             id,
				OrganizationID: webhookTestOrgID,
				Name:           req.Name,
				URL:            req.URL,
				IsActive:       true,
			}, nil
		},
	}
	mockAuditService := &MockAuditServiceImpl{}

	handler := NewWebhookHandlerWithInterfaces(mockWebhookService, mockAuditService)
	app := setupWebhookTestApp(handler)

	reqBody := map[string]interface{}{
		"name": "Updated Webhook",
		"url":  "https://example.com/updated",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, "/webhooks/"+webhookID.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Errorf("Expected status 200, got %d. Body: %s", resp.StatusCode, string(respBody))
	}
}

func TestWebhookHandler_UpdateWebhook_InvalidIDIntegration(t *testing.T) {
	mockWebhookService := &MockWebhookServicerImpl{}
	mockAuditService := &MockAuditServiceImpl{}

	handler := NewWebhookHandlerWithInterfaces(mockWebhookService, mockAuditService)
	app := setupWebhookTestApp(handler)

	req := httptest.NewRequest(http.MethodPut, "/webhooks/invalid-uuid", nil)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}
}

func TestWebhookHandler_UpdateWebhook_NotFound(t *testing.T) {
	webhookID := uuid.New()

	mockWebhookService := &MockWebhookServicerImpl{
		GetWebhookFunc: func(ctx context.Context, id uuid.UUID) (*domain.Webhook, error) {
			return nil, errors.New("not found")
		},
	}
	mockAuditService := &MockAuditServiceImpl{}

	handler := NewWebhookHandlerWithInterfaces(mockWebhookService, mockAuditService)
	app := setupWebhookTestApp(handler)

	reqBody := map[string]interface{}{"name": "Updated"}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, "/webhooks/"+webhookID.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", resp.StatusCode)
	}
}

func TestWebhookHandler_UpdateWebhook_AccessDenied(t *testing.T) {
	webhookID := uuid.New()
	otherOrgID := uuid.New()

	mockWebhookService := &MockWebhookServicerImpl{
		GetWebhookFunc: func(ctx context.Context, id uuid.UUID) (*domain.Webhook, error) {
			return &domain.Webhook{
				ID:             webhookID,
				OrganizationID: otherOrgID,
			}, nil
		},
	}
	mockAuditService := &MockAuditServiceImpl{}

	handler := NewWebhookHandlerWithInterfaces(mockWebhookService, mockAuditService)
	app := setupWebhookTestApp(handler)

	reqBody := map[string]interface{}{"name": "Updated"}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, "/webhooks/"+webhookID.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", resp.StatusCode)
	}
}

func TestWebhookHandler_UpdateWebhook_InvalidBody(t *testing.T) {
	webhookID := uuid.New()

	mockWebhookService := &MockWebhookServicerImpl{
		GetWebhookFunc: func(ctx context.Context, id uuid.UUID) (*domain.Webhook, error) {
			return &domain.Webhook{
				ID:             webhookID,
				OrganizationID: webhookTestOrgID,
			}, nil
		},
	}
	mockAuditService := &MockAuditServiceImpl{}

	handler := NewWebhookHandlerWithInterfaces(mockWebhookService, mockAuditService)
	app := setupWebhookTestApp(handler)

	req := httptest.NewRequest(http.MethodPut, "/webhooks/"+webhookID.String(), bytes.NewReader([]byte("invalid")))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}
}

func TestWebhookHandler_UpdateWebhook_ServiceError(t *testing.T) {
	webhookID := uuid.New()

	mockWebhookService := &MockWebhookServicerImpl{
		GetWebhookFunc: func(ctx context.Context, id uuid.UUID) (*domain.Webhook, error) {
			return &domain.Webhook{
				ID:             webhookID,
				OrganizationID: webhookTestOrgID,
			}, nil
		},
		UpdateWebhookFunc: func(ctx context.Context, id uuid.UUID, req *application.CreateWebhookRequest) (*domain.Webhook, error) {
			return nil, errors.New("update failed")
		},
	}
	mockAuditService := &MockAuditServiceImpl{}

	handler := NewWebhookHandlerWithInterfaces(mockWebhookService, mockAuditService)
	app := setupWebhookTestApp(handler)

	reqBody := map[string]interface{}{"name": "Updated"}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, "/webhooks/"+webhookID.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", resp.StatusCode)
	}
}

// ===========================
// DeleteWebhook Tests
// ===========================

func TestWebhookHandler_DeleteWebhook_Success(t *testing.T) {
	webhookID := uuid.New()

	mockWebhookService := &MockWebhookServicerImpl{
		GetWebhookFunc: func(ctx context.Context, id uuid.UUID) (*domain.Webhook, error) {
			return &domain.Webhook{
				ID:             webhookID,
				OrganizationID: webhookTestOrgID,
			}, nil
		},
		DeleteWebhookFunc: func(ctx context.Context, id uuid.UUID) error {
			return nil
		},
	}
	mockAuditService := &MockAuditServiceImpl{}

	handler := NewWebhookHandlerWithInterfaces(mockWebhookService, mockAuditService)
	app := setupWebhookTestApp(handler)

	req := httptest.NewRequest(http.MethodDelete, "/webhooks/"+webhookID.String(), nil)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("Expected status 204, got %d", resp.StatusCode)
	}
}

func TestWebhookHandler_DeleteWebhook_InvalidIDIntegration(t *testing.T) {
	mockWebhookService := &MockWebhookServicerImpl{}
	mockAuditService := &MockAuditServiceImpl{}

	handler := NewWebhookHandlerWithInterfaces(mockWebhookService, mockAuditService)
	app := setupWebhookTestApp(handler)

	req := httptest.NewRequest(http.MethodDelete, "/webhooks/invalid-uuid", nil)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}
}

func TestWebhookHandler_DeleteWebhook_NotFound(t *testing.T) {
	webhookID := uuid.New()

	mockWebhookService := &MockWebhookServicerImpl{
		GetWebhookFunc: func(ctx context.Context, id uuid.UUID) (*domain.Webhook, error) {
			return nil, errors.New("not found")
		},
	}
	mockAuditService := &MockAuditServiceImpl{}

	handler := NewWebhookHandlerWithInterfaces(mockWebhookService, mockAuditService)
	app := setupWebhookTestApp(handler)

	req := httptest.NewRequest(http.MethodDelete, "/webhooks/"+webhookID.String(), nil)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", resp.StatusCode)
	}
}

func TestWebhookHandler_DeleteWebhook_AccessDenied(t *testing.T) {
	webhookID := uuid.New()
	otherOrgID := uuid.New()

	mockWebhookService := &MockWebhookServicerImpl{
		GetWebhookFunc: func(ctx context.Context, id uuid.UUID) (*domain.Webhook, error) {
			return &domain.Webhook{
				ID:             webhookID,
				OrganizationID: otherOrgID,
			}, nil
		},
	}
	mockAuditService := &MockAuditServiceImpl{}

	handler := NewWebhookHandlerWithInterfaces(mockWebhookService, mockAuditService)
	app := setupWebhookTestApp(handler)

	req := httptest.NewRequest(http.MethodDelete, "/webhooks/"+webhookID.String(), nil)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", resp.StatusCode)
	}
}

func TestWebhookHandler_DeleteWebhook_ServiceError(t *testing.T) {
	webhookID := uuid.New()

	mockWebhookService := &MockWebhookServicerImpl{
		GetWebhookFunc: func(ctx context.Context, id uuid.UUID) (*domain.Webhook, error) {
			return &domain.Webhook{
				ID:             webhookID,
				OrganizationID: webhookTestOrgID,
			}, nil
		},
		DeleteWebhookFunc: func(ctx context.Context, id uuid.UUID) error {
			return errors.New("delete failed")
		},
	}
	mockAuditService := &MockAuditServiceImpl{}

	handler := NewWebhookHandlerWithInterfaces(mockWebhookService, mockAuditService)
	app := setupWebhookTestApp(handler)

	req := httptest.NewRequest(http.MethodDelete, "/webhooks/"+webhookID.String(), nil)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", resp.StatusCode)
	}
}

// ===========================
// TestWebhook Tests
// ===========================

func TestWebhookHandler_TestWebhook_Success(t *testing.T) {
	webhookID := uuid.New()

	mockWebhookService := &MockWebhookServicerImpl{
		GetWebhookFunc: func(ctx context.Context, id uuid.UUID) (*domain.Webhook, error) {
			return &domain.Webhook{
				ID:             webhookID,
				OrganizationID: webhookTestOrgID,
				Name:           "Test Webhook",
				URL:            "https://example.com/webhook",
			}, nil
		},
		TestWebhookFunc: func(ctx context.Context, id uuid.UUID) (*application.WebhookTestResult, error) {
			return &application.WebhookTestResult{
				Success:    true,
				StatusCode: 200,
			}, nil
		},
	}
	mockAuditService := &MockAuditServiceImpl{}

	handler := NewWebhookHandlerWithInterfaces(mockWebhookService, mockAuditService)
	app := setupWebhookTestApp(handler)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/"+webhookID.String()+"/test", nil)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Errorf("Expected status 200, got %d. Body: %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if result["success"] != true {
		t.Error("Expected success to be true")
	}
	if result["responseCode"].(float64) != 200 {
		t.Errorf("Expected responseCode 200, got %v", result["responseCode"])
	}
}

func TestWebhookHandler_TestWebhook_InvalidIDIntegration(t *testing.T) {
	mockWebhookService := &MockWebhookServicerImpl{}
	mockAuditService := &MockAuditServiceImpl{}

	handler := NewWebhookHandlerWithInterfaces(mockWebhookService, mockAuditService)
	app := setupWebhookTestApp(handler)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/invalid-uuid/test", nil)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}
}

func TestWebhookHandler_TestWebhook_NotFound(t *testing.T) {
	webhookID := uuid.New()

	mockWebhookService := &MockWebhookServicerImpl{
		GetWebhookFunc: func(ctx context.Context, id uuid.UUID) (*domain.Webhook, error) {
			return nil, errors.New("not found")
		},
	}
	mockAuditService := &MockAuditServiceImpl{}

	handler := NewWebhookHandlerWithInterfaces(mockWebhookService, mockAuditService)
	app := setupWebhookTestApp(handler)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/"+webhookID.String()+"/test", nil)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", resp.StatusCode)
	}
}

func TestWebhookHandler_TestWebhook_AccessDenied(t *testing.T) {
	webhookID := uuid.New()
	otherOrgID := uuid.New()

	mockWebhookService := &MockWebhookServicerImpl{
		GetWebhookFunc: func(ctx context.Context, id uuid.UUID) (*domain.Webhook, error) {
			return &domain.Webhook{
				ID:             webhookID,
				OrganizationID: otherOrgID,
			}, nil
		},
	}
	mockAuditService := &MockAuditServiceImpl{}

	handler := NewWebhookHandlerWithInterfaces(mockWebhookService, mockAuditService)
	app := setupWebhookTestApp(handler)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/"+webhookID.String()+"/test", nil)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", resp.StatusCode)
	}
}

func TestWebhookHandler_TestWebhook_ServiceError(t *testing.T) {
	webhookID := uuid.New()

	mockWebhookService := &MockWebhookServicerImpl{
		GetWebhookFunc: func(ctx context.Context, id uuid.UUID) (*domain.Webhook, error) {
			return &domain.Webhook{
				ID:             webhookID,
				OrganizationID: webhookTestOrgID,
				Name:           "Test Webhook",
			}, nil
		},
		TestWebhookFunc: func(ctx context.Context, id uuid.UUID) (*application.WebhookTestResult, error) {
			return nil, errors.New("failed to send test")
		},
	}
	mockAuditService := &MockAuditServiceImpl{}

	handler := NewWebhookHandlerWithInterfaces(mockWebhookService, mockAuditService)
	app := setupWebhookTestApp(handler)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/"+webhookID.String()+"/test", nil)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", resp.StatusCode)
	}
}

func TestWebhookHandler_TestWebhook_FailedResponse(t *testing.T) {
	webhookID := uuid.New()

	mockWebhookService := &MockWebhookServicerImpl{
		GetWebhookFunc: func(ctx context.Context, id uuid.UUID) (*domain.Webhook, error) {
			return &domain.Webhook{
				ID:             webhookID,
				OrganizationID: webhookTestOrgID,
				Name:           "Test Webhook",
				URL:            "https://example.com/webhook",
			}, nil
		},
		TestWebhookFunc: func(ctx context.Context, id uuid.UUID) (*application.WebhookTestResult, error) {
			return &application.WebhookTestResult{
				Success:      false,
				StatusCode:   500,
				ErrorMessage: "Server error",
			}, nil
		},
	}
	mockAuditService := &MockAuditServiceImpl{}

	handler := NewWebhookHandlerWithInterfaces(mockWebhookService, mockAuditService)
	app := setupWebhookTestApp(handler)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/"+webhookID.String()+"/test", nil)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}

	// Handler always returns 200 with result details
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Errorf("Expected status 200, got %d. Body: %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if result["success"] != false {
		t.Error("Expected success to be false")
	}
}
