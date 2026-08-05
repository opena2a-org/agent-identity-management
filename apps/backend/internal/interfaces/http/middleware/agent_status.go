package middleware

import (
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// agentStatusPermitsAuth reports whether an agent in this status may authenticate.
//
// SECURITY: allow-list, for the same reason as MemberOrAPIKeyMiddleware's role check —
// an unrecognised status must fail closed, not fall through. `agents.status` is a plain
// VARCHAR(50) with no CHECK constraint (migration 001), so a value outside the four
// domain constants is storable. A deny-list on {revoked, suspended} would authenticate
// every such value.
//
// `pending` PASSES deliberately. AgentService.RegisterAgent sets it at registration
// ("agents start as pending, verification is earned"), so it is the default state of an
// honestly registered agent, not an earned one. Denying it would break enrollment for
// every new agent while blocking nothing an attacker holds.
//
// Denial states are the ones a human or EnforceKeyExpiry assigns on purpose:
// `revoked` (AgentService.RevokeAgent) and `suspended` (SuspendAgent, EnforceKeyExpiry).
// Before this check existed, both wrote a status that no signature or API-key path read,
// so revoking an agent did not stop it authenticating with key material it already held.
func agentStatusPermitsAuth(status domain.AgentStatus) bool {
	switch status {
	case domain.AgentStatusVerified, domain.AgentStatusPending:
		return true
	default:
		return false
	}
}

// agentStatusDeniedMessage is the 401 body for a status-denied request. It names the
// status so an operator whose agent stopped working can see why without opening a
// support ticket; atc_auth.go already discloses revocation the same way ("ATC has been
// revoked"), and the request is only reached by a caller holding valid key material for
// that specific agent.
func agentStatusDeniedMessage(status domain.AgentStatus) string {
	return "Agent is not permitted to authenticate (status: " + string(status) + ")"
}
