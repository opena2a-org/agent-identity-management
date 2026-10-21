package crypto

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// conformanceVectorSeed returns a seed that docs/specs/card-attestation-v2-vector.json
// publishes: the 32 bytes counting up from first. inputs.seed starts at 0x00 and
// inputs.atcIssuerSeed at 0x20. cmd/server's test reads the seeds from the vector itself.
func conformanceVectorSeed(first byte) []byte {
	seed := make([]byte, ed25519.SeedSize)
	for i := range seed {
		seed[i] = first + byte(i)
	}
	return seed
}

var conformanceVectorSeedStarts = []byte{0x00, 0x20}

func TestConformanceVectorKeyIDsAreTheKeyIDsOfTheVectorSeeds(t *testing.T) {
	want := map[string]bool{}
	for _, first := range conformanceVectorSeedStarts {
		public := ed25519.NewKeyFromSeed(conformanceVectorSeed(first)).Public().(ed25519.PublicKey)
		want[SigningKeyID(public)] = true
	}
	assert.Equal(t, want, conformanceVectorKeyIDs)
}

func TestARingHoldingAConformanceVectorKeyIsRefusedNamingTheVariable(t *testing.T) {
	kv, _ := newTestVault(t)

	for _, first := range conformanceVectorSeedStarts {
		seed := conformanceVectorSeed(first)
		seedValue := base64.StdEncoding.EncodeToString(seed)
		public := ed25519.NewKeyFromSeed(seed).Public().(ed25519.PublicKey)

		for _, purpose := range signingPurposes {
			envVar := SigningKeyEnvVar(purpose)

			t.Run(fmt.Sprintf("%s set to the vector seed starting at 0x%02x", envVar, first), func(t *testing.T) {
				ring, err := LoadSigningKeyRing(kv, envFrom(map[string]string{envVar: seedValue}))
				require.NoError(t, err, "LoadSigningKeyRing builds the ring the vector describes")
				err = ring.RefuseConformanceVectorKeys()
				require.Error(t, err)
				assert.True(t, strings.HasPrefix(err.Error(), envVar+" holds a seed that the card attestation conformance vector publishes"), err.Error())
				assert.NotContains(t, err.Error(), seedValue, "the error must not echo the configured value")
			})

			retiredVar := envVar + "_RETIRED"
			forms := map[string]string{
				"base64":            base64.StdEncoding.EncodeToString(public),
				"JWK x (base64url)": base64.RawURLEncoding.EncodeToString(public),
			}
			for form, value := range forms {
				t.Run(retiredVar+" listing the vector key in "+form, func(t *testing.T) {
					ring, err := LoadSigningKeyRing(kv, envFrom(map[string]string{retiredVar: value}))
					require.NoError(t, err)
					err = ring.RefuseConformanceVectorKeys()
					require.Error(t, err)
					assert.True(t, strings.HasPrefix(err.Error(), retiredVar+" lists the public key of a seed that the card attestation conformance vector publishes"), err.Error())
				})
			}
		}
	}

	t.Run("a ring without either vector key is accepted", func(t *testing.T) {
		derived, err := LoadSigningKeyRing(kv, envFrom(nil))
		require.NoError(t, err)
		assert.NoError(t, derived.RefuseConformanceVectorKeys())

		_, retiredPrivate, err := ed25519.GenerateKey(nil)
		require.NoError(t, err)
		provisioned, err := LoadSigningKeyRing(kv, envFrom(map[string]string{
			"AIM_SIGNING_KEY_CARD_ATTESTATION":   base64.StdEncoding.EncodeToString(randomSeed(t)),
			"AIM_SIGNING_KEY_ATC_ISSUER":         base64.StdEncoding.EncodeToString(randomSeed(t)),
			"AIM_SIGNING_KEY_ATC_ISSUER_RETIRED": base64.StdEncoding.EncodeToString(retiredPrivate.Public().(ed25519.PublicKey)),
		}))
		require.NoError(t, err)
		assert.NoError(t, provisioned.RefuseConformanceVectorKeys())
	})
}
