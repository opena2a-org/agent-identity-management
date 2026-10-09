package main

import (
	"context"
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/interfaces/http/middleware"
)

// agentBoundRouter registers routes on a group that mounts an agent
// authenticator, with middleware.AgentPathBinding in front of each handler
// chain (middleware.AgentPathObservation for a held route). It is the one way a
// route outside the SDK-API table binds its agent parameter to the
// authenticated agent: an agent principal may act only on its own ID, and a
// user JWT caller passes to the handler unchanged.
//
// agent_binding_routes_test.go walks the source tree and requires every route
// with an agent parameter under such a group to be registered through this
// type (bound, or held and observed), or to be listed there as an admitted
// exception.
type agentBoundRouter struct {
	helper string
	group  fiber.Router
	prefix string
	hold   bool
}

// bindAgentRoutes wraps group. It must be a group (not the app) so each bound
// route can be named by its full path in the 500 a mis-mount produces.
func bindAgentRoutes(group fiber.Router) agentBoundRouter {
	return newAgentRouter("bindAgentRoutes", group, false)
}

// holdAgentRoutes wraps group for routes whose binding is held until its effect
// on existing callers is measured. Each route gets
// middleware.AgentPathObservation in place of the binding: it refuses nothing,
// and records on the request's api_calls row whether an agent caller named its
// own ID. Every route registered here must be listed in agentBindingHeld, and
// every held route must be registered here, so a hold always has the
// measurement that ends it.
func holdAgentRoutes(group fiber.Router) agentBoundRouter {
	return newAgentRouter("holdAgentRoutes", group, true)
}

func newAgentRouter(helper string, group fiber.Router, hold bool) agentBoundRouter {
	g, ok := group.(*fiber.Group)
	if !ok {
		panic(helper + ": needs a *fiber.Group")
	}
	return agentBoundRouter{helper: helper, group: group, prefix: g.Prefix, hold: hold}
}

// Get registers a GET route with the router's binding (or, held, its observation).
func (r agentBoundRouter) Get(path string, handlers ...fiber.Handler) {
	r.add(http.MethodGet, path, handlers)
}

// Post registers a POST route with the router's binding (or, held, its observation).
func (r agentBoundRouter) Post(path string, handlers ...fiber.Handler) {
	r.add(http.MethodPost, path, handlers)
}

// Put registers a PUT route with the router's binding (or, held, its observation).
func (r agentBoundRouter) Put(path string, handlers ...fiber.Handler) {
	r.add(http.MethodPut, path, handlers)
}

// Delete registers a DELETE route with the router's binding (or, held, its observation).
func (r agentBoundRouter) Delete(path string, handlers ...fiber.Handler) {
	r.add(http.MethodDelete, path, handlers)
}

func (r agentBoundRouter) add(method, path string, handlers []fiber.Handler) {
	fullPath := r.prefix + path
	param := agentPathParam(fullPath)
	if param == "" {
		panic(r.helper + ": " + method + " " + fullPath + " has no agent parameter to bind")
	}
	if len(handlers) == 0 {
		panic(r.helper + ": " + method + " " + fullPath + " has no handler")
	}
	chain := make([]any, 0, len(handlers))
	for _, h := range handlers {
		chain = append(chain, h)
	}
	route := method + " " + fullPath
	gate := middleware.AgentPathBinding(route, param, nil)
	if r.hold {
		gate = middleware.AgentPathObservation(route, param)
	}
	r.group.Add([]string{method}, path, gate, chain...)
}

// agentGetter is the agent-service surface agentNameResolver needs.
type agentGetter interface {
	GetAgent(ctx context.Context, id uuid.UUID) (*domain.Agent, error)
}

// agentNameResolver backs the routes that admit the caller's own agent name in
// place of its ID. middleware.AgentPathBinding calls it with the caller's ID
// only, so the one row it reads is the caller's.
func agentNameResolver(agents agentGetter) middleware.AgentNameResolver {
	return func(ctx context.Context, id uuid.UUID) (string, error) {
		agent, err := agents.GetAgent(ctx, id)
		if err != nil {
			return "", err
		}
		if agent == nil {
			return "", nil
		}
		return agent.Name, nil
	}
}

// agentIDSegments are the path segments whose next segment, when it is a
// parameter, is an agent ID: "agents", and the A2A aliases that address an
// agent's card (/a2a/cards/:id) and trust score (/a2a/trust/:id) by that ID.
var agentIDSegments = map[string]bool{"agents": true, "cards": true, "trust": true}

// agentPathParam returns the name of the agent parameter in path: the
// parameter that directly follows a segment in agentIDSegments, or one named
// agentId or agent_id. It returns "" for a path with none. A parameter
// elsewhere names some other resource (a verification, an MCP server, a task)
// and is bound to the caller, if at all, by its own handler.
func agentPathParam(path string) string {
	segments := strings.Split(path, "/")
	for i, segment := range segments {
		if !strings.HasPrefix(segment, ":") {
			continue
		}
		name := strings.TrimSuffix(strings.TrimPrefix(segment, ":"), "?")
		if name == "agentId" || name == "agent_id" || (i > 0 && agentIDSegments[segments[i-1]]) {
			return name
		}
	}
	return ""
}

// pathHasParam reports whether path has the parameter :param as a segment.
func pathHasParam(path, param string) bool {
	for _, segment := range strings.Split(path, "/") {
		if segment == ":"+param {
			return true
		}
	}
	return false
}
