package handlers

import (
	"fmt"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/interfaces/http/middleware"
)

// The token rotation guide said rotation makes stolen tokens "expire
// immediately" after use. A refresh retires only the refresh token it is
// given, so a thief who refreshes a stolen copy first holds the newest token
// of its family and keeps refreshing until the agent presents the retired
// one, and a refresh retires no access token. The guide states those cases
// and how long a family's tokens last while the agent is silent; each
// statement is pinned here to the refresh route and the auth middleware, at
// the server's default lifetimes.

// stolenTokenCase builds a refresh route and returns it with its JWT service
// and the refresh token its agent holds, for each kind of token the route
// rotates.
type stolenTokenCase struct {
	name  string
	build func(t *testing.T) (*fiber.App, *auth.JWTService, string)
}

func stolenTokenCases() []stolenTokenCase {
	return []stolenTokenCase{
		{"a sign-in on a server with Redis", func(t *testing.T) (*fiber.App, *auth.JWTService, string) {
			f := familyApp(t, &rotationStore{}, false)
			return f.app, f.svc, f.p1
		}},
		{"an SDK download on a server with Redis", func(t *testing.T) (*fiber.App, *auth.JWTService, string) {
			f := newSDKFamilyFixture(t, &rotationStore{})
			return f.refresh, f.svc, f.downloadSDK(t)
		}},
		{"an SDK download on a server without Redis", func(t *testing.T) (*fiber.App, *auth.JWTService, string) {
			f := newSDKFamilyFixture(t, nil)
			return f.refresh, f.svc, f.downloadSDK(t)
		}},
	}
}

// bearerStatus is the status a route behind AuthMiddleware answers to a
// request that carries accessToken.
func bearerStatus(t *testing.T, svc *auth.JWTService, accessToken string) int {
	t.Helper()
	app := fiber.New()
	app.Use(middleware.AuthMiddleware(svc))
	app.Get("/me", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	req := httptest.NewRequest("GET", "/me", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	return resp.StatusCode
}

// mintedFor is how long token was minted to last: its exp less its iat.
func mintedFor(t *testing.T, svc *auth.JWTService, token string) time.Duration {
	t.Helper()
	claims, err := svc.ValidateToken(token)
	require.NoError(t, err)
	require.NotNil(t, claims.ExpiresAt)
	require.NotNil(t, claims.IssuedAt)
	return claims.ExpiresAt.Sub(claims.IssuedAt.Time)
}

// accessTokenSignedIn shapes an access token of a sign-in made at signedIn:
// issued a minute ago and valid for an hour more.
func accessTokenSignedIn(userID, orgID uuid.UUID, signedIn time.Time) auth.JWTClaims {
	now := time.Now()
	return auth.JWTClaims{
		UserID:         userID.String(),
		OrganizationID: orgID.String(),
		Email:          "family@example.com",
		Role:           "admin",
		TokenType:      auth.TokenTypeAccess,
		SessionID:      uuid.New().String(),
		AuthTime:       jwt.NewNumericDate(signedIn),
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now.Add(-time.Minute)),
			NotBefore: jwt.NewNumericDate(now.Add(-time.Minute)),
			Issuer:    auth.IssuerUser,
			Subject:   userID.String(),
			ID:        uuid.New().String(),
		},
	}
}

func TestTokenRotationGuideStatesHowLongAStolenTokenWorks(t *testing.T) {
	// Unset, so each lifetime is the server default the guide quotes.
	t.Setenv("JWT_ACCESS_TTL", "")
	t.Setenv("JWT_SESSION_MAX_AGE", "")

	rotation := readBackendFile(t, tokenRotationGuidePath)
	plain := strings.ToLower(oneLine(strings.ReplaceAll(rotation, "*", "")))
	for _, claim := range []string{"expire immediately", "immediate protection against stolen", "work forever", "prevents attackers from using stolen tokens"} {
		textLacks(t, plain, claim, "the guide says a stolen token stops working at once, or never")
	}
	opening := oneLine(guideSection(t, rotation, "## What is Token Rotation?"))
	problem := oneLine(guideSection(t, rotation, "### The Problem: Token Theft"))
	solution := oneLine(guideSection(t, rotation, "### The Solution: Token Rotation"))
	benefits := oneLine(guideSection(t, rotation, "### Security Benefits"))

	t.Run("a refresh asks for the refresh token and nothing else", func(t *testing.T) {
		request := reflect.TypeOf(RefreshTokenRequest{})
		require.Equal(t, 1, request.NumField(), "the refresh route binds one field")
		assert.Equal(t, "refreshToken", strings.Split(request.Field(0).Tag.Get("json"), ",")[0])
		main := readBackendFile(t, "cmd/server/main.go")
		textHas(t, main, `v1 := app.Group("/api/v1")`, "the v1 routes are under /api/v1")
		textHas(t, main, `auth := v1.Group("/auth")`, "the auth routes are under /api/v1/auth")
		textHas(t, main, `auth.Post("/refresh", h.AuthRefresh.RefreshToken)`, "the refresh route")
		textHas(t, problem, "A refresh at `POST /api/v1/auth/refresh` asks for the refresh token and nothing else, "+
			"so whoever holds a copy of a refresh token can refresh with it, as the agent does.", "the guide says what a refresh asks for")
	})

	t.Run("the agent refreshes first", func(t *testing.T) {
		for _, tc := range stolenTokenCases() {
			t.Run(tc.name, func(t *testing.T) {
				// The thief's copy is the agent's token, byte for byte.
				app, _, token := tc.build(t)
				agents, status := postRefresh(t, app, token)
				require.Equal(t, fiber.StatusOK, status)
				require.True(t, agents.Rotated, "the agent's refresh retires the token the thief copied")

				_, status = postRefreshRaw(t, app, token)
				require.Equal(t, fiber.StatusUnauthorized, status, "the thief's copy is refused")
				_, status = postRefreshRaw(t, app, agents.RefreshToken)
				assert.Equal(t, fiber.StatusUnauthorized, status, "and the family ended: the agent's new token is refused too")
			})
		}
		textHas(t, solution, "**The agent refreshes first.** The stolen copy is retired. When the thief presents it, it is "+
			"refused and the token family ends, the agent's new token included", "the guide states the agent-first case")
		textHas(t, opening, "A stolen refresh token is refused once rotation has retired it.", "the opening says when a stolen token is refused")
	})

	t.Run("the thief refreshes first", func(t *testing.T) {
		for _, tc := range stolenTokenCases() {
			t.Run(tc.name, func(t *testing.T) {
				app, _, token := tc.build(t)
				first, status := postRefresh(t, app, token)
				require.Equal(t, fiber.StatusOK, status, "the thief refreshes the copy")
				require.True(t, first.Rotated)
				second, status := postRefresh(t, app, first.RefreshToken)
				require.Equal(t, fiber.StatusOK, status, "and keeps refreshing while the agent is silent")
				require.True(t, second.Rotated)

				_, status = postRefreshRaw(t, app, token)
				require.Equal(t, fiber.StatusUnauthorized, status, "the agent's next refresh presents the token the thief's refresh retired")
				_, status = postRefreshRaw(t, app, second.RefreshToken)
				assert.Equal(t, fiber.StatusUnauthorized, status, "which ends the family: the thief's token is refused")
			})
		}
		textHas(t, solution, "**The thief refreshes first.** The thief now holds the newest refresh token and can keep "+
			"refreshing while the agent is silent. The agent's next refresh presents the retired token, and that ends the "+
			"family, the thief's token included.", "the guide states the thief-first case")
		textHas(t, solution, "While the agent is silent, the thief's tokens keep working at most this long:", "the guide gives the bounds as upper bounds")
	})

	t.Run("a sign-in ends JWT_SESSION_MAX_AGE after it began, however often it is refreshed", func(t *testing.T) {
		var hours int
		for _, tc := range []struct {
			name  string
			store *rotationStore
		}{
			{name: "with Redis", store: &rotationStore{}},
			{name: "without Redis"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				f := familyApp(t, tc.store, false)
				maxAge := f.svc.SessionMaxAge()
				require.Zero(t, maxAge%time.Hour, "the guide quotes the default in whole hours")
				hours = int(maxAge / time.Hour)

				out, status := postRefresh(t, f.app, f.p1)
				require.Equal(t, fiber.StatusOK, status)
				before, err := f.svc.ValidateToken(f.p1)
				require.NoError(t, err)
				after, err := f.svc.ValidateToken(out.RefreshToken)
				require.NoError(t, err)
				assert.Equal(t, before.SignedInAt(), after.SignedInAt(), "a refresh carries the sign-in's time to the token it issues")

				// Tokens a refresh issued a minute ago, of a sign-in a minute
				// past the limit and of one a minute inside it.
				now := time.Now()
				ended, live := now.Add(-maxAge-time.Minute), now.Add(-maxAge+time.Minute)
				body, status := postRefreshRaw(t, f.app, signSessionClaims(t, loginTokenSignedIn(f.userID, f.orgID, uuid.New().String(), ended, now.Add(-time.Minute))))
				require.Equal(t, fiber.StatusUnauthorized, status, "the refresh token of an ended sign-in is refused")
				assert.JSONEq(t, sessionExpiredBody, body)
				_, status = postRefresh(t, f.app, signSessionClaims(t, loginTokenSignedIn(f.userID, f.orgID, uuid.New().String(), live, now.Add(-time.Minute))))
				assert.Equal(t, fiber.StatusOK, status, "control: inside the limit it refreshes")

				assert.Equal(t, fiber.StatusUnauthorized, bearerStatus(t, f.svc, signSessionClaims(t, accessTokenSignedIn(f.userID, f.orgID, ended))),
					"an access token of an ended sign-in is refused")
				assert.Equal(t, fiber.StatusOK, bearerStatus(t, f.svc, signSessionClaims(t, accessTokenSignedIn(f.userID, f.orgID, live))),
					"control: inside the limit it is accepted")
			})
		}
		textHas(t, solution, fmt.Sprintf("**A sign-in from `aim-sdk login`**: until `JWT_SESSION_MAX_AGE` after the sign-in "+
			"(**%d hours** by default), however often it is refreshed. From then on its refresh token is refused, and so are "+
			"its access tokens.", hours), "the guide quotes the sign-in's maximum age")
	})

	t.Run("an SDK download has no maximum age and each refresh issues a token that lasts 90 days", func(t *testing.T) {
		f := newSDKFamilyFixture(t, &rotationStore{})
		out, status := postRefresh(t, f.refresh, f.downloadSDK(t))
		require.Equal(t, fiber.StatusOK, status)
		require.True(t, out.Rotated)

		day := 24 * time.Hour
		lasts := mintedFor(t, f.svc, out.RefreshToken)
		require.Zero(t, lasts%day, "the guide quotes the lifetime in whole days")
		row := f.repo.row(out.RefreshToken)
		require.NotNil(t, row, "the refresh created the new token's row")
		assert.Equal(t, lasts, row.ExpiresAt.Sub(row.CreatedAt), "the row expires with the token")

		// An SDK download issued longer ago than a sign-in may last still
		// refreshes.
		issued := time.Now().Add(-f.svc.SessionMaxAge() - time.Hour)
		id := uuid.New().String()
		claims := auth.JWTClaims{
			UserID:         f.userID.String(),
			OrganizationID: f.orgID.String(),
			Email:          "sdkfam@example.com",
			Role:           "admin",
			TokenType:      auth.TokenTypeSDK,
			SessionID:      id,
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(issued.Add(lasts)),
				IssuedAt:  jwt.NewNumericDate(issued),
				NotBefore: jwt.NewNumericDate(issued),
				Issuer:    auth.IssuerSDK,
				Subject:   f.userID.String(),
				ID:        id,
			},
		}
		old := signSessionClaims(t, claims)
		require.NoError(t, f.repo.Create(&domain.SDKToken{
			ID: uuid.New(), UserID: f.userID, OrganizationID: f.orgID, TokenHash: sha256Hex(old), TokenID: id,
			CreatedAt: issued, ExpiresAt: issued.Add(lasts),
		}))
		out, status = postRefresh(t, f.refresh, old)
		require.Equal(t, fiber.StatusOK, status, "an SDK download older than a sign-in's maximum age refreshes")
		assert.True(t, out.Rotated)

		days := int(lasts / day)
		textHas(t, solution, fmt.Sprintf("**A token from an SDK download**: as long as it is refreshed at least every "+
			"**%d days**. It has no maximum age, and each refresh issues a refresh token that lasts %d days.", days, days),
			"the guide quotes the SDK download token's lifetime")
	})

	t.Run("a refresh does not retire an access token", func(t *testing.T) {
		var hours int
		for _, tc := range stolenTokenCases() {
			t.Run(tc.name, func(t *testing.T) {
				app, svc, token := tc.build(t)
				first, status := postRefresh(t, app, token)
				require.Equal(t, fiber.StatusOK, status)
				later, status := postRefresh(t, app, first.RefreshToken)
				require.Equal(t, fiber.StatusOK, status)
				require.True(t, later.Rotated)

				assert.Equal(t, fiber.StatusOK, bearerStatus(t, svc, first.AccessToken),
					"the access token issued before the later refresh is still accepted")
				ttl := mintedFor(t, svc, first.AccessToken)
				assert.Equal(t, time.Duration(svc.AccessTTLSeconds())*time.Second, ttl, "it lasts JWT_ACCESS_TTL")
				require.Zero(t, ttl%time.Hour, "the guide quotes the default in whole hours")
				hours = int(ttl / time.Hour)
			})
		}
		textHas(t, solution, fmt.Sprintf("**An access token**: at most `JWT_ACCESS_TTL` from when it was issued "+
			"(**%d hours** by default). A refresh does not retire an access token, so one stolen before a refresh is still "+
			"accepted after it.", hours), "the guide quotes the access token lifetime")
	})

	t.Run("on a server without Redis a sign-in is not rotated", func(t *testing.T) {
		f := familyApp(t, nil, false)
		thief, status := postRefresh(t, f.app, f.p1)
		require.Equal(t, fiber.StatusOK, status)
		assert.False(t, thief.Rotated)
		assert.Equal(t, f.p1, thief.RefreshToken, "the thief is answered with the token it presented")
		_, status = postRefresh(t, f.app, f.p1)
		assert.Equal(t, fiber.StatusOK, status, "and the agent's refresh with the same token still works")
		// The sign-in's end holds without Redis: the sign-in subtest above.
		textHas(t, solution, "On a server without Redis a sign-in from `aim-sdk login` is not rotated, so a stolen copy of "+
			"its refresh token keeps working as long as the agent's own, until the sign-in ends.", "the guide states the case without Redis")
	})

	t.Run("the security benefits say what a refresh retires", func(t *testing.T) {
		textLacks(t, benefits, "Every token use triggers revocation", "a refresh without Redis, or a request with an access token, retires nothing")
		textHas(t, benefits, "A refresh that rotates retires the refresh token it was given", "the benefits say what a refresh retires")
		textHas(t, benefits, "A retired refresh token presented again is refused and ends its token family", "the benefits say what a reuse does")
		textHas(t, benefits, "[The Solution: Token Rotation](#the-solution-token-rotation)", "the benefits point to the bounds")
	})
}
