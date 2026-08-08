package domain

import "testing"

// AgentStatusPermitsAuth is the single rule deciding whether an agent in a given status may
// authenticate. It lives in domain because two packages enforce it at two different moments
// — the auth middlewares at USE, the OAuth token endpoint at ISSUANCE — and a second copy
// would be a second thing to keep in step.
//
// The cases are written as string literals rather than derived from the exported constants,
// so this table states the contract independently of the values it checks. If someone
// renames AgentStatusRevoked to something else, this test still asserts that the string
// "revoked" is denied, which is what is actually stored in the column.
func TestAgentStatusPermitsAuth(t *testing.T) {
	for _, tc := range []struct {
		status string
		permit bool
		why    string
	}{
		{"verified", true, "the ordinary authenticated agent"},
		{"pending", true, "RegisterAgent sets this at registration, so it is the default state of an honestly registered agent, not an earned one; denying it would break enrollment for every new agent while blocking nothing an attacker holds"},
		{"revoked", false, "RevokeAgent writes this and it is the whole point of the rule"},
		{"suspended", false, "SuspendAgent writes this"},

		// agents.status is a plain VARCHAR(50) with no CHECK constraint (migration 001), so
		// a value outside the four constants is storable — by a future migration, a hand-run
		// UPDATE, or a bug. An allow-list denies every one of them. A deny-list on
		// {revoked, suspended} would authenticate all of them, which is the failure this
		// shape exists to prevent.
		{"deactivated", false, "unrecognised value must fail closed"},
		{"", false, "empty is unrecognised, not a default-allow"},
		{"VERIFIED", false, "status comparison is case-sensitive; an almost-right value is still unrecognised"},
		{" verified", false, "whitespace makes it a different value, and nothing trims it before storage"},
		{"admin", false, "an unrelated string must not fall through"},
	} {
		t.Run(tc.status, func(t *testing.T) {
			if got := AgentStatusPermitsAuth(AgentStatus(tc.status)); got != tc.permit {
				t.Errorf("AgentStatusPermitsAuth(%q) = %v, want %v — %s", tc.status, got, tc.permit, tc.why)
			}
		})
	}
}

// The denial message names the status, so an operator whose agent stopped working can see
// why without opening a support ticket. The request is only reached by a caller already
// holding valid credentials for that specific agent, so this discloses nothing to anyone
// who did not already know it.
func TestAgentStatusDeniedMessageNamesTheStatus(t *testing.T) {
	for _, status := range []string{"revoked", "suspended", "deactivated"} {
		msg := AgentStatusDeniedMessage(AgentStatus(status))
		if msg == "" {
			t.Fatalf("AgentStatusDeniedMessage(%q) returned an empty message", status)
		}
		if !contains(msg, status) {
			t.Errorf("AgentStatusDeniedMessage(%q) = %q, which does not name the status — an "+
				"operator cannot tell why the agent stopped working", status, msg)
		}
	}
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
