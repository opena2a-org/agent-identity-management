package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// A user asks for the deletion of their own account: the store deactivates it
// and records the request, and the service writes the audit row. The purge
// deletes the accounts whose request is domain.AccountDeletionDelay old, batch
// by batch, and writes one audit row for each deleted account whose
// organization is kept.

type accountDeletionStoreFake struct {
	requestedAt time.Time
	requestOrg  uuid.UUID
	requestErr  error

	batches     [][]domain.AccountDeletion
	purgeErr    error // returned with the last batch
	requestedBy []time.Time
	limits      []int
}

func (f *accountDeletionStoreFake) RequestDeletion(_ context.Context, userID uuid.UUID, at time.Time) (domain.AccountDeletion, error) {
	f.requestedAt = at
	if f.requestErr != nil {
		return domain.AccountDeletion{}, f.requestErr
	}
	return domain.AccountDeletion{UserID: userID, OrganizationID: f.requestOrg, RequestedAt: at}, nil
}

func (f *accountDeletionStoreFake) PurgeDeletionRequests(_ context.Context, requestedBy time.Time, limit int) ([]domain.AccountDeletion, error) {
	f.requestedBy = append(f.requestedBy, requestedBy)
	f.limits = append(f.limits, limit)
	if len(f.batches) == 0 {
		return nil, f.purgeErr
	}
	batch := f.batches[0]
	f.batches = f.batches[1:]
	if len(f.batches) == 0 {
		return batch, f.purgeErr
	}
	return batch, nil
}

type accountDeletionAuditRow struct {
	orgID, userID uuid.UUID
	action        domain.AuditAction
	resourceType  string
	resourceID    uuid.UUID
	ip, userAgent string
	metadata      map[string]interface{}
}

type accountDeletionAuditFake struct {
	rows []accountDeletionAuditRow
	err  error
}

func (f *accountDeletionAuditFake) LogAction(_ context.Context, orgID, userID uuid.UUID, action domain.AuditAction, resourceType string,
	resourceID uuid.UUID, ipAddress, userAgent string, metadata map[string]interface{}) error {
	f.rows = append(f.rows, accountDeletionAuditRow{orgID, userID, action, resourceType, resourceID, ipAddress, userAgent, metadata})
	return f.err
}

var accountDeletionNow = time.Date(2026, 10, 19, 12, 30, 0, 0, time.UTC)

func accountDeletionServiceFor(store *accountDeletionStoreFake, audit *accountDeletionAuditFake) *AccountDeletionService {
	return &AccountDeletionService{store: store, audit: audit, now: func() time.Time { return accountDeletionNow }}
}

func accountDeletionsOf(n int, orgID uuid.UUID, requestedAt time.Time) []domain.AccountDeletion {
	out := make([]domain.AccountDeletion, n)
	for i := range out {
		out[i] = domain.AccountDeletion{UserID: uuid.New(), OrganizationID: orgID, RequestedAt: requestedAt}
	}
	return out
}

func TestAccountDeletionService_RequestRecordsTheRequestAndAnAuditRow(t *testing.T) {
	orgID, userID := uuid.New(), uuid.New()
	store := &accountDeletionStoreFake{requestOrg: orgID}
	audit := &accountDeletionAuditFake{}

	deletion, err := accountDeletionServiceFor(store, audit).Request(context.Background(), userID, "198.51.100.7", "test-agent")
	require.NoError(t, err)

	assert.True(t, store.requestedAt.Equal(accountDeletionNow), "the request is recorded at the service's clock")
	assert.Equal(t, userID, deletion.UserID)
	assert.Equal(t, orgID, deletion.OrganizationID)
	assert.True(t, deletion.DueAt().Equal(accountDeletionNow.Add(29*24*time.Hour)), "the account is due 29 days after the request")

	require.Len(t, audit.rows, 1)
	row := audit.rows[0]
	assert.Equal(t, orgID, row.orgID, "the row is in the organization the store read")
	assert.Equal(t, userID, row.userID)
	assert.Equal(t, domain.AuditActionAccountDeletionRequested, row.action)
	assert.Equal(t, "user", row.resourceType)
	assert.Equal(t, userID, row.resourceID)
	assert.Equal(t, "198.51.100.7", row.ip)
	assert.Equal(t, "test-agent", row.userAgent)
	assert.Equal(t, map[string]interface{}{"deletionDueAt": "2026-11-17T12:30:00Z"}, row.metadata)
}

func TestAccountDeletionService_ARefusedRequestWritesNoAuditRow(t *testing.T) {
	for _, refusal := range []error{domain.ErrLastActiveAdmin, domain.ErrAccountDeactivated, domain.ErrUserNotFound} {
		store := &accountDeletionStoreFake{requestErr: refusal}
		audit := &accountDeletionAuditFake{}

		_, err := accountDeletionServiceFor(store, audit).Request(context.Background(), uuid.New(), "", "")
		assert.True(t, errors.Is(err, refusal), "got %v, want %v", err, refusal)
		assert.Empty(t, audit.rows, "a refused request writes no audit row (%v)", refusal)
	}
}

func TestAccountDeletionService_AFailedAuditWriteKeepsTheRequest(t *testing.T) {
	store := &accountDeletionStoreFake{requestOrg: uuid.New()}
	audit := &accountDeletionAuditFake{err: errors.New("audit table locked")}

	deletion, err := accountDeletionServiceFor(store, audit).Request(context.Background(), uuid.New(), "", "")
	require.NoError(t, err, "the request is recorded; only its audit row failed")
	assert.True(t, deletion.RequestedAt.Equal(accountDeletionNow))
}

func TestAccountDeletionService_PurgeDeletesTheRequestsMadeTheDelayAgo(t *testing.T) {
	orgID := uuid.New()
	requestedAt := accountDeletionNow.Add(-30 * 24 * time.Hour)
	deleted := accountDeletionsOf(2, orgID, requestedAt)
	store := &accountDeletionStoreFake{batches: [][]domain.AccountDeletion{deleted}}
	audit := &accountDeletionAuditFake{}

	n, err := accountDeletionServiceFor(store, audit).Purge(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, n)

	require.Len(t, store.requestedBy, 1)
	assert.True(t, store.requestedBy[0].Equal(accountDeletionNow.Add(-29*24*time.Hour)),
		"the purge deletes the requests made 29 days ago or earlier, got %s", store.requestedBy[0])
	assert.Equal(t, []int{100}, store.limits)

	require.Len(t, audit.rows, 2)
	for i, row := range audit.rows {
		assert.Equal(t, orgID, row.orgID)
		assert.Equal(t, uuid.Nil, row.userID, "the purge's row names no user")
		assert.Equal(t, domain.AuditActionAccountDeleted, row.action)
		assert.Equal(t, "user", row.resourceType)
		assert.Equal(t, deleted[i].UserID, row.resourceID, "the row's resource is the deleted user")
		assert.Equal(t, map[string]interface{}{"deletionRequestedAt": "2026-09-19T12:30:00Z"}, row.metadata)
	}
}

// An account whose organization the store deleted with it gets no audit row:
// the organization's audit rows went with it. A log line names the user and the
// organization. The account of an organization that is kept gets its row.
func TestAccountDeletionService_PurgeWritesNoAuditRowInAnOrganizationItDeleted(t *testing.T) {
	deletedOrg, keptOrg := uuid.New(), uuid.New()
	requestedAt := accountDeletionNow.Add(-30 * 24 * time.Hour)
	lastUsers := accountDeletionsOf(2, deletedOrg, requestedAt)
	for i := range lastUsers {
		lastUsers[i].OrganizationDeleted = true
	}
	member := accountDeletionsOf(1, keptOrg, requestedAt)[0]
	store := &accountDeletionStoreFake{batches: [][]domain.AccountDeletion{{lastUsers[0], member, lastUsers[1]}}}
	audit := &accountDeletionAuditFake{}
	logs := captureLog(t)

	n, err := accountDeletionServiceFor(store, audit).Purge(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 3, n, "every deleted account is counted")

	require.Len(t, audit.rows, 1, "only the account of the kept organization has an audit row")
	assert.Equal(t, keptOrg, audit.rows[0].orgID)
	assert.Equal(t, member.UserID, audit.rows[0].resourceID)

	for _, d := range lastUsers {
		assert.Contains(t, logs.String(),
			"Account deletion: user "+d.UserID.String()+" was deleted, and with it organization "+deletedOrg.String()+
				", which had no user left")
	}
	assert.NotContains(t, logs.String(), member.UserID.String())
	assert.NotContains(t, logs.String(), "audit row", "no audit write was attempted and failed")
}

func TestAccountDeletionService_PurgeRepeatsWhileABatchIsFull(t *testing.T) {
	orgID := uuid.New()
	requestedAt := accountDeletionNow.Add(-40 * 24 * time.Hour)
	store := &accountDeletionStoreFake{batches: [][]domain.AccountDeletion{
		accountDeletionsOf(100, orgID, requestedAt),
		accountDeletionsOf(100, orgID, requestedAt),
		accountDeletionsOf(3, orgID, requestedAt),
	}}
	audit := &accountDeletionAuditFake{}

	n, err := accountDeletionServiceFor(store, audit).Purge(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 203, n)
	assert.Len(t, store.requestedBy, 3, "a full batch is followed by another, and the batch of 3 ends the purge")
	assert.Len(t, audit.rows, 203)
}

func TestAccountDeletionService_PurgeStopsAtAFailureAndCountsWhatItDeleted(t *testing.T) {
	orgID := uuid.New()
	store := &accountDeletionStoreFake{
		batches:  [][]domain.AccountDeletion{accountDeletionsOf(100, orgID, accountDeletionNow), accountDeletionsOf(1, orgID, accountDeletionNow)},
		purgeErr: errors.New("connection reset"),
	}
	audit := &accountDeletionAuditFake{}

	n, err := accountDeletionServiceFor(store, audit).Purge(context.Background())
	require.Error(t, err)
	assert.Equal(t, 101, n)
	assert.Len(t, store.requestedBy, 2)
	assert.Len(t, audit.rows, 101, "each account deleted before the failure has its row")
}

func TestAccountDeletionService_WithoutAnAuditService(t *testing.T) {
	store := &accountDeletionStoreFake{
		requestOrg: uuid.New(),
		batches:    [][]domain.AccountDeletion{accountDeletionsOf(1, uuid.New(), accountDeletionNow)},
	}
	svc := NewAccountDeletionService(store, nil)

	_, err := svc.Request(context.Background(), uuid.New(), "", "")
	require.NoError(t, err)
	n, err := svc.Purge(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, n)
}
