package application

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

func newCardAttestationTestRing(t *testing.T) *crypto.SigningKeyRing {
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

func TestCardAttestationRecordsTheCardAttestationKey(t *testing.T) {
	ring := newCardAttestationTestRing(t)
	key := ring.Key(crypto.PurposeCardAttestation)
	s := &A2AService{cardAttestationKey: key}

	attestation, err := s.createCardAttestation(&domain.Agent{ID: uuid.New()}, json.RawMessage(`{"name":"card"}`), time.Now().Add(time.Hour))
	require.NoError(t, err)

	assert.Equal(t, key.KeyID, attestation.keyID, "the attestation must name the card-attestation key")
	assert.Equal(t, "EdDSA", attestation.alg)
	assert.NotEqual(t, ring.Key(crypto.PurposeATCIssuer).KeyID, attestation.keyID)

	signature, err := base64.StdEncoding.DecodeString(attestation.signature)
	require.NoError(t, err)
	assert.Len(t, signature, ed25519.SignatureSize)
}

func TestCardAttestationWithoutAKeyIsRefused(t *testing.T) {
	s := &A2AService{}
	attestation, err := s.createCardAttestation(&domain.Agent{ID: uuid.New()}, json.RawMessage(`{"name":"card"}`), time.Now().Add(time.Hour))
	require.Error(t, err)
	assert.Nil(t, attestation)
}
