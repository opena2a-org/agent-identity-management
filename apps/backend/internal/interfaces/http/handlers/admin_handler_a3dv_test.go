package handlers

import (
	"context"
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

// adminTestUserRepo is a domain.UserRepository stub that only
// implements GetByID. Other 10 methods inherited from the nil-
// embedded interface, will panic if reached — that's the regression-
// proofing signal.
type adminTestUserRepo struct {
	domain.UserRepository
	getByID func(id uuid.UUID) (*domain.User, error)
}

func (r *adminTestUserRepo) GetByID(id uuid.UUID) (*domain.User, error) {
	return r.getByID(id)
}

// sameOrgUserStub builds a getByID closure that returns a user in the
// supplied org (so the LoadOwned guard treats the request as legitimate).
// Used by the existing approve/reject success/error tests to satisfy
// the new A3d-v guard without changing their assertions.
func sameOrgUserStub(userID, orgID uuid.UUID) func(uuid.UUID) (*domain.User, error) {
	return func(id uuid.UUID) (*domain.User, error) {
		return &domain.User{ID: userID, OrganizationID: orgID, Status: domain.UserStatusPending}, nil
	}
}

// ===========================
// A3d-v: cross-tenant access on the 4 admin approval routes must
// return 404 with existence-secrecy body. Each row exercises the new
// LoadOwned (or inline nullable-aware) guard wired in this PR. The
// stub repos return resources in a DIFFERENT org from the caller; the
// gate must short-circuit before any service mutation runs.
//
// Regression-proofing pattern:
//   - ApproveUser / RejectUser: AdminService is intentionally nil; a
//     bypass would nil-deref on the service call → fiber 500 → the
//     assert.Equal(404) below fires.
//   - ApproveRegistrationRequest / RejectRegistrationRequest: the
//     reg-service mock's Approve/Reject funcs are NOT set; if the
//     gate is bypassed, the default mock returns nil/nil and the
//     handler returns 200 → assert.Equal(404) fires.
// ===========================

func TestAdminHandler_Approvals_CrossOrgReturns404(t *testing.T) {
	callerOrgID := uuid.New()
	differentOrgID := uuid.New()
	adminID := uuid.New()
	targetUserID := uuid.New()
	requestID := uuid.New()

	userRepoForeignOrg := &adminTestUserRepo{
		getByID: func(id uuid.UUID) (*domain.User, error) {
			return &domain.User{ID: targetUserID, OrganizationID: differentOrgID, Status: domain.UserStatusPending}, nil
		},
	}
	regSvcForeignOrg := &MockRegistrationServiceImpl{
		GetRegistrationRequestFunc: func(ctx context.Context, reqID uuid.UUID) (*domain.UserRegistrationRequest, error) {
			oID := differentOrgID
			return &domain.UserRegistrationRequest{ID: reqID, OrganizationID: &oID, Status: domain.RegistrationStatusPending}, nil
		},
		// Approve/Reject funcs intentionally NOT set — a bypass would
		// fall through to the default mock impls which return nil/nil
		// or 200, distinguishable from the 404 the gate enforces.
	}

	// AdminService intentionally nil — a bypass on ApproveUser /
	// RejectUser nil-derefs and fiber returns 500.
	handler := NewAdminHandlerWithInterfaces(nil, nil, nil, nil, nil, nil, regSvcForeignOrg, nil, userRepoForeignOrg)

	tests := []struct {
		name        string
		method      string
		requestPath string
		body        string
		setup       func(*fiber.App, *AdminHandler)
	}{
		{
			name:        "ApproveUser",
			method:      "POST",
			requestPath: "/admin/users/" + targetUserID.String() + "/approve",
			body:        "",
			setup: func(app *fiber.App, h *AdminHandler) {
				app.Post("/admin/users/:id/approve", func(c fiber.Ctx) error {
					c.Locals("organization_id", callerOrgID)
					c.Locals("user_id", adminID)
					return h.ApproveUser(c)
				})
			},
		},
		{
			name:        "RejectUser",
			method:      "POST",
			requestPath: "/admin/users/" + targetUserID.String() + "/reject",
			body:        `{"reason":"x"}`,
			setup: func(app *fiber.App, h *AdminHandler) {
				app.Post("/admin/users/:id/reject", func(c fiber.Ctx) error {
					c.Locals("organization_id", callerOrgID)
					c.Locals("user_id", adminID)
					return h.RejectUser(c)
				})
			},
		},
		{
			name:        "ApproveRegistrationRequest",
			method:      "POST",
			requestPath: "/admin/registration-requests/" + requestID.String() + "/approve",
			body:        "",
			setup: func(app *fiber.App, h *AdminHandler) {
				app.Post("/admin/registration-requests/:id/approve", func(c fiber.Ctx) error {
					c.Locals("organization_id", callerOrgID)
					c.Locals("user_id", adminID)
					return h.ApproveRegistrationRequest(c)
				})
			},
		},
		{
			name:        "RejectRegistrationRequest",
			method:      "POST",
			requestPath: "/admin/registration-requests/" + requestID.String() + "/reject",
			body:        `{"reason":"x"}`,
			setup: func(app *fiber.App, h *AdminHandler) {
				app.Post("/admin/registration-requests/:id/reject", func(c fiber.Ctx) error {
					c.Locals("organization_id", callerOrgID)
					c.Locals("user_id", adminID)
					return h.RejectRegistrationRequest(c)
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

// Unassigned-claim flow: ApproveRegistrationRequest must STILL succeed
// when the request has OrganizationID == nil (a new-org signup that
// any admin can claim). This preserves existing product behavior.
func TestAdminHandler_ApproveRegistrationRequest_UnassignedRequestAllowed(t *testing.T) {
	callerOrgID := uuid.New()
	adminID := uuid.New()
	requestID := uuid.New()

	regSvc := &MockRegistrationServiceImpl{
		GetRegistrationRequestFunc: func(ctx context.Context, reqID uuid.UUID) (*domain.UserRegistrationRequest, error) {
			return &domain.UserRegistrationRequest{ID: reqID, OrganizationID: nil, Status: domain.RegistrationStatusPending}, nil
		},
		ApproveRegistrationRequestFunc: func(ctx context.Context, reqID, adminID, oID uuid.UUID) (*domain.User, error) {
			return &domain.User{ID: uuid.New(), OrganizationID: oID, Email: "new@example.com"}, nil
		},
	}
	auditSvc := &MockAuditServiceImpl{LogActionFunc: func(ctx context.Context, orgID uuid.UUID, userID uuid.UUID, action domain.AuditAction, resourceType string, resourceID uuid.UUID, ipAddress string, userAgent string, metadata map[string]interface{}) error {
		return nil
	}}
	handler := NewAdminHandlerWithInterfaces(nil, nil, nil, nil, auditSvc, nil, regSvc, nil, nil)

	app := fiber.New()
	app.Post("/admin/registration-requests/:id/approve", func(c fiber.Ctx) error {
		c.Locals("organization_id", callerOrgID)
		c.Locals("user_id", adminID)
		return handler.ApproveRegistrationRequest(c)
	})

	req := httptest.NewRequest("POST", "/admin/registration-requests/"+requestID.String()+"/approve", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode,
		"unassigned (OrganizationID=nil) registration request must remain claimable by any admin")
}
