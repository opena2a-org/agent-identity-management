package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
)

// Refresh token rotation for login-issued pairs: the presented token is
// retired (its jti denylisted for its remaining lifetime) and a new one
// issued; a refresh token is rotated only when the old one was actually
// retired, so a login chain never holds two live refresh tokens. Without a
// revocation store, or when the store refuses the write, the presented token
// comes back unchanged with a fresh access token and rotated=false.

// rotationStore is a RevocationStore double whose writes can be made to fail.
type rotationStore struct {
	data   map[string]bool
	setErr error
	writes int
}

func (s *rotationStore) Exists(_ context.Context, key string) (bool, error) { return s.data[key], nil }
func (s *rotationStore) Set(_ context.Context, key string, _ interface{}, _ time.Duration) error {
	if s.setErr != nil {
		return s.setErr
	}
	if s.data == nil {
		s.data = map[string]bool{}
	}
	s.data[key] = true
	s.writes++
	return nil
}

// rotationSDKRepo records what the SDK-token path does to the sdk_tokens table.
type rotationSDKRepo struct {
	domain.SDKTokenRepository
	token   *domain.SDKToken
	revoked []struct{ hash, reason string }
	created []*domain.SDKToken
}

func (r *rotationSDKRepo) GetByTokenHash(hash string) (*domain.SDKToken, error) {
	if r.token == nil {
		return nil, fmt.Errorf("not tracked")
	}
	return r.token, nil
}
func (r *rotationSDKRepo) RecordUsage(string, string) error { return nil }
func (r *rotationSDKRepo) RevokeByTokenHash(hash, reason string) error {
	r.revoked = append(r.revoked, struct{ hash, reason string }{hash, reason})
	return nil
}
func (r *rotationSDKRepo) Create(token *domain.SDKToken) error {
	r.created = append(r.created, token)
	return nil
}

func rotationApp(t *testing.T, store *rotationStore, failOpen bool) (*fiber.App, *auth.JWTService, string) {
	t.Helper()
	userID, orgID := uuid.New(), uuid.New()
	users := &refreshTestUserRepo{getByID: func(uuid.UUID) (*domain.User, error) {
		return activeUser(userID, orgID, domain.RoleAdmin, "rotate@example.com"), nil
	}}
	app, jwtSvc := newRefreshTestApp(t, users, nil)
	if store != nil {
		jwtSvc.SetRevoker(auth.NewTokenRevoker(store, failOpen))
	}
	_, refresh, err := jwtSvc.GenerateTokenPair(userID.String(), orgID.String(), "rotate@example.com", "admin")
	require.NoError(t, err)
	return app, jwtSvc, refresh
}

func postRefreshRaw(t *testing.T, app *fiber.App, refreshToken string) (string, int) {
	t.Helper()
	req := httptest.NewRequest("POST", "/auth/refresh", strings.NewReader(fmt.Sprintf(`{"refreshToken":%q}`, refreshToken)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return string(body), resp.StatusCode
}

func jtiOfToken(t *testing.T, svc *auth.JWTService, token string) string {
	t.Helper()
	id, err := svc.GetTokenID(token)
	require.NoError(t, err)
	return id
}

// R1: a rotated login refresh token is refused on its next use.
func TestRefreshToken_RotatedLoginTokenIsRefused(t *testing.T) {
	store := &rotationStore{}
	app, jwtSvc, p1 := rotationApp(t, store, false)

	out, status := postRefresh(t, app, p1)
	require.Equal(t, fiber.StatusOK, status)
	assert.NotEqual(t, p1, out.RefreshToken, "a new refresh token is issued")
	assert.True(t, out.Rotated)
	assert.True(t, jwtSvc.IsRevoked(context.Background(), jtiOfToken(t, jwtSvc, p1)), "the old jti is denylisted")

	body, status := postRefreshRaw(t, app, p1)
	assert.Equal(t, fiber.StatusUnauthorized, status)
	assert.Contains(t, body, "Token has been revoked or is invalid")
	assert.NotContains(t, body, "accessToken")

	_, status = postRefresh(t, app, out.RefreshToken)
	assert.Equal(t, fiber.StatusOK, status, "the new token works")
}

// R2: rotation chains, and every retired token stays dead.
func TestRefreshToken_RotationChainsAndEachOldTokenDies(t *testing.T) {
	store := &rotationStore{}
	app, jwtSvc, p1 := rotationApp(t, store, false)
	out2, status := postRefresh(t, app, p1)
	require.Equal(t, fiber.StatusOK, status)
	out3, status := postRefresh(t, app, out2.RefreshToken)
	require.Equal(t, fiber.StatusOK, status)

	_, status = postRefresh(t, app, p1)
	assert.Equal(t, fiber.StatusUnauthorized, status)
	_, status = postRefresh(t, app, out2.RefreshToken)
	assert.Equal(t, fiber.StatusUnauthorized, status)
	// The newest token is live until it is itself presented and rotated.
	assert.False(t, jwtSvc.IsRevoked(context.Background(), jtiOfToken(t, jwtSvc, out3.RefreshToken)))
	_, status = postRefresh(t, app, out3.RefreshToken)
	assert.Equal(t, fiber.StatusOK, status)
	assert.True(t, jwtSvc.IsRevoked(context.Background(), jtiOfToken(t, jwtSvc, out3.RefreshToken)), "and retired once presented")
}

// R3: without a revocation store nothing can be retired, so nothing is
// rotated: a fresh access token, the presented refresh token unchanged.
func TestRefreshToken_NoRevokerDoesNotRotate(t *testing.T) {
	app, jwtSvc, p1 := rotationApp(t, nil, false)
	out, status := postRefresh(t, app, p1)
	require.Equal(t, fiber.StatusOK, status)
	assert.Equal(t, p1, out.RefreshToken, "the presented refresh token comes back unchanged")
	assert.False(t, out.Rotated)
	claims, err := jwtSvc.ValidateToken(out.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, auth.TokenTypeAccess, claims.TokenType)
	role, email := decodeRoleEmail(t, out.AccessToken)
	assert.Equal(t, "admin", role)
	assert.Equal(t, "rotate@example.com", email)
}

// R4: a store that refuses the write does not rotate; when the store
// recovers, rotation resumes and the old token dies. Same under both
// fail-open settings.
func TestRefreshToken_StoreWriteFailureDoesNotRotateAndRotationResumes(t *testing.T) {
	for _, failOpen := range []bool{false, true} {
		t.Run(fmt.Sprintf("failOpen=%v", failOpen), func(t *testing.T) {
			store := &rotationStore{setErr: errors.New("store down")}
			app, jwtSvc, p1 := rotationApp(t, store, failOpen)
			for i := 0; i < 3; i++ {
				out, status := postRefresh(t, app, p1)
				require.Equal(t, fiber.StatusOK, status)
				assert.Equal(t, p1, out.RefreshToken)
				assert.False(t, out.Rotated)
				assert.NotEmpty(t, out.AccessToken)
			}
			assert.Equal(t, 0, store.writes, "no denylist key is written while the store refuses")

			store.setErr = nil
			out, status := postRefresh(t, app, p1)
			require.Equal(t, fiber.StatusOK, status)
			assert.NotEqual(t, p1, out.RefreshToken)
			assert.True(t, out.Rotated)
			assert.True(t, jwtSvc.IsRevoked(context.Background(), jtiOfToken(t, jwtSvc, p1)))
			_, status = postRefresh(t, app, p1)
			assert.Equal(t, fiber.StatusUnauthorized, status)
		})
	}
}

// R5: the SDK-token path is unchanged: the old token is retired by row in
// sdk_tokens (one revoke with the old hash, reason token_rotation), a new row
// is created for the new token, and the JTI denylist is not used for it.
func TestRefreshToken_SDKTokenPathUnchanged(t *testing.T) {
	userID, orgID := uuid.New(), uuid.New()
	users := &refreshTestUserRepo{getByID: func(uuid.UUID) (*domain.User, error) {
		return activeUser(userID, orgID, domain.RoleAdmin, "sdk@example.com"), nil
	}}
	sdkRepo := &rotationSDKRepo{token: &domain.SDKToken{
		ID: uuid.New(), UserID: userID, OrganizationID: orgID, ExpiresAt: time.Now().Add(24 * time.Hour),
	}}
	app, jwtSvc := newRefreshTestApp(t, users, sdkRepo)
	store := &rotationStore{}
	jwtSvc.SetRevoker(auth.NewTokenRevoker(store, false))
	old, err := jwtSvc.GenerateSDKRefreshToken(userID.String(), orgID.String(), "sdk@example.com", "admin")
	require.NoError(t, err)

	out, status := postRefresh(t, app, old)
	require.Equal(t, fiber.StatusOK, status)
	assert.NotEqual(t, old, out.RefreshToken)
	assert.True(t, out.Rotated)

	sum := sha256.Sum256([]byte(old))
	require.Len(t, sdkRepo.revoked, 1)
	assert.Equal(t, hex.EncodeToString(sum[:]), sdkRepo.revoked[0].hash)
	assert.Equal(t, "token_rotation", sdkRepo.revoked[0].reason)
	require.Len(t, sdkRepo.created, 1)
	assert.Equal(t, jtiOfToken(t, jwtSvc, out.RefreshToken), sdkRepo.created[0].TokenID)
	assert.False(t, jwtSvc.IsRevoked(context.Background(), jtiOfToken(t, jwtSvc, old)), "SDK tokens are retired by row, not by the jti denylist")
}
