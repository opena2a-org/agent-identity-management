package handlers

import (
	"context"
	"errors"
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

// Logout ends the presented refresh token's whole sign-in. A browser sends
// the refresh_token cookie set at login, which after any rotation is a
// retired token; revoking only that jti ended nothing, and the refreshed
// token stayed valid until it expired. The answer claims a revocation only
// when the session write also succeeded.

// familyKeyFailingStore refuses writes of family keys only.
type familyKeyFailingStore struct{ logoutMemStore }

func (s *familyKeyFailingStore) Set(ctx context.Context, key string, v interface{}, ttl time.Duration) error {
	if strings.HasPrefix(key, "revoked:fam:") {
		return errors.New("family write refused")
	}
	return s.logoutMemStore.Set(ctx, key, v, ttl)
}

func logoutFamilyFixture(t *testing.T, store auth.RevocationStore) (*auth.JWTService, *fiber.App, string, string) {
	t.Helper()
	t.Setenv("JWT_SECRET", "test-only-jwt-secret-not-a-real-value-0123456789")
	svc := auth.NewJWTService()
	svc.SetRevoker(auth.NewTokenRevoker(store, false))
	userID, orgID := uuid.New(), uuid.New()
	users := &refreshTestUserRepo{getByID: func(uuid.UUID) (*domain.User, error) {
		return activeUser(userID, orgID, domain.RoleAdmin, "logout@example.com"), nil
	}}
	rh := NewAuthRefreshHandler(svc, application.NewSDKTokenService(&refreshTestSDKRepo{}), users, application.NewAuditService(&familyAuditRepo{}))
	app := fiber.New()
	app.Post("/auth/refresh", rh.RefreshToken)
	access, refresh, err := svc.GenerateTokenPair(userID.String(), orgID.String(), "logout@example.com", "admin")
	require.NoError(t, err)
	return svc, app, access, refresh
}

// L1: a dashboard-shaped logout (bearer plus the refreshed token in the body, no
// cookie) after one refresh ends the session: the refreshed token is refused.
func TestAuthHandler_Logout_EndsTheWholeSession(t *testing.T) {
	store := &logoutMemStore{}
	svc, app, _, p1 := logoutFamilyFixture(t, store)
	out, status := postRefresh(t, app, p1)
	require.Equal(t, fiber.StatusOK, status)
	require.True(t, out.Rotated)
	p2, a2 := out.RefreshToken, out.AccessToken

	_, _, answer, status := postLogout(t, &AuthHandler{jwtService: svc}, a2, `{"refreshToken":"`+p2+`"}`, "")
	require.Equal(t, fiber.StatusOK, status)
	assert.True(t, answer.Revoked["refreshToken"])

	_, status = postRefresh(t, app, p2)
	assert.Equal(t, fiber.StatusUnauthorized, status, "the refreshed token is refused after logout")
	assert.True(t, store.m["revoked:fam:"+jtiOf(t, svc, p1)], "the family key is written under the login token's jti")
}

// L2: when the session write fails, the answer does not claim a revocation,
// although the presented token's own jti was denylisted.
func TestAuthHandler_Logout_ReportsFalseWhenTheSessionWriteFails(t *testing.T) {
	store := &familyKeyFailingStore{}
	svc, _, access, p1 := logoutFamilyFixture(t, store)
	_, _, answer, status := postLogout(t, &AuthHandler{jwtService: svc}, access, `{"refreshToken":"`+p1+`"}`, "")
	require.Equal(t, fiber.StatusOK, status)
	assert.False(t, answer.Revoked["refreshToken"], "no claim of a revocation the store did not complete")
	assert.True(t, svc.IsRevoked(context.Background(), jtiOf(t, svc, p1)), "the presented token itself is denylisted")
}
