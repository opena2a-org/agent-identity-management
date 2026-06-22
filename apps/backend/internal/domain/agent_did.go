package domain

import (
	"fmt"

	"github.com/google/uuid"
)

// AgentDIDPrefix is the AIM-specific DID prefix. AIM agents are identified by the
// did:aip method, and the AIM namespace identifier is "aim_" followed by the
// agent's UUID (see internal/interfaces/http/handlers/aip_handler.go for the
// resolver that maps a did:aip:aim_<uuid> back to GetByID).
const AgentDIDPrefix = "did:aip:aim_"

// BuildAgentDID returns the canonical did:aip identifier for an AIM agent:
// did:aip:aim_<uuid>. This is the single source of truth for AIM's DID format,
// shared by the DID resolver (aip_handler) and the ATC issuance trigger so the
// agentDid carried into a credential is byte-identical to the one the resolver
// answers for.
func BuildAgentDID(agentID uuid.UUID) string {
	return fmt.Sprintf("%s%s", AgentDIDPrefix, agentID.String())
}
