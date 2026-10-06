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

// MismatchError reports an agent whose state rebuilt from its records is not
// its state in the state tables.
type MismatchError struct {
	AgentID  uuid.UUID
	Replayed State
	Tables   State
}

func (e *MismatchError) Error() string {
	return fmt.Sprintf("transition: agent %s: the state rebuilt from records is not the state in the tables", e.AgentID)
}

// Replayed is an organization's agent states rebuilt from its chain.
type Replayed struct {
	// States is each agent's new_state in its newest transition.
	States map[uuid.UUID]State
	// Seqs is the chain position of each agent's transitions, in order.
	Seqs map[uuid.UUID][]int64
}

// Replay verifies an organization's chain with the record key and rebuilds
// each agent's state from its authorization_transition records alone. An
// agent's first record opens its history; each later record's
// previous_state must equal the new_state before it, or Replay returns a
// *ContinuityError. A chain that does not verify, or a transition whose
// tenant part has been erased, is an error.
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

	out := Replayed{States: map[uuid.UUID]State{}, Seqs: map[uuid.UUID][]int64{}}
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
		if err != nil || tenant.PreviousState == nil || tenant.NewState == nil {
			return Replayed{}, fmt.Errorf("transition: the record at seq %d has no agent or no states", seq)
		}
		if seqs := out.Seqs[agentID]; len(seqs) > 0 &&
			!Equal(out.States[agentID], tenant.PreviousState.state()) {
			return Replayed{}, &ContinuityError{AgentID: agentID, Seq: seq, PreviousSeq: seqs[len(seqs)-1]}
		}
		out.States[agentID] = tenant.NewState.state()
		out.Seqs[agentID] = append(out.Seqs[agentID], seq)
	}
	return out, nil
}

// CheckReplay replays an organization's chain and compares each agent's
// rebuilt state with its state in the tables. It returns a *MismatchError for
// the first agent whose states differ.
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
