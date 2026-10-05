package atc

import (
	"crypto/ed25519"
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	atcdomain "github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/atc"
)

// jwtFormatATC mints a token in the retired JWT ATC format: an EdDSA JWT signed
// with the server signing key, issuer "aim-server", carrying agent_id, atc_id
// and capabilities claims. Nothing issues this format any more.
func jwtFormatATC(t *testing.T, serverKey ed25519.PrivateKey, agentID uuid.UUID) string {
	t.Helper()
	now := time.Now()
	atcID := uuid.New().String()
	token, err := jwt.NewWithClaims(jwt.SigningMethodEdDSA, jwt.MapClaims{
		"iss":          "aim-server",
		"sub":          agentID.String(),
		"iat":          now.Unix(),
		"exp":          now.Add(5 * time.Minute).Unix(),
		"jti":          atcID,
		"agent_id":     agentID.String(),
		"capabilities": []string{"secrets:resolve"},
		"atc_id":       atcID,
	}).SignedString(serverKey)
	if err != nil {
		t.Fatalf("failed to sign JWT-format token: %v", err)
	}
	return token
}

func TestServerATCVerifier_RefusesJWTFormatToken(t *testing.T) {
	_, serverKey := testIssuerKey()
	verifier, err := NewServerATCVerifier("https://aim.test.opena2a.org", serverKey, nil)
	if err != nil {
		t.Fatalf("NewServerATCVerifier: %v", err)
	}

	claims, err := verifier.Verify(jwtFormatATC(t, serverKey, uuid.New()))
	if err == nil {
		t.Fatalf("a JWT-format token signed with the server key was accepted (authMethod %q); only CBOR ATCs may be accepted", claims.AuthMethod)
	}
	var atcErr *atcdomain.ATCError
	if !errors.As(err, &atcErr) || atcErr.Code != atcdomain.ErrCodeMalformed {
		t.Fatalf("expected %s, got %v", atcdomain.ErrCodeMalformed, err)
	}
}

func TestServerATCVerifier_AcceptsATCSignedWithServerKey(t *testing.T) {
	_, serverKey := testIssuerKey()
	issuerURI := "https://aim.test.opena2a.org"
	verifier, err := NewServerATCVerifier(issuerURI, serverKey, nil)
	if err != nil {
		t.Fatalf("NewServerATCVerifier: %v", err)
	}

	agentID := uuid.New()
	claims, err := verifier.Verify(issueTestATC(t, serverKey, agentID, issuerURI, []string{"secrets:resolve"}, 5*time.Minute))
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if claims.AgentID != agentID {
		t.Errorf("agent ID mismatch: got %s, want %s", claims.AgentID, agentID)
	}
	if claims.AuthMethod != "atc" {
		t.Errorf("auth method: got %s, want atc", claims.AuthMethod)
	}
}

func TestServerATCVerifier_RefusesInvalidServerKey(t *testing.T) {
	for name, key := range map[string]ed25519.PrivateKey{
		"nil":   nil,
		"short": make(ed25519.PrivateKey, 10),
	} {
		if _, err := NewServerATCVerifier("https://aim.test.opena2a.org", key, nil); err == nil {
			t.Errorf("%s key: expected an error", name)
		}
	}
}
