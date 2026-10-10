package application

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record/transition"
)

// SetTransitionRecorder makes GrantCapability and RevokeCapability change the
// agent's capabilities through r, and VerifyAction suspend an agent whose
// violations cross the compromise threshold through r, so each change commits
// together with its authorization_transition record. The trust score
// recalculated after a grant or a revocation is then stored through its
// history row alone (saveRecalculatedAgent). When unset, they change the
// agent as before and write no record.
func (s *CapabilityService) SetTransitionRecorder(r *transition.Recorder) {
	s.transitions = r
}

// SetTransitionRecorder makes CreateAgent, SuspendAgent, ReactivateAgent,
// RevokeAgent, DeleteAgent, VerifyAgent, RotateCredentials, UpdateAgentPublicKey,
// UpdateAgentPQCKey, RotateAgentPQCKey, EnforceKeyExpiry, AddMCPServers,
// RemoveMCPServers, and the talks_to replacement and capability revocations
// of UpdateAgent change the agent through r, so each change commits together
// with its authorization_transition record. UpdateAgent then stores the
// agent's descriptive columns through a statement of their own, and the trust
// score recalculated after a change, or by RecalculateTrustScore, is stored
// through its history row alone (saveRecalculatedAgent). When unset, they
// change the agent as before and write no record.
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

// SetTransitionRecorder makes the key SignA2ARequest generates for an agent
// that holds no server key commit together with its authorization_transition
// record. When unset, the key is stored as before and no record is written.
func (s *A2AService) SetTransitionRecorder(r *transition.Recorder) {
	s.transitions = r
}

// saveRecalculatedAgent saves agent's whole row after its trust score was
// recalculated, as the services did before a transition recorder existed.
// With a recorder set it writes nothing: the trust_scores row the caller
// inserts next sets agents.trust_score through the trigger of migration 093,
// which writes no other column. agent was read before the change that led to
// the recalculation, so saving its whole row would put back a status, a key
// or a talks_to list that another request changed since, and no record would
// show it.
func saveRecalculatedAgent(agents domain.AgentRepository, recorded bool, agent *domain.Agent) error {
	if recorded {
		return nil
	}
	return agents.Update(agent)
}

// errAgentNotActive is returned by the statement of a compromise suspension
// for an agent that is already suspended or revoked.
var errAgentNotActive = errors.New("the agent is already suspended or revoked")

// markAsCompromised suspends an agent whose capability violations crossed the
// compromise threshold. With a transition recorder set, an agent that is
// neither suspended nor revoked is suspended through a statement that writes
// only its status, together with a compromise_suspension record whose actor
// is the system; an agent already suspended or revoked is left as it is, and
// no record is written.
func (s *CapabilityService) markAsCompromised(ctx context.Context, agent *domain.Agent) error {
	if s.transitions == nil {
		return s.agentRepo.MarkAsCompromised(agent.ID)
	}
	err := recordAgentChange(transition.WithActor(ctx, transition.System()), s.transitions, agent,
		transition.TriggerCompromiseSuspension, transition.System(),
		func(ctx context.Context, tx *sql.Tx) error {
			done, err := repository.SuspendActiveAgentTx(ctx, tx, agent.ID)
			if err == nil && !done {
				err = errAgentNotActive
			}
			return err
		})
	if errors.Is(err, errAgentNotActive) {
		return nil
	}
	return err
}

// errServerKeyHeld is returned by the statement that stores a generated key
// for an agent that came to hold a server key after it was read.
var errServerKeyHeld = errors.New("the agent already holds a server key")

// storeServerKey stores the Ed25519 key the service generated for an agent
// that signs through it, in one transaction with a key_updated record. The
// key replaces the agent's public key, and no previous key is kept. When
// another request stored a server key for the agent first, nothing is written
// and the agent is read again, so the caller signs with the stored key. It
// returns that agent, or nil when it stored the pair, which the caller then
// sets on the agent it holds.
func (s *A2AService) storeServerKey(ctx context.Context, agent *domain.Agent, publicKey, encryptedPrivateKey string) (*domain.Agent, error) {
	err := recordAgentChange(ctx, s.transitions, agent, transition.TriggerKeyUpdated, transition.System(),
		func(ctx context.Context, tx *sql.Tx) error {
			done, err := repository.SetAgentServerKeyTx(ctx, tx, agent.ID, publicKey, encryptedPrivateKey)
			if err == nil && !done {
				err = errServerKeyHeld
			}
			return err
		})
	switch {
	case errors.Is(err, errServerKeyHeld):
		stored, err := s.agentRepo.GetByID(agent.ID)
		if err != nil || stored == nil || stored.EncryptedPrivateKey == nil || *stored.EncryptedPrivateKey == "" {
			return nil, fmt.Errorf("failed to read the agent's server key: %w", errors.Join(errServerKeyHeld, err))
		}
		return stored, nil
	case err != nil:
		return nil, fmt.Errorf("failed to update agent keys: %w", err)
	}
	// The pair was stored; the caller sets it on the agent it holds.
	return nil, nil
}

// recordTalksToChange changes agent's talks_to list through r. edit returns
// the list to store given a list the row holds, without changing its
// argument. It runs on agent.TalksTo to class the change (TalksToClass), and
// again, inside the change's transaction, on the list read under the agent's
// row lock, which is the list it stores. A change whose lists are equal as
// sets stores nothing and writes no record. It returns the list the row held
// and the list it holds afterwards; the two are the same when nothing
// changed. Neither is nil, so an empty list is answered as [], not null.
func recordTalksToChange(
	ctx context.Context,
	r *transition.Recorder,
	agent *domain.Agent,
	trigger transition.Trigger,
	fallback transition.Actor,
	edit func(current []string) []string,
) (before, after []string, err error) {
	read := append(make([]string, 0, len(agent.TalksTo)), agent.TalksTo...)
	class, changed := transition.TalksToClass(read, edit(append([]string(nil), read...)))
	if !changed {
		return read, read, nil
	}
	held, stored := make([]string, 0), make([]string, 0)
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
			held = append(held[:0], current...)
			stored = append(stored[:0], edit(append([]string(nil), current...))...)
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
