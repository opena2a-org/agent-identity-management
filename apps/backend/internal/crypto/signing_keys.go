package crypto

import (
	"crypto/ed25519"
	"crypto/hkdf"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
)

// SigningPurpose names one thing the server signs. Each purpose has its own key,
// so a signature made for one purpose never verifies for another.
type SigningPurpose string

const (
	// PurposeCardAttestation signs A2A agent card attestations.
	PurposeCardAttestation SigningPurpose = "card-attestation"
	// PurposeATCIssuer is the key the ATC verifier trusts for this server's issuer URI.
	PurposeATCIssuer SigningPurpose = "atc-issuer"
)

// signingPurposes is every purpose the server signs for, in a fixed order. Derivation
// refuses any purpose not listed here.
var signingPurposes = []SigningPurpose{
	PurposeCardAttestation,
	PurposeATCIssuer,
}

// publishedSigningPurposes are the purposes whose public keys are served in the JWK Set.
var publishedSigningPurposes = []SigningPurpose{
	PurposeCardAttestation,
	PurposeATCIssuer,
}

const (
	// SigningAlgEdDSA is the JOSE algorithm name for every server signing key.
	SigningAlgEdDSA = "EdDSA"

	signingKeyHKDFSalt       = "opena2a-aim-signing-v1"
	signingKeyHKDFInfoPrefix = "opena2a-aim/ed25519/"
	signingKeyEnvPrefix      = "AIM_SIGNING_KEY_"
	retiredKeysEnvSuffix     = "_RETIRED"
)

// SigningKeySource says where a signing key came from.
type SigningKeySource string

const (
	// SigningKeySourceDerived keys are derived from KEYVAULT_MASTER_KEY with HKDF-SHA256.
	SigningKeySourceDerived SigningKeySource = "derived"
	// SigningKeySourceProvisioned keys are a seed the operator set in AIM_SIGNING_KEY_<PURPOSE>.
	SigningKeySourceProvisioned SigningKeySource = "provisioned"
)

// SigningKey is the active key for one purpose.
type SigningKey struct {
	Purpose SigningPurpose
	Alg     string
	KeyID   string
	Source  SigningKeySource

	privateKey ed25519.PrivateKey
}

// PrivateKey returns the Ed25519 private key.
func (k *SigningKey) PrivateKey() ed25519.PrivateKey {
	return k.privateKey
}

// PublicKey returns the Ed25519 public key.
func (k *SigningKey) PublicKey() ed25519.PublicKey {
	return k.privateKey.Public().(ed25519.PublicKey)
}

// Sign signs message with this key.
func (k *SigningKey) Sign(message []byte) []byte {
	return ed25519.Sign(k.privateKey, message)
}

// SigningKeyRing holds one active key per purpose and the retired public keys that
// still verify for that purpose.
type SigningKeyRing struct {
	active  map[SigningPurpose]*SigningKey
	retired map[SigningPurpose][]ed25519.PublicKey
}

// SigningKeyEnvVar returns the environment variable that provisions the key for purpose,
// for example AIM_SIGNING_KEY_CARD_ATTESTATION.
func SigningKeyEnvVar(purpose SigningPurpose) string {
	return signingKeyEnvPrefix + strings.ToUpper(strings.ReplaceAll(string(purpose), "-", "_"))
}

// SigningKeyID returns the key ID for an Ed25519 public key: the hex SHA-256 of its bytes.
func SigningKeyID(publicKey ed25519.PublicKey) string {
	sum := sha256.Sum256(publicKey)
	return hex.EncodeToString(sum[:])
}

func isSigningPurpose(purpose SigningPurpose) bool {
	for _, p := range signingPurposes {
		if p == purpose {
			return true
		}
	}
	return false
}

// DeriveSigningKey derives the Ed25519 key for purpose from masterKey with HKDF-SHA256.
// It refuses every purpose that is not one of the server's signing purposes.
func DeriveSigningKey(masterKey []byte, purpose SigningPurpose) (ed25519.PrivateKey, error) {
	if !isSigningPurpose(purpose) {
		return nil, fmt.Errorf("no signing key is derived for purpose %q", purpose)
	}
	if len(masterKey) == 0 {
		return nil, fmt.Errorf("master key is required to derive the %s signing key", purpose)
	}
	seed, err := hkdf.Key(sha256.New, masterKey, []byte(signingKeyHKDFSalt), signingKeyHKDFInfoPrefix+string(purpose), ed25519.SeedSize)
	if err != nil {
		return nil, fmt.Errorf("failed to derive the %s signing key: %w", purpose, err)
	}
	return ed25519.NewKeyFromSeed(seed), nil
}

// LoadSigningKeyRing builds the server's signing keys. For each purpose,
// AIM_SIGNING_KEY_<PURPOSE> holds a base64 32-byte Ed25519 seed; when it is unset the key
// is derived from the vault's master key. AIM_SIGNING_KEY_<PURPOSE>_RETIRED holds
// comma-separated public keys that still verify for that purpose, each in base64 or in
// the base64url form of a JWK "x" member. A malformed value, or one key used for two
// purposes, is an error naming the variable.
func LoadSigningKeyRing(kv *KeyVault, getenv func(string) string) (*SigningKeyRing, error) {
	if kv == nil {
		return nil, fmt.Errorf("key vault is required to load signing keys")
	}

	ring := &SigningKeyRing{
		active:  make(map[SigningPurpose]*SigningKey, len(signingPurposes)),
		retired: make(map[SigningPurpose][]ed25519.PublicKey, len(signingPurposes)),
	}

	for _, purpose := range signingPurposes {
		envVar := SigningKeyEnvVar(purpose)

		var privateKey ed25519.PrivateKey
		source := SigningKeySourceDerived
		if value := strings.TrimSpace(getenv(envVar)); value != "" {
			seed, err := base64.StdEncoding.DecodeString(value)
			if err != nil || len(seed) != ed25519.SeedSize {
				return nil, fmt.Errorf("%s must be a base64-encoded %d-byte Ed25519 seed", envVar, ed25519.SeedSize)
			}
			privateKey = ed25519.NewKeyFromSeed(seed)
			source = SigningKeySourceProvisioned
		} else {
			derived, err := DeriveSigningKey(kv.masterKey, purpose)
			if err != nil {
				return nil, err
			}
			privateKey = derived
		}

		key := &SigningKey{
			Purpose:    purpose,
			Alg:        SigningAlgEdDSA,
			Source:     source,
			privateKey: privateKey,
		}
		key.KeyID = SigningKeyID(key.PublicKey())
		ring.active[purpose] = key

		retiredVar := envVar + retiredKeysEnvSuffix
		for _, entry := range strings.Split(getenv(retiredVar), ",") {
			entry = strings.TrimSpace(entry)
			if entry == "" {
				continue
			}
			publicKey, err := decodePublicKey(entry)
			if err != nil || len(publicKey) != ed25519.PublicKeySize {
				return nil, fmt.Errorf("%s must list base64-encoded %d-byte Ed25519 public keys", retiredVar, ed25519.PublicKeySize)
			}
			ring.retired[purpose] = append(ring.retired[purpose], ed25519.PublicKey(publicKey))
		}
	}

	// Fail closed on a shared key: no public key may serve two purposes, a retired key
	// may not also be the active key it was retired from, and a retired list may not
	// name one key twice. Each key ID remembers the variable that contributed it, so
	// the error names the two variables to change.
	type keyEntry struct {
		keyID   string
		purpose SigningPurpose
		envVar  string
	}
	owner := make(map[string]keyEntry)
	for _, purpose := range signingPurposes {
		activeVar := SigningKeyEnvVar(purpose)
		entries := []keyEntry{{ring.active[purpose].KeyID, purpose, activeVar}}
		for _, publicKey := range ring.retired[purpose] {
			entries = append(entries, keyEntry{SigningKeyID(publicKey), purpose, activeVar + retiredKeysEnvSuffix})
		}
		for _, entry := range entries {
			other, seen := owner[entry.keyID]
			if !seen {
				owner[entry.keyID] = entry
				continue
			}
			switch {
			case other.envVar == entry.envVar:
				return nil, fmt.Errorf("%s lists the same key twice", entry.envVar)
			case other.purpose == entry.purpose:
				return nil, fmt.Errorf("%s lists a key that is already in use for %s", entry.envVar, entry.purpose)
			default:
				return nil, fmt.Errorf("the %s and %s signing keys are the same key; set %s and %s to different keys", other.purpose, entry.purpose, other.envVar, entry.envVar)
			}
		}
	}

	return ring, nil
}

// decodePublicKey reads a public key written in base64 or base64url, padded or not.
func decodePublicKey(value string) ([]byte, error) {
	var lastErr error
	for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		decoded, err := enc.DecodeString(value)
		if err == nil {
			return decoded, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

// Key returns the active key for purpose, or nil if the purpose is unknown.
func (r *SigningKeyRing) Key(purpose SigningPurpose) *SigningKey {
	return r.active[purpose]
}

// RetiredPublicKeys returns the retired public keys that still verify for purpose.
func (r *SigningKeyRing) RetiredPublicKeys(purpose SigningPurpose) []ed25519.PublicKey {
	return append([]ed25519.PublicKey(nil), r.retired[purpose]...)
}

// Keys returns the active key of every purpose, in a fixed order.
func (r *SigningKeyRing) Keys() []*SigningKey {
	keys := make([]*SigningKey, 0, len(signingPurposes))
	for _, purpose := range signingPurposes {
		keys = append(keys, r.active[purpose])
	}
	return keys
}

// JWK is one public key in a JSON Web Key Set (RFC 7517, OKP keys per RFC 8037), with
// the purpose, status and source of the key as extra members.
type JWK struct {
	Kty     string `json:"kty"`
	Crv     string `json:"crv"`
	X       string `json:"x"`
	Kid     string `json:"kid"`
	Alg     string `json:"alg"`
	Use     string `json:"use"`
	Purpose string `json:"purpose"`
	Status  string `json:"status"`
	Source  string `json:"source,omitempty"`
}

// JWKSet is a JSON Web Key Set.
type JWKSet struct {
	Keys []JWK `json:"keys"`
}

func newPublicJWK(publicKey ed25519.PublicKey, purpose SigningPurpose, status string, source SigningKeySource) JWK {
	return JWK{
		Kty:     "OKP",
		Crv:     "Ed25519",
		X:       base64.RawURLEncoding.EncodeToString(publicKey),
		Kid:     SigningKeyID(publicKey),
		Alg:     SigningAlgEdDSA,
		Use:     "sig",
		Purpose: string(purpose),
		Status:  status,
		Source:  string(source),
	}
}

// JWKS returns the public keys of the published purposes, active keys first and then
// retired keys. It carries public key material only.
func (r *SigningKeyRing) JWKS() JWKSet {
	set := JWKSet{Keys: []JWK{}}
	for _, purpose := range publishedSigningPurposes {
		key := r.active[purpose]
		set.Keys = append(set.Keys, newPublicJWK(key.PublicKey(), purpose, "active", key.Source))
		for _, publicKey := range r.retired[purpose] {
			set.Keys = append(set.Keys, newPublicJWK(publicKey, purpose, "retired", ""))
		}
	}
	return set
}
