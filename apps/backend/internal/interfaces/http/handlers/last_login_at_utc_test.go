package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
)

// Every response that carries lastLoginAt names the instant in UTC, with a Z
// suffix, whatever location the value was read or clocked with. A sign-in
// response and /api/v1/auth/me then name the same instant in the same form.

// lastLoginStored is a stored last_login_at that carries a non-UTC location.
var lastLoginStored = time.Date(2026, 10, 11, 6, 25, 44, 184309000, time.FixedZone("", -6*3600))

// lastLoginAtOf reads the lastLoginAt string under the given JSON object and
// checks it is RFC 3339 in UTC. It returns the instant it names.
func lastLoginAtOf(t *testing.T, object map[string]any, where string) time.Time {
	t.Helper()
	raw, ok := object["lastLoginAt"].(string)
	require.True(t, ok, "%s carries lastLoginAt as a string: %v", where, object["lastLoginAt"])
	assert.True(t, strings.HasSuffix(raw, "Z"), "%s lastLoginAt is in UTC: %s", where, raw)
	at, err := time.Parse(time.RFC3339Nano, raw)
	require.NoError(t, err, "%s lastLoginAt parses as RFC 3339: %s", where, raw)
	return at
}

func TestLastLoginAt_SignInAnswersInUTCLikeAuthMe(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-only-jwt-secret-not-a-real-value-0123456789")
	jsonLogin := func() string {
		b, _ := json.Marshal(map[string]string{"email": "issuance@example.com", "password": issuancePassword})
		return string(b)
	}
	rows := []struct {
		name        string
		forceChange bool
		route       string
		body        func() string
	}{
		{"public login", false, "/api/v1/public/login", jsonLogin},
		// A forced password change answers with the stored value, not a new one.
		{"public login, password change required", true, "/api/v1/public/login", jsonLogin},
		{"public change-password", true, "/api/v1/public/change-password", func() string {
			b, _ := json.Marshal(map[string]string{"email": "issuance@example.com", "oldPassword": issuancePassword, "newPassword": "Another-Cell-Passw0rd!"})
			return string(b)
		}},
	}

	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			user := issuanceUser(t, row.forceChange)
			stored := lastLoginStored
			user.LastLoginAt = &stored
			users := &issuanceUserRepo{user: user}
			authService := application.NewAuthService(users, nil, nil, nil, nil, nil)
			jwtService := auth.NewJWTService()

			app := fiber.New()
			public := NewPublicRegistrationHandler(application.NewRegistrationService(refusalRegistrationRepo{}, users, nil, nil, nil), authService, jwtService, nil)
			app.Post("/api/v1/public/login", public.Login)
			app.Post("/api/v1/public/change-password", public.ChangePassword)
			me := NewAuthHandler(authService, jwtService, &MockOrganizationRepository{}, nil, nil)
			app.Get("/api/v1/auth/me", func(c fiber.Ctx) error {
				c.Locals("user_id", user.ID)
				return me.Me(c)
			})

			req := httptest.NewRequest("POST", row.route, strings.NewReader(row.body()))
			req.Header.Set("Content-Type", "application/json")
			resp, err := app.Test(req, fiber.TestConfig{Timeout: 30 * time.Second, FailOnTimeout: true})
			require.NoError(t, err)
			defer resp.Body.Close()
			raw, _ := io.ReadAll(resp.Body)
			require.Equal(t, fiber.StatusOK, resp.StatusCode, "the sign-in succeeds: %s", string(raw))

			var body struct {
				User map[string]any `json:"user"`
			}
			require.NoError(t, json.Unmarshal(raw, &body))
			signedInAt := lastLoginAtOf(t, body.User, "the sign-in response")

			meReq := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
			meResp, err := app.Test(meReq, fiber.TestConfig{Timeout: 30 * time.Second, FailOnTimeout: true})
			require.NoError(t, err)
			defer meResp.Body.Close()
			meRaw, _ := io.ReadAll(meResp.Body)
			require.Equal(t, fiber.StatusOK, meResp.StatusCode, "/api/v1/auth/me answers: %s", string(meRaw))

			var meBody map[string]any
			require.NoError(t, json.Unmarshal(meRaw, &meBody))
			meAt := lastLoginAtOf(t, meBody, "/api/v1/auth/me")
			assert.True(t, signedInAt.Equal(meAt), "the sign-in response (%s) and /api/v1/auth/me (%s) name the same instant", signedInAt, meAt)
		})
	}
}

func TestLastLoginAt_AdminUserListAnswersInUTC(t *testing.T) {
	orgID := uuid.New()
	adminID := uuid.New()
	stored := lastLoginStored
	authService := &MockAuthServiceImpl{
		GetUsersByOrganizationFunc: func(ctx context.Context, id uuid.UUID) ([]*domain.User, error) {
			return []*domain.User{{
				ID:             uuid.New(),
				OrganizationID: orgID,
				Email:          "listed@example.com",
				Name:           "Listed User",
				Role:           domain.RoleMember,
				Status:         domain.UserStatusActive,
				LastLoginAt:    &stored,
			}}, nil
		},
	}
	registration := &MockRegistrationServiceImpl{
		ListPendingRegistrationRequestsFunc: func(ctx context.Context, id uuid.UUID, limit, offset int) ([]*domain.UserRegistrationRequest, int, error) {
			return nil, 0, nil
		},
	}
	handler := NewAdminHandlerWithInterfaces(authService, nil, nil, nil, &MockAuditServiceImpl{}, nil, registration, nil, nil)

	app := fiber.New()
	app.Get("/api/v1/admin/users", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", adminID)
		return handler.ListUsers(c)
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/api/v1/admin/users", nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	require.Equal(t, fiber.StatusOK, resp.StatusCode, "the list answers: %s", string(raw))

	var body struct {
		Users []map[string]any `json:"users"`
	}
	require.NoError(t, json.Unmarshal(raw, &body))
	require.Len(t, body.Users, 1)
	at := lastLoginAtOf(t, body.Users[0], "the admin user list")
	assert.True(t, at.Equal(lastLoginStored), "the admin user list names the stored instant: %s", at)
}
