package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// ===========================
// ComplianceHandler Integration Tests
// ===========================

func TestComplianceHandler_GetComplianceStatus_Integration_Success(t *testing.T) {
	app := fiber.New()
	orgID := uuid.New()
	userID := uuid.New()

	mockComplianceService := &MockComplianceServiceExtendedImpl{
		GetComplianceStatusFunc: func(ctx context.Context, org uuid.UUID) (interface{}, error) {
			return map[string]interface{}{
				"overallScore": 85.5,
				"frameworks": []string{"aim"},
			}, nil
		},
	}

	mockAuditService := &MockAuditServiceImpl{}

	handler := NewComplianceHandlerWithInterfaces(mockComplianceService, mockAuditService)

	app.Get("/compliance/status", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		return handler.GetComplianceStatus(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/compliance/status", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	if result["overallScore"] != 85.5 {
		t.Errorf("Expected overallScore 85.5, got %v", result["overallScore"])
	}
}

func TestComplianceHandler_GetComplianceStatus_Integration_ServiceError(t *testing.T) {
	app := fiber.New()
	orgID := uuid.New()
	userID := uuid.New()

	mockComplianceService := &MockComplianceServiceExtendedImpl{
		GetComplianceStatusFunc: func(ctx context.Context, org uuid.UUID) (interface{}, error) {
			return nil, errors.New("database error")
		},
	}

	mockAuditService := &MockAuditServiceImpl{}

	handler := NewComplianceHandlerWithInterfaces(mockComplianceService, mockAuditService)

	app.Get("/compliance/status", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		return handler.GetComplianceStatus(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/compliance/status", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", resp.StatusCode)
	}
}

func TestComplianceHandler_GetComplianceMetrics_Integration_Success(t *testing.T) {
	app := fiber.New()
	orgID := uuid.New()
	userID := uuid.New()

	mockComplianceService := &MockComplianceServiceExtendedImpl{
		GetComplianceMetricsFunc: func(ctx context.Context, org uuid.UUID, start, end time.Time, interval string) (interface{}, error) {
			return map[string]interface{}{
				"dataPoints": []map[string]interface{}{
					{"date": "2025-01-01", "score": 80.0},
					{"date": "2025-01-02", "score": 82.0},
				},
			}, nil
		},
	}

	mockAuditService := &MockAuditServiceImpl{}

	handler := NewComplianceHandlerWithInterfaces(mockComplianceService, mockAuditService)

	app.Get("/compliance/metrics", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		return handler.GetComplianceMetrics(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/compliance/metrics?interval=day", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestComplianceHandler_GetComplianceMetrics_Integration_ServiceError(t *testing.T) {
	app := fiber.New()
	orgID := uuid.New()
	userID := uuid.New()

	mockComplianceService := &MockComplianceServiceExtendedImpl{
		GetComplianceMetricsFunc: func(ctx context.Context, org uuid.UUID, start, end time.Time, interval string) (interface{}, error) {
			return nil, errors.New("service error")
		},
	}

	mockAuditService := &MockAuditServiceImpl{}

	handler := NewComplianceHandlerWithInterfaces(mockComplianceService, mockAuditService)

	app.Get("/compliance/metrics", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		return handler.GetComplianceMetrics(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/compliance/metrics", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", resp.StatusCode)
	}
}

func TestComplianceHandler_GetAccessReview_Integration_Success(t *testing.T) {
	app := fiber.New()
	orgID := uuid.New()
	userID := uuid.New()

	mockComplianceService := &MockComplianceServiceExtendedImpl{
		GetAccessReviewFunc: func(ctx context.Context, org uuid.UUID) (interface{}, error) {
			return map[string]interface{}{
				"users": []map[string]interface{}{
					{"id": uuid.New().String(), "name": "Test User"},
				},
			}, nil
		},
	}

	mockAuditService := &MockAuditServiceImpl{}

	handler := NewComplianceHandlerWithInterfaces(mockComplianceService, mockAuditService)

	app.Get("/compliance/access-review", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		return handler.GetAccessReview(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/compliance/access-review", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestComplianceHandler_GetAccessReview_Integration_ServiceError(t *testing.T) {
	app := fiber.New()
	orgID := uuid.New()
	userID := uuid.New()

	mockComplianceService := &MockComplianceServiceExtendedImpl{
		GetAccessReviewFunc: func(ctx context.Context, org uuid.UUID) (interface{}, error) {
			return nil, errors.New("access review failed")
		},
	}

	mockAuditService := &MockAuditServiceImpl{}

	handler := NewComplianceHandlerWithInterfaces(mockComplianceService, mockAuditService)

	app.Get("/compliance/access-review", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		return handler.GetAccessReview(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/compliance/access-review", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", resp.StatusCode)
	}
}

func TestComplianceHandler_RunComplianceCheck_Integration_Success(t *testing.T) {
	app := fiber.New()
	orgID := uuid.New()
	userID := uuid.New()

	mockComplianceService := &MockComplianceServiceExtendedImpl{
		RunComplianceCheckFunc: func(ctx context.Context, org uuid.UUID, checkType string) (interface{}, error) {
			return map[string]interface{}{
				"checkType": checkType,
				"results":   []map[string]interface{}{{"name": "test", "passed": true}},
			}, nil
		},
	}

	mockAuditService := &MockAuditServiceImpl{}

	handler := NewComplianceHandlerWithInterfaces(mockComplianceService, mockAuditService)

	app.Post("/compliance/check", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		return handler.RunComplianceCheck(c)
	})

	body := `{"checkType": "all"}`
	req := httptest.NewRequest(http.MethodPost, "/compliance/check", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestComplianceHandler_RunComplianceCheck_Integration_InvalidBody(t *testing.T) {
	app := fiber.New()
	orgID := uuid.New()
	userID := uuid.New()

	mockComplianceService := &MockComplianceServiceExtendedImpl{}
	mockAuditService := &MockAuditServiceImpl{}

	handler := NewComplianceHandlerWithInterfaces(mockComplianceService, mockAuditService)

	app.Post("/compliance/check", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		return handler.RunComplianceCheck(c)
	})

	body := `{invalid json}`
	req := httptest.NewRequest(http.MethodPost, "/compliance/check", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}
}

func TestComplianceHandler_RunComplianceCheck_Integration_ServiceError(t *testing.T) {
	app := fiber.New()
	orgID := uuid.New()
	userID := uuid.New()

	mockComplianceService := &MockComplianceServiceExtendedImpl{
		RunComplianceCheckFunc: func(ctx context.Context, org uuid.UUID, checkType string) (interface{}, error) {
			return nil, errors.New("check failed")
		},
	}

	mockAuditService := &MockAuditServiceImpl{}

	handler := NewComplianceHandlerWithInterfaces(mockComplianceService, mockAuditService)

	app.Post("/compliance/check", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		return handler.RunComplianceCheck(c)
	})

	body := `{"checkType": "all"}`
	req := httptest.NewRequest(http.MethodPost, "/compliance/check", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", resp.StatusCode)
	}
}

func TestComplianceHandler_ExportComplianceReport_Integration_JSON(t *testing.T) {
	app := fiber.New()
	orgID := uuid.New()
	userID := uuid.New()

	mockComplianceService := &MockComplianceServiceExtendedImpl{
		ExportComplianceReportFullFunc: func(ctx context.Context, org uuid.UUID, framework string, start, end time.Time, user uuid.UUID) (*domain.ComplianceExportReport, error) {
			return &domain.ComplianceExportReport{
				GeneratedAt: time.Now(),
			}, nil
		},
	}

	mockAuditService := &MockAuditServiceImpl{}

	handler := NewComplianceHandlerWithInterfaces(mockComplianceService, mockAuditService)

	app.Get("/compliance/export", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		return handler.ExportComplianceReport(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/compliance/export?format=json&framework=aim", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "application/json") {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}
}

func TestComplianceHandler_ExportComplianceReport_Integration_CSV(t *testing.T) {
	app := fiber.New()
	orgID := uuid.New()
	userID := uuid.New()

	mockComplianceService := &MockComplianceServiceExtendedImpl{
		ExportToCSVFunc: func(ctx context.Context, org uuid.UUID, framework string) (string, error) {
			return "header1,header2\nvalue1,value2", nil
		},
	}

	mockAuditService := &MockAuditServiceImpl{}

	handler := NewComplianceHandlerWithInterfaces(mockComplianceService, mockAuditService)

	app.Get("/compliance/export", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		return handler.ExportComplianceReport(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/compliance/export?format=csv&framework=aim", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "text/csv") {
		t.Errorf("Expected Content-Type text/csv, got %s", contentType)
	}
}

func TestComplianceHandler_ExportComplianceReport_Integration_InvalidFormat(t *testing.T) {
	app := fiber.New()
	orgID := uuid.New()
	userID := uuid.New()

	mockComplianceService := &MockComplianceServiceExtendedImpl{}
	mockAuditService := &MockAuditServiceImpl{}

	handler := NewComplianceHandlerWithInterfaces(mockComplianceService, mockAuditService)

	app.Get("/compliance/export", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		return handler.ExportComplianceReport(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/compliance/export?format=xml", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}
}

func TestComplianceHandler_ExportComplianceReport_Integration_InvalidFramework(t *testing.T) {
	app := fiber.New()
	orgID := uuid.New()
	userID := uuid.New()

	mockComplianceService := &MockComplianceServiceExtendedImpl{}
	mockAuditService := &MockAuditServiceImpl{}

	handler := NewComplianceHandlerWithInterfaces(mockComplianceService, mockAuditService)

	app.Get("/compliance/export", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		return handler.ExportComplianceReport(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/compliance/export?format=csv&framework=invalid", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}
}

func TestComplianceHandler_GetComplianceTrending_Integration_Success(t *testing.T) {
	app := fiber.New()
	orgID := uuid.New()
	userID := uuid.New()

	mockComplianceService := &MockComplianceServiceExtendedImpl{
		GetComplianceTrendingFunc: func(ctx context.Context, org uuid.UUID, framework string, start, end time.Time) (interface{}, error) {
			return map[string]interface{}{
				"trending": []map[string]interface{}{
					{"date": "2025-01-01", "score": 80.0},
				},
			}, nil
		},
	}

	mockAuditService := &MockAuditServiceImpl{}

	handler := NewComplianceHandlerWithInterfaces(mockComplianceService, mockAuditService)

	app.Get("/compliance/trending", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		return handler.GetComplianceTrending(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/compliance/trending?framework=aim", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestComplianceHandler_GetComplianceTrending_Integration_InvalidFramework(t *testing.T) {
	app := fiber.New()
	orgID := uuid.New()
	userID := uuid.New()

	mockComplianceService := &MockComplianceServiceExtendedImpl{}
	mockAuditService := &MockAuditServiceImpl{}

	handler := NewComplianceHandlerWithInterfaces(mockComplianceService, mockAuditService)

	app.Get("/compliance/trending", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		return handler.GetComplianceTrending(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/compliance/trending?framework=invalid", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}
}

func TestComplianceHandler_GetComplianceTrending_Integration_ServiceError(t *testing.T) {
	app := fiber.New()
	orgID := uuid.New()
	userID := uuid.New()

	mockComplianceService := &MockComplianceServiceExtendedImpl{
		GetComplianceTrendingFunc: func(ctx context.Context, org uuid.UUID, framework string, start, end time.Time) (interface{}, error) {
			return nil, errors.New("service error")
		},
	}

	mockAuditService := &MockAuditServiceImpl{}

	handler := NewComplianceHandlerWithInterfaces(mockComplianceService, mockAuditService)

	app.Get("/compliance/trending", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		return handler.GetComplianceTrending(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/compliance/trending?framework=aim", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", resp.StatusCode)
	}
}

func TestComplianceHandler_RecordComplianceSnapshot_Integration_Success(t *testing.T) {
	app := fiber.New()
	orgID := uuid.New()
	userID := uuid.New()

	mockComplianceService := &MockComplianceServiceExtendedImpl{
		RecordComplianceSnapshotFunc: func(ctx context.Context, org uuid.UUID, framework domain.ComplianceFramework) (*domain.ComplianceSnapshot, error) {
			return &domain.ComplianceSnapshot{
				ID:             uuid.New(),
				OrganizationID: org,
				Framework:      framework,
				Score:          90.0,
			}, nil
		},
	}

	mockAuditService := &MockAuditServiceImpl{}

	handler := NewComplianceHandlerWithInterfaces(mockComplianceService, mockAuditService)

	app.Post("/compliance/snapshot", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		return handler.RecordComplianceSnapshot(c)
	})

	body := `{"framework": "aim"}`
	req := httptest.NewRequest(http.MethodPost, "/compliance/snapshot", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestComplianceHandler_RecordComplianceSnapshot_Integration_InvalidBody(t *testing.T) {
	app := fiber.New()
	orgID := uuid.New()
	userID := uuid.New()

	mockComplianceService := &MockComplianceServiceExtendedImpl{}
	mockAuditService := &MockAuditServiceImpl{}

	handler := NewComplianceHandlerWithInterfaces(mockComplianceService, mockAuditService)

	app.Post("/compliance/snapshot", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		return handler.RecordComplianceSnapshot(c)
	})

	body := `{invalid}`
	req := httptest.NewRequest(http.MethodPost, "/compliance/snapshot", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}
}

func TestComplianceHandler_RecordComplianceSnapshot_Integration_InvalidFramework(t *testing.T) {
	app := fiber.New()
	orgID := uuid.New()
	userID := uuid.New()

	mockComplianceService := &MockComplianceServiceExtendedImpl{}
	mockAuditService := &MockAuditServiceImpl{}

	handler := NewComplianceHandlerWithInterfaces(mockComplianceService, mockAuditService)

	app.Post("/compliance/snapshot", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		return handler.RecordComplianceSnapshot(c)
	})

	body := `{"framework": "invalid"}`
	req := httptest.NewRequest(http.MethodPost, "/compliance/snapshot", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}
}

func TestComplianceHandler_ListEvidence_Integration_Success(t *testing.T) {
	app := fiber.New()
	orgID := uuid.New()
	userID := uuid.New()

	mockComplianceService := &MockComplianceServiceExtendedImpl{
		ListEvidenceFunc: func(ctx context.Context, org uuid.UUID, framework string, limit, offset int) ([]*domain.ComplianceEvidence, error) {
			return []*domain.ComplianceEvidence{
				{
					ID:             uuid.New(),
					OrganizationID: org,
					Framework:      domain.ComplianceFramework(framework),
					CheckName:      "test_check",
				},
			}, nil
		},
	}

	mockAuditService := &MockAuditServiceImpl{}

	handler := NewComplianceHandlerWithInterfaces(mockComplianceService, mockAuditService)

	app.Get("/compliance/evidence", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		return handler.ListEvidence(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/compliance/evidence?framework=aim&limit=10", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	if result["count"] != float64(1) {
		t.Errorf("Expected count 1, got %v", result["count"])
	}
}

func TestComplianceHandler_ListEvidence_Integration_ServiceError(t *testing.T) {
	app := fiber.New()
	orgID := uuid.New()
	userID := uuid.New()

	mockComplianceService := &MockComplianceServiceExtendedImpl{
		ListEvidenceFunc: func(ctx context.Context, org uuid.UUID, framework string, limit, offset int) ([]*domain.ComplianceEvidence, error) {
			return nil, errors.New("failed to list evidence")
		},
	}

	mockAuditService := &MockAuditServiceImpl{}

	handler := NewComplianceHandlerWithInterfaces(mockComplianceService, mockAuditService)

	app.Get("/compliance/evidence", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		return handler.ListEvidence(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/compliance/evidence", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", resp.StatusCode)
	}
}

func TestComplianceHandler_CollectEvidence_Integration_Success(t *testing.T) {
	app := fiber.New()
	orgID := uuid.New()
	userID := uuid.New()

	mockComplianceService := &MockComplianceServiceExtendedImpl{
		CollectEvidenceFunc: func(ctx context.Context, org uuid.UUID, framework domain.ComplianceFramework, checkName string, user uuid.UUID) (*domain.ComplianceEvidence, error) {
			return &domain.ComplianceEvidence{
				ID:             uuid.New(),
				OrganizationID: org,
				Framework:      framework,
				CheckName:      checkName,
			}, nil
		},
	}

	mockAuditService := &MockAuditServiceImpl{}

	handler := NewComplianceHandlerWithInterfaces(mockComplianceService, mockAuditService)

	app.Post("/compliance/evidence/collect", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		return handler.CollectEvidence(c)
	})

	body := `{"framework": "aim", "checkName": "test_check"}`
	req := httptest.NewRequest(http.MethodPost, "/compliance/evidence/collect", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestComplianceHandler_CollectEvidence_Integration_MissingFields(t *testing.T) {
	app := fiber.New()
	orgID := uuid.New()
	userID := uuid.New()

	mockComplianceService := &MockComplianceServiceExtendedImpl{}
	mockAuditService := &MockAuditServiceImpl{}

	handler := NewComplianceHandlerWithInterfaces(mockComplianceService, mockAuditService)

	app.Post("/compliance/evidence/collect", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		return handler.CollectEvidence(c)
	})

	// Missing checkName
	body := `{"framework": "aim"}`
	req := httptest.NewRequest(http.MethodPost, "/compliance/evidence/collect", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}
}

func TestComplianceHandler_CollectEvidence_Integration_InvalidFramework(t *testing.T) {
	app := fiber.New()
	orgID := uuid.New()
	userID := uuid.New()

	mockComplianceService := &MockComplianceServiceExtendedImpl{}
	mockAuditService := &MockAuditServiceImpl{}

	handler := NewComplianceHandlerWithInterfaces(mockComplianceService, mockAuditService)

	app.Post("/compliance/evidence/collect", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		return handler.CollectEvidence(c)
	})

	body := `{"framework": "invalid", "checkName": "test_check"}`
	req := httptest.NewRequest(http.MethodPost, "/compliance/evidence/collect", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}
}

func TestComplianceHandler_GetEvidenceForCheck_Integration_Success(t *testing.T) {
	app := fiber.New()
	orgID := uuid.New()
	userID := uuid.New()

	mockComplianceService := &MockComplianceServiceExtendedImpl{
		GetEvidenceForCheckFunc: func(ctx context.Context, org uuid.UUID, checkName string) ([]*domain.ComplianceEvidence, error) {
			return []*domain.ComplianceEvidence{
				{
					ID:             uuid.New(),
					OrganizationID: org,
					CheckName:      checkName,
				},
			}, nil
		},
	}

	mockAuditService := &MockAuditServiceImpl{}

	handler := NewComplianceHandlerWithInterfaces(mockComplianceService, mockAuditService)

	app.Get("/compliance/evidence/check/:checkName", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		return handler.GetEvidenceForCheck(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/compliance/evidence/check/test_check", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	if result["checkName"] != "test_check" {
		t.Errorf("Expected checkName test_check, got %v", result["checkName"])
	}
}

func TestComplianceHandler_GetEvidenceForCheck_Integration_EmptyCheckName(t *testing.T) {
	app := fiber.New()
	orgID := uuid.New()
	userID := uuid.New()

	mockComplianceService := &MockComplianceServiceExtendedImpl{}
	mockAuditService := &MockAuditServiceImpl{}

	handler := NewComplianceHandlerWithInterfaces(mockComplianceService, mockAuditService)

	app.Get("/compliance/evidence/check/:checkName", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		return handler.GetEvidenceForCheck(c)
	})

	// No checkName in path - this won't match the route, but test empty param handling
	req := httptest.NewRequest(http.MethodGet, "/compliance/evidence/check/", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}
	defer resp.Body.Close()

	// With Fiber, an empty param will result in 404 since the route won't match
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", resp.StatusCode)
	}
}

func TestComplianceHandler_GetEvidenceForCheck_Integration_ServiceError(t *testing.T) {
	app := fiber.New()
	orgID := uuid.New()
	userID := uuid.New()

	mockComplianceService := &MockComplianceServiceExtendedImpl{
		GetEvidenceForCheckFunc: func(ctx context.Context, org uuid.UUID, checkName string) ([]*domain.ComplianceEvidence, error) {
			return nil, errors.New("failed to get evidence")
		},
	}

	mockAuditService := &MockAuditServiceImpl{}

	handler := NewComplianceHandlerWithInterfaces(mockComplianceService, mockAuditService)

	app.Get("/compliance/evidence/check/:checkName", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		return handler.GetEvidenceForCheck(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/compliance/evidence/check/test_check", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", resp.StatusCode)
	}
}

func TestNewComplianceHandlerWithInterfaces(t *testing.T) {
	mockComplianceService := &MockComplianceServiceExtendedImpl{}
	mockAuditService := &MockAuditServiceImpl{}

	handler := NewComplianceHandlerWithInterfaces(mockComplianceService, mockAuditService)

	if handler == nil {
		t.Error("Expected handler to be non-nil")
	}

	// Test that interfaces are set
	if handler.complianceServicer == nil {
		t.Error("Expected complianceServicer to be set")
	}
	if handler.auditServicer == nil {
		t.Error("Expected auditServicer to be set")
	}

	// Test that concrete types are nil
	if handler.complianceService != nil {
		t.Error("Expected complianceService to be nil")
	}
	if handler.auditService != nil {
		t.Error("Expected auditService to be nil")
	}
}
