package handlers

import (
	"encoding/json"
	"fmt"
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

// The recovery path previously minted the new pair with role "" and email ""
// (a comment claimed "will be populated from DB"; nothing did). The recovered
// pair now carries the account's CURRENT role and email, and an account that
// may not hold a session cannot recover one.

func newRecoveryTestApp(t *testing.T, users domain.UserRepository, sdkRepo domain.SDKTokenRepository) (*fiber.App, *auth.JWTService) {
	t.Helper()
	t.Setenv("JWT_SECRET", "test-secret-key-for-unit-tests-32")
	jwtSvc := auth.NewJWTService()
	h := NewSDKTokenRecoveryHandler(application.NewSDKTokenService(sdkRepo), jwtSvc, users)
	app := fiber.New()
	app.Post("/auth/sdk/recover", h.RecoverRevokedToken)
	return app, jwtSvc
}

func postRecover(t *testing.T, app *fiber.App, oldRefreshToken string) (*RecoverTokenResponse, int) {
	t.Helper()
	body := fmt.Sprintf(`{"oldRefreshToken":%q}`, oldRefreshToken)
	req := httptest.NewRequest("POST", "/auth/sdk/recover", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	var out RecoverTokenResponse
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return &out, resp.StatusCode
}

func revokedSDKToken(userID, orgID uuid.UUID) *domain.SDKToken {
	revoked := time.Now().Add(-time.Hour)
	return &domain.SDKToken{
		ID:             uuid.New(),
		UserID:         userID,
		OrganizationID: orgID,
		RevokedAt:      &revoked,
		ExpiresAt:      time.Now().Add(24 * time.Hour),
	}
}

// R1: the recovered pair carries the DB role and email (pre-fix: "" and "").
func TestRecoverRevokedToken_MintsRoleAndEmailFromUserRecord(t *testing.T) {
	userID, orgID := uuid.New(), uuid.New()
	users := &refreshTestUserRepo{getByID: func(id uuid.UUID) (*domain.User, error) {
		require.Equal(t, userID, id)
		return activeUser(userID, orgID, domain.RoleManager, "current@example.com"), nil
	}}
	sdkRepo := &refreshTestSDKRepo{getByTokenHash: func(string) (*domain.SDKToken, error) {
		return revokedSDKToken(userID, orgID), nil
	}}
	app, jwtSvc := newRecoveryTestApp(t, users, sdkRepo)
	oldRefresh, err := jwtSvc.GenerateSDKRefreshToken(userID.String(), orgID.String(), "old@example.com", "admin")
	require.NoError(t, err)

	out, status := postRecover(t, app, oldRefresh)
	require.Equal(t, fiber.StatusOK, status)
	role, email := decodeRoleEmail(t, out.AccessToken)
	assert.Equal(t, "manager", role, "recovered pair must carry the DB role, not an empty one")
	assert.Equal(t, "current@example.com", email)
}

// R2: an account that may not hold a session cannot recover one.
func TestRecoverRevokedToken_RefusedForInactiveAccount(t *testing.T) {
	userID, orgID := uuid.New(), uuid.New()
	users := &refreshTestUserRepo{getByID: func(uuid.UUID) (*domain.User, error) {
		return &domain.User{ID: userID, OrganizationID: orgID, Role: domain.RoleAdmin, Status: domain.UserStatusSuspended}, nil
	}}
	sdkRepo := &refreshTestSDKRepo{getByTokenHash: func(string) (*domain.SDKToken, error) {
		return revokedSDKToken(userID, orgID), nil
	}}
	app, jwtSvc := newRecoveryTestApp(t, users, sdkRepo)
	oldRefresh, err := jwtSvc.GenerateSDKRefreshToken(userID.String(), orgID.String(), "old@example.com", "admin")
	require.NoError(t, err)

	out, status := postRecover(t, app, oldRefresh)
	assert.Equal(t, fiber.StatusUnauthorized, status)
	assert.Empty(t, out.AccessToken)
}

// The nil-users boot refusal mirrors the refresh handler's.
func TestNewSDKTokenRecoveryHandler_NilUserRepoPanics(t *testing.T) {
	assert.Panics(t, func() {
		NewSDKTokenRecoveryHandler(nil, nil, nil)
	})
}
