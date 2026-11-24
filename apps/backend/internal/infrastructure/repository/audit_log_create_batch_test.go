package repository

import (
	"database/sql/driver"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// CreateBatch is how the verification audit queue writes the decisions it has
// gathered: one statement for the batch, a stored id skipped, and each written
// entry given to OnCreate, so the SIEM sees every row as it does from Create.

func batchEntries(n int) []*domain.AuditLog {
	entries := make([]*domain.AuditLog, n)
	for i := range entries {
		agentID := uuid.New()
		entries[i] = &domain.AuditLog{
			OrganizationID: uuid.New(),
			AgentID:        &agentID,
			Action:         "db:write",
			ResourceType:   "agent_action",
			ResourceID:     agentID,
			Metadata:       map[string]interface{}{"i": i},
		}
	}
	return entries
}

func TestAuditLogCreateBatch_WritesTheBatchWithOneStatement(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := NewAuditLogRepository(db)

	var given []*domain.AuditLog
	repo.OnCreate(func(entry *domain.AuditLog) { given = append(given, entry) })

	entries := batchEntries(3)
	args := make([]driver.Value, 3*auditLogInsertColumnCount)
	for i := range args {
		args[i] = sqlmock.AnyArg()
	}
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO audit_logs (` + auditLogInsertColumns + `) VALUES ` +
		`($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11), ` +
		`($12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22), ` +
		`($23, $24, $25, $26, $27, $28, $29, $30, $31, $32, $33) ON CONFLICT (id) DO NOTHING`)).
		WithArgs(args...).
		WillReturnResult(sqlmock.NewResult(0, 3))

	require.NoError(t, repo.CreateBatch(entries))
	require.NoError(t, mock.ExpectationsWereMet())
	require.Len(t, given, 3)
	for i, entry := range entries {
		assert.Same(t, entry, given[i])
		assert.NotEqual(t, uuid.Nil, entry.ID)
		assert.False(t, entry.Timestamp.IsZero())
	}
}

func TestAuditLogCreateBatch_AFailedStatementGivesNothingToOnCreate(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := NewAuditLogRepository(db)

	called := false
	repo.OnCreate(func(*domain.AuditLog) { called = true })
	mock.ExpectExec(`INSERT INTO audit_logs`).WillReturnError(errors.New("connection refused"))

	err = repo.CreateBatch(batchEntries(2))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "0 of 2 written")
	assert.False(t, called)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAuditLogCreateBatch_SplitsALargeBatchIntoStatements(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := NewAuditLogRepository(db)

	mock.ExpectExec(`INSERT INTO audit_logs`).WillReturnResult(sqlmock.NewResult(0, auditLogBatchRows))
	mock.ExpectExec(`INSERT INTO audit_logs`).WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repo.CreateBatch(batchEntries(auditLogBatchRows+1)))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAuditLogCreateBatch_Empty(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, NewAuditLogRepository(db).CreateBatch(nil))
	require.NoError(t, mock.ExpectationsWereMet())
}
