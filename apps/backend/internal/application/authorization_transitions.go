package application

import (
	"context"
	"database/sql"
	"errors"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
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
// UpdateAgentPQCKey, RotateAgentPQCKey, EnforceKeyExpiry, AddMCPServers,
// RemoveMCPServers, and the talks_to replacement and capability revocations
// of UpdateAgent change the agent through r, so each change commits together
// with its authorization_transition record. UpdateAgent then stores the
// agent's descriptive columns through a statement of their own. When unset,
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

// SetTransitionRecorder makes the talks_to entry added for an agent that
// registers or attests an MCP server commit together with its
// authorization_transition record. When unset, the entry is added as before
// and no record is written.
func (s *MCPService) SetTransitionRecorder(r *transition.Recorder) {
	s.transitions = r
}

// SetTransitionRecorder makes the talks_to entries a detection report adds
// commit in one statement, together with one authorization_transition record
// whose actor is the reporting agent. When unset, each entry is added as
// before and no record is written.
func (s *DetectionService) SetTransitionRecorder(r *transition.Recorder) {
	s.transitions = r
}

// recordTalksToChange changes agent's talks_to list through r. edit returns
// the list to store given a list the row holds, without changing its
// argument. It runs on agent.TalksTo to class the change (TalksToClass), and
// again, inside the change's transaction, on the list read under the agent's
// row lock, which is the list it stores. A change whose lists are equal as
// sets stores nothing and writes no record. It returns the list the row held
// and the list it holds afterwards; the two are the same when nothing
// changed.
func recordTalksToChange(
	ctx context.Context,
	r *transition.Recorder,
	agent *domain.Agent,
	trigger transition.Trigger,
	fallback transition.Actor,
	edit func(current []string) []string,
) (before, after []string, err error) {
	read := append([]string(nil), agent.TalksTo...)
	class, changed := transition.TalksToClass(read, edit(append([]string(nil), read...)))
	if !changed {
		return read, read, nil
	}
	var held, stored []string
	err = recordChange(ctx, r, transition.Change{
		OrganizationID: agent.OrganizationID,
		AgentID:        agent.ID,
		Trigger:        trigger,
		Class:          class,
		Apply: func(ctx context.Context, tx *sql.Tx) error {
			current, err := repository.AgentTalksToTx(ctx, tx, agent.ID)
			if err != nil {
				return err
			}
			held = current
			stored = edit(append([]string(nil), current...))
			return repository.SetAgentTalksToTx(ctx, tx, agent.ID, stored)
		},
	}, fallback)
	switch {
	case errors.Is(err, transition.ErrNoChange):
		return held, held, nil
	case err != nil:
		return nil, nil, err
	}
	return held, stored, nil
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
