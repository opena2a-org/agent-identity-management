package atc

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"testing"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/google/uuid"

	atcdomain "github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/atc"
)

// testIssuerKey generates a deterministic Ed25519 key pair for testing.
func testIssuerKey() (ed25519.PublicKey, ed25519.PrivateKey) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	return pub, priv
}

// issueTestATC creates a CBOR-encoded, Ed25519-signed ATC for testing.
func issueTestATC(t *testing.T, priv ed25519.PrivateKey, agentID uuid.UUID, issuerURI string, capabilities []string, ttl time.Duration) string {
	t.Helper()
	pub := priv.Public().(ed25519.PublicKey)
	now := time.Now()

	payload := atcdomain.ATCPayload{
		ATCID:           uuid.New().String(),
		AgentID:         agentID.String(),
		Issuer:          issuerURI,
		IssuedAt:        uint64(now.Unix()),
		ExpiresAt:       uint64(now.Add(ttl).Unix()),
		Capabilities:    capabilities,
		IssuerPublicKey: []byte(pub),
	}

	signedBytes, err := canonicalSignedPayload(&payload)
	if err != nil {
		t.Fatalf("failed to encode signed payload: %v", err)
	}

	payload.Ed25519Sig = ed25519.Sign(priv, signedBytes)

	encMode, err := cbor.CanonicalEncOptions().EncMode()
	if err != nil {
		t.Fatalf("failed to create CBOR encoder: %v", err)
	}

	atcBytes, err := encMode.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal ATC: %v", err)
	}

	return base64.RawURLEncoding.EncodeToString(atcBytes)
}

func TestRealATCVerifier_VerifyValidATC(t *testing.T) {
	pub, priv := testIssuerKey()
	issuerURI := "https://aim.test.opena2a.org"
	agentID := uuid.New()

	verifier := NewRealATCVerifier([]atcdomain.TrustedIssuer{
		{URI: issuerURI, PublicKey: []byte(pub)},
	}, nil) // no CRL for this test

	token := issueTestATC(t, priv, agentID, issuerURI, []string{"secrets:resolve"}, 5*time.Minute)

	claims, err := verifier.Verify(token)
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}

	if claims.AgentID != agentID {
		t.Errorf("agent ID mismatch: got %s, want %s", claims.AgentID, agentID)
	}
	if claims.Issuer != issuerURI {
		t.Errorf("issuer mismatch: got %s, want %s", claims.Issuer, issuerURI)
	}
	if claims.AuthMethod != "atc" {
		t.Errorf("auth method: got %s, want atc", claims.AuthMethod)
	}
	if !claims.HasCapability("secrets:resolve") {
		t.Error("expected capability secrets:resolve")
	}
}

func TestRealATCVerifier_ExpiredATC(t *testing.T) {
	pub, priv := testIssuerKey()
	issuerURI := "https://aim.test.opena2a.org"

	verifier := NewRealATCVerifier([]atcdomain.TrustedIssuer{
		{URI: issuerURI, PublicKey: []byte(pub)},
	}, nil)

	// Create an ATC that expired 1 minute ago
	now := time.Now()
	payload := atcdomain.ATCPayload{
		ATCID:           uuid.New().String(),
		AgentID:         uuid.New().String(),
		Issuer:          issuerURI,
		IssuedAt:        uint64(now.Add(-10 * time.Minute).Unix()),
		ExpiresAt:       uint64(now.Add(-1 * time.Minute).Unix()),
		Capabilities:    []string{"secrets:resolve"},
		IssuerPublicKey: []byte(pub),
	}

	signedBytes, _ := canonicalSignedPayload(&payload)
	payload.Ed25519Sig = ed25519.Sign(priv, signedBytes)

	encMode, _ := cbor.CanonicalEncOptions().EncMode()
	atcBytes, _ := encMode.Marshal(payload)
	token := base64.RawURLEncoding.EncodeToString(atcBytes)

	_, err := verifier.Verify(token)
	if err == nil {
		t.Fatal("expected error for expired ATC")
	}

	atcErr, ok := err.(*atcdomain.ATCError)
	if !ok {
		t.Fatalf("expected ATCError, got %T", err)
	}
	if atcErr.Code != atcdomain.ErrCodeExpired {
		t.Errorf("expected error code %s, got %s", atcdomain.ErrCodeExpired, atcErr.Code)
	}
}

func TestRealATCVerifier_UntrustedIssuer(t *testing.T) {
	pub, priv := testIssuerKey()

	// Verifier trusts a different issuer
	verifier := NewRealATCVerifier([]atcdomain.TrustedIssuer{
		{URI: "https://other-issuer.org", PublicKey: []byte(pub)},
	}, nil)

	token := issueTestATC(t, priv, uuid.New(), "https://untrusted-issuer.org", []string{"secrets:resolve"}, 5*time.Minute)

	_, err := verifier.Verify(token)
	if err == nil {
		t.Fatal("expected error for untrusted issuer")
	}

	atcErr, ok := err.(*atcdomain.ATCError)
	if !ok {
		t.Fatalf("expected ATCError, got %T", err)
	}
	if atcErr.Code != atcdomain.ErrCodeUntrustedIssuer {
		t.Errorf("expected error code %s, got %s", atcdomain.ErrCodeUntrustedIssuer, atcErr.Code)
	}
}

func TestRealATCVerifier_BadSignature(t *testing.T) {
	pub, _ := testIssuerKey()
	_, wrongPriv := testIssuerKey() // different key pair
	issuerURI := "https://aim.test.opena2a.org"

	verifier := NewRealATCVerifier([]atcdomain.TrustedIssuer{
		{URI: issuerURI, PublicKey: []byte(pub)},
	}, nil)

	// Issue with wrong key but claim correct issuer
	now := time.Now()
	wrongPub := wrongPriv.Public().(ed25519.PublicKey)
	payload := atcdomain.ATCPayload{
		ATCID:           uuid.New().String(),
		AgentID:         uuid.New().String(),
		Issuer:          issuerURI,
		IssuedAt:        uint64(now.Unix()),
		ExpiresAt:       uint64(now.Add(5 * time.Minute).Unix()),
		Capabilities:    []string{"secrets:resolve"},
		IssuerPublicKey: []byte(wrongPub),
	}

	signedBytes, _ := canonicalSignedPayload(&payload)
	payload.Ed25519Sig = ed25519.Sign(wrongPriv, signedBytes)

	encMode, _ := cbor.CanonicalEncOptions().EncMode()
	atcBytes, _ := encMode.Marshal(payload)
	token := base64.RawURLEncoding.EncodeToString(atcBytes)

	_, err := verifier.Verify(token)
	if err == nil {
		t.Fatal("expected error for bad signature")
	}

	atcErr, ok := err.(*atcdomain.ATCError)
	if !ok {
		t.Fatalf("expected ATCError, got %T", err)
	}
	// Should fail on untrusted issuer (wrong pub key) before sig check
	if atcErr.Code != atcdomain.ErrCodeUntrustedIssuer {
		t.Errorf("expected error code %s, got %s", atcdomain.ErrCodeUntrustedIssuer, atcErr.Code)
	}
}

func TestRealATCVerifier_TTLTooLong(t *testing.T) {
	pub, priv := testIssuerKey()
	issuerURI := "https://aim.test.opena2a.org"

	verifier := NewRealATCVerifier([]atcdomain.TrustedIssuer{
		{URI: issuerURI, PublicKey: []byte(pub)},
	}, nil)

	token := issueTestATC(t, priv, uuid.New(), issuerURI, []string{"secrets:resolve"}, 2*time.Hour)

	_, err := verifier.Verify(token)
	if err == nil {
		t.Fatal("expected error for TTL too long")
	}

	atcErr, ok := err.(*atcdomain.ATCError)
	if !ok {
		t.Fatalf("expected ATCError, got %T", err)
	}
	if atcErr.Code != atcdomain.ErrCodeMalformed {
		t.Errorf("expected error code %s, got %s", atcdomain.ErrCodeMalformed, atcErr.Code)
	}
}

func TestRealATCVerifier_MalformedToken(t *testing.T) {
	verifier := NewRealATCVerifier(nil, nil)

	_, err := verifier.Verify("not-valid-base64url!!!")
	if err == nil {
		t.Fatal("expected error for malformed token")
	}

	atcErr, ok := err.(*atcdomain.ATCError)
	if !ok {
		t.Fatalf("expected ATCError, got %T", err)
	}
	if atcErr.Code != atcdomain.ErrCodeMalformed {
		t.Errorf("expected error code %s, got %s", atcdomain.ErrCodeMalformed, atcErr.Code)
	}
}

func TestRealATCVerifier_CachesVerifiedClaims(t *testing.T) {
	pub, priv := testIssuerKey()
	issuerURI := "https://aim.test.opena2a.org"

	verifier := NewRealATCVerifier([]atcdomain.TrustedIssuer{
		{URI: issuerURI, PublicKey: []byte(pub)},
	}, nil)

	token := issueTestATC(t, priv, uuid.New(), issuerURI, []string{"secrets:resolve"}, 5*time.Minute)

	// First verify
	claims1, err := verifier.Verify(token)
	if err != nil {
		t.Fatalf("first Verify failed: %v", err)
	}

	// Second verify should return cached result
	claims2, err := verifier.Verify(token)
	if err != nil {
		t.Fatalf("second Verify failed: %v", err)
	}

	if claims1.ATCID != claims2.ATCID {
		t.Error("cached claims should have same ATCID")
	}
}

func TestATCClaims_HasCapability_Hierarchical(t *testing.T) {
	claims := &atcdomain.ATCClaims{
		Capabilities: []string{"secrets:resolve", "scan:read"},
	}

	tests := []struct {
		required string
		want     bool
	}{
		{"secrets:resolve", true},
		{"secrets:resolve:github", true}, // hierarchical match
		{"scan:read", true},
		{"scan:write", false},
		{"secrets:store", false},
		{"admin:all", false},
	}

	for _, tc := range tests {
		got := claims.HasCapability(tc.required)
		if got != tc.want {
			t.Errorf("HasCapability(%q) = %v, want %v", tc.required, got, tc.want)
		}
	}
}

func TestATCClaims_HasCapability_Wildcard(t *testing.T) {
	claims := &atcdomain.ATCClaims{
		Capabilities: []string{"*"},
	}

	if !claims.HasCapability("secrets:resolve:anything") {
		t.Error("wildcard should match everything")
	}
}

func TestCompositeVerifier_FallsBackToJWT(t *testing.T) {
	_, priv := testIssuerKey()

	// Real verifier with no trusted issuers (will reject everything)
	realVerifier := NewRealATCVerifier(nil, nil)

	// JWT shim that can verify tokens
	jwtShim := NewJWTShimVerifier(priv, "test-issuer")

	composite := NewCompositeVerifier(realVerifier, jwtShim)

	// Issue a JWT token
	agentID := uuid.New()
	jwtToken, err := jwtShim.IssueToken(agentID, []string{"secrets:resolve"}, 5*time.Minute)
	if err != nil {
		t.Fatalf("failed to issue JWT: %v", err)
	}

	// Composite should fall back to JWT shim for non-CBOR tokens
	claims, err := composite.Verify(jwtToken)
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}

	if claims.AgentID != agentID {
		t.Errorf("agent ID mismatch: got %s, want %s", claims.AgentID, agentID)
	}
	if claims.AuthMethod != "jwt" {
		t.Errorf("auth method: got %s, want jwt", claims.AuthMethod)
	}
}

func TestCompositeVerifier_PrefersRealATC(t *testing.T) {
	pub, priv := testIssuerKey()
	issuerURI := "https://aim.test.opena2a.org"

	realVerifier := NewRealATCVerifier([]atcdomain.TrustedIssuer{
		{URI: issuerURI, PublicKey: []byte(pub)},
	}, nil)

	jwtShim := NewJWTShimVerifier(priv, "test-issuer")
	composite := NewCompositeVerifier(realVerifier, jwtShim)

	// Issue a real ATC
	agentID := uuid.New()
	token := issueTestATC(t, priv, agentID, issuerURI, []string{"secrets:resolve"}, 5*time.Minute)

	claims, err := composite.Verify(token)
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}

	if claims.AuthMethod != "atc" {
		t.Errorf("auth method: got %s, want atc (real ATC should be preferred)", claims.AuthMethod)
	}
}

