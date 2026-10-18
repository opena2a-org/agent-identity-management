package handlers

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// A password change marks the user's sdk_tokens rows that are not yet revoked
// as revoked, with the reason password_changed, so the dashboard's list of SDK
// tokens shows none of them active: the change refuses every token minted
// before it. A row revoked for
// a password change ends its family as a logout does, so the recovery route
// restores none of the family's tokens, and the refresh route refuses the
// family's live token without recording a reuse.

// listedSDKTokens lists the fixture user's SDK tokens through the dashboard's
// route, the revoked ones too when includeRevoked is set, by token id.
func listedSDKTokens(t *testing.T, f *sdkFamilyFixture, includeRevoked bool) map[string]domain.SDKToken {
	t.Helper()
	h := NewSDKTokenHandler(application.NewSDKTokenService(f.repo), f.svc)
	app := fiber.New()
	app.Get("/users/me/sdk-tokens", func(c fiber.Ctx) error {
		c.Locals("user_id", f.userID)
		return h.ListUserTokens(c)
	})
	path := "/users/me/sdk-tokens"
	if includeRevoked {
		path += "?include_revoked=true"
	}
	status, body := doRequest(t, app, "GET", path, "")
	require.Equal(t, fiber.StatusOK, status, body)
	var out struct {
		Tokens []domain.SDKToken `json:"tokens"`
	}
	require.NoError(t, json.Unmarshal([]byte(body), &out))
	byID := map[string]domain.SDKToken{}
	for _, token := range out.Tokens {
		byID[token.TokenID] = token
	}
	return byID
}

// changeFixturePassword changes the fixture user's password through
// POST /auth/change-password, with the fixture's sdk_tokens model attached
// to the auth service as cmd/server attaches the SDK token repository.
func changeFixturePassword(t *testing.T, f *sdkFamilyFixture) {
	t.Helper()
	users := newPasswordChangeUser(t, false)
	users.user.ID, users.user.OrganizationID = f.userID, f.orgID
	authSvc := application.NewAuthService(users, pwChangeOrgRepo{}, nil, nil, nil, nil)
	authSvc.SetSDKTokenRepository(f.repo)
	h := NewAuthHandler(authSvc, f.svc, pwChangeOrgRepo{}, nil, nil)
	app := fiber.New()
	app.Post("/auth/change-password", func(c fiber.Ctx) error {
		c.Locals("user_id", f.userID)
		return h.ChangePassword(c)
	})
	req := httptest.NewRequest("POST", "/auth/change-password", strings.NewReader(changePasswordBody()))
	req.Header.Set("Content-Type", "application/json")
	// The new password is hashed at the production bcrypt cost.
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 30 * time.Second, FailOnTimeout: true})
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, fiber.StatusOK, resp.StatusCode, "the change itself")
	require.NotNil(t, users.passwordChangedAt(), "the change is recorded")
}

func TestPasswordChange_RevokesTheUsersSDKTokenRows(t *testing.T) {
	f := newSDKFamilyFixture(t, nil)
	chain := rotatedChain(t, f, 1)
	other := f.downloadSDK(t)
	tokens := []string{chain[0], chain[1], other}
	require.Len(t, listedSDKTokens(t, f, false), 2, "control: two downloads, each listed with one active token")

	changeFixturePassword(t, f)

	assert.Empty(t, listedSDKTokens(t, f, false), "no SDK token of the user is listed active")
	listed := listedSDKTokens(t, f, true)
	require.Len(t, listed, 3, "the revoked tokens are listed with include_revoked")
	reasonOf := func(token string) string {
		row, ok := listed[f.repo.row(token).TokenID]
		require.True(t, ok, "the token's row is listed")
		if row.RevokedAt == nil || row.RevokeReason == nil {
			return ""
		}
		return *row.RevokeReason
	}
	assert.Equal(t, domain.SDKTokenRevokeReasonRotation, reasonOf(chain[0]), "a row retired before the change keeps its reason")
	assert.Equal(t, domain.SDKTokenRevokeReasonPasswordChanged, reasonOf(chain[1]))
	assert.Equal(t, domain.SDKTokenRevokeReasonPasswordChanged, reasonOf(other))

	// The fixture's refresh route reads a user record that does not carry
	// the change, so its refusal here is the row's own.
	for _, token := range []string{chain[1], other} {
		body, status := postRefreshRaw(t, f.refresh, token)
		assert.Equal(t, fiber.StatusUnauthorized, status, body)
		assert.Contains(t, body, familyRefusal)
	}
	assert.Empty(t, f.reuseRows(), "a row a password change revoked is not a reuse")

	for i, token := range tokens {
		assertNotRecovered(t, f, token, fmt.Sprintf("token %d of a user whose password changed", i))
	}
}
