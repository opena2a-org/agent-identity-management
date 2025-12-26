package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
// VerificationHandler Integration Tests
// ===========================

func setupVerificationTestApp(handler *VerificationHandler) *fiber.App {
	app := fiber.New()

	// Public routes
	app.Get("/api/v1/verifications/:id", handler.GetVerification)
	app.Post("/api/v1/verifications/:id/result", handler.SubmitVerificationResult)

	// SDK API routes
	app.Post("/api/v1/sdk-api/verifications/:id/execution-status", handler.UpdateExecutionStatus)

	// Admin routes - require organization context
	adminGroup := app.Group("/api/v1/admin")
	adminGroup.Use(func(c fiber.Ctx) error {
		orgID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
		userID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)
		c.Locals("user_name", "test-admin")
		return c.Next()
	})
	adminGroup.Get("/verifications/pending", handler.ListPendingVerifications)
	adminGroup.Post("/verifications/:id/approve", handler.ApproveVerification)
	adminGroup.Post("/verifications/:id/deny", handler.DenyVerification)

	return app
}

// ===========================
// GetVerification Tests
// ===========================

func TestVerificationHandler_GetVerification_Success(t *testing.T) {
	// Arrange
	verificationID := uuid.New()
	orgID := uuid.New()
	agentID := uuid.New()
	now := time.Now()
	verifiedResult := domain.VerificationResultVerified

	mockVerifEventService := &MockVerificationEventServiceForVerificationImpl{
		GetVerificationEventFunc: func(ctx context.Context, id uuid.UUID) (*domain.VerificationEvent, error) {
			return &domain.VerificationEvent{
				ID:             verificationID,
				OrganizationID: orgID,
				AgentID:        &agentID,
				Status:         domain.VerificationEventStatusSuccess,
				Result:         &verifiedResult,
				TrustScore:     0.85,
				CreatedAt:      now,
			}, nil
		},
	}

	mockOrgRepo := &MockOrganizationRepositoryerImpl{
		GetByIDFunc: func(id uuid.UUID) (*domain.Organization, error) {
			return &domain.Organization{
				ID:              id,
				EnforcementMode: domain.EnforcementModeStrict,
			}, nil
		},
	}

	handler := NewVerificationHandlerWithInterfaces(
		&MockAgentServiceForVerificationImpl{},
		&MockAuditServiceForVerificationImpl{},
		&MockAlertServiceForVerificationImpl{},
		mockVerifEventService,
		mockOrgRepo,
	)

	app := setupVerificationTestApp(handler)

	// Act
	req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/verifications/%s", verificationID.String()), nil)
	resp, err := app.Test(req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var response VerificationResponse
	body, _ := io.ReadAll(resp.Body)
	err = json.Unmarshal(body, &response)
	require.NoError(t, err)

	assert.Equal(t, verificationID.String(), response.ID)
	assert.Equal(t, "approved", response.Status)
	assert.Equal(t, 0.85, response.TrustScore)
	assert.Equal(t, "strict", response.EnforcementMode)
}

func TestVerificationHandler_GetVerification_NotFound(t *testing.T) {
	// Arrange
	verificationID := uuid.New()

	mockVerifEventService := &MockVerificationEventServiceForVerificationImpl{
		GetVerificationEventFunc: func(ctx context.Context, id uuid.UUID) (*domain.VerificationEvent, error) {
			return nil, fmt.Errorf("not found")
		},
	}

	handler := NewVerificationHandlerWithInterfaces(
		&MockAgentServiceForVerificationImpl{},
		&MockAuditServiceForVerificationImpl{},
		&MockAlertServiceForVerificationImpl{},
		mockVerifEventService,
		&MockOrganizationRepositoryerImpl{},
	)

	app := setupVerificationTestApp(handler)

	// Act
	req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/verifications/%s", verificationID.String()), nil)
	resp, err := app.Test(req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

func TestVerificationHandler_GetVerification_InvalidIDWithMocks(t *testing.T) {
	// Arrange
	handler := NewVerificationHandlerWithInterfaces(
		&MockAgentServiceForVerificationImpl{},
		&MockAuditServiceForVerificationImpl{},
		&MockAlertServiceForVerificationImpl{},
		&MockVerificationEventServiceForVerificationImpl{},
		&MockOrganizationRepositoryerImpl{},
	)

	app := setupVerificationTestApp(handler)

	// Act
	req := httptest.NewRequest("GET", "/api/v1/verifications/invalid-uuid", nil)
	resp, err := app.Test(req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestVerificationHandler_GetVerification_Denied(t *testing.T) {
	// Arrange
	verificationID := uuid.New()
	orgID := uuid.New()
	now := time.Now()
	deniedResult := domain.VerificationResultDenied
	errorReason := "Insufficient trust score"

	mockVerifEventService := &MockVerificationEventServiceForVerificationImpl{
		GetVerificationEventFunc: func(ctx context.Context, id uuid.UUID) (*domain.VerificationEvent, error) {
			return &domain.VerificationEvent{
				ID:             verificationID,
				OrganizationID: orgID,
				Status:         domain.VerificationEventStatusFailed,
				Result:         &deniedResult,
				ErrorReason:    &errorReason,
				TrustScore:     0.3,
				CreatedAt:      now,
			}, nil
		},
	}

	handler := NewVerificationHandlerWithInterfaces(
		&MockAgentServiceForVerificationImpl{},
		&MockAuditServiceForVerificationImpl{},
		&MockAlertServiceForVerificationImpl{},
		mockVerifEventService,
		&MockOrganizationRepositoryerImpl{},
	)

	app := setupVerificationTestApp(handler)

	// Act
	req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/verifications/%s", verificationID.String()), nil)
	resp, err := app.Test(req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var response VerificationResponse
	body, _ := io.ReadAll(resp.Body)
	err = json.Unmarshal(body, &response)
	require.NoError(t, err)

	assert.Equal(t, "denied", response.Status)
	assert.Equal(t, "Insufficient trust score", response.DenialReason)
}

// ===========================
// SubmitVerificationResult Tests
// ===========================

func TestVerificationHandler_SubmitVerificationResult_Success(t *testing.T) {
	// Arrange
	verificationID := uuid.New()

	mockVerifEventService := &MockVerificationEventServiceForVerificationImpl{
		UpdateVerificationResultFunc: func(ctx context.Context, id uuid.UUID, result domain.VerificationResult, reason *string, metadata map[string]interface{}) error {
			return nil
		},
	}

	handler := NewVerificationHandlerWithInterfaces(
		&MockAgentServiceForVerificationImpl{},
		&MockAuditServiceForVerificationImpl{},
		&MockAlertServiceForVerificationImpl{},
		mockVerifEventService,
		&MockOrganizationRepositoryerImpl{},
	)

	app := setupVerificationTestApp(handler)

	reqBody := map[string]interface{}{
		"result":   "success",
		"reason":   "Action completed",
		"metadata": map[string]interface{}{"key": "value"},
	}
	bodyBytes, _ := json.Marshal(reqBody)

	// Act
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/verifications/%s/result", verificationID.String()), bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var response map[string]interface{}
	body, _ := io.ReadAll(resp.Body)
	err = json.Unmarshal(body, &response)
	require.NoError(t, err)

	assert.Equal(t, verificationID.String(), response["id"])
	assert.Equal(t, "result_recorded", response["status"])
	assert.Equal(t, "success", response["result"])
}

func TestVerificationHandler_SubmitVerificationResult_Failure(t *testing.T) {
	// Arrange
	verificationID := uuid.New()

	mockVerifEventService := &MockVerificationEventServiceForVerificationImpl{
		UpdateVerificationResultFunc: func(ctx context.Context, id uuid.UUID, result domain.VerificationResult, reason *string, metadata map[string]interface{}) error {
			return nil
		},
	}

	handler := NewVerificationHandlerWithInterfaces(
		&MockAgentServiceForVerificationImpl{},
		&MockAuditServiceForVerificationImpl{},
		&MockAlertServiceForVerificationImpl{},
		mockVerifEventService,
		&MockOrganizationRepositoryerImpl{},
	)

	app := setupVerificationTestApp(handler)

	reqBody := map[string]interface{}{
		"result": "failure",
		"reason": "Action failed due to error",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	// Act
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/verifications/%s/result", verificationID.String()), bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestVerificationHandler_SubmitVerificationResult_InvalidResultWithMocks(t *testing.T) {
	// Arrange
	verificationID := uuid.New()

	handler := NewVerificationHandlerWithInterfaces(
		&MockAgentServiceForVerificationImpl{},
		&MockAuditServiceForVerificationImpl{},
		&MockAlertServiceForVerificationImpl{},
		&MockVerificationEventServiceForVerificationImpl{},
		&MockOrganizationRepositoryerImpl{},
	)

	app := setupVerificationTestApp(handler)

	reqBody := map[string]interface{}{
		"result": "invalid",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	// Act
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/verifications/%s/result", verificationID.String()), bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestVerificationHandler_SubmitVerificationResult_NotFound(t *testing.T) {
	// Arrange
	verificationID := uuid.New()

	mockVerifEventService := &MockVerificationEventServiceForVerificationImpl{
		UpdateVerificationResultFunc: func(ctx context.Context, id uuid.UUID, result domain.VerificationResult, reason *string, metadata map[string]interface{}) error {
			return fmt.Errorf("not found")
		},
	}

	handler := NewVerificationHandlerWithInterfaces(
		&MockAgentServiceForVerificationImpl{},
		&MockAuditServiceForVerificationImpl{},
		&MockAlertServiceForVerificationImpl{},
		mockVerifEventService,
		&MockOrganizationRepositoryerImpl{},
	)

	app := setupVerificationTestApp(handler)

	reqBody := map[string]interface{}{
		"result": "success",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	// Act
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/verifications/%s/result", verificationID.String()), bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

// ===========================
// ListPendingVerifications Tests
// ===========================

func TestVerificationHandler_ListPendingVerifications_Success(t *testing.T) {
	// Arrange
	orgID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	agentID := uuid.New()
	agentName := "test-agent"
	action := "read_database"
	now := time.Now()

	mockVerifEventService := &MockVerificationEventServiceForVerificationImpl{
		SearchVerificationsFunc: func(ctx context.Context, orgIDParam uuid.UUID, params domain.VerificationQueryParams) ([]*domain.VerificationEvent, int, *domain.VerificationStatusCounts, error) {
			return []*domain.VerificationEvent{
				{
					ID:             uuid.New(),
					OrganizationID: orgID,
					AgentID:        &agentID,
					InitiatorName:  &agentName,
					Action:         &action,
					Status:         domain.VerificationEventStatusPending,
					TrustScore:     0.75,
					CreatedAt:      now,
					Metadata: map[string]interface{}{
						"risk_level": "medium",
					},
				},
			}, 1, &domain.VerificationStatusCounts{Pending: 1}, nil
		},
	}

	handler := NewVerificationHandlerWithInterfaces(
		&MockAgentServiceForVerificationImpl{},
		&MockAuditServiceForVerificationImpl{},
		&MockAlertServiceForVerificationImpl{},
		mockVerifEventService,
		&MockOrganizationRepositoryerImpl{},
	)

	app := setupVerificationTestApp(handler)

	// Act
	req := httptest.NewRequest("GET", "/api/v1/admin/verifications/pending", nil)
	resp, err := app.Test(req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var response PendingVerificationListResponse
	body, _ := io.ReadAll(resp.Body)
	err = json.Unmarshal(body, &response)
	require.NoError(t, err)

	assert.Len(t, response.Verifications, 1)
	assert.Equal(t, 1, response.Pagination.Total)
	assert.Equal(t, "test-agent", response.Verifications[0].AgentName)
	assert.Equal(t, "read_database", response.Verifications[0].ActionType)
}

func TestVerificationHandler_ListPendingVerifications_Empty(t *testing.T) {
	// Arrange
	mockVerifEventService := &MockVerificationEventServiceForVerificationImpl{
		SearchVerificationsFunc: func(ctx context.Context, orgID uuid.UUID, params domain.VerificationQueryParams) ([]*domain.VerificationEvent, int, *domain.VerificationStatusCounts, error) {
			return []*domain.VerificationEvent{}, 0, &domain.VerificationStatusCounts{}, nil
		},
	}

	handler := NewVerificationHandlerWithInterfaces(
		&MockAgentServiceForVerificationImpl{},
		&MockAuditServiceForVerificationImpl{},
		&MockAlertServiceForVerificationImpl{},
		mockVerifEventService,
		&MockOrganizationRepositoryerImpl{},
	)

	app := setupVerificationTestApp(handler)

	// Act
	req := httptest.NewRequest("GET", "/api/v1/admin/verifications/pending", nil)
	resp, err := app.Test(req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestVerificationHandler_ListPendingVerifications_WithPagination(t *testing.T) {
	// Arrange
	mockVerifEventService := &MockVerificationEventServiceForVerificationImpl{
		SearchVerificationsFunc: func(ctx context.Context, orgID uuid.UUID, params domain.VerificationQueryParams) ([]*domain.VerificationEvent, int, *domain.VerificationStatusCounts, error) {
			// Verify pagination params
			assert.Equal(t, 5, params.Limit)
			assert.Equal(t, 10, params.Offset) // page 3 with page_size 5 = offset 10
			return []*domain.VerificationEvent{}, 25, &domain.VerificationStatusCounts{}, nil
		},
	}

	handler := NewVerificationHandlerWithInterfaces(
		&MockAgentServiceForVerificationImpl{},
		&MockAuditServiceForVerificationImpl{},
		&MockAlertServiceForVerificationImpl{},
		mockVerifEventService,
		&MockOrganizationRepositoryerImpl{},
	)

	app := setupVerificationTestApp(handler)

	// Act
	req := httptest.NewRequest("GET", "/api/v1/admin/verifications/pending?page=3&page_size=5", nil)
	resp, err := app.Test(req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var response PendingVerificationListResponse
	body, _ := io.ReadAll(resp.Body)
	err = json.Unmarshal(body, &response)
	require.NoError(t, err)

	assert.Equal(t, 3, response.Pagination.Page)
	assert.Equal(t, 5, response.Pagination.PageSize)
	assert.Equal(t, 25, response.Pagination.Total)
	assert.Equal(t, 5, response.Pagination.TotalPages)
}

func TestVerificationHandler_ListPendingVerifications_ServiceError(t *testing.T) {
	// Arrange
	mockVerifEventService := &MockVerificationEventServiceForVerificationImpl{
		SearchVerificationsFunc: func(ctx context.Context, orgID uuid.UUID, params domain.VerificationQueryParams) ([]*domain.VerificationEvent, int, *domain.VerificationStatusCounts, error) {
			return nil, 0, nil, fmt.Errorf("database error")
		},
	}

	handler := NewVerificationHandlerWithInterfaces(
		&MockAgentServiceForVerificationImpl{},
		&MockAuditServiceForVerificationImpl{},
		&MockAlertServiceForVerificationImpl{},
		mockVerifEventService,
		&MockOrganizationRepositoryerImpl{},
	)

	app := setupVerificationTestApp(handler)

	// Act
	req := httptest.NewRequest("GET", "/api/v1/admin/verifications/pending", nil)
	resp, err := app.Test(req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

// ===========================
// ApproveVerification Tests
// ===========================

func TestVerificationHandler_ApproveVerification_Success(t *testing.T) {
	// Arrange
	verificationID := uuid.New()

	mockVerifEventService := &MockVerificationEventServiceForVerificationImpl{
		UpdateVerificationResultFunc: func(ctx context.Context, id uuid.UUID, result domain.VerificationResult, reason *string, metadata map[string]interface{}) error {
			assert.Equal(t, verificationID, id)
			assert.Equal(t, domain.VerificationResultVerified, result)
			assert.Contains(t, metadata, "approvedBy")
			assert.Contains(t, metadata, "manualApproval")
			return nil
		},
	}

	mockAuditService := &MockAuditServiceForVerificationImpl{
		LogFunc: func(ctx context.Context, entry *domain.AuditLog) error {
			return nil
		},
	}

	handler := NewVerificationHandlerWithInterfaces(
		&MockAgentServiceForVerificationImpl{},
		mockAuditService,
		&MockAlertServiceForVerificationImpl{},
		mockVerifEventService,
		&MockOrganizationRepositoryerImpl{},
	)

	app := setupVerificationTestApp(handler)

	reqBody := map[string]interface{}{
		"reason": "Approved after review",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	// Act
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/admin/verifications/%s/approve", verificationID.String()), bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var response map[string]interface{}
	body, _ := io.ReadAll(resp.Body)
	err = json.Unmarshal(body, &response)
	require.NoError(t, err)

	assert.Equal(t, verificationID.String(), response["id"])
	assert.Equal(t, "approved", response["status"])
	assert.Equal(t, "test-admin", response["approvedBy"])
}

func TestVerificationHandler_ApproveVerification_NotFound(t *testing.T) {
	// Arrange
	verificationID := uuid.New()

	mockVerifEventService := &MockVerificationEventServiceForVerificationImpl{
		UpdateVerificationResultFunc: func(ctx context.Context, id uuid.UUID, result domain.VerificationResult, reason *string, metadata map[string]interface{}) error {
			return fmt.Errorf("not found")
		},
	}

	handler := NewVerificationHandlerWithInterfaces(
		&MockAgentServiceForVerificationImpl{},
		&MockAuditServiceForVerificationImpl{},
		&MockAlertServiceForVerificationImpl{},
		mockVerifEventService,
		&MockOrganizationRepositoryerImpl{},
	)

	app := setupVerificationTestApp(handler)

	// Act
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/admin/verifications/%s/approve", verificationID.String()), nil)
	resp, err := app.Test(req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

func TestVerificationHandler_ApproveVerification_InvalidIDWithMocks(t *testing.T) {
	// Arrange
	handler := NewVerificationHandlerWithInterfaces(
		&MockAgentServiceForVerificationImpl{},
		&MockAuditServiceForVerificationImpl{},
		&MockAlertServiceForVerificationImpl{},
		&MockVerificationEventServiceForVerificationImpl{},
		&MockOrganizationRepositoryerImpl{},
	)

	app := setupVerificationTestApp(handler)

	// Act
	req := httptest.NewRequest("POST", "/api/v1/admin/verifications/invalid-uuid/approve", nil)
	resp, err := app.Test(req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// DenyVerification Tests
// ===========================

func TestVerificationHandler_DenyVerification_Success(t *testing.T) {
	// Arrange
	verificationID := uuid.New()

	mockVerifEventService := &MockVerificationEventServiceForVerificationImpl{
		UpdateVerificationResultFunc: func(ctx context.Context, id uuid.UUID, result domain.VerificationResult, reason *string, metadata map[string]interface{}) error {
			assert.Equal(t, verificationID, id)
			assert.Equal(t, domain.VerificationResultDenied, result)
			assert.NotNil(t, reason)
			assert.Equal(t, "Security risk detected", *reason)
			assert.Contains(t, metadata, "deniedBy")
			assert.Contains(t, metadata, "manualDenial")
			return nil
		},
	}

	mockAuditService := &MockAuditServiceForVerificationImpl{
		LogFunc: func(ctx context.Context, entry *domain.AuditLog) error {
			return nil
		},
	}

	handler := NewVerificationHandlerWithInterfaces(
		&MockAgentServiceForVerificationImpl{},
		mockAuditService,
		&MockAlertServiceForVerificationImpl{},
		mockVerifEventService,
		&MockOrganizationRepositoryerImpl{},
	)

	app := setupVerificationTestApp(handler)

	reqBody := map[string]interface{}{
		"reason": "Security risk detected",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	// Act
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/admin/verifications/%s/deny", verificationID.String()), bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var response map[string]interface{}
	body, _ := io.ReadAll(resp.Body)
	err = json.Unmarshal(body, &response)
	require.NoError(t, err)

	assert.Equal(t, verificationID.String(), response["id"])
	assert.Equal(t, "denied", response["status"])
	assert.Equal(t, "test-admin", response["deniedBy"])
	assert.Equal(t, "Security risk detected", response["denialReason"])
}

func TestVerificationHandler_DenyVerification_MissingReason(t *testing.T) {
	// Arrange
	verificationID := uuid.New()

	handler := NewVerificationHandlerWithInterfaces(
		&MockAgentServiceForVerificationImpl{},
		&MockAuditServiceForVerificationImpl{},
		&MockAlertServiceForVerificationImpl{},
		&MockVerificationEventServiceForVerificationImpl{},
		&MockOrganizationRepositoryerImpl{},
	)

	app := setupVerificationTestApp(handler)

	reqBody := map[string]interface{}{
		"reason": "",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	// Act
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/admin/verifications/%s/deny", verificationID.String()), bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestVerificationHandler_DenyVerification_NotFound(t *testing.T) {
	// Arrange
	verificationID := uuid.New()

	mockVerifEventService := &MockVerificationEventServiceForVerificationImpl{
		UpdateVerificationResultFunc: func(ctx context.Context, id uuid.UUID, result domain.VerificationResult, reason *string, metadata map[string]interface{}) error {
			return fmt.Errorf("not found")
		},
	}

	handler := NewVerificationHandlerWithInterfaces(
		&MockAgentServiceForVerificationImpl{},
		&MockAuditServiceForVerificationImpl{},
		&MockAlertServiceForVerificationImpl{},
		mockVerifEventService,
		&MockOrganizationRepositoryerImpl{},
	)

	app := setupVerificationTestApp(handler)

	reqBody := map[string]interface{}{
		"reason": "Some reason",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	// Act
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/admin/verifications/%s/deny", verificationID.String()), bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

// ===========================
// UpdateExecutionStatus Tests
// ===========================

func TestVerificationHandler_UpdateExecutionStatus_Success(t *testing.T) {
	// Arrange
	verificationID := uuid.New()

	mockVerifEventService := &MockVerificationEventServiceForVerificationImpl{
		UpdateExecutionStatusFunc: func(ctx context.Context, id uuid.UUID, executed bool, strictMode bool, executedAt time.Time, executionError *string) error {
			assert.Equal(t, verificationID, id)
			assert.True(t, executed)
			assert.True(t, strictMode)
			return nil
		},
	}

	handler := NewVerificationHandlerWithInterfaces(
		&MockAgentServiceForVerificationImpl{},
		&MockAuditServiceForVerificationImpl{},
		&MockAlertServiceForVerificationImpl{},
		mockVerifEventService,
		&MockOrganizationRepositoryerImpl{},
	)

	app := setupVerificationTestApp(handler)

	now := time.Now().Format(time.RFC3339)
	reqBody := map[string]interface{}{
		"executed":   true,
		"strictMode": true,
		"executedAt": now,
	}
	bodyBytes, _ := json.Marshal(reqBody)

	// Act
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/sdk-api/verifications/%s/execution-status", verificationID.String()), bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var response map[string]interface{}
	body, _ := io.ReadAll(resp.Body)
	err = json.Unmarshal(body, &response)
	require.NoError(t, err)

	assert.Equal(t, verificationID.String(), response["id"])
	assert.Equal(t, "execution_status_recorded", response["status"])
	assert.Equal(t, true, response["executed"])
	assert.Equal(t, true, response["strictMode"])
}

func TestVerificationHandler_UpdateExecutionStatus_NotExecuted(t *testing.T) {
	// Arrange
	verificationID := uuid.New()

	mockVerifEventService := &MockVerificationEventServiceForVerificationImpl{
		UpdateExecutionStatusFunc: func(ctx context.Context, id uuid.UUID, executed bool, strictMode bool, executedAt time.Time, executionError *string) error {
			assert.False(t, executed)
			assert.NotNil(t, executionError)
			return nil
		},
	}

	handler := NewVerificationHandlerWithInterfaces(
		&MockAgentServiceForVerificationImpl{},
		&MockAuditServiceForVerificationImpl{},
		&MockAlertServiceForVerificationImpl{},
		mockVerifEventService,
		&MockOrganizationRepositoryerImpl{},
	)

	app := setupVerificationTestApp(handler)

	execError := "Permission denied"
	reqBody := map[string]interface{}{
		"executed":       false,
		"strictMode":     true,
		"executionError": execError,
	}
	bodyBytes, _ := json.Marshal(reqBody)

	// Act
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/sdk-api/verifications/%s/execution-status", verificationID.String()), bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestVerificationHandler_UpdateExecutionStatus_NotFound(t *testing.T) {
	// Arrange
	verificationID := uuid.New()

	mockVerifEventService := &MockVerificationEventServiceForVerificationImpl{
		UpdateExecutionStatusFunc: func(ctx context.Context, id uuid.UUID, executed bool, strictMode bool, executedAt time.Time, executionError *string) error {
			return fmt.Errorf("not found")
		},
	}

	handler := NewVerificationHandlerWithInterfaces(
		&MockAgentServiceForVerificationImpl{},
		&MockAuditServiceForVerificationImpl{},
		&MockAlertServiceForVerificationImpl{},
		mockVerifEventService,
		&MockOrganizationRepositoryerImpl{},
	)

	app := setupVerificationTestApp(handler)

	reqBody := map[string]interface{}{
		"executed":   true,
		"strictMode": true,
	}
	bodyBytes, _ := json.Marshal(reqBody)

	// Act
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/sdk-api/verifications/%s/execution-status", verificationID.String()), bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

func TestVerificationHandler_UpdateExecutionStatus_InvalidIDWithMocks(t *testing.T) {
	// Arrange
	handler := NewVerificationHandlerWithInterfaces(
		&MockAgentServiceForVerificationImpl{},
		&MockAuditServiceForVerificationImpl{},
		&MockAlertServiceForVerificationImpl{},
		&MockVerificationEventServiceForVerificationImpl{},
		&MockOrganizationRepositoryerImpl{},
	)

	app := setupVerificationTestApp(handler)

	reqBody := map[string]interface{}{
		"executed":   true,
		"strictMode": true,
	}
	bodyBytes, _ := json.Marshal(reqBody)

	// Act
	req := httptest.NewRequest("POST", "/api/v1/sdk-api/verifications/invalid-uuid/execution-status", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// Helper Function Tests
// ===========================

func TestVerificationHandler_determineAlertSeverity(t *testing.T) {
	handler := &VerificationHandler{}

	tests := []struct {
		name       string
		actionType string
		context    map[string]interface{}
		riskLevel  string
		expected   domain.AlertSeverity
	}{
		{
			name:       "explicit critical risk level",
			actionType: "any_action",
			riskLevel:  "critical",
			expected:   domain.AlertSeverityCritical,
		},
		{
			name:       "explicit high risk level",
			actionType: "any_action",
			riskLevel:  "high",
			expected:   domain.AlertSeverityHigh,
		},
		{
			name:       "delete action is critical",
			actionType: "delete_data",
			riskLevel:  "",
			expected:   domain.AlertSeverityCritical,
		},
		{
			name:       "write action is high",
			actionType: "write_database",
			riskLevel:  "",
			expected:   domain.AlertSeverityHigh,
		},
		{
			name:       "read action is warning",
			actionType: "read_file",
			riskLevel:  "",
			expected:   domain.AlertSeverityWarning,
		},
		{
			name:       "unknown action is info",
			actionType: "unknown_action",
			riskLevel:  "",
			expected:   domain.AlertSeverityInfo,
		},
		{
			name:       "context risk level overrides",
			actionType: "read_file",
			context:    map[string]interface{}{"risk_level": "critical"},
			riskLevel:  "",
			expected:   domain.AlertSeverityCritical,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.determineAlertSeverity(tt.actionType, tt.context, tt.riskLevel)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestVerificationHandler_calculateVerificationConfidence(t *testing.T) {
	handler := &VerificationHandler{}

	tests := []struct {
		name              string
		agent             *domain.Agent
		status            string
		signatureVerified bool
		publicKeyMatched  bool
		expectedMin       float64
		expectedMax       float64
	}{
		{
			name:              "all verified, high trust",
			agent:             &domain.Agent{Status: domain.AgentStatusVerified, TrustScore: 1.0},
			status:            "approved",
			signatureVerified: true,
			publicKeyMatched:  true,
			expectedMin:       0.95,
			expectedMax:       1.0,
		},
		{
			name:              "signature only, low trust",
			agent:             &domain.Agent{Status: domain.AgentStatusVerified, TrustScore: 0.5},
			status:            "approved",
			signatureVerified: true,
			publicKeyMatched:  false,
			expectedMin:       0.5,
			expectedMax:       0.7,
		},
		{
			name:              "denied reduces confidence",
			agent:             &domain.Agent{Status: domain.AgentStatusVerified, TrustScore: 1.0},
			status:            "denied",
			signatureVerified: true,
			publicKeyMatched:  true,
			expectedMin:       0.4,
			expectedMax:       0.6,
		},
		{
			name:              "pending agent lower bonus",
			agent:             &domain.Agent{Status: domain.AgentStatusPending, TrustScore: 0.8},
			status:            "approved",
			signatureVerified: true,
			publicKeyMatched:  true,
			expectedMin:       0.7,
			expectedMax:       0.9,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.calculateVerificationConfidence(tt.agent, tt.status, tt.signatureVerified, tt.publicKeyMatched)
			assert.GreaterOrEqual(t, result, tt.expectedMin)
			assert.LessOrEqual(t, result, tt.expectedMax)
		})
	}
}

func TestIsLowRiskCapability(t *testing.T) {
	tests := []struct {
		capability string
		expected   bool
	}{
		{"db:read", true},
		{"file:read", true},
		{"weather:check", true},
		{"products:search", true},
		{"read_database", true},
		{"delete_data", false},
		{"execute_command", false},
		{"admin_action", false},
		{"unknown_action", false},
	}

	for _, tt := range tests {
		t.Run(tt.capability, func(t *testing.T) {
			result := isLowRiskCapability(tt.capability)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsDemoHighRiskCapability(t *testing.T) {
	tests := []struct {
		capability string
		expected   bool
	}{
		{"notification:send", true},
		{"refund:process", true},
		{"send_notification", true},
		{"process_refund", true},
		{"delete_data", false},
		{"read_file", false},
	}

	for _, tt := range tests {
		t.Run(tt.capability, func(t *testing.T) {
			result := isDemoHighRiskCapability(tt.capability)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNormalizeVerificationStatus(t *testing.T) {
	tests := []struct {
		status   domain.VerificationEventStatus
		expected string
	}{
		{domain.VerificationEventStatusPending, "pending"},
		{domain.VerificationEventStatusSuccess, "approved"},
		{domain.VerificationEventStatusFailed, "denied"},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			result := normalizeVerificationStatus(tt.status)
			assert.Equal(t, tt.expected, result)
		})
	}
}
