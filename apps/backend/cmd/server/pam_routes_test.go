package main

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// pamRoleRank orders the roles a dashboard session carries.
var pamRoleRank = map[domain.UserRole]int{
	domain.RoleViewer:  0,
	domain.RoleMember:  1,
	domain.RoleManager: 2,
	domain.RoleAdmin:   3,
}

// pamGateRefusal is the answer of the gate that refuses a role below the one
// a route needs.
var pamGateRefusal = map[domain.UserRole]string{
	domain.RoleMember:  "Member access required (viewers cannot perform this action)",
	domain.RoleManager: "Manager or admin access required",
	domain.RoleAdmin:   "Admin access required",
}

// The /api/v1/pam routes are mounted by mountAPIRoutes, the call main makes,
// each behind the session check and the role it needs. On the census app the
// PAM handler has no service: a request every gate admits reaches the handler,
// which calls the service and is answered 500 by the app's panic recovery, and
// a request a gate refuses is answered by that gate. The approval request
// carries an agent ID, which the handler reads before it calls the service.
func TestPAMRoutesAreMountedWithTheirRoleGates(t *testing.T) {
	app, jwtService := strictCensusApp(t)
	id := uuid.NewString()
	routes := []struct {
		method, path, body string
		needs              domain.UserRole
	}{
		{fiber.MethodGet, "/api/v1/pam/tiers", "", domain.RoleViewer},
		{fiber.MethodPost, "/api/v1/pam/approvals", `{"agentId":"` + id + `"}`, domain.RoleMember},
		{fiber.MethodGet, "/api/v1/pam/approvals", "", domain.RoleViewer},
		{fiber.MethodGet, "/api/v1/pam/approvals/" + id, "", domain.RoleViewer},
		{fiber.MethodPost, "/api/v1/pam/approvals/" + id + "/decision", "{}", domain.RoleAdmin},
		{fiber.MethodPost, "/api/v1/pam/emergencies", "{}", domain.RoleAdmin},
		{fiber.MethodGet, "/api/v1/pam/emergencies", "", domain.RoleViewer},
		{fiber.MethodPost, "/api/v1/pam/emergencies/" + id + "/extend", "", domain.RoleAdmin},
		{fiber.MethodPost, "/api/v1/pam/emergencies/" + id + "/revoke", "", domain.RoleAdmin},
		{fiber.MethodPost, "/api/v1/pam/campaigns", "{}", domain.RoleAdmin},
		{fiber.MethodGet, "/api/v1/pam/campaigns", "", domain.RoleViewer},
		{fiber.MethodPost, "/api/v1/pam/campaigns/" + id + "/decisions", "{}", domain.RoleManager},
		{fiber.MethodGet, "/api/v1/pam/campaigns/" + id + "/decisions", "", domain.RoleViewer},
	}

	sessions := map[domain.UserRole]string{}
	for role := range pamRoleRank {
		token, err := jwtService.GenerateAccessToken(uuid.NewString(), uuid.NewString(), string(role)+"@example.com", string(role))
		require.NoError(t, err)
		sessions[role] = token
	}

	probe := 0
	send := func(method, path, payload, token string) (int, map[string]any) {
		probe++
		req := httptest.NewRequest(method, path, strings.NewReader(payload))
		req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
		// Each probe has its own client address, so its own rate limiter bucket.
		req.Header.Set("X-Real-IP", fmt.Sprintf("10.9.%d.%d", probe/250, probe%250+1))
		if token != "" {
			req.Header.Set(fiber.HeaderAuthorization, "Bearer "+token)
		}
		resp, err := app.Test(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		var body map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&body)
		return resp.StatusCode, body
	}

	for _, route := range routes {
		t.Run(route.method+" "+strings.ReplaceAll(route.path, id, ":id"), func(t *testing.T) {
			status, body := send(route.method, route.path, route.body, "")
			assert.Equal(t, fiber.StatusUnauthorized, status, "no session")
			assert.Equal(t, "No authentication token provided", body["error"])

			for role, rank := range pamRoleRank {
				status, body := send(route.method, route.path, route.body, sessions[role])
				if rank >= pamRoleRank[route.needs] {
					assert.Equal(t, fiber.StatusInternalServerError, status, "%s reaches the handler", role)
					continue
				}
				assert.Equal(t, fiber.StatusForbidden, status, "%s is refused", role)
				assert.Equal(t, pamGateRefusal[route.needs], body["error"], "%s is refused by the %s gate", role, route.needs)
			}
		})
	}
}
