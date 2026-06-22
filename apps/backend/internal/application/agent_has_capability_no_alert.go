package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// HasCapabilityNoAlert reports whether the agent has a granted capability matching
// the request WITHOUT firing honeytoken detection. It exists for a secondary
// has-capability check inside a request that has ALREADY run detection — notably the
// SDK /verify handler (CreateVerification), which calls VerifyCapability first and
// then needs the raw boolean to decide capability-violation alerting. Using
// HasCapability there would double-fire the honeytoken alert/audit for one request.
//
// This method lives in its own file (not agent_service.go) on purpose: agent_service.go
// is protected from the AIM Cloud sync (the cloud keeps a divergent copy and skips the
// synced version), but the synced AgentServicer / AgentServicerForVerification handler
// interfaces require this method. Defining it in a synced file lets *AgentService satisfy
// those interfaces in the cloud build. Its body only touches s.capabilityRepo and
// s.matchesCapability, both present since the initial release, so the cloud's divergent
// AgentService still compiles.
func (s *AgentService) HasCapabilityNoAlert(ctx context.Context, agentID uuid.UUID, capabilityToCheck string, resource string) (bool, error) {
	capabilities, err := s.capabilityRepo.GetActiveCapabilitiesByAgentID(agentID)
	if err != nil {
		return false, fmt.Errorf("failed to get capabilities: %w", err)
	}
	for _, cap := range capabilities {
		if s.matchesCapability(capabilityToCheck, resource, cap.CapabilityType) {
			return true, nil
		}
	}
	return false, nil
}
