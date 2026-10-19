package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
)

// The auth middleware reads the user record behind a login access token on
// every request: a session of a deactivated account is answered 401, a token
// signed in before the account's last password change is answered 401, and
// while the account must change its password every request but
// GET /api/v1/auth/me and POST /api/v1/auth/change-password is answered 403
// with the code passwordChangeRequired.

type sessionStateFake struct {
	state *domain.UserSessionState
	err   error
}

func (f *sessionStateFake) GetSessionState(context.Context, uuid.UUID) (*domain.UserSessionState, error) {
	return f.state, f.err
}

func passwordChangeTestService(t *testing.T, reader auth.SessionStateReader) (*auth.JWTService, string) {
	t.Helper()
	t.Setenv("JWT_SECRET", "test-secret-key-for-testing-purposes-32chars")
	svc := auth.NewJWTService()
	if reader != nil {
		svc.SetSessionStateReader(reader)
	}
	access, _, err := svc.GenerateTokenPair(uuid.New().String(), uuid.New().String(), "pc@example.com", "admin")
	require.NoError(t, err)
	return svc, access
}

func passwordChangeTestApp(svc *auth.JWTService) *fiber.App {
	app := fiber.New()
	app.Use(AuthMiddleware(svc))
	ok := func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }
	app.Get("/api/v1/auth/me", ok)
	app.Post("/api/v1/auth/me", ok)
	app.Post("/api/v1/auth/change-password", ok)
	app.Get("/api/v1/auth/change-password", ok)
	app.Post("/api/v1/auth/reauth", ok)
	app.Post("/api/v1/agents", ok)
	app.Get("/api/v1/auth/me/sessions", ok)
	return app
}

func passwordChangeTestCall(t *testing.T, app *fiber.App, method, path, token string) (int, map[string]interface{}) {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body := map[string]interface{}{}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	return resp.StatusCode, body
}

func TestAuthMiddleware_PasswordChangeRequired_AdmitsOnlyTheRoutesThatChangeIt(t *testing.T) {
	svc, access := passwordChangeTestService(t, &sessionStateFake{state: &domain.UserSessionState{ForcePasswordChange: true}})
	app := passwordChangeTestApp(svc)

	admitted := [][2]string{
		{"GET", "/api/v1/auth/me"},
		{"GET", "/api/v1/auth/me/"},
		{"GET", "/API/V1/Auth/Me"},
		{"POST", "/api/v1/auth/change-password"},
		{"POST", "/api/v1/auth/change-password/"},
	}
	for _, r := range admitted {
		status, _ := passwordChangeTestCall(t, app, r[0], r[1], access)
		assert.Equal(t, fiber.StatusOK, status, "%s %s is admitted", r[0], r[1])
	}

	refused := [][2]string{
		{"POST", "/api/v1/agents"},
		{"POST", "/api/v1/auth/reauth"},
		{"POST", "/api/v1/auth/me"},
		{"GET", "/api/v1/auth/change-password"},
		{"GET", "/api/v1/auth/me/sessions"},
	}
	for _, r := range refused {
		status, body := passwordChangeTestCall(t, app, r[0], r[1], access)
		assert.Equal(t, fiber.StatusForbidden, status, "%s %s is refused", r[0], r[1])
		assert.Equal(t, PasswordChangeRequiredCode, body["code"], "%s %s", r[0], r[1])
		assert.Contains(t, body["error"], "POST /api/v1/auth/change-password", "the answer names the route that clears it")
	}
}

func TestAuthMiddleware_RefusesATokenSignedInBeforeThePasswordChange(t *testing.T) {
	reader := &sessionStateFake{state: &domain.UserSessionState{}}
	svc, access := passwordChangeTestService(t, reader)
	app := passwordChangeTestApp(svc)

	status, _ := passwordChangeTestCall(t, app, "GET", "/api/v1/auth/me", access)
	require.Equal(t, fiber.StatusOK, status, "control: no change recorded")

	claims, err := svc.ValidateToken(access)
	require.NoError(t, err)
	changedAt := claims.SignedInAt().Add(2 * time.Second)
	reader.state = &domain.UserSessionState{PasswordChangedAt: &changedAt}
	status, body := passwordChangeTestCall(t, app, "GET", "/api/v1/auth/me", access)
	assert.Equal(t, fiber.StatusUnauthorized, status)
	assert.Contains(t, body["error"], "password changed")

	earlier := claims.SignedInAt().Add(-2 * time.Second)
	reader.state = &domain.UserSessionState{PasswordChangedAt: &earlier}
	status, _ = passwordChangeTestCall(t, app, "GET", "/api/v1/auth/me", access)
	assert.Equal(t, fiber.StatusOK, status, "a sign-in after the change is not refused")
}

// Deactivating an account ends its sessions: the session's next request is
// refused on every route, the two a session that must change its password
// may still use among them.
func TestAuthMiddleware_RefusesASessionOfADeactivatedAccount(t *testing.T) {
	reader := &sessionStateFake{state: &domain.UserSessionState{}}
	svc, access := passwordChangeTestService(t, reader)
	app := passwordChangeTestApp(svc)

	status, _ := passwordChangeTestCall(t, app, "GET", "/api/v1/auth/me", access)
	require.Equal(t, fiber.StatusOK, status, "control: the account is not deactivated")

	routes := [][2]string{
		{"GET", "/api/v1/auth/me"},
		{"POST", "/api/v1/auth/change-password"},
		{"POST", "/api/v1/auth/reauth"},
		{"POST", "/api/v1/agents"},
	}
	for _, state := range []domain.UserSessionState{
		{Deactivated: true},
		{Deactivated: true, ForcePasswordChange: true},
	} {
		reader.state = &state
		for _, r := range routes {
			status, body := passwordChangeTestCall(t, app, r[0], r[1], access)
			assert.Equal(t, fiber.StatusUnauthorized, status, "%s %s, password change required: %v", r[0], r[1], state.ForcePasswordChange)
			assert.Equal(t, "Token has been revoked: the account is deactivated", body["error"], "%s %s", r[0], r[1])
		}
	}
}

func TestAuthMiddleware_UserRecordThatCannotBeRead(t *testing.T) {
	t.Run("no such user: 401", func(t *testing.T) {
		svc, access := passwordChangeTestService(t, &sessionStateFake{err: domain.ErrUserNotFound})
		status, _ := passwordChangeTestCall(t, passwordChangeTestApp(svc), "GET", "/api/v1/auth/me", access)
		assert.Equal(t, fiber.StatusUnauthorized, status)
	})
	t.Run("the read fails: 503", func(t *testing.T) {
		svc, access := passwordChangeTestService(t, &sessionStateFake{err: errors.New("connection refused")})
		status, body := passwordChangeTestCall(t, passwordChangeTestApp(svc), "GET", "/api/v1/auth/me", access)
		assert.Equal(t, fiber.StatusServiceUnavailable, status)
		assert.NotContains(t, body["error"], "connection refused", "the store's error stays in the log")
	})
}

// OptionalAuthMiddleware lets a refused session through unauthenticated: the
// route sees no user.
func TestOptionalAuthMiddleware_SessionTheUserRecordRefusesIsAnonymous(t *testing.T) {
	cases := map[string]func(signedIn time.Time) *sessionStateFake{
		"no change": func(time.Time) *sessionStateFake {
			return &sessionStateFake{state: &domain.UserSessionState{}}
		},
		"password changed after it": func(s time.Time) *sessionStateFake {
			at := s.Add(2 * time.Second)
			return &sessionStateFake{state: &domain.UserSessionState{PasswordChangedAt: &at}}
		},
		"password change required": func(time.Time) *sessionStateFake {
			return &sessionStateFake{state: &domain.UserSessionState{ForcePasswordChange: true}}
		},
		"account deactivated": func(time.Time) *sessionStateFake {
			return &sessionStateFake{state: &domain.UserSessionState{Deactivated: true}}
		},
		"the record cannot be read": func(time.Time) *sessionStateFake {
			return &sessionStateFake{err: errors.New("connection refused")}
		},
	}
	for name, fake := range cases {
		t.Run(name, func(t *testing.T) {
			reader := &sessionStateFake{}
			svc, access := passwordChangeTestService(t, reader)
			claims, err := svc.ValidateToken(access)
			require.NoError(t, err)
			*reader = *fake(claims.SignedInAt())

			var sawUser bool
			app := fiber.New()
			app.Use(OptionalAuthMiddleware(svc))
			app.Get("/api/v1/public/x", func(c fiber.Ctx) error {
				_, sawUser = c.Locals("user_id").(uuid.UUID)
				return c.SendStatus(fiber.StatusOK)
			})
			status, _ := passwordChangeTestCall(t, app, "GET", "/api/v1/public/x", access)
			require.Equal(t, fiber.StatusOK, status)
			assert.Equal(t, name == "no change", sawUser)
		})
	}
}
