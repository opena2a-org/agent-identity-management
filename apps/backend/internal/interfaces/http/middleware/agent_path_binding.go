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

// The outcome of comparing a route's agent parameter with the caller, as
// AgentPathBinding and AgentPathObservation record it on the request's
// api_calls row (agent_path_outcome).
const (
	// AgentPathOutcomeSelf: an agent principal named its own ID (or name).
	AgentPathOutcomeSelf = "self"
	// AgentPathOutcomeOther: an agent principal named anything else. A bound
	// route refuses it; a held route lets it through and records it.
	AgentPathOutcomeOther = "other"
	// AgentPathOutcomeNoPrincipal: no agent principal, so a user JWT caller.
	AgentPathOutcomeNoPrincipal = "no_principal"
	// AgentPathOutcomeFault: the parameter read empty (a Use() mount) or the
	// principal could not be read. A bound route answers 500.
	AgentPathOutcomeFault = "fault"
)

// Locals keys under which the binding and the observation leave the route and
// the outcome for AnalyticsTracking.
const (
	agentPathRouteLocal   = "agent_path_route"
	agentPathOutcomeLocal = "agent_path_outcome"
)

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
// route names the route in the 500 a mis-mount produces, and the route and the
// outcome (AgentPathOutcome*) are recorded on the request's api_calls row.
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
		outcome, fault := compareAgentPath(c, route, param, resolveName)
		recordAgentPath(c, route, outcome)
		switch outcome {
		case AgentPathOutcomeFault:
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": fault})
		case AgentPathOutcomeOther:
			return refuseAgentPathMismatch(c)
		default:
			return c.Next()
		}
	}
}

// AgentPathObservation returns the route-level handler for a route whose
// binding is held until its effect is measured. It makes the comparison
// AgentPathBinding makes (with no name resolver), records the outcome on the
// request's api_calls row, and always passes to the handler: it changes no
// response. The count of agent-principal requests recorded with outcome
// AgentPathOutcomeOther is what the binding would have refused, and is what
// decides whether the route is bound or admitted as an exception.
//
// Like AgentPathBinding it MUST be registered on the route, never with Use();
// under Use() every request is recorded as AgentPathOutcomeFault.
func AgentPathObservation(route, param string) fiber.Handler {
	if route == "" || param == "" {
		panic("middleware.AgentPathObservation: route and param are both required")
	}

	return func(c fiber.Ctx) error {
		outcome, _ := compareAgentPath(c, route, param, nil)
		recordAgentPath(c, route, outcome)
		return c.Next()
	}
}

// BindAgentID makes AgentPathBinding's comparison for an agent ID a handler
// reads from somewhere other than its path, such as the request body, and
// records the outcome under route the same way. It returns true when the
// request may go on: the agent principal named its own ID, or there is no
// agent principal (a user JWT caller, whose organization the handler still
// checks). Otherwise it has written the response — for another agent's ID the
// same 403 as AgentPathBinding, before any lookup — and the handler must
// return without acting.
func BindAgentID(c fiber.Ctx, route string, agentID uuid.UUID) bool {
	principal, outcome, fault := agentPrincipal(c, route)
	if outcome == "" {
		outcome = AgentPathOutcomeOther
		if agentID == principal {
			outcome = AgentPathOutcomeSelf
		}
	}
	recordAgentPath(c, route, outcome)
	switch outcome {
	case AgentPathOutcomeFault:
		_ = c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": fault})
		return false
	case AgentPathOutcomeOther:
		_ = refuseAgentPathMismatch(c)
		return false
	default:
		return true
	}
}

// agentPrincipal reads the authenticated agent. outcome is empty when there is
// one; otherwise it is AgentPathOutcomeNoPrincipal, or AgentPathOutcomeFault
// with the 500 text in fault.
func agentPrincipal(c fiber.Ctx, route string) (principal uuid.UUID, outcome, fault string) {
	principalLocal := c.Locals("agent_id")
	if principalLocal == nil {
		return uuid.Nil, AgentPathOutcomeNoPrincipal, ""
	}
	principal, ok := principalLocal.(uuid.UUID)
	if !ok || principal == uuid.Nil {
		return uuid.Nil, AgentPathOutcomeFault, "route " + route + ": the authenticated agent could not be read"
	}
	return principal, "", ""
}

// compareAgentPath compares the route's agent parameter with the agent
// principal. fault carries the 500 text when outcome is AgentPathOutcomeFault.
func compareAgentPath(c fiber.Ctx, route, param string, resolveName AgentNameResolver) (outcome, fault string) {
	value := c.Params(param)
	if value == "" {
		return AgentPathOutcomeFault, "route " + route + " binds :" + param + " to the authenticated agent but the parameter is empty; " +
			"the binding must be registered on the route, not with Use()"
	}

	principal, outcome, fault := agentPrincipal(c, route)
	if outcome != "" {
		return outcome, fault
	}

	if id, err := uuid.Parse(value); err == nil {
		if id == principal {
			return AgentPathOutcomeSelf, ""
		}
		return AgentPathOutcomeOther, ""
	}

	if resolveName != nil {
		name, err := resolveName(c.Context(), principal)
		if err != nil {
			return AgentPathOutcomeFault, "route " + route + ": the authenticated agent could not be read"
		}
		if name != "" && name == value {
			return AgentPathOutcomeSelf, ""
		}
	}

	return AgentPathOutcomeOther, ""
}

// recordAgentPath leaves the route and outcome for AnalyticsTracking. Both are
// strings fixed at registration, never views onto the request's buffers.
func recordAgentPath(c fiber.Ctx, route, outcome string) {
	c.Locals(agentPathRouteLocal, route)
	c.Locals(agentPathOutcomeLocal, outcome)
}

func refuseAgentPathMismatch(c fiber.Ctx) error {
	return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
		"error":      agentPathMismatchMessage,
		"reasonCode": AgentPathMismatchReasonCode,
	})
}
