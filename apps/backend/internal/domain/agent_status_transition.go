package domain

import "fmt"

// AgentStatusAct names a lifecycle act that writes agents.status.
type AgentStatusAct string

const (
	AgentStatusActVerify     AgentStatusAct = "verify"
	AgentStatusActSuspend    AgentStatusAct = "suspend"
	AgentStatusActReactivate AgentStatusAct = "reactivate"
)

// agentStatusTransitions is the one table every lifecycle route reads: the statuses an
// act moves an agent out of, and the status it writes.
//
// SECURITY: no act here moves an agent out of `revoked`. Reactivate and verify both write
// `verified` and leave the agent's keys as they are, so either one accepting a revoked
// agent would let the key it held when it was revoked authenticate again. Suspend refuses
// it too, because a suspended agent is one reactivate accepts.
var agentStatusTransitions = map[AgentStatusAct]struct {
	from []AgentStatus
	to   AgentStatus
}{
	AgentStatusActVerify:     {from: []AgentStatus{AgentStatusPending}, to: AgentStatusVerified},
	AgentStatusActSuspend:    {from: []AgentStatus{AgentStatusPending, AgentStatusVerified}, to: AgentStatusSuspended},
	AgentStatusActReactivate: {from: []AgentStatus{AgentStatusSuspended}, to: AgentStatusVerified},
}

// AgentStatusTransition reports what act does to an agent in status from. It returns the
// status to write and write=true when the act applies; write=false and a nil error when
// the agent is already in the act's target status, so nothing is written; and an
// *AgentStatusTransitionError for any other status, including one outside the four
// constants (agents.status has no CHECK constraint), so an unrecognised value is refused.
func AgentStatusTransition(act AgentStatusAct, from AgentStatus) (to AgentStatus, write bool, err error) {
	t, ok := agentStatusTransitions[act]
	if !ok {
		return from, false, &AgentStatusTransitionError{Act: act, From: from}
	}
	if from == t.to {
		return from, false, nil
	}
	for _, s := range t.from {
		if from == s {
			return t.to, true, nil
		}
	}
	return from, false, &AgentStatusTransitionError{Act: act, From: from}
}

// AgentStatusTransitionError is the refusal of an act the table does not allow from the
// agent's current status. Its text is written for the caller and names the act that
// applies instead, when one exists.
type AgentStatusTransitionError struct {
	Act  AgentStatusAct
	From AgentStatus
}

func (e *AgentStatusTransitionError) Error() string { return e.Message() }

// Message is the line a lifecycle route answers with: the refusal, and the act that
// applies instead when one exists.
func (e *AgentStatusTransitionError) Message() string {
	switch {
	case e.From == AgentStatusRevoked && e.Act == AgentStatusActSuspend:
		return "This agent is revoked, and a revoked agent cannot be suspended"
	case e.From == AgentStatusRevoked && e.Act == AgentStatusActReactivate:
		return "This agent is revoked, and a revoked agent cannot be reactivated: register a new agent to replace it"
	case e.From == AgentStatusRevoked && e.Act == AgentStatusActVerify:
		return "This agent is revoked, and a revoked agent cannot be verified: register a new agent to replace it"
	case e.From == AgentStatusSuspended && e.Act == AgentStatusActVerify:
		return "This agent is suspended: reactivate it instead"
	case e.From == AgentStatusPending && e.Act == AgentStatusActReactivate:
		return "This agent is pending: verify it instead"
	default:
		return fmt.Sprintf("An agent with status %q cannot be changed by %s", string(e.From), string(e.Act))
	}
}
