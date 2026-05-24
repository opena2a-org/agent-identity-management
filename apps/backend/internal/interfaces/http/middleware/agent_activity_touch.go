package middleware

import (
	"context"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

// AgentActivityToucher is the narrow surface this middleware needs
// from the agent service. Defined here (not imported from
// application) so the middleware can be unit-tested without pulling
// the full service graph.
type AgentActivityToucher interface {
	UpdateLastActive(ctx context.Context, agentID uuid.UUID) error
}

// AgentActivityTouchMiddleware bumps agents.last_active on every
// request that authenticated as an agent. Mount AFTER the agent-auth
// middlewares (PQC / API key / ATC) so c.Locals("agent_id") is
// populated by the time this runs.
//
// Discriminator: c.Locals("agent_id") presence. Every agent-auth
// middleware in this repo sets agent_id on success (PQC sets
// ed25519/mldsa/hybrid, api_key.go sets api_key, atc_auth.go sets
// atc). JWT user-auth does NOT set agent_id, so admin requests
// viewing an agent detail panel via JWT are excluded by
// construction.
//
// Behavior:
//   - Runs the handler first via c.Next().
//   - On handler success (no error AND status < 400), fires
//     UpdateLastActive off-goroutine with a background context so a
//     cancelled request context does not abort the touch.
//   - Errors from UpdateLastActive are swallowed: this is a
//     fire-and-forget activity touch, not a transactional write.
//     If it fails, the next request will retry it.
//
// Closes the last_active half of #167. The verified_at half is
// closed by the migration 092 trigger.
func AgentActivityTouchMiddleware(svc AgentActivityToucher) fiber.Handler {
	return func(c fiber.Ctx) error {
		err := c.Next()

		// Only touch on successful agent-auth responses. A 4xx/5xx
		// agent request should not extend liveness.
		if err != nil || c.Response().StatusCode() >= 400 {
			return err
		}

		// JWT-only paths have no agent_id; skip them silently.
		agentIDValue := c.Locals("agent_id")
		if agentIDValue == nil {
			return err
		}
		agentID, ok := agentIDValue.(uuid.UUID)
		if !ok {
			return err
		}

		// Fire-and-forget with background context: do NOT block the
		// response on a counter UPDATE, and do not let the
		// (already-completed) request context cancel the write.
		go func(id uuid.UUID) {
			_ = svc.UpdateLastActive(context.Background(), id)
		}(agentID)

		return err
	}
}
