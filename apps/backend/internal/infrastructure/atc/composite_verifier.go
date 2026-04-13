package atc

import (
	atcdomain "github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/atc"
)

// CompositeVerifier tries the real ATC verifier first, then falls back to the JWT shim.
// This enables migration from JWT-based ATCs to real CBOR-encoded ATCs.
// Implements atcdomain.ATCVerifier.
type CompositeVerifier struct {
	real    *RealATCVerifier
	jwtShim *JWTShimVerifier
}

// NewCompositeVerifier creates a verifier that tries real ATC first, then JWT shim.
func NewCompositeVerifier(real *RealATCVerifier, jwtShim *JWTShimVerifier) *CompositeVerifier {
	return &CompositeVerifier{real: real, jwtShim: jwtShim}
}

// Verify tries real ATC verification first. If the token is not valid CBOR (malformed),
// falls back to the JWT shim. Other errors (expired, revoked, bad sig) are returned as-is.
func (cv *CompositeVerifier) Verify(rawToken string) (*atcdomain.ATCClaims, error) {
	claims, err := cv.real.Verify(rawToken)
	if err == nil {
		return claims, nil
	}

	// Only fall back to JWT if the real verifier says "malformed" (not a CBOR ATC)
	if atcErr, ok := err.(*atcdomain.ATCError); ok && atcErr.Code == atcdomain.ErrCodeMalformed {
		return cv.jwtShim.Verify(rawToken)
	}

	return nil, err
}

// IsRevoked checks the CRL via the real verifier.
func (cv *CompositeVerifier) IsRevoked(atcID string) (bool, error) {
	return cv.real.IsRevoked(atcID)
}
