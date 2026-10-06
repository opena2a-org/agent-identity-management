package transition

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record/store"
)

// openingNamespace is the namespace of the event ids OpeningEventID derives.
var openingNamespace = uuid.MustParse("6f1d4a0e-3b5c-4e8a-9d27-c41b8e5f7a93")

// OpeningEventID is the event_id of the record that opens an agent's history
// in its organization's chain: its registration_baseline, or its
// opening_state. It is a name-based UUID (version 5) of the agent's id, so
// the chain's unique event ids hold at most one such record per agent, and
// the recorder finds it with one indexed read.
func OpeningEventID(agentID uuid.UUID) string {
	return uuid.NewSHA1(openingNamespace, agentID[:]).String()
}

// opened returns the record that opens an agent's history, read through the
// probe of a write under the append lock, or nil when the chain holds none.
// A record at the agent's opening event id that is not a registration or an
// opening state is an error.
func opened(ctx context.Context, probe *store.Probe, agentID uuid.UUID) (*store.Stored, error) {
	s, ok, err := probe.ByEventID(ctx, OpeningEventID(agentID))
	if err != nil || !ok {
		return nil, err
	}
	var h head
	if err := json.Unmarshal(s.Canonical, &h); err != nil {
		return nil, fmt.Errorf("transition: the record at agent %s's opening event id: %w", agentID, err)
	}
	if s.Type != RecordType || h.Type != RecordType || !Trigger(h.Trigger.Type).first() {
		return nil, fmt.Errorf("transition: the record at agent %s's opening event id does not open its history", agentID)
	}
	return &s, nil
}

// openingDraft is the opening_state record of an agent, before its state is
// known. It joins the trace it is written in and has no parent.
func (r *Recorder) openingDraft(organizationID, agentID uuid.UUID, traceID string) record.Draft {
	return r.draft(Change{
		OrganizationID: organizationID,
		AgentID:        agentID,
		Trigger:        TriggerOpeningState,
		Actor:          System(),
		TraceID:        traceID,
		EventID:        OpeningEventID(agentID),
	})
}

// withState places one state as both states of an opening_state draft.
func withState(d *record.Draft, s State) {
	d.Tenant["previous_state"] = s.member()
	d.Tenant["new_state"] = s.member()
}

// Opened is what a sweep of an organization's agents left.
type Opened struct {
	// TraceID is the trace the sweep's records share.
	TraceID string
	// Written holds each agent whose opening_state the sweep wrote.
	Written []uuid.UUID
	// Existing counts the agents whose history a record already opened.
	Existing int
	// Gone counts the agents listed and deleted before the sweep reached
	// them.
	Gone int
}

const organizationAgentsQuery = `SELECT id FROM agents WHERE organization_id = $1 ORDER BY id`

// errAgentGone ends the write of an agent deleted after the sweep listed it.
var errAgentGone = errors.New("transition: the agent was deleted after it was listed")

// ErrChainNotStarted is wrapped by the error of a sweep whose record writer
// is in the pre-chain state and whose organization's chain has not started:
// there is no chain to open a history in, and nothing was written.
var ErrChainNotStarted = errors.New("transition: the organization's chain has not started")

// OpenStates writes an opening_state record for every agent of an
// organization whose history no record opens: the one-time sweep for a
// deployment that holds agents when records start. Each agent's record is
// written in a transaction of its own, under the agent's row lock, from the
// state the tables hold; an agent whose history is already opened, or which
// was deleted after it was listed, is left as it is. The records of one run
// share one trace id, and a run covers one organization. A sweep changes no
// state, so its writes are observations: when one cannot be written, the
// sweep stops and returns the error with what it had written, and a later
// run writes the rest. A sweep of an organization whose chain has not
// started, by a writer in the pre-chain state, writes nothing and returns an
// error that wraps ErrChainNotStarted.
func (r *Recorder) OpenStates(ctx context.Context, organizationID uuid.UUID) (Opened, error) {
	if organizationID == uuid.Nil {
		return Opened{}, fmt.Errorf("%w: the organization is required", ErrInvalidChange)
	}
	traceID, err := NewTraceID()
	if err != nil {
		return Opened{}, err
	}
	agents, err := r.organizationAgents(ctx, organizationID)
	if err != nil {
		return Opened{}, err
	}
	out := Opened{TraceID: traceID}
	for _, agentID := range agents {
		written, err := r.openState(ctx, organizationID, agentID, traceID)
		switch {
		case errors.Is(err, errAgentGone):
			out.Gone++
		case err != nil:
			return out, fmt.Errorf("transition: opening state of agent %s: %w", agentID, err)
		case written:
			out.Written = append(out.Written, agentID)
		default:
			out.Existing++
		}
	}
	return out, nil
}

func (r *Recorder) organizationAgents(ctx context.Context, organizationID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.db.QueryContext(ctx, organizationAgentsQuery, organizationID)
	if err != nil {
		return nil, fmt.Errorf("transition: list the organization's agents: %w", err)
	}
	defer rows.Close()
	var agents []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("transition: list the organization's agents: %w", err)
		}
		agents = append(agents, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("transition: list the organization's agents: %w", err)
	}
	return agents, nil
}

// openState writes one agent's opening_state, unless a record already opens
// its history. It reports whether it wrote one.
func (r *Recorder) openState(ctx context.Context, organizationID, agentID uuid.UUID, traceID string) (bool, error) {
	var state State
	out, err := r.w.Write(ctx, store.Write{
		Class:          store.ClassObservation,
		OrganizationID: organizationID.String(),
		Draft:          r.openingDraft(organizationID, agentID, traceID),
		Apply: func(ctx context.Context, tx *sql.Tx) error {
			s, err := readState(ctx, tx, organizationID, agentID, true)
			if errors.Is(err, ErrAgentNotFound) {
				return errAgentGone
			}
			state = s
			return err
		},
		Guard: func(ctx context.Context, _ time.Time, probe *store.Probe, d *record.Draft) (*store.Stored, error) {
			existing, err := opened(ctx, probe, agentID)
			if err != nil || existing != nil {
				return existing, err
			}
			withState(d, state)
			return nil, nil
		},
	})
	switch {
	case err != nil:
		return false, err
	case out.Unchained:
		return false, ErrChainNotStarted
	}
	return !out.Existing, nil
}
