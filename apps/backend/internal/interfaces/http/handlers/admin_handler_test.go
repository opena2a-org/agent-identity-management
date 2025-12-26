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

// Note: AdminHandler methods use unsafe type assertions for org/user context,
// so we only test validation paths here. Context validation is handled by middleware.

// ===========================
// NewAdminHandler Tests
// ===========================

func TestNewAdminHandler_NilDeps(t *testing.T) {
	handler := NewAdminHandler(nil, nil, nil, nil, nil, nil, nil, nil)
	assert.NotNil(t, handler)
}

// Helper function for tests that need org/user context
func withOrgAndUserContext(handler func(c fiber.Ctx) error) fiber.Handler {
	return func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		c.Locals("user_id", uuid.New())
		return handler(c)
	}
}

// ===========================
// AdminHandler.UpdateUserRole Tests
// ===========================

func TestAdminHandler_UpdateUserRole_InvalidUserID(t *testing.T) {
	handler := &AdminHandler{}
	app := fiber.New()
	app.Put("/admin/users/:id/role", withOrgAndUserContext(handler.UpdateUserRole))

	body := `{"role":"admin"}`
	req := httptest.NewRequest("PUT", "/admin/users/not-a-uuid/role", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestAdminHandler_UpdateUserRole_InvalidJSON(t *testing.T) {
	handler := &AdminHandler{}
	app := fiber.New()
	app.Put("/admin/users/:id/role", withOrgAndUserContext(handler.UpdateUserRole))

	req := httptest.NewRequest("PUT", "/admin/users/"+uuid.New().String()+"/role", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestAdminHandler_UpdateUserRole_InvalidRole(t *testing.T) {
	handler := &AdminHandler{}
	app := fiber.New()
	app.Put("/admin/users/:id/role", withOrgAndUserContext(handler.UpdateUserRole))

	body := `{"role":"invalid_role"}`
	req := httptest.NewRequest("PUT", "/admin/users/"+uuid.New().String()+"/role", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// AdminHandler.DeactivateUser Tests
// ===========================

func TestAdminHandler_DeactivateUser_InvalidUserID(t *testing.T) {
	handler := &AdminHandler{}
	app := fiber.New()
	app.Post("/admin/users/:id/deactivate", withOrgAndUserContext(handler.DeactivateUser))

	req := httptest.NewRequest("POST", "/admin/users/not-a-uuid/deactivate", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// AdminHandler.ActivateUser Tests
// ===========================

func TestAdminHandler_ActivateUser_InvalidUserID(t *testing.T) {
	handler := &AdminHandler{}
	app := fiber.New()
	app.Post("/admin/users/:id/activate", withOrgAndUserContext(handler.ActivateUser))

	req := httptest.NewRequest("POST", "/admin/users/not-a-uuid/activate", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// AdminHandler.PermanentlyDeleteUser Tests
// ===========================

func TestAdminHandler_PermanentlyDeleteUser_InvalidUserID(t *testing.T) {
	handler := &AdminHandler{}
	app := fiber.New()
	app.Delete("/admin/users/:id", withOrgAndUserContext(handler.PermanentlyDeleteUser))

	req := httptest.NewRequest("DELETE", "/admin/users/not-a-uuid", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// AdminHandler.GetAuditLogByID Tests
// ===========================

func TestAdminHandler_GetAuditLogByID_InvalidID(t *testing.T) {
	handler := &AdminHandler{}
	app := fiber.New()
	app.Get("/admin/audit-logs/:id", withOrgAndUserContext(handler.GetAuditLogByID))

	req := httptest.NewRequest("GET", "/admin/audit-logs/not-a-uuid", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// AdminHandler.AcknowledgeAlert Tests
// ===========================

func TestAdminHandler_AcknowledgeAlert_InvalidID(t *testing.T) {
	handler := &AdminHandler{}
	app := fiber.New()
	app.Post("/admin/alerts/:id/acknowledge", withOrgAndUserContext(handler.AcknowledgeAlert))

	req := httptest.NewRequest("POST", "/admin/alerts/not-a-uuid/acknowledge", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// AdminHandler.ResolveAlert Tests
// ===========================

func TestAdminHandler_ResolveAlert_InvalidID(t *testing.T) {
	handler := &AdminHandler{}
	app := fiber.New()
	app.Post("/admin/alerts/:id/resolve", withOrgAndUserContext(handler.ResolveAlert))

	req := httptest.NewRequest("POST", "/admin/alerts/not-a-uuid/resolve", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// AdminHandler.ApproveUser Tests
// ===========================

func TestAdminHandler_ApproveUser_InvalidID(t *testing.T) {
	handler := &AdminHandler{}
	app := fiber.New()
	app.Post("/admin/users/:id/approve", withOrgAndUserContext(handler.ApproveUser))

	req := httptest.NewRequest("POST", "/admin/users/not-a-uuid/approve", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// AdminHandler.RejectUser Tests
// ===========================

func TestAdminHandler_RejectUser_InvalidID(t *testing.T) {
	handler := &AdminHandler{}
	app := fiber.New()
	app.Post("/admin/users/:id/reject", withOrgAndUserContext(handler.RejectUser))

	req := httptest.NewRequest("POST", "/admin/users/not-a-uuid/reject", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// AdminHandler.ApproveRegistrationRequest Tests
// ===========================

func TestAdminHandler_ApproveRegistrationRequest_InvalidID(t *testing.T) {
	handler := &AdminHandler{}
	app := fiber.New()
	app.Post("/admin/registration-requests/:id/approve", withOrgAndUserContext(handler.ApproveRegistrationRequest))

	req := httptest.NewRequest("POST", "/admin/registration-requests/not-a-uuid/approve", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// AdminHandler.RejectRegistrationRequest Tests
// ===========================

func TestAdminHandler_RejectRegistrationRequest_InvalidID(t *testing.T) {
	handler := &AdminHandler{}
	app := fiber.New()
	app.Post("/admin/registration-requests/:id/reject", withOrgAndUserContext(handler.RejectRegistrationRequest))

	body := `{"reason":"test reason"}`
	req := httptest.NewRequest("POST", "/admin/registration-requests/not-a-uuid/reject", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// Note: RejectRegistrationRequest_InvalidJSON test removed because
// the handler treats JSON parsing errors as optional (uses default reason)

// ===========================
// AdminHandler.UpdateEnforcementSettings Tests
// ===========================

func TestAdminHandler_UpdateEnforcementSettings_InvalidJSON(t *testing.T) {
	handler := &AdminHandler{}
	app := fiber.New()
	app.Put("/admin/enforcement-settings", withOrgAndUserContext(handler.UpdateEnforcementSettings))

	req := httptest.NewRequest("PUT", "/admin/enforcement-settings", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// AdminHandler.BulkAcknowledgeAlerts Tests
// ===========================

func TestAdminHandler_BulkAcknowledgeAlerts_InvalidJSON(t *testing.T) {
	handler := &AdminHandler{}
	app := fiber.New()
	app.Post("/admin/alerts/bulk-acknowledge", withOrgAndUserContext(handler.BulkAcknowledgeAlerts))

	req := httptest.NewRequest("POST", "/admin/alerts/bulk-acknowledge", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// Note: BulkAcknowledgeAlerts_EmptyAlertIDs test removed because
// the handler acknowledges all alerts for the user - it doesn't use alertIds from body

// ===========================
// AdminHandler.ListUsers Tests
// ===========================

func TestAdminHandler_ListUsers_NoOrgContext(t *testing.T) {
	handler := &AdminHandler{}
	app := fiber.New()
	app.Get("/admin/users", handler.ListUsers)

	req := httptest.NewRequest("GET", "/admin/users", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAdminHandler_ListUsers_InvalidOrgIDType(t *testing.T) {
	handler := &AdminHandler{}
	app := fiber.New()
	app.Get("/admin/users", func(c fiber.Ctx) error {
		c.Locals("organization_id", "not-a-uuid") // Wrong type
		return handler.ListUsers(c)
	})

	req := httptest.NewRequest("GET", "/admin/users", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

func TestAdminHandler_ListUsers_NoUserContext(t *testing.T) {
	handler := &AdminHandler{}
	app := fiber.New()
	app.Get("/admin/users", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		// No user_id set
		return handler.ListUsers(c)
	})

	req := httptest.NewRequest("GET", "/admin/users", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAdminHandler_ListUsers_InvalidUserIDType(t *testing.T) {
	handler := &AdminHandler{}
	app := fiber.New()
	app.Get("/admin/users", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		c.Locals("user_id", "not-a-uuid") // Wrong type
		return handler.ListUsers(c)
	})

	req := httptest.NewRequest("GET", "/admin/users", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

// ===========================
// AdminHandler.GetAuditLogs Tests
// ===========================

func TestAdminHandler_GetAuditLogs_NoOrgContext(t *testing.T) {
	handler := &AdminHandler{}
	app := fiber.New()
	app.Get("/admin/audit-logs", handler.GetAuditLogs)

	req := httptest.NewRequest("GET", "/admin/audit-logs", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAdminHandler_GetAuditLogs_InvalidOrgIDType(t *testing.T) {
	handler := &AdminHandler{}
	app := fiber.New()
	app.Get("/admin/audit-logs", func(c fiber.Ctx) error {
		c.Locals("organization_id", "not-a-uuid") // Wrong type
		return handler.GetAuditLogs(c)
	})

	req := httptest.NewRequest("GET", "/admin/audit-logs", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

func TestAdminHandler_GetAuditLogs_NoUserContext(t *testing.T) {
	handler := &AdminHandler{}
	app := fiber.New()
	app.Get("/admin/audit-logs", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		// No user_id set
		return handler.GetAuditLogs(c)
	})

	req := httptest.NewRequest("GET", "/admin/audit-logs", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAdminHandler_GetAuditLogs_InvalidUserIDType(t *testing.T) {
	handler := &AdminHandler{}
	app := fiber.New()
	app.Get("/admin/audit-logs", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		c.Locals("user_id", "not-a-uuid") // Wrong type
		return handler.GetAuditLogs(c)
	})

	req := httptest.NewRequest("GET", "/admin/audit-logs", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

// ===========================
// AdminHandler.ExportAuditLogs Tests
// ===========================

func TestAdminHandler_ExportAuditLogs_NoOrgContext(t *testing.T) {
	handler := &AdminHandler{}
	app := fiber.New()
	app.Get("/admin/audit-logs/export", handler.ExportAuditLogs)

	req := httptest.NewRequest("GET", "/admin/audit-logs/export", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAdminHandler_ExportAuditLogs_InvalidOrgIDType(t *testing.T) {
	handler := &AdminHandler{}
	app := fiber.New()
	app.Get("/admin/audit-logs/export", func(c fiber.Ctx) error {
		c.Locals("organization_id", "not-a-uuid")
		return handler.ExportAuditLogs(c)
	})

	req := httptest.NewRequest("GET", "/admin/audit-logs/export", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

func TestAdminHandler_ExportAuditLogs_NoUserContext(t *testing.T) {
	handler := &AdminHandler{}
	app := fiber.New()
	app.Get("/admin/audit-logs/export", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		return handler.ExportAuditLogs(c)
	})

	req := httptest.NewRequest("GET", "/admin/audit-logs/export", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAdminHandler_ExportAuditLogs_InvalidUserIDType(t *testing.T) {
	handler := &AdminHandler{}
	app := fiber.New()
	app.Get("/admin/audit-logs/export", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		c.Locals("user_id", "not-a-uuid")
		return handler.ExportAuditLogs(c)
	})

	req := httptest.NewRequest("GET", "/admin/audit-logs/export", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

// ===========================
// AdminHandler.GetAlerts Tests
// ===========================

func TestAdminHandler_GetAlerts_NoOrgContext(t *testing.T) {
	handler := &AdminHandler{}
	app := fiber.New()
	app.Get("/admin/alerts", handler.GetAlerts)

	req := httptest.NewRequest("GET", "/admin/alerts", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAdminHandler_GetAlerts_InvalidOrgIDType(t *testing.T) {
	handler := &AdminHandler{}
	app := fiber.New()
	app.Get("/admin/alerts", func(c fiber.Ctx) error {
		c.Locals("organization_id", "not-a-uuid")
		return handler.GetAlerts(c)
	})

	req := httptest.NewRequest("GET", "/admin/alerts", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

func TestAdminHandler_GetAlerts_NoUserContext(t *testing.T) {
	handler := &AdminHandler{}
	app := fiber.New()
	app.Get("/admin/alerts", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		return handler.GetAlerts(c)
	})

	req := httptest.NewRequest("GET", "/admin/alerts", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAdminHandler_GetAlerts_InvalidUserIDType(t *testing.T) {
	handler := &AdminHandler{}
	app := fiber.New()
	app.Get("/admin/alerts", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		c.Locals("user_id", "not-a-uuid")
		return handler.GetAlerts(c)
	})

	req := httptest.NewRequest("GET", "/admin/alerts", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

// ===========================
// AdminHandler.GetOrganizationSettings Tests
// ===========================

func TestAdminHandler_GetOrganizationSettings_NoOrgContext(t *testing.T) {
	handler := &AdminHandler{}
	app := fiber.New()
	app.Get("/admin/organization-settings", handler.GetOrganizationSettings)

	req := httptest.NewRequest("GET", "/admin/organization-settings", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAdminHandler_GetOrganizationSettings_InvalidOrgIDType(t *testing.T) {
	handler := &AdminHandler{}
	app := fiber.New()
	app.Get("/admin/organization-settings", func(c fiber.Ctx) error {
		c.Locals("organization_id", "not-a-uuid")
		return handler.GetOrganizationSettings(c)
	})

	req := httptest.NewRequest("GET", "/admin/organization-settings", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

func TestAdminHandler_GetOrganizationSettings_NoUserContext(t *testing.T) {
	handler := &AdminHandler{}
	app := fiber.New()
	app.Get("/admin/organization-settings", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		return handler.GetOrganizationSettings(c)
	})

	req := httptest.NewRequest("GET", "/admin/organization-settings", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAdminHandler_GetOrganizationSettings_InvalidUserIDType(t *testing.T) {
	handler := &AdminHandler{}
	app := fiber.New()
	app.Get("/admin/organization-settings", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		c.Locals("user_id", "not-a-uuid")
		return handler.GetOrganizationSettings(c)
	})

	req := httptest.NewRequest("GET", "/admin/organization-settings", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

// ===========================
// AdminHandler.GetUnacknowledgedAlertCount Tests
// ===========================

func TestAdminHandler_GetUnacknowledgedAlertCount_NoOrgContext(t *testing.T) {
	handler := &AdminHandler{}
	app := fiber.New()
	app.Get("/admin/alerts/unacknowledged-count", handler.GetUnacknowledgedAlertCount)

	req := httptest.NewRequest("GET", "/admin/alerts/unacknowledged-count", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAdminHandler_GetUnacknowledgedAlertCount_InvalidOrgIDType(t *testing.T) {
	handler := &AdminHandler{}
	app := fiber.New()
	app.Get("/admin/alerts/unacknowledged-count", func(c fiber.Ctx) error {
		c.Locals("organization_id", "not-a-uuid")
		return handler.GetUnacknowledgedAlertCount(c)
	})

	req := httptest.NewRequest("GET", "/admin/alerts/unacknowledged-count", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

// ===========================
// AdminHandler.GetEnforcementSettings Tests
// ===========================

func TestAdminHandler_GetEnforcementSettings_NoOrgContext(t *testing.T) {
	handler := &AdminHandler{}
	app := fiber.New()
	app.Get("/admin/enforcement-settings", handler.GetEnforcementSettings)

	req := httptest.NewRequest("GET", "/admin/enforcement-settings", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAdminHandler_GetEnforcementSettings_InvalidOrgIDType(t *testing.T) {
	handler := &AdminHandler{}
	app := fiber.New()
	app.Get("/admin/enforcement-settings", func(c fiber.Ctx) error {
		c.Locals("organization_id", "not-a-uuid")
		return handler.GetEnforcementSettings(c)
	})

	req := httptest.NewRequest("GET", "/admin/enforcement-settings", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

// ===========================
// AdminHandler.UpdateEnforcementSettings Additional Tests
// ===========================

func TestAdminHandler_UpdateEnforcementSettings_NoOrgContext(t *testing.T) {
	handler := &AdminHandler{}
	app := fiber.New()
	app.Put("/admin/enforcement-settings", handler.UpdateEnforcementSettings)

	body := `{"enforcementMode":"strict"}`
	req := httptest.NewRequest("PUT", "/admin/enforcement-settings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAdminHandler_UpdateEnforcementSettings_InvalidOrgIDType(t *testing.T) {
	handler := &AdminHandler{}
	app := fiber.New()
	app.Put("/admin/enforcement-settings", func(c fiber.Ctx) error {
		c.Locals("organization_id", "not-a-uuid")
		return handler.UpdateEnforcementSettings(c)
	})

	body := `{"enforcementMode":"strict"}`
	req := httptest.NewRequest("PUT", "/admin/enforcement-settings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

func TestAdminHandler_UpdateEnforcementSettings_NoUserContext(t *testing.T) {
	handler := &AdminHandler{}
	app := fiber.New()
	app.Put("/admin/enforcement-settings", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		return handler.UpdateEnforcementSettings(c)
	})

	body := `{"enforcementMode":"strict"}`
	req := httptest.NewRequest("PUT", "/admin/enforcement-settings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAdminHandler_UpdateEnforcementSettings_InvalidUserIDType(t *testing.T) {
	handler := &AdminHandler{}
	app := fiber.New()
	app.Put("/admin/enforcement-settings", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		c.Locals("user_id", "not-a-uuid")
		return handler.UpdateEnforcementSettings(c)
	})

	body := `{"enforcementMode":"strict"}`
	req := httptest.NewRequest("PUT", "/admin/enforcement-settings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

func TestAdminHandler_UpdateEnforcementSettings_InvalidEnforcementMode(t *testing.T) {
	handler := &AdminHandler{}
	app := fiber.New()
	app.Put("/admin/enforcement-settings", withOrgAndUserContext(handler.UpdateEnforcementSettings))

	body := `{"enforcementMode":"invalid_mode"}`
	req := httptest.NewRequest("PUT", "/admin/enforcement-settings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// AdminHandler.BulkAcknowledgeAlerts Additional Tests
// ===========================

func TestAdminHandler_BulkAcknowledgeAlerts_InvalidUserIDInBody(t *testing.T) {
	handler := &AdminHandler{}
	app := fiber.New()
	app.Post("/admin/alerts/bulk-acknowledge", withOrgAndUserContext(handler.BulkAcknowledgeAlerts))

	body := `{"userId":"not-a-uuid"}`
	req := httptest.NewRequest("POST", "/admin/alerts/bulk-acknowledge", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestAdminHandler_BulkAcknowledgeAlerts_UserIDMismatch(t *testing.T) {
	handler := &AdminHandler{}
	app := fiber.New()
	// Use specific user ID so we can test mismatch
	specificUserID := uuid.New()
	app.Post("/admin/alerts/bulk-acknowledge", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		c.Locals("user_id", specificUserID)
		return handler.BulkAcknowledgeAlerts(c)
	})

	// Different UUID than what's in context
	differentUserID := uuid.New()
	body := `{"userId":"` + differentUserID.String() + `"}`
	req := httptest.NewRequest("POST", "/admin/alerts/bulk-acknowledge", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusForbidden, resp.StatusCode)
}
