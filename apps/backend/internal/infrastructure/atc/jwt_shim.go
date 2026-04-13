package atc

import (
	"crypto/ed25519"
	"fmt"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	atcdomain "github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/atc"
)

// cacheEntry holds a verified ATCClaims with expiration for CR-007 compliance.
type cacheEntry struct {
	claims    *atcdomain.ATCClaims
	expiresAt time.Time
}

// JWTShimVerifier is a JWT-based implementation of ATCVerifier.
// It uses the server's Ed25519 signing key to issue and verify JWTs that
// stand in for real ATCs until the ATC spec is implemented.
type JWTShimVerifier struct {
	signingKey ed25519.PrivateKey
	publicKey  ed25519.PublicKey
	issuer     string

	// CR-007: cache verified claims to avoid re-parsing on every request
	cache    sync.Map
	cacheTTL time.Duration
}

// NewJWTShimVerifier creates a new JWT-based ATC verifier.
// signingKey is the server's Ed25519 private key (from KeyVault.GetServerSigningKey()).
func NewJWTShimVerifier(signingKey ed25519.PrivateKey, issuer string) *JWTShimVerifier {
	return &JWTShimVerifier{
		signingKey: signingKey,
		publicKey:  signingKey.Public().(ed25519.PublicKey),
		issuer:     issuer,
		cacheTTL:   5 * time.Minute,
	}
}

// atcJWTClaims maps ATC fields into JWT registered + custom claims.
type atcJWTClaims struct {
	jwt.RegisteredClaims
	AgentID      string   `json:"agent_id"`
	Capabilities []string `json:"capabilities"`
	ATCID        string   `json:"atc_id"`
}

// IssueToken creates a JWT that stands in for a real ATC.
// This is used by the secrets service when an agent authenticates via Ed25519
// and needs an ATC-like token for the Resolve flow.
func (v *JWTShimVerifier) IssueToken(agentID uuid.UUID, capabilities []string, ttl time.Duration) (string, error) {
	now := time.Now()
	atcID := uuid.New().String()

	claims := atcJWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    v.issuer,
			Subject:   agentID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			ID:        atcID,
		},
		AgentID:      agentID.String(),
		Capabilities: capabilities,
		ATCID:        atcID,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	return token.SignedString(v.signingKey)
}

// Verify validates a JWT-based ATC token and returns extracted claims.
func (v *JWTShimVerifier) Verify(rawToken string) (*atcdomain.ATCClaims, error) {
	// Check cache first (CR-007)
	if entry, ok := v.cache.Load(rawToken); ok {
		ce := entry.(*cacheEntry)
		if time.Now().Before(ce.expiresAt) {
			return ce.claims, nil
		}
		v.cache.Delete(rawToken)
	}

	token, err := jwt.ParseWithClaims(rawToken, &atcJWTClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodEd25519); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return v.publicKey, nil
	})
	if err != nil {
		return nil, fmt.Errorf("invalid ATC token: %w", err)
	}

	jwtClaims, ok := token.Claims.(*atcJWTClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid ATC claims")
	}

	agentID, err := uuid.Parse(jwtClaims.AgentID)
	if err != nil {
		return nil, fmt.Errorf("invalid agent ID in ATC: %w", err)
	}

	if jwtClaims.Issuer != v.issuer {
		return nil, fmt.Errorf("ATC issuer mismatch: expected %s, got %s", v.issuer, jwtClaims.Issuer)
	}

	claims := &atcdomain.ATCClaims{
		AgentID:      agentID,
		Issuer:       jwtClaims.Issuer,
		Capabilities: jwtClaims.Capabilities,
		IssuedAt:     jwtClaims.IssuedAt.Time,
		ExpiresAt:    jwtClaims.ExpiresAt.Time,
		ATCID:        jwtClaims.ATCID,
	}

	// Cache the verified claims (CR-007)
	v.cache.Store(rawToken, &cacheEntry{
		claims:    claims,
		expiresAt: time.Now().Add(v.cacheTTL),
	})

	return claims, nil
}

// IsRevoked checks whether an ATC has been revoked.
// The JWT shim does not implement CRL — always returns false.
// Real ATC implementation will check against a cached CRL.
func (v *JWTShimVerifier) IsRevoked(atcID string) (bool, error) {
	return false, nil
}
