package domain

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/google/uuid"
)

// The values a delegation takes when its creator leaves the field out. They
// are the column defaults of the a2a_delegation table (migrations 070 and 077).
const (
	DefaultA2ADelegationTrustAttenuationFactor = 0.8
	DefaultA2ADelegationMinTrustScore          = 0.3
	DefaultA2ADelegationMaxChainDepth          = 3
)

// MaxA2ADelegationChainLength is the most delegations one chain verification
// reads, and the largest maxChainDepth a delegation may be created with.
const MaxA2ADelegationChainLength = 10

// A2ADelegationActiveMessage answers a grant for a delegator and delegate
// that already have an active delegation.
const A2ADelegationActiveMessage = "an active delegation from this delegator to this delegate exists; revoke it first"

var (
	// ErrA2ADelegationNotFound is a delegation that does not exist or belongs
	// to another organization. The two are one answer on purpose.
	ErrA2ADelegationNotFound = errors.New("delegation not found")
	// ErrA2ADelegationActive is a second active delegation for a delegator and
	// delegate that already have one.
	ErrA2ADelegationActive = errors.New(A2ADelegationActiveMessage)
)

// A2ADelegationHop is one delegation's result in a chain verification.
type A2ADelegationHop struct {
	Index                  int       `json:"index"`
	DelegationID           uuid.UUID `json:"delegationId"`
	DelegatorAgentID       uuid.UUID `json:"delegatorAgentId"`
	DelegateAgentID        uuid.UUID `json:"delegateAgentId"`
	TrustAttenuationFactor float64   `json:"trustAttenuationFactor"`
	// TrustFloor is the floor this delegation's effective trust is held to:
	// the highest minDelegatedTrustScore of this delegation and the ones
	// before it in the chain.
	TrustFloor     float64 `json:"trustFloor"`
	EffectiveTrust float64 `json:"effectiveTrust"`
	Error          string  `json:"error,omitempty"`
}

// A2ADelegationChainVerification is the server's verdict on a delegation chain.
type A2ADelegationChainVerification struct {
	Valid          bool               `json:"valid"`
	RootAgentID    uuid.UUID          `json:"rootAgentId"`
	RootTrustScore float64            `json:"rootTrustScore"`
	EffectiveTrust float64            `json:"effectiveTrust"`
	SkillID        string             `json:"skillId,omitempty"`
	Hops           []A2ADelegationHop `json:"hops"`
}

// VerifyA2ADelegationChain checks a chain of delegations in order. chain[0] is
// the root delegation, and each later delegation's delegator must be the
// previous delegation's delegate. agents holds the chain's agents as read in
// the verifier's organization; an agent missing from it fails the delegation
// that names it.
//
// The trust starts at the root delegator's trust score. Every delegation, the
// root delegation included, multiplies it by its trustAttenuationFactor, and
// the result is rounded to six decimal places. A delegation whose effective
// trust is below its floor fails. The floor is the highest
// minDelegatedTrustScore of the delegation and the ones before it, so a later
// delegation cannot lower the floor an earlier one set.
//
// A delegation also fails when it is revoked, outside its validity window,
// held beyond the maxChainDepth of itself or an earlier delegation, reached
// through a delegation whose delegate may not delegate further, or names an
// agent whose status does not permit it to authenticate. skillID, when not
// empty, must be in every delegation's skill list; an empty list covers every
// skill. The chain is valid when it holds a delegation and none fails.
func VerifyA2ADelegationChain(chain []*A2ADelegation, agents map[uuid.UUID]*Agent, skillID string, now time.Time) *A2ADelegationChainVerification {
	result := &A2ADelegationChainVerification{SkillID: skillID, Hops: make([]A2ADelegationHop, 0, len(chain))}
	if len(chain) == 0 {
		return result
	}

	result.RootAgentID = chain[0].DelegatorAgentID
	trust := 0.0
	if root, ok := agents[result.RootAgentID]; ok {
		result.RootTrustScore = root.TrustScore
		if inUnitInterval(root.TrustScore) {
			trust = root.TrustScore
		}
	}

	floor := 0.0
	valid := true
	for i, d := range chain {
		if inUnitInterval(d.TrustAttenuationFactor) {
			trust = roundTrust(trust * d.TrustAttenuationFactor)
		} else {
			trust = 0
		}
		if inUnitInterval(d.MinDelegatedTrustScore) && d.MinDelegatedTrustScore > floor {
			floor = d.MinDelegatedTrustScore
		}
		hop := A2ADelegationHop{
			Index:                  i,
			DelegationID:           d.ID,
			DelegatorAgentID:       d.DelegatorAgentID,
			DelegateAgentID:        d.DelegateAgentID,
			TrustAttenuationFactor: d.TrustAttenuationFactor,
			TrustFloor:             floor,
			EffectiveTrust:         trust,
			Error:                  delegationHopError(chain, i, agents, skillID, now, trust, floor),
		}
		if hop.Error != "" {
			valid = false
		}
		result.Hops = append(result.Hops, hop)
	}
	result.EffectiveTrust = trust
	result.Valid = valid
	return result
}

// delegationHopError is the first check chain[i] fails, in the order the
// checks are listed here, or "" when it passes them all.
func delegationHopError(chain []*A2ADelegation, i int, agents map[uuid.UUID]*Agent, skillID string, now time.Time, trust, floor float64) string {
	d := chain[i]
	switch {
	case !d.IsActive:
		return "the delegation is revoked"
	case now.Before(d.ValidFrom):
		return "the delegation is not valid before " + d.ValidFrom.UTC().Format(time.RFC3339)
	case d.ValidUntil != nil && !now.Before(*d.ValidUntil):
		return "the delegation expired at " + d.ValidUntil.UTC().Format(time.RFC3339)
	}

	if i > 0 {
		prev := chain[i-1]
		if d.DelegatorAgentID != prev.DelegateAgentID {
			return "the chain is broken: the delegator is not the previous delegation's delegate"
		}
		if !prev.CanDelegateFurther {
			return "the previous delegation does not let its delegate delegate further"
		}
	}

	for j := 0; j <= i; j++ {
		if held := i - j + 1; held > chain[j].MaxChainDepth {
			return fmt.Sprintf("delegation %d has maxChainDepth %d, and this is delegation %d counted from it", j, chain[j].MaxChainDepth, held)
		}
	}

	for _, id := range []uuid.UUID{d.DelegatorAgentID, d.DelegateAgentID} {
		agent, ok := agents[id]
		if !ok {
			return fmt.Sprintf("agent %s is not in this organization", id)
		}
		if !AgentStatusPermitsAuth(agent.Status) {
			return fmt.Sprintf("agent %s is %s", id, agent.Status)
		}
	}
	if i == 0 && !inUnitInterval(agents[d.DelegatorAgentID].TrustScore) {
		return "the root delegator's trust score " + formatTrust(agents[d.DelegatorAgentID].TrustScore) + " is outside 0 to 1"
	}

	if skillID != "" && len(d.SkillIDs) > 0 && !containsString(d.SkillIDs, skillID) {
		return fmt.Sprintf("the delegation does not cover skill %q", skillID)
	}

	if !inUnitInterval(d.TrustAttenuationFactor) {
		return "trustAttenuationFactor " + formatTrust(d.TrustAttenuationFactor) + " is outside 0 to 1"
	}
	if !inUnitInterval(d.MinDelegatedTrustScore) {
		return "minDelegatedTrustScore " + formatTrust(d.MinDelegatedTrustScore) + " is outside 0 to 1"
	}
	if trust < floor {
		return "effective trust " + formatTrust(trust) + " is below the trust floor " + formatTrust(floor)
	}
	return ""
}

func inUnitInterval(v float64) bool {
	return !math.IsNaN(v) && v >= 0 && v <= 1
}

// roundTrust rounds a trust figure to six decimal places, so that 0.9 x 0.8
// is reported and compared as 0.72.
func roundTrust(v float64) float64 {
	return math.Round(v*1e6) / 1e6
}

func formatTrust(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}
