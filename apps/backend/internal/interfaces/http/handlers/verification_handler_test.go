package handlers

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===========================
// NewVerificationHandler Tests
// ===========================

func TestNewVerificationHandler_NilDeps(t *testing.T) {
	handler := NewVerificationHandler(nil, nil, nil, nil, nil, nil)
	assert.NotNil(t, handler)
}

// ===========================
// VerificationHandler.CreateVerification Tests
// ===========================

func TestVerificationHandler_CreateVerification_InvalidJSON(t *testing.T) {
	handler := &VerificationHandler{}
	app := fiber.New()
	app.Post("/verifications", handler.CreateVerification)

	req := httptest.NewRequest("POST", "/verifications", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestVerificationHandler_CreateVerification_MissingAgentID(t *testing.T) {
	handler := &VerificationHandler{}
	app := fiber.New()
	app.Post("/verifications", handler.CreateVerification)

	body := `{"capability":"read","signature":"sig","publicKey":"pk"}`
	req := httptest.NewRequest("POST", "/verifications", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestVerificationHandler_CreateVerification_MissingCapability(t *testing.T) {
	handler := &VerificationHandler{}
	app := fiber.New()
	app.Post("/verifications", handler.CreateVerification)

	body := `{"agentId":"` + uuid.New().String() + `","signature":"sig","publicKey":"pk"}`
	req := httptest.NewRequest("POST", "/verifications", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestVerificationHandler_CreateVerification_MissingSignature(t *testing.T) {
	handler := &VerificationHandler{}
	app := fiber.New()
	app.Post("/verifications", handler.CreateVerification)

	body := `{"agentId":"` + uuid.New().String() + `","capability":"read","publicKey":"pk"}`
	req := httptest.NewRequest("POST", "/verifications", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestVerificationHandler_CreateVerification_MissingPublicKey(t *testing.T) {
	handler := &VerificationHandler{}
	app := fiber.New()
	app.Post("/verifications", handler.CreateVerification)

	body := `{"agentId":"` + uuid.New().String() + `","capability":"read","signature":"sig"}`
	req := httptest.NewRequest("POST", "/verifications", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestVerificationHandler_CreateVerification_InvalidAgentID(t *testing.T) {
	handler := &VerificationHandler{}
	app := fiber.New()
	app.Post("/verifications", handler.CreateVerification)

	body := `{"agentId":"not-a-uuid","capability":"read","signature":"sig","publicKey":"pk"}`
	req := httptest.NewRequest("POST", "/verifications", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// VerificationHandler.GetVerification Tests
// ===========================

func TestVerificationHandler_GetVerification_InvalidID(t *testing.T) {
	handler := &VerificationHandler{}
	app := fiber.New()
	app.Get("/verifications/:id", handler.GetVerification)

	req := httptest.NewRequest("GET", "/verifications/not-a-uuid", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// VerificationHandler.SubmitVerificationResult Tests
// ===========================

func TestVerificationHandler_SubmitVerificationResult_InvalidID(t *testing.T) {
	handler := &VerificationHandler{}
	app := fiber.New()
	app.Post("/verifications/:id/result", handler.SubmitVerificationResult)

	body := `{"result":"success"}`
	req := httptest.NewRequest("POST", "/verifications/not-a-uuid/result", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestVerificationHandler_SubmitVerificationResult_InvalidJSON(t *testing.T) {
	handler := &VerificationHandler{}
	app := fiber.New()
	app.Post("/verifications/:id/result", handler.SubmitVerificationResult)

	req := httptest.NewRequest("POST", "/verifications/"+uuid.New().String()+"/result", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestVerificationHandler_SubmitVerificationResult_InvalidResult(t *testing.T) {
	handler := &VerificationHandler{}
	app := fiber.New()
	app.Post("/verifications/:id/result", handler.SubmitVerificationResult)

	body := `{"result":"invalid_result"}`
	req := httptest.NewRequest("POST", "/verifications/"+uuid.New().String()+"/result", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// VerificationHandler.ListPendingVerifications Tests
// ===========================

func TestVerificationHandler_ListPendingVerifications_NoOrgContext(t *testing.T) {
	handler := &VerificationHandler{}
	app := fiber.New()
	app.Get("/admin/verifications/pending", handler.ListPendingVerifications)

	req := httptest.NewRequest("GET", "/admin/verifications/pending", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

// ===========================
// VerificationHandler.ApproveVerification Tests
// ===========================

func TestVerificationHandler_ApproveVerification_InvalidID(t *testing.T) {
	handler := &VerificationHandler{}
	app := fiber.New()
	app.Post("/admin/verifications/:id/approve", handler.ApproveVerification)

	req := httptest.NewRequest("POST", "/admin/verifications/not-a-uuid/approve", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// VerificationHandler.DenyVerification Tests
// ===========================

func TestVerificationHandler_DenyVerification_InvalidID(t *testing.T) {
	handler := &VerificationHandler{}
	app := fiber.New()
	app.Post("/admin/verifications/:id/deny", handler.DenyVerification)

	body := `{"reason":"test reason"}`
	req := httptest.NewRequest("POST", "/admin/verifications/not-a-uuid/deny", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestVerificationHandler_DenyVerification_InvalidJSON(t *testing.T) {
	handler := &VerificationHandler{}
	app := fiber.New()
	app.Post("/admin/verifications/:id/deny", handler.DenyVerification)

	req := httptest.NewRequest("POST", "/admin/verifications/"+uuid.New().String()+"/deny", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestVerificationHandler_DenyVerification_EmptyReason(t *testing.T) {
	handler := &VerificationHandler{}
	app := fiber.New()
	app.Post("/admin/verifications/:id/deny", handler.DenyVerification)

	body := `{"reason":""}`
	req := httptest.NewRequest("POST", "/admin/verifications/"+uuid.New().String()+"/deny", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// VerificationHandler.UpdateExecutionStatus Tests
// ===========================

func TestVerificationHandler_UpdateExecutionStatus_InvalidID(t *testing.T) {
	handler := &VerificationHandler{}
	app := fiber.New()
	app.Post("/verifications/:id/execution-status", handler.UpdateExecutionStatus)

	body := `{"executed":true}`
	req := httptest.NewRequest("POST", "/verifications/not-a-uuid/execution-status", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestVerificationHandler_UpdateExecutionStatus_InvalidJSON(t *testing.T) {
	handler := &VerificationHandler{}
	app := fiber.New()
	app.Post("/verifications/:id/execution-status", handler.UpdateExecutionStatus)

	req := httptest.NewRequest("POST", "/verifications/"+uuid.New().String()+"/execution-status", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// VerificationHandler Helper Function Tests
// ===========================

func TestVerificationHandler_isLowRiskCapability(t *testing.T) {
	tests := []struct {
		name       string
		capability string
		expected   bool
	}{
		{"db:read", "db:read", true},
		{"file:read", "file:read", true},
		{"api:read", "api:read", true},
		{"weather:check", "weather:check", true},
		{"products:search", "products:search", true},
		{"read_database", "read_database", true},
		{"read_file", "read_file", true},
		{"query_api", "query_api", true},
		{"delete_data", "delete_data", false},
		{"write_file", "write_file", false},
		{"execute_command", "execute_command", false},
		{"unknown_capability", "unknown_capability", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isLowRiskCapability(tt.capability)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestVerificationHandler_isDemoHighRiskCapability(t *testing.T) {
	tests := []struct {
		name       string
		capability string
		expected   bool
	}{
		{"notification:send", "notification:send", true},
		{"refund:process", "refund:process", true},
		{"send_notification", "send_notification", true},
		{"process_refund", "process_refund", true},
		{"delete_data", "delete_data", false},
		{"read_file", "read_file", false},
		{"unknown", "unknown", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isDemoHighRiskCapability(tt.capability)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestVerificationHandler_normalizeVerificationStatus(t *testing.T) {
	tests := []struct {
		name     string
		status   string
		expected string
	}{
		{"pending", "pending", "pending"},
		{"success", "success", "approved"},
		{"failed", "failed", "denied"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Import the domain status type and use it
			var domainStatus domain.VerificationEventStatus
			switch tt.status {
			case "pending":
				domainStatus = domain.VerificationEventStatusPending
			case "success":
				domainStatus = domain.VerificationEventStatusSuccess
			case "failed":
				domainStatus = domain.VerificationEventStatusFailed
			}
			result := normalizeVerificationStatus(domainStatus)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestVerificationHandler_getCapabilityRiskAdjustment(t *testing.T) {
	handler := &VerificationHandler{}

	tests := []struct {
		name       string
		capability string
		expected   float64
	}{
		{"read_database", "read_database", 1.0},
		{"read_file", "read_file", 1.0},
		{"query_api", "query_api", 1.0},
		{"write_database", "write_database", 0.8},
		{"write_file", "write_file", 0.8},
		{"send_email", "send_email", 0.8},
		{"modify_config", "modify_config", 0.7},
		{"delete_data", "delete_data", 0.5},
		{"delete_file", "delete_file", 0.5},
		{"execute_command", "execute_command", 0.3},
		{"admin_action", "admin_action", 0.3},
		{"unknown_capability", "unknown_capability", 0.8}, // default
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.getCapabilityRiskAdjustment(tt.capability)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestVerificationHandler_customJSONFormat(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple key:value", `{"key":"value"}`, `{"key": "value"}`},
		{"comma separated", `{"a":"1","b":"2"}`, `{"a": "1", "b": "2"}`},
		{"nested object", `{"a":{"b":"c"}}`, `{"a": {"b": "c"}}`},
		{"with spaces in string", `{"key":"value with spaces"}`, `{"key": "value with spaces"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := customJSONFormat(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ===========================
// VerificationHandler.ApproveVerification Additional Tests
// ===========================

func TestVerificationHandler_ApproveVerification_EmptyID(t *testing.T) {
	handler := &VerificationHandler{}
	app := fiber.New()
	// Test with empty path - this is a different route so not directly testable
	// Instead, test the handler's internal empty check by using a pattern that allows empty
	app.Post("/admin/verifications/approve", func(c fiber.Ctx) error {
		return handler.ApproveVerification(c)
	})

	req := httptest.NewRequest("POST", "/admin/verifications/approve", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// VerificationHandler.DenyVerification Additional Tests
// ===========================

func TestVerificationHandler_DenyVerification_EmptyID(t *testing.T) {
	handler := &VerificationHandler{}
	app := fiber.New()
	app.Post("/admin/verifications/deny", func(c fiber.Ctx) error {
		return handler.DenyVerification(c)
	})

	req := httptest.NewRequest("POST", "/admin/verifications/deny", strings.NewReader(`{"reason":"test"}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// VerificationHandler.GetVerification Additional Tests
// ===========================

func TestVerificationHandler_GetVerification_EmptyID(t *testing.T) {
	handler := &VerificationHandler{}
	app := fiber.New()
	app.Get("/verifications", func(c fiber.Ctx) error {
		return handler.GetVerification(c)
	})

	req := httptest.NewRequest("GET", "/verifications", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// VerificationHandler.SubmitVerificationResult Additional Tests
// ===========================

func TestVerificationHandler_SubmitVerificationResult_EmptyID(t *testing.T) {
	handler := &VerificationHandler{}
	app := fiber.New()
	app.Post("/verifications/result", func(c fiber.Ctx) error {
		return handler.SubmitVerificationResult(c)
	})

	req := httptest.NewRequest("POST", "/verifications/result", strings.NewReader(`{"result":"success"}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// VerificationHandler.UpdateExecutionStatus Additional Tests
// ===========================

func TestVerificationHandler_UpdateExecutionStatus_EmptyID(t *testing.T) {
	handler := &VerificationHandler{}
	app := fiber.New()
	app.Post("/verifications/execution-status", func(c fiber.Ctx) error {
		return handler.UpdateExecutionStatus(c)
	})

	req := httptest.NewRequest("POST", "/verifications/execution-status", strings.NewReader(`{"executed":true}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// Struct Tests
// ===========================

func TestVerificationRequest_Struct(t *testing.T) {
	req := VerificationRequest{
		AgentID:    "agent-123",
		Capability: "file:read",
		Resource:   "/path/to/file",
		Context:    map[string]interface{}{"key": "value"},
		Timestamp:  "2024-01-01T00:00:00Z",
		RiskLevel:  "low",
		Signature:  "base64signature",
		PublicKey:  "base64publickey",
	}

	assert.Equal(t, "agent-123", req.AgentID)
	assert.Equal(t, "file:read", req.Capability)
	assert.Equal(t, "/path/to/file", req.Resource)
	assert.NotNil(t, req.Context)
	assert.Equal(t, "2024-01-01T00:00:00Z", req.Timestamp)
	assert.Equal(t, "low", req.RiskLevel)
	assert.Equal(t, "base64signature", req.Signature)
	assert.Equal(t, "base64publickey", req.PublicKey)
}

func TestVerificationRequest_EmptyStruct(t *testing.T) {
	req := VerificationRequest{}

	assert.Empty(t, req.AgentID)
	assert.Empty(t, req.Capability)
	assert.Empty(t, req.Resource)
	assert.Nil(t, req.Context)
	assert.Empty(t, req.Timestamp)
	assert.Empty(t, req.RiskLevel)
	assert.Empty(t, req.Signature)
	assert.Empty(t, req.PublicKey)
}

func TestVerificationResponse_Struct(t *testing.T) {
	resp := VerificationResponse{
		ID:               "verification-123",
		Status:           "approved",
		ApprovedBy:       "admin-user",
		DenialReason:     "",
		TrustScore:       0.95,
		EnforcementMode:  "strict",
		RiskLevel:        "low",
		RiskAutoDetected: true,
	}

	assert.Equal(t, "verification-123", resp.ID)
	assert.Equal(t, "approved", resp.Status)
	assert.Equal(t, "admin-user", resp.ApprovedBy)
	assert.Empty(t, resp.DenialReason)
	assert.Equal(t, 0.95, resp.TrustScore)
	assert.Equal(t, "strict", resp.EnforcementMode)
	assert.Equal(t, "low", resp.RiskLevel)
	assert.True(t, resp.RiskAutoDetected)
}

func TestVerificationResponse_Denied(t *testing.T) {
	resp := VerificationResponse{
		ID:              "verification-456",
		Status:          "denied",
		DenialReason:    "Insufficient trust score",
		TrustScore:      0.3,
		EnforcementMode: "monitoring",
		RiskLevel:       "high",
	}

	assert.Equal(t, "denied", resp.Status)
	assert.Equal(t, "Insufficient trust score", resp.DenialReason)
	assert.Equal(t, 0.3, resp.TrustScore)
	assert.Equal(t, "monitoring", resp.EnforcementMode)
	assert.Equal(t, "high", resp.RiskLevel)
}

func TestPendingVerificationResponse_Struct(t *testing.T) {
	resp := PendingVerificationResponse{
		ID:         "pending-123",
		AgentID:    "agent-456",
		AgentName:  "Test Agent",
		ActionType: "file:write",
		Resource:   "/path/to/resource",
		Context:    map[string]interface{}{"reason": "backup"},
		RiskLevel:  "medium",
		TrustScore: 0.7,
		Status:     "pending",
	}

	assert.Equal(t, "pending-123", resp.ID)
	assert.Equal(t, "agent-456", resp.AgentID)
	assert.Equal(t, "Test Agent", resp.AgentName)
	assert.Equal(t, "file:write", resp.ActionType)
	assert.Equal(t, "/path/to/resource", resp.Resource)
	assert.NotNil(t, resp.Context)
	assert.Equal(t, "medium", resp.RiskLevel)
	assert.Equal(t, 0.7, resp.TrustScore)
	assert.Equal(t, "pending", resp.Status)
}

func TestPendingVerificationListResponse_Struct(t *testing.T) {
	resp := PendingVerificationListResponse{
		Verifications: []PendingVerificationResponse{
			{ID: "v1", AgentID: "a1", Status: "pending"},
			{ID: "v2", AgentID: "a2", Status: "pending"},
		},
	}
	resp.Pagination.Page = 1
	resp.Pagination.PageSize = 10
	resp.Pagination.Total = 2
	resp.Pagination.TotalPages = 1

	assert.Len(t, resp.Verifications, 2)
	assert.Equal(t, 1, resp.Pagination.Page)
	assert.Equal(t, 10, resp.Pagination.PageSize)
	assert.Equal(t, 2, resp.Pagination.Total)
	assert.Equal(t, 1, resp.Pagination.TotalPages)
}

func TestApproveVerificationRequest_Struct(t *testing.T) {
	req := ApproveVerificationRequest{
		Reason: "Approved by security team after review",
	}

	assert.Equal(t, "Approved by security team after review", req.Reason)
}

func TestApproveVerificationRequest_EmptyReason(t *testing.T) {
	req := ApproveVerificationRequest{}

	assert.Empty(t, req.Reason)
}

func TestDenyVerificationRequest_Struct(t *testing.T) {
	req := DenyVerificationRequest{
		Reason: "Agent not authorized for this action",
	}

	assert.Equal(t, "Agent not authorized for this action", req.Reason)
}

func TestUpdateExecutionStatusRequest_Struct(t *testing.T) {
	errorMsg := "Connection timeout"
	req := UpdateExecutionStatusRequest{
		Executed:       false,
		StrictMode:     true,
		ExecutedAt:     "2024-01-01T12:00:00Z",
		ExecutionError: &errorMsg,
	}

	assert.False(t, req.Executed)
	assert.True(t, req.StrictMode)
	assert.Equal(t, "2024-01-01T12:00:00Z", req.ExecutedAt)
	assert.NotNil(t, req.ExecutionError)
	assert.Equal(t, "Connection timeout", *req.ExecutionError)
}

func TestUpdateExecutionStatusRequest_Successful(t *testing.T) {
	req := UpdateExecutionStatusRequest{
		Executed:       true,
		StrictMode:     false,
		ExecutedAt:     "2024-01-01T12:00:00Z",
		ExecutionError: nil,
	}

	assert.True(t, req.Executed)
	assert.False(t, req.StrictMode)
	assert.Nil(t, req.ExecutionError)
}
