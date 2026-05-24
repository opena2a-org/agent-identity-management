package handlers

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===========================
// A3d-viii: cross-tenant access on the 4 admin user-lifecycle routes
// must return 404 with the existence-secrecy body. Each row exercises
// the new LoadOwned guard wired in this PR (or, for ActivateUser /
// PermanentlyDeleteUser, the LoadOwned guard that REPLACES the pre-fix
// explicit 404/403 existence oracle).
//
// Regression-proofing pattern: AdminServicer and AuthServicer are
// intentionally nil. A bypass of the LoadOwned guard would reach
//   - h.getAuthService().UpdateUserRole / DeactivateUser, or
//   - h.getAdminService().ActivateUser / PermanentlyDeleteUser, or
//   - h.isSuperAdmin -> h.getAuthService().GetUserByID
// All four nil-deref → fiber 500 → assert.Equal(404) below fires.
//
// PermanentlyDeleteUser has an additional dimension: the LoadOwned
// returned user is reused for the audit log; a bypass also surfaces as
// a nil-pointer panic on `user.Email` even before any service call.
// ===========================

func TestAdminHandler_UserLifecycle_CrossOrgReturns404(t *testing.T) {
	callerOrgID := uuid.New()
	differentOrgID := uuid.New()
	adminID := uuid.New()
	targetUserID := uuid.New()

	userRepoForeignOrg := &adminTestUserRepo{
		getByID: func(id uuid.UUID) (*domain.User, error) {
			return &domain.User{
				ID:             targetUserID,
				OrganizationID: differentOrgID,
				Status:         domain.UserStatusActive,
				Email:          "victim@other-org.example",
				Name:           "Victim Other-Org",
			}, nil
		},
	}

	// authServicer + adminServicer intentionally nil — a bypass on any
	// of the four handlers nil-derefs (either on the service call
	// directly, or via isSuperAdmin) and fiber returns 500.
	handler := NewAdminHandlerWithInterfaces(nil, nil, nil, nil, nil, nil, nil, nil, userRepoForeignOrg)

	tests := []struct {
		name        string
		method      string
		requestPath string
		body        string
		setup       func(*fiber.App, *AdminHandler)
	}{
		{
			name:        "UpdateUserRole",
			method:      "PUT",
			requestPath: "/admin/users/" + targetUserID.String() + "/role",
			// Valid role required — role validation happens BEFORE the
			// LoadOwned gate (the gate is the last check before the
			// service call), so an invalid role would short-circuit to
			// 400 and the test would not exercise the gate.
			body: `{"role":"member"}`,
			setup: func(app *fiber.App, h *AdminHandler) {
				app.Put("/admin/users/:id/role", func(c fiber.Ctx) error {
					c.Locals("organization_id", callerOrgID)
					c.Locals("user_id", adminID)
					return h.UpdateUserRole(c)
				})
			},
		},
		{
			name:        "DeactivateUser",
			method:      "POST",
			requestPath: "/admin/users/" + targetUserID.String() + "/deactivate",
			body:        "",
			setup: func(app *fiber.App, h *AdminHandler) {
				app.Post("/admin/users/:id/deactivate", func(c fiber.Ctx) error {
					c.Locals("organization_id", callerOrgID)
					c.Locals("user_id", adminID)
					return h.DeactivateUser(c)
				})
			},
		},
		{
			name:        "ActivateUser",
			method:      "POST",
			requestPath: "/admin/users/" + targetUserID.String() + "/activate",
			body:        "",
			setup: func(app *fiber.App, h *AdminHandler) {
				app.Post("/admin/users/:id/activate", func(c fiber.Ctx) error {
					c.Locals("organization_id", callerOrgID)
					c.Locals("user_id", adminID)
					return h.ActivateUser(c)
				})
			},
		},
		{
			name:        "PermanentlyDeleteUser",
			method:      "DELETE",
			requestPath: "/admin/users/" + targetUserID.String(),
			body:        "",
			setup: func(app *fiber.App, h *AdminHandler) {
				app.Delete("/admin/users/:id", func(c fiber.Ctx) error {
					c.Locals("organization_id", callerOrgID)
					c.Locals("user_id", adminID)
					return h.PermanentlyDeleteUser(c)
				})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := fiber.New()
			tt.setup(app, handler)

			var req *http.Request
			if tt.body == "" {
				req = httptest.NewRequest(tt.method, tt.requestPath, nil)
			} else {
				req = httptest.NewRequest(tt.method, tt.requestPath, strings.NewReader(tt.body))
				req.Header.Set("Content-Type", "application/json")
			}

			resp, err := app.Test(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, fiber.StatusNotFound, resp.StatusCode,
				"%s: cross-tenant request must return 404, got %d", tt.name, resp.StatusCode)
			body, _ := io.ReadAll(resp.Body)
			assert.JSONEq(t, `{"error":"not found"}`, string(body),
				"%s: cross-tenant 404 body must be existence-secrecy shape, got %s", tt.name, string(body))
		})
	}
}

// Self-target short-circuits MUST still fire BEFORE the LoadOwned gate
// for DeactivateUser and PermanentlyDeleteUser. The handlers reject
// "deactivate / delete your own account" with 400 before any DB load,
// because the attacker provides their own userID — no cross-tenant
// information is leaked by the 400 response.
//
// This test pins the ordering: if a future refactor moves LoadOwned
// before the self-check, the test would still pass (404 is the
// existence-secrecy outcome). But if a refactor REMOVES the self-check
// entirely, the test fails (handler proceeds to service call with
// targetUserID == adminID and either succeeds or nil-derefs).
func TestAdminHandler_UserLifecycle_SelfTargetReturns400(t *testing.T) {
	orgID := uuid.New()
	adminID := uuid.New()
	// userRepo returns a same-org user so LoadOwned would pass if reached.
	userRepo := &adminTestUserRepo{
		getByID: func(id uuid.UUID) (*domain.User, error) {
			return &domain.User{ID: id, OrganizationID: orgID, Status: domain.UserStatusActive}, nil
		},
	}
	// All services nil — only the 400 self-target branch is meant to
	// fire. A regression that reaches the service crashes (500) or
	// returns 200, distinguishable from 400.
	handler := NewAdminHandlerWithInterfaces(nil, nil, nil, nil, nil, nil, nil, nil, userRepo)

	cases := []struct {
		name        string
		method      string
		requestPath string
		setup       func(*fiber.App, *AdminHandler)
	}{
		{
			name:        "DeactivateUser",
			method:      "POST",
			requestPath: "/admin/users/" + adminID.String() + "/deactivate",
			setup: func(app *fiber.App, h *AdminHandler) {
				app.Post("/admin/users/:id/deactivate", func(c fiber.Ctx) error {
					c.Locals("organization_id", orgID)
					c.Locals("user_id", adminID)
					return h.DeactivateUser(c)
				})
			},
		},
		{
			name:        "PermanentlyDeleteUser",
			method:      "DELETE",
			requestPath: "/admin/users/" + adminID.String(),
			setup: func(app *fiber.App, h *AdminHandler) {
				app.Delete("/admin/users/:id", func(c fiber.Ctx) error {
					c.Locals("organization_id", orgID)
					c.Locals("user_id", adminID)
					return h.PermanentlyDeleteUser(c)
				})
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := fiber.New()
			tc.setup(app, handler)
			req := httptest.NewRequest(tc.method, tc.requestPath, nil)
			resp, err := app.Test(req)
			require.NoError(t, err)
			defer resp.Body.Close()
			assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode,
				"%s: self-target must return 400, got %d", tc.name, resp.StatusCode)
		})
	}
}
