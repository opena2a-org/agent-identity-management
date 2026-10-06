package handlers

import (
	"context"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record/transition"
)

// agentKeyAuthMethods are the auth_method values set when an agent signed the
// request with its own key.
var agentKeyAuthMethods = map[string]bool{"ed25519": true, "mldsa": true, "hybrid": true}

// withTransitionActor returns the request's context naming the actor of the
// authorization changes the request makes: the agent, when it signed the
// request with its own key, else the signed-in user. A request that names
// neither gets its context unchanged, and the service names the actor.
func withTransitionActor(c fiber.Ctx) context.Context {
	ctx := c.Context()
	method, _ := c.Locals("auth_method").(string)
	if agentID, ok := c.Locals("agent_id").(uuid.UUID); ok && agentID != uuid.Nil && agentKeyAuthMethods[method] {
		return transition.WithActor(ctx, transition.Agent(agentID))
	}
	if userID, ok := c.Locals("user_id").(uuid.UUID); ok && userID != uuid.Nil {
		return transition.WithActor(ctx, transition.User(userID))
	}
	return ctx
}
