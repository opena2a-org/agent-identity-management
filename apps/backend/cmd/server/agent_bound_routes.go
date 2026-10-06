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
// chain. It is the one way a route outside the SDK-API table binds its agent
// parameter to the authenticated agent: an agent principal may act only on its
// own ID, and a user JWT caller passes to the handler unchanged.
//
// agent_binding_routes_test.go walks the source tree and requires every route
// with an agent parameter under such a group to be registered through this
// type, or to be listed there as an admitted exception or a dated hold.
type agentBoundRouter struct {
	group  fiber.Router
	prefix string
}

// bindAgentRoutes wraps group. It must be a group (not the app) so each bound
// route can be named by its full path in the 500 a mis-mount produces.
func bindAgentRoutes(group fiber.Router) agentBoundRouter {
	g, ok := group.(*fiber.Group)
	if !ok {
		panic("bindAgentRoutes: needs a *fiber.Group")
	}
	return agentBoundRouter{group: group, prefix: g.Prefix}
}

// Get registers a GET route whose agent parameter is bound to the caller.
func (r agentBoundRouter) Get(path string, handlers ...fiber.Handler) {
	r.add(http.MethodGet, path, handlers)
}

// Post registers a POST route whose agent parameter is bound to the caller.
func (r agentBoundRouter) Post(path string, handlers ...fiber.Handler) {
	r.add(http.MethodPost, path, handlers)
}

func (r agentBoundRouter) add(method, path string, handlers []fiber.Handler) {
	fullPath := r.prefix + path
	param := agentPathParam(fullPath)
	if param == "" {
		panic("bindAgentRoutes: " + method + " " + fullPath + " has no agent parameter to bind")
	}
	if len(handlers) == 0 {
		panic("bindAgentRoutes: " + method + " " + fullPath + " has no handler")
	}
	chain := make([]any, 0, len(handlers))
	for _, h := range handlers {
		chain = append(chain, h)
	}
	r.group.Add([]string{method}, path, middleware.AgentPathBinding(method+" "+fullPath, param, nil), chain...)
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

// agentPathParam returns the name of the agent parameter in path: the
// parameter that directly follows an "agents" segment, or one named agentId
// or agent_id. It returns "" for a path with none. A parameter elsewhere
// names some other resource (a verification, an MCP server, a task) and is
// bound to the caller, if at all, by its own handler.
func agentPathParam(path string) string {
	segments := strings.Split(path, "/")
	for i, segment := range segments {
		if !strings.HasPrefix(segment, ":") {
			continue
		}
		name := strings.TrimSuffix(strings.TrimPrefix(segment, ":"), "?")
		if name == "agentId" || name == "agent_id" || (i > 0 && segments[i-1] == "agents") {
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
