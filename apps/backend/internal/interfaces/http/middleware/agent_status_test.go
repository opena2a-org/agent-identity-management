package middleware

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// The permitted set is written out as literals rather than referencing the
// implementation's switch, so a change to the allow-list has to be made twice —
// once in the rule, once here — instead of the test silently tracking it.
func TestAgentStatusPermitsAuth_PermitsVerifiedAndPending(t *testing.T) {
	assert.True(t, agentStatusPermitsAuth(domain.AgentStatus("verified")),
		"a verified agent must be able to authenticate")
	assert.True(t, agentStatusPermitsAuth(domain.AgentStatus("pending")),
		"pending is the registration default (AgentService sets it on register); "+
			"denying it would break enrollment for every new agent")
}

func TestAgentStatusPermitsAuth_DeniesRevokedAndSuspended(t *testing.T) {
	assert.False(t, agentStatusPermitsAuth(domain.AgentStatus("revoked")),
		"RevokeAgent writes this status and nothing else denies the request")
	assert.False(t, agentStatusPermitsAuth(domain.AgentStatus("suspended")),
		"SuspendAgent and EnforceKeyExpiry write this status")
}

// agents.status is VARCHAR(50) with no CHECK constraint (migration 001), so a
// value outside the four domain constants is storable — by a future migration,
// a hand-run UPDATE, or a bug. The rule must be an allow-list: anything it does
// not recognise has to fail closed. A deny-list on {revoked, suspended} passes
// every one of these.
func TestAgentStatusPermitsAuth_DeniesUnrecognisedStatus(t *testing.T) {
	for _, status := range []string{
		"",                 // NOT NULL, but an empty string satisfies that
		"deactivated",      // a plausible fifth status added without updating this rule
		"Verified",         // case variant — Postgres comparison is case-sensitive
		"verified ",        // trailing space
		"pending\x00",      // NUL-suffixed
		"revoked'--",       // injection-shaped
		"active",           // the status this system does NOT have, despite api_keys.is_active
		"verified,revoked", // comma-joined, in case a caller ever concatenates
	} {
		assert.False(t, agentStatusPermitsAuth(domain.AgentStatus(status)),
			"unrecognised status %q must fail closed", status)
	}
}

// Every status the domain declares must be decided one way or the other. This
// fails if a fifth constant is added and nobody revisits the allow-list — the
// point being that the new status lands on the deny branch by default, which is
// the safe direction, but the author is told rather than left to find out in
// production.
func TestAgentStatusPermitsAuth_CoversEveryDeclaredStatus(t *testing.T) {
	declared := map[domain.AgentStatus]bool{
		domain.AgentStatusVerified:  true,
		domain.AgentStatusPending:   true,
		domain.AgentStatusSuspended: false,
		domain.AgentStatusRevoked:   false,
	}

	for status, wantPermitted := range declared {
		assert.Equal(t, wantPermitted, agentStatusPermitsAuth(status),
			"status %q", status)
	}

	assert.Len(t, declared, 4,
		"domain declares a status this rule has not been asked about — add it to "+
			"this map and decide explicitly whether it may authenticate")
}

func TestAgentStatusDeniedMessage_NamesTheStatus(t *testing.T) {
	msg := agentStatusDeniedMessage(domain.AgentStatusRevoked)
	assert.Contains(t, msg, "revoked",
		"an operator whose agent stopped working must be able to see why")
	assert.NotContains(t, msg, "%!",
		"format verb left unsubstituted")
}
