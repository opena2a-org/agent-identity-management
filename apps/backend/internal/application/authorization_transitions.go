package application

import (
	"context"
	"database/sql"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record/transition"
)

// SetTransitionRecorder makes GrantCapability and RevokeCapability change the
// agent's capabilities through r, so each change commits together with its
// authorization_transition record. When unset, they change capabilities as
// before and write no record.
func (s *CapabilityService) SetTransitionRecorder(r *transition.Recorder) {
	s.transitions = r
}

// SetTransitionRecorder makes CreateAgent, SuspendAgent, ReactivateAgent,
// RevokeAgent, VerifyAgent, RotateCredentials, UpdateAgentPublicKey,
// UpdateAgentPQCKey, RotateAgentPQCKey, EnforceKeyExpiry and the capability
// revocations of UpdateAgent change the agent through r, so each change
// commits together with its authorization_transition record. When unset,
// they change the agent as before and write no record.
func (s *AgentService) SetTransitionRecorder(r *transition.Recorder) {
	s.transitions = r
}

// SetTransitionRecorder makes a pending capability request, its approval or
// rejection, and a monitoring-mode automatic approval commit together with
// its authorization_transition record. A decision's record names the pending
// request's record as its parent. When unset, requests are filed and decided
// as before and no record is written.
func (s *CapabilityRequestService) SetTransitionRecorder(r *transition.Recorder) {
	s.transitions = r
}

// SetTransitionRecorder makes the suspension of an agent whose trust score
// fell below the critical threshold commit together with its
// authorization_transition record. When unset, the agent is suspended as
// before and no record is written.
func (s *SecurityPolicyService) SetTransitionRecorder(r *transition.Recorder) {
	s.transitions = r
}

// recordAgentChange makes one change of an agent's authorization state
// through r. The actor is the one the context names, else fallback. Each call
// mints its own trace id.
func recordAgentChange(
	ctx context.Context,
	r *transition.Recorder,
	agent *domain.Agent,
	trigger transition.Trigger,
	fallback transition.Actor,
	apply func(ctx context.Context, tx *sql.Tx) error,
) error {
	return recordChange(ctx, r, transition.Change{
		OrganizationID: agent.OrganizationID,
		AgentID:        agent.ID,
		Trigger:        trigger,
		Apply:          apply,
	}, fallback)
}

// recordChange makes c through r. Its actor is the one the context names,
// else fallback. A change that does not join its parent's trace mints a trace
// id of its own.
func recordChange(ctx context.Context, r *transition.Recorder, c transition.Change, fallback transition.Actor) error {
	if c.TraceID == "" {
		trace, err := transition.NewTraceID()
		if err != nil {
			return err
		}
		c.TraceID = trace
	}
	actor, ok := transition.ActorFrom(ctx)
	if !ok {
		actor = fallback
	}
	c.Actor = actor
	_, err := r.Record(ctx, c)
	return err
}
