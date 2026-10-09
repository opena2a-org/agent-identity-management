package atc

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"testing"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	aimcrypto "github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto"
	atcdomain "github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/atc"
)

const separationIssuerURI = "https://aim.test.opena2a.org"

// newSeparatedKeys loads the server's per-purpose signing keys from a master key minted
// for this run, the way the server does at startup.
func newSeparatedKeys(t *testing.T) *aimcrypto.SigningKeyRing {
	t.Helper()
	master := make([]byte, 32)
	_, err := rand.Read(master)
	require.NoError(t, err)
	kv, err := aimcrypto.NewKeyVault(base64.StdEncoding.EncodeToString(master))
	require.NoError(t, err)
	ring, err := aimcrypto.LoadSigningKeyRing(kv, func(string) string { return "" })
	require.NoError(t, err)
	return ring
}

// serverVerifier builds the ATC verifier from the ring exactly as cmd/server/main.go wires it.
func serverVerifier(t *testing.T, ring *aimcrypto.SigningKeyRing, retired ...ed25519.PublicKey) *RealATCVerifier {
	t.Helper()
	verifier, err := NewServerATCVerifier(separationIssuerURI, ring.Key(aimcrypto.PurposeATCIssuer).PrivateKey(), nil, retired...)
	require.NoError(t, err)
	return verifier
}

func TestATCVerifierRefusesATCsSignedForOtherPurposes(t *testing.T) {
	ring := newSeparatedKeys(t)
	verifier := serverVerifier(t, ring)
	agentID := uuid.New()

	own := issueTestATC(t, ring.Key(aimcrypto.PurposeATCIssuer).PrivateKey(), agentID, separationIssuerURI, []string{"secrets:resolve"}, time.Minute)
	_, err := verifier.Verify(own)
	require.NoError(t, err, "the ATC issuer key must verify its own ATCs")

	issuerPublic := ring.Key(aimcrypto.PurposeATCIssuer).PublicKey()
	other := ring.Key(aimcrypto.PurposeCardAttestation).PrivateKey()

	// Signed with the card-attestation key and carrying it as the issuer key.
	forged := issueTestATC(t, other, agentID, separationIssuerURI, []string{"secrets:resolve"}, time.Minute)
	_, err = verifier.Verify(forged)
	assert.Error(t, err, "the ATC verifier accepted an ATC signed with the card-attestation key")

	// Signed with the card-attestation key while claiming the trusted issuer key.
	claimed := issueTestATCClaimingKey(t, other, issuerPublic, agentID)
	_, err = verifier.Verify(claimed)
	assert.Error(t, err, "the ATC verifier accepted an ATC signed with the card-attestation key under the issuer's public key")
}

func TestRetiredKeysStillVerifyAfterRotation(t *testing.T) {
	ring := newSeparatedKeys(t)
	agentID := uuid.New()
	_, retiredIssuer, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	_, stranger, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	verifier := serverVerifier(t, ring, retiredIssuer.Public().(ed25519.PublicKey))
	_, err = verifier.Verify(issueTestATC(t, retiredIssuer, agentID, separationIssuerURI, nil, time.Minute))
	assert.NoError(t, err, "an ATC signed by the retired issuer key must still verify")
	_, err = verifier.Verify(issueTestATC(t, stranger, agentID, separationIssuerURI, nil, time.Minute))
	assert.Error(t, err, "an ATC signed by an unlisted key verified")
}

func TestServerATCVerifierRefusesAMalformedRetiredKey(t *testing.T) {
	ring := newSeparatedKeys(t)
	_, err := NewServerATCVerifier(separationIssuerURI, ring.Key(aimcrypto.PurposeATCIssuer).PrivateKey(), nil, ed25519.PublicKey{1, 2, 3})
	assert.Error(t, err, "a retired issuer key of the wrong size was accepted")
}

// issueTestATCClaimingKey signs an ATC with signer while naming claimed as the issuer key.
func issueTestATCClaimingKey(t *testing.T, signer ed25519.PrivateKey, claimed ed25519.PublicKey, agentID uuid.UUID) string {
	t.Helper()
	now := time.Now()
	payload := atcdomain.ATCPayload{
		ATCID:           uuid.New().String(),
		AgentID:         agentID.String(),
		Issuer:          separationIssuerURI,
		IssuedAt:        uint64(now.Unix()),
		ExpiresAt:       uint64(now.Add(time.Minute).Unix()),
		IssuerPublicKey: []byte(claimed),
	}
	signedBytes, err := canonicalSignedPayload(&payload)
	require.NoError(t, err)
	payload.Ed25519Sig = ed25519.Sign(signer, signedBytes)
	encMode, err := cbor.CanonicalEncOptions().EncMode()
	require.NoError(t, err)
	atcBytes, err := encMode.Marshal(payload)
	require.NoError(t, err)
	return base64.RawURLEncoding.EncodeToString(atcBytes)
}
