package atc

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// AgentStatusReader reads one agent's status from the server's store.
type AgentStatusReader interface {
	// AgentStatus reads the status of the agent with id under ctx. found is
	// false when no agent has that id.
	AgentStatus(ctx context.Context, id uuid.UUID) (status domain.AgentStatus, found bool, err error)
}

// AgentStatusRevocation is the revocation source of the ATCs this server
// verifies: the status of the agent an ATC names, read from the server's own
// store, and no list fetched from another service. An agent's ATCs are revoked
// when no agent has its id, or when its status is one the signature, API-key
// and service-token paths refuse (domain.AgentStatusPermitsAuth): suspended,
// revoked, or a value other than verified and pending.
type AgentStatusRevocation struct {
	agents AgentStatusReader
}

// NewAgentStatusRevocation returns the revocation source that reads agents.
func NewAgentStatusRevocation(agents AgentStatusReader) *AgentStatusRevocation {
	return &AgentStatusRevocation{agents: agents}
}

// AgentRevoked implements atcdomain.RevocationSource with one read of the
// agent's status under ctx. A failed read is returned as an error.
func (s *AgentStatusRevocation) AgentRevoked(ctx context.Context, agentID uuid.UUID) (bool, error) {
	if s == nil || s.agents == nil {
		return false, errors.New("no agent store to read the revocation state from")
	}
	status, found, err := s.agents.AgentStatus(ctx, agentID)
	if err != nil {
		return false, err
	}
	if !found {
		return true, nil
	}
	return !domain.AgentStatusPermitsAuth(status), nil
}
