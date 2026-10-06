package main

import (
	"net/http"

	"github.com/gofiber/fiber/v3"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/interfaces/http/middleware"
)

// SDK-API route table.
//
// Every path the SDK clients call is written down exactly once, here, and both
// main.go and the tests mount the table through registerSDKAPIRoutes. Before
// this file existed the registration lived inline in main() and the integration
// tests hand-registered each handler at a path of their own choosing — so the
// tests agreed with themselves while every shipped SDK 404'd against the real
// server. That is how POST /agents/:id/isolation-attestation went un-served
// from the moment the SDKs shipped: the suite never asked the server what it
// listens on. Tests may hand-mount handlers (a stub instead of a live service);
// they may not hand-choose paths.
//
// recorded architecture decision of 2026-08-29: the canonical path is the SDK/spec path,
// /api/v1/sdk-api/agents/:id/isolation-attestation. The backend gains it on the
// existing handler, and /agents/:id/isolation stays registered as a deprecated
// alias to the same handler — Binding Decision 6 forbids removing a shipped
// path.

// sdkAPIBasePath is the prefix every SDK-facing route hangs off. Route paths in
// the table below are relative to it; sdkAPIRoute.FullPath joins the two.
const sdkAPIBasePath = "/api/v1/sdk-api"

const (
	// sdkAPIIsolationAttestationPath is the canonical isolation-attestation
	// path: the one the shipped TypeScript, Python and Java clients emit, and
	// the one the execution-isolation spec documents.
	sdkAPIIsolationAttestationPath = "/agents/:id/isolation-attestation"

	// sdkAPIIsolationAliasPath is the path the backend served alone until this
	// change. No shipped client emits it, but it is a published route and BD6
	// forbids removal, so it stays mounted on the same handler.
	sdkAPIIsolationAliasPath = "/agents/:id/isolation"
)

// sdkAPIHandlers is one fiber.Handler per entry in the SDK-API route table.
//
// The table is expressed over handlers rather than over the concrete *Handlers
// struct so a test can mount the real paths with stand-in handlers: what a test
// is allowed to substitute is the handler, never the path.
type sdkAPIHandlers struct {
	CreateVerification          fiber.Handler
	GetVerificationSDK          fiber.Handler
	SubmitVerificationResult    fiber.Handler
	UpdateExecutionStatus       fiber.Handler
	GetAgentByIdentifier        fiber.Handler
	GrantCapability             fiber.Handler
	RegisterCapability          fiber.Handler
	ListAgentCapabilityRequests fiber.Handler
	CreateCapabilityRequest     fiber.Handler
	CreateMCPServer             fiber.Handler
	ListMCPServers              fiber.Handler
	GetMCPServerByName          fiber.Handler
	RecordMCPConnection         fiber.Handler
	RecordMCPUsageReport        fiber.Handler
	ReportDetection             fiber.Handler
	Heartbeat                   fiber.Handler
	SubmitIsolationAttestation  fiber.Handler
}

// sdkAPIRoute is one registered SDK-API route.
type sdkAPIRoute struct {
	// Method is an http.Method* constant.
	Method string

	// Path is relative to sdkAPIBasePath.
	Path string

	// Handler serves the route.
	Handler fiber.Handler

	// Bare marks a route registered directly on the app, ABOVE the group.
	// Fiber matches in registration order, so a bare route is never reached by
	// the group's Ed25519/JWT middleware — its only middleware is the rate
	// limiter, which authenticates nothing. Every bare route must establish its
	// own caller identity inside the handler, and routes_gate_test.go requires
	// each one to be enumerated there with the control that justifies it.
	Bare bool

	// Deprecated marks a path kept only so existing callers keep working. It is
	// still registered and still served; it is not advertised.
	Deprecated bool

	// AgentParam names the path parameter that must equal the authenticated
	// agent. registerSDKAPIRoutes puts middleware.AgentPathBinding in front of
	// the handler, so when an agent principal is set a different ID is refused
	// 403 before the handler runs; a user JWT caller is unaffected. Every
	// grouped route with an agent parameter declares either this or
	// AgentBindingException, and registration panics on one that declares
	// neither.
	AgentParam string

	// AgentParamMayBeName also admits the caller's own agent name in
	// AgentParam. The name is resolved through sdkAPIDeps.AgentNameResolver for
	// the caller only, never for the agent the path names.
	AgentParamMayBeName bool

	// AgentBindingException records why a grouped route with an agent
	// parameter serves agent principals other than the one the path names.
	AgentBindingException string

	// Note records why the route is shaped the way it is.
	Note string
}

// FullPath is the path a client calls.
func (r sdkAPIRoute) FullPath() string {
	return sdkAPIBasePath + r.Path
}

// sdkAPIDeps is everything registerSDKAPIRoutes needs in order to mount the
// table. Production passes the real middleware and handlers; a test passes
// whatever it needs to observe, and gets the real paths either way.
type sdkAPIDeps struct {
	// BareMiddleware runs ahead of the handler on the app-level routes
	// registered above the group. Optional: nil mounts the handler alone.
	//
	// It is one handler, not a chain, on purpose — a bare route is by
	// definition one the group's authentication does not reach, so the only
	// thing that belongs here is the rate limiter. Anything that needs a chain
	// needs the group.
	BareMiddleware fiber.Handler

	// GroupMiddleware runs on every grouped route. Registered before any route
	// in the group, because Fiber applies a group's Use chain only to routes
	// registered after it.
	GroupMiddleware []fiber.Handler

	// AgentNameResolver returns an agent's name by ID. It backs the routes that
	// declare AgentParamMayBeName and is only ever asked about the caller.
	// Required when the table has such a route.
	AgentNameResolver middleware.AgentNameResolver

	// Handlers supplies one handler per table entry.
	Handlers sdkAPIHandlers
}

// sdkAPIRouteTable is the single source of truth for SDK-facing paths.
func sdkAPIRouteTable(h sdkAPIHandlers) []sdkAPIRoute {
	return []sdkAPIRoute{
		// Bare mounts first: registration order decides matching order, and
		// these two must keep winning over anything in the group.
		{
			Method: http.MethodPost, Path: "/verifications", Bare: true,
			Handler: h.CreateVerification,
			Note:    "verifies an Ed25519 signature over the request and gates on agent.Status inside the handler",
		},
		{
			Method: http.MethodGet, Path: "/verifications/:id", Bare: true,
			Handler: h.GetVerificationSDK,
			Note:    "read path; verifies three X-AIM-* headers, an Ed25519 signature and event ownership inside the handler (defect #160)",
		},

		// Grouped routes: authenticated by the group's middleware chain.
		{Method: http.MethodPost, Path: "/verifications/:id/result", Handler: h.SubmitVerificationResult, Note: "withdrawn: 403 for every caller"},
		{Method: http.MethodPost, Path: "/verifications/:id/execution-status", Handler: h.UpdateExecutionStatus, Note: "agent-owned execution report"},
		//
		// Every /agents/ route binds its agent parameter to the authenticated
		// agent: the shipped SDKs call each one with their own ID, and an agent
		// credential must not reach a sibling agent's record. The name lookup
		// on /agents/:identifier runs under a user token during first-run setup
		// and is unaffected; under an agent principal it admits only the
		// caller's own ID or name.
		{
			Method: http.MethodGet, Path: "/agents/:identifier", Handler: h.GetAgentByIdentifier,
			AgentParam: "identifier", AgentParamMayBeName: true,
			Note: "get agent by ID or name (SDK)",
		},
		{Method: http.MethodPost, Path: "/agents/:id/capabilities", Handler: h.GrantCapability, AgentParam: "id", Note: "SDK capability reporting (legacy)"},
		{Method: http.MethodPost, Path: "/agents/:id/capabilities/register", Handler: h.RegisterCapability, AgentParam: "id", Note: "SDK capability registration (respects enforcement mode)"},
		{Method: http.MethodGet, Path: "/agents/:id/capability-requests", Handler: h.ListAgentCapabilityRequests, AgentParam: "id", Note: "SDK list agent's capability requests"},
		{Method: http.MethodPost, Path: "/agents/:id/capability-requests", Handler: h.CreateCapabilityRequest, AgentParam: "id", Note: "SDK capability request creation"},
		{Method: http.MethodPost, Path: "/agents/:id/mcp-servers", Handler: h.CreateMCPServer, AgentParam: "id", Note: "SDK MCP registration (create new MCP server)"},
		{Method: http.MethodGet, Path: "/agents/:id/mcp-servers", Handler: h.ListMCPServers, AgentParam: "id", Note: "SDK list MCP servers for agent's org"},
		{Method: http.MethodGet, Path: "/agents/:id/mcp-servers/by-name", Handler: h.GetMCPServerByName, AgentParam: "id", Note: "SDK get MCP by name (capability caching)"},
		{Method: http.MethodPost, Path: "/agents/:id/mcp-connections", Handler: h.RecordMCPConnection, AgentParam: "id", Note: "SDK record agent-MCP connection (use_mcp_tool)"},
		{Method: http.MethodPost, Path: "/agents/:id/mcp-usage-report", Handler: h.RecordMCPUsageReport, AgentParam: "id", Note: "SDK MCP supply chain usage analytics"},
		{Method: http.MethodPost, Path: "/agents/:id/detection/report", Handler: h.ReportDetection, AgentParam: "id", Note: "SDK MCP detection and integration reporting"},
		{Method: http.MethodPost, Path: "/agents/:id/heartbeat", Handler: h.Heartbeat, AgentParam: "id", Note: "SDK agent heartbeat (liveness)"},

		// Trust factor 9. Canonical path first, alias second; both reach the
		// same handler, so an agent's posture lands the same row either way.
		{
			Method: http.MethodPost, Path: sdkAPIIsolationAttestationPath,
			Handler: h.SubmitIsolationAttestation, AgentParam: "id",
			Note: "SDK self-report of runtime isolation posture (trust factor 9); canonical path per the recorded architecture decision of 2026-08-29",
		},
		{
			Method: http.MethodPost, Path: sdkAPIIsolationAliasPath,
			Handler: h.SubmitIsolationAttestation, Deprecated: true, AgentParam: "id",
			Note: "deprecated alias for " + sdkAPIIsolationAttestationPath + "; kept because BD6 forbids removing a published path",
		},
	}
}

// registerSDKAPIRoutes mounts the SDK-API route table on app and returns what
// it registered, so a caller (in practice, a test) can assert against the real
// table rather than against a second copy of the paths.
func registerSDKAPIRoutes(app *fiber.App, deps sdkAPIDeps) []sdkAPIRoute {
	return mountSDKAPIRoutes(app, deps, sdkAPIRouteTable(deps.Handlers))
}

// mountSDKAPIRoutes mounts table. It is registerSDKAPIRoutes with the table as
// an argument, so a test can show that registration refuses a row the real
// table must never contain.
func mountSDKAPIRoutes(app *fiber.App, deps sdkAPIDeps, table []sdkAPIRoute) []sdkAPIRoute {
	// Every binding is built before anything is mounted, so a misdeclared row
	// stops the boot before a single route is served.
	bindings := make([]fiber.Handler, len(table))
	for i, route := range table {
		bindings[i] = sdkAPIAgentBinding(route, deps.AgentNameResolver)
	}

	// SECURITY: bare routes are registered first and deliberately. Fiber matches
	// in registration order, so these are not reached by the group middleware
	// below — see sdkAPIRoute.Bare and routes_gate_test.go. The argument order
	// is middleware-then-handler, exactly as when these lived inline in main().
	for _, route := range table {
		if !route.Bare {
			continue
		}
		path := route.FullPath()
		switch {
		case route.Method == http.MethodGet && deps.BareMiddleware != nil:
			app.Get(path, deps.BareMiddleware, route.Handler)
		case route.Method == http.MethodGet:
			app.Get(path, route.Handler)
		case route.Method == http.MethodPost && deps.BareMiddleware != nil:
			app.Post(path, deps.BareMiddleware, route.Handler)
		case route.Method == http.MethodPost:
			app.Post(path, route.Handler)
		default:
			panic(unsupportedSDKAPIMethod(route))
		}
	}

	group := app.Group(sdkAPIBasePath)
	// Every Use() precedes every route registration: Fiber applies a group's
	// chain only to routes registered after it, so this ordering is what makes
	// the grouped routes authenticated at all.
	for _, mw := range deps.GroupMiddleware {
		group.Use(mw)
	}
	for i, route := range table {
		if route.Bare {
			continue
		}
		// The agent binding sits on the route, between the group chain and the
		// handler: Fiber fills Params only after route matching, so it cannot
		// be a group Use().
		chain := []any{route.Handler}
		if bindings[i] != nil {
			chain = []any{bindings[i], route.Handler}
		}
		switch route.Method {
		case http.MethodGet:
			group.Get(route.Path, chain[0], chain[1:]...)
		case http.MethodPost:
			group.Post(route.Path, chain[0], chain[1:]...)
		default:
			panic(unsupportedSDKAPIMethod(route))
		}
	}

	return table
}

// sdkAPIAgentBinding returns the middleware.AgentPathBinding a route declares,
// or nil for a route with no agent parameter or an admitted exception. It
// panics, as unsupportedSDKAPIMethod does, on a declaration that cannot be
// honoured: a grouped route with an agent parameter and neither AgentParam nor
// AgentBindingException, an AgentParam the path does not contain, a name
// lookup with no resolver, or a binding on a bare route, which the group's
// authentication never reaches.
func sdkAPIAgentBinding(route sdkAPIRoute, resolveName middleware.AgentNameResolver) fiber.Handler {
	name := route.Method + " " + route.FullPath()
	switch {
	case route.Bare && (route.AgentParam != "" || route.AgentBindingException != ""):
		panic("registerSDKAPIRoutes: " + name + " is bare, so no agent principal is set on it; an agent binding there means nothing")
	case route.Bare:
		return nil
	case route.AgentParam != "" && route.AgentBindingException != "":
		panic("registerSDKAPIRoutes: " + name + " declares both AgentParam and AgentBindingException")
	case route.AgentParamMayBeName && route.AgentParam == "":
		panic("registerSDKAPIRoutes: " + name + " sets AgentParamMayBeName without AgentParam")
	case route.AgentBindingException != "":
		return nil
	case route.AgentParam != "":
		if !pathHasParam(route.Path, route.AgentParam) {
			panic("registerSDKAPIRoutes: " + name + " binds :" + route.AgentParam + " but its path has no such parameter")
		}
		var resolve middleware.AgentNameResolver
		if route.AgentParamMayBeName {
			if resolveName == nil {
				panic("registerSDKAPIRoutes: " + name + " admits the caller's name but sdkAPIDeps.AgentNameResolver is nil")
			}
			resolve = resolveName
		}
		return middleware.AgentPathBinding(name, route.AgentParam, resolve)
	case agentPathParam(route.Path) != "":
		panic("registerSDKAPIRoutes: " + name + " has an agent parameter but declares neither AgentParam nor " +
			"AgentBindingException; an agent credential would reach every agent in its organization on this route")
	default:
		return nil
	}
}

// unsupportedSDKAPIMethod is unreachable in practice: the table is a literal in
// this file, so a new method is a compile-time-visible edit away from this
// switch. It exists so that edit fails loudly at boot instead of silently
// leaving a route unregistered — the failure mode this whole file is about.
func unsupportedSDKAPIMethod(route sdkAPIRoute) string {
	return "registerSDKAPIRoutes: unsupported method " + route.Method + " for " + route.FullPath()
}
