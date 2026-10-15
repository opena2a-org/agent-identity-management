package middleware

import (
	"context"
	"errors"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A revoked session (token family: a reused refresh token, logout, or a
// dashboard revoke of an SDK token) ends its access tokens on every route
// behind AuthMiddleware, not only at the credential-minting routes: the
// middleware reads the access token's family beside its own id, through the
// revoker's negative cache, and answers the refusal a token revoked by its
// own id gets. On the instance that wrote the revocation the refusal is
// immediate; the cross-instance bound is pinned in the auth package.

// sessionFamilyStore is a RevocationStore double whose family-key reads can
// be made to fail on their own.
type sessionFamilyStore struct {
	keys      map[string]bool
	familyErr error
}

func (s *sessionFamilyStore) Exists(_ context.Context, key string) (bool, error) {
	if s.familyErr != nil && strings.HasPrefix(key, "revoked:fam:") {
		return false, s.familyErr
	}
	return s.keys[key], nil
}

func (s *sessionFamilyStore) Set(_ context.Context, key string, _ interface{}, _ time.Duration) error {
	if s.keys == nil {
		s.keys = map[string]bool{}
	}
	s.keys[key] = true
	return nil
}

func sessionFamilyService(t *testing.T, store *sessionFamilyStore, failOpen bool) *auth.JWTService {
	t.Helper()
	t.Setenv("JWT_SECRET", "test-secret-key-for-testing-purposes-32chars")
	svc := auth.NewJWTService()
	svc.SetRevoker(auth.NewTokenRevoker(store, failOpen))
	return svc
}

// sessionFamilyApp serves GET /api/v1/agents behind AuthMiddleware, as the
// agents group does for a dashboard session.
func sessionFamilyApp(svc *auth.JWTService) *fiber.App {
	app := fiber.New()
	app.Use(AuthMiddleware(svc))
	app.Get("/api/v1/agents", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"agents": []string{}})
	})
	return app
}

// sessionFamilyGet calls GET /api/v1/agents with the bearer token and returns
// the status and the raw body.
func sessionFamilyGet(t *testing.T, app *fiber.App, token string) (int, []byte) {
	t.Helper()
	req := httptest.NewRequest("GET", "/api/v1/agents", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, body
}

// revokeSessionOf ends the family of the given refresh token (login or
// SDK-download), as reuse detection, logout and the dashboard's SDK token
// revoke do. The access token's own id is not denylisted.
func revokeSessionOf(t *testing.T, svc *auth.JWTService, refreshToken string) {
	t.Helper()
	claims, err := svc.ValidateToken(refreshToken)
	require.NoError(t, err)
	written, err := svc.RevokeFamily(context.Background(), claims)
	require.NoError(t, err)
	require.True(t, written, "the family key is written")
}

// C1: a login access token whose session was revoked, and the access token
// the SDK refresh path minted for a revoked SDK-download family, are refused
// on GET /api/v1/agents with the body a jti-revoked token gets, byte for
// byte; each is served before its revocation (control), and another
// session's token is served after it.
func TestAuthMiddleware_RefusesAnAccessTokenWhoseSessionWasRevoked(t *testing.T) {
	store := &sessionFamilyStore{}
	svc := sessionFamilyService(t, store, false)
	app := sessionFamilyApp(svc)
	ctx := context.Background()
	userID, orgID := uuid.New().String(), uuid.New().String()

	// The reference refusal: a token revoked by its own id.
	byJTI, _, err := svc.GenerateTokenPair(userID, orgID, "c1@example.com", "admin")
	require.NoError(t, err)
	require.NoError(t, svc.RevokeToken(ctx, byJTI))
	status, jtiBody := sessionFamilyGet(t, app, byJTI)
	require.Equal(t, fiber.StatusUnauthorized, status)
	require.JSONEq(t, `{"error":"Token has been revoked"}`, string(jtiBody))

	other, _, err := svc.GenerateTokenPair(userID, orgID, "c1@example.com", "admin")
	require.NoError(t, err)

	t.Run("login access token", func(t *testing.T) {
		access, refresh, err := svc.GenerateTokenPair(userID, orgID, "c1@example.com", "admin")
		require.NoError(t, err)
		status, _ := sessionFamilyGet(t, app, access)
		require.Equal(t, fiber.StatusOK, status, "control: served before its session is revoked")

		revokeSessionOf(t, svc, refresh)
		jti, err := svc.GetTokenID(access)
		require.NoError(t, err)
		require.False(t, store.keys["revoked:jti:"+jti], "only the family is revoked")

		status, body := sessionFamilyGet(t, app, access)
		assert.Equal(t, fiber.StatusUnauthorized, status, "refused right after its session is revoked")
		assert.Equal(t, jtiBody, body, "with the body a jti-revoked token gets")

		status, _ = sessionFamilyGet(t, app, other)
		assert.Equal(t, fiber.StatusOK, status, "another session's token is served")
	})

	t.Run("access token minted by the SDK refresh path", func(t *testing.T) {
		sdk, err := svc.GenerateSDKRefreshToken(userID, orgID, "c1@example.com", "admin")
		require.NoError(t, err)
		access, _, err := svc.RefreshTokenPair(sdk, "c1@example.com", "admin")
		require.NoError(t, err)
		claims, err := svc.ValidateToken(access)
		require.NoError(t, err)
		require.NotEmpty(t, claims.AccessFamilyID(), "the SDK path's access token carries its download's family")
		status, _ := sessionFamilyGet(t, app, access)
		require.Equal(t, fiber.StatusOK, status, "control: served before its family is revoked")

		revokeSessionOf(t, svc, sdk)

		status, body := sessionFamilyGet(t, app, access)
		assert.Equal(t, fiber.StatusUnauthorized, status, "refused right after its family is revoked")
		assert.Equal(t, jtiBody, body)
	})
}

// C3: a store error on the family read answers as the jti read does: refused
// when the revoker fails closed, served when it fails open. A token with no
// family reads no family key, so the error never reaches it.
func TestAuthMiddleware_FamilyReadErrorFollowsTheFailOpenSetting(t *testing.T) {
	for _, tc := range []struct {
		name     string
		failOpen bool
		want     int
	}{
		{"fail closed", false, fiber.StatusUnauthorized},
		{"fail open", true, fiber.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &sessionFamilyStore{familyErr: errors.New("store down")}
			svc := sessionFamilyService(t, store, tc.failOpen)
			app := sessionFamilyApp(svc)
			userID, orgID := uuid.New().String(), uuid.New().String()

			access, _, err := svc.GenerateTokenPair(userID, orgID, "c3@example.com", "admin")
			require.NoError(t, err)
			status, body := sessionFamilyGet(t, app, access)
			assert.Equal(t, tc.want, status)
			if tc.want == fiber.StatusUnauthorized {
				assert.JSONEq(t, `{"error":"Token has been revoked"}`, string(body))
			}

			standalone, err := svc.GenerateAccessToken(userID, orgID, "c3@example.com", "admin")
			require.NoError(t, err)
			status, _ = sessionFamilyGet(t, app, standalone)
			assert.Equal(t, fiber.StatusOK, status, "a token with no family reads its id key only")
		})
	}
}

// C5: an optional-auth route serves a token whose session was revoked as an
// anonymous request, as it serves a jti-revoked token; before the revocation
// the same token sets the user (control).
func TestOptionalAuthMiddleware_RevokedSessionIsAnonymous(t *testing.T) {
	store := &sessionFamilyStore{}
	svc := sessionFamilyService(t, store, false)
	access, refresh, err := svc.GenerateTokenPair(uuid.New().String(), uuid.New().String(), "c5@example.com", "admin")
	require.NoError(t, err)

	var hasUserID bool
	app := fiber.New()
	app.Use(OptionalAuthMiddleware(svc))
	app.Get("/public", func(c fiber.Ctx) error {
		hasUserID = c.Locals("user_id") != nil
		return c.JSON(fiber.Map{"ok": true})
	})
	call := func() int {
		req := httptest.NewRequest("GET", "/public", nil)
		req.Header.Set("Authorization", "Bearer "+access)
		resp, err := app.Test(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		return resp.StatusCode
	}

	require.Equal(t, fiber.StatusOK, call())
	require.True(t, hasUserID, "control: the token sets the user before its session is revoked")

	revokeSessionOf(t, svc, refresh)

	assert.Equal(t, fiber.StatusOK, call(), "the route still answers")
	assert.False(t, hasUserID, "but the revoked session sets no user")
}
