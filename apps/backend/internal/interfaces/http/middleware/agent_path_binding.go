package middleware

import (
	"context"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

// AgentPathMismatchReasonCode is the reasonCode an AgentPathBinding refusal
// carries, so refusals can be counted per route from request records.
const AgentPathMismatchReasonCode = "agent_path_mismatch"

// agentPathMismatchMessage is the whole refusal body text. It is a constant on
// purpose: the response must not depend on what the path parameter names.
const agentPathMismatchMessage = "An agent may only act on its own agent ID on this route"

// AgentNameResolver returns the name of the agent with the given ID.
//
// AgentPathBinding calls it with the authenticated agent's own ID and nothing
// else: the binding compares the path parameter against the caller and never
// reads the agent the path names, so it cannot be used to learn whether a name
// exists.
type AgentNameResolver func(ctx context.Context, agentID uuid.UUID) (string, error)

// AgentPathBinding returns the route-level handler that binds a path parameter
// to the authenticated agent.
//
// When an agent authenticator has set c.Locals("agent_id") — a PQC or Ed25519
// signature, an API key, an ATC or a service principal — the parameter named
// param must parse as a UUID equal to that agent, or the request is refused
// 403 with reasonCode AgentPathMismatchReasonCode. The refusal comes before
// any lookup and its body does not depend on the parameter, so a sibling
// agent's existing ID and an ID that names nothing get identical responses.
// With no agent principal (a user JWT) the request passes unchanged and the
// handler's own organization and role checks apply as before.
//
// resolveName, when non-nil, also admits the caller's own name in place of its
// ID. It is called with the caller's ID only.
//
// route names the route in the 500 a mis-mount produces.
//
// MUST be registered on the route — group.Get(path, binding, handler) — never
// with Use(). Fiber fills Params only after route matching, so under Use() the
// parameter reads empty; that is answered with a 500 naming the route for every
// caller, so a mis-mount shows as a server fault and never as a pass.
func AgentPathBinding(route, param string, resolveName AgentNameResolver) fiber.Handler {
	if route == "" || param == "" {
		panic("middleware.AgentPathBinding: route and param are both required")
	}

	return func(c fiber.Ctx) error {
		value := c.Params(param)
		if value == "" {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "route " + route + " binds :" + param + " to the authenticated agent but the parameter is empty; " +
					"the binding must be registered on the route, not with Use()",
			})
		}

		principalLocal := c.Locals("agent_id")
		if principalLocal == nil {
			return c.Next()
		}
		principal, ok := principalLocal.(uuid.UUID)
		if !ok || principal == uuid.Nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "route " + route + ": the authenticated agent could not be read",
			})
		}

		if id, err := uuid.Parse(value); err == nil {
			if id == principal {
				return c.Next()
			}
			return refuseAgentPathMismatch(c)
		}

		if resolveName != nil {
			name, err := resolveName(c.Context(), principal)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": "route " + route + ": the authenticated agent could not be read",
				})
			}
			if name != "" && name == value {
				return c.Next()
			}
		}

		return refuseAgentPathMismatch(c)
	}
}

func refuseAgentPathMismatch(c fiber.Ctx) error {
	return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
		"error":      agentPathMismatchMessage,
		"reasonCode": AgentPathMismatchReasonCode,
	})
}
