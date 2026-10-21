package middleware

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A sign-in lasts at most JWT_SESSION_MAX_AGE however often it is refreshed.
// The last access token a refresh issues before the limit used to stay valid
// until its own expiry (JWT_ACCESS_TTL). A login access token now carries
// the sign-in's time, and AuthMiddleware refuses one whose sign-in is past
// the limit by comparing that claim with the clock: no store is read.

const sessionAgeTestSecret = "test-only-jwt-secret-not-a-real-value"

// loginAccessToken signs an access token of a login family whose sign-in
// happened at signedIn and which was issued now; a zero signedIn leaves
// auth_time out, as an access token minted before the claim reached access
// tokens or from an SDK-download token.
func loginAccessToken(t *testing.T, userID, orgID uuid.UUID, signedIn time.Time) string {
	t.Helper()
	now := time.Now()
	claims := auth.JWTClaims{
		UserID:         userID.String(),
		OrganizationID: orgID.String(),
		Email:          "age@example.com",
		Role:           "admin",
		TokenType:      auth.TokenTypeAccess,
		SessionID:      uuid.New().String(),
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(2 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    auth.IssuerUser,
			Subject:   userID.String(),
			ID:        uuid.New().String(),
		},
	}
	if !signedIn.IsZero() {
		claims.AuthTime = jwt.NewNumericDate(signedIn)
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(sessionAgeTestSecret))
	require.NoError(t, err)
	return token
}

func sessionAgeApp(t *testing.T, maxAge string, mw func(*auth.JWTService) fiber.Handler) (*fiber.App, *map[string]interface{}) {
	t.Helper()
	t.Setenv("JWT_SECRET", sessionAgeTestSecret)
	t.Setenv("JWT_SESSION_MAX_AGE", maxAge)
	jwtService := auth.NewJWTService()
	seen := map[string]interface{}{}
	app := fiber.New()
	app.Use(mw(jwtService))
	app.Get("/protected", func(c fiber.Ctx) error {
		seen["user_id"] = c.Locals("user_id")
		return c.JSON(fiber.Map{"message": "success"})
	})
	return app, &seen
}

func getWithBearer(t *testing.T, app *fiber.App, token string) (int, map[string]interface{}) {
	t.Helper()
	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &result))
	return resp.StatusCode, result
}

// M1: an access token whose sign-in is past the cap is refused as an
// expired token, although the token itself has not expired.
func TestAuthMiddleware_RefusesAnAccessTokenWhoseSignInIsPastTheMaxAge(t *testing.T) {
	app, _ := sessionAgeApp(t, "1h", AuthMiddleware)
	userID, orgID := uuid.New(), uuid.New()

	status, body := getWithBearer(t, app, loginAccessToken(t, userID, orgID, time.Now().Add(-2*time.Hour)))
	assert.Equal(t, fiber.StatusUnauthorized, status)
	assert.Equal(t, "Invalid or expired token", body["error"])
}

// M2: inside the cap the token is accepted, as is one without auth_time.
func TestAuthMiddleware_AcceptsAnAccessTokenInsideTheMaxAge(t *testing.T) {
	app, seen := sessionAgeApp(t, "1h", AuthMiddleware)
	userID, orgID := uuid.New(), uuid.New()

	status, _ := getWithBearer(t, app, loginAccessToken(t, userID, orgID, time.Now().Add(-30*time.Minute)))
	assert.Equal(t, fiber.StatusOK, status)
	assert.Equal(t, userID, (*seen)["user_id"])

	status, _ = getWithBearer(t, app, loginAccessToken(t, userID, orgID, time.Time{}))
	assert.Equal(t, fiber.StatusOK, status, "an access token without auth_time is not checked")
}

// M3: the pair a sign-in mints passes, and a bearer the service mints for a
// login refreshed past the cap would carry the sign-in's time.
func TestAuthMiddleware_AcceptsTheAccessTokenOfAFreshSignIn(t *testing.T) {
	app, seen := sessionAgeApp(t, "1h", AuthMiddleware)
	t.Setenv("JWT_SECRET", sessionAgeTestSecret)
	svc := auth.NewJWTService()
	userID, orgID := uuid.New(), uuid.New()
	access, _, err := svc.GenerateTokenPair(userID.String(), orgID.String(), "age@example.com", "admin")
	require.NoError(t, err)

	status, _ := getWithBearer(t, app, access)
	assert.Equal(t, fiber.StatusOK, status)
	assert.Equal(t, userID, (*seen)["user_id"])
}

// M4: OptionalAuthMiddleware leaves a request anonymous on such a token.
func TestOptionalAuthMiddleware_LeavesAnEndedSignInAnonymous(t *testing.T) {
	app, seen := sessionAgeApp(t, "1h", OptionalAuthMiddleware)
	userID, orgID := uuid.New(), uuid.New()

	status, _ := getWithBearer(t, app, loginAccessToken(t, userID, orgID, time.Now().Add(-2*time.Hour)))
	assert.Equal(t, fiber.StatusOK, status)
	assert.Nil(t, (*seen)["user_id"], "no user context from an ended sign-in")

	status, _ = getWithBearer(t, app, loginAccessToken(t, userID, orgID, time.Now().Add(-30*time.Minute)))
	assert.Equal(t, fiber.StatusOK, status)
	assert.Equal(t, userID, (*seen)["user_id"])
}
