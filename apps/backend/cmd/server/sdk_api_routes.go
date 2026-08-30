package main

import (
	"net/http"

	"github.com/gofiber/fiber/v3"
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
// [CHIEF-CA] 2026-08-29 ruling: the canonical path is the SDK/spec path,
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
		{Method: http.MethodGet, Path: "/agents/:identifier", Handler: h.GetAgentByIdentifier, Note: "get agent by ID or name (SDK)"},
		{Method: http.MethodPost, Path: "/agents/:id/capabilities", Handler: h.GrantCapability, Note: "SDK capability reporting (legacy)"},
		{Method: http.MethodPost, Path: "/agents/:id/capabilities/register", Handler: h.RegisterCapability, Note: "SDK capability registration (respects enforcement mode)"},
		{Method: http.MethodGet, Path: "/agents/:id/capability-requests", Handler: h.ListAgentCapabilityRequests, Note: "SDK list agent's capability requests"},
		{Method: http.MethodPost, Path: "/agents/:id/capability-requests", Handler: h.CreateCapabilityRequest, Note: "SDK capability request creation"},
		{Method: http.MethodPost, Path: "/agents/:id/mcp-servers", Handler: h.CreateMCPServer, Note: "SDK MCP registration (create new MCP server)"},
		{Method: http.MethodGet, Path: "/agents/:id/mcp-servers", Handler: h.ListMCPServers, Note: "SDK list MCP servers for agent's org"},
		{Method: http.MethodGet, Path: "/agents/:id/mcp-servers/by-name", Handler: h.GetMCPServerByName, Note: "SDK get MCP by name (capability caching)"},
		{Method: http.MethodPost, Path: "/agents/:id/mcp-connections", Handler: h.RecordMCPConnection, Note: "SDK record agent-MCP connection (use_mcp_tool)"},
		{Method: http.MethodPost, Path: "/agents/:id/mcp-usage-report", Handler: h.RecordMCPUsageReport, Note: "SDK MCP supply chain usage analytics"},
		{Method: http.MethodPost, Path: "/agents/:id/detection/report", Handler: h.ReportDetection, Note: "SDK MCP detection and integration reporting"},
		{Method: http.MethodPost, Path: "/agents/:id/heartbeat", Handler: h.Heartbeat, Note: "SDK agent heartbeat (liveness)"},

		// Trust factor 9. Canonical path first, alias second; both reach the
		// same handler, so an agent's posture lands the same row either way.
		{
			Method: http.MethodPost, Path: sdkAPIIsolationAttestationPath,
			Handler: h.SubmitIsolationAttestation,
			Note:    "SDK self-report of runtime isolation posture (trust factor 9); canonical path per the [CHIEF-CA] 2026-08-29 ruling",
		},
		{
			Method: http.MethodPost, Path: sdkAPIIsolationAliasPath,
			Handler: h.SubmitIsolationAttestation, Deprecated: true,
			Note: "deprecated alias for " + sdkAPIIsolationAttestationPath + "; kept because BD6 forbids removing a published path",
		},
	}
}

// registerSDKAPIRoutes mounts the SDK-API route table on app and returns what
// it registered, so a caller (in practice, a test) can assert against the real
// table rather than against a second copy of the paths.
func registerSDKAPIRoutes(app *fiber.App, deps sdkAPIDeps) []sdkAPIRoute {
	table := sdkAPIRouteTable(deps.Handlers)

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
	for _, route := range table {
		if route.Bare {
			continue
		}
		switch route.Method {
		case http.MethodGet:
			group.Get(route.Path, route.Handler)
		case http.MethodPost:
			group.Post(route.Path, route.Handler)
		default:
			panic(unsupportedSDKAPIMethod(route))
		}
	}

	return table
}

// unsupportedSDKAPIMethod is unreachable in practice: the table is a literal in
// this file, so a new method is a compile-time-visible edit away from this
// switch. It exists so that edit fails loudly at boot instead of silently
// leaving a route unregistered — the failure mode this whole file is about.
func unsupportedSDKAPIMethod(route sdkAPIRoute) string {
	return "registerSDKAPIRoutes: unsupported method " + route.Method + " for " + route.FullPath()
}
