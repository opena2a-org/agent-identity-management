package atc

import (
	"crypto/ed25519"
	"fmt"

	atcdomain "github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/atc"
)

// NewServerATCVerifier builds the verifier the server checks agent ATCs with.
// It accepts CBOR-encoded ATCs from one trusted issuer, issuerURI, signed with
// issuerKey or with one of retiredKeys, the issuer's earlier keys. issuerKey is
// the server's ATC issuer key, which signs nothing else. Any other token format
// is refused as malformed.
func NewServerATCVerifier(issuerURI string, issuerKey ed25519.PrivateKey, crlClient *CachedCRLClient, retiredKeys ...ed25519.PublicKey) (*RealATCVerifier, error) {
	if len(issuerKey) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("ATC issuer key is not a valid Ed25519 key")
	}
	issuerPubKey, ok := issuerKey.Public().(ed25519.PublicKey)
	if !ok || len(issuerPubKey) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("ATC issuer key is not a valid Ed25519 public key")
	}
	// Defensive copy: isolate from any future mutation of the source key.
	issuerPubKeyCopy := make([]byte, ed25519.PublicKeySize)
	copy(issuerPubKeyCopy, issuerPubKey)
	var retired [][]byte
	for _, key := range retiredKeys {
		if len(key) != ed25519.PublicKeySize {
			return nil, fmt.Errorf("retired ATC issuer key is not a valid Ed25519 public key")
		}
		retired = append(retired, append([]byte(nil), key...))
	}
	return NewRealATCVerifier([]atcdomain.TrustedIssuer{
		{URI: issuerURI, PublicKey: issuerPubKeyCopy, RetiredPublicKeys: retired},
	}, crlClient), nil
}
