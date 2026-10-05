package middleware

import (
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// deferBearerApp mounts one path on two groups with the same prefix, the way the server
// mounts GET /mcp-servers/:id/attestations: first on the agent-signed group, whose
// middleware passes a bearer request through, then on the JWT group.
func deferBearerApp() *fiber.App {
	app := fiber.New()

	agentSigned := app.Group("/mcp-servers")
	agentSigned.Use(func(c fiber.Ctx) error { return c.Next() })
	agentSigned.Get("/:id/attestations", DeferBearerRequests(func(c fiber.Ctx) error {
		return c.SendString("agent route")
	}))

	signedIn := app.Group("/mcp-servers")
	signedIn.Use(func(c fiber.Ctx) error {
		c.Locals("organization_id", "set by the JWT group")
		return c.Next()
	})
	signedIn.Get("/:id/attestations", func(c fiber.Ctx) error {
		org, _ := c.Locals("organization_id").(string)
		return c.SendString("signed-in route: " + org)
	})
	return app
}

func deferBearerGet(t *testing.T, app *fiber.App, authorization string) string {
	t.Helper()
	req := httptest.NewRequest("GET", "/mcp-servers/7/attestations", nil)
	if authorization != "" {
		req.Header.Set(fiber.HeaderAuthorization, authorization)
	}
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return string(body)
}

// A dashboard request reached the agent-signed route with no organization in its
// context, because the agent middleware does not authenticate a bearer token. It now
// continues to the JWT group's route, which does.
func TestDeferBearerRequestsContinuesToTheJWTGroup(t *testing.T) {
	app := deferBearerApp()

	assert.Equal(t, "signed-in route: set by the JWT group", deferBearerGet(t, app, "Bearer placeholder"))
	assert.Equal(t, "agent route", deferBearerGet(t, app, ""))
}
