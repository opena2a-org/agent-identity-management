package main

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// GET /api/v1/mcp-servers/:id/challenge and POST /api/v1/mcp-servers/:id/attest are
// mounted on the agent-signed MCP group. PQCAgentMiddleware passes a request on
// unauthenticated when it carries an Authorization header or lacks X-Agent-ID or
// X-Timestamp; AuthMiddleware then answers it, as on the /agents, /detection and /a2a
// groups, so no such request reaches the handler without an organization.
func TestMCPAttestationAgentRoutesFallBackToAuthMiddleware(t *testing.T) {
	app, jwtService := strictCensusApp(t)
	serverID, agentID := uuid.NewString(), uuid.NewString()
	session, err := jwtService.GenerateAccessToken(uuid.NewString(), uuid.NewString(), "attestation@example.com", "member")
	require.NoError(t, err)

	routes := []struct{ method, path, body string }{
		{fiber.MethodGet, "/api/v1/mcp-servers/" + serverID + "/challenge?agent_id=" + agentID, ""},
		{fiber.MethodPost, "/api/v1/mcp-servers/" + serverID + "/attest", "{}"},
	}
	probes := []struct {
		name    string
		headers map[string]string
		status  int
		body    map[string]any
	}{
		{"no credentials", nil,
			fiber.StatusUnauthorized, map[string]any{"error": "No authentication token provided"}},
		{"an agent ID without a timestamp", map[string]string{"X-Agent-ID": agentID},
			fiber.StatusUnauthorized, map[string]any{"error": "No authentication token provided"}},
		{"a bearer value that is not a token", map[string]string{"Authorization": "Bearer not-a-token"},
			fiber.StatusUnauthorized, map[string]any{"error": "Invalid or expired token"}},
		{"an Authorization header of another scheme", map[string]string{"Authorization": "Basic not-a-credential"},
			fiber.StatusUnauthorized, map[string]any{"error": "Invalid authorization header format"}},
		// AuthMiddleware admits a signed-in user's session and sets its organization. The
		// handler then runs; it has no service in this test and answers 500.
		{"a signed-in user's session", map[string]string{"Authorization": "Bearer " + session},
			fiber.StatusInternalServerError, nil},
	}

	for _, route := range routes {
		for _, probe := range probes {
			t.Run(route.method+" "+strings.SplitN(route.path, "?", 2)[0][len("/api/v1/mcp-servers/")+len(serverID):]+"/"+probe.name, func(t *testing.T) {
				req := httptest.NewRequest(route.method, route.path, strings.NewReader(route.body))
				req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
				for name, value := range probe.headers {
					req.Header.Set(name, value)
				}
				resp, err := app.Test(req)
				require.NoError(t, err)
				defer resp.Body.Close()

				assert.Equal(t, probe.status, resp.StatusCode)
				if probe.body != nil {
					var body map[string]any
					require.NoError(t, json.NewDecoder(resp.Body).Decode(&body), "the answer is AuthMiddleware's JSON refusal")
					assert.Equal(t, probe.body, body)
				}
			})
		}
	}
}
