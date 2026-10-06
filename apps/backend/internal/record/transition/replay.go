package transition

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record/store"
)

// ErrNoChain is returned by Replay for an organization whose chain has not
// started.
var ErrNoChain = errors.New("transition: the organization's chain has not started")

// ContinuityError reports an agent's transition whose previous_state is not
// the new_state of the agent's transition before it: some path changed the
// agent between the two without a record.
type ContinuityError struct {
	AgentID     uuid.UUID
	Seq         int64
	PreviousSeq int64
}

func (e *ContinuityError) Error() string {
	return fmt.Sprintf("transition: agent %s: the previous_state of the record at seq %d is not the new_state of the record at seq %d",
		e.AgentID, e.Seq, e.PreviousSeq)
}

// UnopenedError reports an agent whose first transition is neither its
// registration nor its opening state, so replay has no state to start from.
type UnopenedError struct {
	AgentID uuid.UUID
	Seq     int64
	Trigger Trigger
}

func (e *UnopenedError) Error() string {
	return fmt.Sprintf("transition: agent %s: its first record, at seq %d, is %s, not its registration or its opening state",
		e.AgentID, e.Seq, e.Trigger)
}

// MismatchError reports an agent whose state rebuilt from its records is not
// its state in the state tables.
type MismatchError struct {
	AgentID  uuid.UUID
	Replayed State
	Tables   State
	// Deleted reports that the agent's records end in its deletion while the
	// tables still hold a row with its id. Replayed is then zero.
	Deleted bool
}

func (e *MismatchError) Error() string {
	if e.Deleted {
		return fmt.Sprintf("transition: agent %s: the records end in the agent's deletion, and the tables still hold it", e.AgentID)
	}
	return fmt.Sprintf("transition: agent %s: the state rebuilt from records is not the state in the tables", e.AgentID)
}

// Replayed is an organization's agent states rebuilt from its chain.
type Replayed struct {
	// States is each agent's new_state in its newest transition. A deleted
	// agent has no entry.
	States map[uuid.UUID]State
	// Deleted holds each agent whose newest transition is its deletion.
	Deleted map[uuid.UUID]bool
	// Seqs is the chain position of each agent's transitions, in order.
	Seqs map[uuid.UUID][]int64
}

// Replay verifies an organization's chain with the record key and rebuilds
// each agent's state from its authorization_transition records alone. An
// agent's first record opens its history and must be its registration or
// its opening state, or Replay returns an *UnopenedError; each later
// record's previous_state must equal the new_state before it, or Replay
// returns a *ContinuityError. A registration's previous_state is null, an
// opening state's previous_state equals its new_state, and either one that
// is not the agent's first record is a *ContinuityError.
// A deletion's new_state is null, and any record of the agent after its
// deletion is a *ContinuityError. A chain that does not verify, a transition
// whose tenant part has been erased, and a null previous_state or new_state
// on any other trigger are errors.
func Replay(ctx context.Context, q store.Querier, organizationID uuid.UUID, key record.PublicKey) (Replayed, error) {
	status, err := store.ReadChainState(ctx, q, organizationID.String())
	if err != nil {
		return Replayed{}, err
	}
	if status.ChainID == nil {
		return Replayed{}, ErrNoChain
	}
	records, err := store.ReadChain(ctx, q, *status.ChainID)
	if err != nil {
		return Replayed{}, err
	}
	res, err := record.Verify(records, key)
	if err != nil {
		return Replayed{}, err
	}
	if !res.OK() {
		return Replayed{}, fmt.Errorf("transition: the chain does not verify: %v", res.Failure)
	}

	out := Replayed{States: map[uuid.UUID]State{}, Deleted: map[uuid.UUID]bool{}, Seqs: map[uuid.UUID][]int64{}}
	for _, rec := range records {
		head, err := retainedHead(rec)
		if err != nil {
			return Replayed{}, err
		}
		if head.Type != RecordType || head.Opena2a.StateSpace != StateSpaceAgent {
			continue
		}
		seq := head.Opena2a.Chain.Seq
		if len(rec.TenantPart) == 0 {
			return Replayed{}, fmt.Errorf("transition: the tenant part of the record at seq %d is erased", seq)
		}
		var tenant struct {
			PreviousState *stateMember `json:"previous_state"`
			NewState      *stateMember `json:"new_state"`
			Opena2a       struct {
				OrganizationID string `json:"organization_id"`
				SubjectAgentID string `json:"subject_agent_id"`
			} `json:"opena2a"`
		}
		if err := json.Unmarshal(rec.TenantPart, &tenant); err != nil {
			return Replayed{}, fmt.Errorf("transition: the tenant part of the record at seq %d: %w", seq, err)
		}
		if tenant.Opena2a.OrganizationID != organizationID.String() {
			return Replayed{}, fmt.Errorf("transition: the record at seq %d names another organization", seq)
		}
		agentID, err := uuid.Parse(tenant.Opena2a.SubjectAgentID)
		trigger := Trigger(head.Trigger.Type)
		opens, closes := trigger.opens(), trigger.closes()
		if err != nil || (tenant.NewState == nil) != closes || (tenant.PreviousState == nil) != opens {
			return Replayed{}, fmt.Errorf("transition: the record at seq %d has no agent, or a previous_state or new_state its trigger does not take", seq)
		}
		if trigger == TriggerOpeningState && !Equal(tenant.PreviousState.state(), tenant.NewState.state()) {
			return Replayed{}, fmt.Errorf("transition: the opening state at seq %d changes the agent's state", seq)
		}
		seqs := out.Seqs[agentID]
		if len(seqs) == 0 && !trigger.first() {
			return Replayed{}, &UnopenedError{AgentID: agentID, Seq: seq, Trigger: trigger}
		}
		if len(seqs) > 0 &&
			(trigger.first() || out.Deleted[agentID] || !Equal(out.States[agentID], tenant.PreviousState.state())) {
			return Replayed{}, &ContinuityError{AgentID: agentID, Seq: seq, PreviousSeq: seqs[len(seqs)-1]}
		}
		if closes {
			delete(out.States, agentID)
			out.Deleted[agentID] = true
		} else {
			out.States[agentID] = tenant.NewState.state()
		}
		out.Seqs[agentID] = append(out.Seqs[agentID], seq)
	}
	return out, nil
}

// CheckReplay replays an organization's chain and compares each agent's
// rebuilt state with its state in the tables. It returns a *MismatchError for
// the first agent whose states differ, or whose records end in its deletion
// while the tables still hold it.
func CheckReplay(ctx context.Context, q store.Querier, organizationID uuid.UUID, key record.PublicKey) (Replayed, error) {
	replayed, err := Replay(ctx, q, organizationID, key)
	if err != nil {
		return Replayed{}, err
	}
	for agentID, rebuilt := range replayed.States {
		tables, err := CurrentState(ctx, q, organizationID, agentID)
		if err != nil {
			return Replayed{}, err
		}
		if !Equal(rebuilt, tables) {
			return Replayed{}, &MismatchError{AgentID: agentID, Replayed: rebuilt, Tables: tables}
		}
	}
	for agentID := range replayed.Deleted {
		tables, err := CurrentState(ctx, q, organizationID, agentID)
		switch {
		case errors.Is(err, ErrAgentNotFound):
		case err != nil:
			return Replayed{}, err
		default:
			return Replayed{}, &MismatchError{AgentID: agentID, Tables: tables, Deleted: true}
		}
	}
	return replayed, nil
}

type head struct {
	Type    string `json:"type"`
	Trigger struct {
		Type string `json:"type"`
	} `json:"trigger"`
	Opena2a struct {
		StateSpace string `json:"state_space"`
		Chain      struct {
			Seq int64 `json:"seq"`
		} `json:"chain"`
	} `json:"opena2a"`
}

// retainedHead decodes the members of a verified record's retained part that
// replay reads.
func retainedHead(rec record.Record) (head, error) {
	payload, err := base64.StdEncoding.DecodeString(rec.Envelope.Payload)
	if err != nil {
		return head{}, fmt.Errorf("transition: record payload: %w", err)
	}
	var h head
	if err := json.Unmarshal(payload, &h); err != nil {
		return head{}, fmt.Errorf("transition: record payload: %w", err)
	}
	return h, nil
}
