package atc

import (
	"crypto/ed25519"
	"fmt"

	atcdomain "github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/atc"
)

// NewServerATCVerifier builds the verifier the server checks agent ATCs with.
// It accepts CBOR-encoded ATCs from one trusted issuer, issuerURI, signed with
// the server signing key. Any other token format is refused as malformed.
func NewServerATCVerifier(issuerURI string, serverSigningKey ed25519.PrivateKey, crlClient *CachedCRLClient) (*RealATCVerifier, error) {
	if len(serverSigningKey) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("server signing key is not a valid Ed25519 key")
	}
	issuerPubKey, ok := serverSigningKey.Public().(ed25519.PublicKey)
	if !ok || len(issuerPubKey) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("server signing key is not a valid Ed25519 public key")
	}
	// Defensive copy: isolate from any future mutation of the source key.
	issuerPubKeyCopy := make([]byte, ed25519.PublicKeySize)
	copy(issuerPubKeyCopy, issuerPubKey)
	return NewRealATCVerifier([]atcdomain.TrustedIssuer{
		{URI: issuerURI, PublicKey: issuerPubKeyCopy},
	}, crlClient), nil
}
