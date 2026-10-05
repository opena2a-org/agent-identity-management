package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
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

// An SDK-download token is tracked by row in sdk_tokens, and the dashboard
// lists that row. Logout used to denylist the token only: the refresh route
// refused it while the dashboard still listed it as active. Logout now
// retires the row too, and the answer claims the revocation only when it did.

// sdkRowRepo is an in-memory sdk_tokens table keyed by token hash.
type sdkRowRepo struct {
	domain.SDKTokenRepository
	rows      map[string]*domain.SDKToken
	revokeErr error
	revokes   int
}

func (r *sdkRowRepo) GetByTokenHash(hash string) (*domain.SDKToken, error) {
	if row, ok := r.rows[hash]; ok {
		return row, nil
	}
	return nil, errors.New("sdk token not found")
}

func (r *sdkRowRepo) RevokeByTokenHash(hash string, reason string) error {
	r.revokes++
	if r.revokeErr != nil {
		return r.revokeErr
	}
	row, ok := r.rows[hash]
	if !ok || row.RevokedAt != nil {
		return errors.New("SDK token not found or already revoked")
	}
	row.Revoke(reason)
	return nil
}

// RevokeFamily revokes the family's download row; these fixtures track one
// download and no row rotated from it.
func (r *sdkRowRepo) RevokeFamily(userID uuid.UUID, familyID string, reason string) error {
	for _, row := range r.rows {
		if row.UserID == userID && row.TokenID == familyID && row.RevokedAt == nil {
			row.Revoke(reason)
		}
	}
	return nil
}

func (r *sdkRowRepo) GetByUserID(userID uuid.UUID, includeRevoked bool) ([]*domain.SDKToken, error) {
	var out []*domain.SDKToken
	for _, row := range r.rows {
		if row.UserID == userID && (includeRevoked || row.RevokedAt == nil) {
			out = append(out, row)
		}
	}
	return out, nil
}

func sdkRowHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

type sdkLogoutFixture struct {
	svc    *auth.JWTService
	repo   *sdkRowRepo
	tokens *application.SDKTokenService
	userID uuid.UUID
	token  string
}

// newSDKLogoutFixture issues an SDK-download token and tracks it by an active
// row, as the SDK download route does.
func newSDKLogoutFixture(t *testing.T) *sdkLogoutFixture {
	t.Helper()
	t.Setenv("JWT_SECRET", "test-only-jwt-secret-not-a-real-value-0123456789")
	svc := auth.NewJWTService()
	svc.SetRevoker(auth.NewTokenRevoker(&logoutMemStore{}, false))
	userID, orgID := uuid.New(), uuid.New()
	token, err := svc.GenerateSDKRefreshToken(userID.String(), orgID.String(), "sdk@example.com", "admin")
	require.NoError(t, err)
	repo := &sdkRowRepo{rows: map[string]*domain.SDKToken{
		sdkRowHash(token): {
			ID:             uuid.New(),
			UserID:         userID,
			OrganizationID: orgID,
			TokenHash:      sdkRowHash(token),
			TokenID:        jtiOf(t, svc, token),
			CreatedAt:      time.Now(),
			ExpiresAt:      time.Now().Add(90 * 24 * time.Hour),
		},
	}}
	return &sdkLogoutFixture{svc: svc, repo: repo, tokens: application.NewSDKTokenService(repo), userID: userID, token: token}
}

func (f *sdkLogoutFixture) handler() *AuthHandler {
	return NewAuthHandler(nil, f.svc, nil, nil, f.tokens)
}

func (f *sdkLogoutFixture) activeRows(t *testing.T) []*domain.SDKToken {
	t.Helper()
	rows, err := f.tokens.GetUserTokens(context.Background(), f.userID, false)
	require.NoError(t, err)
	return rows
}

// S1: logout of an SDK-download token retires its row, so the dashboard list
// agrees with the denylist and the refresh route.
func TestAuthHandler_Logout_SDKTokenRetiresItsRow(t *testing.T) {
	f := newSDKLogoutFixture(t)
	require.Len(t, f.activeRows(t), 1)

	_, _, out, status := postLogout(t, f.handler(), "", `{"refreshToken":"`+f.token+`"}`, "")
	require.Equal(t, fiber.StatusOK, status)
	assert.True(t, out.Revoked["refreshToken"])
	assert.True(t, f.svc.IsRevoked(context.Background(), jtiOf(t, f.svc, f.token)), "the token's jti is denylisted")
	assert.Empty(t, f.activeRows(t), "the dashboard lists no active row for the logged-out token")
	row := f.repo.rows[sdkRowHash(f.token)]
	require.NotNil(t, row.RevokeReason)
	assert.Equal(t, "logout", *row.RevokeReason)

	rh := &AuthRefreshHandler{jwtService: f.svc, sdkTokenService: f.tokens, users: &refreshTestUserRepo{}}
	app := fiber.New()
	app.Post("/auth/refresh", rh.RefreshToken)
	_, status = postRefresh(t, app, f.token)
	assert.Equal(t, fiber.StatusUnauthorized, status, "the logged-out token is refused at refresh")
}

// S2: when the row write fails, the answer does not claim a revocation,
// although the token's jti was denylisted.
func TestAuthHandler_Logout_SDKTokenReportsFalseWhenTheRowWriteFails(t *testing.T) {
	f := newSDKLogoutFixture(t)
	f.repo.revokeErr = errors.New("database unavailable")

	_, _, out, status := postLogout(t, f.handler(), "", `{"refreshToken":"`+f.token+`"}`, "")
	require.Equal(t, fiber.StatusOK, status)
	assert.False(t, out.Revoked["refreshToken"], "no claim of a revocation the row does not show")
	assert.True(t, f.svc.IsRevoked(context.Background(), jtiOf(t, f.svc, f.token)), "the token itself is denylisted")
}

// S3: a row already retired by rotation is the state logout wants: it is kept
// with its reason, and the answer reports the revocation.
func TestAuthHandler_Logout_SDKTokenRowAlreadyRetiredIsKept(t *testing.T) {
	f := newSDKLogoutFixture(t)
	f.repo.rows[sdkRowHash(f.token)].Revoke("token_rotation")

	_, _, out, status := postLogout(t, f.handler(), "", `{"refreshToken":"`+f.token+`"}`, "")
	require.Equal(t, fiber.StatusOK, status)
	assert.True(t, out.Revoked["refreshToken"])
	assert.Equal(t, "token_rotation", *f.repo.rows[sdkRowHash(f.token)].RevokeReason)
}

// S4: a login refresh token is not tracked in sdk_tokens; its logout writes no
// row and still reports the session revoked.
func TestAuthHandler_Logout_LoginTokenTouchesNoSDKRow(t *testing.T) {
	f := newSDKLogoutFixture(t)
	access, refresh := logoutTestPair(t, f.svc)

	_, _, out, status := postLogout(t, f.handler(), access, `{"refreshToken":"`+refresh+`"}`, "")
	require.Equal(t, fiber.StatusOK, status)
	assert.True(t, out.Revoked["refreshToken"])
	assert.Zero(t, f.repo.revokes, "no sdk_tokens write for a login token")
	assert.Len(t, f.activeRows(t), 1, "the SDK-download token's row is untouched")
}
