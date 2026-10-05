package repository

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// The statements' shape. Their behaviour against a real table, including the
// exact window boundaries, is in agent_request_nonce_integration_test.go.

func TestAgentRequestNonceStatements_ReadOnlyTheDatabaseStatementClock(t *testing.T) {
	for name, stmt := range map[string]string{"admission": admitActionRequestNonceSQL, "purge": purgeStatement} {
		lower := strings.ToLower(stmt)
		assert.Contains(t, lower, "statement_timestamp()", name)
		// now() and CURRENT_TIMESTAMP are the transaction's start, not the statement's.
		assert.NotContains(t, lower, "now()", name)
		assert.NotContains(t, lower, "current_timestamp", name)
		assert.NotContains(t, lower, "clock_timestamp()", name)
	}
	// The signed timestamp reaches SQL as a typed parameter, never as text
	// PostgreSQL would parse ('now', 'infinity', 'epoch').
	assert.Contains(t, admitActionRequestNonceSQL, "$4::timestamptz AS signed_at")
	// Refused on conflict, never an upsert.
	assert.Contains(t, admitActionRequestNonceSQL, "ON CONFLICT (agent_id, nonce) DO NOTHING")
	assert.NotContains(t, strings.ToUpper(admitActionRequestNonceSQL), "DO UPDATE")
	// Every purge carries the organization predicate.
	assert.Contains(t, purgeStatement, "WHERE organization_id = $1")
}

var (
	setAdmissionTimeout = `^` + regexp.QuoteMeta(`SET LOCAL statement_timeout = '5000ms'`) + `$`
	admissionQuery      = `^` + regexp.QuoteMeta(strings.TrimSpace(admitActionRequestNonceSQL)) + `$`
	purgeQuery          = `^` + regexp.QuoteMeta(strings.TrimSpace(purgeStatement)) + `$`
)

func admissionMock(t *testing.T) (*AgentRequestNonceRepository, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	t.Cleanup(func() { assert.NoError(t, mock.ExpectationsWereMet()) })
	return NewAgentRequestNonceRepository(db), mock
}

// Admission runs alone in its transaction, bounded by SET LOCAL (never a
// session setting on a pooled connection), and maps the statement's verdict.
func TestAgentRequestNonceAdmit_MapsTheVerdict(t *testing.T) {
	agentID, orgID := uuid.New(), uuid.New()
	nonce := make([]byte, 16)
	signedAt := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)

	for _, tc := range []struct {
		side     string
		inserted int64
		want     domain.ActionRequestAdmission
	}{
		{"inside", 1, domain.ActionRequestAdmitted},
		{"inside", 0, domain.ActionRequestNonceReused},
		{"behind", 0, domain.ActionRequestBehindClock},
		{"ahead", 0, domain.ActionRequestAheadOfClock},
	} {
		repo, mock := admissionMock(t)
		mock.ExpectBegin()
		mock.ExpectExec(setAdmissionTimeout).WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectQuery(admissionQuery).
			WithArgs(agentID, orgID, nonce, signedAt, domain.ActionRequestWindowSeconds).
			WillReturnRows(sqlmock.NewRows([]string{"side", "count"}).AddRow(tc.side, tc.inserted))
		mock.ExpectCommit()

		got, err := repo.Admit(context.Background(), agentID, orgID, nonce, signedAt)
		require.NoError(t, err)
		assert.Equal(t, tc.want, got, "%s/%d", tc.side, tc.inserted)
	}
}

func TestAgentRequestNonceAdmit_ErrorsAdmitNothing(t *testing.T) {
	agentID, orgID := uuid.New(), uuid.New()

	repo, mock := admissionMock(t)
	mock.ExpectBegin()
	mock.ExpectExec(setAdmissionTimeout).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(admissionQuery).WillReturnError(&pq.Error{Code: "57014", Message: "canceling statement due to statement timeout"})
	mock.ExpectRollback()
	_, err := repo.Admit(context.Background(), agentID, orgID, make([]byte, 16), time.Now())
	assert.ErrorIs(t, err, domain.ErrActionRequestAdmissionTimedOut)

	repo, mock = admissionMock(t)
	mock.ExpectBegin()
	mock.ExpectExec(setAdmissionTimeout).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(admissionQuery).WillReturnError(errors.New("connection reset"))
	mock.ExpectRollback()
	_, err = repo.Admit(context.Background(), agentID, orgID, make([]byte, 16), time.Now())
	require.ErrorContains(t, err, "connection reset")
	assert.NotErrorIs(t, err, domain.ErrActionRequestAdmissionTimedOut)

	// Without the bound, the statement is never run.
	repo, mock = admissionMock(t)
	mock.ExpectBegin()
	mock.ExpectExec(setAdmissionTimeout).WillReturnError(errors.New("bad setting"))
	mock.ExpectRollback()
	_, err = repo.Admit(context.Background(), agentID, orgID, make([]byte, 16), time.Now())
	assert.Error(t, err)

	// A failed commit admits nothing.
	repo, mock = admissionMock(t)
	mock.ExpectBegin()
	mock.ExpectExec(setAdmissionTimeout).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(admissionQuery).
		WillReturnRows(sqlmock.NewRows([]string{"side", "count"}).AddRow("inside", int64(1)))
	mock.ExpectCommit().WillReturnError(errors.New("commit failed"))
	_, err = repo.Admit(context.Background(), agentID, orgID, make([]byte, 16), time.Now())
	assert.Error(t, err)
}

func TestAgentRequestNoncePurgeOrganization_DeletesPastExpiresAtPlusK(t *testing.T) {
	orgID := uuid.New()
	repo, mock := admissionMock(t)
	mock.ExpectExec(purgeQuery).
		WithArgs(orgID, domain.ActionRequestNonceRetentionSeconds).
		WillReturnResult(sqlmock.NewResult(0, 4))

	deleted, err := repo.PurgeOrganization(context.Background(), orgID)
	require.NoError(t, err)
	assert.Equal(t, int64(4), deleted)
}
