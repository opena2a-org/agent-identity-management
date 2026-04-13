package atc

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"sync"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/google/uuid"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto/pqc"
	atcdomain "github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/atc"
)

// cacheEntry holds a verified ATCClaims with expiration.
type realCacheEntry struct {
	claims    *atcdomain.ATCClaims
	expiresAt time.Time
}

// RealATCVerifier verifies CBOR-encoded Agent Trust Certificates per spec v1.
// Implements atcdomain.ATCVerifier.
type RealATCVerifier struct {
	trustedIssuers map[string]*atcdomain.TrustedIssuer // keyed by issuer URI
	crlClient      *CachedCRLClient

	cache    sync.Map
	cacheTTL time.Duration
}

// NewRealATCVerifier creates a verifier with the given trusted issuers and CRL client.
func NewRealATCVerifier(issuers []atcdomain.TrustedIssuer, crlClient *CachedCRLClient) *RealATCVerifier {
	issuerMap := make(map[string]*atcdomain.TrustedIssuer, len(issuers))
	for i := range issuers {
		issuerMap[issuers[i].URI] = &issuers[i]
	}
	return &RealATCVerifier{
		trustedIssuers: issuerMap,
		crlClient:      crlClient,
		cacheTTL:       5 * time.Minute,
	}
}

// Verify validates a base64url-encoded ATC token and returns extracted claims.
// Implements the 10-step verification algorithm from spec v1 Section 4.1.
func (v *RealATCVerifier) Verify(rawToken string) (*atcdomain.ATCClaims, error) {
	// Check cache first (CR-007: avoid re-parsing)
	if entry, ok := v.cache.Load(rawToken); ok {
		ce := entry.(*realCacheEntry)
		if time.Now().Before(ce.expiresAt) {
			if ce.claims == nil {
				// Negative cache entry — previously failed verification
				return nil, &atcdomain.ATCError{Code: atcdomain.ErrCodeMalformed, Message: "previously rejected ATC"}
			}
			return ce.claims, nil
		}
		v.cache.Delete(rawToken)
	}

	// Step 0: INPUT SIZE — reject oversized base64url before decoding to prevent allocation DoS.
	// Base64 expands ~4/3x, so MaxATCSize * 2 is a safe upper bound for encoded form.
	if len(rawToken) > atcdomain.MaxATCSize*2 {
		return nil, &atcdomain.ATCError{Code: atcdomain.ErrCodeOversized, Message: fmt.Sprintf("encoded ATC size %d exceeds limit", len(rawToken))}
	}

	// Step 1: DECODE — base64url decode
	raw, err := base64.RawURLEncoding.DecodeString(rawToken)
	if err != nil {
		// Negative cache: prevent CPU DoS from repeated malformed tokens (short TTL for transient errors)
		v.cache.Store(rawToken, &realCacheEntry{claims: nil, expiresAt: time.Now().Add(5 * time.Second)})
		return nil, &atcdomain.ATCError{Code: atcdomain.ErrCodeMalformed, Message: "base64url decode failed"}
	}

	// Step 3: SIZE — check before parsing
	if len(raw) > atcdomain.MaxATCSize {
		return nil, &atcdomain.ATCError{Code: atcdomain.ErrCodeOversized, Message: fmt.Sprintf("ATC size %d exceeds %d limit", len(raw), atcdomain.MaxATCSize)}
	}

	// Step 2: PARSE — CBOR decode
	var payload atcdomain.ATCPayload
	if err := cbor.Unmarshal(raw, &payload); err != nil {
		return nil, &atcdomain.ATCError{Code: atcdomain.ErrCodeMalformed, Message: "CBOR decode failed: " + err.Error()}
	}

	// Validate required fields
	if payload.ATCID == "" || payload.AgentID == "" || payload.Issuer == "" || payload.IssuerPublicKey == nil || payload.Ed25519Sig == nil {
		return nil, &atcdomain.ATCError{Code: atcdomain.ErrCodeMalformed, Message: "missing required ATC fields"}
	}

	if len(payload.Capabilities) > atcdomain.MaxCapabilities {
		return nil, &atcdomain.ATCError{Code: atcdomain.ErrCodeMalformed, Message: fmt.Sprintf("too many capabilities: %d > %d", len(payload.Capabilities), atcdomain.MaxCapabilities)}
	}

	for _, cap := range payload.Capabilities {
		if len(cap) > atcdomain.MaxCapabilityLen {
			return nil, &atcdomain.ATCError{Code: atcdomain.ErrCodeMalformed, Message: fmt.Sprintf("capability too long: %d > %d", len(cap), atcdomain.MaxCapabilityLen)}
		}
	}

	if len(payload.DelegationChain) > atcdomain.MaxDelegationChainDepth {
		return nil, &atcdomain.ATCError{Code: atcdomain.ErrCodeMalformed, Message: fmt.Sprintf("delegation chain too deep: %d > %d", len(payload.DelegationChain), atcdomain.MaxDelegationChainDepth)}
	}

	// Step 4: EXPIRY
	now := time.Now()
	issuedAt := time.Unix(int64(payload.IssuedAt), 0)
	expiresAt := time.Unix(int64(payload.ExpiresAt), 0)

	if now.After(expiresAt.Add(atcdomain.MaxClockSkew)) {
		return nil, &atcdomain.ATCError{Code: atcdomain.ErrCodeExpired, Message: fmt.Sprintf("ATC expired at %s", expiresAt.Format(time.RFC3339))}
	}
	if now.Before(issuedAt.Add(-atcdomain.MaxClockSkew)) {
		return nil, &atcdomain.ATCError{Code: atcdomain.ErrCodeExpired, Message: fmt.Sprintf("ATC not yet valid, issued at %s", issuedAt.Format(time.RFC3339))}
	}

	ttl := expiresAt.Sub(issuedAt)
	if ttl > atcdomain.MaxTTL {
		return nil, &atcdomain.ATCError{Code: atcdomain.ErrCodeMalformed, Message: fmt.Sprintf("ATC TTL %s exceeds maximum %s", ttl, atcdomain.MaxTTL)}
	}

	// Step 5: ISSUER — check trusted issuers list
	issuer, ok := v.trustedIssuers[payload.Issuer]
	if !ok {
		return nil, &atcdomain.ATCError{Code: atcdomain.ErrCodeUntrustedIssuer, Message: fmt.Sprintf("issuer %s not trusted", payload.Issuer)}
	}

	// Verify issuer public key matches
	if len(payload.IssuerPublicKey) != ed25519.PublicKeySize {
		return nil, &atcdomain.ATCError{Code: atcdomain.ErrCodeSignatureInvalid, Message: "invalid issuer public key size"}
	}
	if !ed25519KeysEqual(payload.IssuerPublicKey, issuer.PublicKey) {
		return nil, &atcdomain.ATCError{Code: atcdomain.ErrCodeUntrustedIssuer, Message: "issuer public key does not match trusted key"}
	}

	// Step 6: ED25519 — verify signature over canonical fields 1-7
	signedBytes, err := canonicalSignedPayload(&payload)
	if err != nil {
		return nil, &atcdomain.ATCError{Code: atcdomain.ErrCodeMalformed, Message: "failed to encode signed payload: " + err.Error()}
	}

	if !ed25519.Verify(ed25519.PublicKey(payload.IssuerPublicKey), signedBytes, payload.Ed25519Sig) {
		return nil, &atcdomain.ATCError{Code: atcdomain.ErrCodeSignatureInvalid, Message: "Ed25519 signature verification failed"}
	}

	// Step 7: ML-DSA — verify PQC signature if present
	if len(payload.MLDSASig) > 0 {
		if len(payload.MLDSAPublicKey) == 0 {
			return nil, &atcdomain.ATCError{Code: atcdomain.ErrCodePQCSigInvalid, Message: "ML-DSA signature present but no public key"}
		}
		if payload.PQCAlgorithm == "" {
			return nil, &atcdomain.ATCError{Code: atcdomain.ErrCodePQCSigInvalid, Message: "ML-DSA signature present but no algorithm specified"}
		}

		alg := pqc.Algorithm(payload.PQCAlgorithm)
		if !pqc.IsValidAlgorithm(alg) || !pqc.IsPQCAlgorithm(alg) {
			return nil, &atcdomain.ATCError{Code: atcdomain.ErrCodePQCSigInvalid, Message: fmt.Sprintf("unsupported PQC algorithm: %s", payload.PQCAlgorithm)}
		}

		if err := pqc.VerifyMLDSA(alg, payload.MLDSAPublicKey, signedBytes, payload.MLDSASig); err != nil {
			return nil, &atcdomain.ATCError{Code: atcdomain.ErrCodePQCSigInvalid, Message: "ML-DSA signature verification failed: " + err.Error()}
		}
	}

	// Step 8: CRL — check revocation
	if v.crlClient != nil {
		revoked, err := v.crlClient.IsRevoked(payload.ATCID)
		if err != nil {
			return nil, &atcdomain.ATCError{Code: atcdomain.ErrCodeRevoked, Message: "CRL check failed: " + err.Error()}
		}
		if revoked {
			return nil, &atcdomain.ATCError{Code: atcdomain.ErrCodeRevoked, Message: fmt.Sprintf("ATC %s has been revoked", payload.ATCID)}
		}
	}

	// Step 9: CHAIN — verify delegation chain if present
	if len(payload.DelegationChain) > 0 {
		if err := v.verifyDelegationChain(&payload); err != nil {
			return nil, err
		}
	}

	// Build claims (Step 10: CAPABILITY is checked by caller)
	agentID, err := uuid.Parse(payload.AgentID)
	if err != nil {
		return nil, &atcdomain.ATCError{Code: atcdomain.ErrCodeMalformed, Message: "invalid agent ID: " + err.Error()}
	}

	claims := &atcdomain.ATCClaims{
		AgentID:         agentID,
		Issuer:          payload.Issuer,
		Capabilities:    payload.Capabilities,
		IssuedAt:        issuedAt,
		ExpiresAt:       expiresAt,
		ATCID:           payload.ATCID,
		IssuerPublicKey: payload.IssuerPublicKey,
		PQCAlgorithm:    payload.PQCAlgorithm,
		AuthMethod:      "atc",
	}

	if len(payload.DelegationChain) > 0 {
		claims.DelegationChain = make([]atcdomain.DelegationEntry, len(payload.DelegationChain))
		for i, ce := range payload.DelegationChain {
			claims.DelegationChain[i] = atcdomain.DelegationEntry{
				IssuerURI:       ce.IssuerURI,
				IssuerPublicKey: ce.IssuerPublicKey,
				Signature:       ce.Signature,
			}
		}
	}

	// Cache verified claims
	v.cache.Store(rawToken, &realCacheEntry{
		claims:    claims,
		expiresAt: time.Now().Add(v.cacheTTL),
	})

	return claims, nil
}

// IsRevoked checks whether an ATC has been revoked via CRL.
func (v *RealATCVerifier) IsRevoked(atcID string) (bool, error) {
	if v.crlClient == nil {
		return false, nil
	}
	return v.crlClient.IsRevoked(atcID)
}

// canonicalSignedPayload encodes fields 1-7 as deterministic CBOR.
func canonicalSignedPayload(p *atcdomain.ATCPayload) ([]byte, error) {
	sp := atcdomain.SignedPayload{
		ATCID:           p.ATCID,
		AgentID:         p.AgentID,
		Issuer:          p.Issuer,
		IssuedAt:        p.IssuedAt,
		ExpiresAt:       p.ExpiresAt,
		Capabilities:    p.Capabilities,
		IssuerPublicKey: p.IssuerPublicKey,
	}

	encMode, err := cbor.CanonicalEncOptions().EncMode()
	if err != nil {
		return nil, err
	}
	return encMode.Marshal(sp)
}

// verifyDelegationChain verifies each link in the delegation chain.
func (v *RealATCVerifier) verifyDelegationChain(payload *atcdomain.ATCPayload) *atcdomain.ATCError {
	for i, entry := range payload.DelegationChain {
		if len(entry.IssuerPublicKey) != ed25519.PublicKeySize {
			return &atcdomain.ATCError{Code: atcdomain.ErrCodeChainInvalid, Message: fmt.Sprintf("chain entry %d: invalid public key size", i)}
		}

		// Each chain entry's signature covers the next entry (or the ATC payload for the last entry)
		var signedData []byte
		if i < len(payload.DelegationChain)-1 {
			next := payload.DelegationChain[i+1]
			encMode, err := cbor.CanonicalEncOptions().EncMode()
			if err != nil {
				return &atcdomain.ATCError{Code: atcdomain.ErrCodeChainInvalid, Message: fmt.Sprintf("chain entry %d: failed to create canonical encoder", i)}
			}
			signedData, err = encMode.Marshal(next)
			if err != nil {
				return &atcdomain.ATCError{Code: atcdomain.ErrCodeChainInvalid, Message: fmt.Sprintf("chain entry %d: failed to encode next entry", i)}
			}
		} else {
			// Last entry signs the ATC signed payload
			var err error
			signedData, err = canonicalSignedPayload(payload)
			if err != nil {
				return &atcdomain.ATCError{Code: atcdomain.ErrCodeChainInvalid, Message: fmt.Sprintf("chain entry %d: failed to encode signed payload", i)}
			}
		}

		if !ed25519.Verify(ed25519.PublicKey(entry.IssuerPublicKey), signedData, entry.Signature) {
			return &atcdomain.ATCError{Code: atcdomain.ErrCodeChainInvalid, Message: fmt.Sprintf("chain entry %d: signature verification failed", i)}
		}

		// Verify chain entry issuer is trusted (root of chain must be trusted)
		if i == 0 {
			if _, ok := v.trustedIssuers[entry.IssuerURI]; !ok {
				return &atcdomain.ATCError{Code: atcdomain.ErrCodeChainInvalid, Message: fmt.Sprintf("chain root issuer %s is not trusted", entry.IssuerURI)}
			}
		}
	}

	return nil
}

// ed25519KeysEqual compares two Ed25519 public keys in constant time.
func ed25519KeysEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var diff byte
	for i := range a {
		diff |= a[i] ^ b[i]
	}
	return diff == 0
}
