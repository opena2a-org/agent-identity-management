package main

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/interfaces/http/handlers"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/interfaces/http/middleware"
)

// An agent credential identifies one agent. Every route that takes an agent ID
// in its path and is reached by an agent authenticator must either refuse an
// agent caller whose ID differs (middleware.AgentPathBinding), or carry a
// written reason it does not. Before this guard the SDK heartbeat, capability,
// MCP and detection routes checked only that the target agent was in the
// caller's organization, so one agent's key could report liveness,
// capabilities, MCP connections and detections for every sibling agent.

// sdkAPITestNameResolver satisfies sdkAPIDeps.AgentNameResolver for tests that
// mount the real table but do not exercise name lookups.
var sdkAPITestNameResolver middleware.AgentNameResolver = func(context.Context, uuid.UUID) (string, error) {
	return "", nil
}

const (
	bindingTestAgentHeader = "X-Test-Agent"
	bindingSentinelStatus  = http.StatusTeapot
)

// bindingTestPrincipal stands in for the group's authenticators. A request
// carrying bindingTestAgentHeader authenticates as that agent, the way PQC,
// Ed25519, API-key, ATC and service-principal authentication do; one without
// it authenticates as a user JWT would (organization and user, no agent).
func bindingTestPrincipal(orgID uuid.UUID) fiber.Handler {
	return func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		if raw := c.Get(bindingTestAgentHeader); raw != "" {
			id, err := uuid.Parse(raw)
			if err != nil {
				return c.SendStatus(http.StatusBadRequest)
			}
			c.Locals("agent_id", id)
			c.Locals("auth_method", "ed25519")
			return c.Next()
		}
		c.Locals("user_id", uuid.New())
		c.Locals("auth_method", "jwt")
		return c.Next()
	}
}

type bindingCall struct {
	status int
	body   string
}

func callAs(t *testing.T, app *fiber.App, method, path string, agent *uuid.UUID) bindingCall {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	if agent != nil {
		req.Header.Set(bindingTestAgentHeader, agent.String())
	}
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return bindingCall{status: resp.StatusCode, body: string(body)}
}

// pathWith fills param with value and every other parameter with a fixed UUID.
func pathWith(path, param, value string) string {
	segments := strings.Split(path, "/")
	for i, segment := range segments {
		switch {
		case segment == ":"+param:
			segments[i] = value
		case strings.HasPrefix(segment, ":"):
			segments[i] = "6f1a8f4e-6b6f-4a3a-9a9a-2f2b1c0d4e5f"
		}
	}
	return strings.Join(segments, "/")
}

type recordingNames struct {
	names map[uuid.UUID]string
	asked []uuid.UUID
}

func (r *recordingNames) resolve(_ context.Context, id uuid.UUID) (string, error) {
	r.asked = append(r.asked, id)
	return r.names[id], nil
}

// TestSDKAPIAgentRoutesBindTheAuthenticatedAgent mounts the real SDK-API table
// and, for every grouped route with an agent parameter, asserts: the caller's
// own ID reaches the handler; a same-organization sibling's ID is refused 403;
// a random UUID gets a byte-identical 403; a user JWT caller is unchanged.
// Grouped routes whose parameter names something else (a verification) are a
// different class, and the binding must not touch them.
func TestSDKAPIAgentRoutesBindTheAuthenticatedAgent(t *testing.T) {
	orgID, self, sibling := uuid.New(), uuid.New(), uuid.New()
	names := &recordingNames{names: map[uuid.UUID]string{self: "self-agent", sibling: "sibling-agent"}}

	app := fiber.New()
	table := registerSDKAPIRoutes(app, sdkAPIDeps{
		GroupMiddleware:   []fiber.Handler{bindingTestPrincipal(orgID)},
		AgentNameResolver: names.resolve,
		Handlers:          sdkAPITestHandlers(func(c fiber.Ctx) error { return c.SendStatus(bindingSentinelStatus) }),
	})

	var bound, otherClass int
	for _, route := range table {
		if route.Bare {
			continue
		}
		name := route.Method + " " + route.FullPath()
		param := agentPathParam(route.Path)

		if param == "" {
			otherClass++
			t.Run(name+" is not agent-bound", func(t *testing.T) {
				got := callAs(t, app, route.Method, concreteSDKAPIPath(route.FullPath()), &self)
				assert.Equal(t, bindingSentinelStatus, got.status,
					"the parameter on %s names another resource; its handler binds it, not the agent binding", name)
			})
			continue
		}

		if route.AgentBindingException != "" {
			continue
		}
		require.Equal(t, param, route.AgentParam, "%s must bind its agent parameter :%s", name, param)
		bound++

		t.Run(name, func(t *testing.T) {
			full := route.FullPath()

			own := callAs(t, app, route.Method, pathWith(full, param, self.String()), &self)
			assert.Equal(t, bindingSentinelStatus, own.status, "the agent's own ID must reach the handler")

			foreign := callAs(t, app, route.Method, pathWith(full, param, sibling.String()), &self)
			assert.Equal(t, http.StatusForbidden, foreign.status, "a sibling agent's ID must be refused")
			assert.Contains(t, foreign.body, `"reasonCode":"`+middleware.AgentPathMismatchReasonCode+`"`)

			unknown := callAs(t, app, route.Method, pathWith(full, param, uuid.New().String()), &self)
			assert.Equal(t, foreign, unknown, "a sibling's ID and a random UUID must get byte-identical responses")

			user := callAs(t, app, route.Method, pathWith(full, param, sibling.String()), nil)
			assert.Equal(t, bindingSentinelStatus, user.status, "a user JWT caller must be unchanged")

			if !route.AgentParamMayBeName {
				return
			}
			names.asked = nil
			assert.Equal(t, bindingSentinelStatus,
				callAs(t, app, route.Method, pathWith(full, param, "self-agent"), &self).status,
				"the agent's own name must reach the handler")
			siblingName := callAs(t, app, route.Method, pathWith(full, param, "sibling-agent"), &self)
			unknownName := callAs(t, app, route.Method, pathWith(full, param, "no-such-agent"), &self)
			assert.Equal(t, http.StatusForbidden, siblingName.status)
			assert.Equal(t, siblingName, unknownName, "a sibling's name and an unknown name must get byte-identical responses")
			require.NotEmpty(t, names.asked, "a name must be resolved through the resolver")
			for _, asked := range names.asked {
				assert.Equal(t, self, asked, "only the caller's own row may be read")
			}
		})
	}

	// Zero is inconclusive: a table that bound nothing proves nothing.
	assert.Positive(t, bound, "no agent-bound route was exercised")
	assert.Positive(t, otherClass, "no other-class grouped route was exercised")
	for _, required := range []string{
		"POST " + sdkAPIBasePath + "/agents/:id/heartbeat",
		"POST " + sdkAPIBasePath + sdkAPIIsolationAttestationPath,
		"POST " + sdkAPIBasePath + sdkAPIIsolationAliasPath,
		"GET " + sdkAPIBasePath + "/agents/:identifier",
	} {
		var found bool
		for _, route := range table {
			if route.Method+" "+route.FullPath() == required {
				found = route.AgentParam != ""
			}
		}
		assert.True(t, found, "%s must be agent-bound", required)
	}
}

// TestSDKAPIRegistrationRefusesAnUndeclaredAgentRoute: a grouped route with an
// agent parameter that declares neither a binding nor an exception stops the
// boot, as an unsupported method does.
func TestSDKAPIRegistrationRefusesAnUndeclaredAgentRoute(t *testing.T) {
	stub := func(c fiber.Ctx) error { return c.SendStatus(bindingSentinelStatus) }
	deps := sdkAPIDeps{AgentNameResolver: sdkAPITestNameResolver, Handlers: sdkAPITestHandlers(stub)}
	withRow := func(row sdkAPIRoute) []sdkAPIRoute {
		return append(sdkAPIRouteTable(deps.Handlers), row)
	}

	cases := map[string]sdkAPIRoute{
		"undeclared agent route":   {Method: http.MethodPost, Path: "/agents/:id/new-thing", Handler: stub},
		"undeclared agentId route": {Method: http.MethodGet, Path: "/reports/:agentId", Handler: stub},
		"binding names a parameter the path lacks": {
			Method: http.MethodPost, Path: "/agents/:id/new-thing", Handler: stub, AgentParam: "agent",
		},
		"binding and exception together": {
			Method: http.MethodPost, Path: "/agents/:id/new-thing", Handler: stub, AgentParam: "id", AgentBindingException: "x",
		},
		"name lookup without a binding": {
			Method: http.MethodGet, Path: "/agents/:id/new-thing", Handler: stub, AgentParamMayBeName: true, AgentBindingException: "x",
		},
		"binding on a bare route": {
			Method: http.MethodGet, Path: "/agents/:id/new-thing", Handler: stub, Bare: true, AgentParam: "id",
		},
	}
	for name, row := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Panics(t, func() { mountSDKAPIRoutes(fiber.New(), deps, withRow(row)) })
		})
	}

	t.Run("name lookup without a resolver", func(t *testing.T) {
		noResolver := deps
		noResolver.AgentNameResolver = nil
		assert.Panics(t, func() { registerSDKAPIRoutes(fiber.New(), noResolver) })
	})

	t.Run("an admitted exception mounts unbound", func(t *testing.T) {
		app := fiber.New()
		row := sdkAPIRoute{
			Method: http.MethodGet, Path: "/agents/:id/peer-card", Handler: stub,
			AgentBindingException: "peer discovery: any agent may read another agent's public card",
		}
		deps := deps
		deps.GroupMiddleware = []fiber.Handler{bindingTestPrincipal(uuid.New())}
		mountSDKAPIRoutes(app, deps, []sdkAPIRoute{row})
		self := uuid.New()
		got := callAs(t, app, http.MethodGet, sdkAPIBasePath+"/agents/"+uuid.New().String()+"/peer-card", &self)
		assert.Equal(t, bindingSentinelStatus, got.status)
	})
}

// twoAgentStub knows two agents in one organization, so an unbound route would
// let the handler accept the sibling.
type twoAgentStub struct {
	handlers.AgentServicer
	agents map[uuid.UUID]*domain.Agent
}

func (s *twoAgentStub) GetAgent(_ context.Context, id uuid.UUID) (*domain.Agent, error) {
	if agent, ok := s.agents[id]; ok {
		return agent, nil
	}
	return nil, fmt.Errorf("agent %s not found", id)
}

// TestIsolationAttestationRefusesASiblingThroughTheRealTable runs the real
// handler behind the real table. The handler no longer carries its own copy of
// the comparison; the route does, and a sibling's posture is never recorded.
func TestIsolationAttestationRefusesASiblingThroughTheRealTable(t *testing.T) {
	orgID, self, sibling := uuid.New(), uuid.New(), uuid.New()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := &aim03CountingRepo{IsolationAttestationRepository: repository.NewIsolationAttestationRepository(db)}
	calculator := &application.TrustCalculator{}
	calculator.SetIsolationRepo(repo)

	agents := &twoAgentStub{agents: map[uuid.UUID]*domain.Agent{
		self:    {ID: self, OrganizationID: orgID, Name: "self-agent"},
		sibling: {ID: sibling, OrganizationID: orgID, Name: "sibling-agent"},
	}}
	handler := handlers.NewTrustScoreHandlerWithInterfaces(calculator, agents, &aim03AuditStub{})

	set := sdkAPITestHandlers(func(c fiber.Ctx) error { return c.SendStatus(http.StatusNotImplemented) })
	set.SubmitIsolationAttestation = handler.SubmitIsolationAttestation
	app := fiber.New()
	registerSDKAPIRoutes(app, sdkAPIDeps{
		GroupMiddleware:   []fiber.Handler{bindingTestPrincipal(orgID)},
		AgentNameResolver: agentNameResolver(agents),
		Handlers:          set,
	})

	for _, path := range []string{sdkAPIIsolationAttestationPath, sdkAPIIsolationAliasPath} {
		target := sdkAPIBasePath + strings.ReplaceAll(path, ":id", sibling.String())
		req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(aim03TSSDKPayload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(bindingTestAgentHeader, self.String())
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
		assert.Equal(t, http.StatusForbidden, resp.StatusCode, "an agent must not attest for a sibling agent via %s", path)
	}
	assert.Zero(t, repo.creates, "no attestation may be recorded for a sibling agent")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestAgentBoundRouterBindsAndNamesTheRoute covers the one registration helper
// the groups outside the SDK-API table use.
func TestAgentBoundRouterBindsAndNamesTheRoute(t *testing.T) {
	orgID, self, sibling := uuid.New(), uuid.New(), uuid.New()
	app := fiber.New()
	group := app.Group("/api/v1/detection")
	group.Use(bindingTestPrincipal(orgID))
	bindAgentRoutes(group).Post("/agents/:id/report", func(c fiber.Ctx) error { return c.SendStatus(bindingSentinelStatus) })

	path := "/api/v1/detection/agents/%s/report"
	assert.Equal(t, bindingSentinelStatus, callAs(t, app, http.MethodPost, fmt.Sprintf(path, self), &self).status)
	assert.Equal(t, http.StatusForbidden, callAs(t, app, http.MethodPost, fmt.Sprintf(path, sibling), &self).status)
	assert.Equal(t, bindingSentinelStatus, callAs(t, app, http.MethodPost, fmt.Sprintf(path, sibling), nil).status)

	assert.Panics(t, func() { bindAgentRoutes(fiber.New()) }, "the app is not a group: its routes could not be named")
	assert.Panics(t, func() {
		bindAgentRoutes(fiber.New().Group("/api/v1/a2a")).Post("/tasks/:id/state", func(c fiber.Ctx) error { return nil })
	}, "a route with no agent parameter has nothing to bind")
}

// TestHeldAgentRouteServesEveryCallerAndRecordsTheComparison covers
// holdAgentRoutes: a held route refuses nothing, and leaves for the request
// record the route and whether an agent caller named its own ID or another.
func TestHeldAgentRouteServesEveryCallerAndRecordsTheComparison(t *testing.T) {
	orgID, self, sibling, unknown := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	app := fiber.New()
	group := app.Group("/api/v1/agents")
	group.Use(bindingTestPrincipal(orgID))
	echoRecord := func(c fiber.Ctx) error {
		route, _ := c.Locals("agent_path_route").(string)
		outcome, _ := c.Locals("agent_path_outcome").(string)
		return c.Status(bindingSentinelStatus).SendString(route + "|" + outcome)
	}
	held := holdAgentRoutes(group)
	held.Get("/:id", echoRecord)
	held.Put("/:id", echoRecord)
	held.Delete("/:id/tags/:tagId", echoRecord)

	tag := uuid.New().String()
	for _, tc := range []struct {
		name, method, path string
		agent              *uuid.UUID
		want               string
	}{
		{"own id", http.MethodGet, "/api/v1/agents/" + self.String(), &self, "GET /api/v1/agents/:id|self"},
		{"sibling id", http.MethodGet, "/api/v1/agents/" + sibling.String(), &self, "GET /api/v1/agents/:id|other"},
		{"unknown id", http.MethodGet, "/api/v1/agents/" + unknown.String(), &self, "GET /api/v1/agents/:id|other"},
		{"user caller", http.MethodGet, "/api/v1/agents/" + sibling.String(), nil, "GET /api/v1/agents/:id|no_principal"},
		{"put", http.MethodPut, "/api/v1/agents/" + sibling.String(), &self, "PUT /api/v1/agents/:id|other"},
		{"delete", http.MethodDelete, "/api/v1/agents/" + sibling.String() + "/tags/" + tag, &self,
			"DELETE /api/v1/agents/:id/tags/:tagId|other"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := callAs(t, app, tc.method, tc.path, tc.agent)
			assert.Equal(t, bindingSentinelStatus, got.status, "a held route must serve every caller")
			assert.Equal(t, tc.want, got.body)
		})
	}

	assert.Panics(t, func() { holdAgentRoutes(fiber.New()) }, "the app is not a group: its routes could not be named")
	assert.Panics(t, func() {
		holdAgentRoutes(fiber.New().Group("/api/v1/agents")).Get("/", func(c fiber.Ctx) error { return nil })
	}, "a route with no agent parameter has nothing to hold")
}

// agentBindingHoldUntil bounds the hold on the routes in agentBindingHeld. The
// hold exists because the effect of binding them has not been measured: they
// serve the dashboard and API-key automation as well as SDK agents. Each is
// released by a count, over 14 days of request records, of agent-principal
// requests whose path ID differs from the caller: zero binds the route;
// non-zero makes it an admitted exception naming the call site, or binds it
// with a dated migration. After this date a route still held fails the test —
// the remedy is the measurement, never a later date.
//
// A held route is registered through holdAgentRoutes, which records that count
// on each request's api_calls row (agent_path_route, agent_path_outcome,
// auth_method); scripts/agent_path_binding_report.sql reads it per route.
const agentBindingHoldUntil = "2026-11-05"

const agentBindingHoldReason = "held: binding effect on dashboard, API-key and service-principal callers not yet measured"

// agentBindingHeld lists, by route, every agent-parameter route under an
// agent-authenticated group that is neither bound nor an admitted exception.
var agentBindingHeld = []string{
	"GET /api/v1/agents/:id",
	"PUT /api/v1/agents/:id",
	"DELETE /api/v1/agents/:id",
	"POST /api/v1/agents/:id/verify",
	"POST /api/v1/agents/:id/suspend",
	"POST /api/v1/agents/:id/reactivate",
	"POST /api/v1/agents/:id/revoke",
	"POST /api/v1/agents/:id/rotate-credentials",
	"PUT /api/v1/agents/:id/keys",
	"POST /api/v1/agents/:id/verify-capability",
	"POST /api/v1/agents/:id/log-capability/:audit_id",
	"POST /api/v1/agents/:id/authorize",
	"POST /api/v1/agents/:id/atc",
	"GET /api/v1/agents/:id/sdk",
	"GET /api/v1/agents/:id/credentials",
	"GET /api/v1/agents/:id/mcp-servers",
	"PUT /api/v1/agents/:id/mcp-servers",
	"DELETE /api/v1/agents/:id/mcp-servers/:mcp_id",
	"POST /api/v1/agents/:id/attestations/revoke-all",
	"POST /api/v1/agents/:id/mcp-servers/detect",
	"GET /api/v1/agents/:id/trust-score",
	"GET /api/v1/agents/:id/trust-score/history",
	"PUT /api/v1/agents/:id/trust-score",
	"POST /api/v1/agents/:id/trust-score/recalculate",
	"GET /api/v1/agents/:id/alerts",
	"GET /api/v1/agents/:id/key-vault",
	"GET /api/v1/agents/:id/audit-logs",
	"GET /api/v1/agents/:id/activity",
	"GET /api/v1/agents/:id/pqc-key",
	"POST /api/v1/agents/:id/pqc-key",
	"PUT /api/v1/agents/:id/pqc-key",
	"POST /api/v1/agents/:id/hybrid-mode",
	"GET /api/v1/agents/:id/tags",
	"POST /api/v1/agents/:id/tags",
	"DELETE /api/v1/agents/:id/tags/:tagId",
	"GET /api/v1/agents/:id/tags/suggestions",
	"GET /api/v1/agents/:id/capabilities",
	"POST /api/v1/agents/:id/capabilities",
	"DELETE /api/v1/agents/:id/capabilities/:capabilityId",
	"PUT /api/v1/agents/:id/capabilities/:capabilityId/honeytoken",
	"GET /api/v1/agents/:id/violations",

	// A2A reads: candidates for admitted exceptions (peer discovery reads
	// another agent's card, trust and skills); the count decides.
	"GET /api/v1/a2a/agents/:id/card",
	"GET /api/v1/a2a/agents/:id/trust-score",
	"GET /api/v1/a2a/agents/:id/peers/:peer_id/trust",
	"GET /api/v1/a2a/agents/:id/skills",
	"GET /api/v1/a2a/attestations/:agentId/:skillId",
	"GET /api/v1/a2a/consensus/:agentId/:skillId",
	"GET /api/v1/a2a/agents/:id/attestations",
	"GET /api/v1/a2a/agents/:id/skills/:skillId/consensus",
}

// agentBindingExceptions maps a route to the reason it serves agent callers
// other than the one its path names. Empty today.
var agentBindingExceptions = map[string]string{}

// TestAgentParameterRoutesUnderAgentGroupsAreDispositioned walks every non-test
// Go file of the backend, finds each group that mounts an agent authenticator
// and each route on it with an agent parameter, and fails unless the route is
// bound (registered through bindAgentRoutes), an admitted exception with a
// reason, or a dated hold. The SDK-API table is mounted from a data table the
// walk cannot read, so its rows are covered by
// TestSDKAPIAgentRoutesBindTheAuthenticatedAgent instead; the walk requires
// every authenticator mount to be one of the two shapes it understands.
func TestAgentParameterRoutesUnderAgentGroupsAreDispositioned(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	backend := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))

	census := walkAgentRouteCensus(t, backend)
	require.Positive(t, census.agentGroups, "no group mounting an agent authenticator was found; the walk is broken")
	require.NotEmpty(t, census.routes, "no agent-parameter route was found; zero is inconclusive")

	held := map[string]bool{}
	for _, route := range agentBindingHeld {
		require.False(t, held[route], "%s is listed twice", route)
		held[route] = true
	}

	holdEnd, err := time.Parse("2006-01-02", agentBindingHoldUntil)
	require.NoError(t, err)
	holdExpired := time.Now().UTC().After(holdEnd.Add(24 * time.Hour))

	seen := map[string]bool{}
	var bound int
	for _, route := range census.routes {
		seen[route.key] = true
		reason, excepted := agentBindingExceptions[route.key]
		switch {
		case route.bound && (held[route.key] || excepted):
			t.Errorf("%s (%s) is bound; delete its hold or exception", route.key, route.at)
		case route.observed && !held[route.key]:
			t.Errorf("%s (%s) is registered through holdAgentRoutes but is not in agentBindingHeld; "+
				"bind it (bindAgentRoutes) or list the hold", route.key, route.at)
		case held[route.key] && !route.observed:
			t.Errorf("%s (%s) is held but not registered through holdAgentRoutes, so nothing records the count "+
				"that ends the hold", route.key, route.at)
		case route.bound:
			bound++
		case excepted && strings.TrimSpace(reason) == "":
			t.Errorf("%s (%s) is an exception with no reason", route.key, route.at)
		case excepted && held[route.key]:
			t.Errorf("%s (%s) is both held and excepted", route.key, route.at)
		case excepted:
		case held[route.key] && holdExpired:
			t.Errorf("%s (%s) is still held after %s (%s). Read the per-route count and bind it "+
				"(bindAgentRoutes) or move it to agentBindingExceptions with the call site that needs it.",
				route.key, route.at, agentBindingHoldUntil, agentBindingHoldReason)
		case held[route.key]:
		default:
			t.Errorf("%s (%s) takes an agent ID under a group that authenticates agents, and is neither bound nor "+
				"an admitted exception. Register it through bindAgentRoutes, or add it to agentBindingExceptions with the "+
				"reason an agent may act on another agent's ID there.", route.key, route.at)
		}
	}
	for _, route := range agentBindingHeld {
		if !seen[route] {
			t.Errorf("held route %s is no longer registered; delete it from agentBindingHeld", route)
		}
	}
	for route := range agentBindingExceptions {
		if !seen[route] {
			t.Errorf("exception %s is no longer registered; delete it", route)
		}
	}

	var observed int
	for _, route := range census.routes {
		if route.observed {
			observed++
		}
	}
	t.Logf("census: %d agent-parameter routes under %d agent-authenticated groups; %d bound, %d held until %s "+
		"(%d observed), %d excepted",
		len(census.routes), census.agentGroups, bound, len(agentBindingHeld), agentBindingHoldUntil, observed,
		len(agentBindingExceptions))
	assert.Positive(t, bound, "no route registered through bindAgentRoutes was found")
	for _, required := range []string{
		"POST /api/v1/detection/agents/:id/report",
		"GET /api/v1/detection/agents/:id/status",
		"POST /api/v1/detection/agents/:id/capabilities/report",
		"GET /api/v1/detection/agents/:id/capabilities/latest",
		"POST /api/v1/a2a/agents/:id/card",
		"POST /api/v1/a2a/agents/:id/card/refresh",
		"POST /api/v1/a2a/agents/:id/sign",
		"POST /api/v1/a2a/agents/:id/trust-score/compute",
	} {
		var isBound bool
		for _, route := range census.routes {
			if route.key == required && route.bound {
				isBound = true
			}
		}
		assert.True(t, isBound, "%s must be registered through bindAgentRoutes", required)
	}
}

type censusRoute struct {
	key      string // "METHOD /full/path"
	at       string // file:line
	bound    bool   // registered through bindAgentRoutes
	observed bool   // registered through holdAgentRoutes
}

type agentRouteCensus struct {
	agentGroups int
	routes      []censusRoute
}

var fiberRouteMethods = map[string]string{
	"Get": http.MethodGet, "Post": http.MethodPost, "Put": http.MethodPut,
	"Delete": http.MethodDelete, "Patch": http.MethodPatch, "Head": http.MethodHead,
	"Options": http.MethodOptions, "All": "ALL",
}

// walkAgentRouteCensus parses every non-test Go file under backend.
func walkAgentRouteCensus(t *testing.T, backend string) agentRouteCensus {
	t.Helper()
	fset := token.NewFileSet()
	var files []*ast.File
	err := filepath.WalkDir(backend, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if path != backend && (strings.HasPrefix(name, ".") || name == "vendor" || name == "testdata" || name == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		files = append(files, file)
		return nil
	})
	require.NoError(t, err)

	authenticators := agentAuthenticators(t, fset, files)
	require.NotEmpty(t, authenticators, "no middleware function sets the agent principal; the walk is broken")

	at := func(pos token.Pos) string {
		p := fset.Position(pos)
		rel, err := filepath.Rel(backend, p.Filename)
		if err != nil {
			rel = p.Filename
		}
		return rel + ":" + strconv.Itoa(p.Line)
	}
	isAuthenticator := func(expr ast.Expr) bool {
		call, ok := expr.(*ast.CallExpr)
		if !ok {
			return false
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return false
		}
		pkg, ok := sel.X.(*ast.Ident)
		return ok && pkg.Name == "middleware" && authenticators[sel.Sel.Name]
	}

	// A group follows into the functions it is passed to (main passes the
	// /api/v1 group to setupRoutes as a parameter), so each function is
	// analysed with the prefix, and the agent authentication, its callers give
	// its parameters, until nothing new is learned. bindAgentRoutes and
	// holdAgentRoutes are the helpers the walk reads natively and are not
	// followed.
	type funcKey struct{ pkg, name string }
	funcs := map[funcKey]*ast.FuncDecl{}
	pkgOf := map[*ast.FuncDecl]string{}
	for _, file := range files {
		// A package is a directory: several binaries each have a package main.
		pkg := filepath.Dir(fset.Position(file.Pos()).Filename) + ":" + file.Name.Name
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Body != nil && fn.Recv == nil {
				funcs[funcKey{pkg, fn.Name.Name}] = fn
				pkgOf[fn] = pkg
			}
		}
	}
	type seed struct {
		prefix    string
		agent     bool
		ambiguous bool
	}
	seeds := map[*ast.FuncDecl]map[string]seed{}

	type scope struct {
		prefixes    map[string]string // group variable -> full prefix
		agentGroup  map[string]bool   // group variable -> mounts an authenticator
		boundRouter map[string]string // bindAgentRoutes variable -> group variable
		heldRouter  map[string]string // holdAgentRoutes variable -> group variable
		unresolved  []string          // authenticator mounted on a group of unknown prefix
	}
	analyse := func(fn *ast.FuncDecl) scope {
		sc := scope{prefixes: map[string]string{}, agentGroup: map[string]bool{}, boundRouter: map[string]string{},
			heldRouter: map[string]string{}}
		for name, sd := range seeds[fn] {
			if !sd.ambiguous {
				sc.prefixes[name] = sd.prefix
			}
			if !sd.ambiguous && sd.agent {
				sc.agentGroup[name] = true
			}
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.CallExpr:
				sel, ok := node.Fun.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "Use" {
					return true
				}
				recv, ok := sel.X.(*ast.Ident)
				if !ok {
					return true
				}
				for _, arg := range node.Args {
					if isAuthenticator(arg) {
						if _, known := sc.prefixes[recv.Name]; !known {
							sc.unresolved = append(sc.unresolved, at(arg.Pos())+" on "+recv.Name)
							continue
						}
						sc.agentGroup[recv.Name] = true
					}
				}
			case *ast.AssignStmt:
				if len(node.Lhs) != 1 || len(node.Rhs) != 1 {
					return true
				}
				lhs, ok := node.Lhs[0].(*ast.Ident)
				if !ok {
					return true
				}
				call, ok := node.Rhs[0].(*ast.CallExpr)
				if !ok {
					return true
				}
				if fun, ok := call.Fun.(*ast.Ident); ok && len(call.Args) == 1 &&
					(fun.Name == "bindAgentRoutes" || fun.Name == "holdAgentRoutes") {
					if group, ok := call.Args[0].(*ast.Ident); ok {
						if fun.Name == "bindAgentRoutes" {
							sc.boundRouter[lhs.Name] = group.Name
						} else {
							sc.heldRouter[lhs.Name] = group.Name
						}
					}
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "Group" || len(call.Args) == 0 {
					return true
				}
				lit, ok := call.Args[0].(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					return true
				}
				prefix, err := strconv.Unquote(lit.Value)
				require.NoError(t, err)
				if parent, ok := sel.X.(*ast.Ident); ok {
					prefix = sc.prefixes[parent.Name] + prefix
				}
				sc.prefixes[lhs.Name] = prefix
				for _, arg := range call.Args[1:] {
					if isAuthenticator(arg) {
						sc.agentGroup[lhs.Name] = true
					}
				}
			}
			return true
		})
		return sc
	}

	for round := 0; ; round++ {
		require.Less(t, round, 10, "group prefixes did not settle")
		learned := false
		for _, fn := range funcs {
			sc := analyse(fn)
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				fun, ok := call.Fun.(*ast.Ident)
				if !ok {
					return true
				}
				callee, ok := funcs[funcKey{pkgOf[fn], fun.Name}]
				if !ok || callee.Name.Name == "bindAgentRoutes" || callee.Name.Name == "holdAgentRoutes" {
					return true
				}
				var params []string
				for _, field := range callee.Type.Params.List {
					for _, name := range field.Names {
						params = append(params, name.Name)
					}
				}
				for i, arg := range call.Args {
					ident, ok := arg.(*ast.Ident)
					if !ok || i >= len(params) {
						continue
					}
					prefix, ok := sc.prefixes[ident.Name]
					if !ok {
						continue
					}
					if seeds[callee] == nil {
						seeds[callee] = map[string]seed{}
					}
					next := seed{prefix: prefix, agent: sc.agentGroup[ident.Name]}
					existing, seen := seeds[callee][params[i]]
					switch {
					case !seen:
						seeds[callee][params[i]] = next
						learned = true
					case existing.ambiguous || existing == next:
					default:
						require.False(t, existing.agent || next.agent,
							"%s: %s receives different groups in %s, one of them agent-authenticated; the census cannot follow it",
							at(call.Pos()), callee.Name.Name, params[i])
						seeds[callee][params[i]] = seed{ambiguous: true}
						learned = true
					}
				}
				return true
			})
		}
		if !learned {
			break
		}
	}

	var census agentRouteCensus
	understood := map[token.Pos]bool{}
	var mounts []token.Pos

	for _, fn := range funcs {
		sc := analyse(fn)
		for _, where := range sc.unresolved {
			t.Errorf("%s: agent authenticator mounted on a group whose prefix the walk cannot resolve", where)
		}
		census.agentGroups += len(sc.agentGroup)

		ast.Inspect(fn.Body, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.KeyValueExpr:
				if key, ok := node.Key.(*ast.Ident); ok && key.Name == "GroupMiddleware" {
					if list, ok := node.Value.(*ast.CompositeLit); ok {
						for _, elt := range list.Elts {
							if isAuthenticator(elt) {
								understood[elt.Pos()] = true
							}
						}
					}
				}
				return true
			case *ast.CallExpr:
				if isAuthenticator(node) {
					mounts = append(mounts, node.Pos())
				}
				sel, ok := node.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				recv, ok := sel.X.(*ast.Ident)
				if !ok {
					return true
				}
				if sel.Sel.Name == "Use" || sel.Sel.Name == "Group" {
					if sc.agentGroup[recv.Name] || sel.Sel.Name == "Group" {
						for _, arg := range node.Args {
							if isAuthenticator(arg) {
								understood[arg.Pos()] = true
							}
						}
					}
					return true
				}
				method, isRoute := fiberRouteMethods[sel.Sel.Name]
				if !isRoute || len(node.Args) == 0 {
					return true
				}
				group, bound := sc.boundRouter[recv.Name]
				heldGroup, observed := sc.heldRouter[recv.Name]
				switch {
				case bound:
				case observed:
					group = heldGroup
				default:
					group = recv.Name
				}
				if !sc.agentGroup[group] {
					return true
				}
				lit, ok := node.Args[0].(*ast.BasicLit)
				require.True(t, ok && lit.Kind == token.STRING,
					"%s: route on agent-authenticated group %s with a non-literal path; the census cannot read it", at(node.Pos()), group)
				path, err := strconv.Unquote(lit.Value)
				require.NoError(t, err)
				full := sc.prefixes[group] + path
				if agentPathParam(full) == "" {
					return true
				}
				census.routes = append(census.routes, censusRoute{
					key: method + " " + full, at: at(node.Pos()), bound: bound, observed: observed,
				})
			}
			return true
		})
	}

	for _, pos := range mounts {
		assert.True(t, understood[pos], "%s: agent authenticator mounted in a shape the census cannot see "+
			"(only Group(...)/Use(...) on a named group, or sdkAPIDeps.GroupMiddleware, are understood)", at(pos))
	}

	sort.Slice(census.routes, func(i, j int) bool { return census.routes[i].at < census.routes[j].at })
	return census
}

// agentAuthenticators derives, from the middleware package source, the
// exported functions that set the agent principal (c.Locals("agent_id", …)).
// Every such setter must sit inside one, so no authenticator is missed.
func agentAuthenticators(t *testing.T, fset *token.FileSet, files []*ast.File) map[string]bool {
	t.Helper()
	found := map[string]bool{}
	for _, file := range files {
		if file.Name.Name != "middleware" {
			continue
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok || len(call.Args) != 2 {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "Locals" {
					return true
				}
				key, ok := call.Args[0].(*ast.BasicLit)
				if !ok || key.Value != `"agent_id"` {
					return true
				}
				require.True(t, fn.Name.IsExported() && fn.Recv == nil,
					"%s: the agent principal is set inside %s, which is not an exported middleware constructor; "+
						"the census cannot attribute it to a mount", fset.Position(call.Pos()), fn.Name.Name)
				found[fn.Name.Name] = true
				return true
			})
		}
	}
	return found
}
