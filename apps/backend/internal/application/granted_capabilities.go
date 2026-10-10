package application

import (
	"errors"
	"regexp"
	"sort"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// activeCapabilityReader lists an agent's capability grants that have not been
// revoked. It is the agent_capabilities table, the same source the capability
// verification path authorizes against.
type activeCapabilityReader interface {
	GetActiveCapabilitiesByAgentID(agentID uuid.UUID) ([]*domain.AgentCapability, error)
}

// errCapabilityReaderNotConfigured is returned when a service has no grant
// source. Serving a capability list from anywhere else is wrong, so the caller
// fails closed instead.
var errCapabilityReaderNotConfigured = errors.New("capability grants: no grant source is configured")

// capabilityTokenPattern is the namespace:operation token grammar of the ATX
// credential schema (atx-credential-v1.1, capabilities items). A grant whose type
// does not match it cannot be carried in a credential as it is.
var capabilityTokenPattern = regexp.MustCompile(`^[a-z0-9_-]+:[A-Za-z0-9_.*-]+$`)

// grantedCapabilityTokens turns an agent's grant rows into the capability list
// AIM serves for the agent: the capability type of every grant that is not
// revoked and not a honeytoken, deduplicated and sorted ascending. The order is
// part of the contract, not style: an issued credential's signature covers the
// list, so two issuances of an unchanged grant set must carry identical bytes
// whatever order the rows were read in.
//
// A grant whose type is outside the token grammar is left out and counted in
// dropped. It is never rewritten into something that fits and it never fails the
// caller; the count is for the caller's log. The returned slice is never nil, so
// an agent with no active grant serializes as the empty array the schema requires.
func grantedCapabilityTokens(grants []*domain.AgentCapability) (tokens []string, dropped int) {
	tokens = make([]string, 0, len(grants))
	seen := make(map[string]struct{}, len(grants))
	for _, g := range grants {
		if g == nil || g.RevokedAt != nil || g.Honeytoken {
			continue
		}
		if !capabilityTokenPattern.MatchString(g.CapabilityType) {
			dropped++
			continue
		}
		if _, dup := seen[g.CapabilityType]; dup {
			continue
		}
		seen[g.CapabilityType] = struct{}{}
		tokens = append(tokens, g.CapabilityType)
	}
	sort.Strings(tokens)
	return tokens, dropped
}

// activeCapabilityTokens reads the agent's active grants and returns them as
// grantedCapabilityTokens lists them. A nil reader or a failed read is an error:
// the registration-time list on the agent row is a self-reported declaration and
// is never the fallback.
func activeCapabilityTokens(reader activeCapabilityReader, agentID uuid.UUID) (tokens []string, dropped int, err error) {
	if reader == nil {
		return nil, 0, errCapabilityReaderNotConfigured
	}
	grants, err := reader.GetActiveCapabilitiesByAgentID(agentID)
	if err != nil {
		return nil, 0, err
	}
	tokens, dropped = grantedCapabilityTokens(grants)
	return tokens, dropped, nil
}
