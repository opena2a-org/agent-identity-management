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

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
)

// The logout contract for a client that holds the user pair outside a
// browser (the Python SDK after `aim-sdk login`), and now for the dashboard too:
// the refresh token travels in a JSON body, the route revokes it together with
// the bearer, and the answer reports what was written to the denylist so the
// client can tell the truth about it.

// logoutMemStore is a test double for auth.RevocationStore.
type logoutMemStore struct{ m map[string]bool }

func (s *logoutMemStore) Exists(_ context.Context, key string) (bool, error) { return s.m[key], nil }
func (s *logoutMemStore) Set(_ context.Context, key string, _ interface{}, _ time.Duration) error {
	if s.m == nil {
		s.m = map[string]bool{}
	}
	s.m[key] = true
	return nil
}

func logoutTestService(t *testing.T, withRevoker bool) *auth.JWTService {
	t.Helper()
	t.Setenv("JWT_SECRET", "test-only-jwt-secret-not-a-real-value-0123456789")
	svc := auth.NewJWTService()
	if withRevoker {
		svc.SetRevoker(auth.NewTokenRevoker(&logoutMemStore{}, false))
	}
	return svc
}

func logoutTestPair(t *testing.T, svc *auth.JWTService) (access, refresh string) {
	t.Helper()
	access, refresh, err := svc.GenerateTokenPair(uuid.New().String(), uuid.New().String(), "dev@example.com", "admin")
	require.NoError(t, err)
	return access, refresh
}

func jtiOf(t *testing.T, svc *auth.JWTService, token string) string {
	t.Helper()
	id, err := svc.GetTokenID(token)
	require.NoError(t, err)
	return id
}

type logoutAnswer struct {
	Message string          `json:"message"`
	Revoked map[string]bool `json:"revoked"`
}

func postLogout(t *testing.T, h *AuthHandler, bearer, body, cookie string) (*fiber.App, *httptest.ResponseRecorder, logoutAnswer, int) {
	t.Helper()
	app := fiber.New()
	app.Post("/api/v1/auth/logout", h.Logout)
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest("POST", "/api/v1/auth/logout", reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("User-Agent", "logout-cell/1")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	if cookie != "" {
		req.Header.Set("Cookie", "refresh_token="+cookie)
	}
	resp, err := app.Test(req)
	require.NoError(t, err)
	raw, _ := io.ReadAll(resp.Body)
	var out logoutAnswer
	_ = json.Unmarshal(raw, &out)
	rec := httptest.NewRecorder()
	for k, vs := range resp.Header {
		for _, v := range vs {
			rec.Header().Add(k, v)
		}
	}
	return app, rec, out, resp.StatusCode
}

// C1: the refresh token in the body is revoked alongside the bearer, and the
// answer reports both writes.
func TestAuthHandler_Logout_RevokesBodyRefreshToken(t *testing.T) {
	svc := logoutTestService(t, true)
	access, refresh := logoutTestPair(t, svc)
	h := &AuthHandler{jwtService: svc}

	_, _, out, status := postLogout(t, h, access, `{"refreshToken":"`+refresh+`"}`, "")
	assert.Equal(t, fiber.StatusOK, status)
	assert.True(t, svc.IsRevoked(context.Background(), jtiOf(t, svc, access)), "the bearer's jti is denylisted")
	assert.True(t, svc.IsRevoked(context.Background(), jtiOf(t, svc, refresh)), "the body refresh token's jti is denylisted")
	assert.Equal(t, "Logged out successfully", out.Message)
	assert.True(t, out.Revoked["accessToken"])
	assert.True(t, out.Revoked["refreshToken"])
}

// C2: after that logout the refresh route refuses the token.
func TestAuthHandler_Logout_ThenRefreshIs401(t *testing.T) {
	svc := logoutTestService(t, true)
	access, refresh := logoutTestPair(t, svc)
	h := &AuthHandler{jwtService: svc}
	_, _, _, status := postLogout(t, h, access, `{"refreshToken":"`+refresh+`"}`, "")
	require.Equal(t, fiber.StatusOK, status)

	rh := &AuthRefreshHandler{jwtService: svc, users: &refreshTestUserRepo{}}
	app := fiber.New()
	app.Post("/api/v1/auth/refresh", rh.RefreshToken)
	req := httptest.NewRequest("POST", "/api/v1/auth/refresh", strings.NewReader(`{"refreshToken":"`+refresh+`"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	raw, _ := io.ReadAll(resp.Body)
	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
	assert.Contains(t, string(raw), "Token has been revoked or is invalid")
}

// C3: cookies are not credentials. A logout that presents the pair only as the
// access_token and refresh_token cookies revokes nothing, and the cookie's refresh
// token still refreshes. Both cookies are still cleared, for browsers that kept
// them from an earlier version. Control in the same test: the bearer plus the body
// revokes both.
func TestAuthHandler_Logout_CookiesAreNotCredentialsButAreCleared(t *testing.T) {
	store := &logoutMemStore{}
	svc, refreshApp, access, refresh := logoutFamilyFixture(t, store)
	h := &AuthHandler{jwtService: svc}

	app := fiber.New()
	app.Post("/api/v1/auth/logout", h.Logout)
	req := httptest.NewRequest("POST", "/api/v1/auth/logout", nil)
	req.Header.Set("Cookie", "access_token="+access+"; refresh_token="+refresh)
	resp, err := app.Test(req)
	require.NoError(t, err)
	raw, _ := io.ReadAll(resp.Body)
	var out logoutAnswer
	require.NoError(t, json.Unmarshal(raw, &out))

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.False(t, out.Revoked["accessToken"], "a cookie is not a credential")
	assert.False(t, out.Revoked["refreshToken"], "a cookie is not a credential")
	assert.False(t, svc.IsRevoked(context.Background(), jtiOf(t, svc, access)))
	assert.False(t, svc.IsRevoked(context.Background(), jtiOf(t, svc, refresh)))
	setCookies := strings.Join(resp.Header.Values("Set-Cookie"), "\n")
	assert.Contains(t, setCookies, "access_token=", "the earlier access_token cookie is cleared")
	assert.Contains(t, setCookies, "refresh_token=", "the earlier refresh_token cookie is cleared")

	refreshed, status := postRefresh(t, refreshApp, refresh)
	require.Equal(t, fiber.StatusOK, status, "the cookie's refresh token still refreshes")

	_, _, control, status := postLogout(t, h, refreshed.AccessToken, `{"refreshToken":"`+refreshed.RefreshToken+`"}`, "")
	assert.Equal(t, fiber.StatusOK, status)
	assert.True(t, control.Revoked["accessToken"], "control: the bearer is revoked")
	assert.True(t, control.Revoked["refreshToken"], "control: the body refresh token is revoked")
}

// C4: the refresh token in a cookie is ignored; only the body's is revoked.
func TestAuthHandler_Logout_CookieRefreshTokenIsIgnored(t *testing.T) {
	svc := logoutTestService(t, true)
	access, fromBody := logoutTestPair(t, svc)
	_, fromCookie := logoutTestPair(t, svc)
	h := &AuthHandler{jwtService: svc}

	_, _, out, status := postLogout(t, h, access, `{"refreshToken":"`+fromBody+`"}`, fromCookie)
	assert.Equal(t, fiber.StatusOK, status)
	assert.True(t, svc.IsRevoked(context.Background(), jtiOf(t, svc, fromBody)))
	assert.False(t, svc.IsRevoked(context.Background(), jtiOf(t, svc, fromCookie)))
	assert.True(t, out.Revoked["refreshToken"])
}

// C5: the report is false whenever nothing was written to the denylist: no
// service wired, garbage tokens, or a service without a revoker.
func TestAuthHandler_Logout_ReportsFalseWhenNothingWasRevoked(t *testing.T) {
	t.Run("no service, no tokens", func(t *testing.T) {
		_, _, out, status := postLogout(t, &AuthHandler{}, "", "", "")
		assert.Equal(t, fiber.StatusOK, status)
		assert.Equal(t, "Logged out successfully", out.Message)
		assert.False(t, out.Revoked["accessToken"])
		assert.False(t, out.Revoked["refreshToken"])
	})
	t.Run("garbage tokens", func(t *testing.T) {
		svc := logoutTestService(t, true)
		_, _, out, status := postLogout(t, &AuthHandler{jwtService: svc}, "not.a.jwt", `{"refreshToken":"not.a.jwt"}`, "")
		assert.Equal(t, fiber.StatusOK, status)
		assert.False(t, out.Revoked["accessToken"])
		assert.False(t, out.Revoked["refreshToken"])
	})
	t.Run("no revoker configured", func(t *testing.T) {
		svc := logoutTestService(t, false)
		access, refresh := logoutTestPair(t, svc)
		_, _, out, status := postLogout(t, &AuthHandler{jwtService: svc}, access, `{"refreshToken":"`+refresh+`"}`, "")
		assert.Equal(t, fiber.StatusOK, status)
		assert.False(t, out.Revoked["accessToken"])
		assert.False(t, out.Revoked["refreshToken"])
	})
}

// C6: a malformed body is "no body token", not an error: the bearer is still
// revoked and no refresh token is.
func TestAuthHandler_Logout_MalformedBodyIsNotAnError(t *testing.T) {
	svc := logoutTestService(t, true)
	access, refresh := logoutTestPair(t, svc)
	h := &AuthHandler{jwtService: svc}

	_, _, out, status := postLogout(t, h, access, `{not json`, "")
	assert.Equal(t, fiber.StatusOK, status)
	assert.True(t, svc.IsRevoked(context.Background(), jtiOf(t, svc, access)))
	assert.True(t, out.Revoked["accessToken"])
	assert.False(t, svc.IsRevoked(context.Background(), jtiOf(t, svc, refresh)))
	assert.False(t, out.Revoked["refreshToken"])
}
