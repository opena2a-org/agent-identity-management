package application

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto/ed25519key"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/secrets"
)

// These tests pin one rule at every place an Ed25519 public key is accepted and
// stored, or a stored key verifies a signature: a key that
// ed25519key.Acceptable refuses is not stored, and verifies nothing. Each is
// run for the fourteen small-order encodings, with a forged signature that
// plain ed25519.Verify accepts under them over a message the test chooses.

func eachSmallOrderKey(t *testing.T, run func(t *testing.T, key []byte, encoded string)) {
	t.Helper()
	keys := ed25519key.SmallOrderEncodings()
	require.Len(t, keys, 14)
	for _, key := range keys {
		key := key
		encoded := base64.StdEncoding.EncodeToString(key)
		t.Run(encoded, func(t *testing.T) { run(t, key, encoded) })
	}
}

// forgeFor returns the first message built(i) under which plain ed25519.Verify
// accepts the forged signature for key, and that signature.
func forgeFor(t *testing.T, key []byte, built func(i int) []byte) ([]byte, []byte) {
	t.Helper()
	sig := forgedSmallOrderSignature()
	for i := 0; i < 10000; i++ {
		message := built(i)
		if ed25519.Verify(ed25519.PublicKey(key), message, sig) {
			return message, sig
		}
	}
	t.Fatalf("no message found for which the forged signature verifies under %x", key)
	return nil, nil
}

func TestBootstrapExchangeRefusesSmallOrderKeysAndRegistersNothing(t *testing.T) {
	eachSmallOrderKey(t, func(t *testing.T, _ []byte, encoded string) {
		rig := newBootstrapTestRig(t)
		minted := rig.mint(t, uuid.New(), uuid.New())

		_, err := rig.exchange(minted.Plaintext, BootstrapExchangeRequest{PublicKey: encoded})

		assert.ErrorIs(t, err, ErrBootstrapInvalidPublicKey)
		assert.Empty(t, rig.agents.registrations(), "nothing is stored")
		_, err = rig.exchange(minted.Plaintext, BootstrapExchangeRequest{PublicKey: newEd25519PublicKey(t)})
		assert.NoError(t, err, "the refusal did not consume the token")
	})
}

func TestMCPServerKeysRefuseSmallOrderKeysBeforeAnythingIsStored(t *testing.T) {
	// The service has no repository: a refusal that reached it would panic.
	svc := &MCPService{}
	eachSmallOrderKey(t, func(t *testing.T, _ []byte, encoded string) {
		_, err := svc.CreateMCPServer(context.Background(),
			&CreateMCPServerRequest{Name: "m", URL: "https://mcp.example.com", PublicKey: encoded},
			uuid.New(), uuid.New(), nil, nil, nil)
		assert.ErrorIs(t, err, ErrPublicKeyNotAccepted, "create")

		_, err = svc.UpdateMCPServer(context.Background(), uuid.New(), &UpdateMCPServerRequest{PublicKey: encoded})
		assert.ErrorIs(t, err, ErrPublicKeyNotAccepted, "update")

		err = svc.AddPublicKey(context.Background(), uuid.New(), &AddPublicKeyRequest{PublicKey: encoded, KeyType: "ed25519"})
		assert.ErrorIs(t, err, ErrPublicKeyNotAccepted, "add key")
	})
}

func TestCapabilitySignatureUnderAStoredSmallOrderKeyVerifiesNothing(t *testing.T) {
	service := &CapabilityService{}
	eachSmallOrderKey(t, func(t *testing.T, key []byte, encoded string) {
		payload, sig := forgeFor(t, key, func(i int) []byte { return []byte(fmt.Sprintf("payload-%d", i)) })

		assert.True(t, ed25519.Verify(ed25519.PublicKey(key), payload, sig), "the forgery works for plain Verify")
		assert.False(t, service.verifySignature(encoded, "Ed25519", sig, payload))
	})
}

func TestSecretsResolveUnderAStoredSmallOrderKeyIsNotSignedByTheAgent(t *testing.T) {
	eachSmallOrderKey(t, func(t *testing.T, key []byte, encoded string) {
		svc, _, _, agentRepo, _, _, _ := setupTestService(t)
		agentID, _, _ := createTestAgent(t, agentRepo)
		agentRepo.agents[agentID].PublicKey = &encoded
		stamp := time.Now().Format(time.RFC3339Nano)
		nonce := func(i int) string { return fmt.Sprintf("%s:%d", stamp, i) }
		message, sig := forgeFor(t, key, func(i int) []byte { return []byte("github|read|" + nonce(i)) })
		var chosen string
		for i := 0; i < 10000; i++ {
			if "github|read|"+nonce(i) == string(message) {
				chosen = nonce(i)
				break
			}
		}
		require.NotEmpty(t, chosen)

		result, err := svc.Resolve(agentID, &secrets.ResolutionRequest{
			Namespace: "github", Operation: "read", Nonce: chosen, Signature: sig,
		})

		assert.Nil(t, result)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid agent public key")
	})
}

func TestA2ARequestUnderAStoredSmallOrderKeyIsNotSignedByTheAgent(t *testing.T) {
	eachSmallOrderKey(t, func(t *testing.T, key []byte, encoded string) {
		id, orgID := uuid.New(), uuid.New()
		hash, sig := forgeFor(t, key, func(i int) []byte {
			h := make([]byte, 32)
			h[0], h[1] = byte(i), byte(i>>8)
			return h
		})
		agentDB, agentMock := newSQLMock(t)
		nonceDB, nonceMock := newSQLMock(t)
		svc := a2aServiceForGrants(t, agentDB, nonceDB, &fakeCapabilityReader{})
		req := VerifyA2ARequestRequest{
			AgentID: id, Timestamp: time.Now().UTC().Unix(), Nonce: uuid.New().String(),
			Signature: base64.StdEncoding.EncodeToString(sig), RequestHash: hex.EncodeToString(hash),
		}
		expectAgentRowInStatus(agentMock, id, orgID, encoded, domain.AgentStatusVerified)
		nonceMock.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM a2a_request_nonces WHERE nonce = \$1\)`).
			WithArgs(req.Nonce).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

		result, err := svc.VerifyA2ARequest(context.Background(), req)

		require.NoError(t, err)
		assert.False(t, result.Valid)
		assert.Equal(t, "invalid public key format", result.Error)
	})
}
