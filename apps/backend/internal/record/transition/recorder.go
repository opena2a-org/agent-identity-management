package transition

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record/store"
)

// EventTransitionUnrecorded is the console event of a reduction that
// committed without its record.
const EventTransitionUnrecorded = "authorization_transition_unrecorded"

// ErrInvalidChange is wrapped by the error of a change the recorder refuses
// before anything runs.
var ErrInvalidChange = errors.New("transition: invalid change")

// ErrNoChange is returned for a talks_to change whose list, read under the
// agent's row lock, is the list it would store, compared as a set. Nothing
// changed and no record was written.
var ErrNoChange = errors.New("transition: the change leaves the agent's talks_to as it is")

// ErrRecordUnavailable is wrapped by the error of an expansion or a
// destruction refused because its record could not be written. Nothing
// changed.
var ErrRecordUnavailable = errors.New("transition: the change was not made because its audit record could not be written")

// Config configures a Recorder.
type Config struct {
	// Writer appends the records.
	Writer *store.Writer
	// DB commits a reduction whose record could not be written.
	DB *sql.DB
	// Issuer is opena2a.issuer: the deployment's issuer, urn:uuid: followed
	// by a lowercase hyphenated UUID.
	Issuer string
	// Logger receives the console lines. When nil, the standard logger does.
	Logger *log.Logger
}

// Recorder makes changes of an agent's authorization state, each with its
// authorization_transition record in the change's transaction.
type Recorder struct {
	w      *store.Writer
	db     *sql.DB
	issuer string
	log    *log.Logger
}

// NewRecorder returns a recorder.
func NewRecorder(cfg Config) (*Recorder, error) {
	if cfg.Writer == nil || cfg.DB == nil {
		return nil, errors.New("transition: a recorder needs a record writer and a database")
	}
	id, ok := strings.CutPrefix(cfg.Issuer, "urn:uuid:")
	if parsed, err := uuid.Parse(id); !ok || err != nil || parsed.String() != id {
		return nil, errors.New("transition: the issuer is not urn:uuid: followed by a lowercase hyphenated UUID")
	}
	r := &Recorder{w: cfg.Writer, db: cfg.DB, issuer: cfg.Issuer, log: cfg.Logger}
	if r.log == nil {
		r.log = log.Default()
	}
	return r, nil
}

// Change is one change of an agent's authorization state.
type Change struct {
	OrganizationID uuid.UUID
	AgentID        uuid.UUID
	Trigger        Trigger
	// Class is set for a trigger classed by comparison, and only for one: the
	// class TalksToClass gave for the list the caller read before the change.
	// The change is refused when the list under the agent's row lock gives
	// another.
	Class store.Class
	Actor Actor
	// TraceID is trace_id: the parent record's trace when ParentID is set,
	// otherwise one the server minted with NewTraceID for this request or
	// job run.
	TraceID string
	// ParentID, when set, is parent_id: the event id of the record that
	// caused this change.
	ParentID string
	// EventID, when set, is the record's event_id; otherwise the recorder
	// mints a random one. A pending capability request's record takes
	// RequestEventID of the request.
	EventID string
	// Apply makes the change in tx, after the agent's row is locked and its
	// previous state read. It runs statements only.
	Apply func(ctx context.Context, tx *sql.Tx) error
}

// Result is what a change left.
type Result struct {
	// Previous is zero for a registration, whose agent had no state.
	Previous, New State
	// Recorded is false for a reduction that committed without its record.
	Recorded bool
	// Appended is the record, when Recorded.
	Appended store.Appended
}

// Record makes the change and appends its record in one transaction.
//
// When the record cannot be written, an expansion or a destruction is
// refused with an error that wraps ErrRecordUnavailable and the record
// writer's *store.WriteError, and nothing changes. A reduction commits
// without its record and returns Recorded false. An error of Apply is
// returned as it is, and nothing changes. A talks_to change that changes
// nothing returns ErrNoChange.
func (r *Recorder) Record(ctx context.Context, c Change) (Result, error) {
	class, err := r.check(c)
	if err != nil {
		return Result{}, err
	}
	var prev, next State
	apply := func(ctx context.Context, tx *sql.Tx) error {
		p, n, err := change(ctx, tx, c)
		prev, next = p, n
		return err
	}
	// The states are read in Apply, before the append lock; the guard only
	// places them in the tenant part. A registration has no previous state.
	guard := func(_ context.Context, _ time.Time, _ *store.Probe, d *record.Draft) (*store.Stored, error) {
		if !c.Trigger.opens() {
			d.Tenant["previous_state"] = prev.member()
		}
		d.Tenant["new_state"] = next.member()
		return nil, nil
	}
	out, err := r.w.Write(ctx, store.Write{
		Class:          class,
		OrganizationID: c.OrganizationID.String(),
		Draft:          r.draft(c),
		Apply:          apply,
		Guard:          guard,
		// A reduction whose record cannot be written commits below, with
		// its SECURITY line, not with a debt row.
		NoDebt: true,
	})
	var we *store.WriteError
	switch {
	case err == nil:
		// A pre-chain writer commits the change with no record.
		return Result{Previous: prev, New: next, Recorded: !out.Unchained, Appended: out}, nil
	case class == store.ClassReduction && (errors.As(err, &we) || lockNotAvailable(err)):
		// The agent's row lock is waited for inside the record writer's lock
		// budget, so a lock timeout there is a record path failure too.
		return r.commitUnrecorded(ctx, c)
	case !errors.As(err, &we):
		return Result{}, err
	default:
		return Result{}, fmt.Errorf("%w (%s): %w", ErrRecordUnavailable, c.Trigger, err)
	}
}

// lockNotAvailable reports a statement that ended at its lock timeout.
func lockNotAvailable(err error) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && pqErr.Code == "55P03"
}

func (r *Recorder) check(c Change) (store.Class, error) {
	class, ok := c.Trigger.Class()
	if c.Trigger.ClassedByComparison() {
		class, ok = c.Class, true
	}
	switch {
	case c.Trigger.ClassedByComparison() && c.Class != store.ClassExpansion && c.Class != store.ClassReduction:
		return "", fmt.Errorf("%w: %s needs the class its lists give, an expansion or a reduction", ErrInvalidChange, c.Trigger)
	case !c.Trigger.ClassedByComparison() && c.Class != "":
		return "", fmt.Errorf("%w: %s takes the class of its trigger, not one given", ErrInvalidChange, c.Trigger)
	case !ok:
		return "", fmt.Errorf("%w: %q is not an agent-space trigger", ErrInvalidChange, c.Trigger)
	case c.OrganizationID == uuid.Nil || c.AgentID == uuid.Nil:
		return "", fmt.Errorf("%w: the organization and the agent are required", ErrInvalidChange)
	case !c.Actor.valid():
		return "", fmt.Errorf("%w: the actor is not a user, an agent or the system", ErrInvalidChange)
	case !validTraceID(c.TraceID):
		return "", fmt.Errorf("%w: trace_id is not 32 lowercase hex characters other than all zeros", ErrInvalidChange)
	case c.ParentID != "" && !isCanonicalUUID(c.ParentID):
		return "", fmt.Errorf("%w: parent_id is not a lowercase hyphenated UUID", ErrInvalidChange)
	case c.EventID != "" && !isCanonicalUUID(c.EventID):
		return "", fmt.Errorf("%w: event_id is not a lowercase hyphenated UUID", ErrInvalidChange)
	case c.Apply == nil:
		return "", fmt.Errorf("%w: the change has no statement", ErrInvalidChange)
	}
	return class, nil
}

// draft is the change's record before its states are known.
func (r *Recorder) draft(c Change) record.Draft {
	origin := "server"
	var parent any
	if c.ParentID != "" {
		origin, parent = "parent", c.ParentID
	}
	eventID := c.EventID
	if eventID == "" {
		eventID = uuid.NewString()
	}
	retained := map[string]any{
		"issuer":         r.issuer,
		"writer_version": WriterVersion,
		"source":         Source,
		"state_space":    StateSpaceAgent,
		"trace_origin":   origin,
	}
	if o := c.Trigger.outcome(); o != "" {
		retained["outcome"] = o
	}
	return record.Draft{
		EventID: eventID,
		Type:    RecordType,
		Retained: map[string]any{
			"trigger": map[string]any{"type": string(c.Trigger)},
			"opena2a": retained,
		},
		Tenant: map[string]any{
			"trace_id":       c.TraceID,
			"parent_id":      parent,
			"previous_state": nil,
			"new_state":      nil,
			"opena2a": map[string]any{
				"organization_id":  c.OrganizationID.String(),
				"subject_agent_id": c.AgentID.String(),
			},
		},
		Personal: map[string]any{"actor": c.Actor.String()},
	}
}

// change locks the agent, reads its state, makes the change and reads the
// state again, all in tx. A registration finds no agent before its statement
// runs and leaves prev zero; one that finds the agent is refused. A change
// whose states break its trigger's rule (checkStates) is refused. A refused
// change's transaction rolls back.
func change(ctx context.Context, tx *sql.Tx, c Change) (prev, next State, err error) {
	if c.Trigger.opens() {
		_, err = readState(ctx, tx, c.OrganizationID, c.AgentID, false)
		switch {
		case err == nil:
			return State{}, State{}, fmt.Errorf("%w: %s for an agent that already exists", ErrInvalidChange, c.Trigger)
		case !errors.Is(err, ErrAgentNotFound):
			return State{}, State{}, err
		}
	} else if prev, err = readState(ctx, tx, c.OrganizationID, c.AgentID, true); err != nil {
		return State{}, State{}, err
	}
	if err = c.Apply(ctx, tx); err != nil {
		return State{}, State{}, err
	}
	if next, err = readState(ctx, tx, c.OrganizationID, c.AgentID, false); err != nil {
		return State{}, State{}, err
	}
	if err = checkStates(c, prev, next); err != nil {
		return State{}, State{}, err
	}
	return prev, next, nil
}

// checkStates refuses a change whose states break its trigger's rule. A null
// transition changes nothing. Only a talks_to trigger, or the registration
// that opens the agent, sets the talks_to list. A talks_to trigger changes
// the list and nothing else, in the class the caller gave; one that changes
// nothing returns ErrNoChange.
func checkStates(c Change, prev, next State) error {
	switch {
	case c.Trigger.null() && !Equal(prev, next):
		return fmt.Errorf("%w: %s changed the agent's authorization state", ErrInvalidChange, c.Trigger)
	case c.Trigger.opens():
		return nil
	case !c.Trigger.ClassedByComparison():
		if !equalStrings(prev.TalksTo, next.TalksTo) {
			return fmt.Errorf("%w: %s changed the agent's talks_to", ErrInvalidChange, c.Trigger)
		}
		return nil
	}
	class, changed := TalksToClass(prev.TalksTo, next.TalksTo)
	if !changed {
		return ErrNoChange
	}
	rest := next
	rest.TalksTo = prev.TalksTo
	if !Equal(prev, rest) {
		return fmt.Errorf("%w: %s changed more than the agent's talks_to", ErrInvalidChange, c.Trigger)
	}
	if class != c.Class {
		return fmt.Errorf("%w: %s is a %s by its lists, not the %s it was made as", ErrInvalidChange, c.Trigger, class, c.Class)
	}
	return nil
}

// commitUnrecorded makes a reduction whose record could not be written, in
// a transaction of its own, and writes the SECURITY line that names it. Like
// the change it stands for when no recorder is set, it waits for the agent's
// row lock without a timeout of its own. The record writer has already
// counted the failed write.
func (r *Recorder) commitUnrecorded(ctx context.Context, c Change) (Result, error) {
	occurred := time.Now()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Result{}, fmt.Errorf("transition: commit reduction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	prev, next, err := change(ctx, tx, c)
	if err != nil {
		return Result{}, err
	}
	if err := tx.Commit(); err != nil {
		return Result{}, fmt.Errorf("transition: commit reduction: %w", err)
	}
	ts, _ := record.FormatTimestamp(occurred)
	r.log.Printf("SECURITY %s organization=%s state_space=%s trigger=%s subject=%s occurred_at=%s",
		EventTransitionUnrecorded, c.OrganizationID, StateSpaceAgent, c.Trigger, c.AgentID, ts)
	return Result{Previous: prev, New: next}, nil
}

// isCanonicalUUID reports whether s is a UUID in lowercase hyphenated form.
func isCanonicalUUID(s string) bool {
	u, err := uuid.Parse(s)
	return err == nil && u.String() == s
}
