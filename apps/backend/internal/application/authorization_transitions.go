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

// SetTransitionRecorder makes SuspendAgent, ReactivateAgent, RevokeAgent and
// RotateCredentials change the agent through r, so each change commits
// together with its authorization_transition record. When unset, they change
// the agent as before and write no record.
func (s *AgentService) SetTransitionRecorder(r *transition.Recorder) {
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
	trace, err := transition.NewTraceID()
	if err != nil {
		return err
	}
	actor, ok := transition.ActorFrom(ctx)
	if !ok {
		actor = fallback
	}
	_, err = r.Record(ctx, transition.Change{
		OrganizationID: agent.OrganizationID,
		AgentID:        agent.ID,
		Trigger:        trigger,
		Actor:          actor,
		TraceID:        trace,
		Apply:          apply,
	})
	return err
}
