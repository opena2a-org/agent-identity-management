package application

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// The verifier below is what a third party writes from docs/specs/card-attestation-v2.md.
// It reads only the served card JSON and the JWK Set JSON, and uses none of the server's
// signing helpers, so a server change that breaks the published procedure fails here.

type thirdPartyAttestation struct {
	Format    string `json:"format"`
	Issuer    string `json:"issuer"`
	CardHash  string `json:"cardHash"`
	IssuedAt  string `json:"issuedAt"`
	ExpiresAt string `json:"expiresAt"`
	Signature string `json:"signature"`
	KeyID     string `json:"keyId"`
	Alg       string `json:"alg"`
}

type thirdPartyJWKSet struct {
	Keys []struct {
		Kty     string `json:"kty"`
		Crv     string `json:"crv"`
		X       string `json:"x"`
		Kid     string `json:"kid"`
		Purpose string `json:"purpose"`
	} `json:"keys"`
}

var (
	thirdPartyHex64     = regexp.MustCompile(`^[0-9a-f]{64}$`)
	thirdPartyUUID      = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	thirdPartyTimestamp = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(Z|[+-]\d{2}:\d{2})$`)
)

// thirdPartyTime normalises a served timestamp to UTC whole seconds, YYYY-MM-DDTHH:MM:SSZ.
// A timestamp with a fractional second, even .0, is malformed.
func thirdPartyTime(value string) (string, bool) {
	if !thirdPartyTimestamp.MatchString(value) {
		return "", false
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return "", false
	}
	return t.UTC().Format("2006-01-02T15:04:05Z"), true
}

func verifyAsThirdParty(agentID string, att thirdPartyAttestation, jwks thirdPartyJWKSet) string {
	if att.Format != "opena2a-aim/card-attestation/v2" || att.Alg != "EdDSA" {
		return "unsupported-format"
	}
	issuedAt, okIssued := thirdPartyTime(att.IssuedAt)
	expiresAt, okExpires := thirdPartyTime(att.ExpiresAt)
	signature, sigErr := base64.StdEncoding.DecodeString(att.Signature)
	if !okIssued || !okExpires || sigErr != nil || len(signature) != ed25519.SignatureSize ||
		!thirdPartyHex64.MatchString(att.CardHash) || !thirdPartyUUID.MatchString(agentID) || att.Issuer != "aim-server" {
		return "malformed"
	}

	var publicKey ed25519.PublicKey
	for _, k := range jwks.Keys {
		if k.Kid == att.KeyID && k.Purpose == "card-attestation" && k.Kty == "OKP" && k.Crv == "Ed25519" {
			x, err := base64.RawURLEncoding.DecodeString(k.X)
			if err == nil && len(x) == ed25519.PublicKeySize {
				publicKey = x
			}
		}
	}
	if publicKey == nil {
		return "unknown-key"
	}

	payload := `{"cardHash":"` + att.CardHash + `","agentId":"` + agentID + `","issuer":"` + att.Issuer +
		`","issuedAt":"` + issuedAt + `","expiresAt":"` + expiresAt + `"}`
	digest := sha256.Sum256([]byte("opena2a-aim/card-attestation/v2\n" + payload))
	if !ed25519.Verify(publicKey, digest[:], signature) {
		return "signature-invalid"
	}
	return "verified"
}

// verifyServedCard verifies the attestation of a served agent card from the card JSON and
// the JWK Set JSON alone.
func verifyServedCard(t *testing.T, cardJSON, jwksJSON []byte) string {
	t.Helper()
	var card struct {
		AIM struct {
			AgentID     string                 `json:"agentId"`
			Attestation *thirdPartyAttestation `json:"attestation"`
		} `json:"aim"`
	}
	require.NoError(t, json.Unmarshal(cardJSON, &card))
	require.NotNil(t, card.AIM.Attestation, "the served card carries no attestation")
	var jwks thirdPartyJWKSet
	require.NoError(t, json.Unmarshal(jwksJSON, &jwks))
	return verifyAsThirdParty(card.AIM.AgentID, *card.AIM.Attestation, jwks)
}

// servedCardJSON is the agent card as GetEnhancedAgentCard serves it, with the attestation
// built from the stored card row.
func servedCardJSON(t *testing.T, card *domain.A2AAgentCard) []byte {
	t.Helper()
	body, err := json.Marshal(domain.A2AAgentCardParsed{
		Name:    "card",
		Version: "1.0.0",
		AIM: &domain.A2AAIMExtension{
			AgentID:     card.AgentID,
			Attestation: servedCardAttestation(card),
		},
	})
	require.NoError(t, err)
	return body
}

func jwksJSON(t *testing.T, ring *crypto.SigningKeyRing) []byte {
	t.Helper()
	body, err := json.Marshal(ring.JWKS())
	require.NoError(t, err)
	return body
}

// roundTripThroughStorage returns card as it reads back from PostgreSQL: TIMESTAMPTZ keeps
// microseconds, and the driver may hand back the instant in the session's time zone.
func roundTripThroughStorage(card *domain.A2AAgentCard) *domain.A2AAgentCard {
	zone := time.FixedZone("UTC-6", -6*60*60)
	stored := *card
	issuedAt := card.AttestationIssuedAt.Truncate(time.Microsecond).In(zone)
	expiresAt := card.AttestationExpiresAt.Truncate(time.Microsecond).In(zone)
	stored.AttestationIssuedAt = &issuedAt
	stored.AttestationExpiresAt = &expiresAt
	return &stored
}

func newRingWithCardAttestationEnv(t *testing.T, master []byte, env map[string]string) *crypto.SigningKeyRing {
	t.Helper()
	kv, err := crypto.NewKeyVault(base64.StdEncoding.EncodeToString(master))
	require.NoError(t, err)
	ring, err := crypto.LoadSigningKeyRing(kv, func(name string) string { return env[name] })
	require.NoError(t, err)
	return ring
}

func randomBytes(t *testing.T, n int) []byte {
	t.Helper()
	b := make([]byte, n)
	_, err := rand.Read(b)
	require.NoError(t, err)
	return b
}

// attestedCard registers an attestation for a new agent the way RegisterAgentCard does,
// signed now, with the clock's full precision.
func attestedCard(t *testing.T, ring *crypto.SigningKeyRing) *domain.A2AAgentCard {
	t.Helper()
	cardData := []byte(`{"name":"card","version":"1.0.0"}`)
	hash := sha256.Sum256(cardData)
	card := &domain.A2AAgentCard{
		ID:       uuid.New(),
		AgentID:  uuid.New(),
		CardData: cardData,
		CardHash: hex.EncodeToString(hash[:]),
	}
	s := &A2AService{cardAttestationKey: ring.Key(crypto.PurposeCardAttestation)}
	now := time.Date(2026, 10, 5, 12, 0, 0, 123456789, time.UTC)
	attestation, err := s.createCardAttestation(&domain.Agent{ID: card.AgentID}, card.CardHash, now, now.Add(24*time.Hour))
	require.NoError(t, err)
	attestation.applyTo(card)
	return card
}

func TestServedCardAttestationVerifiesFromTheCardAndTheJWKSetAlone(t *testing.T) {
	ring := newCardAttestationTestRing(t)
	card := roundTripThroughStorage(attestedCard(t, ring))

	served := servedCardJSON(t, card)
	assert.Equal(t, "verified", verifyServedCard(t, served, jwksJSON(t, ring)), "served card: %s", served)

	assert.Contains(t, string(served), `"issuedAt":"2026-10-05T12:00:00Z"`, "timestamps are served in UTC, whole seconds")
	assert.Contains(t, string(served), `"format":"opena2a-aim/card-attestation/v2"`)

	tampered := strings.Replace(string(served), card.CardHash, strings.Repeat("0", 64), 1)
	assert.Equal(t, "signature-invalid", verifyServedCard(t, []byte(tampered), jwksJSON(t, ring)))
}

func TestCardAttestationIsNotVerifiedWithAnotherPurposesKey(t *testing.T) {
	ring := newCardAttestationTestRing(t)
	card := attestedCard(t, ring)
	atcKeyID := ring.Key(crypto.PurposeATCIssuer).KeyID
	card.AttestationKeyID = &atcKeyID

	assert.Equal(t, "unknown-key", verifyServedCard(t, servedCardJSON(t, card), jwksJSON(t, ring)),
		"the ATC-issuer key is published with purpose atc-issuer and never verifies a card attestation")
}

func TestCardAttestationWithoutAFormatIsNotThirdPartyVerifiable(t *testing.T) {
	ring := newCardAttestationTestRing(t)
	card := attestedCard(t, ring)
	card.AttestationFormat = nil

	served := servedCardJSON(t, card)
	assert.NotContains(t, string(served), `"format"`)
	assert.Equal(t, "unsupported-format", verifyServedCard(t, served, jwksJSON(t, ring)))
}

func TestRotatedCardAttestationKeyVerifiesWithItsXCopiedFromTheJWKSet(t *testing.T) {
	master := randomBytes(t, 32)
	oldSeed := base64.StdEncoding.EncodeToString(randomBytes(t, 32))
	newSeed := base64.StdEncoding.EncodeToString(randomBytes(t, 32))

	before := newRingWithCardAttestationEnv(t, master, map[string]string{"AIM_SIGNING_KEY_CARD_ATTESTATION": oldSeed})
	card := attestedCard(t, before)

	// The operator copies the active key's x from the JWK Set served before the restart.
	var published thirdPartyJWKSet
	require.NoError(t, json.Unmarshal(jwksJSON(t, before), &published))
	oldX := ""
	for _, k := range published.Keys {
		if k.Purpose == "card-attestation" {
			oldX = k.X
		}
	}
	require.NotEmpty(t, oldX)

	rotatedWithoutRetired := newRingWithCardAttestationEnv(t, master, map[string]string{"AIM_SIGNING_KEY_CARD_ATTESTATION": newSeed})
	assert.Equal(t, "unknown-key", verifyServedCard(t, servedCardJSON(t, card), jwksJSON(t, rotatedWithoutRetired)))

	rotated := newRingWithCardAttestationEnv(t, master, map[string]string{
		"AIM_SIGNING_KEY_CARD_ATTESTATION":         newSeed,
		"AIM_SIGNING_KEY_CARD_ATTESTATION_RETIRED": oldX,
	})
	assert.Equal(t, "verified", verifyServedCard(t, servedCardJSON(t, card), jwksJSON(t, rotated)))
}

func TestRotatingTheMasterKeyMovesEveryDerivedKey(t *testing.T) {
	oldMaster, newMaster := randomBytes(t, 32), randomBytes(t, 32)
	before := newRingWithCardAttestationEnv(t, oldMaster, nil)
	after := newRingWithCardAttestationEnv(t, newMaster, nil)

	for _, key := range before.Keys() {
		assert.Equal(t, crypto.SigningKeySourceDerived, key.Source)
		assert.NotEqual(t, key.KeyID, after.Key(key.Purpose).KeyID, "%s moved with the master key", key.Purpose)
	}

	card := attestedCard(t, before)
	assert.Equal(t, "unknown-key", verifyServedCard(t, servedCardJSON(t, card), jwksJSON(t, after)))

	oldX := base64.RawURLEncoding.EncodeToString(before.Key(crypto.PurposeCardAttestation).PublicKey())
	afterWithRetired := newRingWithCardAttestationEnv(t, newMaster, map[string]string{
		"AIM_SIGNING_KEY_CARD_ATTESTATION_RETIRED": oldX,
	})
	assert.Equal(t, "verified", verifyServedCard(t, servedCardJSON(t, card), jwksJSON(t, afterWithRetired)))
}

type cardAttestationVector struct {
	Format string `json:"format"`
	Inputs struct {
		Seed      string `json:"seed"`
		Card      string `json:"card"`
		AgentID   string `json:"agentId"`
		IssuedAt  string `json:"issuedAt"`
		ExpiresAt string `json:"expiresAt"`
	} `json:"inputs"`
	JWKS         json.RawMessage `json:"jwks"`
	Intermediate struct {
		CardHash     string `json:"cardHash"`
		Payload      string `json:"payload"`
		SigningInput string `json:"signingInput"`
		Digest       string `json:"digest"`
	} `json:"intermediate"`
	Cases []struct {
		Name        string                `json:"name"`
		AgentID     string                `json:"agentId"`
		Attestation thirdPartyAttestation `json:"attestation"`
		Result      string                `json:"result"`
	} `json:"cases"`
}

func loadCardAttestationVector(t *testing.T) cardAttestationVector {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "docs", "specs", "card-attestation-v2-vector.json"))
	require.NoError(t, err)
	var vector cardAttestationVector
	require.NoError(t, json.Unmarshal(body, &vector))
	require.NotEmpty(t, vector.Cases)
	return vector
}

func TestCardAttestationConformanceVectorIsWhatTheServerSigns(t *testing.T) {
	vector := loadCardAttestationVector(t)
	require.Equal(t, CardAttestationFormatV2, vector.Format)

	ring := newRingWithCardAttestationEnv(t, randomBytes(t, 32), map[string]string{
		"AIM_SIGNING_KEY_CARD_ATTESTATION": vector.Inputs.Seed,
	})
	var published, expected thirdPartyJWKSet
	require.NoError(t, json.Unmarshal(jwksJSON(t, ring), &published))
	require.NoError(t, json.Unmarshal(vector.JWKS, &expected))
	assert.Equal(t, expected.Keys[0], published.Keys[0], "the vector's JWK is the card-attestation key the server publishes for the seed")

	hash := sha256.Sum256([]byte(vector.Inputs.Card))
	assert.Equal(t, vector.Intermediate.CardHash, hex.EncodeToString(hash[:]))

	issuedAt, err := time.Parse(time.RFC3339, vector.Inputs.IssuedAt)
	require.NoError(t, err)
	expiresAt, err := time.Parse(time.RFC3339, vector.Inputs.ExpiresAt)
	require.NoError(t, err)
	agentID := uuid.MustParse(vector.Inputs.AgentID)

	input, err := cardAttestationSigningInput(vector.Intermediate.CardHash, agentID, issuedAt, expiresAt)
	require.NoError(t, err)
	assert.Equal(t, vector.Intermediate.SigningInput, string(input))
	assert.Equal(t, CardAttestationFormatV2+"\n"+vector.Intermediate.Payload, string(input))
	digest := sha256.Sum256(input)
	assert.Equal(t, vector.Intermediate.Digest, hex.EncodeToString(digest[:]))

	card := &domain.A2AAgentCard{AgentID: agentID, CardHash: vector.Intermediate.CardHash}
	s := &A2AService{cardAttestationKey: ring.Key(crypto.PurposeCardAttestation)}
	attestation, err := s.createCardAttestation(&domain.Agent{ID: agentID}, card.CardHash, issuedAt, expiresAt)
	require.NoError(t, err)
	attestation.applyTo(card)

	servedBody, err := json.Marshal(servedCardAttestation(card))
	require.NoError(t, err)
	var served thirdPartyAttestation
	require.NoError(t, json.Unmarshal(servedBody, &served))
	assert.Equal(t, vector.Cases[0].Attestation, served, "the first case is the attestation exactly as served")
}

func TestCardAttestationConformanceVectorCases(t *testing.T) {
	vector := loadCardAttestationVector(t)
	var jwks thirdPartyJWKSet
	require.NoError(t, json.Unmarshal(vector.JWKS, &jwks))
	for _, c := range vector.Cases {
		t.Run(c.Name, func(t *testing.T) {
			assert.Equal(t, c.Result, verifyAsThirdParty(c.AgentID, c.Attestation, jwks))
		})
	}
}
