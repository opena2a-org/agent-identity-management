package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
)

// strictRateLimitedRoutes is every route StrictRateLimitMiddleware covers: the
// password, password-reset, registration, OAuth token, device-code and
// bootstrap-token surfaces. A group Use() reaches every route registered under
// its prefix after it, so the signed-in /auth routes share the /auth limiter and
// /oauth/device/approve shares the /oauth one.
var strictRateLimitedRoutes = []string{
	"POST /api/v1/public/agents/register",
	"POST /api/v1/public/register",
	"GET /api/v1/public/register/:requestId/status",
	"POST /api/v1/public/login",
	"POST /api/v1/public/change-password",
	"POST /api/v1/public/forgot-password",
	"POST /api/v1/public/reset-password",
	"POST /api/v1/public/request-access",
	"POST /api/v1" + bootstrapTokenMintPath,
	"POST /api/v1" + bootstrapTokenExchangePath,
	"POST /api/v1/oauth/token",
	"POST /api/v1/oauth/device/code",
	"POST /api/v1/oauth/device/token",
	"GET /api/v1/oauth/device/verify",
	"POST /api/v1/oauth/device/approve",
	"POST /api/v1/auth/login/local",
	"POST /api/v1/auth/logout",
	"POST /api/v1/auth/refresh",
	"GET /api/v1/auth/me",
	"POST /api/v1/auth/change-password",
	"POST /api/v1/auth/sdk/recover",
}

// strictCensusApp mounts every /api/v1 route through mountAPIRoutes, the call
// main makes, with the real middleware and empty handlers. A handler that runs
// panics on its missing service; the first middleware turns that into a 500, so
// a probe sees what the limiters in front of the handler decided.
func strictCensusApp(t *testing.T) (*fiber.App, *auth.JWTService) {
	t.Helper()
	// The production limits: ENVIRONMENT unset.
	withEnvironment(t, "", false)
	t.Setenv("JWT_SECRET", "strict-limiter-census-secret-of-at-least-32-chars")
	// app.Test connects from 0.0.0.0. Trusting it as a proxy lets each probe
	// present its own client address in X-Real-IP, so each probe has its own
	// bucket in every limiter it passes.
	t.Setenv("TRUSTED_PROXIES", "0.0.0.0")
	jwtService := auth.NewJWTService()

	// Some handlers log before they reach their missing service; thousands of
	// probes would bury the result.
	prevLog := log.Writer()
	log.SetOutput(io.Discard)
	t.Cleanup(func() { log.SetOutput(prevLog) })

	app := fiber.New()
	app.Use(func(c fiber.Ctx) (err error) {
		defer func() {
			if recover() != nil {
				err = c.SendStatus(fiber.StatusInternalServerError)
			}
		}()
		return c.Next()
	})
	mountAPIRoutes(app, &Handlers{}, &Services{Secrets: &application.SecretsService{}}, jwtService, nil, nil)
	return app, jwtService
}

// concreteRoutePath turns a registered path into one a request can carry.
func concreteRoutePath(path string) string {
	segments := strings.Split(concreteSDKAPIPath(path), "/")
	for i, segment := range segments {
		if segment == "*" || segment == "+" {
			segments[i] = "x"
		}
	}
	return strings.Join(segments, "/")
}

// admittedBeforeRefusal sends eleven requests to one route as one client and
// returns how many were admitted before the first 429.
func admittedBeforeRefusal(t *testing.T, app *fiber.App, method, path, client, token string) int {
	t.Helper()
	for i := 0; i < 11; i++ {
		req := httptest.NewRequest(method, path, nil)
		req.Header.Set("X-Real-IP", client)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := app.Test(req)
		require.NoError(t, err, "%s %s", method, path)
		_ = resp.Body.Close()
		if resp.StatusCode == fiber.StatusTooManyRequests {
			return i
		}
	}
	return 11
}

// TestStrictRateLimiterCoversExactlyTheCredentialRoutes probes every route the
// server mounts under /api/v1, as a fresh client each time: eleven requests
// with no credentials, then eleven with an organization admin's session. A
// route carries the strict limiter when a probe is admitted ten times and
// refused the eleventh; the general limiter admits a hundred and never shows
// here. The set found must be strictRateLimitedRoutes, no more and no fewer.
//
// A limiter behind a gate the probe cannot pass (an agent signature, the
// platform-admin allowlist) is not reached, so it is not counted.
func TestStrictRateLimiterCoversExactlyTheCredentialRoutes(t *testing.T) {
	app, jwtService := strictCensusApp(t)

	probed := map[string]bool{}
	var found []string
	client := 0
	for _, route := range app.GetRoutes(true) {
		switch route.Method {
		case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		default:
			continue
		}
		key := route.Method + " " + route.Path
		if probed[key] || !strings.HasPrefix(route.Path, "/api/v1/") {
			continue
		}
		probed[key] = true
		path := concreteRoutePath(route.Path)

		strict := false
		for _, signedIn := range []bool{false, true} {
			client++
			token := ""
			if signedIn {
				var err error
				token, err = jwtService.GenerateAccessToken(uuid.NewString(), uuid.NewString(), "census@example.com", "admin")
				require.NoError(t, err)
			}
			address := fmt.Sprintf("10.%d.%d.%d", client>>16&255, client>>8&255, client&255)
			admitted := admittedBeforeRefusal(t, app, route.Method, path, address, token)
			if admitted < 10 {
				t.Errorf("%s: a fresh client was refused after %d requests; no limiter here is tighter than 10 a minute",
					key, admitted)
			}
			strict = strict || admitted == 10
		}
		if strict {
			found = append(found, key)
		}
	}

	require.Greater(t, len(probed), len(strictRateLimitedRoutes), "the census mounted too few routes to be the server's table")
	t.Logf("probed %d routes; %d carry the strict limiter", len(probed), len(found))

	want := append([]string(nil), strictRateLimitedRoutes...)
	sort.Strings(want)
	sort.Strings(found)
	assert.Equal(t, want, found,
		"the routes behind StrictRateLimitMiddleware changed. A credential route that lost it is "+
			"open to ten times the guessing; a route that gained it is limited to 10 a minute. "+
			"Update strictRateLimitedRoutes only when the change is intended.")
}
