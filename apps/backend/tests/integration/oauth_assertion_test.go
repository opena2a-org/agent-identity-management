package integration

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// This test needs no backend. The token endpoint refuses an assertion with no
// exp, an exp more than five minutes (plus a minute of clock skew) ahead, or
// an aud that does not name the server; the assertion the OAuth tests sign
// carries an exp within five minutes and the audience it is given.
func TestBuildSignedJWTAssertion_CarriesExpIatAndTheAudience(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	before := time.Now().Unix()
	assertion := buildSignedJWTAssertion("agent-1", "http://backend.invalid:18080", priv)
	after := time.Now().Unix()

	parts := strings.Split(assertion, ".")
	require.Len(t, parts, 3)

	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	require.NoError(t, err)
	assert.True(t, ed25519.Verify(pub, []byte(parts[0]+"."+parts[1]), sig), "the signature covers header and payload")

	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	require.NoError(t, err)
	var claims struct {
		Sub string `json:"sub"`
		Iss string `json:"iss"`
		Aud string `json:"aud"`
		Iat int64  `json:"iat"`
		Exp int64  `json:"exp"`
	}
	require.NoError(t, json.Unmarshal(payloadJSON, &claims))

	assert.Equal(t, "agent-1", claims.Sub)
	assert.Equal(t, "agent-1", claims.Iss)
	assert.Equal(t, "http://backend.invalid:18080", claims.Aud)
	assert.GreaterOrEqual(t, claims.Iat, before)
	assert.LessOrEqual(t, claims.Iat, after)
	assert.Greater(t, claims.Exp, after, "exp is in the future")
	assert.LessOrEqual(t, claims.Exp, before+int64((5*time.Minute).Seconds()), "exp is at most five minutes ahead")
}
