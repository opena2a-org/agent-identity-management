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

// Helper function for capability tests that need org/user context
func withCapabilityContext(handler func(c fiber.Ctx) error) fiber.Handler {
	return func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		c.Locals("user_id", uuid.New())
		return handler(c)
	}
}

// ===========================
// NewCapabilityHandler Tests
// ===========================

func TestNewCapabilityHandler_NilDeps(t *testing.T) {
	handler := NewCapabilityHandler(nil, nil, nil, nil)
	assert.NotNil(t, handler)
}

// ===========================
// CapabilityHandler.GrantCapability Tests
// ===========================

func TestCapabilityHandler_GrantCapability_InvalidAgentID(t *testing.T) {
	handler := &CapabilityHandler{}
	app := fiber.New()
	app.Post("/agents/:id/capabilities", withCapabilityContext(handler.GrantCapability))

	body := `{"capabilityType":"read:data"}`
	req := httptest.NewRequest("POST", "/agents/not-a-uuid/capabilities", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestCapabilityHandler_GrantCapability_InvalidJSON(t *testing.T) {
	handler := &CapabilityHandler{}
	app := fiber.New()
	app.Post("/agents/:id/capabilities", withCapabilityContext(handler.GrantCapability))

	req := httptest.NewRequest("POST", "/agents/"+uuid.New().String()+"/capabilities", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestCapabilityHandler_GrantCapability_NoOrgContext(t *testing.T) {
	handler := &CapabilityHandler{}
	app := fiber.New()
	// Only set user_id, not organization_id
	app.Post("/agents/:id/capabilities", func(c fiber.Ctx) error {
		c.Locals("user_id", uuid.New())
		return handler.GrantCapability(c)
	})

	body := `{"capabilityType":"read:data"}`
	req := httptest.NewRequest("POST", "/agents/"+uuid.New().String()+"/capabilities", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

// ===========================
// CapabilityHandler.RegisterCapability Tests
// ===========================

func TestCapabilityHandler_RegisterCapability_InvalidAgentID(t *testing.T) {
	handler := &CapabilityHandler{}
	app := fiber.New()
	app.Post("/agents/:id/capabilities/register", withCapabilityContext(handler.RegisterCapability))

	body := `{"capabilityType":"read:data"}`
	req := httptest.NewRequest("POST", "/agents/not-a-uuid/capabilities/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestCapabilityHandler_RegisterCapability_InvalidJSON(t *testing.T) {
	handler := &CapabilityHandler{}
	app := fiber.New()
	app.Post("/agents/:id/capabilities/register", withCapabilityContext(handler.RegisterCapability))

	req := httptest.NewRequest("POST", "/agents/"+uuid.New().String()+"/capabilities/register", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// CapabilityHandler.GetAgentCapabilities Tests
// ===========================

func TestCapabilityHandler_GetAgentCapabilities_InvalidAgentID(t *testing.T) {
	handler := &CapabilityHandler{}
	app := fiber.New()
	app.Get("/agents/:id/capabilities", withCapabilityContext(handler.GetAgentCapabilities))

	req := httptest.NewRequest("GET", "/agents/not-a-uuid/capabilities", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// CapabilityHandler.RevokeCapability Tests
// ===========================

func TestCapabilityHandler_RevokeCapability_InvalidAgentID(t *testing.T) {
	handler := &CapabilityHandler{}
	app := fiber.New()
	app.Delete("/agents/:id/capabilities/:capability_id", withCapabilityContext(handler.RevokeCapability))

	req := httptest.NewRequest("DELETE", "/agents/not-a-uuid/capabilities/"+uuid.New().String(), nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestCapabilityHandler_RevokeCapability_InvalidCapabilityID(t *testing.T) {
	handler := &CapabilityHandler{}
	app := fiber.New()
	app.Delete("/agents/:id/capabilities/:capability_id", withCapabilityContext(handler.RevokeCapability))

	req := httptest.NewRequest("DELETE", "/agents/"+uuid.New().String()+"/capabilities/not-a-uuid", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// CapabilityHandler.VerifyAction Tests
// ===========================

func TestCapabilityHandler_VerifyAction_InvalidJSON(t *testing.T) {
	handler := &CapabilityHandler{}
	app := fiber.New()
	app.Post("/agents/verify-action", withCapabilityContext(handler.VerifyAction))

	req := httptest.NewRequest("POST", "/agents/verify-action", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestCapabilityHandler_VerifyAction_EmptyFields(t *testing.T) {
	handler := &CapabilityHandler{}
	app := fiber.New()
	app.Post("/agents/verify-action", withCapabilityContext(handler.VerifyAction))

	body := `{"agentId":"","actionType":""}`
	req := httptest.NewRequest("POST", "/agents/verify-action", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestCapabilityHandler_VerifyAction_InvalidAgentID(t *testing.T) {
	handler := &CapabilityHandler{}
	app := fiber.New()
	app.Post("/agents/verify-action", withCapabilityContext(handler.VerifyAction))

	body := `{"agentId":"not-a-uuid","actionType":"read"}`
	req := httptest.NewRequest("POST", "/agents/verify-action", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// CapabilityHandler.GetViolationsByAgent Tests
// ===========================

func TestCapabilityHandler_GetViolationsByAgent_InvalidAgentID(t *testing.T) {
	handler := &CapabilityHandler{}
	app := fiber.New()
	app.Get("/agents/:id/violations", withCapabilityContext(handler.GetViolationsByAgent))

	req := httptest.NewRequest("GET", "/agents/not-a-uuid/violations", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// CapabilityHandler.ListCapabilities Tests
// ===========================

func TestCapabilityHandler_ListCapabilities_NoOrgContext(t *testing.T) {
	handler := &CapabilityHandler{}
	app := fiber.New()
	// No context set
	app.Get("/capabilities", handler.ListCapabilities)

	req := httptest.NewRequest("GET", "/capabilities", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

// ===========================
// CapabilityHandler.GrantCapability Additional Tests
// ===========================

func TestCapabilityHandler_GrantCapability_InvalidOrgIDType(t *testing.T) {
	handler := &CapabilityHandler{}
	app := fiber.New()
	// Set org_id as wrong type
	app.Post("/agents/:id/capabilities", func(c fiber.Ctx) error {
		c.Locals("organization_id", "not-a-uuid")
		c.Locals("user_id", uuid.New())
		return handler.GrantCapability(c)
	})

	body := `{"capabilityType":"read:data"}`
	req := httptest.NewRequest("POST", "/agents/"+uuid.New().String()+"/capabilities", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

func TestCapabilityHandler_GrantCapability_NoUserContext(t *testing.T) {
	handler := &CapabilityHandler{}
	app := fiber.New()
	// Set org_id but not user_id
	app.Post("/agents/:id/capabilities", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		// No user_id set
		return handler.GrantCapability(c)
	})

	body := `{"capabilityType":"read:data"}`
	req := httptest.NewRequest("POST", "/agents/"+uuid.New().String()+"/capabilities", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

// ===========================
// CapabilityHandler.RegisterCapability Additional Tests
// ===========================

func TestCapabilityHandler_RegisterCapability_EmptyCapabilityType(t *testing.T) {
	handler := &CapabilityHandler{}
	app := fiber.New()
	app.Post("/agents/:id/capabilities/register", withCapabilityContext(handler.RegisterCapability))

	body := `{"capabilityType":""}`
	req := httptest.NewRequest("POST", "/agents/"+uuid.New().String()+"/capabilities/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// CapabilityHandler.RevokeCapability Additional Tests
// ===========================

func TestCapabilityHandler_RevokeCapability_NoUserContext(t *testing.T) {
	handler := &CapabilityHandler{}
	app := fiber.New()
	// No user_id set
	app.Delete("/agents/:agentId/capabilities/:capabilityId", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		return handler.RevokeCapability(c)
	})

	req := httptest.NewRequest("DELETE", "/agents/"+uuid.New().String()+"/capabilities/"+uuid.New().String(), nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

// ===========================
// CapabilityHandler.VerifyAction Additional Tests
// ===========================

func TestCapabilityHandler_VerifyAction_InvalidSignatureEncoding(t *testing.T) {
	handler := &CapabilityHandler{}
	app := fiber.New()
	app.Post("/agents/verify-action", withCapabilityContext(handler.VerifyAction))

	// Valid agent ID but invalid signature encoding
	body := `{"agentId":"` + uuid.New().String() + `","signature":"not-valid-base64!!!","requestPayload":"dGVzdA==","requestedCapability":"read:data"}`
	req := httptest.NewRequest("POST", "/agents/verify-action", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestCapabilityHandler_VerifyAction_InvalidPayloadEncoding(t *testing.T) {
	handler := &CapabilityHandler{}
	app := fiber.New()
	app.Post("/agents/verify-action", withCapabilityContext(handler.VerifyAction))

	// Valid signature but invalid payload encoding
	body := `{"agentId":"` + uuid.New().String() + `","signature":"dGVzdA==","requestPayload":"not-valid-base64!!!","requestedCapability":"read:data"}`
	req := httptest.NewRequest("POST", "/agents/verify-action", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// CapabilityHandler.ListCapabilities Additional Tests
// ===========================

func TestCapabilityHandler_ListCapabilities_InvalidOrgIDType(t *testing.T) {
	handler := &CapabilityHandler{}
	app := fiber.New()
	app.Get("/capabilities", func(c fiber.Ctx) error {
		c.Locals("organization_id", "not-a-uuid") // Wrong type
		return handler.ListCapabilities(c)
	})

	req := httptest.NewRequest("GET", "/capabilities", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

// ===========================
// CapabilityHandler.getUserIDFromContext Tests
// ===========================

func TestCapabilityHandler_getUserIDFromContext_APIKeyAuth(t *testing.T) {
	handler := &CapabilityHandler{}
	app := fiber.New()
	app.Get("/test", func(c fiber.Ctx) error {
		c.Locals("auth_method", "api_key")
		c.Locals("organization_id", uuid.New())
		userID, err := handler.getUserIDFromContext(c)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
		}
		// For API key auth, should return uuid.Nil (not an error)
		return c.JSON(fiber.Map{"user_id": userID.String()})
	})

	req := httptest.NewRequest("GET", "/test", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestCapabilityHandler_getUserIDFromContext_InvalidUserIDType(t *testing.T) {
	handler := &CapabilityHandler{}
	app := fiber.New()
	app.Get("/test", func(c fiber.Ctx) error {
		c.Locals("user_id", "not-a-uuid") // Wrong type
		userID, err := handler.getUserIDFromContext(c)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
		}
		return c.JSON(fiber.Map{"user_id": userID.String()})
	})

	req := httptest.NewRequest("GET", "/test", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

// ===========================
// Struct Tests
// ===========================

func TestRegisterCapabilityRequest_Struct(t *testing.T) {
	req := RegisterCapabilityRequest{
		CapabilityType: "file:read",
		Description:    "Allows reading files",
		RiskLevel:      "low",
	}

	assert.Equal(t, "file:read", req.CapabilityType)
	assert.Equal(t, "Allows reading files", req.Description)
	assert.Equal(t, "low", req.RiskLevel)
}

func TestRegisterCapabilityResponse_Struct(t *testing.T) {
	resp := RegisterCapabilityResponse{
		Success:        true,
		CapabilityType: "file:write",
		Status:         "granted",
		Message:        "Capability granted",
		RequestID:      "req-123",
	}

	assert.True(t, resp.Success)
	assert.Equal(t, "file:write", resp.CapabilityType)
	assert.Equal(t, "granted", resp.Status)
	assert.Equal(t, "Capability granted", resp.Message)
	assert.Equal(t, "req-123", resp.RequestID)
}

func TestGrantCapabilityRequest_Struct(t *testing.T) {
	req := GrantCapabilityRequest{
		CapabilityType: "db:execute",
	}

	assert.Equal(t, "db:execute", req.CapabilityType)
}

func TestVerifyActionRequest_Struct(t *testing.T) {
	req := VerifyActionRequest{
		AgentID:             "agent-123",
		Signature:           "c2lnbmF0dXJl",
		RequestPayload:      "cGF5bG9hZA==",
		RequestedCapability: "network:connect",
		Metadata:            map[string]interface{}{"key": "value"},
	}

	assert.Equal(t, "agent-123", req.AgentID)
	assert.Equal(t, "c2lnbmF0dXJl", req.Signature)
	assert.Equal(t, "cGF5bG9hZA==", req.RequestPayload)
	assert.Equal(t, "network:connect", req.RequestedCapability)
	assert.NotNil(t, req.Metadata)
}

func TestViolationsResponse_Struct(t *testing.T) {
	resp := ViolationsResponse{
		Violations: nil,
		Total:      0,
		Limit:      50,
		Offset:     0,
	}

	assert.Nil(t, resp.Violations)
	assert.Equal(t, 0, resp.Total)
	assert.Equal(t, 50, resp.Limit)
	assert.Equal(t, 0, resp.Offset)
}

func TestErrorResponse_Struct(t *testing.T) {
	resp := ErrorResponse{
		Error: "Something went wrong",
	}

	assert.Equal(t, "Something went wrong", resp.Error)
}

func TestSuccessResponse_Struct(t *testing.T) {
	resp := SuccessResponse{
		Message: "Operation completed successfully",
	}

	assert.Equal(t, "Operation completed successfully", resp.Message)
}

// ===========================
// CapabilityHandler Service Getter Tests
// ===========================

func TestCapabilityHandler_getCapabilityService_NilInterfaceReturnsNil(t *testing.T) {
	handler := &CapabilityHandler{}
	result := handler.getCapabilityService()
	assert.Nil(t, result)
}

