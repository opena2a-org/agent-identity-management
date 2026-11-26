package application

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto/ed25519key"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto/ed25519key/ed25519keytest"
	infracrypto "github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/crypto"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
)

// newKeyTestService returns an MCPService on a mock database. A statement the
// test did not expect fails it, so nothing can be stored unannounced.
func newKeyTestService(t *testing.T) (*MCPService, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return &MCPService{
		mcpRepo:       repository.NewMCPServerRepository(db),
		cryptoService: infracrypto.NewED25519Service(),
		challenges:    make(map[string]ChallengeData),
	}, mock
}

// expectServerRow answers MCPServerRepository.GetByID with a server whose
// stored key is publicKey.
func expectServerRow(mock sqlmock.Sqlmock, serverID uuid.UUID, publicKey string) {
	now := time.Now().UTC()
	mock.ExpectQuery(`FROM mcp_servers\s+WHERE id = \$1`).
		WithArgs(serverID).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "organization_id", "name", "description", "url", "version",
			"public_key", "status", "is_verified", "last_verified_at", "verification_url",
			"capabilities", "trust_score", "registered_by_agent", "created_by", "created_at", "updated_at",
			"verification_method", "attestation_count", "confidence_score", "last_attested_at",
			"created_by_name", "created_by_email", "created_by_sdk_token_id", "created_by_api_key_id",
			"updated_by", "updated_by_name", "updated_by_email",
		}).AddRow(
			serverID, uuid.New(), "files", nil, "https://files.example.com", nil,
			publicKey, "pending", false, nil, nil,
			[]byte("[]"), 0.5, nil, uuid.New(), now, now,
			"", 0, 0.0, nil,
			"", "", nil, nil,
			nil, "", "",
		))
}

// eachRefusedKey runs run for each small-order encoding and the off-curve
// encoding, under every spelling of the supported key type.
func eachRefusedKey(t *testing.T, run func(t *testing.T, encoded, keyType string)) {
	t.Helper()
	keys := ed25519key.SmallOrderEncodings()
	keys = append(keys, ed25519keytest.OffCurve())
	for _, key := range keys {
		require.False(t, ed25519key.Acceptable(key))
		encoded := base64.StdEncoding.EncodeToString(key)
		for _, keyType := range []string{"ed25519", "ED25519", "Ed25519"} {
			encoded, keyType := encoded, keyType
			t.Run(keyType+"/"+encoded, func(t *testing.T) { run(t, encoded, keyType) })
		}
	}
}

func TestToolServerKeyIsRefusedWhenItsTypeIsEmptyOmittedOrUnsupported(t *testing.T) {
	genuine := newEd25519PublicKey(t)
	for name, req := range map[string]*AddPublicKeyRequest{
		"empty":         {PublicKey: genuine, KeyType: ""},
		"blank":         {PublicKey: genuine, KeyType: " "},
		"unsupported":   {PublicKey: genuine, KeyType: "rsa"},
		"ecdsa":         {PublicKey: genuine, KeyType: "ecdsa"},
		"padded":        {PublicKey: genuine, KeyType: " ed25519"},
		"empty, no key": {},
	} {
		t.Run(name, func(t *testing.T) {
			svc, mock := newKeyTestService(t)

			err := svc.AddPublicKey(context.Background(), uuid.New(), req)

			assert.ErrorIs(t, err, ErrPublicKeyTypeNotSupported)
			assert.NoError(t, mock.ExpectationsWereMet(), "no statement was issued, so nothing is stored")
		})
	}
}

func TestToolServerKeyWithTheTypeOmittedFromTheJSONIsRefused(t *testing.T) {
	var req AddPublicKeyRequest
	require.NoError(t, json.Unmarshal([]byte(fmt.Sprintf(`{"publicKey":%q}`, newEd25519PublicKey(t))), &req))
	svc, mock := newKeyTestService(t)

	err := svc.AddPublicKey(context.Background(), uuid.New(), &req)

	assert.ErrorIs(t, err, ErrPublicKeyTypeNotSupported)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestToolServerKeyThatIsNotAcceptableIsRefusedUnderEverySpellingAndNothingIsStored(t *testing.T) {
	eachRefusedKey(t, func(t *testing.T, encoded, keyType string) {
		svc, mock := newKeyTestService(t)

		err := svc.AddPublicKey(context.Background(), uuid.New(), &AddPublicKeyRequest{PublicKey: encoded, KeyType: keyType})

		assert.ErrorIs(t, err, ErrPublicKeyNotAccepted)
		assert.NoError(t, mock.ExpectationsWereMet(), "no statement was issued, so nothing is stored")
	})
}

func TestToolServerGenuineKeyIsStoredAndVerifiesAGenuineSignature(t *testing.T) {
	for _, keyType := range []string{"ed25519", "ED25519", "Ed25519"} {
		t.Run(keyType, func(t *testing.T) {
			pub, priv, err := ed25519.GenerateKey(rand.Reader)
			require.NoError(t, err)
			encoded := base64.StdEncoding.EncodeToString(pub)
			serverID := uuid.New()
			svc, mock := newKeyTestService(t)

			expectServerRow(mock, serverID, "")
			mock.ExpectExec(`INSERT INTO mcp_server_keys`).
				WithArgs(sqlmock.AnyArg(), serverID, encoded, keyType, sqlmock.AnyArg()).
				WillReturnResult(sqlmock.NewResult(1, 1))
			require.NoError(t, svc.AddPublicKey(context.Background(), serverID, &AddPublicKeyRequest{PublicKey: encoded, KeyType: keyType}))
			require.NoError(t, mock.ExpectationsWereMet())

			// The server's own verification, with the key the server holds.
			challenge, err := svc.cryptoService.GenerateChallenge()
			require.NoError(t, err)
			svc.challenges[serverID.String()] = ChallengeData{Challenge: challenge, ServerID: serverID, ExpiresAt: time.Now().Add(time.Minute)}
			expectServerRow(mock, serverID, encoded)
			signature := base64.StdEncoding.EncodeToString(ed25519.Sign(priv, []byte(challenge)))

			assert.NoError(t, svc.VerifyChallengeResponse(context.Background(), serverID, signature))
		})
	}
}

// A row stored before keys were checked may hold a key the helper refuses. The
// server's verification under it verifies nothing, a forged signature included.
func TestToolServerStoredKeyThatIsRefusedVerifiesNothing(t *testing.T) {
	small := ed25519key.SmallOrderEncodings()
	keys := append(small, ed25519keytest.OffCurve())
	for _, key := range keys {
		encoded := base64.StdEncoding.EncodeToString(key)
		t.Run(encoded, func(t *testing.T) {
			serverID := uuid.New()
			challenge, signature := func() (string, []byte) {
				sig := forgedSmallOrderSignature()
				for i := 0; i < 10000; i++ {
					c := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("challenge-%d", i)))
					if ed25519.Verify(ed25519.PublicKey(key), []byte(c), sig) {
						return c, sig
					}
				}
				return "challenge", sig // off-curve: plain Verify refuses every message
			}()
			svc, mock := newKeyTestService(t)
			svc.challenges[serverID.String()] = ChallengeData{Challenge: challenge, ServerID: serverID, ExpiresAt: time.Now().Add(time.Minute)}
			expectServerRow(mock, serverID, encoded)

			err := svc.VerifyChallengeResponse(context.Background(), serverID, base64.StdEncoding.EncodeToString(signature))

			assert.Error(t, err)
			assert.ErrorIs(t, err, ed25519key.ErrNotAccepted)
			assert.Contains(t, svc.challenges, serverID.String(), "the challenge is not consumed by a failed verification")
		})
	}
}
