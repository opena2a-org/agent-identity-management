package handlers

import (
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

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/interfaces/http/middleware"
)

// A dashboard revoke of an SDK token (one row, or revoke-all) ends the row's
// token family as a reuse or a logout does: the family key is written under
// the download token's jti, which the row's rotation lineage leads to, so the
// access token the SDK path minted for that family is refused at the
// credential-minting routes (/sdk/download) at once instead of until it
// expires, and the family's refresh token is refused at /auth/refresh.

// The dashboard's revoke routes read the row by id and the user's rows; the
// sdk_tokens model of the family fixture answers those reads as the
// repository does, by copy.
func (r *sdkFamilyRepo) GetByID(id uuid.UUID) (*domain.SDKToken, error) {
	for _, row := range r.rows {
		if row.ID == id {
			cp := *row
			return &cp, nil
		}
	}
	return nil, fmt.Errorf("not tracked")
}

func (r *sdkFamilyRepo) GetByUserID(userID uuid.UUID, includeRevoked bool) ([]*domain.SDKToken, error) {
	out := make([]*domain.SDKToken, 0)
	for _, row := range r.rows {
		if row.UserID != userID || (!includeRevoked && row.RevokedAt != nil) {
			continue
		}
		cp := *row
		out = append(out, &cp)
	}
	return out, nil
}

func (r *sdkFamilyRepo) RevokeAllForUser(userID uuid.UUID, reason string) error {
	for _, row := range r.rows {
		if row.UserID == userID && row.RevokedAt == nil {
			row.Revoke(reason)
		}
	}
	return nil
}

// dashboardRevokeApps wires the dashboard's two revoke routes, acting as the
// fixture's user, and a download route behind the real auth middleware, so a
// download is made with a bearer access token as an SDK client makes it.
type dashboardRevokeApps struct {
	dashboard *fiber.App
	download  *fiber.App
}

func newDashboardRevokeApps(t *testing.T, f *sdkFamilyFixture) *dashboardRevokeApps {
	t.Helper()
	h := NewSDKTokenHandler(application.NewSDKTokenService(f.repo), f.svc)
	asUser := func(next fiber.Handler) fiber.Handler {
		return func(c fiber.Ctx) error {
			c.Locals("user_id", f.userID)
			return next(c)
		}
	}
	dashboard := fiber.New()
	dashboard.Post("/users/me/sdk-tokens/:id/revoke", asUser(h.RevokeToken))
	dashboard.Post("/users/me/sdk-tokens/revoke-all", asUser(h.RevokeAllTokens))

	dh := NewSDKHandler(f.svc, f.repo, application.NewAuditService(f.audit))
	download := fiber.New()
	download.Use(middleware.AuthMiddleware(f.svc))
	download.Get("/sdk/download", dh.DownloadSDK)
	return &dashboardRevokeApps{dashboard: dashboard, download: download}
}

// revokeRow posts the dashboard revoke of the row that holds refreshToken.
func (a *dashboardRevokeApps) revokeRow(t *testing.T, f *sdkFamilyFixture, refreshToken string) (int, string) {
	t.Helper()
	row := f.repo.row(refreshToken)
	require.NotNil(t, row, "the token has a row")
	return doRequest(t, a.dashboard, "POST", "/users/me/sdk-tokens/"+row.ID.String()+"/revoke", "")
}

func (a *dashboardRevokeApps) revokeAll(t *testing.T) (int, string) {
	t.Helper()
	return doRequest(t, a.dashboard, "POST", "/users/me/sdk-tokens/revoke-all", "")
}

// downloadWith downloads the Python SDK with a bearer access token and
// returns the status and, for a refusal, the body.
func (a *dashboardRevokeApps) downloadWith(t *testing.T, accessToken string) (int, string) {
	t.Helper()
	req := httptest.NewRequest("GET", "/sdk/download?sdk=python", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := a.download.Test(req, fiber.TestConfig{Timeout: 10 * time.Second})
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == fiber.StatusOK {
		return resp.StatusCode, ""
	}
	return resp.StatusCode, string(body)
}

// sdkAccessToken refreshes the SDK token once and returns the access token
// the SDK path minted (it carries the family) and the rotated refresh token.
func sdkAccessToken(t *testing.T, f *sdkFamilyFixture, refreshToken string) (access, rotated string) {
	t.Helper()
	out, status := postRefresh(t, f.refresh, refreshToken)
	require.Equal(t, fiber.StatusOK, status)
	require.True(t, out.Rotated)
	return out.AccessToken, out.RefreshToken
}

// D1 (Done-when 1): a dashboard revoke of one SDK token row writes the
// family key from the row's lineage (the row revoked is the rotated row, so
// the family is read from the download row it descends from), and the
// family's access token, which downloaded before the revoke, is refused at
// /sdk/download right after it; the family's live refresh token is refused
// at /auth/refresh.
func TestSDKTokenHandler_RevokeToken_EndsTheRowsFamily(t *testing.T) {
	f := newSDKFamilyFixture(t, &rotationStore{})
	apps := newDashboardRevokeApps(t, f)
	s1 := f.downloadSDK(t)
	family := jtiOfToken(t, f.svc, s1)
	a1, s2 := sdkAccessToken(t, f, s1)

	status, _ := apps.downloadWith(t, a1)
	require.Equal(t, fiber.StatusOK, status, "control: before the revoke the family's access token downloads")
	before := f.repo.activeRows()

	status, body := apps.revokeRow(t, f, s2)
	require.Equal(t, fiber.StatusOK, status, body)
	assert.NotNil(t, f.repo.row(s2).RevokedAt, "the row is revoked")
	assert.Equal(t, before-1, f.repo.activeRows(), "only the revoked row left the active set")

	assert.True(t, f.store.data["revoked:fam:"+family], "the family key is written under the download token's jti")
	assert.Equal(t, 1, familyKeyWrites(f.store), "one family key is written")

	status, body = apps.downloadWith(t, a1)
	assert.Equal(t, fiber.StatusUnauthorized, status, "the family's access token is refused at /sdk/download right after the revoke")
	assert.Contains(t, body, familyRefusal)

	body, status = postRefreshRaw(t, f.refresh, s2)
	assert.Equal(t, fiber.StatusUnauthorized, status, "the family's refresh token is refused")
	assert.NotContains(t, body, "accessToken")
}

// D2 (Done-when 1): revoke-all writes one family key per family among the
// user's active rows, and every family's access token is refused at
// /sdk/download right after it.
func TestSDKTokenHandler_RevokeAllTokens_EndsEveryFamily(t *testing.T) {
	f := newSDKFamilyFixture(t, &rotationStore{})
	apps := newDashboardRevokeApps(t, f)
	s1 := f.downloadSDK(t)
	t1 := f.downloadSDK(t)
	familyS, familyT := jtiOfToken(t, f.svc, s1), jtiOfToken(t, f.svc, t1)
	require.NotEqual(t, familyS, familyT)
	a1, s2 := sdkAccessToken(t, f, s1)
	b1, t2 := sdkAccessToken(t, f, t1)
	require.Equal(t, 2, f.repo.activeRows(), "one live row per family")

	status, body := apps.revokeAll(t)
	require.Equal(t, fiber.StatusOK, status, body)
	assert.Equal(t, 0, f.repo.activeRows(), "every row is revoked")

	assert.True(t, f.store.data["revoked:fam:"+familyS], "the first family's key is written")
	assert.True(t, f.store.data["revoked:fam:"+familyT], "the second family's key is written")
	assert.Equal(t, 2, familyKeyWrites(f.store), "one key per family")

	for name, access := range map[string]string{"first family": a1, "second family": b1} {
		status, body := apps.downloadWith(t, access)
		assert.Equal(t, fiber.StatusUnauthorized, status, "%s: the access token is refused at /sdk/download", name)
		assert.Contains(t, body, familyRefusal, name)
	}
	for name, refresh := range map[string]string{"first family": s2, "second family": t2} {
		body, status := postRefreshRaw(t, f.refresh, refresh)
		assert.Equal(t, fiber.StatusUnauthorized, status, "%s: the refresh token is refused", name)
		assert.NotContains(t, body, "accessToken", name)
	}
}

// D3: with no revocation store the revoke still answers 200 and revokes the
// row, as before; the family key has nowhere to go. And a dashboard revoke
// that revoked no row (the row is another user's, or does not exist) writes
// no family key.
func TestSDKTokenHandler_RevokeToken_FamilyKeyNeedsAStoreAndARevokedRow(t *testing.T) {
	t.Run("no store", func(t *testing.T) {
		f := newSDKFamilyFixture(t, nil)
		apps := newDashboardRevokeApps(t, f)
		s1 := f.downloadSDK(t)
		status, body := apps.revokeRow(t, f, s1)
		require.Equal(t, fiber.StatusOK, status, body)
		assert.NotNil(t, f.repo.row(s1).RevokedAt, "the row is revoked")
		assert.Equal(t, 0, f.repo.activeRows())
	})
	t.Run("no row revoked", func(t *testing.T) {
		f := newSDKFamilyFixture(t, &rotationStore{})
		apps := newDashboardRevokeApps(t, f)
		s1 := f.downloadSDK(t)
		f.repo.row(s1).UserID = uuid.New()
		status, body := apps.revokeRow(t, f, s1)
		require.Equal(t, fiber.StatusNotFound, status, body)
		assert.Equal(t, 0, familyKeyWrites(f.store), "no family key for a row the user does not own")
		assert.True(t, strings.Contains(body, "not found"))
	})
}
