package middleware

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
)

// The auth middleware exposes the acting access token's sign-in time
// (auth_time) as a time.Time local: the sign-in's time for a token minted with
// or from a login refresh token, and the zero time for a token that carries
// none (one minted from an SDK-download token, or a standalone one). A
// credential-minting route reads it to refuse a token that records no sign-in.
func TestAuthMiddleware_ExposesTheActingTokensSignInTime(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret-key-for-testing-purposes-32chars")
	svc := auth.NewJWTService()
	userID, orgID := uuid.New(), uuid.New()
	signIn, _, err := svc.GenerateTokenPair(userID.String(), orgID.String(), "m2@example.com", "admin")
	require.NoError(t, err)
	sdk, err := svc.GenerateSDKRefreshToken(userID.String(), orgID.String(), "m2@example.com", "admin")
	require.NoError(t, err)
	sdkAccess, _, err := svc.RefreshTokenPair(sdk, "m2@example.com", "admin")
	require.NoError(t, err)
	standalone, err := svc.GenerateAccessToken(userID.String(), orgID.String(), "m2@example.com", "admin")
	require.NoError(t, err)

	var seen time.Time
	var seenSet bool
	app := fiber.New()
	app.Use(AuthMiddleware(svc))
	app.Get("/protected", func(c fiber.Ctx) error {
		seen, seenSet = c.Locals("auth_time").(time.Time)
		return c.JSON(fiber.Map{"ok": true})
	})
	call := func(token string) int {
		seen, seenSet = time.Time{}, false
		req := httptest.NewRequest("GET", "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := app.Test(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		return resp.StatusCode
	}

	assert.Equal(t, fiber.StatusOK, call(signIn))
	require.True(t, seenSet, "the local is a time.Time")
	claims, err := svc.ValidateToken(signIn)
	require.NoError(t, err)
	require.NotNil(t, claims.AuthTime)
	assert.Equal(t, claims.AuthTime.Unix(), seen.Unix(), "a sign-in's access token exposes its sign-in time")

	assert.Equal(t, fiber.StatusOK, call(sdkAccess), "the token itself is a valid bearer")
	require.True(t, seenSet)
	assert.True(t, seen.IsZero(), "an access token minted from an SDK-download token exposes the zero time")

	assert.Equal(t, fiber.StatusOK, call(standalone))
	require.True(t, seenSet)
	assert.True(t, seen.IsZero(), "a standalone access token exposes the zero time")
}
