package repository

import (
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// CreateBatch is how the audit batcher writes the queued verification
// entries: one insert for the batch.

func TestAuditLogCreateBatch_WritesTheBatchInOneInsert(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := NewAuditLogRepository(db)

	var given []*domain.AuditLog
	repo.OnCreate(func(entry *domain.AuditLog) { given = append(given, entry) })

	nobody := uuid.Nil
	entries := []*domain.AuditLog{
		{OrganizationID: uuid.New(), UserID: &nobody, Action: domain.AuditActionVerify, ResourceType: "agent_capability", ResourceID: uuid.New()},
		{OrganizationID: uuid.New(), Action: domain.AuditActionVerify, ResourceType: "agent_capability", ResourceID: uuid.New()},
	}
	mock.ExpectExec(regexp.QuoteMeta(`VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11), ($12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22)`)).
		WillReturnResult(sqlmock.NewResult(0, 2))
	require.NoError(t, repo.CreateBatch(entries))
	require.NoError(t, mock.ExpectationsWereMet())

	require.Len(t, given, 2)
	for i, entry := range entries {
		assert.Same(t, entry, given[i])
		assert.NotEqual(t, uuid.Nil, entry.ID)
		assert.False(t, entry.Timestamp.IsZero())
	}
	assert.Nil(t, entries[0].UserID, "uuid.Nil names no user")
}

func TestAuditLogCreateBatch_FailedInsertGivesNothingToOnCreate(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := NewAuditLogRepository(db)

	called := false
	repo.OnCreate(func(*domain.AuditLog) { called = true })

	mock.ExpectExec(`INSERT INTO audit_logs`).WillReturnError(errors.New("connection refused"))
	err = repo.CreateBatch([]*domain.AuditLog{{OrganizationID: uuid.New(), Action: domain.AuditActionVerify}})

	require.Error(t, err)
	assert.False(t, called)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAuditLogCreateBatch_EmptyAndOversizedBatches(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := NewAuditLogRepository(db)

	require.NoError(t, repo.CreateBatch(nil))
	require.Error(t, repo.CreateBatch(make([]*domain.AuditLog, AuditLogBatchMaxRows+1)))
	require.NoError(t, mock.ExpectationsWereMet(), "neither runs an insert")
}
