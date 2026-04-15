package atc

import (
	"strings"
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

	// Fields added in ATC Verification Spec v1
	IssuerPublicKey []byte            `json:"issuerPublicKey,omitempty"`
	PQCAlgorithm    string            `json:"pqcAlgorithm,omitempty"`
	DelegationChain []DelegationEntry `json:"delegationChain,omitempty"`
	AuthMethod      string            `json:"authMethod,omitempty"` // "atc", "jwt" (for shim compat)
}

// DelegationEntry represents one link in an ATC issuer delegation chain.
type DelegationEntry struct {
	IssuerURI       string `json:"issuerUri"`
	IssuerPublicKey []byte `json:"issuerPublicKey"`
	Signature       []byte `json:"signature"`
}

// ATCPayload is the CBOR-encoded ATC wire format (spec v1, fields 1-12).
type ATCPayload struct {
	ATCID           string            `cbor:"1,keyasint"`
	AgentID         string            `cbor:"2,keyasint"`
	Issuer          string            `cbor:"3,keyasint"`
	IssuedAt        uint64            `cbor:"4,keyasint"`
	ExpiresAt       uint64            `cbor:"5,keyasint"`
	Capabilities    []string          `cbor:"6,keyasint"`
	IssuerPublicKey []byte            `cbor:"7,keyasint"`
	Ed25519Sig      []byte            `cbor:"8,keyasint"`
	MLDSAPublicKey  []byte            `cbor:"9,keyasint,omitempty"`
	MLDSASig        []byte            `cbor:"10,keyasint,omitempty"`
	DelegationChain []ChainEntryCBOR  `cbor:"11,keyasint,omitempty"`
	PQCAlgorithm    string            `cbor:"12,keyasint,omitempty"`
}

// ChainEntryCBOR is the CBOR wire format for a delegation chain entry.
type ChainEntryCBOR struct {
	IssuerURI       string `cbor:"1,keyasint"`
	IssuerPublicKey []byte `cbor:"2,keyasint"`
	Signature       []byte `cbor:"3,keyasint"`
}

// SignedPayload is the CBOR-encoded subset of fields 1-7 that signatures cover.
type SignedPayload struct {
	ATCID           string   `cbor:"1,keyasint"`
	AgentID         string   `cbor:"2,keyasint"`
	Issuer          string   `cbor:"3,keyasint"`
	IssuedAt        uint64   `cbor:"4,keyasint"`
	ExpiresAt       uint64   `cbor:"5,keyasint"`
	Capabilities    []string `cbor:"6,keyasint"`
	IssuerPublicKey []byte   `cbor:"7,keyasint"`
}

// ATC size and TTL constraints from spec v1.
const (
	MaxATCSize              = 8 * 1024 // 8 KiB
	MaxCapabilities         = 64
	MaxCapabilityLen        = 128
	MaxDelegationChainDepth = 5
	MaxTTL                  = time.Hour
	DefaultSecretsTTL       = 5 * time.Minute
	MaxClockSkew            = 30 * time.Second
)

// IsExpired returns true if the ATC has expired.
func (c *ATCClaims) IsExpired() bool {
	return time.Now().After(c.ExpiresAt)
}

// HasCapability checks if the ATC grants a specific capability.
// Supports hierarchical matching: "secrets:resolve" satisfies "secrets:resolve:github".
func (c *ATCClaims) HasCapability(required string) bool {
	for _, cap := range c.Capabilities {
		if cap == "*" || cap == required {
			return true
		}
		// Hierarchical prefix match: "secrets:resolve" satisfies "secrets:resolve:github"
		if strings.HasPrefix(required, cap+":") {
			return true
		}
	}
	return false
}

// ATCVerifier verifies Agent Trust Certificates and extracts claims.
// Implementations: JWTShimVerifier (Phase 0 compat), ATCRealVerifier (Phase 6).
type ATCVerifier interface {
	// Verify validates a raw ATC token and returns the extracted claims.
	// Returns an error if the token is invalid, expired, or revoked.
	Verify(rawToken string) (*ATCClaims, error)

	// IsRevoked checks whether an ATC has been revoked via CRL.
	// Returns true if the ATC is on the revocation list.
	IsRevoked(atcID string) (bool, error)
}

// ATCIssuer issues Agent Trust Certificates.
type ATCIssuer interface {
	// Issue creates a new ATC for the given agent with the specified capabilities and TTL.
	// Returns the base64url-encoded ATC token.
	Issue(agentID uuid.UUID, capabilities []string, ttl time.Duration) (string, error)
}

// TrustedIssuer holds the public keys for a trusted ATC issuer.
type TrustedIssuer struct {
	URI            string `json:"uri"`
	PublicKey      []byte `json:"publicKey"`      // Ed25519
	MLDSAPublicKey []byte `json:"mldsaPublicKey"` // optional
	MLDSAAlgorithm string `json:"mldsaAlgorithm"` // e.g., "ML-DSA-65"
}

// CRLEntry represents a single revoked ATC in the CRL.
type CRLEntry struct {
	ATCID     string    `json:"atcId"`
	RevokedAt time.Time `json:"revokedAt"`
	Reason    string    `json:"reason"`
}

// CRLResponse is the response from the CRL endpoint.
type CRLResponse struct {
	Version     int        `json:"version"`
	GeneratedAt time.Time  `json:"generatedAt"`
	ExpiresAt   time.Time  `json:"expiresAt"`
	RevokedATCs []CRLEntry `json:"revokedATCs"`
}

// CRLClient fetches and caches the certificate revocation list.
type CRLClient interface {
	// IsRevoked checks whether the given ATC ID is on the CRL.
	IsRevoked(atcID string) (bool, error)

	// Refresh forces a CRL refresh from the endpoint.
	Refresh() error
}

// ATC error codes from spec v1 Section 4.2.
const (
	ErrCodeMalformed          = "atc_malformed"
	ErrCodeOversized          = "atc_oversized"
	ErrCodeExpired            = "atc_expired"
	ErrCodeUntrustedIssuer    = "atc_untrusted_issuer"
	ErrCodeSignatureInvalid   = "atc_signature_invalid"
	ErrCodePQCSigInvalid      = "atc_pqc_signature_invalid"
	ErrCodeRevoked            = "atc_revoked"
	ErrCodeChainInvalid       = "atc_chain_invalid"
	ErrCodeInsufficientScope  = "atc_insufficient_scope"
)

// ATCError is a typed error returned by ATC verification.
type ATCError struct {
	Code    string `json:"error"`
	Message string `json:"message"`
}

func (e *ATCError) Error() string {
	return e.Code + ": " + e.Message
}
