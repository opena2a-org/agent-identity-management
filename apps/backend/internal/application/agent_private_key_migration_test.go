package application

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"database/sql/driver"
	"encoding/base64"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const migrationTestMasterKey = "test-master-key-32-bytes-long!!!"

var (
	migrationMarkerQuery = regexp.QuoteMeta(`SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`)
	migrationListQuery   = regexp.QuoteMeta(`SELECT id, encrypted_private_key FROM agents`)
	migrationUpdateExec  = regexp.QuoteMeta(`UPDATE agents SET encrypted_private_key = $1 WHERE id = $2 AND encrypted_private_key = $3`)
	migrationRecordExec  = regexp.QuoteMeta(`INSERT INTO schema_migrations (version) VALUES ($1) ON CONFLICT (version) DO NOTHING`)
)

func migrationTestVault(t *testing.T) *crypto.KeyVault {
	t.Helper()
	kv, err := crypto.NewKeyVault(base64.StdEncoding.EncodeToString([]byte(migrationTestMasterKey)))
	require.NoError(t, err)
	return kv
}

// sealV1 writes the format stored before storage binding: nonce || AES-256-GCM
// with no additional data. The fixed nonce keeps the leading byte away from
// the v2 version byte.
func sealV1(t *testing.T, plaintext string) string {
	t.Helper()
	block, err := aes.NewCipher([]byte(migrationTestMasterKey))
	require.NoError(t, err)
	gcm, err := cipher.NewGCM(block)
	require.NoError(t, err)
	nonce := make([]byte, gcm.NonceSize())
	for i := range nonce {
		nonce[i] = byte(0x20 + i)
	}
	return base64.StdEncoding.EncodeToString(gcm.Seal(nonce, nonce, []byte(plaintext), nil))
}

// capture records the value bound to a statement argument.
type capture struct{ value string }

func (c *capture) Match(v driver.Value) bool {
	s, ok := v.(string)
	c.value = s
	return ok
}

func TestMigrateAgentPrivateKeysToV2_ConvertsEveryRowAndRecordsCompletion(t *testing.T) {
	kv := migrationTestVault(t)
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	legacyID, currentID := uuid.New(), uuid.New()
	legacy := sealV1(t, "legacy-agent-private-key")
	current, err := kv.EncryptPrivateKey(currentID, "current-agent-private-key")
	require.NoError(t, err)

	written := &capture{}
	mock.ExpectQuery(migrationMarkerQuery).WithArgs(agentPrivateKeyMigrationVersion).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectQuery(migrationListQuery).
		WillReturnRows(sqlmock.NewRows([]string{"id", "encrypted_private_key"}).
			AddRow(legacyID.String(), legacy).
			AddRow(currentID.String(), current))
	mock.ExpectExec(migrationUpdateExec).WithArgs(written, legacyID, legacy).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(migrationRecordExec).WithArgs(agentPrivateKeyMigrationVersion).
		WillReturnResult(sqlmock.NewResult(1, 1))

	result, err := MigrateAgentPrivateKeysToV2(context.Background(), db, kv)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())

	assert.Equal(t, AgentPrivateKeyMigrationResult{V1Read: 1, V2Written: 1, AlreadyV2: 1, Recorded: true}, result)

	// The rewritten value is v2 bound to its own row.
	got, err := kv.DecryptPrivateKey(legacyID, written.value)
	require.NoError(t, err)
	assert.Equal(t, "legacy-agent-private-key", got)
	_, err = kv.DecryptPrivateKey(currentID, written.value)
	assert.Error(t, err)
}

func TestMigrateAgentPrivateKeysToV2_FailuresAreCountedAndLeaveItPending(t *testing.T) {
	kv := migrationTestVault(t)
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	logs := captureLog(t)

	legacyID, ownerID, copyID, garbageID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	plantedKey := "planted-agent-private-key"
	legacy := sealV1(t, plantedKey)
	ownersV2, err := kv.EncryptPrivateKey(ownerID, "owner-agent-private-key")
	require.NoError(t, err)
	garbage := base64.StdEncoding.EncodeToString([]byte("not-a-ciphertext-of-either-format"))

	written := &capture{}
	mock.ExpectQuery(migrationMarkerQuery).WithArgs(agentPrivateKeyMigrationVersion).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectQuery(migrationListQuery).
		WillReturnRows(sqlmock.NewRows([]string{"id", "encrypted_private_key"}).
			AddRow(legacyID.String(), legacy).
			AddRow(ownerID.String(), ownersV2).
			AddRow(copyID.String(), ownersV2). // v2 copied from another row
			AddRow(garbageID.String(), garbage))
	mock.ExpectExec(migrationUpdateExec).WithArgs(written, legacyID, legacy).
		WillReturnResult(sqlmock.NewResult(0, 1))
	// No completion record while any row failed.

	result, err := MigrateAgentPrivateKeysToV2(context.Background(), db, kv)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())

	assert.Equal(t, AgentPrivateKeyMigrationResult{V1Read: 1, V2Written: 1, AlreadyV2: 1, Failures: 2}, result)

	// The log holds counts only: no key material, no ciphertext, no agent ID.
	out := logs.String()
	assert.Contains(t, out, "v1Read=1 v2Written=1 alreadyV2=1 failures=2 recorded=false")
	for _, secret := range []string{plantedKey, legacy, ownersV2, garbage, written.value,
		legacyID.String(), ownerID.String(), copyID.String(), garbageID.String()} {
		assert.NotContains(t, out, secret)
	}
}

func TestMigrateAgentPrivateKeysToV2_RowChangedDuringRunIsNotCountedWritten(t *testing.T) {
	kv := migrationTestVault(t)
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	legacyID := uuid.New()
	legacy := sealV1(t, "legacy-agent-private-key")

	mock.ExpectQuery(migrationMarkerQuery).WithArgs(agentPrivateKeyMigrationVersion).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectQuery(migrationListQuery).
		WillReturnRows(sqlmock.NewRows([]string{"id", "encrypted_private_key"}).AddRow(legacyID.String(), legacy))
	mock.ExpectExec(migrationUpdateExec).WithArgs(sqlmock.AnyArg(), legacyID, legacy).
		WillReturnResult(sqlmock.NewResult(0, 0))

	result, err := MigrateAgentPrivateKeysToV2(context.Background(), db, kv)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())

	assert.Equal(t, AgentPrivateKeyMigrationResult{V1Read: 1, Failures: 1}, result)
}

// Once the migration is recorded, a v1 ciphertext planted in a row later is
// never read by it again, and the request path refuses it.
func TestMigrateAgentPrivateKeysToV2_AfterCompletionV1IsNeverRead(t *testing.T) {
	kv := migrationTestVault(t)
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery(migrationMarkerQuery).WithArgs(agentPrivateKeyMigrationVersion).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

	result, err := MigrateAgentPrivateKeysToV2(context.Background(), db, kv)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet(), "a completed migration must not list agent keys")
	assert.Equal(t, AgentPrivateKeyMigrationResult{AlreadyComplete: true}, result)

	plantedKey := "planted-agent-private-key"
	planted := sealV1(t, plantedKey)
	publicKey := "planted-row-public-key"
	agent := &domain.Agent{ID: uuid.New(), PublicKey: &publicKey, EncryptedPrivateKey: &planted}
	repo := new(MockAgentRepository)
	repo.On("GetByID", agent.ID).Return(agent, nil)
	service := &AgentService{agentRepo: repo, keyVault: kv}

	_, gotPrivate, err := service.GetAgentCredentials(context.Background(), agent.ID)
	require.Error(t, err)
	assert.ErrorIs(t, err, crypto.ErrUnsupportedPrivateKeyFormat)
	assert.Empty(t, gotPrivate)
	assert.NotContains(t, err.Error(), plantedKey)
}
