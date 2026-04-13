package atc

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/google/uuid"

	atcdomain "github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/atc"
)

// RealATCIssuer creates CBOR-encoded Agent Trust Certificates signed with Ed25519.
// Implements atcdomain.ATCIssuer.
type RealATCIssuer struct {
	signingKey      ed25519.PrivateKey
	publicKey       ed25519.PublicKey
	issuerURI       string
}

// NewRealATCIssuer creates an issuer using the given Ed25519 signing key.
func NewRealATCIssuer(signingKey ed25519.PrivateKey, issuerURI string) *RealATCIssuer {
	return &RealATCIssuer{
		signingKey: signingKey,
		publicKey:  signingKey.Public().(ed25519.PublicKey),
		issuerURI:  issuerURI,
	}
}

// Issue creates a new ATC for the given agent.
// Returns a base64url-encoded ATC token (no padding).
func (iss *RealATCIssuer) Issue(agentID uuid.UUID, capabilities []string, ttl time.Duration) (string, error) {
	if ttl > atcdomain.MaxTTL {
		return "", fmt.Errorf("requested TTL %s exceeds maximum %s", ttl, atcdomain.MaxTTL)
	}

	if len(capabilities) > atcdomain.MaxCapabilities {
		return "", fmt.Errorf("too many capabilities: %d > %d", len(capabilities), atcdomain.MaxCapabilities)
	}

	now := time.Now()
	atcID := uuid.New().String()

	payload := atcdomain.ATCPayload{
		ATCID:           atcID,
		AgentID:         agentID.String(),
		Issuer:          iss.issuerURI,
		IssuedAt:        uint64(now.Unix()),
		ExpiresAt:       uint64(now.Add(ttl).Unix()),
		Capabilities:    capabilities,
		IssuerPublicKey: []byte(iss.publicKey),
	}

	// Encode the signed payload (fields 1-7) in canonical CBOR
	signedBytes, err := canonicalSignedPayload(&payload)
	if err != nil {
		return "", fmt.Errorf("failed to encode signed payload: %w", err)
	}

	// Sign with Ed25519
	payload.Ed25519Sig = ed25519.Sign(iss.signingKey, signedBytes)

	// Encode the full ATC payload
	encMode, err := cbor.CanonicalEncOptions().EncMode()
	if err != nil {
		return "", fmt.Errorf("failed to create CBOR encoder: %w", err)
	}

	atcBytes, err := encMode.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to encode ATC: %w", err)
	}

	if len(atcBytes) > atcdomain.MaxATCSize {
		return "", fmt.Errorf("encoded ATC size %d exceeds %d limit", len(atcBytes), atcdomain.MaxATCSize)
	}

	return base64.RawURLEncoding.EncodeToString(atcBytes), nil
}
