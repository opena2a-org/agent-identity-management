package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto"
)

// The card attestation conformance vector publishes two seeds so that anyone can
// reproduce its signatures, and so anyone can sign with them. The server refuses to start
// with either one as a signing key, provisioned or listed as retired, for either purpose,
// and the error names the variable to change.
func TestServerRefusesTheConformanceVectorKeysAsSigningKeys(t *testing.T) {
	requirePublicRepositoryFiles(t)

	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "docs", "specs", "card-attestation-v2-vector.json"))
	require.NoError(t, err)
	var vector struct {
		Inputs struct {
			Seed          string `json:"seed"`
			ATCIssuerSeed string `json:"atcIssuerSeed"`
		} `json:"inputs"`
		JWKS struct {
			Keys []struct {
				X string `json:"x"`
			} `json:"keys"`
		} `json:"jwks"`
	}
	require.NoError(t, json.Unmarshal(raw, &vector))
	seeds := []string{vector.Inputs.Seed, vector.Inputs.ATCIssuerSeed}
	require.NotContains(t, seeds, "", "the vector names both seeds")
	require.Len(t, vector.JWKS.Keys, len(seeds), "the vector publishes one key per seed")

	master := make([]byte, 32)
	_, err = rand.Read(master)
	require.NoError(t, err)
	keyVault, err := crypto.NewKeyVault(base64.StdEncoding.EncodeToString(master))
	require.NoError(t, err)
	load := func(env map[string]string) (*crypto.SigningKeyRing, error) {
		return loadServerSigningKeys(keyVault, func(name string) string { return env[name] })
	}

	envVars := []string{"AIM_SIGNING_KEY_CARD_ATTESTATION", "AIM_SIGNING_KEY_ATC_ISSUER"}
	for _, envVar := range envVars {
		for _, seed := range seeds {
			ring, err := load(map[string]string{envVar: seed})
			require.Error(t, err, "%s set to a vector seed", envVar)
			assert.Nil(t, ring)
			assert.True(t, strings.HasPrefix(err.Error(), envVar+" holds a seed that the card attestation conformance vector publishes"), err.Error())
			assert.NotContains(t, err.Error(), seed, "the error must not echo the configured value")
		}
		// The x member of each published key, as an operator copies a retired key from a
		// JWK Set.
		for _, key := range vector.JWKS.Keys {
			ring, err := load(map[string]string{envVar + "_RETIRED": key.X})
			require.Error(t, err, "%s_RETIRED listing a vector key", envVar)
			assert.Nil(t, ring)
			assert.True(t, strings.HasPrefix(err.Error(), envVar+"_RETIRED lists the public key of a seed that the card attestation conformance vector publishes"), err.Error())
		}
	}

	own := make([]byte, 32)
	_, err = rand.Read(own)
	require.NoError(t, err)
	ring, err := load(map[string]string{"AIM_SIGNING_KEY_CARD_ATTESTATION": base64.StdEncoding.EncodeToString(own)})
	require.NoError(t, err, "a seed of the operator's own loads")
	require.NotNil(t, ring)
}
