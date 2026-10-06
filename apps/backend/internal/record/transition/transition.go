// Package transition writes the authorization_transition record that
// accompanies a change of an agent's authorization state, and replays an
// organization's chain to rebuild each agent's state from its records alone.
//
// An agent's authorization state has four members:
//
//   - scope: the capability types in force, empty while the agent is
//     suspended or revoked;
//   - opena2a.granted_scope: the capability types granted and not revoked;
//   - opena2a.status: the agent's status;
//   - opena2a.keys: each key the agent row holds, with its algorithm, its role
//     and an identifier from which the public key is recovered (the did:key
//     form for an Ed25519 key).
//
// The recorder reads the state before a change from the state tables inside
// the change's transaction, under a row lock on the agent, and the state after
// it from the same transaction. Each record's previous_state therefore equals
// the new_state of the agent's record before it, unless some path changed the
// agent without a record. Replay reports that case as a *ContinuityError.
//
// A record's members sit in the three parts of record.Draft: the record type,
// the trigger type, the state space and the writer's own fields in the
// retained part; the trace, the organization, the agent and both states in
// the tenant part; the actor in the personal part.
//
// A registration opens an agent's history: the agent has no state before it,
// so its record's previous_state is null, and replay accepts a null
// previous_state only on an agent's first record.
//
// A pending capability request and a rejected one are null transitions: the
// recorder refuses either when its previous and new states differ. A
// rejection's record carries opena2a.outcome "rejected" in the retained part.
// A pending request's record takes an event id derived from the request's id
// (RequestEventID), so the decision on the request finds it and names it as
// its parent_id, in the request's trace.
//
// The class of a trigger decides what a failed record write does. An
// expansion or a destruction is refused whole: no record and no change, and
// the error wraps ErrRecordUnavailable. A reduction is never blocked by the
// record path: when its record cannot be written, the change commits without
// one, the record writer counts the failure, and a SECURITY line names the
// organization, the trigger and the agent.
//
// Not built here: the debt row that lets such a record be written late and
// its settlement, the opening_state record and its sweep, and the records of
// a talks_to change, an agent's deletion, a hybrid mode change, a compromise
// suspension and the key the service generates for an agent that signs
// through it.
package transition

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record/store"
)

const (
	// RecordType is the type of every record this package writes.
	RecordType = "authorization_transition"
	// StateSpaceAgent is opena2a.state_space of an agent's transition.
	StateSpaceAgent = "agent"
	// WriterVersion is opena2a.writer_version of every record this package
	// writes. It changes when the meaning of a member this package writes
	// changes.
	WriterVersion = "transition-1"
	// Source is opena2a.source: the record states a change the service made.
	Source = "service"
)

// Trigger is trigger.type: what caused a transition. The set is closed.
type Trigger string

// The agent-space triggers.
const (
	TriggerRegistrationBaseline       Trigger = "registration_baseline"
	TriggerDirectGrant                Trigger = "direct_grant"
	TriggerCapabilityRequested        Trigger = "capability_requested"
	TriggerRequestApproved            Trigger = "request_approved"
	TriggerRequestAutoApproved        Trigger = "request_auto_approved"
	TriggerRequestRejected            Trigger = "request_rejected"
	TriggerRevocation                 Trigger = "revocation"
	TriggerRevocationOnReregistration Trigger = "revocation_on_reregistration"
	TriggerCapabilityAttributeChanged Trigger = "capability_attribute_changed"
	TriggerAgentVerified              Trigger = "agent_verified"
	TriggerAgentSuspended             Trigger = "agent_suspended"
	TriggerAgentReactivated           Trigger = "agent_reactivated"
	TriggerAgentRevoked               Trigger = "agent_revoked"
	TriggerAgentDeleted               Trigger = "agent_deleted"
	TriggerKeyRotated                 Trigger = "key_rotated"
	TriggerKeyUpdated                 Trigger = "key_updated"
	TriggerKeyExpiredSuspension       Trigger = "key_expired_suspension"
	TriggerCompromiseSuspension       Trigger = "compromise_suspension"
)

// classes is the class of each agent-space trigger. An expansion widens what
// an agent may do or who may act as it; a reduction narrows it; a destruction
// removes rows that bear records.
var classes = map[Trigger]store.Class{
	TriggerRegistrationBaseline:       store.ClassExpansion,
	TriggerDirectGrant:                store.ClassExpansion,
	TriggerCapabilityRequested:        store.ClassExpansion,
	TriggerRequestApproved:            store.ClassExpansion,
	TriggerRequestAutoApproved:        store.ClassExpansion,
	TriggerCapabilityAttributeChanged: store.ClassExpansion,
	TriggerAgentVerified:              store.ClassExpansion,
	TriggerAgentReactivated:           store.ClassExpansion,
	TriggerKeyRotated:                 store.ClassExpansion,
	TriggerKeyUpdated:                 store.ClassExpansion,
	TriggerAgentDeleted:               store.ClassDestruction,
	TriggerRevocation:                 store.ClassReduction,
	TriggerRevocationOnReregistration: store.ClassReduction,
	TriggerRequestRejected:            store.ClassReduction,
	TriggerAgentSuspended:             store.ClassReduction,
	TriggerAgentRevoked:               store.ClassReduction,
	TriggerKeyExpiredSuspension:       store.ClassReduction,
	TriggerCompromiseSuspension:       store.ClassReduction,
}

// Class returns the class of a trigger, and false for a trigger outside the
// set.
func (t Trigger) Class() (store.Class, bool) {
	c, ok := classes[t]
	return c, ok
}

// opens reports whether a trigger opens the agent's history: the agent has
// no row before the change, and the record's previous_state is null.
func (t Trigger) opens() bool {
	return t == TriggerRegistrationBaseline
}

// null reports whether a trigger records an act that leaves the agent's
// state as it was: a pending capability request, or a rejected one.
func (t Trigger) null() bool {
	return t == TriggerCapabilityRequested || t == TriggerRequestRejected
}

// OutcomeRejected is opena2a.outcome of a request_rejected record.
const OutcomeRejected = "rejected"

// outcome is opena2a.outcome of a trigger's record: OutcomeRejected for a
// rejection, which records a decision that left the agent's state as it was,
// and "" for every other trigger, whose record carries no outcome.
func (t Trigger) outcome() string {
	if t == TriggerRequestRejected {
		return OutcomeRejected
	}
	return ""
}

// ActorType is the kind of party that caused a transition.
type ActorType string

const (
	ActorUser   ActorType = "user"
	ActorAgent  ActorType = "agent"
	ActorSystem ActorType = "system"
)

// Actor is who caused a transition. It sits in the record's personal part.
type Actor struct {
	Type ActorType
	// ID is the user's or the agent's id, and uuid.Nil for the system.
	ID uuid.UUID
}

// User is a transition caused by a user.
func User(id uuid.UUID) Actor { return Actor{Type: ActorUser, ID: id} }

// Agent is a transition caused by an agent.
func Agent(id uuid.UUID) Actor { return Actor{Type: ActorAgent, ID: id} }

// System is a transition the service made on its own.
func System() Actor { return Actor{Type: ActorSystem} }

func (a Actor) valid() bool {
	switch a.Type {
	case ActorSystem:
		return a.ID == uuid.Nil
	case ActorUser, ActorAgent:
		return a.ID != uuid.Nil
	default:
		return false
	}
}

// String is the actor member: "system", or the type and the id joined by a
// colon.
func (a Actor) String() string {
	if a.Type == ActorSystem {
		return string(ActorSystem)
	}
	return string(a.Type) + ":" + a.ID.String()
}

type contextKey int

const (
	actorKey contextKey = iota
	triggerKey
)

// WithActor returns a context that names the actor of the transitions made
// under it. A route sets it from the authenticated caller.
func WithActor(ctx context.Context, a Actor) context.Context {
	return context.WithValue(ctx, actorKey, a)
}

// ActorFrom returns the actor WithActor set, if any.
func ActorFrom(ctx context.Context) (Actor, bool) {
	a, ok := ctx.Value(actorKey).(Actor)
	return a, ok
}

// WithTrigger returns a context that names the trigger of a change whose
// service method serves more than one cause.
func WithTrigger(ctx context.Context, t Trigger) context.Context {
	return context.WithValue(ctx, triggerKey, t)
}

// TriggerFrom returns the trigger WithTrigger set, or fallback.
func TriggerFrom(ctx context.Context, fallback Trigger) Trigger {
	if t, ok := ctx.Value(triggerKey).(Trigger); ok {
		return t
	}
	return fallback
}

// NewTraceID mints a trace id: 16 bytes from the operating system's CSPRNG
// as 32 lowercase hex characters, never all zeros.
func NewTraceID() (string, error) {
	var b [16]byte
	for {
		if _, err := rand.Read(b[:]); err != nil {
			return "", fmt.Errorf("transition: trace id: %w", err)
		}
		if b != [16]byte{} {
			return hex.EncodeToString(b[:]), nil
		}
	}
}

// validTraceID reports whether s is 32 lowercase hex characters, not all
// zeros.
func validTraceID(s string) bool {
	if len(s) != 32 {
		return false
	}
	zero := true
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
		if c != '0' {
			zero = false
		}
	}
	return !zero
}
