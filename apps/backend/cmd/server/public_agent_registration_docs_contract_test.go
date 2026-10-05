package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// dashboardDocsEntry returns the text the dashboard's API reference holds for
// one path: from its path line to the next endpoint's path line.
func dashboardDocsEntry(t *testing.T, src, path string) string {
	t.Helper()
	marker := `path: "` + path + `",`
	start := strings.Index(src, marker)
	require.NotEqual(t, -1, start, "the API reference has no entry for %q", path)
	rest := src[start+len(marker):]
	if end := strings.Index(rest, `path: "`); end != -1 {
		return rest[:end]
	}
	return rest
}

// TestPublicAgentRegistrationDocsContract pins what the dashboard's API
// reference and the handler's API description say about
// POST /api/v1/public/agents/register to what the handler does. The handler
// registers an agent only for a signed-in user and answers every other caller
// 401. The reference used to list the route as public with no auth required, so
// its request builder sent no token and the documented call could not succeed.
// A handler that stops refusing those callers, or a reference that stops saying
// a token is required, fails here, in the tree that carries both.
func TestPublicAgentRegistrationDocsContract(t *testing.T) {
	mainSrc := aim03ReadRepoFile(t, "apps/backend/cmd/server/main.go")
	srcContains(t, mainSrc, `public.Post("/agents/register", h.PublicAgent.Register)`,
		"the route is served by PublicAgentHandler.Register")

	handler := aim03ReadRepoFile(t, "apps/backend/internal/interfaces/http/handlers/public_agent_handler.go")
	srcContains(t, handler, `if !hasUser || !hasOrg || userID == uuid.Nil || orgID == uuid.Nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": publicRegisterAuthRequiredMessage,`,
		"Register answers 401 to a caller with no user access token")
	srcContains(t, handler, "// @Security BearerAuth\n", "the API description declares the bearer token")
	srcContains(t, handler, "// @Failure 401 {object} ErrorResponse", "the API description lists the 401")
	srcNotContains(t, handler, "without authentication", "the API description must not call the route unauthenticated")

	entry := dashboardDocsEntry(t,
		aim03ReadRepoFile(t, "apps/web/lib/api-documentation.ts"), "/api/v1/public/agents/register")
	srcContains(t, entry, "requiresAuth: true,",
		"the reference must mark the route as needing auth, or its request builder sends no token")
	srcNotContains(t, entry, "requiresAuth: false", "the reference must not mark the route as needing no auth")
	srcContains(t, entry, `auth: "Bearer Token (JWT)",`, "the reference names the credential the handler reads")
	srcNotContains(t, entry, "None (Public)", "the reference must not call the route public")
	srcContains(t, entry, "POST /api/v1/agents", "the reference says where an API key registers an agent")

	// The handler rejects a body without these four fields, so the documented
	// request has to carry them under the names the handler binds.
	srcContains(t, handler, `req.Name == "" || req.DisplayName == "" || req.Description == "" || req.AgentType == ""`,
		"Register requires name, displayName, description and agentType")
	for _, field := range []string{`"name":`, `"displayName":`, `"description":`, `"agentType":`} {
		srcContains(t, entry, field, "the documented request example carries every required field")
	}
}
