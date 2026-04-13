package atc

import (
	"time"

	"github.com/google/uuid"
)

// ATCClaims represents the claims extracted from an Agent Trust Certificate.
// This is the data surface that downstream services (secrets, delegation, etc.) consume.
type ATCClaims struct {
	AgentID      uuid.UUID `json:"agentId"`
	Issuer       string    `json:"issuer"`
	Capabilities []string  `json:"capabilities"`
	IssuedAt     time.Time `json:"issuedAt"`
	ExpiresAt    time.Time `json:"expiresAt"`
	ATCID        string    `json:"atcId"`
}

// IsExpired returns true if the ATC has expired.
func (c *ATCClaims) IsExpired() bool {
	return time.Now().After(c.ExpiresAt)
}

// HasCapability checks if the ATC grants a specific capability.
func (c *ATCClaims) HasCapability(capability string) bool {
	for _, cap := range c.Capabilities {
		if cap == capability {
			return true
		}
	}
	return false
}

// ATCVerifier verifies Agent Trust Certificates and extracts claims.
// The initial implementation is a JWT-based shim; this interface enables
// a clean swap to real ATC verification when the ATC spec is implemented.
type ATCVerifier interface {
	// Verify validates a raw ATC token and returns the extracted claims.
	// Returns an error if the token is invalid, expired, or revoked.
	Verify(rawToken string) (*ATCClaims, error)

	// IsRevoked checks whether an ATC has been revoked via CRL.
	// Returns true if the ATC is on the revocation list.
	IsRevoked(atcID string) (bool, error)
}
