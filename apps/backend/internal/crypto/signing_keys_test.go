package crypto

import (
	"crypto/ed25519"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestVault returns a vault over a master key minted for this run, and the master bytes.
func newTestVault(t *testing.T) (*KeyVault, []byte) {
	t.Helper()
	master := make([]byte, 32)
	_, err := rand.Read(master)
	require.NoError(t, err)
	kv, err := NewKeyVault(base64.StdEncoding.EncodeToString(master))
	require.NoError(t, err)
	return kv, master
}

func envFrom(values map[string]string) func(string) string {
	return func(name string) string { return values[name] }
}

func randomSeed(t *testing.T) []byte {
	t.Helper()
	seed := make([]byte, ed25519.SeedSize)
	_, err := rand.Read(seed)
	require.NoError(t, err)
	return seed
}

// legacyServerKey is the single key every purpose shared before keys were separated:
// SHA-256 over a fixed label and the master key.
func legacyServerKey(master []byte) ed25519.PublicKey {
	h := sha256.New()
	h.Write([]byte("aim-server-attestation-signing-key-v1:"))
	h.Write(master)
	return ed25519.NewKeyFromSeed(h.Sum(nil)).Public().(ed25519.PublicKey)
}

func TestSigningKeysAreOnePerPurposeAndDerivedWithHKDF(t *testing.T) {
	kv, master := newTestVault(t)
	ring, err := LoadSigningKeyRing(kv, envFrom(nil))
	require.NoError(t, err)

	// The derivation strings are pinned here, not read from the code under test, so a
	// change to either one (which would silently rotate every derived key) fails this test.
	want := map[SigningPurpose]string{
		PurposeCardAttestation: "opena2a-aim/ed25519/card-attestation",
		PurposeATCIssuer:       "opena2a-aim/ed25519/atc-issuer",
	}
	require.Len(t, ring.Keys(), len(want))

	seen := map[string]SigningPurpose{}
	legacy := legacyServerKey(master)
	for purpose, info := range want {
		key := ring.Key(purpose)
		require.NotNil(t, key, "purpose %s has no key", purpose)

		seed, err := hkdf.Key(sha256.New, master, []byte("opena2a-aim-signing-v1"), info, ed25519.SeedSize)
		require.NoError(t, err)
		assert.Equal(t, ed25519.NewKeyFromSeed(seed), key.PrivateKey(), "%s is not HKDF-SHA256 over the pinned strings", purpose)

		assert.Equal(t, SigningKeySourceDerived, key.Source)
		assert.Equal(t, "EdDSA", key.Alg)
		sum := sha256.Sum256(key.PublicKey())
		assert.Equal(t, hex.EncodeToString(sum[:]), key.KeyID, "kid is the hex SHA-256 of the public key")

		assert.False(t, key.PublicKey().Equal(legacy), "%s still uses the key every purpose used to share", purpose)
		if other, dup := seen[key.KeyID]; dup {
			t.Fatalf("%s and %s resolve to the same key", other, purpose)
		}
		seen[key.KeyID] = purpose
	}
}

func TestEachPurposeKeyVerifiesOnlyItsOwnSignatures(t *testing.T) {
	kv, _ := newTestVault(t)
	ring, err := LoadSigningKeyRing(kv, envFrom(nil))
	require.NoError(t, err)

	message := []byte("signed for one purpose")
	for _, signer := range ring.Keys() {
		signature := signer.Sign(message)
		for _, verifier := range ring.Keys() {
			ok := ed25519.Verify(verifier.PublicKey(), message, signature)
			if verifier.Purpose == signer.Purpose {
				assert.True(t, ok, "%s does not verify its own signature", signer.Purpose)
			} else {
				assert.False(t, ok, "a %s signature verifies under the %s key", signer.Purpose, verifier.Purpose)
			}
		}
	}
}

func TestProvisionedKeyOverridesOnlyItsOwnPurpose(t *testing.T) {
	kv, _ := newTestVault(t)
	derived, err := LoadSigningKeyRing(kv, envFrom(nil))
	require.NoError(t, err)

	seed := randomSeed(t)
	ring, err := LoadSigningKeyRing(kv, envFrom(map[string]string{
		"AIM_SIGNING_KEY_ATC_ISSUER": base64.StdEncoding.EncodeToString(seed),
	}))
	require.NoError(t, err)

	issuer := ring.Key(PurposeATCIssuer)
	assert.Equal(t, ed25519.NewKeyFromSeed(seed), issuer.PrivateKey())
	assert.Equal(t, SigningKeySourceProvisioned, issuer.Source)
	sum := sha256.Sum256(ed25519.NewKeyFromSeed(seed).Public().(ed25519.PublicKey))
	assert.Equal(t, hex.EncodeToString(sum[:]), issuer.KeyID)

	for _, purpose := range []SigningPurpose{PurposeCardAttestation} {
		assert.Equal(t, derived.Key(purpose).KeyID, ring.Key(purpose).KeyID, "provisioning the issuer key moved the %s key", purpose)
		assert.Equal(t, SigningKeySourceDerived, ring.Key(purpose).Source)
	}
}

func TestSigningKeyEnvVarNames(t *testing.T) {
	assert.Equal(t, "AIM_SIGNING_KEY_CARD_ATTESTATION", SigningKeyEnvVar(PurposeCardAttestation))
	assert.Equal(t, "AIM_SIGNING_KEY_ATC_ISSUER", SigningKeyEnvVar(PurposeATCIssuer))
}

func TestMalformedSigningKeyIsRefusedNamingTheVariable(t *testing.T) {
	kv, _ := newTestVault(t)
	shortSeed := base64.StdEncoding.EncodeToString(randomSeed(t)[:16])
	shortPublicKey := base64.StdEncoding.EncodeToString(randomSeed(t)[:31])

	cases := []struct {
		name, envVar, value string
	}{
		{"seed is not base64", "AIM_SIGNING_KEY_CARD_ATTESTATION", "not*base64*at*all"},
		{"seed is 16 bytes", "AIM_SIGNING_KEY_ATC_ISSUER", shortSeed},
		{"seed is 16 bytes", "AIM_SIGNING_KEY_CARD_ATTESTATION", shortSeed},
		{"retired key is not base64", "AIM_SIGNING_KEY_CARD_ATTESTATION_RETIRED", "not*base64*at*all"},
		{"retired key is 31 bytes", "AIM_SIGNING_KEY_ATC_ISSUER_RETIRED", shortPublicKey},
	}
	for _, tc := range cases {
		t.Run(tc.envVar+"/"+tc.name, func(t *testing.T) {
			ring, err := LoadSigningKeyRing(kv, envFrom(map[string]string{tc.envVar: tc.value}))
			require.Error(t, err)
			assert.Nil(t, ring)
			assert.Contains(t, err.Error(), tc.envVar)
			assert.NotContains(t, err.Error(), tc.value, "the error must not echo the configured value")
		})
	}
}

func TestDerivationRefusesEveryOtherPurpose(t *testing.T) {
	_, master := newTestVault(t)
	for _, purpose := range []SigningPurpose{"record-signing", "", "card-attestation ", "CARD-ATTESTATION", "atc", "atc-shim-token"} {
		key, err := DeriveSigningKey(master, purpose)
		assert.Error(t, err, "derivation accepted purpose %q", purpose)
		assert.Nil(t, key)
	}
	for _, purpose := range signingPurposes {
		_, err := DeriveSigningKey(master, purpose)
		assert.NoError(t, err)
	}
	_, err := DeriveSigningKey(nil, PurposeCardAttestation)
	assert.Error(t, err, "derivation accepted an empty master key")
}

func TestSharedSigningKeyIsRefused(t *testing.T) {
	kv, _ := newTestVault(t)
	seed := base64.StdEncoding.EncodeToString(randomSeed(t))

	t.Run("two purposes provisioned with one seed", func(t *testing.T) {
		_, err := LoadSigningKeyRing(kv, envFrom(map[string]string{
			"AIM_SIGNING_KEY_CARD_ATTESTATION": seed,
			"AIM_SIGNING_KEY_ATC_ISSUER":       seed,
		}))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "AIM_SIGNING_KEY_CARD_ATTESTATION")
		assert.Contains(t, err.Error(), "AIM_SIGNING_KEY_ATC_ISSUER")
		assert.NotContains(t, err.Error(), seed)
	})

	t.Run("a retired key is another purpose's active key", func(t *testing.T) {
		derived, err := LoadSigningKeyRing(kv, envFrom(nil))
		require.NoError(t, err)
		cardPublic := base64.StdEncoding.EncodeToString(derived.Key(PurposeCardAttestation).PublicKey())
		_, err = LoadSigningKeyRing(kv, envFrom(map[string]string{
			"AIM_SIGNING_KEY_ATC_ISSUER_RETIRED": cardPublic,
		}))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "same key")
		assert.Contains(t, err.Error(), "set AIM_SIGNING_KEY_CARD_ATTESTATION and AIM_SIGNING_KEY_ATC_ISSUER_RETIRED to different keys")
	})

	t.Run("one retired list names a key twice", func(t *testing.T) {
		_, retiredPrivate, err := ed25519.GenerateKey(rand.Reader)
		require.NoError(t, err)
		retired := base64.StdEncoding.EncodeToString(retiredPrivate.Public().(ed25519.PublicKey))
		_, err = LoadSigningKeyRing(kv, envFrom(map[string]string{
			"AIM_SIGNING_KEY_ATC_ISSUER_RETIRED": retired + "," + retired,
		}))
		require.Error(t, err)
		assert.Equal(t, "AIM_SIGNING_KEY_ATC_ISSUER_RETIRED lists the same key twice", err.Error())
	})

	t.Run("one key in both retired lists", func(t *testing.T) {
		_, retiredPrivate, err := ed25519.GenerateKey(rand.Reader)
		require.NoError(t, err)
		retired := base64.StdEncoding.EncodeToString(retiredPrivate.Public().(ed25519.PublicKey))
		_, err = LoadSigningKeyRing(kv, envFrom(map[string]string{
			"AIM_SIGNING_KEY_CARD_ATTESTATION_RETIRED": retired,
			"AIM_SIGNING_KEY_ATC_ISSUER_RETIRED":       retired,
		}))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "same key")
		assert.Contains(t, err.Error(), "set AIM_SIGNING_KEY_CARD_ATTESTATION_RETIRED and AIM_SIGNING_KEY_ATC_ISSUER_RETIRED to different keys")
	})

	t.Run("a retired key is its own purpose's active key", func(t *testing.T) {
		derived, err := LoadSigningKeyRing(kv, envFrom(nil))
		require.NoError(t, err)
		cardPublic := base64.StdEncoding.EncodeToString(derived.Key(PurposeCardAttestation).PublicKey())
		_, err = LoadSigningKeyRing(kv, envFrom(map[string]string{
			"AIM_SIGNING_KEY_CARD_ATTESTATION_RETIRED": cardPublic,
		}))
		require.Error(t, err)
		assert.Equal(t, "AIM_SIGNING_KEY_CARD_ATTESTATION_RETIRED lists a key that is already in use for card-attestation", err.Error())
	})
}

func TestJWKSPublishesCardAndIssuerKeysWithoutPrivateMaterial(t *testing.T) {
	kv, _ := newTestVault(t)
	seed := randomSeed(t)
	_, retiredPrivate, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	retiredPublic := retiredPrivate.Public().(ed25519.PublicKey)

	ring, err := LoadSigningKeyRing(kv, envFrom(map[string]string{
		"AIM_SIGNING_KEY_CARD_ATTESTATION":         base64.StdEncoding.EncodeToString(seed),
		"AIM_SIGNING_KEY_CARD_ATTESTATION_RETIRED": " " + base64.StdEncoding.EncodeToString(retiredPublic) + " ,",
	}))
	require.NoError(t, err)

	set := ring.JWKS()
	body, err := json.Marshal(set)
	require.NoError(t, err)

	// Exactly the card-attestation (active and retired) and ATC-issuer keys.
	require.Len(t, set.Keys, 3)
	byKid := map[string]JWK{}
	for _, k := range set.Keys {
		byKid[k.Kid] = k
		assert.Equal(t, "OKP", k.Kty)
		assert.Equal(t, "Ed25519", k.Crv)
		assert.Equal(t, "EdDSA", k.Alg)
		assert.Equal(t, "sig", k.Use)
		x, err := base64.RawURLEncoding.DecodeString(k.X)
		require.NoError(t, err)
		assert.Equal(t, k.Kid, SigningKeyID(x), "kid does not match the published key")
	}

	card := byKid[ring.Key(PurposeCardAttestation).KeyID]
	assert.Equal(t, "card-attestation", card.Purpose)
	assert.Equal(t, "active", card.Status)
	assert.Equal(t, "provisioned", card.Source)

	retired := byKid[SigningKeyID(retiredPublic)]
	assert.Equal(t, "card-attestation", retired.Purpose)
	assert.Equal(t, "retired", retired.Status)

	issuer := byKid[ring.Key(PurposeATCIssuer).KeyID]
	assert.Equal(t, "atc-issuer", issuer.Purpose)
	assert.Equal(t, "active", issuer.Status)
	assert.Equal(t, "derived", issuer.Source)

	var raw map[string][]map[string]any
	require.NoError(t, json.Unmarshal(body, &raw))
	for _, k := range raw["keys"] {
		_, hasD := k["d"]
		assert.False(t, hasD, "a published key carries a private member")
	}
	for _, key := range ring.Keys() {
		private := key.PrivateKey()
		for _, form := range []string{
			base64.StdEncoding.EncodeToString(private.Seed()),
			base64.RawURLEncoding.EncodeToString(private.Seed()),
			base64.RawURLEncoding.EncodeToString(private),
		} {
			assert.NotContains(t, string(body), form, "the %s private key appears in the JWK Set", key.Purpose)
		}
	}
	assert.NotContains(t, string(body), base64.StdEncoding.EncodeToString(seed))
}

func TestRetiredKeyAcceptsTheJWKXForm(t *testing.T) {
	kv, _ := newTestVault(t)
	_, retiredPrivate, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	retiredPublic := retiredPrivate.Public().(ed25519.PublicKey)

	// The "x" member of a published key, copied as-is from /.well-known/jwks.json.
	ring, err := LoadSigningKeyRing(kv, envFrom(map[string]string{
		"AIM_SIGNING_KEY_ATC_ISSUER_RETIRED": base64.RawURLEncoding.EncodeToString(retiredPublic),
	}))
	require.NoError(t, err)
	require.Len(t, ring.RetiredPublicKeys(PurposeATCIssuer), 1)
	assert.True(t, ring.RetiredPublicKeys(PurposeATCIssuer)[0].Equal(retiredPublic))
	assert.Empty(t, ring.RetiredPublicKeys(PurposeCardAttestation))
}

// TestNoProductionCodeUsesTheSharedServerKey pins that the one-key-for-everything
// construction is gone from production code, and that the master key's encrypt and
// decrypt paths still exist for the agent private keys it protects.
func TestNoProductionCodeUsesTheSharedServerKey(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	backend := filepath.Join(filepath.Dir(file), "..", "..")

	forbidden := []string{"GetServerSigningKey", "GetServerSigningPublicKey", "aim-server-attestation-signing-key-v1"}
	checked := 0
	err := filepath.WalkDir(backend, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if name := d.Name(); name == "vendor" || name == "node_modules" || strings.HasPrefix(name, ".") && path != backend {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		src, err := os.ReadFile(path) //nolint:gosec // G304: path walked from this package's own source tree
		if err != nil {
			return err
		}
		checked++
		for _, f := range forbidden {
			assert.NotContains(t, string(src), f, "%s still references %s", path, f)
		}
		return nil
	})
	require.NoError(t, err)
	require.Greater(t, checked, 100, "the walk did not reach the backend source tree")

	kv, _ := newTestVault(t)
	sealed, err := kv.EncryptPrivateKey(testAgentID, "agent-private-key")
	require.NoError(t, err)
	opened, err := kv.DecryptPrivateKey(testAgentID, sealed)
	require.NoError(t, err)
	assert.Equal(t, "agent-private-key", opened)
}
