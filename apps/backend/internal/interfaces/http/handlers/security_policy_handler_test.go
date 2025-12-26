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

// Helper function for security policy tests that need org context
func withSecurityPolicyContext(handler func(c fiber.Ctx) error) fiber.Handler {
	return func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		c.Locals("user_id", uuid.New())
		return handler(c)
	}
}

// ===========================
// NewSecurityPolicyHandler Tests
// ===========================

func TestNewSecurityPolicyHandler_NilDeps(t *testing.T) {
	handler := NewSecurityPolicyHandler(nil)
	assert.NotNil(t, handler)
}

// ===========================
// SecurityPolicyHandler.ListPolicies Tests
// ===========================

func TestSecurityPolicyHandler_ListPolicies_NoOrgContext(t *testing.T) {
	handler := &SecurityPolicyHandler{}
	app := fiber.New()
	app.Get("/security/policies", handler.ListPolicies)

	req := httptest.NewRequest("GET", "/security/policies", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestSecurityPolicyHandler_ListPolicies_InvalidOrgIDType(t *testing.T) {
	handler := &SecurityPolicyHandler{}
	app := fiber.New()
	app.Get("/security/policies", func(c fiber.Ctx) error {
		c.Locals("organization_id", "not-a-uuid") // Wrong type
		return handler.ListPolicies(c)
	})

	req := httptest.NewRequest("GET", "/security/policies", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

// ===========================
// SecurityPolicyHandler.GetPolicy Tests
// ===========================

func TestSecurityPolicyHandler_GetPolicy_InvalidID(t *testing.T) {
	handler := &SecurityPolicyHandler{}
	app := fiber.New()
	app.Get("/security/policies/:id", handler.GetPolicy)

	req := httptest.NewRequest("GET", "/security/policies/not-a-uuid", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// SecurityPolicyHandler.UpdatePolicy Tests
// ===========================

func TestSecurityPolicyHandler_UpdatePolicy_InvalidID(t *testing.T) {
	handler := &SecurityPolicyHandler{}
	app := fiber.New()
	app.Put("/security/policies/:id", withSecurityPolicyContext(handler.UpdatePolicy))

	body := `{"name":"test"}`
	req := httptest.NewRequest("PUT", "/security/policies/not-a-uuid", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestSecurityPolicyHandler_UpdatePolicy_InvalidJSON(t *testing.T) {
	handler := &SecurityPolicyHandler{}
	app := fiber.New()
	app.Put("/security/policies/:id", withSecurityPolicyContext(handler.UpdatePolicy))

	req := httptest.NewRequest("PUT", "/security/policies/"+uuid.New().String(), strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// SecurityPolicyHandler.DeletePolicy Tests
// ===========================

func TestSecurityPolicyHandler_DeletePolicy_InvalidID(t *testing.T) {
	handler := &SecurityPolicyHandler{}
	app := fiber.New()
	app.Delete("/security/policies/:id", handler.DeletePolicy)

	req := httptest.NewRequest("DELETE", "/security/policies/not-a-uuid", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// SecurityPolicyHandler.TogglePolicy Tests
// ===========================

func TestSecurityPolicyHandler_TogglePolicy_InvalidID(t *testing.T) {
	handler := &SecurityPolicyHandler{}
	app := fiber.New()
	app.Put("/security/policies/:id/toggle", handler.TogglePolicy)

	body := `{"isEnabled":true}`
	req := httptest.NewRequest("PUT", "/security/policies/not-a-uuid/toggle", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestSecurityPolicyHandler_TogglePolicy_InvalidJSON(t *testing.T) {
	handler := &SecurityPolicyHandler{}
	app := fiber.New()
	app.Put("/security/policies/:id/toggle", handler.TogglePolicy)

	req := httptest.NewRequest("PUT", "/security/policies/"+uuid.New().String()+"/toggle", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// SecurityPolicyHandler.CreatePolicy Tests
// ===========================

func TestSecurityPolicyHandler_CreatePolicy_InvalidJSON(t *testing.T) {
	handler := &SecurityPolicyHandler{}
	app := fiber.New()
	app.Post("/security/policies", withSecurityPolicyContext(handler.CreatePolicy))

	req := httptest.NewRequest("POST", "/security/policies", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// Struct Tests
// ===========================

func TestCreatePolicyRequest_Struct(t *testing.T) {
	req := CreatePolicyRequest{
		Name:              "Test Policy",
		Description:       "A test security policy",
		PolicyType:        "access_control",
		EnforcementAction: "deny",
		SeverityThreshold: "high",
		Rules:             map[string]interface{}{"maxAttempts": 5},
		AppliesTo:         "all_agents",
		IsEnabled:         true,
		Priority:          1,
	}

	assert.Equal(t, "Test Policy", req.Name)
	assert.Equal(t, "A test security policy", req.Description)
	assert.Equal(t, "access_control", string(req.PolicyType))
	assert.Equal(t, "deny", string(req.EnforcementAction))
	assert.Equal(t, "high", string(req.SeverityThreshold))
	assert.NotNil(t, req.Rules)
	assert.Equal(t, "all_agents", req.AppliesTo)
	assert.True(t, req.IsEnabled)
	assert.Equal(t, 1, req.Priority)
}

func TestCreatePolicyRequest_EmptyStruct(t *testing.T) {
	req := CreatePolicyRequest{}

	assert.Empty(t, req.Name)
	assert.Empty(t, req.Description)
	assert.False(t, req.IsEnabled)
	assert.Equal(t, 0, req.Priority)
}

func TestTogglePolicyRequest_Struct(t *testing.T) {
	tests := []struct {
		name      string
		isEnabled bool
	}{
		{"Enabled", true},
		{"Disabled", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := TogglePolicyRequest{
				IsEnabled: tt.isEnabled,
			}
			assert.Equal(t, tt.isEnabled, req.IsEnabled)
		})
	}
}
