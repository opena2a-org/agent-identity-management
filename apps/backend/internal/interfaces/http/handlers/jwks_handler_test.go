package handlers

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto"
)

func newJWKSTestRing(t *testing.T) *crypto.SigningKeyRing {
	t.Helper()
	master := make([]byte, 32)
	_, err := rand.Read(master)
	require.NoError(t, err)
	kv, err := crypto.NewKeyVault(base64.StdEncoding.EncodeToString(master))
	require.NoError(t, err)
	ring, err := crypto.LoadSigningKeyRing(kv, func(string) string { return "" })
	require.NoError(t, err)
	return ring
}

// The handler is built from the key ring alone — no repository, no database — so
// serving the key set can neither read nor write storage.
func TestJWKSServesPublishedKeysWithoutAuthentication(t *testing.T) {
	ring := newJWKSTestRing(t)
	app := fiber.New()
	app.Get("/.well-known/jwks.json", NewJWKSHandler(ring).GetJWKS)

	resp, err := app.Test(httptest.NewRequest("GET", "/.well-known/jwks.json", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Equal(t, "public, max-age=300", resp.Header.Get("Cache-Control"))
	assert.Contains(t, resp.Header.Get("Content-Type"), "application/json")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var set struct {
		Keys []map[string]any `json:"keys"`
	}
	require.NoError(t, json.Unmarshal(body, &set))
	require.Len(t, set.Keys, 2, "one active key each for card-attestation and atc-issuer")

	want := map[string]string{
		ring.Key(crypto.PurposeCardAttestation).KeyID: "card-attestation",
		ring.Key(crypto.PurposeATCIssuer).KeyID:       "atc-issuer",
	}
	for _, k := range set.Keys {
		_, hasD := k["d"]
		assert.False(t, hasD, "a published key carries its private member")
		kid, _ := k["kid"].(string)
		assert.Equal(t, want[kid], k["purpose"], "kid %s is not a published key or has the wrong purpose", kid)
		assert.Equal(t, "OKP", k["kty"])
		assert.Equal(t, "Ed25519", k["crv"])
		assert.Equal(t, "active", k["status"])
	}
}

// TestJWKSRouteIsMountedOnTheRootApp is a source-wiring guard: the route must be
// registered on the root app beside /.well-known/agent.json, outside every
// authenticated group, or verifiers cannot fetch the keys.
func TestJWKSRouteIsMountedOnTheRootApp(t *testing.T) {
	mainGo := readBackendFile(t, "cmd/server/main.go")

	var found int
	for _, l := range strings.Split(mainGo, "\n") {
		trimmed := strings.TrimSpace(l)
		if strings.HasPrefix(trimmed, "//") || !strings.Contains(trimmed, `"/.well-known/jwks.json"`) {
			continue
		}
		found++
		assert.Equal(t, `app.Get("/.well-known/jwks.json", h.JWKS.GetJWKS)`, trimmed)
	}
	assert.Equal(t, 1, found, "expected exactly one registration of /.well-known/jwks.json in main.go")
	assert.Contains(t, mainGo, "JWKS: handlers.NewJWKSHandler(services.SigningKeys)")
}
