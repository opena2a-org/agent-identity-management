package repository

import (
	"context"
	"database/sql/driver"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// The statements' shape and parameters. What they do to a real table,
// including two claims racing for the same rows, is in
// registry_atc_revocation_integration_test.go.

func TestRegistryATCRevocationStatements_ReadOnlyTheDatabaseStatementClock(t *testing.T) {
	for name, stmt := range map[string]string{
		"store": storeRegistryATCRevocationSQL,
		"claim": claimRegistryATCRevocationsSQL,
		"retry": retryRegistryATCRevocationSQL,
	} {
		lower := strings.ToLower(stmt)
		assert.Contains(t, lower, "statement_timestamp()", name)
		// now() and CURRENT_TIMESTAMP are the transaction's start, and
		// clock_timestamp() moves within the statement.
		assert.NotContains(t, lower, "now()", name)
		assert.NotContains(t, lower, "current_timestamp", name)
		assert.NotContains(t, lower, "clock_timestamp()", name)
	}
	// A claim leaves the rows another claim is taking to it.
	assert.Contains(t, claimRegistryATCRevocationsSQL, "FOR UPDATE SKIP LOCKED")
	// An answer to a request acts only on the request it answers.
	for name, stmt := range map[string]string{"delete": deleteRegistryATCRevocationSQL, "retry": retryRegistryATCRevocationSQL} {
		assert.Contains(t, stmt, "WHERE agent_id = $1 AND request_id = $2", name)
	}
}

var (
	storeRegistryATCRevocationQuery  = `^` + regexp.QuoteMeta(strings.TrimSpace(storeRegistryATCRevocationSQL)) + `$`
	claimRegistryATCRevocationsQuery = `^` + regexp.QuoteMeta(strings.TrimSpace(claimRegistryATCRevocationsSQL)) + `$`
	deleteRegistryATCRevocationQuery = `^` + regexp.QuoteMeta(strings.TrimSpace(deleteRegistryATCRevocationSQL)) + `$`
	retryRegistryATCRevocationQuery  = `^` + regexp.QuoteMeta(strings.TrimSpace(retryRegistryATCRevocationSQL)) + `$`
)

func registryATCRevocationMock(t *testing.T) (*RegistryATCRevocationRepository, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	t.Cleanup(func() { assert.NoError(t, mock.ExpectationsWereMet()) })
	return NewRegistryATCRevocationRepository(db), mock
}

// requestIDArg matches a UUID parameter and records it.
type requestIDArg struct {
	seen *[]string
}

func (a requestIDArg) Match(v driver.Value) bool {
	s, ok := v.(string)
	if !ok {
		return false
	}
	if _, err := uuid.Parse(s); err != nil {
		return false
	}
	*a.seen = append(*a.seen, s)
	return true
}

// Each Store names the agent and a request ID of its own, so storing again
// replaces the request an earlier claim holds.
func TestRegistryATCRevocationStore_NamesTheAgentAndANewRequestID(t *testing.T) {
	repo, mock := registryATCRevocationMock(t)
	agentID := uuid.New()
	var requestIDs []string
	for range 2 {
		mock.ExpectExec(storeRegistryATCRevocationQuery).
			WithArgs(agentID, requestIDArg{seen: &requestIDs}).
			WillReturnResult(sqlmock.NewResult(0, 1))
	}

	require.NoError(t, repo.Store(context.Background(), agentID))
	require.NoError(t, repo.Store(context.Background(), agentID))

	require.Len(t, requestIDs, 2)
	assert.NotEqual(t, requestIDs[0], requestIDs[1])
}

func TestRegistryATCRevocationClaim_SendsTheLimitAndTheHoldInSecondsAndReadsEachRequest(t *testing.T) {
	repo, mock := registryATCRevocationMock(t)
	first, second := uuid.New(), uuid.New()
	firstRequest, secondRequest := uuid.New(), uuid.New()
	requestedAt := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	mock.ExpectQuery(claimRegistryATCRevocationsQuery).
		WithArgs(16, float64(60)).
		WillReturnRows(sqlmock.NewRows([]string{"agent_id", "request_id", "requested_at", "attempts"}).
			AddRow(first.String(), firstRequest.String(), requestedAt, 1).
			AddRow(second.String(), secondRequest.String(), requestedAt.Add(time.Second), 3))

	claimed, err := repo.Claim(context.Background(), 16, time.Minute)

	require.NoError(t, err)
	assert.Equal(t, []domain.RegistryATCRevocation{
		{AgentID: first, RequestID: firstRequest, RequestedAt: requestedAt, Attempts: 1},
		{AgentID: second, RequestID: secondRequest, RequestedAt: requestedAt.Add(time.Second), Attempts: 3},
	}, claimed)
}

func TestRegistryATCRevocationClaim_ReturnsAnEmptyListWhenNothingIsDue(t *testing.T) {
	repo, mock := registryATCRevocationMock(t)
	mock.ExpectQuery(claimRegistryATCRevocationsQuery).
		WithArgs(16, float64(60)).
		WillReturnRows(sqlmock.NewRows([]string{"agent_id", "request_id", "requested_at", "attempts"}))

	claimed, err := repo.Claim(context.Background(), 16, time.Minute)

	require.NoError(t, err)
	assert.NotNil(t, claimed)
	assert.Empty(t, claimed)
}

func TestRegistryATCRevocationDeleteAndRetry_NameTheClaimedRequest(t *testing.T) {
	repo, mock := registryATCRevocationMock(t)
	claimed := domain.RegistryATCRevocation{AgentID: uuid.New(), RequestID: uuid.New(), Attempts: 2}
	mock.ExpectExec(retryRegistryATCRevocationQuery).
		WithArgs(claimed.AgentID, claimed.RequestID, float64(30)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(deleteRegistryATCRevocationQuery).
		WithArgs(claimed.AgentID, claimed.RequestID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	require.NoError(t, repo.Retry(context.Background(), claimed, 30*time.Second))
	require.NoError(t, repo.Delete(context.Background(), claimed))
}

func TestRegistryATCRevocationRepository_ReturnsTheDatabaseError(t *testing.T) {
	unavailable := errors.New("database unavailable")
	claimed := domain.RegistryATCRevocation{AgentID: uuid.New(), RequestID: uuid.New(), Attempts: 1}

	t.Run("store", func(t *testing.T) {
		repo, mock := registryATCRevocationMock(t)
		mock.ExpectExec(storeRegistryATCRevocationQuery).WillReturnError(unavailable)
		assert.ErrorIs(t, repo.Store(context.Background(), claimed.AgentID), unavailable)
	})
	t.Run("claim", func(t *testing.T) {
		repo, mock := registryATCRevocationMock(t)
		mock.ExpectQuery(claimRegistryATCRevocationsQuery).WillReturnError(unavailable)
		claimedRows, err := repo.Claim(context.Background(), 16, time.Minute)
		assert.ErrorIs(t, err, unavailable)
		assert.Nil(t, claimedRows)
	})
	t.Run("retry", func(t *testing.T) {
		repo, mock := registryATCRevocationMock(t)
		mock.ExpectExec(retryRegistryATCRevocationQuery).WillReturnError(unavailable)
		assert.ErrorIs(t, repo.Retry(context.Background(), claimed, time.Second), unavailable)
	})
	t.Run("delete", func(t *testing.T) {
		repo, mock := registryATCRevocationMock(t)
		mock.ExpectExec(deleteRegistryATCRevocationQuery).WillReturnError(unavailable)
		assert.ErrorIs(t, repo.Delete(context.Background(), claimed), unavailable)
	})
}
