package application

import (
	"context"
	"encoding/base64"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var checkListQuery = regexp.QuoteMeta(`SELECT id, organization_id, encrypted_private_key FROM agents`) +
	`[\s\S]*` + regexp.QuoteMeta(`ORDER BY organization_id, id`)

// The startup conversion reports a row whose key does not open only as a count. The
// check names each such agent and its organization, marks the v1 keys the conversion can
// still convert, and writes nothing: sqlmock fails on a statement it was not told to
// expect.
func TestCheckAgentPrivateKeys_NamesEachAgentWhoseKeyDoesNotOpenForItsOwnRow(t *testing.T) {
	kv := migrationTestVault(t)
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	orgA := uuid.MustParse("0a000000-0000-4000-8000-000000000000")
	orgB := uuid.MustParse("0b000000-0000-4000-8000-000000000000")
	ownerID, legacyID, copyID, garbageID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	ownersStored, err := kv.EncryptPrivateKey(ownerID, "owner-agent-private-key")
	require.NoError(t, err)
	legacy := sealV1(t, "legacy-agent-private-key")
	garbage := base64.StdEncoding.EncodeToString([]byte("not-a-ciphertext-of-either-format"))

	mock.ExpectQuery(migrationMarkerQuery).WithArgs(agentPrivateKeyMigrationVersion).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectQuery(checkListQuery).
		WillReturnRows(sqlmock.NewRows([]string{"id", "organization_id", "encrypted_private_key"}).
			AddRow(ownerID.String(), orgA.String(), ownersStored).
			AddRow(legacyID.String(), orgA.String(), legacy).
			AddRow(copyID.String(), orgB.String(), ownersStored). // copied from another row
			AddRow(garbageID.String(), orgB.String(), garbage))

	check, err := CheckAgentPrivateKeys(context.Background(), db, kv)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())

	assert.Equal(t, AgentPrivateKeyCheck{
		Read:               4,
		ConversionComplete: true,
		Unopenable: []UnopenableAgentPrivateKey{
			{AgentID: legacyID, OrganizationID: orgA, V1: true},
			{AgentID: copyID, OrganizationID: orgB},
			{AgentID: garbageID, OrganizationID: orgB},
		},
	}, check)
}

func TestCheckAgentPrivateKeys_EveryKeyOpensListsNone(t *testing.T) {
	kv := migrationTestVault(t)
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	agentID, orgID := uuid.New(), uuid.New()
	stored, err := kv.EncryptPrivateKey(agentID, "agent-private-key")
	require.NoError(t, err)

	mock.ExpectQuery(migrationMarkerQuery).WithArgs(agentPrivateKeyMigrationVersion).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectQuery(checkListQuery).
		WillReturnRows(sqlmock.NewRows([]string{"id", "organization_id", "encrypted_private_key"}).
			AddRow(agentID.String(), orgID.String(), stored))

	check, err := CheckAgentPrivateKeys(context.Background(), db, kv)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
	assert.Equal(t, AgentPrivateKeyCheck{Read: 1, Unopenable: []UnopenableAgentPrivateKey{}}, check)
}
