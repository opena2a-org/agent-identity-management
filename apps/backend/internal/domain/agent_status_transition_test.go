package domain

import (
	"errors"
	"testing"
)

// The table is written out with string literals for the statuses, so it states the
// contract independently of the constants it checks (see agent_status_auth_test.go).
func TestAgentStatusTransition(t *testing.T) {
	const refused = "refused"
	const unchanged = "unchanged"
	for _, tc := range []struct {
		act  AgentStatusAct
		from string
		want string // the status written, unchanged, or refused
	}{
		{AgentStatusActVerify, "pending", "verified"},
		{AgentStatusActVerify, "verified", unchanged},
		{AgentStatusActVerify, "suspended", refused},
		{AgentStatusActVerify, "revoked", refused},

		{AgentStatusActSuspend, "pending", "suspended"},
		{AgentStatusActSuspend, "verified", "suspended"},
		{AgentStatusActSuspend, "suspended", unchanged},
		{AgentStatusActSuspend, "revoked", refused},

		{AgentStatusActReactivate, "suspended", "verified"},
		{AgentStatusActReactivate, "verified", unchanged},
		{AgentStatusActReactivate, "pending", refused},
		{AgentStatusActReactivate, "revoked", refused},

		// agents.status is a VARCHAR with no CHECK constraint: a value outside the four
		// constants is refused by every act, never moved to verified or suspended.
		{AgentStatusActVerify, "deactivated", refused},
		{AgentStatusActSuspend, "deactivated", refused},
		{AgentStatusActReactivate, "", refused},
		{AgentStatusActReactivate, "Revoked", refused},
		{AgentStatusAct("restore"), "revoked", refused},
	} {
		t.Run(string(tc.act)+"/"+tc.from, func(t *testing.T) {
			to, write, err := AgentStatusTransition(tc.act, AgentStatus(tc.from))
			switch tc.want {
			case refused:
				var refusal *AgentStatusTransitionError
				if !errors.As(err, &refusal) {
					t.Fatalf("got (%q, %v, %v), want a refusal", to, write, err)
				}
				if write || to != AgentStatus(tc.from) {
					t.Errorf("a refusal writes nothing, got (%q, %v)", to, write)
				}
			case unchanged:
				if err != nil || write || to != AgentStatus(tc.from) {
					t.Errorf("got (%q, %v, %v), want (%q, false, nil)", to, write, err, tc.from)
				}
			default:
				if err != nil || !write || to != AgentStatus(tc.want) {
					t.Errorf("got (%q, %v, %v), want (%q, true, nil)", to, write, err, tc.want)
				}
			}
		})
	}
}

// No act returns a revoked agent to verified or suspended: either would let the key it
// held when it was revoked authenticate again, the first at once and the second after a
// reactivate.
func TestNoAgentStatusActLeavesRevoked(t *testing.T) {
	for act := range agentStatusTransitions {
		if to, write, err := AgentStatusTransition(act, AgentStatusRevoked); err == nil || write {
			t.Errorf("%s on a revoked agent = (%q, %v, %v), want a refusal", act, to, write, err)
		}
	}
}

func TestAgentStatusTransitionRefusalNamesTheActThatApplies(t *testing.T) {
	for _, tc := range []struct {
		act  AgentStatusAct
		from AgentStatus
		want string
	}{
		{AgentStatusActReactivate, AgentStatusRevoked, "This agent is revoked, and a revoked agent cannot be reactivated: register a new agent to replace it"},
		{AgentStatusActVerify, AgentStatusRevoked, "This agent is revoked, and a revoked agent cannot be verified: register a new agent to replace it"},
		{AgentStatusActSuspend, AgentStatusRevoked, "This agent is revoked, and a revoked agent cannot be suspended"},
		{AgentStatusActVerify, AgentStatusSuspended, "This agent is suspended: reactivate it instead"},
		{AgentStatusActReactivate, AgentStatusPending, "This agent is pending: verify it instead"},
		{AgentStatusActSuspend, AgentStatus("deactivated"), `An agent with status "deactivated" cannot be changed by suspend`},
	} {
		_, _, err := AgentStatusTransition(tc.act, tc.from)
		if err == nil || err.Error() != tc.want {
			t.Errorf("%s from %s: got %v, want %q", tc.act, tc.from, err, tc.want)
		}
	}
}
