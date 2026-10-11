package repository

import (
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// OnCreate is how the server forwards audit log entries to a SIEM: the
// function it registers is given each entry Create has written.

func TestAuditLogCreate_GivesTheWrittenEntryToOnCreate(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := NewAuditLogRepository(db)

	var given []*domain.AuditLog
	repo.OnCreate(func(entry *domain.AuditLog) { given = append(given, entry) })

	mock.ExpectExec(`INSERT INTO audit_logs`).WillReturnResult(sqlmock.NewResult(0, 1))
	entry := &domain.AuditLog{OrganizationID: uuid.New(), Action: domain.AuditActionLogin, ResourceType: "user", ResourceID: uuid.New()}
	require.NoError(t, repo.Create(entry))

	require.Len(t, given, 1)
	assert.Same(t, entry, given[0])
	assert.NotEqual(t, uuid.Nil, given[0].ID, "the entry is given with the id it was written with")
	assert.False(t, given[0].Timestamp.IsZero(), "and the time it was written with")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAuditLogCreate_DoesNotGiveAFailedWriteToOnCreate(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := NewAuditLogRepository(db)

	called := false
	repo.OnCreate(func(*domain.AuditLog) { called = true })

	mock.ExpectExec(`INSERT INTO audit_logs`).WillReturnError(errors.New("connection refused"))
	err = repo.Create(&domain.AuditLog{OrganizationID: uuid.New(), Action: domain.AuditActionLogin})

	require.Error(t, err)
	assert.False(t, called)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAuditLogCreate_WithoutOnCreate(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := NewAuditLogRepository(db)

	mock.ExpectExec(`INSERT INTO audit_logs`).WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repo.Create(&domain.AuditLog{OrganizationID: uuid.New(), Action: domain.AuditActionLogin}))
	require.NoError(t, mock.ExpectationsWereMet())
}
