package main

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/interfaces/http/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// bootstrapRouteApp mounts the real route table with the real role gate and
// real limiters. Authentication is a stand-in that admits a request carrying
// X-Test-Role, as the user named by X-Test-User, and refuses any other, so a
// test can tell whether a route ran it.
func bootstrapRouteApp(t *testing.T) *fiber.App {
	t.Helper()
	// rateLimitMax multiplies limits by 10 under ENVIRONMENT=test; pin the
	// production limit, which is the one under test.
	t.Setenv("ENVIRONMENT", "production")

	authenticate := func(c fiber.Ctx) error {
		role := c.Get("X-Test-Role")
		if role == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "session required"})
		}
		c.Locals("role", role)
		user, err := uuid.Parse(c.Get("X-Test-User"))
		if err != nil {
			user = uuid.Nil
		}
		c.Locals("user_id", user)
		c.Locals("organization_id", uuid.New())
		return c.Next()
	}
	ok := func(status int) fiber.Handler {
		return func(c fiber.Ctx) error { return c.SendStatus(status) }
	}

	app := fiber.New()
	v1 := app.Group("/api/v1")
	registerBootstrapTokenRoutes(v1, bootstrapTokenRouteDeps{
		Authenticate:  authenticate,
		RequireMember: middleware.MemberMiddleware(),
		MintLimit:     middleware.StrictRateLimitMiddleware(),
		RevokeLimit:   middleware.RateLimitMiddleware(),
		ExchangeLimit: middleware.StrictRateLimitMiddleware(),
		Mint:          ok(fiber.StatusCreated),
		Revoke:        ok(fiber.StatusOK),
		Exchange:      ok(fiber.StatusCreated),
	})
	return app
}

func bootstrapRouteStatus(t *testing.T, app *fiber.App, path, role string, user ...uuid.UUID) int {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1"+path, nil)
	if role != "" {
		req.Header.Set("X-Test-Role", role)
	}
	if len(user) > 0 {
		req.Header.Set("X-Test-User", user[0].String())
	}
	resp, err := app.Test(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
	return resp.StatusCode
}

func TestBootstrapTokenRoutes_MintAndRevokeNeedAMemberSession(t *testing.T) {
	app := bootstrapRouteApp(t)
	for _, path := range []string{bootstrapTokenMintPath, bootstrapTokenRevokePath} {
		assert.Equal(t, fiber.StatusUnauthorized, bootstrapRouteStatus(t, app, path, ""), path)
		assert.Equal(t, fiber.StatusForbidden, bootstrapRouteStatus(t, app, path, "viewer"), path)
		assert.Equal(t, fiber.StatusForbidden, bootstrapRouteStatus(t, app, path, "service"), path)
		assert.NotEqual(t, fiber.StatusForbidden, bootstrapRouteStatus(t, app, path, "member"), path)
	}
}

// The exchange is called by an SDK that holds only the token, so the session
// check must not sit in front of it, even though it shares the path prefix.
func TestBootstrapTokenRoutes_ExchangeDoesNotRunTheSessionCheck(t *testing.T) {
	app := bootstrapRouteApp(t)
	assert.Equal(t, fiber.StatusCreated, bootstrapRouteStatus(t, app, bootstrapTokenExchangePath, ""))
}

// Mint is limited per user: the limiter runs after authentication, so one
// user's minting does not use up another's allowance.
func TestBootstrapTokenRoutes_MintIsRateLimitedPerUser(t *testing.T) {
	app := bootstrapRouteApp(t)
	alice, bob := uuid.New(), uuid.New()
	statuses := map[int]int{}
	for i := 0; i < 11; i++ {
		statuses[bootstrapRouteStatus(t, app, bootstrapTokenMintPath, "member", alice)]++
	}
	assert.Equal(t, 10, statuses[fiber.StatusCreated])
	assert.Equal(t, 1, statuses[fiber.StatusTooManyRequests])
	assert.Equal(t, fiber.StatusCreated, bootstrapRouteStatus(t, app, bootstrapTokenMintPath, "member", bob))
}

func TestBootstrapTokenRoutes_ExchangeIsRateLimited(t *testing.T) {
	app := bootstrapRouteApp(t)
	statuses := map[int]int{}
	for i := 0; i < 11; i++ {
		statuses[bootstrapRouteStatus(t, app, bootstrapTokenExchangePath, "")]++
	}
	assert.Equal(t, 10, statuses[fiber.StatusCreated])
	assert.Equal(t, 1, statuses[fiber.StatusTooManyRequests])
}

// Source-wiring guard: production mounts the bootstrap routes with the
// session check, the member gate and the strict limiter on mint and exchange.
func TestBootstrapTokenRoutes_MainWiresTheRealMiddleware(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	src, err := os.ReadFile(filepath.Join(filepath.Dir(thisFile), "main.go")) //nolint:gosec // G304: path derived from runtime.Caller
	require.NoError(t, err)

	for field, want := range map[string]string{
		"Authenticate":  `middleware\.AuthMiddleware\(jwtService\)`,
		"RequireMember": `middleware\.MemberMiddleware\(\)`,
		"MintLimit":     `middleware\.StrictRateLimitMiddleware\(\)`,
		"ExchangeLimit": `middleware\.StrictRateLimitMiddleware\(\)`,
		"RevokeLimit":   `middleware\.(Strict)?RateLimitMiddleware\(\)`,
	} {
		pattern := regexp.MustCompile(`(?m)^\s*` + field + `:\s*` + want + `,`)
		assert.True(t, pattern.Match(src), "main.go must set %s to %s in registerBootstrapTokenRoutes", field, want)
	}
	assert.Regexp(t, `registerBootstrapTokenRoutes\(v1, bootstrapTokenRouteDeps\{`, string(src))
}

// An unmatched path carrying a token reaches customErrorHandler, whose log
// line repeats the path twice (once in Fiber's 404 text); neither copy may
// carry the token.
func TestCustomErrorHandler_RedactsABootstrapTokenInThePath(t *testing.T) {
	token, _, _, err := domain.GenerateBootstrapToken()
	require.NoError(t, err)

	var logged bytes.Buffer
	prevOut, prevFlags := log.Writer(), log.Flags()
	log.SetOutput(&logged)
	t.Cleanup(func() { log.SetOutput(prevOut); log.SetFlags(prevFlags) })

	app := fiber.New(fiber.Config{ErrorHandler: customErrorHandler})
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/v1/onboarding/bootstrap-tokens/"+token, nil))
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Equal(t, fiber.StatusNotFound, resp.StatusCode)

	out := logged.String()
	assert.Contains(t, out, "aim_ob_[REDACTED]")
	assert.NotContains(t, out, token[len(domain.BootstrapTokenPrefix):])
}
