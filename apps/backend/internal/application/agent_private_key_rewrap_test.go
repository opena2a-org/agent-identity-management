package application

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	rewrapFirstPageQuery = `SELECT id, encrypted_private_key FROM agents\s+WHERE encrypted_private_key IS NOT NULL AND encrypted_private_key <> ''\s+ORDER BY id LIMIT \$1`
	rewrapNextPageQuery  = `SELECT id, encrypted_private_key FROM agents\s+WHERE encrypted_private_key IS NOT NULL AND encrypted_private_key <> '' AND id > \$1\s+ORDER BY id LIMIT \$2`
)

// rewrapTestKey returns a fixed base64 master key per label.
func rewrapTestKey(label string) string {
	sum := sha256.Sum256([]byte("rewrap test key " + label))
	return base64.StdEncoding.EncodeToString(sum[:])
}

func rewrapTestRing(t *testing.T, current byte, labels map[byte]string) *crypto.KeyVault {
	t.Helper()
	keys := make(map[byte]string, len(labels))
	for id, label := range labels {
		keys[id] = rewrapTestKey(label)
	}
	kv, err := crypto.NewKeyVaultRing(current, keys)
	require.NoError(t, err)
	return kv
}

func TestRewrapAgentPrivateKeys_ReadsNothingWithOneKeyInTheRing(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	result, err := RewrapAgentPrivateKeys(context.Background(), db, rewrapTestRing(t, 2, map[byte]string{2: "two"}))
	require.NoError(t, err)
	assert.True(t, result.Skipped)
	assert.Equal(t, byte(2), result.StorageKeyID)
	assert.Empty(t, result.Remaining)
	require.NoError(t, mock.ExpectationsWereMet(), "no statement runs")
}

func TestRewrapAgentPrivateKeys_RewritesOlderIDsAndCountsByID(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	ring := rewrapTestRing(t, 2, map[byte]string{1: "one", 2: "two"})
	older := rewrapTestRing(t, 1, map[byte]string{1: "one"})
	outside := rewrapTestRing(t, 9, map[byte]string{9: "nine"})

	olderRow, currentRow, outsideRow, legacyRow, changedRow, misboundRow, undecodableRow :=
		uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()

	underOlder, err := older.EncryptPrivateKey(olderRow, "older-row-key")
	require.NoError(t, err)
	underCurrent, err := ring.EncryptPrivateKey(currentRow, "current-row-key")
	require.NoError(t, err)
	underOutside, err := outside.EncryptPrivateKey(outsideRow, "outside-row-key")
	require.NoError(t, err)
	changedUnderOlder, err := older.EncryptPrivateKey(changedRow, "changed-row-key")
	require.NoError(t, err)
	// A ciphertext under id 1 bound to another row: it does not decrypt here.
	misbound, err := older.EncryptPrivateKey(uuid.New(), "misbound-row-key")
	require.NoError(t, err)

	mock.ExpectQuery(rewrapFirstPageQuery).
		WithArgs(agentPrivateKeyRewrapBatchSize).
		WillReturnRows(sqlmock.NewRows([]string{"id", "encrypted_private_key"}).
			AddRow(olderRow.String(), underOlder).
			AddRow(currentRow.String(), underCurrent).
			AddRow(outsideRow.String(), underOutside).
			AddRow(legacyRow.String(), sealV1(t, "legacy-row-key")).
			AddRow(changedRow.String(), changedUnderOlder).
			AddRow(misboundRow.String(), misbound).
			AddRow(undecodableRow.String(), "not base64!"))

	written := &capture{}
	mock.ExpectExec(migrationUpdateExec).
		WithArgs(written, olderRow.String(), underOlder).
		WillReturnResult(sqlmock.NewResult(0, 1))
	// Rewritten by someone else since it was read: the compare-and-swap
	// matches no row.
	mock.ExpectExec(migrationUpdateExec).
		WithArgs(sqlmock.AnyArg(), changedRow.String(), changedUnderOlder).
		WillReturnResult(sqlmock.NewResult(0, 0))

	result, err := RewrapAgentPrivateKeys(context.Background(), db, ring)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())

	assert.False(t, result.Skipped)
	assert.Equal(t, byte(2), result.StorageKeyID)
	assert.Equal(t, 1, result.Rewritten)
	assert.Equal(t, 1, result.AlreadyCurrent)
	assert.Equal(t, 2, result.Failures, "the changed row and the row that does not decrypt")
	assert.Equal(t, 2, result.NotV2, "the v1 row and the undecodable row")
	assert.Equal(t, map[byte]int{1: 2, 9: 1}, result.Remaining)

	id, err := crypto.PrivateKeyStorageKeyID(written.value)
	require.NoError(t, err)
	assert.Equal(t, byte(2), id, "the row is rewritten under the current id")
	got, err := ring.DecryptPrivateKey(olderRow, written.value)
	require.NoError(t, err)
	assert.Equal(t, "older-row-key", got)
	_, err = older.DecryptPrivateKey(olderRow, written.value)
	assert.Error(t, err, "the rewritten row is no longer under the older key")
}

func TestRewrapAgentPrivateKeys_ListsEveryOlderRingIDEvenAtZero(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	ring := rewrapTestRing(t, 3, map[byte]string{1: "one", 2: "two", 3: "three"})
	row := uuid.New()
	underCurrent, err := ring.EncryptPrivateKey(row, "current-row-key")
	require.NoError(t, err)

	mock.ExpectQuery(rewrapFirstPageQuery).
		WithArgs(agentPrivateKeyRewrapBatchSize).
		WillReturnRows(sqlmock.NewRows([]string{"id", "encrypted_private_key"}).AddRow(row.String(), underCurrent))

	result, err := RewrapAgentPrivateKeys(context.Background(), db, ring)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
	assert.Equal(t, map[byte]int{1: 0, 2: 0}, result.Remaining)
	assert.Equal(t, 1, result.AlreadyCurrent)
	assert.Equal(t, "1:0 2:0", formatRemaining(result.Remaining))
}

func TestRewrapAgentPrivateKeys_ReadsInBatchesAfterTheLastID(t *testing.T) {
	saved := agentPrivateKeyRewrapBatchSize
	agentPrivateKeyRewrapBatchSize = 2
	t.Cleanup(func() { agentPrivateKeyRewrapBatchSize = saved })

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	ring := rewrapTestRing(t, 2, map[byte]string{1: "one", 2: "two"})
	older := rewrapTestRing(t, 1, map[byte]string{1: "one"})
	rows := []uuid.UUID{
		uuid.MustParse("00000000-0000-4000-8000-000000000001"),
		uuid.MustParse("00000000-0000-4000-8000-000000000002"),
		uuid.MustParse("00000000-0000-4000-8000-000000000003"),
	}
	stored := make([]string, len(rows))
	for i, row := range rows {
		stored[i], err = older.EncryptPrivateKey(row, "batched-key")
		require.NoError(t, err)
	}

	mock.ExpectQuery(rewrapFirstPageQuery).
		WithArgs(2).
		WillReturnRows(sqlmock.NewRows([]string{"id", "encrypted_private_key"}).
			AddRow(rows[0].String(), stored[0]).
			AddRow(rows[1].String(), stored[1]))
	mock.ExpectExec(migrationUpdateExec).WithArgs(sqlmock.AnyArg(), rows[0].String(), stored[0]).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(migrationUpdateExec).WithArgs(sqlmock.AnyArg(), rows[1].String(), stored[1]).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(rewrapNextPageQuery).
		WithArgs(rows[1].String(), 2).
		WillReturnRows(sqlmock.NewRows([]string{"id", "encrypted_private_key"}).
			AddRow(rows[2].String(), stored[2]))
	mock.ExpectExec(migrationUpdateExec).WithArgs(sqlmock.AnyArg(), rows[2].String(), stored[2]).WillReturnResult(sqlmock.NewResult(0, 1))

	result, err := RewrapAgentPrivateKeys(context.Background(), db, ring)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
	assert.Equal(t, 3, result.Rewritten)
	assert.Equal(t, map[byte]int{1: 0}, result.Remaining)
}

func TestRewrapAgentPrivateKeys_ListingErrorIsReturned(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery(rewrapFirstPageQuery).WillReturnError(errors.New("connection reset"))

	_, err = RewrapAgentPrivateKeys(context.Background(), db, rewrapTestRing(t, 2, map[byte]string{1: "one", 2: "two"}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to list stored private keys")
	require.NoError(t, mock.ExpectationsWereMet())
}
