package domain

import (
	"time"

	"github.com/google/uuid"
)

// RegistryATCRevocation is a stored request asking the Registry to revoke the
// Agent Trust Credentials it issued for an agent: one row of
// registry_atc_revocations, as a sender claimed it.
type RegistryATCRevocation struct {
	// AgentID is the agent the request names.
	AgentID uuid.UUID
	// RequestID names this storing of the request. Storing the request again
	// for the same agent replaces it, and a row is updated or deleted only
	// under the RequestID it was claimed with.
	RequestID uuid.UUID
	// RequestedAt is when RequestID was stored.
	RequestedAt time.Time
	// Attempts counts the requests started for RequestID, the claimed one
	// included, so it is at least 1.
	Attempts int
}
