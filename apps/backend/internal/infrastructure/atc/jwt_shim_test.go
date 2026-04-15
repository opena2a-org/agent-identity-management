package atc

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"

	"github.com/google/uuid"
)

func testSigningKey(t *testing.T) ed25519.PrivateKey {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return priv
}

func TestJWTShimVerifier_IssueAndVerify(t *testing.T) {
	key := testSigningKey(t)
	verifier := NewJWTShimVerifier(key, "aim-test")

	agentID := uuid.New()
	caps := []string{"secrets:resolve", "file:read"}

	token, err := verifier.IssueToken(agentID, caps, 5*time.Minute)
	if err != nil {
		t.Fatalf("IssueToken failed: %v", err)
	}

	claims, err := verifier.Verify(token)
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}

	if claims.AgentID != agentID {
		t.Errorf("AgentID = %s, want %s", claims.AgentID, agentID)
	}
	if claims.Issuer != "aim-test" {
		t.Errorf("Issuer = %s, want aim-test", claims.Issuer)
	}
	if len(claims.Capabilities) != 2 {
		t.Errorf("Capabilities len = %d, want 2", len(claims.Capabilities))
	}
	if claims.IsExpired() {
		t.Error("Claims should not be expired")
	}
	if !claims.HasCapability("secrets:resolve") {
		t.Error("Claims should have secrets:resolve capability")
	}
	if claims.HasCapability("nonexistent") {
		t.Error("Claims should not have nonexistent capability")
	}
}

func TestJWTShimVerifier_ExpiredToken(t *testing.T) {
	key := testSigningKey(t)
	verifier := NewJWTShimVerifier(key, "aim-test")

	token, err := verifier.IssueToken(uuid.New(), nil, -1*time.Second)
	if err != nil {
		t.Fatalf("IssueToken failed: %v", err)
	}

	_, err = verifier.Verify(token)
	if err == nil {
		t.Fatal("Verify should fail for expired token")
	}
}

func TestJWTShimVerifier_WrongIssuer(t *testing.T) {
	key := testSigningKey(t)
	issuer := NewJWTShimVerifier(key, "issuer-a")
	verifierB := NewJWTShimVerifier(key, "issuer-b")

	token, _ := issuer.IssueToken(uuid.New(), nil, 5*time.Minute)
	_, err := verifierB.Verify(token)
	if err == nil {
		t.Fatal("Verify should fail for wrong issuer")
	}
}

func TestJWTShimVerifier_WrongKey(t *testing.T) {
	key1 := testSigningKey(t)
	key2 := testSigningKey(t)
	issuer := NewJWTShimVerifier(key1, "aim-test")
	verifier := NewJWTShimVerifier(key2, "aim-test")

	token, _ := issuer.IssueToken(uuid.New(), nil, 5*time.Minute)
	_, err := verifier.Verify(token)
	if err == nil {
		t.Fatal("Verify should fail with wrong signing key")
	}
}

func TestJWTShimVerifier_CachesVerifiedClaims(t *testing.T) {
	key := testSigningKey(t)
	verifier := NewJWTShimVerifier(key, "aim-test")

	agentID := uuid.New()
	token, _ := verifier.IssueToken(agentID, []string{"secrets:resolve"}, 5*time.Minute)

	// First verify
	claims1, err := verifier.Verify(token)
	if err != nil {
		t.Fatal(err)
	}

	// Second verify should return cached
	claims2, err := verifier.Verify(token)
	if err != nil {
		t.Fatal(err)
	}

	if claims1.ATCID != claims2.ATCID {
		t.Error("Cached claims should have same ATCID")
	}
}

func TestJWTShimVerifier_IsRevokedAlwaysFalse(t *testing.T) {
	key := testSigningKey(t)
	verifier := NewJWTShimVerifier(key, "aim-test")

	revoked, err := verifier.IsRevoked("any-id")
	if err != nil {
		t.Fatal(err)
	}
	if revoked {
		t.Error("JWT shim should never report revoked")
	}
}
