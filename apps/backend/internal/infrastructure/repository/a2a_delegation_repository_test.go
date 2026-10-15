package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// The SQL itself runs against Postgres in
// a2a_delegation_repository_integration_test.go; these tests pin the error
// mapping and the organization guards, which a mock can.

func delegationMock(t *testing.T) (*A2ADelegationRepository, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	t.Cleanup(func() { assert.NoError(t, mock.ExpectationsWereMet()) })
	return NewA2ADelegationRepository(db), mock
}

var a2aDelegationTestColumns = []string{
	"id", "organization_id", "delegator_agent_id", "delegate_agent_id", "skill_ids", "max_chain_depth",
	"trust_attenuation_factor", "min_delegated_trust_score", "can_delegate_further", "requires_consent",
	"is_active", "valid_from", "valid_until", "created_by", "created_at", "updated_at",
}

func TestA2ADelegationCreate_ASecondActiveDelegationForThePairIsErrA2ADelegationActive(t *testing.T) {
	repo, mock := delegationMock(t)
	mock.ExpectQuery(`INSERT INTO a2a_delegation`).
		WillReturnError(&pq.Error{Code: "23505", Constraint: activeA2ADelegationIndex})

	err := repo.Create(context.Background(), &domain.A2ADelegation{OrganizationID: uuid.New()})
	assert.ErrorIs(t, err, domain.ErrA2ADelegationActive)
}

func TestA2ADelegationCreate_AnotherUniqueViolationIsNotMappedToTheActivePair(t *testing.T) {
	repo, mock := delegationMock(t)
	mock.ExpectQuery(`INSERT INTO a2a_delegation`).
		WillReturnError(&pq.Error{Code: "23505", Constraint: "a2a_delegation_pkey"})

	err := repo.Create(context.Background(), &domain.A2ADelegation{OrganizationID: uuid.New()})
	require.Error(t, err)
	assert.NotErrorIs(t, err, domain.ErrA2ADelegationActive)
}

func TestA2ADelegationCreate_ReadsTheStoredRowBack(t *testing.T) {
	repo, mock := delegationMock(t)
	org, delegator, delegate, creator := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	d := &domain.A2ADelegation{
		ID: uuid.New(), OrganizationID: org, DelegatorAgentID: delegator, DelegateAgentID: delegate,
		MaxChainDepth: 3, TrustAttenuationFactor: 0.8, MinDelegatedTrustScore: 0.3,
		IsActive: true, ValidFrom: now, CreatedBy: &creator,
	}
	mock.ExpectQuery(`INSERT INTO a2a_delegation`).
		WithArgs(d.ID, org, delegator, delegate, []byte(`[]`), 3, 0.8, 0.3, false, true, now, sql.NullTime{},
			uuid.NullUUID{UUID: creator, Valid: true}).
		WillReturnRows(sqlmock.NewRows(a2aDelegationTestColumns).AddRow(
			d.ID.String(), org.String(), delegator.String(), delegate.String(), []byte(`[]`), 3, 0.8, 0.3, false, true,
			true, now, nil, creator.String(), now, now))

	require.NoError(t, repo.Create(context.Background(), d))
	assert.True(t, d.RequiresConsent, "the column default is read back")
	assert.Equal(t, []string{}, d.SkillIDs)
	assert.Equal(t, now, d.CreatedAt)
	assert.Nil(t, d.ValidUntil)
}

func TestA2ADelegationRevoke_ADelegationOutsideTheOrganizationIsNotFound(t *testing.T) {
	repo, mock := delegationMock(t)
	org, id := uuid.New(), uuid.New()
	mock.ExpectQuery(`UPDATE a2a_delegation\s+SET is_active = FALSE, updated_at = NOW\(\)\s+WHERE id = \$1 AND organization_id = \$2`).
		WithArgs(id, org).
		WillReturnError(sql.ErrNoRows)

	_, err := repo.Revoke(context.Background(), org, id)
	assert.ErrorIs(t, err, domain.ErrA2ADelegationNotFound)
}

func TestA2ADelegationReads_RefuseANilOrganizationBeforeQuerying(t *testing.T) {
	repo, _ := delegationMock(t)
	ctx := context.Background()

	_, err := repo.GetByIDs(ctx, uuid.Nil, []uuid.UUID{uuid.New()})
	assert.ErrorContains(t, err, "requires a caller organization")
	_, err = repo.ListByOrganization(ctx, uuid.Nil, 10, 0)
	assert.ErrorContains(t, err, "requires a caller organization")
	_, err = repo.Revoke(ctx, uuid.Nil, uuid.New())
	assert.ErrorContains(t, err, "requires a caller organization")

	found, err := repo.GetByIDs(ctx, uuid.New(), nil)
	assert.NoError(t, err)
	assert.Empty(t, found)
}

func TestA2ADelegationGetByIDs_ScopesTheQueryToTheOrganization(t *testing.T) {
	repo, mock := delegationMock(t)
	org, id := uuid.New(), uuid.New()
	mock.ExpectQuery(`WHERE organization_id = \$1 AND id = ANY\(\$2::uuid\[\]\)`).
		WithArgs(org, pq.Array([]string{id.String()})).
		WillReturnRows(sqlmock.NewRows(a2aDelegationTestColumns))

	found, err := repo.GetByIDs(context.Background(), org, []uuid.UUID{id})
	require.NoError(t, err)
	assert.Empty(t, found)
}
