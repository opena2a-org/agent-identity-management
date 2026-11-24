package application

import (
	"context"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// VerifyReads are the repositories VerifyCapability reads the agent, its
// granted capabilities and its organization's enforcement mode from.
type VerifyReads struct {
	Agent        domain.AgentRepository
	Capability   domain.CapabilityRepository
	Organization domain.OrganizationRepository
}

// VerifyReadSource picks the repositories one verification reads from.
// Implementations must return the replica's only when it holds every write the
// primary had committed by the time of the call; a revocation committed
// before a verification began must be visible to it.
type VerifyReadSource interface {
	VerifyReads(ctx context.Context) VerifyReads
}

// SetVerifyReads sets where VerifyCapability reads from. When unset, which is
// the default, it reads from the repositories the service was built with. The
// writes VerifyCapability makes always go to those.
func (s *AgentService) SetVerifyReads(source VerifyReadSource) {
	s.verifyReadSource = source
}

// verifyReads returns the repositories one verification reads from.
func (s *AgentService) verifyReads(ctx context.Context) VerifyReads {
	if s.verifyReadSource != nil {
		reads := s.verifyReadSource.VerifyReads(ctx)
		if reads.Agent != nil && reads.Capability != nil && reads.Organization != nil {
			return reads
		}
	}
	return VerifyReads{Agent: s.agentRepo, Capability: s.capabilityRepo, Organization: s.orgRepo}
}

// ReplicaVerifyReads reads from Replica when Fence says the replica holds every
// committed write, and from Primary otherwise.
type ReplicaVerifyReads struct {
	Primary VerifyReads
	Replica VerifyReads
	// Fence reports whether the replica may answer a read made now.
	Fence func(ctx context.Context) bool
	// OnRoute, when set, is told where each verification's reads went.
	OnRoute func(replica bool)
}

// VerifyReads implements VerifyReadSource.
func (r *ReplicaVerifyReads) VerifyReads(ctx context.Context) VerifyReads {
	useReplica := r.Fence != nil && r.Fence(ctx)
	if r.OnRoute != nil {
		r.OnRoute(useReplica)
	}
	if useReplica {
		return r.Replica
	}
	return r.Primary
}
